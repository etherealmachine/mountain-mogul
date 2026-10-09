package sim

import (
	"math"
	"time"

	"mountain-mogul/internal/world"
)

// Calendar maps SimTime to a date. Weather samples its month profile,
// demand and costs follow World.ResortOpen, and credit bills at month ends.

// secondsPerSimDay is one game day of sim seconds (world.SecondsPerSimDay).
const (
	secondsPerSimDay  = world.SecondsPerSimDay
	simSecondsPerHour = world.SimSecondsPerHour
)

// A season closes at the end of April, the last of the open months
// (world.SeasonMonths, December to April), for the demand system's season
// rollover and goal deadlines. Whether the resort is open on any given
// day is the player's call (World.ResortOpen), not the calendar's.
const seasonCloseMonth = time.April

// SeasonCloseYearFor returns the calendar year in which the season that
// contains t closes. Seasons run from November to April, so dates from
// November onward belong to the season that closes next year.
func SeasonCloseYearFor(t time.Time) int {
	if t.Month() >= time.November {
		return t.Year() + 1
	}
	return t.Year()
}

// SeasonCloseDate returns the last game day of April in the given year,
// the final day of the season that opened the previous winter.
func SeasonCloseDate(year int) time.Time {
	return world.GameDayStart(year, seasonCloseMonth, world.DaysPerMonth)
}

// Date is a calendar position derived from SimTime.
type Date struct {
	Day   int    // game day of the month, 1..world.DaysPerMonth
	Month string // "Nov", "Dec", "Jan", ...
	Year  int    // calendar year, e.g. 2026
}

// CalendarAt returns the in-game date for the given SimTime in a world
// whose calendar starts on start (World.StartDate).
func CalendarAt(start time.Time, simTime float64) Date {
	t := DateAt(start, simTime)
	return Date{
		Day:   world.GameDay(t),
		Month: t.Month().String()[:3],
		Year:  t.Year(),
	}
}

// DateAt returns the real date of the game day containing simTime:
// start (World.StartDate, the date SimTime 0 maps to) moved on one game
// day per secondsPerSimDay, each the first real day of its stretch
// (world.DaysPerMonth). The calendar runs continuously from start through
// the off-season as well as the season. Negative simTime (events from
// before a starter scenario was rebased) counts back from start.
func DateAt(start time.Time, simTime float64) time.Time {
	return world.GameDayAt(world.GameDayIndex(start) + int(math.Floor(simTime/secondsPerSimDay)))
}

// DateAt is DateAt for this simulation's world.
func (s *Simulation) DateAt(simTime float64) time.Time {
	return DateAt(s.World.StartDate, simTime)
}

// HourOfDay returns the clock time of simTime in hours since midnight,
// in [0, 24). Clock time is local solar time: the sun peaks at 12:00.
func HourOfDay(simTime float64) float64 {
	return (simTime - math.Floor(simTime/secondsPerSimDay)*secondsPerSimDay) / simSecondsPerHour
}

// TimeAt returns the calendar date and clock time of simTime as one
// time.Time (UTC stands in for local solar time).
func TimeAt(start time.Time, simTime float64) time.Time {
	return DateAt(start, simTime).Add(time.Duration(HourOfDay(simTime) * float64(time.Hour)))
}

// SimTimeAt returns the SimTime of clock hour h on day index day.
func SimTimeAt(day int, h float64) float64 {
	return float64(day)*secondsPerSimDay + h*simSecondsPerHour
}

// dayIndex is the whole-day count of simTime since StartDate.
func dayIndex(simTime float64) int {
	return int(math.Floor(simTime / secondsPerSimDay))
}

// SkipClockEffects marks the hours and polls up to SimTime as already
// run, after the clock is set directly (a jump within the day), so the
// sim doesn't replay them.
func (s *Simulation) SkipClockEffects() {
	s.lastHour = int(s.SimTime / simSecondsPerHour)
	s.Demand.LastPoll = s.SimTime
	s.closedForDay = s.ClosedForDay()
	s.World.ClosedForDay = s.closedForDay
}

// LiftsRunning reports whether lifts are loading right now: the resort is
// open for the season and the clock is inside the operating hours.
func (s *Simulation) LiftsRunning() bool {
	w := s.World
	if !w.ResortOpen {
		return false
	}
	h := float32(HourOfDay(s.SimTime))
	return h >= w.OpenHour && h < w.CloseHour
}

// GameForecast returns the weather of the next n game days, each its
// first real day's (the one shown and played live), from the same
// deterministic chain the day rollover advances.
func (s *Simulation) GameForecast(n int) []DayWeather {
	return gameForecast(s.Weather, s.DateAt(s.SimTime), n)
}

func gameForecast(c *Chain, from time.Time, n int) []DayWeather {
	idx := world.GameDayIndex(from)
	realDays := func(d time.Time) int { return int(d.Sub(from).Hours()/24 + 0.5) }
	real := c.Forecast(from, realDays(world.GameDayAt(idx+n)))
	out := make([]DayWeather, n)
	for i := range out {
		out[i] = real[realDays(world.GameDayAt(idx+1+i))-1]
	}
	return out
}

// playOffscreenDays runs the weather of the real days between the game
// day from and the next one, all but from itself, which played live:
// each day's snowfall, snow changes, and a whole day of melt, so a month
// of ten game days gets a month of weather.
func (s *Simulation) playOffscreenDays(from, next time.Time) {
	for d := from.AddDate(0, 0, 1); d.Before(next); d = d.AddDate(0, 0, 1) {
		dw := s.Weather.Advance(d)
		s.applyDailyWeather(dw)
		if !dw.IsSnowing() {
			s.applyDayMelt(dw, d)
		}
	}
}
