package scene

import (
	"fmt"
	"mountain-mogul/internal/edition"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// ScenarioPicker lists every starter scenario in assets/scenarios/ so the
// user can pick which one a New Game begins from — or, in editor mode,
// which one the Scenario Editor opens. In play mode each scenario is a card
// with its name, where it is and how hard (or that it's the tutorial), and
// a click starts it; in editor mode it's a list of buttons that open the file.
// Back returns to the start menu.
type ScenarioPicker struct {
	app     *engine.App
	buttons []*ui.Button // editor mode: the scenarios
	back    *ui.Button   // top left, away from where New Game was clicked
	// open, when set, opens the picked file ("" for a blank world) and
	// lists files as buttons, with a "New blank scenario" entry. The
	// editor's picker sets it (NewEditorScenarioPicker).
	open func(path string)

	cards  []*scenarioCard // play mode
	scroll float32         // how far the list is scrolled up, when it's taller than the screen
}

// scenarioCard is one scenario in the play-mode list: its entry and where
// it was last laid out.
type scenarioCard struct {
	entry      scenarioEntry
	x, y, w, h float32
	hovered    bool
}

func NewScenarioPicker() *ScenarioPicker { return &ScenarioPicker{} }

// scenarioEntry is one scenario file and the info read from its header.
type scenarioEntry struct {
	path string
	file string // basename without extension
	info world.ScenarioInfo
}

// label is the scenario's display name, or its file name when unnamed.
func (e scenarioEntry) label() string {
	if e.info.Name != "" {
		return e.info.Name
	}
	return e.file
}

// listScenarioFiles returns the sorted basenames of every save file in dir.
// A missing directory yields an empty list.
func listScenarioFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), save.SaveExt) {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)
	return files
}

// listScenarios reads every scenario in dir in campaign order. A file
// whose header can't be read is still listed, under its file name.
func listScenarios(dir string) []scenarioEntry {
	var out []scenarioEntry
	for _, name := range listScenarioFiles(dir) {
		path := filepath.Join(dir, name)
		info, _ := save.ReadScenarioInfo(path)
		out = append(out, scenarioEntry{path: path, file: strings.TrimSuffix(name, save.SaveExt), info: info})
	}
	sortScenarios(out)
	return out
}

// sortScenarios puts the tutorial first, then ordered scenarios by Order,
// then unordered ones, breaking ties by label.
func sortScenarios(s []scenarioEntry) {
	rank := func(e scenarioEntry) (int, int) {
		switch {
		case e.info.Tutorial:
			return 0, 0
		case e.info.Order > 0:
			return 1, e.info.Order
		}
		return 2, 0
	}
	sort.SliceStable(s, func(i, j int) bool {
		gi, oi := rank(s[i])
		gj, oj := rank(s[j])
		if gi != gj {
			return gi < gj
		}
		if oi != oj {
			return oi < oj
		}
		return s[i].label() < s[j].label()
	})
}

func (s *ScenarioPicker) Init(app *engine.App) error {
	s.app = app
	dir := filepath.Join(app.AssetDir, "scenarios")
	addBtn := func(label string, fn func()) {
		btn := ui.NewButton(0, 0, pickerBtnW, pickerBtnH, label, fn)
		s.buttons = append(s.buttons, btn)
	}
	for _, entry := range listScenarios(dir) {
		entry := entry
		if s.open == nil {
			s.cards = append(s.cards, &scenarioCard{entry: entry})
			continue
		}
		label := entry.label()
		if entry.info.Name != "" {
			label += "  (" + entry.file + ")"
		}
		addBtn(label, func() { s.open(entry.path) })
	}
	if !edition.Demo && s.open != nil {
		addBtn("New blank scenario", func() {
			s.open("")
		})
	}
	s.back = ui.NewButton(24, 24, 120, pickerBtnH, "Back", func() { s.app.PopScene() })
	return nil
}

const (
	pickerBtnW, pickerBtnH, pickerSpacing = float32(280), float32(40), float32(12)
	cardW, cardPad                        = float32(460), float32(16)
)

// play starts a new game of the card's scenario.
func (s *ScenarioPicker) play(c *scenarioCard) {
	s.app.ReplaceScene(NewGameFromScenario(c.entry.path))
}

// layout places the cards (or the editor's buttons) as a column centred on
// the screen, with Back under them.
func (s *ScenarioPicker) layout() {
	sw := float32(s.app.Renderer.ScreenWidth())
	sh := float32(s.app.Renderer.ScreenHeight())
	lineH := float32(render.GlyphH) + 4
	var total float32
	for _, c := range s.cards {
		c.w = cardW
		c.h = cardPad*2 + lineH*2
		total += c.h + pickerSpacing
	}
	total += float32(len(s.buttons)) * (pickerBtnH + pickerSpacing)
	// Centred when it fits; else it starts under the title and scrolls.
	room := sh - 120 - 20
	s.scroll = max(0, min(s.scroll, total-room))
	y := max((sh-total)/2, 120) - s.scroll
	for _, c := range s.cards {
		c.x, c.y = (sw-c.w)/2, y
		y += c.h + pickerSpacing
	}
	for _, b := range s.buttons {
		b.X, b.Y, b.W, b.H = (sw-pickerBtnW)/2, y, pickerBtnW, pickerBtnH
		y += pickerBtnH + pickerSpacing
	}
}

// listTop is the top of the first card or button.
func (s *ScenarioPicker) listTop() float32 {
	if len(s.cards) > 0 {
		return s.cards[0].y
	}
	if len(s.buttons) > 0 {
		return s.buttons[0].Y
	}
	return 0
}

func (s *ScenarioPicker) Update(dt float64) {
	tickMenuBackdrop(s.app, dt)
	inp := s.app.Input
	mx, my := inp.MousePos[0], inp.MousePos[1]
	s.scroll -= inp.ScrollDelta * 40
	s.layout()
	if inp.Pressed[glfw.KeyEscape] {
		s.app.PopScene()
		return
	}
	for _, c := range s.cards {
		c.hovered = mx >= c.x && mx <= c.x+c.w && my >= c.y && my <= c.y+c.h
		if inp.LeftClick && c.hovered {
			s.play(c)
			return
		}
	}
	if (inp.Pressed[glfw.KeyEnter] || inp.Pressed[glfw.KeyKPEnter]) && len(s.cards) == 1 {
		s.play(s.cards[0])
		return
	}
	for _, btn := range append(s.buttons, s.back) {
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
	renderMenuBackdrop(r)

	sw := float32(r.ScreenWidth())
	s.layout()
	drawables := []render.UIDrawable{&titleDrawable{
		x: sw/2 - 100,
		y: s.listTop() - 70,
		w: 200,
		h: 40,
	}}
	for _, c := range s.cards {
		drawables = append(drawables, c)
	}
	for _, btn := range s.buttons {
		drawables = append(drawables, btn)
	}
	drawables = append(drawables, s.back)
	r.DrawUI(drawables)
}

// Draw renders a scenario card: name, then where and how hard; lit when
// hovered, since a click starts it.
func (c *scenarioCard) Draw(r *render.Renderer) {
	fill := ui.ModalFill
	if c.hovered {
		fill = mgl32.Vec4{0.14, 0.22, 0.40, 0.97}
	}
	ui.DrawPanelColor(r, c.x, c.y, c.w, c.h, fill)
	if c.hovered {
		r.DrawRoundRectOutline(c.x, c.y, c.w, c.h, ui.PanelRadius, 2, mgl32.Vec4{0.55, 0.75, 1, 0.9})
	}
	if r.Font == nil {
		return
	}
	lineH := float32(render.GlyphH) + 4
	tx, ty := c.x+cardPad, c.y+cardPad
	r.Font.DrawText(r, c.entry.label(), tx, ty, mgl32.Vec4{1, 0.95, 0.8, 1})
	if c.hovered {
		play := "Play"
		r.Font.DrawText(r, play, c.x+c.w-cardPad-r.Font.TextWidth(play), ty, mgl32.Vec4{0.6, 0.95, 0.65, 1})
	}
	r.Font.DrawText(r, scenarioSubtitle(c.entry.info), tx, ty+lineH, mgl32.Vec4{0.65, 0.75, 0.9, 1})
}

// scenarioSubtitle is the line under a scenario's name: where it is and
// how hard it is, e.g. "Donner Pass, California · Tutorial".
func scenarioSubtitle(info world.ScenarioInfo) string {
	var parts []string
	if info.Location != "" {
		parts = append(parts, info.Location)
	}
	switch {
	case info.Tutorial:
		parts = append(parts, "Tutorial")
	case info.Difficulty > 0:
		parts = append(parts, fmt.Sprintf("Difficulty %d of %d", info.Difficulty, world.MaxScenarioDifficulty))
	}
	return strings.Join(parts, " · ")
}

func (s *ScenarioPicker) Destroy() {}
