package scene

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
)

// ScenarioPicker lists every starter scenario in assets/scenarios/ so the
// user can pick which one a New Game begins from. Models on TestbedMenu's
// centred-button-stack pattern; Back returns to the start menu.
type ScenarioPicker struct {
	app     *engine.App
	buttons []*ui.Button
}

func NewScenarioPicker() *ScenarioPicker { return &ScenarioPicker{} }

func (s *ScenarioPicker) Init(app *engine.App) error {
	s.app = app
	dir := filepath.Join(app.AssetDir, "scenarios")
	entries, err := os.ReadDir(dir)
	if err == nil {
		files := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			files = append(files, e.Name())
		}
		sort.Strings(files)
		for _, name := range files {
			path := filepath.Join(dir, name)
			label := strings.TrimSuffix(name, ".json")
			btn := ui.NewButton(0, 0, 280, 40, label, func() {
				s.app.ReplaceScene(NewScenarioFromFile(path))
			})
			btn.Color = mgl32.Vec4{0.15, 0.25, 0.45, 0.95}
			btn.HoverColor = mgl32.Vec4{0.25, 0.45, 0.75, 0.95}
			s.buttons = append(s.buttons, btn)
		}
	}
	back := ui.NewButton(0, 0, 280, 40, "Back", func() { s.app.PopScene() })
	back.Color = mgl32.Vec4{0.4, 0.25, 0.25, 0.95}
	back.HoverColor = mgl32.Vec4{0.6, 0.4, 0.4, 0.95}
	s.buttons = append(s.buttons, back)
	return nil
}

func (s *ScenarioPicker) layout() {
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

func (s *ScenarioPicker) Update(dt float64) {
	s.layout()
	inp := s.app.Input
	mx, my := inp.MousePos[0], inp.MousePos[1]
	for _, btn := range s.buttons {
		btn.SetHovered(btn.Contains(mx, my))
		if inp.LeftClick && btn.Contains(mx, my) {
			btn.Click()
			return
		}
	}
}

func (s *ScenarioPicker) Render(r *render.Renderer) {
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

func (s *ScenarioPicker) Destroy() {}
