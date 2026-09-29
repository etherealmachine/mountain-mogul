package save

import (
	"testing"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Loaded guests must keep their spending money: a zero DailyBudget prices
// every non-pass guest out of the day ticket and demand collapses.
func TestGuestBudgetRoundTrip(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(16, 16))
	world.SeedGuests(w, 1, 20)
	onMtn := w.Guests[0]
	onMtn.State = world.OnMountain
	onMtn.RemainingBudget = 37.5
	w.OnMountain = append(w.OnMountain, onMtn)

	got := dataToWorld(worldToData(w, false))
	if len(got.Guests) != len(w.Guests) {
		t.Fatalf("guests = %d, want %d", len(got.Guests), len(w.Guests))
	}
	for i, g := range got.Guests {
		if want := w.Guests[i].Traits.DailyBudget; g.Traits.DailyBudget != want || want <= 0 {
			t.Fatalf("guest %d DailyBudget = %v, want %v (> 0)", i, g.Traits.DailyBudget, want)
		}
	}
	if rb := got.Guests[0].RemainingBudget; rb != 37.5 {
		t.Fatalf("on-mountain RemainingBudget = %v, want 37.5", rb)
	}
}

// Saves from before the day/night clock had 240-second days; loading
// scales absolute sim times so the calendar date is unchanged.
func TestLegacyClockRescales(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(16, 16))
	world.SeedGuests(w, 1, 1)
	w.SimTime = 10.5 * world.LegacySecondsPerSimDay
	w.Guests[0].SeasonPassExpiry = 90 * world.LegacySecondsPerSimDay
	data := worldToData(w, false)
	data.DaySec = 0

	got := dataToWorld(data)
	if want := 10.5 * world.SecondsPerSimDay; got.SimTime != want {
		t.Errorf("SimTime = %v, want %v", got.SimTime, want)
	}
	if want := 90 * world.SecondsPerSimDay; got.Guests[0].SeasonPassExpiry != want {
		t.Errorf("SeasonPassExpiry = %v, want %v", got.Guests[0].SeasonPassExpiry, want)
	}

	w.OpenHour, w.CloseHour = 8.5, 17
	again := dataToWorld(worldToData(w, false))
	if again.SimTime != w.SimTime || again.OpenHour != 8.5 || again.CloseHour != 17 {
		t.Errorf("current-format round trip: SimTime %v hours %v-%v", again.SimTime, again.OpenHour, again.CloseHour)
	}
}

// Pools saved when Skill was a 0/1/2 tier enum re-roll into each tier's
// [0, 1] band on load.
func TestLegacySkillEnumMigrates(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(16, 16))
	data := worldToData(w, false)
	data.Guests = []GuestData{{ID: 1, Skill: 0}, {ID: 2, Skill: 1}, {ID: 3, Skill: 2}}
	got := dataToWorld(data)
	bands := [][2]float32{{0, ai.SkillIntermediateThreshold}, {ai.SkillIntermediateThreshold, ai.SkillAdvancedThreshold}, {ai.SkillAdvancedThreshold, 1}}
	for i, g := range got.Guests {
		s := g.Traits.Skill
		if s < bands[i][0] || s > bands[i][1] {
			t.Fatalf("tier %d skill = %v, want in %v", i, s, bands[i])
		}
		if g.Traits.DailyBudget != world.DailyBudgetFor(s) {
			t.Fatalf("tier %d budget = %v, want %v", i, g.Traits.DailyBudget, world.DailyBudgetFor(s))
		}
	}

	// Modern pools that happen to contain 0 and 1 are left alone.
	data.Guests = []GuestData{{ID: 1, Skill: 0}, {ID: 2, Skill: 1}, {ID: 3, Skill: 0.5}}
	got = dataToWorld(data)
	if got.Guests[1].Traits.Skill != 1 || got.Guests[2].Traits.Skill != 0.5 {
		t.Fatalf("modern skills changed: %v, %v", got.Guests[1].Traits.Skill, got.Guests[2].Traits.Skill)
	}
}
