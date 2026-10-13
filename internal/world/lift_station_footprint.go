package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// A station's footprint: the ground under its structure, where skiers
// don't go (the models; StationGeom). StationXZs is only the points that
// stand on the ground; the footprint is everything overhead too.

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

// StationBoxes is lift l's stations' footprints: at each end the
// terminal over the line and the hut. Nil for a
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
		for _, r := range l.Station().rects {
			c := at.Add(back.Mul((r.front + r.rear) / 2)).Add(side.Mul((r.in + r.out) / 2))
			out = append(out, StationBox{Center: c, Axis: back, HalfBack: (r.rear - r.front) / 2,
				HalfAcross: (r.out - r.in) / 2, Top: top, Hut: r.hut})
		}
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
	topRunEndBack   = 6.0 // behind either terminal, level with the fixed-grip hut's far end
	topRunEndAcross = 8.5 // past the legs, posts and huts
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
