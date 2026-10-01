package save

import (
	"reflect"
	"testing"

	"mountain-mogul/internal/world"
)

func TestServiceBuildingRoundTrip(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(32, 32))
	b := w.PlaceServiceBuilding(map[[2]int]world.Service{
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

// Painted lodges from before service tiles keep their food court; the
// rest of the shell becomes lounge.
func TestPaintedLodgeLoadsAsTiles(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(32, 32))
	data := worldToData(w, false)
	data.Buildings = append(data.Buildings, BuildingData{
		ID: 3, Type: uint8(world.BuildingLodge),
		Cells:          [][2]int{{5, 5}, {6, 5}, {7, 5}},
		DoorCells:      [][2]int{{6, 5}},
		FoodCourtCells: [][2]int{{7, 5}},
	})
	g := dataToWorld(data).Buildings[0]
	if g.ServiceAt([2]int{7, 5}) != world.ServiceFood || g.ServiceAt([2]int{5, 5}) != world.ServiceLounge || !g.Usable() {
		t.Fatalf("tiles %v doors %v", g.TileMap(), g.Doors)
	}
}

// Lodges, bars and ticket offices saved as point meshes load as service
// buildings.
func TestLegacyMeshBuildingsLoadAsTiles(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(32, 32))
	data := worldToData(w, false)
	data.Buildings = append(data.Buildings,
		BuildingData{ID: 7, Type: uint8(world.BuildingLodge), X: 80, Z: 80},
		BuildingData{ID: 8, Type: uint8(world.BuildingBar), X: 40, Z: 40},
		BuildingData{ID: 9, Type: uint8(world.BuildingTicketOffice), X: 120, Z: 40},
	)
	got := dataToWorld(data)
	lodge, bar, office := got.Buildings[0], got.Buildings[1], got.Buildings[2]
	if !lodge.IsShell() || len(lodge.Cells) != 12 || !lodge.Usable() || lodge.ID != 7 || lodge.MealPrice != world.DefaultMealPrice {
		t.Fatalf("legacy lodge: shell=%v cells=%d doors=%d id=%d", lodge.IsShell(), len(lodge.Cells), len(lodge.Doors), lodge.ID)
	}
	if bar.ID != 8 || !bar.Offers(world.ServiceBar) || office.ID != 9 || !office.Offers(world.ServiceTickets) {
		t.Fatalf("bar %d offers bar %v; office %d offers tickets %v", bar.ID, bar.Offers(world.ServiceBar), office.ID, office.Offers(world.ServiceTickets))
	}
}
