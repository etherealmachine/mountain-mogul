package sim

import (
	"testing"

	"mountain-mogul/internal/world"
)

// TestParkingFeePaidOnArrival: each arriving guest pays their share of
// the per-car fee at the lot, pass holders included, and every
// GuestsPerCar arrivals together pay exactly one ParkingPrice even when
// it doesn't divide evenly.
func TestParkingFeePaidOnArrival(t *testing.T) {
	s, lot, _ := dayTicketWorld()
	w := s.World
	w.ParkingPrice = 30
	cash0 := w.Cash

	var shares []int
	for i := 0; i < 2*GuestsPerCar; i++ {
		g := dayTicketGuest(w, 200)
		if i == 0 {
			g.SeasonPassExpiry = s.SimTime + 1e6
		}
		before := w.Cash
		if !s.spawnGuest(lot, g) {
			t.Fatalf("spawn %d failed", i)
		}
		share := w.Cash - before
		shares = append(shares, share)
		if want := float32(200 - share - g.DayTicketDue); g.RemainingBudget != want {
			t.Fatalf("guest %d RemainingBudget = %v, want %v", i, g.RemainingBudget, want)
		}
	}
	if got, want := w.Cash-cash0, 2*w.ParkingPrice; got != want {
		t.Fatalf("parking collected %d (shares %v), want %d", got, shares, want)
	}
	if shares[0] == 0 {
		t.Fatal("pass holder parked for free")
	}
	if got := w.History.RevenueToday; got != 2*w.ParkingPrice {
		t.Fatalf("RevenueToday = %d, want %d", got, 2*w.ParkingPrice)
	}
}

// TestParkingFeeDampsDemand: a steep parking fee lowers the price factor,
// free parking leaves it untouched, and a fee that pushes the visit past
// the guest's budget keeps them home.
func TestParkingFeeDampsDemand(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(4, 4))
	w.DayTicketPrice = world.DayTicketReferencePrice
	g := dayTicketGuest(w, 100)

	free := visitPriceFactor(w, g, 0, 0.5)
	if free != 1 {
		t.Fatalf("free parking factor = %v, want 1", free)
	}
	w.ParkingPrice = world.ParkingReferencePrice
	if f := visitPriceFactor(w, g, 0, 0.5); f != 1 {
		t.Fatalf("reference parking factor = %v, want 1", f)
	}
	w.ParkingPrice = 100
	if f := visitPriceFactor(w, g, 0, 0.5); f <= 0 || f >= 1 {
		t.Fatalf("steep parking factor = %v, want in (0, 1)", f)
	}
	w.ParkingPrice = 200
	if f := visitPriceFactor(w, g, 0, 0.5); f != 0 {
		t.Fatalf("unaffordable parking factor = %v, want 0", f)
	}
}
