package save

// ScenarioData is the JSON-serialisable representation of a full scenario.
type ScenarioData struct {
	Name      string         `json:"name"`
	Width     int            `json:"width"`
	Height    int            `json:"height"`
	Cells     []CellData     `json:"cells"` // flat array, row-major (x-major)
	Objects   []ObjectData   `json:"objects"`
	Buildings []BuildingData `json:"buildings"`
	Lifts     []LiftData     `json:"lifts"`
	Agents    []AgentData    `json:"agents"`
	Snowcats  []SnowcatData  `json:"snowcats,omitempty"`
	RoadNodes []RoadNodeData `json:"road_nodes,omitempty"`
	RoadEdges []RoadEdgeData `json:"road_edges,omitempty"`
	Cash      int            `json:"cash,omitempty"`
	Camera    *CameraData    `json:"camera,omitempty"`
}

// RoadNodeData is one vertex in the road graph. ID is preserved across
// save/load so RoadEdgeData.A/B references stay valid.
type RoadNodeData struct {
	ID   uint64  `json:"id,omitempty"`
	X    float32 `json:"x"`
	Z    float32 `json:"z"`
	Kind uint8   `json:"k,omitempty"`
}

// RoadEdgeData is a straight road segment between two RoadNodes.
type RoadEdgeData struct {
	ID uint64 `json:"id,omitempty"`
	A  uint64 `json:"a"`
	B  uint64 `json:"b"`
}

// CameraData is the saved orthographic-camera state: where it's
// looking and how it's framed. First-person (perspective) state is
// excluded — it's tied to a followed skier and resets on load.
// Saving lets reloading a scenario or capturing a screenshot land on
// the same view the player left.
type CameraData struct {
	TargetX, TargetY, TargetZ float32
	Yaw                       float32
	Pitch                     float32
	OrthoScale                float32
}

// CellData is the serialisable representation of a terrain cell. Schema
// matches world.Cell; field names are short to keep the per-cell JSON
// footprint reasonable on big maps.
//
// JSON tags are stable across the elevation-contract rename — `e` holds
// ground elevation (formerly "elevation" before the ground/snow split),
// so old saves still load even though the Go field is now `Ground`.
type CellData struct {
	Ground      float32 `json:"e,omitempty"`  // ground elevation (rock/dirt)
	Snow        float32 `json:"s,omitempty"`  // snow depth
	Grooming    float32 `json:"gr,omitempty"`
	Packed      float32 `json:"pk,omitempty"`
	Ice         float32 `json:"ic,omitempty"`
	MogulSize   float32 `json:"mg,omitempty"`
	TreeDensity float32 `json:"td,omitempty"`

	// Natural-layer shadow — what the cell would be with no road /
	// building / lift on top. Persisted so structure moves can restore
	// the underlying terrain after a reload. HasNatural distinguishes
	// "present and zero-valued" from "absent in pre-natural-layer save";
	// when absent the loader falls back to copying display→natural.
	HasNatural   bool    `json:"nh,omitempty"`
	NaturalGround float32 `json:"ne,omitempty"`
	NaturalSnow  float32 `json:"ns,omitempty"`
	NaturalTrees float32 `json:"nt,omitempty"`
}

// ObjectData is a placed natural object.
type ObjectData struct {
	Type     uint8   `json:"t"`
	X        int     `json:"x"`
	Z        int     `json:"z"`
	Rotation float32 `json:"r,omitempty"`
}

// BuildingData is a placed building (lodge, shed, …). ID is preserved
// across save/load so agent.TargetID references stay valid. X/Z are
// continuous world XZ (metres); Y is reconstructed from terrain at
// load time. Type defaults to lodge when omitted so saves predating
// the multi-building work load as all-lodges.
type BuildingData struct {
	ID            uint64  `json:"id,omitempty"`
	Type          uint8   `json:"bt,omitempty"`
	X             float32 `json:"x"`
	Z             float32 `json:"z"`
	Rotation      float32 `json:"r,omitempty"`
	MeanSpawnRate float64 `json:"mean_spawn_rate"`
	SkierCount    int     `json:"skier_count,omitempty"`

	// Shed-only state.
	Cats       int        `json:"cats,omitempty"`
	RouteCells [][2]int   `json:"route,omitempty"`

	// Parking-only state.
	MaxCars         int      `json:"max_cars,omitempty"`
	CurrentCars     float32  `json:"cur_cars,omitempty"`
	DrivewayNodeIDs []uint64 `json:"driveway_ids,omitempty"` // road-network attach nodes, one per parking mesh slot
}

// SnowcatData is a saved cat. ShedID links it back to its shed; both
// IDs survive save/load so the cat → shed → route chain rehydrates
// correctly.
type SnowcatData struct {
	ID         uint64     `json:"id,omitempty"`
	ShedID     uint64     `json:"shed,omitempty"`
	Pos        [3]float32 `json:"pos"`
	Heading    float32    `json:"heading,omitempty"`
	TargetCell [2]int     `json:"tc,omitempty"`
}

// ChairData is one chair on a lift loop — its position around the loop and
// the IDs of the agents currently riding it. PassengerIDs is sized to the
// parent lift's per-chair capacity; 0 means "empty slot."
type ChairData struct {
	Progress     float32  `json:"p"`
	PassengerIDs []uint64 `json:"pax,omitempty"`
}

// LiftData is a placed lift, including its full runtime state (chair
// positions and passenger references, queue order) so a save round-trips
// without freezing skiers in mid-air. Base/Top are continuous world XZ
// (metres).
type LiftData struct {
	ID          uint64      `json:"id,omitempty"`
	Type        uint8       `json:"type,omitempty"`
	Name        string      `json:"name,omitempty"`
	Services    uint8       `json:"services,omitempty"` // TerrainDifficulty bitfield
	BaseX       float32     `json:"bx"`
	BaseZ       float32     `json:"bz"`
	TopX        float32     `json:"tx"`
	TopZ        float32     `json:"tz"`
	Speed       float32     `json:"speed,omitempty"`
	TicketPrice int         `json:"ticket,omitempty"`
	Chairs      []ChairData `json:"chairs,omitempty"`
	QueueIDs    []uint64    `json:"queue,omitempty"`
}

// AgentData is a saved agent state. ID is preserved so that lift chair /
// queue references resolve back to the same agent on load.
type AgentData struct {
	ID       uint64     `json:"id,omitempty"`
	Pos      [3]float32 `json:"pos"`
	Heading  float32    `json:"heading"`
	Path     [][2]int   `json:"path,omitempty"`
	PathIdx  int        `json:"path_idx,omitempty"`
	Speed    float32    `json:"speed,omitempty"`
	TargetID uint64     `json:"target_id,omitempty"`
	OnLiftID uint64     `json:"on_lift_id,omitempty"`
	Queued   bool       `json:"queued,omitempty"`
	Energy   float32    `json:"energy,omitempty"` // 0 on load (incl. saves predating this field) → defaulted to 1.0
}
