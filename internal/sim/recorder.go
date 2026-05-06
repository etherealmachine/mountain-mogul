package sim

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// RecorderFrame is one tick of state for a skiing agent. Fields cover every
// signal that drives the AI tick (perception inputs, intent, motor command,
// balance) so a CSV log is enough to reason about the agent's decision after
// the fact.
type RecorderFrame struct {
	SimTime float64
	AgentID uint64
	State   world.AgentState

	Pos     mgl32.Vec3
	Heading float32
	Target  mgl32.Vec3
	Dist    float32
	Speed   float32

	Technique ai.Technique
	TurnPhase int8

	FallLine       mgl32.Vec2
	AxisHeading    float32 // intent.AxisHeading
	DesiredHeading float32 // motor cmd.Heading

	TargetSpeed float32 // intent.Speed
	Urgency     float32 // intent.Urgency
	Balance     float32

	ProbeC          float32
	ProbeR          float32
	ProbeL          float32
	SlopeCos        float32
	InArrivalRadius bool
}

// Recorder consumes per-tick skiing frames. Implementations should be cheap —
// Record runs inside the simulation hot path.
type Recorder interface {
	AgentID() uint64
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

func (c *CSVRecorder) Path() string     { return c.path }
func (c *CSVRecorder) AgentID() uint64  { return c.agentID }

func (c *CSVRecorder) Record(fr RecorderFrame) {
	row := []string{
		strconv.FormatFloat(fr.SimTime, 'f', 4, 64),
		strconv.FormatUint(fr.AgentID, 10),
		fmt.Sprintf("%d", fr.State),
		f32(fr.Pos[0]), f32(fr.Pos[1]), f32(fr.Pos[2]),
		f32(fr.Heading),
		f32(fr.Target[0]), f32(fr.Target[1]), f32(fr.Target[2]),
		f32(fr.Dist),
		f32(fr.Speed),
		strconv.Itoa(int(fr.Technique)),
		strconv.Itoa(int(fr.TurnPhase)),
		f32(fr.FallLine[0]), f32(fr.FallLine[1]),
		f32(fr.AxisHeading),
		f32(fr.DesiredHeading),
		f32(fr.TargetSpeed),
		f32(fr.Urgency),
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
	"sim_t", "agent_id", "state",
	"pos_x", "pos_y", "pos_z",
	"heading_rad",
	"tgt_x", "tgt_y", "tgt_z",
	"dist", "speed",
	"technique", "turn_phase",
	"fall_x", "fall_z",
	"axis_head", "desired_head",
	"target_speed", "urgency", "balance",
	"probe_c", "probe_r", "probe_l",
	"slope_cos",
	"in_arrival_radius",
}

func f32(v float32) string {
	return strconv.FormatFloat(float64(v), 'f', 4, 32)
}
