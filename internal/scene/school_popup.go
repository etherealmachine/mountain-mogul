//go:build !demo

package scene

import (
	"fmt"

	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// addSchoolControls adds a ski school room's settings to its popup: the
// lesson price, the instructors it employs (and what they cost), the
// guests waiting at its meeting spot for a class, and today's lessons.
func (s *Scenario) addSchoolControls(w *ui.Window, b *world.Building) {
	w.AddIntStepper("Lesson price ($)", &b.LessonPrice, 5, 0, 400)
	w.AddIntStepper("Instructors", &b.Instructors, 1, 0, 20)
	w.AddLabel("Instructors cost", func() string {
		return fmt.Sprintf("%s/day", formatDollars(b.Instructors*world.InstructorDailyCost))
	})
	w.AddLabel("Waiting for a class", func() string {
		n := 0
		for _, g := range s.world.OnMountain {
			if g.Lesson.SchoolID == b.ID && !g.Lesson.InClass {
				n++
			}
		}
		return fmt.Sprintf("%d", n)
	})
	w.AddLabel("Lessons today", func() string {
		if int(s.world.SimTime/world.SecondsPerSimDay) != b.LessonDay {
			return "0"
		}
		return fmt.Sprintf("%d (%d moved up a level)", b.LessonsToday, b.MovedUpToday)
	})
}
