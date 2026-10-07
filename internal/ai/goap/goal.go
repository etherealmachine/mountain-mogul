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
	GetSeasonPass{},
	KeepSkiing{},
	FulfillNeed{ai.NeedHunger},
	FulfillNeed{ai.NeedThirst},
	FulfillNeed{ai.NeedRest},
	FulfillNeed{ai.NeedRentals},
	FulfillNeed{ai.NeedApres},
	FulfillNeed{ai.NeedWarmth},
	Explore{},
	GoHome{},
}

// GetSeasonPass fires early in a visit when there is a ticket office and the
// guest can afford a pass. Weight is high so guests buy a pass before skiing,
// but it immediately drops to zero once satisfied (HasSeasonPass is true).
type GetSeasonPass struct{}

func (GetSeasonPass) Name() string { return "GetSeasonPass" }

func (GetSeasonPass) IsSatisfied(s *WorldSnapshot, w *world.World) bool {
	return s.HasSeasonPass
}

func (GetSeasonPass) Weight(s *WorldSnapshot, w *world.World) float32 {
	if s.HasSeasonPass || s.RemainingBudget < passCost(s, w) {
		return 0
	}
	if s.Need[ai.NeedRentals] > 0 && !canRent(s, w) {
		return 0 // no use for a pass without skis
	}
	for _, b := range w.Buildings {
		if b.Offers(world.ServiceTickets) {
			return 0.9 // just below GoHome (1.0) but above all skiing goals
		}
	}
	return 0
}

// KeepSkiing is the baseline drive: ride one more lift. Weighted
// proportional to Patience so a patient skier wants to keep going; an
// impatient one loses to Rest or GoHome.
type KeepSkiing struct{}

func (KeepSkiing) Name() string { return "KeepSkiing" }

func (KeepSkiing) IsSatisfied(s *WorldSnapshot, w *world.World) bool {
	// Satisfied when AtLiftTop is non-zero — i.e. the plan ends in a fresh
	// lift unload. Cheapest path to "I just rode a lift."
	return s.AtLiftTop != 0
}

func (KeepSkiing) Weight(s *WorldSnapshot, w *world.World) float32 {
	combined := s.Patience
	if s.Energy < combined {
		combined = s.Energy
	}
	if combined < 0.2 {
		return combined * 0.5
	}
	return combined
}

// FulfillNeed is the goal of meeting one need (ai.NeedKind) at a
// building that offers something for it. Each need's thresholds and
// weight curve are in needSpecs; the goal only reads urgencies, never
// the stats behind them.
type FulfillNeed struct{ Kind ai.NeedKind }

// needSpec is how a need competes for a guest's time: active above
// urgency from, met at or below urgency done, and weighted by weight
// while active. preempts lets the need cut into a plan made before it
// pressed; blocked is the thought when no building can meet it.
type needSpec struct {
	from, done float32
	weight     func(u float32, s *WorldSnapshot) float32
	preempts   bool
	blocked    ai.ThoughtKind
}

// bodilyWeight is hunger's and thirst's: above KeepSkiing's best (1.0)
// as soon as they press, so a guest heads for food or a drink before it
// sours their day, and above GoHome's base when nearly empty.
func bodilyWeight(u float32, _ *WorldSnapshot) float32 { return 1.05 + (u - 0.75) }

var needSpecs = [ai.NeedCount]needSpec{
	// Hunger and thirst press below a quarter left, and a meal or a drink
	// fills them.
	ai.NeedHunger: {from: 0.75, done: 0.75, weight: bodilyWeight, preempts: true},
	ai.NeedThirst: {from: 0.75, done: 0.75, weight: bodilyWeight, preempts: true},
	// Rest presses only when energy or patience is nearly gone (under
	// 0.15), and a rest runs until both are back over 0.85. Quadratic, so
	// skiing wins until the guest is genuinely spent; nearly empty it tops
	// KeepSkiing and GoHome, so they try for a lodge before giving up.
	ai.NeedRest: {from: 0.85, done: 0.15, weight: func(u float32, _ *WorldSnapshot) float32 {
		if u > 0.95 {
			return 1.5
		}
		return u * u
	}, blocked: ai.ThoughtNeedsLodge},
	// Rentals is a hard gate (JoinQueue won't let a guest without skis
	// ride): first thing on arrival, ahead of skiing and a pass, and with
	// nowhere to rent the guest goes home (GoHome).
	ai.NeedRentals: {from: 0.5, done: 0.5, weight: func(float32, *WorldSnapshot) float32 { return 1.2 }, blocked: ai.ThoughtNoRentals},
	// Après is a want: worth little next to skiing in the late afternoon,
	// but a guest who's done for the day then (bored, or spent) stops at
	// the bar on the way out, and once the lifts close (urgency 1) every
	// après guest does.
	ai.NeedApres: {from: 0, done: 0, weight: func(u float32, s *WorldSnapshot) float32 {
		switch {
		case u >= 1:
			return goHomeClosedWeight + 0.1
		case s.Bored || min(s.Patience, s.Energy) < 0.05:
			return goHomeBoredWeight + 0.01
		}
		return 0.6 * u
	}},
	// Warmth presses like hunger once a guest is chilled, and they look
	// for a lounge's fire.
	ai.NeedWarmth: {from: 0.75, done: 0.1, weight: bodilyWeight, preempts: true, blocked: ai.ThoughtNoLounge},
}

func (g FulfillNeed) Name() string { return "Fulfill" + ai.NeedLabels[g.Kind] }

func (g FulfillNeed) IsSatisfied(s *WorldSnapshot, w *world.World) bool {
	return s.Need[g.Kind] <= needSpecs[g.Kind].done
}

func (g FulfillNeed) Weight(s *WorldSnapshot, w *world.World) float32 {
	spec := needSpecs[g.Kind]
	if u := s.Need[g.Kind]; u > spec.from {
		return spec.weight(u, s)
	}
	return 0
}

// PressingNeeds returns the needs that preempt plans (needSpec.preempts)
// and are unmet.
func PressingNeeds(s *WorldSnapshot, w *world.World) ai.NeedMask {
	var m ai.NeedMask
	for k := ai.NeedKind(0); k < ai.NeedCount; k++ {
		if needSpecs[k].preempts && !(FulfillNeed{k}).IsSatisfied(s, w) {
			m |= k.Mask()
		}
	}
	return m
}

// NeedPreempts reports whether a need that was not pressing when plan
// was made now outweighs the plan's goal, so the guest should replan
// rather than finish it.
func NeedPreempts(s *WorldSnapshot, w *world.World, plan *ai.Plan) bool {
	var planWeight float32
	for _, g := range AllGoals {
		if g.Name() == plan.GoalName {
			planWeight = g.Weight(s, w)
		}
	}
	for k := ai.NeedKind(0); k < ai.NeedCount; k++ {
		g := FulfillNeed{k}
		if !needSpecs[k].preempts || plan.Pressing.Has(k) || g.Name() == plan.GoalName || g.IsSatisfied(s, w) {
			continue
		}
		if g.Weight(s, w) > planWeight {
			return true
		}
	}
	return false
}

// Explore is satisfied once every skill-accessible lift has been ridden at
// least once. Weight is the fraction of accessible lifts still unridden —
// drops to zero when the skier has sampled every lift they can ride, which
// hands off to KeepSkiing (lapping) or GoHome.
type Explore struct{}

func (Explore) Name() string { return "Explore" }

func (Explore) IsSatisfied(s *WorldSnapshot, w *world.World) bool {
	for _, l := range w.Lifts {
		if !liftAccessible(l, s.Skill, w) {
			continue
		}
		if ai.RideCountOf(s.RidenLifts, l.ID) == 0 {
			return false
		}
	}
	return true
}

func (Explore) Weight(s *WorldSnapshot, w *world.World) float32 {
	total, unridden := 0, 0
	for _, l := range w.Lifts {
		if !liftAccessible(l, s.Skill, w) {
			continue
		}
		total++
		if ai.RideCountOf(s.RidenLifts, l.ID) == 0 {
			unridden++
		}
	}
	if total == 0 || unridden == 0 {
		return 0
	}
	// Gate on the worse of Patience and Energy — a frustrated or exhausted
	// skier shouldn't keep exploring new runs.
	gate := s.Patience
	if s.Energy < gate {
		gate = s.Energy
	}
	frac := float32(unridden) / float32(total)
	return frac * gate
}

// liftAccessible reports whether a guest at the given skill can ride lift l.
// Advanced guests ride any lift; beginners/intermediates need a matching
// difficulty service on the lift.
func liftAccessible(l *world.Lift, skill float32, w *world.World) bool {
	diff := skillDiff(skill)
	if diff == 0 {
		return true // Advanced: no filter
	}
	return w.ServicesForLift(l.ID).Has(diff)
}

// goHomeClosedWeight outranks every other goal's weight (rest tops out
// at 1.5), so at closing time guests head for their car.
const goHomeClosedWeight = 2.0

// goHomeBoredWeight puts going home just above KeepSkiing's best (1.0)
// for a bored guest, below a pressing need or a rest.
const goHomeBoredWeight = 1.02

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
	// Closed for the day: home before anything else, even a rest.
	if w.ClosedForDay {
		return goHomeClosedWeight
	}
	// Nothing left worth another run: home, ahead of more skiing.
	if s.Bored {
		return goHomeBoredWeight
	}
	// GoHome fires when Patience or Energy is critically low (Rest handles
	// the recoverable range), when Hunger or Thirst is exhausted, or when
	// the guest can no longer afford any lift.
	combined := s.Patience
	if s.Energy < combined {
		combined = s.Energy
	}
	if combined < 0.05 {
		return 1.0
	}
	if s.Need[ai.NeedHunger] > 0.95 || s.Need[ai.NeedThirst] > 0.95 {
		return 1.0
	}
	if !s.HasSeasonPass && s.CheapestTicket > 0 && s.RemainingBudget < s.CheapestTicket {
		return 1.0
	}
	// Came without skis and nowhere will rent them any.
	if s.Need[ai.NeedRentals] > 0 && !canRent(s, w) {
		return 1.0
	}
	return 0
}

// canRent reports whether some building rents skis at a price the guest
// will pay.
func canRent(s *WorldSnapshot, w *world.World) bool {
	for _, b := range w.Buildings {
		if b.Type == world.BuildingLodge && b.OffersUse(ai.OfferRentals) && affordable(b, ai.OfferRentals, s) {
			return true
		}
	}
	return false
}

// SelectGoal returns the highest-weighted unsatisfied goal, or nil if
// every goal is satisfied (shouldn't happen in normal play — KeepSkiing
// is satisfiable but the planner picks a new ride afterward).
func SelectGoal(s *WorldSnapshot, w *world.World) Goal {
	var best Goal
	bestW := float32(0)
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
