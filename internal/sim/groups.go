package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/ai/goap"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// Groups on the mountain (notes/next/Groups.md). A group's first member
// out of the car leads: plans for the whole group (goap.poolGroup: the
// weakest skier, the most pressing needs), routes round the trees, and
// steers with the full fan. The others follow:
//
//   - Plans: when a follower needs a plan, they copy the leader's latest
//     one made from the same place (the lot they arrived at, a lift's
//     top, a lodge), filtered for what they need themselves (a ticket,
//     rentals). With none yet, they wait a while for the leader to catch
//     up (at the top of a lift the leader's still on, or a lodge they're
//     still in), then plan for themselves.
//   - Descents: the leader drops a crumb every few metres on their line;
//     a follower on the same descent steers for a point a little ahead
//     of them on it, off to their own side (pure pursuit), with no route
//     search and no fan of hazard samples. Their physics, tracks, snow,
//     falls, and swerving round whoever's close are their own.
//   - The leader waits at the top of a lift for followers still riding it.
//
// A follower who loses the group (a fall, a different lift) skis and
// plans as a guest alone until a copy fits again.

// GroupStats counts what groups did, for measuring them: steps
// followers skied (Steps) and of those on the leader's line (OnLine);
// plans followers copied (Copied), made themselves (Own), and steps they
// stood waiting (Waits); and plans leaders made (Led).
type GroupStats struct {
	Steps, OnLine, Copied, Own, Waits, Led int
	// Off counts follower steps off the line by why (offLine), and
	// OwnWhy followers' own plans by why (own*).
	Off    [offReasons]int
	OwnWhy [ownReasons]int
}

// Why a follower made their own plan.
const (
	ownBroke  = iota // the leader went on to a plan that doesn't fit theirs
	ownWaited        // they waited for the leader as long as they would
	ownLost          // they were on their own plan already
	ownBoard         // boarding a lift the leader isn't on, lost
	ownReasons
)

// Why a follower skied a step off their leader's line.
const (
	offNoLine   = iota // the leader has no line yet
	offNotDown         // not on a descent
	offOtherEnd        // on a descent to somewhere else
	offFinish          // close to where they're going
	offLost            // too far from the line
	offHead            // at its head, level with or ahead of the leader
	offLong            // on the same descent for followRunSec
	offReasons
)

// GroupStats is the counts so far.
func (s *Simulation) GroupStats() GroupStats { return s.groupStats }

// joinParty puts g, just out of their car, with their group: following
// the member who leads, or leading if they're the first out.
func (s *Simulation) joinParty(g *world.Guest) {
	g.Party = world.Party{}
	if g.GroupID == 0 {
		return
	}
	for _, m := range s.World.GroupOf(g) {
		if m == g || m.State != world.OnMountain || m.Removed || m.Party.Leader != nil {
			continue
		}
		g.Party.Leader = m
		m.Party.Followers = append(m.Party.Followers, g)
		g.Party.Rank = len(m.Party.Followers)
		// Alternate sides, a little further out every pair, never
		// quite the same.
		side := float32(followSideMin) + followSideStep*float32((g.Party.Rank-1)/2)
		side = min(side, followSideMax) + (rng.Global().Float32()-0.5)*followSideJitter
		if g.Party.Rank%2 == 0 {
			side = -side
		}
		g.Party.FollowSide = side
		return
	}
}

// Followers ski followSideMin to followSideMax metres to one side of the
// leader's line, followSideStep further out each pair, give or take
// half of followSideJitter.
const (
	followSideMin    = 2.0
	followSideStep   = 1.5
	followSideMax    = float32(6)
	followSideJitter = float32(1.5)
)

// leaveParty takes g, leaving the mountain, out of their group: a
// leader hands the group to their first follower still out.
func leaveParty(g *world.Guest) {
	if l := g.Party.Leader; l != nil {
		for i, f := range l.Party.Followers {
			if f == g {
				l.Party.Followers = append(l.Party.Followers[:i], l.Party.Followers[i+1:]...)
				break
			}
		}
		g.Party.Leader = nil
		return
	}
	var next *world.Guest
	for _, f := range g.Party.Followers {
		if f.Removed || f.State != world.OnMountain {
			f.Party.Leader = nil
			continue
		}
		if next == nil {
			next = f
			f.Party.Leader = nil
			continue
		}
		f.Party.Leader = next
		next.Party.Followers = append(next.Party.Followers, f)
		f.Party.Rank = len(next.Party.Followers)
	}
	if next != nil {
		next.Party.Crumbs = g.Party.Crumbs
		next.Party.Shared = g.Party.Shared
		next.Party.SharedHead = g.Party.SharedHead
	}
	g.Party.Followers = nil
}

// regroup puts guests already out (a loaded game) in their groups.
func (s *Simulation) regroup() {
	for _, g := range s.World.OnMountain {
		g.Party = world.Party{}
	}
	for _, g := range s.World.OnMountain {
		if !g.Removed {
			s.joinParty(g)
		}
	}
}

// leads reports whether a has followers out on the mountain.
func leads(a *world.Guest) bool { return len(a.Party.Followers) > 0 }

// planFrom is where a plan for a would be made from: the lot they're
// arriving at (s.arriving), or the top of the lift they've just got off.
// Zero anywhere else.
func (s *Simulation) planFrom(a *world.Guest) world.PlanFrom {
	if s.arriving != 0 {
		return world.PlanFrom{Arrive: s.arriving}
	}
	if a.Unload.LiftID != 0 {
		return world.PlanFrom{LiftTop: a.Unload.LiftID}
	}
	return world.PlanFrom{LiftTop: goap.Extract(a, s.World).AtLiftTop}
}

// share records the plan p the leader a has just taken on, replacing
// their last after done of its steps, for followers (who may not be out
// of the car yet). from is where it was made from.
func (s *Simulation) share(a *world.Guest, from world.PlanFrom, p ai.Plan, done int) {
	if a.GroupID == 0 || a.Party.Leader != nil || len(s.World.GroupOf(a)) < 2 {
		return
	}
	s.planGen++
	p.Step = 0
	a.Party.Shared[a.Party.SharedHead] = world.SharedPlan{
		Gen: s.planGen, Prev: a.Party.Gen, Done: done, From: from, At: s.SimTime, Plan: p,
	}
	a.Party.SharedHead = (a.Party.SharedHead + 1) % world.SharedPlans
	a.Party.Gen = s.planGen
	s.groupStats.Led++
}

// successor is the leader l's plan that replaced plan gen, nil when they
// haven't replaced it (or it's long gone).
func successor(l *world.Guest, gen uint32) *world.SharedPlan {
	for i := range l.Party.Shared {
		if sp := &l.Party.Shared[i]; sp.Prev == gen && sp.Gen != 0 {
			return sp
		}
	}
	return nil
}

// sameStep reports whether two plan steps do the same thing (a season
// pass and a day ticket are both buying a ticket).
func sameStep(x, y ai.PlanAction) bool {
	buy := func(k ai.PlanActionKind) ai.PlanActionKind {
		if k == ai.ActBuySeasonPass {
			return ai.ActBuyDayTicket
		}
		return k
	}
	return buy(x.Kind) == buy(y.Kind) && x.LiftID == y.LiftID && x.BldgID == y.BldgID &&
		x.TrailID == y.TrailID && x.Via == y.Via && x.Use == y.Use
}

// adopt moves the follower a, who has done done steps of their copy of
// the leader's plan a.Party.Gen, on to the newest plan the leader went on
// to from it: when the leader switched plans at or before where a is,
// and a's steps since are the new plan's first. Reports whether it did.
func (s *Simulation) adopt(a *world.Guest, done int) bool {
	l := a.Party.Leader
	if l == nil || a.Party.Gen == 0 {
		return false
	}
	steps, at := a.Plan.Steps, done
	var got *world.SharedPlan
	for gen, hops := a.Party.Gen, 0; hops < world.SharedPlans; hops++ {
		next := successor(l, gen)
		if next == nil || next.Done > at || next.Done > len(steps) {
			break
		}
		m := at - next.Done
		if m > len(next.Plan.Steps) {
			break
		}
		for i := 0; i < m; i++ {
			if !sameStep(steps[next.Done+i], next.Plan.Steps[i]) {
				return false // a went another way since: they plan for themselves
			}
		}
		got, gen, steps, at = next, next.Gen, next.Plan.Steps, m
	}
	if got == nil {
		return false
	}
	p, ok := s.copyPlan(a, got.Plan)
	if !ok || !canStart(a, p, at) {
		return false
	}
	s.takePlan(a, p, got.Gen, at)
	return true
}

// boardedDone is how many of plan p's steps are done for a guest who's
// just boarded lift: the steps before its ride (a guest in a lift with
// several lanes boards before their line step is seen to finish).
func boardedDone(p *ai.Plan, lift uint64) int {
	for i := p.Step; i < len(p.Steps); i++ {
		switch st := p.Steps[i]; {
		case st.Kind == ai.ActRideLift && st.LiftID == lift:
			return i
		case st.Kind == ai.ActJoinQueue && st.LiftID == lift:
			return i + 1
		}
	}
	return min(p.Step, len(p.Steps))
}

// canStart reports whether the guest a can take up plan p at step: a
// lift ride only from its line or its chair.
func canStart(a *world.Guest, p ai.Plan, step int) bool {
	if step >= len(p.Steps) {
		return true
	}
	if st := p.Steps[step]; st.Kind == ai.ActRideLift {
		return a.OnLiftID == st.LiftID || a.Queued && queuedLift(a) == st.LiftID
	}
	return true
}

// takePlan puts the follower a on p, the leader's plan gen, at step:
// starting it there unless they're on it already (a lift ride).
func (s *Simulation) takePlan(a *world.Guest, p ai.Plan, gen uint32, step int) {
	a.Plan = p
	a.Plan.Step = step
	a.Party.Gen, a.Party.PlanAt, a.Party.WaitSince = gen, s.SimTime, 0
	s.groupStats.Copied++
	if p.GoalName == (goap.GoHome{}).Name() {
		why := s.departReasonFor(a)
		if l := a.Party.Leader; why == ai.DepartDone && l != nil && l.DepartReason != ai.DepartNone {
			why = l.DepartReason // went home with the group
		}
		s.setDepartReason(a, why)
	}
	if a.Plan.Done() || a.Plan.Head().Kind == ai.ActRideLift && a.OnLiftID != 0 {
		return
	}
	s.onPlanStepStart(a)
}

// sharedPlanAge is how old, in sim seconds, a leader's plan can be for a
// follower who's lost the group to pick it up where it was made.
const sharedPlanAge = 900.0

// sharedFrom is the leader's newest plan made from from since the
// follower a's own plan was, nil for none.
func (s *Simulation) sharedFrom(a *world.Guest, from world.PlanFrom) *world.SharedPlan {
	l := a.Party.Leader
	if l == nil || from == (world.PlanFrom{}) {
		return nil
	}
	var best *world.SharedPlan
	for i := range l.Party.Shared {
		sp := &l.Party.Shared[i]
		if sp.Gen == 0 || sp.From != from || sp.At < a.Party.PlanAt || s.SimTime-sp.At > sharedPlanAge {
			continue
		}
		if best == nil || sp.At > best.At {
			best = sp
		}
	}
	return best
}

// copyPlan is the leader's plan p as the follower a will ski it, step for
// step: a season pass bought as a day ticket (one with a ticket already
// pays nothing). No fit when a needs a ticket or skis the plan doesn't
// get them before a lift, or when it leaves from a lot their car isn't
// in. (Rentals are skipped by those who don't need them: onPlanStepStart.)
func (s *Simulation) copyPlan(a *world.Guest, p ai.Plan) (ai.Plan, bool) {
	ticketed, rented := a.HasSeasonPass || a.HasDayTicket, !a.NeedsGear
	steps := make([]ai.PlanAction, len(p.Steps))
	for i, st := range p.Steps {
		switch st.Kind {
		case ai.ActBuySeasonPass, ai.ActBuyDayTicket:
			st.Kind, ticketed = ai.ActBuyDayTicket, true
		case ai.ActUseService:
			rented = rented || st.Use == ai.OfferRentals
		case ai.ActJoinQueue, ai.ActRideLift:
			if !ticketed || !rented {
				return ai.Plan{}, false
			}
		case ai.ActSkiToParking, ai.ActWalkToParking, ai.ActDepart:
			if a.CarLot != 0 && st.BldgID != a.CarLot {
				return ai.Plan{}, false
			}
		case ai.ActSkiTrail:
			if b := findBuildingByID(s.World, st.BldgID); b != nil && b.Type == world.BuildingParking && a.CarLot != 0 && st.BldgID != a.CarLot {
				return ai.Plan{}, false
			}
		}
		steps[i] = st
	}
	p.Steps, p.Step, p.Blocked = steps, 0, nil
	return p, true
}

// stepDone reports whether a plan step that puts the guest somewhere is
// already done for a (they're there): a follower picking up the leader's
// plan where it was made skips them.
func stepDone(st ai.PlanAction, a *world.Guest, snap goap.WorldSnapshot) bool {
	switch st.Kind {
	case ai.ActBuySeasonPass, ai.ActBuyDayTicket, ai.ActUseService, ai.ActDepart:
		return false
	}
	return planActionComplete(st, a, snap)
}

// pickUp puts the follower a, lost from the group, on the leader's plan
// made from from (where a is now), past the steps they've done. Reports
// whether there was one that fits.
func (s *Simulation) pickUp(a *world.Guest, from world.PlanFrom) bool {
	sp := s.sharedFrom(a, from)
	if sp == nil {
		return false
	}
	p, ok := s.copyPlan(a, sp.Plan)
	if !ok {
		return false
	}
	snap := goap.Extract(a, s.World)
	step := 0
	for step < len(p.Steps) && stepDone(p.Steps[step], a, snap) {
		step++
	}
	if step == len(p.Steps) || !canStart(a, p, step) {
		return false
	}
	s.takePlan(a, p, sp.Gen, step)
	return true
}

// followStep is the follower a's step boundary (advancePlan): with the
// head just done, wait at the top of a lift for a leader still on it, or
// move on to the leader's next plan. Reports whether it handled the
// boundary.
func (s *Simulation) followStep(a *world.Guest) bool {
	l := a.Party.Leader
	if l == nil || l.Removed || l.State != world.OnMountain || s.World.ClosedForDay {
		return false
	}
	a.Party.WaitSince = 0
	if a.Party.Gen == 0 {
		return s.joinLeader(a) // lost: fall in with the leader if they're near
	}
	return s.adopt(a, a.Plan.Step+1)
}

// followPlan gives the follower a, about to plan (their plan done, or
// its step no longer possible), the leader's plan instead, or has them
// wait for it; false when they should plan for themselves (no leader,
// closing time, or nothing to take or wait for).
func (s *Simulation) followPlan(a *world.Guest) bool {
	l := a.Party.Leader
	if l == nil || l.Removed || l.State != world.OnMountain || s.World.ClosedForDay {
		return false
	}
	done := min(a.Plan.Step, len(a.Plan.Steps))
	if s.adopt(a, done) {
		return true
	}
	// The leader is still on the plan a's finished: wait for them.
	if a.Party.Gen != 0 && l.Party.Gen == a.Party.Gen && s.waiting(a) {
		return true
	}
	from := s.planFrom(a)
	if from.LiftTop != 0 && s.leaderOnLift(l, from.LiftTop) && s.waiting(a) {
		return true // lost, but the leader's on this lift: their plan's coming
	}
	if s.pickUp(a, from) || s.joinLeader(a) {
		return true
	}
	switch {
	case a.Party.WaitSince != 0:
		s.groupStats.OwnWhy[ownWaited]++
	case a.Party.Gen == 0:
		s.groupStats.OwnWhy[ownLost]++
	default:
		s.groupStats.OwnWhy[ownBroke]++
	}
	a.Party.WaitSince = 0
	return false
}

// joinNear is how close, in metres, a lost follower has to be to their
// leader to fall in with them.
const joinNear = 40

// joinLeader puts the follower a, near their leader on the way
// somewhere, on the leader's plan at the leader's step. Reports whether
// they fell in.
func (s *Simulation) joinLeader(a *world.Guest) bool {
	l := a.Party.Leader
	if l.Party.Gen == 0 || l.Plan.Done() || l.Pos.Sub(a.Pos).Len() > joinNear {
		return false
	}
	switch l.Plan.Head().Kind {
	case ai.ActSkiToLift, ai.ActSkiToService, ai.ActSkiToParking, ai.ActSkiTrail,
		ai.ActWalkToLift, ai.ActWalkToService, ai.ActWalkToParking:
	default:
		return false
	}
	p, ok := s.copyPlan(a, l.Plan)
	if !ok || !canStart(a, p, l.Plan.Step) {
		return false
	}
	s.takePlan(a, p, l.Party.Gen, l.Plan.Step)
	return true
}

// followWaitSec is how long, in sim seconds, a follower waits for the
// leader before going on alone: long enough for the leader to finish
// lunch.
const followWaitSec = 600.0

// waiting has the follower a stand and wait, and reports whether they
// still will: false once they've waited followWaitSec.
func (s *Simulation) waiting(a *world.Guest) bool {
	if a.Party.WaitSince == 0 {
		a.Party.WaitSince = s.SimTime
	}
	if s.SimTime-a.Party.WaitSince > followWaitSec {
		return false
	}
	a.TargetID, a.Path, a.PathIdx, a.Speed = 0, nil, 0, 0
	a.Plan.Target = mgl32.Vec3{}
	s.groupStats.Waits++
	return true
}

// leaderOnLift reports whether the leader l is in lift's line, on it, or
// getting off it.
func (s *Simulation) leaderOnLift(l *world.Guest, lift uint64) bool {
	return l.OnLiftID == lift || l.Unload.LiftID == lift || l.Queued && queuedLift(l) == lift
}

// queuedLift is the lift whose line a is in, 0 for none.
func queuedLift(a *world.Guest) uint64 {
	head := a.Plan.Head()
	if head.Kind == ai.ActRideLift || head.Kind == ai.ActJoinQueue {
		return head.LiftID
	}
	return 0
}

// boardFollower settles the follower a, just on a chair of lift, on the
// leader's plan: the one the leader rode it on, if they boarded first,
// else the one a's on, to pick up the leader's at the top. False when
// a's lost the group and the leader isn't on this lift: they plan for
// themselves.
func (s *Simulation) boardFollower(a *world.Guest, lift *world.Lift) bool {
	l := a.Party.Leader
	if l == nil || l.Removed || l.State != world.OnMountain {
		return false
	}
	// On the leader's plans: carry on, picking up the leader's next
	// where it fits (at the top, if not now).
	if s.adopt(a, boardedDone(&a.Plan, lift.ID)) || a.Party.Gen != 0 {
		return true
	}
	if sp := s.sharedFrom(a, world.PlanFrom{LiftTop: lift.ID}); sp != nil {
		if p, ok := s.copyPlan(a, sp.Plan); ok && len(p.Steps) > 0 && p.Steps[0].Kind == ai.ActRideLift && p.Steps[0].LiftID == lift.ID {
			s.takePlan(a, p, sp.Gen, 0)
			return true
		}
	}
	if !s.leaderOnLift(l, lift.ID) {
		return false
	}
	// Lost, but the leader's on this lift too: end the plan at the ride,
	// so the top is where a picks up the leader's.
	for i := a.Plan.Step; i < len(a.Plan.Steps); i++ {
		if a.Plan.Steps[i].Kind == ai.ActRideLift && a.Plan.Steps[i].LiftID == lift.ID {
			a.Plan.Steps = a.Plan.Steps[: i+1 : i+1]
			return true
		}
	}
	a.Plan = ai.Plan{Steps: []ai.PlanAction{{Kind: ai.ActRideLift, LiftID: lift.ID}}}
	return true
}

// leaderWaitSec is how long, in sim seconds, a leader waits at the top
// of a lift for followers still riding it.
const leaderWaitSec = 60.0

// holdForGroup reports whether a should stand at the top of the lift
// they've just ridden: a follower whose leader is still in its line or on
// it, or a leader with followers still on it.
func (s *Simulation) holdForGroup(a *world.Guest) bool {
	if l := a.Party.Leader; l != nil {
		if a.Party.TopLift == 0 {
			return false
		}
		if s.SimTime-a.Party.TopSince < followWaitSec && !l.Removed &&
			(s.leaderOnLift(l, a.Party.TopLift) || l.Run.LiftID == a.Party.TopLift && s.holdForGroup(l)) {
			return true
		}
		a.Party.TopLift = 0
		return false
	}
	if !leads(a) || a.Run.LiftID == 0 || a.Run.Distance > 1 || s.SimTime-a.Run.Start > leaderWaitSec {
		return false
	}
	for _, f := range a.Party.Followers {
		if f.OnLiftID == a.Run.LiftID || f.Unload.LiftID == a.Run.LiftID {
			return true
		}
	}
	return false
}

// crumbSpacing is how far apart, in metres, a leader drops crumbs, and
// crumbBreakSec how long without one (riding a lift, eating) ends a line.
const (
	crumbSpacing  = float32(3)
	crumbBreakSec = 20.0
)

// descentEnd is where the descent at plan p's step goes: the lift or
// building its trail steps end at; 0 when the step isn't a descent.
func descentEnd(p *ai.Plan) uint64 {
	for i := p.Step; i < len(p.Steps); i++ {
		st := p.Steps[i]
		if !isDescentKind(st.Kind) {
			return 0
		}
		if st.LiftID != 0 {
			return st.LiftID
		}
		if st.BldgID != 0 {
			return st.BldgID
		}
	}
	return 0
}

// startCrumbs starts a new line for the leader a setting off on a
// descent, unless it carries on the one they were skiing.
func (s *Simulation) startCrumbs(a *world.Guest) {
	c := a.Party.Crumbs
	if c == nil {
		c = new(world.Crumbs)
		a.Party.Crumbs = c
	}
	end := descentEnd(&a.Plan)
	if c.N > 0 && s.SimTime-c.Last < crumbBreakSec {
		// Still skiing: the line goes on, somewhere new.
		if end != c.End {
			c.PrevEnd, c.End = c.End, end
		}
		return
	}
	c.Seq++
	c.N, c.End, c.PrevEnd, c.Last = 0, end, end, s.SimTime
}

// dropCrumb adds the leader a's position to their line when they've
// come crumbSpacing from the last crumb.
func (s *Simulation) dropCrumb(a *world.Guest) {
	c := a.Party.Crumbs
	if c == nil {
		s.startCrumbs(a)
		c = a.Party.Crumbs
	}
	p := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	var at float32
	if c.N > 0 {
		last := c.At(c.N - 1)
		d := p.Sub(last.P).Len()
		if d < crumbSpacing {
			c.Last = s.SimTime
			return
		}
		at = last.S + d
	}
	c.Pts[c.N%world.CrumbRing] = world.Crumb{P: p, S: at}
	c.N++
	c.Last = s.SimTime
}

// follows is the leader whose line the follower a can ski on their
// current step: on a descent to where the leader's line goes. Nil, with
// why (offLine), when they can't.
func (s *Simulation) follows(a *world.Guest) (*world.Guest, int8) {
	l := a.Party.Leader
	if l == nil || l.Removed {
		return nil, offNoLine
	}
	c := l.Party.Crumbs
	switch {
	case c == nil || c.N < 2:
		return nil, offNoLine
	case s.SimTime-a.Run.Start > followRunSec:
		return nil, offLong // something's wrong: ski the rest alone
	case !isDescentKind(a.Plan.Head().Kind):
		return nil, offNotDown
	case descentEnd(&a.Plan) != c.End && descentEnd(&a.Plan) != c.PrevEnd:
		return nil, offOtherEnd
	}
	return l, onLine
}

// Following: a follower stays within followLost metres of the line or
// skis alone; steers for a point followLook seconds ahead on it (at
// least followLookMin metres); keeps followGap metres per pair of
// followers ahead of them behind the leader; and finishes alone within
// followFinish metres of where they're going.
const (
	followLost    = float32(50)
	followLook    = float32(1.0)
	followLookMin = float32(8)
	followGap     = float32(6)
	followFinish  = float32(20)
	// followRunSec is how long, in sim seconds, a follower keeps to the
	// leader's line on one descent: a descent takes a few minutes.
	followRunSec = 600.0
	// followStuckSec is how long, in sim seconds, a follower skis one
	// descent before giving up the leader's plan for one of their own.
	followStuckSec = 900.0
	// followAheadPace is how fast a follower ahead of their leader
	// skis, against the leader's speed.
	followAheadPace = float32(0.7)
)

// followPace is how fast a follower skis: their usual speed times scale,
// and no faster than cap (m/s) when cap > 0.
type followPace struct{ scale, cap float32 }

// freePace is a skier's own pace.
var freePace = followPace{scale: 1}

// followTarget is where the follower a steers on their leader l's line
// this step, and how fast they ski against their usual speed to keep
// their place; with a reason (offLine) instead when they've lost the
// line, are ahead of the leader, or are close enough to goal to finish
// alone. The line runs up to where the leader is now. It moves only a's
// place on the line, so followers can work it out at once.
func followTarget(t *world.Terrain, a, l *world.Guest, goal mgl32.Vec3) (mgl32.Vec3, followPace, int) {
	c := l.Party.Crumbs
	p := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	if (mgl32.Vec2{goal[0], goal[2]}).Sub(p).Len() < followFinish {
		return mgl32.Vec3{}, freePace, offFinish
	}
	lo := c.Oldest()
	idx := a.Party.FollowIdx
	if a.Party.FollowSeq != c.Seq || idx < lo || idx >= c.N {
		// Find their place afresh: the nearest crumb held.
		idx, best := lo, float32(math.MaxFloat32)
		for i := lo; i < c.N; i++ {
			if d := c.At(i).P.Sub(p).LenSqr(); d < best {
				idx, best = i, d
			}
		}
		a.Party.FollowSeq, a.Party.FollowIdx = c.Seq, idx
	}
	// Move up the line while the next crumb is nearer.
	for idx+1 < c.N && c.At(idx+1).P.Sub(p).LenSqr() <= c.At(idx).P.Sub(p).LenSqr() {
		idx++
	}
	a.Party.FollowIdx = idx
	here := c.At(idx)
	// The leader, at the end of the line.
	last := c.At(c.N - 1)
	lp := mgl32.Vec2{l.Pos[0], l.Pos[2]}
	head := world.Crumb{P: lp, S: last.S + lp.Sub(last.P).Len()}
	hx, hz := float32(math.Sin(float64(l.Heading))), float32(math.Cos(float64(l.Heading)))
	if aheadOf(a, l) || idx == c.N-1 && here.P.Sub(p).Len() > followLost {
		// Past the leader, or beyond the end of their line: ease off
		// and let them lead.
		return mgl32.Vec3{}, followPace{1, l.Speed * followAheadPace}, offHead
	}
	if here.P.Sub(p).Len() > followLost {
		// Off the line: head for the leader, if they're in sight
		// across open snow.
		d := lp.Sub(p).Len()
		if d > followSight || !openBetween(t, p, lp) {
			return mgl32.Vec3{}, freePace, offLost
		}
		pace := freePace
		if want := followGap * float32((a.Party.Rank+1)/2); d < want {
			pace.cap = l.Speed
		}
		return mgl32.Vec3{l.Pos[0], l.Pos[1], l.Pos[2]}, pace, onLine
	}
	// Keep their place: no faster than the leader close behind, push on
	// when dropped.
	gap := head.S - here.S
	want := followGap * float32((a.Party.Rank+1)/2)
	pace := freePace
	switch {
	case gap < want*0.5:
		pace.cap = l.Speed * 0.8
	case gap < want:
		pace.cap = l.Speed
	case gap > want+30:
		pace.scale = 1.25
	}
	// The point ahead (the leader, at the most), off to their side unless
	// it's in the trees. Ahead in space too: where the leader's line
	// came back past them, the line further on is no way forward.
	look := max(followLookMin, a.Speed*followLook)
	at, prev := head, last
	for j := idx + 1; j < c.N; j++ {
		if c.At(j).S >= here.S+look && c.At(j).P.Sub(p).Len() >= look/2 {
			at, prev = c.At(j), c.At(j-1)
			break
		}
	}
	if at.P.Sub(p).Len() < look/2 {
		// The leader's beside them: let them lead.
		return mgl32.Vec3{}, followPace{1, l.Speed * followAheadPace}, offHead
	}
	dir := at.P.Sub(prev.P)
	if n := dir.Len(); n > 1e-3 {
		dir = dir.Mul(1 / n)
	} else {
		dir = mgl32.Vec2{hx, hz}
	}
	side := a.Party.FollowSide
	if t.TreeCoverAt(at.P[0], at.P[1]) > inTreesThreshold {
		side = 0 // single file through the trees, on the leader's line
	}
	x, z := at.P[0]+dir[1]*side, at.P[1]-dir[0]*side
	return mgl32.Vec3{x, t.InterpolatedSurfaceElevationAt(x, z), z}, pace, onLine
}

// followSight is how far, in metres, a follower off the leader's line
// will head straight for the leader.
const followSight = 150

// openBetween reports whether the snow from a to b is clear of trees,
// checked every openStep metres.
func openBetween(t *world.Terrain, a, b mgl32.Vec2) bool {
	d := b.Sub(a)
	n := int(d.Len()/openStep) + 1
	for i := 1; i <= n; i++ {
		q := a.Add(d.Mul(float32(i) / float32(n)))
		if t.TreeCoverAt(q[0], q[1]) > inTreesThreshold {
			return false
		}
	}
	return true
}

// openStep is how far apart, in metres, openBetween looks for trees.
const openStep = 10

// aheadOf reports whether the follower a is ahead of their leader l on
// the descent they both set off on at about the same time.
func aheadOf(a, l *world.Guest) bool {
	return math.Abs(a.Run.Start-l.Run.Start) < sameStartSec && a.Run.Distance > l.Run.Distance+aheadMargin
}

// Followers who set off within sameStartSec of their leader are on the
// same descent; one aheadMargin metres further along it is ahead.
const (
	sameStartSec = 90.0
	aheadMargin  = float32(3)
)

// onLine is followTarget's "on the line" (not an offLine reason).
const onLine = -1

// LiveTargetForDebug is where a is heading (liveTarget), for tools.
func LiveTargetForDebug(w *world.World, a *world.Guest) (mgl32.Vec3, bool) { return liveTarget(w, a) }

// FollowForDebug is what the follower a would steer at on their leader's
// line now (followTarget), and why not, for tools.
func FollowForDebug(s *Simulation, a *world.Guest) (mgl32.Vec3, int, int8) {
	l, why := s.follows(a)
	if l == nil {
		return mgl32.Vec3{}, -2, why
	}
	goal, _ := liveTarget(s.World, a)
	t, _, off := followTarget(s.World.Terrain, a, l, goal)
	return t, off, why
}
