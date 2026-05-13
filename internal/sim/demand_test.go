package sim

import (
	"math"
	"testing"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// TestVisitProbability locks the multiplicative shape — any zero factor
// kills demand, full factors give full probability.
func TestVisitProbability(t *testing.T) {
	cases := []struct {
		name                   string
		rating, match, occ, want float32
	}{
		{"all-on", 1, 1, 0, 1},
		{"rating-half", 0.5, 1, 0, 0.5},
		{"no-terrain", 1, 0, 0, 0},
		{"full-occupancy", 1, 1, 1, 0},
		{"clamped-overflow", 1.5, 1, -0.2, 1},
	}
	for _, tc := range cases {
		got := visitProbability(tc.rating, tc.match, tc.occ)
		if math.Abs(float64(got-tc.want)) > 1e-5 {
			t.Errorf("%s: got %.3f want %.3f", tc.name, got, tc.want)
		}
	}
}

// TestScoreDepartureExtremes pins the score equation's endpoints — a
// perfect run lands at 1.0, a disastrous one at 0.0. Mid-cases just
// have to sit inside [0,1]; their exact values are tuning.
func TestScoreDepartureExtremes(t *testing.T) {
	perfect := &world.Agent{Fun: 1, Energy: 1, Events: []ai.AgentEvent{{Kind: ai.EventRun}, {Kind: ai.EventRun}}}
	if s := scoreDeparture(perfect); s < 0.999 {
		t.Errorf("perfect run scored %.3f, want ~1.0", s)
	}
	disaster := &world.Agent{Fun: 0, Energy: 0, Events: []ai.AgentEvent{
		{Kind: ai.EventRun}, {Kind: ai.EventFall}, {Kind: ai.EventFall},
	}}
	if s := scoreDeparture(disaster); s > 0.001 {
		t.Errorf("disaster scored %.3f, want ~0.0", s)
	}
}

// TestScoreDepartureNoRuns guards the divide-by-zero case — a session
// with no completed descents and no falls should still score the Fun
// and Energy axes; cleanness defaults to 1.
func TestScoreDepartureNoRuns(t *testing.T) {
	a := &world.Agent{Fun: 0.5, Energy: 0.5}
	got := scoreDeparture(a)
	want := float32(scoreWeightFun*0.5 + scoreWeightEnergy*0.5 + scoreWeightClean*1.0)
	if math.Abs(float64(got-want)) > 1e-5 {
		t.Errorf("zero-run session scored %.3f, want %.3f", got, want)
	}
}

// TestTerrainMatchBinary checks the skill → difficulty bit lookup
// against a single lift advertising green-only.
func TestTerrainMatchBinary(t *testing.T) {
	terrain := world.NewTerrain(20, 20)
	w := world.NewWorld(terrain)
	lift := w.PlaceLift(world.LiftDouble, 10, 10, 50, 10)
	lift.Services = world.DiffGreen

	if got := terrainMatch(w, ai.SkillBeginner); got != 1 {
		t.Errorf("beginner on green lift: got %.0f want 1", got)
	}
	if got := terrainMatch(w, ai.SkillIntermediate); got != 0 {
		t.Errorf("intermediate on green-only lift: got %.0f want 0", got)
	}
	if got := terrainMatch(w, ai.SkillAdvanced); got != 0 {
		t.Errorf("advanced on green-only lift: got %.0f want 0", got)
	}
}
