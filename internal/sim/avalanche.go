package sim

import (
	"math"

	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

const (
	avySlopeMin        = float32(0.55)  // rise/run ≈ 29° — below this, never releases
	avySlopeMax        = float32(1.40)  // rise/run ≈ 54° — above this, slopeExcess = 1.0
	avyMinSWE          = float32(0.10)  // minimum TotalSWE for a cell to be avalanche-prone
	avyLoadScale       = float32(0.30)  // TotalSWE that saturates the load factor at 1.0
	avyMaxChance       = float32(0.60)  // maximum stochastic release probability
	avyMaxHops         = 30             // hop limit per avalanche chain
	avyReleaseBase     = float32(0.60)  // fraction of Base SWE cleared on release
	avyHopDecay        = float32(0.85)  // fraction of transported SWE that continues each hop
	avyRunoutSlope     = float32(0.25)  // momentum drains when slope drops below this
	avyTreeStop        = float32(0.70)  // chain stops when tree density exceeds this
	avyHazardDecay     = float32(0.04)  // hazard intensity lost per sim-second
	avyHopsPerSec      = float32(2.0)   // avalanche front advances 2 cells/wall-second ≈ 10 m/s
	avyMomentumGain    = float32(2.0)   // momentum change per unit slope above/below runout threshold
	avyMomentumMax     = float32(4.0)   // cap on carried momentum
	avyMomentumTransfer = float32(0.10) // fraction of momentum that offsets uphill slope in spread weights
)

// avyItem is a single particle in a spreading avalanche front.
type avyItem struct {
	x, z     int
	swe      float32
	dx, dz   float32  // incoming travel direction — defines forward hemisphere for spreading
	momentum float32  // builds on steep terrain, drains on flat; carries chain through runout
}

// avyChain is an in-flight avalanche. The front is a set of avyItems that
// all advance one hop per tick. Each hop fans out laterally based on terrain
// slope, so the chain widens as it descends.
type avyChain struct {
	front    []avyItem
	visited  map[[2]int]bool
	hopsLeft int
	budget   float32 // fractional hops accumulated; advance one hop when ≥1
}

// kindAvalancheMult returns a snow-instability multiplier for the given kind.
func kindAvalancheMult(k world.SnowKind) float32 {
	switch k {
	case world.KindWindSlab, world.KindCrust:
		return 1.5
	case world.KindFrozenGranular:
		return 1.2
	case world.KindPowder, world.KindCement:
		return 1.0
	case world.KindSlush, world.KindCorn:
		return 0.8
	default: // PackedPowder, Boilerplate, Base, AvalancheDebris
		return 0.2
	}
}

// instabilityScore returns a score ≥1.0 for cells that qualify for release.
func instabilityScore(c *world.Cell, slope float32) float32 {
	if slope < avySlopeMin || c.TotalSWE() < avyMinSWE {
		return 0
	}
	slopeExcess := clamp32((slope-avySlopeMin)/(avySlopeMax-avySlopeMin), 0, 1)
	kindMult := float32(1.0)
	if top := c.TopLayer(); top != nil {
		kindMult = kindAvalancheMult(top.Kind)
	}
	treeAnchor := c.TreeDensity * 0.4
	return (c.TotalSWE() / avyLoadScale) * slopeExcess * kindMult * (1 - treeAnchor)
}

// checkAvalanches identifies unstable cells and triggers releases. Called from
// applyDailyWeather on heavy-snow and rain days.
func (s *Simulation) checkAvalanches() {
	t := s.World.Terrain
	h := s.avalancheHazard
	for x := range t.Cells {
		for z := range t.Cells[x] {
			c := &t.Cells[x][z]
			gx, gz := t.GradientAt(x, z)
			slope := float32(math.Sqrt(float64(gx*gx + gz*gz)))
			score := instabilityScore(c, slope)
			if score < 1.0 {
				continue
			}
			chance := clamp32((score-1.0)*avyMaxChance, 0, avyMaxChance)
			if rng.Global().Float32() > chance {
				continue
			}
			s.releaseAvalanche(t, h, x, z, gx, gz)
		}
	}
}

// releaseAvalanche clears the source cell's snow and enqueues a propagating
// chain. The chain advances hop-by-hop each tick via tickAvalancheChains,
// spreading laterally as it descends.
func (s *Simulation) releaseAvalanche(t *world.Terrain, h [][]float32, sx, sz int, gx, gz float32) {
	c := &t.Cells[sx][sz]
	clearedSWE := c.Top.Accumulation + c.Base*avyReleaseBase
	c.Top = world.SnowLayer{}
	c.Base *= 1 - avyReleaseBase
	c.Grooming = 0
	c.MogulSize = 0
	h[sx][sz] = 1.0
	t.SnowDirty = true

	if clearedSWE >= 0.001 {
		s.avyChains = append(s.avyChains, avyChain{
			front: []avyItem{{
				x: sx, z: sz,
				swe:      clearedSWE,
				dx:       -gx, dz: -gz,
				momentum: 0,
			}},
			visited:  map[[2]int]bool{{sx, sz}: true},
			hopsLeft: avyMaxHops,
		})
	}
}

// tickAvalancheChains advances each in-flight avalanche front by dt seconds.
// Each hop advances the entire front one step, spreading SWE laterally.
func (s *Simulation) tickAvalancheChains(dt float64) {
	t := s.World.Terrain
	h := s.avalancheHazard
	alive := s.avyChains[:0]
	for i := range s.avyChains {
		ch := &s.avyChains[i]
		ch.budget += float32(dt) * avyHopsPerSec
		for ch.budget >= 1.0 && ch.hopsLeft > 0 && len(ch.front) > 0 {
			ch.budget -= 1.0
			ch.hopsLeft--
			var next []avyItem
			for _, it := range ch.front {
				next = s.spreadAvyItem(t, h, it, ch.visited, next)
			}
			ch.front = next
		}
		if ch.hopsLeft > 0 && len(ch.front) > 0 {
			alive = append(alive, *ch)
		}
	}
	s.avyChains = alive
}

// spreadAvyItem fans one avyItem out to its qualifying forward neighbours,
// depositing debris and producing child items for the next hop. Returns the
// updated next slice.
func (s *Simulation) spreadAvyItem(
	t *world.Terrain, h [][]float32,
	it avyItem, visited map[[2]int]bool, next []avyItem,
) []avyItem {
	mag := float32(math.Sqrt(float64(it.dx*it.dx + it.dz*it.dz)))
	if mag < 1e-6 {
		return next
	}
	ndx, ndz := it.dx/mag, it.dz/mag

	type candidate struct {
		nx, nz int
		weight float32
	}
	var cands [4]candidate
	n := 0
	totalWeight := float32(0)

	for _, off := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, nz := it.x+off[0], it.z+off[1]
		if !t.InBounds(nx, nz) {
			continue
		}
		if visited[[2]int{nx, nz}] {
			continue
		}
		nc := &t.Cells[nx][nz]
		if !nc.Passable || nc.TreeDensity > avyTreeStop {
			continue
		}
		dot := float32(off[0])*ndx + float32(off[1])*ndz
		if dot <= 0 {
			continue // backward hemisphere — never spread uptrail
		}
		drop := t.Cells[it.x][it.z].GroundElevation - nc.GroundElevation
		slopeToN := drop / CellSize
		// Momentum offsets uphill: a fast-moving chain can crest a small rise.
		effective := slopeToN + it.momentum*avyMomentumTransfer
		if effective <= 0 {
			continue
		}
		w := dot * effective
		cands[n] = candidate{nx, nz, w}
		n++
		totalWeight += w
	}
	if totalWeight <= 0 {
		return next
	}

	for _, c := range cands[:n] {
		p := c.weight / totalWeight
		childSWE := it.swe * avyHopDecay * p
		if childSWE < 0.001 {
			continue
		}
		visited[[2]int{c.nx, c.nz}] = true

		nc := &t.Cells[c.nx][c.nz]
		deposit := it.swe * (1 - avyHopDecay) * p
		if nc.Top.Accumulation > 0 && nc.Top.Kind != world.KindAvalancheDebris {
			nc.Base += nc.Top.Accumulation
			nc.Top = world.SnowLayer{}
		}
		nc.Top.Accumulation += deposit
		nc.Top.Kind = world.KindAvalancheDebris
		nc.Grooming = 0
		nc.MogulSize = 0
		h[c.nx][c.nz] = 1.0
		t.SnowDirty = true

		drop := t.Cells[it.x][it.z].GroundElevation - t.Cells[c.nx][c.nz].GroundElevation
		newMom := it.momentum + (drop/CellSize-avyRunoutSlope)*avyMomentumGain
		if newMom < 0 {
			newMom = 0
		}
		if newMom > avyMomentumMax {
			newMom = avyMomentumMax
		}
		next = append(next, avyItem{
			x: c.nx, z: c.nz,
			swe:      childSWE,
			dx:       float32(c.nx - it.x),
			dz:       float32(c.nz - it.z),
			momentum: newMom,
		})
	}
	return next
}

// decayAvalancheHazard reduces the hazard map every sim tick.
func (s *Simulation) decayAvalancheHazard(dt float64) {
	dec := float32(dt) * avyHazardDecay
	for x := range s.avalancheHazard {
		for z := range s.avalancheHazard[x] {
			if v := s.avalancheHazard[x][z]; v > 0 {
				if v -= dec; v < 0 {
					v = 0
				}
				s.avalancheHazard[x][z] = v
			}
		}
	}
}
