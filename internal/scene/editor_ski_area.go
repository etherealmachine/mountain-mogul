package scene

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/world"
)

// The scenario editor's ski-area boundary tool. Click to place corners;
// click the first corner (or press Enter) to close the outline; right-
// click to take back the last corner, or, with no outline in progress,
// to delete the outline under the cursor. O copies OpenStreetMap's
// ski-area boundary, where the import has one, to draw over or keep.
// The ground inside is the resort's skiable terrain (world.SkiArea).

const toolSkiArea = toolMode(108)

// skiAreaCloseDist is how near the first corner a click closes the
// outline, in metres.
const skiAreaCloseDist = 10

func (e *Editor) activateSkiAreaTool() {
	e.skiDraft = nil
	e.setTool(toolSkiArea)
	if e.activeTool != toolSkiArea {
		return
	}
	msg := "Click to place corners; click the first corner or press Enter to close. Right-click: undo a corner, or delete an outline."
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

// skiAreaRightClick takes back the last corner, or deletes the outline
// under pos when none is in progress.
func (e *Editor) skiAreaRightClick(pos mgl32.Vec2) {
	if n := len(e.skiDraft); n > 0 {
		e.skiDraft = e.skiDraft[:n-1]
		return
	}
	if e.world.RemoveSkiAreaAt(pos[0], pos[1]) {
		e.markDirty()
		e.setToast("Removed that part of the ski area.")
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
	for _, o := range e.world.SkiArea {
		shade(o, 35, 170)
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
