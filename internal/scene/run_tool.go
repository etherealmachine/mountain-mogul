package scene

import (
	"fmt"
	"math"
	"strings"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// The run tool, shared by the game and the scenario editor: trails are
// drawn as lines of nodes (world/trail_shape.go). Click near a lift top,
// building, or trail to start (it snaps and attaches), click to add
// nodes, and click a lift base, building, or trail to finish ([ and ]
// narrow and widen the next node; Enter ends it loose; right-click takes
// back a node; Shift-click places a node that doesn't join the trail
// under it, so several runs can leave one lift top side by side). A
// selected run (its popup open) shows its handles, and clicks on them or
// on the run edit it before anything else: drag a node to move it, drag
// a width handle to set the width there, click the run to insert a node,
// right-click a node to delete it, [ and ] over a node to change its
// width.

const (
	runSnapRadius   = 15   // metres: how near a lift end, door, or trail a click snaps
	runHandleRadius = 4    // metres: how near a handle a click grabs it
	runMinNodeGap   = 3    // metres: clicks closer than this to the last node are ignored
	runWidthStep    = 5    // metres per [ or ]
	runDrapeLift    = 0.25 // metres the drawn trails float over the snow
	runFillStrip    = 4.0  // metres: the most a fill quad spans across a run
	runEdgeWidth    = 0.9  // metres: a run's edge line
	runHandleSize   = 2.2  // metres: a node handle's side
)

// runTool is a run-tool session.
type runTool struct {
	diff    world.TerrainDifficulty // for a new run
	width   float32                 // for the next node
	editing uint64                  // the selected run being edited (its popup is open); 0 when drawing

	drawing bool
	nodes   []world.TrailNode
	start   world.TrailEnd

	cursor   mgl32.Vec2
	cursorOK bool
	snap     runSnap // where a click would land
	focus    uint64  // run whose handles show
	hover    runHandle
	drag     runHandle // the handle being dragged
	dragging bool
	rightAt  mgl32.Vec2
}

// runSnap is where a click lands: on something a run can attach to, or
// loose at the cursor.
type runSnap struct {
	pos  mgl32.Vec2
	end  world.TrailEnd
	what string // what it's attached to, for toasts
}

// runHandle is a handle of the focused run: node index, and whether it's
// the node's width handle; ok false when there's none.
type runHandle struct {
	node  int
	width bool
	ok    bool
}

// runEnv is what the run tool works in.
type runEnv struct {
	w       *world.World
	toast   func(string)
	changed func() // after any trail change (game: snowcat sections; editor: dirty)
}

// runInput is one frame's input for the run tool.
type runInput struct {
	toolInput
	widen, narrow bool
	newRun        bool // Shift: a node placed while drawing doesn't join the trail under it
}

func newRunTool(diff world.TerrainDifficulty) runTool {
	return runTool{diff: diff, width: world.TrailDefaultWidth}
}

// runEndLabel names what end e is attached to.
func runEndLabel(w *world.World, e world.TrailEnd) string {
	if !e.Set {
		return "nothing (loose)"
	}
	switch e.Kind {
	case world.KindLiftTop, world.KindLiftBase:
		for _, l := range w.Lifts {
			if l.ID == e.ID {
				if e.Kind == world.KindLiftTop {
					return l.Name + " top"
				}
				return l.Name + " base"
			}
		}
	case world.KindBuilding:
		if b := w.BuildingByID(e.ID); b != nil {
			return b.Label()
		}
	case world.KindTrail:
		if t := w.FindTrail(e.ID); t != nil {
			return t.Name
		}
	}
	return "nothing (loose)"
}

// snapRun is where a click at p lands: the nearest lift end within
// runSnapRadius, else a building door (or lot) there, else another
// trail (exclude aside; none with noTrails), else loose at p.
func snapRun(w *world.World, p mgl32.Vec2, exclude uint64, noTrails bool) runSnap {
	best := float32(runSnapRadius)
	s := runSnap{pos: p}
	for _, l := range w.Lifts {
		if l.IsHeli() {
			continue
		}
		for _, end := range []struct {
			pos  mgl32.Vec2
			kind world.EdgeKind
		}{{l.Top, world.KindLiftTop}, {l.Base, world.KindLiftBase}} {
			if d := end.pos.Sub(p).Len(); d < best {
				best = d
				s = runSnap{pos: end.pos, end: world.TrailEnd{Set: true, Kind: end.kind, ID: l.ID}}
			}
		}
	}
	if !s.end.Set {
		for _, b := range w.Buildings {
			if !b.IsShell() {
				continue
			}
			for _, e := range b.Entrances() {
				if d := e.Sub(p).Len(); d < best {
					best = d
					s = runSnap{pos: e, end: world.TrailEnd{Set: true, Kind: world.KindBuilding, ID: b.ID}}
				}
			}
		}
	}
	if !s.end.Set {
		c := [2]int{int(p[0] / world.CellSize), int(p[1] / world.CellSize)}
		if b := w.PaintedBuildingAt(c[0], c[1]); b != nil && b.Type == world.BuildingParking {
			s = runSnap{pos: p, end: world.TrailEnd{Set: true, Kind: world.KindBuilding, ID: b.ID}}
		}
	}
	if !s.end.Set && !noTrails {
		if t, q, ok := trailUnder(w, p, exclude); ok {
			s = runSnap{pos: q, end: world.TrailEnd{Set: true, Kind: world.KindTrail, ID: t.ID}}
		}
	}
	s.what = runEndLabel(w, s.end)
	return s
}

// trailUnder returns the trail drawn under p (exclude aside) and the
// point a run joining it there should end on: the nearest point of a
// run's centre line, or p in an area.
func trailUnder(w *world.World, p mgl32.Vec2, exclude uint64) (*world.Trail, mgl32.Vec2, bool) {
	var best *world.Trail
	var at mgl32.Vec2
	bestD := float32(math.Inf(1))
	for _, t := range w.Trails {
		if t.ID == exclude {
			continue
		}
		if t.Kind.IsArea() {
			if world.PointInPolygon(p, t.Outline) && bestD > 0 {
				best, at, bestD = t, p, 0
			}
			continue
		}
		q, d, width := nearestOnLine(t.Centerline(), p)
		if d <= width/2+2 && d < bestD {
			best, at, bestD = t, q, d
		}
	}
	return best, at, best != nil
}

// nearestOnLine is the point of line nearest p, its distance, and the
// run's width there.
func nearestOnLine(line []world.TrailSample, p mgl32.Vec2) (mgl32.Vec2, float32, float32) {
	best := float32(math.Inf(1))
	var q mgl32.Vec2
	var width float32
	for i := 1; i < len(line); i++ {
		a, b := line[i-1], line[i]
		ab := b.Pos.Sub(a.Pos)
		u := float32(0)
		if l2 := ab.Dot(ab); l2 > 0 {
			u = min(max(p.Sub(a.Pos).Dot(ab)/l2, 0), 1)
		}
		c := a.Pos.Add(ab.Mul(u))
		if d := c.Sub(p).Len(); d < best {
			best, q, width = d, c, a.Width+(b.Width-a.Width)*u
		}
	}
	return q, best, width
}

// nodeSide is the unit vector across run t at node i (to its left).
func nodeSide(t *world.Trail, i int) mgl32.Vec2 {
	a := t.Nodes[max(i-1, 0)].Pos
	b := t.Nodes[min(i+1, len(t.Nodes)-1)].Pos
	d := b.Sub(a)
	if d.Len() < 1e-3 {
		return mgl32.Vec2{1, 0}
	}
	d = d.Normalize()
	return mgl32.Vec2{-d[1], d[0]}
}

// handleAt is the handle of run t under p.
func handleAt(t *world.Trail, p mgl32.Vec2) runHandle {
	if t == nil {
		return runHandle{}
	}
	for i, n := range t.Nodes {
		if n.Pos.Sub(p).Len() <= runHandleRadius {
			return runHandle{node: i, ok: true}
		}
	}
	for i, n := range t.Nodes {
		side := nodeSide(t, i).Mul(n.Width / 2)
		for _, q := range [2]mgl32.Vec2{n.Pos.Add(side), n.Pos.Sub(side)} {
			if q.Sub(p).Len() <= runHandleRadius {
				return runHandle{node: i, width: true, ok: true}
			}
		}
	}
	return runHandle{}
}

// input runs one frame of the run tool.
func (env runEnv) input(st *runTool, in runInput) bool {
	w := env.w
	st.cursor = mgl32.Vec2{in.ground[0], in.ground[2]}
	st.cursorOK = in.groundValid && !in.covered
	if in.rightClick {
		st.rightAt = in.mouse
	}
	rightTap := in.rightRelease && in.mouse.Sub(st.rightAt).Len() < 4

	if st.dragging {
		env.dragTo(st, in)
		return true
	}
	if st.editing != 0 {
		return env.edit(st, in, rightTap)
	}

	if in.widen || in.narrow {
		d := float32(runWidthStep)
		if in.narrow {
			d = -d
		}
		st.width = clampWidth(st.width + d)
		env.toast(fmt.Sprintf("Width of the next node: %.0f m", st.width))
	}
	if !st.cursorOK {
		return false
	}
	st.snap = snapRun(w, st.cursor, 0, st.drawing && in.newRun)

	if !st.drawing {
		if in.leftClick {
			st.drawing = true
			st.nodes = []world.TrailNode{{Pos: st.snap.pos, Width: st.width}}
			st.start = st.snap.end
			env.toast(fmt.Sprintf("Starting at %s. Click to add nodes; click a lift base, building, or trail to finish, or press Enter. [ and ] set the width (%.0f m); right-click takes back a node; Shift-click places a node without joining the trail under it.", st.snap.what, st.width))
		}
		return in.leftClick
	}
	switch {
	case in.leftClick:
		if last := st.nodes[len(st.nodes)-1]; last.Pos.Sub(st.snap.pos).Len() < runMinNodeGap {
			return true
		}
		st.nodes = append(st.nodes, world.TrailNode{Pos: st.snap.pos, Width: st.width})
		if st.snap.end.Set {
			env.finish(st, st.snap.end)
		}
	case in.enter && len(st.nodes) >= 2:
		env.finish(st, world.TrailEnd{})
	case rightTap:
		st.nodes = st.nodes[:len(st.nodes)-1]
		if len(st.nodes) == 0 {
			st.drawing = false
		}
	}
	return in.leftClick
}

// edit runs a frame of editing the selected run (st.editing): its handles
// show, and clicks on them or on the run go to it before anything else.
// It reports whether it used the left click; clicks elsewhere are left
// for the scene (selecting something else).
func (env runEnv) edit(st *runTool, in runInput, rightTap bool) bool {
	t := env.w.FindTrail(st.editing)
	st.focus, st.hover, st.snap = 0, runHandle{}, runSnap{}
	if t == nil || t.Kind.IsArea() {
		return false
	}
	st.focus = t.ID
	if st.cursorOK {
		st.hover = handleAt(t, st.cursor)
	}
	if in.widen || in.narrow {
		d := float32(runWidthStep)
		if in.narrow {
			d = -d
		}
		if st.hover.ok {
			n := &t.Nodes[st.hover.node]
			n.Width = clampWidth(n.Width + d)
			env.reshaped(t)
			env.toast(fmt.Sprintf("Width here: %.0f m", n.Width))
		}
	}
	if !st.cursorOK {
		return false
	}
	_, d, width := nearestOnLine(t.Centerline(), st.cursor)
	onRun := d <= width/2
	switch {
	case in.leftClick && st.hover.ok:
		st.drag, st.dragging = st.hover, true
		return true
	case in.leftClick && onRun:
		env.insertNode(t, st.cursor)
		return true
	case rightTap && st.hover.ok && !st.hover.width:
		if len(t.Nodes) <= 2 {
			env.toast("A run needs two nodes: delete the run from its popup")
			return false
		}
		t.Nodes = append(t.Nodes[:st.hover.node], t.Nodes[st.hover.node+1:]...)
		env.fixEnds(t)
		env.reshaped(t)
	}
	return false
}

// dragTo moves the dragged handle to the cursor; letting go settles it,
// re-attaching an end node to whatever it was dropped on.
func (env runEnv) dragTo(st *runTool, in runInput) {
	w := env.w
	t := w.FindTrail(st.focus)
	if t == nil || st.drag.node >= len(t.Nodes) {
		st.dragging = false
		return
	}
	n := &t.Nodes[st.drag.node]
	if in.groundValid {
		p := mgl32.Vec2{in.ground[0], in.ground[2]}
		if st.drag.width {
			n.Width = clampWidth(2 * p.Sub(n.Pos).Len())
		} else {
			n.Pos = p
			// Moving an end node frees it until it's dropped.
			if st.drag.node == 0 {
				t.Start = world.TrailEnd{}
			} else if st.drag.node == len(t.Nodes)-1 {
				t.End = world.TrailEnd{}
			}
		}
		w.ShapeTrail(t)
	}
	if in.leftHeld {
		return
	}
	st.dragging = false
	if !st.drag.width {
		last := len(t.Nodes) - 1
		if st.drag.node == 0 || st.drag.node == last {
			s := snapRun(w, n.Pos, t.ID, false)
			n.Pos = s.pos
			if st.drag.node == 0 {
				t.Start = s.end
			} else {
				t.End = s.end
			}
			if s.end.Set {
				env.toast("Attached to " + s.what)
			}
		}
	}
	env.reshaped(t)
}

// insertNode adds a node to run t where the cursor is on it.
func (env runEnv) insertNode(t *world.Trail, p mgl32.Vec2) {
	best, at := float32(math.Inf(1)), 1
	for i := 1; i < len(t.Nodes); i++ {
		a, b := t.Nodes[i-1].Pos, t.Nodes[i].Pos
		ab := b.Sub(a)
		u := float32(0)
		if l2 := ab.Dot(ab); l2 > 0 {
			u = min(max(p.Sub(a).Dot(ab)/l2, 0), 1)
		}
		if d := a.Add(ab.Mul(u)).Sub(p).Len(); d < best {
			best, at = d, i
		}
	}
	width := (t.Nodes[at-1].Width + t.Nodes[at].Width) / 2
	t.Nodes = append(t.Nodes[:at], append([]world.TrailNode{{Pos: p, Width: width}}, t.Nodes[at:]...)...)
	env.reshaped(t)
}

// fixEnds drops attachments that no longer sit on the end nodes after
// one was deleted.
func (env runEnv) fixEnds(t *world.Trail) {
	if len(t.Nodes) == 0 {
		return
	}
	if t.Start.Set && snapRun(env.w, t.Nodes[0].Pos, t.ID, false).end != t.Start {
		t.Start = world.TrailEnd{}
	}
	if t.End.Set && snapRun(env.w, t.Nodes[len(t.Nodes)-1].Pos, t.ID, false).end != t.End {
		t.End = world.TrailEnd{}
	}
}

// reshaped settles a change to t's shape: cells, graph, and the scene's
// hook.
func (env runEnv) reshaped(t *world.Trail) {
	env.w.RebuildTrailGraph()
	if env.changed != nil {
		env.changed()
	}
}

// finish turns the nodes drawn so far into a run ending at end.
func (env runEnv) finish(st *runTool, end world.TrailEnd) {
	w := env.w
	t := w.PlaceRun("", st.diff, st.nodes, st.start, end)
	t.Groomed = true
	w.RebuildTrailGraph()
	if env.changed != nil {
		env.changed()
	}
	st.drawing, st.nodes, st.start = false, nil, world.TrailEnd{}
	env.toast(fmt.Sprintf("%s: %s to %s, %.0f m long, %.0f m drop. Click it with no tool to name it or set its difficulty.",
		t.Name, runEndLabel(w, t.Start), runEndLabel(w, t.End), t.Length(), runDrop(w, t)))
}

func clampWidth(v float32) float32 {
	return min(max(v, world.TrailMinWidth), world.TrailMaxWidth)
}

// runDrop is the height difference between run t's ends.
func runDrop(w *world.World, t *world.Trail) float32 {
	if len(t.Nodes) < 2 {
		return 0
	}
	a, b := t.Nodes[0].Pos, t.Nodes[len(t.Nodes)-1].Pos
	return float32(math.Abs(float64(w.Terrain.InterpolatedSurfaceElevationAt(a[0], a[1]) - w.Terrain.InterpolatedSurfaceElevationAt(b[0], b[1]))))
}

// runPitch is run t's average and steepest pitch along its centre line,
// in degrees, over 20 m stretches.
func runPitch(w *world.World, t *world.Trail) (avg, steepest float32) {
	line := t.Centerline()
	const stretch = 20
	var dist, drop float32
	from := 0
	for i := 1; i < len(line); i++ {
		dist += line[i].Pos.Sub(line[i-1].Pos).Len()
		a, b := line[from].Pos, line[i].Pos
		if run := b.Sub(a).Len(); run >= stretch {
			dh := float32(math.Abs(float64(w.Terrain.InterpolatedSurfaceElevationAt(a[0], a[1]) - w.Terrain.InterpolatedSurfaceElevationAt(b[0], b[1]))))
			steepest = max(steepest, float32(math.Atan2(float64(dh), float64(run))*180/math.Pi))
			from = i
		}
	}
	drop = runDrop(w, t)
	if dist > 0 {
		avg = float32(math.Atan2(float64(drop), float64(dist)) * 180 / math.Pi)
	}
	return avg, steepest
}

// ── Drawing ────────────────────────────────────────────────────────────

// trailColors is a difficulty's fill and edge colours.
func trailColors(d world.TerrainDifficulty) (fill, edge mgl32.Vec3) {
	switch d {
	case world.DiffGreen:
		return mgl32.Vec3{0.25, 0.75, 0.3}, mgl32.Vec3{0.12, 0.5, 0.16}
	case world.DiffBlue:
		return mgl32.Vec3{0.25, 0.5, 0.9}, mgl32.Vec3{0.1, 0.3, 0.7}
	}
	return mgl32.Vec3{0.15, 0.15, 0.17}, mgl32.Vec3{0.05, 0.05, 0.06}
}

// trailDraper builds draped triangles for drawn trails.
type trailDraper struct {
	w           *world.World
	t           *world.Terrain
	fill, lines []float32
}

// Joins. Runs that are attached to each other, or to the same lift
// station or building, are drawn as one: their fills merge (the renderer
// shades each pixel once) and the edges of each that fall inside the
// other are left out, so a junction or a lift station reads as one
// shape. Runs that only overlap keep both outlines crossing, and a loose
// end gets an orange bar, so a missed snap shows.

const (
	runCapSteps  = 10  // segments in a rounded end cap
	runJoinInset = 0.5 // metres: an edge this far inside a joined run is hidden
)

// runLoose is the colour of a loose end's bar.
var runLoose = mgl32.Vec3{1, 0.5, 0.1}

// joinedWith returns the trails t is drawn merged with: attached to it
// (either way), or attached to the same lift station or building.
func joinedWith(w *world.World, t *world.Trail) []*world.Trail {
	var out []*world.Trail
	ends := func(x *world.Trail) []world.TrailEnd {
		var e []world.TrailEnd
		for _, end := range [2]world.TrailEnd{x.Start, x.End} {
			if end.Set {
				e = append(e, end)
			}
		}
		return e
	}
	mine := ends(t)
	for _, o := range w.Trails {
		if o == t {
			continue
		}
		joined := false
		for _, e := range mine {
			if e.Kind == world.KindTrail && e.ID == o.ID {
				joined = true
			}
		}
		for _, e := range ends(o) {
			if e.Kind == world.KindTrail && e.ID == t.ID {
				joined = true
			}
			for _, m := range mine {
				if m.Kind != world.KindTrail && m == e {
					joined = true // the same lift station or building
				}
			}
		}
		if joined {
			out = append(out, o)
		}
	}
	return out
}

// runCovers reports whether p is inside trail t by more than inset: an
// area's outline, or a run's ribbon or the rounded cap of an attached end.
func runCovers(t *world.Trail, p mgl32.Vec2, inset float32) bool {
	if t.Kind.IsArea() {
		return world.PointInPolygon(p, t.Outline)
	}
	line := t.Centerline()
	if _, d, width := nearestOnLine(line, p); d <= width/2-inset {
		return true
	}
	if len(line) == 0 {
		return false
	}
	for i, end := range [2]world.TrailEnd{t.Start, t.End} {
		s := line[0]
		if i == 1 {
			s = line[len(line)-1]
		}
		if end.Set && s.Pos.Sub(p).Len() <= s.Width/2-inset {
			return true
		}
	}
	return false
}

// hider is the test for leaving out t's edges: inside a run it's joined to.
func (d *trailDraper) hider(t *world.Trail) func(p mgl32.Vec2) bool {
	if d.w == nil || t == nil {
		return nil
	}
	joined := joinedWith(d.w, t)
	if len(joined) == 0 {
		return nil
	}
	return func(p mgl32.Vec2) bool {
		for _, o := range joined {
			if runCovers(o, p, runJoinInset) {
				return true
			}
		}
		return false
	}
}

// y is the drawn surface at (x, z) (the terrain's triangles with their
// snow, detail, and moguls, as the GPU draws them) plus a small lift.
func (d *trailDraper) y(x, z float32) float32 {
	return render.VisualElevationAt(d.t, x, z) + runDrapeLift
}

func (d *trailDraper) tri(buf *[]float32, a, b, c mgl32.Vec2, col mgl32.Vec3) {
	for _, p := range [3]mgl32.Vec2{a, b, c} {
		*buf = append(*buf, p[0], d.y(p[0], p[1]), p[1], col[0], col[1], col[2])
	}
}

func (d *trailDraper) quad(buf *[]float32, a, b, c, e mgl32.Vec2, col mgl32.Vec3) {
	d.tri(buf, a, b, c, col)
	d.tri(buf, a, c, e, col)
}

// run drapes a run's ribbon (fill, and edge lines) along line. ends are
// what its first and last samples are attached to: an attached end gets
// a rounded cap, a loose one an orange bar (nil: neither, for a ghost).
// hide, if set, leaves out edges inside runs it's joined to.
func (d *trailDraper) run(line []world.TrailSample, fill, edge mgl32.Vec3, withFill bool, ends *[2]world.TrailEnd, hide func(mgl32.Vec2) bool) {
	if len(line) < 2 {
		return
	}
	left := make([]mgl32.Vec2, len(line))
	right := make([]mgl32.Vec2, len(line))
	for i, s := range line {
		a, b := line[max(i-1, 0)].Pos, line[min(i+1, len(line)-1)].Pos
		dir := b.Sub(a)
		if dir.Len() < 1e-4 {
			dir = mgl32.Vec2{0, 1}
		}
		dir = dir.Normalize()
		side := mgl32.Vec2{-dir[1], dir[0]}.Mul(s.Width / 2)
		left[i], right[i] = s.Pos.Add(side), s.Pos.Sub(side)
	}
	for i := 1; i < len(line); i++ {
		if withFill {
			// Strips across the run, so a wide one follows the ground
			// between its edges instead of cutting through it.
			n := max(int(math.Ceil(float64(max(line[i-1].Width, line[i].Width)/runFillStrip))), 1)
			for k := 0; k < n; k++ {
				u0, u1 := float32(k)/float32(n), float32(k+1)/float32(n)
				a0, a1 := lerp2(left[i-1], right[i-1], u0), lerp2(left[i-1], right[i-1], u1)
				b0, b1 := lerp2(left[i], right[i], u0), lerp2(left[i], right[i], u1)
				d.quad(&d.fill, a0, a1, b1, b0, fill)
			}
		}
		for _, e := range [2][]mgl32.Vec2{left, right} {
			if hide == nil || !hide(e[i-1].Add(e[i]).Mul(0.5)) {
				d.segment(e[i-1], e[i], runEdgeWidth, edge)
			}
		}
	}
	if ends == nil {
		return
	}
	n := len(line) - 1
	for k, end := range ends {
		i, prev := 0, 1
		if k == 1 {
			i, prev = n, n-1
		}
		out := line[i].Pos.Sub(line[prev].Pos)
		if out.Len() < 1e-4 {
			continue
		}
		out = out.Normalize()
		if !end.Set {
			d.segment(left[i], right[i], runEdgeWidth*2, runLoose)
			continue
		}
		d.cap(line[i].Pos, out, line[i].Width/2, fill, edge, withFill, hide)
	}
}

// cap drapes a rounded end: a half disc of radius r beyond c, facing out.
func (d *trailDraper) cap(c, out mgl32.Vec2, r float32, fill, edge mgl32.Vec3, withFill bool, hide func(mgl32.Vec2) bool) {
	side := mgl32.Vec2{-out[1], out[0]}
	pt := func(k int) mgl32.Vec2 {
		a := float64(k) / runCapSteps * math.Pi
		return c.Add(side.Mul(r * float32(math.Cos(a)))).Add(out.Mul(r * float32(math.Sin(a))))
	}
	for k := 0; k < runCapSteps; k++ {
		a, b := pt(k), pt(k+1)
		if withFill {
			// Fan from the centre, split in two so a wide cap follows the
			// ground.
			ca, cb := lerp2(c, a, 0.5), lerp2(c, b, 0.5)
			d.tri(&d.fill, c, ca, cb, fill)
			d.quad(&d.fill, ca, a, b, cb, fill)
		}
		if hide == nil || !hide(a.Add(b).Mul(0.5)) {
			d.segment(a, b, runEdgeWidth, edge)
		}
	}
}

// segment drapes a line w metres wide from a to b.
func (d *trailDraper) segment(a, b mgl32.Vec2, w float32, col mgl32.Vec3) {
	dir := b.Sub(a)
	if dir.Len() < 1e-4 {
		return
	}
	side := mgl32.Vec2{-dir[1], dir[0]}.Normalize().Mul(w / 2)
	d.quad(&d.lines, a.Add(side), a.Sub(side), b.Sub(side), b.Add(side), col)
}

// outline drapes an area's outline.
func (d *trailDraper) outline(poly []mgl32.Vec2, col mgl32.Vec3) {
	for i := range poly {
		d.segment(poly[i], poly[(i+1)%len(poly)], runEdgeWidth*1.5, col)
	}
}

// square drapes a handle centred on p.
func (d *trailDraper) square(p mgl32.Vec2, size float32, col mgl32.Vec3) {
	h := size / 2
	d.quad(&d.lines, p.Add(mgl32.Vec2{-h, -h}), p.Add(mgl32.Vec2{h, -h}), p.Add(mgl32.Vec2{h, h}), p.Add(mgl32.Vec2{-h, h}), col)
}

// trail drapes one trail.
func (d *trailDraper) trail(t *world.Trail, bright bool) {
	fill, edge := trailColors(t.Difficulty)
	if t.Groomed {
		fill = fill.Add(mgl32.Vec3{1, 1, 1}.Sub(fill).Mul(0.25))
	}
	if bright {
		edge = mgl32.Vec3{1, 0.85, 0.2}
	}
	if t.Kind.IsArea() {
		d.outline(t.Outline, edge)
		return
	}
	ends := [2]world.TrailEnd{t.Start, t.End}
	d.run(t.Centerline(), fill, edge, true, &ends, d.hider(t))
}

// buildSettledTrails drapes every trail (all when showAll, else only
// active), leaving out skip (drawn live instead).
func buildSettledTrails(w *world.World, showAll bool, active, skip uint64) (fill, lines []float32) {
	d := &trailDraper{w: w, t: w.Terrain}
	for _, t := range w.Trails {
		if t.ID == skip || (!showAll && t.ID != active) {
			continue
		}
		d.trail(t, t.ID == active)
	}
	return d.fill, d.lines
}

// buildLiveTrails drapes what changes every frame: the focused run with
// its handles, and a run being drawn with the ghost of its next node and
// the snap target.
func buildLiveTrails(w *world.World, st *runTool) (fill, lines []float32) {
	d := &trailDraper{w: w, t: w.Terrain}
	handle := mgl32.Vec3{1, 1, 1}
	hot := mgl32.Vec3{1, 0.85, 0.2}
	if t := w.FindTrail(st.focus); t != nil && !st.drawing {
		d.trail(t, true)
		for i, n := range t.Nodes {
			side := nodeSide(t, i).Mul(n.Width / 2)
			for _, q := range [2]mgl32.Vec2{n.Pos.Add(side), n.Pos.Sub(side)} {
				col := handle
				if st.hover.ok && st.hover.node == i && st.hover.width {
					col = hot
				}
				d.segment(n.Pos, q, 0.3, col)
				d.square(q, runHandleSize*0.7, col)
			}
			col := handle
			if st.hover.ok && st.hover.node == i && !st.hover.width {
				col = hot
			}
			d.square(n.Pos, runHandleSize, col)
		}
	}
	if st.drawing && len(st.nodes) > 0 {
		nodes := st.nodes
		if st.cursorOK {
			nodes = append(append([]world.TrailNode(nil), nodes...), world.TrailNode{Pos: st.snap.pos, Width: st.width})
		}
		ghost := &world.Trail{Kind: world.TrailRun, Nodes: nodes}
		fill, _ := trailColors(st.diff)
		d.run(ghost.Centerline(), fill, mgl32.Vec3{1, 1, 1}, true, nil, nil)
		for _, n := range st.nodes {
			d.square(n.Pos, runHandleSize, handle)
		}
	}
	if st.cursorOK && !st.dragging && (st.drawing || !st.hover.ok) && st.snap.end.Set {
		d.square(st.snap.pos, runHandleSize*1.6, hot)
	}
	return d.fill, d.lines
}

// trailDrawing keeps the settled trail layer up to date: redrawn when
// trails change, what's shown changes, or (for the snow under them)
// every few seconds.
type trailDrawing struct {
	version      uint64
	showAll      bool
	active, skip uint64
	drawnAt      float32
	drawnOnce    bool
	liveShown    bool // the live layer has something on it
}

// live draws the run tool's live layer while the tool is on (st non-nil),
// and clears it on the first frame it's off, however the tool was left.
func (td *trailDrawing) live(r *render.Renderer, w *world.World, st *runTool) {
	if st == nil {
		if td.liveShown {
			r.SetTrailLayer(render.TrailLayerLive, nil, nil)
			td.liveShown = false
		}
		return
	}
	fill, lines := buildLiveTrails(w, st)
	r.SetTrailLayer(render.TrailLayerLive, fill, lines)
	td.liveShown = true
}

const trailRedrape = 3 // seconds between redraws for the snow

func (td *trailDrawing) update(r *render.Renderer, w *world.World, now float32, showAll bool, active, skip uint64) {
	if td.drawnOnce && td.version == w.TrailVersion && td.showAll == showAll && td.active == active &&
		td.skip == skip && now-td.drawnAt < trailRedrape {
		return
	}
	td.version, td.showAll, td.active, td.skip, td.drawnAt, td.drawnOnce = w.TrailVersion, showAll, active, skip, now, true
	fill, lines := buildSettledTrails(w, showAll, active, skip)
	r.SetTrailLayer(render.TrailLayerSettled, fill, lines)
}

// runPopupLines adds a run's shape readouts to its popup text: length,
// drop, pitch, and ends.
func runSummary(w *world.World, t *world.Trail) string {
	if t.Kind.IsArea() {
		return fmt.Sprintf("%s, %d cells", strings.ToLower(t.Kind.Label()), len(t.Cells))
	}
	avg, steep := runPitch(w, t)
	return fmt.Sprintf("%.0f m, %.0f m drop, %.0f° average, %.0f° steepest", t.Length(), runDrop(w, t), avg, steep)
}

func lerp2(a, b mgl32.Vec2, u float32) mgl32.Vec2 { return a.Add(b.Sub(a).Mul(u)) }
