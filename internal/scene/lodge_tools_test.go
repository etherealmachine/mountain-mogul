package scene

import (
	"testing"

	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

func TestLodgePaintSession(t *testing.T) {
	rng.Init(1)
	w := world.NewWorld(world.NewTerrain(32, 32))
	var p lodgePaint
	p.start(0, lodgeEditShell, false)

	if created := p.addShellCell(w, [2]int{5, 5}); created == nil {
		t.Fatal("first cell should create the lodge")
	}
	for _, c := range [][2]int{{6, 5}, {7, 5}, {5, 6}, {6, 6}, {7, 6}} {
		if !p.shellAddable(w, c) {
			t.Fatalf("cell %v should be addable", c)
		}
		p.addShellCell(w, c)
	}
	b := p.lodge(w)
	if len(b.Cells) != 6 || p.shellAddable(w, [2]int{6, 6}) {
		t.Fatalf("cells = %d; painted cell still addable", len(b.Cells))
	}
	if w.Terrain.Cells[6][6].Passable {
		t.Error("shell cell still walkable")
	}

	p.mode = lodgeEditDoors
	if !p.lodgeHoverOK(w, [2]int{6, 6}) || w.ToggleDoor(b, [2]int{9, 9}) {
		t.Fatal("door rules: perimeter cell should take a door, outside cell shouldn't")
	}
	w.ToggleDoor(b, [2]int{6, 6})

	p.mode = lodgeEditFood
	if !p.setFoodCourt(w, [2]int{5, 5}, true) || p.setFoodCourt(w, [2]int{5, 5}, true) || p.setFoodCourt(w, [2]int{9, 9}, true) {
		t.Fatal("food court: shell cell once, never outside the shell")
	}
	if !b.ServesFood() || b.Seats() != world.FoodCourtSeatsPerCell {
		t.Fatalf("ServesFood=%v seats=%d", b.ServesFood(), b.Seats())
	}

	// Erasing the door's cell drops the door; the lodge stops serving.
	p.mode = lodgeEditShell
	p.removeShellCell(w, [2]int{6, 6})
	if len(b.DoorCells) != 0 || b.Usable() || !w.Terrain.Cells[6][6].Passable {
		t.Fatalf("doors=%v usable=%v after erasing the door cell", b.DoorCells, b.Usable())
	}
	p.removeShellCell(w, [2]int{5, 5})
	if len(b.FoodCourtCells) != 0 {
		t.Fatal("food court cell survived its shell cell")
	}
}
