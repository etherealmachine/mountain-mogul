package world

import (
	"math"

	"mountain-mogul/internal/edition"
)

// Snowmaking on runs: pipe and hydrants bought along a run, by the metre,
// after which the run makes machine snow over its whole area whenever the
// air at a cell is cold enough, up to the run's target depth, while its
// lifts are stopped. It costs only by the water it makes snow from, and
// the player switches it off for a run to save money. The sim side is
// sim/snowmaking.go.
const (
	SnowmakingCostPerM = 250 // dollars a metre of run for pipe and hydrants

	// SnowmakingMinTempC is the warmest air snow can be made in, and
	// SnowmakingFullTempC the air at which it's made at full rate.
	SnowmakingMinTempC  = float32(-2)
	SnowmakingFullTempC = float32(-10)
	// SnowmakingSWEPerHour is the water (metres) a cell gets an hour at
	// full rate: a cold twelve-hour night lays about 19 cm of machine
	// snow.
	SnowmakingSWEPerHour = float32(0.008)
	// SnowmakingWaterCost is dollars per cubic metre of water made into
	// snow (pumping and the guns' power).
	SnowmakingWaterCost = 2.0
	// DefaultSnowTarget is a run's target depth when snowmaking's put in.
	DefaultSnowTarget = float32(0.6)
	// SnowmakingStopBeforeOpen is the clock hours before opening the
	// guns stop, so the cats can groom over the last of it.
	SnowmakingStopBeforeOpen = 2.0
)

// SnowmakingAvailable reports whether the edition has snowmaking (the
// demo doesn't).
func SnowmakingAvailable() bool { return !edition.Demo }

// SnowmakingInstallCost is what pipe and hydrants cost along run t.
func SnowmakingInstallCost(t *Trail) int {
	return int(math.Round(float64(t.Length()))) * SnowmakingCostPerM
}

// SnowmakingRate is the share of full rate snow is made at in air tempC:
// nothing at SnowmakingMinTempC or warmer, all of it at
// SnowmakingFullTempC or colder.
func SnowmakingRate(tempC float32) float32 {
	return clamp01((SnowmakingMinTempC - tempC) / (SnowmakingMinTempC - SnowmakingFullTempC))
}

// AddMachineSnow lays swe metres of machine snow on c, which buries the
// corduroy. The surface becomes machine snow, taking in whatever was on
// top at its own density; bare base laid open by melt goes back under
// it.
func AddMachineSnow(c *Cell, swe float32) {
	machine := KindDensity(KindMachine)
	switch {
	case c.Top.Accumulation <= 0:
		c.Top = SnowLayer{Kind: KindMachine, Accumulation: swe, Density: machine}
	case c.Top.Kind == KindMachine:
		c.Top.AddSnow(swe, machine)
		return
	case c.Top.Kind == KindBase:
		c.FoldTopIntoBase()
		c.Top = SnowLayer{Kind: KindMachine, Accumulation: swe, Density: machine}
	default:
		c.Top.SetKind(KindMachine)
		c.Top.AddSnow(swe, machine)
	}
	c.Grooming = 0
	c.SkierTraffic = 0
}
