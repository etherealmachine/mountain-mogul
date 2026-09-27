package ui

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

// EventRow is one line in the EventPanel. The scene converts world.Events
// into rows so the ui package stays free of world/sim imports.
type EventRow struct {
	When string     // short timestamp, e.g. "Dec 14"
	Text string     // event message; truncated to fit the panel width
	Tint mgl32.Vec4 // colour of the left-edge kind marker

	// Jumpable marks X/Z (world metres) as a camera target; clicking the
	// row calls EventPanel.OnJump with them.
	Jumpable bool
	X, Z     float32
}

// EventPanel is the left-side list of recent world events, newest at the
// top. Rows with a position are clickable and jump the camera there. Like
// OverlayPanel, it has zero hit-test area while hidden.
type EventPanel struct {
	// Top and Bottom are the vertical span in screen coordinates; the
	// caller keeps them between the top bar and the bottom tool bar.
	Top, Bottom float32
	Width       float32
	Visible     bool

	// GetRows returns the rows to show, newest-first. Called every frame
	// the panel is visible.
	GetRows func() []EventRow

	// OnJump is called with a row's world XZ when a jumpable row is clicked.
	OnJump func(x, z float32)

	rows    []EventRow // snapshot from the last HandleInput / Draw
	hovered int        // index into rows, -1 when none
	bgColor mgl32.Vec4
}

const (
	eventPanelHeaderH = float32(22)
	eventPanelRowH    = float32(52) // two text lines: timestamp + message
)

// NewEventPanel returns a hidden panel; the top-bar button toggles it.
func NewEventPanel() *EventPanel {
	return &EventPanel{
		Width:   380,
		hovered: -1,
		bgColor: mgl32.Vec4{0.07, 0.09, 0.15, 0.96},
	}
}

// Toggle flips visibility and returns the new state.
func (p *EventPanel) Toggle() bool {
	p.Visible = !p.Visible
	return p.Visible
}

// ContainsXY reports whether a screen point is inside the visible panel.
func (p *EventPanel) ContainsXY(mx, my float32) bool {
	if !p.Visible {
		return false
	}
	return mx >= 0 && mx <= p.Width && my >= p.Top && my <= p.Bottom
}

// visibleRowCount is how many rows fit between the header and Bottom.
func (p *EventPanel) visibleRowCount() int {
	n := int((p.Bottom - p.Top - eventPanelHeaderH) / eventPanelRowH)
	if n > len(p.rows) {
		n = len(p.rows)
	}
	if n < 0 {
		n = 0
	}
	return n
}

func (p *EventPanel) refresh() {
	p.rows = p.rows[:0]
	if p.GetRows != nil {
		p.rows = append(p.rows, p.GetRows()...)
	}
}

// HandleInput updates hover state and fires OnJump for a clicked row.
// Consumes any left click inside the panel so world tools don't fire.
func (p *EventPanel) HandleInput(input *engine.Input) {
	p.hovered = -1
	if !p.Visible {
		return
	}
	p.refresh()
	mx, my := input.MousePos[0], input.MousePos[1]
	if !p.ContainsXY(mx, my) {
		return
	}
	if input.LeftClick {
		input.LeftClickConsumed = true
	}
	i := int((my - p.Top - eventPanelHeaderH) / eventPanelRowH)
	if my < p.Top+eventPanelHeaderH || i >= p.visibleRowCount() {
		return
	}
	p.hovered = i
	if input.LeftClick && p.rows[i].Jumpable && p.OnJump != nil {
		p.OnJump(p.rows[i].X, p.rows[i].Z)
	}
}

// Draw renders the header and as many rows as fit.
func (p *EventPanel) Draw(r *render.Renderer) {
	if !p.Visible {
		return
	}
	p.refresh()
	r.DrawColorRect(0, p.Top, p.Width, p.Bottom-p.Top, p.bgColor)
	r.DrawColorRect(0, p.Top, p.Width, eventPanelHeaderH, mgl32.Vec4{0.12, 0.17, 0.30, 1})
	r.DrawColorRectOutline(0, p.Top, p.Width, p.Bottom-p.Top, mgl32.Vec4{0.30, 0.44, 0.72, 0.45})
	if r.Font == nil {
		return
	}
	label := "Events"
	r.Font.DrawText(r, label, (p.Width-r.Font.TextWidth(label))/2,
		p.Top+(eventPanelHeaderH-float32(render.GlyphH))/2, mgl32.Vec4{0.92, 0.94, 1.0, 1})

	if len(p.rows) == 0 {
		r.Font.DrawText(r, "Nothing yet", 12, p.Top+eventPanelHeaderH+8, mgl32.Vec4{0.55, 0.60, 0.70, 1})
		return
	}

	const pad = float32(12)
	const markerW = float32(4)
	textX := pad + markerW + 8
	textMaxW := p.Width - textX - pad
	for i := 0; i < p.visibleRowCount(); i++ {
		row := p.rows[i]
		y := p.Top + eventPanelHeaderH + float32(i)*eventPanelRowH
		if i == p.hovered && row.Jumpable {
			r.DrawColorRect(0, y, p.Width, eventPanelRowH, mgl32.Vec4{0.14, 0.20, 0.34, 1})
		}
		r.DrawColorRect(pad, y+6, markerW, eventPanelRowH-12, row.Tint)
		lineH := eventPanelRowH / 2
		r.Font.DrawText(r, row.When, textX, y+2, mgl32.Vec4{0.55, 0.60, 0.70, 1})
		r.Font.DrawText(r, fitText(r.Font, row.Text, textMaxW), textX, y+lineH-2,
			mgl32.Vec4{0.90, 0.93, 1.0, 1})
	}
}

// fitText truncates s with a trailing "..." so it renders within maxW pixels.
func fitText(f *render.Font, s string, maxW float32) string {
	if f.TextWidth(s) <= maxW {
		return s
	}
	runes := []rune(s)
	for n := len(runes) - 1; n > 0; n-- {
		t := string(runes[:n]) + "..."
		if f.TextWidth(t) <= maxW {
			return t
		}
	}
	return "..."
}
