package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

const (
	CableHeight   = 18.0 // metres above terrain that cables run
	CableGap      = 1.5  // lateral half-gap between up and down cables (metres)
	TowerHeight   = 18.0 // height of lift tower poles — top of crossbar aligns with cable (metres)
	CrossbarHalf  = 2.5  // half-length of tower T crossbar (metres)
	ChairSpacingM = 30.0 // one chair per N metres of loop (approx)
)

// Chair holds one chair on the lift loop.
// Progress 0→1 is a full loop: 0=at base going up, 0.5=at top going down, 1.0=back at base.
type Chair struct {
	Progress   float32
	Passengers [2]*Agent
}

// ChairPos returns the world-space position and heading for a chair at the given
// progress value on the given lift. Used by both simulation and renderer.
//
// Base/Top are cell indices and resolve to cell-CENTER world positions,
// matching the convention used by the agent target-resolution code
// (resolveTarget, pickTopTarget). Cell-edge positioning here used to
// place the chair line 5 m offset from where the agent walked to queue,
// producing a "loop near the lift, never board" behaviour as the agent
// hovered around the queue point but the boarding handoff happened at
// the chair's actual position.
func (l *Lift) ChairPos(progress float32, t *Terrain) (mgl32.Vec3, float32) {
	const cellSize = float32(10.0)
	bx := (float32(l.Base[0]) + 0.5) * cellSize
	bz := (float32(l.Base[1]) + 0.5) * cellSize
	tx := (float32(l.Top[0]) + 0.5) * cellSize
	tz := (float32(l.Top[1]) + 0.5) * cellSize
	dx := tx - bx
	dz := tz - bz
	length := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if length < 1 {
		length = 1
	}
	dirX := dx / length
	dirZ := dz / length
	perpX := -dirZ
	perpZ := dirX

	var frac float32
	var perpSign float32
	var heading float32
	if progress < 0.5 {
		// Going up: base → top
		frac = progress * 2
		perpSign = 1
		heading = float32(math.Atan2(float64(dx), float64(dz)))
	} else {
		// Going down: top → base
		frac = (1.0 - progress) * 2
		perpSign = -1
		heading = float32(math.Atan2(float64(-dx), float64(-dz)))
	}

	cx := bx + dx*frac + perpX*CableGap*perpSign
	cz := bz + dz*frac + perpZ*CableGap*perpSign
	cy := t.InterpolatedElevationAt(cx, cz) + CableHeight

	return mgl32.Vec3{cx, cy, cz}, heading
}

// Lift represents a ski lift connecting a base to a top station.
type Lift struct {
	ID    uint64
	Base  [2]int
	Top   [2]int
	Speed float32 // cable speed in m/s (typical real lift: 2–3 m/s)

	Queue  []*Agent
	Chairs []Chair
}

// LoopLength returns the total length of the chair loop in metres (2× cable length).
func (l *Lift) LoopLength() float32 {
	const cellSize = float32(10.0)
	dx := float32(l.Top[0]-l.Base[0]) * cellSize
	dz := float32(l.Top[1]-l.Base[1]) * cellSize
	return 2 * float32(math.Sqrt(float64(dx*dx+dz*dz)))
}

// PassengerCount returns the total number of skiers currently on chairs.
func (l *Lift) PassengerCount() int {
	n := 0
	for _, c := range l.Chairs {
		for _, p := range c.Passengers {
			if p != nil {
				n++
			}
		}
	}
	return n
}
