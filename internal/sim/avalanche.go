package sim

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

const (
	avyRunoutSlope   = float32(0.25)    // threshold: above → gains momentum; below → deposits
	avyTreeStop      = float32(0.70)    // wave halts when tree density exceeds this
	avyHopsPerSec    = float32(2.0)     // front advances 2 cells/wall-second ≈ 10 m/s
	avyMomentumGain  = float32(2.0)     // momentum change per unit slope vs runout threshold
	avyMomentumMax   = float32(8.0)     // cap on wave momentum
	avyMomentumStart = float32(4.0)     // a slab breaking away is already moving
	avySlabShare     = float32(0.8)     // of the top layer that breaks away
	avyLateralSpread = float32(0.30)    // allow spread to neighbours up to this slope uphill
	avyDebrisMark    = float32(0.003)   // share of the wave left on each cell of its track
	avyMinSnow       = float32(0.00001) // wave dies when SWE in transit drops below this
	// avyStopSnow is the SWE below which the wave stops where it is, once
	// the ground's too gentle to start a slide, and below which a share
	// of it rejoins the main flow instead of branching off: a slide runs
	// as a stream and piles where it stops, rather than spreading over
	// the slope in a film.
	avyStopSnow = float32(0.03)
	// avyChannel sharpens the wave's preference for the steepest way
	// down: each neighbour's share goes as its weight to this power, so
	// the slide runs in its gully and fans out only where the ground
	// levels off and every way is about as good.
	avyChannel = 2
	// A wave moving fast on steep ground picks up avyEntrain of the
	// surface snow it runs over, at most avyEntrainMax SWE a cell and
	// avyEntrainFlow of the snow running through it: a slide grows down
	// its track (several times over, as big slides do) and scours it.
	avyEntrain     = float32(0.5)
	avyEntrainMax  = float32(0.08)
	avyEntrainFlow = float32(0.1)
	// avyRunoutBrake is how much harder the wave slows on ground flatter
	// than avyRunoutSlope than it speeds up on steeper: it stops in a
	// short runout and piles its debris there.
	avyRunoutBrake = float32(4)
	// avyInertia is how much a wave at full momentum favours the way it's
	// heading over the ground's fall line (0 to 1); avyAheadSlack lets
	// it veer: a neighbour 90° off still gets a share.
	avyInertia    = float32(0.8)
	avyAheadSlack = float32(0.3)
)

// avySqrt2 is the distance multiplier for diagonal Moore-neighbourhood steps.
const avySqrt2 = float32(1.4142136)

// startAvalanche lifts a fraction of the top snow layer into transit and adds
// this cell to the active avalanche front. The wave spreads downhill each tick
// via tickAvalanche. Returns false when the cell had too little snow to release.
func (s *Simulation) startAvalanche(t *world.Terrain, x, z int) bool {
	c := &t.Cells[x][z]
	if c.Top.Accumulation < avyMinSnow {
		return false
	}
	// The slab is the top layer: most of it breaks away, a skin stays on
	// the bed surface it slid on.
	released := c.Top.Accumulation * avySlabShare
	c.Top.Accumulation -= released
	c.Grooming = 0
	c.MogulSize = 0
	c.AvySnow = released
	c.AvyMomentum = avyMomentumStart
	c.AvyDirX, c.AvyDirZ = t.FallLineAt(x, z)
	c.AvyTick = s.avyGen
	s.avyFront = append(s.avyFront, [2]int{x, z})
	t.MarkSnowDirty(x, z)
	return true
}

// tickAvalanche advances all in-flight avalanche fronts by dt seconds,
// and catches anyone the snow reaches.
func (s *Simulation) tickAvalanche(dt float64) {
	if len(s.World.AvyShots) > 0 || len(s.World.AvyBursts) > 0 {
		s.pruneAvyEffects()
	}
	if len(s.avyFront) == 0 {
		return
	}
	defer s.catchInAvalanche()
	t := s.World.Terrain
	s.avyBudget += float32(dt) * avyHopsPerSec
	for s.avyBudget >= 1.0 && len(s.avyFront) > 0 {
		s.avyBudget -= 1.0
		s.avyGen++
		s.avyNext = s.avyNext[:0]
		for _, pos := range s.avyFront {
			s.spreadAvyCell(t, pos[0], pos[1])
		}
		s.avyFront, s.avyNext = s.avyNext, s.avyFront[:0]
	}
	s.World.AvyFront = append(s.World.AvyFront[:0], s.avyFront...)
	if len(s.avyFront) == 0 {
		s.slumpAvyDebris(t)
	}
}

// Debris slumps where a slide left it: the wave drops its snow cell by
// cell, often piling the rest in one cell where it stops (against trees,
// in a hollow), and real debris spreads into a cone and lobes. Once a
// slide has stopped, wherever a cell's debris stands more than
// debrisStep deeper than a neighbour's, part of the difference slides
// across, over debrisSlumpPasses passes.
const (
	debrisStep        = float32(0.2) // metres of depth between neighbours
	debrisSlumpPasses = 30
)

// slumpAvyDebris spreads the debris the slides just stopped left.
func (s *Simulation) slumpAvyDebris(t *world.Terrain) {
	if len(s.avyDebris) == 0 {
		return
	}
	// In the order they were reached, each once, so a seeded run slumps
	// the same way every time.
	seen := map[[2]int]bool{}
	var cells [][2]int
	add := func(c [2]int) {
		if !seen[c] {
			seen[c] = true
			cells = append(cells, c)
		}
	}
	for _, c := range s.avyDebris {
		add(c)
	}
	s.avyDebris = s.avyDebris[:0]
	density := world.KindDensity(world.KindAvalancheDebris)
	for range debrisSlumpPasses {
		var next [][2]int
		for _, c := range cells {
			cell := &t.Cells[c[0]][c[1]]
			for dx := -1; dx <= 1; dx++ {
				for dz := -1; dz <= 1; dz++ {
					nx, nz := c[0]+dx, c[1]+dz
					if (dx == 0 && dz == 0) || !t.InBounds(nx, nz) {
						continue
					}
					nb := &t.Cells[nx][nz]
					if !nb.Passable {
						continue
					}
					// Down or across the slope only: the pile's top is above
					// the neighbour's snow.
					top := cell.GroundElevation + cell.VisibleSnowDepth()
					if top <= nb.GroundElevation+nb.VisibleSnowDepth() {
						continue
					}
					step := debrisStep
					if dx != 0 && dz != 0 {
						step *= avySqrt2
					}
					excess := cell.DebrisDepth() - nb.DebrisDepth() - step
					if excess <= 0 {
						continue
					}
					// A quarter of the excess a pass: eight neighbours share.
					if moved := cell.TakeDebris(excess * density / 4); moved > 0 {
						nb.AddDebris(moved)
						t.MarkSnowDirty(nx, nz)
						next = append(next, [2]int{nx, nz})
					}
				}
			}
			t.MarkSnowDirty(c[0], c[1])
		}
		for _, c := range next {
			add(c)
		}
	}
}

// spreadAvyCell processes one cell in the active front: deposits debris at this
// cell, then distributes the remaining snow across eligible Moore neighbours.
func (s *Simulation) spreadAvyCell(t *world.Terrain, x, z int) {
	c := &t.Cells[x][z]
	snow := c.AvySnow
	momentum := c.AvyMomentum
	dir := mgl32.Vec2{c.AvyDirX, c.AvyDirZ}
	if l := dir.Len(); l > 0 {
		dir = dir.Mul(1 / l)
	}
	c.AvySnow = 0
	c.AvyMomentum = 0
	c.AvyDirX, c.AvyDirZ = 0, 0

	// ── Entrain the snow it runs over ─────────────────────────────────────────
	if c.Slope > avyRunoutSlope && momentum >= 1 && c.Debris == 0 {
		if take := min(c.Top.Accumulation*avyEntrain, avyEntrainMax, snow*avyEntrainFlow); take > 0 {
			c.Top.Accumulation -= take
			snow += take
		}
	}

	// ── Deposit at this cell ──────────────────────────────────────────────────
	// Deposit fraction rises as slope drops: almost nothing on very steep
	// terrain (snow keeps moving), building up through mid-slope, and
	// depositing heavily in the runout. Momentum delays deposition —
	// a fast-moving wave carries snow further before it settles.
	//
	// slopeFrac: 0 on ground steep enough to release (AvySlopeMin), 1 at
	// the runout threshold and below.
	slopeFrac := clamp01((world.AvySlopeMin - c.Slope) / (world.AvySlopeMin - avyRunoutSlope))
	// Momentum reduces deposition — fast wave passes more snow downslope.
	momentumFrac := float32(1.0) - momentum/avyMomentumMax
	if momentumFrac < 0 {
		momentumFrac = 0
	}
	depositFrac := avyDebrisMark + (1.0-avyDebrisMark)*slopeFrac*momentumFrac
	deposit := snow * depositFrac
	passing := snow - deposit

	c.AddDebris(deposit)
	s.avyDebris = append(s.avyDebris, [2]int{x, z})
	t.MarkSnowDirty(x, z)

	if passing < avyMinSnow {
		return
	}

	// ── Spread to Moore neighbourhood ─────────────────────────────────────────
	type candidate struct {
		nx, nz int
		weight float32
		slope  float32
	}
	var cands [8]candidate
	n := 0
	totalWeight := float32(0)

	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			if dx == 0 && dz == 0 {
				continue
			}
			nx, nz := x+dx, z+dz
			if !t.InBounds(nx, nz) {
				continue
			}
			nc := &t.Cells[nx][nz]
			if !nc.Passable || nc.TreeCover() > avyTreeStop {
				continue
			}
			dist := float32(CellSize)
			if dx != 0 && dz != 0 {
				dist *= avySqrt2
			}
			drop := c.GroundElevation - nc.GroundElevation
			slope := drop / dist
			if slope < -avyLateralSpread {
				continue // too far uphill
			}
			// Shifted so flat neighbours get a small share, then sharpened.
			weight := slope + avyLateralSpread
			for range avyChannel - 1 {
				weight *= slope + avyLateralSpread
			}
			// A fast wave keeps going the way it was: on flat ground it
			// runs on in a tongue instead of spreading every way.
			ahead := (float32(dx)*dir[0] + float32(dz)*dir[1]) * CellSize / dist
			keepOn := max(ahead+avyAheadSlack, 0)
			weight *= 1 + (keepOn*keepOn-1)*min(momentum/avyMomentumMax, 1)*avyInertia
			cands[n] = candidate{nx, nz, weight, slope}
			n++
			totalWeight += weight
		}
	}

	if totalWeight <= 0 || (passing < avyStopSnow && c.Slope < world.AvySlopeMin) {
		// Nowhere to go, or too little to run on once the ground eases:
		// the rest piles here.
		c.AddDebris(passing)
		t.MarkSnowDirty(x, z)
		return
	}

	// Shares too thin to run on stay with the main flow: drop them and
	// share their weight among the rest (the biggest always stays).
	var best float32
	for _, cd := range cands[:n] {
		best = max(best, cd.weight)
	}
	kept := 0
	for _, cd := range cands[:n] {
		if passing*cd.weight/totalWeight >= avyStopSnow || cd.weight == best {
			cands[kept] = cd
			kept++
		}
	}
	n, totalWeight = kept, 0
	for _, cd := range cands[:n] {
		totalWeight += cd.weight
	}

	for _, cd := range cands[:n] {
		p := cd.weight / totalWeight
		childSnow := passing * p
		gain := (cd.slope - avyRunoutSlope) * avyMomentumGain
		if gain < 0 {
			gain *= avyRunoutBrake
		}
		newMom := momentum + gain
		if newMom < 0 {
			newMom = 0
		}
		if newMom > avyMomentumMax {
			newMom = avyMomentumMax
		}

		nc := &t.Cells[cd.nx][cd.nz]
		nc.AvySnow += childSnow
		step := mgl32.Vec2{float32(cd.nx - x), float32(cd.nz - z)}.Normalize()
		nc.AvyDirX += step[0] * childSnow
		nc.AvyDirZ += step[1] * childSnow
		if newMom > nc.AvyMomentum {
			nc.AvyMomentum = newMom
		}
		if nc.AvyTick < s.avyGen {
			nc.AvyTick = s.avyGen
			s.avyNext = append(s.avyNext, [2]int{cd.nx, cd.nz})
		}
		nc.Grooming = 0
		nc.MogulSize = 0
		t.MarkSnowDirty(cd.nx, cd.nz)
	}
}
