package sim

import (
	"encoding/csv"
	"os"
	"strconv"

	"github.com/go-gl/mathgl/mgl32"
)

// RecorderFrame is one tick of state for a guest. Skiing-physics fields
// (probes, fall line, etc.) are populated only during tickSkier ticks and
// are zero for walking/queuing rows — use the activity column to tell them
// apart. Plan fields are populated for every activity type.
type RecorderFrame struct {
	SimTime  float64
	GuestID  uint64
	Activity string

	Pos     mgl32.Vec3
	Heading float32
	Target  mgl32.Vec3
	Dist    float32
	Speed   float32

	// Plan state — set for every activity, not just skiing.
	PlanStep string // e.g. "WalkToLift(Lift1)"
	GoalName string // e.g. "KeepSkiing"
	PathLen  int    // len(agent.Path)
	PathIdx  int    // agent.PathIdx

	// Skiing-physics fields — zero for non-skiing rows.
	FallLine       mgl32.Vec2
	AxisHeading    float32
	DesiredHeading float32

	TargetSpeed float32
	Brake       float32 // commanded brakeAngle (rad)
	TurnSide    int8    // -1/0/+1 carve commit
	Mode        string  // "straight" | "carve" | "brake"

	Balance float32

	ProbeC          float32
	ProbeR          float32
	ProbeL          float32
	SlopeCos        float32
	InArrivalRadius bool
	TacticalOffset  float32 // rad; sampler's winning lateral offset from axis
}

// Recorder consumes per-tick skiing frames. Implementations should be cheap —
// Record runs inside the simulation hot path.
type Recorder interface {
	GuestID() uint64
	Record(RecorderFrame)
	Close() error
}

// CSVRecorder writes RecorderFrames as CSV rows to disk.
type CSVRecorder struct {
	agentID uint64
	f       *os.File
	w       *csv.Writer
	path    string
}

// NewCSVRecorder opens path for writing and writes the header row.
// agentID == 0 records every skier; otherwise only frames matching the ID.
func NewCSVRecorder(path string, agentID uint64) (*CSVRecorder, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := csv.NewWriter(f)
	if err := w.Write(csvHeader); err != nil {
		f.Close()
		return nil, err
	}
	return &CSVRecorder{agentID: agentID, f: f, w: w, path: path}, nil
}

func (c *CSVRecorder) Path() string    { return c.path }
func (c *CSVRecorder) GuestID() uint64 { return c.agentID }

func (c *CSVRecorder) Record(fr RecorderFrame) {
	row := []string{
		strconv.FormatFloat(fr.SimTime, 'f', 4, 64),
		strconv.FormatUint(fr.GuestID, 10),
		fr.Activity,
		f32(fr.Pos[0]), f32(fr.Pos[1]), f32(fr.Pos[2]),
		f32(fr.Heading),
		f32(fr.Target[0]), f32(fr.Target[1]), f32(fr.Target[2]),
		f32(fr.Dist),
		f32(fr.Speed),
		fr.PlanStep,
		fr.GoalName,
		strconv.Itoa(fr.PathLen),
		strconv.Itoa(fr.PathIdx),
		f32(fr.FallLine[0]), f32(fr.FallLine[1]),
		f32(fr.AxisHeading),
		f32(fr.DesiredHeading),
		f32(fr.TargetSpeed),
		f32(fr.Brake),
		strconv.Itoa(int(fr.TurnSide)),
		fr.Mode,
		f32(fr.Balance),
		f32(fr.ProbeC), f32(fr.ProbeR), f32(fr.ProbeL),
		f32(fr.SlopeCos),
		strconv.FormatBool(fr.InArrivalRadius),
	}
	_ = c.w.Write(row)
}

func (c *CSVRecorder) Close() error {
	c.w.Flush()
	return c.f.Close()
}

var csvHeader = []string{
	"sim_t", "agent_id", "activity",
	"pos_x", "pos_y", "pos_z",
	"heading_rad",
	"tgt_x", "tgt_y", "tgt_z",
	"dist", "speed",
	"plan_step", "goal", "path_len", "path_idx",
	"fall_x", "fall_z",
	"axis_head", "desired_head",
	"target_speed", "brake_rad", "turn_side", "mode",
	"balance",
	"probe_c", "probe_r", "probe_l",
	"slope_cos",
	"in_arrival_radius",
}

func f32(v float32) string {
	return strconv.FormatFloat(float64(v), 'f', 4, 32)
}
