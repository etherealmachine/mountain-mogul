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
	Pos      mgl32.Vec3
	Patience float32 // 0..1; drains while queuing, restored by skiing/riding/lodge
	Energy   float32 // 0..1; drains while skiing, restored by RestAtLodge
	// Need is each need's urgency, 0..1 (world.Guest.NeedUrgency). The
	// need goals read these, never the stats behind them; using an offer
	// zeroes the needs it fulfils. Patience and Energy above stay for the
	// skiing goals.
	Need   [ai.NeedCount]float32
	Skill  float32
	Tastes ai.Tastes // what snow and terrain the guest enjoys, for choosing lifts
	Bored  bool      // nothing left worth another run (sim.checkBoredom): time to go home

	AtLiftBase     uint64 // 0 or lift ID — at the base of this lift, not yet queued
	AtLiftTop      uint64 // 0 or lift ID — just unloaded at the top
	Queued         uint64 // 0 or lift ID — standing in this lift's queue
	OnLift         uint64 // 0 or lift ID — riding a chair
	AtService      uint64 // 0 or the building guests can use something at (a seat, a meal, a drink)
	AtParking      uint64 // 0 or parking building ID
	AtTrailEnd     uint64 // 0 or trail ID — arrived at a trail-to-trail junction
	AtTicketOffice uint64 // 0 or ticket office building ID

	// CarLot is the lot the guest's car is parked in; they depart only
	// from there. 0 = any lot.
	CarLot uint64

	// RemainingBudget is the guest's unspent visit money after the day
	// ticket (bought or still owed at the window). Decremented by heli
	// fares (or the season pass fee); when it falls below CheapestTicket
	// the GoHome goal fires (unless the guest has a pass).
	RemainingBudget float32
	// Budget is the guest's whole daily budget: how much they expect to
	// pay for things (world.ExpectedPrice), not what they have left.
	Budget float32
	// PassCredit is today's day ticket, bought or owed, credited toward a
	// season pass bought this visit. A pass costs SeasonPassPrice -
	// PassCredit out of RemainingBudget.
	PassCredit float32
	// CheapestTicket is the minimum per-ride fare across all lifts,
	// precomputed at Extract time so goal/action logic needs no world walk.
	// Zero when the world has no lifts or any lift is free per ride.
	CheapestTicket float32

	// HasSeasonPass is true when the guest holds a valid season pass. Pass
	// holders skip lift charges and are never budget-gated from joining a queue.
	HasSeasonPass bool
	// HasDayTicket is true once the guest has bought today's day ticket at
	// a ticket office. JoinQueue needs this or HasSeasonPass.
	HasDayTicket bool

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
		Pos:             a.Pos,
		Patience:        a.Patience,
		Energy:          a.Energy,
		Need:            needUrgencies(a),
		Skill:           a.Traits.Skill,
		Tastes:          a.Traits.Tastes,
		Bored:           a.Conditions.Has(ai.ThoughtBored) || a.Conditions.Has(ai.ThoughtNotMySkiing),
		RemainingBudget: a.RemainingBudget,
		Budget:          a.Traits.DailyBudget,
		PassCredit:      float32(a.DayTicketPaid + a.DayTicketDue),
		CheapestTicket:  cheapestTicket(w),
		HasSeasonPass:   a.HasSeasonPass,
		HasDayTicket:    a.HasDayTicket,
		OnLift:          a.OnLiftID,
		AtTrailEnd:      a.AtTrailEnd,
		RidenLifts:      a.RidenLifts,
		CarLot:          a.CarLot,
	}
	poolGroup(&snap, a)
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
	// qArrR2 matches sim.ArrivalThreshold (2 m) — the distance at which the
	// L1 controller snaps a skier to their target. Used to fire AtLiftBase
	// when a SkiToLift guest reaches the back of a long queue whose end is
	// beyond the 8 m base proximity radius.
	const qArrR2 = float32(2.0 * 2.0)
	for _, l := range w.Lifts {
		if !l.Open || l.OnHold {
			continue
		}
		if sqDistXZ(a.Pos, l.Base[0], l.Base[1]) < r2 {
			snap.AtLiftBase = l.ID
			return snap
		}
		qback := l.BackOfQueueWorldPos(w.Terrain)
		if sqDistXZ(a.Pos, qback[0], qback[2]) < qArrR2 {
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
		if !b.Usable() {
			continue
		}
		p, _ := b.NearestEntrance(mgl32.Vec2{a.Pos[0], a.Pos[2]})
		if sqDistXZ(a.Pos, p[0], p[1]) >= r2 {
			continue
		}
		if setAtBuilding(&snap, b) {
			return snap
		}
	}
	return snap
}

// setAtBuilding anchors s at building b: a parking lot, or every service
// b offers (a service building is one place inside, whichever door the
// guest came in by). Clears the other building anchors. Reports whether
// b is somewhere guests can be.
func setAtBuilding(s *WorldSnapshot, b *world.Building) bool {
	s.AtService, s.AtParking, s.AtTicketOffice = 0, 0, 0
	switch {
	case b.Type == world.BuildingParking:
		s.AtParking = b.ID
	case b.IsShell() && b.ServesGuests():
		if b.OffersAnyUse() {
			s.AtService = b.ID
		}
		if b.Offers(world.ServiceTickets) {
			s.AtTicketOffice = b.ID
		}
	default:
		return false
	}
	return true
}

// ExtractLookahead returns a WorldSnapshot as if agent a has just unloaded
// from liftID: AtLiftTop is set, OnLift/Queued are cleared, and the ride is
// pre-recorded in a copy of RidenLifts. Called at chair-load time so the
// planner can build a post-ride plan while the agent is still airborne.
func ExtractLookahead(a *world.Guest, liftID uint64, w *world.World) WorldSnapshot {
	rides := append(make([]ai.RideCount, 0, len(a.RidenLifts)), a.RidenLifts...)
	rides = ai.AddRide(rides, liftID)
	snap := WorldSnapshot{
		Pos:             a.Pos,
		Patience:        a.Patience,
		Energy:          a.Energy,
		Need:            needUrgencies(a),
		Skill:           a.Traits.Skill,
		Tastes:          a.Traits.Tastes,
		Bored:           a.Conditions.Has(ai.ThoughtBored) || a.Conditions.Has(ai.ThoughtNotMySkiing),
		RemainingBudget: a.RemainingBudget,
		Budget:          a.Traits.DailyBudget,
		PassCredit:      float32(a.DayTicketPaid + a.DayTicketDue),
		CheapestTicket:  cheapestTicket(w),
		HasSeasonPass:   a.HasSeasonPass,
		HasDayTicket:    a.HasDayTicket,
		AtLiftTop:       liftID,
		AtTrailEnd:      a.AtTrailEnd,
		RidenLifts:      rides,
	}
	poolGroup(&snap, a)
	return snap
}

// poolGroup plans for a group's leader as for the whole group: the
// weakest member's skill, the lowest patience and energy, and the most
// pressing of each need the group sees to together (not rentals, which
// only the guest without skis needs). Followers copy the leader's plans
// (sim/groups.go).
//
// Only followers with the leader count: near them, on the leader's plan
// (not lost and planning for themselves).
func poolGroup(s *WorldSnapshot, a *world.Guest) {
	for _, f := range a.Party.Followers {
		if f.State != world.OnMountain || f.Removed || !f.WithLeader() || f.Pos.Sub(a.Pos).Len() > poolNear {
			continue
		}
		s.Skill = min(s.Skill, f.Traits.Skill)
		s.Patience = min(s.Patience, f.Patience)
		s.Energy = min(s.Energy, f.Energy)
		for k := ai.NeedKind(0); k < ai.NeedCount; k++ {
			if k != ai.NeedRentals {
				s.Need[k] = max(s.Need[k], f.NeedUrgency(k))
			}
		}
	}
}

// poolNear is how close, in metres, a follower has to be for the leader
// to plan for them.
const poolNear = 200

// needUrgencies is each of a's needs' urgency.
func needUrgencies(a *world.Guest) [ai.NeedCount]float32 {
	var u [ai.NeedCount]float32
	for k := ai.NeedKind(0); k < ai.NeedCount; k++ {
		u[k] = a.NeedUrgency(k)
	}
	return u
}

// cheapestTicket returns the minimum per-ride fare across all lifts, or 0
// if there are none or any lift is free per ride. Precomputed into
// WorldSnapshot so goal and action logic never walk the lift list themselves.
func cheapestTicket(w *world.World) float32 {
	min := float32(0)
	for i, l := range w.Lifts {
		if p := float32(l.RideFare()); i == 0 || p < min {
			min = p
		}
	}
	return min
}

// hasTicket reports whether the guest may ride: a valid season pass or
// today's day ticket bought at the window.
func hasTicket(s *WorldSnapshot) bool {
	return s.HasSeasonPass || s.HasDayTicket
}

// passCost is what a season pass costs this guest right now: the pass
// price less today's day ticket credit, floored at zero.
func passCost(s *WorldSnapshot, w *world.World) float32 {
	c := float32(w.SeasonPassPrice) - s.PassCredit
	if c < 0 {
		return 0
	}
	return c
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
