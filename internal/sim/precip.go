package sim

import (
	"math"
	"time"
)

// Precipitation by altitude. The weather chain's day is the base area's
// (DayWeather.AccumSWE or RainMM at the lowest ground). Higher up there's
// more of it (orographic lift), and a day that rains at the base snows
// where the air is below freezing. Until 2026-10-15 the base's day was
// laid on the whole mountain, so a storm near freezing rained on the
// summit too and no early-season snow built up (Bugs: snowpack too thin).

const (
	// orographicPer100m is how much more precipitation falls per 100 m
	// above the base: Sierra resorts get about a third more at the top
	// than at a base 800 m lower.
	orographicPer100m = 0.04
	// orographicMin bounds the factor below the base.
	orographicMin = 0.6
)

// orographic is the precipitation factor rise metres above the base
// (below it when negative): exponential, so scaling down to the base and
// back up to a station's altitude comes back to the station's amount.
func orographic(rise float32) float32 {
	return max(float32(math.Pow(1+orographicPer100m, float64(rise/100))), orographicMin)
}

// snowAllBelowC is the day's mean air temperature at and below which
// precipitation falls as snow: snow survives the fall through air a
// little above freezing (hydrology models put the rain-snow threshold at
// about +1 °C, a little higher in the dry Sierra air).
const snowAllBelowC = 2.5

// snowShareAt is the share of dw's precipitation that falls as snow rise
// metres above the base. A snow day snows everywhere above the base. A
// rain day turns to snow with height, from none at the base to all of it
// where the day's mean temperature (lapsed) falls to snowAllBelowC, at
// least 100 m up.
func snowShareAt(dw DayWeather, rise float32) float32 {
	switch {
	case dw.AccumSWE > 0:
		return 1
	case dw.RainMM <= 0 || rise <= 0:
		return 0
	}
	span := max(dw.TempC-snowAllBelowC, lapseRate*100)
	return clamp32(lapseRate*rise/span, 0, 1)
}

// PrecipAt is dw's precipitation rise metres above the base: new snow in
// metres SWE, and rain in mm.
func PrecipAt(dw DayWeather, rise float32) (snowSWE, rainMM float32) {
	water := (dw.AccumSWE*1000 + dw.RainMM) * orographic(rise)
	share := snowShareAt(dw, rise)
	return water * share / 1000, water * (1 - share)
}

// meltSeason scales the melt factor through the year: meltSeasonMin
// through the heart of winter (mid-November to mid-February), back to 1
// by late April, and down again over October and early November. A
// winter snowpack is colder than 0 °C through and has to warm before it
// melts, and gets little sun (the cold content degree-day models fold
// into a seasonal melt factor, as in the NWS's SNOW-17); by spring it's
// ripe. Calibrated on the Carson Pass (2,548 m) and Palisades Tahoe
// (2,441 m) SNOTELs, 2016–2025.
func meltSeason(date time.Time) float32 {
	doy := float32(date.YearDay())
	var f float32
	switch {
	case doy < meltRiseStart:
		f = 0
	case doy < meltRiseEnd:
		f = smoothstep((doy - meltRiseStart) / (meltRiseEnd - meltRiseStart))
	case doy < meltFallStart:
		f = 1
	case doy < meltFallEnd:
		f = 1 - smoothstep((doy-meltFallStart)/(meltFallEnd-meltFallStart))
	}
	return meltSeasonMin + (1-meltSeasonMin)*f
}

// The seasonal melt factor's turns, days of the year: rising from about
// 15 February to 20 April, falling from about 1 October to 15 November.
const (
	meltSeasonMin = float32(0.05)
	meltRiseStart = float32(46)
	meltRiseEnd   = float32(110)
	meltFallStart = float32(274)
	meltFallEnd   = float32(319)
)

func smoothstep(t float32) float32 {
	t = clamp32(t, 0, 1)
	return t * t * (3 - 2*t)
}
