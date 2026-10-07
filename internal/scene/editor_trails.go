package scene

import (
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// The scenario editor's trail tool: the Trail button starts a new green
// trail and paints it (drag), right-drag erases; clicking a trail with no
// tool opens its popup (name, difficulty, grooming, Add and Remove cells,
// delete). Painting, colours, and the popup are shared with the game
// (trail_tool.go).

// editorTrail is the trail the editor is painting or showing.
type editorTrail struct {
	id       uint64 // the trail being painted, or whose popup is open
	erase    bool   // left-drag removes cells (the popup's Remove)
	stroking bool   // cells changed since the trail graph was rebuilt
}

// activateTrailTool starts painting a new trail, or stops painting.
func (e *Editor) activateTrailTool() {
	if e.activeTool == toolTrailPaint {
		e.setTool(toolTrailPaint) // toggles off
		e.tidyTrail()
		return
	}
	t := e.world.PlaceTrail("", world.DiffGreen)
	e.trail = editorTrail{id: t.ID}
	e.setTool(toolTrailPaint)
	e.setToast("Drag to add cells. Right-drag to remove. Click the trail with no tool to set its difficulty. Esc to finish.")
}

// paintTrailAt paints (or erases) the active trail under the brush.
func (e *Editor) paintTrailAt(c [2]int, erase bool) {
	if e.activeTool != toolTrailPaint || e.trail.id == 0 || !e.world.Terrain.InBounds(c[0], c[1]) {
		return
	}
	paintTrail(e.world, e.trail.id, c[0], c[1], erase)
	e.trail.stroking = true
	e.markDirty()
}

// finishTrailStroke rebuilds the trail graph once a stroke ends, and
// drops a new trail left with no cells once the tool is put down.
func (e *Editor) finishTrailStroke() {
	if e.trail.stroking {
		e.world.RebuildTrailGraph()
		e.trail.stroking = false
	}
	if e.activeTool != toolTrailPaint && (e.trailPopup == nil || !e.trailPopup.Visible) {
		e.tidyTrail()
	}
}

// tidyTrail forgets the active trail, deleting it if it has no cells.
func (e *Editor) tidyTrail() {
	if e.trail.id == 0 {
		return
	}
	if t := e.world.FindTrail(e.trail.id); t != nil && len(t.Cells) == 0 {
		e.world.DeleteTrail(t.ID)
		e.world.RebuildTrailGraph()
	}
	e.trail = editorTrail{}
}

// openTrailPopup shows trail t's popup.
func (e *Editor) openTrailPopup(t *world.Trail, screenW, screenH int) {
	e.openTrailWindow(t, false, screenW, screenH)
}

func (e *Editor) openTrailWindow(t *world.Trail, confirmClear bool, screenW, screenH int) {
	for _, p := range []*ui.Window{e.parcelPopup, e.lotPopup, e.entryPopup} {
		if p != nil {
			p.Visible = false
		}
	}
	e.trail = editorTrail{id: t.ID}
	win := newTrailWindow(e.world, t, confirmClear, trailHooks{
		changed: e.markDirty,
		edit: func(erase bool) {
			e.trailPopup.Visible = false
			e.setTool(toolTrailPaint)
			e.trail = editorTrail{id: t.ID, erase: erase}
			if erase {
				e.setToast("Drag to remove cells. Esc to finish.")
			} else {
				e.setToast("Drag to add cells. Right-drag to remove. Esc to finish.")
			}
		},
		deleted: func() {
			e.markDirty()
			e.trailPopup.Visible = false
			e.trail = editorTrail{}
		},
		reopen: func(confirm bool) { e.openTrailWindow(t, confirm, screenW, screenH) },
	})
	win.Visible = true
	win.Center(screenW, screenH)
	e.trailPopup = win
}

// showingTrail is the trail drawn brighter: the one being painted or
// whose popup is open.
func (e *Editor) showingTrail() uint64 {
	if e.activeTool == toolTrailPaint || (e.trailPopup != nil && e.trailPopup.Visible) {
		return e.trail.id
	}
	return 0
}
