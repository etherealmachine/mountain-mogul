package scene

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Road cell-state clearance radii. Measured as closest-sample distance
// from a cell vertex to the chain's Catmull-Rom polyline.
//
// Snow uses a two-band approach: inside the inner radius cells go to
// SnowAccumulation=0; between inner and outer the snow scales linearly with
// distance. The gradient blurs the grid-alignment artifact that a
// hard threshold produced (visible asymmetric clearing on roads whose
// centreline doesn't align to the 5 m cell grid — the cleared corridor
// would extend further to whichever side happened to have a vertex
// just inside the radius).
//
// Tree clearance stays binary; trees are large enough that a sharp
// canopy edge reads as intentional rather than artifactual.
const (
	roadSnowInnerRadius = float32(8.0)
	roadSnowOuterRadius = float32(14.0)
	roadTreeClearRadius = float32(14.0)
)

// applyRoadCellState walks every road chain in the world and stamps
// the carriageway footprint onto adjacent terrain cells: SnowAccumulation=0
// inside the snow band, and removes the trees inside the (wider) tree band.
//
// Idempotent — running it twice produces the same final state, so the
// cheapest correct call pattern is "run after any change to the road
// graph, and once on scene load." Doesn't touch ground elevation,
// grooming, packed, ice, or moguls; just snow + trees.
//
// Note this is intentionally one-way: removing a road later doesn't
// restore the cells it cleared. That's fine for the current iteration
// — non-destructive restoration would need a snapshot of natural
// state, which we'll add later if it actually matters in play.
func applyRoadCellState(w *world.World) {
	t := w.Terrain
	for _, chain := range w.FindRoadChains() {
		samples := world.SampleRoadChain(chain, t, world.RoadChainSamplesPerSegment)
		if len(samples) < 2 {
			continue
		}
		applyChainCellState(t, samples)
	}
	applyFootpathCellState(w)
}

// Footpath clearance, beyond a path's half width: cells whose centre is
// within pathSnowInner of the path's edge are shovelled bare, with the
// snow coming back over the next pathSnowFalloff; trees within
// pathTreeClear of the edge are cut.
const (
	pathSnowInner   = float32(0)
	pathSnowFalloff = float32(3.0)
	pathTreeClear   = float32(1.0)
)

// applyFootpathCellState shovels the snow off every footpath and cuts
// the trees on it, as roads are plowed: after paths change, on load, and
// at each day rollover, so a storm's snow is cleared overnight. Like the
// roads, one-way: a removed path's cells stay as they were.
func applyFootpathCellState(w *world.World) {
	t := w.Terrain
	for _, f := range w.Footpaths {
		line := f.Centerline()
		if len(line) < 2 {
			continue
		}
		var reach float32
		for _, s := range line {
			reach = max(reach, s.Width/2)
		}
		reach += pathSnowInner + pathSnowFalloff + world.CellSize
		minX, maxX, minZ, maxZ := line[0].Pos[0], line[0].Pos[0], line[0].Pos[1], line[0].Pos[1]
		for _, s := range line {
			minX, maxX = min(minX, s.Pos[0]), max(maxX, s.Pos[0])
			minZ, maxZ = min(minZ, s.Pos[1]), max(maxZ, s.Pos[1])
		}
		// edgeDist is how far p is outside the path's edge (negative
		// inside it).
		edgeDist := func(p mgl32.Vec2) float32 {
			best := float32(math.MaxFloat32)
			for i := 1; i < len(line); i++ {
				cp := world.ClosestPointOnRoadSegment(p, line[i-1].Pos, line[i].Pos)
				hw := (line[i-1].Width + line[i].Width) / 4
				best = min(best, p.Sub(cp).Len()-hw)
			}
			return best
		}
		for x := max(int((minX-reach)/world.CellSize), 0); x <= min(int((maxX+reach)/world.CellSize), t.Width-1); x++ {
			for z := max(int((minZ-reach)/world.CellSize), 0); z <= min(int((maxZ+reach)/world.CellSize), t.Height-1); z++ {
				c := &t.Cells[x][z]
				centre := mgl32.Vec2{(float32(x) + 0.5) * world.CellSize, (float32(z) + 0.5) * world.CellSize}
				d := edgeDist(centre)
				if c.TreeCount > 0 && d <= pathTreeClear+world.CellSize {
					t.RemoveTreesIn(x, z, x, z, func(tr world.Tree) bool {
						return edgeDist(mgl32.Vec2{tr.X, tr.Z}) <= pathTreeClear
					})
				}
				switch {
				case d <= pathSnowInner:
					c.Base = 0
					c.Top = world.SnowLayer{}
				case d <= pathSnowInner+pathSnowFalloff:
					blend := (d - pathSnowInner) / pathSnowFalloff
					c.Base *= blend
					c.Top.Accumulation *= blend
				}
			}
		}
	}
}

// applyChainCellState clears snow + trees on cells near one chain's
// sampled curve. Closest-sample distance is the same approximation
// the road-effects pass used: cheap, slightly overestimates the true
// curve-perpDist by ~1 sample spacing, harmless at our radii.
func applyChainCellState(t *world.Terrain, samples []mgl32.Vec2) {
	const cellSize = float32(5.0)

	treeR2 := roadTreeClearRadius * roadTreeClearRadius
	treeReach := roadTreeClearRadius + cellSize*math.Sqrt2/2
	treeReach2 := treeReach * treeReach
	snowInnerR2 := roadSnowInnerRadius * roadSnowInnerRadius
	snowOuterR2 := roadSnowOuterRadius * roadSnowOuterRadius
	snowFalloff := roadSnowOuterRadius - roadSnowInnerRadius

	// Sample bbox expanded by the wider (tree) radius.
	minX := samples[0][0]
	maxX := samples[0][0]
	minZ := samples[0][1]
	maxZ := samples[0][1]
	for _, s := range samples[1:] {
		if s[0] < minX {
			minX = s[0]
		}
		if s[0] > maxX {
			maxX = s[0]
		}
		if s[1] < minZ {
			minZ = s[1]
		}
		if s[1] > maxZ {
			maxZ = s[1]
		}
	}
	minX -= treeReach
	maxX += treeReach
	minZ -= treeReach
	maxZ += treeReach

	x0 := int(minX / cellSize)
	x1 := int(maxX/cellSize) + 1
	z0 := int(minZ / cellSize)
	z1 := int(maxZ/cellSize) + 1

	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			// Snow is measured from the CELL CENTER. Measuring from the
			// corner produced a half-cell side-bias: cells whose corners
			// sat on the +X / +Z side of the road always tested closer,
			// so the corridor cleared visibly further on that side.
			cx := (float32(x) + 0.5) * cellSize
			cz := (float32(z) + 0.5) * cellSize
			d2 := roadDistSq(mgl32.Vec2{cx, cz}, samples)

			// Trees are measured from their own trunks; any cell whose
			// centre is within a half-diagonal of the band may hold one.
			if d2 <= treeReach2 && t.Cells[x][z].TreeCount > 0 {
				t.RemoveTreesIn(x, z, x, z, func(tr world.Tree) bool {
					return roadDistSq(mgl32.Vec2{tr.X, tr.Z}, samples) <= treeR2
				})
			}
			if d2 > treeR2 {
				continue
			}

			switch {
			case d2 <= snowInnerR2:
				t.Cells[x][z].Base = 0
				t.Cells[x][z].Top = world.SnowLayer{}
			case d2 <= snowOuterR2:
				// Linear falloff between inner and outer — distance is
				// the sqrt of d2, paid once per cell in the falloff band.
				d := float32(math.Sqrt(float64(d2)))
				blend := (d - roadSnowInnerRadius) / snowFalloff
				if blend < 0 {
					blend = 0
				} else if blend > 1 {
					blend = 1
				}
				t.Cells[x][z].Base *= blend
				t.Cells[x][z].Top.Accumulation *= blend
			}
		}
	}
}

// roadDistSq is the squared distance from p to the sampled road
// polyline, projecting onto each segment. Symmetric across the curve
// (closest-sample wasn't: points on the inside of a curve sit close to
// several samples and got over-cleared, while points on the outside at
// the same perpendicular distance got under-cleared).
func roadDistSq(p mgl32.Vec2, samples []mgl32.Vec2) float32 {
	best := float32(math.MaxFloat32)
	for i := 0; i < len(samples)-1; i++ {
		cp := world.ClosestPointOnRoadSegment(p, samples[i], samples[i+1])
		dx, dz := p[0]-cp[0], p[1]-cp[1]
		if d := dx*dx + dz*dz; d < best {
			best = d
		}
	}
	return best
}
