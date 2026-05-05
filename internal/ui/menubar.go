package ui

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

// MenuBar is a horizontal toolbar across the top of the screen.
type MenuBar struct {
	Y, H    float32
	Buttons []*Button

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

// AddButton appends a button to the menu bar and returns it.
func (m *MenuBar) AddButton(label string, onClick func()) *Button {
	const padding = float32(4)
	x := float32(0)
	for _, b := range m.Buttons {
		x = b.X + b.W + padding
	}
	w := float32(len(label)*8) + 16
	btn := NewButton(x, m.Y+padding, w, m.H-padding*2, label, onClick)
	m.Buttons = append(m.Buttons, btn)
	return btn
}

// HandleInput processes mouse input for the menu bar.
func (m *MenuBar) HandleInput(input *engine.Input, screenH float32) {
	mx := input.MousePos[0]
	my := input.MousePos[1]
	for _, btn := range m.Buttons {
		btn.SetHovered(btn.Contains(mx, my))
		if input.LeftClick && btn.Contains(mx, my) {
			btn.Click()
		}
	}
}

// Draw renders the menu bar background and all buttons.
func (m *MenuBar) Draw(r *render.Renderer) {
	// Draw full-width background
	r.DrawColorRect(0, m.Y, float32(r.ScreenWidth()), m.H, m.bgColor)
	for _, btn := range m.Buttons {
		btn.Draw(r)
	}
}

// ContainsY returns true if the given Y coordinate is within the menubar.
func (m *MenuBar) ContainsY(y float32) bool {
	return y >= m.Y && y <= m.Y+m.H
}
