package sim

import (
	"math"
	"testing"
	"time"
)

func TestPositiveDegreeDays(t *testing.T) {
	cases := []struct{ mean, swing, want float32 }{
		{5, 0, 5},           // no swing: just the mean
		{-3, 0, 0},          // no swing, below freezing
		{-6, 5, 0},          // never reaches 0
		{8, 5, 8},           // never drops below 0
		{0, 5, 5 / math.Pi}, // half the cycle above 0: a/π
	}
	for _, c := range cases {
		if got := positiveDegreeDays(c.mean, c.swing); math.Abs(float64(got-c.want)) > 1e-4 {
			t.Errorf("pdd(%v, %v) = %v, want %v", c.mean, c.swing, got, c.want)
		}
	}
	// A cold clear day that peaks just above freezing melts a little.
	if got := positiveDegreeDays(-3, 5); got <= 0.3 || got >= 0.5 {
		t.Errorf("pdd(-3, 5) = %v, want ≈0.39", got)
	}
}

func TestSunExposureByAspect(t *testing.T) {
	slope := float32(math.Tan(30 * math.Pi / 180))
	// World +Z is south; a south-facing slope drops toward +Z (gz < 0).
	south, north, east := [2]float32{0, -slope}, [2]float32{0, slope}, [2]float32{-slope, 0}
	at := func(month time.Month, g [2]float32) float32 {
		return newDaySun(time.Date(2026, month, 21, 0, 0, 0, 0, time.UTC), 0).exposure(g[0], g[1])
	}

	if flat := at(time.March, [2]float32{}); math.Abs(float64(flat-1)) > 0.05 {
		t.Errorf("flat equinox exposure = %v, want ≈1", flat)
	}
	decFlat, decS, decN, decE := at(time.December, [2]float32{}), at(time.December, south), at(time.December, north), at(time.December, east)
	if !(decS > 2*decFlat && decFlat > decE && decE > decN) {
		t.Errorf("December exposure S=%v flat=%v E=%v N=%v, want S ≫ flat > E > N", decS, decFlat, decE, decN)
	}
	if decN > 0.02 {
		t.Errorf("December 30° north face exposure = %v, want ≈0 (sun below the slope)", decN)
	}
	if aprFlat := at(time.April, [2]float32{}); aprFlat < 2*decFlat {
		t.Errorf("April flat %v should be well over twice December flat %v", aprFlat, decFlat)
	}
	cloudy := newDaySun(time.Date(2026, time.March, 21, 0, 0, 0, 0, time.UTC), 1).exposure(0, 0)
	if cloudy > 0.2 {
		t.Errorf("overcast exposure = %v, want ≤0.2", cloudy)
	}
}
