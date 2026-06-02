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

// LiftType is the chair variant a lift carries. Capacity, chair mesh,
// and per-instance slot anchors all derive from this — the simulation
// loads N riders per chair where N = Type.Capacity(), and the renderer
// picks the matching chair OBJ via Type.MeshID().
type LiftType uint8

const (
	LiftDouble    LiftType = iota // 2-seat fixed grip (the original)
	LiftFixedQuad                 // 4-seat fixed grip
	LiftHSQuad                    // 4-seat high-speed detachable quad
	LiftHS6Pack                   // 6-seat high-speed detachable 6-pack
	LiftGondola                   // 8-person monocable detachable gondola
	LiftHeli                      // helicopter heli-ski "lift" (no cable)
)

// HeliPhase is the state of a heli-ski helicopter in its flight cycle.
type HeliPhase uint8

const (
	HeliAtBase HeliPhase = iota // parked at helipad, loading passengers
	HeliToTop                   // flying to the drop zone
	HeliAtTop                   // at drop zone, unloading
	HeliToBase                  // flying back empty
)

const (
	HeliCapacity   = 6    // seats per helicopter (typical light-heli capacity)
	HeliAirspeedMs = 20.0 // m/s cruise speed (~72 km/h, conservative for a Bell 407)
	HeliFlightAlt  = 30.0 // metres AGL during transit
)

// HeliData holds the dynamic state of a heli-ski helicopter. Non-nil only
// on Lift records with Type == LiftHeli; all other lift types leave this nil.
type HeliData struct {
	Phase      HeliPhase
	Progress   float32   // 0→1 within the current HeliToTop or HeliToBase leg
	Passengers []*Guest  // up to HeliCapacity; nil slots are empty seats
}

// Capacity returns the number of riders a single chair of this lift
// type carries.
func (t LiftType) Capacity() int {
	switch t {
	case LiftFixedQuad, LiftHSQuad:
		return 4
	case LiftHS6Pack:
		return 6
	case LiftGondola:
		return 8
	case LiftHeli:
		return HeliCapacity
	}
	return 2
}

// MeshID returns the chair-mesh ID the renderer should draw for this
// lift type. Slot anchors registered for the same mesh ID drive
// per-rider seat positioning in sim.tickRiding.
func (t LiftType) MeshID() uint32 {
	switch t {
	case LiftFixedQuad, LiftHSQuad:
		return MeshChairQuad
	case LiftHS6Pack:
		return MeshChair6Pack
	case LiftGondola:
		return MeshGondolaCabin
	case LiftHeli:
		return MeshHelicopter
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
	}
	return 2.5
}

// Label returns a short human-readable name for HUD / popup display.
func (t LiftType) Label() string {
	switch t {
	case LiftFixedQuad:
		return "Fixed Quad"
	case LiftHSQuad:
		return "High-Speed Quad"
	case LiftHS6Pack:
		return "High-Speed 6-Pack"
	case LiftGondola:
		return "Gondola"
	case LiftHeli:
		return "Helicopter"
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
//
// Passengers is sized to the parent Lift's Type.Capacity() at construction
// time; iterate it directly rather than assuming a fixed length.
type Chair struct {
	Progress   float32
	Passengers []*Guest
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
	Type  LiftType
	Name  string // free-form label; empty means "unnamed"
	Base  mgl32.Vec2
	Top   mgl32.Vec2
	Speed float32 // cable speed in m/s (typical real lift: 2–3 m/s)

	// TicketPrice is the per-ride fare credited to World.Cash when a
	// skier boards a chair. Set per-lift via the lift popup.
	TicketPrice int

	// Open controls whether guests may join the queue. Closed lifts still
	// run (chairs keep moving, seated guests complete their ride) but no
	// new guests board. Defaults to true on placement.
	Open bool

	// OnHold is set automatically when the lift base cell has no snow.
	// A held lift drains its chairs (unloading at the top as normal) but
	// does not board new riders; once all chairs are empty it stops moving.
	// Cleared automatically when snow returns to the base. Not persisted.
	OnHold bool

	Queue       []*Guest
	QueueConfig LiftQueueConfig
	Lines       []LiftLine // nil/empty → use Queue (all non-Double lifts and Doubles without lanes)
	QueueRound  int        // round-robin boarding index into Lines
	Chairs      []Chair

	// HeliState is non-nil only for LiftHeli type lifts. Cable lifts leave
	// this nil; callers should use IsHeli() to branch rather than testing
	// the pointer directly.
	HeliState *HeliData

	// towerCache memoises TowerXZs() — towers are immutable after
	// placement, so the slice can be reused across the per-tick L1
	// hazard sampler instead of rebuilt every call.
	towerCache []mgl32.Vec2
}

// LoopLength returns the total length of the chair loop in metres (2× cable length).
func (l *Lift) LoopLength() float32 {
	dx := l.Top[0] - l.Base[0]
	dz := l.Top[1] - l.Base[1]
	return 2 * float32(math.Sqrt(float64(dx*dx+dz*dz)))
}

// TowerSpacing is the target distance between adjacent towers along a
// lift cable. Real chairlifts run with 40–60 m spacing; 50 m sits in
// the middle. Kept package-level so render and sim agree on tower XZ
// positions without re-deriving spacing.
const TowerSpacing = 50.0

// TowerXZs returns the XZ positions of every tower along the lift,
// matching the spacing the renderer draws. Returns nil for lifts too
// short to fit any towers (length ≤ 2 × StationOffset). The first and
// last entries sit StationOffset inboard from the base and top stations
// respectively; intermediate entries are evenly spaced.
//
// The result is cached on the Lift after the first call — towers are
// purely a function of Base / Top / StationOffset / TowerSpacing, all
// of which are immutable after placement. Editing tools that mutate
// Base or Top must call ResetTowerCache() to discard the stale slice
// (or set towerCache = nil); none currently do because lifts aren't
// movable post-placement.
func (l *Lift) TowerXZs() []mgl32.Vec2 {
	if l.towerCache != nil {
		return l.towerCache
	}
	bx, bz := l.Base[0], l.Base[1]
	tx, tz := l.Top[0], l.Top[1]
	dx := tx - bx
	dz := tz - bz
	length := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	stationOffset := float32(StationOffset)
	if length <= 2*stationOffset {
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
	out := make([]mgl32.Vec2, 0, intervals+1)
	for i := 0; i <= intervals; i++ {
		d := stationOffset + float32(i)*spacing
		out = append(out, mgl32.Vec2{bx + dirX*d, bz + dirZ*d})
	}
	l.towerCache = out
	return out
}

// ResetTowerCache discards the cached tower-position slice. Call after
// mutating Base or Top so the next TowerXZs() call recomputes.
func (l *Lift) ResetTowerCache() {
	l.towerCache = nil
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

// Line-queue geometry constants. Only used when a LiftDouble has Lines configured.
const (
	LineWidth       = 2.0 // lateral metres per pair slot within a row
	LineGap         = 0.5 // downhill gap between adjacent rows (aisle width)
	PairHalfWidth   = 0.5 // each person stands this far from their slot centre laterally
	SingleLineWidth = 1.0 // lateral metres per slot in a single-rider row
)

// LiftQueueConfig controls how the boarding queue at a lift base is laid out.
// Applicable only to LiftDouble; other lift types use the flat Queue.
type LiftQueueConfig struct {
	LeftLines   int     // pair-wide lanes on the left side of the boarding area (0–5)
	RightLines  int     // pair-wide lanes on the right side (0–5)
	SingleRider bool    // add a single-rider lane on each side to fill partial pairs
	RopeDepth   float32 // metres ropes extend from base; 0 → DefaultRopeDepth
}

// DefaultRopeDepth is the lateral rope extent (metres from cable axis to
// each tip) when RopeDepth is not set.
const DefaultRopeDepth = float32(10.0)

// EffectiveRopeDepth returns the configured rope depth, falling back to DefaultRopeDepth.
func (cfg LiftQueueConfig) EffectiveRopeDepth() float32 {
	if cfg.RopeDepth > 0 {
		return cfg.RopeDepth
	}
	return DefaultRopeDepth
}

// LiftLine is one queue lane at a lift base. Regular lanes hold pairs
// (two skiers side-by-side per depth slot). Single-rider lanes hold one
// skier per depth slot and exist on both sides so they can pair up at
// boarding or fill a gap when a pair lane has an odd skier at the front.
type LiftLine struct {
	Guests   []*Guest
	IsSingle bool // true → single-rider (1-wide); false → pair (2-wide)
	IsRight  bool // true → right of cable axis; false → left
	Idx      int  // 0 = innermost lane, 1 = next outward, etc.
}

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

// QueueRopeBoundaries returns the sorted downhill offsets (metres from the
// lift base along queueDir) at which lateral rope dividers should be drawn.
// Each row contributes a front rope (at the base-side edge) and a back rope
// (at the downhill edge); the 0.5 m LineGap between adjacent rows is the
// aisle skiers use to advance.
func (l *Lift) QueueRopeBoundaries() []float32 {
	maxRows := l.QueueConfig.LeftLines
	if l.QueueConfig.RightLines > maxRows {
		maxRows = l.QueueConfig.RightLines
	}
	if l.QueueConfig.SingleRider && maxRows > 0 {
		maxRows++ // single-rider row sits furthest downhill
	}
	if maxRows == 0 {
		return nil
	}
	seen := map[float32]bool{}
	for i := 0; i < maxRows; i++ {
		frontDepth := float32(i) * (LineWidth + LineGap)
		backDepth := frontDepth + LineWidth
		seen[frontDepth] = true
		seen[backDepth] = true
	}
	out := make([]float32, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
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
	if len(l.Lines) > 0 {
		return l.backOfLinesWorldPos(t)
	}
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

// QueueLen returns the total number of skiers waiting across all lines,
// or the length of the flat Queue for lifts not using lane mode.
func (l *Lift) QueueLen() int {
	if len(l.Lines) > 0 {
		n := 0
		for i := range l.Lines {
			n += len(l.Lines[i].Guests)
		}
		return n
	}
	return len(l.Queue)
}

// ShortestLineIdx returns the index into Lines of the lane with the
// fewest guests. Returns 0 when Lines is empty.
func (l *Lift) ShortestLineIdx() int {
	if len(l.Lines) == 0 {
		return 0
	}
	best, bestLen := 0, len(l.Lines[0].Guests)
	for i := 1; i < len(l.Lines); i++ {
		if n := len(l.Lines[i].Guests); n < bestLen {
			best, bestLen = i, n
		}
	}
	return best
}

// RebuildLines reconstructs Lines from QueueConfig, evicting any
// currently queued guests (returned as a slice) so the caller can
// clear their plans. If the resulting config has no lanes, Lines is
// set to nil and the lift falls back to flat-Queue mode.
func (l *Lift) RebuildLines() []*Guest {
	var evicted []*Guest
	for i := range l.Lines {
		evicted = append(evicted, l.Lines[i].Guests...)
	}
	l.Lines = l.Lines[:0]
	l.QueueRound = 0

	cfg := l.QueueConfig
	for i := 0; i < cfg.RightLines; i++ {
		l.Lines = append(l.Lines, LiftLine{IsRight: true, Idx: i})
	}
	for i := 0; i < cfg.LeftLines; i++ {
		l.Lines = append(l.Lines, LiftLine{IsRight: false, Idx: i})
	}
	if cfg.SingleRider {
		l.Lines = append(l.Lines, LiftLine{IsRight: true, IsSingle: true})
		l.Lines = append(l.Lines, LiftLine{IsRight: false, IsSingle: true})
	}
	if len(l.Lines) == 0 {
		l.Lines = nil
	}
	return evicted
}

// EjectLinesGuests removes all guests from all Lines and returns them
// so the caller can clear their plan and queued state.
func (l *Lift) EjectLinesGuests() []*Guest {
	var evicted []*Guest
	for i := range l.Lines {
		evicted = append(evicted, l.Lines[i].Guests...)
		l.Lines[i].Guests = nil
	}
	return evicted
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

// lineGuestWorldPos returns the world-space position for a guest at
// slotIdx (0 = closest to cable axis) and pairPos (0 = inner, 1 = outer;
// ignored for single-rider rows) in the given row.
//
// New layout: each LiftLine is a ROW that extends LATERALLY (perpendicular
// to the cable) from the cable axis. Multiple rows are stacked DOWNHILL
// from the base station. slotIdx steps the guest outward along the row;
// pairPos places the two members of a pair within their slot.
//
// Row downhill position: Idx * (LineWidth + LineGap) from base.
// Slot lateral position: (slotIdx + 0.5) * slotWidth from cable axis,
// signed by IsRight.
func (l *Lift) lineGuestWorldPos(lineIdx, slotIdx, pairPos int, t *Terrain) mgl32.Vec3 {
	rx, rz := l.lateralRight()
	qx, qz := l.queueDir()
	li := &l.Lines[lineIdx]

	sign := float32(1)
	if !li.IsRight {
		sign = -1
	}

	// Downhill distance of this row from the base.
	rowDepth := float32(li.Idx) * (LineWidth + LineGap)

	// Lateral distance outward from the cable axis.
	var latOff float32
	if li.IsSingle {
		latOff = sign * (float32(slotIdx) + 0.5) * SingleLineWidth
	} else {
		// pairPos 0 → inner half of slot (−PairHalfWidth from slot centre)
		// pairPos 1 → outer half of slot (+PairHalfWidth from slot centre)
		pairOff := float32(pairPos)*PairHalfWidth*2 - PairHalfWidth
		latOff = sign * ((float32(slotIdx)+0.5)*LineWidth + pairOff)
	}

	x := l.Base[0] + latOff*rx + qx*rowDepth
	z := l.Base[1] + latOff*rz + qz*rowDepth
	y := t.InterpolatedSurfaceElevationAt(x, z)
	return mgl32.Vec3{x, y, z}
}

// GuestSlotWorldPos finds guest g in Lines and returns their assigned
// world-space queue position. ok=false when g is not in any line.
func (l *Lift) GuestSlotWorldPos(g *Guest, t *Terrain) (mgl32.Vec3, bool) {
	for lineIdx := range l.Lines {
		li := &l.Lines[lineIdx]
		for gIdx, candidate := range li.Guests {
			if candidate != g {
				continue
			}
			var slotIdx, pairPos int
			if li.IsSingle {
				slotIdx = gIdx
			} else {
				slotIdx = gIdx / 2
				pairPos = gIdx % 2
			}
			return l.lineGuestWorldPos(lineIdx, slotIdx, pairPos, t), true
		}
	}
	return mgl32.Vec3{}, false
}

// backOfLinesWorldPos returns the centre of the next open slot in the
// shortest row — the walk target for incoming skiers before they formally
// join a row.
func (l *Lift) backOfLinesWorldPos(t *Terrain) mgl32.Vec3 {
	lineIdx := l.ShortestLineIdx()
	li := &l.Lines[lineIdx]

	sign := float32(1)
	if !li.IsRight {
		sign = -1
	}

	var nextSlot int
	if li.IsSingle {
		nextSlot = len(li.Guests)
	} else {
		nextSlot = len(li.Guests) / 2
	}

	rx, rz := l.lateralRight()
	qx, qz := l.queueDir()

	rowDepth := float32(li.Idx) * (LineWidth + LineGap)
	var slotWidth float32
	if li.IsSingle {
		slotWidth = SingleLineWidth
	} else {
		slotWidth = LineWidth
	}
	latOff := sign * (float32(nextSlot)+0.5) * slotWidth

	x := l.Base[0] + latOff*rx + qx*rowDepth
	z := l.Base[1] + latOff*rz + qz*rowDepth
	y := t.InterpolatedSurfaceElevationAt(x, z)
	return mgl32.Vec3{x, y, z}
}

// BoardNextPair pulls up to cap guests from the configured lines using
// a round-robin over regular pair lanes (one pair per turn) with both
// single-rider lanes combined as one additional turn. If a pair lane
// contributes fewer than cap guests, single-rider lines fill the gap.
func (l *Lift) BoardNextPair(cap int) []*Guest {
	if len(l.Lines) == 0 {
		return nil
	}

	var regularIdxs, singleIdxs []int
	for i := range l.Lines {
		if l.Lines[i].IsSingle {
			singleIdxs = append(singleIdxs, i)
		} else {
			regularIdxs = append(regularIdxs, i)
		}
	}

	hasSingle := len(singleIdxs) > 0
	totalTurns := len(regularIdxs)
	if hasSingle {
		totalTurns++
	}
	if totalTurns == 0 {
		return nil
	}

	var taken []*Guest
	usedSingles := false

	for attempts := 0; attempts < totalTurns; attempts++ {
		turn := l.QueueRound % totalTurns
		l.QueueRound++

		if turn < len(regularIdxs) {
			li := &l.Lines[regularIdxs[turn]]
			if len(li.Guests) == 0 {
				continue
			}
			take := cap - len(taken)
			if take > 2 {
				take = 2
			}
			for take > 0 && len(li.Guests) > 0 {
				taken = append(taken, li.Guests[0])
				li.Guests = li.Guests[1:]
				take--
			}
			break
		}
		// Single-rider turn: 1 from each side, combined.
		usedSingles = true
		for _, si := range singleIdxs {
			if len(taken) >= cap {
				break
			}
			li := &l.Lines[si]
			if len(li.Guests) > 0 {
				taken = append(taken, li.Guests[0])
				li.Guests = li.Guests[1:]
			}
		}
		if len(taken) > 0 {
			break
		}
	}

	// Gap fill: if a regular pair lane had only 1 person (or all regular
	// lanes were empty), pull from single-rider lanes to reach capacity.
	if !usedSingles && len(taken) < cap {
		for _, si := range singleIdxs {
			if len(taken) >= cap {
				break
			}
			li := &l.Lines[si]
			if len(li.Guests) > 0 {
				taken = append(taken, li.Guests[0])
				li.Guests = li.Guests[1:]
			}
		}
	}

	return taken
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

// IsHeli reports whether this lift is a heli-ski helicopter rather than a
// cable lift. Helicopter lifts use HeliState for their simulation and have
// no Chairs, cables, or towers.
func (l *Lift) IsHeli() bool {
	return l.Type == LiftHeli
}

// HeliWorldPos returns the current world-space position of the helicopter
// for a LiftHeli lift. During transit legs, the helicopter follows the
// straight line between Base and Top at HeliFlightAlt above the terrain,
// rising and falling on a sine arc. At base/top the helicopter sits on
// the helipad surface.
func (l *Lift) HeliWorldPos(t *Terrain) mgl32.Vec3 {
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

// HeliHeading returns the Y-axis heading (radians) for the helicopter mesh
// so it always faces its current destination.
func (l *Lift) HeliHeading() float32 {
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
