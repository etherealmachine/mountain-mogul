package sim

import (
	"fmt"
	"math"

	"mountain-mogul/internal/world"
)

// applyCredit runs the credit line for the rollover that closes day dayIdx:
// accrues interest on the drawn balance for every calendar day until the
// next sim day (normally 1; the whole closed gap across the off-season),
// charges the accrued total to Cash when the next day is in a new month,
// and advances the bankruptcy counter. Returns the interest charged in
// dollars (0 on non-billing days) so the day's sample counts it as a cost.
func (s *Simulation) applyCredit(dayIdx int) int {
	w := s.World
	today := DateAt(float64(dayIdx) * secondsPerSimDay)
	next := DateAt(float64(dayIdx+1) * secondsPerSimDay)
	days := int(math.Round(next.Sub(today).Hours() / 24))

	w.AccruedInterest += float64(w.CreditDrawn()) * world.CreditAnnualRate / 365 * float64(days)

	charged := 0
	if next.Month() != today.Month() || next.Year() != today.Year() {
		charged = int(math.Round(w.AccruedInterest))
		w.AccruedInterest = 0
		if charged > 0 {
			w.Cash -= charged
			// Across the off-season gap one bill covers every closed month.
			period := today.Format("Jan")
			if last := next.AddDate(0, 0, -1); last.Month() != today.Month() {
				period += "–" + last.Format("Jan")
			}
			w.LogEvent(world.EventFinance, s.SimTime, fmt.Sprintf(
				"%s interest charged: $%d on $%d drawn", period, charged, w.CreditDrawn()))
		}
	}

	if w.Bankrupt {
		return charged
	}
	if w.Cash < -w.CreditLimit {
		w.DaysBelowFloor++
		switch left := world.BankruptcyGraceDays - w.DaysBelowFloor; {
		case left <= 0:
			w.Bankrupt = true
			w.LogEvent(world.EventFinance, s.SimTime, fmt.Sprintf(
				"Bankrupt: cash below the credit floor for %d days", world.BankruptcyGraceDays))
		case w.DaysBelowFloor == 1:
			w.LogEvent(world.EventFinance, s.SimTime, fmt.Sprintf(
				"Cash below the credit floor: %d days to recover", left))
		case left == 7:
			w.LogEvent(world.EventFinance, s.SimTime, "Cash below the credit floor: 7 days to recover")
		}
	} else if w.DaysBelowFloor > 0 {
		w.DaysBelowFloor = 0
		w.LogEvent(world.EventFinance, s.SimTime, "Cash back above the credit floor")
	}
	return charged
}
