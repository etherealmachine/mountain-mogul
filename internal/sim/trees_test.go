package sim

import (
	"math"
	"testing"

	"mountain-mogul/internal/world"
)

func TestHitsTrunk(t *testing.T) {
	ter := world.NewTerrain(10, 10)
	ter.AddTree(world.Tree{X: 20, Z: 20.4})
	g := &world.Guest{Pos: [3]float32{20, 0, 20}, Speed: 8}

	g.Heading = 0 // +Z, toward the trunk
	if !hitsTrunk(ter, g) {
		t.Error("skiing into a trunk 0.4 m ahead didn't hit")
	}
	g.Heading = math.Pi // −Z, away from it
	if hitsTrunk(ter, g) {
		t.Error("skiing away from a trunk counted as a hit")
	}
	g.Heading, g.Speed = 0, 1
	if hitsTrunk(ter, g) {
		t.Error("a slow approach counted as a hit")
	}
	g.Speed, g.Pos[2] = 8, 18
	if hitsTrunk(ter, g) {
		t.Error("a trunk 2.4 m ahead counted as a hit")
	}
}

// Steering sees a lone trunk the cell cover alone would mostly miss.
func TestHazardSeesSingleTrunk(t *testing.T) {
	ter := world.NewTerrain(10, 10)
	ter.AddTree(world.Tree{X: 24.9, Z: 22})
	beside := hazardDensityAt(ter, nil, nil, 0, 25.4, 22, 1) // next cell over, 0.5 m away
	if beside < 0.9 {
		t.Errorf("hazard 0.5 m from a trunk = %v, want close to 1", beside)
	}
	if far := hazardDensityAt(ter, nil, nil, 0, 32, 22, 1); far != 0 {
		t.Errorf("hazard 7 m from a trunk = %v, want 0", far)
	}
}
