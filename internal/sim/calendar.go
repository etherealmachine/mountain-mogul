package sim

import (
	"math"
	"time"

	"mountain-mogul/internal/world"
)

// Calendar maps SimTime to a date. Weather samples its month profile,
// demand and costs follow World.ResortOpen, and credit bills at month ends.

// secondsPerSimDay is one calendar day of sim seconds (world.SecondsPerSimDay):
// 4320 s, or 18 real minutes at 4× TimeScale.
const (
	secondsPerSimDay  = world.SecondsPerSimDay
	simSecondsPerHour = world.SimSecondsPerHour
)

// Memorial Day (last Monday of May) marks the end of a season for the
// demand system's season rollover. Whether the resort is open on any
// given day is the player's call (World.ResortOpen), not the calendar's.
const seasonCloseMonth = time.May // last Monday of this month

// SeasonCloseYearFor returns the calendar year in which the season that
// contains t closes (i.e. the Memorial Day year). Seasons run Nov→May, so
// dates from November onward belong to the season that closes next year.
func SeasonCloseYearFor(t time.Time) int {
	if t.Month() >= time.November {
		return t.Year() + 1
	}
	return t.Year()
}

// SeasonCloseDate returns Memorial Day (last Monday of May) for the given
// calendar year — the final day of the season that opened the previous Nov.
func SeasonCloseDate(year int) time.Time {
	d := time.Date(year, seasonCloseMonth, 31, 0, 0, 0, 0, time.UTC)
	back := (int(d.Weekday()) - int(time.Monday) + 7) % 7
	return d.AddDate(0, 0, -back)
}

// Date is a calendar position derived from SimTime.
type Date struct {
	Day   int    // 1..31
	Month string // "Nov", "Dec", "Jan", ...
	Year  int    // calendar year, e.g. 2026
}

// CalendarAt returns the in-game date for the given SimTime in a world
// whose calendar starts on start (World.StartDate).
func CalendarAt(start time.Time, simTime float64) Date {
	t := DateAt(start, simTime)
	return Date{
		Day:   t.Day(),
		Month: t.Month().String()[:3],
		Year:  t.Year(),
	}
}

// DateAt returns the calendar date of the day containing simTime: start
// (World.StartDate, the date SimTime 0 maps to) plus one day per
// secondsPerSimDay. The calendar runs continuously from start through the
// off-season as well as the season. Negative simTime (events from before a
// starter scenario was rebased) counts back from start.
func DateAt(start time.Time, simTime float64) time.Time {
	return start.AddDate(0, 0, int(math.Floor(simTime/secondsPerSimDay)))
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
