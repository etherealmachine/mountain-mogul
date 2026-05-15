package sim

import (
	"testing"
	"time"
)

// TestSeasonCloseDate pins the Memorial Day computation across leap and
// non-leap years.
func TestSeasonCloseDate(t *testing.T) {
	cases := []struct {
		year     int
		wantDay  int
	}{
		{2027, 31}, // May 31, 2027 = Monday
		{2028, 29}, // May 31, 2028 = Wednesday → back to May 29
		{2029, 28}, // May 31, 2029 = Thursday → back to May 28
		{2030, 27}, // May 31, 2030 = Friday   → back to May 27
		{2031, 26}, // May 31, 2031 = Saturday → back to May 26
	}
	for _, tc := range cases {
		got := SeasonCloseDate(tc.year)
		if got.Year() != tc.year || got.Month() != time.May || got.Day() != tc.wantDay {
			t.Errorf("SeasonCloseDate(%d) = %s, want May %d, %d",
				tc.year, got.Format("Jan 2, 2006"), tc.wantDay, tc.year)
		}
		if got.Weekday() != time.Monday {
			t.Errorf("SeasonCloseDate(%d) is %s, want Monday", tc.year, got.Weekday())
		}
	}
}

// TestCalendarAt_SeasonSkip verifies the calendar walks day-by-day inside
// a season and jumps from Memorial Day directly to the next Nov 25.
func TestCalendarAt_SeasonSkip(t *testing.T) {
	open1 := SeasonOpenDate(seasonEpochYear)               // Nov 25, 2026
	close1 := SeasonCloseDate(seasonEpochYear + 1)         // May 31, 2027
	season1Days := int(close1.Sub(open1).Hours()/24) + 1   // 188
	open2 := SeasonOpenDate(seasonEpochYear + 1)           // Nov 25, 2027

	cases := []struct {
		name    string
		simTime float64
		want    Date
	}{
		{"day 0 = opening", 0, Date{25, "Nov", 2026}},
		{"day 1", secondsPerSimDay, Date{26, "Nov", 2026}},
		{"6 days in = Dec 1", 6 * secondsPerSimDay, Date{1, "Dec", 2026}},
		{"crosses new year", 37 * secondsPerSimDay, Date{1, "Jan", 2027}},
		{"last day of season 1", float64(season1Days-1) * secondsPerSimDay, Date{close1.Day(), "May", 2027}},
		{"first day of season 2", float64(season1Days) * secondsPerSimDay, Date{open2.Day(), "Nov", 2027}},
	}
	for _, tc := range cases {
		got := CalendarAt(tc.simTime)
		if got != tc.want {
			t.Errorf("%s: CalendarAt(%.1f) = %+v, want %+v", tc.name, tc.simTime, got, tc.want)
		}
	}
}

// TestCalendarAt_SeasonLength_AtTargetScale documents the design contract:
// at 4× TimeScale, one season ≈ 1 real hour. Lets future tuning catch
// regressions in either secondsPerSimDay or the season window.
func TestCalendarAt_SeasonLength_AtTargetScale(t *testing.T) {
	open := SeasonOpenDate(seasonEpochYear)
	close := SeasonCloseDate(seasonEpochYear + 1)
	days := int(close.Sub(open).Hours()/24) + 1
	const targetScale = 4.0
	realSeconds := float64(days) * secondsPerSimDay / targetScale
	realMinutes := realSeconds / 60
	if realMinutes < 50 || realMinutes > 75 {
		t.Errorf("season at %g× takes %.1f real min; want ~60 (50–75 OK)", targetScale, realMinutes)
	}
}
