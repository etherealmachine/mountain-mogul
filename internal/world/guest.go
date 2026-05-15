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
	LastScore        float32 // most recent scoreGuest captured at departure

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
	// reads this via scoreGuest.
	Fun float32

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
	g.RidenLifts = g.RidenLifts[:0]
	g.RestTimer = 0
	g.Removed = false
	g.Events = g.Events[:0]
	g.Sense = ai.Sense{}
	g.LastTrackPos = mgl32.Vec3{}
}
