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

// BuildingData is a placed building.
type BuildingData struct {
	X         int     `json:"x"`
	Z         int     `json:"z"`
	Rotation  float32 `json:"r,omitempty"`
	SpawnRate float64 `json:"spawn_rate"`
}

// LiftData is a placed lift.
type LiftData struct {
	BaseX int `json:"bx"`
	BaseZ int `json:"bz"`
	TopX  int `json:"tx"`
	TopZ  int `json:"tz"`
}

// AgentData is a saved agent state.
type AgentData struct {
	Pos     [3]float32 `json:"pos"`
	Heading float32    `json:"heading"`
	State   int        `json:"state"`
	Path    [][2]int   `json:"path,omitempty"`
	PathIdx int        `json:"path_idx,omitempty"`
}
