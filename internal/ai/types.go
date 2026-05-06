// Package ai holds the persistent skier-AI types — anything that must live on
// world.Agent across ticks. Per-tick computation types (Perception, Intent,
// MotorCmd) stay in internal/sim because they're transient inputs/outputs
// of the steering pipeline. Keeping persistent types in this leaf package
// breaks the import cycle between world and sim.
package ai

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// =============================================================================
// SKILL & TECHNIQUE
// =============================================================================

// SkillLevel is the gross category of a skier's ability. It maps to a
// TechniqueSet that bounds which motor commands the AI may issue.
type SkillLevel int

const (
	SkillBeginner SkillLevel = iota
	SkillIntermediate
	SkillAdvanced
)

// Technique enumerates discrete skiing motor patterns. Each is implemented in
// the motor layer with a distinct heading-control / scrub-rate / balance-cost
// profile.
type Technique uint8

const (
	TechStraight  Technique = iota // schuss; no scrub, no oscillation
	TechPizza                      // wedge / snowplow; constant scrub, no turn
	TechWedgeTurn                  // pizza-based direction change
	TechParallel                   // linked S-turns; arc width modulates speed
	TechHockey                     // hard 90° edge-set; brief, expensive
	TechSideslip                   // perpendicular descent; low speed, high control
)

// TechniqueSet is a bitmask of Techniques the skier is allowed to use.
type TechniqueSet uint32

// Has reports whether t is in the set.
func (s TechniqueSet) Has(t Technique) bool {
	return s&(1<<uint(t)) != 0
}

// KitFor returns the canonical TechniqueSet for a skill level. Beginners can
// only pizza and straight-line; intermediate adds parallel turns and sideslip;
// advanced adds the hockey stop.
func KitFor(level SkillLevel) TechniqueSet {
	base := TechniqueSet(1<<TechStraight | 1<<TechPizza | 1<<TechWedgeTurn)
	if level == SkillBeginner {
		return base
	}
	base |= TechniqueSet(1<<TechParallel | 1<<TechSideslip)
	if level == SkillIntermediate {
		return base
	}
	return base | TechniqueSet(1<<TechHockey)
}

// SkierTraits captures the per-skier inputs the AI reads. All transient AI
// state (Route, MotorState, Balance) lives in separate fields on the Agent.
type SkierTraits struct {
	Skill        SkillLevel
	Techniques   TechniqueSet
	ComfortSpeed float32 // m/s above which stress accumulates
	ComfortSlope float32 // radians; steeper than this is uncomfortable
	Aggression   float32 // 0..1; scales target speed up
	SightRange   float32 // metres; perception cone length
}

// TraitsFor returns sensible defaults for a skill level. Callers can mutate
// the returned struct for per-skier variation.
func TraitsFor(level SkillLevel) SkierTraits {
	switch level {
	case SkillBeginner:
		return SkierTraits{
			Skill:        SkillBeginner,
			Techniques:   KitFor(SkillBeginner),
			ComfortSpeed: 5,
			ComfortSlope: 10 * math.Pi / 180,
			Aggression:   0.2,
			SightRange:   15,
		}
	case SkillIntermediate:
		return SkierTraits{
			Skill:        SkillIntermediate,
			Techniques:   KitFor(SkillIntermediate),
			ComfortSpeed: 10,
			ComfortSlope: 20 * math.Pi / 180,
			Aggression:   0.5,
			SightRange:   25,
		}
	default:
		return SkierTraits{
			Skill:        SkillAdvanced,
			Techniques:   KitFor(SkillAdvanced),
			ComfortSpeed: 16,
			ComfortSlope: 30 * math.Pi / 180,
			Aggression:   0.8,
			SightRange:   35,
		}
	}
}

// =============================================================================
// ROUTE
// =============================================================================

// GoalKind is the high-level intent of a Route.
type GoalKind int

const (
	GoalNone GoalKind = iota
	GoalLift
	GoalLodge
)

// Route is the strategic plan: where the skier is heading. Updated rarely.
// Path-finding around signs / hazards is a future extension that will populate
// additional fields here without changing the lower layers' contract.
type Route struct {
	Goal    GoalKind
	GoalID  uint64     // entity id of the destination (lift or lodge)
	GoalPos mgl32.Vec3 // world-space target position
	StaleAt float64    // sim-time when this route should be replanned
}

// =============================================================================
// MOTOR
// =============================================================================

// MotorState is the persistent state of the technique dispatcher: which
// technique is currently engaged and how far through its cycle we are.
type MotorState struct {
	Active    Technique
	TurnPhase int8    // -1 / 0 / +1; current side of an oscillating turn
	PhaseTime float32 // seconds elapsed since the last technique/phase change
}
