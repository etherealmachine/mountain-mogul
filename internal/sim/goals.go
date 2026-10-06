package sim

import (
	"fmt"
	"time"

	"mountain-mogul/internal/world"
)

// checkGoals updates each scenario goal from the day that just ended
// (dayIdx, summarised in sample), fails goals whose deadline has passed,
// and decides whether the scenario is won or lost. Goals met, missed, and
// the outcome go to the event feed. Called at every day rollover; does
// nothing for a world without goals. Goals keep updating after a win, so
// bonus goals can still be met.
func (s *Simulation) checkGoals(dayIdx int, sample world.DailySample) {
	w := s.World
	if len(w.Goals) == 0 {
		return
	}
	if len(w.GoalProgress) != len(w.Goals) {
		w.GoalProgress = make([]world.GoalProgress, len(w.Goals))
	}
	day := sample.Day
	at := float64(dayIdx+1) * secondsPerSimDay

	// Everything but "all required" first: it reads the others.
	for pass := 0; pass < 2; pass++ {
		for i, g := range w.Goals {
			if (g.Kind == world.GoalAllRequired) != (pass == 1) {
				continue
			}
			p := &w.GoalProgress[i]
			if p.Met || p.Failed {
				continue
			}
			if updateGoal(w, g, p, sample) {
				p.Met, p.MetDay = true, dayIdx
				w.LogEvent(world.EventGoal, at, goalLabel(g)+" met: "+g.Describe())
				continue
			}
			if g.Season > 0 && day.After(seasonDeadline(w, g.Season)) {
				p.Failed = true
				w.LogEvent(world.EventGoal, at, goalLabel(g)+" missed: "+g.Describe())
			}
		}
	}

	if w.Outcome != world.Playing {
		return
	}
	required, met := 0, 0
	for i, g := range w.Goals {
		if g.Bonus {
			continue
		}
		required++
		switch {
		case w.GoalProgress[i].Met:
			met++
		case w.GoalProgress[i].Failed:
			s.endScenario(world.Lost, dayIdx, at, "Scenario failed: "+g.Describe())
			return
		}
	}
	switch {
	case w.Bankrupt:
		s.endScenario(world.Lost, dayIdx, at, "Scenario failed: bankrupt")
	case required > 0 && met == required:
		s.endScenario(world.Won, dayIdx, at, "Scenario complete! Keep playing as long as you like.")
	}
}

func (s *Simulation) endScenario(o world.Outcome, dayIdx int, at float64, msg string) {
	w := s.World
	w.Outcome, w.OutcomeDay = o, dayIdx
	w.LogEvent(world.EventScenario, at, msg)
}

// updateGoal folds the day into g's progress and reports whether it's
// met now.
func updateGoal(w *world.World, g world.Goal, p *world.GoalProgress, sample world.DailySample) bool {
	streak := func(ok bool) bool {
		if ok {
			p.Streak++
		} else {
			p.Streak = 0
		}
		p.Best = max(p.Best, float64(p.Streak))
		return p.Streak >= max(g.Days, 1)
	}
	switch g.Kind {
	case world.GoalLiftsOpen:
		open := 0
		for _, l := range w.Lifts {
			if l.Open && !l.OnHold {
				open++
			}
		}
		p.Best = max(p.Best, float64(open))
		return float64(open) >= g.Target
	case world.GoalGuestsInDay:
		p.Best = max(p.Best, float64(sample.ArrivalsToday))
		return float64(sample.ArrivalsToday) >= g.Target
	case world.GoalRating:
		return streak(float64(w.Rating) >= g.Target)
	case world.GoalCash:
		p.Best = max(p.Best, float64(w.Cash))
		return float64(w.Cash) >= g.Target
	case world.GoalDebtFree:
		return streak(w.Cash >= 0)
	case world.GoalAllRequired:
		for i, o := range w.Goals {
			if !o.Bonus && !w.GoalProgress[i].Met {
				return false
			}
		}
		return true
	}
	return false
}

// seasonDeadline is the last day of season n (1 = the season the
// scenario starts in).
func seasonDeadline(w *world.World, n int) time.Time {
	return SeasonCloseDate(SeasonCloseYearFor(w.StartDate) + n - 1)
}

func goalLabel(g world.Goal) string {
	if g.Bonus {
		return "Bonus goal"
	}
	return "Goal"
}

// GoalStatus is a goal's progress in words, for the goals panel, e.g.
// "best 812 of 1,500".
func GoalStatus(w *world.World, i int) string {
	g, p := w.Goals[i], w.GoalProgress[i]
	switch {
	case p.Met:
		return "done"
	case p.Failed:
		return "missed"
	}
	switch g.Kind {
	case world.GoalLiftsOpen, world.GoalGuestsInDay, world.GoalCash:
		return fmt.Sprintf("best %.0f of %.0f", p.Best, g.Target)
	case world.GoalRating, world.GoalDebtFree:
		return fmt.Sprintf("%d of %d days", p.Streak, max(g.Days, 1))
	}
	return ""
}
