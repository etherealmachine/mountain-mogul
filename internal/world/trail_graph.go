package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// trailMinDescentMeters is the minimum elevation drop required for a trail
// edge to be added to the graph. Mirrors minDescentMeters in ai/goap.
const trailMinDescentMeters = 20.0

// TrailEdge is one directed edge in the trail connectivity graph. An edge
// goes downhill from FromID to ToID via the named trail.
type TrailEdge struct {
	TrailID  uint64
	FromID   uint64
	FromKind EdgeKind
	ToID     uint64
	ToKind   EdgeKind
	Distance float32 // straight-line metres between entity positions
}

// TrailGraph is the derived connectivity structure built from trail cell data.
// It is rebuilt whenever the player paints or removes cells; guests use it to
// find which trails they can take from their current anchor.
type TrailGraph struct {
	Edges    []TrailEdge
	byFromID map[uint64][]int // indices into Edges keyed by FromID
}

// EdgesFrom returns all edges starting from the given entity ID.
// Returns nil when the graph is empty or no edges start from that ID.
func (g *TrailGraph) EdgesFrom(fromID uint64) []TrailEdge {
	if g == nil || len(g.Edges) == 0 {
		return nil
	}
	idxs := g.byFromID[fromID]
	if len(idxs) == 0 {
		return nil
	}
	out := make([]TrailEdge, len(idxs))
	for i, idx := range idxs {
		out[i] = g.Edges[idx]
	}
	return out
}

// BuildTrailGraph derives the connectivity graph from the current world state.
// For each trail, it finds all entity footprints (lift tops/bases, buildings,
// other trails) overlapping the trail's cells, then creates directed edges from
// higher-elevation to lower-elevation entities. Edges are indexed by FromID so
// ApplicableActions can look up available descents in O(1).
func BuildTrailGraph(w *World) *TrailGraph {
	g := &TrailGraph{
		byFromID: make(map[uint64][]int),
	}

	for _, trail := range w.Trails {
		if len(trail.Cells) == 0 {
			continue
		}
		cellSet := trail.cellSet()

		type entityInfo struct {
			id   uint64
			kind EdgeKind
			elev float32
			pos  mgl32.Vec2
		}

		var entities []entityInfo

		for _, lift := range w.Lifts {
			top := lift.TopCell()
			if cellSet[top] {
				entities = append(entities, entityInfo{
					id:   lift.ID,
					kind: KindLiftTop,
					elev: w.Terrain.SurfaceElevationAt(top[0], top[1]),
					pos:  lift.Top,
				})
			}
			base := lift.QueueCell()
			if cellSet[base] {
				entities = append(entities, entityInfo{
					id:   lift.ID,
					kind: KindLiftBase,
					elev: w.Terrain.SurfaceElevationAt(base[0], base[1]),
					pos:  lift.Base,
				})
			}
		}

		for _, b := range w.Buildings {
			if b.Type != BuildingLodge && b.Type != BuildingParking {
				continue
			}
			door := b.DoorCell()
			if cellSet[door] {
				entities = append(entities, entityInfo{
					id:   b.ID,
					kind: KindBuilding,
					elev: w.Terrain.SurfaceElevationAt(door[0], door[1]),
					pos:  b.Pos,
				})
			}
		}

		for _, other := range w.Trails {
			if other.ID == trail.ID || len(other.Cells) == 0 {
				continue
			}
			if trailsTouching(cellSet, other) {
				c := other.Centroid()
				cx := int(c[0] / CellSize)
				cz := int(c[1] / CellSize)
				entities = append(entities, entityInfo{
					id:   other.ID,
					kind: KindTrail,
					elev: w.Terrain.SurfaceElevationAt(cx, cz),
					pos:  c,
				})
			}
		}

		// Entity-to-entity directed edges (downhill only).
		for i, from := range entities {
			if from.kind == KindLiftBase {
				continue // guests don't start a descent from a lift base
			}
			for j, to := range entities {
				if i == j || to.kind == KindLiftTop {
					continue // don't generate edges that go to a lift top
				}
				if from.elev-to.elev < trailMinDescentMeters {
					continue
				}
				addEdge(g, TrailEdge{
					TrailID:  trail.ID,
					FromID:   from.id,
					FromKind: from.kind,
					ToID:     to.id,
					ToKind:   to.kind,
					Distance: dist2D(from.pos, to.pos),
				})
			}
		}

		// Trail-as-source edges: when a guest arrives at this trail from
		// a connected trail, they can reach all of this trail's lower
		// destinations. Keyed by trail.ID so EdgesFrom(trail.ID) works.
		centroid := trail.Centroid()
		centroidElev := avgCellElev(w, trail.Cells)
		for _, to := range entities {
			if to.kind == KindLiftTop {
				continue
			}
			if centroidElev-to.elev < trailMinDescentMeters {
				continue
			}
			addEdge(g, TrailEdge{
				TrailID:  trail.ID,
				FromID:   trail.ID,
				FromKind: KindTrail,
				ToID:     to.id,
				ToKind:   to.kind,
				Distance: dist2D(centroid, to.pos),
			})
		}
	}

	return g
}

// addEdge appends an edge to the graph and updates the byFromID index.
func addEdge(g *TrailGraph, e TrailEdge) {
	idx := len(g.Edges)
	g.Edges = append(g.Edges, e)
	g.byFromID[e.FromID] = append(g.byFromID[e.FromID], idx)
}

// trailsTouching returns true if any cell from other overlaps or is
// Chebyshev-adjacent (including diagonals) to a cell in cellSet.
func trailsTouching(cellSet map[[2]int]bool, other *Trail) bool {
	for _, c := range other.Cells {
		for dx := -1; dx <= 1; dx++ {
			for dz := -1; dz <= 1; dz++ {
				if cellSet[[2]int{c[0] + dx, c[1] + dz}] {
					return true
				}
			}
		}
	}
	return false
}

// avgCellElev returns the average surface elevation across the given cells.
func avgCellElev(w *World, cells [][2]int) float32 {
	if len(cells) == 0 {
		return 0
	}
	var sum float64
	for _, c := range cells {
		sum += float64(w.Terrain.SurfaceElevationAt(c[0], c[1]))
	}
	return float32(sum / float64(len(cells)))
}

func dist2D(a, b mgl32.Vec2) float32 {
	dx := float64(a[0] - b[0])
	dz := float64(a[1] - b[1])
	return float32(math.Sqrt(dx*dx + dz*dz))
}

// ServicesForLift returns the union of all trail difficulties whose edges
// depart from liftID's top station. When no trails connect to this lift top
// (including when TrailGraph is nil), it returns all three difficulties so
// that a resort with no painted trails still admits guests of every skill.
func (w *World) ServicesForLift(liftID uint64) TerrainDifficulty {
	if w.TrailGraph == nil || len(w.TrailGraph.Edges) == 0 {
		return DiffGreen | DiffBlue | DiffBlack
	}
	var services TerrainDifficulty
	for _, e := range w.TrailGraph.Edges {
		if e.FromID == liftID && e.FromKind == KindLiftTop {
			t := w.FindTrail(e.TrailID)
			if t != nil {
				services |= t.Difficulty
			}
		}
	}
	if services == 0 {
		// Lift top not covered by any trail — fall back to all difficulties
		// so the lift remains accessible regardless of trail layout.
		return DiffGreen | DiffBlue | DiffBlack
	}
	return services
}
