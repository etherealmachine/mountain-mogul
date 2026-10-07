package sim

import (
	"math"
	"runtime"
	"sync"
	"time"

	"mountain-mogul/internal/world"
)

// SeasonSnowpack is a season's snowpack for a place, day by day from 1
// September: the weather chain's days for that season (the same ones the
// sim plays, Chain.RunUpTo) run through the melt model the sim uses
// (snowmelt.go). Each day brings its own snowfall, rain, high and low,
// and cloud, lapsed from the base area to each altitude band. It's
// tabled by altitude, by slope (the elevation gradient) and by how much
// of the sun the surrounding terrain lets through, so lookups for every
// cell are cheap.
type SeasonSnowpack struct {
	Start time.Time // 1 September of the season

	alt0 float32 // altitude of the first band, metres above sea level
	nAlt int
	days int
	// swe and recent are [day][alt][gz][gx][shade]: metres SWE on the
	// ground, and of it the snow that fell in the last packRecentDays.
	swe, recent []float32
}

const (
	packDays       = 300 // 1 September to late June
	packAltStep    = 25  // metres between altitude bands
	packGradSteps  = 9   // gradient samples per axis, from -packGradMax to packGradMax
	packGradMax    = 1.2 // rise/run; steeper ground clamps to it
	packShadeSteps = 3   // sun let through by terrain: 0, ½, 1
	packRecentDays = 7
)

// packClasses is the slope and shade classes per altitude band.
const packClasses = packGradSteps * packGradSteps * packShadeSteps

// NewSeasonSnowpack runs the season starting 1 September of year for
// climate c at latitude latDeg, for altitudes minAlt to maxAlt, with the
// weather at baseAlt as the sim has it (NewChainFor).
func NewSeasonSnowpack(c *world.Climate, latDeg float64, year int, baseAlt, minAlt, maxAlt float32) *SeasonSnowpack {
	s := &SeasonSnowpack{
		Start: time.Date(year, time.September, 1, 0, 0, 0, 0, time.UTC),
		alt0:  minAlt,
		nAlt:  int((maxAlt-minAlt)/packAltStep) + 2,
		days:  packDays,
	}
	n := s.days * s.nAlt * packClasses
	s.swe, s.recent = make([]float32, n), make([]float32, n)

	// Per day: the weather at the base and each slope class's sun.
	type dayInputs struct {
		wx       DayWeather
		exposure [packGradSteps * packGradSteps]float32
	}
	inputs := make([]dayInputs, s.days)
	chain := NewChainFor(c, baseAlt)
	for d := range inputs {
		date := s.Start.AddDate(0, 0, d)
		in := &inputs[d]
		in.wx = chain.Advance(date)
		sun := newDaySun(latDeg, date, in.wx.CloudCover)
		for gj := 0; gj < packGradSteps; gj++ {
			for gi := 0; gi < packGradSteps; gi++ {
				in.exposure[gj*packGradSteps+gi] = sun.exposure(gradSample(gi), gradSample(gj))
			}
		}
	}

	var wg sync.WaitGroup
	bands := make(chan int)
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fell := make([]float32, packRecentDays)
			for a := range bands {
				alt := s.alt0 + float32(a)*packAltStep
				lapse := lapseRate * (alt - baseAlt)
				for k := 0; k < packClasses; k++ {
					g, shade := k/packShadeSteps, float32(k%packShadeSteps)/(packShadeSteps-1)
					var swe, recent float32
					clear(fell)
					for d := range inputs {
						in := &inputs[d]
						mean := (in.wx.TempHigh+in.wx.TempLow)/2 - lapse
						half := (in.wx.TempHigh - in.wx.TempLow) / 2
						snow := in.wx.AccumSWE
						melt := meltFactor(in.exposure[g]*shade)*positiveDegreeDays(mean, half) + in.wx.RainMM*rainMeltPerMM
						swe = max(swe+snow-melt, 0)
						recent += snow - fell[d%packRecentDays]
						fell[d%packRecentDays] = snow
						i := (d*s.nAlt+a)*packClasses + k
						s.swe[i], s.recent[i] = swe, min(max(recent, 0), swe)
					}
				}
			}
		}()
	}
	for a := 0; a < s.nAlt; a++ {
		bands <- a
	}
	close(bands)
	wg.Wait()
	return s
}

// gradSample is the elevation gradient of slope sample i.
func gradSample(i int) float32 {
	return -packGradMax + 2*packGradMax*float32(i)/(packGradSteps-1)
}

// climateOn is c's climate on date, interpolated between the months'
// middles.
func climateOn(c *world.Climate, date time.Time) world.ClimateMonth {
	m := int(date.Month()) - 1
	days := float32(time.Date(date.Year(), date.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day())
	f := (float32(date.Day())-0.5)/days - 0.5
	other := (m + 1) % 12
	if f < 0 {
		other, f = (m+11)%12, -f
	}
	a, b := c.Months[m], c.Months[other]
	lerp := func(x, y float32) float32 { return x + (y-x)*f }
	return world.ClimateMonth{
		TempMean:  lerp(a.TempMean, b.TempMean),
		TempRange: lerp(a.TempRange, b.TempRange),
		WetDays:   lerp(a.WetDays, b.WetDays),
		WetMM:     lerp(a.WetMM, b.WetMM),
		Cloud:     lerp(a.Cloud, b.Cloud),
	}
}

// Day is date's index into the season, clamped to it.
func (s *SeasonSnowpack) Day(date time.Time) int {
	d := int(date.Sub(s.Start).Hours() / 24)
	return min(max(d, 0), s.days-1)
}

// At is the snow on day (see Day) at altitude alt on ground with
// elevation gradient (gx, gz), shade the fraction of the sun the terrain
// lets through: the metres SWE on the ground, and how much of that fell
// in the last week.
func (s *SeasonSnowpack) At(day int, alt, gx, gz, shade float32) (swe, recent float32) {
	fa := clamp32((alt-s.alt0)/packAltStep, 0, float32(s.nAlt-1))
	fx := clamp32((gx+packGradMax)/(2*packGradMax)*(packGradSteps-1), 0, packGradSteps-1)
	fz := clamp32((gz+packGradMax)/(2*packGradMax)*(packGradSteps-1), 0, packGradSteps-1)
	fs := clamp32(shade, 0, 1) * (packShadeSteps - 1)
	a0, x0, z0, s0 := int(fa), int(fx), int(fz), int(fs)
	a1, x1, z1, s1 := min(a0+1, s.nAlt-1), min(x0+1, packGradSteps-1), min(z0+1, packGradSteps-1), min(s0+1, packShadeSteps-1)
	ta, tx, tz, ts := fa-float32(a0), fx-float32(x0), fz-float32(z0), fs-float32(s0)
	for _, ca := range [2]struct {
		i int
		w float32
	}{{a0, 1 - ta}, {a1, ta}} {
		for _, cz := range [2]struct {
			i int
			w float32
		}{{z0, 1 - tz}, {z1, tz}} {
			for _, cx := range [2]struct {
				i int
				w float32
			}{{x0, 1 - tx}, {x1, tx}} {
				for _, cs := range [2]struct {
					i int
					w float32
				}{{s0, 1 - ts}, {s1, ts}} {
					wt := ca.w * cz.w * cx.w * cs.w
					if wt == 0 {
						continue
					}
					k := (cz.i*packGradSteps+cx.i)*packShadeSteps + cs.i
					i := (day*s.nAlt+ca.i)*packClasses + k
					swe += s.swe[i] * wt
					recent += s.recent[i] * wt
				}
			}
		}
	}
	return swe, recent
}

// OpeningDay is the first day of the season that flat, open ground at
// altitude alt holds swe metres of snow, or false if it never does.
func (s *SeasonSnowpack) OpeningDay(alt, swe float32) (time.Time, bool) {
	for d := 0; d < s.days; d++ {
		if v, _ := s.At(d, alt, 0, 0, 1); v >= swe {
			return s.Start.AddDate(0, 0, d), true
		}
	}
	return time.Time{}, false
}

// SnowShade is, for every cell (indexed x*Height+z), the share of its
// direct sun on date that the surrounding terrain lets through, at
// latitude latDeg. 1 where nothing shades it, or it gets no sun at all.
func SnowShade(t *world.Terrain, latDeg float64, date time.Time) []float32 {
	sun := newDaySun(latDeg, date, 0)
	hz := t.Horizon()
	out := make([]float32, t.Width*t.Height)
	var wg sync.WaitGroup
	cols := make(chan int)
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for x := range cols {
				for z := 0; z < t.Height; z++ {
					gx, gz := t.GradientAt(x, z)
					nx, ny, nz := float64(-gx), 1.0, float64(-gz)
					inv := 1 / math.Sqrt(nx*nx+ny*ny+nz*nz)
					var lit, all float64
					for i, v := range sun.dirs {
						c := (v[0]*nx + v[1]*ny + v[2]*nz) * inv
						if c <= 0 {
							continue
						}
						w := c * sun.weight[i]
						all += w
						lit += w * float64(hz.SunVisibility(x, z, [3]float32{float32(v[0]), float32(v[1]), float32(v[2])}))
					}
					s := float32(1)
					if all > 0 {
						s = float32(lit / all)
					}
					out[x*t.Height+z] = s
				}
			}
		}()
	}
	for x := 0; x < t.Width; x++ {
		cols <- x
	}
	close(cols)
	wg.Wait()
	return out
}
