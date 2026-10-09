package scene

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// editorToastSeconds is how long an editor toast stays on screen (s).
const editorToastSeconds = 3.0

// scenarioName is the open file's basename without extension, or "" for a
// blank scenario that has not been saved yet.
func (e *Editor) scenarioName() string {
	if e.scenarioPath == "" {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(e.scenarioPath), save.SaveExt)
}

// editorTitle is the top-bar caption: the scenario's display name and file
// name, plus a "*" marker while there are unsaved edits.
func (e *Editor) editorTitle() string {
	name := e.scenarioName()
	switch {
	case name == "":
		name = "Untitled scenario"
	case e.world.Scenario.Name != "":
		name = e.world.Scenario.Name + " (" + name + ")"
	}
	if e.dirty {
		name += " *"
	}
	return "Editing: " + name
}

// markDirty flags the scenario as having unsaved edits. Called from each
// mutation entry point; coarse on purpose (e.g. the first click of a
// two-click lift placement already counts).
func (e *Editor) markDirty() { e.dirty = true }

func (e *Editor) setToast(text string) {
	e.toastText = text
	e.toastExpiry = e.time + editorToastSeconds
}

// openDetailsPrompt edits the scenario's name, description, and campaign
// placing (World.Scenario).
func (e *Editor) openDetailsPrompt() {
	e.escapeMenu.Hide()
	e.detailsPrompt = newScenarioDetailsPrompt(e.world.Scenario, e.world.Goals, e.world.Rules,
		func(info world.ScenarioInfo, goals []world.Goal, rules []string) {
			if mix := e.detailsPrompt.mix; mix != e.world.Mix() {
				e.world.GroupMix = mix
				world.RegroupGuests(e.world, e.world.Seed)
				e.markDirty()
			}
			e.detailsPrompt = nil
			if info != e.world.Scenario || !slices.Equal(goals, e.world.Goals) || !slices.Equal(rules, e.world.Rules) {
				e.world.Scenario = info
				e.world.Goals, e.world.Rules = goals, rules
				e.world.GoalProgress = make([]world.GoalProgress, len(goals))
				e.markDirty()
			}
		},
		func() { e.detailsPrompt = nil },
	)
	e.detailsPrompt.mix = e.world.Mix()
}

// saveCurrent overwrites the open file. A blank scenario has no file yet,
// so its first Save behaves like Save As.
func (e *Editor) saveCurrent() {
	if e.scenarioPath == "" {
		e.openSaveAsPrompt()
		return
	}
	e.writeScenario(e.scenarioPath)
}

// openSaveAsPrompt asks for a scenario name, pre-filled with the current one.
func (e *Editor) openSaveAsPrompt() {
	e.savePrompt = newSavePrompt(e.scenarioName(),
		func(name string) {
			e.savePrompt = nil
			e.commitSaveAs(name)
		},
		func() { e.savePrompt = nil },
	)
}

// commitSaveAs resolves name to assets/scenarios/<name>.save and writes it,
// asking first when that would overwrite a different existing file.
func (e *Editor) commitSaveAs(name string) {
	clean := save.SanitizeSaveName(name)
	if clean == "" {
		e.setToast("Scenario name cannot be empty")
		return
	}
	path := filepath.Join(e.app.AssetDir, "scenarios", clean+save.SaveExt)
	if _, err := os.Stat(path); err == nil && path != e.scenarioPath {
		e.confirmPrompt = newConfirmPrompt("Overwrite "+clean+"?", "Overwrite",
			func() {
				e.confirmPrompt = nil
				e.writeScenario(path)
			},
			func() { e.confirmPrompt = nil },
		)
		return
	}
	e.writeScenario(path)
}

// writeScenario saves to path and, on success, makes it the open file.
func (e *Editor) writeScenario(path string) {
	if err := save.SaveScenario(path, e.world, editorCameraSnapshot(e)); err != nil {
		e.setToast("Save error: " + err.Error())
		return
	}
	e.scenarioPath = path
	e.dirty = false
	e.setToast("Saved " + e.scenarioName())
}

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
	r.DrawColorRect(0, 0, sw, sh, mgl32.Vec4{0, 0, 0, 0.55})
	x := (sw - confirmPromptW) / 2
	y := (sh - p.h) / 2
	r.DrawColorRect(x, y, confirmPromptW, p.h, mgl32.Vec4{0.08, 0.12, 0.22, 0.98})
	if r.Font != nil {
		for k, l := range lines {
			r.Font.DrawText(r, l, x+16, y+20+float32(k)*lineH, mgl32.Vec4{1, 0.95, 0.8, 1})
		}
	}
	p.okBtn.Draw(r)
	p.cancelBtn.Draw(r)
}
