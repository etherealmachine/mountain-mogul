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
	OnLiftID   uint64 // nonzero ⇒ riding the named lift's chair (locomotion is suspended)
	Queued     bool   // in some lift.Queue, waiting to board
	Fallen     bool   // briefly immobilised after a fall; clears when FallTimer expires
	FallTimer  float32
	AtTrailEnd uint64 // nonzero ⇒ arrived at a trail-to-trail junction (ID = destination trail)

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

	// Satisfaction is the 0..1 session quality score. Initialised to 0.6
	// on arrival; drifts continuously toward a terrain-quality target each
	// skiing tick; spikes up or down on discrete events (novel lift ride,
	// fall, long queue, no lodge). Rating() returns it directly; at
	// departure it is captured as LastScore and folded into ResortRating
	// via EMA. Thoughts are display-only and no longer drive this value.
	Satisfaction float32

	// Thoughts is a small ring of recent ai.Thought entries — the
	// player-visible "what's this guest thinking" surface. The newest
	// thought is at Thoughts[ThoughtsHead-1] (mod len); CurrentThought
	// walks the ring oldest-first ignoring expired entries.
	Thoughts     [thoughtsCap]ai.Thought
	ThoughtsHead int // next write index

	// ThoughtCounts tallies every AddThought call per kind for the session.
	// Indexed by ai.ThoughtKind; accumulated into History.ThoughtCountsToday
	// when the guest departs.
	ThoughtCounts [ai.ThoughtKindCount]int

	// RidenLifts is the per-guest ride tally. The MVP novelty mechanic:
	// first ride of a lift is the biggest Fun bump, subsequent rides
	// taper. The planner reads this through goap.WorldSnapshot to weight
	// Explore and to compute RideLift cost. Stored as a flat slice (not
	// a map) so the planner's per-expansion Clone is a cheap slice copy.
	RidenLifts []ai.RideCount

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
	if g.Fallen {
		return "Fallen"
	}
	if g.OnLiftID != 0 {
		return "On Lift"
	}
	if g.Queued {
		return "Queuing"
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

// thoughtsCap is the size of the Thoughts ring. Six is enough that a
// few simultaneous stimuli (in-trees + low-energy + fall) all fit
// without crowding out the oldest of the bunch within ai.ThoughtTTL.
const thoughtsCap = 6

// Rating returns the guest's current 0..1 session satisfaction score.
// Backed by the Satisfaction float, which drifts toward terrain quality
// each tick and spikes on events. Captured as LastScore at departure.
func (g *Guest) Rating() float32 {
	return g.Satisfaction
}

// AddThought pushes a new Thought onto the ring at simTime for display.
// Duplicates within the TTL window are suppressed. Thoughts are
// display-only; Satisfaction is updated separately by the caller.
func (g *Guest) AddThought(kind ai.ThoughtKind, simTime float64) {
	if kind == ai.ThoughtNone {
		return
	}
	// Suppress if the same kind is already in-ring and fresh.
	for _, t := range g.Thoughts {
		if t.Kind == kind && simTime-t.Time < ai.ThoughtTTL {
			return
		}
	}
	g.Thoughts[g.ThoughtsHead] = ai.Thought{Kind: kind, Time: simTime}
	g.ThoughtsHead = (g.ThoughtsHead + 1) % thoughtsCap
	g.ThoughtCounts[kind]++
}

// CurrentThought returns the most-recent unexpired thought (relative to
// simTime), or ThoughtNone when the ring is empty / all expired. Walks
// the ring backwards from the head so the latest push wins.
func (g *Guest) CurrentThought(simTime float64) ai.ThoughtKind {
	for i := 0; i < thoughtsCap; i++ {
		idx := (g.ThoughtsHead - 1 - i + thoughtsCap) % thoughtsCap
		t := g.Thoughts[idx]
		if t.Kind == ai.ThoughtNone {
			continue
		}
		if simTime-t.Time > ai.ThoughtTTL {
			continue
		}
		return t.Kind
	}
	return ai.ThoughtNone
}

// LastThought returns the most recently added thought regardless of TTL,
// or ThoughtNone if no thought has been recorded this session.
func (g *Guest) LastThought() ai.ThoughtKind {
	for i := 0; i < thoughtsCap; i++ {
		idx := (g.ThoughtsHead - 1 - i + thoughtsCap) % thoughtsCap
		if g.Thoughts[idx].Kind != ai.ThoughtNone {
			return g.Thoughts[idx].Kind
		}
	}
	return ai.ThoughtNone
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
	g.Plan = ai.Plan{}
	g.Balance = 0
	g.TurnSide = 0
	g.TurnDwell = 0
	g.LastTactical = 0
	g.Patience = 0
	g.Satisfaction = 0
	for i := range g.Thoughts {
		g.Thoughts[i] = ai.Thought{}
	}
	g.ThoughtsHead = 0
	g.RidenLifts = g.RidenLifts[:0]
	g.RestTimer = 0
	g.Removed = false
	g.Events = g.Events[:0]
	g.Sense = ai.Sense{}
	g.LastTrackPos = mgl32.Vec3{}
}
