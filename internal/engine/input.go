package engine

import (
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

// Input holds the per-frame input snapshot.
type Input struct {
	Pressed     map[glfw.Key]bool
	Held        map[glfw.Key]bool
	Released    map[glfw.Key]bool
	MousePos    mgl32.Vec2
	MouseDelta  mgl32.Vec2
	LeftClick    bool
	LeftHeld     bool
	LeftRelease  bool
	RightClick   bool
	RightHeld    bool
	RightRelease bool
	ScrollDelta  float32
	CharInput   []rune // Unicode characters typed this frame

	// internal state for building deltas
	prevMousePos mgl32.Vec2
	scrollAcc    float32
}

// NewInput creates an Input and registers GLFW callbacks on the window.
func NewInput(w *glfw.Window) *Input {
	in := &Input{
		Pressed:  make(map[glfw.Key]bool),
		Held:     make(map[glfw.Key]bool),
		Released: make(map[glfw.Key]bool),
	}

	w.SetKeyCallback(func(_ *glfw.Window, key glfw.Key, _ int, action glfw.Action, _ glfw.ModifierKey) {
		switch action {
		case glfw.Press:
			in.Pressed[key] = true
			in.Held[key] = true
		case glfw.Release:
			delete(in.Held, key)
			in.Released[key] = true
		}
	})

	w.SetMouseButtonCallback(func(_ *glfw.Window, button glfw.MouseButton, action glfw.Action, _ glfw.ModifierKey) {
		switch button {
		case glfw.MouseButtonLeft:
			if action == glfw.Press {
				in.LeftClick = true
				in.LeftHeld = true
			} else if action == glfw.Release {
				in.LeftHeld = false
				in.LeftRelease = true
			}
		case glfw.MouseButtonRight:
			if action == glfw.Press {
				in.RightClick = true
				in.RightHeld = true
			} else if action == glfw.Release {
				in.RightHeld = false
				in.RightRelease = true
			}
		}
	})

	w.SetCursorPosCallback(func(_ *glfw.Window, x, y float64) {
		in.MousePos = mgl32.Vec2{float32(x), float32(y)}
	})

	w.SetScrollCallback(func(_ *glfw.Window, _, yOff float64) {
		in.scrollAcc += float32(yOff)
	})

	w.SetCharCallback(func(_ *glfw.Window, char rune) {
		in.CharInput = append(in.CharInput, char)
	})

	return in
}

// BeginFrame clears transient fields and computes deltas.
func (in *Input) BeginFrame() {
	// Clear per-frame transients
	for k := range in.Pressed {
		delete(in.Pressed, k)
	}
	for k := range in.Released {
		delete(in.Released, k)
	}
	in.LeftClick = false
	in.LeftRelease = false
	in.RightClick = false
	in.RightRelease = false
	in.CharInput = in.CharInput[:0]

	// Mouse delta
	in.MouseDelta = in.MousePos.Sub(in.prevMousePos)
	in.prevMousePos = in.MousePos

	// Scroll
	in.ScrollDelta = in.scrollAcc
	in.scrollAcc = 0
}
