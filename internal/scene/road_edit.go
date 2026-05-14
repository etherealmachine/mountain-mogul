package scene

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// roadEditSelection is the state of the click-to-edit road tool. Active
// whenever node or edge is non-nil; mutually exclusive (at most one is
// set at a time). dragging means the user is mid-drag on a node — the
// node's Pos is being updated each frame as the cursor moves.
type roadEditSelection struct {
	node     *world.RoadNode
	edge     *world.RoadEdge
	dragging bool
}

func (s *roadEditSelection) active() bool {
	return s.node != nil || s.edge != nil
}

func (s *roadEditSelection) clear() {
	s.node = nil
	s.edge = nil
	s.dragging = false
}

// tryStartRoadEdit hit-tests a click against the road graph and updates
// sel in place. Priority: node first (within RoadSnapRadius), then edge
// (within RoadSnapRadius). Parking-driveway nodes are skipped — they
// belong to a parking lot's mesh slot and shouldn't be edited via the
// road tool; the lot itself owns their lifecycle.
//
// Returns true when something was selected (caller should consume the
// click). On a node hit, sel.dragging is set so the next LeftHeld frame
// moves the node.
func tryStartRoadEdit(w *world.World, raw mgl32.Vec2, sel *roadEditSelection) bool {
	if n := nearestEditableNode(w, raw, world.RoadSnapRadius); n != nil {
		sel.node = n
		sel.edge = nil
		sel.dragging = true
		return true
	}
	if e, _, ok := w.SnapToRoadEdge(raw, world.RoadSnapRadius); ok {
		sel.node = nil
		sel.edge = e
		sel.dragging = false
		return true
	}
	return false
}

// nearestEditableNode is SnapRoadNode restricted to nodes the edit
// tool is allowed to touch. Parking-driveway nodes are owned by the
// parking lot — moving or deleting one would leave the lot dangling.
func nearestEditableNode(w *world.World, pos mgl32.Vec2, radius float32) *world.RoadNode {
	r2 := radius * radius
	var best *world.RoadNode
	bestD2 := r2
	for _, n := range w.RoadNodes {
		if n.Kind == world.RoadNodeParkingDriveway {
			continue
		}
		dx := n.Pos[0] - pos[0]
		dz := n.Pos[1] - pos[1]
		d2 := dx*dx + dz*dz
		if d2 <= bestD2 {
			best = n
			bestD2 = d2
		}
	}
	return best
}

// dragRoadNode moves the selected node to pos (grid-snapped) and re-
// rebuilds road meshes and terrain stamping so the carriageway visibly
// follows the cursor. RebuildStaticBatch is deferred to release —
// re-uploading tree instances every frame stalls the drag.
func dragRoadNode(r *render.Renderer, w *world.World, sel *roadEditSelection, pos mgl32.Vec2) {
	if sel.node == nil {
		return
	}
	snapped := snapToCellGrid(pos)
	if snapped == sel.node.Pos {
		return
	}
	sel.node.Pos = snapped
	applyRoadCellState(w)
	r.FlushTerrainVerts(w.Terrain)
	r.RebuildRoads(w)
}

// commitRoadDrag finalises a drag: pushes the cleared-cell tree state
// to the GPU. Called once on mouse release rather than per frame.
func commitRoadDrag(r *render.Renderer, w *world.World) {
	r.RebuildStaticBatch(w)
}

// deleteSelectedRoad removes whatever is currently selected and clears
// sel. For nodes: if degree-2, dissolve into a single bridge edge
// between the two neighbours; otherwise drop the node and every
// incident edge. For edges: remove that one edge, and any endpoint
// that was left freestanding-and-isolated.
//
// Always followed by a full road / terrain / static rebuild — deleting
// removes carriageway, but the cleared cells stay cleared (matching
// applyRoadCellState's one-way clearance contract).
func deleteSelectedRoad(r *render.Renderer, w *world.World, sel *roadEditSelection) {
	if sel.node != nil {
		deleteRoadNodeWithDissolve(w, sel.node)
	} else if sel.edge != nil {
		deleteRoadEdge(w, sel.edge)
	} else {
		return
	}
	sel.clear()
	applyRoadCellState(w)
	r.FlushTerrainVerts(w.Terrain)
	r.RebuildRoads(w)
	r.RebuildStaticBatch(w)
}

// deleteRoadNodeWithDissolve removes a node. If exactly two edges meet
// at it, the node is dissolved: both edges go away and a new edge
// bridges the two neighbours. Otherwise the node + all incident edges
// are dropped (existing world.RemoveRoadNode behaviour).
func deleteRoadNodeWithDissolve(w *world.World, n *world.RoadNode) {
	incident := make([]*world.RoadEdge, 0, 2)
	for _, e := range w.RoadEdges {
		if e.A == n.ID || e.B == n.ID {
			incident = append(incident, e)
		}
	}
	if len(incident) == 2 {
		a := otherEnd(incident[0], n.ID)
		b := otherEnd(incident[1], n.ID)
		w.RemoveRoadNode(n.ID)
		if a != b {
			w.AddRoadEdge(a, b)
		}
		return
	}
	w.RemoveRoadNode(n.ID)
}

// deleteRoadEdge removes one edge by ID and tidies up any endpoint
// that is now a freestanding orphan. Edge-connection and parking-
// driveway nodes are kept even when isolated — they have semantic
// roles (map-edge spawn point, parking-lot driveway) the edit tool
// must not silently delete.
func deleteRoadEdge(w *world.World, e *world.RoadEdge) {
	a, b := e.A, e.B
	for i, edge := range w.RoadEdges {
		if edge.ID == e.ID {
			w.RoadEdges = append(w.RoadEdges[:i], w.RoadEdges[i+1:]...)
			break
		}
	}
	maybeDeleteOrphan(w, a)
	maybeDeleteOrphan(w, b)
}

// maybeDeleteOrphan drops a node only if it now has zero incident
// edges AND is a plain freestanding node — junctions become degree-1
// dead-ends after an edge delete and that's intentional; edge-connection
// and driveway nodes anchor world features and must stay.
func maybeDeleteOrphan(w *world.World, id uint64) {
	n := w.RoadNodeByID(id)
	if n == nil || n.Kind != world.RoadNodeFreestanding && n.Kind != world.RoadNodeIntersection {
		return
	}
	for _, e := range w.RoadEdges {
		if e.A == id || e.B == id {
			return
		}
	}
	w.RemoveRoadNode(id)
}

func otherEnd(e *world.RoadEdge, id uint64) uint64 {
	if e.A == id {
		return e.B
	}
	return e.A
}

// Tints for road edit mode. Muted matches the placement-tool palette;
// the selection tint is hot orange so a selected node visibly pops over
// the soft amber of the rest of the network.
var roadEditSelectionTint = [3]float32{1.00, 0.45, 0.10}

// Edit-mode marker dimensions. The base marker (used by the placement-
// tool snap preview) is a flat disc 3 m wide × 0.15 m tall — fine as a
// "snap target" hint but too low to read as a drag handle. Edit mode
// renders taller knobs; the active selection is taller and wider so
// the player can see which one their drag is moving without zooming in.
//
// Stamped as a non-uniform scale on top of RoadNodeMarkerTransform so
// the underlying cylinder mesh stays shared with the placement path.
const (
	roadEditMarkerRadiusScale = float32(1.6)
	roadEditMarkerHeightScale = float32(8.0)
	roadEditSelectionRadiusScale = float32(2.0)
	roadEditSelectionHeightScale = float32(12.0)
)

// emitRoadEditMarkers renders the editor's road-handle layer: every
// non-driveway node gets a muted marker, the selected node (or both
// endpoints of the selected edge) gets the hot tint. Driveway nodes
// stay invisible to match the no-edit-allowed hit-test rule.
//
// Mirrors emitRoadNodeMarkers' batch contract — clears MeshRoadNode
// when there's nothing to draw so a stale frame doesn't leak.
func emitRoadEditMarkers(r *render.Renderer, w *world.World, sel *roadEditSelection) {
	if !sel.active() && len(w.RoadNodes) == 0 {
		r.SetGhosts(render.MeshRoadNode, nil)
		return
	}

	hotA, hotB := uint64(0), uint64(0)
	if sel.node != nil {
		hotA = sel.node.ID
	} else if sel.edge != nil {
		hotA = sel.edge.A
		hotB = sel.edge.B
	}

	instances := make([]render.StaticInstance, 0, len(w.RoadNodes))
	for _, n := range w.RoadNodes {
		if n.Kind == world.RoadNodeParkingDriveway {
			continue
		}
		tint := roadNodeMarkerTint
		rs, hs := roadEditMarkerRadiusScale, roadEditMarkerHeightScale
		if n.ID == hotA || n.ID == hotB {
			tint = roadEditSelectionTint
			rs, hs = roadEditSelectionRadiusScale, roadEditSelectionHeightScale
		}
		instances = append(instances, roadEditMarkerInstance(n.Pos, w.Terrain, tint, rs, hs))
	}
	r.SetGhosts(render.MeshRoadNode, instances)
}

// roadEditMarkerInstance is roadNodeMarkerInstance with a non-uniform
// scale stamped on top — radius and height multipliers applied to the
// shared MeshRoadNode disc so the same mesh can render as a small snap
// hint, a medium edit handle, or a tall selected knob.
func roadEditMarkerInstance(pos mgl32.Vec2, t *world.Terrain, tint [3]float32, radiusScale, heightScale float32) render.StaticInstance {
	m := render.RoadNodeMarkerTransform(pos, t).Mul4(mgl32.Scale3D(radiusScale, heightScale, radiusScale))
	inst := render.StaticInstance{ColorTint: tint}
	copy(inst.Transform[:], m[:])
	return inst
}
