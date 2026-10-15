//go:build !demo

package scene

import (
	"fmt"

	"github.com/go-gl/glfw/v3.3/glfw"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// Lifts, ropes and the test run in the scenario editor: a lift's popup
// (picked with no tool, as its station's dragged), the game's rope tool,
// free, and the test run: the lifts and trains run empty (sim.Preview)
// from the start; the top bar's pause holds them still to look at, play
// and fast run them on.

// editorChairTypes are the chairlifts a chairlift's popup can change it
// to: the ones on the game's upgrade ladder, either way.
var editorChairTypes = []world.LiftType{
	world.LiftDouble, world.LiftFixedTriple, world.LiftFixedQuad,
	world.LiftHSQuad, world.LiftHS6Pack, world.LiftHS8Pack,
}

// openLiftPopup shows lift l's popup: its name, type, speed and chairs,
// a change to another chairlift, bubbles, lights, whether it starts
// open, and Delete.
func (e *Editor) openLiftPopup(l *world.Lift) {
	r := e.app.Renderer
	e.closePopups()
	win := ui.NewWindow("Ski Lift", 0, 0)
	win.AddTextInput("Name", l.Name, func(text string) { l.Name = text; e.markDirty() })
	win.AddLabel("Type", func() string { return l.Type.Label() })
	win.AddLabel("Length", func() string { return fmt.Sprintf("%.0f m", l.Top.Sub(l.Base).Len()) })
	if !l.Cableless() {
		win.AddLabel("Chairs", func() string { return fmt.Sprintf("%d × %d-seat", len(l.Chairs), l.Type.Capacity()) })
	}
	win.AddStepper("Speed (m/s)", &l.Speed, 0.5, 0.5, 8.0)
	if l.Cableless() {
		win.AddIntStepper("Fare per ride ($)", &l.TicketPrice, 5, 0, 200)
	}
	win.AddBoolToggle("Open at the start", func() bool { return l.Open }, func(v bool) { l.Open = v })
	if l.Type.Bubbles() {
		win.AddBoolToggle("Bubble covers", func() bool { return l.Bubble }, func(v bool) { l.Bubble = v })
	}
	if world.NightSkiingAvailable() && !l.Cableless() {
		win.AddBoolToggle("Lights", func() bool { return l.Lit }, func(v bool) { l.Lit = v })
	}
	if isChairType(l.Type) {
		for _, t := range editorChairTypes {
			if t == l.Type {
				continue
			}
			win.AddActionButton("Make it a "+t.Label(), func() { e.retypeLift(l, t) })
		}
	}
	win.AddActionButton("Delete lift", func() {
		e.structureEdit = structureEditSelection{lift: l}
		deleteSelectedStructure(r, e.world, &e.structureEdit)
		e.layerCache.fields = nil
		e.markDirty()
		win.Visible = false
	})
	win.Visible = true
	win.Center(r.ScreenWidth(), r.ScreenHeight())
	e.liftPopup = win
}

func isChairType(t world.LiftType) bool {
	for _, c := range editorChairTypes {
		if c == t {
			return true
		}
	}
	return false
}

// retypeLift makes lift l a t on the same line, if its line suits one.
func (e *Editor) retypeLift(l *world.Lift, t world.LiftType) {
	r := e.app.Renderer
	if p := liftLineProblem(e.world.Terrain, t, l.Base, l.Top); p != "" {
		e.setToast(p)
		return
	}
	world.RetypeLift(l, t)
	l.Bubble = l.Bubble && t.Bubbles()
	applyLiftPlacementEffects(e.world, l)
	r.FlushTerrainEdit(e.world.Terrain)
	r.RemoveLiftCable(l.ID)
	r.AddLiftCable(l, e.world.Terrain)
	r.RebuildStaticBatch(e.world)
	e.layerCache.fields = nil
	e.markDirty()
	e.openLiftPopup(l)
}

// runPreview runs the lifts and trains on dt seconds at the test run's
// speed: from the editor's start, empty, and still while paused.
func (e *Editor) runPreview(dt float64) {
	if e.preview == nil || e.preview.World() != e.world {
		e.preview = sim.NewPreview(e.world) // a new or reloaded map
	}
	for range e.testSpeed {
		e.preview.Tick(dt)
	}
}

// startTestRun runs the lifts and trains at speed (1 or 8).
func (e *Editor) startTestRun(speed int) {
	e.testSpeed = speed
	e.syncToolButtons()
}

// stopTestRun pauses the lifts and trains where they are, to look at.
func (e *Editor) stopTestRun() {
	e.testSpeed = 0
	e.syncToolButtons()
}

// activateRopeTool starts the rope tool (rope_tool.go); re-clicking the
// Rope button ends it.
func (e *Editor) activateRopeTool() {
	e.setTool(toolRope)
	if e.activeTool == toolRope {
		e.ropeTool = newRopeTool()
	}
}

// updateRopeTool runs a frame of the rope tool, free.
func (e *Editor) updateRopeTool(r *render.Renderer, inp *engine.Input, covered bool) {
	if e.activeTool != toolRope {
		return
	}
	before := len(e.world.Ropes)
	env := ropeEnv{w: e.world, r: r, toast: e.setToast, free: true}
	env.input(&e.ropeTool, ropeInput{
		toolInput: toolInput{
			mouse: inp.MousePos, covered: covered,
			ground: e.hoverWorld, groundValid: e.hoverValid,
			leftClick: inp.LeftClick && !inp.LeftClickConsumed, leftHeld: inp.LeftHeld,
			rightClick: inp.RightClick, rightRelease: inp.RightRelease,
			enter: inp.Pressed[glfw.KeyEnter] || inp.Pressed[glfw.KeyKPEnter],
		},
		insert: inp.Held[glfw.KeyLeftShift] || inp.Held[glfw.KeyRightShift],
	})
	if len(e.world.Ropes) != before || e.ropeTool.dragging {
		e.markDirty()
	}
}

// StartTestRun turns the test run on and runs it ahead seconds at once
// (the -editor-test screenshot).
func (e *Editor) StartTestRun(ahead float64) {
	e.startTestRun(1)
	e.runPreview(0)
	for t := 0.0; t < ahead; t += 0.1 {
		e.preview.Tick(0.1)
	}
}
