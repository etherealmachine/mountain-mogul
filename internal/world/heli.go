//go:build !demo

package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// The heli-ski helicopter: a lift with no cable that flies its riders
// from Base to Top. Not in the demo (heli_demo.go).

// HeliPhase is the state of a heli-ski helicopter in its flight cycle.
type HeliPhase uint8

const (
	HeliAtBase HeliPhase = iota // parked at helipad, loading passengers
	HeliToTop                   // flying to the drop zone
	HeliAtTop                   // at drop zone, unloading
	HeliToBase                  // flying back empty
)

// HeliData holds the dynamic state of a heli-ski helicopter. Non-nil only
// on Lift records with Type == LiftHeli; all other lift types leave this nil.
type HeliData struct {
	Phase      HeliPhase
	Progress   float32  // 0→1 within the current HeliToTop or HeliToBase leg
	Passengers []*Guest // up to HeliCapacity; nil slots are empty seats
}

// liftExtras is a Lift's flight state. HeliState is non-nil only for
// LiftHeli lifts; callers should use Cableless() to branch rather than
// testing the pointer directly.
type liftExtras struct {
	HeliState *HeliData
}

// initHeli sets a new heli lift on its pad at Base.
func initHeli(l *Lift) {
	l.HeliState = &HeliData{
		Phase:      HeliAtBase,
		Passengers: make([]*Guest, 0, HeliCapacity),
	}
}

// HeliWorldPos returns the current world-space position of l's
// helicopter. During transit legs, the helicopter follows the
// straight line between Base and Top at HeliFlightAlt above the terrain,
// rising and falling on a sine arc. At base/top the helicopter sits on
// the helipad surface.
func HeliWorldPos(l *Lift, t *Terrain) mgl32.Vec3 {
	if l.HeliState == nil {
		y := t.InterpolatedSurfaceElevationAt(l.Base[0], l.Base[1])
		return mgl32.Vec3{l.Base[0], y, l.Base[1]}
	}
	h := l.HeliState
	var wx, wz, frac float32
	switch h.Phase {
	case HeliAtBase:
		wx, wz = l.Base[0], l.Base[1]
		y := t.InterpolatedSurfaceElevationAt(wx, wz)
		return mgl32.Vec3{wx, y, wz}
	case HeliAtTop:
		wx, wz = l.Top[0], l.Top[1]
		y := t.InterpolatedSurfaceElevationAt(wx, wz)
		return mgl32.Vec3{wx, y, wz}
	case HeliToTop:
		frac = h.Progress
		wx = l.Base[0] + (l.Top[0]-l.Base[0])*frac
		wz = l.Base[1] + (l.Top[1]-l.Base[1])*frac
	case HeliToBase:
		frac = h.Progress
		wx = l.Top[0] + (l.Base[0]-l.Top[0])*frac
		wz = l.Top[1] + (l.Base[1]-l.Top[1])*frac
	}
	groundY := t.InterpolatedSurfaceElevationAt(wx, wz)
	alt := float32(math.Sin(float64(frac)*math.Pi)) * HeliFlightAlt
	return mgl32.Vec3{wx, groundY + alt, wz}
}

// HeliHeading returns l's Y-axis heading (radians) for the helicopter mesh
// so it always faces its current destination.
func HeliHeading(l *Lift) float32 {
	if l.HeliState == nil {
		return 0
	}
	var dx, dz float32
	switch l.HeliState.Phase {
	case HeliAtBase, HeliToTop:
		dx = l.Top[0] - l.Base[0]
		dz = l.Top[1] - l.Base[1]
	case HeliAtTop, HeliToBase:
		dx = l.Base[0] - l.Top[0]
		dz = l.Base[1] - l.Top[1]
	}
	return float32(math.Atan2(float64(dx), float64(dz)))
}
