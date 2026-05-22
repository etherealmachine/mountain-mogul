package world

import (
	"fmt"
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
)

// EdgeKind tags what kind of entity a TrailEdge endpoint refers to.
type EdgeKind uint8

const (
	KindLiftTop  EdgeKind = iota // lift top station — guest just unloaded
	KindLiftBase                 // lift queue — guest heads here to board
	KindBuilding                 // lodge or parking lot
	KindTrail                    // trail-to-trail junction
)

// Trail is a named ski run defined by the grid cells it covers. Players
// paint cells interactively; the simulation derives connectivity from
// whichever entity footprints overlap those cells.
type Trail struct {
	ID         uint64
	Name       string
	Difficulty TerrainDifficulty
	Groomed    bool     // true ⇒ nearest shed services this trail automatically
	Cells      [][2]int
}

// ContainsCell reports whether grid cell (cx, cz) belongs to this trail.
// O(n) linear scan — suitable for per-tick use given typical trail sizes.
func (t *Trail) ContainsCell(cx, cz int) bool {
	for _, c := range t.Cells {
		if c[0] == cx && c[1] == cz {
			return true
		}
	}
	return false
}

// NearestCellCenter returns the world-space XZ centre of the trail cell
// closest to (x, z), and true. Returns zero vector and false when the
// trail has no cells.
func (t *Trail) NearestCellCenter(x, z float32) (wx, wz float32, ok bool) {
	if len(t.Cells) == 0 {
		return 0, 0, false
	}
	best := float32(math.MaxFloat32)
	for _, c := range t.Cells {
		cx := (float32(c[0]) + 0.5) * CellSize
		cz := (float32(c[1]) + 0.5) * CellSize
		dx := cx - x
		dz := cz - z
		if d2 := dx*dx + dz*dz; d2 < best {
			best = d2
			wx, wz = cx, cz
		}
	}
	return wx, wz, true
}

// cellSet returns a map of the trail's cells for O(1) membership testing.
func (t *Trail) cellSet() map[[2]int]bool {
	m := make(map[[2]int]bool, len(t.Cells))
	for _, c := range t.Cells {
		m[c] = true
	}
	return m
}

// Centroid returns the average world-space XZ position of the trail's cells.
func (t *Trail) Centroid() mgl32.Vec2 {
	if len(t.Cells) == 0 {
		return mgl32.Vec2{}
	}
	var sx, sz float64
	for _, c := range t.Cells {
		sx += float64(c[0])
		sz += float64(c[1])
	}
	n := float64(len(t.Cells))
	return mgl32.Vec2{
		float32((sx/n + 0.5) * CellSize),
		float32((sz/n + 0.5) * CellSize),
	}
}

// PlaceTrail creates a new empty trail. Cells are added via AddTrailCells.
// Callers should call RebuildTrailGraph after painting cells.
func (w *World) PlaceTrail(name string, diff TerrainDifficulty) *Trail {
	if name == "" {
		name = w.nextTrailDefaultName()
	}
	t := &Trail{
		ID:         w.NextID(),
		Name:       name,
		Difficulty: diff,
	}
	w.Trails = append(w.Trails, t)
	return t
}

// DeleteTrail removes a trail by ID. Callers must call RebuildTrailGraph
// and replan any guests whose current plan step references the deleted trail.
func (w *World) DeleteTrail(id uint64) {
	for i, t := range w.Trails {
		if t.ID == id {
			w.Trails = append(w.Trails[:i], w.Trails[i+1:]...)
			return
		}
	}
}

// FindTrail returns the trail with the given ID, or nil.
func (w *World) FindTrail(id uint64) *Trail {
	for _, t := range w.Trails {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// SortCells normalises Cells to (x asc, z asc) order so that all consumers
// — routing, save diffs, rendering — see a deterministic sequence regardless
// of the order cells were painted or loaded.
func (t *Trail) SortCells() {
	sort.Slice(t.Cells, func(i, j int) bool {
		if t.Cells[i][0] != t.Cells[j][0] {
			return t.Cells[i][0] < t.Cells[j][0]
		}
		return t.Cells[i][1] < t.Cells[j][1]
	})
}

// AddTrailCells appends cells to a trail, silently ignoring duplicates.
// Cells are kept in (x asc, z asc) order after every mutation.
func (w *World) AddTrailCells(id uint64, cells [][2]int) {
	t := w.FindTrail(id)
	if t == nil {
		return
	}
	existing := t.cellSet()
	for _, c := range cells {
		if !existing[c] {
			t.Cells = append(t.Cells, c)
			existing[c] = true
		}
	}
	t.SortCells()
}

// RemoveTrailCells removes the given cells from a trail.
// Order is preserved (already sorted from prior AddTrailCells calls).
func (w *World) RemoveTrailCells(id uint64, cells [][2]int) {
	t := w.FindTrail(id)
	if t == nil {
		return
	}
	rm := make(map[[2]int]bool, len(cells))
	for _, c := range cells {
		rm[c] = true
	}
	out := t.Cells[:0]
	for _, c := range t.Cells {
		if !rm[c] {
			out = append(out, c)
		}
	}
	t.Cells = out
}

// RebuildTrailGraph recomputes the connectivity graph from current trail
// cell data and stores it on the world. Called after any trail mutation.
func (w *World) RebuildTrailGraph() {
	w.TrailGraph = BuildTrailGraph(w)
}

// BrushCells returns all grid cells within a circular radius of (cx, cz).
// Radius is in cells (1 cell = CellSize metres).
func BrushCells(cx, cz, radius int) [][2]int {
	var out [][2]int
	r2 := radius * radius
	for dx := -radius; dx <= radius; dx++ {
		for dz := -radius; dz <= radius; dz++ {
			if dx*dx+dz*dz <= r2 {
				out = append(out, [2]int{cx + dx, cz + dz})
			}
		}
	}
	return out
}

// PolylineCells returns all grid cells within radius of the polyline defined
// by consecutive waypoints, with no gaps between stamps. Safe to pass
// directly to AddTrailCells — duplicates are removed.
func PolylineCells(waypoints [][2]int, radius int) [][2]int {
	seen := make(map[[2]int]struct{})
	var out [][2]int
	stamp := func(cx, cz int) {
		for _, c := range BrushCells(cx, cz, radius) {
			if _, ok := seen[c]; !ok {
				seen[c] = struct{}{}
				out = append(out, c)
			}
		}
	}
	for i := 1; i < len(waypoints); i++ {
		x0, z0 := float64(waypoints[i-1][0]), float64(waypoints[i-1][1])
		x1, z1 := float64(waypoints[i][0]), float64(waypoints[i][1])
		dx, dz := x1-x0, z1-z0
		steps := int(math.Max(math.Abs(dx), math.Abs(dz)))
		if steps == 0 {
			stamp(int(math.Round(x0)), int(math.Round(z0)))
			continue
		}
		for s := 0; s <= steps; s++ {
			t := float64(s) / float64(steps)
			stamp(int(math.Round(x0+t*dx)), int(math.Round(z0+t*dz)))
		}
	}
	return out
}

func (w *World) nextTrailDefaultName() string {
	max := 0
	for _, t := range w.Trails {
		var n int
		if _, err := fmt.Sscanf(t.Name, "Run %d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("Run %d", max+1)
}
