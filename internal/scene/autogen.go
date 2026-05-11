package scene

import (
	"math"

	"mountain-mogul/internal/world"
)

// elevFields caches the per-cell quantities that depend only on
// GroundElevation: MFD flow accumulation, discrete Laplacian (curvature),
// and the overall elevation range. Both the snow and forest auto-generators
// consume these fields, and the flow-accumulation pass is the most expensive
// part of either generator (O(N log N) sort + propagation), so caching it
// across slider tweaks lets the auto tool re-run interactively without
// recomputing every drag-frame.
//
// The cache is tied to a specific terrain instance and a specific snapshot
// of its elevation. Any caller that mutates GroundElevation (raise/lower
// brushes, terrain import) must invalidate the cache by dropping the
// pointer; tree-density and snow-state edits leave it valid.
type elevFields struct {
	flow       []float32
	curv       []float32
	minE, maxE float32
}

// computeElevFields runs the elevation-derived passes once. Use this at the
// edge of a hot loop and reuse the returned struct across multiple
// generator invocations.
func computeElevFields(t *world.Terrain) *elevFields {
	minE, maxE := elevRange(t)
	return &elevFields{
		flow: flowAccumulation(t),
		curv: curvature(t),
		minE: minE,
		maxE: maxE,
	}
}

// elevRange returns the (min, max) of GroundElevation across the terrain.
// Kept separate from computeElevFields so the auto-generators can call it
// directly when they don't have a cached struct on hand.
func elevRange(t *world.Terrain) (float32, float32) {
	minE, maxE := float32(math.Inf(1)), float32(math.Inf(-1))
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			e := t.Cells[x][z].GroundElevation
			if e < minE {
				minE = e
			}
			if e > maxE {
				maxE = e
			}
		}
	}
	return minE, maxE
}
