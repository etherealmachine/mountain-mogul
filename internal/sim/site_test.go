package sim

import (
	"math"
	"testing"
	"time"

	"mountain-mogul/internal/world"
)

// Kirkwood on Pacific time: published sunrise/sunset within a few minutes,
// with daylight saving moving the March clock an hour.
func TestSiteClockIsLocalTime(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(8, 8))
	w.Geo = &world.GeoBounds{MinLat: 38.66, MaxLat: 38.69, MinLon: -120.09, MaxLon: -120.05}
	w.TimeZone = "America/Los_Angeles"
	site := SiteOf(w)
	if site.Zone == nil {
		t.Fatal("time zone didn't load")
	}
	for _, tc := range []struct {
		date      time.Time
		rise, set float64
	}{
		{time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC), 7.2, 16.77},
		{time.Date(2027, 3, 20, 0, 0, 0, 0, time.UTC), 7.08, 19.22},
	} {
		rise, set := site.SunriseSunset(tc.date)
		if math.Abs(rise-tc.rise) > 0.1 || math.Abs(set-tc.set) > 0.1 {
			t.Errorf("%s: sunrise %.2f sunset %.2f, want ~%.2f / ~%.2f", tc.date.Format("Jan 2"), rise, set, tc.rise, tc.set)
		}
		noon := (rise + set) / 2
		if e := site.SunAt(tc.date, noon).Dir; math.Abs(float64(e[0])) > 0.01 {
			t.Errorf("%s: sun not due south at solar noon (east %.3f)", tc.date.Format("Jan 2"), e[0])
		}
	}
	// Lower latitude, higher sun: Kirkwood's December noon beats 45°N's.
	dec := time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC)
	r, s := site.SunriseSunset(dec)
	if site.SunAt(dec, (r+s)/2).Elevation <= DefaultSite.SunAt(dec, 12).Elevation {
		t.Error("38.7°N noon sun no higher than 45°N")
	}
	if SiteOf(world.NewWorld(world.NewTerrain(8, 8))) != DefaultSite {
		t.Error("a drawn map isn't on the default site")
	}
}

// The chain's long-run weather follows the climate it was built from.
func TestChainFollowsClimate(t *testing.T) {
	c := &world.Climate{RefAltitude: 2500}
	for m := range c.Months {
		c.Months[m] = world.ClimateMonth{TempMean: -6, TempRange: 10, WetDays: 0.4, WetMM: 15, Cloud: 0.6}
	}
	ch := NewChainFor(c, 2000) // 500 m lower: 3.25 °C warmer
	d := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	var temp, mm float64
	wet, n := 0, 365*30
	for i := 0; i < n; i++ {
		dw := ch.Advance(d)
		temp += float64(dw.TempC)
		mm += float64(dw.AccumSWE*1000 + dw.RainMM)
		if dw.IsSnowing() || dw.IsRaining() {
			wet++
		}
		d = d.AddDate(0, 0, 1)
	}
	if got := temp / float64(n); math.Abs(got-(-2.75)) > 0.6 {
		t.Errorf("mean temperature %.2f, want ~-2.75", got)
	}
	if got := float64(wet) / float64(n); math.Abs(got-0.4) > 0.05 {
		t.Errorf("wet-day share %.2f, want ~0.4", got)
	}
	if got := mm / float64(n); math.Abs(got-6) > 1 {
		t.Errorf("precipitation %.1f mm/day, want ~6", got)
	}
}
