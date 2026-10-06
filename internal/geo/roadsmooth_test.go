package geo

import (
	"math"
	"testing"
)

// A road cut 8 m into a tilted plane comes back as the plane: the fill
// spans from the ground either side, and a plane has no bumps to add.
func TestSmoothRoadsFillsCut(t *testing.T) {
	const w, ht, spacing = 201, 201, 1.25
	b := Bounds{MinLat: 0, MaxLat: 1, MinLon: 0, MaxLon: 1}
	plane := func(i, j int) float32 { return float32(0.3*float64(i)*spacing + 0.1*float64(j)*spacing) }
	h := make([]float32, w*ht)
	for j := 0; j < ht; j++ {
		for i := 0; i < w; i++ {
			h[j*w+i] = plane(i, j)
			if d := math.Abs(float64(j - 100)); d*spacing < 15 {
				h[j*w+i] -= float32(8 * math.Min(1, (15-d*spacing)/5))
			}
		}
	}
	road := Road{Kind: "primary", Width: 10, Path: []LatLon{{Lat: 0.5, Lon: -0.1}, {Lat: 0.5, Lon: 1.1}}}
	if n := SmoothRoads(h, w, ht, b, spacing, []Road{road}, StandardRoads); n == 0 {
		t.Fatal("nothing smoothed")
	}
	var worst float64
	wi, wj := 0, 0
	for j := 0; j < ht; j++ {
		for i := 0; i < w; i++ {
			if d := math.Abs(float64(h[j*w+i] - plane(i, j))); d > worst {
				worst, wi, wj = d, i, j
			}
		}
	}
	if worst > 0.25 {
		t.Errorf("worst departure from the plane %.2f m at (%d, %d)", worst, wi, wj)
	}
}

func TestSmoothRoadsSkipsTunnels(t *testing.T) {
	h := make([]float32, 50*50)
	road := Road{Width: 10, Tunnel: true, Path: []LatLon{{Lat: 0.5, Lon: 0}, {Lat: 0.5, Lon: 1}}}
	if n := SmoothRoads(h, 50, 50, Bounds{MaxLat: 1, MaxLon: 1}, 1.25, []Road{road}, StandardRoads); n != 0 {
		t.Errorf("tunnel smoothed %d samples", n)
	}
}

func TestGroundNoiseUnitRMS(t *testing.T) {
	var sum, sq float64
	n := 0
	for y := 0.0; y < 400; y += 0.7 {
		for x := 0.0; x < 400; x += 0.7 {
			v := float64(groundNoise(x, y))
			sum += v
			sq += v * v
			n++
		}
	}
	mean, rms := sum/float64(n), math.Sqrt(sq/float64(n))
	if math.Abs(mean) > 0.2 || rms < 0.85 || rms > 1.15 {
		t.Errorf("mean %.3f rms %.3f, want 0 and 1", mean, rms)
	}
}
