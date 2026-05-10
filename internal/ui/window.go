package ui

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

const (
	winTitleH    = float32(render.GlyphH + 10) // 36
	winPadding   = float32(10)
	winRowH      = float32(render.GlyphH + 10) // 36
	winBtnW      = float32(24)
	winValueAreaW = float32(130) // right portion: value text + stepper buttons
)

// rowKind classifies a window row.
type rowKind int

const (
	rowLabel        rowKind = iota // read-only label + value
	rowStepper                     // label + [−] value [+] buttons (float)
	rowIntStepper                  // same controls, int-backed value
	rowStepperFn                   // label + [−] valueFn [+] buttons backed by callbacks (no direct pointer)
	rowActionButton                // single full-width button row
)

type windowRow struct {
	kind     rowKind
	label    string
	getText  func() string
	val      *float32
	step     float32
	minVal   float32
	maxVal   float32
	intVal   *int
	intStep  int
	intMin   int
	intMax   int
	minusBtn *Button
	plusBtn  *Button

	// rowActionButton — single full-width clickable.
	actionBtn *Button
}

// Window is a simple floating info panel.
type Window struct {
	Title   string
	X, Y    float32
	Visible bool

	rows     []*windowRow
	closeBtn *Button
	width    float32 // auto-computed from label widths
	height   float32 // auto-computed from row count
	labelW   float32 // label column right edge, relative to X
}

// NewWindow creates a window at the given screen position.
func NewWindow(title string, x, y float32) *Window {
	w := &Window{Title: title, X: x, Y: y}
	w.closeBtn = NewButton(0, 0, winBtnW, float32(render.GlyphH+4), "x", func() { w.Visible = false })
	w.rebuildLayout()
	return w
}

// AddLabel adds a read-only row.
func (w *Window) AddLabel(label string, getText func() string) {
	w.rows = append(w.rows, &windowRow{kind: rowLabel, label: label, getText: getText})
	w.rebuildLayout()
}

// AddStepper adds a row with [−] value [+] controls for a float value.
func (w *Window) AddStepper(label string, val *float32, step, minVal, maxVal float32) {
	row := &windowRow{
		kind:   rowStepper,
		label:  label,
		val:    val,
		step:   step,
		minVal: minVal,
		maxVal: maxVal,
	}
	captured := row
	row.minusBtn = NewButton(0, 0, winBtnW, winRowH-4, "-", func() {
		v := *captured.val - captured.step
		if v < captured.minVal {
			v = captured.minVal
		}
		*captured.val = v
	})
	row.plusBtn = NewButton(0, 0, winBtnW, winRowH-4, "+", func() {
		v := *captured.val + captured.step
		if v > captured.maxVal {
			v = captured.maxVal
		}
		*captured.val = v
	})
	w.rows = append(w.rows, row)
	w.rebuildLayout()
}

// AddIntStepperFn is a stepper backed by callbacks instead of a direct
// pointer to the value. The display is whatever `getText` returns —
// callers typically format the count plus a max (e.g. "1/3"). +/-
// clicks call the supplied handlers; the handlers are responsible for
// clamping and any side effects (spawning entities, deducting cash, …).
func (w *Window) AddIntStepperFn(label string, getText func() string, onMinus, onPlus func()) {
	row := &windowRow{
		kind:    rowStepperFn,
		label:   label,
		getText: getText,
	}
	row.minusBtn = NewButton(0, 0, winBtnW, winRowH-4, "-", onMinus)
	row.plusBtn = NewButton(0, 0, winBtnW, winRowH-4, "+", onPlus)
	w.rows = append(w.rows, row)
	w.rebuildLayout()
}

// AddActionButton adds a full-width clickable row labelled `label`.
// Used for one-shot actions (e.g. "Paint Route") that don't have a
// stepper-style value.
func (w *Window) AddActionButton(label string, onClick func()) {
	row := &windowRow{
		kind:      rowActionButton,
		label:     label,
		actionBtn: NewButton(0, 0, 0, winRowH-4, label, onClick),
	}
	w.rows = append(w.rows, row)
	w.rebuildLayout()
}

// AddIntStepper adds a row with [−] value [+] controls for an int value.
// Same look and behaviour as AddStepper but the value displays as a bare
// integer (no decimals) and clamps in int space.
func (w *Window) AddIntStepper(label string, val *int, step, minVal, maxVal int) {
	row := &windowRow{
		kind:    rowIntStepper,
		label:   label,
		intVal:  val,
		intStep: step,
		intMin:  minVal,
		intMax:  maxVal,
	}
	captured := row
	row.minusBtn = NewButton(0, 0, winBtnW, winRowH-4, "-", func() {
		v := *captured.intVal - captured.intStep
		if v < captured.intMin {
			v = captured.intMin
		}
		*captured.intVal = v
	})
	row.plusBtn = NewButton(0, 0, winBtnW, winRowH-4, "+", func() {
		v := *captured.intVal + captured.intStep
		if v > captured.intMax {
			v = captured.intMax
		}
		*captured.intVal = v
	})
	w.rows = append(w.rows, row)
	w.rebuildLayout()
}

func (w *Window) rebuildLayout() {
	// Width: derive from widest label + fixed value area, floored by title width.
	maxLabelPx := float32(0)
	for _, row := range w.rows {
		lw := float32((len(row.label)+1)*render.GlyphAdvance) // +1 for ':'
		if lw > maxLabelPx {
			maxLabelPx = lw
		}
	}
	w.labelW = winPadding + maxLabelPx + 8 // 8px gap between label and value

	titleMinW := winPadding + float32(len(w.Title)*render.GlyphAdvance) + winBtnW + 20
	w.width = w.labelW + winValueAreaW
	if w.width < titleMinW {
		w.width = titleMinW
	}

	// Height: title bar + rows + half-padding top and bottom.
	h := winTitleH + winPadding/2
	for _, row := range w.rows {
		rowY := w.Y + h
		switch row.kind {
		case rowStepper, rowIntStepper, rowStepperFn:
			btnH := winRowH - 4
			row.plusBtn.X = w.X + w.width - winBtnW - winPadding
			row.plusBtn.Y = rowY + (winRowH-btnH)/2
			row.minusBtn.X = row.plusBtn.X - winBtnW - 4
			row.minusBtn.Y = row.plusBtn.Y
		case rowActionButton:
			btnH := winRowH - 4
			row.actionBtn.X = w.X + winPadding
			row.actionBtn.Y = rowY + (winRowH-btnH)/2
			row.actionBtn.W = w.width - 2*winPadding
			row.actionBtn.H = btnH
		}
		h += winRowH
	}
	h += winPadding / 2
	w.height = h

	// Close button flush with top-right corner.
	w.closeBtn.X = w.X + w.width - winBtnW - 4
	w.closeBtn.Y = w.Y + (winTitleH-float32(render.GlyphH+4))/2
}

// Reposition moves the window and rebuilds button positions.
func (w *Window) Reposition(x, y float32) {
	w.X = x
	w.Y = y
	w.rebuildLayout()
}

// Center repositions the window so it is centred on the screen.
func (w *Window) Center(screenW, screenH int) {
	w.Reposition(
		float32(screenW)/2-w.width/2,
		float32(screenH)/2-w.height/2,
	)
}

// HandleInput processes mouse clicks.
func (w *Window) HandleInput(inp *engine.Input) {
	if !w.Visible {
		return
	}
	mx, my := inp.MousePos[0], inp.MousePos[1]
	w.closeBtn.SetHovered(w.closeBtn.Contains(mx, my))
	for _, row := range w.rows {
		switch row.kind {
		case rowStepper, rowIntStepper, rowStepperFn:
			row.minusBtn.SetHovered(row.minusBtn.Contains(mx, my))
			row.plusBtn.SetHovered(row.plusBtn.Contains(mx, my))
		case rowActionButton:
			row.actionBtn.SetHovered(row.actionBtn.Contains(mx, my))
		}
	}
	if !inp.LeftClick {
		return
	}
	if w.closeBtn.Contains(mx, my) {
		w.closeBtn.Click()
		return
	}
	for _, row := range w.rows {
		switch row.kind {
		case rowStepper, rowIntStepper, rowStepperFn:
			if row.minusBtn.Contains(mx, my) {
				row.minusBtn.Click()
				return
			}
			if row.plusBtn.Contains(mx, my) {
				row.plusBtn.Click()
				return
			}
		case rowActionButton:
			if row.actionBtn.Contains(mx, my) {
				row.actionBtn.Click()
				return
			}
		}
	}
}

// ContainsPoint returns true if the given point is inside the window.
func (w *Window) ContainsPoint(x, y float32) bool {
	return w.Visible && x >= w.X && x <= w.X+w.width && y >= w.Y && y <= w.Y+w.height
}

// Draw renders the window.
func (w *Window) Draw(r *render.Renderer) {
	if !w.Visible {
		return
	}

	bgColor    := mgl32.Vec4{0.08, 0.10, 0.14, 0.95}
	titleBg    := mgl32.Vec4{0.15, 0.20, 0.35, 1.0}
	textColor  := mgl32.Vec4{0.9, 0.95, 1.0, 1.0}
	labelColor := mgl32.Vec4{0.6, 0.7, 0.85, 1.0}

	r.DrawColorRect(w.X, w.Y, w.width, w.height, bgColor)
	r.DrawColorRect(w.X, w.Y, w.width, winTitleH, titleBg)
	if r.Font != nil {
		titleY := w.Y + (winTitleH-float32(render.GlyphH))/2
		r.Font.DrawText(r, w.Title, w.X+winPadding, titleY, textColor)
	}
	w.closeBtn.Draw(r)

	y := w.Y + winTitleH + winPadding/2
	textOffY := (winRowH - float32(render.GlyphH)) / 2

	for _, row := range w.rows {
		// Action buttons get their own row treatment — full width, no
		// label column.
		if row.kind == rowActionButton {
			row.actionBtn.Draw(r)
			y += winRowH
			continue
		}
		if r.Font != nil {
			r.Font.DrawText(r, row.label+":", w.X+winPadding, y+textOffY, labelColor)
		}
		switch row.kind {
		case rowLabel:
			val := ""
			if row.getText != nil {
				val = row.getText()
			}
			if r.Font != nil {
				r.Font.DrawText(r, val, w.X+w.labelW, y+textOffY, textColor)
			}
		case rowStepper:
			val := fmt.Sprintf("%.2f", *row.val)
			if r.Font != nil {
				r.Font.DrawText(r, val, w.X+w.labelW, y+textOffY, textColor)
			}
			row.minusBtn.Draw(r)
			row.plusBtn.Draw(r)
		case rowIntStepper:
			val := fmt.Sprintf("%d", *row.intVal)
			if r.Font != nil {
				r.Font.DrawText(r, val, w.X+w.labelW, y+textOffY, textColor)
			}
			row.minusBtn.Draw(r)
			row.plusBtn.Draw(r)
		case rowStepperFn:
			val := ""
			if row.getText != nil {
				val = row.getText()
			}
			if r.Font != nil {
				r.Font.DrawText(r, val, w.X+w.labelW, y+textOffY, textColor)
			}
			row.minusBtn.Draw(r)
			row.plusBtn.Draw(r)
		}
		y += winRowH
	}
}
