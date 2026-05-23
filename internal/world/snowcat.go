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
	// lays corduroy, in metres. Real machines are 4–6 m; we pick one
	// cell width so a cat grooms exactly the cell it stands in.
	SnowcatTillerWidth = 5.0

	// CatPurchasePrice is the one-time cost to add a cat to the global fleet.
	// The first cat is bundled into ShedCost; every additional cat costs this.
	CatPurchasePrice = 25_000

	// CatActiveCostDay is the daily operating cost for a cat actively grooming.
	CatActiveCostDay = 500

	// CatStandbyCostDay is the daily cost to keep a cat parked in standby.
	CatStandbyCostDay = 75

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

// CatColumn is one (trail, x-column) pair in a cat's assigned section.
// Cats own a slice of these; the slice may span columns from multiple trails.
type CatColumn struct {
	TrailID uint64
	X       int
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

	// Section — the (trail, x-column) pairs this cat is responsible for.
	// Assigned globally by reassignAllSections; nil means unassigned.
	Section []CatColumn

	// Route — active pass through the section, copied from Section when a
	// grooming run starts. Nil means idle (no active route).
	Route    []CatColumn
	RouteIdx int  // current index in Route
	CellIdx  int  // current cell within Route[RouteIdx]'s column
	GoingDown bool // true = traversing column top→bottom

	// Transit — BFS path of trail cells to follow when moving between columns.
	// The cat drives these waypoints without grooming before starting the next
	// column. Nil means no active transit (groom or drive straight to shed).
	Transit    [][2]int
	TransitIdx int
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
	cell := shed.DoorCell()
	cx := (float32(cell[0]) + 0.5) * CellSize
	cz := (float32(cell[1]) + 0.5) * CellSize
	cy := w.Terrain.SurfaceElevationAt(cell[0], cell[1])
	cat := &Snowcat{
		ID:     w.NextID(),
		ShedID: shed.ID,
		Pos:    mgl32.Vec3{cx, cy, cz},
	}
	w.Snowcats = append(w.Snowcats, cat)
	return cat
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
