package save

// ScenarioData is the JSON-serialisable representation of a full scenario.
type ScenarioData struct {
	// Name through Tutorial are world.ScenarioInfo. They come first so
	// ReadScenarioInfo can stop decoding before the cells. Older saves
	// wrote the placeholder Name "scenario", which loads as no name.
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
	Difficulty  int    `json:"difficulty,omitempty"`
	Order       int    `json:"order,omitempty"`
	Tutorial    bool   `json:"tutorial,omitempty"`
	// ScenarioFile is world.ScenarioInfo.File.
	ScenarioFile string `json:"scenario_file,omitempty"`

	Width   int     `json:"width"`
	Height  int     `json:"height"`
	Seed    int64   `json:"seed,omitempty"`
	SimTime float64 `json:"sim_time,omitempty"` // sim clock in seconds at save time
	// DaySec is the sim seconds per calendar day the clocks in this save
	// (SimTime, event times, pass expiries) were written with. Absent =
	// world.LegacySecondsPerSimDay; the loader rescales to the current day.
	DaySec float64 `json:"day_sec,omitempty"`
	// OpenHour/CloseHour are World.OpenHour/CloseHour. Absent loads the
	// defaults.
	OpenHour  float32 `json:"open_hour,omitempty"`
	CloseHour float32 `json:"close_hour,omitempty"`
	// StartDate is World.StartDate, the date SimTime 0 maps to, as
	// "2006-01-02". Absent loads world.DefaultStartDate.
	StartDate string       `json:"start_date,omitempty"`
	Cells     []CellData   `json:"cells"` // flat array, row-major (x-major)
	Objects   []ObjectData `json:"objects"`
	// Trees is every stored tree as flat world-XZ pairs: x0, z0, x1, z1, …
	// Older saves have none and carry per-cell TreeDensity instead.
	Trees []float32 `json:"trees,omitempty"`
	// Groom is the 1 m groom map (world.GroomMap pixels); empty when
	// nothing is groomed. Saves without it rebuild from cell grooming.
	Groom []byte `json:"groom,omitempty"`
	// Detail is sub-cell ground detail at 1.25 m (world.TerrainDetail
	// bytes), e.g. from lidar; empty for the plain 5 m mesh.
	Detail []byte `json:"detail,omitempty"`
	// Material is world.TerrainMaterial, one byte per detail sample.
	Material []byte `json:"material,omitempty"`
	// LakeOf is world.Terrain.LakeOf, one byte per cell; Lakes are the
	// lakes it indexes, with their ice.
	LakeOf    []byte     `json:"lake_of,omitempty"`
	LakeDepth []byte     `json:"lake_depth,omitempty"` // decimetres per cell
	Lakes     []LakeData `json:"lakes,omitempty"`
	// Geo is [minLat, maxLat, minLon, maxLon] of an imported terrain.
	Geo []float64 `json:"geo,omitempty"`
	// BaseAltitude, TimeZone and Climate are the World fields of the
	// same names; absent for drawn maps and older imports.
	BaseAltitude float32      `json:"base_alt,omitempty"`
	TimeZone     string       `json:"time_zone,omitempty"`
	Climate      *ClimateData `json:"climate,omitempty"`
	// TerrainBase is world.TerrainBase: the imported ground before
	// terrain layers. Scenario files only; absent for drawn maps.
	TerrainBase *TerrainBaseData `json:"terrain_base,omitempty"`
	Buildings   []BuildingData   `json:"buildings"`
	Lifts       []LiftData       `json:"lifts"`
	Trails      []TrailData      `json:"trails,omitempty"`
	Guests      []GuestData      `json:"guests"`
	Snowcats    []SnowcatData    `json:"snowcats,omitempty"`
	Snowmobiles []SnowmobileData `json:"snowmobiles,omitempty"`
	Patrollers  []PatrollerData  `json:"patrollers,omitempty"`
	RoadNodes   []RoadNodeData   `json:"road_nodes,omitempty"`
	RoadEdges   []RoadEdgeData   `json:"road_edges,omitempty"`
	Cars        []CarData        `json:"cars,omitempty"`
	Parcels     []ParcelData     `json:"parcels,omitempty"`
	Cash        int              `json:"cash,omitempty"`
	// Credit line state. CreditLimit is a pointer so a $0 line round-trips;
	// nil (older saves) loads as DefaultCreditLimit.
	CreditLimit     *int    `json:"credit_limit,omitempty"`
	AccruedInterest float64 `json:"accrued_interest,omitempty"`
	DaysBelowFloor  int     `json:"days_below_floor,omitempty"`
	Bankrupt        bool    `json:"bankrupt,omitempty"`
	// DayTicket is World.DayTicketPrice. Pointer so a player-set $0 round-
	// trips; nil (older saves) loads as DefaultDayTicketPrice.
	DayTicket *int `json:"day_ticket,omitempty"`
	// Parking is World.ParkingPrice, per car. Absent loads free.
	Parking int `json:"parking,omitempty"`
	// ResortOpen is World.ResortOpen. Absent loads closed.
	ResortOpen bool `json:"resort_open,omitempty"`
	// Rating is World.Rating; nil (older saves) loads as
	// world.InitialRating.
	Rating *float32 `json:"rating,omitempty"`
	// Goals and Rules are what the scenario asks (world.Goal, world.Rules);
	// GoalProgress is how each goal stands, player saves only.
	Goals        []GoalData         `json:"goals,omitempty"`
	Rules        []string           `json:"rules,omitempty"`
	GoalProgress []GoalProgressData `json:"goal_progress,omitempty"`
	Outcome      uint8              `json:"outcome,omitempty"`
	OutcomeDay   int                `json:"outcome_day,omitempty"`
	Camera       *CameraData        `json:"camera,omitempty"`
	History      *HistoryData       `json:"history,omitempty"`
	Events       []EventData        `json:"events,omitempty"`
}

// GoalData is world.Goal.
type GoalData struct {
	Kind   uint8   `json:"k"`
	Target float64 `json:"t,omitempty"`
	Days   int     `json:"days,omitempty"`
	Season int     `json:"season,omitempty"`
	Bonus  bool    `json:"bonus,omitempty"`
}

// GoalProgressData is world.GoalProgress.
type GoalProgressData struct {
	Met    bool    `json:"met,omitempty"`
	MetDay int     `json:"day,omitempty"`
	Streak int     `json:"streak,omitempty"`
	Best   float64 `json:"best,omitempty"`
	Failed bool    `json:"failed,omitempty"`
}

// ClimateData is world.Climate.
type ClimateData struct {
	Source      string             `json:"source,omitempty"`
	RefAltitude float32            `json:"ref_alt"`
	WindDeg     float32            `json:"wind"`
	Months      []ClimateMonthData `json:"months"`
}

// TerrainBaseData is world.TerrainBase. Geo is [minLat, maxLat, minLon,
// maxLon]; Heights is TerrainBase.HeightsBytes.
type TerrainBaseData struct {
	Geo           []float64          `json:"geo"`
	W             int                `json:"w"`
	H             int                `json:"h"`
	Detail        bool               `json:"detail,omitempty"`
	Heights       []byte             `json:"heights"`
	Roads         []BaseRoadData     `json:"roads,omitempty"`
	RoadNote      string             `json:"road_note,omitempty"`
	LidarCoverage float32            `json:"lidar,omitempty"`
	LidarNote     string             `json:"lidar_note,omitempty"`
	LayersOff     []string           `json:"off,omitempty"`
	Strengths     map[string]float32 `json:"strength,omitempty"`
	Lifts         []BaseLiftData     `json:"lifts,omitempty"`
	Runs          []BaseRunData      `json:"runs,omitempty"`
	Areas         []BaseAreaData     `json:"areas,omitempty"`
	Streams       []BaseStreamData   `json:"streams,omitempty"`
	Lakes         []BaseLakeData     `json:"lakes,omitempty"`
}

// BaseStreamData is world.BaseStream, its path as flat lat, lon pairs.
type BaseStreamData struct {
	Name         string    `json:"name,omitempty"`
	Kind         string    `json:"kind"`
	Intermittent bool      `json:"intermittent,omitempty"`
	Path         []float64 `json:"path"`
}

// BaseLakeData is world.BaseLake, each path as flat lat, lon pairs.
type BaseLakeData struct {
	Name  string      `json:"name,omitempty"`
	Kind  string      `json:"kind,omitempty"`
	Paths [][]float64 `json:"paths"`
}

// LakeData is world.Lake.
type LakeData struct {
	Name     string  `json:"name,omitempty"`
	Altitude float32 `json:"alt"`
	AreaHa   float32 `json:"ha"`
	MaxDepth float32 `json:"max_depth"`
	Frost    float32 `json:"frost"`
	Thaw     float32 `json:"thaw"`
}

// BaseRoadData is world.BaseRoad, its path as flat lat, lon pairs.
type BaseRoadData struct {
	Name   string    `json:"name,omitempty"`
	Kind   string    `json:"kind,omitempty"`
	Width  float32   `json:"w"`
	Tunnel bool      `json:"tunnel,omitempty"`
	Path   []float64 `json:"path"`
}

// BaseLiftData is world.BaseLift, its path as flat lat, lon pairs.
type BaseLiftData struct {
	Name  string    `json:"name,omitempty"`
	Kind  string    `json:"kind"`
	Seats int       `json:"seats,omitempty"`
	Path  []float64 `json:"path"`
}

// BaseRunData is world.BaseRun, its path as flat lat, lon pairs.
type BaseRunData struct {
	Name       string    `json:"name,omitempty"`
	Difficulty string    `json:"diff,omitempty"`
	Area       bool      `json:"area,omitempty"`
	Path       []float64 `json:"path"`
}

// BaseAreaData is world.BaseArea, each path as flat lat, lon pairs.
type BaseAreaData struct {
	Name  string      `json:"name,omitempty"`
	Paths [][]float64 `json:"paths"`
}

// ClimateMonthData is world.ClimateMonth.
type ClimateMonthData struct {
	TempMean  float32 `json:"t"`
	TempRange float32 `json:"tr"`
	WetDays   float32 `json:"wet"`
	WetMM     float32 `json:"mm"`
	Cloud     float32 `json:"cloud"`
}

// EventData is one entry of the world event feed (world.Event). Saved
// oldest-first; the loader pushes them back in order, so the ring's head
// bookkeeping is not persisted. Saves without the field load an empty feed.
type EventData struct {
	Kind     uint8   `json:"k,omitempty"`
	SimTime  float64 `json:"t,omitempty"`
	Message  string  `json:"m,omitempty"`
	HasPos   bool    `json:"hp,omitempty"`
	X        float32 `json:"x,omitempty"`
	Z        float32 `json:"z,omitempty"`
	EntityID uint64  `json:"id,omitempty"`
}

// ParcelData is one scenario-authored land parcel. State: 0=owned,
// 1=purchasable, 2=off-limits. Cells is the full list of terrain grid
// coordinates belonging to this parcel.
type ParcelData struct {
	ID    uint16   `json:"id"`
	Name  string   `json:"name,omitempty"`
	State uint8    `json:"state"`
	Price int      `json:"price,omitempty"`
	Cells [][2]int `json:"cells,omitempty"`
}

// PatrollerData is a saved ski patroller. HutID links it to the building
// whose patrol service bases it; Snowmobile is the one they have out,
// Target the guest they're helping, and TargetPos where they're headed
// (a walk's route is found again on load).
type PatrollerData struct {
	ID         uint64     `json:"id,omitempty"`
	HutID      uint64     `json:"hut,omitempty"`
	Pos        [3]float32 `json:"pos"`
	Heading    float32    `json:"heading,omitempty"`
	State      uint8      `json:"state,omitempty"`
	Snowmobile uint64     `json:"sled,omitempty"`
	Target     uint64     `json:"target,omitempty"`
	TargetPos  [3]float32 `json:"target_pos,omitempty"`
	Timer      float32    `json:"timer,omitempty"`
	OnSkis     bool       `json:"skis,omitempty"`
	Lift       uint64     `json:"lift,omitempty"`
	LiftT      float32    `json:"lift_t,omitempty"`
}

// TrailData is a saved player-defined ski trail. Cells is the complete
// list of grid cells the trail covers; connectivity is derived on load
// and not persisted.
type TrailData struct {
	ID         uint64   `json:"id,omitempty"`
	Name       string   `json:"name,omitempty"`
	Difficulty uint8    `json:"diff,omitempty"`
	Groomed    bool     `json:"groomed,omitempty"`
	Cells      [][2]int `json:"cells,omitempty"`
}

// HistoryData is the saved daily ring of resort stats — see
// world.History. Samples is stored in chronological order
// (oldest-first), so loaders can iterate without bothering about the
// ring's runtime head index; the loader resets Head to len(Samples) %
// HistoryCapacity and Filled when Samples is at capacity.
type HistoryData struct {
	Samples         []DailySampleData `json:"s,omitempty"`
	ArrivalsToday   int               `json:"a,omitempty"`
	DeparturesToday int               `json:"d,omitempty"`
	RevenueToday    int               `json:"r,omitempty"`
	RevenueByKind   []int             `json:"rk,omitempty"` // world.RevenueKind order
}

// DailySampleData mirrors world.DailySample with msgpack-compact field
// tags. Day is stored as Unix seconds; non-zero for any persisted row.
type DailySampleData struct {
	DayUnix          int64   `json:"d,omitempty"`
	GuestsOnMountain int     `json:"g,omitempty"`
	ArrivalsToday    int     `json:"a,omitempty"`
	DeparturesToday  int     `json:"x,omitempty"`
	Cash             int     `json:"c,omitempty"`
	Revenue          int     `json:"r,omitempty"`
	Costs            int     `json:"co,omitempty"`
	RevenueByKind    []int   `json:"rk,omitempty"` // world.RevenueKind order
	CostsByKind      []int   `json:"ck,omitempty"` // world.CostKind order
	Open             bool    `json:"o,omitempty"`
	Rating           float32 `json:"rt,omitempty"`
}

// RoadNodeData is one vertex in the road graph. ID is preserved across
// save/load so RoadEdgeData.A/B references stay valid.
type RoadNodeData struct {
	ID   uint64  `json:"id,omitempty"`
	X    float32 `json:"x"`
	Z    float32 `json:"z"`
	Kind uint8   `json:"k,omitempty"`
	// Name and Pool are an entry's (world.RoadNode).
	Name string `json:"name,omitempty"`
	Pool int    `json:"pool,omitempty"`
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

// LayerData is one entry in a cell's snow-layer stack. Fields are
// minimal-width for compact JSON on large maps.
type LayerData struct {
	A float32 `json:"a"`           // SWE metres (Accumulation)
	K uint8   `json:"k,omitempty"` // SnowKind (0=Powder if omitted)
}

// CellData is the serialisable representation of a terrain cell. Schema
// matches world.Cell; field names are short to keep the per-cell JSON
// footprint reasonable on big maps.
type CellData struct {
	Ground       float32     `json:"e,omitempty"`
	Layers       []LayerData `json:"ls,omitempty"`
	Grooming     float32     `json:"gr,omitempty"`
	MogulSize    float32     `json:"mg,omitempty"`
	SkierTraffic float32     `json:"st,omitempty"`
	// TreeDensity is the old per-cell forest density. Read on load and
	// converted to stored trees (ScenarioData.Trees); never written.
	TreeDensity float32 `json:"td,omitempty"`
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
	ID       uint64  `json:"id,omitempty"`
	Type     uint8   `json:"bt,omitempty"`
	X        float32 `json:"x"`
	Z        float32 `json:"z"`
	Rotation float32 `json:"r,omitempty"`

	// Parking-only state. A lot is a rectangle: X, Z is its centre,
	// Rotation its turn and LotSize its extent along local X and Z.
	// MaxCars is the stall count, re-derived on load. DriveEdge is the
	// lot's driveway edge.
	LotSize         [2]float32 `json:"lot_size,omitempty"`
	DriveEdge       uint64     `json:"drive_edge,omitempty"`
	MaxCars         int        `json:"max_cars,omitempty"`
	DrivewayNodeIDs []uint64   `json:"driveway_ids,omitempty"` // road-network attach nodes, one per parking mesh slot

	// SnowGun-only state.
	SnowGunEnabled bool `json:"sg_on,omitempty"`

	// Service-building state. OriginX, OriginZ and Rotation place the
	// building's own grid; Tiles is each shell cell in that grid with its
	// world.Service. Doors are derived on load. Older forms (painted
	// cells, point-placed lodges, bars and ticket offices) aren't loaded.
	Kind       uint8    `json:"kind,omitempty"`    // world.ShellKind
	Storeys    int      `json:"storeys,omitempty"` // 0 or 1: one storey
	OriginX    float32  `json:"ox,omitempty"`
	OriginZ    float32  `json:"oz,omitempty"`
	Tiles      [][3]int `json:"tiles,omitempty"`
	FloorY     float32  `json:"floor,omitempty"`
	FloorSet   bool     `json:"floor_set,omitempty"`
	StyleSeed  uint32   `json:"style,omitempty"`
	MealPrice  int      `json:"meal,omitempty"`
	DrinkPrice int      `json:"drink,omitempty"`
}

// SnowcatData is a saved cat. ShedID links it back to its shed; both
// IDs survive save/load so the cat → shed chain rehydrates correctly.
// Section assignments are recomputed on load and are not persisted.
type SnowcatData struct {
	ID      uint64     `json:"id,omitempty"`
	ShedID  uint64     `json:"shed,omitempty"`
	Pos     [3]float32 `json:"pos"`
	Heading float32    `json:"heading,omitempty"`
	Status  uint8      `json:"status,omitempty"` // 0=Active, 1=Standby
}

// SnowmobileData is a saved patrol snowmobile; GarageID is the building
// whose garage houses it.
type SnowmobileData struct {
	ID       uint64     `json:"id,omitempty"`
	GarageID uint64     `json:"garage,omitempty"`
	Pos      [3]float32 `json:"pos"`
	Heading  float32    `json:"heading,omitempty"`
	InGarage bool       `json:"in,omitempty"`
	TakenBy  uint64     `json:"taken,omitempty"`
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
	Open        bool        `json:"open"`
	Chairs      []ChairData `json:"chairs,omitempty"`
	QueueIDs    []uint64    `json:"queue,omitempty"`
	// HeliPhase and HeliProgress are only meaningful when Type == LiftHeli.
	HeliPhase    uint8   `json:"heli_phase,omitempty"`
	HeliProgress float32 `json:"heli_progress,omitempty"`
	// Queue line config — only non-zero for LiftDouble with lanes configured.
	LeftLines    int        `json:"left_lines,omitempty"`
	RightLines   int        `json:"right_lines,omitempty"`
	SingleRider  bool       `json:"single_rider,omitempty"`
	LineQueueIDs [][]uint64 `json:"line_queues,omitempty"` // per-lane ordered guest IDs
}

// PlanActionData is one serialised step in a guest's L0 plan. Fields map
// 1:1 to ai.PlanAction; omitempty keeps dormant-skier rows compact.
type PlanActionData struct {
	Kind    uint8   `json:"k,omitempty"`
	LiftID  uint64  `json:"l,omitempty"`
	BldgID  uint64  `json:"b,omitempty"`
	TrailID uint64  `json:"t,omitempty"`
	Cost    float32 `json:"c,omitempty"`
}

// GuestData is a saved guest record. Covers both dormant (AtHome) and
// active (OnMountain) guests — the master Guests slice persists every
// entry so identity + career stats round-trip. Sim-scratch fields are
// only meaningful for State==OnMountain; they're omitted (zero) for
// dormant rows so the on-wire footprint stays small.
type GuestData struct {
	// Identity.
	ID              uint64  `json:"id,omitempty"`
	Name            string  `json:"name,omitempty"`
	Discipline      uint8   `json:"disc,omitempty"`
	Skill           float32 `json:"skill,omitempty"`
	VisitsPerSeason float32 `json:"vps,omitempty"`
	HomeEntry       uint64  `json:"entry,omitempty"` // world.Guest.HomeEntryID
	LikesGlades     bool    `json:"glades,omitempty"`
	PrefersGroomed  bool    `json:"groomed,omitempty"`

	// Career stats.
	VisitsThisSeason int     `json:"vts,omitempty"`
	LifetimeVisits   int     `json:"lv,omitempty"`
	LastVisitUnix    int64   `json:"lvu,omitempty"` // 0 = never visited
	LastScore        float32 `json:"lsc,omitempty"`

	// Season pass. SeasonPassExpiry is the SimTime at which the pass expires;
	// HasSeasonPass is the precomputed validity flag for guests OnMountain.
	SeasonPassExpiry float64 `json:"spe,omitempty"`
	HasSeasonPass    bool    `json:"hsp,omitempty"`

	// Day ticket for the current visit (OnMountain only): the price still
	// owed at the window, the price already paid, and whether it's bought.
	DayTicketDue  int  `json:"dtd,omitempty"`
	DayTicketPaid int  `json:"dtp,omitempty"`
	HasDayTicket  bool `json:"hdt,omitempty"`

	// RemainingBudget is what's left of the day's spending money after
	// ticket, parking and food (OnMountain only).
	RemainingBudget float32 `json:"rb,omitempty"`

	// Visit state. 0 = AtHome (default), 1 = OnMountain, 2 = InCar.
	State uint8 `json:"state,omitempty"`
	// CarID and CarLot are world.Guest's: the car they came in and the
	// lot it's parked in.
	CarID  uint64 `json:"car,omitempty"`
	CarLot uint64 `json:"car_lot,omitempty"`

	// Sim scratch — only populated when State == OnMountain.
	Pos      [3]float32 `json:"pos,omitempty"`
	Heading  float32    `json:"heading,omitempty"`
	Path     [][2]int   `json:"path,omitempty"`
	PathIdx  int        `json:"path_idx,omitempty"`
	Speed    float32    `json:"speed,omitempty"`
	TargetID uint64     `json:"target_id,omitempty"`
	OnLiftID uint64     `json:"on_lift_id,omitempty"`
	Queued   bool       `json:"queued,omitempty"`
	Patience float32    `json:"patience,omitempty"`
	Energy   float32    `json:"energy,omitempty"`
	Hunger   float32    `json:"hunger,omitempty"`
	Thirst   float32    `json:"thirst,omitempty"`
	// Plan steps and cursor so agents resume mid-plan after load rather than
	// replanning from an anchor-zero in-transit snapshot. GoalName and Target
	// are re-derived by onPlanStepStart; only Steps+Step are stored.
	PlanSteps []PlanActionData `json:"plan,omitempty"`
	PlanStep  int              `json:"plan_step,omitempty"`
}

// CarData is a saved world.Car. Guests are guest IDs; Route is road node
// IDs. Scenario files carry no cars.
type CarData struct {
	ID      uint64     `json:"id"`
	Guests  []uint64   `json:"guests,omitempty"`
	Entry   uint64     `json:"entry,omitempty"`
	Lot     uint64     `json:"lot,omitempty"`
	Stall   int        `json:"stall"`
	State   uint8      `json:"state,omitempty"`
	Route   []uint64   `json:"route,omitempty"`
	Leg     int        `json:"leg,omitempty"`
	D       float32    `json:"d,omitempty"`
	Speed   float32    `json:"speed,omitempty"`
	Pos     [2]float32 `json:"pos"`
	Heading float32    `json:"heading,omitempty"`
	InLot   bool       `json:"in_lot,omitempty"`
}
