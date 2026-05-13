package sim

import "mountain-mogul/internal/world"

// spatialCellSize is the agent-bucket grid resolution in metres. Picked
// so that 3×3 neighbour cells (= ±1 cell from the query point) fully
// cover any radius up to spatialCellSize — currently skierHazardRadius
// (2.5 m), which is the max distance hazardDensityAt cares about.
const spatialCellSize = 5.0

// spatialGrid is a flat 2D bucket of agents keyed by world cell. Built
// once per Tick from w.Agents; queried by hazardDensityAt for each
// candidate-arc sample point. Replaces the O(N) full-agent iteration
// inside the L1 sampler with O(neighbours) — the per-substep work
// for sampleTactical drops from O(168 × N²) to O(168 × N × k) where k
// is average neighbours per query (~few).
//
// Underlying storage is a single flat slice indexed by z*width+x; the
// per-cell bucket slices reuse their backing arrays across ticks via
// [:0] resets, so steady-state usage is allocation-free.
type spatialGrid struct {
	width, height int
	buckets       [][]*world.Agent
}

// newSpatialGrid sizes the grid to cover the given terrain extent in
// metres. Slightly oversized (+1 cell each axis) so floor-rounding at
// the boundary doesn't drop agents at exactly the maximum coordinate.
func newSpatialGrid(widthM, heightM float32) *spatialGrid {
	w := int(widthM/spatialCellSize) + 1
	h := int(heightM/spatialCellSize) + 1
	return &spatialGrid{
		width:   w,
		height:  h,
		buckets: make([][]*world.Agent, w*h),
	}
}

// reset clears every bucket while keeping the underlying capacity for
// reuse. Called at the top of each Tick before re-inserting agents.
func (g *spatialGrid) reset() {
	for i := range g.buckets {
		g.buckets[i] = g.buckets[i][:0]
	}
}

// insert buckets one agent by its XZ position. Out-of-bounds agents
// (shouldn't exist post-spawn but guarded just in case) are dropped.
func (g *spatialGrid) insert(a *world.Agent) {
	cx, cz, ok := g.cellOf(a.Pos[0], a.Pos[2])
	if !ok {
		return
	}
	idx := cz*g.width + cx
	g.buckets[idx] = append(g.buckets[idx], a)
}

// rebuild clears and refills the grid from agents. Convenience wrapper.
func (g *spatialGrid) rebuild(agents []*world.Agent) {
	g.reset()
	for _, a := range agents {
		g.insert(a)
	}
}

// cellOf maps world XZ → grid cell. Returns ok=false for out-of-bounds.
func (g *spatialGrid) cellOf(x, z float32) (int, int, bool) {
	if x < 0 || z < 0 {
		return 0, 0, false
	}
	cx := int(x / spatialCellSize)
	cz := int(z / spatialCellSize)
	if cx >= g.width || cz >= g.height {
		return 0, 0, false
	}
	return cx, cz, true
}

// forEachNear invokes fn on every agent in the 3×3 cell neighbourhood
// of world position (x, z). Skips out-of-bounds query points (no
// neighbours to visit). Agents are visited at most once. Out-of-bounds
// neighbour cells are skipped silently.
func (g *spatialGrid) forEachNear(x, z float32, fn func(a *world.Agent)) {
	cx, cz, ok := g.cellOf(x, z)
	if !ok {
		return
	}
	zMin, zMax := cz-1, cz+1
	if zMin < 0 {
		zMin = 0
	}
	if zMax >= g.height {
		zMax = g.height - 1
	}
	xMin, xMax := cx-1, cx+1
	if xMin < 0 {
		xMin = 0
	}
	if xMax >= g.width {
		xMax = g.width - 1
	}
	for nz := zMin; nz <= zMax; nz++ {
		row := nz * g.width
		for nx := xMin; nx <= xMax; nx++ {
			for _, a := range g.buckets[row+nx] {
				fn(a)
			}
		}
	}
}
