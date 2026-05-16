// Package goap is the L0 strategic-layer planner for skier AI.
//
// It implements goal-oriented action planning over a tiny typed world
// snapshot: each agent decides between high-level goals (KeepSkiing /
// Rest / Explore / GoHome) at replan time, and a forward A* search
// produces the cheapest action chain that satisfies the chosen goal.
// The per-tick continuous controller in internal/sim consumes the head
// action's goal target and never re-reads strategic state mid-tick.
//
// Trails are deliberately absent in this MVP — SkiToLift / SkiToLodge /
// SkiToParking enumerate over elevation-gated reachable destinations
// instead, and novelty rides on a per-agent RidenLifts ring rather than
// a per-trail RecentRuns ring. Trails land in a later phase.
package goap

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// WorldSnapshot is the per-agent typed state the planner reads. Extracted
// fresh at replan time; the planner mutates *copies* during search and
// never writes back through the snapshot. Positional fields are ID-valued
// (0 means "not there") — booleans like "at a lift base" are implicit in
// AtLiftBase != 0.
type WorldSnapshot struct {
	Pos    mgl32.Vec3
	Energy float32 // 0..1; depletes while skiing, restored at lodge
	Fun    float32 // 0..1; smoothed satisfaction signal
	Skill  ai.SkillLevel

	AtLiftBase uint64 // 0 or lift ID — at the base of this lift, not yet queued
	AtLiftTop  uint64 // 0 or lift ID — just unloaded at the top
	Queued     uint64 // 0 or lift ID — standing in this lift's queue
	OnLift     uint64 // 0 or lift ID — riding a chair
	AtLodge    uint64 // 0 or lodge building ID
	AtParking  uint64 // 0 or parking building ID
	AtTrailEnd uint64 // 0 or trail ID — arrived at a trail-to-trail junction

	// Removed flags a terminal state: agent has Departed. Planner treats
	// this as the unique goal-state for GoHome.
	Removed bool

	// RidenLifts is the per-lift ride tally. The novelty bonus on
	// RideLift reads this — first ride of a lift is a big Fun gain,
	// subsequent rides taper geometrically. Stored as a flat slice so
	// Clone is a cheap copy; A* allocates one Clone per node expansion
	// and a map there was the dominant source of main-loop stalls.
	RidenLifts []ai.RideCount
}

// Clone returns a deep copy suitable for planner search expansion.
// RidenLifts is copied; the rest are value types.
func (s WorldSnapshot) Clone() WorldSnapshot {
	out := s
	if len(s.RidenLifts) > 0 {
		out.RidenLifts = append(make([]ai.RideCount, 0, len(s.RidenLifts)), s.RidenLifts...)
	} else {
		out.RidenLifts = nil
	}
	return out
}

// Extract builds a fresh snapshot for agent a from the live world. Called
// at replan time. The positional ID fields are derived by proximity to
// known anchors (within proximityRadius m) plus the agent's implicit-state
// markers (OnLiftID, Queued). An agent in transit between anchors lands
// with all positional IDs zero — the planner treats that as "complete the
// current action first" by re-deriving once the agent reaches the next
// anchor.
//
// The lift queue lookup walks every lift's queue slice; with ≤10 lifts
// and short queues this is microseconds. A queued-lift back-pointer on
// Agent would avoid the walk but adds a field that has to be kept in
// sync with the queue mutations in tickLifts — not worth it yet.
func Extract(a *world.Guest, w *world.World) WorldSnapshot {
	snap := WorldSnapshot{
		Pos:        a.Pos,
		Energy:     a.Energy,
		Fun:        a.Fun,
		Skill:      a.Traits.Skill,
		OnLift:     a.OnLiftID,
		AtTrailEnd: a.AtTrailEnd,
		RidenLifts: a.RidenLifts,
	}
	if a.Queued {
		for _, l := range w.Lifts {
			for _, q := range l.Queue {
				if q == a {
					snap.Queued = l.ID
					break
				}
			}
			if snap.Queued != 0 {
				break
			}
		}
	}
	if snap.OnLift != 0 || snap.Queued != 0 {
		return snap
	}
	if len(a.Path) > 0 && a.PathIdx < len(a.Path) {
		// Walking a pathfinder route — in transit, no anchor.
		return snap
	}
	// Proximity check: pick the nearest anchor (lift base, lift top, lodge,
	// parking) within proximityRadius. Ties prefer lift bases over tops over
	// buildings since base/top are tighter targets in practice.
	const r2 = proximityRadius * proximityRadius
	for _, l := range w.Lifts {
		if sqDistXZ(a.Pos, l.Base[0], l.Base[1]) < r2 {
			snap.AtLiftBase = l.ID
			return snap
		}
	}
	for _, l := range w.Lifts {
		if sqDistXZ(a.Pos, l.Top[0], l.Top[1]) < r2 {
			snap.AtLiftTop = l.ID
			return snap
		}
	}
	for _, b := range w.Buildings {
		if sqDistXZ(a.Pos, b.Pos[0], b.Pos[1]) >= r2 {
			continue
		}
		switch b.Type {
		case world.BuildingLodge:
			snap.AtLodge = b.ID
			return snap
		case world.BuildingParking:
			snap.AtParking = b.ID
			return snap
		}
	}
	return snap
}

// proximityRadius is the radius (m) within which an agent counts as "at"
// an anchor for snapshot extraction. ArrivalThreshold (2 m) is too tight
// — the agent may stop walking a few metres short of the canonical anchor
// because the pathfinder routes to a queue slot, not the base anchor.
// 8 m matches the boarding spot tolerance used elsewhere in the sim.
const proximityRadius = 8.0

func sqDistXZ(p mgl32.Vec3, x, z float32) float32 {
	dx := p[0] - x
	dz := p[2] - z
	return dx*dx + dz*dz
}
