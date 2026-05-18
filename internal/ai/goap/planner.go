package goap

import (
	"container/heap"
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl32"

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
	closed := make(map[string]float32)

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
func (p *Planner) StoredPlanFor(a *world.Guest, w *world.World, simTime float64) ai.Plan {
	snap := Extract(a, w)
	return p.planFromSnap(snap, a, w, simTime)
}

// StoredPlanForLookahead plans as if agent a has just unloaded from liftID.
// Called at chair-load time so the guest has a complete post-ride plan before
// reaching the top. The returned ai.Plan does NOT include the in-flight
// RideLift step — callers prepend it.
func (p *Planner) StoredPlanForLookahead(a *world.Guest, liftID uint64, w *world.World, simTime float64) ai.Plan {
	snap := ExtractLookahead(a, liftID, w)
	return p.planFromSnap(snap, a, w, simTime)
}

// planFromSnap runs goal-selection and A* from snap. Goals with weight ≤ 0
// are skipped — zero-weight goals (GoHome at full patience) must not win
// by default. If Rest is unreachable a thought is emitted and the next
// goal is tried. Falls back to defaultLapPlan when no goal produces a plan.
func (p *Planner) planFromSnap(snap WorldSnapshot, a *world.Guest, w *world.World, simTime float64) ai.Plan {
	for _, gr := range RankedGoals(&snap, w) {
		if gr.Satisfied || gr.Weight <= 0 {
			continue
		}
		actions := p.Plan(snap, gr.Goal, w)
		if actions == nil {
			if _, ok := gr.Goal.(Rest); ok {
				a.AddThought(ai.ThoughtNeedsLodge, simTime)
				a.Satisfaction -= 0.06
				if a.Satisfaction < 0 {
					a.Satisfaction = 0
				}
			}
			continue
		}
		out := ai.Plan{GoalName: gr.Goal.Name()}
		if len(actions) > 0 {
			out.Steps = ToPlanActions(actions, snap, w)
		}
		return out
	}
	// No unsatisfied goal with positive weight — keep lapping.
	return defaultLapPlan(snap, a, w)
}

// defaultLapPlan builds a minimal [SkiToLift, JoinQueue, RideLift] plan
// directly when no goal has positive unsatisfied weight. Picks the
// skill-accessible lift reachable from snap.AtLiftTop with the fewest
// prior rides (preferring novelty even when Explore is satisfied).
// Returns an empty plan if no lap is possible.
func defaultLapPlan(snap WorldSnapshot, a *world.Guest, w *world.World) ai.Plan {
	if snap.AtLiftTop == 0 {
		return ai.Plan{}
	}
	src := findLift(w, snap.AtLiftTop)
	if src == nil {
		return ai.Plan{}
	}
	var best *world.Lift
	bestRides := int(^uint(0) >> 1)
	for _, l := range w.Lifts {
		if !liftAccessible(l, snap.Skill, w) {
			continue
		}
		if liftTopElev(w, src)-liftBaseElev(w, l) < minDescentMeters {
			continue
		}
		if len(l.Queue) > MaxQueuePersons {
			continue
		}
		rides := ai.RideCountOf(snap.RidenLifts, l.ID)
		if rides < bestRides {
			bestRides = rides
			best = l
		}
	}
	if best == nil {
		return ai.Plan{}
	}
	skiCost := distXZ(mgl32.Vec3{src.Top[0], 0, src.Top[1]}, best.Base[0], best.Base[1]) / skiSpeedMps
	qCost := float32(len(best.Queue)) * queueSlotSec
	rideCost := best.LoopLength() / (2 * best.Speed)
	return ai.Plan{
		GoalName: "KeepSkiing",
		Steps: []ai.PlanAction{
			{Kind: ai.ActSkiToLift, LiftID: best.ID, Cost: skiCost},
			{Kind: ai.ActJoinQueue, LiftID: best.ID, Cost: qCost},
			{Kind: ai.ActRideLift, LiftID: best.ID, Cost: rideCost},
		},
	}
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

// stateKey serialises a snapshot to a stable string for closed-set
// membership. Pos isn't included — two different positions that share
// all anchor IDs and stat values are equivalent for planning purposes
// (the L1 controller handles the spatial detail), and including Pos
// would force re-expansion of essentially-the-same state every time
// Apply slightly nudges Pos. Patience is bucketed to 0.01 to keep
// the closed set finite.
func stateKey(s *WorldSnapshot) string {
	pb := int(s.Patience * 100)
	ridden := 0
	for _, r := range s.RidenLifts {
		ridden += r.Count
	}
	return fmt.Sprintf("P%dB%dT%dQ%dL%dD%dPt%dR%dX%vJ%d",
		pb,
		s.AtLiftBase, s.AtLiftTop, s.Queued, s.OnLift,
		s.AtLodge, s.AtParking,
		ridden, s.Removed,
		s.AtTrailEnd,
	)
}
