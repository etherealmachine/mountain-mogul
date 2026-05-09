package save

// ScenarioData is the JSON-serialisable representation of a full scenario.
type ScenarioData struct {
	Name      string         `json:"name"`
	Width     int            `json:"width"`
	Height    int            `json:"height"`
	Cells     []CellData     `json:"cells"`     // flat array, row-major (x-major)
	Objects   []ObjectData   `json:"objects"`
	Buildings []BuildingData `json:"buildings"`
	Lifts     []LiftData     `json:"lifts"`
	Agents    []AgentData    `json:"agents"`
	Cash      int            `json:"cash,omitempty"`
}

// CellData is the serialisable representation of a terrain cell.
type CellData struct {
	Elevation   float32 `json:"e,omitempty"`
	Groomed     bool    `json:"g,omitempty"`
	SnowDepth   float32 `json:"s,omitempty"`
	TreeDensity float32 `json:"td,omitempty"`
}

// ObjectData is a placed natural object.
type ObjectData struct {
	Type     uint8   `json:"t"`
	X        int     `json:"x"`
	Z        int     `json:"z"`
	Rotation float32 `json:"r,omitempty"`
}

// BuildingData is a placed lodge. ID is preserved across save/load so that
// agent.TargetID references stay valid. X/Z are continuous world XZ
// (metres); Y is reconstructed from terrain at load time.
type BuildingData struct {
	ID            uint64  `json:"id,omitempty"`
	X             float32 `json:"x"`
	Z             float32 `json:"z"`
	Rotation      float32 `json:"r,omitempty"`
	MeanSpawnRate float64 `json:"mean_spawn_rate"`
	SkierCount    int     `json:"skier_count,omitempty"`
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
	ID       uint64      `json:"id,omitempty"`
	BaseX    float32     `json:"bx"`
	BaseZ    float32     `json:"bz"`
	TopX     float32     `json:"tx"`
	TopZ     float32     `json:"tz"`
	Speed    float32     `json:"speed,omitempty"`
	Chairs   []ChairData `json:"chairs,omitempty"`
	QueueIDs []uint64    `json:"queue,omitempty"`
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
