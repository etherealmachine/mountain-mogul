package ui

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
)

// The UI's shared look: translucent rounded panels over the world, rounded
// buttons, and round close buttons. Every panel draws through these so the
// style stays one decision.

const (
	// PanelRadius rounds panel corners; ButtonRadius rounds buttons.
	PanelRadius  = float32(10)
	ButtonRadius = float32(6)
	// sidePanelInset is the gap between a side panel and the screen edge
	// and bars.
	sidePanelInset = float32(8)
)

var (
	// PanelFill lets the world show faintly through panels.
	PanelFill = mgl32.Vec4{0.07, 0.10, 0.17, 0.88}
	// ModalFill is a dialog's fill: nearly opaque, so text over a
	// busy background stays easy to read.
	ModalFill = mgl32.Vec4{0.08, 0.11, 0.19, 0.97}
	// PanelHeader is a panel's title strip.
	PanelHeader = mgl32.Vec4{0.14, 0.21, 0.38, 0.92}
	// PanelBorder is the thin light edge round panels.
	PanelBorder = mgl32.Vec4{0.60, 0.74, 1.00, 0.30}
	// panelShadow is the soft dark offset under panels.
	panelShadow = mgl32.Vec4{0, 0, 0, 0.22}
	// Scrim dims the world behind a modal dialog.
	Scrim = mgl32.Vec4{0.02, 0.03, 0.06, 0.55}
)

// DrawPanel draws a floating panel: shadow, translucent fill, edge.
func DrawPanel(r *render.Renderer, x, y, w, h float32) {
	DrawPanelColor(r, x, y, w, h, PanelFill)
}

// DrawPanelColor is DrawPanel with its own fill.
func DrawPanelColor(r *render.Renderer, x, y, w, h float32, fill mgl32.Vec4) {
	r.DrawRoundRect(x+1, y+3, w, h, PanelRadius+1, panelShadow)
	r.DrawRoundRect(x, y, w, h, PanelRadius, fill)
	r.DrawRoundRectOutline(x, y, w, h, PanelRadius, 1, PanelBorder)
}

// DrawPanelHeader draws a panel's title strip, rounded to match the
// panel's top corners, with title in it.
func DrawPanelHeader(r *render.Renderer, x, y, w, h float32, title string) {
	r.DrawRoundRectCorners(x, y, w, h, PanelRadius, render.CornersTop, PanelHeader)
	r.DrawColorRect(x+1, y+h-1, w-2, 1, PanelBorder)
	if r.Font != nil && title != "" {
		r.Font.DrawText(r, title, x+12, y+(h-float32(render.GlyphH))/2, mgl32.Vec4{0.94, 0.96, 1.0, 1})
	}
}

// CloseButton is a panel's round ✕ button.
type CloseButton struct {
	CX, CY, R float32 // centre and radius
	hovered   bool
	onClick   func()
}

// NewCloseButton creates a close button that calls onClick.
func NewCloseButton(onClick func()) *CloseButton {
	return &CloseButton{R: 11, onClick: onClick}
}

// PlaceInHeader centres the button vertically in a header strip of height
// h at y, inset from the panel's right edge right.
func (c *CloseButton) PlaceInHeader(right, y, h float32) {
	c.CX = right - h/2 - 2
	c.CY = y + h/2
	c.R = min(11, h/2-3)
}

// Contains reports whether (mx, my) is on the button.
func (c *CloseButton) Contains(mx, my float32) bool {
	dx, dy := mx-c.CX, my-c.CY
	return dx*dx+dy*dy <= (c.R+2)*(c.R+2)
}

// SetHovered updates the hover state.
func (c *CloseButton) SetHovered(v bool) { c.hovered = v }

// Click calls the button's handler.
func (c *CloseButton) Click() {
	if c.onClick != nil {
		c.onClick()
	}
}

// Draw renders the disc and its ✕.
func (c *CloseButton) Draw(r *render.Renderer) {
	fill := mgl32.Vec4{0.26, 0.34, 0.55, 0.95}
	if c.hovered {
		fill = mgl32.Vec4{0.82, 0.28, 0.26, 1}
	}
	r.DrawColorDisc(c.CX, c.CY, c.R, fill)
	r.DrawRing(c.CX, c.CY, c.R, 1, mgl32.Vec4{1, 1, 1, 0.25})
	a := c.R * 0.38
	white := mgl32.Vec4{1, 1, 1, 0.95}
	r.DrawColorLine(c.CX-a, c.CY-a, c.CX+a, c.CY+a, 2, white)
	r.DrawColorLine(c.CX-a, c.CY+a, c.CX+a, c.CY-a, 2, white)
}

// DrawTooltip draws a small label box for text, above (x, y) when there
// is room, else below, kept on screen.
func DrawTooltip(r *render.Renderer, text string, x, y float32, below bool) {
	if r.Font == nil || text == "" {
		return
	}
	const padX, padY = float32(10), float32(5)
	w := r.Font.TextWidth(text) + padX*2
	h := float32(render.GlyphH) + padY*2
	tx := x - w/2
	ty := y - h - 6
	if below {
		ty = y + 6
	}
	sw := float32(r.ScreenWidth())
	tx = max(4, min(tx, sw-w-4))
	r.DrawRoundRect(tx, ty, w, h, 6, mgl32.Vec4{0.04, 0.06, 0.11, 0.94})
	r.DrawRoundRectOutline(tx, ty, w, h, 6, 1, PanelBorder)
	r.Font.DrawText(r, text, tx+padX, ty+padY, mgl32.Vec4{0.96, 0.97, 1.0, 1})
}

// DrawCheckbox draws a size×size check box at (x, y), ticked when on.
func DrawCheckbox(r *render.Renderer, x, y, size float32, on, hovered bool) {
	fill := mgl32.Vec4{0.10, 0.14, 0.24, 0.95}
	edge := mgl32.Vec4{0.60, 0.75, 1.00, 0.55}
	if on {
		fill = mgl32.Vec4{0.18, 0.55, 0.32, 1}
		edge = mgl32.Vec4{0.50, 0.90, 0.62, 0.80}
	}
	if hovered {
		edge[3] = 1
	}
	r.DrawRoundRect(x, y, size, size, 5, fill)
	r.DrawRoundRectOutline(x, y, size, size, 5, 1.5, edge)
	if on {
		white := mgl32.Vec4{1, 1, 1, 1}
		r.DrawColorLine(x+size*0.24, y+size*0.52, x+size*0.43, y+size*0.70, 2.5, white)
		r.DrawColorLine(x+size*0.43, y+size*0.70, x+size*0.77, y+size*0.32, 2.5, white)
	}
}

// DrawClosedSign draws the "closed" mark, a red circle with a slash
// across it on white, centred at (cx, cy).
func DrawClosedSign(r *render.Renderer, cx, cy, radius float32) {
	red := mgl32.Vec4{0.86, 0.16, 0.14, 1}
	r.DrawColorDisc(cx+1, cy+2, radius+1, mgl32.Vec4{0, 0, 0, 0.3})
	r.DrawColorDisc(cx, cy, radius, mgl32.Vec4{1, 1, 1, 0.95})
	t := radius * 0.24
	r.DrawRing(cx, cy, radius, t, red)
	a := (radius - t) * 0.72
	r.DrawColorLine(cx-a, cy-a, cx+a, cy+a, t, red)
}

// DrawWarningSign draws a warning sign centred at (cx, cy): a yellow
// triangle, black-edged, with an exclamation mark (a railway's
// unelectrified track, notes/next/Rail Network.md).
func DrawWarningSign(r *render.Renderer, cx, cy, radius float32) {
	tri := func(rad float32, col mgl32.Vec4, dx, dy float32) {
		h := rad * 0.866
		r.DrawColorTriangle(cx+dx, cy-rad+dy, cx+h+dx, cy+rad/2+dy, cx-h+dx, cy+rad/2+dy, col)
	}
	tri(radius+2, mgl32.Vec4{0, 0, 0, 0.3}, 1, 2)
	tri(radius+1.5, mgl32.Vec4{0.08, 0.08, 0.08, 1}, 0, 0)
	tri(radius-1, mgl32.Vec4{1, 0.82, 0.12, 1}, 0, 0)
	t := radius * 0.16
	r.DrawColorLine(cx, cy-radius*0.45, cx, cy+radius*0.05, t, mgl32.Vec4{0.08, 0.08, 0.08, 1})
	r.DrawColorDisc(cx, cy+radius*0.27, t*0.6, mgl32.Vec4{0.08, 0.08, 0.08, 1})
}

// DrawSpeedSign draws a speed-limit sign centred at (cx, cy): a white
// disc in a red ring with the limit in it (a railway's slower track).
func DrawSpeedSign(r *render.Renderer, cx, cy, radius float32, limit string) {
	red := mgl32.Vec4{0.86, 0.16, 0.14, 1}
	r.DrawColorDisc(cx+1, cy+2, radius+1, mgl32.Vec4{0, 0, 0, 0.3})
	r.DrawColorDisc(cx, cy, radius, mgl32.Vec4{1, 1, 1, 0.97})
	r.DrawRing(cx, cy, radius, radius*0.2, red)
	if r.Font != nil {
		_, h := r.Font.CharSize()
		r.Font.DrawText(r, limit, cx-r.Font.TextWidth(limit)/2, cy-float32(h)/2, mgl32.Vec4{0.08, 0.08, 0.08, 1})
	}
}
