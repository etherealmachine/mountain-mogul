package sim

import (
	"math"
	"time"

	"github.com/go-gl/mathgl/mgl32"
)

// The sun follows the real path for resortLatitudeDeg on the calendar
// date. Clock time is local solar time, so the sun peaks due south at
// 12:00 and sunrise/sunset are symmetric about noon.

// sunriseAltitudeRad is the sun-centre altitude at sunrise/sunset: the
// disc's upper limb touches the horizon, lifted by refraction.
const sunriseAltitudeRad = -0.833 * math.Pi / 180

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

// sunVector is the world-space unit vector toward the sun at solar hour
// angle h (radians, 0 = noon, negative = morning) for declination decl.
func sunVector(decl, h float64) [3]float64 {
	lat := resortLatitudeDeg * math.Pi / 180
	east := -math.Cos(decl) * math.Sin(h)
	north := math.Cos(lat)*math.Sin(decl) - math.Sin(lat)*math.Cos(decl)*math.Cos(h)
	up := math.Sin(lat)*math.Sin(decl) + math.Cos(lat)*math.Cos(decl)*math.Cos(h)
	return [3]float64{east, up, -north}
}

// hourAngle converts a clock hour to the solar hour angle in radians.
func hourAngle(hour float64) float64 {
	return (hour - 12) * math.Pi / 12
}

// SunAt returns the sun's position on date at the given clock hour.
func SunAt(date time.Time, hour float64) SunState {
	v := sunVector(solarDeclination(date), hourAngle(hour))
	return SunState{
		Dir:       mgl32.Vec3{float32(v[0]), float32(v[1]), float32(v[2])},
		Elevation: float32(math.Asin(v[1])),
	}
}

// SunriseSunset returns the clock hours of sunrise and sunset on date.
func SunriseSunset(date time.Time) (rise, set float64) {
	lat := resortLatitudeDeg * math.Pi / 180
	decl := solarDeclination(date)
	cosH := (math.Sin(sunriseAltitudeRad) - math.Sin(lat)*math.Sin(decl)) / (math.Cos(lat) * math.Cos(decl))
	cosH = math.Max(-1, math.Min(1, cosH))
	half := math.Acos(cosH) * 12 / math.Pi
	return 12 - half, 12 + half
}

// Sun returns the sun's position right now.
func (s *Simulation) Sun() SunState {
	return SunAt(s.DateAt(s.SimTime), HourOfDay(s.SimTime))
}
