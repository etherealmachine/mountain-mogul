package scene

import (
	"fmt"
	"math"
	"strings"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// maxScenarioOrder caps the Order stepper in the details dialog.
const maxScenarioOrder = 99

// scenarioDetailsPrompt is the editor's modal for World.Scenario and its
// goals, on two tabs. Details: name, location, and description fields,
// difficulty and order steppers, and a tutorial toggle. Goals: the goals
// and rules (editor_goals.go). Guests: how guests come in groups
// (World.GroupMix), read back from mix. It edits copies; OK hands them back,
// Cancel or Escape drops them. Tab moves between the text fields.
type scenarioDetailsPrompt struct {
	info   world.ScenarioInfo
	goals  *goalsTab
	tab    int // 0 details, 1 goals, 2 guests
	tabBtn [3]*ui.Button
	mix    world.GroupMix
	mixBtn [6]*ui.Button   // size, lessons, mixed: down and up each
	fields []*ui.TextInput // name, location, description
	focus  int

	diffDown, diffUp, orderDown, orderUp *ui.Button
	tutorialBtn, okBtn, cancelBtn        *ui.Button

	onOK     func(world.ScenarioInfo, []world.Goal, []string)
	onCancel func()

	x, y float32 // panel origin, from the last layout
}

const (
	detailsPromptW = float32(820)
	detailsPromptH = float32(520)
	detailsLabelW  = float32(110)
	detailsPad     = float32(16)
	detailsRowH    = float32(32)
)

func newScenarioDetailsPrompt(info world.ScenarioInfo, goals []world.Goal, rules []string, onOK func(world.ScenarioInfo, []world.Goal, []string), onCancel func()) *scenarioDetailsPrompt {
	p := &scenarioDetailsPrompt{info: info, goals: newGoalsTab(goals, rules), onOK: onOK, onCancel: onCancel}
	p.tabBtn[0] = ui.NewButton(0, 0, 110, detailsRowH, "Details", func() { p.tab = 0 })
	p.tabBtn[1] = ui.NewButton(0, 0, 110, detailsRowH, "Goals", func() { p.tab = 1 })
	p.tabBtn[2] = ui.NewButton(0, 0, 110, detailsRowH, "Guests", func() { p.tab = 2 })
	stepMix := func(v *float32, d, lo, hi float32) func() {
		return func() { *v = max(lo, min(hi, float32(math.Round(float64((*v+d)*100)))/100)) }
	}
	for i, st := range []struct {
		v         *float32
		d, lo, hi float32
	}{
		{&p.mix.MeanSize, 0.2, 1, 6},
		{&p.mix.Lessons, 0.01, 0, 0.5},
		{&p.mix.Mixed, 0.1, 0, 1},
	} {
		p.mixBtn[2*i] = ui.NewButton(0, 0, 26, detailsRowH, "<", stepMix(st.v, -st.d, st.lo, st.hi))
		p.mixBtn[2*i+1] = ui.NewButton(0, 0, 26, detailsRowH, ">", stepMix(st.v, st.d, st.lo, st.hi))
	}
	name := ui.NewTextInput(0, 0, 0, detailsRowH, info.Name)
	location := ui.NewTextInput(0, 0, 0, detailsRowH, info.Location)
	desc := ui.NewTextInput(0, 0, 0, 0, info.Description)
	desc.Multiline = true
	desc.MaxLen = 800
	p.fields = []*ui.TextInput{name, location, desc}
	for i, f := range p.fields {
		next := (i + 1) % len(p.fields)
		f.OnSubmit = func(string) { p.focus = next }
		f.OnCancel = func() { p.onCancel() }
	}

	step := func(v *int, d, max int) func() {
		return func() { *v = clampInt(*v+d, 0, max) }
	}
	p.diffDown = ui.NewButton(0, 0, 26, detailsRowH, "<", step(&p.info.Difficulty, -1, world.MaxScenarioDifficulty))
	p.diffUp = ui.NewButton(0, 0, 26, detailsRowH, ">", step(&p.info.Difficulty, 1, world.MaxScenarioDifficulty))
	p.orderDown = ui.NewButton(0, 0, 26, detailsRowH, "<", step(&p.info.Order, -1, maxScenarioOrder))
	p.orderUp = ui.NewButton(0, 0, 26, detailsRowH, ">", step(&p.info.Order, 1, maxScenarioOrder))
	p.tutorialBtn = ui.NewButton(0, 0, 130, detailsRowH, "", func() { p.info.Tutorial = !p.info.Tutorial })
	p.okBtn = ui.NewButton(0, 0, 90, detailsRowH, "OK", func() { p.onOK(p.result(), p.goals.goals, p.goals.rules) })
	p.cancelBtn = ui.NewButton(0, 0, 90, detailsRowH, "Cancel", func() { p.onCancel() })
	return p
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// result is the edited info with the text fields read back and trimmed.
func (p *scenarioDetailsPrompt) result() world.ScenarioInfo {
	info := p.info
	info.Name = strings.TrimSpace(p.fields[0].Text)
	info.Location = strings.TrimSpace(p.fields[1].Text)
	info.Description = strings.TrimSpace(p.fields[2].Text)
	return info
}

func (p *scenarioDetailsPrompt) buttons() []*ui.Button {
	out := []*ui.Button{p.tabBtn[0], p.tabBtn[1], p.okBtn, p.cancelBtn}
	switch p.tab {
	case 1:
		return append(out, p.goals.buttons()...)
	case 2:
		return append(out, p.mixBtn[:]...)
	}
	return append(out, p.diffDown, p.diffUp, p.orderDown, p.orderUp, p.tutorialBtn)
}

// stepperValueW is the width of the value slot between a stepper's arrows.
const stepperValueW = float32(40)

func (p *scenarioDetailsPrompt) layout(sw, sh float32) {
	p.x = (sw - detailsPromptW) / 2
	p.y = (sh - detailsPromptH) / 2
	fx := p.x + detailsPad + detailsLabelW
	fw := detailsPromptW - 2*detailsPad - detailsLabelW
	for i, f := range p.fields[:2] {
		f.X, f.Y, f.W = fx, p.y+50+float32(i)*(detailsRowH+10), fw
	}
	rowY := p.y + 50 + 2*(detailsRowH+10)
	p.diffDown.X, p.diffDown.Y = fx, rowY
	p.diffUp.X, p.diffUp.Y = fx+26+stepperValueW, rowY
	ox := p.diffUp.X + 26 + 100
	p.orderDown.X, p.orderDown.Y = ox, rowY
	p.orderUp.X, p.orderUp.Y = ox+26+stepperValueW, rowY
	p.tutorialBtn.X, p.tutorialBtn.Y = p.x+detailsPromptW-detailsPad-p.tutorialBtn.W, rowY
	if p.info.Tutorial {
		p.tutorialBtn.Label = "Tutorial: yes"
	} else {
		p.tutorialBtn.Label = "Tutorial: no"
	}

	desc := p.fields[2]
	desc.X, desc.Y = p.x+detailsPad, rowY+detailsRowH+36
	desc.W = detailsPromptW - 2*detailsPad
	p.okBtn.X = p.x + detailsPromptW - detailsPad - p.okBtn.W
	p.okBtn.Y = p.y + detailsPromptH - detailsPad - p.okBtn.H
	p.cancelBtn.X, p.cancelBtn.Y = p.okBtn.X-12-p.cancelBtn.W, p.okBtn.Y
	desc.H = p.okBtn.Y - 16 - desc.Y

	tx := p.x + detailsPromptW - detailsPad
	for i := len(p.tabBtn) - 1; i >= 0; i-- {
		tx -= p.tabBtn[i].W
		p.tabBtn[i].X, p.tabBtn[i].Y = tx, p.y+10
		p.tabBtn[i].SetActive(p.tab == i)
		tx -= 8
	}
	p.goals.layout(p.x+detailsPad, p.y+56, detailsPromptW-2*detailsPad)
	for i := 0; i < 3; i++ {
		y := p.y + 50 + float32(i)*(detailsRowH+10)
		p.mixBtn[2*i].X, p.mixBtn[2*i].Y = p.x+detailsPad+mixLabelW, y
		p.mixBtn[2*i+1].X, p.mixBtn[2*i+1].Y = p.x+detailsPad+mixLabelW+26+mixValueW, y
	}
}

// The Guests tab's label column and stepper value widths.
const (
	mixLabelW = float32(220)
	mixValueW = float32(70)
)

func (p *scenarioDetailsPrompt) HandleInput(inp *engine.Input, sw, sh float32) {
	p.layout(sw, sh)
	mx, my := inp.MousePos[0], inp.MousePos[1]
	click := inp.LeftClick
	if click {
		inp.LeftClickConsumed = true
	}
	if p.tab == 0 {
		if click {
			for i, f := range p.fields {
				if f.Contains(mx, my) {
					p.focus = i
				}
			}
		}
		if inp.Pressed[glfw.KeyTab] {
			p.focus = (p.focus + 1) % len(p.fields)
		} else {
			p.fields[p.focus].HandleInput(inp)
		}
		for i, f := range p.fields {
			f.HideCursor = i != p.focus
		}
	}
	for _, b := range p.buttons() {
		b.SetHovered(b.Contains(mx, my))
		if click && b.Contains(mx, my) {
			b.Click()
			return
		}
	}
}

func (p *scenarioDetailsPrompt) Draw(r *render.Renderer) {
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	p.layout(sw, sh)
	r.DrawColorRect(0, 0, sw, sh, mgl32.Vec4{0, 0, 0, 0.55})
	r.DrawColorRect(p.x, p.y, detailsPromptW, detailsPromptH, mgl32.Vec4{0.08, 0.12, 0.22, 0.98})
	for _, b := range p.buttons() {
		b.Draw(r)
	}
	if r.Font == nil {
		return
	}
	title := mgl32.Vec4{1, 0.95, 0.8, 1}
	r.Font.DrawText(r, "Scenario details", p.x+detailsPad, p.y+16, title)
	if p.tab == 1 {
		p.goals.draw(r, p.x+detailsPad, p.y+56)
		return
	}
	if p.tab == 2 {
		p.drawMix(r)
		return
	}
	for _, f := range p.fields {
		f.Draw(r)
	}
	label := mgl32.Vec4{0.8, 0.86, 0.95, 1}
	textOff := (detailsRowH - float32(render.GlyphH)) / 2
	lx := p.x + detailsPad
	r.Font.DrawText(r, "Name", lx, p.fields[0].Y+textOff, label)
	r.Font.DrawText(r, "Location", lx, p.fields[1].Y+textOff, label)
	rowY := p.diffDown.Y + textOff
	r.Font.DrawText(r, "Difficulty", lx, rowY, label)
	r.Font.DrawText(r, "Order", p.orderDown.X-60, rowY, label)
	stepperValue := func(v int, x float32) {
		s := "-"
		if v > 0 {
			s = fmt.Sprint(v)
		}
		r.Font.DrawText(r, s, x+26+(stepperValueW-r.Font.TextWidth(s))/2, rowY, mgl32.Vec4{1, 1, 1, 1})
	}
	stepperValue(p.info.Difficulty, p.diffDown.X)
	stepperValue(p.info.Order, p.orderDown.X)
	r.Font.DrawText(r, "Description", lx, p.fields[2].Y-float32(render.GlyphH)-8, label)
}

// drawMix draws the Guests tab: the group mix's steppers, and what
// they mean.
func (p *scenarioDetailsPrompt) drawMix(r *render.Renderer) {
	label := mgl32.Vec4{0.8, 0.86, 0.95, 1}
	white := mgl32.Vec4{1, 1, 1, 1}
	textOff := (detailsRowH - float32(render.GlyphH)) / 2
	lx := p.x + detailsPad
	rows := []struct{ name, value string }{
		{"Group size (average)", fmt.Sprintf("%.1f", p.mix.MeanSize)},
		{"Ski lessons (groups of 13)", fmt.Sprintf("%.0f%%", p.mix.Lessons*100)},
		{"Mixed-skill groups", fmt.Sprintf("%.0f%%", p.mix.Mixed*100)},
	}
	for i, row := range rows {
		b := p.mixBtn[2*i]
		r.Font.DrawText(r, row.name, lx, b.Y+textOff, label)
		r.Font.DrawText(r, row.value, b.X+26+(mixValueW-r.Font.TextWidth(row.value))/2, b.Y+textOff, white)
	}
	y := p.mixBtn[4].Y + detailsRowH + 24
	for _, line := range []string{
		"Guests come in groups of 1 to 13 that ski together. A group skis",
		"where its weakest skier can, and eats and goes home together.",
		"Groups that aren't mixed share one skill level. Lessons are beginners.",
		"Guests are regrouped when you press OK.",
	} {
		r.Font.DrawText(r, line, lx, y, label)
		y += float32(render.GlyphH) + 8
	}
}
