package ui

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

// MenuBar is a horizontal toolbar across the top of the screen.
type MenuBar struct {
	Y, H         float32
	Buttons      []*Button // left-aligned, packed left-to-right
	RightButtons []*Button // right-aligned, packed right-to-left from screen edge

	bgColor mgl32.Vec4
}

// NewMenuBar creates a menu bar at the given Y position and height.
func NewMenuBar(y, h float32) *MenuBar {
	return &MenuBar{
		Y:       y,
		H:       h,
		bgColor: mgl32.Vec4{0.1, 0.1, 0.15, 0.95},
	}
}

// AddButton appends a left-aligned button to the menu bar.
func (m *MenuBar) AddButton(label string, onClick func()) *Button {
	const padding = float32(6)
	x := float32(0)
	for _, b := range m.Buttons {
		x = b.X + b.W + padding
	}
	w := float32(len(label)*render.GlyphAdvance) + 20
	btn := NewButton(x, m.Y+padding, w, m.H-padding*2, label, onClick)
	m.Buttons = append(m.Buttons, btn)
	return btn
}

// AddRightButton appends a right-aligned button. Position is recomputed each
// frame from the current screen width so resize works automatically.
func (m *MenuBar) AddRightButton(label string, onClick func()) *Button {
	const padding = float32(6)
	w := float32(len(label)*render.GlyphAdvance) + 20
	btn := NewButton(0, m.Y+padding, w, m.H-padding*2, label, onClick)
	m.RightButtons = append(m.RightButtons, btn)
	return btn
}

// layoutRight anchors all RightButtons to the right edge of the screen, packed
// left-to-right in insertion order with `padding` between them.
func (m *MenuBar) layoutRight(screenW float32) {
	const padding = float32(6)
	x := screenW - padding
	for i := len(m.RightButtons) - 1; i >= 0; i-- {
		btn := m.RightButtons[i]
		x -= btn.W
		btn.X = x
		x -= padding
	}
}

// HandleInput processes mouse input for the menu bar.
func (m *MenuBar) HandleInput(input *engine.Input, screenW, screenH float32) {
	m.layoutRight(screenW)
	mx := input.MousePos[0]
	my := input.MousePos[1]
	for _, btn := range m.Buttons {
		btn.SetHovered(btn.Contains(mx, my))
		if input.LeftClick && btn.Contains(mx, my) {
			btn.Click()
		}
	}
	for _, btn := range m.RightButtons {
		btn.SetHovered(btn.Contains(mx, my))
		if input.LeftClick && btn.Contains(mx, my) {
			btn.Click()
		}
	}
}

// Draw renders the menu bar background and all buttons.
func (m *MenuBar) Draw(r *render.Renderer) {
	screenW := float32(r.ScreenWidth())
	m.layoutRight(screenW)
	r.DrawColorRect(0, m.Y, screenW, m.H, m.bgColor)
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
