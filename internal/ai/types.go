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
type GuestTraits struct {
	Skill        float32 // 0..1; 0–0.33 beginner, 0.33–0.66 intermediate, 0.66+ advanced
	ComfortSpeed float32 // m/s; above ~comfort the brake controller engages
	ComfortSlope float32 // radians; steeper than this is uncomfortable
	Aggression   float32 // 0..1; scales target speed up

	// Tastes is what snow and terrain the guest enjoys, separate from
	// what their skill lets them handle (Snow Tastes).
	Tastes Tastes

	// DailyBudget is the total amount a guest is willing to spend per visit.
	// Derived from skill at pool creation ($40 + skill×$160); campaigns may
	// override it per-scenario to model different demographics.
	DailyBudget float32
}

// TraitsFor returns sensible defaults for a skill value in [0, 1]. Callers
// can mutate the returned struct for per-skier variation.
func TraitsFor(skill float32) GuestTraits {
	switch {
	case skill < SkillIntermediateThreshold:
		return GuestTraits{
			Skill:        skill,
			ComfortSpeed: 5,
			ComfortSlope: 10 * math.Pi / 180,
			Aggression:   0.2,
			Tastes:       Archetypes[ArchetypeCruiser].Centre,
		}
	case skill < SkillAdvancedThreshold:
		return GuestTraits{
			Skill:        skill,
			ComfortSpeed: 10,
			ComfortSlope: 20 * math.Pi / 180,
			Aggression:   0.5,
			Tastes:       Archetypes[ArchetypeCruiser].Centre,
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
// TASTES
// =============================================================================

// TasteKind indexes Tastes: one kind of snow or terrain.
type TasteKind uint8

const (
	TasteGroomed TasteKind = iota // corduroy
	TastePowder                   // fresh, deep snow
	TasteMoguls                   // bumps
	TasteTrees                    // glades
	TasteSteep                    // steep pitches for the trail's grade
	TasteIce                      // boilerplate, crust, frozen granular
	TasteCrowds                   // other skiers close by
	TasteCount
)

// TasteName is each taste's short label for the follow panel.
var TasteName = [TasteCount]string{"groomed", "powder", "moguls", "trees", "steep", "ice", "crowds"}

// Tastes is how much a guest enjoys each kind of snow and terrain, from
// -1 (hates it) to +1 (loves it). Rolled per guest around an archetype
// (world.RollTastes); TasteLabel names the nearest one for the player.
type Tastes [TasteCount]float32

// Until snow underfoot reads tastes directly (Snow Tastes step 2), the
// older glade and corduroy reactions key off these thresholds.
const (
	gladeLoverTaste   = 0.4
	groomedLoverTaste = 0.3
)

// LikesGlades reports whether time in the trees is a pleasure (loving
// these glades) rather than a fright (too many trees).
func (t Tastes) LikesGlades() bool { return t[TasteTrees] >= gladeLoverTaste }

// PrefersGroomed reports whether the guest seeks out corduroy.
func (t Tastes) PrefersGroomed() bool { return t[TasteGroomed] >= groomedLoverTaste }

// ArchetypeKind indexes Archetypes.
type ArchetypeKind uint8

const (
	ArchetypeCruiser ArchetypeKind = iota
	ArchetypePowderHound
	ArchetypeBumpSkier
	ArchetypeGladeRat
	ArchetypeCharger
	ArchetypeCount
)

// Archetype is a kind of skier: the tastes guests of that kind gather
// around, and how common they are among beginners, intermediates, and
// advanced guests (relative weights within each tier).
type Archetype struct {
	Name   string
	Centre Tastes
	Share  [3]float32
}

// Archetypes are the kinds of skier guests are rolled around. Centres
// list groomed, powder, moguls, trees, steep, ice, crowds.
var Archetypes = [ArchetypeCount]Archetype{
	ArchetypeCruiser:     {"Cruiser", Tastes{+0.8, -0.4, -0.6, -0.5, -0.5, -0.6, -0.2}, [3]float32{0.80, 0.50, 0.15}},
	ArchetypePowderHound: {"Powder Hound", Tastes{-0.4, +0.9, 0, +0.4, +0.3, -0.7, -0.7}, [3]float32{0.02, 0.10, 0.30}},
	ArchetypeBumpSkier:   {"Bump Skier", Tastes{-0.3, +0.1, +0.9, 0, +0.4, -0.3, -0.2}, [3]float32{0.05, 0.20, 0.20}},
	ArchetypeGladeRat:    {"Glade Rat", Tastes{-0.2, +0.5, +0.1, +0.9, +0.2, -0.5, -0.6}, [3]float32{0.03, 0.10, 0.20}},
	ArchetypeCharger:     {"Charger", Tastes{+0.5, +0.2, -0.5, -0.3, +0.9, -0.1, -0.4}, [3]float32{0.10, 0.10, 0.15}},
}

// TasteLabel is the archetype nearest t: how the player sees a guest's
// tastes. The sim never reads it.
func TasteLabel(t Tastes) string {
	best, bestD := ArchetypeCruiser, float32(math.MaxFloat32)
	for k, a := range Archetypes {
		var d float32
		for i := range t {
			diff := t[i] - a.Centre[i]
			d += diff * diff
		}
		if d < bestD {
			best, bestD = ArchetypeKind(k), d
		}
	}
	return Archetypes[best].Name
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
	GoalDepart        // heading to a parking lot / bus stop / train station to leave the resort
	GoalRelieveThirst // heading to a bar to drink something
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
	ActSkiTrail           // ski a player-defined trail from one entity to another
	ActWalkToTicketOffice // walk to the ticket office building
	ActBuySeasonPass      // purchase a season pass at the ticket office
	ActRelieveThirst      // stop at a bar to drink something
	ActBuyDayTicket       // buy today's day ticket at the ticket office
	ActEat                // buy a meal at a lodge food court
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
	// Needs already pressing when the plan was made. A need that turns
	// pressing mid-plan can preempt it at the next step boundary.
	Pressing NeedMask
	// Blocked lists the conditions that kept higher-ranked goals from
	// planning (no lodge to rest in, no lift to ride). The planner only
	// reports them; the sim turns them into condition thoughts.
	Blocked []ThoughtKind
}

// NeedMask is a set of bodily needs (hunger, thirst).
type NeedMask uint8

const (
	NeedHunger NeedMask = 1 << iota
	NeedThirst
)

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
	EventInjury                       // fall severe enough to require rescue
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

// ThoughtKind enumerates the canned thoughts a guest can have. A thought
// is a read-only report of something that changed the guest's stats: an
// event (Effects[k].Condition false) or a condition that holds for a
// while (true). Every entry needs an Effects row, a thoughtText string,
// and a chart colour. The HUD reads the most-recent current thought via
// Guest.CurrentThought.
type ThoughtKind uint8

const (
	ThoughtNone ThoughtKind = iota

	// Glade-trait reactions to the tree cover underfoot.
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

	// Fatigue conditions.
	ThoughtTired     // Energy below the rest threshold; needs a lodge
	ThoughtExhausted // min(Patience,Energy) critically low; guest departs
	ThoughtImpatient // Patience below the rest threshold; fed up with waiting

	// Budget events.
	ThoughtTooExpensive   // RemainingBudget < cheapest lift ticket; guest departs
	ThoughtNoTicketWindow // no reachable ticket office to buy a day ticket; guest departs

	// Tree events.
	ThoughtHitTree // skied into a tree trunk and fell

	// Arrival events: the guest came but can't ski, and goes home.
	ThoughtLiftsClosed  // no lift is open
	ThoughtNothingForMe // no open lift serves a trail at the guest's level

	// A minor injury: the guest gets themselves down and goes home early.
	ThoughtHurtGoingHome

	// Knocked over by avalanche debris; a fall or injury thought follows.
	ThoughtCaughtInAvalanche

	// Terrain against skill.
	ThoughtTooEasy       // a run mostly below their level; holds until one at or above it
	ThoughtGreatRun      // a run at their level, with real vertical, no fall, and room to ski
	ThoughtTooHard       // a run above their level, or much of it past their comfort slope
	ThoughtCrowdedRun    // a run with other skiers close around them much of the way
	ThoughtFellUnloading // fell getting off a chair at the top
	ThoughtLinesFull     // every lift line they'd use is longer than they'll join

	// Services.
	ThoughtGoodMeal   // finished a meal at a food court
	ThoughtGoodDrink  // finished a drink at a bar
	ThoughtRested     // finished a rest at a lounge or food court
	ThoughtPatrolFast // patrol reached them quickly
	ThoughtPatrolCame // patrol reached them in a reasonable time
	ThoughtPatrolSlow // patrol took a long time to reach them

	thoughtKindSentinel // must stay last; equals the total count
)

// ThoughtKindCount is the number of distinct ThoughtKind values (including
// ThoughtNone). Use this to size arrays indexed by ThoughtKind.
const ThoughtKindCount = int(thoughtKindSentinel)

// ConditionMask has one bit per ThoughtKind.
const _ = uint(64 - ThoughtKindCount) // fails to compile past 64 kinds

// Effect is what one ThoughtKind reports. An event's Satisfaction is a
// one-off delta, applied once by sim.applyEvent, and its Baseline moves
// the guest's baseline for the rest of the visit: the level their mood
// drifts back to. A condition's Satisfaction is a pull on the mood
// target that lasts while the condition holds; the condition's thought
// is added once when it starts.
type Effect struct {
	Condition    bool
	Satisfaction float32
	Baseline     float32
}

// Effects is the single table of what every thought reports. Nothing
// else holds event deltas or condition pulls.
var Effects = [ThoughtKindCount]Effect{
	// Events.
	ThoughtFell:              {Satisfaction: -0.10},
	ThoughtInjured:           {Satisfaction: -0.25, Baseline: -0.05},
	ThoughtAbandoned:         {Satisfaction: -0.30, Baseline: -0.10},
	ThoughtHurtGoingHome:     {Satisfaction: -0.15, Baseline: -0.03},
	ThoughtHitTree:           {Satisfaction: -0.15},
	ThoughtCaughtInAvalanche: {Satisfaction: -0.10, Baseline: -0.03},
	ThoughtLongLine:          {Satisfaction: -0.08},
	ThoughtLineTooLong:       {Satisfaction: -0.08},
	ThoughtLovingCorduroy:    {},                                     // why a run was great (sim.judgeRun): a report, no effect of its own
	ThoughtGreatRun:          {Satisfaction: +0.04, Baseline: +0.04}, // Baseline scaled down by repeats (sim.judgeRun)
	ThoughtTooHard:           {Satisfaction: -0.08, Baseline: -0.02},
	ThoughtCrowdedRun:        {Satisfaction: -0.05},
	ThoughtFellUnloading:     {Satisfaction: -0.05},
	ThoughtGoodMeal:          {Satisfaction: +0.05},
	ThoughtGoodDrink:         {Satisfaction: +0.04},
	ThoughtRested:            {Satisfaction: +0.03},
	ThoughtPatrolFast:        {Satisfaction: +0.06},
	ThoughtPatrolCame:        {Satisfaction: +0.02},
	ThoughtPatrolSlow:        {Satisfaction: -0.08, Baseline: -0.03},

	// Conditions. Zero-pull conditions report a reason to leave.
	ThoughtLovingGlades:   {Condition: true, Satisfaction: +0.12},
	ThoughtScaredInTrees:  {Condition: true, Satisfaction: -0.18},
	ThoughtHungry:         {Condition: true, Satisfaction: -0.10},
	ThoughtThirsty:        {Condition: true, Satisfaction: -0.10},
	ThoughtImpatient:      {Condition: true, Satisfaction: -0.10},
	ThoughtNeedsLodge:     {Condition: true, Satisfaction: -0.10},
	ThoughtTooEasy:        {Condition: true, Satisfaction: -0.08},
	ThoughtTired:          {Condition: true},
	ThoughtExhausted:      {Condition: true},
	ThoughtTooExpensive:   {Condition: true},
	ThoughtNoTicketWindow: {Condition: true},
	ThoughtLiftsClosed:    {Condition: true},
	ThoughtNothingForMe:   {Condition: true},
	ThoughtLinesFull:      {Condition: true},
}

// ConditionMask is the set of condition thoughts currently holding for a
// guest, one bit per ThoughtKind.
type ConditionMask uint64

// Has reports whether condition k is active.
func (m ConditionMask) Has(k ThoughtKind) bool { return m&(1<<k) != 0 }

// Set turns condition k on or off.
func (m *ConditionMask) Set(k ThoughtKind, on bool) {
	if on {
		*m |= 1 << k
	} else {
		*m &^= 1 << k
	}
}

// thoughtText is the canonical base text for each ThoughtKind — the
// fallback used by Display() when no entity context is available, and
// the source for ThoughtLabel (wrapped in quotes). Define each thought
// string exactly once here; Display() and ThoughtLabel are both derived
// from it. A new ThoughtKind only needs an entry added here.
var thoughtText = [ThoughtKindCount]string{
	ThoughtLovingGlades:      "loving these glades",
	ThoughtScaredInTrees:     "too many trees!",
	ThoughtLovingCorduroy:    "this corduroy is perfect",
	ThoughtFell:              "ouch, that hurt",
	ThoughtInjured:           "I'm hurt, I can't move",
	ThoughtAbandoned:         "no one came to help me",
	ThoughtLongLine:          "this line is way too long",
	ThoughtLineTooLong:       "that line will take forever",
	ThoughtNeedsLodge:        "this place needs a lodge",
	ThoughtHungry:            "I could really use a meal",
	ThoughtThirsty:           "I need something to drink",
	ThoughtTired:             "I need a break",
	ThoughtExhausted:         "I'm too tired to ski",
	ThoughtImpatient:         "I'm sick of waiting around",
	ThoughtTooExpensive:      "I can't afford this",
	ThoughtNoTicketWindow:    "couldn't find where to buy a ticket",
	ThoughtHitTree:           "I hit a tree!",
	ThoughtLiftsClosed:       "the lifts are all closed",
	ThoughtNothingForMe:      "there's nothing here I can ski",
	ThoughtHurtGoingHome:     "I tweaked something, calling it a day",
	ThoughtCaughtInAvalanche: "an avalanche knocked me over!",
	ThoughtTooEasy:           "I want something more challenging",
	ThoughtGreatRun:          "what a great run!",
	ThoughtTooHard:           "that run was too much for me",
	ThoughtCrowdedRun:        "way too crowded on that run",
	ThoughtFellUnloading:     "I fell getting off the lift!",
	ThoughtLinesFull:         "every lift line is way too long",
	ThoughtGoodMeal:          "that meal hit the spot",
	ThoughtGoodDrink:         "just what I needed",
	ThoughtRested:            "good to sit down for a bit",
	ThoughtPatrolFast:        "patrol got to me so fast",
	ThoughtPatrolCame:        "thank goodness, patrol is here",
	ThoughtPatrolSlow:        "help took forever to get here",
}

// ThoughtLabel is the chart series label for each thought kind — the
// base text from thoughtText wrapped in double-quotes. Derived in init().
// An empty string excludes the kind from charts (only ThoughtNone).
var ThoughtLabel [ThoughtKindCount]string

func init() {
	for k := ThoughtKind(1); int(k) < ThoughtKindCount; k++ {
		if thoughtText[k] != "" {
			ThoughtLabel[k] = `"` + thoughtText[k] + `"`
		}
	}
}

// ThoughtChartColor is the RGBA bar colour for each thought kind in charts.
// Must match ThoughtLabel: every entry with a non-empty label needs a colour.
var ThoughtChartColor = [ThoughtKindCount][4]float32{
	ThoughtLovingGlades:      {0.30, 0.75, 0.40, 1},
	ThoughtScaredInTrees:     {0.90, 0.35, 0.30, 1},
	ThoughtLovingCorduroy:    {0.45, 0.85, 0.55, 1},
	ThoughtFell:              {0.85, 0.20, 0.20, 1},
	ThoughtInjured:           {0.95, 0.10, 0.10, 1},
	ThoughtAbandoned:         {0.60, 0.10, 0.80, 1},
	ThoughtLongLine:          {0.80, 0.45, 0.70, 1},
	ThoughtLineTooLong:       {0.70, 0.30, 0.60, 1},
	ThoughtNeedsLodge:        {0.60, 0.50, 0.80, 1},
	ThoughtHungry:            {0.95, 0.60, 0.20, 1},
	ThoughtThirsty:           {0.25, 0.65, 0.90, 1},
	ThoughtTired:             {0.80, 0.70, 0.30, 1},
	ThoughtExhausted:         {0.65, 0.50, 0.20, 1},
	ThoughtImpatient:         {0.75, 0.40, 0.55, 1},
	ThoughtTooExpensive:      {0.95, 0.85, 0.20, 1},
	ThoughtNoTicketWindow:    {0.85, 0.40, 0.30, 1},
	ThoughtHitTree:           {0.55, 0.35, 0.15, 1},
	ThoughtLiftsClosed:       {0.50, 0.55, 0.65, 1},
	ThoughtNothingForMe:      {0.75, 0.55, 0.45, 1},
	ThoughtHurtGoingHome:     {0.90, 0.45, 0.35, 1},
	ThoughtCaughtInAvalanche: {0.85, 0.85, 0.95, 1},
	ThoughtTooEasy:           {0.55, 0.60, 0.85, 1},
	ThoughtGreatRun:          {0.35, 0.90, 0.75, 1},
	ThoughtTooHard:           {0.90, 0.50, 0.25, 1},
	ThoughtCrowdedRun:        {0.70, 0.45, 0.50, 1},
	ThoughtFellUnloading:     {0.90, 0.55, 0.40, 1},
	ThoughtLinesFull:         {0.75, 0.35, 0.65, 1},
	ThoughtGoodMeal:          {0.95, 0.75, 0.35, 1},
	ThoughtGoodDrink:         {0.40, 0.80, 0.95, 1},
	ThoughtRested:            {0.70, 0.85, 0.50, 1},
	ThoughtPatrolFast:        {0.30, 0.85, 0.60, 1},
	ThoughtPatrolCame:        {0.60, 0.80, 0.60, 1},
	ThoughtPatrolSlow:        {0.80, 0.30, 0.45, 1},
}

// DepartReason is why a guest went home: one per visit, recorded when
// they decide to leave. Separate from thoughts, which report what moved
// their stats along the way.
type DepartReason uint8

const (
	DepartNone         DepartReason = iota
	DepartDone                      // nothing left they wanted to do
	DepartTired                     // out of energy
	DepartHungry                    // ran out of hunger with no meal
	DepartThirsty                   // ran out of thirst with no drink
	DepartClosing                   // the lifts closed for the day
	DepartHurt                      // a minor injury, or patched up by patrol
	DepartAbandoned                 // hurt, and no one came to help
	DepartLines                     // patience gone, mostly to lift lines
	DepartMoney                     // can't afford another ticket
	DepartNoTicket                  // couldn't reach a ticket window
	DepartLiftsClosed               // every lift stopped during open hours
	DepartNothingToSki              // no running lift with a trail for them
	departReasonSentinel
)

// DepartReasonCount sizes arrays indexed by DepartReason.
const DepartReasonCount = int(departReasonSentinel)

// DepartReasonLabel is the player-facing text for each reason.
var DepartReasonLabel = [DepartReasonCount]string{
	DepartDone:         "Done for the day",
	DepartTired:        "Tired out",
	DepartHungry:       "Too hungry",
	DepartThirsty:      "Too thirsty",
	DepartClosing:      "Lifts closed for the day",
	DepartHurt:         "Hurt",
	DepartAbandoned:    "Hurt, no one came to help",
	DepartLines:        "Fed up with lines",
	DepartMoney:        "Out of money",
	DepartNoTicket:     "Couldn't buy a ticket",
	DepartLiftsClosed:  "Lifts stopped",
	DepartNothingToSki: "Nothing to ski",
}

// DepartNormal reports whether a reason is an ordinary end to a day:
// these cost the guest nothing beyond what their conditions already did.
func DepartNormal(r DepartReason) bool {
	switch r {
	case DepartDone, DepartTired, DepartHungry, DepartThirsty, DepartClosing:
		return true
	}
	return false
}

// DepartReasonColor is the chart colour for each reason: greens for an
// ordinary end to the day, warm colours for the rest.
var DepartReasonColor = [DepartReasonCount][4]float32{
	DepartDone:         {0.35, 0.85, 0.45, 1},
	DepartTired:        {0.55, 0.80, 0.40, 1},
	DepartHungry:       {0.75, 0.80, 0.35, 1},
	DepartThirsty:      {0.40, 0.75, 0.70, 1},
	DepartClosing:      {0.45, 0.65, 0.85, 1},
	DepartHurt:         {0.90, 0.45, 0.35, 1},
	DepartAbandoned:    {0.60, 0.10, 0.80, 1},
	DepartLines:        {0.80, 0.45, 0.70, 1},
	DepartMoney:        {0.95, 0.85, 0.20, 1},
	DepartNoTicket:     {0.85, 0.40, 0.30, 1},
	DepartLiftsClosed:  {0.50, 0.55, 0.65, 1},
	DepartNothingToSki: {0.75, 0.55, 0.45, 1},
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
// Falls back to thoughtText[t.Kind] for thoughts without entity context.
func (t Thought) Display(resolve func(uint64) string) string {
	name := func(i int) string {
		if i < len(t.Context) && t.Context[i] != 0 {
			if s := resolve(t.Context[i]); s != "" {
				return s
			}
		}
		return ""
	}
	// Only thoughts that embed an entity name mid-sentence need cases here;
	// everything else falls through to the canonical thoughtText string.
	switch t.Kind {
	case ThoughtFell:
		if n := name(0); n != "" {
			return "ouch, that hurt on " + n
		}
	case ThoughtInjured:
		if n := name(0); n != "" {
			return "I'm hurt on " + n + ", I can't move"
		}
	case ThoughtGreatRun:
		if n := name(0); n != "" {
			return "what a great run on " + n + "!"
		}
	case ThoughtTooHard:
		if n := name(0); n != "" {
			return n + " was too much for me"
		}
	case ThoughtTooEasy:
		if n := name(0); n != "" {
			return n + " is too easy, I want something harder"
		}
	case ThoughtCrowdedRun:
		if n := name(0); n != "" {
			return "way too crowded on " + n
		}
	case ThoughtLongLine:
		if n := name(0); n != "" {
			return "the " + n + " line is way too long"
		}
	case ThoughtLineTooLong:
		if n := name(0); n != "" {
			return "the " + n + " line will take forever"
		}
	}
	return thoughtText[t.Kind]
}

// ThoughtTTL is how long an event thought stays "currently on the
// guest's mind" in the HUD. A condition thought stays current while its
// condition holds.
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
