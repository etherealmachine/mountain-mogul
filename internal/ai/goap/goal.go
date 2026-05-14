package goap

import (
	"sort"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Goal is one strategic objective. The planner picks the highest-weighted
// unsatisfied goal at replan time, then searches for the cheapest action
// chain whose terminal state satisfies the goal predicate. Weights are
// per-snapshot so they can react to Energy / Fun / unridden-lift count
// without rerunning the planner.
//
// Concrete goal types are plain structs with no fields — the goal's
// behavior is entirely in the IsSatisfied / Weight closures.
type Goal interface {
	Name() string
	IsSatisfied(s *WorldSnapshot, w *world.World) bool
	Weight(s *WorldSnapshot, w *world.World) float32
}

// AllGoals is the full set considered at replan time. Order is irrelevant
// — Select picks by weight.
var AllGoals = []Goal{
	KeepSkiing{},
	Rest{},
	Explore{},
	GoHome{},
}

// KeepSkiing is the baseline drive: ride one more lift. Always weighted
// proportional to Energy so a fresh skier wants to ski; a depleted one
// loses to Rest or GoHome. The IsSatisfied predicate looks for "rode any
// lift" — the planner's terminal state will have at least one lift in
// RidenLifts with a positive count above whatever it started with.
type KeepSkiing struct{}

func (KeepSkiing) Name() string { return "KeepSkiing" }

func (KeepSkiing) IsSatisfied(s *WorldSnapshot, w *world.World) bool {
	// Satisfied when AtLiftTop is non-zero — i.e. the plan ends in a fresh
	// lift unload. Cheapest path to "I just rode a lift."
	return s.AtLiftTop != 0
}

func (KeepSkiing) Weight(s *WorldSnapshot, w *world.World) float32 {
	if s.Energy < 0.2 {
		return s.Energy * 0.5
	}
	return s.Energy
}

// Rest is satisfied at Energy ≥ restThreshold. Weight is quadratic in
// (1 − Energy) so it only fires when the skier is genuinely tired —
// otherwise KeepSkiing dominates and the skier keeps cycling lifts.
type Rest struct{}

const restThreshold = 0.85

func (Rest) Name() string { return "Rest" }

func (Rest) IsSatisfied(s *WorldSnapshot, w *world.World) bool {
	return s.Energy >= restThreshold
}

func (Rest) Weight(s *WorldSnapshot, w *world.World) float32 {
	if s.Energy >= restThreshold {
		return 0
	}
	d := 1 - s.Energy
	return d * d
}

// Explore is satisfied once every lift in the world has been ridden at
// least once. Weight is the fraction of lifts still unridden — drops to
// zero when the skier has sampled the whole resort, which naturally
// hands off to GoHome via the rising GoHome weight.
type Explore struct{}

func (Explore) Name() string { return "Explore" }

func (Explore) IsSatisfied(s *WorldSnapshot, w *world.World) bool {
	if len(w.Lifts) == 0 {
		return true
	}
	for _, l := range w.Lifts {
		if ai.RideCountOf(s.RidenLifts, l.ID) == 0 {
			return false
		}
	}
	return true
}

func (Explore) Weight(s *WorldSnapshot, w *world.World) float32 {
	if len(w.Lifts) == 0 {
		return 0
	}
	unridden := 0
	for _, l := range w.Lifts {
		if ai.RideCountOf(s.RidenLifts, l.ID) == 0 {
			unridden++
		}
	}
	// Gate on Energy — a worn-out skier shouldn't keep "exploring" past
	// the point of exhaustion. Without this, Explore stays at 1.0 for an
	// agent who hasn't ridden anything, and dominates Rest at low energy
	// even though the skier physically can't keep skiing. Once Boredom
	// lands as a real affect signal it replaces this stand-in.
	frac := float32(unridden) / float32(len(w.Lifts))
	return frac * s.Energy
}

// GoHome is satisfied when the agent has Departed (terminal Removed
// flag). Weight rises with tiredness AND with how much of the resort
// the skier has already explored — a fresh skier who's ridden every
// lift is "done" and goes home; a tired skier goes home regardless of
// exploration.
type GoHome struct{}

func (GoHome) Name() string { return "GoHome" }

func (GoHome) IsSatisfied(s *WorldSnapshot, w *world.World) bool {
	return s.Removed
}

func (GoHome) Weight(s *WorldSnapshot, w *world.World) float32 {
	// MVP scope: GoHome only fires at critically low Energy (below 0.05
	// the L1 controller's energyLowThreshold). Rest is preferred for any
	// recoverable fatigue level. The "I've ridden everything, time to
	// leave" pathway needs a richer signal — Boredom or a session-time
	// counter — and lands with the affect work.
	if s.Energy < 0.05 {
		return 1.0
	}
	return 0
}

// SelectGoal returns the highest-weighted unsatisfied goal, or nil if
// every goal is satisfied (shouldn't happen in normal play — KeepSkiing
// is satisfiable but the planner picks a new ride afterward).
func SelectGoal(s *WorldSnapshot, w *world.World) Goal {
	var best Goal
	bestW := float32(-1)
	for _, g := range AllGoals {
		if g.IsSatisfied(s, w) {
			continue
		}
		wt := g.Weight(s, w)
		if wt > bestW {
			bestW = wt
			best = g
		}
	}
	return best
}

// GoalRanking is one row in RankedGoals' output: a goal, its current
// Weight, and whether it's already satisfied. The debug HUD renders
// these to make the planner's decision auditable ("why Explore over
// Rest?" — the weights show why).
type GoalRanking struct {
	Goal      Goal
	Weight    float32
	Satisfied bool
}

// RankedGoals returns every goal in AllGoals tagged with its current
// weight and satisfaction state, sorted so unsatisfied goals come
// first (highest weight first), then satisfied goals. The top entry is
// the same goal SelectGoal would return.
func RankedGoals(s *WorldSnapshot, w *world.World) []GoalRanking {
	out := make([]GoalRanking, 0, len(AllGoals))
	for _, g := range AllGoals {
		out = append(out, GoalRanking{
			Goal:      g,
			Weight:    g.Weight(s, w),
			Satisfied: g.IsSatisfied(s, w),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Satisfied != out[j].Satisfied {
			return !out[i].Satisfied
		}
		return out[i].Weight > out[j].Weight
	})
	return out
}
