package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Snowcat parameters — chosen for realistic ski-resort feel and to keep
// the math behind RouteCap simple.
const (
	// SnowcatSpeed is the working speed of a cat over snow, in m/s.
	// Real piste bashers run ~15 km/h while grooming (~4.2 m/s) and a
	// touch faster in transit. Single speed is fine at game scale.
	SnowcatSpeed = 4.0

	// SnowcatTillerWidth is the lateral width of the rear comb that
	// lays corduroy, in metres. Real machines are 4–6 m.
	SnowcatTillerWidth = 5.0

	// SnowcatLaneSpacing is how far apart neighbouring passes run, so
	// each overlaps the last by half a metre and leaves a seam.
	SnowcatLaneSpacing = 4.5

	// CatPurchasePrice is the one-time cost to add a cat to the global fleet.
	// The first cat is bundled into ShedCost; every additional cat costs this.
	CatPurchasePrice = 150_000

	// CatActiveCostDay is the daily operating cost for a cat actively grooming.
	CatActiveCostDay = 1_200 // operator + fuel

	// CatStandbyCostDay is the daily cost to keep a cat parked in standby.
	CatStandbyCostDay = 150

	// CellSize is one terrain cell in metres. Mirrored from a few places
	// so snowcat helpers don't pull it from a constants package.
	CellSize = 5.0
)

// CatStatus indicates whether a cat is actively grooming or parked in standby.
type CatStatus uint8

const (
	CatActive  CatStatus = 0 // grooming normally
	CatStandby CatStatus = 1 // parked at shed, lower daily cost
)

// GroomPass is one tiller-down run of a grooming plan: points about 1 m
// apart in world metres (x, z), driven from either end. Lats is the run's
// lateral coordinate at each point on the centreline; it grows by Sign
// per metre toward the left of the Pts order (see GroomMap).
type GroomPass struct {
	TrailID uint64
	Pts     [][2]float32
	Lats    []float32
	Sign    float32
}

// Length is the pass length in metres.
func (p GroomPass) Length() float32 {
	var l float32
	for i := 1; i < len(p.Pts); i++ {
		dx, dz := p.Pts[i][0]-p.Pts[i-1][0], p.Pts[i][1]-p.Pts[i-1][1]
		l += float32(math.Sqrt(float64(dx*dx + dz*dz)))
	}
	return l
}

// RouteStep is one waypoint of a cat's route, in world metres.
type RouteStep struct {
	P     [2]float32
	Groom bool // tiller down on the way to P
	// Lateral coordinate, as in GroomPass, at the previous waypoint and
	// at P, for this step's direction of travel.
	LatFrom, Lat float32
	Sign         float32
}

// Snowcat is a single grooming machine. Lives in World.Snowcats with a
// persistent ID so saves round-trip. Each cat is assigned to exactly one shed
// (its home base), but section assignments are computed globally across all
// sheds and trails by reassignAllSections.
type Snowcat struct {
	ID      uint64
	ShedID  uint64 // owning shed; cat despawns when shed is removed
	Pos     mgl32.Vec3
	Heading float32
	Status  CatStatus // Active (grooming) or Standby (parked, cheaper)

	// Section — the passes this cat grooms, and the cells under them.
	// Assigned globally by reassignAllSections; nil means unassigned.
	Section      []GroomPass
	SectionCells [][2]int

	// Route — waypoints for the current night's pass of the section:
	// grooming passes joined by turns and transits. Nil means idle.
	Route    []RouteStep
	RouteIdx int
}

// DriveToward advances the cat one tick (`dt` seconds) toward
// `targetWX, targetWZ`. Returns true if the cat is within `arriveDist`
// metres of the target after the step — caller treats that as arrival.
//
// Movement is straight-line: cats don't pathfind. They drive over
// whatever's between them and the next route cell, including tree
// cells. That's a deliberate simplification for the first pass; if
// it produces obvious "cat ran through a stand of trees" visuals we
// can layer pathfinding on top later.
func (c *Snowcat) DriveToward(targetWX, targetWZ float32, dt float64, arriveDist float32) bool {
	dx := targetWX - c.Pos[0]
	dz := targetWZ - c.Pos[2]
	dist := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if dist <= arriveDist {
		return true
	}
	step := float32(SnowcatSpeed * dt)
	if step > dist {
		step = dist
	}
	c.Pos[0] += dx / dist * step
	c.Pos[2] += dz / dist * step
	c.Heading = float32(math.Atan2(float64(dx), float64(dz)))
	return dist-step <= arriveDist
}

// CatWorking reports whether the cat is out on the hill — on a grooming
// pass or driving to or from its shed — rather than parked at the door.
func (w *World) CatWorking(c *Snowcat) bool {
	if len(c.Route) > 0 {
		return true
	}
	for _, b := range w.Buildings {
		if b.ID != c.ShedID {
			continue
		}
		home := w.SnowcatParkPos(b)
		dx, dz := home[0]-c.Pos[0], home[2]-c.Pos[2]
		return dx*dx+dz*dz > CellSize*CellSize
	}
	return false
}

// CatsOwnedBy returns the snowcats whose ShedID matches `shedID`.
// Allocates a fresh slice; not on a hot path.
func (w *World) CatsOwnedBy(shedID uint64) []*Snowcat {
	var out []*Snowcat
	for _, c := range w.Snowcats {
		if c.ShedID == shedID {
			out = append(out, c)
		}
	}
	return out
}

// SpawnSnowcat creates a new cat at the given shed's door cell and
// appends it to the world. The cat starts with no target so the sim's
// first tick assigns one (or parks it if the route is empty).
func (w *World) SpawnSnowcat(shed *Building) *Snowcat {
	cat := &Snowcat{
		ID:     w.NextID(),
		ShedID: shed.ID,
		Pos:    w.SnowcatParkPos(shed),
	}
	w.Snowcats = append(w.Snowcats, cat)
	return cat
}

// SnowcatParkPos is where a cat sits when parked at home: just outside
// its building's garage door, on the snow surface.
func (w *World) SnowcatParkPos(shed *Building) mgl32.Vec3 {
	return w.ServiceHome(shed, ServiceGarage)
}

// RemoveSnowcat drops the cat with the given ID. Used when a shed
// downsizes its fleet or is demolished.
func (w *World) RemoveSnowcat(id uint64) {
	for i, c := range w.Snowcats {
		if c.ID == id {
			w.Snowcats = append(w.Snowcats[:i], w.Snowcats[i+1:]...)
			return
		}
	}
}

// RemoveSnowcatsOwnedBy drops every cat owned by the given shed. Called
// when the shed is demolished so orphaned cats don't keep driving to a
// non-existent home.
func (w *World) RemoveSnowcatsOwnedBy(shedID uint64) {
	out := w.Snowcats[:0]
	for _, c := range w.Snowcats {
		if c.ShedID != shedID {
			out = append(out, c)
		}
	}
	w.Snowcats = out
}
