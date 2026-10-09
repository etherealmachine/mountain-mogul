package sim

import (
	"math"
	"time"
)

// StormStopLead is how long before a storm's first snowfall the
// fast-forward stop lands: one clock hour (23:00), three real minutes of
// watching at 1× before the snow comes down at midnight.
const StormStopLead = simSecondsPerHour

// NextStormStop finds the next storm onset within maxDays game days — the
// first forecast day of heavy snow that follows a day without it — and returns
// the sim time StormStopLead before it begins, plus that day's date.
// Onsets whose stop time has already passed are skipped, so asking on the
// evening before a storm finds the one after. The forecast is the same
// deterministic chain the day rollover advances, so the storm is certain
// to arrive.
func (s *Simulation) NextStormStop(maxDays int) (stopAt float64, day time.Time, ok bool) {
	today := int(math.Floor(s.SimTime / secondsPerSimDay))
	prev := s.Weather.Today().State
	for i, d := range s.GameForecast(maxDays) {
		onset := d.State == WeatherHeavySnow && prev != WeatherHeavySnow
		prev = d.State
		if !onset {
			continue
		}
		dayIdx := today + 1 + i
		stopAt = float64(dayIdx)*secondsPerSimDay - StormStopLead
		if stopAt > s.SimTime {
			return stopAt, s.DateAt(float64(dayIdx) * secondsPerSimDay), true
		}
	}
	return 0, time.Time{}, false
}
