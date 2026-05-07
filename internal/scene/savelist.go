package scene

import (
	"fmt"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/ui"
)

// SaveList lists every named save in ~/.mountain-mogul/saves/ newest-first.
// Click a row to load that save; Back returns to the start menu. Shows a
// "No saves yet" placeholder when the directory is empty.
type SaveList struct {
	app     *engine.App
	buttons []*ui.Button
	empty   bool
}

func NewSaveList() *SaveList { return &SaveList{} }

func (s *SaveList) Init(app *engine.App) error {
	s.app = app
	saves := save.ListSaves()
	s.empty = len(saves) == 0
	now := time.Now()
	for _, info := range saves {
		path := info.Path
		label := fmt.Sprintf("%s   %s", info.Name, relativeTime(now, info.ModTime))
		btn := ui.NewButton(0, 0, 360, 40, label, func() {
			s.app.ReplaceScene(NewScenarioFromFile(path))
		})
		btn.Color = mgl32.Vec4{0.15, 0.25, 0.45, 0.95}
		btn.HoverColor = mgl32.Vec4{0.25, 0.45, 0.75, 0.95}
		s.buttons = append(s.buttons, btn)
	}
	back := ui.NewButton(0, 0, 360, 40, "Back", func() { s.app.PopScene() })
	back.Color = mgl32.Vec4{0.4, 0.25, 0.25, 0.95}
	back.HoverColor = mgl32.Vec4{0.6, 0.4, 0.4, 0.95}
	s.buttons = append(s.buttons, back)
	return nil
}

func (s *SaveList) layout() {
	sw := float32(s.app.Renderer.ScreenWidth())
	sh := float32(s.app.Renderer.ScreenHeight())
	const btnW, btnH, spacing = float32(360), float32(40), float32(12)
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

func (s *SaveList) Update(dt float64) {
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

func (s *SaveList) Render(r *render.Renderer) {
	gl.ClearColor(0.635, 0.682, 0.918, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	drawables := make([]render.UIDrawable, 0, len(s.buttons)+2)
	drawables = append(drawables, &titleDrawable{
		x: sw/2 - 100,
		y: sh/2 - 220,
		w: 200,
		h: 40,
	})
	if s.empty {
		drawables = append(drawables, &emptyHint{
			text: "No saves yet — click New Game to start one.",
			y:    sh/2 - 80,
		})
	}
	for _, btn := range s.buttons {
		drawables = append(drawables, btn)
	}
	r.DrawUI(drawables)
}

func (s *SaveList) Destroy() {}

// emptyHint draws a single line of dimmed text at the given Y, horizontally
// centred. Used when the SaveList is empty.
type emptyHint struct {
	text string
	y    float32
}

func (e *emptyHint) Draw(r *render.Renderer) {
	if r.Font == nil {
		return
	}
	w := float32(len(e.text) * render.GlyphAdvance)
	x := (float32(r.ScreenWidth()) - w) / 2
	r.Font.DrawText(r, e.text, x, e.y, mgl32.Vec4{0.85, 0.85, 0.85, 1})
}

// relativeTime renders a coarse "5m ago" / "2h ago" / "3d ago" string for
// the SaveList rows. Avoids importing humanize for one helper.
func relativeTime(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
