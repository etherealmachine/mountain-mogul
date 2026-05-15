package sim

import "time"

// Calendar drives the date HUD. The simulation itself doesn't read the
// date yet — it's decorative — but season/weather systems will plug in
// here when they grow up.

// secondsPerSimDay sets how fast in-game days tick relative to sim seconds.
// 77 sim seconds per day at 4× TimeScale ≈ 19 real seconds per day, so a
// ~186-day ski season (Nov 25 → Memorial Day) takes ~1 real hour. Pure
// tuning knob — adjust freely.
const secondsPerSimDay = 77.0

// Ski-season window. Opens Nov 25 (post-Thanksgiving, traditional US
// resort opening); closes Memorial Day (last Monday of May). Off-season
// days are skipped — SimTime advances continuously, but the calendar
// jumps from Memorial Day directly to the next Nov 25. Held as constants
// for now; a future scenario format can override these per-resort.
const (
	seasonOpenMonth  = time.November
	seasonOpenDay    = 25
	seasonCloseMonth = time.May // last Monday of this month
)

// seasonEpochYear is the calendar year of the first season opening — i.e.
// SimTime 0 maps to Nov 25 of this year. The "2026-27 season" opens here.
const seasonEpochYear = 2026

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

// CalendarAt returns the in-game date for the given SimTime, walking
// season-by-season and skipping the off-season gap each year. The loop
// runs once per full season elapsed (cheap for any realistic session).
func CalendarAt(simTime float64) Date {
	totalDays := int(simTime / secondsPerSimDay)
	year := seasonEpochYear
	cur := SeasonOpenDate(year)
	for {
		end := SeasonCloseDate(year + 1)
		lenDays := int(end.Sub(cur).Hours()/24) + 1
		if totalDays < lenDays {
			d := cur.AddDate(0, 0, totalDays)
			return Date{
				Day:   d.Day(),
				Month: d.Month().String()[:3],
				Year:  d.Year(),
			}
		}
		totalDays -= lenDays
		year++
		cur = SeasonOpenDate(year)
	}
}

// WeatherKind enumerates the visual weather states the HUD can show. Today
// the rotation is purely time-based; a later world model will drive these
// from terrain + season + RNG.
type WeatherKind int

const (
	WeatherSunny WeatherKind = iota
	WeatherCloudy
	WeatherSnowing
	WeatherStormy
)

// String returns a short label suitable for the HUD.
func (w WeatherKind) String() string {
	switch w {
	case WeatherSunny:
		return "Sunny"
	case WeatherCloudy:
		return "Cloudy"
	case WeatherSnowing:
		return "Snow"
	case WeatherStormy:
		return "Storm"
	}
	return "?"
}

// WeatherState is a snapshot of the current and upcoming weather plus the
// current temperature. Decorative stub — see file header.
type WeatherState struct {
	Now   WeatherKind
	Next  WeatherKind
	TempF int
}

// WeatherAt returns a deterministic weather snapshot for the given SimTime.
// Cycles through the four kinds on a slow loop so the HUD has motion.
func WeatherAt(simTime float64) WeatherState {
	const cycleSeconds = 120.0 // sim seconds per weather slot
	slot := int(simTime / cycleSeconds)
	now := WeatherKind(slot % 4)
	next := WeatherKind((slot + 1) % 4)
	// Temperature wanders 10..30°F sinusoidally with the slot index.
	temp := 20 + ((slot*7)%11 - 5)
	return WeatherState{Now: now, Next: next, TempF: temp}
}
