package scene

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/ui"
)

// SettingsMenu is a modal overlay for player preferences: a row per
// setting, each a choice of options or a check box. Opened from the start
// menu and the in-game menu; "Back" returns to whichever opened it.
type SettingsMenu struct {
	visible bool
	app     *engine.App
	onBack  func()

	rows    []*settingRow
	backBtn *ui.Button
}

// settingRow is one setting: a label, with either a choice of option
// buttons (the active one picked by selected) or a check box (on reports
// it, toggle flips it), and an optional dim note under the label.
type settingRow struct {
	label    string
	note     string
	options  []*ui.Button
	selected func() int

	on      func() bool
	toggle  func()
	y       float32 // row top, set by layout
	hovered bool
}

// isCheck reports whether the row is a check box.
func (row *settingRow) isCheck() bool { return row.toggle != nil }

// height is the row's height: taller with a note.
func (row *settingRow) height() float32 {
	if row.note != "" {
		return settingsRowH + float32(render.GlyphH)
	}
	return settingsRowH
}

// NewSettingsMenu creates the settings menu. onBack is called when the
// player clicks "Back"; typically it shows the menu that opened it.
func NewSettingsMenu(app *engine.App, onBack func()) *SettingsMenu {
	m := &SettingsMenu{app: app, onBack: onBack}
	st := settings.Get

	m.addRow("Units", []string{"Imperial", "Metric"},
		func() int { return int(st().Units) },
		func(i int) { st().Units = settings.Units(i) })
	m.addCheck("Full screen", "",
		app.Fullscreen,
		func(on bool) {
			st().Fullscreen = on
			app.SetFullscreen(on)
		})
	aaLabels := make([]string, len(render.AASampleOptions))
	for i, n := range render.AASampleOptions {
		aaLabels[i] = "Off"
		if n > 0 {
			aaLabels[i] = fmt.Sprintf("%dx", n)
		}
	}
	m.addRow("Anti-aliasing", aaLabels,
		func() int {
			for i, n := range render.AASampleOptions {
				if n == st().AntiAliasing {
					return i
				}
			}
			return -1
		},
		func(i int) {
			st().AntiAliasing = render.AASampleOptions[i]
			app.Renderer.SetAntiAliasing(st().AntiAliasing)
		})
	m.addCheck("Show the day report each night", "",
		func() bool { return !st().HideDailyReport },
		func(on bool) { st().HideDailyReport = !on })
	m.addCheck("Pause each night", "",
		func() bool { return !st().NoNightPause },
		func(on bool) { st().NoNightPause = !on })

	m.backBtn = ui.NewButton(0, 0, 160, 40, "Back", func() {
		m.visible = false
		if onBack != nil {
			onBack()
		}
	})
	return m
}

// addRow adds a setting with options; pick applies option i and the
// settings are saved.
func (m *SettingsMenu) addRow(label string, options []string, selected func() int, pick func(int)) {
	row := &settingRow{label: label, selected: selected}
	for i, o := range options {
		b := ui.NewButton(0, 0, settingsOptionW, settingsRowH, o, func() {
			pick(i)
			_ = settings.Save()
		})
		row.options = append(row.options, b)
	}
	m.rows = append(m.rows, row)
}

// addCheck adds an on/off setting as a check box; set applies it and the
// settings are saved.
func (m *SettingsMenu) addCheck(label, note string, on func() bool, set func(bool)) {
	m.rows = append(m.rows, &settingRow{label: label, note: note, on: on, toggle: func() {
		set(!on())
		_ = settings.Save()
	}})
}

func (m *SettingsMenu) Visible() bool { return m.visible }
func (m *SettingsMenu) Show()         { m.visible = true }
func (m *SettingsMenu) Hide()         { m.visible = false }

const (
	settingsPanelW  float32 = 520
	settingsPad     float32 = 24
	settingsRowH    float32 = 36
	settingsRowGap  float32 = 12
	settingsOptionW float32 = 100
	settingsCheckSz float32 = 26
)

// panel returns the panel's rect, centred on the screen.
func (m *SettingsMenu) panel(sw, sh float32) (x, y, w, h float32) {
	h = float32(render.GlyphH) + 10 + settingsPad + settingsPad + 40 + settingsPad
	for i, row := range m.rows {
		if i > 0 {
			h += settingsRowGap
		}
		h += row.height()
	}
	return (sw - settingsPanelW) / 2, (sh - h) / 2, settingsPanelW, h
}

func (m *SettingsMenu) layout(sw, sh float32) {
	px, py, pw, ph := m.panel(sw, sh)
	y := py + float32(render.GlyphH) + 10 + settingsPad
	for _, row := range m.rows {
		row.y = y
		x := px + pw - settingsPad
		for i := len(row.options) - 1; i >= 0; i-- {
			b := row.options[i]
			x -= b.W
			b.X, b.Y = x, y
			x -= 6
		}
		y += row.height() + settingsRowGap
	}
	m.backBtn.X = px + (pw-m.backBtn.W)/2
	m.backBtn.Y = py + ph - settingsPad - m.backBtn.H
}

func (m *SettingsMenu) buttons() []*ui.Button {
	var out []*ui.Button
	for _, row := range m.rows {
		out = append(out, row.options...)
	}
	return append(out, m.backBtn)
}

// HandleInput processes settings menu input. A check-box row toggles on a
// click anywhere along it.
func (m *SettingsMenu) HandleInput(inp *engine.Input) {
	sw := float32(m.app.Renderer.ScreenWidth())
	sh := float32(m.app.Renderer.ScreenHeight())
	m.layout(sw, sh)

	if inp.LeftClick {
		inp.LeftClickConsumed = true
	}

	mx, my := inp.MousePos[0], inp.MousePos[1]
	px, _, pw, _ := m.panel(sw, sh)
	for _, row := range m.rows {
		if !row.isCheck() {
			continue
		}
		row.hovered = mx >= px && mx <= px+pw && my >= row.y && my <= row.y+row.height()
		if row.hovered && inp.LeftClick {
			row.toggle()
			return
		}
	}
	for _, btn := range m.buttons() {
		btn.SetHovered(btn.Contains(mx, my))
		if inp.LeftClick && btn.Contains(mx, my) {
			btn.Click()
			return
		}
	}
}

// Draw renders the settings modal.
func (m *SettingsMenu) Draw(r *render.Renderer) {
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	m.layout(sw, sh)

	r.DrawColorRect(0, 0, sw, sh, ui.Scrim)
	px, py, pw, ph := m.panel(sw, sh)
	ui.DrawPanelColor(r, px, py, pw, ph, ui.ModalFill)
	ui.DrawPanelHeader(r, px, py, pw, float32(render.GlyphH)+10, "Settings")
	if r.Font == nil {
		return
	}

	labelCol := mgl32.Vec4{0.85, 0.89, 0.97, 1}
	noteCol := mgl32.Vec4{0.58, 0.64, 0.76, 1}
	for _, row := range m.rows {
		r.Font.DrawText(r, row.label, px+settingsPad, row.y+(settingsRowH-float32(render.GlyphH))/2, labelCol)
		if row.note != "" {
			r.Font.DrawText(r, row.note, px+settingsPad, row.y+settingsRowH-4, noteCol)
		}
		if row.isCheck() {
			ui.DrawCheckbox(r, px+pw-settingsPad-settingsCheckSz, row.y+(settingsRowH-settingsCheckSz)/2,
				settingsCheckSz, row.on(), row.hovered)
			continue
		}
		sel := row.selected()
		for i, b := range row.options {
			b.SetActive(i == sel)
			b.Draw(r)
		}
	}
	m.backBtn.Draw(r)
}
