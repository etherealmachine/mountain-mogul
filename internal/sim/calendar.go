package sim

import (
	"time"

	"mountain-mogul/internal/world"
)

// Calendar maps SimTime to a date. Weather samples its month profile,
// demand and costs follow ResortOpen, and credit bills at month ends.

// secondsPerSimDay sets how fast in-game days tick relative to sim seconds.
// 240 sim seconds per day at 4× TimeScale = 60 real seconds per day (1 min),
// so a ~186-day ski season ≈ 3 real hours and a full year ≈ 6 real hours.
// Pure tuning knob — adjust freely.
const secondsPerSimDay = 240.0

// calendarEpoch is the date SimTime 0 maps to: Nov 25, 2026, opening day
// of the 2026-27 season. The calendar runs continuously from here, one
// day per secondsPerSimDay, through the off-season as well as the season.
var calendarEpoch = time.Date(2026, time.November, 25, 0, 0, 0, 0, time.UTC)

// Ski-season window. Opens Nov 25 (post-Thanksgiving, traditional US
// resort opening); closes Memorial Day (last Monday of May). Until the
// player can open and close the resort, ResortOpen uses this fixed window.
const (
	seasonOpenMonth  = time.November
	seasonOpenDay    = 25
	seasonCloseMonth = time.May // last Monday of this month
)

// ResortOpen reports whether the resort is open for skiing on the day
// containing simTime: Nov 25 through Memorial Day inclusive. While closed
// the demand poll spawns nobody and the day rollover charges standby
// rather than operating costs. Placeholder for player open/close, which
// will decide from w; the fixed window ignores it.
func ResortOpen(w *world.World, simTime float64) bool {
	t := DateAt(simTime)
	return !t.Before(SeasonOpenDate(t.Year())) || !t.After(SeasonCloseDate(t.Year()))
}

// SeasonCloseYearFor returns the calendar year in which the season that
// contains t closes (i.e. the Memorial Day year). Seasons run Nov→May, so
// dates from November onward belong to the season that closes next year.
func SeasonCloseYearFor(t time.Time) int {
	if t.Month() >= time.November {
		return t.Year() + 1
	}
	return t.Year()
}

// SeasonOpenDate returns Nov 25 of the given year.
func SeasonOpenDate(year int) time.Time {
	return time.Date(year, seasonOpenMonth, seasonOpenDay, 0, 0, 0, 0, time.UTC)
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

// CalendarAt returns the in-game date for the given SimTime.
func CalendarAt(simTime float64) Date {
	t := DateAt(simTime)
	return Date{
		Day:   t.Day(),
		Month: t.Month().String()[:3],
		Year:  t.Year(),
	}
}

// DateAt returns the calendar date of the day containing simTime: the
// epoch plus one day per secondsPerSimDay.
func DateAt(simTime float64) time.Time {
	return calendarEpoch.AddDate(0, 0, int(simTime/secondsPerSimDay))
}
