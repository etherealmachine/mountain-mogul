package scene

import (
	"path/filepath"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// goalsPanel is the scenario's goals, opened from the top bar's trophy:
// the scenario's name, place, and description, then each goal with its
// progress, required goals first and bonus goals after. When the
// scenario is won or lost it also shows, once, as the result panel:
// "Scenario complete" with Keep playing, or "Scenario failed" with Retry
// and Quit to menu.
type goalsPanel struct {
	open bool
	// result is the outcome being shown as the result panel, or Playing
	// for the plain goals view. shownOutcome is the last outcome shown,
	// so each is announced once.
	result       world.Outcome
	shownOutcome world.Outcome

	closeBtn, keepBtn, retryBtn, quitBtn *ui.Button
	x, y, w, h                           float32
}

const (
	goalsPanelW   = float32(620)
	goalsPad      = float32(18)
	goalsBarH     = float32(8)
	goalsRowGap   = float32(10)
	goalsTitleGap = float32(8)
)

var (
	goalsBg      = mgl32.Vec4{0.07, 0.09, 0.14, 0.97}
	goalsTitleBg = mgl32.Vec4{0.15, 0.20, 0.35, 1}
	goalsText    = mgl32.Vec4{0.92, 0.95, 1, 1}
	goalsDim     = mgl32.Vec4{0.62, 0.67, 0.78, 1}
	goalsDone    = mgl32.Vec4{0.40, 0.85, 0.50, 1}
	goalsMissed  = mgl32.Vec4{0.90, 0.40, 0.35, 1}
	goalsBarBg   = mgl32.Vec4{0.18, 0.22, 0.30, 1}
	goalsBarFill = mgl32.Vec4{0.55, 0.85, 1.00, 1}
)

// initGoalsPanel sets up the panel and the top bar's trophy button. Games
// without goals get neither.
func (s *Scenario) initGoalsPanel() {
	p := &s.goals
	p.shownOutcome = s.world.Outcome // a loaded game doesn't re-announce
	if len(s.world.Goals) == 0 {
		return
	}
	s.topBar.SetGoalsToggle(func() { s.toggleGoals() })
	p.closeBtn = ui.NewButton(0, 0, 110, 32, "Close", func() { s.toggleGoals() })
	p.keepBtn = ui.NewButton(0, 0, 150, 32, "Keep playing", func() { s.toggleGoals() })
	p.retryBtn = ui.NewButton(0, 0, 110, 32, "Retry", func() { s.retryScenario() })
	p.quitBtn = ui.NewButton(0, 0, 150, 32, "Quit to menu", func() { s.app.PopScene() })
}

// ShowGoals opens the goals panel, as the result panel when the game is
// already won or lost. For screenshots.
func (s *Scenario) ShowGoals() {
	p := &s.goals
	p.open, p.result = true, s.world.Outcome
	s.topBar.SetGoalsActive(true)
}

func (s *Scenario) toggleGoals() {
	p := &s.goals
	p.open = !p.open
	p.result = world.Playing
	s.topBar.SetGoalsActive(p.open)
}

// retryScenario starts the scenario this game came from again.
func (s *Scenario) retryScenario() {
	f := s.world.Scenario.File
	if f == "" {
		return
	}
	s.app.ReplaceScene(NewScenarioFromFile(filepath.Join(s.app.AssetDir, "scenarios", f)))
}

// pollGoalsOutcome opens the result panel the first time the scenario is
// won or lost, pausing the game.
func (s *Scenario) pollGoalsOutcome() {
	p := &s.goals
	o := s.world.Outcome
	if o == p.shownOutcome || o == world.Playing {
		return
	}
	p.shownOutcome = o
	p.open, p.result = true, o
	s.topBar.SetGoalsActive(true)
	s.pauseForResult()
}

// pauseForResult pauses the game behind the result panel.
func (s *Scenario) pauseForResult() {
	s.paused = true
	s.syncSpeedButtons()
}

// handleGoalsInput routes clicks on the panel, consuming any inside it.
func (s *Scenario) handleGoalsInput(inp *engine.Input) {
	p := &s.goals
	if !p.open {
		return
	}
	mx, my := inp.MousePos[0], inp.MousePos[1]
	for _, b := range s.goalsButtons() {
		b.SetHovered(b.Contains(mx, my))
		if inp.LeftClick && !inp.LeftClickConsumed && b.Contains(mx, my) {
			inp.LeftClickConsumed = true
			b.Click()
			return
		}
	}
	if inp.LeftClick && p.contains(mx, my) {
		inp.LeftClickConsumed = true
	}
}

func (p *goalsPanel) contains(x, y float32) bool {
	return p.open && x >= p.x && x <= p.x+p.w && y >= p.y && y <= p.y+p.h
}

// goalsButtons are the buttons the panel shows now.
func (s *Scenario) goalsButtons() []*ui.Button {
	p := &s.goals
	switch p.result {
	case world.Won:
		return []*ui.Button{p.keepBtn}
	case world.Lost:
		if s.world.Scenario.File != "" {
			return []*ui.Button{p.retryBtn, p.quitBtn}
		}
		return []*ui.Button{p.quitBtn}
	}
	return []*ui.Button{p.closeBtn}
}

// drawGoalsPanel draws the panel centred on screen.
func (s *Scenario) drawGoalsPanel(r *render.Renderer) {
	p := &s.goals
	if !p.open || r.Font == nil {
		return
	}
	w := s.world
	f := r.Font
	lineH := float32(render.GlyphH + 4)
	textW := goalsPanelW - 2*goalsPad
	wrap := func(t string, maxW float32) []string { return ui.WrapText(t, maxW, f.TextWidth) }

	title := w.Scenario.Name
	if title == "" {
		title = "Goals"
	}
	var headline string
	var headCol mgl32.Vec4
	switch p.result {
	case world.Won:
		headline, headCol = "Scenario complete!", goalsDone
	case world.Lost:
		headline, headCol = "Scenario failed", goalsMissed
	}
	// The result panel skips the description: the player knows it, and
	// the headline needs the room.
	var desc []string
	if p.result == world.Playing {
		desc = wrap(w.Scenario.Description, textW)
	}

	// Measure: title bar, headline, place, description, goals, buttons.
	type goalLine struct {
		i     int
		lines []string
	}
	var req, bonus []goalLine
	for i, g := range w.Goals {
		gl := goalLine{i, wrap(g.Describe(), textW-34-160)}
		if g.Bonus {
			bonus = append(bonus, gl)
		} else {
			req = append(req, gl)
		}
	}
	goalH := func(gl goalLine) float32 { return float32(len(gl.lines))*lineH + goalsBarH + 6 + goalsRowGap }
	h := lineH + 10 + goalsPad
	if headline != "" {
		h += lineH*1.4 + goalsTitleGap
	}
	if w.Scenario.Location != "" && p.result == world.Playing {
		h += lineH
	}
	h += float32(len(desc))*lineH + goalsTitleGap*2
	h += lineH + goalsTitleGap
	for _, gl := range req {
		h += goalH(gl)
	}
	if len(bonus) > 0 {
		h += lineH + goalsTitleGap
		for _, gl := range bonus {
			h += goalH(gl)
		}
	}
	h += 32 + goalsPad

	sw, sh := float32(r.ScreenWidth()), float32(r.ScreenHeight())
	p.w, p.h = goalsPanelW, h
	p.x, p.y = (sw-p.w)/2, max((sh-p.h)/2, 70)

	r.DrawColorRect(p.x, p.y, p.w, p.h, goalsBg)
	r.DrawColorRect(p.x, p.y, p.w, lineH+10, goalsTitleBg)
	f.DrawText(r, title, p.x+goalsPad, p.y+5, goalsText)
	y := p.y + lineH + 10 + goalsPad/2
	if headline != "" {
		f.DrawText(r, headline, p.x+goalsPad, y, headCol)
		y += lineH*1.4 + goalsTitleGap
	}
	if w.Scenario.Location != "" && p.result == world.Playing {
		f.DrawText(r, w.Scenario.Location, p.x+goalsPad, y, goalsDim)
		y += lineH
	}
	for _, l := range desc {
		f.DrawText(r, l, p.x+goalsPad, y, goalsText)
		y += lineH
	}
	y += goalsTitleGap * 2

	drawGoal := func(gl goalLine) {
		pr := w.GoalProgress[gl.i]
		col, fill := goalsText, goalsBarFill
		boxY := y + (float32(render.GlyphH)-14)/2
		switch {
		case pr.Met:
			col, fill = goalsDone, goalsDone
			r.DrawColorRect(p.x+goalsPad, boxY, 14, 14, goalsDone)
		case pr.Failed:
			col = goalsMissed
			r.DrawColorRect(p.x+goalsPad, boxY, 14, 14, goalsMissed)
		default:
			r.DrawColorRectOutline(p.x+goalsPad, boxY, 14, 14, goalsDim)
		}
		status := sim.GoalStatus(w, gl.i)
		f.DrawText(r, status, p.x+p.w-goalsPad-f.TextWidth(status), y, goalsDim)
		for _, l := range gl.lines {
			f.DrawText(r, l, p.x+goalsPad+34, y, col)
			y += lineH
		}
		barX, barW := p.x+goalsPad+34, textW-34
		r.DrawColorRect(barX, y+2, barW, goalsBarH, goalsBarBg)
		r.DrawColorRect(barX, y+2, barW*goalFraction(w, gl.i), goalsBarH, fill)
		y += goalsBarH + 6 + goalsRowGap
	}
	f.DrawText(r, "Goals", p.x+goalsPad, y, goalsDim)
	y += lineH + goalsTitleGap
	for _, gl := range req {
		drawGoal(gl)
	}
	if len(bonus) > 0 {
		f.DrawText(r, "Bonus", p.x+goalsPad, y, goalsDim)
		y += lineH + goalsTitleGap
		for _, gl := range bonus {
			drawGoal(gl)
		}
	}

	// Buttons, right-aligned along the bottom.
	bx := p.x + p.w - goalsPad
	for _, b := range s.goalsButtons() {
		bx -= b.W
		b.X, b.Y = bx, p.y+p.h-goalsPad-b.H
		b.Draw(r)
		bx -= 10
	}
}

// goalFraction is how far along goal i is, 0–1, for its bar.
func goalFraction(w *world.World, i int) float32 {
	g, p := w.Goals[i], w.GoalProgress[i]
	if p.Met {
		return 1
	}
	switch g.Kind {
	case world.GoalLiftsOpen, world.GoalGuestsInDay, world.GoalCash:
		if g.Target > 0 {
			return float32(min(p.Best/g.Target, 1))
		}
	case world.GoalRating, world.GoalDebtFree:
		return float32(min(float64(p.Streak)/float64(max(g.Days, 1)), 1))
	case world.GoalAllRequired:
		req, met := 0, 0
		for j, o := range w.Goals {
			if !o.Bonus {
				req++
				if w.GoalProgress[j].Met {
					met++
				}
			}
		}
		if req > 0 {
			return float32(met) / float32(req)
		}
	}
	return 0
}
