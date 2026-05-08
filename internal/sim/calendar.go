package sim

// Calendar / weather stubs for the top-bar HUD. Today these are decorative —
// the simulation doesn't read them. They exist so the menu has something to
// render and the eventual season/weather systems have a home to grow into.

// secondsPerSimDay sets how fast in-game days tick relative to sim seconds.
// 1 sim minute = 1 in-game day at 5× TimeScale (the default), which feels
// right for a tycoon's "watch the season unfold" pacing — fast enough to see
// weather change in a session, slow enough that the date isn't a blur.
const secondsPerSimDay = 60.0

// monthNames is the display-order list used for the date HUD.
var monthNames = [12]string{
	"Dec", "Jan", "Feb", "Mar", "Apr", "May",
	"Jun", "Jul", "Aug", "Sep", "Oct", "Nov",
}

// Date is a calendar position derived from SimTime. The season starts in
// December (year 1) and rolls forward; months are uniform 30-day buckets so
// the math stays trivial.
type Date struct {
	Day   int    // 1..30
	Month string // "Dec", "Jan", ...
	Year  int    // 1, 2, ...
}

// CalendarAt returns the in-game date for the given SimTime.
func CalendarAt(simTime float64) Date {
	totalDays := int(simTime / secondsPerSimDay)
	day := totalDays%30 + 1
	monthIdx := (totalDays / 30) % 12
	year := 1 + totalDays/(30*12)
	return Date{
		Day:   day,
		Month: monthNames[monthIdx],
		Year:  year,
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
	Now      WeatherKind
	Next     WeatherKind
	TempF    int
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
