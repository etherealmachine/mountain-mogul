package scene

import (
	"math/rand"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/ui"
)

// TestbedMenu lists registered sim.Testbeds and launches one as a Scenario
// using the testbed's recommended seed for deterministic playback.
type TestbedMenu struct {
	app     *engine.App
	buttons []*ui.Button
}

func NewTestbedMenu() *TestbedMenu { return &TestbedMenu{} }

func (s *TestbedMenu) Init(app *engine.App) error {
	s.app = app
	for _, tb := range sim.Testbeds {
		tb := tb
		btn := ui.NewButton(0, 0, 280, 40, tb.Name, func() {
			w := tb.Build(rand.New(rand.NewSource(tb.Seed)))
			s.app.PushScene(NewScenarioFromWorld(w, tb.Seed))
		})
		btn.Color = mgl32.Vec4{0.15, 0.25, 0.45, 0.95}
		btn.HoverColor = mgl32.Vec4{0.25, 0.45, 0.75, 0.95}
		s.buttons = append(s.buttons, btn)
	}
	back := ui.NewButton(0, 0, 280, 40, "Back", func() {
		s.app.PopScene()
	})
	back.Color = mgl32.Vec4{0.4, 0.25, 0.25, 0.95}
	back.HoverColor = mgl32.Vec4{0.6, 0.4, 0.4, 0.95}
	s.buttons = append(s.buttons, back)
	return nil
}

func (s *TestbedMenu) layout() {
	sw := float32(s.app.Renderer.ScreenWidth())
	sh := float32(s.app.Renderer.ScreenHeight())
	const btnW, btnH, spacing = float32(280), float32(40), float32(12)
	n := float32(len(s.buttons))
	totalH := n*btnH + (n-1)*spacing
	startY := (sh - totalH) / 2
	for i, btn := range s.buttons {
		btn.X = (sw - btnW) / 2
		btn.Y = startY + float32(i)*(btnH+spacing)
		btn.W = btnW
		btn.H = btnH
	}
}

func (s *TestbedMenu) Update(dt float64) {
	s.layout()
	inp := s.app.Input
	mx := inp.MousePos[0]
	my := inp.MousePos[1]
	for _, btn := range s.buttons {
		btn.SetHovered(btn.Contains(mx, my))
		if inp.LeftClick && btn.Contains(mx, my) {
			btn.Click()
		}
	}
}

func (s *TestbedMenu) Render(r *render.Renderer) {
	gl.ClearColor(0.635, 0.682, 0.918, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	drawables := make([]render.UIDrawable, 0, len(s.buttons)+1)
	drawables = append(drawables, &titleDrawable{
		x: sw/2 - 100,
		y: sh/2 - 200,
		w: 200,
		h: 40,
	})
	for _, btn := range s.buttons {
		drawables = append(drawables, btn)
	}
	r.DrawUI(drawables)
}

func (s *TestbedMenu) Destroy() {}
