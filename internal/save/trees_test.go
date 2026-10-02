package save

import (
	"testing"

	"mountain-mogul/internal/world"
)

func TestTreesRoundTrip(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(16, 16))
	in := []world.Tree{{X: 12.25, Z: 13.5}, {X: 14.75, Z: 11}, {X: 61.1, Z: 40.9}}
	for _, tr := range in {
		w.Terrain.AddTree(tr)
	}
	data := worldToData(w, false)
	for _, c := range data.Cells {
		if c.TreeDensity != 0 {
			t.Fatal("save wrote per-cell TreeDensity")
		}
	}
	got := dataToWorld(data).Terrain
	if got.TotalTrees() != len(in) {
		t.Fatalf("loaded %d trees, want %d", got.TotalTrees(), len(in))
	}
	for _, tr := range in {
		if !got.RemoveTree(tr) {
			t.Errorf("tree %v missing after load", tr)
		}
	}
}

// Old saves carry density per cell and lone trees as objects; both load
// as stored trees, the density ones exactly where the old rule drew them.
func TestLegacyTreesConvert(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(16, 16))
	data := worldToData(w, false)
	idx := func(x, z int) int { return x*16 + z }
	data.Cells[idx(3, 4)].TreeDensity = 1
	data.Cells[idx(5, 5)].TreeDensity = 0.4
	data.Objects = append(data.Objects,
		ObjectData{Type: uint8(world.ObjTree), X: 9, Z: 9},
		ObjectData{Type: uint8(world.ObjRock), X: 2, Z: 2})

	got := dataToWorld(data)
	ref := world.NewTerrain(16, 16)
	ref.SetCellTreesFromDensity(3, 4, 1)
	ref.SetCellTreesFromDensity(5, 5, 0.4)
	for _, c := range [][2]int{{3, 4}, {5, 5}} {
		a, b := got.Terrain.TreesInCell(c[0], c[1]), ref.TreesInCell(c[0], c[1])
		if len(a) != len(b) {
			t.Fatalf("cell %v: %d trees, want %d", c, len(a), len(b))
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("cell %v tree %d at %v, want %v", c, i, a[i], b[i])
			}
		}
	}
	if len(got.Terrain.TreesInCell(9, 9)) != 1 {
		t.Error("lone tree object did not become a stored tree")
	}
	if len(got.Objects) != 1 || got.Objects[0].Type != world.ObjRock {
		t.Errorf("objects after load = %v, want only the rock", got.Objects)
	}
}
