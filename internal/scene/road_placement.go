package scene

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// snapToCellGrid rounds a continuous world XZ position to the nearest
// terrain cell-vertex coordinate (multiples of cellSize). All new
// road-node positions go through this so that the per-cell snow / tree
// clearance pass treats both sides of the centerline symmetrically.
// For an off-grid centerline, the vertices flanking the road sit at
// unequal perpendicular distances and one side ends up over-cleared
// relative to the other.
//
// Slight loss: free-form placement is now 5 m-quantised. The chain
// curve still bends through arbitrary directions, so the visual
// effect is small (a node placed at world (37, 51) lands at (35, 50)
// instead, but the curve through it bends just the same).
func snapToCellGrid(pos mgl32.Vec2) mgl32.Vec2 {
	const cellSize = float32(5.0)
	return mgl32.Vec2{
		float32(math.Round(float64(pos[0]/cellSize))) * cellSize,
		float32(math.Round(float64(pos[1]/cellSize))) * cellSize,
	}
}

// Road-node marker tints. The "all nodes" colour is muted so a busy
// network doesn't drown the scene, and the "snap target" colour is
// bright so the player can see which one their click will hit.
var (
	roadNodeMarkerTint     = [3]float32{0.85, 0.75, 0.20} // soft amber
	roadNodeMarkerSnapTint = [3]float32{1.00, 0.95, 0.40} // bright yellow
)

// emitRoadNodeMarkers writes one MeshRoadNode ghost instance per
// existing road node so the player can see snap targets while the
// road tool is active. The current snap target — either an existing
// node or a projected point on an existing edge — gets the bright
// tint; everything else gets the muted tint.
//
// Called every frame from the scenario / editor update loop AFTER
// updatePlacementGhost (which clears all ghost batches). No-op when
// hoverValid is false and there's nothing to highlight — the markers
// still render at every node so the player sees the network even
// without a hover.
func emitRoadNodeMarkers(r *render.Renderer, w *world.World, hoverPos mgl32.Vec2, hoverValid bool) {
	if len(w.RoadNodes) == 0 && !hoverValid {
		r.SetGhosts(render.MeshRoadNode, nil)
		return
	}

	// Default tint per node. Resolve the snap target to identify the one
	// (if any) the cursor would click. Snap-to-edge produces a projected
	// point that isn't a node yet; we render an extra marker there.
	var snapNodeID uint64
	var snapEdgePos mgl32.Vec2
	hasEdgeSnap := false
	if hoverValid {
		ep := resolveRoadEndpoint(w, hoverPos)
		if ep.node != nil {
			snapNodeID = ep.node.ID
		} else if ep.edge != nil {
			snapEdgePos = ep.pos
			hasEdgeSnap = true
		}
	}

	instances := make([]render.StaticInstance, 0, len(w.RoadNodes)+1)
	for _, n := range w.RoadNodes {
		tint := roadNodeMarkerTint
		if n.ID == snapNodeID {
			tint = roadNodeMarkerSnapTint
		}
		instances = append(instances, roadNodeMarkerInstance(n.Pos, w.Terrain, tint))
	}
	if hasEdgeSnap {
		instances = append(instances, roadNodeMarkerInstance(snapEdgePos, w.Terrain, roadNodeMarkerSnapTint))
	}
	r.SetGhosts(render.MeshRoadNode, instances)
}

// roadNodeMarkerInstance wraps RoadNodeMarkerTransform into a
// StaticInstance for the ghost-batch path.
func roadNodeMarkerInstance(pos mgl32.Vec2, t *world.Terrain, tint [3]float32) render.StaticInstance {
	m := render.RoadNodeMarkerTransform(pos, t)
	inst := render.StaticInstance{ColorTint: tint}
	copy(inst.Transform[:], m[:])
	return inst
}

// roadEndpoint is the resolved form of a click during road placement —
// either an existing node, a point on an existing edge (which the
// commit will split), or a fresh freestanding point. Exactly one of
// `node` / `edge` is non-nil; if both are nil the endpoint becomes a
// new freestanding node on commit.
type roadEndpoint struct {
	pos  mgl32.Vec2
	node *world.RoadNode
	edge *world.RoadEdge
}

// resolveRoadEndpoint snaps a raw cursor position to the road graph,
// preferring an existing node (cheaper, makes clean joints) and falling
// back to projecting onto the nearest edge inside RoadSnapRadius. When
// nothing is in range, returns the raw position untouched.
//
// When the projection lands near a degree-1 endpoint of the hit edge
// (a dead-end), the snap upgrades to that endpoint node rather than
// producing a near-the-end split. Lets the player click the visible
// asphalt at the tip of an end segment — including the inland end of
// an edge-connect stub — to pick the dead-end up as the chain anchor.
//
// Used by both scenario and editor so the player and the level designer
// see the same snap behaviour.
func resolveRoadEndpoint(w *world.World, raw mgl32.Vec2) roadEndpoint {
	raw = snapToCellGrid(raw)
	if n := w.SnapRoadNode(raw, world.RoadSnapRadius); n != nil {
		return roadEndpoint{pos: n.Pos, node: n}
	}
	if e, cp, ok := w.SnapToRoadEdge(raw, world.RoadSnapRadius); ok {
		if n := nearDegreeOneEndpoint(w, e, cp, world.RoadSnapRadius); n != nil {
			return roadEndpoint{pos: n.Pos, node: n}
		}
		// Snap the edge-projected point too — leaves the new
		// intersection node slightly off the original edge's line, but
		// the resulting kink is at most ~2.5 m and the Catmull-Rom
		// chain smooths it. Keeps every road node grid-aligned.
		return roadEndpoint{pos: snapToCellGrid(cp), edge: e}
	}
	return roadEndpoint{pos: raw}
}

// nearDegreeOneEndpoint returns whichever endpoint of e is a dead-end
// (degree 1) AND whose distance to cp is within tol, or nil if neither
// qualifies. cp is expected to be the projection of the raw click onto
// the edge — i.e. SnapToRoadEdge's contact point.
func nearDegreeOneEndpoint(w *world.World, e *world.RoadEdge, cp mgl32.Vec2, tol float32) *world.RoadNode {
	tol2 := tol * tol
	var best *world.RoadNode
	bestD2 := tol2
	for _, id := range [2]uint64{e.A, e.B} {
		if !isDegreeOneNode(w, id) {
			continue
		}
		n := w.RoadNodeByID(id)
		if n == nil {
			continue
		}
		dx := n.Pos[0] - cp[0]
		dz := n.Pos[1] - cp[1]
		d2 := dx*dx + dz*dz
		if d2 <= bestD2 {
			best = n
			bestD2 = d2
		}
	}
	return best
}

// isDegreeOneNode reports whether the given node has exactly one
// incident edge — i.e., it's the tip of a chain.
func isDegreeOneNode(w *world.World, id uint64) bool {
	count := 0
	for _, e := range w.RoadEdges {
		if e.A == id || e.B == id {
			count++
			if count > 1 {
				return false
			}
		}
	}
	return count == 1
}

// placeRoadSegment commits a road segment between two resolved
// endpoints. Edge-snapped endpoints trigger SplitRoadEdge first so the
// new segment plugs into a real intersection node; node-snapped and
// freestanding endpoints feed straight through.
//
// Returns nil (without mutating the world) when the two endpoints
// would resolve to the same node — same existing node, same edge,
// near-coincident freestanding positions that would collapse on top
// of each other, or one endpoint sitting on an edge whose other
// endpoint is the snapped-to node.
func placeRoadSegment(w *world.World, a, b roadEndpoint) *world.RoadEdge {
	// Both endpoints target the same node directly — nothing to add.
	if a.node != nil && b.node != nil && a.node.ID == b.node.ID {
		return nil
	}
	// Both endpoints project onto the same edge — drawing a chord
	// across a single segment is almost always a misclick. Reject.
	if a.edge != nil && b.edge != nil && a.edge.ID == b.edge.ID {
		return nil
	}
	// Neither side resolved to anything, and the two raw positions
	// would snap together once the first becomes a node. Bail before
	// allocating two coincident nodes.
	if a.node == nil && a.edge == nil && b.node == nil && b.edge == nil {
		dx := b.pos[0] - a.pos[0]
		dz := b.pos[1] - a.pos[1]
		if dx*dx+dz*dz < world.RoadSnapRadius*world.RoadSnapRadius {
			return nil
		}
	}

	na := resolveOrCreate(w, a)
	nb := resolveOrCreate(w, b)
	if na.ID == nb.ID {
		return nil
	}
	return w.AddRoadEdge(na.ID, nb.ID)
}

// edgeConnectTolerance is how close to a map edge a click has to land
// for the edge-connect tool to accept it (metres). Generous enough that
// the designer doesn't have to pixel-hunt along the perimeter, tight
// enough that an off-edge click obviously reads as "not on the edge".
const edgeConnectTolerance = float32(30.0)

// edgeConnectStubLength is how far inland the stub road extends from
// the perimeter post (metres). Four cells at the 5 m grid — short
// enough not to commit much of the map to the stub, long enough to
// read as a deliberate road segment and give the player something to
// connect their network to.
const edgeConnectStubLength = float32(20.0)

// projectToMapEdge clamps a click to the nearest terrain perimeter edge
// when within `tolerance` metres of it. Returns the snapped position,
// the inward unit vector (perpendicular to that edge, pointing into
// the map) and ok=true on success; on failure returns (raw, zero,
// false) so the ghost preview can red-tint and placement can refuse.
func projectToMapEdge(t *world.Terrain, pos mgl32.Vec2, tolerance float32) (mgl32.Vec2, mgl32.Vec2, bool) {
	const cellSize = float32(5.0)
	// The terrain mesh's surface quads span (Width-1) × (Height-1)
	// cells — the last vertex sits at (Width-1)*cellSize, not at
	// Width*cellSize. Snapping to the cell-count-based edge dropped
	// the connection node a cell past the visible map.
	mapW := float32(t.Width-1) * cellSize
	mapH := float32(t.Height-1) * cellSize

	distLeft := pos[0]
	distRight := mapW - pos[0]
	distTop := pos[1]
	distBottom := mapH - pos[1]

	minDist := distLeft
	side := 0 // 0=left, 1=right, 2=top, 3=bottom
	if distRight < minDist {
		minDist = distRight
		side = 1
	}
	if distTop < minDist {
		minDist = distTop
		side = 2
	}
	if distBottom < minDist {
		minDist = distBottom
		side = 3
	}
	if minDist > tolerance {
		return pos, mgl32.Vec2{}, false
	}

	snapped := pos
	var inward mgl32.Vec2
	switch side {
	case 0:
		snapped[0] = 0
		inward = mgl32.Vec2{1, 0}
	case 1:
		snapped[0] = mapW
		inward = mgl32.Vec2{-1, 0}
	case 2:
		snapped[1] = 0
		inward = mgl32.Vec2{0, 1}
	case 3:
		snapped[1] = mapH
		inward = mgl32.Vec2{0, -1}
	}
	// Clamp the off-axis component to the map so a near-corner click
	// doesn't end up just outside the bounds.
	if snapped[0] < 0 {
		snapped[0] = 0
	} else if snapped[0] > mapW {
		snapped[0] = mapW
	}
	if snapped[1] < 0 {
		snapped[1] = 0
	} else if snapped[1] > mapH {
		snapped[1] = mapH
	}
	return snapped, inward, true
}

// resolveOrCreate turns a roadEndpoint into a concrete *RoadNode:
//   - existing node → return it
//   - on an edge   → split the edge and return the new intersection
//   - freestanding → add a new freestanding node
func resolveOrCreate(w *world.World, ep roadEndpoint) *world.RoadNode {
	if ep.node != nil {
		return ep.node
	}
	if ep.edge != nil {
		return w.SplitRoadEdge(ep.edge, ep.pos)
	}
	return w.AddRoadNode(ep.pos, world.RoadNodeFreestanding)
}
