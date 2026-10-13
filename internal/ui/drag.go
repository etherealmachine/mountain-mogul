package ui

import "mountain-mogul/internal/engine"

// Panels move by their title bars: press on the bar and drag. Only one
// moves at a time, and while it does the scene keeps the held mouse to
// itself (Dragging), so a drag that runs off the panel doesn't paint,
// glade or pan underneath.

// dragging is whether any panel is being dragged.
var dragging bool

// Dragging reports whether a panel is being dragged by its title bar.
func Dragging() bool { return dragging }

// TitleDrag moves one panel by its title bar.
type TitleDrag struct {
	on     bool
	dx, dy float32 // the grab point, from the panel's corner
}

// Handle runs a frame of dragging the panel at (*x, *y), w wide, whose
// title bar is titleH tall, within a screen sw × sh. avoid reports points
// on the bar that aren't for grabbing (its close button). It reports
// whether the panel moved, and whether a drag just ended.
func (d *TitleDrag) Handle(inp *engine.Input, x, y *float32, w, titleH, sw, sh float32, avoid func(mx, my float32) bool) (moved, ended bool) {
	mx, my := inp.MousePos[0], inp.MousePos[1]
	if d.on {
		if !inp.LeftHeld {
			d.on, dragging = false, false
			return false, true
		}
		nx := max(0, min(mx-d.dx, sw-w))
		ny := max(0, min(my-d.dy, sh-titleH))
		if nx != *x || ny != *y {
			*x, *y = nx, ny
			moved = true
		}
		return moved, false
	}
	if inp.LeftClick && !inp.LeftClickConsumed && !dragging &&
		mx >= *x && mx <= *x+w && my >= *y && my <= *y+titleH && (avoid == nil || !avoid(mx, my)) {
		d.on, dragging = true, true
		d.dx, d.dy = mx-*x, my-*y
		inp.LeftClickConsumed = true
	}
	return false, false
}

// Active reports whether this panel is being dragged.
func (d *TitleDrag) Active() bool { return d.on }
