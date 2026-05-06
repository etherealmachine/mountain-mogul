package scene

import (
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
)

// EscapeMenu is a modal overlay shown when the player presses Escape.
type EscapeMenu struct {
	visible bool
	buttons []*ui.Button
	app     *engine.App
}

// NewEscapeMenu constructs the menu. saveFunc is called when the player clicks
// Save; loadFunc is called when the player clicks Load. Either may be nil to
// disable that action (button still draws but does nothing).
func NewEscapeMenu(app *engine.App, saveFunc, loadFunc func()) *EscapeMenu {
	m := &EscapeMenu{app: app}

	type def struct {
		label string
		fn    func()
	}
	defs := []def{
		{"Main Menu", func() { app.PopScene() }},
		{"Save", func() {
			if saveFunc != nil {
				saveFunc()
			}
			m.visible = false
		}},
		{"Load", func() {
			if loadFunc != nil {
				loadFunc()
			}
			m.visible = false
		}},
		{"Settings", func() {
			m.visible = false
		}},
		{"Exit Game", func() { app.Window.SetShouldClose(true) }},
	}

	m.buttons = make([]*ui.Button, 0, len(defs))
	for _, d := range defs {
		btn := ui.NewButton(0, 0, 200, 40, d.label, d.fn)
		btn.Color = mgl32.Vec4{0.15, 0.25, 0.45, 0.95}
		btn.HoverColor = mgl32.Vec4{0.25, 0.45, 0.75, 0.95}
		m.buttons = append(m.buttons, btn)
	}
	return m
}

func (m *EscapeMenu) Visible() bool { return m.visible }
func (m *EscapeMenu) Show()         { m.visible = true }
func (m *EscapeMenu) Hide()         { m.visible = false }
func (m *EscapeMenu) Toggle()       { m.visible = !m.visible }

func (m *EscapeMenu) layout(sw, sh float32) {
	const btnW, btnH, spacing float32 = 200, 40, 12
	n := float32(len(m.buttons))
	totalH := n*btnH + (n-1)*spacing
	// Centre cluster, push below mid to leave room for title.
	startY := (sh-totalH)/2 + 30

	for i, btn := range m.buttons {
		btn.X = (sw - btnW) / 2
		btn.Y = startY + float32(i)*(btnH+spacing)
		btn.W = btnW
		btn.H = btnH
	}
}

// HandleInput processes escape menu input. Call this instead of normal scene
// input handling when the menu is visible.
func (m *EscapeMenu) HandleInput(inp *engine.Input) {
	sw := float32(m.app.Renderer.ScreenWidth())
	sh := float32(m.app.Renderer.ScreenHeight())
	m.layout(sw, sh)

	mx, my := inp.MousePos[0], inp.MousePos[1]
	for _, btn := range m.buttons {
		btn.SetHovered(btn.Contains(mx, my))
		if inp.LeftClick && btn.Contains(mx, my) {
			btn.Click()
			return
		}
	}
}

// Draw implements render.UIDrawable. Must be the last element in DrawUI's
// drawable list so it renders on top of everything else.
func (m *EscapeMenu) Draw(r *render.Renderer) {
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	m.layout(sw, sh)

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	defer gl.Disable(gl.BLEND)

	// Full-screen dimming overlay.
	r.DrawColorRect(0, 0, sw, sh, mgl32.Vec4{0, 0, 0, 0.6})

	// Panel background.
	const btnW, btnH, spacing, pad float32 = 200, 40, 12, 24
	n := float32(len(m.buttons))
	panelH := n*btnH + (n-1)*spacing + pad*2 + 36 // 36 for title row
	panelW := btnW + pad*2
	panelX := (sw - panelW) / 2
	panelY := (sh - panelH) / 2
	r.DrawColorRect(panelX, panelY, panelW, panelH, mgl32.Vec4{0.08, 0.12, 0.22, 0.97})

	if r.Font != nil {
		r.Font.DrawText(r, "Menu", panelX+pad, panelY+pad, mgl32.Vec4{1, 0.95, 0.8, 1})
	}

	for _, btn := range m.buttons {
		btn.Draw(r)
	}
}
