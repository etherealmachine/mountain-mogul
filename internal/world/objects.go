package world

// ObjectType represents the type of a placed natural object.
type ObjectType uint8

const (
	ObjTree  ObjectType = iota
	ObjRock
	ObjStump
)

// MeshID constants mirror render.Mesh* constants to avoid circular imports.
const (
	MeshTree       uint32 = 0
	MeshTree2      uint32 = 1
	MeshTree3      uint32 = 2
	MeshRock       uint32 = 3
	MeshStump      uint32 = 4
	MeshBuilding   uint32 = 5
	MeshTower      uint32 = 6
	MeshSkier      uint32 = 7
	MeshChair      uint32 = 9
	MeshShed        uint32 = 10
	MeshParkingPad  uint32 = 12
	MeshRoadConnect uint32 = 14
	MeshRoadNode    uint32 = 15
	MeshChairQuad   uint32 = 16 // 4-seat fixed grip chair
	MeshChair6Pack  uint32 = 17 // 6-seat high-speed detachable chair
	MeshGondolaCabin uint32 = 18 // MDG gondola cabin (8-person enclosed)
	MeshWalker       uint32 = 19 // guest with skis off (building footprints, bare ground)
	MeshHelipad      uint32 = 20 // flat pad with H marking at heli-ski base and drop zone
	MeshHelicopter   uint32 = 21 // heli-ski helicopter (dynamic — one per HeliLift)
	MeshSnowGun      uint32 = 22 // snowmaking cannon on a tripod
	MeshPatrolHut    uint32 = MeshShed // patrol hut reuses shed mesh
)

// MeshSlot is an anchor point baked into a mesh by the SCAD pipeline
// — e.g. a chairlift seat position. The coordinate is in the mesh's
// local frame (game-space, post the scad2obj Z-up→Y-up rotation,
// before any per-instance heading rotation the renderer applies).
type MeshSlot struct {
	Index int
	Pos   [3]float32
}

var meshSlots = map[uint32][]MeshSlot{}

// RegisterMeshSlots records slot metadata for a mesh. Called by the
// renderer when it loads OBJs so simulation code can query slots by
// mesh ID without taking a dependency on the render package.
func RegisterMeshSlots(meshID uint32, slots []MeshSlot) {
	meshSlots[meshID] = slots
}

// SlotsFor returns the slots registered for meshID, or nil if the mesh
// did not declare any in its .scad source.
func SlotsFor(meshID uint32) []MeshSlot {
	return meshSlots[meshID]
}

// MeshFootprint is the building's ground-plane half-extents (in metres,
// game coords), baked into the OBJ from the .scad source. The placement-
// effects pass uses this to size the apron and the tree-clearance zone so
// per-type dimensions don't have to be duplicated in Go.
type MeshFootprint struct {
	HalfX, HalfZ float32
}

var meshFootprints = map[uint32]MeshFootprint{}

// RegisterMeshFootprint records the ground-plane half-extents of a mesh.
// Called by the renderer when it loads each building OBJ.
func RegisterMeshFootprint(meshID uint32, fp MeshFootprint) {
	meshFootprints[meshID] = fp
}

// FootprintFor returns the footprint registered for meshID, or
// (zero, false) if the mesh did not declare one in its .scad source.
func FootprintFor(meshID uint32) (MeshFootprint, bool) {
	fp, ok := meshFootprints[meshID]
	return fp, ok
}

// MeshID returns the renderer mesh ID for a building type. Centralised
// so scene-side helpers (footprint lookup, placement effects) can resolve
// the mesh without depending on the render package.
func (t BuildingType) MeshID() uint32 {
	switch t {
	case BuildingShed:
		return MeshShed
	case BuildingPatrolHut:
		return MeshPatrolHut
	case BuildingParking:
		return MeshParkingPad
	case BuildingSnowGun:
		return MeshSnowGun
	case BuildingLodge:
		fallthrough
	default:
		return MeshBuilding
	}
}

// MeshID maps an ObjectType to its mesh ID constant.
func (t ObjectType) MeshID() uint32 {
	switch t {
	case ObjTree:
		return MeshTree
	case ObjRock:
		return MeshRock
	case ObjStump:
		return MeshStump
	}
	return MeshTree
}

// PlacedObject is a static natural object placed on the terrain.
type PlacedObject struct {
	ID       uint64
	Type     ObjectType
	Pos      [2]int
	Rotation float32
}
