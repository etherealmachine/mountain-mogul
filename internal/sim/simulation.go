package sim

import (
	"math"
	"math/rand"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/ai/goap"
	"mountain-mogul/internal/world"
)

const (
	WalkSpeed = 2.0 // m/s
	CellSize  = 5.0 // metres per grid cell

	// patienceDrainPerSecQueuing is patience drained per sim-second
	// while standing in a lift queue. 600 sim-seconds (10 min) of pure
	// queuing exhausts a full patience budget.
	patienceDrainPerSecQueuing = 1.0 / 600.0

	// patienceGainPerSecRiding is patience restored per sim-second
	// while riding a lift chair.
	patienceGainPerSecRiding = 1.0 / 800.0

	// longQueuePersons is the queue depth at which a guest considers
	// the line "long." At ~8 s/person this is ~120 s of expected wait.
	longQueuePersons = 15
)

// Simulation drives all agent and building behaviour.
type Simulation struct {
	World      *world.World
	Pathfinder *Pathfinder
	TimeScale  float64    // simulation speed multiplier (default 4 — ~1 hr per ski season)
	SimTime    float64    // accumulated sim seconds (post-TimeScale)
	Rng        *rand.Rand // single source for all gameplay randomness; testbeds seed this for determinism

	// lastSampledDay is the most recent in-game day index whose end has
	// been written to World.History. Initialised to int(SimTime /
	// secondsPerSimDay) so a sim loaded mid-day starts recording from
	// the next rollover rather than back-filling the partial day.
	lastSampledDay int

	// Planner is the L0 GOAP planner — picks goals and chains actions
	// for every agent in the world.
	Planner *goap.Planner

	// Demand owns the global skier pool and resort rating. The
	// per-30-sim-seconds poll fires from Tick.
	Demand *DemandSystem

	// Recorder, if non-nil, receives one RecorderFrame per skiing tick.
	// Used by the debug CSV log; default nil.
	Recorder Recorder

	// towersScratch is a reusable slice holding every lift tower's XZ
	// position. Rebuilt at the top of Tick (cheap — len(lifts) work)
	// and shared by every L1 sampleTactical call during the frame so
	// the hot inner loop doesn't allocate. The per-Lift TowerXZs()
	// cache means each refill is also allocation-free in steady state.
	towersScratch []mgl32.Vec2

	// spatial is the per-Tick agent grid the L1 hazard sampler reads.
	// Without it, hazardDensityAt iterates every other agent per
	// query and sampleTactical becomes O(N²) in agent count — at 50×
	// with the demand system pushing N past ~150 that wedged the main
	// thread for seconds at a time. The grid is rebuilt once per Tick
	// alongside towersScratch.
	spatial *spatialGrid
}

// NewSimulation creates a Simulation wrapping the given world, seeded from
// the wall clock. Use NewSimulationWithSeed for reproducible runs.
func NewSimulation(w *world.World) *Simulation {
	return NewSimulationWithSeed(w, time.Now().UnixNano())
}

// NewSimulationWithSeed creates a Simulation with a fixed RNG seed. Identical
// seed + identical world produces identical agent trajectories — the property
// testbeds rely on.
//
// Any agents already present in w (testbed seeds, save-restored agents) get
// onPlanStepStart called for their head action so TargetID, the L1 plan
// target, and any pathfinder route are materialised before the first
// tick. Agents with no plan are left alone — tickPlanning's plan-empty
// branch picks them up.
func NewSimulationWithSeed(w *world.World, seed int64) *Simulation {
	widthM := float32(w.Terrain.Width) * CellSize
	heightM := float32(w.Terrain.Height) * CellSize
	sim := &Simulation{
		World:          w,
		Pathfinder:     NewPathfinder(w.Terrain),
		TimeScale:      4.0,
		Rng:            rand.New(rand.NewSource(seed)),
		Planner:        goap.NewPlanner(),
		Demand:         NewDemandSystem(),
		spatial:        newSpatialGrid(widthM, heightM),
		lastSampledDay: 0,
	}
	for _, a := range w.OnMountain {
		if !a.Plan.Done() {
			sim.onPlanStepStart(a)
		}
	}
	return sim
}

// maxSubstepSec caps the sim-time delta passed to any per-tick handler.
// At 1× the render frame's dt (~16 ms) is already well below this so
// substepping is a no-op; at 100× we run ~5 substeps per frame, which
// keeps the L1 controller's arrival check, heading-rate cap, and
// Euler physics integration on a small enough step to stay accurate.
const maxSubstepSec = 1.0 / 30.0

// maxWallDtSec caps the wall-clock dt the simulation will catch up on
// in a single Tick. Without this, a load-screen hitch (scene Init does
// BuildTerrainMesh / RebuildStaticBatch synchronously, then the next
// frame's dt captures all that time) or an OS sleep / debugger pause
// telescopes into many sim-seconds of one-shot advance — every chair on
// every lift can then cross the unload threshold inside the same Tick
// and a whole lift dumps its passengers at once. Clamping here keeps
// chair unloads sequential regardless of TimeScale. 0.1 s × 50× = 5
// sim-seconds, still well under the ~30-s chair-loop period.
const maxWallDtSec = 0.1

// Tick advances the simulation by dt real seconds. dt is scaled by
// TimeScale then sliced into substeps of at most maxSubstepSec so the
// continuous controllers never see a huge dt — necessary above ~10×
// because position/heading integrators in L1 assume a small step.
//
// dt is first clamped to maxWallDtSec so any wall-clock hitch (load
// screen, debugger pause, dropped frame) loses the excess time instead
// of catching up in one telescoped Tick.
func (s *Simulation) Tick(dt float64) {
	if dt > maxWallDtSec {
		dt = maxWallDtSec
	}
	// Refill the shared tower-position scratch once per Tick. Lifts
	// don't move during a frame, so the L1 sampler in every substep /
	// every agent can read the same slice.
	s.refillTowersScratch()
	// Rebucket every agent into the spatial grid for the L1 hazard
	// sampler. Cheap — O(agents) — and turns hazardDensityAt's agent
	// iteration from O(N) into O(near).
	s.spatial.rebuild(s.World.OnMountain)

	remaining := dt * s.TimeScale
	for remaining > 0 {
		sub := remaining
		if sub > maxSubstepSec {
			sub = maxSubstepSec
		}
		s.subTick(sub)
		remaining -= sub
	}
}

// refillTowersScratch rebuilds s.towersScratch in place from the live
// lift list. Each per-lift TowerXZs() is cached on the Lift so this
// is a flat copy in steady state — no allocations after the first
// call per Lift.
func (s *Simulation) refillTowersScratch() {
	s.towersScratch = s.towersScratch[:0]
	for _, lift := range s.World.Lifts {
		s.towersScratch = append(s.towersScratch, lift.TowerXZs()...)
	}
}

// subTick is one indivisible simulation step. All per-handler dt's
// come from here so substepping in Tick is the single place that
// controls step size.
func (s *Simulation) subTick(dt float64) {
	s.SimTime += dt
	s.Demand.maybePoll(s)
	s.Demand.maybePollRating(s)
	s.maybeSampleHistory()
	s.tickLifts(dt)
	s.tickGuests(dt)
	s.tickSnowcats(dt)
}

func (s *Simulation) tickLifts(dt float64) {
	w := s.World
	for _, lift := range w.Lifts {
		loopLen := lift.LoopLength()
		if loopLen < 1 {
			continue
		}
		fracPerSec := float64(lift.Speed) / float64(loopLen)
		moving := lift.Open || lift.PassengerCount() > 0
		for i := range lift.Chairs {
			chair := &lift.Chairs[i]
			prev := chair.Progress
			if moving {
				chair.Progress += float32(fracPerSec * dt)
			}

			// At top (progress crosses 0.5): unload passengers.
			if prev < 0.5 && chair.Progress >= 0.5 {
				for j := range chair.Passengers {
					agent := chair.Passengers[j]
					if agent == nil {
						continue
					}
					chair.Passengers[j] = nil
					agent.OnLiftID = 0
					agent.Speed = 0
					agent.TurnSide = 0
					if agent.Balance < 0.5 {
						agent.Balance = 1.0 // ride up restored balance
					}

					// Emit a positive thought on the first ride of this lift;
					// then update the ride tally so the planner's Explore
					// goal prefers unridden lifts.
					if ai.RideCountOf(agent.RidenLifts, lift.ID) == 0 {
						agent.AddThought(ai.ThoughtLovingALift, s.SimTime)
					}
					agent.RidenLifts = ai.AddRide(agent.RidenLifts, lift.ID)
					topCell := lift.TopCell()
					ty := w.Terrain.SurfaceElevationAt(topCell[0], topCell[1])
					agent.Pos = mgl32.Vec3{lift.Top[0], ty, lift.Top[1]}

					// Advance the plan past the just-completed RideLift so
					// onPlanStepStart sets TargetID for the next step
					// (typically SkiToLift/SkiToLodge/SkiToParking). Then
					// orient heading toward that target — while riding,
					// Heading points up the lift cable, which would make
					// the new tickSkier physics treat gravity as
					// decelerating and the skier would never start.
					s.advancePlan(agent)
					if pos, ok := planTargetWorldPos(w, agent); ok {
						dx := pos[0] - agent.Pos[0]
						dz := pos[2] - agent.Pos[2]
						agent.Heading = float32(math.Atan2(float64(dx), float64(dz)))
					}
				}
			}

			// At base (progress wraps past 1.0): fill the chair from the
			// queue up to its capacity. Each boarder pays TicketPrice
			// into the resort's bank.
			if chair.Progress >= 1.0 {
				chair.Progress -= 1.0
				for j := 0; j < len(chair.Passengers) && len(lift.Queue) > 0; j++ {
					agent := lift.Queue[0]
					lift.Queue = lift.Queue[1:]
					chair.Passengers[j] = agent
					agent.OnLiftID = lift.ID
					agent.Queued = false
					w.Cash += lift.TicketPrice
					w.History.RecordRevenue(lift.TicketPrice)
				}
			}
		}
	}
}


// parkingWorldPos returns the parking lot's anchor as a world-space Vec3,
// with Y from the terrain mesh under the lot's door cell. Centralised so
// the "ID → world pos" lookup pattern lives in one place.
func parkingWorldPos(w *world.World, b *world.Building) mgl32.Vec3 {
	cell := b.DoorCell()
	return mgl32.Vec3{b.Pos[0], w.Terrain.SurfaceElevationAt(cell[0], cell[1]), b.Pos[1]}
}

// liftBaseWorldPos returns the lift base anchor as a world-space Vec3.
func liftBaseWorldPos(w *world.World, l *world.Lift) mgl32.Vec3 {
	cell := l.QueueCell()
	return mgl32.Vec3{l.Base[0], w.Terrain.SurfaceElevationAt(cell[0], cell[1]), l.Base[1]}
}

// tickGuests dispatches each agent to the appropriate handler based on its
// implicit state. tickPlanning runs first per agent so any replan / step
// advance settles the implicit state (Queued / TargetID / RestTimer /
// Removed) before the switch picks a handler. Order of checks matters:
// fallen short-circuits everything, then on-lift, then queued, then
// resting, then path-walking, then goal locomotion. Removed agents are
// reaped from w.OnMountain after the loop so range iteration isn't shifted
// mid-pass.
func (s *Simulation) tickGuests(dt float64) {
	w := s.World
	for _, agent := range w.OnMountain {
		if agent.Removed {
			continue
		}
		s.tickPlanning(agent)
		if agent.Removed {
			continue
		}
		switch {
		case agent.Fallen:
			s.tickFallen(agent, dt)
		case agent.OnLiftID != 0:
			s.tickRiding(agent, dt)
		case agent.Queued:
			s.tickQueued(agent, dt)
		case agent.RestTimer > 0:
			s.tickResting(agent, dt)
		case len(agent.Path) > 0 && agent.PathIdx < len(agent.Path):
			s.tickPath(agent, dt)
		default:
			s.tickLocomote(agent, dt)
		}
	}
	s.reapDeparted()
}

// spawnGuest moves an existing dormant Guest onto the mountain: sets
// State=OnMountain, places them at the lot's anchor, appends to
// w.OnMountain, and lets the planner lay out their first plan. The
// Guest record itself is the same pointer that lives in w.Guests, so
// identity + career stats persist across the visit. Returns false
// (and rolls back state/slice) when no viable plan can be laid — the
// caller treats that as a failed spawn and skips the CurrentCars bump.
func (s *Simulation) spawnGuest(lot *world.Building, g *world.Guest) bool {
	w := s.World
	cell := lot.DoorCell()
	elev := w.Terrain.SurfaceElevationAt(cell[0], cell[1])
	g.State = world.OnMountain
	g.Pos = mgl32.Vec3{lot.Pos[0], elev, lot.Pos[1]}
	g.Heading = 0
	g.Speed = 0
	g.Balance = 1.0
	g.Patience = 1.0
	g.Removed = false
	w.OnMountain = append(w.OnMountain, g)
	s.replan(g)
	head := g.Plan.Head()
	bad := g.Plan.Done() ||
		(head.Kind == ai.ActWalkToLift && len(g.Path) == 0)
	if bad {
		// Roll back: pop from OnMountain, reset to AtHome.
		w.OnMountain = w.OnMountain[:len(w.OnMountain)-1]
		g.ResetForDeparture()
		return false
	}
	w.History.RecordArrival()
	return true
}

// maybeSampleHistory pushes one DailySample per in-game day boundary
// the sim has crossed since the last call. Snapshots GuestsOnMountain
// + Cash at the moment of rollover, and consumes the per-day arrival/
// departure counters that have been accumulating in World.History.
// No-op when World.History is nil (testbeds, pre-history saves).
func (s *Simulation) maybeSampleHistory() {
	if s.World == nil || s.World.History == nil {
		return
	}
	today := int(s.SimTime / secondsPerSimDay)
	for s.lastSampledDay < today {
		dayIdx := s.lastSampledDay
		w := s.World

		// Compute and debit operational costs for the day.
		costs := len(w.Lifts) * 2 * world.LiftAttendantDailyCost
		costs += len(w.Snowcats) * world.SnowcatDailyCost
		w.Cash -= costs

		w.History.Push(world.DailySample{
			Day:              DateAt(float64(dayIdx) * secondsPerSimDay),
			GuestsOnMountain: len(w.OnMountain),
			ArrivalsToday:    w.History.ArrivalsToday,
			DeparturesToday:  w.History.DeparturesToday,
			Cash:             w.Cash,
			Revenue:          w.History.RevenueToday,
			Costs:            costs,
		})
		s.lastSampledDay++
	}
}

// =============================================================================
// Planning layer — drives target / queue / removal off the stored ai.Plan
// =============================================================================

// restAtLodgeSec mirrors goap.restDurationSec — the planner costs
// RestAtLodge as ~60 s and tickResting counts down for the same
// duration so plan cost and runtime stay in sync.
const restAtLodgeSec = 60.0

// tickPlanning is the per-agent replan / advance check. Runs first in
// the per-agent loop so any implicit state it sets (Queued, TargetID,
// RestTimer, Removed) is visible to the dispatch switch below it.
//
// Replan triggers (a deliberate subset of the SKIER_AI.md design):
//
//   - plan empty / done
//   - head action complete (snapshot matches the action's post-state)
//   - head action precondition broken (entity gone, etc)
//
// A periodic safety re-check is intentionally NOT implemented — it
// caused mid-descent goal re-elections (KeepSkiing flipping back on
// once AtLiftTop dropped) that made skiers loop indefinitely.
// Future replans for genuine world changes (lift closure, queue
// spikes) should be explicit event hooks, not a fixed-interval poll.
//
// Order matters: the completion check has to run before the precondition
// check because most actions' preconditions go false at the moment they
// complete (e.g. JoinQueue's precondition AtLiftBase==L is broken as
// soon as Queued==L is set).
func (s *Simulation) tickPlanning(a *world.Guest) {
	if a.Plan.Done() {
		s.replan(a)
		return
	}
	snap := goap.Extract(a, s.World)
	head := a.Plan.Head()
	if planActionComplete(head, a, snap) {
		if isDescentKind(head.Kind) {
			a.Events = append(a.Events, ai.GuestEvent{Kind: ai.EventRun, Time: s.SimTime})
		}
		// For trail-to-trail steps, mark the arrival at the junction so the
		// next step's precondition (AtTrailEnd == destTrailID) can fire.
		if head.Kind == ai.ActSkiTrail && head.LiftID == 0 && head.BldgID == 0 {
			a.AtTrailEnd = head.TrailID
		} else {
			a.AtTrailEnd = 0
		}
		s.advancePlan(a)
		return
	}
	if !planActionPreconditionHolds(head, snap, s.World) {
		s.replan(a)
		return
	}
}

// isDescentKind reports whether a step represents a completed ski
// descent. Used to log EventRun on completion so the demand system
// can score sessions by run count.
func isDescentKind(k ai.PlanActionKind) bool {
	switch k {
	case ai.ActSkiToLift, ai.ActSkiToLodge, ai.ActSkiToParking, ai.ActSkiTrail:
		return true
	}
	return false
}

// replan generates a fresh plan and starts its head step. Called at
// spawn, when the plan exhausts, and when a precondition breaks.
func (s *Simulation) replan(a *world.Guest) {
	a.AtTrailEnd = 0 // clear any stale junction anchor before re-planning
	a.OnTrailID = 0
	a.Plan = s.Planner.StoredPlanFor(a, s.World)
	if !a.Plan.Done() {
		s.onPlanStepStart(a)
	}
}

// advancePlan moves the cursor to the next step and starts it; if the
// cursor walks off the end, the plan is done and we re-plan instead.
func (s *Simulation) advancePlan(a *world.Guest) {
	a.Plan.Step++
	if a.Plan.Done() {
		s.replan(a)
		return
	}
	s.onPlanStepStart(a)
}

// onPlanStepStart materialises the head action's effect on the live
// agent + world. Locomotion steps (Walk/Ski) set TargetID + lay
// pathfinder routes; transition steps (JoinQueue / RestAtLodge /
// Depart) flip the implicit-state bits the per-tick dispatcher reads.
// RideLift is a no-op — boarding belongs to tickLifts.
func (s *Simulation) onPlanStepStart(a *world.Guest) {
	w := s.World
	step := a.Plan.Head()
	// Clear trail context at every step start; ActSkiTrail sets it below.
	a.OnTrailID = 0

	switch step.Kind {
	case ai.ActWalkToLift:
		lift := findLiftByID(w, step.LiftID)
		if lift == nil {
			return
		}
		a.TargetID = lift.ID
		a.Plan.Goal = ai.GoalLift
		a.Plan.GoalID = lift.ID
		a.Plan.Target = lift.BackOfQueueWorldPos(w.Terrain)
		startCell := [2]int{
			int(math.Floor(float64(a.Pos[0] / CellSize))),
			int(math.Floor(float64(a.Pos[2] / CellSize))),
		}
		if path := s.Pathfinder.FindPath(startCell, lift.QueueCell()); path != nil {
			a.Path = path
			a.PathIdx = 0
		} else {
			a.Path = nil
			a.PathIdx = 0
		}
	case ai.ActJoinQueue:
		lift := findLiftByID(w, step.LiftID)
		if lift == nil {
			return
		}
		if len(lift.Queue) >= longQueuePersons {
			a.AddThought(ai.ThoughtLongLine, s.SimTime)
		}
		lift.Queue = append(lift.Queue, a)
		a.Queued = true
		a.TargetID = 0
	case ai.ActRideLift:
		// Boarding handled by tickLifts' chair-load branch.
	case ai.ActSkiToLift:
		lift := findLiftByID(w, step.LiftID)
		if lift == nil {
			return
		}
		a.TargetID = lift.ID
		a.Plan.Goal = ai.GoalLift
		a.Plan.GoalID = lift.ID
		a.Plan.Target = lift.BackOfQueueWorldPos(w.Terrain)
	case ai.ActSkiToLodge:
		b := findBuildingByID(w, step.BldgID)
		if b == nil {
			return
		}
		a.TargetID = b.ID
		a.Plan.Goal = ai.GoalNone
		a.Plan.GoalID = b.ID
		a.Plan.Target = parkingWorldPos(w, b)
	case ai.ActSkiToParking:
		b := findBuildingByID(w, step.BldgID)
		if b == nil {
			return
		}
		a.TargetID = b.ID
		a.Plan.Goal = ai.GoalDepart
		a.Plan.GoalID = b.ID
		a.Plan.Target = parkingWorldPos(w, b)
	case ai.ActSkiTrail:
		a.OnTrailID = step.TrailID
		switch {
		case step.LiftID != 0:
			lift := findLiftByID(w, step.LiftID)
			if lift == nil {
				return
			}
			a.TargetID = lift.ID
			a.Plan.Goal = ai.GoalLift
			a.Plan.GoalID = lift.ID
			a.Plan.Target = lift.BackOfQueueWorldPos(w.Terrain)
		case step.BldgID != 0:
			b := findBuildingByID(w, step.BldgID)
			if b == nil {
				return
			}
			a.TargetID = b.ID
			if b.Type == world.BuildingParking {
				a.Plan.Goal = ai.GoalDepart
			} else {
				a.Plan.Goal = ai.GoalNone
			}
			a.Plan.GoalID = b.ID
			a.Plan.Target = parkingWorldPos(w, b)
		default:
			// Trail-to-trail: steer toward destination trail's centroid.
			if t := w.FindTrail(step.TrailID); t != nil {
				c := t.Centroid()
				y := w.Terrain.InterpolatedSurfaceElevationAt(c[0], c[1])
				a.Plan.Target = mgl32.Vec3{c[0], y, c[1]}
				a.Plan.Goal = ai.GoalNone
				a.Plan.GoalID = step.TrailID
				a.TargetID = 0
			}
		}
	case ai.ActRestAtLodge:
		a.RestTimer = restAtLodgeSec
		a.Speed = 0
		a.TargetID = 0
	case ai.ActDepart:
		// Capture session stats (LastScore, LifetimeVisits, LastVisit,
		// VisitsThisSeason) on the persistent Guest record before the
		// reaper clears sim scratch fields. Decrement the lot's visible
		// car count (4 departures = -1 car), then flip Removed so
		// reapDeparted will splice this Guest out of OnMountain.
		s.Demand.recordDeparture(a, s.SimTime)
		w.History.RecordDeparture()
		if b := findBuildingByID(w, step.BldgID); b != nil {
			b.CurrentCars -= 1.0 / float32(GuestsPerCar)
			if b.CurrentCars < 0 {
				b.CurrentCars = 0
			}
		}
		a.Removed = true
	}
}

// planActionComplete returns true when the head step's post-state is
// observable in snap. Drives advancePlan.
func planActionComplete(step ai.PlanAction, a *world.Guest, snap goap.WorldSnapshot) bool {
	switch step.Kind {
	case ai.ActWalkToLift, ai.ActSkiToLift:
		return snap.AtLiftBase == step.LiftID
	case ai.ActJoinQueue:
		return snap.Queued == step.LiftID
	case ai.ActRideLift:
		return snap.AtLiftTop == step.LiftID
	case ai.ActSkiToLodge:
		return snap.AtLodge == step.BldgID
	case ai.ActSkiToParking:
		return snap.AtParking == step.BldgID
	case ai.ActRestAtLodge:
		return a.RestTimer <= 0
	case ai.ActDepart:
		// Terminal — the Removed flag is the real signal; this is
		// queried only when the agent hasn't been reaped yet.
		return false
	case ai.ActSkiTrail:
		if step.LiftID != 0 {
			return snap.AtLiftBase == step.LiftID
		}
		if step.BldgID != 0 {
			return snap.AtLodge == step.BldgID || snap.AtParking == step.BldgID
		}
		// Trail-to-trail: proximity to Plan.Target (the destination trail centroid).
		dx := a.Pos[0] - a.Plan.Target[0]
		dz := a.Pos[2] - a.Plan.Target[2]
		return dx*dx+dz*dz < ArrivalThreshold*ArrivalThreshold
	}
	return false
}

// planActionPreconditionHolds returns true when the head step's
// precondition is still satisfied. Mainly catches "entity disappeared
// under the agent's feet" — most preconditions are trivially true while
// the agent is in transit (no anchor IDs set).
func planActionPreconditionHolds(step ai.PlanAction, snap goap.WorldSnapshot, w *world.World) bool {
	switch step.Kind {
	case ai.ActWalkToLift, ai.ActJoinQueue, ai.ActRideLift, ai.ActSkiToLift:
		return findLiftByID(w, step.LiftID) != nil
	case ai.ActSkiToLodge, ai.ActSkiToParking, ai.ActRestAtLodge, ai.ActDepart:
		return findBuildingByID(w, step.BldgID) != nil
	case ai.ActSkiTrail:
		// Destination entity must still exist.
		if step.LiftID != 0 {
			return findLiftByID(w, step.LiftID) != nil
		}
		if step.BldgID != 0 {
			return findBuildingByID(w, step.BldgID) != nil
		}
		return w.FindTrail(step.TrailID) != nil
	}
	return true
}

// tickResting counts down the atomic RestAtLodge timer. On expiry
// Patience resets to 1; tickPlanning on the next frame advances the plan.
func (s *Simulation) tickResting(a *world.Guest, dt float64) {
	if a.RestTimer <= 0 {
		return
	}
	a.RestTimer -= float32(dt)
	if a.RestTimer <= 0 {
		a.RestTimer = 0
		a.Patience = 1
	}
}

// reapDeparted removes agents flagged by ActDepart at the end of
// tickGuests. Defers removal out of the range loop so the slice header
// doesn't shift mid-iteration.
func (s *Simulation) reapDeparted() {
	w := s.World
	for i := len(w.OnMountain) - 1; i >= 0; i-- {
		if w.OnMountain[i].Removed {
			w.RemoveFromOnMountain(w.OnMountain[i].ID)
		}
	}
}

// planTargetWorldPos returns the world-space position the agent's head
// step is steering toward, if any. Lift / building targets resolve via
// the same routes resolveTarget uses, so heading orientation at lift
// unload matches what tickLocomote will steer at next tick.
func planTargetWorldPos(w *world.World, a *world.Guest) (mgl32.Vec3, bool) {
	step := a.Plan.Head()
	switch step.Kind {
	case ai.ActWalkToLift, ai.ActSkiToLift:
		if l := findLiftByID(w, step.LiftID); l != nil {
			return l.BackOfQueueWorldPos(w.Terrain), true
		}
	case ai.ActSkiToLodge, ai.ActSkiToParking:
		if b := findBuildingByID(w, step.BldgID); b != nil {
			return parkingWorldPos(w, b), true
		}
	case ai.ActSkiTrail:
		return a.Plan.Target, a.Plan.Target != (mgl32.Vec3{})
	}
	return mgl32.Vec3{}, false
}

func findLiftByID(w *world.World, id uint64) *world.Lift {
	for _, l := range w.Lifts {
		if l.ID == id {
			return l
		}
	}
	return nil
}

func findBuildingByID(w *world.World, id uint64) *world.Building {
	for _, b := range w.Buildings {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// recordWalkTick emits one RecorderFrame for non-skiing activities (walking
// a path or goal-based locomotion). Skiing rows come from recordFrame in
// skiing.go; this covers the gaps so the CSV is complete for all activities.
// target is the world-space point the agent is currently steering toward.
func (s *Simulation) recordWalkTick(a *world.Guest, target mgl32.Vec3) {
	if s.Recorder == nil {
		return
	}
	if id := s.Recorder.GuestID(); id != 0 && id != a.ID {
		return
	}
	dx := target[0] - a.Pos[0]
	dz := target[2] - a.Pos[2]
	dist := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	s.Recorder.Record(RecorderFrame{
		SimTime:  s.SimTime,
		GuestID:  a.ID,
		Activity: world.Activity(s.World, a),
		Pos:      a.Pos,
		Heading:  a.Heading,
		Target:   target,
		Dist:     dist,
		Speed:    a.Speed,
		PlanStep: goap.PlanActionLabel(a.Plan.Head(), s.World),
		GoalName: a.Plan.GoalName,
		PathLen:  len(a.Path),
		PathIdx:  a.PathIdx,
	})
}

// tickPath walks the agent along their pathfinder route at WalkSpeed. When
// the path ends and the target is a lift, they queue up; otherwise they fall
// through to goal-based locomotion next tick.
func (s *Simulation) tickPath(agent *world.Guest, dt float64) {
	w := s.World
	target := agent.Path[agent.PathIdx]
	tx := (float32(target[0]) + 0.5) * CellSize
	tz := (float32(target[1]) + 0.5) * CellSize
	ty := w.Terrain.SurfaceElevationAt(target[0], target[1])
	targetPos := mgl32.Vec3{tx, ty, tz}
	s.recordWalkTick(agent, targetPos)

	dir := targetPos.Sub(agent.Pos)
	dist := dir.Len()

	step := float32(WalkSpeed * dt)
	if dist <= step {
		agent.Pos = targetPos
		agent.PathIdx++
		if agent.PathIdx >= len(agent.Path) {
			agent.Path = nil
			agent.PathIdx = 0
			// Path arrival is detected by tickPlanning's snapshot check
			// next frame (snap.AtLiftBase == L within proximityRadius);
			// it advances the plan into JoinQueue. Nothing else to do
			// here.
		}
		return
	}
	dirNorm := dir.Normalize()
	agent.Pos = agent.Pos.Add(dirNorm.Mul(step))
	agent.Heading = float32(math.Atan2(float64(dirNorm[0]), float64(dirNorm[2])))
}

// tickQueued walks the agent toward their assigned single-file queue slot
// (index in lift.Queue) and orients them to face the boarding spot. Slot 0
// is at the base, so the front-of-line skier holds station there; deeper
// slots are further downhill of the base axis. As skiers board off the
// front and lift.Queue shrinks, each remaining agent's index drops by one
// and they shuffle forward on subsequent ticks. Patience drains while
// queuing — long waits make guests want to rest or leave.
func (s *Simulation) tickQueued(agent *world.Guest, dt float64) {
	agent.Patience -= float32(dt * patienceDrainPerSecQueuing)
	if agent.Patience < 0 {
		agent.Patience = 0
	}
	w := s.World
	for _, lift := range w.Lifts {
		idx := -1
		for i, q := range lift.Queue {
			if q == agent {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		slot := lift.QueueSlotWorldPos(idx, w.Terrain)
		s.tickWalkToward(agent, slot, dt)
		// Always face the lift base — tickWalkToward sets heading to the
		// walk direction, but skiers who've reached their slot need to
		// keep looking forward instead of holding whatever heading they
		// last walked in with.
		faceX := lift.Base[0] - agent.Pos[0]
		faceZ := lift.Base[1] - agent.Pos[2]
		if faceX*faceX+faceZ*faceZ > 0.01 {
			agent.Heading = float32(math.Atan2(float64(faceX), float64(faceZ)))
		}
		return
	}
}

// tickRiding glues the agent to its current chair's position. Seat
// anchors come from the lift type's chair mesh slot metadata, which
// scad2obj baked into chair.obj / chair_quad.obj from echo()
// declarations in the .scad source. The slot Pos is in the chair-local
// game frame; we rotate by heading and offset from the chair's cable
// anchor. Patience recovers while riding — a pleasant lift ride offsets
// earlier queue frustration.
func (s *Simulation) tickRiding(agent *world.Guest, dt float64) {
	agent.Patience += float32(dt * patienceGainPerSecRiding)
	if agent.Patience > 1 {
		agent.Patience = 1
	}
	w := s.World
	for _, lift := range w.Lifts {
		if lift.ID != agent.OnLiftID {
			continue
		}
		slots := world.SlotsFor(lift.Type.MeshID())
		for _, chair := range lift.Chairs {
			for slotIdx, p := range chair.Passengers {
				if p != agent {
					continue
				}
				pos, heading := lift.ChairPos(chair.Progress, w.Terrain)
				agent.Pos = seatWorldPos(pos, heading, slotIdx, slots)
				agent.Heading = heading
				return
			}
		}
	}
}

// seatWorldPos maps a passenger-slot index on a chair to its world-space
// position. The chair-local slot anchor is rotated by `heading` (same
// rotation the dynamic shader applies to the chair geometry) and offset
// from the chair's cable-attachment point `chairPos`. If no slot
// metadata is registered for the chair mesh, the rider sits on the
// cable anchor — visibly wrong, but a stable fallback that flags the
// missing data rather than crashing.
func seatWorldPos(chairPos mgl32.Vec3, heading float32, slotIdx int, slots []world.MeshSlot) mgl32.Vec3 {
	if slotIdx >= len(slots) {
		return chairPos
	}
	local := slots[slotIdx].Pos
	c := float32(math.Cos(float64(heading)))
	s := float32(math.Sin(float64(heading)))
	// Match the dynamic shader's heading rotation around game Y:
	//   (x, y, z) → (s·x − c·z, y, c·x + s·z)
	return mgl32.Vec3{
		chairPos[0] + s*local[0] - c*local[2],
		chairPos[1] + local[1],
		chairPos[2] + c*local[0] + s*local[2],
	}
}

// tickLocomote moves the agent toward TargetID, choosing ski or walk based
// on local slope and goal direction. Steep downhill toward the goal → ski;
// flat or uphill → walk straight. Arrival is observed by tickPlanning on
// the next frame via a snapshot extract — the implicit AtLiftBase /
// AtLodge / AtParking signal is what tells the planning layer the head
// step is done, so this function doesn't need an explicit on-arrival
// callback.
func (s *Simulation) tickLocomote(agent *world.Guest, dt float64) {
	w := s.World
	var targetPos mgl32.Vec3
	if agent.TargetID != 0 {
		var ok bool
		targetPos, ok = resolveTarget(w, agent.TargetID)
		if !ok {
			// Target vanished — drop it; tickPlanning's precondition check
			// will re-plan on the next tick.
			agent.TargetID = 0
			return
		}
	} else if agent.Plan.Target != (mgl32.Vec3{}) {
		// Trail-to-trail steps set TargetID=0 and steer via Plan.Target
		// (the destination trail centroid). Use it directly.
		targetPos = agent.Plan.Target
	} else {
		return
	}

	if shouldSki(w.Terrain, agent.Pos, targetPos) {
		s.tickSkier(agent, targetPos, dt)
		return
	}
	s.recordWalkTick(agent, targetPos)
	s.tickWalkToward(agent, targetPos, dt)
}

// shouldSki returns true when the goal lies in the downhill direction from
// the agent's position. Slope magnitude is irrelevant — the skiing physics
// already handles gentle and flat sections (low gravity accel, friction
// dominates) so a runout doesn't deserve a special case. Flat or uphill
// goals fall back to walking.
func shouldSki(t *world.Terrain, pos, target mgl32.Vec3) bool {
	n := t.NormalAt(pos[0]/CellSize, pos[2]/CellSize)
	fall := mgl32.Vec2{n[0], n[2]}
	fl := fall.Len()
	if fl < 1e-4 {
		return false // truly flat — no fall line to follow
	}
	fallDir := fall.Mul(1.0 / fl)
	dx := target[0] - pos[0]
	dz := target[2] - pos[2]
	axisLen := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if axisLen < 1e-4 {
		return false
	}
	axis := mgl32.Vec2{dx / axisLen, dz / axisLen}
	return axis.Dot(fallDir) > 0
}

// tickWalkToward marches the agent straight at WalkSpeed toward target;
// returns true on arrival (within ArrivalThreshold).
func (s *Simulation) tickWalkToward(agent *world.Guest, target mgl32.Vec3, dt float64) bool {
	dir := target.Sub(agent.Pos)
	dx, dz := dir[0], dir[2]
	distXZ := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if distXZ < ArrivalThreshold {
		agent.Pos = target
		agent.Speed = 0
		return true
	}
	dirNorm := mgl32.Vec2{dx / distXZ, dz / distXZ}
	step := float32(WalkSpeed * dt)
	if step > distXZ {
		step = distXZ
	}
	agent.Pos[0] += dirNorm[0] * step
	agent.Pos[2] += dirNorm[1] * step
	agent.Pos[1] = s.World.Terrain.InterpolatedSurfaceElevationAt(agent.Pos[0], agent.Pos[2])
	agent.Heading = float32(math.Atan2(float64(dirNorm[0]), float64(dirNorm[1])))
	agent.Speed = WalkSpeed
	return false
}

// resolveTarget looks up an agent's TargetID against the world's lifts
// and buildings, returning the target's world-space position. ok=false
// when the ID matches nothing (e.g. the entity was removed mid-plan —
// tickPlanning's precondition check will re-plan on the next tick).
// Y is taken from the terrain mesh under the entity's cell.
func resolveTarget(w *world.World, id uint64) (mgl32.Vec3, bool) {
	for _, l := range w.Lifts {
		if l.ID == id {
			// Aim for the back of the queue, not the base anchor —
			// pulled each locomotion tick so the target shifts as the
			// queue grows (more skiers behind us) or shrinks (boarders
			// peel off the front).
			return l.BackOfQueueWorldPos(w.Terrain), true
		}
	}
	for _, b := range w.Buildings {
		if b.ID == id {
			return parkingWorldPos(w, b), true
		}
	}
	return mgl32.Vec3{}, false
}

// findBuilding returns the building with the given ID, or nil.
func findBuilding(w *world.World, id uint64) *world.Building {
	for _, b := range w.Buildings {
		if b.ID == id {
			return b
		}
	}
	return nil
}
