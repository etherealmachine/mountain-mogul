package sim

import (
	"fmt"
	"math"

	"mountain-mogul/internal/world"
)

// Wind through the day (DayWeather.WindMS is the day's mean at the base):
// stronger in the afternoon, gusting hour to hour, and stronger up high
// and on exposed tops. Lifts hold past their limit (world/wind.go), and
// reopen after a test run.

// WindNow is the wind at the base right now, m/s, gusts included.
func (s *Simulation) WindNow() float32 {
	dw := s.Weather.Today()
	h := HourOfDay(s.SimTime)
	// The afternoon blows harder than the night: ±15% about 15:00.
	diurnal := 1 + 0.15*float32(math.Cos((h-15)/24*2*math.Pi))
	// Gusts: a smooth hour-to-hour wobble, 0.8 to 1.3.
	day := int(s.SimTime / secondsPerSimDay)
	g0, g1 := gustAt(day, int(h)), gustAt(day, int(h)+1)
	f := float32(h - math.Floor(h))
	return dw.WindMS * diurnal * (g0 + (g1-g0)*f)
}

// gustAt is the gust factor for hour h of day: 0.8 to 1.3, the same
// whenever it's asked.
func gustAt(day, h int) float32 {
	x := uint32(day*7919+h*104729) * 2654435761
	x ^= x >> 15
	x *= 2246822519
	x ^= x >> 13
	return 0.8 + 0.5*float32(x%1000)/1000
}

// windAtLift is the wind at lift l's top now: the base's, stronger with
// height (60% more a kilometre up) and on a top that stands above the
// ground around it (up to half as much again on a ridge).
func (s *Simulation) windAtLift(l *world.Lift) float32 {
	t := s.World.Terrain
	top := t.InterpolatedGroundElevationAt(l.Top[0], l.Top[1])
	height := 1 + 0.6*max(top-s.snowmakingBaseElev(), 0)/1000
	return s.WindNow() * height * s.liftExposure(l)
}

// liftExposure is how exposed lift l's top is: 1, and up to 1.5 for one
// standing 60 m or more above the ground 150 m around it. Worked out once
// a lift.
func (s *Simulation) liftExposure(l *world.Lift) float32 {
	if e, ok := s.exposure[l.ID]; ok {
		return e
	}
	t := s.World.Terrain
	top := t.InterpolatedGroundElevationAt(l.Top[0], l.Top[1])
	var sum float32
	const n, r = 16, 150.0
	for i := range n {
		a := float64(i) / n * 2 * math.Pi
		sum += t.InterpolatedGroundElevationAt(l.Top[0]+float32(r*math.Cos(a)), l.Top[1]+float32(r*math.Sin(a)))
	}
	e := 1 + 0.5*clamp32((top-sum/n)/60, 0, 1)
	if s.exposure == nil {
		s.exposure = map[uint64]float32{}
	}
	s.exposure[l.ID] = e
	return e
}

// tickLiftWind holds lift l when the wind at its top passes its limit,
// and ends the hold once the wind has stayed under WindResumeShare of it
// for WindCalmHours, with a test run.
func (s *Simulation) tickLiftWind(l *world.Lift) {
	limit := l.Type.WindLimit()
	l.Wind = s.windAtLift(l)
	if limit <= 0 {
		l.WindHold = false
		return
	}
	switch {
	case !l.WindHold && l.Wind > limit:
		l.WindHold, l.CalmSince, l.TestUntil = true, 0, 0
		if l.Open && s.LiftsRunning() {
			s.World.LogEventAt(world.EventLiftClosed, s.SimTime,
				fmt.Sprintf("%s on wind hold: %.0f m/s at the top", l.Name, l.Wind), l.Base, l.ID)
		}
	case l.WindHold && l.Wind < limit*world.WindResumeShare:
		if l.CalmSince == 0 {
			l.CalmSince = s.SimTime
		}
		if s.SimTime-l.CalmSince >= world.WindCalmHours*simSecondsPerHour {
			l.WindHold, l.CalmSince = false, 0
			l.TestUntil = s.SimTime + world.TestRunHours*simSecondsPerHour
			if l.Open && s.LiftsRunning() {
				s.World.LogEventAt(world.EventLiftOpened, s.SimTime,
					fmt.Sprintf("%s testing after the wind", l.Name), l.Base, l.ID)
			}
		}
	case l.WindHold:
		l.CalmSince = 0
	}
}
