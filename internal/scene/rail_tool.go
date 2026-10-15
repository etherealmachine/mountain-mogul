package scene

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// Railways in the game (notes/next/Rail Network.md).
//
// The Rail tool (one button a style) lays track point by point: it
// starts on a joint, on a track (a junction there), or anywhere (an
// entry if it's at the map's edge); clicking a joint or a track finishes
// it there, as does clicking its last point again. A ghost of the track
// follows the mouse, red where it can't be laid.
//
// The Station tool puts a station beside a track, its platform along it
// as long as the network's train, on the side of the track the mouse is.
//
// With no tool, clicking a track selects it: its popup (style, speed,
// upgrade) and handles on its points and between them, as a creek's in
// the editor: drag a point (an end drags its joint, joining another
// joint or track it's dropped on), grab a handle between two to add a
// point, right-click a point to take it out. An edit that would make
// track too steep, or that can't be paid for, is put back.
//
// The scenario editor has the same tools, free (editor_rail.go): they
// work in a railEnv, the game's or the editor's.

// railTool is the Rail tool's session.
type railTool struct {
	style  world.RailStyle
	points []mgl32.Vec2
	start  railEnd
	// The ghost: the hover and point count it was built for, and what it
	// costs and whether it can be laid.
	ghostAt   mgl32.Vec2
	ghostN    int
	ghostCost int
}

// railEnd is what one end of track being laid is on: a joint, a track
// (at its sample i), or nothing (a new joint at pos, an entry if edge:
// on the map's edge).
type railEnd struct {
	joint *world.RailJoint
	track *world.RailTrack
	i     int
	pos   mgl32.Vec2
	edge  bool
}

// height is the rail's height at e: its joint's, its track's there, or
// over the ground at a new joint.
func (e railEnd) height(w *world.World) float32 {
	switch {
	case e.joint != nil:
		return e.joint.Y
	case e.track != nil:
		return e.track.Track[e.i].Y
	}
	return w.NewRailJoint(e.pos).Y
}

// resolveRailEnd is what world point p is on, for an end of track.
func resolveRailEnd(w *world.World, p mgl32.Vec2) railEnd {
	if j := w.JointNear(p, world.RailJointReach, 0); j != nil {
		return railEnd{joint: j, pos: j.Pos}
	}
	if tr, i, _ := w.TrackNear(p, world.RailTrackReach); tr != nil {
		return railEnd{track: tr, i: i, pos: tr.Track[i].Pos}
	}
	if q, ok := w.EdgeSnap(p); ok {
		return railEnd{pos: q, edge: true}
	}
	return railEnd{pos: p}
}

// railEnv is what the railway tools work in: the world, the mouse on
// the ground, the tools' state, and the scene's toast and popups. In the
// editor (free) nothing's paid for, and a track's popup sets its style
// instead of upgrading it.
type railEnv struct {
	w          *world.World
	r          *render.Renderer
	tool       *railTool
	edit       *railEdit
	active     toolMode
	hover      mgl32.Vec2
	hoverValid bool
	toast      func(string)
	free       bool
	// changed runs after the railway changes (the editor marks itself
	// dirty); show puts a popup up, and up is whether win is the popup up
	// now.
	changed func()
	show    func(win *ui.Window)
	up      func(win *ui.Window) bool
}

// railEnv is the game's railway tools' env.
func (s *Scenario) railEnv() railEnv {
	r := s.app.Renderer
	return railEnv{
		w: s.world, r: r, tool: &s.railTool, edit: &s.railEdit, active: s.activeTool,
		hover: mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]}, hoverValid: s.hoverValid,
		toast: s.setToast, changed: func() {},
		show: func(win *ui.Window) {
			s.placePopup(win, r.ScreenWidth(), r.ScreenHeight())
			s.popup = win
		},
		up: func(win *ui.Window) bool { return s.popup == win },
	}
}

// offMap points the Rail tool at the map's edge when the mouse (at
// screen point mouse, not over the scene's chrome) is off the map with
// track being laid: where the line from the last point toward the mouse
// leaves the map. A click there finishes the track at the edge.
func (env *railEnv) offMap(mouse mgl32.Vec2) {
	rt := env.tool
	if env.hoverValid || env.active != toolRail || len(rt.points) == 0 {
		return
	}
	t := env.w.Terrain
	last := rt.points[len(rt.points)-1]
	mx, mz := float32(t.Width-1)*world.CellSize, float32(t.Height-1)*world.CellSize
	// The mouse's ray meets the ground's height at the edge: start from
	// the last point's and settle.
	y := t.InterpolatedSurfaceElevationAt(last[0], last[1])
	var edge mgl32.Vec2
	for range 3 {
		p3 := env.r.Camera.ScreenToTerrain(mouse, y)
		p := mgl32.Vec2{p3[0], p3[2]}
		if p[0] >= 0 && p[0] <= mx && p[1] >= 0 && p[1] <= mz {
			return // over the map after all
		}
		// Where the line from last to p crosses the map's border.
		d := p.Sub(last)
		f := float32(1)
		for k, hi := range [2]float32{mx, mz} {
			switch {
			case d[k] > 0 && last[k]+d[k] > hi:
				f = min(f, (hi-last[k])/d[k])
			case d[k] < 0 && last[k]+d[k] < 0:
				f = min(f, -last[k]/d[k])
			}
		}
		edge = last.Add(d.Mul(max(f, 0)))
		edge = mgl32.Vec2{min(max(edge[0], 0), mx), min(max(edge[1], 0), mz)}
		y = t.InterpolatedSurfaceElevationAt(edge[0], edge[1])
	}
	env.hover, env.hoverValid = edge, true
}

// afford is whether cost can be paid, toasting what's short for what if
// not; always in the editor.
func (env railEnv) afford(cost int, what string) bool {
	if env.free || env.w.CanAfford(cost) {
		return true
	}
	env.toast(fmt.Sprintf("Need %s for %s: short by %s", settings.FormatDollars(cost), what, settings.FormatDollars(cost-env.w.Available())))
	return false
}

// pay pays cost, but not in the editor.
func (env railEnv) pay(cost int) {
	if !env.free && cost > 0 {
		env.w.Spend(cost)
	}
}

// priced is msg with its cost, but not in the editor.
func (env railEnv) priced(msg string, cost int) string {
	if env.free {
		return msg
	}
	return msg + ": " + settings.FormatDollars(cost)
}

// canAfford is whether cost can be paid, quietly.
func (env railEnv) canAfford(cost int) bool { return env.free || env.w.CanAfford(cost) }

// railHandleReach is how near a handle, in pixels, the mouse picks it.
const railHandleReach = 9

// railFinishReach is how near the last point a click counts as
// clicking it again (to finish there).
const railFinishReach = 8

// startRailTool picks style and turns the Rail tool on.
func (s *Scenario) startRailTool(style world.RailStyle) {
	if s.activeTool == toolRail && s.railTool.style == style {
		s.setTool(toolRail) // off
		return
	}
	s.railTool = railTool{style: style}
	if s.activeTool != toolRail {
		s.setTool(toolRail)
	}
	s.setToast("Click to lay track; finish on a joint or another track, at the map's edge (click off the map), or click the last point again.")
}

// click handles a click with the Rail tool at p.
func (env railEnv) click(p mgl32.Vec2) {
	w := env.w
	rt := env.tool
	e := resolveRailEnd(w, p)
	if len(rt.points) == 0 {
		rt.start, rt.points = e, []mgl32.Vec2{e.pos}
		return
	}
	last := rt.points[len(rt.points)-1]
	// On a joint, a track or the map's edge (an entry, always an end), it
	// finishes there.
	onNetwork := e.joint != nil || e.track != nil || e.edge
	if !onNetwork && (e.pos.Sub(last).Len() > railFinishReach || len(rt.points) < 2) {
		rt.points = append(rt.points, e.pos)
		return
	}
	points := rt.points
	if onNetwork {
		points = append(append([]mgl32.Vec2(nil), points...), e.pos)
	} else {
		e = railEnd{pos: last} // finishing at its last point
	}
	if env.lay(rt.style, rt.start, e, points) {
		*rt = railTool{style: rt.style}
	}
}

// lay lays track of style through points from end a to end b, if it can
// be laid and paid for.
func (env railEnv) lay(style world.RailStyle, a, b railEnd, points []mgl32.Vec2) bool {
	w := env.w
	ya, yb := a.height(w), b.height(w)
	if why := world.TrackProblem(style, points, ya, yb); why != "" {
		env.toast(why)
		return false
	}
	cost := world.TrackCost(w.PreviewTrack(style, points, ya, yb))
	if !env.afford(cost, "this track") {
		return false
	}
	ja := railJoint(w, a)
	// Splitting for the first end may have replaced the track the second
	// was on: find it again.
	if b.track != nil {
		b = resolveRailEnd(w, b.pos)
	}
	jb := railJoint(w, b)
	if ja == jb {
		env.toast("Track has to run between two different places")
		return false
	}
	points[0], points[len(points)-1] = ja.Pos, jb.Pos
	env.pay(cost)
	w.AddRailTrack(style, points, ja, jb, keepClaimed(w))
	env.railChanged()
	env.r.SetRailGhost(nil, nil, [3]float32{})
	env.toast(env.priced(style.Spec().Label+" track laid", cost))
	return true
}

// railJoint is the joint for end e: its own, a junction split into its
// track, or a new one.
func railJoint(w *world.World, e railEnd) *world.RailJoint {
	switch {
	case e.joint != nil:
		return e.joint
	case e.track != nil:
		return w.SplitTrack(e.track, e.i)
	}
	return w.AddRailJoint(w.NewRailJoint(e.pos))
}

// railChanged redraws the ground and the railway after it changes.
func (env railEnv) railChanged() {
	w := env.w
	w.Terrain.RestampTreeWells()
	env.r.FlushTerrainVerts(w.Terrain)
	env.r.RebuildStaticBatch(w)
	env.changed()
}

// updateRailGhost shows the track as it would be laid with the mouse at
// the next point.
func (env railEnv) updateGhost() {
	rt := env.tool
	w := env.w
	r := env.r
	if len(rt.points) == 0 || !env.hoverValid {
		if rt.ghostN != 0 {
			r.SetRailGhost(nil, nil, [3]float32{})
			rt.ghostN = 0
		}
		return
	}
	hover := resolveRailEnd(w, env.hover)
	points := append([]mgl32.Vec2(nil), rt.points...)
	end := railEnd{pos: points[len(points)-1]}
	if hover.pos.Sub(end.pos).Len() > railFinishReach || hover.joint != nil || hover.track != nil || hover.edge {
		points = append(points, hover.pos)
		end = hover
	}
	if len(points) == rt.ghostN && hover.pos.Sub(rt.ghostAt).Len() < 1 {
		return
	}
	rt.ghostAt, rt.ghostN = hover.pos, len(points)
	if len(points) < 2 {
		r.SetRailGhost(nil, nil, [3]float32{})
		return
	}
	ya, yb := rt.start.height(w), end.height(w)
	tr := w.PreviewTrack(rt.style, points, ya, yb)
	rt.ghostCost = world.TrackCost(tr)
	r.SetRailGhost(tr, w.Terrain, ghostTint(env.canAfford(rt.ghostCost), world.TrackProblem(rt.style, points, ya, yb) == ""))
}

// ---- Stations ----

// stationSpot is where the Station tool would put a station for the
// mouse at p: on a track, its platform's middle and side, the station's
// price, and why it can't go there ("" if it can).
type stationSpot struct {
	st    world.RailStation
	cost  int
	ok    bool
	why   string
	train world.RailStyle
}

// except is a station left out (one being moved).
func stationSpotAt(w *world.World, p mgl32.Vec2, except uint64) stationSpot {
	tr, i, _ := w.TrackNear(p, 15)
	if tr == nil {
		return stationSpot{why: "Put a station beside a track"}
	}
	_, of := w.RailNetworks()
	train := of[tr.ID].Train
	sample := tr.Track[i]
	_, _, heading, _ := tr.At(sample.Dist)
	dir := mgl32.Vec2{float32(math.Sin(float64(heading))), float32(math.Cos(float64(heading)))}
	side := float32(1)
	if off := p.Sub(sample.Pos); dir[0]*off[1]-dir[1]*off[0] < 0 {
		side = -1
	}
	sp := stationSpot{st: world.RailStation{TrackID: tr.ID, Dist: sample.Dist, Pos: sample.Pos, Side: side, Interval: world.DefaultRailInterval}, train: train}
	sp.cost = train.Spec().StationCost
	half := train.Spec().PlatformLength() / 2
	switch {
	case sample.Tunnel:
		sp.why = "Not in a tunnel"
	case sample.Dist < half || sample.Dist > tr.Length()-half:
		sp.why = fmt.Sprintf("Its platform needs %.0f m of track clear of junctions and ends", 2*half)
	default:
		sp.ok = true
		for _, o := range w.RailStations {
			if o.ID != except && o.TrackID == tr.ID && abs32f(o.Dist-sample.Dist) < 2*half {
				sp.ok, sp.why = false, "Too near another station"
			}
		}
	}
	return sp
}

func abs32f(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// updateStationGhost shows the station the Station tool would place.
func (env railEnv) updateStationGhost() {
	if !env.hoverValid {
		return
	}
	sp := stationSpotAt(env.w, env.hover, 0)
	if sp.st.TrackID == 0 {
		return
	}
	m, ok := render.StationTransform(env.w, &sp.st)
	if !ok {
		return
	}
	inst := render.StaticInstance{ColorTint: ghostTint(env.canAfford(sp.cost), sp.ok)}
	copy(inst.Transform[:], m[:])
	env.r.SetGhosts(render.MeshRailStation, []render.StaticInstance{inst})
}

// stationClick places a station where the Station tool is pointing.
func (env railEnv) stationClick(p mgl32.Vec2) {
	w := env.w
	sp := stationSpotAt(w, p, 0)
	if !sp.ok {
		env.toast(sp.why)
		return
	}
	if !env.afford(sp.cost, "a station") {
		return
	}
	env.pay(sp.cost)
	st := sp.st
	b := &world.Building{ID: w.NextID(), Type: world.BuildingStation}
	st.ID = b.ID
	w.Buildings = append(w.Buildings, b)
	w.RailStations = append(w.RailStations, &st)
	b.Pos = w.PlatformSpot(&st)
	w.RailRev++
	env.r.RebuildStaticBatch(w)
	env.changed()
	env.toast(env.priced("Station built", sp.cost))
}

// moveStation moves station b (its building) to the track spot nearest
// p, if a station can stand there; whether it moved.
func moveStation(r *render.Renderer, w *world.World, b *world.Building, p mgl32.Vec2) bool {
	st := w.RailStationByID(b.ID)
	if st == nil {
		return false
	}
	sp := stationSpotAt(w, p, st.ID)
	if !sp.ok || (sp.st.TrackID == st.TrackID && sp.st.Dist == st.Dist && sp.st.Side == st.Side) {
		return false
	}
	st.TrackID, st.Dist, st.Pos, st.Side = sp.st.TrackID, sp.st.Dist, sp.st.Pos, sp.st.Side
	b.Pos = w.PlatformSpot(st)
	w.RailRev++
	r.RebuildRails(w)
	r.RebuildStaticBatch(w)
	return true
}

// flipStation turns station b's platform to the other side of its track.
func flipStation(r *render.Renderer, w *world.World, b *world.Building) {
	st := w.RailStationByID(b.ID)
	if st == nil {
		return
	}
	st.Side = -st.Side
	b.Pos = w.PlatformSpot(st)
	w.RailRev++
	r.RebuildRails(w)
	r.RebuildStaticBatch(w)
}

// openStationPopup shows station st: its train, its service, how often,
// and who it's bringing.
func (env railEnv) openStationPopup(st *world.RailStation) {
	w := env.w
	win := ui.NewWindow("Station", 0, 0)
	win.AddLabel("Trains", func() string {
		r, style, why := w.StationRoute(st, false)
		if r == nil {
			return "none: " + why
		}
		spec := style.Spec()
		kind := "out and back"
		if !r.Back {
			kind = "through"
		}
		return fmt.Sprintf("%s, %s, %d riders", lowerFirstWord(spec.Label), kind, spec.Capacity)
	})
	win.AddIntStepperFn("Trains every",
		func() string { return fmt.Sprintf("%.0f minutes", st.Interval) },
		func() { st.Interval = max(st.Interval/2, 15); env.changed() },
		func() { st.Interval = min(st.Interval*2, 240); env.changed() })
	if env.free {
		win.Visible = true
		env.show(win)
		return
	}
	win.AddLabel("Booked", func() string {
		n := 0
		for _, g := range st.Bound {
			n += len(g)
		}
		return fmt.Sprintf("%d guests on their way", n)
	})
	win.AddLabel("Here by train", func() string {
		n := 0
		for _, g := range w.OnMountain {
			if g.CarLot == st.ID {
				n++
			}
		}
		return fmt.Sprintf("%d on the mountain", n)
	})
	win.Visible = true
	env.show(win)
}

func lowerFirstWord(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]+'a'-'A') + s[1:]
}

// ---- Selecting and editing track ----

// railEdit is the selected track and an edit of it in progress.
type railEdit struct {
	track uint64 // selected track, 0 for none
	// win is its popup: closing it ends the selection.
	win   *ui.Window
	hover railHandle
	drag  railHandle
	// before is the railway and its cost when the drag started.
	before     world.RailSnapshot
	beforeCost int
	moved      bool
}

// railHandle is a handle on the selected track: point i, or the one
// between points i and i+1 (mid).
type railHandle struct {
	i       int
	mid, ok bool
}

func (e *railEdit) clear() { *e = railEdit{} }

// railSelected is the selected track, while its popup's open.
func (env railEnv) selected() *world.RailTrack {
	e := env.edit
	if e.track == 0 || e.win == nil || !env.up(e.win) || !e.win.Visible {
		if !e.drag.ok {
			e.clear()
		}
		return nil
	}
	return env.w.RailTrackByID(e.track)
}

// handlePos is a handle's place in the world.
func (env railEnv) handlePos(p mgl32.Vec2) mgl32.Vec3 {
	return mgl32.Vec3{p[0], render.VisualElevationAt(env.w.Terrain, p[0], p[1]) + 1, p[1]}
}

// railMid is the handle between points i and i+1 of points: on the curve.
func railMid(points []mgl32.Vec2, i int) mgl32.Vec2 {
	ps := make([][2]float32, len(points))
	for k, p := range points {
		ps[k] = [2]float32{p[0], p[1]}
	}
	m := world.CurveAt(ps, i, 0.5)
	return mgl32.Vec2{m[0], m[1]}
}

// railHandleAt is the selected track's handle under screen point mouse,
// points before midpoints.
func (env railEnv) handleAt(mouse mgl32.Vec2) railHandle {
	r := env.r
	tr := env.w.RailTrackByID(env.edit.track)
	if tr == nil {
		return railHandle{}
	}
	best, bestD := railHandle{}, float32(railHandleReach)
	try := func(h railHandle, p mgl32.Vec2) {
		sx, sy, ok := r.WorldToScreen(env.handlePos(p))
		if d := (mgl32.Vec2{sx, sy}).Sub(mouse).Len(); ok && d < bestD {
			best, bestD = h, d
			best.ok = true
		}
	}
	for i, p := range tr.Points {
		try(railHandle{i: i}, p)
	}
	if best.ok {
		return best
	}
	for i := 0; i+1 < len(tr.Points); i++ {
		try(railHandle{i: i, mid: true}, railMid(tr.Points, i))
	}
	return best
}

// railEditInput handles the selected track's handles with no tool:
// whether it used the mouse this frame.
func (env railEnv) editInput(inp *engine.Input) bool {
	e := env.edit
	w := env.w
	tr := w.RailTrackByID(e.track)
	if tr == nil || (env.selected() == nil && !e.drag.ok) {
		return false
	}
	if e.drag.ok {
		if inp.LeftHeld {
			if env.hoverValid {
				env.dragTo(tr, env.hover)
			}
		} else {
			env.release(tr)
		}
		return true
	}
	e.hover = env.handleAt(inp.MousePos)
	if !e.hover.ok {
		return false
	}
	switch {
	case inp.LeftClick && !inp.LeftClickConsumed:
		e.before, e.beforeCost, e.moved = w.SnapshotRail(), w.RailCost(), false
		e.drag = e.hover
		if e.drag.mid {
			// A new point there, dragged from now on.
			i := e.drag.i + 1
			p := railMid(tr.Points, e.drag.i)
			tr.Points = append(tr.Points[:i], append([]mgl32.Vec2{p}, tr.Points[i:]...)...)
			e.drag = railHandle{i: i, ok: true}
			e.moved = true
		}
		inp.LeftClickConsumed = true
		return true
	case inp.RightClick:
		if e.hover.mid || e.hover.i == 0 || e.hover.i == len(tr.Points)-1 || len(tr.Points) <= 2 {
			return true // an end, or nothing left to take out
		}
		e.before, e.beforeCost = w.SnapshotRail(), w.RailCost()
		tr.Points = append(tr.Points[:e.hover.i], tr.Points[e.hover.i+1:]...)
		env.settle([]*world.RailTrack{tr})
		return true
	}
	return false
}

// railDragTo moves the dragged handle to p: a point of the track, or its
// end's joint (and every track at it).
func (env railEnv) dragTo(tr *world.RailTrack, p mgl32.Vec2) {
	e := env.edit
	w := env.w
	e.moved = true
	if e.drag.i == 0 || e.drag.i == len(tr.Points)-1 {
		j := w.RailJointByID(tr.Ends[min(e.drag.i, 1)])
		if j != nil {
			w.MoveJoint(j, p)
		}
	} else {
		tr.Points[e.drag.i] = p
		w.LayOutTrack(tr)
		w.RailRev++
	}
	env.r.RebuildRails(w)
}

// release ends a drag: an end dropped on another joint joins it,
// on another track makes a junction there; then the edit's settled.
func (env railEnv) release(tr *world.RailTrack) {
	e := env.edit
	w := env.w
	drag := e.drag
	e.drag = railHandle{}
	if !e.moved {
		return
	}
	var changed []*world.RailTrack
	if drag.i == 0 || drag.i == len(tr.Points)-1 {
		j := w.RailJointByID(tr.Ends[min(drag.i, 1)])
		if j != nil {
			if o := w.JointNear(j.Pos, world.RailJointReach, j.ID); o != nil {
				w.MergeJoints(j, o)
				j = o
			} else if ot, i, _ := w.TrackNear(j.Pos, world.RailTrackReach); ot != nil && ot.Ends[0] != j.ID && ot.Ends[1] != j.ID {
				nj := w.SplitTrack(ot, i)
				w.MergeJoints(j, nj)
				j = nj
			}
			changed = w.TracksAt(j.ID)
		}
	} else {
		changed = []*world.RailTrack{tr}
	}
	if w.RailTrackByID(e.track) == nil {
		e.track = 0 // merged away
	}
	env.settle(changed)
}

// settle checks an edit of tracks: too steep, or not paid for, and
// it's put back as it was; else charged the difference and the ground
// graded to the tracks that moved.
func (env railEnv) settle(tracks []*world.RailTrack) {
	e := env.edit
	w := env.w
	for _, tr := range tracks {
		w.LayOutTrack(tr)
	}
	why := ""
	for _, tr := range tracks {
		a, b := w.RailJointByID(tr.Ends[0]), w.RailJointByID(tr.Ends[1])
		if a == nil || b == nil {
			continue
		}
		if p := world.TrackProblem(tr.Style, tr.Points, a.Y, b.Y); p != "" {
			why = p
		}
	}
	extra := w.RailCost() - e.beforeCost
	if why == "" && extra > 0 && !env.canAfford(extra) {
		why = fmt.Sprintf("Need %s for this change: short by %s", settings.FormatDollars(extra), settings.FormatDollars(extra-w.Available()))
	}
	if why != "" {
		w.RestoreRail(e.before)
		env.toast(why)
		env.r.RebuildRails(w)
		return
	}
	env.pay(extra)
	keep := keepClaimed(w)
	for _, tr := range tracks {
		w.GradeTrack(tr, keep)
	}
	w.PlaceStations()
	env.railChanged()
}

// trackAt is the track whose line passes within reach of p, for
// selecting.
func trackAt(w *world.World, p mgl32.Vec2) *world.RailTrack {
	tr, _, _ := w.TrackNear(p, world.RailTrackReach)
	return tr
}

// openTrackPopup selects track tr and shows its popup: its style, speed,
// sign, and the upgrade to the next style (in the editor, buttons for
// each style).
func (env railEnv) openTrackPopup(tr *world.RailTrack) {
	w := env.w
	*env.edit = railEdit{track: tr.ID}
	spec := tr.Style.Spec()
	win := ui.NewWindow(spec.Label+" track", 0, 0)
	win.AddLabel("Length", func() string { return fmt.Sprintf("%.0f m", tr.Length()) })
	win.AddLabel("Speed", func() string { return fmt.Sprintf("up to %.0f km/h", tr.Style.Spec().Speed*3.6) })
	win.AddLabel("Note", func() string {
		_, of := w.RailNetworks()
		n := of[tr.ID]
		switch w.TrackSigns()[tr.ID] {
		case world.SignWarning:
			return "not electrified: the " + lowerFirstWord(n.Train.Spec().Label) + "'s trains can't run here"
		case world.SignSlow:
			return fmt.Sprintf("high-speed trains slow to %.0f km/h here", tr.Style.Spec().Speed*3.6)
		}
		return "-"
	})
	switch {
	case env.free:
		for style := world.RailStyle(0); style < world.RailStyles; style++ {
			if style != tr.Style {
				win.AddActionButton("Make it "+lowerFirstWord(style.Spec().Label), func() { env.restyleTrack(tr, style) })
			}
		}
	case tr.Style+1 < world.RailStyles:
		next := tr.Style + 1
		cost := world.UpgradeCost(tr, next)
		win.AddActionButton(fmt.Sprintf("Upgrade to %s (%s)", lowerFirstWord(next.Spec().Label), settings.FormatDollars(cost)), func() {
			env.restyleTrack(tr, next)
		})
	}
	win.Visible = true
	env.show(win)
	env.edit.win = win
}

// restyleTrack makes track tr style (an upgrade, in the game), if it can
// be.
func (env railEnv) restyleTrack(tr *world.RailTrack, style world.RailStyle) {
	w := env.w
	a, b := w.RailJointByID(tr.Ends[0]), w.RailJointByID(tr.Ends[1])
	if a == nil || b == nil {
		return
	}
	if why := world.TrackProblem(style, tr.Points, a.Y, b.Y); why != "" {
		env.toast(why)
		return
	}
	cost := world.UpgradeCost(tr, style)
	if !env.afford(cost, "the upgrade") {
		return
	}
	env.pay(cost)
	tr.Style = style
	w.LayOutTrack(tr)
	w.GradeTrack(tr, keepClaimed(w))
	env.railChanged()
	env.toast(env.priced("Now "+lowerFirstWord(style.Spec().Label), cost))
	env.openTrackPopup(tr)
}

// drawRailHandles draws the selected track's points and the dots between
// them, the one under the cursor or dragged highlighted.
func (env railEnv) drawHandles(r *render.Renderer) {
	e := env.edit
	tr := env.selected()
	if tr == nil || env.active != toolNone {
		return
	}
	active := e.hover
	if e.drag.ok {
		active = e.drag
	}
	dot := func(h railHandle, p mgl32.Vec2, radius float32) {
		sx, sy, ok := r.WorldToScreen(env.handlePos(p))
		if !ok {
			return
		}
		fill := mgl32.Vec4{0.95, 0.55, 0.2, 1}
		if h.mid {
			fill = mgl32.Vec4{0.95, 0.95, 0.95, 0.7}
		}
		if active.ok && h.i == active.i && h.mid == active.mid {
			fill, radius = mgl32.Vec4{1, 0.9, 0.47, 1}, radius+2
		}
		r.DrawColorDisc(sx, sy, radius+1.5, mgl32.Vec4{0.05, 0.1, 0.15, 0.9})
		r.DrawColorDisc(sx, sy, radius, fill)
	}
	for i := 0; i+1 < len(tr.Points); i++ {
		dot(railHandle{i: i, mid: true}, railMid(tr.Points, i), 3)
	}
	for i, p := range tr.Points {
		dot(railHandle{i: i}, p, 5)
	}
}

// drawRailMarks draws the railway's signs: a warning over unelectrified
// track its network's trains can't run on, a speed limit over track that
// slows them; and, while laying track, its joints as snap targets.
func (env railEnv) drawMarks(r *render.Renderer) {
	w := env.w
	if len(w.RailTracks) == 0 {
		return
	}
	for id, sign := range w.TrackSigns() {
		tr := w.RailTrackByID(id)
		if tr == nil || len(tr.Track) == 0 {
			continue
		}
		pos, y, _, _ := tr.At(tr.Length() / 2)
		sx, sy, ok := r.WorldToScreen(mgl32.Vec3{pos[0], y + 6, pos[1]})
		if !ok {
			continue
		}
		if sign == world.SignWarning {
			ui.DrawWarningSign(r, sx, sy, 13)
		} else {
			ui.DrawSpeedSign(r, sx, sy, 14, fmt.Sprintf("%.0f", tr.Style.Spec().Speed*3.6))
		}
	}
	if env.active == toolRail {
		for _, j := range w.RailJoints {
			if sx, sy, ok := r.WorldToScreen(env.handlePos(j.Pos)); ok {
				fill := mgl32.Vec4{0.95, 0.55, 0.2, 0.9}
				if j.Entry {
					fill = mgl32.Vec4{0.3, 0.75, 1, 0.9}
				}
				r.DrawColorDisc(sx, sy, 6.5, mgl32.Vec4{0.05, 0.1, 0.15, 0.9})
				r.DrawColorDisc(sx, sy, 5, fill)
			}
		}
	}
}
