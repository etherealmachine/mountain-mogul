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

// Skill tier boundaries. 0..SkillAdvancedThreshold is beginner or
// intermediate; SkillAdvancedThreshold..1 is advanced.
const (
	SkillIntermediateThreshold = float32(0.33)
	SkillAdvancedThreshold     = float32(0.66)
)

// SkillTierName returns a human-readable tier label for HUD / debug overlays.
func SkillTierName(skill float32) string {
	switch {
	case skill < SkillIntermediateThreshold:
		return "Beginner"
	case skill < SkillAdvancedThreshold:
		return "Intermediate"
	default:
		return "Advanced"
	}
}

// GuestTraits captures the per-guest inputs the controller reads.
// Boolean preferences are coarse-grained for now (likes / doesn't);
// fractional or per-axis preferences land if we need finer behaviour.
type GuestTraits struct {
	Skill        float32 // 0..1; 0–0.33 beginner, 0.33–0.66 intermediate, 0.66+ advanced
	ComfortSpeed float32 // m/s; above ~comfort the brake controller engages
	ComfortSlope float32 // radians; steeper than this is uncomfortable
	Aggression   float32 // 0..1; scales target speed up

	// LikesGlades: true ⇒ time in trees emits ThoughtLovingGlades
	// (positive). False ⇒ emits ThoughtScaredInTrees (negative).
	LikesGlades bool

	// PrefersGroomed: true ⇒ groomed snow emits ThoughtLovingCorduroy (positive).
	PrefersGroomed bool
}

// TraitsFor returns sensible defaults for a skill value in [0, 1]. Callers
// can mutate the returned struct for per-skier variation.
func TraitsFor(skill float32) GuestTraits {
	switch {
	case skill < SkillIntermediateThreshold:
		return GuestTraits{
			Skill:          skill,
			ComfortSpeed:   5,
			ComfortSlope:   10 * math.Pi / 180,
			Aggression:     0.2,
			PrefersGroomed: true,
		}
	case skill < SkillAdvancedThreshold:
		return GuestTraits{
			Skill:          skill,
			ComfortSpeed:   10,
			ComfortSlope:   20 * math.Pi / 180,
			Aggression:     0.5,
			PrefersGroomed: true,
		}
	default:
		return GuestTraits{
			Skill:        skill,
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
	ActSkiTrail // ski a player-defined trail from one entity to another
)

// PlanAction is one step in the stored L0 plan — plain data, no behaviour.
// For ActSkiTrail: TrailID is the via-trail (also the destination for
// trail-to-trail steps); LiftID is the destination lift base (if any);
// BldgID is the destination building (if any). At most one of LiftID /
// BldgID is non-zero per step.
type PlanAction struct {
	Kind    PlanActionKind
	LiftID  uint64
	BldgID  uint64
	TrailID uint64 // ActSkiTrail: via trail (= destination for trail-to-trail)
	Cost    float32
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
	EventFall   GuestEventKind = iota // L1 controller detected Balance ≤ 0
	EventRun                          // agent completed a descent (any ActSkiTo* or ActSkiTrail)
	EventInjury                        // fall severe enough to require rescue
)

// GuestEvent is one row in an agent's per-session log. Time is sim-time
// at emission. Storage is a flat slice on the agent — appended only,
// inspected at depart, then garbage-collected with the agent.
type GuestEvent struct {
	Kind GuestEventKind
	Time float64
}

// =============================================================================
// THOUGHTS — RCT-style "what's this guest thinking" surface
// =============================================================================

// ThoughtKind enumerates the canned thoughts a guest can have in
// response to game events. Keep the catalogue small and high-signal —
// every entry needs a clear Display() string and a trigger somewhere in
// the sim. The HUD reads the most-recent unexpired thought via
// Guest.CurrentThought.
type ThoughtKind uint8

const (
	ThoughtNone ThoughtKind = iota

	// Glade-trait reactions to TreeDensity > threshold.
	ThoughtLovingGlades  // LikesGlades = true, in trees
	ThoughtScaredInTrees // LikesGlades = false, in trees

	// Grooming-trait reactions to cell Grooming.
	ThoughtLovingCorduroy // PrefersGroomed = true, on groomed snow

	// Skill events.
	ThoughtFell      // Balance went to 0; fall recovery
	ThoughtInjured   // injured; stuck until rescued
	ThoughtAbandoned // gave up waiting for help; crawling home

	// Queue events.
	ThoughtLongLine    // Joining a queue whose estimated wait is long
	ThoughtLineTooLong // Queue wait would exhaust patience; guest departs instead

	// Planning events.
	ThoughtNeedsLodge // Rest goal selected but no lodge reachable; fell back to next goal

	// Hunger / thirst events.
	ThoughtHungry  // Hunger critically low; guest departs
	ThoughtThirsty // Thirst critically low; guest departs

	thoughtKindSentinel // must stay last; equals the total count
)

// ThoughtKindCount is the number of distinct ThoughtKind values (including
// ThoughtNone). Use this to size arrays indexed by ThoughtKind.
const ThoughtKindCount = int(thoughtKindSentinel)

// ThoughtSatisfactionWeight is the signed satisfaction impact of each thought
// kind. Positive = improves guest satisfaction, negative = reduces it.
// Terrain thoughts use the drift-target deviation from the neutral 0.5 baseline;
// discrete events use the exact delta applied at the moment the thought fires.
var ThoughtSatisfactionWeight = [ThoughtKindCount]float64{
	ThoughtLovingGlades:   +0.12,
	ThoughtScaredInTrees:  -0.18,
	ThoughtLovingCorduroy: +0.15,
	ThoughtFell:           -0.10,
	ThoughtInjured:        -0.25,
	ThoughtAbandoned:      -0.30,
	ThoughtLongLine:       -0.08,
	ThoughtLineTooLong:    -0.08,
	ThoughtNeedsLodge: -0.06,
	ThoughtHungry:     -0.08,
	ThoughtThirsty:    -0.08,
}

// ThoughtLabel is the short chart label for each thought kind. An empty
// string means the thought is intentionally excluded from charts (only
// ThoughtNone). Any new ThoughtKind MUST get a non-empty label here so
// it automatically appears in both the in-resort and exit-thought charts.
var ThoughtLabel = [ThoughtKindCount]string{
	ThoughtLovingGlades:   "Loving glades",
	ThoughtScaredInTrees:  "Scared in trees",
	ThoughtLovingCorduroy: "Loving corduroy",
	ThoughtFell:           "Fell",
	ThoughtInjured:        "Injured",
	ThoughtAbandoned:      "Abandoned",
	ThoughtLongLine:       "Long line",
	ThoughtLineTooLong:    "Line too long",
	ThoughtNeedsLodge: "Needs lodge",
	ThoughtHungry:     "Hungry",
	ThoughtThirsty:    "Thirsty",
}

// ThoughtChartColor is the RGBA bar colour for each thought kind in charts.
// Must match ThoughtLabel: every entry with a non-empty label needs a colour.
var ThoughtChartColor = [ThoughtKindCount][4]float32{
	ThoughtLovingGlades:   {0.30, 0.75, 0.40, 1},
	ThoughtScaredInTrees:  {0.90, 0.35, 0.30, 1},
	ThoughtLovingCorduroy: {0.45, 0.85, 0.55, 1},
	ThoughtFell:           {0.85, 0.20, 0.20, 1},
	ThoughtInjured:        {0.95, 0.10, 0.10, 1},
	ThoughtAbandoned:      {0.60, 0.10, 0.80, 1},
	ThoughtLongLine:       {0.80, 0.45, 0.70, 1},
	ThoughtLineTooLong:    {0.70, 0.30, 0.60, 1},
	ThoughtNeedsLodge: {0.60, 0.50, 0.80, 1},
	ThoughtHungry:     {0.95, 0.60, 0.20, 1},
	ThoughtThirsty:    {0.25, 0.65, 0.90, 1},
}

// Thought is one entry in a Guest's bounded thoughts ring. Persists in
// the ring until either ThoughtTTL expires it or another thought
// displaces it past the ring's capacity.
type Thought struct {
	Kind    ThoughtKind
	Time    float64  // sim-time at emission; for TTL + display ordering
	Context []uint64 // entity IDs for format-string substitution (lift, trail, etc.)
}

// Display returns the player-readable phrasing of the thought with entity
// names substituted in. resolve maps an entity ID to a human-readable name
// (e.g. lift name, trail name); it is called once per Context slot. If an
// ID resolves to an empty string the slot is omitted from the output.
func (t Thought) Display(resolve func(uint64) string) string {
	name := func(i int) string {
		if i < len(t.Context) && t.Context[i] != 0 {
			if s := resolve(t.Context[i]); s != "" {
				return s
			}
		}
		return ""
	}
	switch t.Kind {
	case ThoughtLovingGlades:
		return "loving these glades"
	case ThoughtScaredInTrees:
		return "too many trees!"
	case ThoughtLovingCorduroy:
		return "this corduroy is perfect"
	case ThoughtFell:
		if n := name(0); n != "" {
			return "ouch, that hurt on " + n
		}
		return "ouch, that hurt"
	case ThoughtInjured:
		if n := name(0); n != "" {
			return "I'm hurt on " + n + ", I can't move"
		}
		return "I'm hurt, I can't move"
	case ThoughtAbandoned:
		return "no one came to help me"
	case ThoughtLongLine:
		if n := name(0); n != "" {
			return "the " + n + " line is way too long"
		}
		return "this line is way too long"
	case ThoughtLineTooLong:
		if n := name(0); n != "" {
			return "the " + n + " line will take forever"
		}
		return "that line will take forever"
	case ThoughtNeedsLodge:
		return "this place needs a lodge"
	case ThoughtHungry:
		return "I could really use a meal"
	case ThoughtThirsty:
		return "I need something to drink"
	}
	return ""
}

// ThoughtTTL is the sim-time window during which a thought counts as
// "currently on the guest's mind." Older entries are skipped by
// CurrentThought even if they're still inside the ring.
const ThoughtTTL = 12.0

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
