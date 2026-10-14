package sim

import (
	"mountain-mogul/internal/world"
)

// Night skiing (world/lights.go): whether it's too dark to ski unlit, and
// the lights' running cost. After dusk only lit lifts load (tickLifts,
// world.Lift.InService) and guests choose lit runs; once no lit lift is
// running the mountain is closed for the day (ClosedForDay).

// tickDark sets World.Dark and DuskHour for now, and charges the lights
// from sunset to closing.
func (s *Simulation) tickDark(dt float64) {
	w := s.World
	day := int(s.SimTime / secondsPerSimDay)
	if day != s.sunDay || s.sunSet == 0 {
		rise, set := s.Site.SunriseSunset(s.DateAt(s.SimTime))
		s.sunDay, s.sunRise, s.sunSet = day, float32(rise), float32(set)
		w.DuskHour = s.sunSet + world.DuskAfterSunset
	}
	h := float32(HourOfDay(s.SimTime))
	w.Dark = h >= w.DuskHour || h < s.sunRise-world.DawnBeforeSunrise
	if h >= s.sunSet && s.LiftsRunning() {
		if m := w.LitLength(); m > 0 {
			w.LightingToday += float64(m) * world.LightsCostPerMHour * dt / simSecondsPerHour
		}
	}
}

// litLiftRunning reports whether a lit lift is in service: the night
// session's going.
func litLiftRunning(w *world.World) bool {
	for _, l := range w.Lifts {
		if l.Lit && l.InService(w) {
			return true
		}
	}
	return false
}
