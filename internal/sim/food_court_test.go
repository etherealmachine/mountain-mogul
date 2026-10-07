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
	tiles := map[[2]int]world.Service{}
	for x := 10; x < 14; x++ {
		for z := 10; z < 13; z++ {
			tiles[[2]int{x, z}] = world.ServiceLounge
		}
	}
	tiles[[2]int{12, 10}] = world.ServiceFood
	tiles[[2]int{13, 10}] = world.ServiceFood
	b := w.PlaceServiceBuilding(mgl32.Vec2{}, 0, tiles, 1)
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
	if h := g.Plan.Head(); h.Kind != ai.ActUseService || h.Use != ai.OfferMeal {
		t.Fatalf("head step = %v, want a meal (goal %s)", h, g.Plan.GoalName)
	}
	if w.Cash != cash+20 || w.History.RevenueByKindToday[world.RevenueFood] != 20 {
		t.Fatalf("cash %d→%d, food revenue %d; want +20", cash, w.Cash, w.History.RevenueByKindToday[world.RevenueFood])
	}
	if g.RemainingBudget != 80 || b.InUse[world.PoolFoodSeats] != 1 {
		t.Fatalf("budget %v diners %d, want 80 and 1", g.RemainingBudget, b.InUse[world.PoolFoodSeats])
	}
	s.tickResting(g, float64(world.OfferDuration(ai.OfferMeal))+1)
	if g.Hunger != 1 {
		t.Fatalf("hunger after meal = %v, want 1", g.Hunger)
	}
}

func TestFullFoodCourtTurnsGuestsAway(t *testing.T) {
	s, b := foodCourtWorld(t)
	g := hungryGuestAtDoor(s, b)
	// Every seat taken and a line as long as the room holds.
	b.InUse[world.PoolFoodSeats] = b.Seats()
	b.Waiting[world.PoolFoodSeats] = b.Seats()
	s.replan(g)
	if h := g.Plan.Head(); h.Kind == ai.ActUseService && h.Use == ai.OfferMeal {
		t.Fatal("guest sat down in a full food court")
	}
}

func TestDoorlessLodgeIsUnreachable(t *testing.T) {
	s, b := foodCourtWorld(t)
	g := hungryGuestAtDoor(s, b)
	// Wall the building in so no outside wall opens onto walkable ground.
	for x := 9; x < 15; x++ {
		for z := 9; z < 14; z++ {
			if !b.HasCell([2]int{x, z}) {
				s.World.Terrain.Cells[x][z].Passable = false
			}
		}
	}
	s.World.RefreshDoors(b)
	if b.Usable() || b.ServesFood() {
		t.Fatal("lodge without doors should be unusable")
	}
	s.replan(g)
	if h := g.Plan.Head(); h.Kind == ai.ActUseService && h.Use == ai.OfferMeal {
		t.Fatal("guest ate in a lodge with no doors")
	}
}
