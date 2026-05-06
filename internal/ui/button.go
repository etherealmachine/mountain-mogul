package ui

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
)

// Button is a clickable rectangular UI element.
type Button struct {
	X, Y, W, H  float32
	Label        string
	Color        mgl32.Vec4
	HoverColor   mgl32.Vec4
	ActiveColor  mgl32.Vec4
	active       bool
	hovered      bool
	onClick      func()
}

// NewButton creates a button with default colors.
func NewButton(x, y, w, h float32, label string, onClick func()) *Button {
	return &Button{
		X:           x,
		Y:           y,
		W:           w,
		H:           h,
		Label:       label,
		Color:       mgl32.Vec4{0.2, 0.3, 0.5, 0.9},
		HoverColor:  mgl32.Vec4{0.3, 0.5, 0.8, 0.9},
		ActiveColor: mgl32.Vec4{0.15, 0.50, 0.28, 1.0},
		onClick:     onClick,
	}
}

// Contains returns true if the given screen coordinate is inside the button.
func (b *Button) Contains(mx, my float32) bool {
	return mx >= b.X && mx <= b.X+b.W && my >= b.Y && my <= b.Y+b.H
}

// SetHovered updates the hover state.
func (b *Button) SetHovered(v bool) { b.hovered = v }

// SetActive marks the button as toggled on or off.
func (b *Button) SetActive(v bool) { b.active = v }

// Click triggers the button's onClick handler.
func (b *Button) Click() {
	if b.onClick != nil {
		b.onClick()
	}
}

// Draw renders the button using the UI shader.
func (b *Button) Draw(r *render.Renderer) {
	var color mgl32.Vec4
	switch {
	case b.active:
		color = b.ActiveColor
	case b.hovered:
		color = b.HoverColor
	default:
		color = b.Color
	}
	r.DrawColorRect(b.X, b.Y, b.W, b.H, color)

	if r.Font != nil {
		textX := b.X + 6
		textY := b.Y + (b.H-float32(render.GlyphH))/2
		r.Font.DrawText(r, b.Label, textX, textY, mgl32.Vec4{1, 1, 1, 1})
	}
}
