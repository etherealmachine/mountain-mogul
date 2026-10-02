package scene

import (
	"math/rand"
	"testing"

	"mountain-mogul/internal/world"
)

func forestedTerrain(n int) *world.Terrain {
	t := world.NewTerrain(n, n)
	for x := 0; x < n-1; x++ {
		for z := 0; z < n-1; z++ {
			t.SetCellTreesFromDensity(x, z, 1)
		}
	}
	return t
}

// The glade preview and the click must agree: removing the selection
// takes exactly those trees, and the next selection picks new ones.
func TestGladeSelectionIsWhatGetsRemoved(t *testing.T) {
	ter := forestedTerrain(20)
	before := ter.TotalTrees()
	sel := gladeSelection(ter, 10, 10, 2, 0.25)
	again := gladeSelection(ter, 10, 10, 2, 0.25)
	if len(sel) == 0 || len(sel) != len(again) {
		t.Fatalf("selection %d then %d trees", len(sel), len(again))
	}
	for i := range sel {
		if sel[i] != again[i] {
			t.Fatal("selection is not repeatable")
		}
	}
	under := 0
	ter.ForEachTreeNear(52.5, 52.5, gladeBrushMetres(2), func(world.Tree, int, int) { under++ })
	if want := (under + 3) / 4; len(sel) != want {
		t.Fatalf("25%% of %d trees selected %d, want %d", under, len(sel), want)
	}
	if gladeCost(sel) != len(sel)*world.GladeCostPerTree {
		t.Fatal("cost is not per tree")
	}
	removeTrees(ter, sel)
	if ter.TotalTrees() != before-len(sel) {
		t.Fatalf("removed %d trees, selected %d", before-ter.TotalTrees(), len(sel))
	}
	for _, tr := range gladeSelection(ter, 10, 10, 2, 0.25) {
		for _, gone := range sel {
			if tr == gone {
				t.Fatal("next selection includes a removed tree")
			}
		}
	}
	if n := len(gladeSelection(ter, 10, 10, 2, 1)); n != under-len(sel) {
		t.Fatalf("100%% share selected %d of %d remaining", n, under-len(sel))
	}
}

func TestPlantTreesUpToConverges(t *testing.T) {
	ter := world.NewTerrain(20, 20)
	rnd := rand.New(rand.NewSource(1)).Float32
	for i := 0; i < 6; i++ {
		plantTreesUpTo(ter, 10, 10, 3, 1, rnd)
	}
	filled := ter.TotalTrees()
	if filled == 0 {
		t.Fatal("plant brush placed nothing")
	}
	plantTreesUpTo(ter, 10, 10, 3, 1, rnd)
	plantTreesUpTo(ter, 10, 10, 3, 0.2, rnd)
	if ter.TotalTrees() != filled {
		t.Fatalf("painting a full area again changed %d → %d trees", filled, ter.TotalTrees())
	}
}

func TestGeneratedTreesKeepSpacing(t *testing.T) {
	ter := world.NewTerrain(40, 40)
	for x := 0; x < 40; x++ {
		for z := 0; z < 40; z++ {
			ter.Cells[x][z].GroundElevation = float32(x+z) * 0.5
		}
	}
	GenerateTreeCover(ter, 8, 0.9, 1, 7)
	if ter.TotalTrees() == 0 {
		t.Fatal("generator placed no trees")
	}
	min2 := world.TreeMinSpacing * world.TreeMinSpacing
	ter.ForEachStoredTree(func(a world.Tree) {
		ter.ForEachTreeNear(a.X, a.Z, world.TreeMinSpacing, func(b world.Tree, _, _ int) {
			if a == b {
				return
			}
			if dx, dz := a.X-b.X, a.Z-b.Z; dx*dx+dz*dz < min2 {
				t.Fatalf("trees %v and %v closer than %v m", a, b, world.TreeMinSpacing)
			}
		})
	})
	n := ter.TotalTrees()
	GenerateTreeCover(ter, 8, 0.9, 1, 7)
	if ter.TotalTrees() != n {
		t.Fatalf("same seed gave %d then %d trees", n, ter.TotalTrees())
	}
}
