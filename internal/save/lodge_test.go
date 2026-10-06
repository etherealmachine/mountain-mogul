package save

import (
	"reflect"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

func TestServiceBuildingRoundTrip(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(32, 32))
	b := w.PlaceServiceBuilding(mgl32.Vec2{}, 0, map[[2]int]world.Service{
		{5, 5}: world.ServiceFood, {5, 6}: world.ServiceFood,
		{6, 5}: world.ServiceBar, {6, 6}: world.ServiceLounge,
		{7, 5}: world.ServiceTickets,
	}, 42)
	b.MealPrice, b.DrinkPrice, b.FloorY = 24, 11, 3.5

	got := dataToWorld(worldToData(w, false))
	if len(got.Buildings) != 1 {
		t.Fatalf("buildings = %d, want 1", len(got.Buildings))
	}
	g := got.Buildings[0]
	if g.ID != b.ID || g.StyleSeed != 42 || g.MealPrice != 24 || g.DrinkPrice != 11 || g.FloorY != 3.5 {
		t.Fatalf("id/style/meal/drink/floor = %d/%d/%d/%d/%v", g.ID, g.StyleSeed, g.MealPrice, g.DrinkPrice, g.FloorY)
	}
	if !reflect.DeepEqual(g.TileMap(), b.TileMap()) {
		t.Errorf("tiles = %v, want %v", g.TileMap(), b.TileMap())
	}
	if !reflect.DeepEqual(g.Doors, b.Doors) || g.Pos != b.Pos {
		t.Errorf("doors %v pos %v, want %v %v", g.Doors, g.Pos, b.Doors, b.Pos)
	}
	if got.Terrain.Cells[6][5].Passable {
		t.Error("shell cell walkable after load")
	}
}
