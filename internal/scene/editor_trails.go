package scene

import (
	"github.com/go-gl/glfw/v3.3/glfw"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// The scenario editor's trail tools: the Trail button starts the run
// tool (run_tool.go), as in the game; clicking a trail with no tool opens
// its popup (name, difficulty, grooming, shape, Edit shape, delete). The
// tool, drawing, and popup are shared with the game.

// editorTrail is the trail whose popup is open.
type editorTrail struct {
	id uint64
}

// activateTrailTool starts the run tool for drawing new runs; re-clicking
// the Trail button ends it. Runs are edited by selecting them
// (editSelectedTrail).
func (e *Editor) activateTrailTool() {
	if e.activeTool == toolTrailPaint {
		e.setTool(toolTrailPaint) // toggles off
		e.endRunTool()
		return
	}
	e.setTool(toolTrailPaint)
	e.runTool = newRunTool(world.DiffGreen)
	e.setToast("Click near a lift top to start a run, then click to add nodes and click a lift base to finish ([ and ] set the width). Click a run with no tool to select and edit it.")
}

// editSelectedTrail runs a frame of editing the selected run (its popup
// open, no tool) and reports whether it used the left click; clicks on
// its handles or on the run go to it before anything else.
func (e *Editor) editSelectedTrail(inp *engine.Input, covered bool) bool {
	if e.activeTool != toolNone || e.showingTrail() == 0 {
		e.trailEdit = runTool{}
		return false
	}
	e.trailEdit.editing = e.showingTrail()
	return e.runEnv().input(&e.trailEdit, runInput{
		toolInput: toolInput{
			mouse: inp.MousePos, covered: covered,
			ground: e.hoverWorld, groundValid: e.hoverValid,
			leftClick: inp.LeftClick && !inp.LeftClickConsumed, leftHeld: inp.LeftHeld,
			rightClick: inp.RightClick, rightRelease: inp.RightRelease,
		},
		widen:  inp.Pressed[glfw.KeyRightBracket],
		narrow: inp.Pressed[glfw.KeyLeftBracket],
	})
}

// endRunTool clears the run tool's live drawing.
func (e *Editor) endRunTool() {
	e.runTool = runTool{}
	e.app.Renderer.SetTrailLayer(render.TrailLayerLive, nil, nil)
}

func (e *Editor) runEnv() runEnv {
	return runEnv{w: e.world, toast: e.setToast, changed: e.markDirty}
}

// updateRunTool runs the run tool for a frame, and keeps the drawn
// trails up to date (every trail is shown in the editor).
func (e *Editor) updateRunTool(r *render.Renderer, inp *engine.Input, covered bool) {
	e.trailEditUsed = e.editSelectedTrail(inp, covered)
	if e.activeTool == toolTrailPaint {
		e.runEnv().input(&e.runTool, runInput{
			toolInput: toolInput{
				mouse: inp.MousePos, covered: covered,
				ground: e.hoverWorld, groundValid: e.hoverValid,
				leftClick: inp.LeftClick && !inp.LeftClickConsumed, leftHeld: inp.LeftHeld,
				rightClick: inp.RightClick, rightRelease: inp.RightRelease,
				enter: inp.Pressed[glfw.KeyEnter] || inp.Pressed[glfw.KeyKPEnter],
			},
			widen:  inp.Pressed[glfw.KeyRightBracket],
			narrow: inp.Pressed[glfw.KeyLeftBracket],
			newRun: inp.Held[glfw.KeyLeftShift] || inp.Held[glfw.KeyRightShift],
		})
	}
	var tool *runTool
	switch {
	case e.activeTool == toolTrailPaint:
		tool = &e.runTool
	case e.trailEdit.editing != 0:
		tool = &e.trailEdit
	}
	e.trailDraw.live(r, e.world, tool)
	var live uint64
	if tool != nil && !tool.drawing {
		live = tool.focus
	}
	e.trailDraw.update(r, e.world, e.time, true, e.showingTrail(), live)
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

// showingTrail is the trail drawn brighter: the one whose popup is open.
func (e *Editor) showingTrail() uint64 {
	if e.trailPopup != nil && e.trailPopup.Visible {
		return e.trail.id
	}
	return 0
}
