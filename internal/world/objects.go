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
)

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
