package sim

import (
	"testing"

	"mountain-mogul/internal/world"
)

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
