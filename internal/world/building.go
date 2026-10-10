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
	BuildingLodge        BuildingType = 0 // a service building (lodge, tent, or shed: see ShellKind)
	BuildingShed         BuildingType = 1 // retired: the equipment shed, now a garage service; kept so saved numbers hold
	BuildingParking      BuildingType = 2
	BuildingPatrolHut    BuildingType = 3 // retired: the patrol hut, now a patrol service
	BuildingSnowGun      BuildingType = 4
	BuildingTicketOffice BuildingType = 5
	BuildingBar          BuildingType = 6 // bar/restaurant — relieve thirst/hunger
	BuildingSkiRack      BuildingType = 7 // a rack for skis by the doors (ski_rack.go)
)

// Building represents a structure placed on the terrain. Lodges are
// reserved for future rest/lunch buildings. Sheds garage snowcat /
// snowmobile equipment. Parking lots hold up to MaxCars parked cars
// (World.Cars parked in the lot's stalls).
//
// Pos is the building's anchor in continuous world XZ coordinates (metres).
// Y is derived from terrain elevation at use time. Rotation turns the
// mesh and its footprint about Y (FootprintRect); passability is still
// just the door cell under the anchor.
type Building struct {
	ID       uint64
	Type     BuildingType
	Pos      mgl32.Vec2
	Rotation float32 // radians about +Y, the renderer's HomogRotate3DY convention

	// Parking-only state (parking.go). LotSize is the lot rectangle's
	// extent along its local X and Z (Pos is its centre, Rotation its
	// turn). Ground (the cells under it, sorted x, z), Gate (the entrance
	// on its edge), Stalls and MaxCars are derived by RefreshParkingLot.
	// DrivewayNodeIDs[0] is the entrance's road node, and DriveEdge the
	// driveway ConnectLotDriveway built from it to the nearest road (0
	// for none).
	LotSize         mgl32.Vec2
	Gate            mgl32.Vec2
	Stalls          []ParkingStall
	MaxCars         int
	DrivewayNodeIDs []uint64
	DriveEdge       uint64
	lotAisles       []float32 // local V of each aisle, from layoutLot

	// Cells is a service building's shell in its own grid (see below).
	// Ground is the map cells under a lot or service building, derived
	// from its rectangle or tiles: what grading, plowing, walking and
	// overlap checks read.
	Cells     [][2]int
	cellSet   map[[2]int]struct{}
	Ground    [][2]int
	groundSet map[[2]int]struct{}
	// groundLo, groundHi bound Ground (cells, inclusive), for a quick
	// reject before the per-cell lookups (FootprintContains).
	groundLo, groundHi [2]int
	// tileCounts is TileCount for each service, kept by SetTiles; zero
	// tileCountsOK (a building built without SetTiles) counts afresh.
	tileCounts   [ServiceCount]int
	tileCountsOK bool

	// SnowGun-only state. Enabled defaults to true on placement; the player
	// can toggle it off from the popup to stop snow production and operating costs.
	SnowGunEnabled bool

	// Service-building state (see lodge.go). A service building has its
	// own 5 m grid: Origin is the world XZ of its cell (0, 0)'s corner and
	// Rotation turns the grid (FootprintRect's convention). Cells is the
	// tiled shell in that grid and Tiles each cell's service. Doors are
	// derived by RefreshDoors (Pos sits on the first). FloorY is the floor
	// height, fixed by the first tile so the building doesn't shift as it
	// grows. StyleSeed picks facade variants and the palette. Diners was
	// the sim's live count of guests eating here (not saved).
	// Kind is what it's built as and Storeys how many storeys it has (a
	// lodge can have up to three; 0 counts as one, see Floors).
	Origin     mgl32.Vec2
	Kind       ShellKind
	Storeys    int
	Tiles      map[[2]int]Service
	Doors      []Door
	FloorY     float32
	FloorSet   bool
	StyleSeed  uint32
	MealPrice  int
	DrinkPrice int
	// FreeWater offers free water where drinks are served (ai.OfferWater):
	// guests help themselves, with no turn at the counter, and it
	// quenches thirst and does nothing else.
	FreeWater   bool
	RentalPrice int
	// Quality is how good the building is to visit, 0..1: it raises what
	// guests will pay and scores each visit (Service Improvements).
	// DefaultQuality until there's a way to raise it.
	Quality float32
	// InUse and Waiting count guests using each pool (seats, counter)
	// and lined up at the door for it; recounted every tick by the sim.
	InUse, Waiting [PoolCount]int

	doorSteps doorStepCache // see cacheDoorSteps
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
	if b.IsRectLot() {
		return b.LotRect()
	}
	return BuildingFootprint(b.Type, b.Pos[0], b.Pos[1], b.Rotation)
}

// BuildingByID returns the building with id, or nil.
func (w *World) BuildingByID(id uint64) *Building {
	if id == 0 {
		return nil
	}
	for _, b := range w.Buildings {
		if b.ID == id {
			return b
		}
	}
	return nil
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

// DrivewayPositions returns the world XZ position of a parking lot's
// entrance (Gate, on the lot's edge). Returns nil for non-parking
// buildings.
func (w *World) DrivewayPositions(b *Building) []mgl32.Vec2 {
	if b.Type != BuildingParking {
		return nil
	}
	return []mgl32.Vec2{b.Gate}
}

// EnsureParkingDriveway gives a parking lot its entrance node: a lot
// that already has a live one keeps it, otherwise one is created at the
// lot's Gate. Idempotent; no-op for non-parking buildings.
func (w *World) EnsureParkingDriveway(b *Building) {
	if b == nil || b.Type != BuildingParking {
		return
	}
	var live []uint64
	for _, id := range b.DrivewayNodeIDs {
		if w.RoadNodeByID(id) != nil {
			live = append(live, id)
		}
	}
	b.DrivewayNodeIDs = live
	if len(live) == 0 {
		b.DrivewayNodeIDs = []uint64{w.AddRoadNode(b.Gate, RoadNodeParkingDriveway).ID}
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

// Label names b for the player: a service building by its one service,
// or "Lodge" when it mixes them; anything else by its type.
func (b *Building) Label() string {
	if !b.IsShell() {
		return b.Type.Label()
	}
	only := ServiceNone
	for sv := ServiceLounge; sv < ServiceCount; sv++ {
		if b.TileCount(sv) == 0 {
			continue
		}
		if only != ServiceNone {
			return b.Kind.Label()
		}
		only = sv
	}
	if only == ServiceNone || only == ServiceLounge {
		return b.Kind.Label()
	}
	return only.Label() + " " + b.Kind.Label()
}

// Label returns a short human-readable name for HUD / event-feed display.
func (t BuildingType) Label() string {
	switch t {
	case BuildingParking:
		return "Parking Lot"
	case BuildingSnowGun:
		return "Snow Gun"
	case BuildingTicketOffice:
		return "Ticket Office"
	case BuildingBar:
		return "Bar"
	case BuildingSkiRack:
		return "Ski Rack"
	}
	return "Lodge"
}
