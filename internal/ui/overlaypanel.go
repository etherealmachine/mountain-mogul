package ui

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

// OverlayPanel is the vertical right-side toolbar that toggles terrain
// view overlays (contour, slope, snow depth, grooming, packed, ice,
// moguls). Toggles are independent — any subset can be active at once —
// and the active set is stored as a bitmask matching the
// render.Overlay* constants.
//
// The panel owns the mask state; callers read it via Mask() each frame
// and forward it to render.Renderer.TerrainOverlayMode. The panel
// itself can be shown or hidden by the top-bar overlay button; when
// hidden it has zero hit-test area, so world clicks pass through.
type OverlayPanel struct {
	// Top and Bottom are the vertical span of the panel in screen
	// coordinates. The caller sets them so the panel fits between the
	// top bar and the bottom toolbar without overlapping either.
	Top, Bottom float32

	// Width is the horizontal width of the panel. Sized so an icon plus
	// a short label fits comfortably.
	Width float32

	Visible bool

	mask    int
	rows    []*overlayRow
	bgColor mgl32.Vec4
}

// overlayRow is one toggle in the panel.
type overlayRow struct {
	bit     int // OverlayContour, OverlaySlope, … (see render.Overlay*)
	label   string
	icon    render.IconName
	tint    mgl32.Vec4 // icon tint when active (matches the heatmap palette)
	x, y    float32
	w, h    float32
	hovered bool
}

// NewOverlayPanel builds the seven-row panel. Visibility defaults to
// hidden — the caller flips it on via the top-bar button.
func NewOverlayPanel() *OverlayPanel {
	p := &OverlayPanel{
		Width:   148,
		bgColor: mgl32.Vec4{0.08, 0.10, 0.14, 0.94},
	}
	p.rows = []*overlayRow{
		{bit: render.OverlayContour, label: "Contour", icon: render.IconChartLine,
			tint: mgl32.Vec4{0.65, 0.70, 0.80, 1}},
		{bit: render.OverlaySlope, label: "Slope", icon: render.IconTriangle,
			tint: mgl32.Vec4{0.93, 0.80, 0.08, 1}},
		{bit: render.OverlaySnowDepth, label: "Snow depth", icon: render.IconWaves,
			tint: mgl32.Vec4{0.30, 0.55, 0.95, 1}},
		{bit: render.OverlayGrooming, label: "Grooming", icon: render.IconBroom,
			tint: mgl32.Vec4{0.30, 0.95, 0.55, 1}},
		{bit: render.OverlayPacked, label: "Packed", icon: render.IconGridFour,
			tint: mgl32.Vec4{0.95, 0.55, 0.20, 1}},
		{bit: render.OverlayIce, label: "Ice", icon: render.IconDrop,
			tint: mgl32.Vec4{0.30, 0.95, 1.00, 1}},
		{bit: render.OverlayMoguls, label: "Moguls", icon: render.IconDotsNine,
			tint: mgl32.Vec4{0.95, 0.30, 0.75, 1}},
	}
	return p
}

// Toggle flips visibility. Returns the new state for callers that want
// to update an associated icon-button active flag.
func (p *OverlayPanel) Toggle() bool {
	p.Visible = !p.Visible
	return p.Visible
}

// ContainsXY returns whether the given screen point is inside the panel
// rectangle. Used by the scenario to gate world clicks: when the panel
// swallows a click it shouldn't also place a building.
func (p *OverlayPanel) ContainsXY(mx, my float32, screenW float32) bool {
	if !p.Visible {
		return false
	}
	x := p.x(screenW)
	return mx >= x && mx <= x+p.Width && my >= p.Top && my <= p.Bottom
}

// Mask returns the current overlay bitmask. Callers wire this into
// render.Renderer.TerrainOverlayMode each frame so the shader sees the
// up-to-date set.
func (p *OverlayPanel) Mask() int { return p.mask }

// SetMask overrides the active bits — used at scene init to restore
// a saved overlay configuration or to clear all overlays.
func (p *OverlayPanel) SetMask(m int) { p.mask = m }

// SetBit forces a single overlay bit on or off without disturbing the
// others. Used by hotkeys (e.g. the legacy `C` key for contour) that
// want to toggle one overlay without opening the panel.
func (p *OverlayPanel) SetBit(bit int, on bool) {
	if on {
		p.mask |= bit
	} else {
		p.mask &^= bit
	}
}

// ToggleBit flips a single overlay bit. Returns the new state of that bit.
func (p *OverlayPanel) ToggleBit(bit int) bool {
	p.mask ^= bit
	return (p.mask & bit) != 0
}

// HandleInput processes hover + click against the panel's seven toggles
// and mutates the internal mask for any clicked rows. No-op when the
// panel is hidden.
func (p *OverlayPanel) HandleInput(input *engine.Input, screenW float32) {
	if !p.Visible {
		return
	}
	p.layout(screenW)
	mx := input.MousePos[0]
	my := input.MousePos[1]
	// Mark the click consumed if it lands anywhere inside the panel rect
	// (including chrome between rows) so world tools don't fire underneath.
	if input.LeftClick && p.ContainsXY(mx, my, screenW) {
		input.LeftClickConsumed = true
	}
	for _, row := range p.rows {
		row.hovered = mx >= row.x && mx <= row.x+row.w && my >= row.y && my <= row.y+row.h
		if input.LeftClick && row.hovered {
			p.mask ^= row.bit
		}
	}
}

// Draw renders the panel background and toggle rows. Reads the panel's
// own mask state — keep it in sync via HandleInput / Set* before draw.
func (p *OverlayPanel) Draw(r *render.Renderer) {
	if !p.Visible {
		return
	}
	screenW := float32(r.ScreenWidth())
	p.layout(screenW)

	x := p.x(screenW)
	r.DrawColorRect(x, p.Top, p.Width, p.Bottom-p.Top, p.bgColor)

	// Header strip — labels what this panel does. Subtle, just a tinted
	// band across the top.
	headerH := float32(22)
	r.DrawColorRect(x, p.Top, p.Width, headerH, mgl32.Vec4{0.14, 0.18, 0.26, 1})
	if r.Font != nil {
		label := "Overlays"
		labelW := float32(len(label) * render.GlyphAdvance)
		r.Font.DrawText(r, label, x+(p.Width-labelW)/2, p.Top+(headerH-float32(render.GlyphH))/2,
			mgl32.Vec4{0.92, 0.94, 1.0, 1})
	}

	for _, row := range p.rows {
		active := (p.mask & row.bit) != 0
		p.drawRow(r, row, active)
	}
}

// drawRow renders one toggle. Hover and active states are visually
// distinct: hover is a faint highlight, active is a brighter background
// matching the overlay's own tint at low saturation.
func (p *OverlayPanel) drawRow(r *render.Renderer, row *overlayRow, active bool) {
	var bg mgl32.Vec4
	switch {
	case active:
		bg = mgl32.Vec4{0.18, 0.32, 0.42, 1}
	case row.hovered:
		bg = mgl32.Vec4{0.16, 0.20, 0.28, 1}
	default:
		bg = mgl32.Vec4{0, 0, 0, 0} // transparent — panel bg shows through
	}
	if bg[3] > 0 {
		r.DrawColorRect(row.x, row.y, row.w, row.h, bg)
	}

	const iconSize = float32(20)
	const pad = float32(8)
	iconY := row.y + (row.h-iconSize)/2
	iconX := row.x + pad

	iconTint := mgl32.Vec4{0.65, 0.70, 0.78, 1}
	if active {
		iconTint = row.tint
	}
	r.DrawIcon(row.icon, iconX, iconY, iconSize, iconTint)

	if r.Font != nil {
		labelX := iconX + iconSize + 8
		labelY := row.y + (row.h-float32(render.GlyphH))/2
		labelCol := mgl32.Vec4{0.82, 0.86, 0.95, 1}
		if active {
			labelCol = mgl32.Vec4{0.96, 0.98, 1.0, 1}
		}
		r.Font.DrawText(r, row.label, labelX, labelY, labelCol)
	}
}

func (p *OverlayPanel) x(screenW float32) float32 {
	return screenW - p.Width
}

func (p *OverlayPanel) layout(screenW float32) {
	const headerH = float32(22)
	x := p.x(screenW)
	rowsTop := p.Top + headerH
	available := p.Bottom - rowsTop
	rowH := available / float32(len(p.rows))
	if rowH < 28 {
		rowH = 28
	}
	for i, row := range p.rows {
		row.x = x
		row.y = rowsTop + float32(i)*rowH
		row.w = p.Width
		row.h = rowH
	}
}
