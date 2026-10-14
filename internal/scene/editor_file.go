//go:build !demo

package scene

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"mountain-mogul/internal/save"
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

// editorTitle is the title panel's caption: the scenario's display name,
// else its file name, plus a "*" marker while there are unsaved edits.
func (e *Editor) editorTitle() string {
	name := e.world.Scenario.Name
	if name == "" {
		name = e.scenarioName()
	}
	if name == "" {
		name = "Untitled scenario"
	}
	if e.dirty {
		name += " *"
	}
	return name
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
			if m := e.detailsPrompt.money.m; m != moneyOf(e.world) {
				m.apply(e.world)
				e.markDirty()
			}
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
	e.detailsPrompt.money = newMoneyTab(moneyOf(e.world))
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
