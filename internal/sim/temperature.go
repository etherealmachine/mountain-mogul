package sim

import (
	"math"
	"time"
)

// The air temperature follows the usual asymmetric daily cycle: the low
// at sunrise, a half-cosine warm-up to the high at peakTempHour, then a
// long cool-down through the night to the next day's low. Temperatures
// are at the base of the mountain; cells higher up are colder by
// lapseRate.
const peakTempHour = 14.5

// tempCurve evaluates the daily cycle at clock hour h on date at site,
// given the previous, current and next days' weather.
func tempCurve(site Site, date time.Time, h float64, prev, today, next DayWeather) float32 {
	rise, _ := site.SunriseSunset(date)
	switch {
	case h < rise:
		// Still cooling from yesterday's peak.
		return cosineBlend(prev.TempHigh, today.TempLow, (h+24-peakTempHour)/(rise+24-peakTempHour))
	case h < peakTempHour:
		return cosineBlend(today.TempLow, today.TempHigh, (h-rise)/(peakTempHour-rise))
	default:
		nRise, _ := site.SunriseSunset(date.AddDate(0, 0, 1))
		return cosineBlend(today.TempHigh, next.TempLow, (h-peakTempHour)/(nRise+24-peakTempHour))
	}
}

// cosineBlend eases from a to b as f goes 0→1.
func cosineBlend(a, b float32, f float64) float32 {
	f = math.Max(0, math.Min(1, f))
	w := float32((1 - math.Cos(math.Pi*f)) / 2)
	return a + (b-a)*w
}

// TempAt returns the base-area air temperature (°C) at simTime, which
// must fall on the current sim day.
func (s *Simulation) TempAt(simTime float64) float32 {
	return tempCurve(s.Site, s.DateAt(simTime), HourOfDay(simTime), s.yesterday, s.Weather.Today(), s.tomorrow)
}

// TempNow is the base-area air temperature right now.
func (s *Simulation) TempNow() float32 { return s.TempAt(s.SimTime) }
