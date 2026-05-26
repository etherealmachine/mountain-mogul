package scene

import (
	"fmt"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

// introHoldSeconds is how long the splash sticks before auto-advancing
// to the start menu. Tuned to read the logo + studio name without
// stalling the player on every launch; click / any-key dismisses early.
const introHoldSeconds = 2.5

// IntroScene is the studio splash shown at app start. Displays the
// Minty Fresh logo and name centred, fades through, and replaces
// itself with the StartMenu on click, key press, or after
// introHoldSeconds.
type IntroScene struct {
	app    *engine.App
	logoID uint32 // 0 until the PNG loads; we fall back to a colour rect
	logoW  float32
	logoH  float32
	t      float32 // seconds since Init
	done   bool
}

// NewIntroScene constructs the splash scene. Asset loading happens in
// Init so the renderer is available for the texture upload.
func NewIntroScene() *IntroScene { return &IntroScene{} }

func (s *IntroScene) Init(app *engine.App) error {
	s.app = app
	// Load the rasterised logo PNG. Errors fall back to texID=0; Render
	// detects that and draws a labelled colour box so a missing asset
	// doesn't crash launch.
	texID, err := render.LoadTexture(app.AssetDir + "/logo.png")
	if err != nil {
		fmt.Printf("IntroScene: logo.png missing (%v); rendering placeholder\n", err)
	}
	s.logoID = texID
	// The logo's source SVG is 240×320; preserve that aspect ratio.
	s.logoW = 240
	s.logoH = 320
	return nil
}

func (s *IntroScene) Update(dt float64) {
	if s.done {
		return
	}
	s.t += float32(dt)

	inp := s.app.Input
	dismiss := inp.LeftClick || inp.RightClick || len(inp.Pressed) > 0 || s.t >= introHoldSeconds
	if dismiss {
		s.done = true
		s.app.ReplaceScene(NewStartMenu())
	}
}

func (s *IntroScene) Render(r *render.Renderer) {
	// Same cool blue background as the start menu so the transition
	// reads as one continuous frame even without a fade.
	gl.ClearColor(0.635, 0.682, 0.918, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())

	// Fit the logo into a comfortable portion of the screen (60% of
	// the smaller axis). Preserves aspect ratio.
	targetH := sh * 0.6
	scale := targetH / s.logoH
	if w := sw * 0.6; s.logoW*scale > w {
		scale = w / s.logoW
	}
	lw := s.logoW * scale
	lh := s.logoH * scale
	lx := (sw - lw) / 2
	ly := (sh-lh)/2 - 30 // shift up a touch to leave room for the name

	// Soft fade-in over the first 0.4 s.
	alpha := s.t / 0.4
	if alpha > 1 {
		alpha = 1
	}

	drawables := []render.UIDrawable{
		&introLogo{x: lx, y: ly, w: lw, h: lh, texID: s.logoID, alpha: alpha},
		&introCaption{cx: sw / 2, y: ly + lh + 24, alpha: alpha},
	}
	r.DrawUI(drawables)
}

func (s *IntroScene) Destroy() {
	// The logo texture leaks here — the engine doesn't expose a
	// gl.DeleteTextures wrapper and the intro is one-shot per app
	// session anyway, so the extra GPU memory dies with the process.
}

// introLogo renders the studio logo PNG, or a placeholder rect when
// the texture failed to load.
type introLogo struct {
	x, y, w, h float32
	texID      uint32
	alpha      float32
}

func (l *introLogo) Draw(r *render.Renderer) {
	if l.texID != 0 {
		r.DrawTexturedRect(l.x, l.y, l.w, l.h, l.texID, mgl32.Vec4{1, 1, 1, l.alpha})
		return
	}
	// Missing-asset placeholder so the scene still reads as "studio splash".
	r.DrawColorRect(l.x, l.y, l.w, l.h, mgl32.Vec4{0.4, 0.7, 0.5, 0.6 * l.alpha})
}

// introCaption renders the "Minty Fresh" wordmark below the logo.
type introCaption struct {
	cx, y, alpha float32
}

func (c *introCaption) Draw(r *render.Renderer) {
	if r.Font == nil {
		return
	}
	const text = "Minty Fresh"
	w := r.Font.TextWidth(text)
	x := c.cx - w/2
	r.Font.DrawText(r, text, x, c.y, mgl32.Vec4{0.15, 0.35, 0.25, c.alpha})
}
