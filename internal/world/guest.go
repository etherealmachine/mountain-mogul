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
	OnLiftID  uint64 // nonzero ⇒ riding the named lift's chair (locomotion is suspended)
	Queued    bool   // in some lift.Queue, waiting to board
	Fallen    bool   // briefly immobilised after a fall; clears when FallTimer expires
	FallTimer float32

	// AI state — populated by sim package.
	Plan         ai.Plan
	Balance      float32 // 1.0 fresh; ≤0 triggers a fall
	TurnSide     int8    // -1/0/+1; current carve-side commit (S-turn state)
	TurnDwell    float32 // seconds since the last TurnSide flip
	LastTactical float32 // rad; previous tick's tactical lateral offset

	// Energy is the session fatigue budget. 1.0 fresh; depletes only while
	// skiing (drained per-tick in tickSkier). When it falls below
	// energyLowThreshold (~one descent's worth) the next decision boundary
	// reroutes the guest to a lodge or out. Calibrated so a fresh skier
	// completes roughly 20 descents before heading home.
	Energy float32

	// Fun is the smoothed satisfaction signal the L0 GOAP planner reads
	// to weight goals. Rises on a fresh lift ride (novelty bonus on
	// unridden lifts, smaller bonus on repeats), decays slowly otherwise.
	// 0..1; clamped on writeback. The demand system's daily rating poll
	// reads this via Guest.Rating().
	Fun float32

	// Fear / FearTarget use the RCT2 "target & current" smoothing
	// pattern: per-tick stat events bump FearTarget; Fear itself eases
	// toward FearTarget so single bad moments don't whiplash Rating().
	// FearTarget decays toward 0 on its own; sustained scary input is
	// what keeps Fear elevated. Both 0..1.
	Fear       float32
	FearTarget float32

	// Thoughts is a small ring of recent ai.Thought entries — the
	// player-visible "what's this guest thinking" surface. The newest
	// thought is at Thoughts[ThoughtsHead-1] (mod len); CurrentThought
	// walks the ring oldest-first ignoring expired entries.
	Thoughts     [thoughtsCap]ai.Thought
	ThoughtsHead int // next write index

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

// Score-equation weights for Guest.Rating(). α + β + γ + δ should sum
// to 1 so a perfect session lands at 1 and a disastrous one at 0. Fear
// enters as a (1 - Fear) factor on its weight — terrified guests rate
// poorly even if Fun/Energy are high.
const (
	ratingWeightFun    = 0.35
	ratingWeightEnergy = 0.25
	ratingWeightClean  = 0.20
	ratingWeightCalm   = 0.20 // (1 - Fear) contribution
)

// Rating computes the 0..1 "how is this guest doing" score from the
// session log + current Fun/Energy/Fear. Read mid-session by the demand
// system's daily rating poll and at departure to stamp LastScore on the
// persistent record. Cleanness penalises falls per completed run — a
// session with zero falls scores 1 on that axis regardless of how few
// runs; many falls per run scores 0. Calmness = (1 - Fear).
func (g *Guest) Rating() float32 {
	falls, runs := 0, 0
	for _, e := range g.Events {
		switch e.Kind {
		case ai.EventFall:
			falls++
		case ai.EventRun:
			runs++
		}
	}
	clean := float32(1)
	if runs > 0 {
		ratio := float32(falls) / float32(runs)
		if ratio > 1 {
			ratio = 1
		}
		clean = 1 - ratio
	}
	calm := 1 - g.Fear
	if calm < 0 {
		calm = 0
	}
	score := ratingWeightFun*g.Fun +
		ratingWeightEnergy*g.Energy +
		ratingWeightClean*clean +
		ratingWeightCalm*calm
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}

// AddThought pushes a new Thought onto the ring at simTime. Duplicates
// within the TTL window are suppressed so a guest standing in the same
// patch of trees doesn't spam ScaredInTrees every tick.
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

// fearDecayPerSec is how fast FearTarget bleeds back toward 0 without
// fresh scary input. Fear (current) eases toward FearTarget at the same
// rate, giving Fear a roughly fearDecayPerSec half-life when the input
// goes quiet.
const fearDecayPerSec = 0.15

// fearEasePerSec is the rate at which Fear (current) lerps toward
// FearTarget. Slightly faster than the target's decay so the visible
// stat keeps pace with bumps without overshooting.
const fearEasePerSec = 0.25

// TickStats advances the per-guest stat smoothers by dt sim-seconds.
// Discipline-agnostic — runs every tick for every OnMountain guest
// regardless of where the dispatcher sent them.
func (g *Guest) TickStats(dt float64) {
	if dt <= 0 {
		return
	}
	// FearTarget decays toward 0; sustained scary input is what keeps
	// it elevated. Per-tick stimuli (in skiing.go) bump it back up.
	d := float32(dt * fearDecayPerSec)
	if g.FearTarget > d {
		g.FearTarget -= d
	} else {
		g.FearTarget = 0
	}
	// Fear eases toward FearTarget at fearEasePerSec.
	step := float32(dt * fearEasePerSec)
	switch {
	case g.Fear < g.FearTarget-step:
		g.Fear += step
	case g.Fear > g.FearTarget+step:
		g.Fear -= step
	default:
		g.Fear = g.FearTarget
	}
	if g.Fear < 0 {
		g.Fear = 0
	}
	if g.Fear > 1 {
		g.Fear = 1
	}
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
	g.Energy = 0
	g.Fun = 0
	g.Fear = 0
	g.FearTarget = 0
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
