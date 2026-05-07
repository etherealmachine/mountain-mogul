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
	Traits     ai.SkierTraits
	Route      ai.Route
	Motor      ai.MotorState
	Avoid      ai.AvoidState
	Balance    float32 // 1.0 fresh; ≤0 triggers a fall
	Confidence float32 // baseline 1.0; multiplier on target speed; clamped [0.5, 1.5]

	// Display-only snapshot of the last skiing tick's perception/intent.
	// Populated by sim.tickSkier; read by the follow HUD and the renderer's
	// perception-cone shader. Stale outside of skiing — gate on Activity.
	Sense ai.Sense

	// Sim-seconds the skier has been "in trees and barely moving." When this
	// crosses stuckTriggerS the skiing pipeline gives up and sets a walk
	// path to the nearest clear cell.
	StuckTimer float32
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
			return "To Lodge"
		}
	}
	for _, l := range w.Lifts {
		if l.ID == a.TargetID {
			return "To Lift"
		}
	}
	return "Traveling"
}
