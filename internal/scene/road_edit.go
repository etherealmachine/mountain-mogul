package scene

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// roadEditSelection is the state of the click-to-edit road tool. Active
// whenever node or edge is non-nil; mutually exclusive (at most one is
// set at a time). dragging means the button is held after picking: on a
// node, its Pos follows the cursor; on an edge, the edge is split where
// it was pressed (pressAt) once the cursor moves off it, and the new
// node dragged from there.
//
// While a node drags it snaps onto a node (into) or road (onto, at
// ontoPos) within roadMergeRadius, and joins it when let go.
type roadEditSelection struct {
	node     *world.RoadNode
	edge     *world.RoadEdge
	dragging bool
	pressAt  mgl32.Vec2

	into    *world.RoadNode
	onto    *world.RoadEdge
	ontoPos mgl32.Vec2
}

// roadMergeRadius is how near, in metres, a dragged road node snaps onto
// another node or road; roadSplitDrag is how far the cursor moves from
// where a road was pressed before the press becomes a drag that adds a
// node there.
const (
	roadMergeRadius = float32(8)
	roadSplitDrag   = float32(3)
)

func (s *roadEditSelection) active() bool {
	return s.node != nil || s.edge != nil
}

func (s *roadEditSelection) clear() {
	*s = roadEditSelection{}
}

// tryStartRoadEdit hit-tests a click against the road graph and updates
// sel in place. Priority: node first (within RoadSnapRadius), then edge
// (within RoadSnapRadius). Parking-driveway nodes are skipped — they
// belong to a parking lot's mesh slot and shouldn't be edited via the
// road tool; the lot itself owns their lifecycle.
//
// Returns true when something was selected (caller should consume the
// click). Either way sel.dragging is set so the next LeftHeld frames
// move the node, or split the edge where it was pressed and move that.
func tryStartRoadEdit(w *world.World, raw mgl32.Vec2, sel *roadEditSelection) bool {
	if n := nearestEditableNode(w, raw, world.RoadSnapRadius); n != nil {
		*sel = roadEditSelection{node: n, dragging: true}
		return true
	}
	if e, cp, ok := w.SnapToRoadEdge(raw, world.RoadSnapRadius); ok {
		*sel = roadEditSelection{edge: e, dragging: true, pressAt: cp}
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

// dragRoadNode moves the selected node to pos (grid-snapped), or onto
// the node or road it snaps to there (roadMergeTarget), and re-rebuilds
// road meshes and terrain stamping so the carriageway visibly follows
// the cursor. A pressed edge splits once the cursor leaves the press.
// RebuildStaticBatch is deferred to release — re-uploading tree
// instances every frame stalls the drag.
func dragRoadNode(r *render.Renderer, w *world.World, sel *roadEditSelection, pos mgl32.Vec2) {
	if sel.edge != nil {
		if pos.Sub(sel.pressAt).Len() < roadSplitDrag {
			return
		}
		sel.node, sel.edge = w.SplitRoadEdge(sel.edge, snapToCellGrid(sel.pressAt)), nil
	}
	if sel.node == nil {
		return
	}
	sel.into, sel.onto = roadMergeTarget(w, sel.node, pos)
	snapped := snapToCellGrid(pos)
	switch {
	case sel.into != nil:
		snapped = sel.into.Pos
	case sel.onto != nil:
		_, cp, _ := w.SnapToRoadEdge(pos, roadMergeRadius)
		sel.ontoPos = snapToCellGrid(cp)
		snapped = sel.ontoPos
	}
	if snapped == sel.node.Pos {
		return
	}
	sel.node.Pos = snapped
	applyRoadCellState(w)
	r.FlushTerrainVerts(w.Terrain)
	r.RebuildRoads(w)
}

// roadMergeTarget is the node (else the road) dragged node n would join
// if let go at pos: the nearest within roadMergeRadius, not n itself or
// a road from it. Only plain nodes join others; an entry stays apart.
func roadMergeTarget(w *world.World, n *world.RoadNode, pos mgl32.Vec2) (*world.RoadNode, *world.RoadEdge) {
	if n.Kind != world.RoadNodeFreestanding && n.Kind != world.RoadNodeIntersection {
		return nil, nil
	}
	var into *world.RoadNode
	best := roadMergeRadius
	for _, m := range w.RoadNodes {
		if d := m.Pos.Sub(pos).Len(); m != n && d <= best {
			into, best = m, d
		}
	}
	if into != nil {
		return into, nil
	}
	var onto *world.RoadEdge
	best = roadMergeRadius
	for _, e := range w.RoadEdges {
		if e.A == n.ID || e.B == n.ID {
			continue
		}
		a, b := w.RoadNodeByID(e.A), w.RoadNodeByID(e.B)
		if a == nil || b == nil {
			continue
		}
		if d := world.ClosestPointOnRoadSegment(pos, a.Pos, b.Pos).Sub(pos).Len(); d <= best {
			onto, best = e, d
		}
	}
	return nil, onto
}

// commitRoadDrag finalises a drag: joins the node to what it snapped to,
// keeping that one selected, and pushes the cleared-cell tree state to
// the GPU. Called once on mouse release rather than per frame; a press
// on an edge that never moved just leaves the edge selected.
func commitRoadDrag(r *render.Renderer, w *world.World, sel *roadEditSelection) {
	into := sel.into
	if sel.onto != nil {
		into = w.SplitRoadEdge(sel.onto, sel.ontoPos)
	}
	if into != nil && sel.node != nil {
		mergeRoadNode(w, sel.node, into)
		sel.node = into
	}
	sel.dragging, sel.into, sel.onto = false, nil, nil
	if sel.edge != nil {
		return
	}
	applyRoadCellState(w)
	r.FlushTerrainVerts(w.Terrain)
	r.RebuildRoads(w)
	r.RebuildStaticBatch(w)
}

// mergeRoadNode joins node n into node into: n's roads end at into
// instead, a road between the two or one into already has goes, and n
// is removed.
func mergeRoadNode(w *world.World, n, into *world.RoadNode) {
	linked := map[uint64]bool{}
	for _, e := range w.RoadEdges {
		if e.A == into.ID || e.B == into.ID {
			linked[otherEnd(e, into.ID)] = true
		}
	}
	var others []uint64
	for _, e := range w.RoadEdges {
		if e.A == n.ID || e.B == n.ID {
			others = append(others, otherEnd(e, n.ID))
		}
	}
	w.RemoveRoadNode(n.ID)
	for _, o := range others {
		if o != into.ID && !linked[o] {
			w.AddRoadEdge(into.ID, o)
			linked[o] = true
		}
	}
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
	roadEditMarkerRadiusScale    = float32(1.6)
	roadEditMarkerHeightScale    = float32(8.0)
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
	// What a dragged node would join shows in the snap colour, a
	// driveway too; a road's join point gets a marker of its own.
	var into uint64
	if sel.into != nil {
		into = sel.into.ID
	}

	instances := make([]render.StaticInstance, 0, len(w.RoadNodes)+1)
	for _, n := range w.RoadNodes {
		if n.Kind == world.RoadNodeParkingDriveway && n.ID != into {
			continue
		}
		tint := roadNodeMarkerTint
		rs, hs := roadEditMarkerRadiusScale, roadEditMarkerHeightScale
		switch n.ID {
		case into:
			tint = roadNodeMarkerSnapTint
			rs, hs = roadEditSelectionRadiusScale*1.5, roadEditMarkerHeightScale
		case hotA, hotB:
			tint = roadEditSelectionTint
			rs, hs = roadEditSelectionRadiusScale, roadEditSelectionHeightScale
		}
		instances = append(instances, roadEditMarkerInstance(n.Pos, w.Terrain, tint, rs, hs))
	}
	if sel.onto != nil {
		instances = append(instances, roadEditMarkerInstance(sel.ontoPos, w.Terrain, roadNodeMarkerSnapTint,
			roadEditSelectionRadiusScale*1.5, roadEditMarkerHeightScale))
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
