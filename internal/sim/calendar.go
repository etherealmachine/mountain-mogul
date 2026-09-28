package sim

import (
	"math"
	"time"
)

// Calendar maps SimTime to a date. Weather samples its month profile,
// demand and costs follow World.ResortOpen, and credit bills at month ends.

// secondsPerSimDay sets how fast in-game days tick relative to sim seconds.
// 240 sim seconds per day at 4× TimeScale = 60 real seconds per day (1 min),
// so a ~186-day ski season ≈ 3 real hours and a full year ≈ 6 real hours.
// Pure tuning knob — adjust freely.
const secondsPerSimDay = 240.0

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
