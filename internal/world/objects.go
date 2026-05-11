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
	MeshTree     uint32 = 0
	MeshTree2    uint32 = 1
	MeshTree3    uint32 = 2
	MeshRock     uint32 = 3
	MeshStump    uint32 = 4
	MeshBuilding uint32 = 5
	MeshTower    uint32 = 6
	MeshAgent    uint32 = 7
	MeshChair    uint32 = 9
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
