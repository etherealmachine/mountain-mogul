package ui

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
)

// Button is a clickable rectangular UI element. Icon is optional — when
// non-empty the icon is drawn above the label; otherwise the label sits
// vertically centred and left-padded as before.
type Button struct {
	X, Y, W, H  float32
	Label        string
	Icon         render.IconName // optional; empty means text-only
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

	white := mgl32.Vec4{1, 1, 1, 1}
	if b.Icon != "" {
		// Icon button — icon centred horizontally, label centred under it.
		// Sized so we get a comfortable 24-px icon with a 4-px gap to the
		// text below in the typical 60-px tall menu bar.
		iconSize := float32(24)
		if b.H < iconSize+float32(render.GlyphH)+8 {
			iconSize = b.H - float32(render.GlyphH) - 8
			if iconSize < 12 {
				iconSize = 12
			}
		}
		iconX := b.X + (b.W-iconSize)/2
		iconY := b.Y + 4
		r.DrawIcon(b.Icon, iconX, iconY, iconSize, white)
		if r.Font != nil && b.Label != "" {
			labelW := float32(len(b.Label) * render.GlyphAdvance)
			labelX := b.X + (b.W-labelW)/2
			labelY := iconY + iconSize + 4
			r.Font.DrawText(r, b.Label, labelX, labelY, white)
		}
		return
	}

	if r.Font != nil {
		textX := b.X + 6
		textY := b.Y + (b.H-float32(render.GlyphH))/2
		r.Font.DrawText(r, b.Label, textX, textY, white)
	}
}
