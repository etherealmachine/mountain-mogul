package world

import (
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
)

// Discipline is the equipment a guest rides on the mountain. Drives which
// per-tick physics module ticks them (sim/skiing.go vs eventually
// sim/snowboarding.go) and which mesh the renderer draws.
type Discipline uint8

const (
	Ski Discipline = iota
	Snowboard
)

// String returns a short label suitable for HUD / debug overlays.
func (d Discipline) String() string {
	switch d {
	case Ski:
		return "Ski"
	case Snowboard:
		return "Snowboard"
	}
	return "?"
}

// GuestState is where a guest is in the visit cycle. The demand poll only
// spawns from AtHome; sim scratch fields (Plan, Path, Pos, ...) are only
// meaningful while OnMountain.
type GuestState uint8

const (
	AtHome GuestState = iota
	OnMountain
	// InCar: riding in a car to or from the resort, or sitting in it in
	// the lot waiting for the rest of the carload.
	InCar
)

// Guest is one person who comes to the resort. The same struct lives in
// the master catchment (World.Guests, ~10k entries, persistent identity)
// and is pointed at by the active-subset slice (World.OnMountain) while
// the guest is actively skiing — sim scratch fields are zero/nil at home
// and populated on arrival. There is no explicit ski-state field beyond
// State: the situation on the mountain is implicit in the combination of
// OnLiftID / Queued / Fallen / Path / TargetID. Activity() derives a
// single human-readable label.
type Guest struct {
	// =====================================================================
	// Identity — set at world init, persists forever.
	// =====================================================================

	ID              uint64
	Name            string
	Discipline      Discipline
	Traits          ai.GuestTraits // includes SkillLevel
	VisitsPerSeason float32        // expected mean visits per ski season
	ArrivalOffset   float32        // preferred arrival, clock hours after opening (negative: before)

	// =====================================================================
	// Career stats — grow over time, drive future hysteresis (e.g. don't
	// re-visit within a cooldown, regulars get loyalty boosts, etc.).
	// =====================================================================

	VisitsThisSeason int
	LifetimeVisits   int
	LastVisit        time.Time
	LastScore        float32 // most recent Guest.Rating() captured at departure

	// =====================================================================
	// Visit lifecycle.
	// =====================================================================

	State GuestState
	// CarID is the car the guest came in, and CarLot the lot it's parked
	// in (0 until it parks); guests leave from that lot. Both are 0 at
	// home.
	CarID  uint64
	CarLot uint64

	// =====================================================================
	// Live sim state — zero/nil when State == AtHome, populated by the
	// spawn path when the guest arrives, cleared at departure.
	// =====================================================================

	Pos     mgl32.Vec3
	Heading float32
	Speed   float32

	// Pathfinder route from a lodge/lot to a lift base. While Path is
	// non-empty and PathIdx is in-range the guest is walking the path.
	Path    [][2]int
	PathIdx int

	// Single goal: the entity (lift or building) the guest is heading
	// toward. 0 = idle. The simulation resolves the entity by ID — the
	// same ID space is used for buildings and lifts so this disambiguates
	// itself at lookup time.
	TargetID uint64

	// Implicit-state markers.
	OnLiftID        uint64 // nonzero ⇒ riding the named lift's chair (locomotion is suspended)
	Queued          bool   // in some lift.Queue, waiting to board
	Fallen          bool   // briefly immobilised after a fall; clears when FallTimer expires
	FallTimer       float32
	Injured         bool    // injured after a severe fall; cannot self-recover
	HurtGoHome      bool    // a minor injury: heads home once back on their feet
	InjuryWaitTimer float32 // counts down while Injured; on expiry guest gives up and crawls home
	OnPatrollerID   uint64  // nonzero ⇒ being transported by this patroller; locomotion suspended
	AtTrailEnd      uint64  // nonzero ⇒ arrived at a trail-to-trail junction (ID = destination trail)

	// AI state — populated by sim package.
	Plan         ai.Plan
	Balance      float32 // 1.0 fresh; ≤0 triggers a fall
	TurnSide     int8    // -1/0/+1; current carve-side commit (S-turn state)
	TurnDwell    float32 // seconds since the last TurnSide flip
	LastTactical float32 // rad; previous tick's tactical lateral offset

	// Patience is the guest's tolerance budget. 1.0 on arrival; drains
	// while queuing, restored by active skiing and riding lifts. A lodge
	// rest restores it to 1. When it reaches 0 the GoHome goal fires and
	// the guest leaves.
	Patience float32

	// Energy is the guest's physical fatigue budget. 1.0 on arrival;
	// drains while skiing (faster on terrain above their skill level, big
	// spike on falls). RestAtLodge restores it to 1. When it hits 0 the
	// Rest goal fires; if no lodge is reachable the guest goes home with
	// a ThoughtNeedsLodge thought.
	Energy float32

	// Hunger and Thirst are one-way countdowns: randomised at spawn,
	// drain continuously, and cannot be restored. When either hits 0 the
	// GoHome goal fires. Hunger drains at a fixed rate; Thirst drains
	// faster at altitude and during harder skiing.
	Hunger float32
	Thirst float32

	// RemainingBudget is the guest's remaining spending money for this visit.
	// Reset at each spawn to Traits.DailyBudget minus the day ticket they
	// intend to buy (pass holders pay no day ticket); further decremented by
	// heli fares and the season pass purchase.
	RemainingBudget float32

	// DayTicketDue is the day ticket priced at this visit's arrival and
	// still owed at the ticket window, in dollars. Already set aside from
	// RemainingBudget; moves to DayTicketPaid when the guest buys it.
	DayTicketDue int
	// DayTicketPaid is the day ticket bought at the window this visit, in
	// dollars (0 for pass holders). Credited toward a season pass bought
	// later in the same visit, then zeroed.
	DayTicketPaid int
	// HasDayTicket is true once the guest has bought today's day ticket.
	// A guest needs this or a valid season pass to join a lift queue.
	HasDayTicket bool

	// SeasonPassExpiry is the SimTime at which the guest's season pass expires.
	// Zero means no pass. Persisted so passes survive save/load.
	SeasonPassExpiry float64
	// HasSeasonPass is a precomputed flag: true when SeasonPassExpiry > 0 and
	// the current SimTime is before it. Updated at spawn and when a pass is
	// purchased. Pass holders ride any open lift for free.
	HasSeasonPass bool

	// Satisfaction is the 0..1 session mood. Initialised to the
	// baseline (0.5) on arrival; drifts toward a target that terrain and active conditions
	// pull on, and jumps on events. Only sim.applyEvent and the drift
	// write it; ai.Effects holds every amount. Rating() returns it; at
	// departure it is captured as LastScore and folded into the rating.
	Satisfaction float32

	// Baseline is the level the guest's mood drifts back to: 0.5 on
	// arrival, raised by good experiences (less for each repeat) and
	// lowered by bad ones, for the rest of the visit.
	Baseline float32

	// TrailTally and LiftTally count this visit's runs, and great runs,
	// by the trail they were mostly on and the lift they started from.
	TrailTally []RunTally
	LiftTally  []RunTally

	// Thoughts is a small ring of recent ai.Thought entries — the
	// player-visible "what's this guest thinking" surface. The newest
	// thought is at Thoughts[ThoughtsHead-1] (mod len); CurrentThought
	// walks the ring oldest-first ignoring expired entries.
	Thoughts     [thoughtsCap]ai.Thought
	ThoughtsHead int // next write index

	// ThoughtCounts tallies thoughts per kind for the session: each event,
	// and each start of a condition. Indexed by ai.ThoughtKind.
	ThoughtCounts [ai.ThoughtKindCount]int

	// SkiTerrainPull and SkiedThisTick are what a skiing tick leaves for
	// the next mood update: the terrain's pull on the mood target, and
	// that the guest skied at all. The mood update zeroes both.
	SkiTerrainPull float32
	SkiedThisTick  bool

	// DepartReason is why the guest is going home, set once when they
	// decide to leave; DepartNone while they're still skiing.
	DepartReason ai.DepartReason

	// Conditions is the set of condition thoughts holding right now. A
	// condition's thought is added when its bit turns on; its pull on
	// the mood target lasts while the bit is set.
	Conditions ai.ConditionMask

	// RidenLifts is the per-guest ride tally. The MVP novelty mechanic:
	// first ride of a lift is the biggest Fun bump, subsequent rides
	// taper. The planner reads this through goap.WorldSnapshot to weight
	// Explore and to compute RideLift cost. Stored as a flat slice (not
	// a map) so the planner's per-expansion Clone is a cheap slice copy.
	// HomeEntryID is the road entry (World.Entries) this guest lives
	// beyond: they always arrive and leave by it. 0 when the map has none.
	HomeEntryID uint64

	RidenLifts []ai.RideCount

	// Underfoot is each taste × feature of the snow under the guest, and
	// UnderfootFear their fear of the slope, averaged over the last few
	// seconds of skiing (sim.tickUnderfoot) so one odd cell doesn't start
	// or stop a thought. Reset at the start of each run.
	Underfoot     [ai.TasteCount]float32
	UnderfootFear float32

	// Unload is the guest's glide off a chair at a lift's top station;
	// LiftID is 0 when they aren't unloading.
	Unload Unloading

	// Run is what the guest has skied on the current descent, from the
	// top (usually a lift's) to wherever it ends; judged at the bottom.
	Run Run

	// SkisOn tracks whether the guest currently has their skis on.
	// True on arrival; toggled via SkiTransitionTimer when entering or
	// leaving terrain that requires walking (building footprints, bare
	// ground). While false the guest walks regardless of slope.
	SkisOn bool

	// SkiTransitionTimer drives the 1-second equip/unequip pause.
	// Positive = removing skis (counts down to 0, then SkisOn→false).
	// Negative = putting skis on (counts up to 0, then SkisOn→true).
	// Zero = no active transition.
	SkiTransitionTimer float32

	// RestTimer counts down the atomic RestAtLodge action. While >0 the
	// guest is parked at a lodge recovering; on expiry Energy resets to
	// 1 and the plan advances.
	RestTimer float32

	// Removed flags the terminal in-session state set by the GOAP Depart
	// action. The per-tick dispatch skips Removed guests and reapDeparted
	// returns them to AtHome (splicing out of w.OnMountain) after
	// tickGuests completes, so the range loop's slice header doesn't
	// shift mid-pass.
	Removed bool

	// Events is the per-session log appended to by the sim (falls, run
	// completions). Read at depart by the demand system to feed
	// LastScore. Cleared on transition back to AtHome.
	Events []ai.GuestEvent

	// Display-only snapshot of the last skiing tick's perception/intent.
	// Populated by sim.tickSkier; read by the follow HUD and the
	// renderer's perception-cone shader. Stale outside of skiing.
	Sense ai.Sense

	// LastTrackPos is the guest's position at the most recent track-splat
	// substep, used so the surface-detail R-channel splatter can draw a
	// segment from previous→current rather than dot-stippling at high
	// speed / TimeScale. Zero before the first splat or after a state
	// reset (lift unload, fall recovery).
	LastTrackPos mgl32.Vec3
}

// Activity returns a short human-readable label describing what the guest
// is doing right now, derived from the implicit-state fields. Used by the
// follow HUD, debug overlays, the CSV recorder and the headless trace.
// The world is needed to resolve TargetID into a building-or-lift label.
func Activity(w *World, g *Guest) string {
	if g.State == AtHome {
		return "At home"
	}
	if g.OnPatrollerID != 0 {
		return "Being Rescued"
	}
	if g.Injured {
		return "Injured"
	}
	if g.Fallen {
		return "Fallen"
	}
	if g.Unload.LiftID != 0 {
		return "Unloading"
	}
	if g.OnLiftID != 0 {
		return "On Lift"
	}
	if g.Queued {
		return "Queuing"
	}
	if g.SkiTransitionTimer > 0 {
		return "Removing Skis"
	}
	if g.SkiTransitionTimer < 0 {
		return "Putting On Skis"
	}
	if len(g.Path) > 0 && g.PathIdx < len(g.Path) {
		return "Walking"
	}
	if g.TargetID == 0 {
		return "Idle"
	}
	for _, b := range w.Buildings {
		if b.ID == g.TargetID {
			return "Departing"
		}
	}
	for _, l := range w.Lifts {
		if l.ID == g.TargetID {
			return "To Lift"
		}
	}
	return "Traveling"
}

// Unloading is a rider's scripted path off a chair (sim.tickUnloading).
type Unloading struct {
	LiftID uint64  // the lift they're getting off; 0 when not unloading
	Side   float32 // where their seat sat across the chair, -1..+1: which way and how hard they peel off
	Gone   float32 // metres travelled since standing up
	FallAt float32 // metres along the path where they fall; 0 when they won't
	SeatY  float32 // the seat's height above the snow where they stood up
}

// UnloadRise is how far, in metres along the unload path, a rider takes
// to stand up from seat height to the snow (half a second at ramp speed).
const UnloadRise = 1.25

// UnloadLift is how far above the snow to draw an unloading rider: easing
// from their seat's height to 0 over UnloadRise.
func (u Unloading) UnloadLift() float32 {
	if u.LiftID == 0 || u.Gone >= UnloadRise {
		return 0
	}
	return u.SeatY * (1 - u.Gone/UnloadRise)
}

// RunTally is one trail's or lift's count of runs in a visit.
type RunTally struct {
	ID    uint64
	Runs  int32
	Great int32
}

// CountRun adds a run on id to tally, returning how many great runs id
// had before this one.
func CountRun(tally []RunTally, id uint64, great bool) ([]RunTally, int32) {
	i := 0
	for i < len(tally) && tally[i].ID != id {
		i++
	}
	if i == len(tally) {
		tally = append(tally, RunTally{ID: id})
	}
	before := tally[i].Great
	tally[i].Runs++
	if great {
		tally[i].Great++
	}
	return tally, before
}

// RunTrailSlots is how many distinct trails a Run tracks time on.
const RunTrailSlots = 4

// Run summarises one descent as it was skied, tick by tick: where the
// time went, how steep and crowded it was, and how much height it
// covered. Times are seconds of skiing.
type Run struct {
	Start    float64    // SimTime the descent began
	LiftID   uint64     // the lift they unloaded from to start it; 0 otherwise
	StartY   float32    // elevation at the start, for vertical
	Time     float32    // seconds skiing
	Distance float32    // metres skied
	ByDiff   [3]float32 // seconds on green, blue, and black trail cells
	OffTrail float32    // seconds on no trail
	Steep    float32    // seconds well past the guest's ComfortSlope (sim.runSteepMargin)
	Crowd    float32    // skiers nearby × seconds
	Groomed  float32    // grooming × seconds
	Trails   [RunTrailSlots]RunTrail
}

// RunTrail is the time a Run spent on one trail.
type RunTrail struct {
	ID  uint64
	Sec float32
}

// AddTrailTime adds dt to trail id's slot, taking a free slot or, when
// all are full, the one with the least time.
func (r *Run) AddTrailTime(id uint64, dt float32) {
	low := 0
	for i := range r.Trails {
		if r.Trails[i].ID == id {
			r.Trails[i].Sec += dt
			return
		}
		if r.Trails[i].Sec < r.Trails[low].Sec {
			low = i
		}
	}
	r.Trails[low] = RunTrail{ID: id, Sec: dt}
}

// MainTrail is the trail the run spent the most time on, 0 for none.
func (r *Run) MainTrail() uint64 {
	best := RunTrail{}
	for _, t := range r.Trails {
		if t.Sec > best.Sec {
			best = t
		}
	}
	return best.ID
}

// thoughtsCap is the size of the Thoughts ring. Six is enough that a
// few simultaneous stimuli (in-trees + low-energy + fall) all fit.
const thoughtsCap = 6

// Rating returns the guest's current 0..1 session satisfaction score.
// Backed by the Satisfaction float, which drifts toward terrain quality
// each tick and spikes on events. Captured as LastScore at departure.
func (g *Guest) Rating() float32 {
	return g.Satisfaction
}

// AddThought records a thought on the ring at simTime and counts it.
// context is an optional list of entity IDs (lift, trail, etc.) used to
// format the display string. It changes no stats: the sim calls it from
// applyEvent and setCondition, which apply the effect it reports.
func (g *Guest) AddThought(kind ai.ThoughtKind, simTime float64, context ...uint64) {
	if kind == ai.ThoughtNone {
		return
	}
	var ctx []uint64
	if len(context) > 0 {
		ctx = append([]uint64(nil), context...)
	}
	g.Thoughts[g.ThoughtsHead] = ai.Thought{Kind: kind, Time: simTime, Context: ctx}
	g.ThoughtsHead = (g.ThoughtsHead + 1) % thoughtsCap
	g.ThoughtCounts[kind]++
}

// CurrentThought returns the most-recent thought still on the guest's
// mind: an event thought within ai.ThoughtTTL, or a condition thought
// whose condition still holds. A zero Thought when there is none.
// Check t.Kind != ai.ThoughtNone to distinguish the "no thought" case.
func (g *Guest) CurrentThought(simTime float64) ai.Thought {
	for i := 0; i < thoughtsCap; i++ {
		idx := (g.ThoughtsHead - 1 - i + thoughtsCap) % thoughtsCap
		t := g.Thoughts[idx]
		if t.Kind == ai.ThoughtNone {
			continue
		}
		if simTime-t.Time > ai.ThoughtTTL && !g.Conditions.Has(t.Kind) {
			continue
		}
		return t
	}
	return ai.Thought{}
}

// LastThought returns the most recently added thought regardless of TTL,
// or a zero Thought if no thought has been recorded this session.
func (g *Guest) LastThought() ai.Thought {
	for i := 0; i < thoughtsCap; i++ {
		idx := (g.ThoughtsHead - 1 - i + thoughtsCap) % thoughtsCap
		if g.Thoughts[idx].Kind != ai.ThoughtNone {
			return g.Thoughts[idx]
		}
	}
	return ai.Thought{}
}

// ResetForDeparture clears every transient sim field on g so the same
// pointer can be re-used by a future arrival. Identity + career stats are
// preserved. Called by reapDeparted when State flips back to AtHome.
func (g *Guest) ResetForDeparture() {
	g.State = AtHome
	g.Pos = mgl32.Vec3{}
	g.Heading = 0
	g.Speed = 0
	g.Path = nil
	g.PathIdx = 0
	g.TargetID = 0
	g.OnLiftID = 0
	g.Queued = false
	g.Fallen = false
	g.FallTimer = 0
	g.Injured = false
	g.HurtGoHome = false
	g.InjuryWaitTimer = 0
	g.OnPatrollerID = 0
	g.Plan = ai.Plan{}
	g.Balance = 0
	g.TurnSide = 0
	g.TurnDwell = 0
	g.LastTactical = 0
	g.Patience = 0
	g.Energy = 0
	g.Hunger = 0
	g.Thirst = 0
	g.Satisfaction = 0
	g.Baseline = 0
	g.TrailTally = g.TrailTally[:0]
	g.LiftTally = g.LiftTally[:0]
	for i := range g.Thoughts {
		g.Thoughts[i] = ai.Thought{}
	}
	g.ThoughtsHead = 0
	g.Conditions = 0
	g.DepartReason = ai.DepartNone
	g.RidenLifts = g.RidenLifts[:0]
	g.DayTicketDue = 0
	g.DayTicketPaid = 0
	g.HasDayTicket = false
	g.Run = Run{}
	g.Unload = Unloading{}
	g.SkisOn = false
	g.SkiTransitionTimer = 0
	g.RestTimer = 0
	g.Removed = false
	g.Events = g.Events[:0]
	g.Sense = ai.Sense{}
	g.LastTrackPos = mgl32.Vec3{}
}
