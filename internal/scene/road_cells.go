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
// SnowDepth=0; between inner and outer the snow scales linearly with
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
// the carriageway footprint onto adjacent terrain cells: SnowDepth=0
// inside the snow band, TreeDensity=0 inside the (wider) tree band.
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
}

// applyChainCellState clears snow + trees on cells near one chain's
// sampled curve. Closest-sample distance is the same approximation
// the road-effects pass used: cheap, slightly overestimates the true
// curve-perpDist by ~1 sample spacing, harmless at our radii.
func applyChainCellState(t *world.Terrain, samples []mgl32.Vec2) {
	const cellSize = float32(5.0)

	treeR2 := roadTreeClearRadius * roadTreeClearRadius
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
	minX -= roadTreeClearRadius
	maxX += roadTreeClearRadius
	minZ -= roadTreeClearRadius
	maxZ += roadTreeClearRadius

	x0 := int(minX / cellSize)
	x1 := int(maxX/cellSize) + 1
	z0 := int(minZ / cellSize)
	z1 := int(maxZ/cellSize) + 1

	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			// Reference point is the CELL CENTER — same anchor the
			// renderer uses to place tree instances ((x+0.5)*cellSize).
			// Measuring from the corner produced a half-cell side-bias:
			// cells whose corners sat on the +X / +Z side of the road
			// always tested closer, so the corridor cleared visibly
			// further on that side than the other.
			cx := (float32(x) + 0.5) * cellSize
			cz := (float32(z) + 0.5) * cellSize
			cell := mgl32.Vec2{cx, cz}

			// Point-to-segment distance — treat the sample polyline as
			// the actual sequence of line segments and project onto
			// each. Symmetric across the curve (closest-sample wasn't:
			// cells on the inside of a curve sit close to multiple
			// samples and got over-cleared, while cells on the outside
			// at the same perpendicular distance got under-cleared).
			var d2 float32 = treeR2 + 1
			for i := 0; i < len(samples)-1; i++ {
				cp := world.ClosestPointOnRoadSegment(cell, samples[i], samples[i+1])
				dx := cx - cp[0]
				dz := cz - cp[1]
				sd := dx*dx + dz*dz
				if sd < d2 {
					d2 = sd
				}
			}
			if d2 > treeR2 {
				continue
			}
			t.Cells[x][z].TreeDensity = 0

			switch {
			case d2 <= snowInnerR2:
				t.Cells[x][z].SnowDepth = 0
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
				t.Cells[x][z].SnowDepth *= blend
			}
		}
	}
}
