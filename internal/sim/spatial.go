package sim

import "mountain-mogul/internal/world"

// spatialCellSize is the agent-bucket grid resolution in metres. Picked
// so that 3×3 neighbour cells (= ±1 cell from the query point) fully
// cover any radius up to spatialCellSize — currently skierHazardRadius
// (2.5 m), which is the max distance hazardDensityAt cares about.
const spatialCellSize = 5.0

// spatialGrid is a flat 2D bucket of agents keyed by world cell. Built
// every sim step from w.OnMountain, so positions are never more than
// one step old at any game speed; queried by hazardDensityAt for each
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
	buckets       [][]*world.Guest
	used          []int // buckets filled since the last reset
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
		buckets: make([][]*world.Guest, w*h),
	}
}

// reset empties the filled buckets, keeping their capacity for reuse.
// Only the buckets in use are touched: the map has ~150k of them and a
// reset runs every step.
func (g *spatialGrid) reset() {
	for _, i := range g.used {
		g.buckets[i] = g.buckets[i][:0]
	}
	g.used = g.used[:0]
}

// insert buckets one agent by its XZ position. Out-of-bounds agents
// (shouldn't exist post-spawn but guarded just in case) are dropped.
func (g *spatialGrid) insert(a *world.Guest) {
	cx, cz, ok := g.cellOf(a.Pos[0], a.Pos[2])
	if !ok {
		return
	}
	idx := cz*g.width + cx
	if len(g.buckets[idx]) == 0 {
		g.used = append(g.used, idx)
	}
	g.buckets[idx] = append(g.buckets[idx], a)
}

// rebuild clears and refills the grid from agents. Convenience wrapper.
func (g *spatialGrid) rebuild(agents []*world.Guest) {
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

// forEachWithin invokes fn on every agent bucketed in a cell that overlaps
// the square of half-side r around (x, z); callers filter by distance.
func (g *spatialGrid) forEachWithin(x, z, r float32, fn func(a *world.Guest)) {
	x0, z0 := max(int((x-r)/spatialCellSize), 0), max(int((z-r)/spatialCellSize), 0)
	x1, z1 := min(int((x+r)/spatialCellSize), g.width-1), min(int((z+r)/spatialCellSize), g.height-1)
	for nz := z0; nz <= z1; nz++ {
		row := nz * g.width
		for nx := x0; nx <= x1; nx++ {
			for _, a := range g.buckets[row+nx] {
				fn(a)
			}
		}
	}
}

// forEachNear invokes fn on every agent in the 3×3 cell neighbourhood
// of world position (x, z). Skips out-of-bounds query points (no
// neighbours to visit). Agents are visited at most once. Out-of-bounds
// neighbour cells are skipped silently.
func (g *spatialGrid) forEachNear(x, z float32, fn func(a *world.Guest)) {
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
