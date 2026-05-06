package scene

import (
	"fmt"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/ui"
)

// StartMenu is the main menu scene.
type StartMenu struct {
	app     *engine.App
	buttons []*ui.Button
}

// NewStartMenu creates a StartMenu scene.
func NewStartMenu() *StartMenu {
	return &StartMenu{}
}

func (s *StartMenu) Init(app *engine.App) error {
	s.app = app

	type btnDef struct {
		label string
		fn    func()
	}
	defs := []btnDef{
		{"Start Game", func() {
			scen := NewScenario(s.app.AssetDir + "/scenarios/tutorial.json")
			s.app.PushScene(scen)
		}},
		{"Load Save", func() {
			if !save.SaveSlotExists() {
				fmt.Println("Load Save: no save file at", save.SaveSlotPath())
				return
			}
			s.app.PushScene(NewScenarioFromSave())
		}},
		{"Scenario Editor", func() {
			ed := NewEditor(s.app.AssetDir + "/scenarios/tutorial.json")
			s.app.PushScene(ed)
		}},
		{"Testbeds", func() {
			s.app.PushScene(NewTestbedMenu())
		}},
		{"Settings", func() {
			fmt.Println("Settings: not yet implemented")
		}},
		{"Exit", func() {
			s.app.Window.SetShouldClose(true)
		}},
	}

	s.buttons = make([]*ui.Button, 0, len(defs))
	for _, d := range defs {
		btn := ui.NewButton(0, 0, 200, 40, d.label, d.fn)
		btn.Color = mgl32.Vec4{0.15, 0.25, 0.45, 0.95}
		btn.HoverColor = mgl32.Vec4{0.25, 0.45, 0.75, 0.95}
		s.buttons = append(s.buttons, btn)
	}
	return nil
}

// layout recomputes button positions relative to the current screen size.
// Called at the top of Update so both hit-testing and rendering use the same coords.
func (s *StartMenu) layout() {
	sw := float32(s.app.Renderer.ScreenWidth())
	sh := float32(s.app.Renderer.ScreenHeight())

	const btnW, btnH, spacing = float32(200), float32(40), float32(12)
	n := float32(len(s.buttons))
	totalH := n*btnH + (n-1)*spacing
	// Centre the button cluster, pushed slightly below mid to leave room for title.
	startY := (sh-totalH)/2 + 60

	for i, btn := range s.buttons {
		btn.X = (sw - btnW) / 2
		btn.Y = startY + float32(i)*(btnH+spacing)
		btn.W = btnW
		btn.H = btnH
	}
}

func (s *StartMenu) Update(dt float64) {
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

func (s *StartMenu) Render(r *render.Renderer) {
	gl.ClearColor(0.635, 0.682, 0.918, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())

	drawables := make([]render.UIDrawable, 0, len(s.buttons)+1)
	drawables = append(drawables, &titleDrawable{
		x: sw/2 - 100,
		y: sh/2 - 120,
		w: 200,
		h: 40,
	})
	for _, btn := range s.buttons {
		drawables = append(drawables, btn)
	}
	r.DrawUI(drawables)
}

func (s *StartMenu) Destroy() {}

// titleDrawable renders the game title.
type titleDrawable struct {
	x, y, w, h float32
}

func (t *titleDrawable) Draw(r *render.Renderer) {
	r.DrawColorRect(t.x-10, t.y-5, t.w+20, t.h+10, mgl32.Vec4{0.1, 0.3, 0.6, 0.8})
	if r.Font != nil {
		r.Font.DrawText(r, "Mountain Mogul", t.x, t.y+8, mgl32.Vec4{1.0, 0.95, 0.8, 1.0})
	}
}
