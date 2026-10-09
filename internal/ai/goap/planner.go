package goap

import (
	"container/heap"
	"math"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Planner runs the A* search over the action graph. Stateless — held as
// a struct so the caller can pre-allocate scratch buffers later without
// changing the API. The search is forward (start state → goal predicate)
// because the trail-free action graph fans heavily on SkiTo* and backward
// search would have to enumerate predecessors over the full lift set per
// expansion.
type Planner struct {
	// MaxPlanLen caps how deep the search expands before giving up. With
	// the MVP action set, a typical plan is 4–8 steps (walk → queue →
	// ride → ski → queue → ride → ski → depart). 16 leaves slack for
	// resorts with several lifts that take multiple rides to exhaust.
	MaxPlanLen int
	// MaxExpansions caps total node expansions per Plan call. The graph
	// has small branching factor but with deep plans and a heuristic of
	// zero (admissible-but-loose), A* can wander; this is a hard ceiling
	// so a pathological state doesn't burn frames. 4000 is generous —
	// typical calls finish in tens of expansions.
	MaxExpansions int
}

// NewPlanner returns a Planner with default tuning.
func NewPlanner() *Planner {
	return &Planner{
		MaxPlanLen:    16,
		MaxExpansions: 4000,
	}
}

// planNode is one search-tree node. Snapshot + cost-so-far + parent link
// for reconstruction.
type planNode struct {
	snap   WorldSnapshot
	gCost  float32
	parent *planNode
	action Action
	depth  int
}

// Plan searches for the cheapest sequence of actions whose terminal state
// satisfies goal. Returns nil if no plan is found within the depth /
// expansion limits. The returned slice is in execution order (head action
// first).
//
// Heuristic is currently zero (Dijkstra). Adding a goal-specific
// admissible heuristic later — e.g. "min lift-ride time to satisfy
// Explore" — would tighten search but the small action graph hasn't
// motivated it yet.
func (p *Planner) Plan(start WorldSnapshot, goal Goal, w *world.World) []Action {
	if goal == nil {
		return nil
	}
	if goal.IsSatisfied(&start, w) {
		return []Action{}
	}

	openList := &nodeHeap{}
	heap.Init(openList)
	closed := make(map[planKey]float32)

	startNode := &planNode{snap: start.Clone()}
	heap.Push(openList, &heapItem{n: startNode, f: 0})

	expansions := 0
	for openList.Len() > 0 {
		expansions++
		if expansions > p.MaxExpansions {
			return nil
		}
		cur := heap.Pop(openList).(*heapItem).n
		if goal.IsSatisfied(&cur.snap, w) {
			return reconstruct(cur)
		}
		if cur.depth >= p.MaxPlanLen {
			continue
		}
		key := stateKey(&cur.snap)
		if prev, ok := closed[key]; ok && prev <= cur.gCost {
			continue
		}
		closed[key] = cur.gCost

		for _, a := range ApplicableActions(&cur.snap, w) {
			next := cur.snap.Clone()
			a.Apply(&next, w)
			cost := a.Cost(&cur.snap, w)
			if cost < 0 || math.IsInf(float64(cost), 0) {
				continue
			}
			gNext := cur.gCost + cost
			child := &planNode{
				snap:   next,
				gCost:  gNext,
				parent: cur,
				action: a,
				depth:  cur.depth + 1,
			}
			heap.Push(openList, &heapItem{n: child, f: gNext})
		}
	}
	return nil
}

// PlanForGuest is the convenience entry point used by sim: extract a
// snapshot, pick the highest-weighted unsatisfied goal, and plan. Returns
// (plan, goal, snapshot) — goal and snapshot are returned for HUD
// display so callers don't repeat the Extract / SelectGoal calls.
func (p *Planner) PlanForGuest(a *world.Guest, w *world.World) ([]Action, Goal, WorldSnapshot) {
	snap := Extract(a, w)
	goal := SelectGoal(&snap, w)
	if goal == nil {
		return nil, nil, snap
	}
	return p.Plan(snap, goal, w), goal, snap
}

// StoredPlanFor returns a freshly computed ai.Plan ready to drop onto
// world.Guest.Plan. The simulation's replan path uses this; the HUD
// reads the stored result instead of recomputing each frame.
func (p *Planner) StoredPlanFor(a *world.Guest, w *world.World) ai.Plan {
	snap := Extract(a, w)
	return p.planFromSnap(snap, a, w)
}

// StoredPlanForLookahead plans as if agent a has just unloaded from liftID.
// Called at chair-load time so the guest has a complete post-ride plan before
// reaching the top. The returned ai.Plan does NOT include the in-flight
// RideLift step — callers prepend it.
func (p *Planner) StoredPlanForLookahead(a *world.Guest, liftID uint64, w *world.World) ai.Plan {
	snap := ExtractLookahead(a, liftID, w)
	return p.planFromSnap(snap, a, w)
}

// planFromSnap runs goal-selection and A* from snap. Goals with weight ≤ 0
// are skipped — zero-weight goals (GoHome at full patience) must not win
// by default. A goal that can't be planned (Rest with no lodge, a ride
// with no lift) is reported in Plan.Blocked and the next goal is tried.
// Falls back to lapPlan when no goal produces a plan.
func (p *Planner) planFromSnap(snap WorldSnapshot, a *world.Guest, w *world.World) ai.Plan {
	plan := p.pickPlan(snap, a, w)
	plan.Pressing = PressingNeeds(&snap, w)
	return plan
}

func (p *Planner) pickPlan(snap WorldSnapshot, a *world.Guest, w *world.World) ai.Plan {
	var blocked []ai.ThoughtKind
	for _, gr := range RankedGoals(&snap, w) {
		if gr.Satisfied || gr.Weight <= 0 {
			continue
		}
		actions := p.Plan(snap, gr.Goal, w)
		if actions == nil {
			if fn, ok := gr.Goal.(FulfillNeed); ok {
				switch {
				case pricedOut(&snap, w, fn.Kind):
					blocked = appendBlocked(blocked, ai.ThoughtPricesTooHigh)
				case needSpecs[fn.Kind].blocked != ai.ThoughtNone:
					blocked = appendBlocked(blocked, needSpecs[fn.Kind].blocked)
				}
			}
			if ridesLift(gr.Goal) {
				if k := rideBlocker(&snap, w); k != ai.ThoughtNone {
					blocked = appendBlocked(blocked, k)
				}
			}
			continue
		}
		out := ai.Plan{GoalName: gr.Goal.Name(), Blocked: blocked}
		if len(actions) > 0 {
			out.Steps = ToPlanActions(actions, snap, w)
		}
		return out
	}
	// No unsatisfied goal with positive weight — keep lapping.
	plan := p.lapPlan(snap, w)
	plan.Blocked = blocked
	return plan
}

// appendBlocked adds k to the blocked list once.
func appendBlocked(blocked []ai.ThoughtKind, k ai.ThoughtKind) []ai.ThoughtKind {
	for _, b := range blocked {
		if b == k {
			return blocked
		}
	}
	return append(blocked, k)
}

// rideBlocker is the thought for why a guest can't plan a lift ride,
// checked in order: no lift running, none running they'd ride (for their
// level; for a novice, all-green terrain), every
// line they could join over world.MaxLineWait, or (with none of those) no
// ticket and no way to buy one. ThoughtNone when none of these is the
// reason.
func rideBlocker(s *WorldSnapshot, w *world.World) ai.ThoughtKind {
	switch {
	case !w.AnyLiftRunning():
		return ai.ThoughtLiftsClosed
	case !runningLiftFor(s.Skill, w):
		return ai.ThoughtNothingForMe
	case s.Patience >= 0.05 && allLinesFull(s, w):
		return ai.ThoughtLinesFull
	case s.Need[ai.NeedRentals] > 0:
		return ai.ThoughtNoRentals
	case !hasTicket(s):
		return ai.ThoughtNoTicketWindow
	}
	return ai.ThoughtNone
}

// allLinesFull reports whether every running lift the guest would ride
// has a line longer than they'll join.
func allLinesFull(s *WorldSnapshot, w *world.World) bool {
	any := false
	for _, l := range w.Lifts {
		if !l.Open || l.OnHold || !liftAccessible(l, s.Skill, w) {
			continue
		}
		any = true
		if l.LineWait() <= world.MaxLineWait {
			return false
		}
	}
	return any
}

// ridesLift reports whether goal g can only be met by riding a lift.
func ridesLift(g Goal) bool {
	switch g.(type) {
	case KeepSkiing, Explore:
		return true
	}
	return false
}

// lapGoal is the fallback when no goal wants anything: get in another
// lift line, the cheapest way the guest's rules allow (trails at their
// level unless they free-roam), favouring lifts they've ridden least
// (RideLift's repeat penalty doesn't apply here; JoinQueue's line cost
// and the descent do).
type lapGoal struct{}

func (lapGoal) Name() string { return "KeepSkiing" }

func (lapGoal) IsSatisfied(s *WorldSnapshot, w *world.World) bool { return s.Queued != 0 }

func (lapGoal) Weight(s *WorldSnapshot, w *world.World) float32 { return 1 }

// lapPlan plans lapGoal from snap: a descent (or a walk) to a lift line
// and joining it; the guest plans the ride from the line. Empty when no
// lap is possible.
func (p *Planner) lapPlan(snap WorldSnapshot, w *world.World) ai.Plan {
	if !hasTicket(&snap) {
		return ai.Plan{}
	}
	actions := p.Plan(snap, lapGoal{}, w)
	if len(actions) == 0 {
		return ai.Plan{}
	}
	return ai.Plan{GoalName: lapGoal{}.Name(), Steps: ToPlanActions(actions, snap, w)}
}

// reconstruct walks the parent chain from a goal node back to the start
// node and reverses to produce a head-first action slice.
func reconstruct(n *planNode) []Action {
	var out []Action
	for cur := n; cur != nil && cur.action != nil; cur = cur.parent {
		out = append(out, cur.action)
	}
	// Reverse in place: parent chain runs goal→start.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// heapItem wraps a planNode with its f-cost for the priority queue. We
// keep f out of planNode so the search node stays a pure state
// description.
type heapItem struct {
	n *planNode
	f float32
}

type nodeHeap []*heapItem

func (h nodeHeap) Len() int           { return len(h) }
func (h nodeHeap) Less(i, j int) bool { return h[i].f < h[j].f }
func (h nodeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *nodeHeap) Push(x any)        { *h = append(*h, x.(*heapItem)) }
func (h *nodeHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// planKey is a snapshot's place in the closed set: every ID that
// defines the agent's discrete location and status. Pos is omitted (L1
// handles continuous movement); stats are bucketed to 0.01 to keep the
// search space finite. A struct, not a formatted string: building the
// string was most of a plan's cost.
type planKey struct {
	patience, energy int32
	need             [ai.NeedCount]int32
	liftBase, liftTop, queued, onLift,
	service, parking, ticketOffice, trailEnd uint64
	ridden                         int
	removed, seasonPass, dayTicket bool
}

// stateKey is s's planKey.
func stateKey(s *WorldSnapshot) planKey {
	k := planKey{
		patience: int32(s.Patience * 100), energy: int32(s.Energy * 100),
		liftBase: s.AtLiftBase, liftTop: s.AtLiftTop, queued: s.Queued, onLift: s.OnLift,
		service: s.AtService, parking: s.AtParking, ticketOffice: s.AtTicketOffice,
		trailEnd: s.AtTrailEnd,
		removed:  s.Removed, seasonPass: s.HasSeasonPass, dayTicket: s.HasDayTicket,
	}
	for i, u := range s.Need {
		k.need[i] = int32(u * 100)
	}
	for _, r := range s.RidenLifts {
		k.ridden += r.Count
	}
	return k
}
