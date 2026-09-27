package sim

import (
	"testing"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// dayTicketWorld is a slope with a parking lot at the bottom and one open
// double chair running up the fall line — enough for spawnGuest to lay a
// WalkToLift plan.
func dayTicketWorld() (*Simulation, *world.Building) {
	w := scene(40, 60).slope(10).
		parkingAt(20, 57).
		liftFromTo(20, 54, 20, 2).
		build()
	return NewSimulationWithSeed(w, 1), w.Buildings[0]
}

func dayTicketGuest(w *world.World, budget float32) *world.Guest {
	g := &world.Guest{ID: w.NextID(), State: world.AtHome}
	g.Traits.Skill = 0.9 // advanced: rides any lift without a matching trail
	g.Traits.DailyBudget = budget
	w.Guests = append(w.Guests, g)
	return g
}

func TestDayTicketCharge(t *testing.T) {
	cases := []struct {
		name      string
		price     int
		budget    float32
		passUntil float64 // SeasonPassExpiry; 0 = no pass
		wantPrice int
		wantOK    bool
	}{
		{"affordable", 60, 100, 0, 60, true},
		{"exact budget", 60, 60, 0, 60, true},
		{"too expensive", 60, 59, 0, 60, false},
		{"valid pass pays nothing", 60, 10, 1e9, 0, true},
		{"expired pass pays", 60, 100, 1, 60, true},
		{"free resort", 0, 0, 0, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := world.NewWorld(world.NewTerrain(4, 4))
			w.DayTicketPrice = c.price
			g := dayTicketGuest(w, c.budget)
			g.SeasonPassExpiry = c.passUntil
			price, ok := dayTicketCharge(w, g, 100)
			if price != c.wantPrice || ok != c.wantOK {
				t.Fatalf("dayTicketCharge = (%d, %v), want (%d, %v)", price, ok, c.wantPrice, c.wantOK)
			}
		})
	}
}

// TestSpawnChargesDayTicketOnce: an arriving guest pays the day ticket
// once at spawn; riding the chair afterwards costs nothing more.
func TestSpawnChargesDayTicketOnce(t *testing.T) {
	s, lot := dayTicketWorld()
	w := s.World
	g := dayTicketGuest(w, 100)
	cash0 := w.Cash

	if !s.spawnGuest(lot, g) {
		t.Fatal("spawnGuest failed")
	}
	if got := w.Cash - cash0; got != world.DefaultDayTicketPrice {
		t.Fatalf("cash delta at arrival = %d, want %d", got, world.DefaultDayTicketPrice)
	}
	if got := w.History.RevenueToday; got != world.DefaultDayTicketPrice {
		t.Fatalf("RevenueToday = %d, want %d", got, world.DefaultDayTicketPrice)
	}
	if want := float32(100 - world.DefaultDayTicketPrice); g.RemainingBudget != want {
		t.Fatalf("RemainingBudget = %v, want %v", g.RemainingBudget, want)
	}

	// Tick until the guest walks, queues, and boards. No further revenue
	// should appear.
	boarded := false
	for i := 0; i < 600 && !boarded; i++ {
		s.Tick(0.1)
		boarded = g.OnLiftID != 0
	}
	if !boarded {
		t.Fatal("guest never boarded the chair")
	}
	if got := w.Cash - cash0; got != world.DefaultDayTicketPrice {
		t.Fatalf("cash delta after riding = %d, want %d", got, world.DefaultDayTicketPrice)
	}
}

func TestSpawnPassHolderPaysNothing(t *testing.T) {
	s, lot := dayTicketWorld()
	w := s.World
	g := dayTicketGuest(w, 100)
	g.SeasonPassExpiry = s.SimTime + 1e6
	cash0 := w.Cash

	if !s.spawnGuest(lot, g) {
		t.Fatal("spawnGuest failed")
	}
	if w.Cash != cash0 || w.History.RevenueToday != 0 {
		t.Fatalf("pass holder charged: cash delta %d, revenue %d", w.Cash-cash0, w.History.RevenueToday)
	}
	if !g.HasSeasonPass || g.RemainingBudget != 100 || g.DayTicketPaid != 0 {
		t.Fatalf("pass holder state: HasSeasonPass=%v budget=%v paid=%d", g.HasSeasonPass, g.RemainingBudget, g.DayTicketPaid)
	}
}

// TestDayOfArrivalsRevenue: N arrivals in a day yield N × price.
func TestDayOfArrivalsRevenue(t *testing.T) {
	const n = 25
	s, lot := dayTicketWorld()
	w := s.World
	w.DayTicketPrice = 75
	for i := 0; i < n; i++ {
		if !s.spawnGuest(lot, dayTicketGuest(w, 200)) {
			t.Fatalf("spawn %d failed", i)
		}
	}
	if got := w.History.ArrivalsToday; got != n {
		t.Fatalf("ArrivalsToday = %d, want %d", got, n)
	}
	if got, want := w.History.RevenueToday, n*75; got != want {
		t.Fatalf("RevenueToday = %d, want %d", got, want)
	}
}

// TestDemandPollSkipsUnaffordable: over many polls, a guest whose budget
// is below the day ticket never arrives while an otherwise identical
// guest who can afford it does.
func TestDemandPollSkipsUnaffordable(t *testing.T) {
	s, _ := dayTicketWorld()
	w := s.World
	w.DayTicketPrice = 60
	// terrainMatch needs a trail at the guests' tier (skill 0.9 → black).
	trail := w.PlaceTrail("", world.DiffBlack)
	trail.Cells = [][2]int{{20, 30}, {20, 31}}
	poor := dayTicketGuest(w, 59)
	rich := dayTicketGuest(w, 61)
	poor.VisitsPerSeason, rich.VisitsPerSeason = 5000, 5000 // p ≈ 1 per poll

	richArrivals := 0
	for i := 0; i < 200; i++ {
		s.SimTime += demandPollInterval
		s.Demand.maybePoll(s)
		if poor.State != world.AtHome {
			t.Fatalf("poll %d: unaffordable guest arrived", i)
		}
		if rich.State == world.OnMountain {
			richArrivals++
			w.OnMountain = w.OnMountain[:0]
			rich.ResetForDeparture()
		}
	}
	if richArrivals == 0 {
		t.Fatal("affordable guest never arrived")
	}
}

// TestSeasonPassCreditsDayTicket: a guest who paid the day ticket and
// then buys a pass in the same visit pays only the difference.
func TestSeasonPassCreditsDayTicket(t *testing.T) {
	s, lot := dayTicketWorld()
	w := s.World
	office := w.PlaceBuildingType(world.BuildingTicketOffice, lot.Pos[0]+10, lot.Pos[1])
	g := dayTicketGuest(w, 200)
	if !s.spawnGuest(lot, g) {
		t.Fatal("spawnGuest failed")
	}
	cash0 := w.Cash
	g.Plan.Steps = []ai.PlanAction{{Kind: ai.ActBuySeasonPass, BldgID: office.ID}}
	g.Plan.Step = 0
	s.onPlanStepStart(g)
	if got, want := w.Cash-cash0, world.DefaultSeasonPassPrice-world.DefaultDayTicketPrice; got != want {
		t.Fatalf("pass purchase charged %d, want %d", got, want)
	}
	if !g.HasSeasonPass || g.DayTicketPaid != 0 {
		t.Fatalf("after purchase: HasSeasonPass=%v DayTicketPaid=%d", g.HasSeasonPass, g.DayTicketPaid)
	}
}
