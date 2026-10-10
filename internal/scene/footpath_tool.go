package scene

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// The footpath tool, shared by the game and the scenario editor
// (world/footpath.go).
//
// Laying: click to lay a path node by node ([ and ] narrow and widen the
// next node), click the last node again or press Enter to finish,
// right-click to take back a node, Esc to drop the path. Nodes snap to
// existing paths' nodes and centre lines, so paths join.
//
// Editing, while not laying one: the path under the cursor shows its
// handles. Drag a node to move it (a click that doesn't move it starts a
// new path from it), or a width handle (either side of a node) to set the
// width there; [ and ] over a node change its width;
// right-click a node to delete it (a path left with one node goes);
// Shift-click a path to put a node in it. A plain click on a path starts
// a new one branching from it. In the game, an edit that makes a path
// bigger charges the extra area, and is undone if that can't be paid.
// The Remove tool removes a whole path.

const (
	pathSnapRadius   = 4.0 // metres: how near an existing node a click snaps to it
	pathMinNodeGap   = 2.0 // metres: clicks closer than this to the last node finish the path
	pathWidthStep    = 0.5 // metres per [ or ]
	pathHandleRadius = 2.0 // metres: how near a handle a click grabs it
	pathHoverReach   = 3.0 // metres beyond a path's edge that still shows its handles
)

// pathTool is a footpath-tool session.
type pathTool struct {
	width    float32 // for the next node
	nodes    []world.TrailNode
	cursor   mgl32.Vec2
	cursorOK bool

	// Editing: the path whose handles show, the handle under the
	// cursor, and the one being dragged with the path's nodes before.
	focus    uint64
	hover    pathHandle
	drag     pathHandle
	dragging bool
	before   []world.TrailNode
}

// pathHandle is a handle of the focused path: a node, or its width
// handle; ok false when there's none.
type pathHandle struct {
	node  int
	width bool
	ok    bool
}

func newPathTool() pathTool { return pathTool{width: world.FootpathDefaultWidth} }

// drawing reports whether a path is being laid.
func (st *pathTool) drawing() bool { return len(st.nodes) > 0 }

// pathEnv is what the footpath tool works in: free in the editor.
// changed runs after any path is laid or reshaped (snow and trees
// cleared, terrain redrawn, the editor marked dirty).
type pathEnv struct {
	w       *world.World
	r       *render.Renderer
	toast   func(string)
	changed func()
	free    bool
}

// pathInput is one frame's input for the footpath tool.
type pathInput struct {
	toolInput
	widen, narrow bool
	insert        bool // Shift: a click on a path puts a node in it
}

// input runs one frame of the footpath tool.
func (env pathEnv) input(st *pathTool, in pathInput) {
	st.cursorOK = in.groundValid && !in.covered
	raw := mgl32.Vec2{in.ground[0], in.ground[2]}
	if st.dragging {
		env.dragTo(st, in, raw)
		return
	}
	if st.cursorOK {
		st.cursor = snapPath(env.w, raw)
	}
	if !st.drawing() && env.edit(st, in, raw) {
		env.r.SetFootpathGhost(env.w, nil, false)
		return
	}
	if in.widen || in.narrow {
		st.width = stepPathWidth(st.width, in.narrow)
		env.toast(fmt.Sprintf("Path width %.1f m", st.width))
	}
	preview := st.nodes
	if st.cursorOK && st.drawing() {
		preview = append(append([]world.TrailNode(nil), st.nodes...), world.TrailNode{Pos: st.cursor, Width: st.width})
	}
	ok, _ := env.check(preview, 0)
	env.r.SetFootpathGhost(env.w, preview, ok)

	switch {
	case in.rightClick && st.drawing():
		st.nodes = st.nodes[:len(st.nodes)-1]
	case in.enter && st.drawing():
		env.finish(st)
	case in.leftClick && st.cursorOK:
		if n := len(st.nodes); n > 0 && st.nodes[n-1].Pos.Sub(st.cursor).Len() < pathMinNodeGap {
			env.finish(st) // clicking the last node again ends the path
			return
		}
		st.nodes = append(st.nodes, world.TrailNode{Pos: st.cursor, Width: st.width})
		if len(st.nodes) == 1 {
			env.toast("Click to add nodes; click the last node again or press Enter to finish. [ and ] change the width, right-click takes back a node.")
		}
	}
}

// stepPathWidth is width a step narrower or wider, within the limits.
func stepPathWidth(width float32, narrow bool) float32 {
	d := float32(pathWidthStep)
	if narrow {
		d = -d
	}
	return min(max(width+d, world.FootpathMinWidth), world.FootpathMaxWidth)
}

// edit handles a frame of editing the path under the cursor, and
// reports whether it used the frame's input.
func (env pathEnv) edit(st *pathTool, in pathInput, raw mgl32.Vec2) bool {
	w := env.w
	st.focus, st.hover = 0, pathHandle{}
	if !st.cursorOK {
		return false
	}
	f := w.FootpathAt(raw, pathHoverReach)
	if f == nil {
		return false
	}
	st.focus = f.ID
	st.hover = pathHandleAt(f, raw)
	switch {
	case (in.widen || in.narrow) && st.hover.ok:
		before := append([]world.TrailNode(nil), f.Nodes...)
		n := &f.Nodes[st.hover.node]
		n.Width = stepPathWidth(n.Width, in.narrow)
		env.commit(f, before)
		env.toast(fmt.Sprintf("Width here: %.1f m", n.Width))
		return true
	case in.leftClick && st.hover.ok:
		st.drag, st.dragging = st.hover, true
		st.before = append([]world.TrailNode(nil), f.Nodes...)
		return true
	case in.rightClick && st.hover.ok && !st.hover.width:
		before := append([]world.TrailNode(nil), f.Nodes...)
		f.Nodes = append(f.Nodes[:st.hover.node], f.Nodes[st.hover.node+1:]...)
		if len(f.Nodes) < 2 {
			w.RemoveFootpath(f.ID)
			env.toast("Path removed")
			env.done()
			return true
		}
		env.commit(f, before)
		return true
	case in.leftClick && in.insert:
		before := append([]world.TrailNode(nil), f.Nodes...)
		insertPathNode(f, raw)
		env.commit(f, before)
		return true
	}
	return false
}

// dragTo moves the dragged handle to the cursor while the button is held,
// and settles the edit when it's let go.
func (env pathEnv) dragTo(st *pathTool, in pathInput, raw mgl32.Vec2) {
	w := env.w
	f := w.FootpathByID(st.focus)
	if f == nil || st.drag.node >= len(f.Nodes) {
		st.dragging = false
		return
	}
	n := &f.Nodes[st.drag.node]
	if in.groundValid {
		if st.drag.width {
			n.Width = min(max(2*raw.Sub(n.Pos).Len(), world.FootpathMinWidth), world.FootpathMaxWidth)
		} else {
			n.Pos = snapPathExcept(w, raw, f.ID)
		}
		w.RebuildFootpaths()
	}
	if in.leftHeld {
		return
	}
	st.dragging = false
	// A node clicked without being moved starts a new path from it.
	if !st.drag.width && n.Pos.Sub(st.before[st.drag.node].Pos).Len() < pathMinNodeGap/2 {
		f.Nodes = st.before
		w.RebuildFootpaths()
		st.nodes = []world.TrailNode{{Pos: f.Nodes[st.drag.node].Pos, Width: st.width}}
		return
	}
	env.commit(f, st.before)
}

// commit settles a change to path f, whose nodes were before: in the
// game the extra area is charged, or the change undone if it can't be.
func (env pathEnv) commit(f *world.Footpath, before []world.TrailNode) {
	w := env.w
	extra := world.FootpathCost(f.Nodes) - world.FootpathCost(before)
	if !env.free && extra > 0 {
		if ok, why := env.check(f.Nodes, extra); !ok {
			f.Nodes = before
			w.RebuildFootpaths()
			env.toast(why)
			return
		}
		w.Cash -= extra
	}
	w.RebuildFootpaths()
	env.done()
}

// done runs the changed hook after paths are laid or reshaped.
func (env pathEnv) done() {
	if env.changed != nil {
		env.changed()
	}
}

// check is whether a path through nodes can be laid, and why not: on
// owned land, and (costing cost, or the whole path's price when cost is
// 0) affordable, in the game.
func (env pathEnv) check(nodes []world.TrailNode, cost int) (bool, string) {
	if len(nodes) < 2 || env.free {
		return true, ""
	}
	t := env.w.Terrain
	f := world.Footpath{Nodes: nodes}
	for _, s := range f.Centerline() {
		cx, cz := int(s.Pos[0]/world.CellSize), int(s.Pos[1]/world.CellSize)
		if !t.InBounds(cx, cz) || !t.IsAccessible(cx, cz) {
			return false, "Paths can only go on land you own"
		}
	}
	if cost == 0 {
		cost = world.FootpathCost(nodes)
	}
	if !env.w.CanAfford(cost) {
		return false, fmt.Sprintf("Need %s for this path — short by %s", formatDollars(cost), formatDollars(cost-env.w.Available()))
	}
	return true, ""
}

// finish lays the path drawn so far, if it can be.
func (env pathEnv) finish(st *pathTool) {
	if len(st.nodes) < 2 {
		st.nodes = nil
		return
	}
	if ok, why := env.check(st.nodes, 0); !ok {
		env.toast(why)
		return
	}
	cost := world.FootpathCost(st.nodes)
	if !env.free {
		env.w.Cash -= cost
	}
	f := env.w.PlaceFootpath(st.nodes)
	st.nodes = nil
	env.r.SetFootpathGhost(env.w, nil, false)
	var length float32
	line := f.Centerline()
	for i := 1; i < len(line); i++ {
		length += line[i].Pos.Sub(line[i-1].Pos).Len()
	}
	if env.free {
		env.toast(fmt.Sprintf("Path laid: %.0f m", length))
	} else {
		env.toast(fmt.Sprintf("Path laid: %.0f m for %s. Guests walk twice as fast on it.", length, formatDollars(cost)))
	}
	env.done()
}

// pathSide is the unit vector across path f at node i.
func pathSide(f *world.Footpath, i int) mgl32.Vec2 {
	a := f.Nodes[max(i-1, 0)].Pos
	b := f.Nodes[min(i+1, len(f.Nodes)-1)].Pos
	d := b.Sub(a)
	if d.Len() < 1e-3 {
		return mgl32.Vec2{1, 0}
	}
	d = d.Normalize()
	return mgl32.Vec2{-d[1], d[0]}
}

// pathHandleAt is the handle of path f under p.
func pathHandleAt(f *world.Footpath, p mgl32.Vec2) pathHandle {
	for i, n := range f.Nodes {
		if n.Pos.Sub(p).Len() <= pathHandleRadius {
			return pathHandle{node: i, ok: true}
		}
	}
	for i, n := range f.Nodes {
		side := pathSide(f, i).Mul(n.Width/2 + pathHandleRadius/2)
		for _, q := range [2]mgl32.Vec2{n.Pos.Add(side), n.Pos.Sub(side)} {
			if q.Sub(p).Len() <= pathHandleRadius {
				return pathHandle{node: i, width: true, ok: true}
			}
		}
	}
	return pathHandle{}
}

// insertPathNode puts a node at p in path f, between the two nodes whose
// stretch p is nearest, as wide as they are on average.
func insertPathNode(f *world.Footpath, p mgl32.Vec2) {
	best, at := float32(1e30), 1
	for i := 1; i < len(f.Nodes); i++ {
		a, b := f.Nodes[i-1].Pos, f.Nodes[i].Pos
		ab := b.Sub(a)
		u := float32(0)
		if l2 := ab.Dot(ab); l2 > 0 {
			u = min(max(p.Sub(a).Dot(ab)/l2, 0), 1)
		}
		if d := a.Add(ab.Mul(u)).Sub(p).Len(); d < best {
			best, at = d, i
		}
	}
	width := (f.Nodes[at-1].Width + f.Nodes[at].Width) / 2
	f.Nodes = append(f.Nodes[:at], append([]world.TrailNode{{Pos: p, Width: width}}, f.Nodes[at:]...)...)
}

// snapPath is where a click at p lands: on an existing path's node if
// one is near, else on a path's centre line if p is on the path, else p.
func snapPath(w *world.World, p mgl32.Vec2) mgl32.Vec2 { return snapPathExcept(w, p, 0) }

// snapPathExcept is snapPath leaving out path skip (the one being
// edited).
func snapPathExcept(w *world.World, p mgl32.Vec2, skip uint64) mgl32.Vec2 {
	for _, f := range w.Footpaths {
		if f.ID == skip {
			continue
		}
		for _, n := range f.Nodes {
			if n.Pos.Sub(p).Len() <= pathSnapRadius {
				return n.Pos
			}
		}
	}
	if skip == 0 && w.OnFootpath(p) {
		if q, ok := w.SnapToFootpath(p); ok {
			return q
		}
	}
	return p
}

// Path handle marker tints: node, width, and the one under the cursor.
var (
	pathNodeTint  = [3]float32{0.95, 0.80, 0.45}
	pathWidthTint = [3]float32{0.55, 0.80, 0.95}
	pathHoverTint = [3]float32{1.00, 1.00, 0.70}
)

// emitPathMarkers shows the focused path's handles while the footpath
// tool is editing it. Called after the frame's ghosts are cleared, as
// the road tool's markers are.
func emitPathMarkers(r *render.Renderer, w *world.World, st *pathTool) {
	f := w.FootpathByID(st.focus)
	if f == nil || st.drawing() {
		return
	}
	active := st.hover
	if st.dragging {
		active = st.drag
	}
	var insts []render.StaticInstance
	for i, n := range f.Nodes {
		tint := pathNodeTint
		if active.ok && active.node == i && !active.width {
			tint = pathHoverTint
		}
		insts = append(insts, roadNodeMarkerInstance(n.Pos, w.Terrain, tint))
		side := pathSide(f, i).Mul(n.Width/2 + pathHandleRadius/2)
		wt := pathWidthTint
		if active.ok && active.node == i && active.width {
			wt = pathHoverTint
		}
		for _, q := range [2]mgl32.Vec2{n.Pos.Add(side), n.Pos.Sub(side)} {
			insts = append(insts, roadNodeMarkerInstance(q, w.Terrain, wt))
		}
	}
	r.SetGhosts(render.MeshRoadNode, insts)
}

// clearForPaths shovels the paths' snow and cuts their trees after paths
// are laid or reshaped, and redraws the ground and trees.
func clearForPaths(r *render.Renderer, w *world.World) {
	applyFootpathCellState(w)
	r.FlushTerrainVerts(w.Terrain)
	r.RebuildStaticBatch(w)
}
