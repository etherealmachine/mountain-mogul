package scene

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// The footpath tool, shared by the game and the scenario editor: click to
// lay a path node by node ([ and ] narrow and widen the next node),
// click the last node again or press Enter to finish, right-click to take
// back a node, Esc to drop the path. Nodes snap to existing paths' nodes
// and centre lines so paths join. The Remove tool removes a path
// (world/footpath.go).

const (
	pathSnapRadius = 4.0 // metres: how near an existing node a click snaps to it
	pathMinNodeGap = 2.0 // metres: clicks closer than this to the last node finish the path
	pathWidthStep  = 0.5 // metres per [ or ]
)

// pathTool is a footpath-tool session.
type pathTool struct {
	width    float32 // for the next node
	nodes    []world.TrailNode
	cursor   mgl32.Vec2
	cursorOK bool
}

func newPathTool() pathTool { return pathTool{width: world.FootpathDefaultWidth} }

// drawing reports whether a path is being laid.
func (st *pathTool) drawing() bool { return len(st.nodes) > 0 }

// pathEnv is what the footpath tool works in: free in the editor.
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
}

// input runs one frame of the footpath tool.
func (env pathEnv) input(st *pathTool, in pathInput) {
	if in.widen || in.narrow {
		d := float32(pathWidthStep)
		if in.narrow {
			d = -d
		}
		st.width = min(max(st.width+d, world.FootpathMinWidth), world.FootpathMaxWidth)
		env.toast(fmt.Sprintf("Path width %.1f m", st.width))
	}
	st.cursorOK = in.groundValid && !in.covered
	if st.cursorOK {
		st.cursor = snapPath(env.w, mgl32.Vec2{in.ground[0], in.ground[2]})
	}
	preview := st.nodes
	if st.cursorOK && st.drawing() {
		preview = append(append([]world.TrailNode(nil), st.nodes...), world.TrailNode{Pos: st.cursor, Width: st.width})
	}
	ok, _ := env.check(preview)
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

// check is whether a path through nodes can be laid, and why not: on
// owned land, and affordable, in the game.
func (env pathEnv) check(nodes []world.TrailNode) (bool, string) {
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
	if cost := world.FootpathCost(nodes); !env.w.CanAfford(cost) {
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
	if ok, why := env.check(st.nodes); !ok {
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
	if env.changed != nil {
		env.changed()
	}
}

// snapPath is where a click at p lands: on an existing path's node if
// one is near, else on a path's centre line if p is on the path, else p.
func snapPath(w *world.World, p mgl32.Vec2) mgl32.Vec2 {
	for _, f := range w.Footpaths {
		for _, n := range f.Nodes {
			if n.Pos.Sub(p).Len() <= pathSnapRadius {
				return n.Pos
			}
		}
	}
	if w.OnFootpath(p) {
		if q, ok := w.SnapToFootpath(p); ok {
			return q
		}
	}
	return p
}
