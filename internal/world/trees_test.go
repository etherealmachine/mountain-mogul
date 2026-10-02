package world

import "testing"

func checkTreeCounts(t *testing.T, ter *Terrain) {
	t.Helper()
	for x := 0; x < ter.Width; x++ {
		for z := 0; z < ter.Height; z++ {
			if got, want := int(ter.Cells[x][z].TreeCount), len(ter.TreesInCell(x, z)); got != want {
				t.Fatalf("cell (%d, %d): TreeCount %d, stored %d", x, z, got, want)
			}
		}
	}
}

func TestTreeCountTracksStore(t *testing.T) {
	ter := NewTerrain(10, 10)
	ter.AddTree(Tree{X: 12, Z: 13})
	ter.AddTree(Tree{X: 13.5, Z: 11})
	ter.AddTree(Tree{X: 31, Z: 7})
	if ter.AddTree(Tree{X: -1, Z: 5}) || ter.AddTree(Tree{X: 5, Z: 50}) {
		t.Fatal("AddTree accepted an off-map tree")
	}
	checkTreeCounts(t, ter)
	if ter.Cells[2][2].TreeCount != 2 || ter.TotalTrees() != 3 {
		t.Fatalf("cell (2,2) count %d, total %d", ter.Cells[2][2].TreeCount, ter.TotalTrees())
	}
	if !ter.RemoveTree(Tree{X: 12, Z: 13}) || ter.RemoveTree(Tree{X: 12, Z: 13}) {
		t.Fatal("RemoveTree should remove an exact match once")
	}
	if n := ter.RemoveTreesIn(0, 0, 9, 9, func(tr Tree) bool { return tr.X > 30 }); n != 1 {
		t.Fatalf("RemoveTreesIn removed %d, want 1", n)
	}
	checkTreeCounts(t, ter)
	ter.ClearAllTrees()
	checkTreeCounts(t, ter)
	if ter.TotalTrees() != 0 {
		t.Fatal("ClearAllTrees left trees")
	}
}

// Old saves convert through SetCellTreesFromDensity; it must draw the
// same number of trees the density rule always drew, inside the cell.
func TestSetCellTreesFromDensityMatchesRule(t *testing.T) {
	ter := NewTerrain(30, 30)
	for x := 0; x < 30; x++ {
		for z := 0; z < 30; z++ {
			d := float32((x*7+z*3)%11) / 10
			ter.SetCellTreesFromDensity(x, z, d)
			want := TreeCountFromDensity(d, TreeInstanceHash(x, z, -1))
			trees := ter.TreesInCell(x, z)
			if len(trees) != want {
				t.Fatalf("cell (%d, %d) density %v: %d trees, want %d", x, z, d, len(trees), want)
			}
			for _, tr := range trees {
				if int(tr.X/CellSize) != x || int(tr.Z/CellSize) != z {
					t.Fatalf("tree %v outside its cell (%d, %d)", tr, x, z)
				}
			}
		}
	}
	checkTreeCounts(t, ter)
}

func TestCanPlaceTreeSpacingAndEdges(t *testing.T) {
	ter := NewTerrain(10, 10)
	ter.AddTree(Tree{X: 20, Z: 20})
	if ter.CanPlaceTree(Tree{X: 21, Z: 20}, TreeMinSpacing) {
		t.Error("placed a tree 1 m from a trunk")
	}
	if !ter.CanPlaceTree(Tree{X: 23, Z: 20}, TreeMinSpacing) {
		t.Error("refused a tree 3 m from a trunk")
	}
	if ter.CanPlaceTree(Tree{X: 47, Z: 20}, TreeMinSpacing) {
		t.Error("placed a tree in the undrawn last column")
	}
	ter.Cells[1][1].Passable = false
	if ter.CanPlaceTree(Tree{X: 7, Z: 7}, TreeMinSpacing) {
		t.Error("placed a tree on an impassable cell")
	}
}

func TestTreeVisualsStableAcrossCopies(t *testing.T) {
	tr := Tree{X: 101.37, Z: 55.02}
	a := TreeInstanceOf(tr, 20, 11, 0)
	b := TreeInstanceOf(Tree{X: 101.37, Z: 55.02}, 20, 11, 0)
	if a != b {
		t.Fatalf("same position, different visuals: %+v vs %+v", a, b)
	}
	if a.Variant > 2 || a.Scale < 1.55 || a.Scale > 1.95 {
		t.Fatalf("visuals out of range: %+v", a)
	}
}

func TestWalkableAndCover(t *testing.T) {
	ter := NewTerrain(4, 4)
	ter.AddTree(Tree{X: 7, Z: 7})
	c := ter.Cells[1][1]
	if !c.Walkable() || c.TreeCover() != 0.5 {
		t.Fatalf("one trunk: walkable %v cover %v", c.Walkable(), c.TreeCover())
	}
	ter.AddTree(Tree{X: 9, Z: 9})
	c = ter.Cells[1][1]
	if c.Walkable() || c.TreeCover() != 1 {
		t.Fatalf("two trunks: walkable %v cover %v", c.Walkable(), c.TreeCover())
	}
}
