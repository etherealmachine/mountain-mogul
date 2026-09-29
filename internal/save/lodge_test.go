package save

import (
	"reflect"
	"testing"

	"mountain-mogul/internal/world"
)

func TestLodgeShellRoundTrip(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(32, 32))
	cells := [][2]int{{5, 5}, {6, 5}, {7, 5}, {5, 6}, {6, 6}, {7, 6}}
	b := w.PlaceLodgeShell(cells, 42)
	w.ToggleDoor(b, [2]int{6, 6})
	w.ToggleDoor(b, [2]int{7, 5})
	b.SetFoodCourtCells([][2]int{{5, 5}, {5, 6}})
	b.MealPrice = 24

	got := dataToWorld(worldToData(w, false))
	if len(got.Buildings) != 1 {
		t.Fatalf("buildings = %d, want 1", len(got.Buildings))
	}
	g := got.Buildings[0]
	if g.ID != b.ID || g.StyleSeed != 42 || g.MealPrice != 24 {
		t.Fatalf("id/style/meal = %d/%d/%d", g.ID, g.StyleSeed, g.MealPrice)
	}
	for name, pair := range map[string][2][][2]int{
		"cells": {g.Cells, b.Cells}, "doors": {g.DoorCells, b.DoorCells}, "food": {g.FoodCourtCells, b.FoodCourtCells},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("%s = %v, want %v", name, pair[0], pair[1])
		}
	}
	if g.Pos != b.Pos {
		t.Errorf("Pos = %v, want %v", g.Pos, b.Pos)
	}
	if got.Terrain.Cells[6][5].Passable {
		t.Error("shell cell walkable after load")
	}
}

// Lodges saved as a point mesh load as a converted shell with one door.
func TestLegacyLodgeLoadsAsShell(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(32, 32))
	data := worldToData(w, false)
	data.Buildings = append(data.Buildings, BuildingData{ID: 7, Type: uint8(world.BuildingLodge), X: 80, Z: 80})

	got := dataToWorld(data)
	b := got.Buildings[0]
	if !b.IsShell() || len(b.Cells) != 12 || len(b.DoorCells) != 1 || !b.Usable() {
		t.Fatalf("legacy lodge: shell=%v cells=%d doors=%d", b.IsShell(), len(b.Cells), len(b.DoorCells))
	}
	if b.ID != 7 || b.MealPrice != world.DefaultMealPrice {
		t.Fatalf("id %d meal %d", b.ID, b.MealPrice)
	}
}
