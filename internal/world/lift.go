package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

const (
	TowerHeight     = 18.0 // height of lift tower poles — top of crossbar aligns with cable (metres)
	CrossbarHalf    = 2.5  // half-length of tower T crossbar (metres)
	CableGap        = 1.5  // lateral half-gap between up and down cables (metres)
	BullwheelHeight = 3.65 // cable height at the station bullwheel — derived from lift_station.scad
	StationOffset   = 25.0 // distance from each station to the first/last tower; cable transitions over this span
	ChairSpacingM   = 30.0 // one chair per N metres of loop (approx)
)

// CableHeightAt returns the cable's height above terrain at a fractional
// position 0→1 along the cable line from base to top. Profile: rises
// linearly from BullwheelHeight at each station to TowerHeight over the
// StationOffset transition span, then stays flat at TowerHeight across
// the inner span.
//
// For lifts shorter than 2× StationOffset there's no inner span; the
// cable stays flat at BullwheelHeight from end to end so chairs stay at
// boarding height the whole way.
func CableHeightAt(frac, length float32) float32 {
	if length <= 2*StationOffset {
		return BullwheelHeight
	}
	transition := float32(StationOffset) / length
	rise := float32(TowerHeight - BullwheelHeight)
	switch {
	case frac <= transition:
		return BullwheelHeight + (frac/transition)*rise
	case frac >= 1.0-transition:
		return BullwheelHeight + ((1.0-frac)/transition)*rise
	default:
		return TowerHeight
	}
}

// Chair holds one chair on the lift loop.
// Progress 0→1 is a full loop: 0=at base going up, 0.5=at top going down, 1.0=back at base.
type Chair struct {
	Progress   float32
	Passengers [2]*Agent
}

// ChairPos returns the world-space position and heading for a chair at the given
// progress value on the given lift. Used by both simulation and renderer.
//
// Base/Top are continuous world XZ positions; this is plain Vec2 math
// with no cell-center fudge.
func (l *Lift) ChairPos(progress float32, t *Terrain) (mgl32.Vec3, float32) {
	bx, bz := l.Base[0], l.Base[1]
	tx, tz := l.Top[0], l.Top[1]
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
	cy := t.InterpolatedSurfaceElevationAt(cx, cz) + CableHeightAt(frac, length)

	return mgl32.Vec3{cx, cy, cz}, heading
}

// Lift represents a ski lift connecting a base to a top station.
//
// Base and Top are continuous world XZ positions (metres). Y is derived
// from terrain elevation at use time.
type Lift struct {
	ID    uint64
	Base  mgl32.Vec2
	Top   mgl32.Vec2
	Speed float32 // cable speed in m/s (typical real lift: 2–3 m/s)

	// TicketPrice is the per-ride fare credited to World.Cash when a
	// skier boards a chair. Set per-lift via the lift popup.
	TicketPrice int

	Queue  []*Agent
	Chairs []Chair
}

// LoopLength returns the total length of the chair loop in metres (2× cable length).
func (l *Lift) LoopLength() float32 {
	dx := l.Top[0] - l.Base[0]
	dz := l.Top[1] - l.Base[1]
	return 2 * float32(math.Sqrt(float64(dx*dx+dz*dz)))
}

// QueueCell returns the grid cell containing the lift's base — the
// pathfinder destination for skiers walking to queue. Same convention as
// Building.DoorCell.
func (l *Lift) QueueCell() [2]int {
	return cellOf(l.Base)
}

// QueueSpacing is the metres between adjacent skiers in a single-file
// lift queue. ~2 m gives room for skis (length ~1.8 m) and a small
// breathing gap; tight enough that a 20-deep queue is only ~40 m long.
const QueueSpacing = 2.0

// queueDir returns the unit vector pointing from the lift base away
// from the cable axis — the direction the single-file queue extends.
// For a lift going up the hill, the queue trails downhill of the base.
func (l *Lift) queueDir() (dirX, dirZ float32) {
	dx := l.Base[0] - l.Top[0]
	dz := l.Base[1] - l.Top[1]
	length := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if length < 1e-3 {
		// Degenerate: lift base and top coincident. Fall back to +Z so
		// the queue still extends in some direction rather than
		// collapsing onto the base.
		return 0, 1
	}
	return dx / length, dz / length
}

// QueueSlotXZ returns the world-XZ position of the index-th queue slot.
// Slot 0 is the boarding spot at the base; slot N+1 sits QueueSpacing
// further along queueDir. Cell-only callers (pathfinder) don't need Y.
func (l *Lift) QueueSlotXZ(index int) (x, z float32) {
	dirX, dirZ := l.queueDir()
	return l.Base[0] + dirX*float32(index)*QueueSpacing,
		l.Base[1] + dirZ*float32(index)*QueueSpacing
}

// QueueSlotWorldPos returns the world-space position of the index-th
// slot in this lift's queue. Y comes from the snow surface so queued
// skiers stand on snow rather than floating.
func (l *Lift) QueueSlotWorldPos(index int, t *Terrain) mgl32.Vec3 {
	x, z := l.QueueSlotXZ(index)
	y := t.InterpolatedSurfaceElevationAt(x, z)
	return mgl32.Vec3{x, y, z}
}

// BackOfQueueWorldPos returns where the next arrival should head. Skiers
// in transit (skiing down or walking from a lodge) target this spot so
// they line up behind any existing queue instead of all converging on
// the base anchor. Pulled each tick by resolveTarget so the target
// shifts as the queue grows or boards.
func (l *Lift) BackOfQueueWorldPos(t *Terrain) mgl32.Vec3 {
	return l.QueueSlotWorldPos(len(l.Queue), t)
}

// BackOfQueueCell returns the grid cell of the back-of-queue slot — the
// pathfinder destination for skiers walking to queue. A snapshot at
// path-plan time; tickQueued handles any residual drift once they
// arrive and join the actual queue.
func (l *Lift) BackOfQueueCell() [2]int {
	x, z := l.QueueSlotXZ(len(l.Queue))
	return cellOf(mgl32.Vec2{x, z})
}

// TopCell returns the grid cell containing the lift's top station —
// used for elevation lookups at unload time and passability rasterisation.
func (l *Lift) TopCell() [2]int {
	return cellOf(l.Top)
}

func cellOf(p mgl32.Vec2) [2]int {
	const cellSize = float32(5.0)
	return [2]int{
		int(math.Floor(float64(p[0] / cellSize))),
		int(math.Floor(float64(p[1] / cellSize))),
	}
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
