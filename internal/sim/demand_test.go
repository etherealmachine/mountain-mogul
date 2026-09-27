package sim

import (
	"testing"

	"mountain-mogul/internal/world"
)

func TestSeasonRolloverResetsVisitsThisSeason(t *testing.T) {
	guests := []*world.Guest{{ID: 1, VisitsThisSeason: 7}, {ID: 2, VisitsThisSeason: 3}}
	s := &Simulation{World: &world.World{Guests: guests}, Demand: NewDemandSystem()}

	// First poll mid-season only seeds the tracked season.
	s.SimTime = 100 * secondsPerSimDay
	s.Demand.maybePoll(s)
	if guests[0].VisitsThisSeason != 7 || guests[1].VisitsThisSeason != 3 {
		t.Fatalf("counters reset without a boundary: %d, %d", guests[0].VisitsThisSeason, guests[1].VisitsThisSeason)
	}

	// Step past the first season's close (2026-27 season is 188 days).
	for day := 101.0; day <= 200; day++ {
		s.SimTime = day * secondsPerSimDay
		s.Demand.maybePoll(s)
	}
	if got := SeasonCloseYearFor(DateAt(s.SimTime)); got != 2028 {
		t.Fatalf("expected to be in the 2027-28 season, got close year %d", got)
	}
	for _, g := range guests {
		if g.VisitsThisSeason != 0 {
			t.Errorf("guest %d VisitsThisSeason = %d after rollover, want 0", g.ID, g.VisitsThisSeason)
		}
	}
}
