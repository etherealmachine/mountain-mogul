package sim

import (
	"testing"

	"mountain-mogul/internal/world"
)

// catNightWorld is a groomed trail down a slope with a shed at the bottom.
// OpenHour/CloseHour are the resort defaults so the cat has a night shift.
func catNightWorld() (*Simulation, *world.Trail) {
	var cells [][2]int
	for x := 8; x <= 12; x++ {
		for z := 5; z <= 30; z++ {
			cells = append(cells, [2]int{x, z})
		}
	}
	w := scene(20, 40).slope(10).
		groomedRect(world.DiffGreen, 8, 5, 5, 26).
		shedAt(10, 35).
		build()
	w.OpenHour, w.CloseHour = world.DefaultOpenHour, world.DefaultCloseHour
	w.ResortOpen = true
	for _, c := range cells {
		cell := &w.Terrain.Cells[c[0]][c[1]]
		cell.Top.Accumulation = 0.5
		cell.Grooming = 1
	}
	return NewSimulationWithSeed(w, 1), w.Trails[0]
}

func runCats(s *Simulation, untilDay int, untilHour float64) {
	const dt = 0.25
	for end := SimTimeAt(untilDay, untilHour); s.SimTime < end; s.SimTime += dt {
		s.tickSnowcats(dt)
	}
}

// A skied lane gets groomed overnight even when the rest of the trail is
// untouched corduroy and the average stays high.
func TestSnowcatGroomsSkiedLaneNightly(t *testing.T) {
	s, trail := catNightWorld()
	w := s.World
	lane := 0
	for _, c := range trail.Cells {
		if c[0] == 10 {
			w.Terrain.Cells[c[0]][c[1]].Grooming = 0.1
			lane++
		}
	}
	s.SimTime = SimTimeAt(1, 17)
	runCats(s, 2, 8.5)
	for _, c := range trail.Cells {
		if g := w.Terrain.Cells[c[0]][c[1]].Grooming; g < 1 {
			t.Fatalf("cell %v Grooming = %v after the night shift, want 1", c, g)
		}
	}

	// One pass per night: re-wearing the lane before dawn doesn't send
	// the cat out again until the next night.
	s.SimTime = SimTimeAt(2, 3)
	for _, c := range trail.Cells {
		if c[0] == 10 {
			w.Terrain.Cells[c[0]][c[1]].Grooming = 0.2
		}
	}
	runCats(s, 2, 8)
	if g := w.Terrain.Cells[10][10].Grooming; g != 0.2 {
		t.Errorf("cat groomed twice in one night (Grooming %v)", g)
	}
	runCats(s, 3, 8)
	if g := w.Terrain.Cells[10][10].Grooming; g != 1 {
		t.Errorf("next night: Grooming %v, want 1", g)
	}
}

// Cats stay parked while the lifts are running, and untouched corduroy
// doesn't get a pass.
func TestSnowcatIdleWhenNotNeeded(t *testing.T) {
	s, trail := catNightWorld()
	w := s.World
	s.SimTime = SimTimeAt(1, 10)
	for _, c := range trail.Cells {
		w.Terrain.Cells[c[0]][c[1]].Grooming = 0.1
	}
	runCats(s, 1, 15)
	if g := w.Terrain.Cells[10][10].Grooming; g != 0.1 {
		t.Errorf("cat groomed during operating hours (Grooming %v)", g)
	}

	for _, c := range trail.Cells {
		w.Terrain.Cells[c[0]][c[1]].Grooming = 1
	}
	s.SimTime = SimTimeAt(1, 17)
	runCats(s, 1, 18)
	if cat := w.Snowcats[0]; len(cat.Route) > 0 {
		t.Errorf("cat out on a fully groomed trail (route %d cells)", len(cat.Route))
	}
}
