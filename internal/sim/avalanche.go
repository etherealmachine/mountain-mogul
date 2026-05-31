package sim

import (
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

const (
	avyMaxChance        = float32(0.60)  // maximum stochastic release probability
	avyRunoutSlope      = float32(0.25)  // threshold: above → gains momentum; below → deposits
	avyTreeStop         = float32(0.70)  // wave halts when tree density exceeds this
	avyHopsPerSec       = float32(2.0)   // front advances 2 cells/wall-second ≈ 10 m/s
	avyMomentumGain     = float32(2.0)   // momentum change per unit slope vs runout threshold
	avyMomentumMax      = float32(8.0)   // cap on wave momentum
	avyLateralSpread    = float32(0.30)  // allow spread to neighbours up to this slope uphill
	avyDebrisMark       = float32(0.02)  // minimum SWE left as a debris marker on steep cells
	avyMinSnow          = float32(0.00001) // wave dies when SWE in transit drops below this
)

// avySqrt2 is the distance multiplier for diagonal Moore-neighbourhood steps.
const avySqrt2 = float32(1.4142136)

// checkAvalanches identifies unstable cells and triggers releases. Called from
// applyDailyWeather on heavy-snow and rain days.
func (s *Simulation) checkAvalanches() {
	t := s.World.Terrain
	for x := range t.Cells {
		for z := range t.Cells[x] {
			c := &t.Cells[x][z]
			score := c.InstabilityScore()
			if score < 1.0 {
				continue
			}
			chance := clamp32((score-1.0)*avyMaxChance, 0, avyMaxChance)
			if rng.Global().Float32() > chance {
				continue
			}
			s.startAvalanche(t, x, z)
		}
	}
}

// startAvalanche lifts a fraction of the top snow layer into transit and adds
// this cell to the active avalanche front. The wave spreads downhill each tick
// via tickAvalanche.
func (s *Simulation) startAvalanche(t *world.Terrain, x, z int) {
	c := &t.Cells[x][z]
	if c.Top.Accumulation < avyMinSnow {
		return
	}
	// Take half the top layer into transit; the rest stays as a debris marker.
	released := c.Top.Accumulation * 0.5
	c.Top.Accumulation -= released
	c.Top.Kind = world.KindAvalancheDebris
	c.Grooming = 0
	c.MogulSize = 0
	c.AvySnow = released
	c.AvyMomentum = 0
	c.AvyTick = s.avyGen
	s.avyFront = append(s.avyFront, [2]int{x, z})
	t.SnowDirty = true
}

// tickAvalanche advances all in-flight avalanche fronts by dt seconds.
func (s *Simulation) tickAvalanche(dt float64) {
	if len(s.avyFront) == 0 {
		return
	}
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
}

// spreadAvyCell processes one cell in the active front: deposits debris at this
// cell, then distributes the remaining snow across eligible Moore neighbours.
func (s *Simulation) spreadAvyCell(t *world.Terrain, x, z int) {
	c := &t.Cells[x][z]
	snow := c.AvySnow
	momentum := c.AvyMomentum
	c.AvySnow = 0
	c.AvyMomentum = 0

	// ── Deposit at this cell ──────────────────────────────────────────────────
	// Deposit fraction rises as slope drops: almost nothing on very steep
	// terrain (snow keeps moving), building up through mid-slope, and
	// depositing heavily in the runout. Momentum delays deposition —
	// a fast-moving wave carries snow further before it settles.
	//
	// slopeFrac: 0 at AvySlopeMax (very steep), 1 at runout threshold and below.
	slopeFrac := float32(1.0)
	if c.Slope > avyRunoutSlope {
		slopeFrac = 1.0 - (c.Slope-avyRunoutSlope)/(world.AvySlopeMax-avyRunoutSlope)
		if slopeFrac < 0 {
			slopeFrac = 0
		}
	}
	// Momentum reduces deposition — fast wave passes more snow downslope.
	momentumFrac := float32(1.0) - momentum/avyMomentumMax
	if momentumFrac < 0 {
		momentumFrac = 0
	}
	depositFrac := avyDebrisMark + (1.0-avyDebrisMark)*slopeFrac*momentumFrac
	deposit := snow * depositFrac
	passing := snow - deposit

	c.Top.Accumulation += deposit
	c.Top.Kind = world.KindAvalancheDebris
	c.Grooming = 0
	c.MogulSize = 0
	t.SnowDirty = true

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
			if !nc.Passable || nc.TreeDensity > avyTreeStop {
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
			weight := slope + avyLateralSpread // shift so flat neighbours get a small share
			cands[n] = candidate{nx, nz, weight, slope}
			n++
			totalWeight += weight
		}
	}

	if totalWeight <= 0 {
		// No eligible neighbours — wave has nowhere to go; pile remaining snow
		// here rather than lose it.
		c.Top.Accumulation += passing
		t.SnowDirty = true
		return
	}

	for _, cd := range cands[:n] {
		p := cd.weight / totalWeight
		childSnow := passing * p
		if childSnow < avyMinSnow {
			// Too thin to keep propagating — deposit at destination for conservation.
			nc := &t.Cells[cd.nx][cd.nz]
			nc.Top.Accumulation += childSnow
			nc.Top.Kind = world.KindAvalancheDebris
			nc.Grooming = 0
			nc.MogulSize = 0
			t.SnowDirty = true
			continue
		}
		newMom := momentum + (cd.slope-avyRunoutSlope)*avyMomentumGain
		if newMom < 0 {
			newMom = 0
		}
		if newMom > avyMomentumMax {
			newMom = avyMomentumMax
		}

		nc := &t.Cells[cd.nx][cd.nz]
		nc.AvySnow += childSnow
		if newMom > nc.AvyMomentum {
			nc.AvyMomentum = newMom
		}
		if nc.AvyTick < s.avyGen {
			nc.AvyTick = s.avyGen
			s.avyNext = append(s.avyNext, [2]int{cd.nx, cd.nz})
		}
		nc.Grooming = 0
		nc.MogulSize = 0
		t.SnowDirty = true
	}
}
