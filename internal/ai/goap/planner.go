package goap

import (
	"container/heap"
	"fmt"
	"math"

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

// PlanForAgent is the convenience entry point used by sim: extract a
// snapshot, pick the highest-weighted unsatisfied goal, and plan. Returns
// (plan, goal, snapshot) — goal and snapshot are returned for HUD
// display so callers don't repeat the Extract / SelectGoal calls.
func (p *Planner) PlanForAgent(a *world.Agent, w *world.World) ([]Action, Goal, WorldSnapshot) {
	snap := Extract(a, w)
	goal := SelectGoal(&snap, w)
	if goal == nil {
		return nil, nil, snap
	}
	return p.Plan(snap, goal, w), goal, snap
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
// Apply slightly nudges Pos. Energy/Fun are bucketed to 0.01 to keep
// the closed set finite.
func stateKey(s *WorldSnapshot) string {
	eb := int(s.Energy * 100)
	fb := int(s.Fun * 100)
	ridden := 0
	if s.RidenLifts != nil {
		for _, v := range s.RidenLifts {
			ridden += v
		}
	}
	return fmt.Sprintf("E%dF%dB%dT%dQ%dL%dD%dP%dR%dX%v",
		eb, fb,
		s.AtLiftBase, s.AtLiftTop, s.Queued, s.OnLift,
		s.AtLodge, s.AtParking,
		ridden, s.Removed,
	)
}
