package scene

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// The Labels tool: names on the map (world.Label), shown in play. A
// click on a label selects it and opens its popup (name, kind, height,
// delete); dragging moves it; a click on bare ground adds a peak there.
// Delete removes the selected label once its popup is closed (the popup's
// name field takes the keys while it's open). An imported map starts
// with OpenStreetMap's named peaks.
const toolLabels = toolMode(113)

// labelTool is the Labels tool's session.
type labelTool struct {
	sel      uint64     // the selected label's ID; 0 for none
	dragging bool       // moving the selected label
	grab     mgl32.Vec2 // the label's offset from the ground under the mouse when grabbed
}

func (e *Editor) activateLabelTool() {
	e.setTool(toolLabels)
	e.labels = labelTool{}
	if e.activeTool == toolLabels {
		e.setToast("Click a label to edit or drag it; click the ground to add a peak.")
	}
}

// selectedLabel is the selected label, or nil.
func (e *Editor) selectedLabel() *world.Label {
	for _, l := range e.world.Labels {
		if l.ID == e.labels.sel {
			return l
		}
	}
	return nil
}

// labelMouse handles the tool's clicks and drags on the map.
func (e *Editor) labelMouse(r *render.Renderer, inp *engine.Input) {
	st := &e.labels
	ground := mgl32.Vec2{e.hoverWorld[0], e.hoverWorld[2]}
	if inp.LeftClick {
		if l := labelAt(r, e.world, inp.MousePos[0], inp.MousePos[1]); l != nil {
			st.sel, st.dragging = l.ID, e.hoverValid
			st.grab = l.Pos.Sub(ground)
			e.openLabelPopup(l)
			return
		}
		if !e.hoverValid {
			return
		}
		t := e.world.Terrain
		alt := t.InterpolatedSurfaceElevationAt(ground[0], ground[1]) + e.world.BaseAltitude
		l := e.world.AddLabel(world.Label{Kind: world.LabelPeak, Name: "New peak", Ele: float32(int(alt + 0.5)), Pos: ground})
		st.sel, st.dragging = l.ID, false
		e.openLabelPopup(l)
		e.markDirty()
		return
	}
	if inp.LeftHeld && st.dragging && e.hoverValid {
		if l := e.selectedLabel(); l != nil {
			if p := ground.Add(st.grab); p != l.Pos {
				l.Pos = p
				e.markDirty()
			}
		}
	}
}

// labelRelease ends a drag.
func (e *Editor) labelRelease() { e.labels.dragging = false }

// deleteSelectedLabel removes the selected label.
func (e *Editor) deleteSelectedLabel() {
	l := e.selectedLabel()
	if l == nil {
		return
	}
	e.world.RemoveLabel(l.ID)
	e.labels = labelTool{}
	if e.labelPopup != nil {
		e.labelPopup.Visible = false
	}
	e.markDirty()
	e.setToast("Deleted " + l.Name)
}

// openLabelPopup opens label l's popup: its name, kind, and for a peak
// its height above sea level (0 shows the ground's).
func (e *Editor) openLabelPopup(l *world.Label) {
	r := e.app.Renderer
	e.closePopups()
	win := ui.NewWindow("Label", 0, 0)
	win.AddTextInput("Name", l.Name, func(text string) { l.Name = text; e.markDirty() })
	// Kind steps through the label kinds (only peaks so far).
	stepKind := func(d int) {
		n := int(world.LabelKindCount)
		l.Kind = world.LabelKind((int(l.Kind) + d + n) % n)
		e.markDirty()
	}
	win.AddIntStepperFn("Kind", func() string { return l.Kind.String() }, func() { stepKind(-1) }, func() { stepKind(1) })
	if l.Kind == world.LabelPeak {
		win.AddStepper("Height (m)", &l.Ele, 10, 0, 9000)
		win.AddLabel("Ground here", func() string {
			alt := e.world.Terrain.InterpolatedSurfaceElevationAt(l.Pos[0], l.Pos[1]) + e.world.BaseAltitude
			return fmt.Sprintf("%.0f m (height 0 shows this)", alt)
		})
	}
	win.AddActionButton("Delete label", func() { e.deleteSelectedLabel() })
	win.Visible = true
	win.Center(r.ScreenWidth(), r.ScreenHeight())
	e.labelPopup = win
}
