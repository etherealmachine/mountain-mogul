package world

import "mountain-mogul/internal/edition"

// Night skiing (notes/next/Night Sessions.md): lights bought for a run or
// a lift, by the metre, and run by the hour. After dusk only lit lifts
// load and guests keep to lit runs where they can; the lifts' closing
// time is the night's close. Lights are on from sunset to closing.
const (
	RunLightsCostPerM  = 120 // dollars a metre of run for poles and lamps
	LiftLightsCostPerM = 60  // dollars a metre of lift line, stations included
	// LightsCostPerMHour is dollars a lit metre an hour.
	LightsCostPerMHour = 0.15
	// DuskAfterSunset is the clock hours after sunset it's too dark to
	// ski unlit (and DawnBeforeSunrise before sunrise).
	DuskAfterSunset   = float32(1.0 / 3)
	DawnBeforeSunrise = float32(1.0 / 3)
	// DayHours is the open hours a lift's and its staff's daily rates
	// are for (9 to 4); they're charged by the hour it's open.
	DayHours = float32(7)
)

// NightSkiingAvailable reports whether the edition sells lights (the
// demo doesn't; its lifts still stop at dusk).
func NightSkiingAvailable() bool { return !edition.Demo }

// RunLightsCost is what lighting run t costs.
func RunLightsCost(t *Trail) int { return int(t.Length()+0.5) * RunLightsCostPerM }

// LiftLightsCost is what lighting lift l costs.
func LiftLightsCost(l *Lift) int { return int(l.cableLen()+0.5) * LiftLightsCostPerM }

// InService reports whether guests can use lift l now: open, not held for
// snow, and lit if it's dark.
func (l *Lift) InService(w *World) bool {
	return l.Open && !l.OnHold && (!w.Dark || l.Lit)
}

// TrailInService reports whether guests choose run t now: any time by
// day, only lit after dark (they can still ski an unlit one down).
func (w *World) TrailInService(t *Trail) bool { return !w.Dark || t.Lit }

// LitLength is the metres of lit runs and lift lines, for the lights'
// running cost.
func (w *World) LitLength() float32 {
	var m float32
	for _, t := range w.Trails {
		if t.Lit {
			m += t.Length()
		}
	}
	for _, l := range w.Lifts {
		if l.Lit {
			m += l.cableLen()
		}
	}
	return m
}

// liftOpenHours is how many clock hours lift l runs on an open day: from
// opening to closing, or to dusk for an unlit one.
func (w *World) liftOpenHours(l *Lift) float32 {
	end := w.CloseHour
	if !l.Lit && w.DuskHour > 0 {
		end = min(end, w.DuskHour)
	}
	return max(end-w.OpenHour, 0)
}
