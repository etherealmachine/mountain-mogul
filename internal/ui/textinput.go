package ui

import (
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

// TextInput is a single-line editable text field. It consumes app.Input
// (CharInput, Pressed) directly so callers don't have to plumb characters
// through. Submit and cancel callbacks let the parent close the modal that
// hosts the field.
type TextInput struct {
	X, Y, W, H float32
	Text       string
	MaxLen     int

	OnSubmit func(text string)
	OnCancel func()

	// blink animates the cursor; ticks once per call to HandleInput so it
	// blinks regardless of the underlying scene's frame rate.
	blink int
}

// NewTextInput creates an input field with the given initial text.
func NewTextInput(x, y, w, h float32, initial string) *TextInput {
	return &TextInput{
		X:      x,
		Y:      y,
		W:      w,
		H:      h,
		Text:   initial,
		MaxLen: 64,
	}
}

// HandleInput consumes typed characters, backspace, enter, and escape from
// the engine input snapshot. Call once per frame while the field is focused.
func (t *TextInput) HandleInput(in *engine.Input) {
	t.blink++
	mx, my := in.MousePos[0], in.MousePos[1]
	if in.LeftClick && mx >= t.X && mx <= t.X+t.W && my >= t.Y && my <= t.Y+t.H {
		in.LeftClickConsumed = true
	}
	for _, r := range in.CharInput {
		if r < 0x20 || r == 0x7f { // skip control chars; printable ASCII+Unicode only
			continue
		}
		if len(t.Text) >= t.MaxLen {
			break
		}
		t.Text += string(r)
	}
	if in.Pressed[glfw.KeyBackspace] && len(t.Text) > 0 {
		// Trim one rune off the end (handles multi-byte UTF-8 correctly).
		runes := []rune(t.Text)
		t.Text = string(runes[:len(runes)-1])
	}
	if in.Pressed[glfw.KeyEnter] || in.Pressed[glfw.KeyKPEnter] {
		if t.OnSubmit != nil {
			t.OnSubmit(t.Text)
		}
	}
	if in.Pressed[glfw.KeyEscape] && t.OnCancel != nil {
		t.OnCancel()
	}
}

// Draw renders the field background, current text, and a blinking cursor.
func (t *TextInput) Draw(r *render.Renderer) {
	r.DrawColorRect(t.X, t.Y, t.W, t.H, mgl32.Vec4{0.1, 0.12, 0.18, 1.0})
	// 1-pixel-ish border via a slightly inset darker rect.
	r.DrawColorRect(t.X+1, t.Y+1, t.W-2, t.H-2, mgl32.Vec4{0.18, 0.22, 0.30, 1.0})

	if r.Font == nil {
		return
	}
	textX := t.X + 8
	textY := t.Y + (t.H-float32(render.GlyphH))/2
	r.Font.DrawText(r, t.Text, textX, textY, mgl32.Vec4{1, 1, 1, 1})

	// Blinking cursor: visible roughly half the time at ~1 Hz @ 60 fps.
	if (t.blink/30)%2 == 0 {
		cursorX := textX + r.Font.TextWidth(t.Text)
		r.DrawColorRect(cursorX, textY, 2, float32(render.GlyphH), mgl32.Vec4{1, 1, 1, 1})
	}
}
