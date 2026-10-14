package scene

import (
	"fmt"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
)

// The studio splash: the Minty Fresh logo rises into place on a soft mint
// background, the wordmark fades in under it, and after a moment the
// screen fades to the start menu's sky, which the start menu fades in
// from. A click or any key skips straight to the fade.

const (
	introHoldSeconds = 2.4  // until the fade out starts
	introFadeOut     = 0.45 // seconds fading to the start menu's sky
	introRise        = 0.7  // seconds the logo takes to settle
)

// introSky is the colour the splash fades to and the start menu fades in
// from: the top of the start menu's sky (drawMenuSky).
var introSky = mgl32.Vec4{0.16, 0.27, 0.52, 1}

// IntroScene is the studio splash shown at app start.
type IntroScene struct {
	app     *engine.App
	logoID  uint32 // 0 until the PNG loads; we fall back to a colour rect
	t       float32
	leaving float32 // seconds since the fade out started; <0 before it
	done    bool
}

// The logo's source SVG is 240×320 (assets/logo.svg); its PNG is drawn
// at that aspect.
const introLogoAspect = float32(240) / 320

// NewIntroScene constructs the splash scene. Asset loading happens in
// Init so the renderer is available for the texture upload.
func NewIntroScene() *IntroScene { return &IntroScene{leaving: -1} }

func (s *IntroScene) Init(app *engine.App) error {
	s.app = app
	texID, err := render.LoadTexture(app.AssetDir + "/logo.png")
	if err != nil {
		fmt.Printf("IntroScene: logo.png missing (%v); rendering placeholder\n", err)
	}
	s.logoID = texID
	startMenuBackdropLoad(app.AssetDir)
	return nil
}

func (s *IntroScene) Update(dt float64) {
	if s.done {
		return
	}
	s.t += float32(dt)
	inp := s.app.Input
	if s.leaving < 0 && (inp.LeftClick || inp.RightClick || len(inp.Pressed) > 0 || s.t >= introHoldSeconds) {
		s.leaving = 0
	}
	if s.leaving >= 0 {
		s.leaving += float32(dt)
		if s.leaving >= introFadeOut {
			s.done = true
			s.app.ReplaceScene(newStartMenuFadingIn())
		}
	}
}

func (s *IntroScene) Render(r *render.Renderer) {
	gl.ClearColor(0.92, 0.97, 0.94, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	r.DrawUI([]render.UIDrawable{uiDrawFunc(s.draw)})
}

// easeOut is a cubic ease-out of t in 0..1.
func easeOut(t float32) float32 {
	t = min(max(t, 0), 1)
	k := 1 - t
	return 1 - k*k*k
}

func (s *IntroScene) draw(r *render.Renderer) {
	sw, sh := float32(r.ScreenWidth()), float32(r.ScreenHeight())

	// Mint sky: pale at the top, deeper toward the bottom.
	r.DrawGradientRect(0, 0, sw, sh, mgl32.Vec4{0.93, 0.98, 0.95, 1}, mgl32.Vec4{0.70, 0.88, 0.78, 1})

	// The logo: up to 55% of the screen's height, rising into place.
	in := easeOut(s.t / introRise)
	lh := min(sh*0.55, sw*0.6/introLogoAspect)
	lw := lh * introLogoAspect
	cx, cy := sw/2, sh*0.43
	scale := 0.9 + 0.1*in
	w, h := lw*scale, lh*scale
	y := cy - h/2 + (1-in)*40

	// A soft glow behind it.
	r.DrawRadialGlow(cx, cy, lh*1.1, mgl32.Vec4{1, 1, 1, 0.35 * in})
	if s.logoID != 0 {
		r.DrawTexturedRect(cx-w/2, y, w, h, s.logoID, mgl32.Vec4{1, 1, 1, in})
	} else {
		r.DrawColorRect(cx-w/2, y, w, h, mgl32.Vec4{0.4, 0.7, 0.5, 0.6 * in})
	}

	// The wordmark, fading in after the logo.
	f := r.TitleFont
	if f == nil {
		f = r.Font
	}
	if f != nil {
		const name = "Minty Fresh"
		a := easeOut((s.t - 0.35) / 0.6)
		tw := f.TextWidth(name)
		ty := cy + lh/2 + 18 + (1-a)*12
		f.DrawText(r, name, cx-tw/2+2, ty+3, mgl32.Vec4{0.05, 0.25, 0.15, 0.18 * a})
		f.DrawText(r, name, cx-tw/2, ty, mgl32.Vec4{0.12, 0.36, 0.22, a})
	}

	// Fading out to the start menu's sky.
	if s.leaving >= 0 {
		c := introSky
		c[3] = easeOut(s.leaving / introFadeOut)
		r.DrawColorRect(0, 0, sw, sh, c)
	}
}

func (s *IntroScene) Destroy() {
	// The logo texture leaks here — the engine doesn't expose a
	// gl.DeleteTextures wrapper and the intro is one-shot per app
	// session anyway, so the extra GPU memory dies with the process.
}
