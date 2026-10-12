package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// A station's footprint: the ground under its structure, where skiers
// don't go (models-src/lift_station.scad). StationXZs is only the points
// that stand on the ground; the footprint is everything overhead too.
// Metres from the bullwheel in the station's frame: back along the line,
// away from the other end, and across it toward the hut.
const (
	// The hood over the wheel (2 m round it), the boom under it and the
	// drive housing behind the crossbeam.
	stationHoodFront  = -2.0
	stationHoodBack   = 3.7
	stationHoodAcross = 2.0
	// The portal frame: the legs on their footings and the crossbeam
	// between them.
	stationFrameFront  = 1.65
	stationFrameBack   = 2.75
	stationFrameAcross = 4.15
	// The operator's hut.
	stationHutFront = stationHutBack - 1.2
	stationHutRear  = stationHutBack + 1.2
	stationHutIn    = stationHutSide - 0.95
	stationHutOut   = stationHutSide + 0.95
)

// StationExtent is the furthest any part of a station's footprint is
// from its bullwheel (the hut's far corner), rounded up.
const StationExtent = 8

// StationBox is one rectangle of a station's footprint on the ground.
type StationBox struct {
	Center mgl32.Vec2
	Axis   mgl32.Vec2 // unit, back along the line (away from the other end)
	// Half its size along Axis and across it.
	HalfBack, HalfAcross float32
	Top                  bool // at the top station
	Hut                  bool // the operator's hut, not the terminal over the line
}

// local is p in the box's frame, from its centre: along Axis, and across.
func (b StationBox) local(p mgl32.Vec2) (along, across float32) {
	d := p.Sub(b.Center)
	return d.Dot(b.Axis), d[0]*b.Axis[1] - d[1]*b.Axis[0]
}

// Dist is how far p is outside the box: 0 or less inside it (minus how
// far in from the nearest edge).
func (b StationBox) Dist(p mgl32.Vec2) float32 {
	along, across := b.local(p)
	ox := abs32(along) - b.HalfBack
	oz := abs32(across) - b.HalfAcross
	if ox <= 0 && oz <= 0 {
		return max(ox, oz)
	}
	return float32(math.Hypot(float64(max(ox, 0)), float64(max(oz, 0))))
}

// Contains reports whether p is within margin of the box (inside it when
// margin is 0).
func (b StationBox) Contains(p mgl32.Vec2, margin float32) bool {
	return b.Dist(p) <= margin
}

// Edge is points every spacing metres or less round the box's edge,
// pushed out by out, appended to buf.
func (b StationBox) Edge(spacing, out float32, buf []mgl32.Vec2) []mgl32.Vec2 {
	hb, ha := b.HalfBack+out, b.HalfAcross+out
	across := mgl32.Vec2{b.Axis[1], -b.Axis[0]}
	corner := func(sb, sa float32) mgl32.Vec2 {
		return b.Center.Add(b.Axis.Mul(sb * hb)).Add(across.Mul(sa * ha))
	}
	cs := [4]mgl32.Vec2{corner(-1, -1), corner(1, -1), corner(1, 1), corner(-1, 1)}
	for i, c := range cs {
		next := cs[(i+1)%4]
		n := max(int(math.Ceil(float64(next.Sub(c).Len()/spacing))), 1)
		for k := 0; k < n; k++ {
			buf = append(buf, c.Add(next.Sub(c).Mul(float32(k)/float32(n))))
		}
	}
	return buf
}

// StationFrame is the frame of lift l's top (or base) station: the
// bullwheel, the unit direction back along the line away from the other
// end, and the one across it toward the hut (StationXZs). ok is false
// for a helicopter or a lift with both ends together.
func (l *Lift) StationFrame(top bool) (at, back, side mgl32.Vec2, ok bool) {
	if l.IsHeli() {
		return
	}
	at, other := l.Base, l.Top
	if top {
		at, other = l.Top, l.Base
	}
	u := other.Sub(at)
	if u.Len() < 1e-3 {
		return
	}
	u = u.Normalize()
	return at, u.Mul(-1), mgl32.Vec2{u[1], -u[0]}, true
}

// StationBoxes is lift l's stations' footprints: at each end the
// terminal over the line (hood, then frame) and the hut. Nil for a
// helicopter.
func (l *Lift) StationBoxes() []StationBox {
	return l.AppendStationBoxes(nil)
}

// AppendStationBoxes is StationBoxes appended to out, for a caller that
// keeps its own buffer.
func (l *Lift) AppendStationBoxes(out []StationBox) []StationBox {
	for _, top := range [2]bool{false, true} {
		at, back, side, ok := l.StationFrame(top)
		if !ok {
			continue
		}
		box := func(front, rear, in, outer float32, hut bool) StationBox {
			c := at.Add(back.Mul((front + rear) / 2)).Add(side.Mul((in + outer) / 2))
			return StationBox{Center: c, Axis: back, HalfBack: (rear - front) / 2, HalfAcross: (outer - in) / 2, Top: top, Hut: hut}
		}
		out = append(out,
			box(stationHoodFront, stationHoodBack, -stationHoodAcross, stationHoodAcross, false),
			box(stationFrameFront, stationFrameBack, -stationFrameAcross, stationFrameAcross, false),
			box(stationHutFront, stationHutRear, stationHutIn, stationHutOut, true),
		)
	}
	return out
}

// InStationFootprint reports whether p is within margin of any of lift
// l's station boxes.
func (l *Lift) InStationFootprint(p mgl32.Vec2, margin float32) bool {
	for _, b := range l.StationBoxes() {
		if b.Contains(p, margin) {
			return true
		}
	}
	return false
}

// Where a run attached to a lift's top starts: beside the station, clear
// of it and its hut, on the side the run heads off to, on the level apron
// riders come round onto after getting off sideways (sim:
// tickUnloading). The run takes them from there.
const (
	topRunEndBack   = 6.0 // level with the hut's far end
	topRunEndAcross = 8.5 // past the frame's legs and the hut
)

// TopRunEnd is where a run attached to l's top starts, on the side of
// the line toward next (the run's next node): the hut's side on a tie.
func (l *Lift) TopRunEnd(next mgl32.Vec2) mgl32.Vec2 {
	at, back, side, ok := l.StationFrame(true)
	if !ok {
		return l.Top
	}
	across := float32(topRunEndAcross)
	if next.Sub(at).Dot(side) < 0 {
		across = -across
	}
	return at.Add(back.Mul(topRunEndBack)).Add(side.Mul(across))
}
