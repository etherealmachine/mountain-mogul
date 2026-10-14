package world

import (
	"math"

	"mountain-mogul/internal/edition"

	"github.com/go-gl/mathgl/mgl32"
)

// The aerial tramway (models-src/tram_cabin.py, tram_terminal.py,
// tram_tower.py): two big cabins on their own tracks, TramGap either side
// of the line, shuttling between the terminals. One goes up as the other
// comes down; they pass in the middle and dock at the same time, wait
// TramDwell, and swap. Riders board from the platform between the
// tracks, the whole line at once if they fit, when a cabin leaves the
// base.
//
// It runs on the chair loop like any cable lift: two places half a loop
// apart, and the loop is a round trip, up, a dwell docked at the top,
// down, and a dwell docked at the base, so progress 0, the load point,
// is a cabin leaving the base and UnloadProgress one arriving at the top.
// Only where a cabin is differs (tramAt): it never turns, and each keeps
// to its own track.

const (
	TramCapacity = 100 // riders a cabin carries, standing (Jackson Hole's 100, Palisades' 110)
	// TramGap is how far each cabin's track runs off the line: two track
	// ropes, TramRopeSpread either side of it, its carriage's wheels on
	// both.
	TramGap        = 3.2
	TramRopeSpread = 0.4
	// TramDockHeight is the track's height over a terminal's platform,
	// where the cabin docks with its floor just over it.
	TramDockHeight = 7.0
	// TramPlatform is how far a terminal's platform stands over the
	// ground.
	TramPlatform = 3.0
	// TramLineHeight is the track's height over the ground between the
	// terminals (TowerHeight a chairlift's).
	TramLineHeight = 32.0
	// TramRamp is the metres beyond a terminal over which the track
	// climbs to TramLineHeight (StationOffset a chairlift's).
	TramRamp = 70.0
	// TramTowerSpacing is the metres between a tram's towers.
	TramTowerSpacing = 350.0
	// TramDwell is the seconds a cabin waits docked at each end.
	TramDwell = 45.0
	// TramMinLength is the shortest tram, metres between its ends.
	TramMinLength = 300.0
)

// Tram reports whether lifts of this type are an aerial tramway. Always
// false in the demo, which has none.
func (t LiftType) Tram() bool { return !edition.Demo && t == LiftTram }

// towerSpacing is the target metres between adjacent towers.
func (t LiftType) towerSpacing() float32 {
	switch {
	case t.Tram(), t == LiftTri:
		return TramTowerSpacing
	case t == LiftFunitel:
		return FunitelTowerSpacing
	}
	return TowerSpacing
}

// lineHeight is the cable's height over the ground between the ramps.
func (t LiftType) lineHeight() float32 {
	switch {
	case t.Tram(), t == LiftTri:
		return TramLineHeight
	case t == LiftFunitel:
		return FunitelLineHeight
	}
	return TowerHeight
}

// Ramp is the metres past a terminal's flat run over which the cable
// climbs to its line height, where the first tower stands.
func (l *Lift) Ramp() float32 {
	switch {
	case l.Type.Tram(), l.Type == LiftTri:
		return TramRamp
	case l.Type == LiftFunitel:
		return FunitelRamp
	}
	return StationOffset
}

// CableOffset is how far each cable (a tram's track) runs off the line,
// the radius of its bullwheels.
func (l *Lift) CableOffset() float32 {
	switch {
	case l.Type.Tram(), l.Type == LiftTri:
		return TramGap
	case l.Type == LiftFunitel:
		return FunitelGap
	case l.Type == LiftHS8Pack:
		return WideCableGap
	}
	return CableGap
}

// RopeOffsets is the ropes each way's cable is, across from it: one, a
// tram's or a 3S's two track ropes (a 3S's haul rope between them), a
// funitel's two haul ropes.
func (l *Lift) RopeOffsets() []float32 {
	switch {
	case l.Type.Tram():
		return []float32{-TramRopeSpread, TramRopeSpread}
	case l.Type == LiftTri:
		return []float32{-TramRopeSpread, 0, TramRopeSpread}
	case l.Type == LiftFunitel:
		return []float32{-FunitelRopeSpread, FunitelRopeSpread}
	}
	return []float32{0}
}

// riderGap is how far off the line riders get on and off: under the
// riders' cable, or a tram's platform, on the line between its tracks.
func (l *Lift) riderGap() float32 {
	if l.Type.Tram() {
		return 0
	}
	return l.CableOffset()
}

// tramSide is which track cabin i runs on: +1 the riders' side
// (ChairPos's +perp), -1 the other.
func tramSide(i int) float32 {
	if i%2 == 0 {
		return 1
	}
	return -1
}

// tramTrips is a tram's run between the docks and its dwell at each, as
// loop metres: the dwell is the distance the cabin would go in TramDwell.
func (l *Lift) tramTrips() (travel, dwell float32) {
	travel = max(l.cableLen()-2*l.loadAhead(), 1)
	dwell = max(l.Speed, 0.1) * TramDwell
	return travel, dwell
}

// tramLoopLength is a tram's round trip: up, docked, down, docked.
func (l *Lift) tramLoopLength() float32 {
	travel, dwell := l.tramTrips()
	return 2 * (travel + dwell)
}

// tramAt is where a tram cabin at progress p on track side is: its
// position on the ground, its heading (always up the line: a cabin never
// turns), and how far it is from the nearer end along the line.
func (l *Lift) tramAt(p, side float32) (pos mgl32.Vec2, heading, d float32) {
	dir, perp := l.lineAxes()
	length := l.cableLen()
	la := l.loadAhead()
	travel, dwell := l.tramTrips()
	loop := 2 * (travel + dwell)
	s := float32(math.Mod(float64(p*loop), float64(loop)))
	if s < 0 {
		s += loop
	}
	var x float32 // metres from the base end
	switch {
	case s < travel: // up
		x = la + s
	case s < travel+dwell: // docked at the top
		x = length - la
	case s < 2*travel+dwell: // down
		x = length - la - (s - travel - dwell)
	default: // docked at the base
		x = la
	}
	pos = l.Base.Add(dir.Mul(x)).Add(perp.Mul(side * TramGap))
	return pos, float32(math.Atan2(float64(dir[0]), float64(dir[1]))), min(x, length-x)
}

// tramStation is a tram's station house (tram_terminal.py): an enclosed
// ground floor with the way in at the back, and over it the platform
// TramPlatform up, both docks either side of the boarding platform on
// the line, under a hall roof on six columns with the drive house hung
// in its back end. The cabins dock LoadAhead in front of the anchor,
// their tracks level through the hall to its front. The operator works
// inside.
var tramStation = StationGeom{
	LoadAhead:  7,
	Flat:       14,
	ApronFront: 18,
	CableH:     TramDockHeight + TramPlatform,
	Enclosed:   true,
	DoorHalf:   2.2,
	operator:   [2]float32{-3.5, -5},
	rects:      []stationRect{{front: -14.2, rear: 5.2, in: -6.6, out: 6.6}},
	points:     wallPoints(-14.2, 5.2, 6.6, 2.2),
}
