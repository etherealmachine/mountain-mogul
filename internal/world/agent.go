package world

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
)

// Agent is a skier/guest in the simulation. There is no explicit state field:
// the agent's situation is implicit in the combination of OnLiftID / Queued /
// Fallen / Path / TargetID. Activity() derives a single human-readable label.
type Agent struct {
	ID      uint64
	Pos     mgl32.Vec3
	Heading float32
	Speed   float32

	// Pathfinder route from a lodge to a lift base. While Path is non-empty
	// and PathIdx is in-range the agent is walking the path.
	Path    [][2]int
	PathIdx int

	// Single goal: the entity (lift or building) the agent is heading
	// toward. 0 = idle. The simulation resolves the entity by ID — the same
	// ID space is used for buildings and lifts so this disambiguates itself
	// at lookup time.
	TargetID uint64

	// Implicit-state markers.
	OnLiftID  uint64 // nonzero ⇒ riding the named lift's chair (locomotion is suspended)
	Queued    bool   // in some lift.Queue, waiting to board
	Fallen    bool   // briefly immobilised after a fall; clears when FallTimer expires
	FallTimer float32

	// AI state — populated by sim package. The persistent types live in
	// internal/ai to avoid a sim ↔ world import cycle.
	Traits        ai.SkierTraits
	Plan          ai.Plan
	Balance       float32 // 1.0 fresh; ≤0 triggers a fall
	TurnSide      int8    // -1/0/+1; current carve-side commit (S-turn state)
	TurnDwell     float32 // seconds since the last TurnSide flip; the controller refuses to flip again until this exceeds turnDwellMin
	LastTactical  float32 // rad; previous tick's tactical lateral offset, used as a side-commit bias by the forward sampler so the skier doesn't flip-flop when both sides are clear

	// Energy is the session fatigue budget. 1.0 fresh; depletes only while
	// skiing (drained per-tick in tickSkier). When it falls below
	// energyLowThreshold (~one descent's worth) the next decision boundary
	// — lift unload (pickTopTarget) or skier arrival at a lift base
	// (onArrive) — reroutes the agent to a lodge, where they despawn.
	// Calibrated so a fresh skier completes roughly 20 descents before
	// heading home.
	Energy float32

	// Fun is the smoothed satisfaction signal the L0 GOAP planner reads
	// to weight goals. Rises on a fresh lift ride (novelty bonus on
	// unridden lifts, smaller bonus on repeats), decays slowly otherwise.
	// 0..1; clamped on writeback. Resort rating will EMA this at Depart
	// once GoHome lands.
	Fun float32

	// RidenLifts is the per-agent ride tally. The MVP novelty mechanic:
	// first ride of a lift is the biggest Fun bump, subsequent rides
	// taper. The planner reads this through goap.WorldSnapshot to weight
	// Explore and to compute RideLift cost. Stored as a flat slice (not
	// a map) so the planner's per-expansion Clone is a cheap slice copy
	// — see ai.RideCount for the why. Appended lazily on first ride of
	// a lift.
	RidenLifts []ai.RideCount

	// RestTimer counts down the atomic RestAtLodge action. While >0 the
	// agent is parked at a lodge recovering; on expiry Energy resets to
	// 1 and the plan advances. Sim-seconds (driven by TimeScale-scaled
	// dt). 0 outside of an active rest.
	RestTimer float32

	// Removed flags the terminal state set by the GOAP Depart action.
	// The per-tick dispatch skips Removed agents and reapDeparted
	// splices them out of the world after tickAgents completes, so
	// the range loop's slice header doesn't shift mid-pass.
	Removed bool

	// Events is the per-session log appended to by the sim (falls, run
	// completions). Read at depart by the demand system to feed the
	// resort-rating score. Append-only; cleared on agent removal.
	Events []ai.AgentEvent

	// Display-only snapshot of the last skiing tick's perception/intent.
	// Populated by sim.tickSkier; read by the follow HUD and the renderer's
	// perception-cone shader. Stale outside of skiing — gate on Activity.
	Sense ai.Sense

	// LastTrackPos is the agent's position at the most recent track-splat
	// substep, used so the surface-detail R-channel splatter can draw a
	// segment from previous→current rather than dot-stippling at high
	// speed / TimeScale. Zero before the first splat or after a state
	// reset (lift unload, fall recovery). Stored on Agent to keep the
	// substep cost O(1) per skier.
	LastTrackPos mgl32.Vec3
}

// Activity returns a short human-readable label describing what the agent is
// doing right now, derived from the implicit-state fields. Used by the follow
// HUD, debug overlays, the CSV recorder and the headless trace. The world is
// needed to resolve TargetID into a building-or-lift label.
func Activity(w *World, a *Agent) string {
	if a.Fallen {
		return "Fallen"
	}
	if a.OnLiftID != 0 {
		return "On Lift"
	}
	if a.Queued {
		return "Queuing"
	}
	if len(a.Path) > 0 && a.PathIdx < len(a.Path) {
		return "Walking"
	}
	if a.TargetID == 0 {
		return "Idle"
	}
	for _, b := range w.Buildings {
		if b.ID == a.TargetID {
			return "Departing"
		}
	}
	for _, l := range w.Lifts {
		if l.ID == a.TargetID {
			return "To Lift"
		}
	}
	return "Traveling"
}
