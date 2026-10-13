package ui

import (
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

const (
	submenuChildH   = float32(48)
	submenuChildPad = float32(4)

	// trayTileW and trayTileH size a dock submenu's picture tiles; the
	// caption sits under the picture.
	trayTileW = float32(96)
	trayTileH = float32(104)
	trayGap   = float32(6)
	trayPad   = float32(8)
)

// SubmenuButton is a toolbar entry that opens a popup of child buttons
// above the toolbar when clicked: a vertical list, or in a dock a row of
// picture tiles. The popup closes when a child is selected or when the
// user clicks outside.
type SubmenuButton struct {
	Btn      *Button
	Children []*Button
	Open     bool

	tray bool // lay out as a dock's picture row (set by MenuBar)
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

// popupRect returns the bounding rect of the popup panel: a column above
// the button, or a dock's row centred over it and kept on screen.
func (s *SubmenuButton) popupRect(screenW float32) (x, y, w, h float32) {
	n := float32(len(s.Children))
	if s.tray {
		w = n*trayTileW + (n-1)*trayGap + trayPad*2
		h = trayTileH + trayPad*2
		x = s.Btn.X + s.Btn.W/2 - w/2
		x = max(8, min(x, screenW-w-8))
		return x, s.Btn.Y - h - 14, w, h
	}
	const tileW = float32(84)
	h = n*submenuChildH + submenuChildPad*2
	return s.Btn.X, s.Btn.Y - h, tileW, h
}

// layoutPopup positions the children inside the popup. Must be called
// after the parent button has been positioned by MenuBar.layout.
func (s *SubmenuButton) layoutPopup(screenW float32) {
	px, py, _, _ := s.popupRect(screenW)
	for i, ch := range s.Children {
		if s.tray {
			ch.X = px + trayPad + float32(i)*(trayTileW+trayGap)
			ch.Y = py + trayPad
			ch.W, ch.H = trayTileW, trayTileH
			continue
		}
		const tileW = float32(84)
		ch.X = px + submenuChildPad
		ch.Y = py + submenuChildPad + float32(i)*submenuChildH
		ch.W = tileW - submenuChildPad*2
		ch.H = submenuChildH - 2
	}
}

// HandlePopupInput handles mouse input for the open popup. Returns true if the
// input was consumed (child click or click inside popup area).
func (s *SubmenuButton) HandlePopupInput(input *engine.Input, screenW float32) bool {
	if !s.Open || len(s.Children) == 0 {
		return false
	}
	s.layoutPopup(screenW)
	mx := input.MousePos[0]
	my := input.MousePos[1]
	px, py, pw, ph := s.popupRect(screenW)

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
func (s *SubmenuButton) DrawPopup(r *render.Renderer, screenW float32) {
	if !s.Open || len(s.Children) == 0 {
		return
	}
	s.layoutPopup(screenW)
	px, py, pw, ph := s.popupRect(screenW)
	DrawPanel(r, px, py, pw, ph)
	if s.tray {
		// A small pointer from the tray down to its dock button.
		cx := s.Btn.X + s.Btn.W/2
		r.DrawColorDiamond(cx, py+ph, 7, PanelFill)
	}
	for _, ch := range s.Children {
		ch.Draw(r)
	}
}
