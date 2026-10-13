package scene

import (
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
)

// confirmPrompt is a modal yes/no box. Enter confirms, Escape cancels.
type confirmPrompt struct {
	message   string
	okBtn     *ui.Button
	cancelBtn *ui.Button
	onOK      func()
	onCancel  func()
	h         float32 // grows with the wrapped message; set by Draw
}

func newConfirmPrompt(message, okLabel string, onOK, onCancel func()) *confirmPrompt {
	p := &confirmPrompt{message: message, onOK: onOK, onCancel: onCancel, h: confirmPromptH}
	p.okBtn = ui.NewButton(0, 0, 110, 32, okLabel, func() { p.onOK() })
	p.cancelBtn = ui.NewButton(0, 0, 90, 32, "Cancel", func() { p.onCancel() })
	return p
}

const confirmPromptW = 420
const confirmPromptH = 120

func (p *confirmPrompt) layout(sw, sh float32) {
	x := (sw - confirmPromptW) / 2
	y := (sh - p.h) / 2
	const pad = 16
	p.okBtn.X = x + confirmPromptW - pad - p.okBtn.W
	p.okBtn.Y = y + p.h - pad - p.okBtn.H
	p.cancelBtn.X = p.okBtn.X - 12 - p.cancelBtn.W
	p.cancelBtn.Y = p.okBtn.Y
}

func (p *confirmPrompt) HandleInput(inp *engine.Input, sw, sh float32) {
	p.layout(sw, sh)
	if inp.LeftClick {
		inp.LeftClickConsumed = true
	}
	if inp.Pressed[glfw.KeyEscape] {
		p.onCancel()
		return
	}
	if inp.Pressed[glfw.KeyEnter] || inp.Pressed[glfw.KeyKPEnter] {
		p.onOK()
		return
	}
	mx, my := inp.MousePos[0], inp.MousePos[1]
	for _, b := range []*ui.Button{p.okBtn, p.cancelBtn} {
		b.SetHovered(b.Contains(mx, my))
		if inp.LeftClick && b.Contains(mx, my) {
			b.Click()
			return
		}
	}
}

func (p *confirmPrompt) Draw(r *render.Renderer) {
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	lines := []string{p.message}
	if r.Font != nil {
		lines = ui.WrapText(p.message, confirmPromptW-32, r.Font.TextWidth)
	}
	lineH := float32(render.GlyphH + 4)
	p.h = max(confirmPromptH, 20+float32(len(lines))*lineH+16+32+16)
	p.layout(sw, sh)
	r.DrawColorRect(0, 0, sw, sh, ui.Scrim)
	x := (sw - confirmPromptW) / 2
	y := (sh - p.h) / 2
	ui.DrawPanelColor(r, x, y, confirmPromptW, p.h, ui.ModalFill)
	if r.Font != nil {
		for k, l := range lines {
			r.Font.DrawText(r, l, x+16, y+20+float32(k)*lineH, mgl32.Vec4{1, 0.95, 0.8, 1})
		}
	}
	p.okBtn.Draw(r)
	p.cancelBtn.Draw(r)
}
