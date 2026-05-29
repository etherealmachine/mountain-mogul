package sim

import (
	"math"

	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

const (
	avySlopeMin    = float32(0.55)  // rise/run ≈ 29° — below this, never releases
	avySlopeMax    = float32(1.40)  // rise/run ≈ 54° — above this, slopeExcess = 1.0
	avyMinSWE      = float32(0.10)  // minimum TotalSWE for a cell to be avalanche-prone
	avyLoadScale   = float32(0.30)  // TotalSWE that saturates the load factor at 1.0
	avyMaxChance   = float32(0.60)  // maximum stochastic release probability
	avyMaxHops     = 30             // BFS hop limit for debris flow
	avyReleaseBase = float32(0.60)  // fraction of Base SWE cleared on release
	avyHopDecay    = float32(0.85)  // fraction of transported SWE that continues each hop
	avyRunoutSlope = float32(0.25)  // BFS stops when slope drops below this (flat runout)
	avyTreeStop    = float32(0.70)  // BFS stops when tree density exceeds this
	avyHazardDecay = float32(0.04)  // hazard intensity lost per sim-second
)

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
	default: // PackedPowder, Boilerplate, Base
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

// releaseAvalanche clears the source cell's snow and propagates debris
// downhill via BFS, marking the hazard map along the path.
func (s *Simulation) releaseAvalanche(t *world.Terrain, h [][]float32, sx, sz int, gx, gz float32) {
	c := &t.Cells[sx][sz]
	clearedSWE := c.Top.Accumulation + c.Base*avyReleaseBase
	c.Top = world.SnowLayer{}
	c.Base *= 1 - avyReleaseBase
	c.Grooming = 0
	c.MogulSize = 0
	h[sx][sz] = 1.0
	t.SnowDirty = true

	type item struct {
		x, z     int
		swe      float32
		dx, dz   float32 // current downhill direction
	}
	queue := []item{{sx, sz, clearedSWE, -gx, -gz}}
	visited := make(map[[2]int]bool, avyMaxHops*2)
	visited[[2]int{sx, sz}] = true

	for hop := 0; hop < avyMaxHops && len(queue) > 0; hop++ {
		next := queue[:0]
		for _, it := range queue {
			nx, nz := downhillNeighbour(t, it.x, it.z, it.dx, it.dz)
			if nx < 0 {
				continue
			}
			key := [2]int{nx, nz}
			if visited[key] {
				continue
			}
			visited[key] = true

			nc := &t.Cells[nx][nz]
			if !nc.Passable || nc.TreeDensity > avyTreeStop {
				continue
			}

			deposit := it.swe * (1 - avyHopDecay)
			nc.Base += deposit
			h[nx][nz] = 1.0

			ngx, ngz := t.GradientAt(nx, nz)
			nSlope := float32(math.Sqrt(float64(ngx*ngx + ngz*ngz)))
			if nSlope < avyRunoutSlope {
				continue // flat runout — debris settles here
			}

			remaining := it.swe * avyHopDecay
			if remaining > 0.001 {
				next = append(next, item{nx, nz, remaining, -ngx, -ngz})
			}
		}
		queue = next
	}
}

// downhillNeighbour returns the cardinal neighbour cell (N/S/E/W) whose
// direction best matches (dx, dz), or (-1, -1) if no in-bounds neighbour
// exists.
func downhillNeighbour(t *world.Terrain, x, z int, dx, dz float32) (int, int) {
	mag := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if mag < 1e-6 {
		return -1, -1
	}
	dx /= mag
	dz /= mag
	bestX, bestZ, bestDot := -1, -1, float32(-1e9)
	for _, off := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, nz := x+off[0], z+off[1]
		if !t.InBounds(nx, nz) {
			continue
		}
		dot := float32(off[0])*dx + float32(off[1])*dz
		if dot > bestDot {
			bestDot = dot
			bestX, bestZ = nx, nz
		}
	}
	return bestX, bestZ
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
