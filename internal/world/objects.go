package world

// ObjectType represents the type of a placed natural object.
type ObjectType uint8

const (
	ObjTree ObjectType = iota
	ObjRock
	ObjStump
)

// MeshID constants mirror render.Mesh* constants to avoid circular imports.
const (
	MeshTree         uint32 = 0
	MeshTree2        uint32 = 1
	MeshTree3        uint32 = 2
	MeshRock         uint32 = 3
	MeshStump        uint32 = 4
	MeshBuilding     uint32 = 5
	MeshTower        uint32 = 6
	MeshSkier        uint32 = 7
	MeshChair        uint32 = 9
	MeshShed         uint32 = 10
	MeshParkingPad   uint32 = 12
	MeshRoadConnect  uint32 = 14
	MeshRoadNode     uint32 = 15
	MeshChairQuad    uint32 = 16       // 4-seat fixed grip chair
	MeshChair6Pack   uint32 = 17       // 6-seat high-speed detachable chair
	MeshGondolaCabin uint32 = 18       // MDG gondola cabin (8-person enclosed)
	MeshWalker       uint32 = 19       // guest with skis off (building footprints, bare ground)
	MeshHelipad      uint32 = 20       // flat pad with H marking at heli-ski base and drop zone
	MeshHelicopter   uint32 = 21       // heli-ski helicopter (dynamic — one per HeliLift)
	MeshSnowGun      uint32 = 22       // snowmaking cannon on a tripod
	MeshPatrolHut    uint32 = MeshShed // patrol hut reuses shed mesh
	MeshBar          uint32 = 23       // bar/restaurant (half-size lodge mesh)
	MeshTicketOffice uint32 = 24       // ticket booth with service windows
	MeshChairTriple  uint32 = 25       // 3-seat fixed grip chair
	MeshSkiRack      uint32 = 26       // A-frame ski rack
	MeshBeltPlace    uint32 = 30       // a rider's place on a magic carpet's belt or a rope tow's rope: nothing drawn, no seat slots
	MeshTBarHanger   uint32 = 35       // a T-bar's hanger, let down to two riders (tbar_hanger.py; its slots are their feet)
	MeshTramCabin    uint32 = 200      // an aerial tram's cabin (tram_cabin.py), clear of the shell kits' IDs from 40
	MeshChairSingle  uint32 = 203      // 1-seat fixed grip chair
	MeshChair8Pack   uint32 = 204      // 8-seat high-speed detachable chair
	MeshFunitelCabin uint32 = 205      // funitel cabin (funitel_cabin.py)
	MeshTriCabin     uint32 = 206      // tricable gondola cabin (tri_cabin.py)
	MeshFunicularCar uint32 = 216      // a funicular's car (funicular_car.py)
)

// MeshSlot is an anchor point baked into a mesh by the model pipeline
// — e.g. a chairlift seat position. The coordinate is in the mesh's
// local frame (game-space, post the model writer's Z-up→Y-up rotation,
// before any per-instance heading rotation the renderer applies).
type MeshSlot struct {
	Index int
	Pos   [3]float32
	// Pose is how a rider is there: on a chair's seat, or in a cabin (no
	// skis: they're in its racks) on a bench or standing. Back is a cabin
	// rider facing the back of the cabin.
	Pose SlotPose
	Back bool
}

// SlotPose is how a rider is at a mesh slot.
type SlotPose uint8

const (
	SlotChair SlotPose = iota // on a chair's seat, skis on, feet on the footrest
	SlotSit                   // on a cabin's bench, feet on the floor
	SlotStand                 // standing on a cabin's floor
)

// RiderSlot is the slot g rides in on lift l, if they're riding it and
// its mesh has slots.
func RiderSlot(l *Lift, g *Guest) (MeshSlot, bool) {
	slots := SlotsFor(l.Type.MeshID())
	if g.OnLiftID != l.ID || g.LiftSeat < 0 || g.LiftSeat >= len(slots) {
		return MeshSlot{}, false
	}
	return slots[g.LiftSeat], true
}

var meshSlots = map[uint32][]MeshSlot{}

// RegisterMeshSlots records slot metadata for a mesh. Called by the
// renderer when it loads OBJs so simulation code can query slots by
// mesh ID without taking a dependency on the render package.
func RegisterMeshSlots(meshID uint32, slots []MeshSlot) {
	meshSlots[meshID] = slots
}

// SlotsFor returns the slots registered for meshID, or nil if the mesh
// did not declare any in its model source.
func SlotsFor(meshID uint32) []MeshSlot {
	return meshSlots[meshID]
}

// MeshFootprint is the building's ground-plane half-extents (in metres,
// game coords), baked into the OBJ from the model source. The placement-
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
// (zero, false) if the mesh did not declare one in its model source.
func FootprintFor(meshID uint32) (MeshFootprint, bool) {
	fp, ok := meshFootprints[meshID]
	return fp, ok
}

// MeshID returns the renderer mesh ID for a building type. Centralised
// so scene-side helpers (footprint lookup, placement effects) can resolve
// the mesh without depending on the render package.
func (t BuildingType) MeshID() uint32 {
	switch t {
	case BuildingParking:
		return MeshParkingPad
	case BuildingSkiRack:
		return MeshSkiRack
	case BuildingBar:
		return MeshBar
	case BuildingTicketOffice:
		return MeshTicketOffice
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
