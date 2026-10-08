package world

import (
	"fmt"
	"math"
	"mountain-mogul/internal/ai"
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

// Trail is a named ski run or ungroomed area. Its shape is what the
// player draws (trail_shape.go): a run's nodes and the ends they're
// attached to, or an area's outline. Cells are derived from the shape
// (ShapeTrail, not saved), and the simulation derives connectivity from
// whichever entity footprints overlap them.
type Trail struct {
	ID         uint64
	Name       string
	Kind       TrailKind
	Difficulty TerrainDifficulty
	Groomed    bool // true ⇒ a garage's cats groom it (runs only)

	Nodes      []TrailNode  // a run's centre line, two or more
	Start, End TrailEnd     // what a run's first and last nodes are attached to
	Outline    []mgl32.Vec2 // an area's outline

	Cells [][2]int // derived from the shape

	// Conditions is what skiing the trail offers right now: the average
	// of its cells' snow and terrain features, in ai.TasteKind order
	// (crowds left 0). Derived by the sim every clock hour; not saved.
	Conditions [ai.TasteCount]float32
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

// RebuildTrailGraph re-derives every trail's cells from its shape (runs'
// ends follow the lifts they're attached to), then recomputes the
// connectivity graph and stores it on the world. Called after any change
// to trails, lifts, or buildings.
func (w *World) RebuildTrailGraph() {
	for _, t := range w.Trails {
		w.ShapeTrail(t)
	}
	w.TrailVersion++
	// Doors face lifts and parking, and trails end at doors.
	w.RefreshAllDoors()
	w.TrailGraph = BuildTrailGraph(w)
	w.rebuildTrailAt()
}

// rebuildTrailAt refills the cell-to-trail index from the trails' cells.
func (w *World) rebuildTrailAt() {
	if w.Terrain == nil {
		return
	}
	n := w.Terrain.Width * w.Terrain.Height
	if len(w.trailAt) != n {
		w.trailAt = make([]uint16, n)
		w.trailDiffs = make([]TerrainDifficulty, n)
	} else {
		clear(w.trailAt)
		clear(w.trailDiffs)
	}
	for i, t := range w.Trails {
		for _, c := range t.Cells {
			if !w.Terrain.InBounds(c[0], c[1]) {
				continue
			}
			k := c[1]*w.Terrain.Width + c[0]
			w.trailDiffs[k] |= t.Difficulty
			if prev := w.trailAt[k]; prev == 0 || w.Trails[prev-1].Difficulty < t.Difficulty {
				w.trailAt[k] = uint16(i + 1)
			}
		}
	}
}

// TrailAt returns the trail painted on cell (cx, cz), the hardest where
// trails overlap, or nil when the cell is off-trail.
func (w *World) TrailAt(cx, cz int) *Trail {
	if w.Terrain == nil || !w.Terrain.InBounds(cx, cz) {
		return nil
	}
	k := cz*w.Terrain.Width + cx
	if k >= len(w.trailAt) || w.trailAt[k] == 0 || int(w.trailAt[k]) > len(w.Trails) {
		return nil
	}
	return w.Trails[w.trailAt[k]-1]
}

// TrailDiffsAt is the difficulties of every trail on cell (cx, cz), 0
// off-trail.
func (w *World) TrailDiffsAt(cx, cz int) TerrainDifficulty {
	if w.Terrain == nil || !w.Terrain.InBounds(cx, cz) {
		return 0
	}
	if k := cz*w.Terrain.Width + cx; k < len(w.trailDiffs) {
		return w.trailDiffs[k]
	}
	return 0
}

// TrailJunction is where a guest skiing via, now at from, reaches dest:
// the first point on via's centre line, onward from the point nearest
// from, that lies on one of dest's cells (looking back along it if
// there's none onward, for a run drawn uphill); else the dest cell
// nearest from. False when dest has no cells.
func TrailJunction(via, dest *Trail, from mgl32.Vec2) (mgl32.Vec2, bool) {
	if dest == nil || len(dest.Cells) == 0 {
		return mgl32.Vec2{}, false
	}
	if via != nil {
		if line := via.Centerline(); len(line) > 0 {
			on := dest.cellSet()
			near, best := 0, float32(math.MaxFloat32)
			for i, s := range line {
				if d := s.Pos.Sub(from).LenSqr(); d < best {
					near, best = i, d
				}
			}
			hit := func(i int) bool {
				p := line[i].Pos
				return on[[2]int{int(p[0] / CellSize), int(p[1] / CellSize)}]
			}
			for i := near; i < len(line); i++ {
				if hit(i) {
					return line[i].Pos, true
				}
			}
			for i := near; i >= 0; i-- {
				if hit(i) {
					return line[i].Pos, true
				}
			}
		}
	}
	x, z, _ := dest.NearestCellCenter(from[0], from[1])
	return mgl32.Vec2{x, z}, true
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
