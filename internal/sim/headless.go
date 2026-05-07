package sim

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"sort"
	"strings"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// HeadlessOptions configures a -testbed CLI run.
type HeadlessOptions struct {
	// SimSeconds caps the run length (in sim time, post-TimeScale). Default 240.
	SimSeconds float64
	// SamplePeriod is the sim-time interval between tabular log rows. Default 0.5.
	SamplePeriod float64
	// Seed overrides the testbed's recommended seed when non-zero.
	Seed int64
}

// RunHeadless runs the named testbed without a GLFW window and writes a
// human-readable trace + summary to `out`. Intended for `go run . -testbed
// "<name prefix>"` so we can iterate on agent behaviour by reading what
// the AI perceived and decided each tick, instead of asserting against
// thresholds.
func RunHeadless(out io.Writer, name string, opts HeadlessOptions) error {
	tb, err := FindTestbed(name)
	if err != nil {
		return err
	}
	if opts.SimSeconds <= 0 {
		opts.SimSeconds = 240
	}
	if opts.SamplePeriod <= 0 {
		opts.SamplePeriod = 0.5
	}
	seed := tb.Seed
	if opts.Seed != 0 {
		seed = opts.Seed
	}

	w := tb.Build(rand.New(rand.NewSource(seed)))
	if len(w.Agents) == 0 {
		return fmt.Errorf("testbed %q produced no agents", tb.Name)
	}
	agent := w.Agents[0]

	sim := NewSimulationWithSeed(w, seed)
	sim.TimeScale = 1.0 // headless ticks are real-time = sim-time so dt is exact
	rec := newTraceRecorder(agent.ID, out, opts.SamplePeriod)
	sim.Recorder = rec

	target := targetPos(w, agent)
	fmt.Fprintf(out, "testbed: %s  seed=%d  sim_seconds=%.1f\n", tb.Name, seed, opts.SimSeconds)
	fmt.Fprintf(out, "agent:   id=%d skill=%s pos=(%.1f,%.1f,%.1f) target=(%.1f,%.1f,%.1f) dist=%.1f\n\n",
		agent.ID, skillName(agent.Traits.Skill),
		agent.Pos[0], agent.Pos[1], agent.Pos[2],
		target[0], target[1], target[2],
		dist3(agent.Pos[0]-target[0], agent.Pos[1]-target[1], agent.Pos[2]-target[2]))

	rec.writeHeader()

	const dt = 1.0 / 60.0
	steps := int(opts.SimSeconds / dt)
	prevActivity := world.Activity(w, agent)
	arrived := false
	var arrivedAt float64

	for i := 0; i < steps; i++ {
		sim.Tick(dt)
		if len(w.Agents) == 0 {
			arrived = true
			arrivedAt = sim.SimTime
			break
		}
		// Track activity transitions for event lines (falls, mode changes).
		cur := world.Activity(w, w.Agents[0])
		if cur != prevActivity {
			fmt.Fprintf(out, "  ! t=%6.2f  %s → %s\n", sim.SimTime, prevActivity, cur)
			prevActivity = cur
		}
	}

	fmt.Fprintln(out)
	if arrived {
		fmt.Fprintf(out, "arrived at t=%.2fs\n", arrivedAt)
	} else {
		a := w.Agents[0]
		d := dist3(a.Pos[0]-target[0], a.Pos[1]-target[1], a.Pos[2]-target[2])
		fmt.Fprintf(out, "did NOT arrive within %.1fs (final pos=(%.1f,%.1f,%.1f) dist=%.1f activity=%s)\n",
			opts.SimSeconds, a.Pos[0], a.Pos[1], a.Pos[2], d, world.Activity(w, a))
	}
	rec.writeSummary()
	return nil
}

// =============================================================================
// trace recorder
// =============================================================================

// traceRecorder writes a tabular row every SamplePeriod seconds and tallies
// summary stats over the full run.
type traceRecorder struct {
	id     uint64
	out    io.Writer
	period float64

	nextSampleAt float64
	frames       int

	maxSpeed          float32
	fallEvents        int
	fallLineCrossings int
	prevSign          int8
	techTicks         map[ai.Technique]int
	totalTicks        int
	prevActivity      string
	prevActivitySet   bool
}

func newTraceRecorder(id uint64, out io.Writer, period float64) *traceRecorder {
	return &traceRecorder{
		id:        id,
		out:       out,
		period:    period,
		techTicks: make(map[ai.Technique]int),
	}
}

func (r *traceRecorder) AgentID() uint64 { return r.id }
func (r *traceRecorder) Close() error    { return nil }

func (r *traceRecorder) writeHeader() {
	fmt.Fprintf(r.out, "  %-7s %-12s %-22s %5s %9s %-8s %4s %5s %-14s %6s\n",
		"t", "activity", "pos(x,y,z)", "spd", "head→axis", "tech", "bal", "slope", "probe C/R/L", "dist")
}

func (r *traceRecorder) Record(f RecorderFrame) {
	r.totalTicks++
	if f.Speed > r.maxSpeed {
		r.maxSpeed = f.Speed
	}
	r.techTicks[f.Technique]++

	// Fall detection: rising edge into "Fallen" activity.
	if r.prevActivitySet && f.Activity == "Fallen" && r.prevActivity != "Fallen" {
		r.fallEvents++
	}
	r.prevActivity = f.Activity
	r.prevActivitySet = true

	// Fall-line crossings: sign of (heading - axis) flips.
	dev := wrapAngle(f.Heading - f.AxisHeading)
	var s int8
	if dev > 0.05 {
		s = 1
	} else if dev < -0.05 {
		s = -1
	}
	if s != 0 && r.prevSign != 0 && s != r.prevSign {
		r.fallLineCrossings++
	}
	if s != 0 {
		r.prevSign = s
	}

	if f.SimTime+1e-9 < r.nextSampleAt {
		return
	}
	r.nextSampleAt = f.SimTime + r.period
	r.frames++

	headDev := wrapAngle(f.Heading - f.AxisHeading)
	slopeDeg := math.Acos(math.Min(1, math.Max(-1, float64(f.SlopeCos)))) * 180 / math.Pi

	fmt.Fprintf(r.out, "  %6.2f  %-12s (%6.1f,%5.1f,%6.1f) %5.2f %+7.2f° %-8s %4.2f %4.1f° %4.2f/%4.2f/%4.2f %6.1f\n",
		f.SimTime,
		f.Activity,
		f.Pos[0], f.Pos[1], f.Pos[2],
		f.Speed,
		headDev*180/math.Pi,
		techName(f.Technique),
		f.Balance,
		slopeDeg,
		f.ProbeC, f.ProbeR, f.ProbeL,
		f.Dist,
	)
}

func (r *traceRecorder) writeSummary() {
	parallelPct := 0.0
	if r.totalTicks > 0 {
		parallelPct = 100 * float64(r.techTicks[ai.TechParallel]) / float64(r.totalTicks)
	}
	fmt.Fprintf(r.out, "summary: ticks=%d  maxSpeed=%.2fm/s  fallEvents=%d  fallLineCrossings=%d  parallel=%.0f%%\n",
		r.totalTicks, r.maxSpeed, r.fallEvents, r.fallLineCrossings, parallelPct)

	// Technique histogram, sorted by tick count desc.
	type tt struct {
		t Technique
		n int
	}
	tt2 := []tt{}
	for k, v := range r.techTicks {
		tt2 = append(tt2, tt{k, v})
	}
	sort.Slice(tt2, func(i, j int) bool { return tt2[i].n > tt2[j].n })
	parts := make([]string, 0, len(tt2))
	for _, x := range tt2 {
		parts = append(parts, fmt.Sprintf("%s=%d", techName(x.t), x.n))
	}
	if len(parts) > 0 {
		fmt.Fprintf(r.out, "techniques: %s\n", strings.Join(parts, " "))
	}
}

// =============================================================================
// helpers
// =============================================================================

// Technique aliasing — local copy of ai.Technique keeps the recorder free of
// import-cycle worries when the file later grows.
type Technique = ai.Technique

func techName(t ai.Technique) string {
	switch t {
	case ai.TechStraight:
		return "straight"
	case ai.TechPizza:
		return "pizza"
	case ai.TechWedgeTurn:
		return "wedge"
	case ai.TechParallel:
		return "parallel"
	case ai.TechHockey:
		return "hockey"
	case ai.TechSideslip:
		return "sideslp"
	}
	return fmt.Sprintf("?%d", t)
}

func skillName(s ai.SkillLevel) string {
	switch s {
	case ai.SkillBeginner:
		return "beginner"
	case ai.SkillIntermediate:
		return "intermediate"
	case ai.SkillAdvanced:
		return "advanced"
	}
	return fmt.Sprintf("?%d", s)
}

// targetPos resolves where an agent is heading (lodge or lift base) so the
// header can print a useful target. Returns the agent's own position if the
// TargetID resolves to nothing.
func targetPos(w *world.World, a *world.Agent) [3]float32 {
	const cellSize = 10.0
	if a.TargetID == 0 {
		return [3]float32{a.Pos[0], a.Pos[1], a.Pos[2]}
	}
	for _, b := range w.Buildings {
		if b.ID == a.TargetID {
			return [3]float32{float32(b.Pos[0]) * cellSize, w.Terrain.ElevationAt(b.Pos[0], b.Pos[1]), float32(b.Pos[1]) * cellSize}
		}
	}
	for _, l := range w.Lifts {
		if l.ID == a.TargetID {
			return [3]float32{float32(l.Base[0]) * cellSize, w.Terrain.ElevationAt(l.Base[0], l.Base[1]), float32(l.Base[1]) * cellSize}
		}
	}
	return [3]float32{a.Pos[0], a.Pos[1], a.Pos[2]}
}

func dist3(dx, dy, dz float32) float32 {
	return float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
}
