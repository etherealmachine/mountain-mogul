package sim

import (
	"testing"
	"time"

	"mountain-mogul/internal/world"
)

// testClimate is a snowy mountain: below freezing from December to
// March at 2000 m, wet in winter and dry in summer.
func testClimate() *world.Climate {
	c := &world.Climate{RefAltitude: 2000}
	temps := [12]float32{-5, -4, -2, 2, 6, 11, 15, 14, 10, 5, 0, -4}
	wet := [12]float32{0.4, 0.4, 0.4, 0.3, 0.2, 0.1, 0.05, 0.05, 0.1, 0.2, 0.3, 0.4}
	for m := range c.Months {
		c.Months[m] = world.ClimateMonth{TempMean: temps[m], TempRange: 10, WetDays: wet[m], WetMM: 20, Cloud: 0.5}
	}
	return c
}

func TestSeasonSnowpack(t *testing.T) {
	p := NewSeasonSnowpack(testClimate(), 40, 2026, 1500, 1500, 3000)
	at := func(date string, alt, gz float32) float32 {
		d, _ := time.Parse("2006-01-02", date)
		swe, recent := p.At(p.Day(d), alt, 0, gz, 1)
		if recent < 0 || recent > swe {
			t.Fatalf("%s at %.0f m: recent %.3f outside [0, %.3f]", date, alt, recent, swe)
		}
		return swe
	}
	if v := at("2026-10-01", 2000, 0); v != 0 {
		t.Errorf("snow on 1 October at 2000 m: %.3f", v)
	}
	if lo, hi := at("2027-02-15", 1800, 0), at("2027-02-15", 2800, 0); !(hi > lo && lo > 0.05) {
		t.Errorf("mid-February: %.3f m SWE at 1800 m, %.3f at 2800 m; want some low and more high", lo, hi)
	}
	// +gz rises to the south, so the slope faces north.
	if n, s := at("2027-04-15", 2200, 0.6), at("2027-04-15", 2200, -0.6); !(n > s) {
		t.Errorf("mid-April at 2200 m: north face %.3f, south face %.3f; want more on the north", n, s)
	}
	if v := at("2027-06-25", 2000, 0); v != 0 {
		t.Errorf("snow in late June at 2000 m: %.3f", v)
	}
	open, ok := p.OpeningDay(2200, 0.15)
	if !ok || open.Month() < time.November || open.Month() > time.January && open.Month() < time.September {
		t.Errorf("opening day %v, %v; want November to January", open, ok)
	}
}
