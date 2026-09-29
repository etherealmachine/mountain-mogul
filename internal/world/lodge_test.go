package world

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

func rectCells(x0, z0, nx, nz int) [][2]int {
	var out [][2]int
	for x := x0; x < x0+nx; x++ {
		for z := z0; z < z0+nz; z++ {
			out = append(out, [2]int{x, z})
		}
	}
	return out
}

func countTiles(tiles []ShellTile) map[ShellTileKind]int {
	n := map[ShellTileKind]int{}
	for _, t := range tiles {
		n[t.Kind]++
	}
	return n
}

func TestRoofTileCoversEveryPattern(t *testing.T) {
	for m := 1; m < 16; m++ {
		o := [4]bool{m&1 != 0, m&2 != 0, m&4 != 0, m&8 != 0}
		if o == [4]bool{true, true, true, true} {
			continue // impossible: some corner is the minimum
		}
		if k, _ := roofTile(o); k == TileRoofFlat {
			t.Errorf("pattern %v fell back to flat", o)
		}
	}
}

func TestResolveRectangleShell(t *testing.T) {
	w := NewWorld(NewTerrain(20, 20))
	b := w.PlaceLodgeShell(rectCells(5, 5, 2, 1), 0) // 10 × 5 m: 4 × 2 tiles
	n := countTiles(ResolveLodgeShell(b))
	walls := n[TileWall] + n[TileWallWindow] + n[TileWallWindowAlt] + n[TileWallGlazed] + n[TileDoor]
	if walls != 12 || n[TileEave] != 12 {
		t.Fatalf("walls %d eaves %d, want 12 each (perimeter half-edges)", walls, n[TileEave])
	}
	if n[TileCornerOuter] != 4 || n[TileCornerInner] != 0 {
		t.Fatalf("corners outer %d inner %d, want 4 and 0", n[TileCornerOuter], n[TileCornerInner])
	}
	if n[TileRoofHip] != 4 || n[TileRoofSlope] != 4 {
		t.Fatalf("roof hips %d slopes %d, want 4 and 4", n[TileRoofHip], n[TileRoofSlope])
	}
}

func TestResolveLShapeHasValley(t *testing.T) {
	w := NewWorld(NewTerrain(20, 20))
	cells := append(rectCells(5, 5, 3, 1), rectCells(5, 6, 1, 2)...)
	b := w.PlaceLodgeShell(cells, 0)
	n := countTiles(ResolveLodgeShell(b))
	if n[TileCornerInner] != 1 || n[TileCornerOuter] != 5 {
		t.Fatalf("corners outer %d inner %d, want 5 and 1", n[TileCornerOuter], n[TileCornerInner])
	}
	if n[TileRoofValley] == 0 {
		t.Fatalf("L-shape roof has no valley tile: %v", n)
	}
}

func TestShellRoofHeightTracksWidth(t *testing.T) {
	w := NewWorld(NewTerrain(40, 40))
	peak := func(b *Building) float32 {
		var top float32
		for _, tl := range ResolveLodgeShell(b) {
			if tl.Kind.IsRoof() && tl.Pos[1] > top {
				top = tl.Pos[1]
			}
		}
		return top
	}
	narrow := peak(w.PlaceLodgeShell(rectCells(2, 2, 6, 1), 0))
	wide := peak(w.PlaceLodgeShell(rectCells(2, 10, 12, 12), 0))
	if narrow >= wide {
		t.Fatalf("narrow roof base %.1f not below wide %.1f", narrow, wide)
	}
	if want := ShellWallHeight + float32(ShellRoofMaxLevel)*ShellRoofRise; wide > want+1e-3 {
		t.Fatalf("wide roof %.1f above the cap %.1f", wide, want)
	}
}

func TestDoorsStayOnPerimeter(t *testing.T) {
	w := NewWorld(NewTerrain(20, 20))
	b := w.PlaceLodgeShell(rectCells(5, 5, 3, 3), 0)
	if w.ToggleDoor(b, [2]int{6, 6}) {
		t.Fatal("interior cell accepted a door")
	}
	if !w.ToggleDoor(b, [2]int{6, 5}) || b.Pos != cellCentre([2]int{6, 5}) {
		t.Fatalf("perimeter door not set as the anchor: doors %v pos %v", b.DoorCells, b.Pos)
	}
	// Growing the shell past the door moves it off the perimeter; it's dropped.
	w.SetShellCells(b, append(b.Cells, rectCells(5, 4, 3, 1)...))
	if len(b.DoorCells) != 0 || b.Usable() {
		t.Fatalf("buried door kept: %v", b.DoorCells)
	}
	for _, c := range b.Cells {
		if w.Terrain.Cells[c[0]][c[1]].Passable {
			t.Fatalf("shell cell %v is walkable", c)
		}
	}
	w.RemoveBuilding(b.ID)
	if !w.Terrain.Cells[6][6].Passable {
		t.Fatal("removing the lodge left its cells blocked")
	}
}

func TestLegacyLodgeConverts(t *testing.T) {
	w := NewWorld(NewTerrain(40, 40))
	w.PlaceLift(LiftDouble, 100, 160, 100, 20)
	b := w.PlaceBuildingType(BuildingLodge, 100, 100)
	if !b.IsShell() || len(b.Cells) != legacyLodgeCellsX*legacyLodgeCellsZ {
		t.Fatalf("legacy lodge cells = %d", len(b.Cells))
	}
	if len(b.DoorCells) != 1 || !b.IsPerimeterCell(b.DoorCells[0]) {
		t.Fatalf("legacy lodge doors = %v", b.DoorCells)
	}
	// The door faces the lift base to the south (+Z).
	if door := cellCentre(b.DoorCells[0]); door[1] < 100 {
		t.Fatalf("door at %v faces away from the lift base", door)
	}
	if b.Pos != cellCentre(b.DoorCells[0]) {
		t.Fatalf("anchor %v not on the door", b.Pos)
	}
}

func TestNearestEntrance(t *testing.T) {
	w := NewWorld(NewTerrain(20, 20))
	b := w.PlaceLodgeShell(rectCells(5, 5, 4, 1), 0)
	w.ToggleDoor(b, [2]int{5, 5})
	w.ToggleDoor(b, [2]int{8, 5})
	if _, c := b.NearestEntrance(mgl32.Vec2{60, 27}); c != [2]int{8, 5} {
		t.Fatalf("nearest door to the east = %v", c)
	}
}

func TestShellFloorsStayClear(t *testing.T) {
	w := NewWorld(NewTerrain(16, 16))
	w.Terrain.Cells[5][5].Top = SnowLayer{Accumulation: 0.3, Kind: KindPowder}
	w.PlaceLodgeShell([][2]int{{5, 5}, {6, 5}}, 1)
	if w.Terrain.Cells[5][5].Top.Accumulation != 0 {
		t.Fatal("painting a shell should clear the snow under it")
	}
	w.Terrain.Cells[6][5].Base = 0.5
	w.ClearLodgeFloors()
	if w.Terrain.Cells[6][5].Base != 0 {
		t.Fatal("ClearLodgeFloors left snow inside the lodge")
	}
}
