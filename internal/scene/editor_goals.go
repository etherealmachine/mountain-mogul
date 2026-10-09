package scene

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// The Goals tab of the editor's Scenario details dialog: one row per goal
// with its kind, target, streak days, season deadline, and whether it's a
// bonus, plus Add goal and the rule switches.

// goalKindNames are the editor's short names for each world.GoalKind.
var goalKindNames = [world.NumGoalKinds]string{
	world.GoalLiftsOpen:   "Lifts open",
	world.GoalGuestsInDay: "Guests in a day",
	world.GoalRating:      "Rating",
	world.GoalCash:        "Cash",
	world.GoalDebtFree:    "Debt free",
	world.GoalAllRequired: "All goals",
}

// goalTargetStep is how far one click moves a goal's target, and its
// default for a new goal of that kind.
func goalTargetStep(k world.GoalKind) (step, def float64) {
	switch k {
	case world.GoalLiftsOpen:
		return 1, 3
	case world.GoalGuestsInDay:
		return 100, 1000
	case world.GoalRating:
		return 0.1, 3
	case world.GoalCash:
		return 10000, 100000
	}
	return 0, 0
}

// goalHasTarget and goalHasDays say which steppers a kind uses.
func goalHasTarget(k world.GoalKind) bool { s, _ := goalTargetStep(k); return s > 0 }
func goalHasDays(k world.GoalKind) bool {
	return k == world.GoalRating || k == world.GoalDebtFree
}

// formatGoalTarget is a target as the editor shows it.
func formatGoalTarget(g world.Goal) string {
	switch g.Kind {
	case world.GoalRating:
		return world.FormatStars(float32(g.Target)) + " stars"
	case world.GoalCash:
		return "$" + world.CommaInt(int(g.Target))
	}
	return world.CommaInt(int(g.Target))
}

// goalRow is one goal's controls.
type goalRow struct {
	kindPrev, kindNext   *ui.Button
	targetDown, targetUp *ui.Button
	daysDown, daysUp     *ui.Button
	seasonDown, seasonUp *ui.Button
	bonusBtn, removeBtn  *ui.Button
	y                    float32
}

// goalsTab is the Goals tab's state: copies of the goals and rules, and
// the controls built over them.
type goalsTab struct {
	goals []world.Goal
	rules []string
	rows  []*goalRow

	addBtn, groomBtn *ui.Button
}

const (
	goalRowH     = float32(34)
	goalKindW    = float32(150)
	goalValueW   = float32(78)
	goalArrowW   = float32(24)
	maxGoals     = 8
	maxGoalDays  = 60
	maxGoalSeasn = 10
)

func newGoalsTab(goals []world.Goal, rules []string) *goalsTab {
	t := &goalsTab{goals: append([]world.Goal(nil), goals...), rules: append([]string(nil), rules...)}
	t.addBtn = ui.NewButton(0, 0, 130, detailsRowH, "Add goal", func() {
		if len(t.goals) < maxGoals {
			_, def := goalTargetStep(world.GoalLiftsOpen)
			t.goals = append(t.goals, world.Goal{Kind: world.GoalLiftsOpen, Target: def})
			t.rebuild()
		}
	})
	t.groomBtn = ui.NewButton(0, 0, 200, detailsRowH, "", func() { t.toggleRule(world.RuleNoGrooming) })
	t.rebuild()
	return t
}

func (t *goalsTab) hasRule(r string) bool {
	for _, x := range t.rules {
		if x == r {
			return true
		}
	}
	return false
}

func (t *goalsTab) toggleRule(r string) {
	for i, x := range t.rules {
		if x == r {
			t.rules = append(t.rules[:i], t.rules[i+1:]...)
			return
		}
	}
	t.rules = append(t.rules, r)
}

// rebuild makes a row of controls for each goal.
func (t *goalsTab) rebuild() {
	t.rows = t.rows[:0]
	for i := range t.goals {
		i := i
		g := func() *world.Goal { return &t.goals[i] }
		setKind := func(d int) func() {
			return func() {
				k := world.GoalKind((int(g().Kind) + d + int(world.NumGoalKinds)) % int(world.NumGoalKinds))
				_, def := goalTargetStep(k)
				g().Kind, g().Target = k, def
				g().Days = 0
				if goalHasDays(k) {
					g().Days = 7
				}
			}
		}
		target := func(d float64) func() {
			return func() {
				step, _ := goalTargetStep(g().Kind)
				v := g().Target + d*step
				lo := step
				if g().Kind == world.GoalRating {
					v, lo = min(v, world.MaxStars), 1
				}
				g().Target = max(v, lo)
			}
		}
		days := func(d int) func() { return func() { g().Days = clampInt(g().Days+d, 1, maxGoalDays) } }
		season := func(d int) func() { return func() { g().Season = clampInt(g().Season+d, 0, maxGoalSeasn) } }
		r := &goalRow{
			kindPrev:   ui.NewButton(0, 0, goalArrowW, goalRowH-4, "<", setKind(-1)),
			kindNext:   ui.NewButton(0, 0, goalArrowW, goalRowH-4, ">", setKind(1)),
			targetDown: ui.NewButton(0, 0, goalArrowW, goalRowH-4, "-", target(-1)),
			targetUp:   ui.NewButton(0, 0, goalArrowW, goalRowH-4, "+", target(1)),
			daysDown:   ui.NewButton(0, 0, goalArrowW, goalRowH-4, "-", days(-1)),
			daysUp:     ui.NewButton(0, 0, goalArrowW, goalRowH-4, "+", days(1)),
			seasonDown: ui.NewButton(0, 0, goalArrowW, goalRowH-4, "-", season(-1)),
			seasonUp:   ui.NewButton(0, 0, goalArrowW, goalRowH-4, "+", season(1)),
			bonusBtn:   ui.NewButton(0, 0, 92, goalRowH-4, "", func() { g().Bonus = !g().Bonus }),
			removeBtn: ui.NewButton(0, 0, 30, goalRowH-4, "x", func() {
				t.goals = append(t.goals[:i], t.goals[i+1:]...)
				t.rebuild()
			}),
		}
		t.rows = append(t.rows, r)
	}
}

// Column x offsets from the panel's left padding.
const (
	goalColTarget = goalArrowW*2 + goalKindW + 16
	goalColDays   = goalColTarget + goalArrowW*2 + goalValueW + 16
	goalColSeason = goalColDays + goalArrowW*2 + 40 + 16
	goalColBonus  = goalColSeason + goalArrowW*2 + 40 + 16
)

func (t *goalsTab) layout(x, y, w float32) {
	for i, r := range t.rows {
		r.y = y + 30 + float32(i)*goalRowH
		by := r.y + 2
		r.kindPrev.X, r.kindPrev.Y = x, by
		r.kindNext.X, r.kindNext.Y = x+goalArrowW+goalKindW, by
		r.targetDown.X, r.targetDown.Y = x+goalColTarget, by
		r.targetUp.X, r.targetUp.Y = x+goalColTarget+goalArrowW+goalValueW, by
		r.daysDown.X, r.daysDown.Y = x+goalColDays, by
		r.daysUp.X, r.daysUp.Y = x+goalColDays+goalArrowW+40, by
		r.seasonDown.X, r.seasonDown.Y = x+goalColSeason, by
		r.seasonUp.X, r.seasonUp.Y = x+goalColSeason+goalArrowW+40, by
		r.bonusBtn.X, r.bonusBtn.Y = x+goalColBonus, by
		r.removeBtn.X, r.removeBtn.Y = x+w-r.removeBtn.W, by
		if t.goals[i].Bonus {
			r.bonusBtn.Label = "Bonus"
		} else {
			r.bonusBtn.Label = "Required"
		}
	}
	ay := y + 30 + float32(len(t.rows))*goalRowH + 10
	t.addBtn.X, t.addBtn.Y = x, ay
	t.groomBtn.X, t.groomBtn.Y = x, ay+detailsRowH+80
	if t.hasRule(world.RuleNoGrooming) {
		t.groomBtn.Label = "No grooming: on"
	} else {
		t.groomBtn.Label = "No grooming: off"
	}
}

// buttons are the controls the tab shows now; a kind's unused steppers
// are left out.
func (t *goalsTab) buttons() []*ui.Button {
	var out []*ui.Button
	for i, r := range t.rows {
		k := t.goals[i].Kind
		out = append(out, r.kindPrev, r.kindNext)
		if goalHasTarget(k) {
			out = append(out, r.targetDown, r.targetUp)
		}
		if goalHasDays(k) {
			out = append(out, r.daysDown, r.daysUp)
		}
		out = append(out, r.seasonDown, r.seasonUp, r.bonusBtn, r.removeBtn)
	}
	if len(t.goals) < maxGoals {
		out = append(out, t.addBtn)
	}
	return append(out, t.groomBtn)
}

// draw draws the tab's labels and values; the buttons draw themselves.
func (t *goalsTab) draw(r *render.Renderer, x, y float32) {
	if r.Font == nil {
		return
	}
	f := r.Font
	label := mgl32.Vec4{0.8, 0.86, 0.95, 1}
	white := mgl32.Vec4{1, 1, 1, 1}
	dim := mgl32.Vec4{0.55, 0.6, 0.7, 1}
	head := func(s string, cx float32) { f.DrawText(r, s, x+cx, y, label) }
	head("Goal", 0)
	head("Target", goalColTarget)
	head("Days", goalColDays)
	head("Season", goalColSeason)
	textOff := (goalRowH - float32(render.GlyphH)) / 2
	centre := func(s string, cx, w float32, col mgl32.Vec4, ry float32) {
		f.DrawText(r, s, x+cx+(w-f.TextWidth(s))/2, ry+textOff, col)
	}
	for i, row := range t.rows {
		g := t.goals[i]
		centre(goalKindNames[g.Kind], goalArrowW, goalKindW, white, row.y)
		if goalHasTarget(g.Kind) {
			centre(formatGoalTarget(g), goalColTarget+goalArrowW, goalValueW, white, row.y)
		}
		if goalHasDays(g.Kind) {
			centre(fmt.Sprint(max(g.Days, 1)), goalColDays+goalArrowW, 40, white, row.y)
		}
		s := "-"
		if g.Season > 0 {
			s = fmt.Sprint(g.Season)
		}
		centre(s, goalColSeason+goalArrowW, 40, white, row.y)
	}
	if len(t.goals) == 0 {
		f.DrawText(r, "No goals: the scenario plays as a sandbox.", x, y+30+textOff, dim)
	}
	f.DrawText(r, "Season is a deadline: the goal must be met by the end of that season. \"-\" is none.", x, t.addBtn.Y+detailsRowH+4, dim)
	f.DrawText(r, "Rules", x, t.groomBtn.Y-float32(render.GlyphH)-4, label)
}

// ShowGoalsTab opens the Scenario details dialog on its Goals tab. For
// screenshots.
func (e *Editor) ShowGoalsTab() {
	e.openDetailsPrompt()
	e.detailsPrompt.tab = 1
}
