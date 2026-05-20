package scene

import (
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/ui"
)

// SettingsMenu is a modal overlay for player-configurable preferences.
// Opened from the EscapeMenu's Settings button; "Back" returns there.
type SettingsMenu struct {
	visible bool
	app     *engine.App
	onBack  func()

	imperialBtn *ui.Button
	metricBtn   *ui.Button
	backBtn     *ui.Button
}

// NewSettingsMenu creates the settings menu. onBack is called when the
// player clicks "← Back"; typically it shows the escape menu again.
func NewSettingsMenu(app *engine.App, onBack func()) *SettingsMenu {
	m := &SettingsMenu{app: app, onBack: onBack}

	btnColor := mgl32.Vec4{0.15, 0.25, 0.45, 0.95}
	hoverColor := mgl32.Vec4{0.25, 0.45, 0.75, 0.95}
	activeColor := mgl32.Vec4{0.20, 0.55, 0.35, 0.95}

	m.imperialBtn = ui.NewButton(0, 0, 95, 36, "Imperial", func() {
		settings.Get().Units = settings.Imperial
		_ = settings.Save()
	})
	m.imperialBtn.Color = btnColor
	m.imperialBtn.HoverColor = hoverColor
	m.imperialBtn.ActiveColor = activeColor

	m.metricBtn = ui.NewButton(0, 0, 95, 36, "Metric", func() {
		settings.Get().Units = settings.Metric
		_ = settings.Save()
	})
	m.metricBtn.Color = btnColor
	m.metricBtn.HoverColor = hoverColor
	m.metricBtn.ActiveColor = activeColor

	m.backBtn = ui.NewButton(0, 0, 200, 40, "← Back", func() {
		m.visible = false
		if onBack != nil {
			onBack()
		}
	})
	m.backBtn.Color = btnColor
	m.backBtn.HoverColor = hoverColor

	return m
}

func (m *SettingsMenu) Visible() bool { return m.visible }
func (m *SettingsMenu) Show()         { m.visible = true }
func (m *SettingsMenu) Hide()         { m.visible = false }

func (m *SettingsMenu) layout(sw, sh float32) {
	const panelW float32 = 280
	const panelH float32 = 200
	panelX := (sw - panelW) / 2
	panelY := (sh - panelH) / 2

	const pad float32 = 24
	const rowH float32 = 36
	const labelRows float32 = 2 // title + units label

	unitsY := panelY + pad + labelRows*float32(render.GlyphH+4) + 8
	m.imperialBtn.X = panelX + pad
	m.imperialBtn.Y = unitsY
	m.imperialBtn.W = 95
	m.imperialBtn.H = rowH

	m.metricBtn.X = panelX + pad + 95 + 8
	m.metricBtn.Y = unitsY
	m.metricBtn.W = 95
	m.metricBtn.H = rowH

	m.backBtn.X = (sw - m.backBtn.W) / 2
	m.backBtn.Y = panelY + panelH - pad - m.backBtn.H
}

// HandleInput processes settings menu input.
func (m *SettingsMenu) HandleInput(inp *engine.Input) {
	sw := float32(m.app.Renderer.ScreenWidth())
	sh := float32(m.app.Renderer.ScreenHeight())
	m.layout(sw, sh)

	if inp.LeftClick {
		inp.LeftClickConsumed = true
	}

	mx, my := inp.MousePos[0], inp.MousePos[1]
	for _, btn := range []*ui.Button{m.imperialBtn, m.metricBtn, m.backBtn} {
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

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	defer gl.Disable(gl.BLEND)

	r.DrawColorRect(0, 0, sw, sh, mgl32.Vec4{0, 0, 0, 0.6})

	const panelW float32 = 280
	const panelH float32 = 200
	panelX := (sw - panelW) / 2
	panelY := (sh - panelH) / 2
	r.DrawColorRect(panelX, panelY, panelW, panelH, mgl32.Vec4{0.08, 0.12, 0.22, 0.97})

	if r.Font == nil {
		return
	}

	const pad float32 = 24
	textCol := mgl32.Vec4{1, 0.95, 0.8, 1}
	labelCol := mgl32.Vec4{0.78, 0.82, 0.92, 1}

	r.Font.DrawText(r, "Settings", panelX+pad, panelY+pad, textCol)
	r.Font.DrawText(r, "Units", panelX+pad, panelY+pad+float32(render.GlyphH+4)+8, labelCol)

	// Highlight the active unit button.
	u := settings.Get().Units
	m.imperialBtn.SetActive(u == settings.Imperial)
	m.metricBtn.SetActive(u == settings.Metric)

	m.imperialBtn.Draw(r)
	m.metricBtn.Draw(r)
	m.backBtn.Draw(r)
}
