package goap

import (
	"testing"

	"mountain-mogul/internal/world"
)

// TestSeasonPassCountsDayTicketCredit: a guest whose post-ticket budget
// is below the pass price still wants the pass when today's day ticket
// covers the difference.
func TestSeasonPassCountsDayTicketCredit(t *testing.T) {
	w, parking, _, _ := buildSmokeWorld(t)
	w.PlaceBuildingType(world.BuildingTicketOffice, parking.Pos[0]+10, parking.Pos[1])
	w.SeasonPassPrice = 150

	cases := []struct {
		name           string
		budget, credit float32
		wantWeight     bool
	}{
		{"credit covers gap", 90, 60, true},
		{"no credit", 90, 0, false},
		{"credit too small", 80, 60, false},
	}
	for _, c := range cases {
		s := WorldSnapshot{RemainingBudget: c.budget, PassCredit: c.credit}
		if got := (GetSeasonPass{}).Weight(&s, w) > 0; got != c.wantWeight {
			t.Errorf("%s: GetSeasonPass wanted=%v, want %v", c.name, got, c.wantWeight)
		}
	}

	s := WorldSnapshot{RemainingBudget: 90, PassCredit: 60}
	(&BuySeasonPass{}).Apply(&s, w)
	if s.RemainingBudget != 0 || s.PassCredit != 0 || !s.HasSeasonPass {
		t.Fatalf("after Apply: budget=%v credit=%v pass=%v, want 0 0 true", s.RemainingBudget, s.PassCredit, s.HasSeasonPass)
	}
}
