package scene

import (
	"math"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/edition"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/ui"
)

// StartMenu is the main menu scene.
type StartMenu struct {
	app         *engine.App
	buttons     []*ui.Button
	hasContinue bool // true when buttons currently include the Continue entry
	settings    *SettingsMenu
}

// NewStartMenu creates a StartMenu scene.
func NewStartMenu() *StartMenu {
	return &StartMenu{}
}

func (s *StartMenu) Init(app *engine.App) error {
	s.app = app
	s.settings = NewSettingsMenu(app, nil)
	s.rebuildButtons()
	return nil
}

// btnDef is one start-menu button.
type btnDef struct {
	label string
	fn    func()
}

// devMenuButtons adds the full game's developer entries (the scenario
// editor and testbeds; startmenu_dev.go). Nil in the demo.
var devMenuButtons func(s *StartMenu) []btnDef

// rebuildButtons regenerates the button list to match the current saves
// state. The "Continue" entry is included only when ListSaves returns at
// least one entry, so a fresh install starts without it.
func (s *StartMenu) rebuildButtons() {
	_, hasSave := save.MostRecentSave()
	s.hasContinue = hasSave

	defs := []btnDef{
		{"New Game", func() {
			s.app.PushScene(NewScenarioPicker())
		}},
	}
	if hasSave {
		defs = append(defs, btnDef{"Continue", func() {
			path, ok := save.MostRecentSave()
			if !ok {
				return
			}
			s.app.PushScene(NewScenarioFromFile(path))
		}})
	}
	defs = append(defs,
		btnDef{"Load Game", func() {
			s.app.PushScene(NewSaveList())
		}},
	)
	if devMenuButtons != nil {
		defs = append(defs, devMenuButtons(s)...)
	}
	defs = append(defs,
		btnDef{"Settings", func() { s.settings.Show() }},
		btnDef{"Exit", func() {
			s.app.Window.SetShouldClose(true)
		}},
	)

	s.buttons = make([]*ui.Button, 0, len(defs))
	for _, d := range defs {
		btn := ui.NewButton(0, 0, startBtnW, startBtnH, d.label, d.fn)
		s.buttons = append(s.buttons, btn)
	}
}

// Start menu geometry: a column of buttons in a panel under the title.
const (
	startBtnW, startBtnH = float32(240), float32(44)
	startBtnGap          = float32(10)
	startPanelPad        = float32(20)
)

// panelRect is the button panel's rect.
func (s *StartMenu) panelRect() (x, y, w, h float32) {
	sw := float32(s.app.Renderer.ScreenWidth())
	sh := float32(s.app.Renderer.ScreenHeight())
	n := float32(len(s.buttons))
	w = startBtnW + startPanelPad*2
	h = n*startBtnH + (n-1)*startBtnGap + startPanelPad*2
	return (sw - w) / 2, sh*0.42 + max(0, (sh*0.5-h)/2), w, h
}

// layout recomputes button positions relative to the current screen size.
// Called at the top of Update so both hit-testing and rendering use the same coords.
func (s *StartMenu) layout() {
	px, py, _, _ := s.panelRect()
	for i, btn := range s.buttons {
		btn.X = px + startPanelPad
		btn.Y = py + startPanelPad + float32(i)*(startBtnH+startBtnGap)
		btn.W = startBtnW
		btn.H = startBtnH
	}
}

func (s *StartMenu) Update(dt float64) {
	// Re-evaluate Continue visibility every frame — when the user pops back
	// here from a scenario where they just saved, the button needs to appear
	// without an explicit lifecycle hook. Rebuild only on state change.
	if _, hasSave := save.MostRecentSave(); hasSave != s.hasContinue {
		s.rebuildButtons()
	}
	s.layout()

	inp := s.app.Input
	if s.settings.Visible() {
		if inp.Pressed[glfw.KeyEscape] {
			s.settings.Hide()
			return
		}
		s.settings.HandleInput(inp)
		return
	}
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

	drawables := []render.UIDrawable{uiDrawFunc(s.drawBackdrop)}
	for _, btn := range s.buttons {
		drawables = append(drawables, btn)
	}
	if s.settings.Visible() {
		drawables = append(drawables, s.settings)
	}
	r.DrawUI(drawables)
}

func (s *StartMenu) Destroy() {}

// drawBackdrop paints a winter sky, a range of peaks, the title, and the
// button panel.
func (s *StartMenu) drawBackdrop(r *render.Renderer) {
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	drawMenuSky(r)

	// Title.
	title := "Mountain Mogul"
	f := r.TitleFont
	if f == nil {
		f = r.Font
	}
	if f != nil {
		tw := f.TextWidth(title)
		ty := sh * 0.16
		f.DrawText(r, title, (sw-tw)/2+3, ty+3, mgl32.Vec4{0.05, 0.10, 0.25, 0.45})
		f.DrawText(r, title, (sw-tw)/2, ty, mgl32.Vec4{1, 1, 1, 1})
		if edition.Demo && r.Font != nil {
			tag := "DEMO"
			w := r.Font.TextWidth(tag) + 24
			r.DrawRoundRect((sw-w)/2, ty+90, w, float32(render.GlyphH)+8, 8, mgl32.Vec4{0.95, 0.55, 0.20, 0.95})
			r.Font.DrawText(r, tag, (sw-w)/2+12, ty+94, mgl32.Vec4{1, 1, 1, 1})
		}
	}

	px, py, pw, ph := s.panelRect()
	ui.DrawPanel(r, px, py, pw, ph)
}

// drawMenuSky paints the menus' backdrop: a winter sky over two ranges of
// snowy peaks.
func drawMenuSky(r *render.Renderer) {
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())

	// Sky: deep blue at the top to pale at the horizon.
	const bands = 48
	top := mgl32.Vec4{0.16, 0.27, 0.52, 1}
	low := mgl32.Vec4{0.74, 0.82, 0.95, 1}
	for i := range bands {
		t := float32(i) / (bands - 1)
		c := top.Mul(1 - t).Add(low.Mul(t))
		r.DrawColorRect(0, sh*float32(i)/bands, sw, sh/bands+1, c)
	}
	// Two ranges of snowy peaks.
	drawRange(r, sw, sh, sh*0.62, 0.13, mgl32.Vec4{0.80, 0.86, 0.96, 1}, 0)
	drawRange(r, sw, sh, sh*0.74, 0.09, mgl32.Vec4{0.95, 0.97, 1.00, 1}, 1.7)
}

// titleDrawable draws the menu backdrop with the game's name at (x, y),
// for the screens behind the start menu (saves, scenarios, testbeds).
type titleDrawable struct {
	x, y, w, h float32
}

func (t *titleDrawable) Draw(r *render.Renderer) {
	drawMenuSky(r)
	if r.Font == nil {
		return
	}
	title := "Mountain Mogul"
	tw := r.Font.TextWidth(title)
	cx := t.x + t.w/2
	r.DrawRoundRect(cx-tw/2-16, t.y, tw+32, t.h, 8, ui.PanelFill)
	r.Font.DrawText(r, title, cx-tw/2, t.y+(t.h-float32(render.GlyphH))/2, mgl32.Vec4{1, 1, 1, 1})
}

// drawRange draws a jagged mountain silhouette whose ridge sits around
// baseY, peaks up to amp×screen height above it, filled to the bottom.
func drawRange(r *render.Renderer, sw, sh, baseY, amp float32, col mgl32.Vec4, phase float64) {
	const steps = 64
	prevX, prevY := float32(0), baseY
	for i := 1; i <= steps; i++ {
		x := sw * float32(i) / steps
		u := float64(i)/steps*7 + phase
		h := 0.55*math.Abs(math.Sin(u)) + 0.30*math.Abs(math.Sin(u*2.3+1)) + 0.15*math.Abs(math.Sin(u*5.1))
		y := baseY - float32(h)*amp*sh
		r.DrawColorTriangle(prevX, prevY, x, y, prevX, sh, col)
		r.DrawColorTriangle(x, y, x, sh, prevX, sh, col)
		prevX, prevY = x, y
	}
}
