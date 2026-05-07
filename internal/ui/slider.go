package ui

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
)

// VSlider is a vertical-track slider with a draggable thumb. The track grows
// downward; the value increases from bottom to top so dragging up means a
// bigger value (matches the visual cue of "more"). Caller reads/writes Value.
type VSlider struct {
	X, Y, W, H float32
	Min, Max   float32
	Value      float32
	Label      string // shown above the track; %.0f formatted with current value

	dragging bool
}

// NewVSlider creates a slider sized as a vertical strip; W is the track
// width and H is the full track height.
func NewVSlider(x, y, w, h, min, max, value float32, label string) *VSlider {
	return &VSlider{X: x, Y: y, W: w, H: h, Min: min, Max: max, Value: value, Label: label}
}

// Contains is true when (mx, my) is inside the slider's clickable bounds —
// the track plus a little vertical padding so the thumb at the extremes is
// still grabbable. Used by the parent scene to suppress conflicting input.
func (s *VSlider) Contains(mx, my float32) bool {
	const pad = 8
	return mx >= s.X-pad && mx <= s.X+s.W+pad && my >= s.Y-pad && my <= s.Y+s.H+pad
}

// HandleInput updates Value based on mouse interaction. Pass leftClick (just
// pressed this frame) and leftHeld (button currently down). Returns true
// when the slider is actively grabbing input — the caller should treat that
// as click-consumed.
func (s *VSlider) HandleInput(mx, my float32, leftClick, leftHeld bool) bool {
	if leftClick && s.Contains(mx, my) {
		s.dragging = true
	}
	if !leftHeld {
		s.dragging = false
	}
	if s.dragging {
		// Top of track = Max, bottom = Min. Clamp before mapping so dragging
		// past the ends doesn't roll past the limits.
		t := (s.Y + s.H - my) / s.H
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		s.Value = s.Min + t*(s.Max-s.Min)
		return true
	}
	return false
}

// Draw renders track, thumb, and value label.
func (s *VSlider) Draw(r *render.Renderer) {
	// Track background.
	r.DrawColorRect(s.X, s.Y, s.W, s.H, mgl32.Vec4{0.12, 0.16, 0.24, 0.95})
	// Filled portion below the thumb so users see "how much".
	t := (s.Value - s.Min) / (s.Max - s.Min)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	fillH := s.H * t
	r.DrawColorRect(s.X, s.Y+s.H-fillH, s.W, fillH, mgl32.Vec4{0.25, 0.55, 0.35, 0.95})

	// Thumb.
	const thumbH float32 = 10
	thumbY := s.Y + s.H - fillH - thumbH/2
	r.DrawColorRect(s.X-3, thumbY, s.W+6, thumbH, mgl32.Vec4{0.05, 0.07, 0.12, 1})

	if r.Font == nil {
		return
	}
	// Label above the track.
	if s.Label != "" {
		labelW := float32(len(s.Label) * render.GlyphAdvance)
		lx := s.X + (s.W-labelW)/2
		r.Font.DrawText(r, s.Label, lx, s.Y-float32(render.GlyphH)-4, mgl32.Vec4{1, 1, 1, 1})
	}
	// Numeric value below the track.
	val := fmt.Sprintf("%.0f", s.Value)
	valW := float32(len(val) * render.GlyphAdvance)
	vx := s.X + (s.W-valW)/2
	r.Font.DrawText(r, val, vx, s.Y+s.H+4, mgl32.Vec4{1, 1, 1, 1})
}
