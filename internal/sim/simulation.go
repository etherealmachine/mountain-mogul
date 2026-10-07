package sim

import (
	"math"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/ai/goap"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

const (
	WalkSpeed = 0.67 // m/s (~1.5 mph); slow shuffle in stiff ski boots
	CellSize  = 5.0  // metres per grid cell

	// patienceGainPerSecRiding is patience restored per sim-second
	// while riding a lift chair: full in about 4.4 clock hours of riding.
	patienceGainPerSecRiding = 1.0 / (4.44 * world.SimSecondsPerHour)

	// patienceDrainPerSecQueuing drains patience while standing in a lift
	// queue: about 20 clock minutes of queuing exhaust it.
	patienceDrainPerSecQueuing = 1.0 / (world.SimSecondsPerHour / 3)

	// patienceDrainPerSecWalking drains patience while a guest walks
	// without skis (on a building footprint or bare ground): a clock hour
	// of it exhausts patience. Slower than the queue drain; a brief lodge
	// crossing is harmless but a long barefoot traverse costs patience.
	patienceDrainPerSecWalking = 1.0 / world.SimSecondsPerHour

	// queueSlotSec mirrors goap.queueSlotSec: expected wait per person in
	// line. Used here for the patience-prediction check at JoinQueue time.
	queueSlotSec = 8.0
)

// longQueuePersons is the queue depth at which a guest considers the line
// "long": about a quarter of a clock hour of expected wait.
var longQueuePersons = int(math.Floor(world.SimSecondsPerHour / 4 / queueSlotSec))

// Simulation drives all agent and building behaviour.
type Simulation struct {
	World      *world.World
	Pathfinder *Pathfinder
	TimeScale  float64 // simulation speed multiplier (default 4 — ~1 hr per ski season)
	SimTime    float64 // accumulated sim seconds (post-TimeScale)
	// StopAt, when > 0, is a sim time Tick never advances past, however
	// large TimeScale is — the fast-forward-to-storm stop. The caller
	// notices SimTime reaching it and clears it.
	StopAt float64
	// lastSampledDay is the most recent in-game day index whose end has
	// been written to World.History. Initialised to int(SimTime /
	// secondsPerSimDay) so a sim loaded mid-day starts recording from
	// the next rollover rather than back-filling the partial day.
	lastSampledDay int

	// Planner is the L0 GOAP planner — picks goals and chains actions
	// for every agent in the world.
	Planner *goap.Planner

	// Weather is the daily Markov-chain weather generator. Advance is called
	// once per in-game day rollover in maybeSampleHistory.
	Weather *Chain
	// Site is where the sun is computed for (SiteOf the world).
	Site Site
	// yesterday and tomorrow bracket Weather.Today() for the hourly
	// temperature curve (TempAt). tomorrow is the deterministic forecast,
	// so it matches what Advance later produces.
	yesterday, tomorrow DayWeather

	// lastHour is the index (SimTime / simSecondsPerHour) of the most
	// recent clock hour whose hourly effects (melt) have run.
	lastHour int

	// Demand owns the global skier pool and resort rating. The
	// per-30-sim-seconds poll fires from Tick.
	Demand *DemandSystem

	// traffic drives the cars (traffic.go).
	traffic *traffic

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

	// OnDayRollover, if non-nil, is called once per in-game day after
	// weather and snowfall have been applied. Used by the scene layer to
	// plow roads without creating an import cycle (scene→sim→scene).
	OnDayRollover func(w *world.World)

	// Avalanche wave state. avyFront holds cells active this generation;
	// avyNext accumulates cells for the following generation. avyGen is the
	// current generation index (incremented each hop); avyBudget accumulates
	// fractional hops and fires a full hop when it reaches 1.0. Not saved.
	avyGen    int
	avyBudget float32
	avyFront  [][2]int
	avyNext   [][2]int

	// sectionsStale triggers a full global section recompute at the start
	// of the next snowcat tick. Set by InvalidateSections whenever the
	// fleet or trail configuration changes.
	sectionsStale bool

	// catPassNight records, per snowcat ID, the night (nightIndex) on
	// which the cat last finished a full pass of its section, so each cat
	// grooms its section once per night.
	catPassNight map[uint64]int

	// groomable is every cell of a groomed trail, rebuilt with sections.
	// catGroomed records, per cat, the cells its current route has
	// groomed, so each gets one pass a night.
	groomable  map[[2]int]bool
	catGroomed map[uint64]map[[2]int]bool

	// QueryServer, if non-nil, services live SQL queries from the HTTP
	// endpoint. Tick() drains pending requests on the game thread.
	QueryServer *QueryServer

	// openToday records whether World.ResortOpen was true at any point in
	// the current sim day; the rollover bills operating costs if so and
	// standby otherwise. Not persisted — a load seeds it from ResortOpen.
	openToday bool

	// closedForDay is ClosedForDay() as of the last tick, to catch the
	// moment the lifts close.
	closedForDay bool
}

// InvalidateSections signals that cat section assignments need a full
// recompute. Call whenever cats are bought/released/toggled or trail
// grooming is changed.
func (s *Simulation) InvalidateSections() {
	s.sectionsStale = true
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
	w.Seed = seed
	rng.Init(seed)
	sim := &Simulation{
		World:          w,
		SimTime:        w.SimTime,
		Pathfinder:     NewPathfinder(w.Terrain),
		TimeScale:      4.0,
		Site:           SiteOf(w),
		Weather:        NewChainFor(w.Climate, w.BaseAltitude+terrainMinElevation(w.Terrain)),
		Planner:        goap.NewPlanner(),
		Demand:         NewDemandSystem(),
		traffic:        newTraffic(),
		spatial:        newSpatialGrid(widthM, heightM),
		lastSampledDay: int(w.SimTime / secondsPerSimDay),
		lastHour:       int(w.SimTime / simSecondsPerHour),
		openToday:      w.ResortOpen,
		sectionsStale:  true, // run reassignment on first tick to pick up loaded cats
	}
	// Start the demand poll timer at the loaded clock so the first poll
	// covers one interval, not the whole elapsed season.
	sim.Demand.LastPoll = w.SimTime
	// Run the weather through the season so far, then sample today's, so
	// the season continues the one the starting snow was laid from
	// (SeasonSnowpack) and an October start opens on October weather.
	sim.Weather.RunUpTo(sim.DateAt(w.SimTime))
	sim.closedForDay = sim.ClosedForDay()
	w.ClosedForDay = sim.closedForDay
	sim.yesterday = sim.Weather.Advance(sim.DateAt(w.SimTime))
	sim.tomorrow = sim.Weather.Forecast(sim.DateAt(w.SimTime), 1)[0]
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
	if s.StopAt > 0 && s.SimTime+remaining > s.StopAt {
		remaining = s.StopAt - s.SimTime
	}
	simulated := remaining
	for remaining > 0 {
		sub := remaining
		if sub > maxSubstepSec {
			sub = maxSubstepSec
		}
		s.subTick(sub)
		remaining -= sub
	}
	// Cars step once per frame, in steps of up to trafficStep: thousands
	// of them can't afford the guests' 1/30 s substeps.
	s.tickTraffic(simulated)
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
	s.World.SimTime = s.SimTime
	s.Demand.maybePoll(s)
	// Hourly effects run before the rollover so the day's last hour
	// still sees that day's weather.
	s.tickHourly()
	s.maybeSampleHistory()
	s.tickResortClosed()
	s.tickLifts(dt)
	s.tickGuests(dt)
	s.tickSnowcats(dt)
	s.tickPatrollers(dt)
	s.tickSnowGuns(dt)
	s.tickAvalanche(dt)
}

func (s *Simulation) tickLifts(dt float64) {
	w := s.World
	running := s.LiftsRunning()
	for _, lift := range w.Lifts {
		if lift.IsHeli() {
			s.tickHeliLift(lift, dt)
			continue
		}
		loopLen := lift.LoopLength()
		if loopLen < 1 {
			continue
		}
		fracPerSec := float64(lift.Speed) / float64(loopLen)

		// Hold: lift base has no snow — drain chairs, don't board, then stop.
		baseCell := lift.QueueCell()
		baseHasSnow := false
		if baseCell[0] >= 0 && baseCell[0] < len(w.Terrain.Cells) &&
			baseCell[1] >= 0 && baseCell[1] < len(w.Terrain.Cells[0]) {
			baseHasSnow = w.Terrain.Cells[baseCell[0]][baseCell[1]].TotalSWE() > 0
		}
		wasHeld := lift.OnHold
		lift.OnHold = !baseHasSnow
		if lift.OnHold != wasHeld && lift.Open {
			s.logLiftHoldChanged(lift)
		}
		if lift.OnHold && !wasHeld {
			// Transition to hold: eject queued guests so they can re-plan.
			ejectQueue(lift)
		}

		// Outside operating hours chairs stop once they've carried
		// everyone up, and nobody boards.
		passengers := lift.PassengerCount()
		moving := (!lift.OnHold && lift.Open && (running || passengers > 0)) || (lift.OnHold && passengers > 0)
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

					// Update ride tally so the planner's Explore goal prefers unridden lifts.
					agent.RidenLifts = ai.AddRide(agent.RidenLifts, lift.ID)

					// Stand the rider up at their seat, then advance the
					// plan past the just-completed RideLift so
					// onPlanStepStart sets TargetID for the next step
					// (typically SkiToLift/SkiToLodge/SkiToParking). They
					// glide off the ramp (tickUnloading) before skiing to it.
					s.startUnloading(agent, lift, j)
					s.advancePlan(agent)
				}
			}

			// At base (progress wraps past 1.0): fill the chair from the
			// queue up to its capacity. Skip when on hold.
			if chair.Progress >= 1.0 {
				chair.Progress -= 1.0
				if !lift.OnHold && running {
					var boarders []*world.Guest
					if len(lift.Lines) > 0 {
						boarders = lift.BoardNextPair(len(chair.Passengers))
					} else {
						for len(lift.Queue) > 0 && len(boarders) < len(chair.Passengers) {
							boarders = append(boarders, lift.Queue[0])
							lift.Queue = lift.Queue[1:]
						}
					}
					for j, agent := range boarders {
						if j >= len(chair.Passengers) {
							break
						}
						chair.Passengers[j] = agent
						agent.OnLiftID = lift.ID
						agent.Queued = false
						s.replanOnBoard(agent, lift)
					}
				}
			}
		}
	}
}

// tickHeliLift drives the state machine for a heli-ski helicopter lift.
// Called by tickLifts for every LiftHeli entry; cable-lift logic is
// handled by the regular tickLifts path.
func (s *Simulation) tickHeliLift(lift *world.Lift, dt float64) {
	w := s.World
	h := lift.HeliState
	if h == nil {
		return
	}
	switch h.Phase {
	case world.HeliAtBase:
		if !lift.Open || !s.LiftsRunning() {
			return
		}
		// Fill empty seats from the queue.
		for len(h.Passengers) < world.HeliCapacity && len(lift.Queue) > 0 {
			agent := lift.Queue[0]
			lift.Queue = lift.Queue[1:]
			h.Passengers = append(h.Passengers, agent)
			agent.OnLiftID = lift.ID
			agent.Queued = false
			// Heli keeps per-ride pricing; cable lifts are covered by the
			// day ticket bought at the ticket window.
			if fare := lift.RideFare(); fare > 0 && !agent.HasSeasonPass {
				w.Cash += fare
				w.History.RecordRevenue(world.RevenueHeli, fare)
				agent.RemainingBudget -= float32(fare)
			}
			s.replanOnBoard(agent, lift)
		}
		// Depart when full, or when at least one passenger is aboard and
		// the queue is drained (don't idle forever waiting for a full load).
		if len(h.Passengers) >= world.HeliCapacity ||
			(len(h.Passengers) > 0 && len(lift.Queue) == 0) {
			h.Phase = world.HeliToTop
			h.Progress = 0
		}

	case world.HeliToTop:
		dx := lift.Top[0] - lift.Base[0]
		dz := lift.Top[1] - lift.Base[1]
		dist := float32(math.Sqrt(float64(dx*dx + dz*dz)))
		if dist < 1 {
			dist = 1
		}
		h.Progress += float32(float64(world.HeliAirspeedMs) / float64(dist) * dt)
		if h.Progress >= 1.0 {
			h.Progress = 1.0
			h.Phase = world.HeliAtTop
		}

	case world.HeliAtTop:
		// Unload all passengers at the drop zone — same pattern as
		// regular lift unload at progress 0.5.
		topCell := lift.TopCell()
		ty := w.Terrain.SurfaceElevationAt(topCell[0], topCell[1])
		for _, agent := range h.Passengers {
			agent.OnLiftID = 0
			agent.Speed = 0
			agent.TurnSide = 0
			if agent.Balance < 0.5 {
				agent.Balance = 1.0
			}
			agent.RidenLifts = ai.AddRide(agent.RidenLifts, lift.ID)
			agent.Pos = mgl32.Vec3{lift.Top[0], ty, lift.Top[1]}
			s.advancePlan(agent)
			if pos, ok := planTargetWorldPos(w, agent); ok {
				dx2 := pos[0] - agent.Pos[0]
				dz2 := pos[2] - agent.Pos[2]
				agent.Heading = float32(math.Atan2(float64(dx2), float64(dz2)))
			}
		}
		h.Passengers = h.Passengers[:0]
		h.Phase = world.HeliToBase
		h.Progress = 0

	case world.HeliToBase:
		dx := lift.Base[0] - lift.Top[0]
		dz := lift.Base[1] - lift.Top[1]
		dist := float32(math.Sqrt(float64(dx*dx + dz*dz)))
		if dist < 1 {
			dist = 1
		}
		h.Progress += float32(float64(world.HeliAirspeedMs) / float64(dist) * dt)
		if h.Progress >= 1.0 {
			h.Phase = world.HeliAtBase
			h.Progress = 0
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

// entranceWorldPos is parkingWorldPos for the building entrance nearest
// from that opens onto service svc: a service building's closest such
// door, or the building anchor otherwise.
func entranceWorldPos(w *world.World, b *world.Building, from mgl32.Vec3, svc world.Service) mgl32.Vec3 {
	if !b.IsShell() {
		return parkingWorldPos(w, b)
	}
	p, cell := b.NearestServiceEntrance(svc, mgl32.Vec2{from[0], from[2]})
	return mgl32.Vec3{p[0], w.Terrain.SurfaceElevationAt(cell[0], cell[1]), p[1]}
}

// visitService is the service guest a is heading to a building for: the
// one the step after the head uses, so guests walk to the right door.
func visitService(w *world.World, a *world.Guest) world.Service {
	if a.Plan.Head().Kind == ai.ActWalkToTicketOffice {
		return world.ServiceTickets
	}
	if next := a.Plan.Step + 1; next < len(a.Plan.Steps) {
		switch step := a.Plan.Steps[next]; step.Kind {
		case ai.ActRestAtLodge:
			// A lodge without a lounge seats resting guests in its food
			// court, so head for that door.
			if b := findBuildingByID(w, step.BldgID); b != nil && !b.Offers(world.ServiceLounge) {
				return world.ServiceFood
			}
			return world.ServiceLounge
		case ai.ActEat:
			return world.ServiceFood
		case ai.ActRelieveThirst:
			if b := findBuildingByID(w, step.BldgID); b != nil {
				return b.DrinkService()
			}
			return world.ServiceBar
		case ai.ActBuyDayTicket, ai.ActBuySeasonPass:
			return world.ServiceTickets
		}
	}
	return world.ServiceNone
}

// endsInDeparture reports whether plan p takes the guest home.
func endsInDeparture(p ai.Plan) bool {
	return len(p.Steps) > 0 && p.Steps[len(p.Steps)-1].Kind == ai.ActDepart
}

// countDiners recounts each lodge's seated diners from guests mid-meal.
// onPlanStepStart bumps the count as guests sit so seats fill within a tick.
func countDiners(w *world.World) {
	for _, b := range w.Buildings {
		b.Diners = 0
	}
	for _, a := range w.OnMountain {
		if a.RestTimer <= 0 || a.Plan.Head().Kind != ai.ActEat {
			continue
		}
		if b := findBuildingByID(w, a.Plan.Head().BldgID); b != nil {
			b.Diners++
		}
	}
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
	countDiners(w)
	// Pre-pass: correct stale SkisOn state introduced between ticks (e.g.
	// by heatwave or other external snow mutations applied outside the
	// substep loop).
	for _, agent := range w.OnMountain {
		if agent.SkisOn && noSnowUnderfoot(w.Terrain, agent.Pos[0], agent.Pos[2]) {
			agent.SkisOn = false
			agent.SkiTransitionTimer = 0
		}
	}
	for _, agent := range w.OnMountain {
		if agent.Removed {
			continue
		}
		s.tickMood(agent, dt)
		if agent.Unload.LiftID != 0 && !agent.Fallen {
			// Scripted off the chair; planning resumes when they're clear.
			// A rider who fell getting off recovers like any fall below.
			s.tickUnloading(agent, dt)
			continue
		}
		if agent.OnPatrollerID != 0 {
			// Patroller is responsible for this guest's position and departure.
			continue
		}
		s.tickPlanning(agent)
		if agent.Removed {
			continue
		}
		// Check whether the agent needs to swap equipment. Only fires
		// when no other transition is active and not in a state that
		// already suspends locomotion.
		if agent.SkiTransitionTimer == 0 && !agent.Fallen && agent.OnLiftID == 0 && !agent.Queued && agent.RestTimer == 0 {
			s.maybeStartSkiTransition(agent)
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
		case agent.SkiTransitionTimer != 0:
			s.tickSkiTransition(agent, dt)
		case len(agent.Path) > 0 && agent.PathIdx < len(agent.Path):
			s.tickPath(agent, dt)
		default:
			s.tickLocomote(agent, dt)
		}
		// Hard invariant: skis cannot be on while standing on bare ground.
		// Use an instant correction (no transition timer) because with high
		// TimeScale the 1-second transition completes within the same
		// wall-clock tick and the cycle can recur before the tick ends.
		if agent.SkisOn && noSnowUnderfoot(s.World.Terrain, agent.Pos[0], agent.Pos[2]) {
			agent.SkisOn = false
			agent.SkiTransitionTimer = 0
		}
	}
	s.reapDeparted()
}

// spawnGuest is spawnGuestAt the lot's anchor with no parking fee.
func (s *Simulation) spawnGuest(lot *world.Building, g *world.Guest) bool {
	return s.spawnGuestAt(lot, g, lot.Pos, 0)
}

// spawnGuestAt moves a Guest out of their car and onto the mountain at
// pos in lot: sets State=OnMountain, appends to w.OnMountain, and lets
// the planner lay out their first plan. parking is their share of the
// car's parking fee, paid now. The Guest record itself is the same
// pointer that lives in w.Guests, so identity + career stats persist
// across the visit. Returns false (and rolls back state/slice, sending
// the guest home) when no viable plan can be laid.
func (s *Simulation) spawnGuestAt(lot *world.Building, g *world.Guest, pos mgl32.Vec2, parking int) bool {
	w := s.World
	cx, cz := int(pos[0]/CellSize), int(pos[1]/CellSize)
	if !w.Terrain.InBounds(cx, cz) {
		cell := lot.DoorCell()
		cx, cz = cell[0], cell[1]
	}
	elev := w.Terrain.SurfaceElevationAt(cx, cz)
	g.State = world.OnMountain
	g.Pos = mgl32.Vec3{pos[0], elev, pos[1]}
	g.Heading = 0
	g.Speed = 0
	g.Balance = 1.0
	g.SkisOn = true
	g.Patience = 1.0
	g.Energy = 1.0
	g.Hunger = 0.5 + rng.Global().Float32()*0.5
	g.Thirst = 0.5 + rng.Global().Float32()*0.5
	g.Satisfaction = scoreStart
	g.HasSeasonPass = hasValidPass(g, s.SimTime)
	// Price the day ticket before planning so the planner sees the
	// post-ticket budget. The guest arrives without a ticket and pays at
	// the window (ActBuyDayTicket); pass holders owe nothing.
	// Parking is paid at the lot on arrival, so it comes out of the
	// budget up front too.
	ticket, _ := dayTicketCharge(w, g, s.SimTime)
	g.DayTicketDue = ticket
	g.DayTicketPaid = 0
	g.HasDayTicket = false
	g.RemainingBudget = g.Traits.DailyBudget - float32(ticket+parking)
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
	if parking > 0 {
		w.Cash += parking
		w.History.RecordRevenue(world.RevenueParking, parking)
	}
	w.History.RecordArrival()
	return true
}

// applyEvent is the one place an event changes a guest's stats: it
// applies the event's ai.Effects row and records the thought reporting
// it, on the guest and in the day's tally.
func (s *Simulation) applyEvent(a *world.Guest, kind ai.ThoughtKind, context ...uint64) {
	s.applyEventScaled(a, kind, 1, context...)
}

// applyEventScaled is applyEvent with the amount scaled, for an event
// that counts for less with repeats (a great run on the same trail).
func (s *Simulation) applyEventScaled(a *world.Guest, kind ai.ThoughtKind, scale float32, context ...uint64) {
	a.Satisfaction = clamp32(a.Satisfaction+ai.Effects[kind].Satisfaction*scale, 0, 1)
	s.recordThought(a, kind, context...)
}

// setCondition turns a condition on or off for a guest. Turning on adds
// its thought once, counted for the day; its pull on the mood target
// (ai.Effects) lasts until it is turned off.
func (s *Simulation) setCondition(a *world.Guest, kind ai.ThoughtKind, on bool, context ...uint64) {
	if on == a.Conditions.Has(kind) {
		return
	}
	a.Conditions.Set(kind, on)
	if on {
		s.recordThought(a, kind, context...)
	}
}

// recordThought adds a thought to the guest's ring and the day's tally.
func (s *Simulation) recordThought(a *world.Guest, kind ai.ThoughtKind, context ...uint64) {
	a.AddThought(kind, s.SimTime, context...)
	s.World.History.RecordThought(kind)
}

// holds reports whether a condition on a stat that falls toward 0 holds:
// it starts below onset and, once on, lasts until the stat recovers past
// clear, so a stat hovering at the threshold doesn't flicker.
func holds(active bool, v, onset, clear float32) bool {
	if active {
		return v < clear
	}
	return v < onset
}

// setBlocked turns the planner-reported conditions on for this plan and
// off for those it no longer reports.
func (s *Simulation) setBlocked(a *world.Guest, blocked []ai.ThoughtKind) {
	for _, k := range plannerConditions {
		on := false
		for _, b := range blocked {
			if b == k {
				on = true
			}
		}
		s.setCondition(a, k, on)
	}
}

// plannerConditions are the conditions only the planner reports.
var plannerConditions = [...]ai.ThoughtKind{ai.ThoughtNeedsLodge, ai.ThoughtLiftsClosed, ai.ThoughtNothingForMe, ai.ThoughtNoTicketWindow, ai.ThoughtLinesFull}

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

		// Debit the day's costs: operating if the resort was open at any
		// point in the day, standby if it stayed closed.
		costs := w.StandbyCosts()
		wasOpen := s.openToday
		if wasOpen {
			costs = w.OperatingCosts()
		}
		s.openToday = w.ResortOpen
		w.Cash -= costs.Total()
		costs[world.CostInterest] = s.applyCredit(dayIdx)

		// The rating is the day's average final satisfaction; a day
		// nobody left keeps the last one.
		if r, ok := w.History.DayRating(); ok {
			w.Rating = r
		}
		sample := world.DailySample{
			Day:              s.DateAt(float64(dayIdx) * secondsPerSimDay),
			GuestsOnMountain: len(w.OnMountain),
			ArrivalsToday:    w.History.ArrivalsToday,
			DeparturesToday:  w.History.DeparturesToday,
			Cash:             w.Cash,
			Revenue:          w.History.RevenueToday,
			Costs:            costs.Total(),
			RevenueByKind:    w.History.RevenueByKindToday,
			CostsByKind:      costs,
			Open:             wasOpen,
			Rating:           w.Rating,
			ThoughtCounts:    w.History.ThoughtCountsToday,
			DepartReasons:    w.History.DepartReasonsToday,
		}
		w.History.Push(sample)
		s.logDaySummary(sample)
		s.checkGoals(dayIdx, sample)
		s.lastSampledDay++

		// Advance weather for the new day and apply terrain effects.
		newDay := s.DateAt(float64(s.lastSampledDay) * secondsPerSimDay)
		s.yesterday = s.Weather.Today()
		dw := s.Weather.Advance(newDay)
		s.tomorrow = s.Weather.Forecast(newDay, 1)[0]
		s.applyDailyWeather(dw)
		if s.OnDayRollover != nil {
			s.OnDayRollover(s.World)
		}
	}
}

// snowKindForTemp returns the kind of a newly deposited snow layer based on
// the storm temperature.
func snowKindForTemp(tempC float32) world.SnowKind {
	if tempC < -5 {
		return world.KindPowder
	}
	if tempC < -1 {
		return world.KindPackedPowder
	}
	return world.KindCement
}

// windProbForTemp returns the probability that wind fires on a Clear or
// Overcast day, driven by temperature as a proxy for winter storminess.
func windProbForTemp(tempC float32) float32 {
	if tempC < -5 {
		return 0.20
	}
	if tempC < 0 {
		return 0.12
	}
	return 0.05
}

// weatherEvent classifies a non-precipitation day into the snow transition
// events defined in SNOW_IMPROVEMENTS.md.
type weatherEvent int

const (
	weatherEventNone weatherEvent = iota
	weatherEventRain
	weatherEventColdClear
	weatherEventWarmClear
	weatherEventWind
)

// kindTransition returns the new snow kind after a non-precipitation event.
func kindTransition(current world.SnowKind, evt weatherEvent) world.SnowKind {
	switch evt {
	case weatherEventRain:
		switch current {
		case world.KindSlush, world.KindCorn:
			return world.KindSlush
		default:
			return world.KindCement
		}
	case weatherEventColdClear:
		switch current {
		case world.KindPowder:
			return world.KindPowder
		case world.KindPackedPowder:
			return world.KindCrust
		case world.KindBase:
			return world.KindBase // base doesn't ice up from cold alone
		case world.KindCement, world.KindSlush, world.KindCorn:
			return world.KindFrozenGranular // first freeze of wet snow
		default: // Crust, WindSlab, FrozenGranular
			return world.KindBoilerplate
		}
	case weatherEventWarmClear:
		switch current {
		case world.KindSlush:
			return world.KindSlush
		case world.KindPowder:
			return world.KindPackedPowder
		case world.KindPackedPowder:
			return world.KindCement // first warm day wets packed surface; needs a second to slush
		case world.KindFrozenGranular, world.KindBase:
			return world.KindCorn // freeze-thaw granulation; warm day activates base surface
		case world.KindCorn:
			return world.KindSlush // over-saturates with continued warmth
		default: // Cement, WindSlab, Crust, Boilerplate
			return world.KindSlush
		}
	case weatherEventWind:
		switch current {
		case world.KindBoilerplate, world.KindBase:
			return current // too dense to reorganise
		default:
			return world.KindWindSlab
		}
	}
	return current
}

// TriggerStorm immediately applies one heavy-snow day to the terrain — same
// effect as a natural blizzard rollover. Called by the debug console cheat.
func (s *Simulation) TriggerStorm() {
	s.applyDailyWeather(DayWeather{
		State:      WeatherHeavySnow,
		TempC:      -10,
		CloudCover: 0.95,
		AccumSWE:   0.025,
	})
}

// TriggerAvalanche picks a single random cell from those with the highest
// instability and releases it. Used by the debug console cheat.
func (s *Simulation) TriggerAvalanche() {
	t := s.World.Terrain

	type candidate struct {
		x, z  int
		score float32
	}
	var best []candidate
	bestScore := float32(0)
	for x := range t.Cells {
		for z := range t.Cells[x] {
			c := &t.Cells[x][z]
			score := c.InstabilityScore()
			if score <= 0 {
				continue
			}
			if score > bestScore+0.05 {
				bestScore = score
				best = best[:0]
			}
			if score >= bestScore-0.05 {
				best = append(best, candidate{x, z, score})
			}
		}
	}
	if len(best) == 0 {
		return
	}
	pick := best[rng.Global().Intn(len(best))]
	if s.startAvalanche(t, pick.x, pick.z) {
		s.logAvalanches(1, [2]int{pick.x, pick.z})
	}
	t.SnowDirty = true
}

// TriggerHeatwave immediately applies one warm sunny day to the terrain —
// same melt and kind-transition effects as a natural spring day. Called by
// the debug console cheat.
func (s *Simulation) TriggerHeatwave() {
	dw := DayWeather{
		State:      WeatherClear,
		TempC:      +10,
		TempHigh:   +15,
		TempLow:    +5,
		CloudCover: 0.05,
	}
	s.applyDailyWeather(dw)
	s.applyDayMelt(dw)
}

// applyDailyWeather runs all terrain snow effects for one day rollover:
// snowfall (push new layer), non-precipitation kind transitions, melt, and
// daily traffic decay.
func (s *Simulation) applyDailyWeather(dw DayWeather) {
	t := s.World.Terrain

	// Stochastic wind roll for Clear and Overcast days.
	windFired := false
	if dw.State == WeatherClear || dw.State == WeatherOvercast {
		windFired = rng.Global().Float32() < windProbForTemp(dw.TempC)
	}

	switch dw.State {
	case WeatherLightSnow, WeatherHeavySnow:
		s.pushSnowLayer(dw)
		s.World.ClearLodgeFloors()
	default:
		// Determine the non-precipitation event type.
		var evt weatherEvent
		switch dw.State {
		case WeatherRain:
			evt = weatherEventRain
		case WeatherClear:
			if windFired {
				evt = weatherEventWind
			} else if dw.TempC < 0 {
				evt = weatherEventColdClear
			} else {
				evt = weatherEventWarmClear
			}
		case WeatherOvercast:
			if windFired {
				evt = weatherEventWind
			}
		}
		if evt != weatherEventNone {
			s.applyKindTransition(evt)
		}
		// Melt runs hourly through the day (tickHourly).
	}

	// Avalanche check: significant snowfall or rain can trigger slab release.
	if dw.AccumSWE > 0.04 || dw.State == WeatherRain {
		s.checkAvalanches()
	}

	// Lakes freeze, thicken, and thaw with the day's temperature.
	s.stepLakes(dw)

	// Decay skier traffic on all cells once per day (~4 day half-life).
	for x := range t.Cells {
		for z := range t.Cells[x] {
			t.Cells[x][z].SkierTraffic *= 0.85
		}
	}

	t.SnowDirty = true
}

// pushSnowLayer adds a new storm layer on top of every terrain cell. If the
// new kind matches Top, Top grows in place. Otherwise the current Top merges
// into Base and a new Top begins. Grooming is diluted by snowfall depth.
func (s *Simulation) pushSnowLayer(dw DayWeather) {
	t := s.World.Terrain
	kind := snowKindForTemp(dw.TempC)
	accumSWE := dw.AccumSWE

	// Grooming burial factor: 2 cm SWE fully covers fresh corduroy.
	const fullBurialSWE = float32(0.02)
	burialFactor := clamp01(accumSWE / fullBurialSWE)

	for x := range t.Cells {
		for z := range t.Cells[x] {
			c := &t.Cells[x][z]

			if c.Top.Accumulation > 0 && c.Top.Kind == kind {
				// Same kind — grow Top in place.
				c.Top.Accumulation += accumSWE
			} else {
				// Different kind: fold current Top into Base, start new Top.
				c.Base += c.Top.Accumulation
				c.Top = world.SnowLayer{Kind: kind, Accumulation: accumSWE}
			}

			c.Grooming *= 1 - burialFactor
		}
	}
	// What steep ground can't hold sluffs down to where it can.
	t.ShedSnow()

	// Bury skier tracks. 2 cm SWE fully covers any track.
	trackFactor := float32(1.0) - burialFactor
	if trackFactor < 1.0 {
		t.Surface.DecayTracks(trackFactor)
	}
	if burialFactor >= 1 {
		t.Groom.Clear()
	}
}

// applyKindTransition modifies the top layer's Kind on every terrain cell
// according to the given non-precipitation weather event.
func (s *Simulation) applyKindTransition(evt weatherEvent) {
	t := s.World.Terrain
	for x := range t.Cells {
		for z := range t.Cells[x] {
			c := &t.Cells[x][z]
			if top := c.TopLayer(); top != nil {
				top.Kind = kindTransition(top.Kind, evt)
			}
		}
	}
}

// tickHourly runs the effects that step once per clock hour: snowmelt
// from that hour's air temperature and sun. Snow days don't melt.
func (s *Simulation) tickHourly() {
	idx := int(s.SimTime / simSecondsPerHour)
	for s.lastHour < idx {
		mid := (float64(s.lastHour) + 0.5) * simSecondsPerHour
		s.lastHour++
		dw := s.Weather.Today()
		if dw.IsSnowing() {
			continue
		}
		s.meltHour(dw, s.DateAt(mid), HourOfDay(mid), s.TempAt(mid), 1.0/24)
	}
}

// applyDayMelt runs a whole day of hourly melt for dw at once, treating it
// as yesterday, today and tomorrow (the heatwave cheat).
func (s *Simulation) applyDayMelt(dw DayWeather) {
	date := s.DateAt(s.SimTime)
	for h := 0; h < 24; h++ {
		hour := float64(h) + 0.5
		s.meltHour(dw, date, hour, tempCurve(s.Site, date, hour, dw, dw, dw), 1.0/24)
	}
}

// meltHour removes one step of melt (days of it: frac) from every cell at
// base-area air temperature tempC, using the melt model in snowmelt.go:
// degree-days at the cell's lapsed temperature, scaled by its direct sun
// at this moment for its slope and aspect, cut to the shade rate while
// surrounding terrain hides the sun (world.HorizonMap).
func (s *Simulation) meltHour(dw DayWeather, date time.Time, hour float64, tempC, frac float32) {
	t := s.World.Terrain
	rainMelt := dw.RainMM * rainMeltPerMM * frac
	if rainMelt <= 0 && tempC <= 0 {
		return // even the base area is below freezing, and it's dry
	}
	baseElev := terrainMinElevation(t)
	sun := s.Site.SunAt(date, hour)
	beam := newInstantSun(sun, dw.CloudCover)
	var horizon *world.HorizonMap
	if beam.weight > 0 {
		horizon = t.Horizon()
	}
	sunDir := [3]float32{sun.Dir[0], sun.Dir[1], sun.Dir[2]}
	melted := false
	for x := range t.Cells {
		for z := range t.Cells[x] {
			c := &t.Cells[x][z]
			if c.Top.Accumulation == 0 && c.Base == 0 {
				continue
			}
			melt := rainMelt
			if temp := tempC - lapseRate*(c.GroundElevation-baseElev); temp > 0 {
				gx, gz := t.GradientAt(x, z)
				exposure := beam.exposure(gx, gz)
				if exposure > 0 {
					exposure *= horizon.SunVisibility(x, z, sunDir)
				}
				melt += meltFactor(exposure) * temp * frac
			}
			if melt > 0 {
				meltCell(c, melt)
				melted = true
			}
		}
	}
	if melted {
		t.SnowDirty = true
	}
}

// terrainMinElevation is the lowest ground elevation: the base area, where
// the weather's temperatures apply.
func terrainMinElevation(t *world.Terrain) float32 {
	lo := t.Cells[0][0].GroundElevation
	for x := range t.Cells {
		for z := range t.Cells[x] {
			lo = min(lo, t.Cells[x][z].GroundElevation)
		}
	}
	return lo
}

// meltCell removes melt metres of SWE from c. Melts Top first; when Top is
// exhausted Base is promoted to Top (clearing grooming and moguls), then
// the promoted layer melts if budget remains.
func meltCell(c *world.Cell, melt float32) {
	if c.Top.Accumulation > 0 {
		if melt >= c.Top.Accumulation {
			melt -= c.Top.Accumulation
			c.Top = world.SnowLayer{}
			c.Grooming = 0
			c.MogulSize = 0
			// Promote Base to a KindBase Top so the invariant holds.
			if c.Base > 0 {
				c.Top = world.SnowLayer{Kind: world.KindBase, Accumulation: c.Base}
				c.Base = 0
			}
		} else {
			c.Top.Accumulation -= melt
			melt = 0
		}
	}

	if melt > 0 && c.Top.Accumulation > 0 {
		if melt >= c.Top.Accumulation {
			c.Top = world.SnowLayer{}
		} else {
			c.Top.Accumulation -= melt
		}
	}
}

// =============================================================================
// Planning layer — drives target / queue / removal off the stored ai.Plan
// =============================================================================

// restAtLodgeSec mirrors goap.restDurationSec — a rest takes 20 clock
// minutes; the planner costs RestAtLodge the same so plan cost and
// runtime stay in sync.
const restAtLodgeSec = world.SimSecondsPerHour / 3

// mealSec mirrors goap.mealDurationSec — how long a diner holds a seat:
// half a clock hour.
const mealSec = world.SimSecondsPerHour / 2

// drinkSec is how long a drink takes: ten clock minutes.
const drinkSec = world.SimSecondsPerHour / 6

// tickPlanning is the per-agent replan / advance check. Runs first in
// the per-agent loop so any implicit state it sets (Queued, TargetID,
// RestTimer, Removed) is visible to the dispatch switch below it.
//
// Replan triggers (a deliberate subset of the notes/specs/Guests Spec.md design):
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
			s.judgeRun(a)
		}
		// For trail-to-trail steps, mark the arrival at the junction so the
		// next step's precondition (AtTrailEnd == destTrailID) can fire.
		if head.Kind == ai.ActSkiTrail && head.LiftID == 0 && head.BldgID == 0 {
			a.AtTrailEnd = head.TrailID
		} else {
			a.AtTrailEnd = 0
		}
		// Closed for the day: a plan that doesn't end in going home (a
		// rest planned on the lift before closing) gives way to GoHome at
		// the next step.
		if s.World.ClosedForDay && head.Kind != ai.ActJoinQueue && !endsInDeparture(a.Plan) {
			s.replan(a)
			return
		}
		// Step boundaries are the safe points to drop a plan for a need
		// that turned pressing mid-plan — except in a lift line.
		if head.Kind != ai.ActJoinQueue && a.Plan.GoalName != "GoHome" && goap.NeedPreempts(&snap, s.World, &a.Plan) {
			s.replan(a)
			return
		}
		s.advancePlan(a)
		return
	}
	if !planActionPreconditionHolds(head, snap, s.World) {
		s.replan(a)
		return
	}
}

// fellSince reports whether the guest fell at or after simTime.
func fellSince(a *world.Guest, simTime float64) bool {
	for i := len(a.Events) - 1; i >= 0 && a.Events[i].Time >= simTime; i-- {
		if a.Events[i].Kind == ai.EventFall {
			return true
		}
	}
	return false
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
	a.Plan = s.Planner.StoredPlanFor(a, s.World)
	s.setBlocked(a, a.Plan.Blocked)
	if a.Plan.GoalName == (goap.GoHome{}).Name() {
		s.setDepartReason(a, s.departReasonFor(a))
	}
	if !a.Plan.Done() {
		s.onPlanStepStart(a)
		return
	}
	// Planner returned nothing — guest is stranded (e.g. all lifts held,
	// not at a lift top). Send them home rather than leaving them frozen.
	if !a.Removed {
		s.directHomePlan(a)
	}
}

// replanOnBoard builds a post-ride plan while the agent is still on the
// chair. StoredPlanForLookahead plans from a simulated "just unloaded"
// snapshot; we then prepend the in-flight RideLift so advancePlan at
// unload steps past it and lands on the first post-ride action.
func (s *Simulation) replanOnBoard(agent *world.Guest, lift *world.Lift) {
	lookahead := s.Planner.StoredPlanForLookahead(agent, lift.ID, s.World)
	s.setBlocked(agent, lookahead.Blocked)
	if lookahead.Done() {
		return
	}
	rideLiftStep := ai.PlanAction{Kind: ai.ActRideLift, LiftID: lift.ID}
	lookahead.Steps = append([]ai.PlanAction{rideLiftStep}, lookahead.Steps...)
	agent.Plan = lookahead
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
		// Replan if the lift has been closed or placed on hold since this
		// plan was built — don't queue on an inoperable lift. Clear the
		// plan and return; tickPlanning will replan next tick (calling
		// replan here would recurse: replan→onPlanStepStart→here→...).
		if !lift.Open || lift.OnHold {
			a.Plan.Steps = nil
			return
		}
		// Closed for the day: lifts load no one. A guest about to queue
		// heads straight for the parking lot instead, skiing or walking
		// down, rather than riding up for one more run on the way out.
		// Before opening they queue and wait for the first chair.
		if s.ClosedForDay() {
			s.setDepartReason(a, ai.DepartClosing)
			s.directHomePlan(a)
			return
		}
		// Safety-net: no riding without a pass or a day ticket.
		if !a.HasSeasonPass && !a.HasDayTicket {
			a.Plan.Steps = nil
			return
		}
		// Safety-net: queue grew past the threshold since this plan was built.
		// Zero patience first (same as the original pattern) so that when
		// replan calls onPlanStepStart for the GoHome JoinQueue step the
		// bail guard (Patience >= 0.05) is false — preventing recursion.
		qLen := lift.QueueLen()
		if a.Patience >= 0.05 && qLen > goap.MaxQueuePersons {
			s.applyEvent(a, ai.ThoughtLineTooLong, lift.ID)
			a.Patience = 0
			s.replan(a)
			return
		}
		if qLen >= longQueuePersons {
			s.applyEvent(a, ai.ThoughtLongLine, lift.ID)
		}
		if len(lift.Lines) > 0 {
			lineIdx := lift.ShortestLineIdx()
			lift.Lines[lineIdx].Guests = append(lift.Lines[lineIdx].Guests, a)
		} else {
			lift.Queue = append(lift.Queue, a)
		}
		a.Queued = true
		a.TargetID = 0
	case ai.ActRideLift:
		// Boarding handled by tickLifts' chair-load branch.
	case ai.ActSkiToLift:
		s.startRun(a)
		lift := findLiftByID(w, step.LiftID)
		if lift == nil {
			return
		}
		a.TargetID = lift.ID
		a.Plan.Goal = ai.GoalLift
		a.Plan.GoalID = lift.ID
		a.Plan.Target = lift.BackOfQueueWorldPos(w.Terrain)
	case ai.ActSkiToLodge:
		s.startRun(a)
		b := findBuildingByID(w, step.BldgID)
		if b == nil {
			return
		}
		a.TargetID = b.ID
		if next := a.Plan.Step + 1; next < len(a.Plan.Steps) && a.Plan.Steps[next].Kind == ai.ActRelieveThirst {
			a.Plan.Goal = ai.GoalRelieveThirst
		} else {
			a.Plan.Goal = ai.GoalNone
		}
		a.Plan.GoalID = b.ID
		a.Plan.Target = entranceWorldPos(w, b, a.Pos, visitService(w, a))
	case ai.ActSkiToParking:
		s.startRun(a)
		b := findBuildingByID(w, step.BldgID)
		if b == nil {
			return
		}
		a.TargetID = b.ID
		a.Plan.Goal = ai.GoalDepart
		a.Plan.GoalID = b.ID
		a.Plan.Target = parkingWorldPos(w, b)
	case ai.ActSkiTrail:
		s.startRun(a)
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
			a.Plan.Target = entranceWorldPos(w, b, a.Pos, visitService(w, a))
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
	case ai.ActWalkToTicketOffice:
		b := findBuildingByID(w, step.BldgID)
		if b == nil {
			return
		}
		a.TargetID = b.ID
		a.Plan.Goal = ai.GoalNone
		a.Plan.GoalID = b.ID
		a.Plan.Target = entranceWorldPos(w, b, a.Pos, world.ServiceTickets)
		startCell := [2]int{
			int(math.Floor(float64(a.Pos[0] / CellSize))),
			int(math.Floor(float64(a.Pos[2] / CellSize))),
		}
		_, door := b.NearestServiceEntrance(world.ServiceTickets, mgl32.Vec2{a.Pos[0], a.Pos[2]})
		a.Path = s.Pathfinder.FindPath(startCell, door)
		a.PathIdx = 0
		if a.Path == nil && !a.HasSeasonPass && !a.HasDayTicket {
			// No walkable route to the window: without a ticket there
			// is nothing to do here, so give up rather than ski free.
			s.setCondition(a, ai.ThoughtNoTicketWindow, true)
			s.setDepartReason(a, ai.DepartNoTicket)
			s.directHomePlan(a)
		}

	case ai.ActBuyDayTicket:
		if findBuildingByID(w, step.BldgID) == nil {
			return
		}
		// Pay at the window. RemainingBudget already excludes the ticket.
		price := a.DayTicketDue
		a.DayTicketDue = 0
		a.DayTicketPaid = price
		a.HasDayTicket = true
		if price > 0 {
			w.Cash += price
			w.History.RecordRevenue(world.RevenueDayTickets, price)
		}

	case ai.ActBuySeasonPass:
		b := findBuildingByID(w, step.BldgID)
		if b == nil {
			return
		}
		// Execute the pass purchase immediately on step start. Today's
		// day ticket is credited toward the pass: a bought ticket
		// reduces what the resort collects, and a ticket still owed was
		// already set aside from RemainingBudget.
		price := w.SeasonPassPrice - a.DayTicketPaid
		if price < 0 {
			price = 0
		}
		spend := price - a.DayTicketDue
		if spend < 0 {
			spend = 0
		}
		a.DayTicketPaid = 0
		a.DayTicketDue = 0
		w.Cash += price
		w.History.RecordRevenue(world.RevenueSeasonPasses, price)
		a.RemainingBudget -= float32(spend)
		// Expiry = end of the current season.
		now := s.DateAt(s.SimTime)
		closeYear := SeasonCloseYearFor(now)
		closeDate := SeasonCloseDate(closeYear)
		daysToClose := closeDate.Sub(now).Hours() / 24.0
		if daysToClose < 1 {
			daysToClose = 1
		}
		a.SeasonPassExpiry = s.SimTime + daysToClose*secondsPerSimDay
		a.HasSeasonPass = true
		_ = b // building exists — purchase confirmed

	case ai.ActRestAtLodge:
		a.RestTimer = restAtLodgeSec
		a.Speed = 0
		a.TargetID = 0
	case ai.ActRelieveThirst:
		a.RestTimer = drinkSec
		a.Speed = 0
		a.TargetID = 0
		if b := findBuildingByID(w, step.BldgID); b != nil && b.DrinkPrice > 0 {
			w.Cash += b.DrinkPrice
			revenue := world.RevenueBar
			if b.DrinkService() == world.ServiceFood {
				revenue = world.RevenueFood
			}
			w.History.RecordRevenue(revenue, b.DrinkPrice)
			a.RemainingBudget -= float32(b.DrinkPrice)
		}
	case ai.ActEat:
		b := findBuildingByID(w, step.BldgID)
		if b == nil {
			return
		}
		a.RestTimer = mealSec
		a.Speed = 0
		a.TargetID = 0
		b.Diners++
		w.Cash += b.MealPrice
		w.History.RecordRevenue(world.RevenueFood, b.MealPrice)
		a.RemainingBudget -= float32(b.MealPrice)
	case ai.ActDepart:
		// Capture session stats (LastScore, LifetimeVisits, LastVisit,
		// VisitsThisSeason) on the persistent Guest record before the
		// reaper clears sim scratch fields, then flip Removed so
		// reapDeparted will splice this Guest out of OnMountain and into
		// their car.
		// Set the departure aside until the car drives off the map
		// (finishDeparture); a guest with no car leaves now.
		s.setDepartReason(a, s.departReasonFor(a))
		a.Leaving = world.Leaving{Pending: true, Score: a.Satisfaction, Conditions: a.Conditions, Reason: a.DepartReason}
		if a.CarID == 0 {
			s.finishDeparture(a)
		}
		a.Removed = true
	}
}

// finishDeparture records a guest's day once they've left the map: their
// score with the end-of-day term (each condition still on as they drove
// away costs its hourly rate once more), their career stats, and the
// day's departure for the rating and the "Why guests left" chart.
func (s *Simulation) finishDeparture(g *world.Guest) {
	l := g.Leaving
	if !l.Pending {
		return
	}
	g.Leaving = world.Leaving{}
	score := l.Score
	for k := ai.ThoughtKind(1); int(k) < ai.ThoughtKindCount; k++ {
		if l.Conditions.Has(k) {
			score += ai.Effects[k].Satisfaction
		}
	}
	score = clamp32(score, 0, 1)
	s.Demand.recordDeparture(s.World, g, score, s.DateAt(s.SimTime))
	s.World.History.RecordDeparture(score, l.Reason)
}

// setDepartReason records why a guest is going home. The first reason
// sticks: a guest sent home by a minor injury who later gets tired
// still left because they were hurt.
func (s *Simulation) setDepartReason(a *world.Guest, why ai.DepartReason) {
	if a.DepartReason == ai.DepartNone {
		a.DepartReason = why
	}
}

// departReasonFor works out why a guest heading home is leaving, from
// what's holding them back now: a blocked ride first, then the stat that
// ran out, else they'd simply done what they came for.
func (s *Simulation) departReasonFor(a *world.Guest) ai.DepartReason {
	has := a.Conditions.Has
	switch {
	case s.ClosedForDay():
		return ai.DepartClosing
	case has(ai.ThoughtNoTicketWindow):
		return ai.DepartNoTicket
	case has(ai.ThoughtLiftsClosed):
		return ai.DepartLiftsClosed
	case has(ai.ThoughtNothingForMe):
		return ai.DepartNothingToSki
	case has(ai.ThoughtLinesFull):
		return ai.DepartLines
	case has(ai.ThoughtTooExpensive):
		return ai.DepartMoney
	case a.Patience < exhaustedThreshold:
		return ai.DepartLines
	case a.Energy < exhaustedThreshold:
		return ai.DepartTired
	case a.Hunger < exhaustedThreshold:
		return ai.DepartHungry
	case a.Thirst < exhaustedThreshold:
		return ai.DepartThirsty
	}
	return ai.DepartDone
}

// directHomePlan assigns a direct [SkiToParking, Depart] plan to a,
// bypassing the GOAP planner. Used when normal replanning would produce
// nothing (e.g. all lifts held, guest stranded at lift base) and for
// injured guests giving up. If no parking lot exists the guest is removed.
func (s *Simulation) directHomePlan(a *world.Guest) {
	w := s.World
	var best *world.Building
	var bestD2 float32
	for _, b := range w.Buildings {
		if b.Type != world.BuildingParking {
			continue
		}
		if b.ID == a.CarLot {
			best = b // their car is here
			break
		}
		dx := b.Pos[0] - a.Pos[0]
		dz := b.Pos[1] - a.Pos[2]
		d2 := dx*dx + dz*dz
		if best == nil || d2 < bestD2 {
			best = b
			bestD2 = d2
		}
	}
	if best == nil {
		a.Removed = true
		return
	}
	a.Plan = ai.Plan{
		GoalName: "GoHome",
		Steps: []ai.PlanAction{
			{Kind: ai.ActSkiToParking, BldgID: best.ID},
			{Kind: ai.ActDepart, BldgID: best.ID},
		},
	}
	s.onPlanStepStart(a)
}

// injuredGiveUpPlan builds a direct [SkiToParking, Depart] plan so an
// injured guest who gave up walks to the nearest parking lot without routing
// through the lift system. If no parking lot exists the guest is removed
// immediately — there is nowhere for them to go.
func (s *Simulation) injuredGiveUpPlan(a *world.Guest) {
	s.setDepartReason(a, ai.DepartAbandoned)
	s.directHomePlan(a)
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
		return snap.AtLodge == step.BldgID || snap.AtBar == step.BldgID
	case ai.ActSkiToParking:
		return snap.AtParking == step.BldgID
	case ai.ActWalkToTicketOffice:
		return snap.AtTicketOffice == step.BldgID
	case ai.ActBuySeasonPass, ai.ActBuyDayTicket:
		return true // executed atomically in onPlanStepStart
	case ai.ActRestAtLodge:
		return a.RestTimer <= 0
	case ai.ActRelieveThirst, ai.ActEat:
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
			return snap.AtLodge == step.BldgID || snap.AtParking == step.BldgID || snap.AtBar == step.BldgID
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
		l := findLiftByID(w, step.LiftID)
		return l != nil && l.Open && !l.OnHold
	case ai.ActSkiToLodge, ai.ActSkiToParking, ai.ActRestAtLodge, ai.ActRelieveThirst,
		ai.ActDepart, ai.ActWalkToTicketOffice, ai.ActBuySeasonPass, ai.ActBuyDayTicket:
		b := findBuildingByID(w, step.BldgID)
		return b != nil && b.Usable()
	case ai.ActEat:
		b := findBuildingByID(w, step.BldgID)
		return b != nil && b.ServesFood()
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
		// Restore stats based on which building we just finished visiting.
		switch a.Plan.Head().Kind {
		case ai.ActRestAtLodge:
			a.Patience = 1
			a.Energy = 1
			s.applyEvent(a, ai.ThoughtRested)
		case ai.ActRelieveThirst:
			a.Thirst = 1
			s.applyEvent(a, ai.ThoughtGoodDrink)
		case ai.ActEat:
			a.Hunger = 1
			a.Thirst = 1 // a meal comes with a drink
			s.applyEvent(a, ai.ThoughtGoodMeal)
		}
	}
}

// reapDeparted removes agents flagged by ActDepart at the end of
// tickGuests. Defers removal out of the range loop so the slice header
// doesn't shift mid-iteration.
func (s *Simulation) reapDeparted() {
	w := s.World
	for i := len(w.OnMountain) - 1; i >= 0; i-- {
		if g := w.OnMountain[i]; g.Removed {
			w.RemoveFromOnMountain(g.ID)
			if g.CarID != 0 {
				g.State = world.InCar // waits in the car for the rest of the carload
			}
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
	case ai.ActSkiToLodge, ai.ActSkiToParking, ai.ActWalkToTicketOffice:
		if b := findBuildingByID(w, step.BldgID); b != nil {
			return entranceWorldPos(w, b, a.Pos, visitService(w, a)), true
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
	agent.Speed = WalkSpeed
}

// tickQueued walks the agent toward their assigned queue slot and orients
// them to face the boarding spot. For line-mode lifts, the slot position
// accounts for lane, depth, and pair position. For legacy flat-queue lifts
// the single-file behaviour is unchanged.
func (s *Simulation) tickQueued(agent *world.Guest, dt float64) {
	w := s.World
	drain := float32(dt * patienceDrainPerSecQueuing)
	if !s.LiftsRunning() {
		drain = 0 // waiting for the lifts to open doesn't try anyone's patience
	}
	for _, lift := range w.Lifts {
		// Line mode: locate the agent in one of the configured lanes.
		if len(lift.Lines) > 0 {
			if pos, ok := lift.GuestSlotWorldPos(agent, w.Terrain); ok {
				agent.Patience -= drain
				if agent.Patience < 0 {
					agent.Patience = 0
				}
				s.tickWalkToward(agent, pos, dt)
				faceX := lift.Base[0] - agent.Pos[0]
				faceZ := lift.Base[1] - agent.Pos[2]
				if faceX*faceX+faceZ*faceZ > 0.01 {
					agent.Heading = float32(math.Atan2(float64(faceX), float64(faceZ)))
				}
				return
			}
		}
		// Legacy flat-queue mode.
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
		agent.Patience -= drain
		if agent.Patience < 0 {
			agent.Patience = 0
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
		if lift.IsHeli() {
			// Heli passenger: glue position to the helicopter body.
			pos := lift.HeliWorldPos(w.Terrain)
			agent.Pos = pos
			agent.Heading = lift.HeliHeading()
			return
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
// on local slope and goal direction. Arrival is observed by tickPlanning on
// the next frame via a snapshot extract — the implicit AtLiftBase /
// AtLodge / AtParking signal is what tells the planning layer the head
// step is done, so this function doesn't need an explicit on-arrival
// callback.
func (s *Simulation) tickLocomote(agent *world.Guest, dt float64) {
	w := s.World
	var targetPos mgl32.Vec3
	if agent.TargetID != 0 {
		var ok bool
		targetPos, ok = resolveTarget(w, agent.TargetID, agent)
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

	// Walk (not ski) when skis are off, or when terrain and momentum
	// no longer call for skiing.
	if !agent.SkisOn || (!shouldSki(w.Terrain, agent.Pos, targetPos) && agent.Speed <= skiWalkSpeed) {
		// Remove skis when close to lodge or parking — guests shouldn't
		// shuffle the last few metres in full gear. Only trigger near the
		// destination so a departing guest still skis down from the lift
		// top rather than removing skis immediately after unloading.
		if agent.SkisOn && agent.SkiTransitionTimer == 0 {
			if d, ok := leavingDistance(agent, targetPos); ok && d < skisOffNearDest {
				agent.SkiTransitionTimer = 1.0
				return
			}
		}
		s.recordWalkTick(agent, targetPos)
		s.tickWalkToward(agent, targetPos, dt)
		if !agent.SkisOn {
			agent.Patience -= float32(dt * patienceDrainPerSecWalking)
			if agent.Patience < 0 {
				agent.Patience = 0
			}
		}
		return
	}
	s.tickSkier(agent, targetPos, dt)
}

// Near a lodge or parking lot guests walk the last stretch: skis come off
// within skisOffNearDest of the door, and don't go back on until beyond
// skisOnNearDest. Both are measured to the same live door (resolveTarget),
// so a guest at the edge can't swap skis forever, as one did when one
// check used the door picked at the step's start and the other the door
// nearest now.
const (
	skisOffNearDest = 30.0
	skisOnNearDest  = 40.0
)

// leavingDistance is how far a guest heading into a lodge or to their
// car is from target (the door they're walking to), and false when
// they're heading anywhere else.
func leavingDistance(a *world.Guest, target mgl32.Vec3) (float32, bool) {
	head := a.Plan.Head()
	if head.Kind != ai.ActSkiToLodge && head.Kind != ai.ActSkiToParking && a.Plan.Goal != ai.GoalDepart {
		return 0, false
	}
	return mgl32.Vec2{target[0] - a.Pos[0], target[2] - a.Pos[2]}.Len(), true
}

// liveTarget is where a guest is heading right now: their target entity's
// door or lift line, else their plan's target point.
func liveTarget(w *world.World, a *world.Guest) (mgl32.Vec3, bool) {
	if a.TargetID != 0 {
		return resolveTarget(w, a.TargetID, a)
	}
	return a.Plan.Target, a.Plan.Target != (mgl32.Vec3{})
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
func resolveTarget(w *world.World, id uint64, a *world.Guest) (mgl32.Vec3, bool) {
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
			return entranceWorldPos(w, b, a.Pos, visitService(w, a)), true
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
