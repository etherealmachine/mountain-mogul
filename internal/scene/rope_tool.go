package scene

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// The rope tool (world/rope.go): rope lines on poles that guests try not
// to cross.
//
// Stringing: click to put up a rope pole by pole, click the last pole
// again or press Enter to finish, right-click to take back a pole, Esc
// to drop the rope. Poles snap to existing ropes' poles, so ropes join.
//
// Editing, while not stringing one: the rope under the cursor shows its
// poles. Drag a pole to move it (a click that doesn't move it starts a
// new rope from it); right-click a pole to take it out (a rope left with
// one goes); Shift-click a rope to put a pole in it. In the game, an
// edit that makes a rope longer charges the extra, and is undone if that
// can't be paid. The Remove tool takes down a whole rope.

const (
	ropeSnapRadius   = 2.0 // metres: how near an existing pole a click snaps to it
	ropeMinNodeGap   = 1.0 // metres: clicks closer than this to the last pole finish the rope
	ropeHandleRadius = 1.5 // metres: how near a pole a click grabs it
	ropeHoverReach   = 2.0 // metres from a rope that still shows its poles
)

// ropeTool is a rope-tool session.
type ropeTool struct {
	nodes    []mgl32.Vec2
	cursor   mgl32.Vec2
	cursorOK bool

	// Editing: the rope whose poles show, the pole under the cursor (-1
	// for none), and the one being dragged with the rope's nodes before.
	focus    uint64
	hover    int
	drag     int
	dragging bool
	before   []mgl32.Vec2
}

func newRopeTool() ropeTool { return ropeTool{hover: -1} }

// drawing reports whether a rope is being strung.
func (st *ropeTool) drawing() bool { return len(st.nodes) > 0 }

// ropeEnv is what the rope tool works in: free in the editor.
type ropeEnv struct {
	w     *world.World
	r     *render.Renderer
	toast func(string)
	free  bool
}

// ropeInput is one frame's input for the rope tool.
type ropeInput struct {
	toolInput
	insert bool // Shift: a click on a rope puts a pole in it
}

// input runs one frame of the rope tool.
func (env ropeEnv) input(st *ropeTool, in ropeInput) {
	st.cursorOK = in.groundValid && !in.covered
	raw := mgl32.Vec2{in.ground[0], in.ground[2]}
	if st.dragging {
		env.dragTo(st, in, raw)
		return
	}
	if st.cursorOK {
		st.cursor = snapRope(env.w, raw, 0)
	}
	if !st.drawing() && env.edit(st, in, raw) {
		env.r.SetRopeGhost(env.w, nil, false)
		return
	}
	preview := st.nodes
	if st.cursorOK && st.drawing() {
		preview = append(append([]mgl32.Vec2(nil), st.nodes...), st.cursor)
	}
	ok, _ := env.check(preview, 0)
	env.r.SetRopeGhost(env.w, preview, ok)

	switch {
	case in.rightClick && st.drawing():
		st.nodes = st.nodes[:len(st.nodes)-1]
	case in.enter && st.drawing():
		env.finish(st)
	case in.leftClick && st.cursorOK:
		if n := len(st.nodes); n > 0 && st.nodes[n-1].Sub(st.cursor).Len() < ropeMinNodeGap {
			env.finish(st) // clicking the last pole again ends the rope
			return
		}
		st.nodes = append(st.nodes, st.cursor)
		if len(st.nodes) == 1 {
			env.toast("Click to add poles; click the last pole again or press Enter to finish. Right-click takes back a pole.")
		}
	}
}

// edit handles a frame of editing the rope under the cursor, and reports
// whether it used the frame's input.
func (env ropeEnv) edit(st *ropeTool, in ropeInput, raw mgl32.Vec2) bool {
	w := env.w
	st.focus, st.hover = 0, -1
	if !st.cursorOK {
		return false
	}
	rp := w.RopeAt(raw, ropeHoverReach)
	if rp == nil {
		return false
	}
	st.focus = rp.ID
	for i, n := range rp.Nodes {
		if n.Sub(raw).Len() <= ropeHandleRadius {
			st.hover = i
			break
		}
	}
	switch {
	case in.leftClick && st.hover >= 0:
		st.drag, st.dragging = st.hover, true
		st.before = append([]mgl32.Vec2(nil), rp.Nodes...)
		return true
	case in.rightClick && st.hover >= 0:
		before := append([]mgl32.Vec2(nil), rp.Nodes...)
		rp.Nodes = append(rp.Nodes[:st.hover], rp.Nodes[st.hover+1:]...)
		if len(rp.Nodes) < 2 {
			w.RemoveRope(rp.ID)
			env.toast("Rope taken down")
			return true
		}
		env.commit(rp, before)
		return true
	case in.leftClick && in.insert:
		before := append([]mgl32.Vec2(nil), rp.Nodes...)
		insertRopeNode(rp, raw)
		env.commit(rp, before)
		return true
	}
	return false
}

// dragTo moves the dragged pole to the cursor while the button is held,
// and settles the edit when it's let go.
func (env ropeEnv) dragTo(st *ropeTool, in ropeInput, raw mgl32.Vec2) {
	w := env.w
	rp := w.RopeByID(st.focus)
	if rp == nil || st.drag >= len(rp.Nodes) {
		st.dragging = false
		return
	}
	if in.groundValid {
		rp.Nodes[st.drag] = snapRope(w, raw, rp.ID)
		w.RebuildRopes()
	}
	if in.leftHeld {
		return
	}
	st.dragging = false
	// A pole clicked without being moved starts a new rope from it.
	if rp.Nodes[st.drag].Sub(st.before[st.drag]).Len() < ropeMinNodeGap/2 {
		rp.Nodes = st.before
		w.RebuildRopes()
		st.nodes = []mgl32.Vec2{rp.Nodes[st.drag]}
		return
	}
	env.commit(rp, st.before)
}

// commit settles a change to rope rp, whose nodes were before: in the
// game a longer rope is charged the extra, or the change undone if it
// can't be.
func (env ropeEnv) commit(rp *world.Rope, before []mgl32.Vec2) {
	w := env.w
	extra := world.RopeCost(rp.Nodes) - world.RopeCost(before)
	if !env.free && extra > 0 {
		if ok, why := env.check(rp.Nodes, extra); !ok {
			rp.Nodes = before
			w.RebuildRopes()
			env.toast(why)
			return
		}
		w.Cash -= extra
	}
	w.RebuildRopes()
}

// check is whether a rope through nodes can be strung, and why not: on
// owned land, and (costing cost, or the whole rope's price when cost is
// 0) affordable, in the game.
func (env ropeEnv) check(nodes []mgl32.Vec2, cost int) (bool, string) {
	if len(nodes) < 2 || env.free {
		return true, ""
	}
	t := env.w.Terrain
	for i := 1; i < len(nodes); i++ {
		a, b := nodes[i-1], nodes[i]
		n := int(b.Sub(a).Len()/world.CellSize) + 1
		for k := 0; k <= n; k++ {
			p := a.Add(b.Sub(a).Mul(float32(k) / float32(n)))
			if !t.IsAccessibleWorld(p[0], p[1]) {
				return false, "Ropes can only go on land you own"
			}
		}
	}
	if cost == 0 {
		cost = world.RopeCost(nodes)
	}
	if !env.w.CanAfford(cost) {
		return false, fmt.Sprintf("Need %s for this rope — short by %s", formatDollars(cost), formatDollars(cost-env.w.Available()))
	}
	return true, ""
}

// finish strings the rope drawn so far, if it can be.
func (env ropeEnv) finish(st *ropeTool) {
	if len(st.nodes) < 2 {
		st.nodes = nil
		return
	}
	if ok, why := env.check(st.nodes, 0); !ok {
		env.toast(why)
		return
	}
	cost := world.RopeCost(st.nodes)
	if !env.free {
		env.w.Cash -= cost
	}
	env.w.PlaceRope(st.nodes)
	length := world.RopeLength(st.nodes)
	st.nodes = nil
	env.r.SetRopeGhost(env.w, nil, false)
	if env.free {
		env.toast(fmt.Sprintf("Rope strung: %.0f m", length))
	} else {
		env.toast(fmt.Sprintf("Rope strung: %.0f m for %s. Guests try not to cross it.", length, formatDollars(cost)))
	}
}

// insertRopeNode puts a pole at p in rope rp, between the two poles whose
// stretch p is nearest.
func insertRopeNode(rp *world.Rope, p mgl32.Vec2) {
	best, at := float32(1e30), 1
	for i := 1; i < len(rp.Nodes); i++ {
		a, b := rp.Nodes[i-1], rp.Nodes[i]
		ab := b.Sub(a)
		u := float32(0)
		if l2 := ab.Dot(ab); l2 > 0 {
			u = min(max(p.Sub(a).Dot(ab)/l2, 0), 1)
		}
		if d := a.Add(ab.Mul(u)).Sub(p).Len(); d < best {
			best, at = d, i
		}
	}
	rp.Nodes = append(rp.Nodes[:at], append([]mgl32.Vec2{p}, rp.Nodes[at:]...)...)
}

// snapRope is where a click at p lands: on another rope's pole if one is
// near (leaving out rope skip, the one being edited), else p.
func snapRope(w *world.World, p mgl32.Vec2, skip uint64) mgl32.Vec2 {
	for _, rp := range w.Ropes {
		if rp.ID == skip {
			continue
		}
		for _, n := range rp.Nodes {
			if n.Sub(p).Len() <= ropeSnapRadius {
				return n
			}
		}
	}
	return p
}

// emitRopeMarkers shows the focused rope's poles while the rope tool is
// editing it.
func emitRopeMarkers(r *render.Renderer, w *world.World, st *ropeTool) {
	rp := w.RopeByID(st.focus)
	if rp == nil || st.drawing() {
		return
	}
	active := st.hover
	if st.dragging {
		active = st.drag
	}
	var insts []render.StaticInstance
	for i, n := range rp.Nodes {
		tint := pathNodeTint
		if i == active {
			tint = pathHoverTint
		}
		insts = append(insts, roadNodeMarkerInstance(n, w.Terrain, tint))
	}
	r.SetGhosts(render.MeshRoadNode, insts)
}
