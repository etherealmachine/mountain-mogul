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

	// MaxCatsPerShed caps how many cats a single shed can dispatch.
	// Sized to the shed model (two bays + circulation = three cats max).
	MaxCatsPerShed = 3

	// RouteCoverageSec is the target sim-time for one cat to complete a
	// full circuit of its painted route at SnowcatSpeed. Used to derive
	// the per-shed paintable area cap so routes stay in scale with the
	// fleet that has to service them.
	//
	// 30 sim-minutes is roughly a real groomer's shift on a moderate
	// trail; with the 5 m tiller that gives each cat 1500 cells of
	// capacity = a strip about 7 500 m long × 5 m wide, or a small
	// run-sized region. A three-cat shed can hold a 4 500-cell route
	// — most of a small resort.
	RouteCoverageSec = 1875.0 // ~31 sim-minutes

	// CatCost is the up-front purchase price for each cat beyond the
	// first (the first is included in ShedCost).
	CatCost = 15000

	// CellSize is one terrain cell in metres. Mirrored from a few places
	// so snowcat helpers don't pull it from a constants package.
	CellSize = 5.0
)

// Snowcat is a single grooming machine. Spawned in pairs with the
// shed that owns it; lives in World.Snowcats with a persistent ID so
// saves round-trip.
type Snowcat struct {
	ID      uint64
	ShedID  uint64 // owning shed; cat despawns when shed is removed
	Pos     mgl32.Vec3
	Heading float32

	// TargetCell is the cell the cat is currently driving toward. The
	// "no target" sentinel is [-1, -1]; in that state the cat is parked
	// at its shed door waiting for a new pick.
	TargetCell [2]int
}

// MaxRouteCells returns how many cells a shed with `numCats` cats can
// paint, given the design rule that one cat should be able to complete
// a full circuit in roughly RouteCoverageSec sim-seconds. Each cat
// grooms at SnowcatSpeed × SnowcatTillerWidth m²/s, so:
//
//	area_cap_m² = numCats × SnowcatSpeed × SnowcatTillerWidth × RouteCoverageSec
//	cells       = area_cap_m² / (CellSize × CellSize)
//
// At default tuning that's 1500 cells per cat (37 500 m² ≈ a
// run-sized region per cat, ~31 sim-minutes per circuit). A three-cat
// shed holds 4500 cells — most of a small resort.
func MaxRouteCells(numCats int) int {
	if numCats <= 0 {
		return 0
	}
	area := float64(numCats) * SnowcatSpeed * SnowcatTillerWidth * RouteCoverageSec
	return int(area / (CellSize * CellSize))
}

// NoCellTarget is the sentinel for "no current target cell" on a Snowcat.
var NoCellTarget = [2]int{-1, -1}

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
		ID:         w.NextID(),
		ShedID:     shed.ID,
		Pos:        mgl32.Vec3{cx, cy, cz},
		TargetCell: NoCellTarget,
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
