package world

import (
	"fmt"
	"time"
)

// The game calendar has DaysPerMonth days a month. Each game day stands
// for a stretch of real days (about three), and is shown and simulated as
// the first of them: the sun, the climate, and the weather run on real
// dates, and the day rollover plays the rest of the stretch's weather
// (sim.maybeSampleHistory). A time.Time in the sim is the real date of a
// game day's first real day; GameDay and FormatGameDate turn it into the
// game's own date ("Dec 10").
const DaysPerMonth = 10

// SeasonMonths are the months a resort is open for: December to April.
// SeasonDays is the season's length in game days.
const (
	SeasonMonths = 5
	SeasonDays   = SeasonMonths * DaysPerMonth
)

// daysIn returns the number of real days in t's month.
func daysIn(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// GameDay returns the game day of the month (1..DaysPerMonth) whose
// stretch of real days holds t.
func GameDay(t time.Time) int {
	// The last game day whose first real day (GameDayStart) is on or
	// before t.
	dim := daysIn(t)
	return (t.Day()*DaysPerMonth + dim - 1) / dim
}

// GameDayStart returns the real date of game day day (1..DaysPerMonth)
// in the given month: the first real day of its stretch.
func GameDayStart(year int, month time.Month, day int) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	off := (day - 1) * daysIn(first) / DaysPerMonth
	return first.AddDate(0, 0, off)
}

// GameDayIndex counts game days from year 0 to the game day holding t;
// the difference of two indexes is the game days between them.
func GameDayIndex(t time.Time) int {
	return (t.Year()*12+int(t.Month())-1)*DaysPerMonth + GameDay(t) - 1
}

// GameDayAt is the real date of game day index n (GameDayIndex).
func GameDayAt(n int) time.Time {
	months := floorDiv(n, DaysPerMonth)
	day := n - months*DaysPerMonth + 1
	year := floorDiv(months, 12)
	return GameDayStart(year, time.Month(months-year*12+1), day)
}

// RealDaysIn returns how many real days the game day holding t stands for.
func RealDaysIn(t time.Time) int {
	next := GameDayAt(GameDayIndex(t) + 1)
	start := GameDayAt(GameDayIndex(t))
	return int(next.Sub(start).Hours()/24 + 0.5)
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// FormatGameDate writes the game date of t: "Dec 10", or "Dec 10, 2026"
// with the year.
func FormatGameDate(t time.Time, withYear bool) string {
	s := fmt.Sprintf("%s %d", t.Month().String()[:3], GameDay(t))
	if withYear {
		s += fmt.Sprintf(", %d", t.Year())
	}
	return s
}

// Holidays fall on the 5th and 10th of every month: the busy days, each
// a one-day rush.
var holidayDays = [...]int{5, 10}

// DayType is how busy a game day is.
type DayType uint8

const (
	OrdinaryDay  DayType = iota
	Holiday              // the 5th or 10th of a month
	NamedHoliday         // Christmas, MLK Day, Presidents' Day, Easter
)

// HolidayAt returns the kind of the game day holding t and, for a named
// holiday, its name.
func HolidayAt(t time.Time) (DayType, string) {
	day := GameDay(t)
	holiday := false
	for _, h := range holidayDays {
		holiday = holiday || day == h
	}
	if !holiday {
		return OrdinaryDay, ""
	}
	switch {
	case t.Month() == time.December && day == 10:
		return NamedHoliday, "Christmas"
	case t.Month() == time.January && day == 5:
		return NamedHoliday, "MLK Day"
	case t.Month() == time.February && day == 5:
		return NamedHoliday, "Presidents' Day"
	case GameDayIndex(t) == easterIndex(t.Year()):
		return NamedHoliday, "Easter"
	}
	return Holiday, ""
}

// easterIndex is the GameDayIndex of the holiday Easter Sunday falls on
// in year: March 10th when Easter is in March, April 5th when it's on or
// before April 20th, and April 10th for the latest.
func easterIndex(year int) int {
	e := easterSunday(year)
	day := 10
	if e.Month() == time.April && e.Day() <= 20 {
		day = 5
	}
	return GameDayIndex(GameDayStart(year, e.Month(), day))
}

// easterSunday returns the date of Easter Sunday in the Gregorian
// calendar (the anonymous Gregorian algorithm).
func easterSunday(year int) time.Time {
	a := year % 19
	b, c := year/100, year%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
