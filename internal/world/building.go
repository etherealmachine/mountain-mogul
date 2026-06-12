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
// Y is derived from terrain elevation at use time. Footprints are still
// effectively a single cell for passability rasterisation; oriented-AABB
// footprints are a future extension.
type Building struct {
	ID       uint64
	Type     BuildingType
	Pos      mgl32.Vec2
	Rotation float32

	// Parking-only state. MaxCars caps how many cars the lot can hold;
	// CurrentCars is the visible count the renderer floors to instance
	// N car models in a grid pattern. The demand system (future) drives
	// CurrentCars; the lot itself is passive.
	// DrivewayNodeIDs holds the road-graph attach points auto-created
	// from the parking mesh's MOGUL_META slot table — one node per slot,
	// in slot-index order. Empty on non-parking buildings and on parking
	// lots that haven't had EnsureParkingDriveway called yet.
	MaxCars         int
	CurrentCars     float32
	DrivewayNodeIDs []uint64

	// SnowGun-only state. Enabled defaults to true on placement; the player
	// can toggle it off from the popup to stop snow production and operating costs.
	SnowGunEnabled bool
}

// DoorCell returns the grid cell containing the building's anchor — the
// pathfinder destination for skiers walking to this lodge. Floor (not
// round) so a Pos exactly on a cell boundary lands in the cell whose
// indices match its floor coordinates, consistent with how skiers map
// their own continuous Pos to a cell elsewhere in the sim.
func (b *Building) DoorCell() [2]int {
	return cellOf(b.Pos)
}

// FootprintAABB returns the axis-aligned ground bounding box of a building
// of typ centred at (x, z) in world XZ coords. Buildings are currently
// placed with Rotation=0 so the AABB is the mesh footprint at the anchor;
// once placement rotation lands this needs an OBB query.
// Returns a zero-extent box at (x, z) if no footprint is registered.
func FootprintAABB(typ BuildingType, x, z float32) (minX, minZ, maxX, maxZ float32) {
	fp, ok := FootprintFor(typ.MeshID())
	if !ok {
		return x, z, x, z
	}
	return x - fp.HalfX, z - fp.HalfZ, x + fp.HalfX, z + fp.HalfZ
}

// BuildingOverlap reports whether a building of typ centred at (x, z)
// would overlap any existing building's footprint AABB.
func (w *World) BuildingOverlap(typ BuildingType, x, z float32) bool {
	return w.BuildingOverlapExcept(typ, x, z, 0)
}

// BuildingOverlapExcept is BuildingOverlap with one building excluded
// from the collision check by ID. Used by the move tool to ask "would
// this position overlap any building OTHER than the one I'm dragging?"
// Passing exceptID == 0 reproduces BuildingOverlap exactly.
func (w *World) BuildingOverlapExcept(typ BuildingType, x, z float32, exceptID uint64) bool {
	minX, minZ, maxX, maxZ := FootprintAABB(typ, x, z)
	for _, b := range w.Buildings {
		if b.ID == exceptID {
			continue
		}
		bMinX, bMinZ, bMaxX, bMaxZ := FootprintAABB(b.Type, b.Pos[0], b.Pos[1])
		if maxX > bMinX && minX < bMaxX && maxZ > bMinZ && minZ < bMaxZ {
			return true
		}
	}
	return false
}

// DrivewayPositions returns the world XZ positions of a parking lot's
// driveway attach points — one per MOGUL_META slot declared by the
// parking mesh, in slot-index order. Each mesh-local slot position is
// rotated by the building's Y rotation before being added to the
// anchor. Returns nil for non-parking buildings or when no slots are
// registered (e.g. before the OBJ has been rebuilt).
func (b *Building) DrivewayPositions() []mgl32.Vec2 {
	if b.Type != BuildingParking {
		return nil
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
// parking lot — one per MOGUL_META slot from the parking mesh. Each
// slot's stored node ID is reused if it still resolves to a live
// RoadNode; missing ones get freshly created. Slot ordering is
// preserved so DrivewayNodeIDs[i] always corresponds to slot i.
//
// Idempotent — safe to call multiple times. No-op for non-parking
// buildings; no-op for parking lots without registered slots (which
// keeps the placement path from crashing during a missing-asset run).
func (w *World) EnsureParkingDriveway(b *Building) {
	if b == nil || b.Type != BuildingParking {
		return
	}
	positions := b.DrivewayPositions()
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
