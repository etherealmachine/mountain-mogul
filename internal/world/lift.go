package world

import (
	"math"

	"mountain-mogul/internal/edition"

	"github.com/go-gl/mathgl/mgl32"
)

const (
	TowerHeight     = 18.0 // height of lift tower poles — top of crossbar aligns with cable (metres)
	CrossbarHalf    = 2.5  // half-length of tower T crossbar (metres)
	CableGap        = 1.5  // lateral half-gap between up and down cables (metres)
	BullwheelHeight = 3.65 // cable height at the station bullwheel — derived from lift_station.py; chairs hang their footrests this far below the cable (lib/chair_kit.py)
	StationOffset   = 25.0 // distance from each station to the first/last tower; cable transitions over this span
	ChairSpacingM   = 30.0 // one chair per N metres of loop (approx)
)

// LiftType is the chair variant a lift carries. Capacity, chair mesh,
// and per-instance slot anchors all derive from this — the simulation
// loads N riders per chair where N = Type.Capacity(), and the renderer
// picks the matching chair OBJ via Type.MeshID().
type LiftType uint8

const (
	LiftDouble      LiftType = iota // 2-seat fixed grip (the original)
	LiftFixedQuad                   // 4-seat fixed grip
	LiftHSQuad                      // 4-seat high-speed detachable quad
	LiftHS6Pack                     // 6-seat high-speed detachable 6-pack
	LiftGondola                     // 8-person monocable detachable gondola
	LiftHeli                        // helicopter heli-ski "lift" (no cable)
	LiftFixedTriple                 // 3-seat fixed grip
	LiftMagicCarpet                 // conveyor belt for beginners, riders standing on it (a surface lift)
	LiftRopeTow                     // a moving rope riders hold, sliding up on their skis (a surface lift)
	LiftTBar                        // T-shaped bars off an overhead cable pulling two riders up on their skis (a surface lift)
	LiftTram                        // aerial tramway: two big cabins shuttling back and forth, each on its own track (tram.go)
	LiftSingle                      // 1-seat fixed grip (Mad River Glen's)
	LiftHS8Pack                     // 8-seat high-speed detachable 8-pack, on wider-set cables
	LiftFunitel                     // 24-person cabins on two haul ropes a way, steady in wind (cabin_lifts.go)
	LiftTri                         // tricable (3S) gondola: 28-person cabins on two track ropes and a haul rope, huge spans (cabin_lifts.go)
	LiftFunicular                   // two counterbalanced cars on a rail up the slope (funicular.go)
)

const (
	HeliCapacity   = 6    // seats per helicopter (typical light-heli capacity)
	HeliAirspeedMs = 20.0 // m/s cruise speed (~72 km/h, conservative for a Bell 407)
	HeliFlightAlt  = 30.0 // metres AGL during transit
)

// Capacity returns the number of riders a single chair of this lift
// type carries.
func (t LiftType) Capacity() int {
	switch t {
	case LiftFixedTriple:
		return 3
	case LiftFixedQuad, LiftHSQuad:
		return 4
	case LiftHS6Pack:
		return 6
	case LiftGondola:
		return 8
	case LiftHeli:
		return HeliCapacity
	case LiftMagicCarpet, LiftRopeTow:
		return 1
	case LiftTram:
		return TramCapacity
	case LiftFunicular:
		return FunicularCapacity
	case LiftSingle:
		return 1
	case LiftHS8Pack:
		return 8
	case LiftFunitel:
		return 24
	case LiftTri:
		return 28
	}
	return 2 // a T-bar too
}

// MeshID returns the chair-mesh ID the renderer should draw for this
// lift type. Slot anchors registered for the same mesh ID drive
// per-rider seat positioning in sim.tickRiding.
func (t LiftType) MeshID() uint32 {
	switch t {
	case LiftFixedTriple:
		return MeshChairTriple
	case LiftFixedQuad, LiftHSQuad:
		return MeshChairQuad
	case LiftHS6Pack:
		return MeshChair6Pack
	case LiftGondola:
		return MeshGondolaCabin
	case LiftHeli:
		return MeshHelicopter
	case LiftMagicCarpet, LiftRopeTow:
		return MeshBeltPlace
	case LiftTBar:
		return MeshTBarHanger
	case LiftTram:
		return MeshTramCabin
	case LiftFunicular:
		return MeshFunicularCar
	case LiftSingle:
		return MeshChairSingle
	case LiftHS8Pack:
		return MeshChair8Pack
	case LiftFunitel:
		return MeshFunitelCabin
	case LiftTri:
		return MeshTriCabin
	}
	return MeshChair
}

// DefaultSpeed returns the cable speed in m/s that PlaceLift assigns to
// a new lift of this type. High-speed quads run at 5 m/s; fixed-grip
// lifts run at 2.5 m/s.
func (t LiftType) DefaultSpeed() float32 {
	switch t {
	case LiftHSQuad, LiftHS6Pack:
		return 5.0
	case LiftGondola:
		return 6.0
	case LiftHeli:
		return HeliAirspeedMs
	case LiftMagicCarpet:
		return 1.0
	case LiftTBar:
		return 3.0
	case LiftTram, LiftFunicular:
		return 10.0
	case LiftHS8Pack, LiftFunitel:
		return 6.0
	case LiftTri:
		return 7.5
	case LiftSingle:
		return 2.2
	}
	return 2.5 // a rope tow too
}

// Label returns a short human-readable name for HUD / popup display.
func (t LiftType) Label() string {
	switch t {
	case LiftFixedTriple:
		return "Triple"
	case LiftFixedQuad:
		return "Fixed Quad"
	case LiftHSQuad:
		return "High-Speed Quad"
	}
	if !edition.Demo {
		switch t {
		case LiftHS6Pack:
			return "High-Speed 6-Pack"
		case LiftGondola:
			return "Gondola"
		case LiftHeli:
			return "Helicopter"
		case LiftMagicCarpet:
			return "Magic Carpet"
		case LiftRopeTow:
			return "Rope Tow"
		case LiftTBar:
			return "T-Bar"
		case LiftTram:
			return "Aerial Tram"
		case LiftFunicular:
			return "Funicular"
		case LiftSingle:
			return "Single"
		case LiftHS8Pack:
			return "High-Speed 8-Pack"
		case LiftFunitel:
			return "Funitel"
		case LiftTri:
			return "3S Gondola"
		}
	}
	return "Double"
}

// TerrainDifficulty is a bitfield of trail-marker categories a lift
// services. A lift can post any subset — including the empty set — so
// the values compose with bitwise OR. Pure metadata for now (the AI
// doesn't read it); the trail-tolerance feature on the roadmap will.
type TerrainDifficulty uint8

const (
	DiffGreen TerrainDifficulty = 1 << iota // beginner runs
	DiffBlue                                // intermediate runs
	DiffBlack                               // advanced / expert runs
)

// Has reports whether the difficulty set includes flag.
func (d TerrainDifficulty) Has(flag TerrainDifficulty) bool {
	return d&flag != 0
}

// Toggle flips flag in the difficulty set and returns the new value.
// Used by UI toggle buttons that operate on a pointer to the set.
func (d TerrainDifficulty) Toggle(flag TerrainDifficulty) TerrainDifficulty {
	return d ^ flag
}

// Chair holds one chair on the lift loop.
// Progress 0→1 is a full loop by distance (Lift.loopAt): 0 is the base's
// load point, UnloadProgress the top's unload point, and it wraps back to
// the load point at 1.
//
// Passengers is sized to the parent Lift's Type.Capacity() at construction
// time; iterate it directly rather than assuming a fixed length.
type Chair struct {
	Progress   float32
	Passengers []*Guest
}

// Lift represents a ski lift connecting a base to a top station.
//
// Base and Top are continuous world XZ positions (metres). Y is derived
// from terrain elevation at use time.
type Lift struct {
	ID    uint64
	Type  LiftType
	Name  string // free-form label; empty means "unnamed"
	Base  mgl32.Vec2
	Top   mgl32.Vec2
	Speed float32 // cable speed in m/s (typical real lift: 2–3 m/s)

	// TicketPrice is the per-ride fare credited to World.Cash when a
	// guest boards. Only heli lifts charge per ride (see RideFare); cable
	// lifts are covered by World.DayTicketPrice. Set via the lift popup.
	TicketPrice int

	// Open controls whether guests may join the queue. Closed lifts still
	// run (chairs keep moving, seated guests complete their ride) but no
	// new guests board. A new lift starts stopped (PlaceLift): that's
	// intentional, the player opens each lift once it's ready, so check
	// this first when guests won't ride.
	Open bool

	// Testing is a test run by hand: it runs empty, nobody boarding, even
	// with the resort closed (wind.go).
	Testing bool
	// WindHold is held for the wind; CalmSince is when the wind last
	// dropped under its resume level (0 while it hasn't), TestUntil the
	// end of the test run after one, and Wind the wind at its top now,
	// m/s. Set by the sim; not saved.
	WindHold bool
	// SnowHold is held for too little snow at its base. OnHold is either.
	SnowHold  bool
	CalmSince float64
	TestUntil float64
	Wind      float32

	// AvyClosed is stopped because every run off its top is closed for
	// avalanche danger (Trail.AvyClosed). Set by the sim; not saved.
	AvyClosed bool

	// Lit is lights along its line and at its stations: it runs after
	// dark (lights.go).
	Lit bool

	// OnHold is set automatically when the lift base has too little snow
	// (LiftHoldDepth).
	// A held lift drains its chairs (unloading at the top as normal) but
	// does not board new riders; once all chairs are empty it stops moving.
	// Cleared automatically when the snow's back (LiftResumeDepth). Not
	// persisted.
	OnHold bool

	// Staff is who works the lift beyond its two operators.
	Staff LiftStaff
	// Bubble is a 6-pack's or 8-pack's bubble covers (Bubbles), bought in
	// its popup: riders ride out of the wind under them, a better ride.
	Bubble bool
	// Fill is how full chairs leave the base while people are still in
	// line, a running average (0 until a chair has loaded with a line
	// behind it); see LineWait. Not persisted.
	Fill float32

	// Queue is who's in line, in boarding order. Crowd is where they
	// stand when the line is a crowd (no line attendant), cheapest place
	// first; nil for a single file. Set by UpdateLineLayouts; not saved.
	Queue  []*Guest
	Crowd  []CrowdPlace
	Chairs []Chair
	// crowdEdge is the crowd's edge for arrivals (crowdEdgeCache), kept
	// up to date as places fill and empty.
	crowdEdge crowdEdgeCache

	// liftExtras is state only the full game's lifts have: a heli lift's
	// flight (heli.go). Empty in the demo, which has no heli.
	liftExtras

	// profileCache is the laid-out cable (lift_profile.go), and
	// towerCache a T-bar's towers; towerXZCache is where either's towers
	// stand, reused by the per-tick hazard sampler. Towers are fixed
	// once placed.
	profileCache *cableProfile
	// funicTrack is a funicular's laid-out track (funicular.go).
	funicTrack   *RailLine
	towerCache   []TowerSpot
	towerXZCache []mgl32.Vec2
	// stationCache memoises StationXZs(), as towerCache does.
	stationCache []mgl32.Vec2
	// loopTime and rideTime cache loopTimes for the speed and ends in
	// timesKey.
	timesKey           [5]float32
	loopTime, rideTime float32

	// QueueFold is how many places of the single-file line run out
	// from the base before it doubles back, 0 for a straight line;
	// QueueFoldSide (+1 or -1 along lateralRight) is the side the lanes
	// step out to. Set by UpdateQueueFolds; not saved.
	QueueFold     int
	QueueFoldSide float32
	// CorralRows is how many rows each lane of the line attendant's corral
	// has, 0 for no corral (lift_corral.go). Not saved.
	CorralRows int
	corralRoom int // the most rows its lanes can have here (updateCorralRoom)
	// mergeTurn is the lane (1 or 2) whose turn it is to send guests into
	// the run, and mergeTaken how many it has sent this turn.
	mergeTurn, mergeTaken int
}

// TowerSpacing is the target distance between a T-bar's towers.
const TowerSpacing = 50.0

// Towers returns every tower along the lift, as the renderer draws them:
// laid out by the ground for an aerial lift (lift_profile.go), evenly
// spaced for a T-bar (towersEven). Nil for lifts with none.
//
// Cached on the Lift: towers are fixed once placed. Editing tools that
// move Base or Top, or reshape the ground under a lift, must call
// ResetTowerCache.
func (l *Lift) Towers(t *Terrain) []TowerSpot {
	if p := l.profile(t); p != nil {
		return p.towers
	}
	if l.towerCache == nil {
		l.towerCache = l.towersEven()
	}
	return l.towerCache
}

// TowerXZs is where Towers stand.
func (l *Lift) TowerXZs(t *Terrain) []mgl32.Vec2 {
	towers := l.Towers(t)
	if len(towers) == 0 {
		return nil
	}
	if l.towerXZCache == nil {
		l.towerXZCache = make([]mgl32.Vec2, len(towers))
		for i, tw := range towers {
			l.towerXZCache[i] = tw.XZ
		}
	}
	return l.towerXZCache
}

// towersEven spaces a T-bar's towers evenly: the first and last
// StationOffset past each terminal's flat run (StationGeom.Flat), the
// rest TowerSpacing apart or a little less, all carrying the cable
// TBarCableHeight over the snow.
func (l *Lift) towersEven() []TowerSpot {
	bx, bz := l.Base[0], l.Base[1]
	tx, tz := l.Top[0], l.Top[1]
	dx := tx - bx
	dz := tz - bz
	length := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	stationOffset := l.Ramp() + l.Station().Flat
	if length <= 2*stationOffset || !l.Type.HasTowers() {
		return nil
	}
	innerLen := length - 2*stationOffset
	intervals := int(innerLen / TowerSpacing)
	if intervals < 1 {
		intervals = 1
	}
	spacing := innerLen / float32(intervals)
	dirX := dx / length
	dirZ := dz / length
	out := make([]TowerSpot, 0, intervals+1)
	for i := 0; i <= intervals; i++ {
		d := stationOffset + float32(i)*spacing
		out = append(out, TowerSpot{XZ: mgl32.Vec2{bx + dirX*d, bz + dirZ*d}, Height: TBarCableHeight})
	}
	return out
}

// ResetTowerCache discards the cached tower-position slice. Call after
// mutating Base or Top so the next TowerXZs() call recomputes.
func (l *Lift) ResetTowerCache() {
	l.profileCache = nil
	l.funicTrack = nil
	l.towerCache = nil
	l.towerXZCache = nil
	l.stationCache = nil
}

// QueueCell returns the grid cell containing the lift's base bullwheel.
// Same convention as Building.DoorCell.
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

// QueueDirXZ is the exported version of queueDir for use by the renderer.
func (l *Lift) QueueDirXZ() (float32, float32) { return l.queueDir() }

// LateralRightXZ is the exported version of lateralRight for use by the renderer.
func (l *Lift) LateralRightXZ() (float32, float32) { return l.lateralRight() }

// QueueSlotXZ returns the world-XZ position of the index-th queue slot.
// Slot 0 is the boarding spot at the base; slot N+1 sits QueueSpacing
// further along queueDir. Cell-only callers (pathfinder) don't need Y.
func (l *Lift) QueueSlotXZ(index int) (x, z float32) {
	along, lat := l.queueSlotFrame(index)
	return l.fromQueueFrame(along, lat)
}

// BackOfQueueWorldPos returns where an arrival at from should head: the
// back of a single file, or the edge of a crowd nearest them
// (LineEndXZ). Pulled each tick by resolveTarget so the target shifts
// as the line grows or boards.
func (l *Lift) BackOfQueueWorldPos(from mgl32.Vec2, t *Terrain) mgl32.Vec3 {
	p := l.LineEndXZ(from)
	return mgl32.Vec3{p[0], t.InterpolatedSurfaceElevationAt(p[0], p[1]), p[1]}
}

// BackOfQueueCell returns the grid cell of BackOfQueueWorldPos — the
// pathfinder destination for skiers walking to queue. A snapshot at
// path-plan time; tickQueued handles any residual drift once they
// arrive and join the actual queue.
func (l *Lift) BackOfQueueCell(from mgl32.Vec2) [2]int {
	return cellOf(l.LineEndXZ(from))
}

// QueueLen returns how many guests are in line.
func (l *Lift) QueueLen() int {
	return len(l.Queue)
}

// lateralRight returns the unit vector pointing to the right of the
// cable axis when facing uphill (base → top). Used to position lanes.
func (l *Lift) lateralRight() (rx, rz float32) {
	dx := l.Top[0] - l.Base[0]
	dz := l.Top[1] - l.Base[1]
	length := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if length < 1e-3 {
		return 1, 0
	}
	cx, cz := dx/length, dz/length
	// Rotate cable direction -90° in XZ → right side when looking uphill.
	return cz, -cx
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

// Cableless reports whether this lift is a heli-ski helicopter rather
// than a cable lift. Helicopter lifts fly (heli.go) and have no Chairs,
// cables, or towers. Always false in the demo, which has no heli, so
// heli branches compile away there.
func (l *Lift) Cableless() bool {
	return l.Type.Cableless()
}

// Cableless reports whether lifts of this type fly rather than run on a
// cable: the heli. Always false in the demo, which has no heli.
func (t LiftType) Cableless() bool {
	return !edition.Demo && t == LiftHeli
}

// Surface reports whether riders of this type keep their skis on the
// snow instead of sitting on a chair under a cable: standing on a belt
// (the magic carpet, Belt) or pulled up on a rope (the rope tow, Tow). A
// surface lift's places ride at ground height, it has no towers or
// overhead cable, and riders go straight off at the top. Always false in
// the demo, which has none.
func (t LiftType) Surface() bool {
	return !edition.Demo && (t == LiftMagicCarpet || t == LiftRopeTow || t == LiftTBar)
}

// Belt reports whether riders of this type stand on a moving belt (the
// magic carpet), BeltHeight over the snow.
func (t LiftType) Belt() bool { return !edition.Demo && t == LiftMagicCarpet }

// Tow reports whether riders of this type are pulled up on their skis
// (the rope tow, the T-bar). Novices won't ride one, and riders can fall
// off.
func (t LiftType) Tow() bool { return t.Rope() || t.Hangers() }

// Rope reports whether riders of this type hold a rope at their side (the
// rope tow), on low posts (RopePostXZs).
func (t LiftType) Rope() bool { return !edition.Demo && t == LiftRopeTow }

// Hangers reports whether riders of this type are pulled by bars hanging
// off an overhead cable on towers (the T-bar), two to a bar. Each place
// on the loop is a hanger: the cable runs TBarCableHeight over the snow
// all the way, so riders' feet, the hanger's seat slots, are on it.
func (t LiftType) Hangers() bool { return !edition.Demo && t == LiftTBar }

// HasTowers reports whether a lift of this type has towers along its
// line (TowerXZs): a chairlift, a T-bar or a tram.
func (t LiftType) HasTowers() bool { return (!t.Surface() && !t.Funicular()) || t.Hangers() }

// TBarCableHeight is a T-bar's cable over the snow, the same all the way
// (its hangers reach from it to the riders' hips).
const TBarCableHeight = 4.5

// RideHeight is how far over the snow a surface lift's riders stand: on
// the belt, else on the snow.
func (l *Lift) RideHeight() float32 {
	if l.Type.Belt() {
		return BeltHeight
	}
	return 0
}

// BeltHeight is how far a magic carpet's belt sits above the ground,
// where its riders stand.
const BeltHeight = 0.12

// A rope tow's rope runs RopeHeight over the snow, RopeGap off the line
// on each side (the up run inside its riders, who ride CableGap off),
// held up by posts every RopePostSpacing.
const (
	RopeHeight      = 0.95
	RopeGap         = CableGap - 0.45
	RopePostSpacing = 25.0
)

// PlaceSpacing is the metres between places on the loop: a chair every
// ChairSpacingM, a rider every few metres on a belt or a rope.
func (t LiftType) PlaceSpacing() float32 {
	switch {
	case t.Belt():
		return 2.5
	case t.Rope():
		return 8
	case t.Hangers():
		return 15
	case t == LiftFunitel:
		return 60
	case t == LiftTri:
		return 100
	}
	return ChairSpacingM // a tram has two cabins whatever its length (PlaceLift)
}

// MaxLength is the longest lift of this type, metres between its ends;
// 0 for no limit.
func (t LiftType) MaxLength() float32 {
	switch {
	case t.Belt():
		return 120
	case t.Rope():
		return 300
	case t.Hangers():
		return 1500
	}
	return 0
}

// MaxGrade is the steepest ground a lift of this type climbs, degrees,
// over any MaxGradeSpan metres of its line (a funicular, its line end
// to end); 0 for no limit.
func (t LiftType) MaxGrade() float32 {
	switch {
	case t.Belt():
		return 12
	case t.Rope():
		return 25
	case t.Hangers():
		return 30
	case t.Funicular():
		return 45
	}
	return 0
}

// RopePostXZs is where a rope tow's posts stand, on the line between its
// ends, about RopePostSpacing apart; nil for any other lift or a tow too
// short to need one.
func (l *Lift) RopePostXZs() []mgl32.Vec2 {
	if !l.Type.Rope() {
		return nil
	}
	length := l.cableLen()
	n := int(length / RopePostSpacing)
	if n < 2 {
		return nil
	}
	dir := l.Top.Sub(l.Base).Mul(1 / length)
	out := make([]mgl32.Vec2, 0, n-1)
	for i := 1; i < n; i++ {
		out = append(out, l.Base.Add(dir.Mul(length*float32(i)/float32(n))))
	}
	return out
}

// MaxGradeSpan is the stretch of line MaxGrade is measured over.
const MaxGradeSpan = 10.0

// Operators is how many operators run a lift of this type: one at each
// station, or one at the base of a magic carpet or rope tow.
func (t LiftType) Operators() int {
	if t.Belt() || t.Rope() {
		return 1
	}
	return LiftOperators
}

// Available reports whether this edition has the lift type: the demo
// leaves out the 6-pack, the gondola, and the heli.
func (t LiftType) Available() bool {
	return !edition.Demo || t == LiftDouble || t == LiftFixedTriple || t == LiftFixedQuad || t == LiftHSQuad
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

// RideFare is what a guest without a season pass pays to board this lift,
// in dollars. Cable lifts are covered by the day ticket and cost nothing
// per ride; only heli keeps per-ride pricing (VISION §7).
func (l *Lift) RideFare() int {
	if l.Type == LiftHeli {
		return l.TicketPrice
	}
	return 0
}

// StationCost returns the fixed station-pair build cost in dollars for a
// lift of this type (VISION §7: fixed quad $700k, HS quad $1.5M, gondola
// $4M). Heli returns 0; helipads are priced by HelipadCost.
func (t LiftType) StationCost() int {
	switch t {
	case LiftDouble:
		return 400_000
	case LiftFixedTriple:
		return 475_000
	case LiftFixedQuad:
		return 550_000
	case LiftHSQuad:
		return 1_150_000
	case LiftHS6Pack:
		return 1_700_000
	case LiftGondola:
		return 4_000_000
	case LiftMagicCarpet:
		return 90_000
	case LiftRopeTow:
		return 40_000
	case LiftTBar:
		return 250_000
	case LiftTram:
		return 7_000_000
	case LiftFunicular:
		return 2_500_000
	case LiftSingle:
		return 300_000
	case LiftHS8Pack:
		return 2_300_000
	case LiftFunitel:
		return 6_000_000
	case LiftTri:
		return 9_000_000
	}
	return 0
}

// PerMeterCost returns the build cost in dollars per metre of cable run
// (towers + cable) for this type. The gondola's rate puts a 2.5 km span
// near $10M.
func (t LiftType) PerMeterCost() int {
	switch t {
	case LiftDouble:
		return 150
	case LiftFixedTriple:
		return 175
	case LiftFixedQuad:
		return 200
	case LiftHSQuad:
		return 200
	case LiftHS6Pack:
		return 250
	case LiftGondola:
		return 2_400
	case LiftMagicCarpet:
		return 600 // the belt itself
	case LiftRopeTow:
		return 120
	case LiftTBar:
		return 140 // towers and cable
	case LiftTram:
		return 2_000 // track ropes, haul rope, a few big towers
	case LiftFunicular:
		return 3_000 // the track bed and rails
	case LiftSingle:
		return 120
	case LiftHS8Pack:
		return 300
	case LiftFunitel:
		return 3_200
	case LiftTri:
		return 4_500
	}
	return 0
}

// RunningCostDay returns the daily running cost in dollars (power,
// maintenance) for an open lift of this type, on top of its staff
// (LiftStaff.Headcount at LiftStaffDailyCost each).
func (t LiftType) RunningCostDay() int {
	switch t {
	case LiftDouble:
		return 900
	case LiftFixedTriple:
		return 1_050
	case LiftFixedQuad:
		return 1_200
	case LiftHSQuad:
		return 2_400
	case LiftHS6Pack:
		return 3_000
	case LiftGondola:
		return 7_500
	case LiftHeli:
		return 9_000 // pilot + fuel
	case LiftMagicCarpet:
		return 200
	case LiftRopeTow:
		return 150
	case LiftTBar:
		return 700
	case LiftTram:
		return 8_000
	case LiftFunicular:
		return 5_000
	case LiftSingle:
		return 700
	case LiftHS8Pack:
		return 3_800
	case LiftFunitel:
		return 9_000
	case LiftTri:
		return 12_000
	}
	return 0
}

// LiftQuality is the quality of a lift ride as a service, toward a
// guest's quality stars (Guest.UseService): the going rate for now
// (notes/next/Service Quality.md). BubbleLiftQuality is a ride under a
// bubble cover.
const (
	LiftQuality       = 0.5
	BubbleLiftQuality = 0.65
)

// RideQuality is the quality of a ride on l.
func (l *Lift) RideQuality() float32 {
	if l.Bubble && l.Type.Bubbles() {
		return BubbleLiftQuality
	}
	return LiftQuality
}

// Bubbles reports whether chairs of this type can have bubble covers:
// the 6-pack and the 8-pack.
func (t LiftType) Bubbles() bool { return !edition.Demo && (t == LiftHS6Pack || t == LiftHS8Pack) }

// BubbleCostPerChair is what a bubble cover costs, each chair.
const BubbleCostPerChair = 6_000

// BubbleCost is what covering every chair of l costs.
func (l *Lift) BubbleCost() int { return len(l.Chairs) * BubbleCostPerChair }

// Line waits. A guest won't join a line whose wait is over their
// MaxLineWaitFor (they look for another lift, or say "every lift line is
// way too long"), and calls one over LongLineWait long. All clock time.
const (
	MaxLineWait = SimSecondsPerHour / 2
	// FirstRideMaxLineWait is the longest wait a guest who hasn't ridden
	// yet today will join: they came to ski, so they queue and grumble.
	// With MaxLineWait for everyone, holiday arrivals took one look at
	// the lines and went home without a run, a 1★ each, and their stalls
	// let the next car in (Boreal Tutorial, 2026-10-12).
	FirstRideMaxLineWait = SimSecondsPerHour * 3 / 2
	LongLineWait         = SimSecondsPerHour / 4
	// fallbackSecPerRider is the wait per person ahead for a lift with no
	// chairs to count (a heli).
	fallbackSecPerRider = 8.0
)

// MaxLineWaitFor is the longest wait a guest will join a line for:
// longer before their first ride of the day.
func MaxLineWaitFor(ridden bool) float32 {
	if ridden {
		return MaxLineWait
	}
	return FirstRideMaxLineWait
}

// RidersPerSecond is how many riders the lift loads a second: every
// chair passes the load point once a loop.
func (l *Lift) RidersPerSecond() float32 {
	loop := l.LoopTime()
	if len(l.Chairs) == 0 || loop <= 0 {
		return 0
	}
	return float32(len(l.Chairs)*len(l.Chairs[0].Passengers)) / loop
}

// LineWait is about how long a guest joining the back of the line would
// wait to board, in sim seconds: chairs leave only Fill full.
func (l *Lift) LineWait() float32 {
	if rps := l.RidersPerSecond(); rps > 0 {
		if l.Fill > 0 {
			rps *= l.Fill
		}
		return float32(l.QueueLen()) / rps
	}
	return float32(l.QueueLen()) * fallbackSecPerRider
}

// LiftStaff is who works a lift. Every cable lift has an operator at the
// bottom and one at the top (in the huts); those aren't optional. A line
// attendant at the loading point fills the chairs: without one, strangers
// rarely share a chair, so chairs leave the base partly empty while people
// wait (sim: boarding). A heli has a pilot instead (RunningCostDay).
type LiftStaff struct {
	LineAttendant bool
}

// LiftOperators is the operators every cable lift has, top and bottom.
const LiftOperators = 2

// Headcount is how many people work lift l, for wages.
func (l *Lift) Headcount() int {
	if l.Cableless() {
		return LiftOperators // ground crew; the pilot is in RunningCostDay
	}
	n := l.Type.Operators()
	if l.Staff.LineAttendant {
		n++
	}
	return n
}

// LiftStaffRole is a job at a lift.
type LiftStaffRole uint8

const (
	StaffBottomOperator LiftStaffRole = iota
	StaffTopOperator
	StaffLineAttendant
)

// LiftStaffPost is where one of a lift's staff stands, facing Heading
// (radians, atan2(x, z) like a guest's), on duty.
type LiftStaffPost struct {
	Role    LiftStaffRole
	Pos     mgl32.Vec2
	Heading float32
}

// StaffPosts is where l's staff stand: an operator at each station's
// hut window, facing the load or unload point, and the line attendant
// (if any) beside the load point on the far side of the chairs from the
// hut, facing the riders. Nil for a heli.
func (l *Lift) StaffPosts() []LiftStaffPost {
	if l.Cableless() {
		return nil
	}
	var hut stationRect
	for _, r := range l.Station().rects {
		if r.hut {
			hut = r
		}
	}
	var out []LiftStaffPost
	post := func(role LiftStaffRole, pos, look mgl32.Vec2) {
		d := look.Sub(pos)
		out = append(out, LiftStaffPost{Role: role, Pos: pos, Heading: float32(math.Atan2(float64(d[0]), float64(d[1])))})
	}
	for _, top := range [2]bool{false, true} {
		if top && l.Type.Operators() < 2 {
			break // a magic carpet's one operator is at the base
		}
		at, back, side, ok := l.StationFrame(top)
		if !ok {
			return nil
		}
		// At the window on the line side of the hut, half a metre out;
		// with no hut (a surface lift), at the station's operator spot.
		p := at.Add(back.Mul((hut.front + hut.rear) / 2)).Add(side.Mul(hut.in - 0.6))
		if hut == (stationRect{}) {
			// The frame turns end for end at the top, so the riders'
			// side is the other way across it.
			op := l.Station().operator
			if top {
				op[1] = -op[1]
			}
			p = at.Add(back.Mul(op[0])).Add(side.Mul(op[1]))
		}
		role, look := StaffBottomOperator, l.LoadXZ()
		if top {
			role, look = StaffTopOperator, l.UnloadXZ()
		}
		post(role, p, look)
	}
	if l.Staff.LineAttendant {
		dir, perp := l.lineAxes()
		load := l.LoadXZ()
		reach := float32(l.Type.Capacity())*0.3 + 0.7 // past the chair's end (0.6 m a seat), by the posts
		if l.Type.Shuttle() {
			reach = 1.0 // on the platform between the cabins, beside the line
		}
		post(StaffLineAttendant, load.Sub(dir.Mul(0.8)).Add(perp.Mul(reach)), load)
	}
	return out
}
