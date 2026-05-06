package sim

import (
	"math"
	"math/rand"
	"testing"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// runTestbed builds the testbed world with the given seed, runs the sim for
// `simSeconds` of post-TimeScale time, and returns the final world plus the
// recorded frame history of the first agent.
func runTestbed(t *testing.T, build func(*rand.Rand) *world.World, seed int64, simSeconds float64) (*world.World, *memRecorder) {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	w := build(rng)
	if len(w.Agents) == 0 {
		t.Fatal("testbed produced no agents")
	}
	sim := NewSimulationWithSeed(w, seed)
	sim.TimeScale = 1.0 // tests run at 1× sim time so dt is exactly the loop dt
	rec := newMemRecorder(w.Agents[0].ID)
	sim.Recorder = rec

	const dt = 1.0 / 60.0
	steps := int(simSeconds / dt)
	for i := 0; i < steps; i++ {
		sim.Tick(dt)
		if len(w.Agents) == 0 {
			break // arrived
		}
	}
	return w, rec
}

func TestFlatPlaneBeginnerArrives(t *testing.T) {
	w, rec := runTestbed(t, BuildFlatPlane, 1, 240)
	if len(w.Agents) != 0 {
		t.Errorf("agent did not arrive: %d still in world after sim", len(w.Agents))
	}
	if len(rec.frames) == 0 {
		t.Fatal("no frames recorded")
	}
	// Speed should stay near the SkiWalkSpeed floor (~2 m/s) on flat terrain.
	maxSpeed := float32(0)
	for _, f := range rec.frames {
		if f.Speed > maxSpeed {
			maxSpeed = f.Speed
		}
	}
	if maxSpeed > 4 {
		t.Errorf("flat-plane skier exceeded expected walk speed: max=%.2f", maxSpeed)
	}
}

func TestSlope10IntermediateArrives(t *testing.T) {
	w, rec := runTestbed(t, BuildSlope10Intermediate, 1, 240)
	if len(w.Agents) != 0 {
		t.Errorf("agent did not arrive: %d still in world", len(w.Agents))
	}
	stats := summarize(rec)
	if stats.maxSpeed < 3 {
		t.Errorf("intermediate on 10° slope should pick up some speed, got max=%.2f", stats.maxSpeed)
	}
	if stats.fellOnce {
		t.Error("intermediate on 10° slope should not fall")
	}
}

func TestSlope20AdvancedLinksTurns(t *testing.T) {
	w, rec := runTestbed(t, BuildSlope20Advanced, 1, 240)
	if len(w.Agents) != 0 {
		t.Errorf("agent did not arrive: %d still in world", len(w.Agents))
	}
	stats := summarize(rec)
	// On a 20° slope the advanced skier should engage the parallel technique
	// and link visible turns. Each fall-line crossing means heading swept
	// past the axis from one side to the other.
	if stats.fallLineCrossings < 4 {
		t.Errorf("expected ≥4 fall-line crossings (linked turns), got %d", stats.fallLineCrossings)
	}
	if stats.parallelTicks < stats.totalTicks/2 {
		t.Errorf("expected parallel technique to dominate (>%d ticks), got %d", stats.totalTicks/2, stats.parallelTicks)
	}
	// And the speed should stay within reasonable advanced bounds.
	if stats.maxSpeed > 22 {
		t.Errorf("advanced skier going dangerously fast: max=%.2f", stats.maxSpeed)
	}
}

func TestRunoutTransitionsToStraight(t *testing.T) {
	w, rec := runTestbed(t, BuildRunout, 1, 360)
	if len(w.Agents) != 0 {
		t.Errorf("agent did not arrive: %d still in world", len(w.Agents))
	}
	if len(rec.frames) < 100 {
		t.Fatal("run too short to evaluate transition")
	}
	// Sample the last 10% of the run — should be on the runout, technique
	// dominated by Straight.
	tailStart := int(0.9 * float64(len(rec.frames)))
	tailStraight := 0
	tail := rec.frames[tailStart:]
	for _, f := range tail {
		if f.Technique == ai.TechStraight {
			tailStraight++
		}
	}
	if tailStraight < len(tail)/2 {
		t.Errorf("expected runout tail to be mostly TechStraight, got %d/%d", tailStraight, len(tail))
	}
}

func TestRunsAreDeterministic(t *testing.T) {
	w1, _ := runTestbed(t, BuildSlope20Advanced, 7, 60)
	w2, _ := runTestbed(t, BuildSlope20Advanced, 7, 60)
	if len(w1.Agents) != len(w2.Agents) {
		t.Fatalf("agent count diverged: %d vs %d", len(w1.Agents), len(w2.Agents))
	}
	if len(w1.Agents) == 0 {
		return
	}
	a1 := w1.Agents[0]
	a2 := w2.Agents[0]
	const tol = 1e-3
	if math.Abs(float64(a1.Pos[0]-a2.Pos[0])) > tol ||
		math.Abs(float64(a1.Pos[2]-a2.Pos[2])) > tol {
		t.Errorf("identical seeds produced divergent positions: %v vs %v", a1.Pos, a2.Pos)
	}
}

// =============================================================================
// Test helpers
// =============================================================================

type runStats struct {
	maxSpeed          float32
	fallLineCrossings int
	parallelTicks     int
	totalTicks        int
	fellOnce          bool
}

func summarize(r *memRecorder) runStats {
	st := runStats{totalTicks: len(r.frames)}
	if len(r.frames) == 0 {
		return st
	}
	prevSign := int8(0)
	for _, f := range r.frames {
		if f.Speed > st.maxSpeed {
			st.maxSpeed = f.Speed
		}
		if f.Technique == ai.TechParallel {
			st.parallelTicks++
		}
		if f.State == world.StateFallen {
			st.fellOnce = true
		}
		// Sign of (heading - axis) flips when the skier crosses the fall line.
		dev := wrapAngle(f.Heading - f.AxisHeading)
		var s int8
		if dev > 0.05 {
			s = 1
		} else if dev < -0.05 {
			s = -1
		}
		if s != 0 && prevSign != 0 && s != prevSign {
			st.fallLineCrossings++
		}
		if s != 0 {
			prevSign = s
		}
	}
	return st
}

type memRecorder struct {
	id     uint64
	frames []RecorderFrame
}

func newMemRecorder(id uint64) *memRecorder {
	return &memRecorder{id: id, frames: make([]RecorderFrame, 0, 1024)}
}

func (m *memRecorder) AgentID() uint64    { return m.id }
func (m *memRecorder) Record(f RecorderFrame) {
	m.frames = append(m.frames, f)
}
func (m *memRecorder) Close() error { return nil }
