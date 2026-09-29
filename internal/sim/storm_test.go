package sim

import (
	"math"
	"testing"
)

// TestFastForwardStopsBeforeStorm: running at a turbo TimeScale with
// StopAt set to NextStormStop halts exactly there — still the day before,
// no storm snow yet — and resuming carries the sim into the storm day.
func TestFastForwardStopsBeforeStorm(t *testing.T) {
	s, _, _ := dayTicketWorld()
	stopAt, day, ok := s.NextStormStop(120)
	if !ok {
		t.Fatal("no storm forecast in 120 days")
	}
	stormDay := int(math.Round((stopAt + StormStopLead) / secondsPerSimDay))
	if got := s.DateAt(float64(stormDay) * secondsPerSimDay); !got.Equal(day) {
		t.Fatalf("reported storm date %v, stop implies %v", day, got)
	}

	s.TimeScale = 500
	s.StopAt = stopAt
	for i := 0; i < 1_000_000 && s.SimTime < stopAt; i++ {
		s.Tick(0.1)
	}
	if math.Abs(s.SimTime-stopAt) > 1e-6 {
		t.Fatalf("SimTime = %v, want the stop %v", s.SimTime, stopAt)
	}
	for i := 0; i < 10; i++ {
		s.Tick(0.1) // pinned at the stop
	}
	if s.SimTime > stopAt+1e-6 {
		t.Fatalf("Tick ran past StopAt: %v > %v", s.SimTime, stopAt)
	}
	if st := s.Weather.Today().State; st == WeatherHeavySnow {
		t.Fatal("already storming at the stop; want the day before")
	}

	s.StopAt = 0
	s.TimeScale = 1
	for s.SimTime < stopAt+StormStopLead+1 {
		s.Tick(0.1)
	}
	if st := s.Weather.Today().State; st != WeatherHeavySnow {
		t.Fatalf("weather after the stop = %v, want the forecast storm", st)
	}
}

// TestNextStormStopSkipsPassedOnset: asking again from the stop itself
// finds a later storm, not the one about to start.
func TestNextStormStopSkipsPassedOnset(t *testing.T) {
	s, _, _ := dayTicketWorld()
	first, _, ok := s.NextStormStop(120)
	if !ok {
		t.Fatal("no storm forecast in 120 days")
	}
	s.TimeScale = 500
	s.StopAt = first
	for s.SimTime < first-1e-9 {
		s.Tick(0.1)
	}
	if next, _, ok := s.NextStormStop(120); ok && next <= s.SimTime {
		t.Fatalf("next stop %v is not after now %v", next, s.SimTime)
	}
}
