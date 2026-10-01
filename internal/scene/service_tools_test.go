package scene

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

func TestServicePick(t *testing.T) {
	rng.Init(1)
	w := world.NewWorld(world.NewTerrain(32, 32))
	b := w.PlaceServiceBuilding(map[[2]int]world.Service{{5, 5}: world.ServiceLounge}, 1)
	floor := w.ShellFloorY(b)
	none := mgl32.Vec3{}

	// Straight down onto the roof switches the tile.
	p := pickService(w, mgl32.Vec3{27.5, floor + 50, 27.5}, mgl32.Vec3{0, -1, 0}, world.ServiceBar, none, [2]int{}, false)
	if p.kind != pickRepaint || p.cell != [2]int{5, 5} || !p.legal {
		t.Fatalf("roof pick = %+v", p)
	}
	p = pickService(w, mgl32.Vec3{27.5, floor + 50, 27.5}, mgl32.Vec3{0, -1, 0}, world.ServiceLounge, none, [2]int{}, false)
	if p.legal {
		t.Error("repainting a tile its own service should be a no-op")
	}

	// Into the +x wall adds a tile on that side.
	p = pickService(w, mgl32.Vec3{60, floor + 2, 27.5}, mgl32.Vec3{-1, 0, 0}, world.ServiceFood, none, [2]int{}, false)
	if p.kind != pickAdd || p.bldg != b.ID || p.cell != [2]int{6, 5} || p.hitCell != [2]int{5, 5} {
		t.Fatalf("wall pick = %+v", p)
	}

	// Ground beside the building joins it; open ground starts a new one.
	down := mgl32.Vec3{0, -1, 0}
	p = pickService(w, mgl32.Vec3{22.5, 100, 32.5}, down, world.ServiceFood, mgl32.Vec3{22.5, 0, 32.5}, [2]int{4, 6}, true)
	if p.kind != pickNew {
		t.Fatalf("diagonal ground pick = %+v, want a new building", p)
	}
	p = pickService(w, mgl32.Vec3{27.5, 100, 32.5}, down, world.ServiceFood, mgl32.Vec3{27.5, 0, 32.5}, [2]int{5, 6}, false)
	if p.kind != pickNone {
		t.Fatalf("no ground hit: %+v", p)
	}
	w.SetTileService(b, [2]int{5, 6}, world.ServiceFood)
	if b.ServiceAt([2]int{5, 6}) != world.ServiceFood || w.Terrain.Cells[5][6].Passable {
		t.Fatal("added tile should be food and block walking")
	}
	p = pickService(w, mgl32.Vec3{27.5, 100, 42.5}, down, world.ServiceFood, mgl32.Vec3{27.5, 0, 42.5}, [2]int{5, 8}, true)
	if p.kind != pickNew {
		t.Fatalf("far ground = %+v", p)
	}
	p = pickService(w, mgl32.Vec3{27.5, 100, 37.5}, down, world.ServiceFood, mgl32.Vec3{27.5, 0, 37.5}, [2]int{5, 7}, true)
	if p.kind != pickAdd || p.bldg != b.ID {
		t.Fatalf("adjacent ground = %+v, want to join", p)
	}
}
