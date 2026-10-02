package scene

import (
	"fmt"
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
// which one the Scenario Editor opens. In play mode a pick opens a detail
// panel with the description and a Play button; in editor mode it opens
// the file straight away. Back returns to the start menu.
type ScenarioPicker struct {
	app       *engine.App
	buttons   []*ui.Button
	forEditor bool // pick opens the editor, plus a "New blank scenario" entry

	detail  *scenarioEntry // play mode: the scenario whose panel is open
	playBtn *ui.Button
	backBtn *ui.Button
}

func NewScenarioPicker() *ScenarioPicker { return &ScenarioPicker{} }

// NewEditorScenarioPicker lists the same files but opens the pick in the
// Scenario Editor, with an extra entry for starting from a blank world.
func NewEditorScenarioPicker() *ScenarioPicker { return &ScenarioPicker{forEditor: true} }

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
		btn := ui.NewButton(0, 0, 280, 40, label, fn)
		btn.Color = mgl32.Vec4{0.15, 0.25, 0.45, 0.95}
		btn.HoverColor = mgl32.Vec4{0.25, 0.45, 0.75, 0.95}
		s.buttons = append(s.buttons, btn)
	}
	for _, entry := range listScenarios(dir) {
		entry := entry
		label := entry.label()
		if s.forEditor && entry.info.Name != "" {
			label += "  (" + entry.file + ")"
		}
		addBtn(label, func() {
			if s.forEditor {
				s.app.ReplaceScene(NewEditor(entry.path))
			} else {
				s.detail = &entry
			}
		})
	}
	if s.forEditor {
		addBtn("New blank scenario", func() {
			s.app.ReplaceScene(NewEditor(""))
		})
	}
	back := ui.NewButton(0, 0, 280, 40, "Back", func() { s.app.PopScene() })
	back.Color = mgl32.Vec4{0.4, 0.25, 0.25, 0.95}
	back.HoverColor = mgl32.Vec4{0.6, 0.4, 0.4, 0.95}
	s.buttons = append(s.buttons, back)

	s.playBtn = ui.NewButton(0, 0, 120, 40, "Play", func() {
		s.app.ReplaceScene(NewScenarioFromFile(s.detail.path))
	})
	s.playBtn.Color = mgl32.Vec4{0.15, 0.45, 0.25, 0.95}
	s.playBtn.HoverColor = mgl32.Vec4{0.25, 0.65, 0.35, 0.95}
	s.backBtn = ui.NewButton(0, 0, 120, 40, "Back", func() { s.detail = nil })
	s.backBtn.Color = back.Color
	s.backBtn.HoverColor = back.HoverColor
	return nil
}

const (
	pickerBtnW, pickerBtnH, pickerSpacing = float32(280), float32(40), float32(12)
)

// listTop is the y of the first scenario button.
func (s *ScenarioPicker) listTop() float32 {
	sh := float32(s.app.Renderer.ScreenHeight())
	n := float32(len(s.buttons))
	return (sh - (n*pickerBtnH + (n-1)*pickerSpacing)) / 2
}

func (s *ScenarioPicker) layout() {
	sw := float32(s.app.Renderer.ScreenWidth())
	startY := s.listTop()
	for i, btn := range s.buttons {
		btn.X = (sw - pickerBtnW) / 2
		btn.Y = startY + float32(i)*(pickerBtnH+pickerSpacing)
		btn.W = pickerBtnW
		btn.H = pickerBtnH
	}
}

func (s *ScenarioPicker) Update(dt float64) {
	inp := s.app.Input
	mx, my := inp.MousePos[0], inp.MousePos[1]
	buttons := s.buttons
	if s.detail != nil {
		if inp.Pressed[glfw.KeyEscape] {
			s.detail = nil
			return
		}
		if inp.Pressed[glfw.KeyEnter] || inp.Pressed[glfw.KeyKPEnter] {
			s.playBtn.Click()
			return
		}
		s.detailPanel(float32(s.app.Renderer.ScreenWidth()), float32(s.app.Renderer.ScreenHeight()), nil)
		buttons = []*ui.Button{s.playBtn, s.backBtn}
	} else {
		s.layout()
	}
	for _, btn := range buttons {
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
	if s.detail != nil {
		r.DrawUI([]render.UIDrawable{uiDrawFunc(func(r *render.Renderer) {
			s.detailPanel(sw, sh, r)
		})})
		return
	}
	s.layout()
	drawables := make([]render.UIDrawable, 0, len(s.buttons)+1)
	drawables = append(drawables, &titleDrawable{
		x: sw/2 - 100,
		y: s.listTop() - 70,
		w: 200,
		h: 40,
	})
	for _, btn := range s.buttons {
		drawables = append(drawables, btn)
	}
	r.DrawUI(drawables)
}

const (
	detailPanelW = float32(560)
	detailPad    = float32(24)
)

// detailPanel lays out the open scenario's panel and its buttons, and
// draws it when r is non-nil. Layout and drawing share one pass because
// the panel's height depends on how the description wraps.
func (s *ScenarioPicker) detailPanel(sw, sh float32, r *render.Renderer) {
	e := s.detail
	lineH := float32(render.GlyphH) + 6
	var desc []string
	if s.app.Renderer.Font != nil {
		desc = ui.WrapText(e.info.Description, detailPanelW-2*detailPad, s.app.Renderer.Font.TextWidth)
	}
	h := detailPad + lineH*2 + 12 + float32(len(desc))*lineH + 24 + pickerBtnH + detailPad
	x := (sw - detailPanelW) / 2
	y := (sh - h) / 2
	by := y + h - detailPad - pickerBtnH
	s.playBtn.X, s.playBtn.Y = x+detailPanelW-detailPad-s.playBtn.W, by
	s.backBtn.X, s.backBtn.Y = s.playBtn.X-12-s.backBtn.W, by
	if r == nil {
		return
	}

	r.DrawColorRect(x, y, detailPanelW, h, mgl32.Vec4{0.08, 0.12, 0.22, 0.96})
	if r.Font != nil {
		tx, ty := x+detailPad, y+detailPad
		r.Font.DrawText(r, e.label(), tx, ty, mgl32.Vec4{1, 0.95, 0.8, 1})
		r.Font.DrawText(r, scenarioSubtitle(e.info), tx, ty+lineH, mgl32.Vec4{0.65, 0.75, 0.9, 1})
		ty += lineH*2 + 12
		for _, line := range desc {
			r.Font.DrawText(r, line, tx, ty, mgl32.Vec4{0.92, 0.94, 1, 1})
			ty += lineH
		}
	}
	s.playBtn.Draw(r)
	s.backBtn.Draw(r)
}

// scenarioSubtitle is the line under a scenario's name: where it is and
// how hard it is, e.g. "Donner Pass, California  -  Tutorial". The font
// is ASCII only.
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
	return strings.Join(parts, "  -  ")
}

func (s *ScenarioPicker) Destroy() {}
