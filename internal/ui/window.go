package ui

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

const (
	winTitleH  = float32(22)
	winPadding = float32(8)
	winRowH    = float32(20)
	winBtnW    = float32(22)
	winW       = float32(240)
)

// rowKind classifies a window row.
type rowKind int

const (
	rowLabel   rowKind = iota // read-only label + value
	rowStepper                // label + [−] value [+] buttons
)

type windowRow struct {
	kind    rowKind
	label   string
	getText func() string   // for rowLabel
	val     *float32        // for rowStepper — pointer to live value
	step    float32
	minVal  float32
	maxVal  float32
	minusBtn *Button
	plusBtn  *Button
}

// Window is a simple floating info panel.
type Window struct {
	Title   string
	X, Y    float32
	Visible bool

	rows      []*windowRow
	closeBtn  *Button
	height    float32
}

// NewWindow creates a window at the given screen position.
func NewWindow(title string, x, y float32) *Window {
	w := &Window{Title: title, X: x, Y: y}
	w.closeBtn = NewButton(0, 0, 18, 18, "x", func() { w.Visible = false })
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
	row.minusBtn = NewButton(0, 0, winBtnW, winRowH-2, "-", func() {
		v := *captured.val - captured.step
		if v < captured.minVal {
			v = captured.minVal
		}
		*captured.val = v
	})
	row.plusBtn = NewButton(0, 0, winBtnW, winRowH-2, "+", func() {
		v := *captured.val + captured.step
		if v > captured.maxVal {
			v = captured.maxVal
		}
		*captured.val = v
	})
	w.rows = append(w.rows, row)
	w.rebuildLayout()
}

func (w *Window) rebuildLayout() {
	h := winTitleH + winPadding
	for _, row := range w.rows {
		rowY := w.Y + h
		if row.kind == rowStepper {
			// [−] at right side, then value text, then [+]
			row.minusBtn.X = w.X + winW - winBtnW*2 - 6
			row.minusBtn.Y = rowY + 1
			row.plusBtn.X = w.X + winW - winBtnW - 4
			row.plusBtn.Y = rowY + 1
		}
		h += winRowH
	}
	h += winPadding
	w.height = h
	// Close button in top-right corner.
	w.closeBtn.X = w.X + winW - 20
	w.closeBtn.Y = w.Y + 2
}

// Reposition moves the window and rebuilds button positions.
func (w *Window) Reposition(x, y float32) {
	w.X = x
	w.Y = y
	w.rebuildLayout()
}

// HandleInput processes mouse clicks.
func (w *Window) HandleInput(inp *engine.Input) {
	if !w.Visible {
		return
	}
	mx, my := inp.MousePos[0], inp.MousePos[1]
	if !inp.LeftClick {
		// Update hover states only.
		w.closeBtn.SetHovered(w.closeBtn.Contains(mx, my))
		for _, row := range w.rows {
			if row.kind == rowStepper {
				row.minusBtn.SetHovered(row.minusBtn.Contains(mx, my))
				row.plusBtn.SetHovered(row.plusBtn.Contains(mx, my))
			}
		}
		return
	}
	if w.closeBtn.Contains(mx, my) {
		w.closeBtn.Click()
		return
	}
	for _, row := range w.rows {
		if row.kind == rowStepper {
			if row.minusBtn.Contains(mx, my) {
				row.minusBtn.Click()
				return
			}
			if row.plusBtn.Contains(mx, my) {
				row.plusBtn.Click()
				return
			}
		}
	}
}

// ContainsPoint returns true if the given point is inside the window.
func (w *Window) ContainsPoint(x, y float32) bool {
	return w.Visible && x >= w.X && x <= w.X+winW && y >= w.Y && y <= w.Y+w.height
}

// Draw renders the window.
func (w *Window) Draw(r *render.Renderer) {
	if !w.Visible {
		return
	}

	bgColor := mgl32.Vec4{0.08, 0.10, 0.14, 0.95}
	titleBg := mgl32.Vec4{0.15, 0.20, 0.35, 1.0}
	textColor := mgl32.Vec4{0.9, 0.95, 1.0, 1.0}
	labelColor := mgl32.Vec4{0.6, 0.7, 0.85, 1.0}

	// Background.
	r.DrawColorRect(w.X, w.Y, winW, w.height, bgColor)
	// Title bar.
	r.DrawColorRect(w.X, w.Y, winW, winTitleH, titleBg)
	if r.Font != nil {
		r.Font.DrawText(r, w.Title, w.X+winPadding, w.Y+5, textColor)
	}
	w.closeBtn.Draw(r)

	y := w.Y + winTitleH + winPadding
	for _, row := range w.rows {
		if r.Font != nil {
			r.Font.DrawText(r, row.label+":", w.X+winPadding, y+4, labelColor)
		}
		switch row.kind {
		case rowLabel:
			val := ""
			if row.getText != nil {
				val = row.getText()
			}
			if r.Font != nil {
				r.Font.DrawText(r, val, w.X+winW/2, y+4, textColor)
			}
		case rowStepper:
			val := fmt.Sprintf("%.3f", *row.val)
			if r.Font != nil {
				r.Font.DrawText(r, val, w.X+winW/2, y+4, textColor)
			}
			row.minusBtn.Draw(r)
			row.plusBtn.Draw(r)
		}
		y += winRowH
	}
}
