package world

import (
	"math"
	"testing"
)

// A 50 m wall 10 cells (50 m) east of a flat-ground cell puts the
// eastern horizon at ~45°; the open west is at the horizon.
func TestHorizonWall(t *testing.T) {
	tr := NewTerrain(40, 20)
	for x := 20; x < 23; x++ {
		for z := 0; z < 20; z++ {
			tr.Cells[x][z].GroundElevation = 50
		}
	}
	tr.RecomputeSlopes()
	hm := tr.Horizon()
	deg := func(r float32) float64 { return float64(r) * 180 / math.Pi }

	east := deg(hm.Angle(10, 10, 0))
	if want := math.Atan((50-float64(horizonBias))/50) * 180 / math.Pi; math.Abs(east-want) > 0.5 {
		t.Errorf("east horizon %.1f°, want %.1f°", east, want)
	}
	if west := deg(hm.Angle(10, 10, math.Pi)); west > 0 || west < -2 {
		t.Errorf("west horizon %.1f°, want just below 0°", west)
	}

	sunAt := func(elevDeg, azRad float64) [3]float32 {
		e := elevDeg * math.Pi / 180
		return [3]float32{float32(math.Cos(e) * math.Cos(azRad)), float32(math.Sin(e)), float32(math.Cos(e) * math.Sin(azRad))}
	}
	if v := hm.SunVisibility(10, 10, sunAt(30, 0)); v != 0 {
		t.Errorf("morning sun behind the wall: visibility %v, want 0", v)
	}
	if v := hm.SunVisibility(10, 10, sunAt(60, 0)); v != 1 {
		t.Errorf("sun above the wall: visibility %v, want 1", v)
	}
	if v := hm.SunVisibility(10, 10, sunAt(10, math.Pi)); v != 1 {
		t.Errorf("low western sun over open ground: visibility %v, want 1", v)
	}
	// Cells on the far (west) side of the ridge, looking east past the
	// wall top, are open once the wall is behind them.
	if v := hm.SunVisibility(30, 10, sunAt(10, 0)); v != 1 {
		t.Errorf("east of the wall, low eastern sun: visibility %v, want 1", v)
	}

	// Raising the terrain invalidates the map.
	for x := 20; x < 23; x++ {
		for z := 0; z < 20; z++ {
			tr.Cells[x][z].GroundElevation = 0
		}
	}
	tr.RecomputeSlopes()
	if a := deg(tr.Horizon().Angle(10, 10, 0)); a > 0 {
		t.Errorf("after flattening, east horizon %.1f°, want ≤ 0", a)
	}
}
