// Package ai holds the persistent guest-AI types — anything that must live
// on world.Guest across ticks. Per-tick computation types (Perception,
// Decision) stay in internal/sim because they're transient inputs/outputs
// of the controller. Keeping persistent types in this leaf package breaks
// the import cycle between world and sim.
package ai

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// =============================================================================
// SKILL & TRAITS
// =============================================================================

// SkillLevel is the gross category of a skier's ability. The Plan-A
// controller is the same shape for every skier; skill differentiates only
// through ComfortSpeed / ComfortSlope / Aggression. There is no per-skill
// technique whitelist anymore — the same controller produces straight,
// carved, and brake-heavy outputs as the situation demands.
type SkillLevel int

const (
	SkillBeginner SkillLevel = iota
	SkillIntermediate
	SkillAdvanced
)

// String returns a human-readable label for HUD / debug overlays.
func (l SkillLevel) String() string {
	switch l {
	case SkillBeginner:
		return "Beginner"
	case SkillIntermediate:
		return "Intermediate"
	case SkillAdvanced:
		return "Advanced"
	}
	return "Unknown"
}

// GuestTraits captures the per-guest inputs the controller reads.
type GuestTraits struct {
	Skill        SkillLevel
	ComfortSpeed float32 // m/s; above ~comfort the brake controller engages
	ComfortSlope float32 // radians; steeper than this is uncomfortable
	Aggression   float32 // 0..1; scales target speed up
}

// TraitsFor returns sensible defaults for a skill level. Callers can mutate
// the returned struct for per-skier variation.
func TraitsFor(level SkillLevel) GuestTraits {
	switch level {
	case SkillBeginner:
		return GuestTraits{
			Skill:        SkillBeginner,
			ComfortSpeed: 5,
			ComfortSlope: 10 * math.Pi / 180,
			Aggression:   0.2,
		}
	case SkillIntermediate:
		return GuestTraits{
			Skill:        SkillIntermediate,
			ComfortSpeed: 10,
			ComfortSlope: 20 * math.Pi / 180,
			Aggression:   0.5,
		}
	default:
		return GuestTraits{
			Skill:        SkillAdvanced,
			ComfortSpeed: 16,
			ComfortSlope: 30 * math.Pi / 180,
			Aggression:   0.8,
		}
	}
}

// =============================================================================
// PLAN
// =============================================================================

// GoalKind labels what kind of entity the Plan is heading at. This is the
// L1 hint — what the continuous controller is steering toward right now,
// derived from the head L0 step's destination.
type GoalKind int

const (
	GoalNone GoalKind = iota
	GoalLift
	GoalDepart // heading to a parking lot / bus stop / train station to leave the resort
)

// PlanActionKind tags an L0 plan step so the simulation can drive
// locomotion off it without importing the GOAP package's action types
// (which would cycle: world → goap → world). One enum value per concrete
// goap.Action implementation.
type PlanActionKind uint8

const (
	ActNone PlanActionKind = iota
	ActWalkToLift
	ActJoinQueue
	ActRideLift
	ActSkiToLift
	ActSkiToLodge
	ActSkiToParking
	ActRestAtLodge
	ActDepart
)

// PlanAction is one step in the stored L0 plan — plain data, no behaviour.
// Either LiftID or BldgID is set depending on Kind; the runtime resolves
// the entity at execute time. Cost is the planner's cost-at-emission for
// HUD display.
type PlanAction struct {
	Kind   PlanActionKind
	LiftID uint64
	BldgID uint64
	Cost   float32
}

// Plan is the per-agent strategic layer state. Steps is the L0 plan
// produced by the GOAP planner; Step indexes the current head action.
// Target / Goal / GoalID are the L1 hint — set once by the simulation
// when a step starts, never per-tick. Replan triggers (plan empty, head
// done, head precondition broken, periodic safety check) regenerate
// Steps; the L1 controller never re-reads strategic state mid-tick.
type Plan struct {
	Goal     GoalKind
	GoalID   uint64
	Target   mgl32.Vec3
	GoalName string // L0 goal name for HUD ("Explore" / "Rest" / ...)
	Steps    []PlanAction
	Step     int
	Prefs    Prefs
}

// Done reports whether the plan is exhausted — no steps or the cursor has
// advanced past the last one. The simulation re-plans when this is true.
func (p *Plan) Done() bool {
	return len(p.Steps) == 0 || p.Step >= len(p.Steps)
}

// Head returns the current head action, or the zero PlanAction (Kind =
// ActNone) if the plan is done.
func (p *Plan) Head() PlanAction {
	if p.Done() {
		return PlanAction{}
	}
	return p.Steps[p.Step]
}

// Prefs is the slot for future strategic preferences (preferred steepness,
// glade tolerance, exploration bias, conditions). Empty for now.
type Prefs struct{}

// RideCount is one entry in an agent's per-lift ride tally. Stored as a
// flat slice rather than a map because the GOAP planner clones the
// per-agent snapshot on every A* node expansion — at ~150 agents
// replanning across substeps that turned into tens of thousands of map
// allocations per tick and froze the main loop for seconds. With ≤10
// lifts a linear scan is faster than a map hash lookup anyway.
type RideCount struct {
	LiftID uint64
	Count  int
}

// RideCountOf returns the ride count for liftID in rides, or 0 if absent.
func RideCountOf(rides []RideCount, liftID uint64) int {
	for i := range rides {
		if rides[i].LiftID == liftID {
			return rides[i].Count
		}
	}
	return 0
}

// AddRide increments the count for liftID in rides, appending a new
// entry if liftID isn't present yet. Returns the (possibly grown) slice.
func AddRide(rides []RideCount, liftID uint64) []RideCount {
	for i := range rides {
		if rides[i].LiftID == liftID {
			rides[i].Count++
			return rides
		}
	}
	return append(rides, RideCount{LiftID: liftID, Count: 1})
}

// =============================================================================
// SENSE SNAPSHOT
// =============================================================================

// =============================================================================
// AGENT EVENTS
// =============================================================================

// GuestEventKind tags a recordable per-agent occurrence. The event log
// is read at depart time by the demand system to feed the resort-rating
// score (cleanness = function of falls vs runs).
type GuestEventKind uint8

const (
	EventFall GuestEventKind = iota // L1 controller detected Balance ≤ 0
	EventRun                        // agent completed a descent (any ActSkiTo*)
)

// GuestEvent is one row in an agent's per-session log. Time is sim-time
// at emission. Storage is a flat slice on the agent — appended only,
// inspected at depart, then garbage-collected with the agent.
type GuestEvent struct {
	Kind GuestEventKind
	Time float64
}

// Sense is a per-tick snapshot of the controller used by the renderer and
// follow HUD. Display-only; the AI never reads this back. Stale outside
// active skiing — readers should gate on Activity.
type Sense struct {
	ProbeDist      float32 // m; forward-sampling horizon at current speed
	ProbeHalfAngle float32 // rad; outermost sampled offset
	ProbeC         float32 // density along centre sample
	ProbeR         float32 // density along right-most sample
	ProbeL         float32 // density along left-most sample

	AxisHeading    float32 // composed axis (target+fall blend), radians
	DesiredHeading float32 // controller output heading, radians
	TargetSpeed    float32 // m/s; controller's desired speed
	Brake          float32 // commanded brake angle (rad); >0 = carving to scrub
	TurnSide       int8    // -1/0/+1; current carve-side commit
	Mode           string  // human-readable: "straight"/"carve"/"brake"

	InTrees       bool
	AtCellDensity float32
}
