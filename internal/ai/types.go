// Package ai holds the persistent skier-AI types — anything that must live
// on world.Agent across ticks. Per-tick computation types (Perception,
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

// SkierTraits captures the per-skier inputs the controller reads.
type SkierTraits struct {
	Skill        SkillLevel
	ComfortSpeed float32 // m/s; above ~comfort the brake controller engages
	ComfortSlope float32 // radians; steeper than this is uncomfortable
	Aggression   float32 // 0..1; scales target speed up
}

// TraitsFor returns sensible defaults for a skill level. Callers can mutate
// the returned struct for per-skier variation.
func TraitsFor(level SkillLevel) SkierTraits {
	switch level {
	case SkillBeginner:
		return SkierTraits{
			Skill:        SkillBeginner,
			ComfortSpeed: 5,
			ComfortSlope: 10 * math.Pi / 180,
			Aggression:   0.2,
		}
	case SkillIntermediate:
		return SkierTraits{
			Skill:        SkillIntermediate,
			ComfortSpeed: 10,
			ComfortSlope: 20 * math.Pi / 180,
			Aggression:   0.5,
		}
	default:
		return SkierTraits{
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

// GoalKind labels what kind of entity the Plan is heading at.
type GoalKind int

const (
	GoalNone GoalKind = iota
	GoalLift
	GoalLodge
)

// Plan is the strategic layer's output: where the skier is heading and
// what they care about. Refreshed on goal change (lift unload, arrival),
// not per-tick. Prefs is reserved for future exploration / lift selection /
// conditions logic.
type Plan struct {
	Goal   GoalKind
	GoalID uint64
	Target mgl32.Vec3
	Prefs  Prefs
}

// Prefs is the slot for future strategic preferences (preferred steepness,
// glade tolerance, exploration bias, conditions). Empty for now.
type Prefs struct{}

// =============================================================================
// SENSE SNAPSHOT
// =============================================================================

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
