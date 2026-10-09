package scene

import (
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/world"
)

// The scenario editor's ski-area boundary tool. Click to place corners;
// click the first corner (or press Enter) to close the outline; right-
// click to take back the last corner. With no outline in progress,
// existing outlines show handles: drag a corner to move it, drag an
// edge's midpoint to add a corner there, right-click a corner to remove
// it, or right-click inside an outline to delete it. Clicking inside an
// outline selects it (Delete removes it, Esc lets go); Shift-click places
// a corner there instead. O copies
// OpenStreetMap's ski-area boundary, where the import has one, to draw
// over or keep. The ground inside is the resort's skiable terrain
// (world.SkiArea).

const toolSkiArea = toolMode(108)

// skiAreaCloseDist is how near the first corner a click closes the
// outline, in metres.
const skiAreaCloseDist = 10

// skiHandleReach is how near a handle the cursor grabs it, in screen
// pixels.
const skiHandleReach = 9

// skiHandle is a grabbable point of an existing outline: corner i, or,
// with mid, the midpoint of the edge from corner i to the next.
type skiHandle struct {
	outline, corner int
	mid, ok         bool
}

// skiHandleAt finds the handle under screen point mouse, corners first.
func (e *Editor) skiHandleAt(r *render.Renderer, mouse mgl32.Vec2) skiHandle {
	best, bestD := skiHandle{}, float32(skiHandleReach)
	try := func(h skiHandle, p mgl32.Vec2) {
		sx, sy, ok := r.WorldToScreen(e.skiHandlePos(p))
		if d := (mgl32.Vec2{sx, sy}).Sub(mouse).Len(); ok && d < bestD {
			best, bestD = h, d
			best.ok = true
		}
	}
	for o, ol := range e.world.SkiArea {
		for i, p := range ol.Points {
			try(skiHandle{outline: o, corner: i}, p)
		}
	}
	if best.ok {
		return best
	}
	for o, ol := range e.world.SkiArea {
		for i, p := range ol.Points {
			q := ol.Points[(i+1)%len(ol.Points)]
			try(skiHandle{outline: o, corner: i, mid: true}, p.Add(q).Mul(0.5))
		}
	}
	return best
}

// skiHandlePos is where a handle for ground point p is drawn.
func (e *Editor) skiHandlePos(p mgl32.Vec2) mgl32.Vec3 {
	return mgl32.Vec3{p[0], e.world.Terrain.InterpolatedSurfaceElevationAt(p[0], p[1]), p[1]}
}

// skiAreaMouse handles the left button: dragging handles, else placing
// corners of a new outline.
func (e *Editor) skiAreaMouse(r *render.Renderer, inp *engine.Input) {
	pos := mgl32.Vec2{e.hoverWorld[0], e.hoverWorld[2]}
	if len(e.skiDraft) == 0 {
		e.skiHover = e.skiHandleAt(r, inp.MousePos)
	} else {
		e.skiHover = skiHandle{}
	}
	switch {
	case inp.LeftClick && e.skiHover.ok:
		h := e.skiHover
		if h.mid {
			ol := e.world.SkiArea[h.outline].Points
			mid := ol[h.corner].Add(ol[(h.corner+1)%len(ol)]).Mul(0.5)
			e.world.InsertSkiAreaPoint(h.outline, h.corner+1, mid)
			h = skiHandle{outline: h.outline, corner: h.corner + 1, ok: true}
			e.markDirty()
		}
		e.skiDrag = h
		e.skiSel = h.outline
	case inp.LeftClick && e.hoverValid:
		shift := inp.Held[glfw.KeyLeftShift] || inp.Held[glfw.KeyRightShift]
		if i := e.world.SkiAreaAt(pos[0], pos[1]); i >= 0 && len(e.skiDraft) == 0 && !shift {
			e.skiSel = i
			e.setToast("Outline selected: Delete removes it, Esc lets go. Shift-click to draw inside it.")
			return
		}
		e.skiSel = -1
		e.skiAreaClick(pos)
	case inp.LeftHeld && e.skiDrag.ok && e.hoverValid:
		e.world.MoveSkiAreaPoint(e.skiDrag.outline, e.skiDrag.corner, pos)
		e.markDirty()
	}
}

// removeSkiOutline deletes outline i, keeping the selection on the same
// outline when it's another.
func (e *Editor) removeSkiOutline(i int) {
	e.world.RemoveSkiArea(i)
	switch {
	case e.skiSel == i:
		e.skiSel = -1
	case e.skiSel > i:
		e.skiSel--
	}
	e.markDirty()
	e.setToast("Removed that part of the ski area. " + e.skiAreaSummary())
}

// deleteSelectedSkiArea removes the selected outline, if any.
func (e *Editor) deleteSelectedSkiArea() bool {
	if e.skiSel < 0 || e.skiSel >= len(e.world.SkiArea) {
		return false
	}
	e.removeSkiOutline(e.skiSel)
	return true
}

// skiAreaRelease ends a handle drag.
func (e *Editor) skiAreaRelease() {
	if e.skiDrag.ok {
		e.skiDrag = skiHandle{}
		e.setToast(e.skiAreaSummary())
	}
}

func (e *Editor) activateSkiAreaTool() {
	e.skiDraft = nil
	e.setTool(toolSkiArea)
	if e.activeTool != toolSkiArea {
		return
	}
	e.skiSel = -1
	msg := "Click to place corners; click the first corner or press Enter to close. Click inside an outline to select it (Delete removes it). Drag a corner to move it, or an edge's midpoint to add one. Right-click: undo or remove a corner."
	if e.world.TerrainBase != nil && len(e.world.TerrainBase.Areas) > 0 {
		msg += " O: copy OpenStreetMap's boundary."
	}
	e.setToast(msg)
}

// skiAreaClick places a corner at pos, or closes the outline when pos is
// on its first corner.
func (e *Editor) skiAreaClick(pos mgl32.Vec2) {
	if n := len(e.skiDraft); n >= 3 && pos.Sub(e.skiDraft[0]).Len() <= skiAreaCloseDist {
		e.closeSkiArea()
		return
	}
	e.skiDraft = append(e.skiDraft, pos)
}

// skiAreaRightClick takes back the last corner; with no outline in
// progress, it removes the corner under the cursor, or else deletes the
// outline under pos.
func (e *Editor) skiAreaRightClick(r *render.Renderer, mouse, pos mgl32.Vec2) {
	if n := len(e.skiDraft); n > 0 {
		e.skiDraft = e.skiDraft[:n-1]
		return
	}
	if h := e.skiHandleAt(r, mouse); h.ok && !h.mid {
		if e.world.RemoveSkiAreaPoint(h.outline, h.corner) {
			e.markDirty()
			e.setToast(e.skiAreaSummary())
		} else {
			e.setToast("An outline needs three corners: right-click inside it to delete it.")
		}
		return
	}
	if i := e.world.SkiAreaAt(pos[0], pos[1]); i >= 0 {
		e.removeSkiOutline(i)
		return
	}
}

// closeSkiArea adds the outline in progress to the boundary.
func (e *Editor) closeSkiArea() {
	if len(e.skiDraft) < 3 {
		e.setToast("An outline needs at least three corners.")
		return
	}
	e.world.AddSkiArea(e.skiDraft)
	e.skiDraft = nil
	e.markDirty()
	e.setToast(e.skiAreaSummary())
}

// copyOSMSkiArea adds OpenStreetMap's ski-area outlines from the import.
func (e *Editor) copyOSMSkiArea() {
	base := e.world.TerrainBase
	if base == nil || len(base.Areas) == 0 {
		e.setToast("This map's import has no OpenStreetMap ski-area boundary.")
		return
	}
	t := e.world.Terrain
	d := &osmDraper{t: t, b: base.Geo, maxX: float32(t.Width-1) * world.CellSize, maxZ: float32(t.Height-1) * world.CellSize}
	added := 0
	for _, a := range base.Areas {
		for _, path := range a.Paths {
			var pts []mgl32.Vec2
			for _, p := range path {
				x, z, _ := d.toWorld(p)
				x = min(max(x, 0), d.maxX)
				z = min(max(z, 0), d.maxZ)
				pts = append(pts, mgl32.Vec2{x, z})
			}
			// OpenStreetMap closes a ring by repeating its first point.
			if n := len(pts); n > 1 && pts[0] == pts[n-1] {
				pts = pts[:n-1]
			}
			if len(pts) >= 3 {
				e.world.AddSkiArea(pts)
				added++
			}
		}
	}
	if added > 0 {
		e.markDirty()
	}
	e.setToast(e.skiAreaSummary())
}

func (e *Editor) skiAreaSummary() string {
	return "Ski area: " + settings.FormatArea(e.world.SkiAreaCells())
}

// drawSkiAreaOverlay shades the ski area into the editor's cell overlay:
// lightly inside, strongly along its edge, and the outline in progress
// (to the cursor) brighter.
func (e *Editor) drawSkiAreaOverlay(set func(cx, cz int, r, g, b, a uint8)) {
	t := e.world.Terrain
	shade := func(o world.SkiAreaOutline, inner, edge uint8) {
		if len(o.Points) < 3 {
			return
		}
		lo, hi := o.Points[0], o.Points[0]
		for _, p := range o.Points {
			lo = mgl32.Vec2{min(lo[0], p[0]), min(lo[1], p[1])}
			hi = mgl32.Vec2{max(hi[0], p[0]), max(hi[1], p[1])}
		}
		x0, z0 := max(int(lo[0]/world.CellSize)-1, 0), max(int(lo[1]/world.CellSize)-1, 0)
		x1, z1 := min(int(hi[0]/world.CellSize)+1, t.Width-1), min(int(hi[1]/world.CellSize)+1, t.Height-1)
		in := func(x, z int) bool {
			return o.Contains((float32(x)+0.5)*world.CellSize, (float32(z)+0.5)*world.CellSize)
		}
		for x := x0; x <= x1; x++ {
			for z := z0; z <= z1; z++ {
				if !in(x, z) {
					continue
				}
				a := inner
				if !in(x-1, z) || !in(x+1, z) || !in(x, z-1) || !in(x, z+1) {
					a = edge
				}
				set(x, z, 170, 90, 220, a)
			}
		}
	}
	drawSkiArea(e.world, 35, 170, set)
	if e.activeTool == toolSkiArea && e.skiSel >= 0 && e.skiSel < len(e.world.SkiArea) {
		shade(e.world.SkiArea[e.skiSel], 110, 255)
	}
	if e.activeTool != toolSkiArea || len(e.skiDraft) == 0 {
		return
	}
	draft := append([]mgl32.Vec2(nil), e.skiDraft...)
	if e.hoverValid {
		draft = append(draft, mgl32.Vec2{e.hoverWorld[0], e.hoverWorld[2]})
	}
	shade(world.SkiAreaOutline{Points: draft}, 70, 200)
	for _, p := range e.skiDraft {
		set(int(p[0]/world.CellSize), int(p[1]/world.CellSize), 255, 230, 120, 230)
	}
}

// drawSkiHandles draws each outline's corners, and its edges' midpoints
// smaller, for dragging; the one under the cursor or being dragged is
// highlighted.
func (e *Editor) drawSkiHandles(r *render.Renderer) {
	active := e.skiHover
	if e.skiDrag.ok {
		active = e.skiDrag
	}
	dot := func(h skiHandle, p mgl32.Vec2, radius float32) {
		sx, sy, ok := r.WorldToScreen(e.skiHandlePos(p))
		if !ok {
			return
		}
		fill := mgl32.Vec4{0.67, 0.35, 0.86, 1}
		if h.mid {
			fill = mgl32.Vec4{0.95, 0.95, 0.95, 0.7}
		}
		if active.ok && h.outline == active.outline && h.corner == active.corner && h.mid == active.mid {
			fill, radius = mgl32.Vec4{1, 0.9, 0.47, 1}, radius+2
		}
		r.DrawColorDisc(sx, sy, radius+1.5, mgl32.Vec4{0.1, 0.05, 0.15, 0.9})
		r.DrawColorDisc(sx, sy, radius, fill)
	}
	for o, ol := range e.world.SkiArea {
		for i, p := range ol.Points {
			q := ol.Points[(i+1)%len(ol.Points)]
			dot(skiHandle{outline: o, corner: i, mid: true}, p.Add(q).Mul(0.5), 3)
		}
		for i, p := range ol.Points {
			dot(skiHandle{outline: o, corner: i}, p, 5)
		}
	}
}
