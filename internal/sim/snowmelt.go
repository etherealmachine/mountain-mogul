package sim

import (
	"math"
	"time"
)

// Snowmelt uses an enhanced temperature-index model (Hock 1999): daily melt
// is positive degree-days × a melt factor that grows with the cell's
// clear-sky direct sun for the date, slope and aspect. North faces in
// December melt at the shade rate; south faces in April several times
// faster.
const (
	// lapseRate is the temperature drop per metre of elevation gain (°C/m).
	lapseRate = float32(0.0065)

	// meltFactorShade is metres SWE per positive degree-day with no direct
	// sun (sensible heat and diffuse light only).
	meltFactorShade = float32(0.0015)
	// meltFactorSun is the extra melt factor at sunExposure 1 — flat ground
	// under a clear equinox sky. Shade + sun spans the observed 2–8 mm SWE
	// per °C·day for seasonal snow.
	meltFactorSun = float32(0.0035)

	// rainMeltPerMM is additional melt per millimetre of rain (m SWE/mm),
	// standing in for the latent and turbulent heat a rain-on-snow day
	// brings; the rain's own heat is ~25× smaller.
	rainMeltPerMM = float32(0.00025)

	// resortLatitudeDeg sets the sun path. 45°N sits between the Alps,
	// Vermont and the Rockies' northern resorts.
	resortLatitudeDeg = 45.0

	// clearSkyTransmissivity is the fraction of direct beam reaching the
	// ground through one air mass; lower sun passes through more air.
	clearSkyTransmissivity = 0.75

	// cloudBeamBlock is the fraction of direct sun removed at full cloud
	// cover; the diffuse remainder is folded into meltFactorShade.
	cloudBeamBlock = float32(0.85)

	sunSteps = 48 // hour-angle samples across the day
)

// meltFactor is metres SWE melted per positive degree-day at the given sun
// exposure (see daySun.exposure).
func meltFactor(exposure float32) float32 {
	return meltFactorShade + meltFactorSun*exposure
}

// positiveDegreeDays integrates max(0, T) over a day whose temperature is a
// sinusoid of the given mean and half-swing, in °C·days. A −3 °C day
// peaking at +2 °C still melts a little in the afternoon.
func positiveDegreeDays(mean, halfSwing float32) float32 {
	if halfSwing <= 0 || mean >= halfSwing {
		return max(mean, 0)
	}
	if mean <= -halfSwing {
		return 0
	}
	m, a := float64(mean), float64(halfSwing)
	th := math.Asin(-m / a)
	return float32((m*(math.Pi-2*th) + 2*a*math.Cos(th)) / (2 * math.Pi))
}

// daySun is one day's sun path, sampled for per-cell exposure.
type daySun struct {
	dirs   [][3]float64 // world-space unit vectors toward the sun (+X east, +Y up, +Z south)
	weight []float64    // beam strength per sample, normalised by flatEquinox
	beam   float32      // cloud attenuation of the direct beam
}

// flatEquinox is the daily direct-beam integral on flat ground under a
// clear sky at the equinox; exposure 1 is defined as that.
var flatEquinox = func() float64 {
	d := sunPath(0)
	var sum float64
	for i, v := range d.dirs {
		sum += v[1] * d.weight[i]
	}
	return sum
}()

func newDaySun(date time.Time, cloud float32) daySun {
	d := sunPath(solarDeclination(date))
	for i := range d.weight {
		d.weight[i] /= flatEquinox
	}
	d.beam = 1 - cloudBeamBlock*min(max(cloud, 0), 1)
	return d
}

// sunPath samples the sun above the horizon for solar declination decl,
// with raw (unnormalised) air-mass-attenuated beam weights.
func sunPath(decl float64) daySun {
	var d daySun
	for i := 0; i < sunSteps; i++ {
		v := sunVector(decl, (float64(i)+0.5)/sunSteps*2*math.Pi-math.Pi)
		if v[1] <= 0.02 {
			continue
		}
		d.dirs = append(d.dirs, v)
		d.weight = append(d.weight, math.Pow(clearSkyTransmissivity, 1/v[1]))
	}
	return d
}

// instantSun is the direct beam at one moment, scaled so that averaging
// its exposure over a whole day gives the daySun exposure.
type instantSun struct {
	dir    [3]float64
	weight float64 // beam strength / the equinox flat-ground daily mean; 0 at night
}

func newInstantSun(sun SunState, cloud float32) instantSun {
	up := float64(sun.Dir[1])
	if up <= 0.02 {
		return instantSun{}
	}
	beam := 1 - cloudBeamBlock*min(max(cloud, 0), 1)
	return instantSun{
		dir:    [3]float64{float64(sun.Dir[0]), up, float64(sun.Dir[2])},
		weight: math.Pow(clearSkyTransmissivity, 1/up) / (flatEquinox / sunSteps) * float64(beam),
	}
}

// exposure is the direct sun right now on a surface with elevation
// gradient (gx, gz), on the daySun scale (1 = flat equinox daily mean).
func (b instantSun) exposure(gx, gz float32) float32 {
	if b.weight == 0 {
		return 0
	}
	nx, ny, nz := float64(-gx), 1.0, float64(-gz)
	c := (b.dir[0]*nx + b.dir[1]*ny + b.dir[2]*nz) / math.Sqrt(nx*nx+ny*ny+nz*nz)
	return float32(max(c, 0) * b.weight)
}

// exposure is the day's direct sun on a surface with elevation gradient
// (gx, gz), relative to flat ground at a clear equinox. Terrain shadowing
// is ignored.
func (d daySun) exposure(gx, gz float32) float32 {
	nx, ny, nz := float64(-gx), 1.0, float64(-gz)
	inv := 1 / math.Sqrt(nx*nx+ny*ny+nz*nz)
	nx, ny, nz = nx*inv, ny*inv, nz*inv
	var sum float64
	for i, v := range d.dirs {
		if c := v[0]*nx + v[1]*ny + v[2]*nz; c > 0 {
			sum += c * d.weight[i]
		}
	}
	return float32(sum) * d.beam
}
