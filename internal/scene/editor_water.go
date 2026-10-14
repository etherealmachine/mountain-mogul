//go:build !demo

package scene

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/geo"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// The scenario editor's Water tool (notes/next/Water Editing.md): the
// Water layer's creeks and lakes (World.Water, fitted from the import's
// OpenStreetMap water; the Layers panel's Water row refits it), drawn
// over the ground while the tool is on. Click a creek or lake to
// select it; its points show handles. Drag a point to move it, or the
// dot between two points to add one there; right-click a point to
// remove it; a creek's end dragged near another creek snaps onto it
// (joinCreeks: a mouth onto a source makes them one creek); Delete removes the whole creek or lake, Esc lets go. When a
// drag ends, or a point is added or removed, the creek's channel is
// re-carved or the lake re-levelled, and the water, ice, snow and trees
// round it redone (rebuildWater).

const toolWater = toolMode(112)

// waterPickReach is how near a creek or lake's line a click selects it,
// in metres.
const waterPickReach = 12

// waterSel is the selected creek (lake false) or lake, by index; ok
// false for none.
type waterSel struct {
	lake bool
	i    int
	ok   bool
}

// creekJoinReach is how near, in metres, a dragged creek end snaps onto
// another creek's line when no point of it is within a handle's reach.
const creekJoinReach = 5

// creekJoin is another creek a dragged creek end has snapped onto: at
// its point i, or on its line between points when i is -1.
type creekJoin struct {
	creek, i int
	ok       bool
}

// waterHandle is a grabbable point of the selected creek or lake: point
// i of ring (always 0 for a creek), or, with mid, the midpoint of the
// segment from point i to the next.
type waterHandle struct {
	ring, i int
	mid, ok bool
}

// waterTool is the Water tool's session.
type waterTool struct {
	sel         waterSel
	hover, drag waterHandle
	dragged     bool       // the drag moved its point, or added one
	join        creekJoin  // where the creek end being dragged has snapped to
	rightDown   mgl32.Vec2 // where the right button went down, to tell a click from a pan
	// The water before the edit in hand (waterSnapshot).
	old *world.Water
	// version counts changes to the lines and selection; drawn is the
	// version last drawn (0: not yet).
	version, drawn int
	// ready is false until the layer stack has run once (startLayerJob
	// warm), seeding the water; until then the tool does nothing.
	ready bool
	// coverBtns set the selected creek or lake's winter cover, by
	// world.WaterCover; made on first use.
	coverBtns [world.WaterCoverCount]*ui.Button
	panel     [4]float32 // the cover panel as last laid out: x, y, w, h
}

// The cover panel, bottom left over the toolbar: the selection's name and
// a button for each world.WaterCover.
const (
	coverPanelW = float32(330)
	coverPanelH = float32(86)
)

// layoutCoverPanel places the cover panel and its buttons.
func (e *Editor) layoutCoverPanel(r *render.Renderer) {
	st := &e.water
	x := float32(12)
	y := float32(r.ScreenHeight()) - e.menuBar.H - coverPanelH - 12
	st.panel = [4]float32{x, y, coverPanelW, coverPanelH}
	for c := range world.WaterCoverCount {
		if st.coverBtns[c] == nil {
			c := c
			st.coverBtns[c] = ui.NewButton(0, 0, 96, 32, c.Label(), func() { e.setWaterCover(c) })
		}
		b := st.coverBtns[c]
		b.X, b.Y, b.W, b.H = x+10+float32(c)*(96+10), y+coverPanelH-32-10, 96, 32
		b.SetActive(e.waterCover() == c)
	}
}

// inCoverPanel reports whether screen point (mx, my) is on the panel.
func (e *Editor) inCoverPanel(mx, my float32) bool {
	p := e.water.panel
	return e.water.sel.ok && mx >= p[0] && mx <= p[0]+p[2] && my >= p[1] && my <= p[1]+p[3]
}

// waterCover is the selected creek or lake's cover.
func (e *Editor) waterCover() world.WaterCover {
	sel := e.water.sel
	switch {
	case !sel.ok:
		return world.WaterAuto
	case sel.lake:
		return e.world.Water.Lakes[sel.i].Cover
	}
	return e.world.Water.Creeks[sel.i].Cover
}

// setWaterCover sets the selected creek or lake's cover and rebuilds its
// water, ice and snow.
func (e *Editor) setWaterCover(c world.WaterCover) {
	sel := e.water.sel
	if !sel.ok || !e.waterUsable() || e.waterCover() == c {
		return
	}
	e.waterSnapshot()
	if sel.lake {
		e.world.Water.Lakes[sel.i].Cover = c
	} else {
		e.world.Water.Creeks[sel.i].Cover = c
	}
	e.rebuildEditedWater()
	e.setToast(e.waterSummary(sel) + ": " + coverText(c, sel.lake))
}

func coverText(c world.WaterCover, lake bool) string {
	switch {
	case c == world.WaterOpen:
		return "open water all winter, no snow on it."
	case c == world.WaterCovered && lake:
		return "frozen and snowed over all winter."
	case c == world.WaterCovered:
		return "snow-bridged all winter."
	case lake:
		return "freezes with the cold, from its shallows out."
	}
	return "open where it's big enough, with snow bridges between."
}

// drawCoverPanel draws the cover panel for the selected creek or lake.
func (e *Editor) drawCoverPanel(r *render.Renderer) {
	st := &e.water
	e.layoutCoverPanel(r)
	p := st.panel
	r.DrawColorRect(p[0], p[1], p[2], p[3], mgl32.Vec4{0.08, 0.10, 0.14, 0.95})
	if r.Font != nil {
		r.Font.DrawText(r, "Winter: "+e.waterSummary(st.sel), p[0]+10, p[1]+8, mgl32.Vec4{0.92, 0.95, 1, 1})
	}
	for _, b := range st.coverBtns {
		b.Draw(r)
	}
}

// activateWaterTool turns the Water tool on (or off). The water has to
// be on the base first: the Creeks and Lakes layers keep it on their
// first run, so if they haven't, the ground's layers run once now.
func (e *Editor) activateWaterTool() {
	e.setTool(toolWater)
	if e.activeTool != toolWater {
		e.endWaterTool()
		return
	}
	base := e.world.TerrainBase
	if base == nil || !base.Detail {
		e.setTool(toolWater)
		e.setToast("Water needs an imported map with lidar.")
		return
	}
	if e.layers.job != nil {
		e.setTool(toolWater)
		e.setToast("Wait for the terrain layers to finish rebuilding.")
		return
	}
	// The stack runs once in the background (the Layers panel shows its
	// progress), seeding the water if it isn't yet; edits then rerun only
	// the Creeks and Lakes layers.
	e.water = waterTool{version: 1}
	e.waterStack()
	e.startLayerJob(true)
	e.setToast("Reading the ground for its water...")
}

// waterUsable reports whether the tool can edit: its stack has run, and
// no layer rebuild is using it.
func (e *Editor) waterUsable() bool {
	return e.water.ready && e.layers.job == nil
}

// waterWarmed readies the tool once its layer stack has run.
func (e *Editor) waterWarmed() {
	if e.activeTool != toolWater {
		return
	}
	e.water.ready = true
	e.water.version++
	e.setToast(fmt.Sprintf("%d creeks, %d lakes. Click one to select it; drag a point to move it, or the dot between two to add one; right-click a point to remove it; Delete removes the creek or lake.",
		len(e.world.Water.Creeks), len(e.world.Water.Lakes)))
}

// waterStack is the layer stack water edits rebuild with: the Layers
// panel's, so a rebuild from either reuses the other's kept layers.
func (e *Editor) waterStack() *geo.LayerStack {
	return e.layerStack()
}

// endWaterTool clears the tool's drawing.
func (e *Editor) endWaterTool() {
	e.water = waterTool{}
	if e.app != nil && e.app.Renderer != nil {
		e.app.Renderer.SetTrailLayer(render.TrailLayerWater, nil, nil)
	}
}

// waterPoints is ring ring of the selected creek or lake: the creek's
// line, or a lake's ring; nil when there's none.
func (e *Editor) waterPoints(sel waterSel, ring int) [][2]float32 {
	switch {
	case !sel.ok:
		return nil
	case sel.lake && sel.i < len(e.world.Water.Lakes) && ring < len(e.world.Water.Lakes[sel.i].Rings):
		return e.world.Water.Lakes[sel.i].Rings[ring]
	case !sel.lake && sel.i < len(e.world.Water.Creeks) && ring == 0:
		return e.world.Water.Creeks[sel.i].Points
	}
	return nil
}

// waterRings is the selected creek or lake's point lists, and whether
// they close (a lake's rings).
func (e *Editor) waterRings(sel waterSel) ([][][2]float32, bool) {
	switch {
	case !sel.ok:
		return nil, false
	case sel.lake && sel.i < len(e.world.Water.Lakes):
		return e.world.Water.Lakes[sel.i].Rings, true
	case !sel.lake && sel.i < len(e.world.Water.Creeks):
		return [][][2]float32{e.world.Water.Creeks[sel.i].Points}, false
	}
	return nil, false
}

// waterHandleAt finds the selected water's handle under screen point
// mouse, points before midpoints.
func (e *Editor) waterHandleAt(r *render.Renderer, mouse mgl32.Vec2) waterHandle {
	rings, closed := e.waterRings(e.water.sel)
	best, bestD := waterHandle{}, float32(skiHandleReach)
	try := func(h waterHandle, p [2]float32) {
		sx, sy, ok := r.WorldToScreen(e.waterHandlePos(p))
		if d := (mgl32.Vec2{sx, sy}).Sub(mouse).Len(); ok && d < bestD {
			best, bestD = h, d
			best.ok = true
		}
	}
	for ri, ps := range rings {
		for i, p := range ps {
			try(waterHandle{ring: ri, i: i}, p)
		}
	}
	if best.ok {
		return best
	}
	for ri, ps := range rings {
		for i := range ps {
			if !closed && i == len(ps)-1 {
				break
			}
			try(waterHandle{ring: ri, i: i, mid: true}, waterMid(ps, i, closed))
		}
	}
	return best
}

// waterMid is the handle for adding a point between points i and i+1: on
// a creek's curve (world.WaterCreek.Curve), or midway along a lake's edge.
func waterMid(ps [][2]float32, i int, closed bool) [2]float32 {
	if !closed {
		return world.CurveAt(ps, i, 0.5)
	}
	q := ps[(i+1)%len(ps)]
	return [2]float32{(ps[i][0] + q[0]) / 2, (ps[i][1] + q[1]) / 2}
}

func (e *Editor) waterHandlePos(p [2]float32) mgl32.Vec3 {
	return mgl32.Vec3{p[0], render.VisualElevationAt(e.world.Terrain, p[0], p[1]), p[1]}
}

// waterAt is the creek or lake nearest world point p within
// waterPickReach of its line, or a lake p lies inside.
func (e *Editor) waterAt(p [2]float32) waterSel {
	best, bestD := waterSel{}, float32(waterPickReach)
	for i, c := range e.world.Water.Creeks {
		if d := lineDist(c.Curve(), p, false); d < bestD {
			best, bestD = waterSel{i: i, ok: true}, d
		}
	}
	for i, l := range e.world.Water.Lakes {
		for _, r := range l.Rings {
			if d := lineDist(r, p, true); d < bestD {
				best, bestD = waterSel{lake: true, i: i, ok: true}, d
			}
		}
	}
	if best.ok {
		return best
	}
	for i, l := range e.world.Water.Lakes {
		if geo.ShapeMask(1, 1, 1, []world.WaterLake{shiftedLake(l, p)})[0] != 0 {
			return waterSel{lake: true, i: i, ok: true}
		}
	}
	return waterSel{}
}

// shiftedLake is l moved so world point p is at the origin, to test p
// against it with a one-sample ShapeMask.
func shiftedLake(l world.WaterLake, p [2]float32) world.WaterLake {
	out := world.WaterLake{}
	for _, r := range l.Rings {
		var rr [][2]float32
		for _, q := range r {
			rr = append(rr, [2]float32{q[0] - p[0], q[1] - p[1]})
		}
		out.Rings = append(out.Rings, rr)
	}
	return out
}

// lineDist is how far p is from line ps (a closed ring when closed).
func lineDist(ps [][2]float32, p [2]float32, closed bool) float32 {
	best := float32(math.Inf(1))
	n := len(ps)
	for i := 0; i < n; i++ {
		if !closed && i == n-1 {
			break
		}
		a, b := ps[i], ps[(i+1)%n]
		dx, dz := b[0]-a[0], b[1]-a[1]
		u := float32(0)
		if l2 := dx*dx + dz*dz; l2 > 0 {
			u = min(max(((p[0]-a[0])*dx+(p[1]-a[1])*dz)/l2, 0), 1)
		}
		qx, qz := a[0]+u*dx-p[0], a[1]+u*dz-p[1]
		best = min(best, float32(math.Hypot(float64(qx), float64(qz))))
	}
	return best
}

// waterSnapshot keeps the water as it is before an edit, for the rebuild
// to compare against.
func (e *Editor) waterSnapshot() {
	e.water.old = e.world.Water.Clone()
}

// waterMouse handles the left button: dragging handles, else selecting.
func (e *Editor) waterMouse(r *render.Renderer, inp *engine.Input) {
	st := &e.water
	if !e.waterUsable() {
		return
	}
	if st.sel.ok {
		e.layoutCoverPanel(r)
		mx, my := inp.MousePos[0], inp.MousePos[1]
		for _, b := range st.coverBtns {
			b.SetHovered(b.Contains(mx, my))
		}
		if e.inCoverPanel(mx, my) {
			st.hover = waterHandle{}
			if inp.LeftClick {
				for _, b := range st.coverBtns {
					if b.Contains(mx, my) {
						b.Click()
					}
				}
			}
			return
		}
	}
	pos := [2]float32{e.hoverWorld[0], e.hoverWorld[2]}
	st.hover = e.waterHandleAt(r, inp.MousePos)
	switch {
	case inp.LeftClick && st.hover.ok:
		e.waterSnapshot()
		h := st.hover
		if h.mid {
			ps := e.waterPoints(st.sel, h.ring)
			mid := waterMid(ps, h.i, st.sel.lake)
			e.setWaterPoints(st.sel, h.ring, insertPoint(ps, h.i+1, mid))
			h = waterHandle{ring: h.ring, i: h.i + 1, ok: true}
			st.dragged = true // a new point rebuilds even if it isn't moved
		}
		st.drag = h
	case inp.LeftClick && e.hoverValid:
		st.sel = e.waterAt(pos)
		st.version++
		if st.sel.ok {
			e.setToast(e.waterSummary(st.sel) + ". Drag a point to move it, or the dot between two to add one; drag a creek's end onto another creek to join them; right-click a point to remove it; Delete removes it; Esc lets go.")
		}
	case inp.LeftHeld && st.drag.ok && e.hoverValid:
		ps := e.waterPoints(st.sel, st.drag.ring)
		st.join = creekJoin{}
		if !st.sel.lake && (st.drag.i == 0 || st.drag.i == len(ps)-1) {
			if j, p := e.creekSnap(r, inp.MousePos, st.sel.i); j.ok {
				st.join, pos = j, p
			}
		}
		if st.drag.i < len(ps) && ps[st.drag.i] != pos {
			ps[st.drag.i] = pos
			st.dragged = true
			st.version++
		}
	}
}

// waterRelease ends a handle drag, rebuilding the water if it moved.
func (e *Editor) waterRelease() {
	st := &e.water
	if !st.drag.ok {
		return
	}
	end := st.drag.i
	st.drag = waterHandle{}
	if st.join.ok {
		e.joinCreeks(end, st.join)
		st.join = creekJoin{}
	}
	if st.dragged {
		st.dragged = false
		e.rebuildEditedWater()
	}
}

// creekSnap is where a dragged end of creek self would join another
// creek, the cursor at screen point mouse: the nearest point of another
// creek within a handle's reach on screen, else the nearest place on
// another creek's curve within creekJoinReach of the cursor's ground.
func (e *Editor) creekSnap(r *render.Renderer, mouse mgl32.Vec2, self int) (creekJoin, [2]float32) {
	best, bestD, at := creekJoin{}, float32(skiHandleReach), [2]float32{}
	for ci, c := range e.world.Water.Creeks {
		if ci == self {
			continue
		}
		for i, p := range c.Points {
			sx, sy, ok := r.WorldToScreen(e.waterHandlePos(p))
			if d := (mgl32.Vec2{sx, sy}).Sub(mouse).Len(); ok && d < bestD {
				best, bestD, at = creekJoin{creek: ci, i: i, ok: true}, d, p
			}
		}
	}
	if best.ok || !e.hoverValid {
		return best, at
	}
	pos := [2]float32{e.hoverWorld[0], e.hoverWorld[2]}
	lineD := float32(creekJoinReach)
	for ci, c := range e.world.Water.Creeks {
		if ci == self {
			continue
		}
		course := c.Curve()
		for k := 1; k < len(course); k++ {
			q := nearestOn(course[k-1], course[k], pos)
			if d := float32(math.Hypot(float64(q[0]-pos[0]), float64(q[1]-pos[1]))); d < lineD {
				best, lineD, at = creekJoin{creek: ci, i: -1, ok: true}, d, q
			}
		}
	}
	return best, at
}

func nearestOn(a, b, p [2]float32) [2]float32 {
	dx, dz := b[0]-a[0], b[1]-a[1]
	u := float32(0)
	if l2 := dx*dx + dz*dz; l2 > 0 {
		u = min(max(((p[0]-a[0])*dx+(p[1]-a[1])*dz)/l2, 0), 1)
	}
	return [2]float32{a[0] + u*dx, a[1] + u*dz}
}

// joinCreeks finishes a drag of the selected creek's point end that
// snapped onto j. A mouth dropped on another creek's source, or a source
// on another's mouth, makes the two one creek (the selected one's name
// and cover, else the other's name); anywhere else the end stays where it
// snapped, a junction.
func (e *Editor) joinCreeks(end int, j creekJoin) {
	st := &e.water
	cs := e.world.Water.Creeks
	a, b := cs[st.sel.i], cs[j.creek]
	last := len(a.Points) - 1
	var merged [][2]float32
	switch {
	case end == last && j.i == 0:
		merged = append(append([][2]float32(nil), a.Points...), b.Points[1:]...)
	case end == 0 && j.i == len(b.Points)-1:
		merged = append(append([][2]float32(nil), b.Points...), a.Points[1:]...)
	default:
		name := "a creek"
		if b.Name != "" {
			name = b.Name
		}
		e.setToast("Joined " + name + ".")
		return
	}
	a.Points = merged
	if a.Name == "" {
		a.Name, a.Kind = b.Name, b.Kind
	}
	cs[st.sel.i] = a
	e.world.Water.Creeks = append(cs[:j.creek:j.creek], cs[j.creek+1:]...)
	if j.creek < st.sel.i {
		st.sel.i--
	}
	st.version++
	st.dragged = true
	e.setToast("Made one creek of the two: " + e.waterSummary(st.sel) + ".")
}

// waterRightClick removes the selected water's point under the cursor.
func (e *Editor) waterRightClick(r *render.Renderer, mouse mgl32.Vec2) {
	st := &e.water
	if !e.waterUsable() {
		return
	}
	h := e.waterHandleAt(r, mouse)
	if !h.ok || h.mid {
		return
	}
	ps := e.waterPoints(st.sel, h.ring)
	least := 2
	if st.sel.lake {
		least = 3
	}
	if len(ps) <= least {
		e.setToast(fmt.Sprintf("That needs at least %d points: Delete removes the whole thing.", least))
		return
	}
	e.waterSnapshot()
	e.setWaterPoints(st.sel, h.ring, append(append([][2]float32(nil), ps[:h.i]...), ps[h.i+1:]...))
	e.rebuildEditedWater()
}

// deleteSelectedWater removes the selected creek or lake.
func (e *Editor) deleteSelectedWater() {
	st := &e.water
	base := e.world.TerrainBase
	if !e.waterUsable() || !st.sel.ok || base == nil {
		return
	}
	e.waterSnapshot()
	name := e.waterSummary(st.sel)
	if st.sel.lake {
		e.world.Water.Lakes = append(e.world.Water.Lakes[:st.sel.i:st.sel.i], e.world.Water.Lakes[st.sel.i+1:]...)
	} else {
		e.world.Water.Creeks = append(e.world.Water.Creeks[:st.sel.i:st.sel.i], e.world.Water.Creeks[st.sel.i+1:]...)
	}
	st.sel = waterSel{}
	e.rebuildEditedWater()
	e.setToast("Removed " + name + ".")
}

func (e *Editor) setWaterPoints(sel waterSel, ring int, ps [][2]float32) {
	if sel.lake {
		e.world.Water.Lakes[sel.i].Rings[ring] = ps
	} else {
		e.world.Water.Creeks[sel.i].Points = ps
	}
	e.water.version++
}

func insertPoint(ps [][2]float32, at int, p [2]float32) [][2]float32 {
	out := make([][2]float32, 0, len(ps)+1)
	out = append(out, ps[:at]...)
	out = append(out, p)
	return append(out, ps[at:]...)
}

func (e *Editor) waterSummary(sel waterSel) string {
	if sel.lake {
		l := e.world.Water.Lakes[sel.i]
		if l.Name != "" {
			return l.Name
		}
		return "a lake"
	}
	c := e.world.Water.Creeks[sel.i]
	if c.Name != "" {
		return c.Name
	}
	return "a creek"
}

// rebuildEditedWater rebuilds the ground and water for the change since
// waterSnapshot, and redraws.
func (e *Editor) rebuildEditedWater() {
	if err := rebuildWater(e.world, e.waterStack(), &e.layerCache, e.water.old, "creeks"); err != nil {
		e.setToast("Water rebuild failed: " + err.Error())
		return
	}
	e.water.old = nil
	e.waterRebuilt()
}

// requestWaterRefit asks before refitting the Water layer from
// OpenStreetMap, which loses the changes made to it.
func (e *Editor) requestWaterRefit() {
	if e.world.TerrainBase == nil || e.layers.job != nil {
		return
	}
	e.confirmPrompt = newConfirmPrompt(
		"Fit the water to OpenStreetMap's again? Changes made to creeks and lakes with the Water tool are lost.",
		"Refit", func() {
			e.confirmPrompt = nil
			e.refitWater()
		}, func() { e.confirmPrompt = nil })
}

// refitWater fits the Water layer from OpenStreetMap afresh and rebuilds
// the ground and water where that changes them.
func (e *Editor) refitWater() {
	w := e.world
	if w.Water == nil {
		w.Water = &world.Water{}
	}
	old := w.Water.Clone()
	*w.Water = world.Water{}
	if err := rebuildWater(w, e.waterStack(), &e.layerCache, old, "water"); err != nil {
		*w.Water = *old
		e.setToast("Water refit failed: " + err.Error())
		return
	}
	e.water.sel = waterSel{}
	e.waterRebuilt()
	e.setToast(fmt.Sprintf("Water fitted to OpenStreetMap: %d creeks, %d lakes.", len(w.Water.Creeks), len(w.Water.Lakes)))
}

// waterRebuilt redraws after the water's ground was rebuilt.
func (e *Editor) waterRebuilt() {
	w := e.world
	e.water.version++
	e.markDirty()
	r := e.app.Renderer
	r.BuildTerrainMesh(w.Terrain)
	r.BuildSnowSurfaceTex(w.Terrain)
	r.RebuildStaticBatch(w)
	r.RebuildRoads(w) // and the bridges, over the water as it now is
}

// updateWaterTool runs the tool's right button and redraws its lines.
func (e *Editor) updateWaterTool(r *render.Renderer, inp *engine.Input, covered bool) {
	if e.activeTool != toolWater || !e.waterUsable() {
		return
	}
	st := &e.water
	if inp.RightClick {
		st.rightDown = inp.MousePos
	}
	if inp.RightRelease && inp.MousePos.Sub(st.rightDown).Len() < 4 && !covered {
		e.waterRightClick(r, inp.MousePos)
	}
	if st.drawn == st.version {
		return
	}
	st.drawn = st.version
	d := &trailDraper{w: e.world, t: e.world.Terrain}
	creek := mgl32.Vec3{0.25, 0.55, 1}
	lake := mgl32.Vec3{0.2, 0.85, 0.95}
	selected := mgl32.Vec3{1, 0.85, 0.2}
	line := func(ps [][2]float32, closed bool, col mgl32.Vec3, w float32) {
		n := len(ps)
		for i := 0; i < n; i++ {
			if !closed && i == n-1 {
				break
			}
			a, b := ps[i], ps[(i+1)%n]
			d.segment(mgl32.Vec2{a[0], a[1]}, mgl32.Vec2{b[0], b[1]}, w, col)
		}
	}
	for i, c := range e.world.Water.Creeks {
		col, wd := creek, float32(1.2)
		if st.sel.ok && !st.sel.lake && st.sel.i == i {
			col, wd = selected, 1.6
		}
		line(c.Curve(), false, col, wd)
	}
	for i, l := range e.world.Water.Lakes {
		col, wd := lake, float32(1.2)
		if st.sel.ok && st.sel.lake && st.sel.i == i {
			col, wd = selected, 1.6
		}
		for _, ring := range l.Rings {
			line(ring, true, col, wd)
		}
	}
	r.SetTrailLayer(render.TrailLayerWater, d.fill, d.lines)
}

// drawWaterHandles draws the selected creek or lake's points, and the
// dots between them smaller; the one under the cursor or being dragged
// is highlighted.
func (e *Editor) drawWaterHandles(r *render.Renderer) {
	st := &e.water
	rings, closed := e.waterRings(st.sel)
	active := st.hover
	if st.drag.ok {
		active = st.drag
	}
	dot := func(h waterHandle, p [2]float32, radius float32) {
		sx, sy, ok := r.WorldToScreen(e.waterHandlePos(p))
		if !ok {
			return
		}
		fill := mgl32.Vec4{0.25, 0.6, 1, 1}
		if h.mid {
			fill = mgl32.Vec4{0.95, 0.95, 0.95, 0.7}
		}
		if active.ok && h.ring == active.ring && h.i == active.i && h.mid == active.mid {
			fill, radius = mgl32.Vec4{1, 0.9, 0.47, 1}, radius+2
		}
		r.DrawColorDisc(sx, sy, radius+1.5, mgl32.Vec4{0.05, 0.1, 0.15, 0.9})
		r.DrawColorDisc(sx, sy, radius, fill)
	}
	for ri, ps := range rings {
		for i := range ps {
			if !closed && i == len(ps)-1 {
				break
			}
			dot(waterHandle{ring: ri, i: i, mid: true}, waterMid(ps, i, closed), 3)
		}
		for i, p := range ps {
			dot(waterHandle{ring: ri, i: i}, p, 5)
		}
	}
	// A dragged creek end snapped onto another creek: a ring round it.
	if st.drag.ok && st.join.ok {
		if ps := e.waterPoints(st.sel, st.drag.ring); st.drag.i < len(ps) {
			if sx, sy, ok := r.WorldToScreen(e.waterHandlePos(ps[st.drag.i])); ok {
				r.DrawColorDisc(sx, sy, 13, mgl32.Vec4{0.3, 0.95, 0.6, 0.45})
				dot(st.drag, ps[st.drag.i], 5)
			}
		}
	}
}
