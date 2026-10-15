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
// (at its sample i), or nothing (a new joint at pos).
type railEnd struct {
	joint *world.RailJoint
	track *world.RailTrack
	i     int
	pos   mgl32.Vec2
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
		p = q
	}
	return railEnd{pos: p}
}

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
	s.setToast("Click to lay track; finish on a joint or another track, or click the last point again. At the map's edge it comes in from beyond.")
}

// railClick handles a click with the Rail tool at world (wx, wz).
func (s *Scenario) railClick(r *render.Renderer, wx, wz float32) {
	w := s.world
	rt := &s.railTool
	e := resolveRailEnd(w, mgl32.Vec2{wx, wz})
	if len(rt.points) == 0 {
		rt.start, rt.points = e, []mgl32.Vec2{e.pos}
		return
	}
	last := rt.points[len(rt.points)-1]
	onNetwork := e.joint != nil || e.track != nil
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
	if s.layTrack(r, rt.style, rt.start, e, points) {
		*rt = railTool{style: rt.style}
	}
}

// layTrack lays track of style through points from end a to end b, if
// it can be laid and paid for.
func (s *Scenario) layTrack(r *render.Renderer, style world.RailStyle, a, b railEnd, points []mgl32.Vec2) bool {
	w := s.world
	ya, yb := a.height(w), b.height(w)
	if why := world.TrackProblem(style, points, ya, yb); why != "" {
		s.setToast(why)
		return false
	}
	cost := world.TrackCost(w.PreviewTrack(style, points, ya, yb))
	if !w.CanAfford(cost) {
		s.setToast(fmt.Sprintf("Need %s for this track: short by %s",
			settings.FormatDollars(cost), settings.FormatDollars(cost-w.Available())))
		return false
	}
	ja := s.railJoint(a)
	// Splitting for the first end may have replaced the track the second
	// was on: find it again.
	if b.track != nil {
		b = resolveRailEnd(w, b.pos)
	}
	jb := s.railJoint(b)
	if ja == jb {
		s.setToast("Track has to run between two different places")
		return false
	}
	points[0], points[len(points)-1] = ja.Pos, jb.Pos
	w.Spend(cost)
	w.AddRailTrack(style, points, ja, jb, keepClaimed(w))
	s.railChanged(r)
	r.SetRailGhost(nil, nil, [3]float32{})
	s.setToast(fmt.Sprintf("%s track laid: %s", style.Spec().Label, settings.FormatDollars(cost)))
	return true
}

// railJoint is the joint for end e: its own, a junction split into its
// track, or a new one.
func (s *Scenario) railJoint(e railEnd) *world.RailJoint {
	w := s.world
	switch {
	case e.joint != nil:
		return e.joint
	case e.track != nil:
		return w.SplitTrack(e.track, e.i)
	}
	return w.AddRailJoint(w.NewRailJoint(e.pos))
}

// railChanged redraws the ground and the railway after it changes.
func (s *Scenario) railChanged(r *render.Renderer) {
	w := s.world
	w.Terrain.RestampTreeWells()
	r.FlushTerrainVerts(w.Terrain)
	r.RebuildStaticBatch(w)
}

// updateRailGhost shows the track as it would be laid with the mouse at
// the next point.
func (s *Scenario) updateRailGhost(r *render.Renderer) {
	rt := &s.railTool
	w := s.world
	if len(rt.points) == 0 || !s.hoverValid {
		if rt.ghostN != 0 {
			r.SetRailGhost(nil, nil, [3]float32{})
			rt.ghostN = 0
		}
		return
	}
	hover := resolveRailEnd(w, mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]})
	points := append([]mgl32.Vec2(nil), rt.points...)
	end := railEnd{pos: points[len(points)-1]}
	if hover.pos.Sub(end.pos).Len() > railFinishReach || hover.joint != nil || hover.track != nil {
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
	r.SetRailGhost(tr, w.Terrain, ghostTint(w.CanAfford(rt.ghostCost), world.TrackProblem(rt.style, points, ya, yb) == ""))
}

// railCost is the cost preview for the Rail tool: the track as the ghost
// last laid it out.
func (s *Scenario) railCost() int { return s.railTool.ghostCost }

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

func (s *Scenario) stationSpotAt(p mgl32.Vec2) stationSpot {
	w := s.world
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
			if o.TrackID == tr.ID && abs32f(o.Dist-sample.Dist) < 2*half {
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
func (s *Scenario) updateStationGhost(r *render.Renderer) {
	if !s.hoverValid {
		return
	}
	sp := s.stationSpotAt(mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]})
	if sp.st.TrackID == 0 {
		return
	}
	m, ok := render.StationTransform(s.world, &sp.st)
	if !ok {
		return
	}
	inst := render.StaticInstance{ColorTint: ghostTint(s.world.CanAfford(sp.cost), sp.ok)}
	copy(inst.Transform[:], m[:])
	r.SetGhosts(render.MeshRailStation, []render.StaticInstance{inst})
}

// stationClick places a station where the Station tool is pointing.
func (s *Scenario) stationClick(r *render.Renderer, p mgl32.Vec2) {
	w := s.world
	sp := s.stationSpotAt(p)
	if !sp.ok {
		s.setToast(sp.why)
		return
	}
	if !w.CanAfford(sp.cost) {
		s.setToast(fmt.Sprintf("Need %s for a station: short by %s", settings.FormatDollars(sp.cost), settings.FormatDollars(sp.cost-w.Available())))
		return
	}
	w.Spend(sp.cost)
	st := sp.st
	b := &world.Building{ID: w.NextID(), Type: world.BuildingStation}
	st.ID = b.ID
	w.Buildings = append(w.Buildings, b)
	w.RailStations = append(w.RailStations, &st)
	b.Pos = w.PlatformSpot(&st)
	w.RailRev++
	r.RebuildStaticBatch(w)
	s.setToast(fmt.Sprintf("Station built: %s", settings.FormatDollars(sp.cost)))
}

// openStationPopup shows station st: its train, its service, how often,
// and who it's bringing.
func (s *Scenario) openStationPopup(st *world.RailStation, screenW, screenH int) {
	w := s.world
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
		func() { st.Interval = max(st.Interval/2, 15) },
		func() { st.Interval = min(st.Interval*2, 240) })
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
	s.placePopup(win, screenW, screenH)
	s.popup = win
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
func (s *Scenario) railSelected() *world.RailTrack {
	e := &s.railEdit
	if e.track == 0 || e.win == nil || s.popup != e.win || !e.win.Visible {
		if !e.drag.ok {
			e.clear()
		}
		return nil
	}
	return s.world.RailTrackByID(e.track)
}

// railHandlePos is a handle's place in the world.
func (s *Scenario) railHandlePos(p mgl32.Vec2) mgl32.Vec3 {
	return mgl32.Vec3{p[0], render.VisualElevationAt(s.world.Terrain, p[0], p[1]) + 1, p[1]}
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
func (s *Scenario) railHandleAt(r *render.Renderer, mouse mgl32.Vec2) railHandle {
	tr := s.world.RailTrackByID(s.railEdit.track)
	if tr == nil {
		return railHandle{}
	}
	best, bestD := railHandle{}, float32(railHandleReach)
	try := func(h railHandle, p mgl32.Vec2) {
		sx, sy, ok := r.WorldToScreen(s.railHandlePos(p))
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
func (s *Scenario) railEditInput(r *render.Renderer, inp *engine.Input) bool {
	e := &s.railEdit
	w := s.world
	tr := w.RailTrackByID(e.track)
	if tr == nil || (s.railSelected() == nil && !e.drag.ok) {
		return false
	}
	hover := mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]}
	if e.drag.ok {
		if inp.LeftHeld {
			if s.hoverValid {
				s.railDragTo(r, tr, hover)
			}
		} else {
			s.railRelease(r, tr, hover)
		}
		return true
	}
	e.hover = s.railHandleAt(r, inp.MousePos)
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
		s.railSettle(r, []*world.RailTrack{tr})
		return true
	}
	return false
}

// railDragTo moves the dragged handle to p: a point of the track, or its
// end's joint (and every track at it).
func (s *Scenario) railDragTo(r *render.Renderer, tr *world.RailTrack, p mgl32.Vec2) {
	e := &s.railEdit
	w := s.world
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
	r.RebuildRails(w)
}

// railRelease ends a drag at p: an end dropped on another joint joins it,
// on another track makes a junction there; then the edit's settled.
func (s *Scenario) railRelease(r *render.Renderer, tr *world.RailTrack, p mgl32.Vec2) {
	e := &s.railEdit
	w := s.world
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
	s.railSettle(r, changed)
}

// railSettle checks an edit of tracks: too steep, or not paid for, and
// it's put back as it was; else charged the difference and the ground
// graded to the tracks that moved.
func (s *Scenario) railSettle(r *render.Renderer, tracks []*world.RailTrack) {
	e := &s.railEdit
	w := s.world
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
	if why == "" && extra > 0 && !w.CanAfford(extra) {
		why = fmt.Sprintf("Need %s for this change: short by %s", settings.FormatDollars(extra), settings.FormatDollars(extra-w.Available()))
	}
	if why != "" {
		w.RestoreRail(e.before)
		s.setToast(why)
		r.RebuildRails(w)
		return
	}
	if extra > 0 {
		w.Spend(extra)
	}
	keep := keepClaimed(w)
	for _, tr := range tracks {
		w.GradeTrack(tr, keep)
	}
	w.PlaceStations()
	s.railChanged(r)
}

// trackAt is the track whose line passes within reach of p, for
// selecting.
func (s *Scenario) trackAt(p mgl32.Vec2) *world.RailTrack {
	tr, _, _ := s.world.TrackNear(p, world.RailTrackReach)
	return tr
}

// openTrackPopup selects track tr and shows its popup: its style, speed,
// sign, and the upgrade to the next style.
func (s *Scenario) openTrackPopup(tr *world.RailTrack, screenW, screenH int) {
	w := s.world
	s.railEdit = railEdit{track: tr.ID}
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
	if tr.Style+1 < world.RailStyles {
		next := tr.Style + 1
		cost := world.UpgradeCost(tr, next)
		win.AddActionButton(fmt.Sprintf("Upgrade to %s (%s)", lowerFirstWord(next.Spec().Label), settings.FormatDollars(cost)), func() {
			s.upgradeTrack(tr, next, screenW, screenH)
		})
	}
	win.Visible = true
	s.placePopup(win, screenW, screenH)
	s.popup = win
	s.railEdit.win = win
}

// upgradeTrack upgrades track tr to style, if it can be.
func (s *Scenario) upgradeTrack(tr *world.RailTrack, style world.RailStyle, screenW, screenH int) {
	w := s.world
	a, b := w.RailJointByID(tr.Ends[0]), w.RailJointByID(tr.Ends[1])
	if a == nil || b == nil {
		return
	}
	if why := world.TrackProblem(style, tr.Points, a.Y, b.Y); why != "" {
		s.setToast(why)
		return
	}
	cost := world.UpgradeCost(tr, style)
	if !w.CanAfford(cost) {
		s.setToast(fmt.Sprintf("Need %s to upgrade: short by %s", settings.FormatDollars(cost), settings.FormatDollars(cost-w.Available())))
		return
	}
	w.Spend(cost)
	tr.Style = style
	w.LayOutTrack(tr)
	w.GradeTrack(tr, keepClaimed(w))
	s.railChanged(s.app.Renderer)
	s.setToast(fmt.Sprintf("Upgraded to %s: %s", lowerFirstWord(style.Spec().Label), settings.FormatDollars(cost)))
	s.openTrackPopup(tr, screenW, screenH)
}

// drawRailHandles draws the selected track's points and the dots between
// them, the one under the cursor or dragged highlighted.
func (s *Scenario) drawRailHandles(r *render.Renderer) {
	e := &s.railEdit
	tr := s.railSelected()
	if tr == nil || s.activeTool != toolNone {
		return
	}
	active := e.hover
	if e.drag.ok {
		active = e.drag
	}
	dot := func(h railHandle, p mgl32.Vec2, radius float32) {
		sx, sy, ok := r.WorldToScreen(s.railHandlePos(p))
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
func (s *Scenario) drawRailMarks(r *render.Renderer) {
	w := s.world
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
	if s.activeTool == toolRail {
		for _, j := range w.RailJoints {
			if sx, sy, ok := r.WorldToScreen(s.railHandlePos(j.Pos)); ok {
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
