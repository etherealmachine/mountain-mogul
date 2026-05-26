package ui

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

// MenuBar is a horizontal toolbar. It can sit at the top or bottom — the
// caller sets `Y` directly and the bar re-lays its buttons every frame so
// changes to Y or screen size apply immediately. Buttons can be packed
// left-aligned (default) or centred with even space-around distribution.
type MenuBar struct {
	Y, H         float32
	Buttons      []*Button // tool palette
	RightButtons []*Button // right-aligned cluster (e.g. speed controls)

	// Centered switches the layout for `Buttons` from left-packed to a
	// horizontally-centred group with a fixed inter-button gap. The cluster
	// reads as a single palette regardless of screen width.
	Centered bool

	bgColor mgl32.Vec4
}

// NewMenuBar creates a menu bar at the given Y position and height.
func NewMenuBar(y, h float32) *MenuBar {
	return &MenuBar{
		Y:       y,
		H:       h,
		bgColor: mgl32.Vec4{0.07, 0.09, 0.15, 0.97},
	}
}

// AddButton appends a button to the menu bar. Width is sized to fit the
// label; X/Y are recomputed each frame, so the value passed to NewButton
// here only matters for the initial construction.
func (m *MenuBar) AddButton(label string, onClick func()) *Button {
	w := float32(len(label)*render.GlyphAdvance) + 20
	btn := NewButton(0, 0, w, 0, label, onClick)
	m.Buttons = append(m.Buttons, btn)
	return btn
}

// AddIconButton appends an icon-and-label button using a fixed tile width.
// Position and inner height are set during the per-frame layout.
func (m *MenuBar) AddIconButton(icon render.IconName, label string, onClick func()) *Button {
	const tileW = float32(84)
	btn := NewButton(0, 0, tileW, 0, label, onClick)
	btn.Icon = icon
	m.Buttons = append(m.Buttons, btn)
	return btn
}

// AddRightButton appends a button to the right-aligned cluster. X is
// recomputed each frame from the current screen width.
func (m *MenuBar) AddRightButton(label string, onClick func()) *Button {
	w := float32(len(label)*render.GlyphAdvance) + 20
	btn := NewButton(0, 0, w, 0, label, onClick)
	m.RightButtons = append(m.RightButtons, btn)
	return btn
}

// layout re-positions every button against the current Y/H and screen
// width. Called from both HandleInput and Draw so hit-tests and rendering
// agree on the same geometry, even after the caller moves the bar.
func (m *MenuBar) layout(screenW float32) {
	const padding = float32(6)
	innerY := m.Y + padding
	innerH := m.H - padding*2

	// Left/centre cluster.
	if m.Centered && len(m.Buttons) > 0 {
		// Pack the buttons next to each other with a fixed 8 px gap, then
		// centre the whole cluster horizontally.
		const gap = float32(8)
		var totalW float32
		for i, b := range m.Buttons {
			if i > 0 {
				totalW += gap
			}
			totalW += b.W
		}
		x := (screenW - totalW) / 2
		for i, b := range m.Buttons {
			if i > 0 {
				x += gap
			}
			b.X = x
			b.Y = innerY
			b.H = innerH
			x += b.W
		}
	} else {
		x := float32(0)
		for i, b := range m.Buttons {
			if i > 0 {
				x += padding
			}
			b.X = x
			b.Y = innerY
			b.H = innerH
			x += b.W
		}
	}

	// Right cluster — packed right-to-left from the screen edge.
	x := screenW - padding
	for i := len(m.RightButtons) - 1; i >= 0; i-- {
		btn := m.RightButtons[i]
		x -= btn.W
		btn.X = x
		btn.Y = innerY
		btn.H = innerH
		x -= padding
	}
}

// HandleInput processes mouse input for the menu bar.
func (m *MenuBar) HandleInput(input *engine.Input, screenW, screenH float32) {
	m.layout(screenW)
	mx := input.MousePos[0]
	my := input.MousePos[1]
	for _, btn := range m.Buttons {
		btn.SetHovered(btn.Contains(mx, my))
		if input.LeftClick && btn.Contains(mx, my) {
			input.LeftClickConsumed = true
			btn.Click()
		}
	}
	for _, btn := range m.RightButtons {
		btn.SetHovered(btn.Contains(mx, my))
		if input.LeftClick && btn.Contains(mx, my) {
			input.LeftClickConsumed = true
			btn.Click()
		}
	}
}

// Draw renders the menu bar background and all buttons.
func (m *MenuBar) Draw(r *render.Renderer) {
	screenW := float32(r.ScreenWidth())
	m.layout(screenW)
	r.DrawColorRect(0, m.Y, screenW, m.H, m.bgColor)
	// 1px powder-blue highlight at the top edge — separates bar from world.
	r.DrawColorRect(0, m.Y, screenW, 1, mgl32.Vec4{0.35, 0.50, 0.80, 0.60})
	// Vertical separators between adjacent buttons.
	sepColor := mgl32.Vec4{0.35, 0.50, 0.80, 0.55}
	for i, btn := range m.Buttons {
		if i > 0 {
			sepX := btn.X - 4
			r.DrawColorRect(sepX, m.Y+4, 2, m.H-8, sepColor)
		}
	}
	for _, btn := range m.Buttons {
		btn.Draw(r)
	}
	for _, btn := range m.RightButtons {
		btn.Draw(r)
	}
}

// ContainsY returns true if the given Y coordinate is within the menubar.
func (m *MenuBar) ContainsY(y float32) bool {
	return y >= m.Y && y <= m.Y+m.H
}
