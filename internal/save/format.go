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
	Cash      int            `json:"cash,omitempty"`
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
	Flat        float32 `json:"f,omitempty"`
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
// the IDs of the agents currently riding it.
type ChairData struct {
	Progress     float32   `json:"p"`
	PassengerIDs [2]uint64 `json:"pax,omitempty"` // 0 = empty slot
}

// LiftData is a placed lift, including its full runtime state (chair
// positions and passenger references, queue order) so a save round-trips
// without freezing skiers in mid-air. Base/Top are continuous world XZ
// (metres).
type LiftData struct {
	ID          uint64      `json:"id,omitempty"`
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
