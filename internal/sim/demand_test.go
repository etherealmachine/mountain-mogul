package sim

import (
	"testing"
	"time"

	"mountain-mogul/internal/world"
)

func TestSeasonRolloverResetsVisitsThisSeason(t *testing.T) {
	guests := []*world.Guest{{ID: 1, VisitsThisSeason: 7}, {ID: 2, VisitsThisSeason: 3}}
	s := &Simulation{World: &world.World{Guests: guests, StartDate: world.DefaultStartDate}, Demand: NewDemandSystem()}

	// First poll mid-season only seeds the tracked season.
	s.SimTime = 20 * secondsPerSimDay
	s.Demand.maybePoll(s)
	if guests[0].VisitsThisSeason != 7 || guests[1].VisitsThisSeason != 3 {
		t.Fatalf("counters reset without a boundary: %d, %d", guests[0].VisitsThisSeason, guests[1].VisitsThisSeason)
	}

	// Step through the summer into the 2027-28 season (Dec 6, 2027).
	last := world.GameDayIndex(world.GameDayStart(2027, time.December, 6)) - world.GameDayIndex(world.DefaultStartDate)
	for day := 21.0; day <= float64(last); day++ {
		s.SimTime = day * secondsPerSimDay
		s.Demand.maybePoll(s)
	}
	if got := SeasonCloseYearFor(s.DateAt(s.SimTime)); got != 2028 {
		t.Fatalf("expected to be in the 2027-28 season, got close year %d", got)
	}
	for _, g := range guests {
		if g.VisitsThisSeason != 0 {
			t.Errorf("guest %d VisitsThisSeason = %d after rollover, want 0", g.ID, g.VisitsThisSeason)
		}
	}
}
