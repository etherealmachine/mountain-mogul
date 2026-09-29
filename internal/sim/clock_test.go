package sim

import (
	"math"
	"testing"
	"time"

	"mountain-mogul/internal/world"
)

func TestHourOfDay(t *testing.T) {
	for _, tc := range []struct {
		day  int
		hour float64
	}{{0, 0}, {0, 9.5}, {3, 16}, {120, 23.75}} {
		st := SimTimeAt(tc.day, tc.hour)
		if got := HourOfDay(st); math.Abs(got-tc.hour) > 1e-9 {
			t.Errorf("HourOfDay(SimTimeAt(%d, %v)) = %v", tc.day, tc.hour, got)
		}
		if got := dayIndex(st); got != tc.day {
			t.Errorf("dayIndex(SimTimeAt(%d, %v)) = %d", tc.day, tc.hour, got)
		}
	}
}

// At 45°N, local solar time: the solstices give ~8.8 h and ~15.5 h of
// daylight, the equinox ~12.1 h (refraction adds a few minutes).
func TestSunriseSunset(t *testing.T) {
	for _, tc := range []struct {
		date     time.Time
		daylight float64
		noonElev float64 // degrees
	}{
		{time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC), 8.75, 21.6},
		{time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC), 12.13, 45},
		{time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC), 15.6, 68.4},
	} {
		rise, set := SunriseSunset(tc.date)
		if math.Abs((rise+set)/2-12) > 1e-6 {
			t.Errorf("%s: sunrise %.2f / sunset %.2f not symmetric about noon", tc.date.Format("Jan 2"), rise, set)
		}
		if d := set - rise; math.Abs(d-tc.daylight) > 0.15 {
			t.Errorf("%s: daylight %.2f h, want ~%.2f", tc.date.Format("Jan 2"), d, tc.daylight)
		}
		elev := float64(SunAt(tc.date, 12).Elevation) * 180 / math.Pi
		if math.Abs(elev-tc.noonElev) > 1 {
			t.Errorf("%s: noon elevation %.1f°, want ~%.1f°", tc.date.Format("Jan 2"), elev, tc.noonElev)
		}
		if SunAt(tc.date, 0).Elevation >= 0 {
			t.Errorf("%s: sun up at midnight", tc.date.Format("Jan 2"))
		}
	}
}

func TestTempCurve(t *testing.T) {
	date := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	prev := DayWeather{TempLow: -12, TempHigh: -2}
	today := DayWeather{TempLow: -8, TempHigh: 3}
	next := DayWeather{TempLow: -5, TempHigh: 1}
	after := DayWeather{TempLow: -9, TempHigh: 0}
	rise, _ := SunriseSunset(date)

	if got := tempCurve(date, rise, prev, today, next); got != today.TempLow {
		t.Errorf("temp at sunrise = %v, want low %v", got, today.TempLow)
	}
	if got := tempCurve(date, peakTempHour, prev, today, next); got != today.TempHigh {
		t.Errorf("temp at %v h = %v, want high %v", peakTempHour, got, today.TempHigh)
	}
	for h := 0.0; h < 24; h += 0.25 {
		v := tempCurve(date, h, prev, today, next)
		if v < min(today.TempLow, next.TempLow) || v > max(prev.TempHigh, today.TempHigh) {
			t.Fatalf("temp at %v h = %v, outside the day's range", h, v)
		}
	}

	// Midnight hands over to the next day's curve without a jump.
	late := tempCurve(date, 24-1e-9, prev, today, next)
	early := tempCurve(date.AddDate(0, 0, 1), 0, today, next, after)
	if math.Abs(float64(late-early)) > 1e-3 {
		t.Errorf("midnight jump: %v -> %v", late, early)
	}
}

func TestArrivalShareSumsToOne(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(8, 8))
	for _, hours := range [][2]float32{{9, 16}, {8.5, 20}, {10, 12}} {
		w.OpenHour, w.CloseHour = hours[0], hours[1]
		sum := 0.0
		for h := 0.0; h < 24; h += 0.1 {
			share := arrivalShare(w, h, h+0.1)
			if share < 0 {
				t.Fatalf("hours %v: negative share at %v", hours, h)
			}
			sum += share
		}
		if math.Abs(sum-1) > 1e-9 {
			t.Errorf("hours %v: shares sum to %v", hours, sum)
		}
		if s := arrivalShare(w, float64(hours[1])-0.5, 24); s != 0 {
			t.Errorf("hours %v: %v of arrivals in the last half hour", hours, s)
		}
	}
}

func TestOperatingHours(t *testing.T) {
	w := world.NewWorld(world.NewTerrain(8, 8))
	w.ResortOpen = true
	s := NewSimulationWithSeed(w, 1)
	for _, tc := range []struct {
		hour           float64
		running, close bool
	}{
		{3, false, true},
		{8.4, false, true},
		{8.6, false, false}, // pre-open: guests queue
		{9, true, false},
		{15.9, true, false},
		{16, false, true},
		{22, false, true},
	} {
		s.SimTime = SimTimeAt(5, tc.hour)
		if got := s.LiftsRunning(); got != tc.running {
			t.Errorf("%v h: LiftsRunning = %v, want %v", tc.hour, got, tc.running)
		}
		if got := s.ClosedForDay(); got != tc.close {
			t.Errorf("%v h: ClosedForDay = %v, want %v", tc.hour, got, tc.close)
		}
	}
	w.ResortOpen = false
	s.SimTime = SimTimeAt(5, 11)
	if s.LiftsRunning() || !s.ClosedForDay() {
		t.Errorf("closed resort: lifts running or open for the day")
	}
}

// At closing time the lift line is sent home, and nobody joins it again.
func TestClosingSendsQueueHome(t *testing.T) {
	s, _, _ := dayTicketWorld()
	w := s.World
	w.OpenHour, w.CloseHour = 9, 16
	lift := w.Lifts[0]
	g := dayTicketGuest(w, 100)
	g.State = world.OnMountain
	w.OnMountain = append(w.OnMountain, g)
	lift.Queue = append(lift.Queue, g)

	s.SimTime = SimTimeAt(1, 15.99)
	s.tickResortClosed()
	if len(lift.Queue) != 1 {
		t.Fatalf("queue emptied before closing")
	}
	s.SimTime = SimTimeAt(1, 16.01)
	s.tickResortClosed()
	if len(lift.Queue) != 0 {
		t.Fatalf("queue = %d after closing, want 0", len(lift.Queue))
	}
	if g.Patience != 0 {
		t.Errorf("patience = %v after closing, want 0 (go home)", g.Patience)
	}
}
