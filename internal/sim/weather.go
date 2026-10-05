package sim

import (
	"math/rand"
	"time"

	"mountain-mogul/internal/world"
)

// WeatherState is the canonical daily weather condition.
type WeatherState uint8

const (
	WeatherClear     WeatherState = iota // sunny, no precipitation
	WeatherOvercast                      // cloudy, no precipitation
	WeatherLightSnow                     // light to moderate snowfall
	WeatherHeavySnow                     // heavy snow / blizzard
	WeatherRain                          // above-freezing precipitation
	weatherStateCount
)

func (s WeatherState) String() string {
	switch s {
	case WeatherClear:
		return "Clear"
	case WeatherOvercast:
		return "Overcast"
	case WeatherLightSnow:
		return "Snow"
	case WeatherHeavySnow:
		return "Storm"
	case WeatherRain:
		return "Rain"
	}
	return "?"
}

// DayWeather is the complete weather description for one in-game day.
// Consumers (sim effects, HUD, demand) read from this; nothing reaches
// into Chain directly. Designed to be the stable contract across
// implementations — a 5-day forecast, a climate model, or a hand-authored
// calendar all produce DayWeather.
type DayWeather struct {
	State      WeatherState // canonical condition for the day
	TempC      float32      // daily mean temperature, °C (negative = below freezing)
	TempHigh   float32      // daily high temperature, °C
	TempLow    float32      // daily low temperature, °C
	CloudCover float32      // 0..1; for sky rendering
	AccumSWE   float32      // new snow, metres snow-water-equivalent (0 on non-snow days)
	RainMM     float32      // rainfall, mm (0 on non-rain days; no game effect yet)
}

// IsSnowing reports whether the day has any snowfall.
func (d DayWeather) IsSnowing() bool {
	return d.State == WeatherLightSnow || d.State == WeatherHeavySnow
}

// IsRaining reports whether the day has rainfall.
func (d DayWeather) IsRaining() bool { return d.State == WeatherRain }

// Chain is a Markov-chain daily weather generator. Call Advance once per
// in-game day rollover, passing the simulation's shared Rng so all weather
// randomness flows through the single deterministic source.
type Chain struct {
	state  WeatherState
	today  DayWeather
	months *[12]monthProfile // shared, never written after construction
}

// NewChain returns a weather chain for the generic climate, starting in
// clear conditions.
func NewChain() *Chain {
	return &Chain{
		state:  WeatherClear,
		today:  DayWeather{State: WeatherClear, TempC: -5, TempHigh: 0, TempLow: -10, CloudCover: 0.1},
		months: &monthProfiles,
	}
}

// NewChainFor returns a weather chain for climate c with temperatures at
// the base area, baseAltitude metres above sea level; the generic one
// when c is nil.
func NewChainFor(c *world.Climate, baseAltitude float32) *Chain {
	ch := NewChain()
	if c != nil {
		ch.months = climateProfiles(c, baseAltitude)
	}
	return ch
}

// Today returns the most recently generated day's weather without advancing.
func (c *Chain) Today() DayWeather { return c.today }

// SetToday overrides today's weather without touching the chain state.
// Used by debug tooling to force conditions for rendering checks.
func (c *Chain) SetToday(d DayWeather) { c.today = d }

// Advance steps to the next day and returns its weather. It uses a
// deterministic per-day seed derived from the current chain state and the
// target calendar date, so Forecast called before this day will have
// predicted the same result.
func (c *Chain) Advance(t time.Time) DayWeather {
	rng := rand.New(rand.NewSource(c.daySeed(t)))
	c.today = c.sample(rng, t.Month())
	return c.today
}

// Forecast returns the predicted weather for the next n days without mutating
// c. Each day is seeded identically to Advance, so the forecast matches what
// will actually happen when those days arrive.
func (c *Chain) Forecast(from time.Time, n int) []DayWeather {
	clone := *c
	out := make([]DayWeather, n)
	for i := range out {
		t := from.AddDate(0, 0, i+1)
		rng := rand.New(rand.NewSource(clone.daySeed(t)))
		out[i] = clone.sample(rng, t.Month())
	}
	return out
}

// daySeed returns the deterministic RNG seed for advancing to day t.
// Keyed on the current chain state and the calendar date (not wall-clock
// seconds) so the seed is stable throughout a day and identical between
// Advance and Forecast for the same step.
func (c *Chain) daySeed(t time.Time) int64 {
	y, m, d := t.Date()
	dateKey := int64(y)*10000 + int64(m)*100 + int64(d)
	return int64(c.state)*1_000_000_007 + dateKey
}

// =============================================================================
// Monthly profiles
// =============================================================================

// monthProfile holds per-month parameters. Indexed by time.Month-1 (Jan=0).
type monthProfile struct {
	// tendency[s] is the unnormalized probability of state s this month.
	// Used as the stationary distribution when the chain re-draws.
	tendency [weatherStateCount]float32
	// meanTempC is the baseline daily mean temperature.
	meanTempC float32
	// diurnalScale multiplies stateDiurnal; 0 means 1.
	diurnalScale float32
	// wetMM is the mean precipitation on a wet day, mm of water; 0 keeps
	// the generic snow and rain amounts.
	wetMM float32
}

// heavySnowShare is the fraction of snow days that are storms. With
// light days drawing 0.3–1 × the wet-day mean and storms 1–3 ×, it keeps
// the mean over snow days at the wet-day mean.
const heavySnowShare = 0.26

// climateProfiles turns c into monthly chain parameters for a base area
// at baseAltitude. Wet days split into rain and snow by the month's base
// temperature, dry days into clear and overcast by its cloud cover. The
// draw weights are divided out by each state's persistence so the
// chain's long-run share of each state matches the climate, and the
// mean temperature and daily swing are corrected for the per-state
// offsets so the month averages to the climate's.
func climateProfiles(c *world.Climate, baseAltitude float32) *[12]monthProfile {
	var out [12]monthProfile
	for m := range out {
		cm := c.Months[m]
		temp := c.TempAt(m, baseAltitude)
		wet := clamp32(cm.WetDays, 0, 1)
		rain := wet * clamp32((temp+1)/5, 0, 1)
		snow := wet - rain
		dry := 1 - wet
		overcast := dry * clamp32((cm.Cloud-0.15)/0.6, 0.1, 0.9)
		share := [weatherStateCount]float32{
			WeatherClear:     dry - overcast,
			WeatherOvercast:  overcast,
			WeatherLightSnow: snow * (1 - heavySnowShare),
			WeatherHeavySnow: snow * heavySnowShare,
			WeatherRain:      rain,
		}
		p := &out[m]
		var offset, diurnal float32
		for s, f := range share {
			p.tendency[s] = f * (1 - persistence[s])
			offset += f * stateTempOffset[s]
			diurnal += f * stateDiurnal[s]
		}
		p.meanTempC = temp - offset
		if cm.TempRange > 0 && diurnal > 0 {
			p.diurnalScale = cm.TempRange / 2 / diurnal
		}
		p.wetMM = cm.WetMM
	}
	return &out
}

// monthProfiles is indexed by time.Month-1. All twelve months are present;
// the calendar runs through the off-season, so every month gets sampled.
//
// Tendency weights (Clear, Overcast, LightSnow, HeavySnow, Rain):
// Jan–Feb: peak cold and snow. Mar: spring thaw starts.
// Apr–May: shoulder season with rain. Jun–Oct: off-season warm.
// Nov–Dec: early season with mixed precip.
var monthProfiles = [12]monthProfile{
	// Jan
	{tendency: [weatherStateCount]float32{0.18, 0.27, 0.32, 0.20, 0.03}, meanTempC: -10},
	// Feb
	{tendency: [weatherStateCount]float32{0.22, 0.27, 0.28, 0.16, 0.07}, meanTempC: -9},
	// Mar
	{tendency: [weatherStateCount]float32{0.28, 0.27, 0.22, 0.10, 0.13}, meanTempC: -5},
	// Apr
	{tendency: [weatherStateCount]float32{0.28, 0.27, 0.15, 0.05, 0.25}, meanTempC: -1},
	// May
	{tendency: [weatherStateCount]float32{0.30, 0.28, 0.08, 0.03, 0.31}, meanTempC: 4},
	// Jun (off-season)
	{tendency: [weatherStateCount]float32{0.38, 0.30, 0.01, 0.00, 0.31}, meanTempC: 9},
	// Jul
	{tendency: [weatherStateCount]float32{0.40, 0.30, 0.00, 0.00, 0.30}, meanTempC: 13},
	// Aug
	{tendency: [weatherStateCount]float32{0.40, 0.30, 0.00, 0.00, 0.30}, meanTempC: 13},
	// Sep
	{tendency: [weatherStateCount]float32{0.38, 0.30, 0.02, 0.00, 0.30}, meanTempC: 8},
	// Oct
	{tendency: [weatherStateCount]float32{0.30, 0.30, 0.08, 0.02, 0.30}, meanTempC: 2},
	// Nov
	{tendency: [weatherStateCount]float32{0.25, 0.30, 0.20, 0.08, 0.17}, meanTempC: -3},
	// Dec
	{tendency: [weatherStateCount]float32{0.22, 0.28, 0.28, 0.15, 0.07}, meanTempC: -7},
}

// persistence[s] is the probability of remaining in state s when the chain
// would otherwise redraw. Captures weather-system inertia.
var persistence = [weatherStateCount]float32{
	0.75, // Clear — clear spells last
	0.68, // Overcast
	0.65, // LightSnow
	0.60, // HeavySnow — storms break up faster
	0.60, // Rain
}

// stateTempOffset[s] shifts the month's meanTempC when this state is active.
// Snow states push colder; rain pushes warmer (physically motivated).
var stateTempOffset = [weatherStateCount]float32{
	+4, // Clear — radiant warming
	0,  // Overcast
	-2, // LightSnow
	-5, // HeavySnow
	+5, // Rain — requires above-freezing air mass
}

// stateTempNoise[s] is the standard deviation of the daily temperature sample.
var stateTempNoise = [weatherStateCount]float32{
	4, // Clear
	4, // Overcast
	3, // LightSnow
	3, // HeavySnow
	3, // Rain
}

// stateDiurnal[s] is the half-swing of the daily temperature cycle in °C.
// Added to the mean to get TempHigh and subtracted to get TempLow.
// Clear days have the largest swing (cold nights, warm sun); storm/overcast
// days suppress radiation and narrow the range.
var stateDiurnal = [weatherStateCount]float32{
	5.0, // Clear — strong radiative swing
	2.5, // Overcast — clouds act as blanket
	2.0, // LightSnow — overcast + latent heat narrow the range
	1.5, // HeavySnow — blizzard; near-uniform temperature
	2.5, // Rain — moderate swing; warm air mass
}

// cloudBase[s] and cloudNoise[s] define the cloud-cover range per state.
var (
	cloudBase  = [weatherStateCount]float32{0.05, 0.70, 0.80, 0.90, 0.75}
	cloudRange = [weatherStateCount]float32{0.15, 0.25, 0.18, 0.10, 0.20}
)

// =============================================================================
// Core sampling
// =============================================================================

// sample draws one day of weather for the given month using rng, updates
// c.state, and returns the result.
func (c *Chain) sample(rng *rand.Rand, month time.Month) DayWeather {
	prof := &c.months[month-1]

	// Transition: persist in current state or redraw from monthly tendency.
	if rng.Float32() >= persistence[c.state] {
		c.state = drawWeatherState(rng, prof.tendency)
	}

	// Temperature: month baseline + state offset + Gaussian noise.
	temp := prof.meanTempC +
		stateTempOffset[c.state] +
		float32(rng.NormFloat64())*stateTempNoise[c.state]

	// Enforce coherence: snow needs sub-freezing, rain needs above-freezing.
	switch c.state {
	case WeatherLightSnow, WeatherHeavySnow:
		if temp > -0.5 {
			temp = -0.5
		}
	case WeatherRain:
		if temp < 0.5 {
			temp = 0.5
		}
	}

	// Diurnal high/low: mean ± half-swing with small independent noise.
	diurnal := stateDiurnal[c.state]
	if prof.diurnalScale > 0 {
		diurnal *= prof.diurnalScale
	}
	tempHigh := temp + diurnal + float32(rng.NormFloat64())*1.5
	tempLow := temp - diurnal + float32(rng.NormFloat64())*1.5
	if tempHigh < tempLow {
		tempHigh, tempLow = tempLow, tempHigh
	}
	// Rain days must stay above freezing even at the low.
	if c.state == WeatherRain && tempLow < 0.5 {
		tempLow = 0.5
	}

	// Cloud cover.
	cloud := cloudBase[c.state] + rng.Float32()*cloudRange[c.state]
	if cloud > 1 {
		cloud = 1
	}

	// Snow accumulation (metres SWE).
	var accumSWE, rainMM float32
	if wet := prof.wetMM; wet > 0 {
		switch c.state {
		case WeatherLightSnow:
			accumSWE = wet * (0.3 + 0.7*rng.Float32()) / 1000
		case WeatherHeavySnow:
			accumSWE = wet * (1 + 2*rng.Float32()) / 1000
		case WeatherRain:
			rainMM = wet * (0.3 + 1.4*rng.Float32())
		}
	} else {
		switch c.state {
		case WeatherLightSnow:
			accumSWE = 0.003 + rng.Float32()*0.009 // 0.3–1.2 cm SWE
		case WeatherHeavySnow:
			accumSWE = 0.010 + rng.Float32()*0.025 // 1.0–3.5 cm SWE
		case WeatherRain:
			rainMM = 2 + rng.Float32()*18 // 2–20 mm
		}
	}

	return DayWeather{
		State:      c.state,
		TempC:      temp,
		TempHigh:   tempHigh,
		TempLow:    tempLow,
		CloudCover: cloud,
		AccumSWE:   accumSWE,
		RainMM:     rainMM,
	}
}

// drawWeatherState samples a state from the given unnormalized weight vector.
func drawWeatherState(rng *rand.Rand, weights [weatherStateCount]float32) WeatherState {
	var sum float32
	for _, w := range weights {
		sum += w
	}
	r := rng.Float32() * sum
	var acc float32
	for s, w := range weights {
		acc += w
		if r < acc {
			return WeatherState(s)
		}
	}
	return weatherStateCount - 1
}
