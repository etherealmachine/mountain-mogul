package sim

import (
	"testing"

	"mountain-mogul/internal/world"
)

// TestServiceSandboxServesEveryService: in the sandbox, each service run
// gets its own door and hungry and thirsty guests book food and bar
// revenue within ten minutes.
func TestServiceSandboxServesEveryService(t *testing.T) {
	tb, err := FindTestbed("Service buildings sandbox")
	if err != nil {
		t.Fatal(err)
	}
	w := tb.NewWorld()
	doors := map[world.Service]int{}
	for _, b := range w.Buildings {
		for _, d := range b.Doors {
			doors[d.Service]++
		}
	}
	for sv := world.ServiceLounge; sv < world.ServiceCount; sv++ {
		if doors[sv] != 1 {
			t.Errorf("%s doors = %d, want 1", sv.Label(), doors[sv])
		}
	}
	s := NewSimulationWithSeed(w, tb.Seed)
	for i := 0; i < 6000; i++ {
		s.Tick(0.1)
	}
	rev := w.History.RevenueByKindToday
	if rev[world.RevenueFood] == 0 || rev[world.RevenueBar] == 0 {
		t.Fatalf("food revenue %d, bar revenue %d; want both > 0", rev[world.RevenueFood], rev[world.RevenueBar])
	}
}
