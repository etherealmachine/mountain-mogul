package sim

import (
	"math"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// The sun follows the real path for the Site's latitude on the calendar
// date. Without a time zone the clock is local solar time, so the sun
// peaks due south at 12:00; with one it is the place's civil time,
// daylight saving included, and solar noon drifts off 12:00 with the
// longitude and the equation of time.

// sunriseAltitudeRad is the sun-centre altitude at sunrise/sunset: the
// disc's upper limb touches the horizon, lifted by refraction.
const sunriseAltitudeRad = -0.833 * math.Pi / 180

// referenceLatitudeDeg is where drawn maps are, and where the melt model's
// sun exposure is normalised: 45°N sits between the Alps, Vermont and the
// Rockies' northern resorts.
const referenceLatitudeDeg = 45.0

// Site is where the sun is computed for.
type Site struct {
	LatDeg, LonDeg float64
	Zone           *time.Location // nil: the clock is solar time
}

// DefaultSite is a drawn map's: 45°N on solar time.
var DefaultSite = Site{LatDeg: referenceLatitudeDeg}

// SiteOf is w's place, from its imported bounds and time zone.
func SiteOf(w *world.World) Site {
	if w.Geo == nil {
		return DefaultSite
	}
	s := Site{
		LatDeg: (w.Geo.MinLat + w.Geo.MaxLat) / 2,
		LonDeg: (w.Geo.MinLon + w.Geo.MaxLon) / 2,
	}
	if w.TimeZone != "" {
		if loc, err := time.LoadLocation(w.TimeZone); err == nil {
			s.Zone = loc
		}
	}
	return s
}

// solarOffset is local solar time minus clock time on date, in hours.
func (s Site) solarOffset(date time.Time) float64 {
	if s.Zone == nil {
		return 0
	}
	y, m, d := date.Date()
	_, off := time.Date(y, m, d, 12, 0, 0, 0, s.Zone).Zone()
	return s.LonDeg/15 - float64(off)/3600 + equationOfTime(date)/60
}

// equationOfTime is apparent minus mean solar time on date, in minutes.
func equationOfTime(date time.Time) float64 {
	b := 2 * math.Pi * float64(date.YearDay()-81) / 364
	return 9.87*math.Sin(2*b) - 7.53*math.Cos(b) - 1.5*math.Sin(b)
}

// SunState is the sun's position at one moment.
type SunState struct {
	// Dir is the unit vector from the ground toward the sun in world
	// space (+X east, +Y up, +Z south). Below the horizon Dir.Y < 0.
	Dir mgl32.Vec3
	// Elevation is the altitude above the horizon in radians (negative
	// at night).
	Elevation float32
}

// solarDeclination returns the sun's declination in radians for date.
func solarDeclination(date time.Time) float64 {
	doy := float64(date.YearDay())
	return -23.44 * math.Pi / 180 * math.Cos(2*math.Pi*(doy+10)/365)
}

// sunVector is the world-space unit vector toward the sun at latitude
// latDeg, solar hour angle h (radians, 0 = noon, negative = morning) and
// declination decl.
func sunVector(latDeg, decl, h float64) [3]float64 {
	lat := latDeg * math.Pi / 180
	east := -math.Cos(decl) * math.Sin(h)
	north := math.Cos(lat)*math.Sin(decl) - math.Sin(lat)*math.Cos(decl)*math.Cos(h)
	up := math.Sin(lat)*math.Sin(decl) + math.Cos(lat)*math.Cos(decl)*math.Cos(h)
	return [3]float64{east, up, -north}
}

// hourAngle converts a solar hour to the solar hour angle in radians.
func hourAngle(hour float64) float64 {
	return (hour - 12) * math.Pi / 12
}

// SunAt returns the sun's position on date at the given clock hour.
func (s Site) SunAt(date time.Time, hour float64) SunState {
	v := sunVector(s.LatDeg, solarDeclination(date), hourAngle(hour+s.solarOffset(date)))
	return SunState{
		Dir:       mgl32.Vec3{float32(v[0]), float32(v[1]), float32(v[2])},
		Elevation: float32(math.Asin(v[1])),
	}
}

// SunriseSunset returns the clock hours of sunrise and sunset on date.
func (s Site) SunriseSunset(date time.Time) (rise, set float64) {
	lat := s.LatDeg * math.Pi / 180
	decl := solarDeclination(date)
	cosH := (math.Sin(sunriseAltitudeRad) - math.Sin(lat)*math.Sin(decl)) / (math.Cos(lat) * math.Cos(decl))
	cosH = math.Max(-1, math.Min(1, cosH))
	half := math.Acos(cosH) * 12 / math.Pi
	noon := 12 - s.solarOffset(date)
	return noon - half, noon + half
}

// Sun returns the sun's position right now.
func (s *Simulation) Sun() SunState {
	return s.Site.SunAt(s.DateAt(s.SimTime), HourOfDay(s.SimTime))
}
