package scene

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// slopedWorld returns a world whose ground falls `grade` metres per metre
// toward +Z.
func slopedWorld(n int, grade float32) *world.World {
	w := world.NewWorld(world.NewTerrain(n, n))
	for x := range w.Terrain.Cells {
		for z := range w.Terrain.Cells[x] {
			w.Terrain.Cells[x][z].GroundElevation = 200 - grade*5*float32(z)
		}
	}
	return w
}

// steepestStep returns the largest ground-elevation difference between
// 4-neighbour cells in rows z >= z0, over metres of run.
func steepestStep(t *world.Terrain, z0 int) float32 {
	var worst float32
	for x := 0; x < t.Width; x++ {
		for z := z0; z < t.Height; z++ {
			e := t.Cells[x][z].GroundElevation
			if x+1 < t.Width {
				worst = max(worst, abs32(e-t.Cells[x+1][z].GroundElevation)/5)
			}
			if z+1 < t.Height {
				worst = max(worst, abs32(e-t.Cells[x][z+1].GroundElevation)/5)
			}
		}
	}
	return worst
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func TestLiftBaseApronEmbankment(t *testing.T) {
	rng.Init(1)
	w := slopedWorld(64, 0.1)
	lift := &world.Lift{Base: mgl32.Vec2{160, 160}, Top: mgl32.Vec2{160, 40}}
	applyLiftPlacementEffects(w, lift)
	// Rows past the top station's shelf, which cuts sharply into the hill.
	if got := steepestStep(w.Terrain, 16); got > embankmentGrade+0.01 {
		t.Fatalf("steepest step %.2f exceeds embankment grade %.2f", got, embankmentGrade)
	}
}

func TestLevelLodgePadEmbankment(t *testing.T) {
	rng.Init(1)
	w := slopedWorld(48, 0.15)
	var cells [][2]int
	for x := 20; x < 26; x++ {
		for z := 20; z < 26; z++ {
			cells = append(cells, [2]int{x, z})
		}
	}
	b := &world.Building{Cells: cells}
	gradePaintedPad(w, b, 0)
	if got := steepestStep(w.Terrain, 0); got > embankmentGrade+0.01 {
		t.Fatalf("steepest step %.2f exceeds embankment grade %.2f", got, embankmentGrade)
	}
}
