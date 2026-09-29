package sim

import (
	"strings"
	"testing"

	"mountain-mogul/internal/world"
)

func newEventTestSim(t *testing.T) *Simulation {
	t.Helper()
	w := world.NewWorld(world.NewTerrain(32, 32))
	return NewSimulationWithSeed(w, 1)
}

// eventsOfKind returns retained events of kind k, oldest-first.
func eventsOfKind(w *world.World, k world.EventKind) []world.Event {
	var out []world.Event
	for i := 0; i < w.Events.Len(); i++ {
		if e := w.Events.At(i); e.Kind == k {
			out = append(out, e)
		}
	}
	return out
}

func TestDaySummaryEventAtRollover(t *testing.T) {
	s := newEventTestSim(t)
	w := s.World
	w.History.ArrivalsToday = 7
	w.History.RevenueToday = 120

	s.SimTime = secondsPerSimDay + 1
	s.maybeSampleHistory()

	got := eventsOfKind(w, world.EventDaySummary)
	if len(got) != 1 {
		t.Fatalf("got %d day-summary events, want 1", len(got))
	}
	msg := got[0].Message
	if !strings.Contains(msg, "7 arrivals") || !strings.Contains(msg, "$120 in") {
		t.Fatalf("day summary message %q missing arrivals/revenue", msg)
	}
	if got[0].SimTime != s.SimTime {
		t.Fatalf("SimTime = %v, want %v", got[0].SimTime, s.SimTime)
	}

	// Crossing two more day boundaries in one call logs two summaries.
	s.SimTime = 3*secondsPerSimDay + 1
	s.maybeSampleHistory()
	if n := len(eventsOfKind(w, world.EventDaySummary)); n != 3 {
		t.Fatalf("after 3 rollovers got %d summaries, want 3", n)
	}
}

func TestDailySampleBreakdown(t *testing.T) {
	s := newEventTestSim(t)
	w := s.World
	w.PlaceLift(world.LiftDouble, 20, 20, 120, 120)
	s.openToday = true
	w.History.RecordRevenue(world.RevenueDayTickets, 300)
	w.History.RecordRevenue(world.RevenueParking, 40)
	wantCosts := w.OperatingCosts()

	s.SimTime = secondsPerSimDay + 1
	s.maybeSampleHistory()

	d := w.History.Ordered()[0]
	if !d.Open {
		t.Fatalf("sample not marked open")
	}
	if d.Revenue != 340 || d.RevenueByKind[world.RevenueDayTickets] != 300 || d.RevenueByKind[world.RevenueParking] != 40 {
		t.Fatalf("revenue = %d %v, want 340 split 300/40", d.Revenue, d.RevenueByKind)
	}
	if d.CostsByKind != wantCosts || d.Costs != wantCosts.Total() || d.CostsByKind[world.CostLifts] == 0 {
		t.Fatalf("costs = %d %v, want %v", d.Costs, d.CostsByKind, wantCosts)
	}

	// A closed day charges the standby rate.
	w.ResortOpen = false
	s.openToday = false
	s.SimTime = 2*secondsPerSimDay + 1
	s.maybeSampleHistory()
	if d := w.History.Ordered()[1]; d.Open || d.CostsByKind != w.StandbyCosts() {
		t.Fatalf("closed day = open %v costs %v, want standby %v", d.Open, d.CostsByKind, w.StandbyCosts())
	}
}

func TestLiftHoldEmitsEvents(t *testing.T) {
	s := newEventTestSim(t)
	w := s.World
	lift := w.PlaceLift(world.LiftDouble, 20, 20, 120, 120)
	base := lift.QueueCell()
	cell := &w.Terrain.Cells[base[0]][base[1]]
	cell.Base, cell.Top.Accumulation = 0, 0 // bare base station

	// Closed lift: hold transitions are not newsworthy.
	s.tickLifts(0.1)
	if w.Events.Len() != 0 {
		t.Fatalf("closed lift logged %d events, want 0", w.Events.Len())
	}

	// Open lift with no snow at the base: one "on hold" event, not repeated.
	lift.Open = true
	lift.OnHold = false
	s.tickLifts(0.1)
	s.tickLifts(0.1)
	closed := eventsOfKind(w, world.EventLiftClosed)
	if len(closed) != 1 {
		t.Fatalf("got %d lift-closed events, want 1", len(closed))
	}
	if e := closed[0]; !e.HasPos || e.Pos != lift.Base || e.EntityID != lift.ID {
		t.Fatalf("hold event = %+v, want pos %v id %d", e, lift.Base, lift.ID)
	}

	// Snow arrives at the base: hold lifts.
	cell.Top.Accumulation = 0.1
	s.tickLifts(0.1)
	if n := len(eventsOfKind(w, world.EventLiftOpened)); n != 1 {
		t.Fatalf("got %d lift-opened events, want 1", n)
	}
}

func TestPlayerActionEvents(t *testing.T) {
	s := newEventTestSim(t)
	w := s.World
	s.SimTime = 500

	b := w.PlaceBuildingType(world.BuildingBar, 30, 40)
	s.LogBuildingPlaced(b)
	lift := w.PlaceLift(world.LiftDouble, 20, 20, 120, 120)
	lift.Open = true
	s.LogLiftOpenChanged(lift)

	if w.Events.Len() != 2 {
		t.Fatalf("Len = %d, want 2", w.Events.Len())
	}
	built := w.Events.At(0)
	if built.Kind != world.EventBuildPlaced || built.Message != "Built Bar" ||
		built.Pos != b.Pos || built.EntityID != b.ID || built.SimTime != 500 {
		t.Fatalf("build event = %+v", built)
	}
	if opened := w.Events.At(1); opened.Kind != world.EventLiftOpened ||
		opened.Message != lift.Name+" opened" {
		t.Fatalf("open event = %+v", opened)
	}
}
