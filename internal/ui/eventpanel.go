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
	Text string     // event message; wrapped to the panel width (up to eventMaxLines)
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
	lines   [][]string // each row's message, wrapped (layoutRows)
	ys      []float32  // each shown row's top; rows past the bottom aren't laid out
	font    *render.Font
	hovered int // index into rows, -1 when none
	bgColor mgl32.Vec4
}

// eventPanelHeaderH covers the gap above the panel and its title strip.
var eventPanelHeaderH = sidePanelInset + float32(render.GlyphH+10)

// eventLineH is a text line in a row, eventRowPad the space round a row,
// and eventMaxLines the most lines a message wraps to.
const (
	eventLineH    = float32(24)
	eventRowPad   = float32(8)
	eventMaxLines = 3
)

// Row text starts here, after the kind marker.
const (
	eventPad     = float32(18)
	eventMarkerW = float32(4)
	eventTextX   = eventPad + eventMarkerW + 8
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

// rowH is row i's height: the timestamp and its message's lines.
func (p *EventPanel) rowH(i int) float32 {
	return eventRowPad + eventLineH*float32(1+len(p.lines[i]))
}

// layoutRows wraps each row's message to the panel and finds where the
// rows that fit stand.
func (p *EventPanel) layoutRows() {
	maxW := p.Width - eventTextX - eventPad
	p.lines = p.lines[:0]
	p.ys = p.ys[:0]
	y := p.Top + eventPanelHeaderH
	for i, row := range p.rows {
		lines := []string{row.Text}
		if p.font != nil {
			lines = WrapText(row.Text, maxW, p.font.TextWidth)
			if len(lines) > eventMaxLines {
				lines = lines[:eventMaxLines]
				lines[eventMaxLines-1] = fitText(p.font, lines[eventMaxLines-1]+" …", maxW)
			}
		}
		p.lines = append(p.lines, lines)
		if y+p.rowH(i) > p.Bottom-sidePanelInset {
			break
		}
		p.ys = append(p.ys, y)
		y += p.rowH(i)
	}
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
	p.layoutRows()
	for i, y := range p.ys {
		if my >= y && my < y+p.rowH(i) {
			p.hovered = i
			if input.LeftClick && p.rows[i].Jumpable && p.OnJump != nil {
				p.OnJump(p.rows[i].X, p.rows[i].Z)
			}
			return
		}
	}
}

// Draw renders the header and as many rows as fit.
func (p *EventPanel) Draw(r *render.Renderer) {
	if !p.Visible {
		return
	}
	p.refresh()
	p.font = r.Font
	DrawPanel(r, sidePanelInset, p.Top+sidePanelInset, p.Width-sidePanelInset, p.Bottom-p.Top-sidePanelInset*2)
	DrawPanelHeader(r, sidePanelInset, p.Top+sidePanelInset, p.Width-sidePanelInset, eventPanelHeaderH-sidePanelInset, "Notifications")
	if r.Font == nil {
		return
	}

	if len(p.rows) == 0 {
		r.Font.DrawText(r, "Nothing yet", 20, p.Top+eventPanelHeaderH+8, mgl32.Vec4{0.55, 0.60, 0.70, 1})
		return
	}

	p.layoutRows()
	for i, y := range p.ys {
		row := p.rows[i]
		h := p.rowH(i)
		if i == p.hovered && row.Jumpable {
			r.DrawRoundRect(sidePanelInset+4, y+1, p.Width-sidePanelInset-8, h-2, ButtonRadius, mgl32.Vec4{0.22, 0.30, 0.50, 0.8})
		}
		r.DrawRoundRect(eventPad, y+6, eventMarkerW, h-12, eventMarkerW/2, row.Tint)
		r.Font.DrawText(r, row.When, eventTextX, y+2, mgl32.Vec4{0.55, 0.60, 0.70, 1})
		for k, line := range p.lines[i] {
			r.Font.DrawText(r, line, eventTextX, y+2+eventLineH*float32(k+1), mgl32.Vec4{0.90, 0.93, 1.0, 1})
		}
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
