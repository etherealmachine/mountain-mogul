package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mountain-mogul/internal/world"
)

// climateYears is how many recent whole calendar years a climate averages.
const climateYears = 10

// SNOTEL stations further than snotelMaxKm from the map, or with fewer than
// snotelMinYears of record, aren't used.
const (
	snotelMaxKm    = 30
	snotelMinYears = 5
)

// FetchClimate builds the climate of the place at (lat, lon) and finds its
// IANA time zone. Open-Meteo's reanalysis archive covers everywhere (and
// supplies cloud cover, the storm wind, and the zone), with temperatures
// at the map's base altitude. In the US, a nearby SNOTEL snow station's
// own temperature and precipitation records replace the reanalysis ones,
// which smear mountain snowfall over a 25 km grid. midAlt, the middle of
// the map's height range, picks the station closest in height.
func FetchClimate(ctx context.Context, lat, lon float64, baseAlt, midAlt float32) (*world.Climate, string, error) {
	now := time.Now().UTC()
	end := time.Date(now.Year()-1, 12, 31, 0, 0, 0, 0, time.UTC)
	start := time.Date(now.Year()-climateYears, 1, 1, 0, 0, 0, 0, time.UTC)
	years := fmt.Sprintf("%d–%d", start.Year(), end.Year())

	c, zone, err := openMeteoClimate(ctx, lat, lon, baseAlt, start, end)
	if err != nil {
		return nil, "", err
	}
	c.Source = "Open-Meteo reanalysis, " + years
	st, err := nearestSnotel(ctx, lat, lon, midAlt, now)
	if err != nil || st == nil {
		return c, zone, nil
	}
	months, err := snotelMonths(ctx, st.triplet, start, end)
	if err != nil {
		return c, zone, nil
	}
	c.RefAltitude = st.altitude
	for m := range c.Months {
		cloud := c.Months[m].Cloud
		c.Months[m] = months[m]
		c.Months[m].Cloud = cloud
	}
	c.Source = fmt.Sprintf("SNOTEL %s (%.0f m, %.0f km away), %s; cloud and wind from Open-Meteo", st.name, st.altitude, st.km, years)
	return c, zone, nil
}

// monthAcc accumulates daily records into a world.ClimateMonth.
type monthAcc struct {
	temp, rng, cloud    float64
	nTemp, nRng, nCloud int
	wet, nPrecip        int
	wetMM               float64
}

func (a *monthAcc) addPrecip(mm float64) {
	a.nPrecip++
	if mm >= world.WetDayMM {
		a.wet++
		a.wetMM += mm
	}
}

func (a *monthAcc) month() world.ClimateMonth {
	var m world.ClimateMonth
	if a.nTemp > 0 {
		m.TempMean = float32(a.temp / float64(a.nTemp))
	}
	if a.nRng > 0 {
		m.TempRange = float32(a.rng / float64(a.nRng))
	}
	if a.nCloud > 0 {
		m.Cloud = float32(a.cloud / float64(a.nCloud))
	}
	if a.nPrecip > 0 {
		m.WetDays = float32(a.wet) / float32(a.nPrecip)
	}
	if a.wet > 0 {
		m.WetMM = float32(a.wetMM / float64(a.wet))
	}
	return m
}

var climateClient = &http.Client{Timeout: 2 * time.Minute}

func getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "mountain-mogul/1.0")
	resp, err := climateClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %s", u[:strings.IndexByte(u, '?')], resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func openMeteoClimate(ctx context.Context, lat, lon float64, baseAlt float32, start, end time.Time) (*world.Climate, string, error) {
	q := url.Values{}
	q.Set("latitude", fmt.Sprintf("%.5f", lat))
	q.Set("longitude", fmt.Sprintf("%.5f", lon))
	q.Set("start_date", start.Format("2006-01-02"))
	q.Set("end_date", end.Format("2006-01-02"))
	q.Set("daily", "temperature_2m_mean,temperature_2m_max,temperature_2m_min,precipitation_sum,cloud_cover_mean,wind_direction_10m_dominant")
	q.Set("elevation", fmt.Sprintf("%.0f", baseAlt))
	q.Set("timezone", "auto")
	var body struct {
		Timezone string `json:"timezone"`
		Daily    struct {
			Time   []string   `json:"time"`
			TMean  []*float64 `json:"temperature_2m_mean"`
			TMax   []*float64 `json:"temperature_2m_max"`
			TMin   []*float64 `json:"temperature_2m_min"`
			Precip []*float64 `json:"precipitation_sum"`
			Cloud  []*float64 `json:"cloud_cover_mean"`
			Wind   []*float64 `json:"wind_direction_10m_dominant"`
		} `json:"daily"`
	}
	if err := getJSON(ctx, "https://archive-api.open-meteo.com/v1/archive?"+q.Encode(), &body); err != nil {
		return nil, "", fmt.Errorf("climate: %w", err)
	}
	d := body.Daily
	var acc [12]monthAcc
	var windE, windN float64
	at := func(v []*float64, i int) (float64, bool) {
		if i < len(v) && v[i] != nil {
			return *v[i], true
		}
		return 0, false
	}
	for i, day := range d.Time {
		t, err := time.Parse("2006-01-02", day)
		if err != nil {
			continue
		}
		a := &acc[t.Month()-1]
		if v, ok := at(d.TMean, i); ok {
			a.temp += v
			a.nTemp++
		}
		hi, ok1 := at(d.TMax, i)
		lo, ok2 := at(d.TMin, i)
		if ok1 && ok2 {
			a.rng += hi - lo
			a.nRng++
		}
		if v, ok := at(d.Cloud, i); ok {
			a.cloud += v / 100
			a.nCloud++
		}
		p, ok := at(d.Precip, i)
		if ok {
			a.addPrecip(p)
		}
		// The storm wind: where it comes from on wet winter days.
		if dir, okW := at(d.Wind, i); okW && ok && p >= 5 && (t.Month() >= time.November || t.Month() <= time.April) {
			r := dir * math.Pi / 180
			windE += math.Sin(r)
			windN += math.Cos(r)
		}
	}
	if acc[0].nTemp == 0 {
		return nil, "", fmt.Errorf("climate: no records for %.3f, %.3f", lat, lon)
	}
	c := &world.Climate{RefAltitude: baseAlt, WindDeg: 270}
	if windE != 0 || windN != 0 {
		from := math.Atan2(windE, windN) * 180 / math.Pi
		c.WindDeg = float32(math.Mod(from+180+360, 360))
	}
	for m := range acc {
		c.Months[m] = acc[m].month()
	}
	return c, body.Timezone, nil
}

type snotelStation struct {
	triplet, name string
	altitude      float32 // metres
	km            float64
}

// nearestSnotel is the SNOTEL station that best stands in for a map at
// (lat, lon) whose middle is midAlt metres up, scoring a kilometre away
// the same as 100 m off in height; nil if none is close enough.
func nearestSnotel(ctx context.Context, lat, lon float64, midAlt float32, now time.Time) (*snotelStation, error) {
	var list []struct {
		Triplet   string  `json:"stationTriplet"`
		Name      string  `json:"name"`
		Lat       float64 `json:"latitude"`
		Lon       float64 `json:"longitude"`
		ElevFt    float64 `json:"elevation"`
		BeginDate string  `json:"beginDate"`
		EndDate   string  `json:"endDate"`
	}
	u := "https://wcc.sc.egov.usda.gov/awdbRestApi/services/v1/stations?stationTriplets=*:*:SNTL&returnForecastPointMetadata=false&returnReservoirMetadata=false&returnStationElements=false"
	if err := getJSON(ctx, u, &list); err != nil {
		return nil, err
	}
	latM, lonM := MetresPerDegree(lat)
	var best *snotelStation
	bestScore := math.Inf(1)
	for _, s := range list {
		km := math.Hypot((s.Lat-lat)*latM, (s.Lon-lon)*lonM) / 1000
		if km > snotelMaxKm {
			continue
		}
		begin, err := time.Parse("2006-01-02 15:04", s.BeginDate)
		if err != nil || now.Sub(begin) < snotelMinYears*365*24*time.Hour {
			continue
		}
		if end, err := time.Parse("2006-01-02 15:04", s.EndDate); err == nil && end.Before(now.AddDate(-1, 0, 0)) {
			continue
		}
		alt := float32(s.ElevFt * 0.3048)
		score := km + math.Abs(float64(alt-midAlt))/100
		if score < bestScore {
			bestScore = score
			best = &snotelStation{triplet: s.Triplet, name: s.Name, altitude: alt, km: km}
		}
	}
	return best, nil
}

// snotelMonths averages a station's daily temperature and precipitation
// by calendar month. Readings outside plausible ranges are dropped; the
// sensors glitch now and then.
func snotelMonths(ctx context.Context, triplet string, start, end time.Time) ([12]world.ClimateMonth, error) {
	var out [12]world.ClimateMonth
	q := url.Values{}
	q.Set("stationTriplets", triplet)
	q.Set("elements", "TAVG,TMAX,TMIN,PRCP")
	q.Set("duration", "DAILY")
	q.Set("beginDate", start.Format("2006-01-02"))
	q.Set("endDate", end.Format("2006-01-02"))
	var body []struct {
		Data []struct {
			Element struct {
				Code string `json:"elementCode"`
			} `json:"stationElement"`
			Values []struct {
				Date  string   `json:"date"`
				Value *float64 `json:"value"`
			} `json:"values"`
		} `json:"data"`
	}
	if err := getJSON(ctx, "https://wcc.sc.egov.usda.gov/awdbRestApi/services/v1/data?"+q.Encode(), &body); err != nil {
		return out, err
	}
	if len(body) == 0 {
		return out, fmt.Errorf("SNOTEL %s: no data", triplet)
	}
	series := map[string]map[string]float64{}
	for _, d := range body[0].Data {
		m := map[string]float64{}
		for _, v := range d.Values {
			if v.Value != nil {
				m[v.Date] = *v.Value
			}
		}
		series[d.Element.Code] = m
	}
	fToC := func(f float64) float64 { return (f - 32) * 5 / 9 }
	var acc [12]monthAcc
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		a := &acc[day.Month()-1]
		if f, ok := series["TAVG"][key]; ok {
			if c := fToC(f); c > -45 && c < 35 {
				a.temp += c
				a.nTemp++
			}
		}
		hi, ok1 := series["TMAX"][key]
		lo, ok2 := series["TMIN"][key]
		if ok1 && ok2 && hi >= lo && hi-lo < 60 {
			a.rng += (hi - lo) * 5 / 9
			a.nRng++
		}
		if in, ok := series["PRCP"][key]; ok && in >= 0 && in < 20 {
			a.addPrecip(in * 25.4)
		}
	}
	for m := range acc {
		if acc[m].nTemp < 60 || acc[m].nPrecip < 60 {
			return out, fmt.Errorf("SNOTEL %s: too few records for month %d", triplet, m+1)
		}
		out[m] = acc[m].month()
	}
	return out, nil
}

// MetresPerDegree is the ground length of a degree of latitude and of
// longitude at latitude lat.
func MetresPerDegree(lat float64) (latM, lonM float64) {
	p := lat * math.Pi / 180
	return 111132.92 - 559.82*math.Cos(2*p) + 1.175*math.Cos(4*p),
		111412.84*math.Cos(p) - 93.5*math.Cos(3*p)
}
