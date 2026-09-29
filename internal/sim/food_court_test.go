package sim

import (
	"testing"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"

	"github.com/go-gl/mathgl/mgl32"
)

func foodCourtWorld(t *testing.T) (*Simulation, *world.Building) {
	t.Helper()
	s := newEventTestSim(t)
	w := s.World
	var cells [][2]int
	for x := 10; x < 14; x++ {
		for z := 10; z < 13; z++ {
			cells = append(cells, [2]int{x, z})
		}
	}
	b := w.PlaceLodgeShell(cells, 1)
	w.ToggleDoor(b, [2]int{11, 12})
	b.SetFoodCourtCells([][2]int{{12, 10}, {13, 10}})
	b.MealPrice = 20
	return s, b
}

func hungryGuestAtDoor(s *Simulation, b *world.Building) *world.Guest {
	g := &world.Guest{ID: 99, Patience: 1, Energy: 1, Hunger: 0.1, Thirst: 1, Satisfaction: 0.5, RemainingBudget: 100, HasSeasonPass: true}
	p, _ := b.NearestEntrance(mgl32.Vec2{})
	g.Pos = mgl32.Vec3{p[0], 0, p[1]}
	s.World.OnMountain = append(s.World.OnMountain, g)
	return g
}

func TestHungryGuestEatsAtFoodCourt(t *testing.T) {
	s, b := foodCourtWorld(t)
	w := s.World
	g := hungryGuestAtDoor(s, b)
	cash := w.Cash

	s.replan(g)
	if k := g.Plan.Head().Kind; k != ai.ActEat {
		t.Fatalf("head step = %v, want ActEat (goal %s)", k, g.Plan.GoalName)
	}
	if w.Cash != cash+20 || w.History.RevenueByKindToday[world.RevenueFood] != 20 {
		t.Fatalf("cash %d→%d, food revenue %d; want +20", cash, w.Cash, w.History.RevenueByKindToday[world.RevenueFood])
	}
	if g.RemainingBudget != 80 || b.Diners != 1 {
		t.Fatalf("budget %v diners %d, want 80 and 1", g.RemainingBudget, b.Diners)
	}
	s.tickResting(g, mealSec+1)
	if g.Hunger != 1 {
		t.Fatalf("hunger after meal = %v, want 1", g.Hunger)
	}
}

func TestFullFoodCourtTurnsGuestsAway(t *testing.T) {
	s, b := foodCourtWorld(t)
	g := hungryGuestAtDoor(s, b)
	b.Diners = b.Seats()
	s.replan(g)
	if g.Plan.Head().Kind == ai.ActEat {
		t.Fatal("guest sat down in a full food court")
	}
}

func TestDoorlessLodgeIsUnreachable(t *testing.T) {
	s, b := foodCourtWorld(t)
	g := hungryGuestAtDoor(s, b)
	s.World.ToggleDoor(b, [2]int{11, 12})
	if b.Usable() || b.ServesFood() {
		t.Fatal("lodge without doors should be unusable")
	}
	s.replan(g)
	if g.Plan.Head().Kind == ai.ActEat {
		t.Fatal("guest ate in a lodge with no doors")
	}
}
