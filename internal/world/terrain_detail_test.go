package world

import (
	"math"
	"testing"
)

func TestTerrainDetailBytesRoundTrip(t *testing.T) {
	tr := NewTerrain(12, 9)
	tr.FillDetailTest()
	tr.Detail.Off[5] = 400
	tr.Detail.Off[6] = -400
	got := LoadTerrainDetail(12, 9, tr.Detail.Bytes())
	if got == nil {
		t.Fatal("LoadTerrainDetail rejected its own bytes")
	}
	for k, want := range tr.Detail.Off {
		want = clampf(want, -327.67, 327.67)
		if d := math.Abs(float64(got.Off[k] - want)); d > 0.0051 {
			t.Fatalf("offset %d: got %v, want %v", k, got.Off[k], want)
		}
	}
	if LoadTerrainDetail(13, 9, tr.Detail.Bytes()) != nil {
		t.Fatal("accepted detail for the wrong terrain size")
	}
}
