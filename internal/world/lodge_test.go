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
	b := w.PlaceLodgeShell(mgl32.Vec2{}, 0, rectCells(5, 5, 2, 1), 0) // 10 × 5 m: 4 × 2 tiles
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
	b := w.PlaceLodgeShell(mgl32.Vec2{}, 0, cells, 0)
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
	narrow := peak(w.PlaceLodgeShell(mgl32.Vec2{}, 0, rectCells(2, 2, 6, 1), 0))
	wide := peak(w.PlaceLodgeShell(mgl32.Vec2{}, 0, rectCells(2, 10, 12, 12), 0))
	if narrow >= wide {
		t.Fatalf("narrow roof base %.1f not below wide %.1f", narrow, wide)
	}
	if want := ShellLodge.WallHeight() + float32(ShellRoofMaxLevel)*ShellLodge.RoofRise(); wide > want+1e-3 {
		t.Fatalf("wide roof %.1f above the cap %.1f", wide, want)
	}
}

func TestAutoDoorsPerServiceRun(t *testing.T) {
	w := NewWorld(NewTerrain(40, 40))
	w.PlaceLift(LiftDouble, 100, 160, 100, 20)
	b := w.PlaceLodgeShell(mgl32.Vec2{}, 0, rectCells(18, 18, 3, 3), 0)
	if len(b.Doors) != 1 || b.Doors[0].Dir != [2]int{0, 1} {
		t.Fatalf("lounge doors = %v, want one facing the lift base (+Z)", b.Doors)
	}
	if b.Pos != cellCentre(b.Doors[0].Cell) {
		t.Fatalf("anchor %v not on the door", b.Pos)
	}
	// A food tile on the corner is its own run with its own door.
	w.SetTileService(b, [2]int{20, 20}, ServiceFood)
	if len(b.Doors) != 2 || !b.ServesFood() {
		t.Fatalf("doors = %v after adding a food tile", b.Doors)
	}
	// The centre tile has no outside wall: no door of its own, reached
	// through the others.
	w.SetTileService(b, [2]int{19, 19}, ServiceBar)
	if len(b.Doors) != 2 || !b.Offers(ServiceBar) {
		t.Fatalf("doors = %v with an enclosed bar", b.Doors)
	}
	if _, c := b.NearestServiceEntrance(ServiceFood, mgl32.Vec2{0, 0}); c != [2]int{20, 20} {
		t.Fatalf("food entrance = %v, want its own door", c)
	}
	for _, c := range b.Cells {
		if w.Terrain.Cells[c[0]][c[1]].Passable {
			t.Fatalf("shell cell %v is walkable", c)
		}
	}
	w.RemoveBuilding(b.ID)
	if !w.Terrain.Cells[19][19].Passable {
		t.Fatal("removing the building left its cells blocked")
	}
}

func TestTicketDoorFacesParking(t *testing.T) {
	w := NewWorld(NewTerrain(40, 40))
	w.PlaceLift(LiftDouble, 100, 160, 100, 20)
	w.PlaceRectLot(FootprintRect{Center: mgl32.Vec2{57.5, 97.5}, HalfX: 7.5, HalfZ: 7.5})
	b := w.PlaceServiceBuilding(mgl32.Vec2{}, 0, map[[2]int]Service{{18, 18}: ServiceTickets, {18, 19}: ServiceLounge}, 0)
	for _, d := range b.Doors {
		if d.Service == ServiceTickets && d.Dir != [2]int{-1, 0} {
			t.Fatalf("ticket door faces %v, want the lot to the west", d.Dir)
		}
	}
}

func TestPointLodge(t *testing.T) {
	w := NewWorld(NewTerrain(40, 40))
	w.PlaceLift(LiftDouble, 100, 160, 100, 20)
	b := w.PlaceBuildingType(BuildingLodge, 100, 100)
	if !b.IsShell() || len(b.Cells) != pointLodgeCellsX*pointLodgeCellsZ || !b.Offers(ServiceLounge) {
		t.Fatalf("legacy lodge cells = %d", len(b.Cells))
	}
	if len(b.Doors) != 1 || b.Doors[0].Dir != [2]int{0, 1} {
		t.Fatalf("legacy lodge doors = %v, want one facing the lift base", b.Doors)
	}
}

func TestPointBarAndOffice(t *testing.T) {
	w := NewWorld(NewTerrain(40, 40))
	bar := w.PlaceBuildingType(BuildingBar, 100, 100)
	office := w.PlaceBuildingType(BuildingTicketOffice, 150, 100)
	if bar.Type != BuildingLodge || !bar.Offers(ServiceBar) || bar.TileCount(ServiceBar) != len(bar.Cells) {
		t.Fatalf("bar converted to type %v with %d bar tiles", bar.Type, bar.TileCount(ServiceBar))
	}
	if office.Type != BuildingLodge || !office.Offers(ServiceTickets) {
		t.Fatalf("office converted to type %v", office.Type)
	}
}

func TestShellFloorsStayClear(t *testing.T) {
	w := NewWorld(NewTerrain(16, 16))
	w.Terrain.Cells[5][5].Top = SnowLayer{Accumulation: 0.3, Kind: KindPowder}
	w.PlaceLodgeShell(mgl32.Vec2{}, 0, [][2]int{{5, 5}, {6, 5}}, 1)
	if w.Terrain.Cells[5][5].Top.Accumulation != 0 {
		t.Fatal("painting a shell should clear the snow under it")
	}
	w.Terrain.Cells[6][5].Base = 0.5
	w.ClearLodgeFloors()
	if w.Terrain.Cells[6][5].Base != 0 {
		t.Fatal("ClearLodgeFloors left snow inside the lodge")
	}
}
