package ui

import (
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"

	"github.com/go-gl/mathgl/mgl32"
)

const (
	submenuChildH   = float32(48)
	submenuChildPad = float32(4)
)

// SubmenuButton is a toolbar entry that opens a vertical popup of child buttons
// above the toolbar when clicked. The popup closes when a child is selected or
// when the user clicks outside.
type SubmenuButton struct {
	Btn      *Button
	Children []*Button
	Open     bool
}

// AddChild appends a child button to the submenu popup and returns it so the
// caller can store the pointer for active-state updates.
func (s *SubmenuButton) AddChild(icon render.IconName, label string, onClick func()) *Button {
	const tileW = float32(84)
	btn := NewButton(0, 0, tileW-submenuChildPad*2, submenuChildH-2, label, onClick)
	btn.Icon = icon
	s.Children = append(s.Children, btn)
	return btn
}

// HasActiveChild returns true if any child is in the active (selected) state.
func (s *SubmenuButton) HasActiveChild() bool {
	for _, ch := range s.Children {
		if ch.active {
			return true
		}
	}
	return false
}

// layoutPopup positions children in a vertical stack directly above Btn.
// Must be called after the parent button has been positioned by MenuBar.layout.
func (s *SubmenuButton) layoutPopup() {
	n := len(s.Children)
	if n == 0 {
		return
	}
	const tileW = float32(84)
	popupH := float32(n)*submenuChildH + submenuChildPad*2
	px := s.Btn.X
	py := s.Btn.Y - popupH
	for i, ch := range s.Children {
		ch.X = px + submenuChildPad
		ch.Y = py + submenuChildPad + float32(i)*submenuChildH
		ch.W = tileW - submenuChildPad*2
		ch.H = submenuChildH - 2
	}
}

// popupRect returns the bounding rect of the popup panel.
func (s *SubmenuButton) popupRect() (x, y, w, h float32) {
	const tileW = float32(84)
	n := len(s.Children)
	ph := float32(n)*submenuChildH + submenuChildPad*2
	return s.Btn.X, s.Btn.Y - ph, tileW, ph
}

// HandlePopupInput handles mouse input for the open popup. Returns true if the
// input was consumed (child click or click inside popup area).
// Must be called after layoutPopup (i.e., after the parent button is positioned).
func (s *SubmenuButton) HandlePopupInput(input *engine.Input) bool {
	if !s.Open || len(s.Children) == 0 {
		return false
	}
	s.layoutPopup()
	mx := input.MousePos[0]
	my := input.MousePos[1]
	px, py, pw, ph := s.popupRect()

	for _, ch := range s.Children {
		ch.SetHovered(ch.Contains(mx, my))
		if input.LeftClick && ch.Contains(mx, my) {
			input.LeftClickConsumed = true
			ch.Click()
			s.Open = false
			return true
		}
	}
	// Consume clicks inside the popup frame that didn't land on a child,
	// so they don't fall through to the terrain.
	if input.LeftClick && mx >= px && mx <= px+pw && my >= py && my <= py+ph {
		input.LeftClickConsumed = true
		return true
	}
	return false
}

// DrawPopup renders the popup panel and its child buttons on top of everything
// else. Must be called after the toolbar bar background has been drawn.
func (s *SubmenuButton) DrawPopup(r *render.Renderer) {
	if !s.Open || len(s.Children) == 0 {
		return
	}
	s.layoutPopup()
	px, py, pw, ph := s.popupRect()
	r.DrawColorRect(px, py, pw, ph, mgl32.Vec4{0.07, 0.09, 0.15, 0.97})
	r.DrawColorRect(px, py, pw, 1, mgl32.Vec4{0.35, 0.50, 0.80, 0.60})
	r.DrawColorRectOutline(px, py, pw, ph, mgl32.Vec4{0.35, 0.50, 0.80, 0.55})
	for _, ch := range s.Children {
		ch.Draw(r)
	}
}
