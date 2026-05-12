package scene

import (
	"math"

	"mountain-mogul/internal/world"
)

// ── Road ground effects ───────────────────────────────────────────────
//
// Roads are processed as chains (Catmull-Rom curves through degree-2
// node sequences) rather than individual edges. Each chain stamps three
// concentric bands onto the terrain:
//
//   - the carriageway band (perpDist ≤ RoadHalfWidth): ground is
//     flattened to a linear interpolation between the two endpoint
//     elevations, snow plowed, surface hard-packed and ungroomed,
//     trees removed.
//   - a soft margin (next ~6 m): smoothstep blend back to natural
//     elevation/snow; trees still cleared.
//   - an outer tree-clearance band (next ~4 m): trees only.
//
// Lives in its own file so the road treatment can iterate without
// touching the lift apron or the generic pad helper.
//
// Cell-coordinate convention: cells[x][z] is the value at the terrain
// MESH VERTEX at world (x*cellSize, z*cellSize). Bilinear interpolation
// reads cells the same way, so the cell-to-world conversion below is
// `float32(x) * cellSize` with no +0.5 offset.

const (
	// roadSoftMargin extends the elevation/snow falloff this far past
	// the carriageway edge. The full-clearance zone
	// (roadInnerFraction × (halfWidth+softMargin)) needs to cover the
	// worst-case sampled vertex by the road quad's bilinear AND
	// absorb the closest-sample overestimate (~1.5 m for our 16-sample
	// chain resolution). Bumped from 9 → 12 so the falloff zone has
	// room to ramp gently (4.5 m wide instead of 2.4 m) — gentler
	// snowbanks at the road edge read less "buried" at oblique
	// camera angles.
	roadSoftMargin = float32(12.0)
	// roadTreeMargin extends tree clearance this far past the soft
	// margin so branches don't visibly invade the road space.
	roadTreeMargin = float32(4.0)
	// roadInnerFraction is the smoothstep threshold inside the road
	// band: the inner core holds the carriageway target until
	// perpDist/(halfWidth+softMargin) crosses this, then blends out.
	// Higher value = wider full-clearance band, narrower falloff
	// (steeper snowbank slope at the edge).
	roadInnerFraction = float32(0.8)
)

// applyAllRoadChainEffects re-stamps the ground effects for every road
// chain in the world. Call after any placement-time edit to the road
// graph: Catmull-Rom curves through neighbouring nodes change whenever
// a chain grows or splits, so per-edge incremental stamping wouldn't
// stay accurate. Re-stamping the whole network on each placement is
// O(chains × samples × cells_in_bbox), fast enough for our scales.
//
// Endpoint elevations are sampled from the live terrain at apply time;
// for a freshly-placed road this is natural ground, for a re-stamp
// it's whatever was set by the previous pass. Endpoint cells therefore
// don't drift across re-stamps (weight=1 at the endpoint reproduces
// the sampled value exactly).
func applyAllRoadChainEffects(w *world.World) {
	t := w.Terrain
	for _, chain := range w.FindRoadChains() {
		applyRoadChainEffects(t, chain)
	}
}

// applyRoadChainEffects stamps a single chain's curve onto the terrain.
// The chain is sampled as a fine Catmull-Rom polyline; for each cell
// in the chain's bbox the closest sample is found, and its perpDist +
// along-fraction drive the same three-band treatment used by the older
// per-edge code.
func applyRoadChainEffects(t *world.Terrain, chain world.RoadChain) {
	samples := world.SampleRoadChain(chain, world.RoadChainSamplesPerSegment)
	if len(samples) < 2 {
		return
	}
	cumDist := world.CumulativeChainDist(samples)
	totalLen := cumDist[len(cumDist)-1]
	if totalLen < 0.5 {
		return
	}

	// Endpoint elevations frame the linear-interp target along the chain.
	// Sampled BEFORE the cell loop modifies anything, so an earlier
	// chain's endpoints won't bleed into this chain mid-pass.
	elevStart := t.InterpolatedGroundElevationAt(samples[0][0], samples[0][1])
	elevEnd := t.InterpolatedGroundElevationAt(samples[len(samples)-1][0], samples[len(samples)-1][1])

	const cellSize = float32(5.0)
	halfWidth := world.RoadHalfWidth
	softEdge := halfWidth + roadSoftMargin
	treeEdge := softEdge + roadTreeMargin

	// Bbox of all samples + tree-clear margin.
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
	minX -= treeEdge
	maxX += treeEdge
	minZ -= treeEdge
	maxZ += treeEdge

	x0 := int(minX / cellSize)
	x1 := int(maxX/cellSize) + 1
	z0 := int(minZ / cellSize)
	z1 := int(maxZ/cellSize) + 1

	treeEdge2 := treeEdge * treeEdge

	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			cx := float32(x) * cellSize
			cz := float32(z) * cellSize

			// Find the closest curve sample for this cell. Linear scan
			// is fine at our chain sizes; if chains grow huge later,
			// stripe the bbox into per-sample tiles and only test
			// cells against samples in their neighbourhood.
			closestIdx := -1
			var closestD2 float32 = treeEdge2 + 1
			for i, s := range samples {
				dx := cx - s[0]
				dz := cz - s[1]
				d2 := dx*dx + dz*dz
				if d2 < closestD2 {
					closestD2 = d2
					closestIdx = i
				}
			}
			if closestIdx < 0 {
				continue
			}
			perpDist := float32(math.Sqrt(float64(closestD2)))
			if perpDist > treeEdge {
				continue
			}

			// Outer band — trees-only, no ground or snow change.
			if perpDist > softEdge {
				t.Cells[x][z].TreeDensity = 0
				continue
			}

			// Inside the soft-edge band: apply the road surface with a
			// smoothstep weight. Inner zone holds full weight; outer
			// ramps to 0 at softEdge.
			frac := perpDist / softEdge
			w := 1 - smoothstep32(roadInnerFraction, 1.0, frac)
			if w <= 0 {
				t.Cells[x][z].TreeDensity = 0
				continue
			}

			alongFrac := cumDist[closestIdx] / totalLen
			target := elevStart + (elevEnd-elevStart)*alongFrac

			cell := &t.Cells[x][z]
			cell.GroundElevation = cell.GroundElevation + (target-cell.GroundElevation)*w
			cell.SnowDepth = cell.SnowDepth * (1 - w)
			if w > cell.Packed {
				cell.Packed = w
			}
			cell.Grooming *= 1 - w
			cell.MogulSize *= 1 - w
			cell.Ice *= 1 - w
			cell.TreeDensity = 0
		}
	}
}
