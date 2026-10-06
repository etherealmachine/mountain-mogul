package world

import "fmt"

// Scenario goals and rules: what a scenario asks of the player, beyond
// the map. Required goals win the scenario when all are met; bonus goals
// are extra and never block the win. Goals are checked once a day at
// rollover (sim.CheckGoals). See notes/next/Scenario Goals and Rules.md.

// GoalKind is what a goal measures.
type GoalKind uint8

const (
	// GoalLiftsOpen: Target lifts open (and not on hold) at the end of a
	// day.
	GoalLiftsOpen GoalKind = iota
	// GoalGuestsInDay: Target guests arriving in one day.
	GoalGuestsInDay
	// GoalRating: the resort rating at or above Target (0–1) at the end
	// of Days days in a row.
	GoalRating
	// GoalCash: cash on hand at the end of a day at or above Target.
	GoalCash
	// GoalDebtFree: nothing drawn on the credit line (cash ≥ 0) at the
	// end of Days days in a row.
	GoalDebtFree
	// GoalAllRequired: every required goal met. A bonus goal with a
	// season deadline asks for the whole scenario within that season.
	GoalAllRequired
	NumGoalKinds
)

// Goal is one thing a scenario asks for.
type Goal struct {
	Kind   GoalKind
	Target float64 // lifts, guests, rating (0–1), or dollars; unused by GoalAllRequired
	Days   int     // days in a row for GoalRating and GoalDebtFree; 0 or 1 = once
	// Season, when set, is a deadline: the goal must be met by the end of
	// season Season (1 = the first). Missing it fails a bonus goal, and
	// loses the scenario for a required one. 0 = no deadline.
	Season int
	Bonus  bool
}

// GoalProgress is how a goal stands in a game. Kept in player saves,
// one per goal, in the same order.
type GoalProgress struct {
	Met    bool
	MetDay int // the day index (0 = the first day) it was met
	Streak int // days in a row the condition has held, for streak goals
	Best   float64
	Failed bool // its deadline passed before it was met
}

// Outcome is how the scenario stands: still playing, won (every required
// goal met; play carries on as a sandbox), or lost (a required goal's
// deadline passed, or bankruptcy).
type Outcome uint8

const (
	Playing Outcome = iota
	Won
	Lost
)

// Rule names, the switches a scenario can set.
const (
	RuleNoGrooming = "no_grooming"
)

// HasRule reports whether the scenario sets rule r.
func (w *World) HasRule(r string) bool {
	for _, x := range w.Rules {
		if x == r {
			return true
		}
	}
	return false
}

// Describe is the goal as the player reads it, e.g. "Get 3 lifts open".
func (g Goal) Describe() string {
	var s string
	switch g.Kind {
	case GoalLiftsOpen:
		s = fmt.Sprintf("Have %s open at the end of a day", plural(int(g.Target), "lift"))
	case GoalGuestsInDay:
		s = fmt.Sprintf("Welcome %s in one day", plural(int(g.Target), "guest"))
	case GoalRating:
		s = fmt.Sprintf("Keep the rating at %d%% or better", int(g.Target*100+0.5))
		if g.Days > 1 {
			s += fmt.Sprintf(" for %d days in a row", g.Days)
		}
	case GoalCash:
		s = fmt.Sprintf("End a day with $%s in the bank", commaInt(int(g.Target)))
	case GoalDebtFree:
		s = "Stay out of debt"
		if g.Days > 1 {
			s += fmt.Sprintf(" for %d days in a row", g.Days)
		}
	case GoalAllRequired:
		s = "Meet every goal"
	default:
		s = "Unknown goal"
	}
	if g.Season > 0 {
		s += fmt.Sprintf(" within season %d", g.Season)
	}
	return s
}

// plural is n and noun, with an s unless n is 1, e.g. "3 lifts".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return commaInt(n) + " " + noun + "s"
}

// commaInt formats n with thousands separators.
func commaInt(n int) string {
	s := fmt.Sprint(n)
	if n < 0 {
		return "-" + commaInt(-n)
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
