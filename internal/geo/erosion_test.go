package geo

import (
	"math"
	"testing"
)

// testHills is a bumpy slope falling to the south.
func testHills(w, h int) []float32 {
	v := make([]float32, w*h)
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			x, y := float64(i), float64(j)
			v[j*w+i] = float32(-0.5*y + 4*math.Sin(x/9)*math.Cos(y/13))
		}
	}
	return v
}

func TestErodeDeterministicAndCuts(t *testing.T) {
	const w, h = 300, 300
	a, b := testHills(w, h), testHills(w, h)
	Erode(a, w, h, 1.25, 1, 1, 7)
	Erode(b, w, h, 1.25, 1, 1, 7)
	orig := testHills(w, h)
	var moved, net float64
	for k := range a {
		if a[k] != b[k] {
			t.Fatalf("sample %d differs between runs: %v vs %v", k, a[k], b[k])
		}
		d := float64(a[k] - orig[k])
		moved += math.Abs(d)
		net += d
	}
	if moved == 0 {
		t.Fatal("erosion changed nothing")
	}
	// Drops leaving the map carry sediment away, so a little net loss is
	// expected, but nothing should be created.
	if net > 1e-3*moved {
		t.Errorf("erosion created ground: net %+.2f of %.2f moved", net, moved)
	}
}

func TestSmoothGroundSparesRock(t *testing.T) {
	const w, h = 80, 80
	v := make([]float32, w*h)
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			switch {
			case i >= 40:
				v[j*w+i] = 30 // a cliff along i = 40
			case (i/2+j/2)%2 == 0 && i < 30:
				v[j*w+i] = 0.5 // small bumps on the flat
			}
		}
	}
	SmoothGround(v, w, h, 1.25, 1, 1)
	bump := 0.0
	for j := 10; j < 70; j++ {
		for i := 10; i < 25; i++ {
			bump = math.Max(bump, math.Abs(float64(v[j*w+i])-0.25))
		}
	}
	if bump > 0.1 {
		t.Errorf("bumps survived: %.2f m from mean", bump)
	}
	if top, foot := v[40*w+41], v[40*w+38]; top < 29 || foot > 1 {
		t.Errorf("cliff softened: foot %.2f, top %.2f", foot, top)
	}
}
