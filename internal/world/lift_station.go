package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// A lift's stations. Each end of a cable lift has a terminal round its
// bullwheel, the same shape at both ends: a fixed-grip lift's is a short
// canopy (models-src/lift_station.py), a detachable's a long terminal
// the chairs slow down in (models-src/lift_terminal.py). Riders get on
// under it, at the load point a little way in front of the wheel on the
// riders' cable, and off at the same place at the top, so the chair has
// come round the wheel behind them. The chair loop runs up one cable,
// round the top wheel, down the other and round the base wheel
// (Lift.ChairPos).
//
// Station frame: metres from the bullwheel back along the line, away
// from the other end (negative is in front, toward the other end), and
// across it toward the hut (StationFrame's side).

// Detachable reports whether the lift's chairs come off the cable in the
// stations and slow down there.
func (t LiftType) Detachable() bool {
	switch t {
	case LiftHSQuad, LiftHS6Pack, LiftHS8Pack, LiftGondola, LiftFunitel, LiftTri:
		return true
	}
	return false
}

// stationRect is one rectangle of a station's footprint in its frame.
type stationRect struct {
	front, rear float32 // back along the line: front edge, rear edge
	in, out     float32 // across toward the hut
	hut         bool    // the operator's hut, not the terminal over the line
}

// StationGeom is the shape of a lift's terminals, mirrored from the
// models.
type StationGeom struct {
	// LoadAhead is how far in front of the bullwheel riders get on (and
	// off at the top), metres along the line.
	LoadAhead float32
	// Flat is how far in front of the wheel the cable stays at terminal
	// height before it climbs to the towers over StationOffset.
	Flat float32
	// SlowZone is how far in front of the wheel a detachable's chairs
	// run at SlowSpeed; they speed up to the line's speed over slowRamp
	// metres past it. SlowSpeed 0 means they never slow.
	SlowZone, SlowSpeed float32
	// ApronFront is how far in front of the wheel the ground is levelled
	// (behind it, the apron's own depth).
	ApronFront float32
	// CableH is the cable's height over the ground through the terminal,
	// where riders get on; 0 for BullwheelHeight.
	CableH float32
	// Enclosed is a station house (the gondola's, the tram's): the cabins
	// dock on a platform up over an enclosed ground floor, which riders
	// come into and leave by a door in its back wall, DoorHalf either
	// side of the line. Its line is always a single file in through the
	// door, and riders getting off walk out of it (tickUnloading).
	Enclosed bool
	DoorHalf float32
	// operator is where the base's operator stands at a station with no
	// hut (back, across, as points): at the control post.
	operator [2]float32

	rects  []stationRect
	points [][2]float32 // ground points skiers steer round: back, across
}

// HalfWidth is how far the station reaches across the line either side.
func (g *StationGeom) HalfWidth() float32 {
	half := float32(0)
	for _, r := range g.rects {
		half = max(half, abs32(r.in), abs32(r.out))
	}
	return half
}

// Rear is how far the station reaches behind its bullwheel.
func (g *StationGeom) Rear() float32 {
	rear := float32(0)
	for _, r := range g.rects {
		rear = max(rear, r.rear)
	}
	return rear
}

// slowRamp is the metres over which a detachable's chairs speed back up
// to the line's speed as they leave a terminal.
const slowRamp = 6

// Fixed-grip station (lift_station.py): the bullwheel hung from a barrel
// vault about as wide as the chairs, from behind the wheel forward over
// the load point, on four posts just outside the chairs' sweep; the hut
// beside the empty chairs' side.
var fixedStation = StationGeom{
	LoadAhead:  3,
	Flat:       6,
	ApronFront: 6,
	rects: []stationRect{
		{front: -5.05, rear: 3.05, in: -3.4, out: 3.4},          // roof, posts and drive
		{front: 2.2, rear: 4.6, in: 4.05, out: 5.95, hut: true}, // operator's hut
	},
	points: [][2]float32{
		{2.2, 3.15}, {2.2, -3.15}, // back posts
		{-4.4, 3.15}, {-4.4, -3.15}, // front posts
		{2.8, 5.0}, {4.0, 5.0}, // hut
	},
}

// Detachable terminal (lift_terminal.py, quad-wide chairs and cabins): a
// long vault on four posts, the bullwheel up in its roof at the back, the
// chairs running flat at loading height round under it and through the
// terminal, slow, loading 4.5 m in front of the wheel; the hut stands off
// its front corner.
var detachStation = StationGeom{
	LoadAhead:  4.5,
	Flat:       13,
	SlowZone:   11,
	SlowSpeed:  1.0,
	ApronFront: 15,
	rects: []stationRect{
		{front: -12.1, rear: 2.7, in: -3.4, out: 3.4},             // roof over the terminal
		{front: -14.4, rear: -12.0, in: 3.6, out: 5.5, hut: true}, // operator's hut
	},
	points: [][2]float32{
		{-11, 3.15}, {-11, -3.15}, {-2, 3.15}, {-2, -3.15}, // posts
		{-12.6, 4.55}, {-13.8, 4.55}, // hut
	},
}

// detachWideStation is the 6-pack's terminal (lift_terminal_wide.py):
// detachStation widened for its 3.6 m chairs.
var detachWideStation = func() StationGeom {
	g := detachStation
	g.rects = []stationRect{
		{front: -12.1, rear: 2.7, in: -4.0, out: 4.0},
		{front: -14.4, rear: -12.0, in: 4.2, out: 6.1, hut: true},
	}
	g.points = [][2]float32{
		{-11, 3.75}, {-11, -3.75}, {-2, 3.75}, {-2, -3.75},
		{-12.6, 5.15}, {-13.8, 5.15},
	}
	return g
}()

// gondolaStation is the gondola's station house (gondola_terminal.py):
// detachStation's line up on a platform gondolaPlatform over an enclosed
// ground floor, under a hall roof on six columns; the operator inside.
var gondolaStation = cabinHouse(detachStation, CableGap, CableGap+1.2, gondolaPlatform, BullwheelHeight)

// gondolaPlatform is how far the gondola's platform stands over the
// ground: its cabins dock that much higher than a chair loads.
const gondolaPlatform = 2.2

// wallPoints is points every metre or so round the walls of a station
// house from front to rear (back along the line) and half across, leaving
// out its door, doorHalf either side of the line in the rear wall, as
// (back, across) points for StationGeom.points.
func wallPoints(front, rear, half, doorHalf float32) [][2]float32 {
	const step = 1.2
	var out [][2]float32
	for b := front; b <= rear+0.01; b += (rear - front) / float32(math.Ceil(float64((rear-front)/step))) {
		out = append(out, [2]float32{b, half}, [2]float32{b, -half})
	}
	n := float32(math.Ceil(float64(2 * half / step)))
	for k := float32(1); k < n; k++ {
		a := -half + 2*half*k/n
		out = append(out, [2]float32{front, a})
		if abs32(a) > doorHalf {
			out = append(out, [2]float32{rear, a})
		}
	}
	return out
}

// beltStation is a magic carpet's end (magic_carpet_end.py): the belt
// runs from a low end plate at the "bullwheel", riders' side only (the
// return run is under it), and riders step on just past the plate at the
// base and off over it at the top, so nothing stands across the line.
// No booth: the operator stands on the snow beside the belt, at the
// control post behind the end plate (carpet_kit.py).
var beltStation = StationGeom{
	LoadAhead:  0.8,
	Flat:       1,
	ApronFront: 3,
	operator:   [2]float32{0.6, -0.15},
}

// ropeStation is a rope tow's end (rope_tow_base.py, rope_tow_top.py): a
// bullwheel lying flat on a low frame, the rope round it RopeGap off the
// line each side, so riders, CableGap off, take hold and let go just
// past it. No hut: the base's operator stands at the control post,
// outside the riders by where they take hold (rope_tow_kit.py).
var ropeStation = StationGeom{
	LoadAhead:  2,
	Flat:       1,
	ApronFront: 3,
	operator:   [2]float32{-1.2, -2.6},
	points: [][2]float32{
		{0, 0}, {0.8, 0}, {0, 0.8}, {0, -0.8}, // the bullwheel's frame
	},
}

// tbarStation is a T-bar's end (tbar_base.py, tbar_top.py): the
// bullwheel lying flat TBarCableHeight up at the end of a boom off a mast
// behind it, so the ground in front, where riders take a bar and let it
// go, is open. No hut: an operator at each end stands at a control post
// outside the riders.
var tbarStation = StationGeom{
	LoadAhead:  4,
	Flat:       1,
	ApronFront: 6,
	operator:   [2]float32{-2.5, -3.2},
	points: [][2]float32{
		{2.2, 0}, {2.2, 0.6}, {2.2, -0.6}, // the mast
	},
}

// Station is the shape of l's terminals.
func (l *Lift) Station() *StationGeom { return stationFor(l.Type) }

func stationFor(t LiftType) *StationGeom {
	switch {
	case t.Belt():
		return &beltStation
	case t.Rope():
		return &ropeStation
	case t.Hangers():
		return &tbarStation
	case t == LiftHS6Pack:
		return &detachWideStation
	case t == LiftGondola:
		return &gondolaStation
	case t == LiftHS8Pack:
		return &detach8Station
	case t == LiftFunitel:
		return &funitelStation
	case t == LiftTri:
		return &triStation
	case t.Tram():
		return &tramStation
	case t.Detachable():
		return &detachStation
	}
	return &fixedStation
}

// StationExtent is the furthest any part of any station's footprint is
// from its bullwheel (the detachable's hut's far corner), rounded up.
const StationExtent = 17

// MinLiftLength is the shortest cable lift of type t, metres between the
// bullwheels: room for both terminals and a little cable between. 0 for
// a heli.
func MinLiftLength(t LiftType) float32 {
	if t == LiftHeli {
		return 0
	}
	if t.Tram() {
		return TramMinLength
	}
	return 2*stationFor(t).Flat + 10
}

// loadAhead is the station's LoadAhead, held short of the middle on a
// lift too short for it.
func (l *Lift) loadAhead() float32 {
	return max(0, min(l.Station().LoadAhead, l.cableLen()/2-1))
}

// cableLen is the distance between the bullwheels.
func (l *Lift) cableLen() float32 { return l.Top.Sub(l.Base).Len() }

// lineAxes is the unit vector from base to top and the one across it on
// the riders' side (ChairPos's +perp).
func (l *Lift) lineAxes() (dir, perp mgl32.Vec2) {
	d := l.Top.Sub(l.Base)
	if n := d.Len(); n > 1e-3 {
		d = d.Mul(1 / n)
	} else {
		d = mgl32.Vec2{0, 1}
	}
	return d, mgl32.Vec2{-d[1], d[0]}
}

// LoadXZ is where riders get on at the base: under the station, in front
// of the bullwheel on the riders' cable. The line runs back from here.
func (l *Lift) LoadXZ() mgl32.Vec2 {
	dir, perp := l.lineAxes()
	return l.Base.Add(dir.Mul(l.loadAhead())).Add(perp.Mul(l.riderGap()))
}

// UnloadXZ is where riders get off at the top: under the top station, in
// front of its bullwheel on the riders' cable.
func (l *Lift) UnloadXZ() mgl32.Vec2 {
	dir, perp := l.lineAxes()
	return l.Top.Sub(dir.Mul(l.loadAhead())).Add(perp.Mul(l.riderGap()))
}

// CableHeightAt is the cable's height above the ground dist metres from
// the base bullwheel along the line: BullwheelHeight, loading height,
// through each terminal, then up to TowerHeight over StationOffset. A
// lift too short for towers stays at loading height.
func (l *Lift) CableHeightAt(dist float32) float32 {
	switch {
	case l.Type.Belt():
		return BeltHeight
	case l.Type.Rope():
		return 0 // riders on the snow; the rope is RopeHeight up
	case l.Type.Hangers():
		return TBarCableHeight
	}
	g := l.Station()
	length := l.cableLen()
	d := min(dist, length-dist)
	low := g.CableH
	if low == 0 {
		low = BullwheelHeight
	}
	ramp := l.Ramp()
	if d <= g.Flat || length <= 2*(g.Flat+ramp) {
		return low
	}
	rise := l.Type.lineHeight() - low
	return low + min((d-g.Flat)/ramp, 1)*rise
}

// LoopLength returns the length of the chair loop in metres: both cables
// and half way round each bullwheel.
func (l *Lift) LoopLength() float32 {
	if l.Type.Tram() {
		return l.tramLoopLength()
	}
	return 2*l.cableLen() + 2*math.Pi*l.CableOffset()
}

// loopAt finds progress p on the loop: the position on the ground, the
// heading of travel, and how far it is from the nearer bullwheel along
// the line (0 round a wheel). Progress 0 is the load point; the loop
// runs up the riders' cable, round the top wheel, down and round the
// base wheel.
func (l *Lift) loopAt(p float32) (pos mgl32.Vec2, heading, d float32) {
	if l.Type.Tram() {
		return l.tramAt(p, 1)
	}
	dir, perp := l.lineAxes()
	length := l.cableLen()
	loop := l.LoopLength()
	gap := l.CableOffset()
	arc := float32(math.Pi * gap)
	s := float32(math.Mod(float64(p*loop+l.loadAhead()), float64(loop)))
	if s < 0 {
		s += loop
	}
	headingOf := func(v mgl32.Vec2) float32 { return float32(math.Atan2(float64(v[0]), float64(v[1]))) }
	switch {
	case s < length: // up the riders' cable
		return l.Base.Add(dir.Mul(s)).Add(perp.Mul(gap)), headingOf(dir), min(s, length-s)
	case s < length+arc: // round the top wheel
		th := float64((s - length) / gap)
		c, sn := float32(math.Cos(th)), float32(math.Sin(th))
		pos = l.Top.Add(perp.Mul(gap * c)).Add(dir.Mul(gap * sn))
		return pos, headingOf(perp.Mul(-sn).Add(dir.Mul(c))), 0
	case s < 2*length+arc: // down the empty cable
		t := s - length - arc
		return l.Top.Sub(dir.Mul(t)).Sub(perp.Mul(gap)), headingOf(dir.Mul(-1)), min(t, length-t)
	default: // round the base wheel
		th := float64((s - 2*length - arc) / gap)
		c, sn := float32(math.Cos(th)), float32(math.Sin(th))
		pos = l.Base.Sub(perp.Mul(gap * c)).Sub(dir.Mul(gap * sn))
		return pos, headingOf(perp.Mul(sn).Sub(dir.Mul(c))), 0
	}
}

// ChairPos returns the world position (on the cable) and heading of a
// chair at progress on the loop. Used by both simulation and renderer.
func (l *Lift) ChairPos(progress float32, t *Terrain) (mgl32.Vec3, float32) {
	pos, heading, d := l.loopAt(progress)
	h := l.CableHeightAt(d) // d is from the nearer wheel (0 round one); the profile is the same at both ends
	return mgl32.Vec3{pos[0], t.InterpolatedSurfaceElevationAt(pos[0], pos[1]) + h, pos[1]}, heading
}

// PlacePos is the world position (on the cable) and heading of chair i:
// ChairPos at its progress, but a tram's cabins each keep to their own
// track (tram.go).
func (l *Lift) PlacePos(i int, t *Terrain) (mgl32.Vec3, float32) {
	if !l.Type.Tram() {
		return l.ChairPos(l.Chairs[i].Progress, t)
	}
	pos, heading, d := l.tramAt(l.Chairs[i].Progress, tramSide(i))
	h := l.CableHeightAt(d)
	return mgl32.Vec3{pos[0], t.InterpolatedSurfaceElevationAt(pos[0], pos[1]) + h, pos[1]}, heading
}

// UnloadPos is where chair i stands at the top's unload point, and its
// heading.
func (l *Lift) UnloadPos(i int, t *Terrain) (mgl32.Vec3, float32) {
	if !l.Type.Tram() {
		return l.ChairPos(l.UnloadProgress(), t)
	}
	pos, heading, d := l.tramAt(l.UnloadProgress(), tramSide(i))
	h := l.CableHeightAt(d)
	return mgl32.Vec3{pos[0], t.InterpolatedSurfaceElevationAt(pos[0], pos[1]) + h, pos[1]}, heading
}

// InTerminal reports whether a chair at progress is inside a
// detachable's terminal, off the cable (where a bubble cover is up).
func (l *Lift) InTerminal(progress float32) bool {
	g := l.Station()
	if g.SlowSpeed <= 0 {
		return false
	}
	_, _, d := l.loopAt(progress)
	return d <= g.SlowZone
}

// UnloadProgress is the loop progress at the top's unload point.
func (l *Lift) UnloadProgress() float32 {
	return (l.cableLen() - 2*l.loadAhead()) / l.LoopLength()
}

// SpeedAt is how fast a chair at progress moves, m/s: the line's speed,
// slower inside a detachable's terminals.
func (l *Lift) SpeedAt(progress float32) float32 {
	g := l.Station()
	if g.SlowSpeed <= 0 {
		return l.Speed
	}
	_, _, d := l.loopAt(progress)
	slow := min(g.SlowSpeed, l.Speed)
	k := clamp01((d - g.SlowZone) / slowRamp)
	return slow + (l.Speed-slow)*k
}

// loopTimes is the time a chair takes round the whole loop and from the
// load point to the unload point, seconds, at the lift's speed; cached
// until the speed or ends change.
func (l *Lift) loopTimes() (loop, ride float32) {
	key := [5]float32{l.Speed, l.Base[0], l.Base[1], l.Top[0], l.Top[1]}
	if l.timesKey == key {
		return l.loopTime, l.rideTime
	}
	if l.Speed <= 0 {
		return 0, 0
	}
	n := max(int(l.LoopLength()/0.5), 1)
	step := 1 / float32(n)
	stepLen := l.LoopLength() / float32(n)
	up := l.UnloadProgress()
	for i := range n {
		p := (float32(i) + 0.5) * step
		dt := stepLen / max(l.SpeedAt(p), 0.01)
		loop += dt
		if p < up {
			ride += dt
		}
	}
	l.timesKey, l.loopTime, l.rideTime = key, loop, ride
	return loop, ride
}

// LoopTime is how long a chair takes round the whole loop, seconds.
func (l *Lift) LoopTime() float32 {
	loop, _ := l.loopTimes()
	return loop
}

// RideTime is how long the ride takes from the load point to the unload
// point, seconds.
func (l *Lift) RideTime() float32 {
	_, ride := l.loopTimes()
	return ride
}

// StationFrame is the frame of lift l's top (or base) station: the
// bullwheel, the unit direction back along the line away from the other
// end, and the one across it toward the hut. ok is false for a
// helicopter or a lift with both ends together.
func (l *Lift) StationFrame(top bool) (at, back, side mgl32.Vec2, ok bool) {
	if l.Cableless() {
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

// StationXZs is where skiers have to steer round the lift's stations, at
// both ends: the legs and posts, and the operator's hut. Nil for a
// helicopter.
func (l *Lift) StationXZs() []mgl32.Vec2 {
	if l.stationCache != nil || l.Cableless() {
		return l.stationCache
	}
	var out []mgl32.Vec2
	g := l.Station()
	for _, top := range [2]bool{false, true} {
		at, back, side, ok := l.StationFrame(top)
		if !ok {
			continue
		}
		for _, p := range g.points {
			out = append(out, at.Add(back.Mul(p[0])).Add(side.Mul(p[1])))
		}
	}
	l.stationCache = out
	return out
}

func clamp01(x float32) float32 { return max(0, min(x, 1)) }
