package world

import (
	"math"

	"mountain-mogul/internal/edition"

	"github.com/go-gl/mathgl/mgl32"
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

	// A lift goes on hold when the snow at its base is under
	// LiftHoldDepth, and runs again from LiftResumeDepth.
	LiftHoldDepth   = float32(0.30)
	LiftResumeDepth = float32(0.35)
	// ThinSnowDepth is the depth under which rock shows through: skiers
	// route round it and hit rocks on it (sim/rocks.go).
	ThinSnowDepth = float32(0.20)
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

// SnowGunSpacing is the metres between the guns drawn along a run with
// snowmaking.
const SnowGunSpacing = 40

// SnowGun is one of the guns along a snowmaking run: where it stands, at
// the run's edge, and its heading (radians, as Guest.Heading) across the
// run.
type SnowGun struct {
	Pos     mgl32.Vec2
	Heading float32
}

// SnowGuns is where the guns stand along run t, SnowGunSpacing apart on
// its right-hand edge going down, each blowing across it; nil without
// snowmaking.
func (t *Trail) SnowGuns() []SnowGun {
	if !t.Snowmaking || t.Kind.IsArea() {
		return nil
	}
	line := t.Centerline()
	var out []SnowGun
	next := float32(SnowGunSpacing / 2)
	var along float32
	for i := 1; i < len(line); i++ {
		a, b := line[i-1], line[i]
		seg := b.Pos.Sub(a.Pos)
		l := seg.Len()
		if l < 1e-3 {
			continue
		}
		dir := seg.Mul(1 / l)
		right := mgl32.Vec2{-dir[1], dir[0]}
		for next <= along+l {
			f := (next - along) / l
			p := a.Pos.Add(seg.Mul(f))
			w := a.Width + (b.Width-a.Width)*f
			pos := p.Add(right.Mul(w/2 + 1))
			in := right.Mul(-1)
			out = append(out, SnowGun{Pos: pos, Heading: float32(math.Atan2(float64(in[0]), float64(in[1])))})
			next += SnowGunSpacing
		}
		along += l
	}
	return out
}
