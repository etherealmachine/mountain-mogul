package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// BuildingType selects what a Building represents and which mesh
// renders it. New types added here also need:
//   - a mesh ID in world/objects.go (mirrored in render/mesh.go)
//   - a .scad source in models-src/
//   - a cost constant + BuildingCost case in world.go
//   - a toolbar button in scene/scenario.go
//   - a case in openBuildingPopup in scene/scenario.go (panics if missed)
type BuildingType uint8

const (
	BuildingLodge        BuildingType = 0
	BuildingShed         BuildingType = 1
	BuildingParking      BuildingType = 2
	BuildingPatrolHut    BuildingType = 3
	BuildingSnowGun      BuildingType = 4
	BuildingTicketOffice BuildingType = 5
	BuildingBar          BuildingType = 6 // bar/restaurant — relieve thirst/hunger
)

// Building represents a structure placed on the terrain. Lodges are
// reserved for future rest/lunch buildings. Sheds garage snowcat /
// snowmobile equipment. Parking lots hold a visible car population
// (CurrentCars, capped by MaxCars) — the demand system writes
// CurrentCars; the lot itself carries no spawn machinery.
//
// Pos is the building's anchor in continuous world XZ coordinates (metres).
// Y is derived from terrain elevation at use time. Rotation turns the
// mesh and its footprint about Y (FootprintRect); passability is still
// just the door cell under the anchor.
type Building struct {
	ID       uint64
	Type     BuildingType
	Pos      mgl32.Vec2
	Rotation float32 // radians about +Y, the renderer's HomogRotate3DY convention; ignored by painted lots

	// Parking-only state. Cells is the painted footprint (sorted x, z);
	// Stalls and MaxCars are derived from it by RefreshParkingLot.
	// CurrentCars is the visible count the renderer floors to instance
	// N cars on the first N stalls; the demand system drives it.
	// DrivewayNodeIDs holds the road-graph attach points on the lot edge.
	// New lots get one; saves from the fixed-pad era may carry more.
	// Empty on non-parking buildings and on lots that haven't had
	// EnsureParkingDriveway called yet.
	Cells           [][2]int
	Stalls          []ParkingStall
	MaxCars         int
	CurrentCars     float32
	DrivewayNodeIDs []uint64
	cellSet         map[[2]int]struct{}

	// SnowGun-only state. Enabled defaults to true on placement; the player
	// can toggle it off from the popup to stop snow production and operating costs.
	SnowGunEnabled bool

	// Lodge-only state (see lodge.go). Cells is the painted shell;
	// DoorCells are perimeter shell cells guests enter by (Pos sits on
	// the first); FoodCourtCells are shell cells fitted out as a food
	// court. StyleSeed picks facade variants and the palette. Diners is
	// the sim's live count of guests eating here (not saved).
	DoorCells      [][2]int
	FoodCourtCells [][2]int
	StyleSeed      uint32
	MealPrice      int
	Diners         int
}

// DoorCell returns the grid cell containing the building's anchor — the
// pathfinder destination for skiers walking to this lodge. Floor (not
// round) so a Pos exactly on a cell boundary lands in the cell whose
// indices match its floor coordinates, consistent with how skiers map
// their own continuous Pos to a cell elsewhere in the sim.
func (b *Building) DoorCell() [2]int {
	return cellOf(b.Pos)
}

// FootprintRect is a building's oriented ground rectangle: half-extents
// along the mesh's local X and Z, turned by Rotation about Y.
type FootprintRect struct {
	Center       mgl32.Vec2
	HalfX, HalfZ float32
	Rotation     float32
}

// Axes returns the world XZ directions of the mesh-local X and Z axes,
// matching the renderer's HomogRotate3DY: (1,0,0) → (cos, −sin) and
// (0,0,1) → (sin, cos).
func (r FootprintRect) Axes() (ax, az mgl32.Vec2) {
	cos := float32(math.Cos(float64(r.Rotation)))
	sin := float32(math.Sin(float64(r.Rotation)))
	return mgl32.Vec2{cos, -sin}, mgl32.Vec2{sin, cos}
}

// Local returns world point p in the rectangle's frame: offsets from
// Center along the local X and Z axes.
func (r FootprintRect) Local(p mgl32.Vec2) (lx, lz float32) {
	ax, az := r.Axes()
	d := p.Sub(r.Center)
	return d.Dot(ax), d.Dot(az)
}

// Contains reports whether world point p lies inside the rectangle grown
// by margin metres on every side.
func (r FootprintRect) Contains(p mgl32.Vec2, margin float32) bool {
	lx, lz := r.Local(p)
	return abs32(lx) <= r.HalfX+margin && abs32(lz) <= r.HalfZ+margin
}

// Bounds returns the world-XZ bounding box of the rectangle.
func (r FootprintRect) Bounds() (minX, minZ, maxX, maxZ float32) {
	ax, az := r.Axes()
	ex := r.HalfX*abs32(ax[0]) + r.HalfZ*abs32(az[0])
	ez := r.HalfX*abs32(ax[1]) + r.HalfZ*abs32(az[1])
	return r.Center[0] - ex, r.Center[1] - ez, r.Center[0] + ex, r.Center[1] + ez
}

// Overlaps reports whether two rectangles' interiors intersect
// (separating-axis test); rectangles that only touch don't overlap.
func (r FootprintRect) Overlaps(o FootprintRect) bool {
	const eps = 1e-3 // absorbs trig round-off so flush neighbours don't collide
	rx, rz := r.Axes()
	ox, oz := o.Axes()
	d := o.Center.Sub(r.Center)
	for _, a := range [4]mgl32.Vec2{rx, rz, ox, oz} {
		rr := r.HalfX*abs32(rx.Dot(a)) + r.HalfZ*abs32(rz.Dot(a))
		ro := o.HalfX*abs32(ox.Dot(a)) + o.HalfZ*abs32(oz.Dot(a))
		if abs32(d.Dot(a)) >= rr+ro-eps {
			return false
		}
	}
	return true
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// BuildingFootprint returns the footprint of a building of typ anchored
// at (x, z) with the given rotation. Zero extents when no footprint is
// registered for the type's mesh.
func BuildingFootprint(typ BuildingType, x, z, rotation float32) FootprintRect {
	r := FootprintRect{Center: mgl32.Vec2{x, z}, Rotation: rotation}
	if fp, ok := FootprintFor(typ.MeshID()); ok {
		r.HalfX, r.HalfZ = fp.HalfX, fp.HalfZ
	}
	return r
}

// Footprint returns b's oriented footprint rectangle.
func (b *Building) Footprint() FootprintRect {
	return BuildingFootprint(b.Type, b.Pos[0], b.Pos[1], b.Rotation)
}

// BuildingOverlap reports whether a building of typ anchored at (x, z)
// with the given rotation would overlap any existing building.
func (w *World) BuildingOverlap(typ BuildingType, x, z, rotation float32) bool {
	return w.BuildingOverlapExcept(typ, x, z, rotation, 0)
}

// BuildingOverlapExcept is BuildingOverlap with one building excluded
// from the collision check by ID. Used by the move and rotate edits to
// ask "would this overlap any building OTHER than the one I'm editing?"
// Passing exceptID == 0 reproduces BuildingOverlap exactly.
func (w *World) BuildingOverlapExcept(typ BuildingType, x, z, rotation float32, exceptID uint64) bool {
	fp := BuildingFootprint(typ, x, z, rotation)
	for _, b := range w.Buildings {
		if b.ID == exceptID {
			continue
		}
		if b.IsPainted() {
			if b.cellsOverlapRect(fp) {
				return true
			}
			continue
		}
		if fp.Overlaps(b.Footprint()) {
			return true
		}
	}
	return false
}

// DrivewayPositions returns the world XZ positions of a parking lot's
// driveway attach points. Painted lots have a single entrance on the
// lot edge (see parkingEntrance). Legacy mesh-pad lots use one point per
// MOGUL_META slot declared by the parking mesh, rotated by the
// building's Y rotation. Returns nil for non-parking buildings.
func (w *World) DrivewayPositions(b *Building) []mgl32.Vec2 {
	if b.Type != BuildingParking {
		return nil
	}
	if b.IsCellLot() {
		return []mgl32.Vec2{w.parkingEntrance(b)}
	}
	slots := SlotsFor(b.Type.MeshID())
	if len(slots) == 0 {
		return nil
	}
	cos := float32(math.Cos(float64(b.Rotation)))
	sin := float32(math.Sin(float64(b.Rotation)))
	out := make([]mgl32.Vec2, len(slots))
	for i, s := range slots {
		// Mesh-local (X, _, Z) → world delta rotated around Y, matching
		// the renderer's HomogRotate3DY convention: (1,0,0) →
		// (cos, 0, -sin) and (0,0,1) → (sin, 0, cos).
		mx, mz := s.Pos[0], s.Pos[2]
		out[i] = mgl32.Vec2{
			b.Pos[0] + mx*cos + mz*sin,
			b.Pos[1] - mx*sin + mz*cos,
		}
	}
	return out
}

// EnsureParkingDriveway creates the road-network attach nodes for a
// parking lot. A painted lot that already has a live driveway node keeps
// it (RefreshParkingLot keeps it on the lot edge); otherwise one node is
// created per DrivewayPositions entry, reusing any stored ID that still
// resolves to a live RoadNode so DrivewayNodeIDs[i] matches position i.
//
// Idempotent — safe to call multiple times. No-op for non-parking
// buildings; no-op for legacy lots without registered mesh slots (which
// keeps the placement path from crashing during a missing-asset run).
func (w *World) EnsureParkingDriveway(b *Building) {
	if b == nil || b.Type != BuildingParking {
		return
	}
	if b.IsCellLot() {
		for _, id := range b.DrivewayNodeIDs {
			if w.RoadNodeByID(id) != nil {
				return
			}
		}
		b.DrivewayNodeIDs = b.DrivewayNodeIDs[:0]
	}
	positions := w.DrivewayPositions(b)
	if len(positions) == 0 {
		return
	}
	// Grow the ID slice to match the slot count; values default to 0,
	// which the existence check below treats as "needs a fresh node".
	for len(b.DrivewayNodeIDs) < len(positions) {
		b.DrivewayNodeIDs = append(b.DrivewayNodeIDs, 0)
	}
	for i, pos := range positions {
		id := b.DrivewayNodeIDs[i]
		if id != 0 && w.RoadNodeByID(id) != nil {
			continue
		}
		n := w.AddRoadNode(pos, RoadNodeParkingDriveway)
		b.DrivewayNodeIDs[i] = n.ID
	}
	if b.IsCellLot() {
		sortStallsFrom(b.Stalls, positions[0])
	}
}

// RemoveRoadNode deletes a road node and every edge incident to it.
// Used by parking-lot teardown to clean up the driveway and whatever
// the player attached to it; safe on a never-existed ID (no-op).
func (w *World) RemoveRoadNode(id uint64) {
	if id == 0 {
		return
	}
	// Drop incident edges first so the node's removal doesn't leave
	// dangling references in RoadEdges.
	filteredEdges := w.RoadEdges[:0]
	for _, e := range w.RoadEdges {
		if e.A == id || e.B == id {
			continue
		}
		filteredEdges = append(filteredEdges, e)
	}
	w.RoadEdges = filteredEdges
	for i, n := range w.RoadNodes {
		if n.ID == id {
			w.RoadNodes = append(w.RoadNodes[:i], w.RoadNodes[i+1:]...)
			return
		}
	}
}

// Label returns a short human-readable name for HUD / event-feed display.
func (t BuildingType) Label() string {
	switch t {
	case BuildingShed:
		return "Equipment Shed"
	case BuildingParking:
		return "Parking Lot"
	case BuildingPatrolHut:
		return "Patrol Hut"
	case BuildingSnowGun:
		return "Snow Gun"
	case BuildingTicketOffice:
		return "Ticket Office"
	case BuildingBar:
		return "Bar"
	}
	return "Lodge"
}
