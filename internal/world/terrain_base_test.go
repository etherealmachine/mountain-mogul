package world

import (
	"math"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

func TestTerrainBaseHeightsRoundTrip(t *testing.T) {
	b := &TerrainBase{W: 37, H: 23}
	for j := 0; j < b.H; j++ {
		for i := 0; i < b.W; i++ {
			b.Heights = append(b.Heights, float32(900*math.Sin(float64(i)/7)+3*float64(j)-450.123))
		}
	}
	got := &TerrainBase{W: b.W, H: b.H}
	if err := got.SetHeightsBytes(b.HeightsBytes()); err != nil {
		t.Fatal(err)
	}
	for k, v := range b.Heights {
		if d := math.Abs(float64(got.Heights[k] - v)); d > 0.006 {
			t.Fatalf("sample %d: %v became %v", k, v, got.Heights[k])
		}
	}
	if err := got.SetHeightsBytes(b.HeightsBytes()[:10]); err == nil {
		t.Error("truncated heights loaded without an error")
	}
}

func TestClearBuiltKeepsTreesAndFreesCells(t *testing.T) {
	w := NewWorld(NewTerrain(40, 40))
	w.Cash = 1 << 30
	if w.PlaceBuilding(50, 50) == nil || w.PlaceLift(LiftDouble, 60, 150, 60, 60) == nil {
		t.Fatal("couldn't place test structures")
	}
	w.AddRoadNode(mgl32.Vec2{10, 10}, 0)
	w.Terrain.AddTree(Tree{X: 120, Z: 120})
	w.ClearBuilt()
	if len(w.Buildings)+len(w.Lifts)+len(w.RoadNodes) != 0 {
		t.Fatalf("left %d buildings, %d lifts, %d road nodes", len(w.Buildings), len(w.Lifts), len(w.RoadNodes))
	}
	for x := range w.Terrain.Cells {
		for z := range w.Terrain.Cells[x] {
			if !w.Terrain.Cells[x][z].Passable {
				t.Fatalf("cell (%d, %d) still blocked", x, z)
			}
		}
	}
	if w.Terrain.TotalTrees() != 1 {
		t.Errorf("trees = %d, want 1", w.Terrain.TotalTrees())
	}
}

func TestTerrainBaseLayerSwitches(t *testing.T) {
	var b TerrainBase
	if !b.LayerOn("erode") {
		t.Fatal("layers should start on")
	}
	b.SetLayer("erode", false)
	b.SetLayer("erode", false)
	if b.LayerOn("erode") || len(b.LayersOff) != 1 {
		t.Fatalf("off twice: %v", b.LayersOff)
	}
	b.SetLayer("erode", true)
	if !b.LayerOn("erode") || len(b.LayersOff) != 0 {
		t.Fatalf("back on: %v", b.LayersOff)
	}
}
