package world

// The funitel and the tricable (3S) gondola, cabin lifts on more than one
// rope a way, and the 8-pack, whose cables are set wider apart. They run
// on the chair loop like any detachable, round bullwheels CableOffset in
// radius; RopeOffsets is the ropes each way's cable is drawn as. Both
// cabin lifts dock in station houses like the gondola's.

const (
	// WideCableGap is the 8-pack's cables off the line, so its 4.8 m
	// chairs pass each other clear.
	WideCableGap = 2.6

	// FunitelGap is how far each way of a funitel runs off the line, its
	// two haul ropes FunitelRopeSpread either side of that: Péclet's ways
	// are 7.2 m apart, its ropes about 3 m.
	FunitelGap        = 3.6
	FunitelRopeSpread = 1.6
	// FunitelLineHeight and FunitelRamp are a funitel's ropes' height
	// over the ground between its terminals, and the metres over which
	// they climb to it, where it has no laid-out profile
	// (lift_profile.go); its tower model is built for that height.
	FunitelLineHeight = 22.0
	FunitelRamp       = 35.0

	// TriDockHeight is a 3S's ropes' height over its terminal's
	// platform, where its cabins dock with their floors just over it.
	// Between the terminals it runs as a tram does (TramLineHeight,
	// TramRamp, the tram's towers).
	TriDockHeight = 5.2

	// funitelPlatform and triPlatform are their station houses' platforms
	// over the ground.
	funitelPlatform = 2.2
	triPlatform     = 2.5
)

// cabinHouse is a cabin lift's station house's geometry, from its cables'
// gap and how far its cabins reach from the axis, as
// station_kit.cabin_house_dims makes the model: the house from behind
// the wheel's rim to 12.6 m in front, its walls a metre and more past the
// columns that stand just outside the cabins, and its door round the
// riders' cable.
func cabinHouse(g StationGeom, gap, sweep, platform, cableH float32) StationGeom {
	half := sweep + 0.45 + 0.95
	rear := max(3.4, gap+1.9) + 0.1
	g.CableH = platform + cableH
	g.Enclosed, g.DoorHalf = true, gap+1.1
	g.operator = [2]float32{-2, -(half - 0.9)}
	g.rects = []stationRect{{front: -12.7, rear: rear, in: -(half + 0.1), out: half + 0.1}}
	g.points = wallPoints(-12.7, rear, half+0.1, g.DoorHalf)
	return g
}

var (
	funitelStation = cabinHouse(detachStation, FunitelGap, FunitelGap+FunitelRopeSpread+0.15, funitelPlatform, BullwheelHeight)
	triStation     = cabinHouse(detachStation, TramGap, TramGap+3.85/2+0.1, triPlatform, TriDockHeight)
)

// detach8Station is the 8-pack's terminal (lift_terminal_8pack.py):
// detachStation widened for its 4.8 m chairs on cables WideCableGap off
// the line.
var detach8Station = func() StationGeom {
	g := detachStation
	g.rects = []stationRect{
		{front: -12.1, rear: 3.8, in: -5.75, out: 5.75},
		{front: -14.4, rear: -12.0, in: 5.9, out: 7.8, hut: true},
	}
	g.points = [][2]float32{
		{-11, 5.45}, {-11, -5.45}, {-2, 5.45}, {-2, -5.45},
		{-12.6, 6.85}, {-13.8, 6.85},
	}
	return g
}()
