package world

// Climate is a scenario's typical weather, month by month, from station
// or reanalysis records for the real place. The sim's weather generator
// draws each day from it; nil falls back to a generic cold mountain.
type Climate struct {
	Source      string  // where the numbers came from, for the editor
	RefAltitude float32 // metres above sea level the temperatures are for
	// WindDeg is the prevailing storm wind: the direction it blows
	// towards, degrees clockwise from north, like the auto-snow wind.
	WindDeg float32
	Months  [12]ClimateMonth // January first
}

// ClimateMonth is one calendar month of a Climate.
type ClimateMonth struct {
	TempMean  float32 // mean daily temperature, °C
	TempRange float32 // mean daily high minus low, °C
	WetDays   float32 // fraction of days with at least WetDayMM of precipitation
	WetMM     float32 // mean precipitation on those days, mm of water
	Cloud     float32 // mean cloud cover, 0..1
}

// WetDayMM is the precipitation that makes a day count as wet: 0.1 in,
// the resolution of the US snow-station gauges.
const WetDayMM = 2.54

// LapseRate is the drop in air temperature per metre of height, °C/m.
const LapseRate = float32(0.0065)

// TempAt is the month's mean temperature lapsed to altitude (metres
// above sea level).
func (c *Climate) TempAt(month int, altitude float32) float32 {
	return c.Months[month].TempMean - LapseRate*(altitude-c.RefAltitude)
}

// SnowlineAltitude is roughly where winter snow stops lying: the height
// at which December to March averages snowlineTempC.
func (c *Climate) SnowlineAltitude() float32 {
	const snowlineTempC = 1
	var t float32
	for _, m := range []int{11, 0, 1, 2} {
		t += c.Months[m].TempMean / 4
	}
	return c.RefAltitude + (t-snowlineTempC)/LapseRate
}
