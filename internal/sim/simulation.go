package sim

import (
	"math"
	"sort"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/ai/goap"
	"mountain-mogul/internal/edition"
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

	// patienceDrainPerSecServiceLine drains patience while waiting at a
	// building's door to be served: half the lift-line rate, since
	// they're out of the wind (Service Improvements).
	patienceDrainPerSecServiceLine = patienceDrainPerSecQueuing / 2

	// serviceLineThoughtSec is a wait at the door long enough to complain
	// about: ten clock minutes.
	serviceLineThoughtSec = world.SimSecondsPerHour / 6

	// patienceDrainPerSecWalking drains patience while a guest walks
	// without skis (on a building footprint or bare ground): a clock hour
	// of it exhausts patience. Slower than the queue drain; a brief lodge
	// crossing is harmless but a long barefoot traverse costs patience.
	patienceDrainPerSecWalking = 1.0 / world.SimSecondsPerHour
)

// Simulation drives all agent and building behaviour.
type Simulation struct {
	// baseElev is the base area's ground height (terrainMinElevation),
	// found on first use by snowmaking.
	baseElev      float32
	baseElevKnown bool
	// sunDay is the sim day sunRise and sunSet (clock hours) are for
	// (tickDark).
	sunDay int
	// quietSlides counts slides away from the runs not yet logged, the
	// first at quietSlideAt; pathTrailsKey is the runs the slide paths'
	// crossings were worked out for (slide_paths.go).
	quietSlides     int
	quietSlideAt    [2]int
	pathTrailsKey   int
	sunRise, sunSet float32
	// exposure is each lift's top's exposure to the wind (wind.go).
	exposure map[uint64]float32
	// snowmakingStopped is whether snowmaking could run last tick (the
	// lifts stopped), to close each night when it ends.
	snowmakingStopped bool

	// PlayerInput is the keys the player skiing (World.Player) is
	// pressing, set by the scene each frame (player.go).
	PlayerInput PlayerInput

	World      *world.World
	Pathfinder *Pathfinder
	TimeScale  float64 // simulation speed multiplier (default 4 — ~1 hr per ski season)
	SimTime    float64 // accumulated sim seconds (post-TimeScale)
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
	// lastSaid is the complaint the event feed last reported as the
	// day's most common (logGuestsSay); cleared at rollover.
	lastSaid ai.ThoughtKind
	// hintsPosted is the day (index + 1) each setup hint was last posted.
	hintsPosted map[string]int
	// longLinesSaid is the lifts whose long line the feed reported today.
	longLinesSaid map[uint64]bool
	// rescues counts patrol rescues since the feed last reported them,
	// and where the latest ended.
	rescues struct {
		n    int
		last mgl32.Vec2
	}
	// cutOff counts guests cut off by open water since the feed last
	// reported them, and where the latest was.
	cutOff struct {
		n    int
		last mgl32.Vec2
	}

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

	// spatial is the per-step agent grid the L1 hazard sampler reads.
	// Without it, hazardDensityAt iterates every other agent per
	// query and sampleTactical becomes O(N²) in agent count — at 50×
	// with the demand system pushing N past ~150 that wedged the main
	// thread for seconds at a time. The grid is rebuilt once per Tick
	// alongside towersScratch.
	spatial *spatialGrid
	// steerScratch is decide's reusable buffers, and steerScratches one
	// per core for parallel steering.
	steerScratch   steerScratch
	steerScratches []steerScratch
	// routeScratch is the route search's reusable state for the serial
	// pass, and routeScratches one per chunk of planRoutes.
	routeScratch   routeScratch
	routeScratches []routeScratch
	// sledScratch is the snowmobile route search's (sled_route.go).
	sledScratch routeScratch
	// boarders are guests who got on a chair and are waiting for their
	// post-ride plan, planned together (planBoarders).
	boarders   []boarder
	boardPlans []ai.Plan
	// planQuiet marks, by index in OnMountain, the guests whose plan
	// step neither finished nor became impossible this step (checkPlans).
	planQuiet []bool
	// skiJobs holds this step's skiers waiting on steering; skiBatch
	// points at it while tickGuests is collecting them (ski_parallel.go).
	skiJobs   []skiJob
	skiBatch  *[]skiJob
	routeJobs []routeJob // planRoutes' scratch
	// arriving is the lot a guest getting out of their car is planning
	// from (spawnGuestAt), for planFrom; 0 otherwise.
	arriving   uint64
	groupStats GroupStats
	// planGen numbers group leaders' plans (groups.go).
	planGen uint32

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
	groomable map[[2]int]bool
	// groomCovered is the groomed trails some active cat has passes on,
	// and groomCats how many cats are active; rebuilt with sections, for
	// the "can't be groomed" hints.
	groomCovered map[uint64]bool
	groomCats    int
	catGroomed   map[uint64]map[[2]int]bool

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
		Pathfinder:     NewPathfinder(w),
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
	sim.refreshTrailConditions()
	w.UpdateLineLayouts()
	sim.closedForDay = sim.ClosedForDay()
	w.ClosedForDay = sim.closedForDay
	sim.yesterday = sim.Weather.Advance(sim.DateAt(w.SimTime))
	sim.tomorrow = sim.GameForecast(1)[0]
	for _, a := range w.OnMountain {
		if !a.Plan.Done() {
			sim.onPlanStepStart(a)
		}
	}
	sim.regroup()
	sim.logSetupHints()
	return sim
}

// maxSubstepSec caps the sim-time delta passed to any per-tick handler:
// small enough for the L1 controller's arrival check, heading-rate cap,
// and Euler physics integration, large enough that a crowd can run fast.
// It was 1/30 s until 2026-10-09. A fifth of a second cut the cost of a
// busy hour by about three (Crowd Scale), and an ordinary day on the
// Boreal Goals Test came out the same: 3.25★ against 3.23★, 226 falls
// against 251, 3.3 rides a guest against 3.2. The user accepted some
// difference in falls and the odd skier clipping a tree.
const maxSubstepSec = 1.0 / 5.0

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
	// Bring the cached trunk hazards up to date with any tree edits
	// since the last frame, before skiers read them (in parallel).
	s.World.Terrain.RefreshTrunkField()
	// And the open-water cells with the lakes' ice and any water edit.
	s.World.RefreshOpenWater()

	remaining := dt * s.TimeScale
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
	// of them can't afford the guests' substeps.
	s.tickTraffic(simulated)
}

// refillTowersScratch rebuilds s.towersScratch in place from the live
// lift list. Each per-lift TowerXZs() is cached on the Lift so this
// is a flat copy in steady state — no allocations after the first
// call per Lift.
func (s *Simulation) refillTowersScratch() {
	s.towersScratch = s.towersScratch[:0]
	for _, lift := range s.World.Lifts {
		s.towersScratch = append(s.towersScratch, lift.TowerXZs(s.World.Terrain)...)
		// Stations' legs and huts: steered round like towers.
		s.towersScratch = append(s.towersScratch, lift.StationXZs()...)
	}
}

// subTick is one indivisible simulation step. All per-handler dt's
// come from here so substepping in Tick is the single place that
// controls step size.
func (s *Simulation) subTick(dt float64) {
	// Rebucket every guest for the skier-avoidance queries, every step:
	// a frame at fast-forward holds dozens of steps, and a skier can
	// cross several buckets in that time. O(guests).
	s.spatial.rebuild(s.World.OnMountain)
	if p := s.World.Player; p != nil {
		s.spatial.insert(p) // the guests dodge the player too
	}
	s.SimTime += dt
	s.World.SimTime = s.SimTime
	s.Demand.maybePoll(s)
	// Hourly effects run before the rollover so the day's last hour
	// still sees that day's weather.
	s.tickHourly()
	s.maybeSampleHistory()
	s.tickDark(dt)
	s.tickResortClosed()
	s.tickLifts(dt)
	s.tickGuests(dt)
	s.tickSnowcats(dt)
	s.tickPatrollers(dt)
	if !edition.Demo { // no snowmaking in the demo
		s.tickSnowmaking(dt)
	}
	s.tickAvalanche(dt)
}

func (s *Simulation) tickLifts(dt float64) {
	w := s.World
	running := s.LiftsRunning()
	defer s.planBoarders(false)
	for _, lift := range w.Lifts {
		if lift.Cableless() {
			tickHeliLift(s, lift, dt)
			continue
		}
		loopLen := lift.LoopLength()
		if loopLen < 1 {
			continue
		}
		unloadAt := lift.UnloadProgress()
		// After dusk an unlit lift stops like at closing; its line goes
		// to find a lit one.
		running := running && (!w.Dark || lift.Lit)
		if !running && w.Dark && len(lift.Queue) > 0 && s.LiftsRunning() {
			ejectQueue(lift)
		}

		// Hold: too little snow at the lift's base (under LiftHoldDepth,
		// back from LiftResumeDepth) — drain chairs, don't board, then
		// stop.
		baseCell := lift.QueueCell()
		var baseDepth float32
		if w.Terrain.InBounds(baseCell[0], baseCell[1]) {
			baseDepth = w.Terrain.Cells[baseCell[0]][baseCell[1]].VisibleSnowDepth()
		}
		wasHeld, wasSnowHeld := lift.OnHold, lift.SnowHold
		lift.SnowHold = baseDepth < world.LiftHoldDepth || (wasSnowHeld && baseDepth < world.LiftResumeDepth)
		if lift.SnowHold != wasSnowHeld && lift.Open {
			s.logLiftHoldChanged(lift)
		}
		s.tickLiftWind(lift)
		lift.OnHold = lift.SnowHold || lift.WindHold
		if lift.OnHold && !wasHeld {
			// Transition to hold: eject queued guests so they can re-plan.
			ejectQueue(lift)
		}

		// Outside operating hours chairs stop once they've carried
		// everyone up, and nobody boards.
		passengers := lift.PassengerCount()
		// A test run runs empty: by hand even with the resort closed, or
		// before reopening after the wind.
		testing := lift.TestingNow(s.SimTime) && !lift.OnHold
		moving := (!lift.OnHold && lift.Open && (running || passengers > 0)) || (lift.OnHold && passengers > 0) ||
			(testing && (lift.Testing || running))
		for i := range lift.Chairs {
			chair := &lift.Chairs[i]
			prev := chair.Progress
			if moving {
				// Detachables' chairs slow in the stations (SpeedAt).
				metres := lift.SpeedAt(chair.Progress) * float32(dt)
				chair.Progress += metres / loopLen
				if lift.Type.Tow() {
					s.tickTowRiders(lift, chair, metres, dt)
				}
			}

			// At the top's unload point: unload passengers.
			if prev < unloadAt && chair.Progress >= unloadAt {
				if len(s.boarders) > 0 && chairOccupied(chair) {
					// A short lift: riders waiting for their plan get it now.
					s.planBoarders(true)
				}
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
					s.startUnloading(agent, lift, i, j)
					if agent.Party.Leader != nil {
						agent.Party.TopLift, agent.Party.TopSince = lift.ID, s.SimTime
					}
					if !agent.Player { // the player has no plan
						s.advancePlan(agent)
					}
					s.aimUnload(agent)
				}
			}

			// At the base's load point (progress wraps past 1.0): fill the
			// chair from the queue up to its capacity. Skip when on hold.
			if chair.Progress >= 1.0 {
				chair.Progress -= 1.0
				if !lift.OnHold && running && !testing {
					boarders := loadChair(lift, len(chair.Passengers))
					s.shuffleLine(lift)
					for j, agent := range boarders {
						if j >= len(chair.Passengers) {
							break
						}
						chair.Passengers[j] = agent
						agent.OnLiftID, agent.LiftSeat = lift.ID, j
						agent.Queued = false
						clipInToRide(agent)
						agent.UseService(lift.RideQuality())
						if !agent.Player {
							s.boarders = append(s.boarders, boarder{agent, lift, s.SimTime})
						}
					}
					s.familyOnChair(chair)
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
		case ai.ActUseService:
			if b := findBuildingByID(w, step.BldgID); b != nil {
				return b.UseDoor(step.Use)
			}
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

// countUse recounts each building's guests using and waiting for each
// pool (seats, counter) from guests mid-visit. beginVisit bumps InUse as
// guests get in, so a pool fills within a tick.
func countUse(w *world.World) {
	for _, b := range w.Buildings {
		b.InUse, b.Waiting = [world.PoolCount]int{}, [world.PoolCount]int{}
	}
	for _, a := range w.OnMountain {
		head := a.Plan.Head()
		if head.Kind != ai.ActUseService || (a.RestTimer <= 0 && !a.Visit.Waiting) {
			continue
		}
		b := findBuildingByID(w, head.BldgID)
		if b == nil {
			continue
		}
		if a.Visit.Waiting {
			b.Waiting[b.UsePool(head.Use)]++
		} else {
			b.InUse[b.UsePool(head.Use)]++
		}
	}
}

// serveLines lets guests waiting at a door in, first come first served,
// as seats and turns at the counter free up.
func (s *Simulation) serveLines() {
	w := s.World
	var waiting []*world.Guest
	for _, a := range w.OnMountain {
		if a.Visit.Waiting {
			waiting = append(waiting, a)
		}
	}
	if len(waiting) == 0 {
		return
	}
	sort.SliceStable(waiting, func(i, j int) bool { return waiting[i].Visit.WaitSince < waiting[j].Visit.WaitSince })
	type doorKey struct {
		b    uint64
		door int
	}
	places := map[doorKey]int{}
	for _, a := range waiting {
		head := a.Plan.Head()
		b := findBuildingByID(w, head.BldgID)
		if b == nil || head.Kind != ai.ActUseService {
			a.Visit.Waiting = false
			continue
		}
		if b.HasRoomFor(head.Use) {
			a.Visit.Waiting = false
			a.Visit.Waited = s.SimTime - a.Visit.WaitSince
			b.Waiting[b.UsePool(head.Use)]--
			s.beginVisit(a, b, head.Use)
			continue
		}
		// Still waiting: next place in their door's line.
		k := doorKey{b.ID, a.Visit.Door}
		a.Visit.Slot = places[k]
		places[k]++
	}
}

// doorAt is Visit.Door for a guest at pos lining up at b for service
// svc: 1 + the index of the nearest door onto it, 0 when b has none.
func doorAt(b *world.Building, svc world.Service, pos mgl32.Vec3) int {
	if !b.IsShell() || len(b.Doors) == 0 {
		return 0
	}
	step, _ := b.NearestServiceEntrance(svc, mgl32.Vec2{pos[0], pos[2]})
	for i, d := range b.Doors {
		if b.DoorStep(d) == step {
			return i + 1
		}
	}
	return 0
}

// beginVisit starts guest a using o at b: they take a seat or a turn at
// the counter, pay, and note what they'll score the visit on when it
// ends (fulfilOffer).
func (s *Simulation) beginVisit(a *world.Guest, b *world.Building, o ai.Offer) {
	w := s.World
	a.RestTimer = world.OfferDuration(o)
	a.Speed = 0
	a.TargetID = 0
	// In through the door: they come back out on its step.
	if d := a.Visit.Door; d > 0 && d <= len(b.Doors) {
		step := b.DoorStep(b.Doors[d-1])
		a.Pos = mgl32.Vec3{step[0], w.Terrain.InterpolatedSurfaceElevationAt(step[0], step[1]), step[1]}
	}
	b.InUse[b.UsePool(o)]++
	a.Visit.Paid, a.Visit.Ratio = 0, 0
	if price := b.UsePrice(o); price > 0 {
		a.Visit.Ratio = b.PriceRatio(o, a.Traits.DailyBudget)
		a.Visit.Paid = price
		w.Cash += price
		w.History.RecordRevenue(useRevenue(b, o), price)
		a.RemainingBudget -= float32(price)
	}
}

// tickWaitingForService holds a guest in the line at a building's door
// while their patience drains: they shuffle up to their place in it
// (Building.DoorLineSlot) and face the guest ahead. serveLines lets them
// in.
func (s *Simulation) tickWaitingForService(a *world.Guest, dt float64) {
	a.Speed = 0
	a.Patience = max(a.Patience-float32(dt*patienceDrainPerSecServiceLine), 0)
	b := findBuildingByID(s.World, a.Plan.Head().BldgID)
	if b == nil || a.Visit.Door <= 0 || a.Visit.Door > len(b.Doors) {
		return
	}
	door := a.Visit.Door - 1
	slot, face := b.DoorLineSlot(door, a.Visit.Slot, b.DoorLineSide(s.World.Terrain, door))
	to := slot.Sub(mgl32.Vec2{a.Pos[0], a.Pos[2]})
	if d := to.Len(); d > doorLineArrive {
		step := min(float32(doorLineShuffle*dt), d)
		to = to.Mul(1 / d)
		a.Pos[0] += to[0] * step
		a.Pos[2] += to[1] * step
		a.Pos[1] = s.World.Terrain.InterpolatedSurfaceElevationAt(a.Pos[0], a.Pos[2])
		a.Heading = float32(math.Atan2(float64(to[0]), float64(to[1])))
		a.Speed = float32(doorLineShuffle)
		return
	}
	if face != (mgl32.Vec2{}) {
		a.Heading = float32(math.Atan2(float64(face[0]), float64(face[1])))
	}
}

// Moving up in a line at a door: guests walk to their place at
// doorLineShuffle m/s, and stop within doorLineArrive of it.
const (
	doorLineShuffle = 1.0
	doorLineArrive  = 0.1
)

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
	countUse(w)
	s.serveLines()
	s.tickVisitNeeds(dt)
	// Pre-pass: correct stale SkisOn state introduced between ticks (e.g.
	// by heatwave or other external snow mutations applied outside the
	// substep loop).
	for _, agent := range w.OnMountain {
		if agent.SkisOn && !agent.Queued && agent.OnLiftID == 0 && noSnowUnderfoot(w.Terrain, agent.Pos[0], agent.Pos[2]) {
			agent.SkisOn = false
			agent.SkiTransitionTimer = 0
		}
	}
	s.tickSchools()
	s.kidsNotAlone()
	s.planRoutes()
	s.checkPlans()
	s.skiJobs = s.skiJobs[:0]
	s.skiBatch = &s.skiJobs
	for i, agent := range w.OnMountain {
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
		// Signed up for a lesson, or back from one waiting to be
		// collected: at the meeting spot till a class (or a parent)
		// takes them, making no plans of their own meanwhile; a fall, a
		// ride or a rest they're in plays out first.
		lessonHold := agent.Lesson.SchoolID != 0 && !agent.Lesson.InClass
		if lessonHold && agent.RestTimer <= 0 && !agent.Fallen && agent.OnLiftID == 0 && !agent.Queued && agent.Unload.LiftID == 0 {
			s.tickLessonWait(agent, dt)
			continue
		}
		s.collectKids(agent)
		if c := agent.Class; c != nil && c.Instructor == agent && c.Ending && agent.Plan.GoalName != backToSchool &&
			agent.OnLiftID == 0 && !agent.Queued && agent.Unload.LiftID == 0 && !agent.Fallen {
			s.headBack(c) // off the lift, or up again: back to the school
		}
		if (i >= len(s.planQuiet) || !s.planQuiet[i]) && !lessonHold && !(agent.Class != nil && agent.Class.Instructor == agent && agent.Class.Ending) {
			s.tickPlanning(agent)
		}
		if agent.Removed {
			continue
		}
		// Check whether the agent needs to swap equipment. Only fires
		// when no other transition is active and not in a state that
		// already suspends locomotion.
		if agent.SkiTransitionTimer == 0 && agent.GearTimer == 0 && !agent.Fallen && agent.OnLiftID == 0 && !agent.Queued && agent.RestTimer == 0 {
			s.maybeStartSkiTransition(agent)
		}
		// In a lift line without skis (it started on bare ground or by a
		// door): put them on where they stand (Boreal Playtest: about 300
		// guests a day rode up with skis off).
		if agent.Queued && !agent.SkisOn && !agent.NeedsGear && agent.SkiTransitionTimer == 0 {
			agent.SkiTransitionTimer = -skiChangeTime(agent)
		}
		switch {
		case agent.Fallen:
			s.tickFallen(agent, dt)
		case agent.OnLiftID != 0:
			s.tickRiding(agent, dt)
		case agent.Queued && agent.SkiTransitionTimer != 0:
			s.tickSkiTransition(agent, dt)
		case agent.Queued:
			s.tickQueued(agent, dt)
		case agent.SkiTransitionTimer == 0 && s.gearErrand(agent, dt):
			// Leaving skis outside, or going back for them (gear.go).
		case agent.Visit.Waiting:
			s.tickWaitingForService(agent, dt)
		case agent.RestTimer > 0:
			s.tickResting(agent, dt)
		case agent.SkiTransitionTimer != 0:
			s.tickSkiTransition(agent, dt)
		case len(agent.Path) > 0 && agent.PathIdx < len(agent.Path) || s.repathToLine(agent):
			s.tickPath(agent, dt)
		case s.holdForGroup(agent):
			// Waiting at the top for the group, at their hold spot
			// (holdAt sets their speed: walking there, then 0).
		default:
			s.tickLocomote(agent, dt)
		}
		// Hard invariant: skis cannot be on while standing on bare ground.
		// Use an instant correction (no transition timer) because with high
		// TimeScale the 1-second transition completes within the same
		// wall-clock tick and the cycle can recur before the tick ends.
		// Not in a lift line or on a chair: they keep their skis on there.
		if agent.SkisOn && !agent.Queued && agent.OnLiftID == 0 && noSnowUnderfoot(s.World.Terrain, agent.Pos[0], agent.Pos[2]) {
			agent.SkisOn = false
			agent.SkiTransitionTimer = 0
		}
	}
	// Skiers collected above decide their steering together, then move
	// in the usual order (ski_parallel.go).
	s.skiBatch = nil
	s.decideSkiers(s.skiJobs, dt)
	for i := range s.skiJobs {
		j := &s.skiJobs[i]
		if j.a.Party.Leader != nil {
			s.groupStats.Steps++
			if j.off == onLine {
				s.groupStats.OnLine++
			} else {
				s.groupStats.Off[j.off]++
			}
		}
		if j.a.Removed {
			continue
		}
		switch {
		case j.walk:
			s.walkStep(j.a, j.goal, j.target, dt)
		case j.arrived:
			j.a.Pos = j.target
		default:
			s.applySkier(j.a, j.target, j.dist, j.perc, j.dec, dt)
		}
		if j.a.SkisOn && noSnowUnderfoot(s.World.Terrain, j.a.Pos[0], j.a.Pos[2]) {
			j.a.SkisOn = false
			j.a.SkiTransitionTimer = 0
		}
	}
	s.tickPlayer(dt)
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
	g.SkisOn = false // carried from the car; on at the snow
	g.Patience = 1.0
	g.Energy = 1.0
	if HourOfDay(s.SimTime) >= world.NightFrom && w.NightSession() {
		g.Energy = eveningEnergy // after a day's work
	}
	g.Hunger = 0.5 + rng.Global().Float32()*0.5
	g.Thirst = 0.5 + rng.Global().Float32()*0.5
	g.HasSeasonPass = hasValidPass(g, s.SimTime)
	g.RollVisitNeeds(rng.Global())
	if g.Child {
		// A kid's lesson: likely when a parent skis a level above them.
		share := world.KidLessonShare
		for _, m := range w.GroupOf(g) {
			if !m.Child && world.SkillTier(m.Traits.Skill) > world.SkillTier(g.Traits.Skill) {
				share = world.KidLessonBehind
			}
		}
		g.WantsLesson = rng.Global().Float32() < share
	}
	noSchool := false
	if g.WantsLesson && !anySchool(w) {
		// Came for a lesson and there's no school (silently in an
		// edition without one).
		g.WantsLesson, noSchool = false, world.ServiceSkiSchool.Available()
	}
	rentedInTown := false
	if g.NeedsGear && !anyRentals(w) {
		// No rental shop here: they rented in town on the way, on their
		// own money, and the resort missed the sale.
		g.NeedsGear, rentedInTown = false, true
	}
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
	if g.NeedsGear {
		// They came knowing they'd rent, and brought the money for it.
		g.RemainingBudget += world.DefaultRentalPrice
	}
	g.Removed = false

	w.OnMountain = append(w.OnMountain, g)
	s.joinParty(g)
	s.arriving = lot.ID
	s.replan(g)
	s.arriving = 0
	if rentedInTown {
		s.applyEvent(g, ai.ThoughtRentedInTown)
	}
	if noSchool {
		s.applyEvent(g, ai.ThoughtNoSchool)
	}
	head := g.Plan.Head()
	bad := g.Plan.Done() ||
		(head.Kind == ai.ActWalkToLift && len(g.Path) == 0)
	if bad {
		// Roll back: pop from OnMountain, reset to AtHome.
		leaveParty(g)
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

// applyEvent is the one place an event happens to a guest: it records
// the thought reporting it, on the guest and in the day's tally, and
// counts it as a moment.
func (s *Simulation) applyEvent(a *world.Guest, kind ai.ThoughtKind, context ...uint64) {
	if a.Player || a.InstructorOf != 0 {
		return // the player and staff have no thoughts for the resort to count
	}
	if a.Child && ai.Effects[kind].Class == ai.Highlight {
		defer s.kidsLovedIt(a) // their parents' day too
	}
	s.recordThought(a, kind, context...)
	var ctx uint64
	if len(context) > 0 {
		ctx = context[0]
	}
	a.AddMoment(kind, ctx)
}

// setCondition turns a condition on or off for a guest. Turning on adds
// its thought once, counted for the day, and starts its grace period:
// held past momentGrace, it counts as a moment (tickMood).
func (s *Simulation) setCondition(a *world.Guest, kind ai.ThoughtKind, on bool, context ...uint64) {
	if on == a.Conditions.Has(kind) {
		return
	}
	a.Conditions.Set(kind, on)
	if on {
		s.recordThought(a, kind, context...)
		a.Held = append(a.Held, world.HeldCondition{Kind: kind, Since: s.SimTime})
		return
	}
	for i := range a.Held {
		if a.Held[i].Kind == kind {
			a.Held = append(a.Held[:i], a.Held[i+1:]...)
			break
		}
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
		if k == ai.ThoughtNothingForMe && on && !a.Conditions.Has(k) {
			s.liftTooHard(a)
		}
		s.setCondition(a, k, on)
	}
}

// liftTooHard is a novice's complaint when nothing they'd ride is
// running but a lift with a green run off it is: the nearest such lift
// looks too difficult, since its terrain (World.TerrainForLift) isn't all green.
func (s *Simulation) liftTooHard(a *world.Guest) {
	if !ai.Novice(a.Traits.Skill) {
		return
	}
	var near *world.Lift
	best := float32(math.Inf(1))
	for _, l := range s.World.Lifts {
		if !l.InService(s.World) || !s.World.ServicesForLift(l.ID).Has(world.DiffGreen) {
			continue
		}
		if d := (mgl32.Vec2{l.Base[0] - a.Pos[0], l.Base[1] - a.Pos[2]}).Len(); d < best {
			near, best = l, d
		}
	}
	if near != nil {
		s.applyEvent(a, ai.ThoughtLiftTooHard, near.ID)
	}
}

// plannerConditions are the conditions only the planner reports.
var plannerConditions = [...]ai.ThoughtKind{ai.ThoughtNeedsLodge, ai.ThoughtLiftsClosed, ai.ThoughtNothingForMe, ai.ThoughtNoTicketWindow, ai.ThoughtLinesFull, ai.ThoughtPricesTooHigh, ai.ThoughtNoRentals, ai.ThoughtNoLounge}

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
		w.SnowmakingToday, w.LightingToday = 0, 0
		costs[world.CostInterest] = s.applyCredit(dayIdx)

		// The rating is the average of the last few days' average
		// stars (RatingDays); a day nobody left keeps the last one.
		if _, today := w.History.DayRating(); today {
			w.Rating, _ = w.History.RecentRating()
		}
		sample := world.DailySample{
			Day:              s.DateAt(float64(dayIdx) * secondsPerSimDay),
			GuestsOnMountain: len(w.OnMountain),
			ArrivalsToday:    w.History.ArrivalsToday,
			DeparturesToday:  w.History.DeparturesToday,
			TurnedAway:       w.History.TurnedAwayToday,
			Cash:             w.Cash,
			Revenue:          w.History.RevenueToday,
			Costs:            costs.Total(),
			RevenueByKind:    w.History.RevenueByKindToday,
			CostsByKind:      costs,
			Open:             wasOpen,
			Rating:           w.Rating,
			ThoughtCounts:    w.History.ThoughtCountsToday,
			DepartReasons:    w.History.DepartReasonsToday,
			Falls:            len(w.History.FallsToday),
			Reviews:          w.History.ReviewsToday,
		}
		why := w.History.TopWhy()
		w.History.Push(sample)
		s.logDaySummary(sample, why)
		s.logRecordDay(sample)
		s.checkGoals(dayIdx, sample)
		s.lastSampledDay++
		s.lastSaid = ai.ThoughtNone
		clear(s.longLinesSaid)

		// Play the rest of the ended game day's real days, then advance
		// the weather to the new day and apply its terrain effects.
		newDay := s.DateAt(float64(s.lastSampledDay) * secondsPerSimDay)
		s.playOffscreenDays(sample.Day, newDay)
		s.yesterday = s.Weather.Today()
		dw := s.Weather.Advance(newDay)
		s.tomorrow = gameForecast(s.Weather, newDay, 1)[0]
		s.applyDailyWeather(dw)
		s.logFreshSnow(dw)
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

// windSlabMS is the day's mean wind at the base, m/s, from which a dry
// day packs the surface into wind slab (weatherEventWind).
const windSlabMS = 7.5

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
		case world.KindBase, world.KindMachine:
			return current // dense enough not to ice up from cold alone
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
		case world.KindFrozenGranular, world.KindBase, world.KindMachine:
			return world.KindCorn // freeze-thaw granulation; warm day activates base surface
		case world.KindCorn:
			return world.KindSlush // over-saturates with continued warmth
		default: // Cement, WindSlab, Crust, Boilerplate
			return world.KindSlush
		}
	case weatherEventWind:
		switch current {
		case world.KindBoilerplate, world.KindBase, world.KindMachine:
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

// TriggerAvalanche releases the most dangerous slide path (the biggest
// among equals). Used by the debug console cheat and a testbed.
func (s *Simulation) TriggerAvalanche() {
	s.ensureSlidePaths()
	var best *world.SlidePath
	for _, p := range s.World.SlidePaths {
		if best == nil || p.Danger > best.Danger || (p.Danger == best.Danger && p.Load > best.Load) {
			best = p
		}
	}
	if best != nil {
		s.releasePath(best, false)
	}
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
	s.applyDayMelt(dw, s.DateAt(s.SimTime))
}

// applyDailyWeather runs all terrain snow effects for one day rollover:
// snowfall (push new layer), non-precipitation kind transitions, melt, and
// daily traffic decay.
func (s *Simulation) applyDailyWeather(dw DayWeather) {
	t := s.World.Terrain

	// Stochastic wind roll for Clear and Overcast days.
	windFired := (dw.State == WeatherClear || dw.State == WeatherOvercast) && dw.WindMS >= windSlabMS

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
		switch evt {
		case weatherEventWarmClear:
			t.ScaleMoguls(1 - mogulThawRound)
		case weatherEventRain:
			t.ScaleMoguls(1 - mogulRainRound)
		}
		// Melt runs hourly through the day (tickHourly).
	}

	// The day's snow and wind load the slide paths (slide_paths.go).
	s.loadSlidePaths(dw)

	// Lakes freeze, thicken, and thaw with the day's temperature.
	s.stepLakes(dw)

	// Age skier tracks once a day so their clocks never wrap (a snowy
	// day already did, burying them).
	if dw.State != WeatherLightSnow && dw.State != WeatherHeavySnow {
		t.Surface.AgeTracks(world.TrackClock(s.SimTime), 1)
	}

	// Decay skier traffic on all cells once per day (~4 day half-life),
	// and the snow settles a day's worth, faster on a day that thaws.
	warm := dw.TempHigh > 0
	for x := range t.Cells {
		for z := range t.Cells[x] {
			t.Cells[x][z].SkierTraffic *= 0.85
			t.Cells[x][z].SettleSnow(warm)
		}
	}

	t.MarkAllSnowDirty()
}

// pushSnowLayer adds a new storm layer on top of every terrain cell. If the
// new kind matches Top, Top grows in place. Otherwise the current Top merges
// into Base and a new Top begins. Grooming is diluted by snowfall depth,
// and gone (stamps and all) once it's buried.
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

			fresh := world.KindDensity(kind)
			if c.Top.Accumulation > 0 && c.Top.Kind == kind {
				// Same kind — grow Top in place.
				c.Top.AddSnow(accumSWE, fresh)
			} else {
				// Different kind: fold current Top into Base, start new Top.
				c.FoldTopIntoBase()
				c.Top = world.SnowLayer{Kind: kind, Accumulation: accumSWE, Density: fresh}
			}

			t.Ungroom(x, z, 1-burialFactor)
			// Fresh snow buries the old tracks: untracked again.
			c.SkierTraffic *= 1 - burialFactor
		}
	}
	// What steep ground can't hold sluffs down to where it can.
	t.ShedSnow()

	// Bury skier tracks. 2 cm SWE fully covers any track.
	t.Surface.AgeTracks(world.TrackClock(s.SimTime), 1-burialFactor)
	t.ScaleMoguls(1 - clamp01(accumSWE/mogulFillSWE))
}

// Moguls are deeper than tracks, so they take more snow to fill: a day's
// snowfall fills them in proportion to its water, all the way at
// mogulFillSWE (about a metre of new snow).
const mogulFillSWE = float32(0.1)

// A thaw rounds moguls off: a warm clear day by mogulThawRound, a day of
// rain by mogulRainRound. A hard freeze leaves them as they are; the
// surface turns icy through its snow kind.
const (
	mogulThawRound = float32(0.1)
	mogulRainRound = float32(0.2)
)

// applyKindTransition modifies the top layer's Kind on every terrain cell
// according to the given non-precipitation weather event. A surface that
// changes kind is no longer the corduroy the cat left: a hard freeze
// keeps a trace of it in the crust (crustGroomKeep), anything else
// (wind slab, wet snow) wipes it.
func (s *Simulation) applyKindTransition(evt weatherEvent) {
	t := s.World.Terrain
	for x := range t.Cells {
		for z := range t.Cells[x] {
			c := &t.Cells[x][z]
			top := c.TopLayer()
			if top == nil {
				continue
			}
			next := kindTransition(top.Kind, evt)
			if next == top.Kind {
				continue
			}
			top.SetKind(next)
			keep := float32(0)
			if next == world.KindCrust {
				keep = crustGroomKeep
			}
			t.Ungroom(x, z, keep)
		}
	}
}

// crustGroomKeep is the grooming left when fresh corduroy freezes to
// crust overnight: frozen ridges, mostly gone as corduroy.
const crustGroomKeep = float32(0.25)

// tickHourly runs the effects that step once per clock hour: snowmelt
// from that hour's air temperature and sun (snow days don't melt), then
// the trails' conditions.
func (s *Simulation) tickHourly() {
	idx := int(s.SimTime / simSecondsPerHour)
	if s.lastHour < idx {
		// Lift lines fold before lots and buildings built since.
		s.World.UpdateLineLayouts()
		defer s.refreshTrailConditions()
		defer s.avalancheHour()
		defer s.logGuestsSay()
		defer s.logLongLines()
		defer s.logRescues()
		defer s.logCutOff()
		defer s.logSetupHints()
	}
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
func (s *Simulation) applyDayMelt(dw DayWeather, date time.Time) {
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
		t.MarkAllSnowDirty()
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
// exhausted Base is promoted to Top (clearing moguls), then the promoted
// layer melts if budget remains. Grooming survives on the promoted base:
// the cat packed through the thin top into it. It goes with the last of
// the snow.
func meltCell(c *world.Cell, melt float32) {
	if c.Top.Accumulation > 0 {
		if melt >= c.Top.Accumulation {
			melt -= c.Top.Accumulation
			c.Top = world.SnowLayer{}
			c.MogulSize = 0
			// Promote Base to a KindBase Top so the invariant holds.
			if c.Base > 0 {
				c.Top = world.SnowLayer{Kind: world.KindBase, Accumulation: c.Base, Density: c.BaseEffDensity()}
				c.Base, c.BaseDensity = 0, 0
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
	if c.Top.Accumulation == 0 {
		c.Grooming = 0
	}
}

// =============================================================================
// Planning layer — drives target / queue / removal off the stored ai.Plan
// =============================================================================

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
			a.Plan.Step++ // the head's done
			s.replan(a)
			return
		}
		// Step boundaries are the safe points to drop a plan for a need
		// that turned pressing mid-plan — except in a lift line.
		// A follower on the leader's plan leaves their needs to the
		// leader (goap.poolGroup).
		if head.Kind != ai.ActJoinQueue && a.Plan.GoalName != "GoHome" && (a.Party.Leader == nil || a.Party.Gen == 0) {
			// A copy, so only this step boundary pays for the snapshot
			// escaping into the goals' Weight (an interface call): snap
			// itself stays on the stack every tick.
			held := snap
			if goap.NeedPreempts(&held, s.World, &a.Plan) {
				a.Plan.Step++ // the head's done
				s.replan(a)
				return
			}
		}
		s.advancePlan(a)
		return
	}
	if !planActionPreconditionHolds(head, snap, s.World) {
		s.replan(a)
		return
	}
	if s.dueHome(a) {
		s.homeAtClosing(a)
		return
	}
	// Waiting for the group short of a lift line: check on the leader
	// (followPlan), so the wait's limit applies.
	if a.Party.WaitSince != 0 && head.Kind == ai.ActJoinQueue && !a.Queued {
		s.replan(a)
		return
	}
	if descentStuck(a, head, s.SimTime) {
		// A descent that has gone on far too long: for a follower, the
		// leader's plan didn't fit where they were; alone, they're lost.
		// Their own, afresh.
		a.AtTrailEnd = 0
		if a.Party.Leader != nil {
			s.planAlone(a)
		} else {
			a.Run.Start = s.SimTime
			s.replan(a)
		}
	}
}

// descentStuck reports whether a's descent step has gone on longer than
// any run should (followStuckSec).
func descentStuck(a *world.Guest, head ai.PlanAction, now float64) bool {
	return isDescentKind(head.Kind) && now-a.Run.Start > followStuckSec
}

// dueHome reports whether a should give up their step and go home now:
// the lifts have closed, their plan doesn't end in going home, and
// they're not in a line, indoors, resting, down, or off for après. A
// guest skiing a run finishes it and goes home by the trails from its
// end (a run gone on too long is descentStuck's).
func (s *Simulation) dueHome(a *world.Guest) bool {
	if !s.World.ClosedForDay || endsInDeparture(a.Plan) || a.Plan.Done() {
		return false
	}
	switch k := a.Plan.Head().Kind; {
	case k == ai.ActJoinQueue, k == ai.ActUseService, isDescentKind(k):
		return false
	}
	return !a.Queued && a.OnLiftID == 0 && !a.Fallen && a.OnPatrollerID == 0 &&
		a.RestTimer == 0 && !a.Visit.Waiting && a.Apres == 0
}

// checkPlans marks, across cores, the guests tickPlanning would leave
// alone this step: a plan step under way that hasn't finished and can
// still be done. The check reads the world as the step began; the
// serial pass runs tickPlanning, which checks again, only for the rest.
// Checking every guest's plan every step was a tenth of a busy step.
func (s *Simulation) checkPlans() {
	w := s.World
	n := len(w.OnMountain)
	s.planQuiet = append(s.planQuiet[:0], make([]bool, n)...)
	chunks := max(min(workers.size()*2, n/plansPerChunk), 1)
	workers.run(chunks, func(c int) {
		for i := n * c / chunks; i < n*(c+1)/chunks; i++ {
			a := w.OnMountain[i]
			if a.Removed || a.Plan.Done() {
				continue
			}
			snap := goap.Extract(a, w)
			head := a.Plan.Head()
			// Never quiet: a guest waiting for their group (the wait
			// has a limit), on a descent gone on too long, or due home
			// at closing (Boreal Playtest: guests stuck mid-step held
			// their cars in the lot overnight).
			s.planQuiet[i] = !planActionComplete(head, a, snap) && planActionPreconditionHolds(head, snap, w) &&
				a.Party.WaitSince == 0 && !descentStuck(a, head, s.SimTime) && !s.dueHome(a)
		}
	})
}

// plansPerChunk is the fewest guests a chunk of plan checks is cut into.
const plansPerChunk = 32

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
	case ai.ActSkiToLift, ai.ActSkiToService, ai.ActSkiToParking, ai.ActSkiTrail:
		return true
	}
	return false
}

// replan generates a fresh plan and starts its head step. Called at
// spawn, when the plan exhausts, and when a precondition breaks.
func (s *Simulation) replan(a *world.Guest) {
	a.AtTrailEnd = 0 // clear any stale junction anchor before re-planning
	if s.followPlan(a) {
		return
	}
	s.planAlone(a)
}

// planAlone is replan for a guest making their own plan: alone, leading
// a group (shared with them), or a follower off the leader's plans.
func (s *Simulation) planAlone(a *world.Guest) {
	done := min(a.Plan.Step, len(a.Plan.Steps))
	a.Plan = s.Planner.StoredPlanFor(a, s.World)
	a.Party.PlanAt = s.SimTime
	if a.Party.Leader != nil {
		a.Party.Gen = 0 // a follower's own plan, off the leader's
		s.groupStats.Own++
	}
	from := s.planFrom(a)
	s.setBlocked(a, a.Plan.Blocked)
	if a.Plan.GoalName == (goap.GoHome{}).Name() {
		s.setDepartReason(a, s.departReasonFor(a))
		defer s.kidsHomeWith(a)
	}
	if !a.Plan.Done() {
		s.share(a, from, a.Plan, done)
		s.onPlanStepStart(a)
		return
	}
	// Planner returned nothing — guest is stranded (e.g. all lifts held,
	// not at a lift top). Send them home rather than leaving them frozen,
	// and their group with them: never share the empty plan, which a
	// follower would take up and wait on (Boreal Playtest).
	if !a.Removed {
		s.directHomePlan(a)
		if !a.Plan.Done() {
			s.share(a, from, a.Plan, done)
		}
	}
}

// replanOnBoard builds a post-ride plan while the agent is still on the
// chair. StoredPlanForLookahead plans from a simulated "just unloaded"
// snapshot; we then prepend the in-flight RideLift so advancePlan at
// unload steps past it and lands on the first post-ride action.
func (s *Simulation) replanOnBoard(agent *world.Guest, lift *world.Lift) {
	s.applyBoardPlan(agent, lift, s.Planner.StoredPlanForLookahead(agent, lift.ID, s.World))
}

// chairOccupied reports whether anyone is on chair.
func chairOccupied(chair *world.Chair) bool {
	for _, p := range chair.Passengers {
		if p != nil {
			return true
		}
	}
	return false
}

// boarder is a guest who got on a chair of lift at time at.
type boarder struct {
	a    *world.Guest
	lift *world.Lift
	at   float64
}

// Boarders wait for their post-ride plan until boardBatch have boarded
// or the first has ridden boardWaitSec: about one guest boards a step in
// a crowd, too few to share out, and a rider needs the plan only at the
// top, a minute or more away.
const (
	boardBatch   = 16
	boardWaitSec = 5.0
)

// planBoarders plans the waiting boarders' post-ride plans (as
// replanOnBoard does) across cores, then applies them in boarding order;
// with now false, only once enough are waiting (boardBatch,
// boardWaitSec). The planner only reads the world; a plan made at each
// boarding cost a busy hour about a second.
func (s *Simulation) planBoarders(now bool) {
	n := len(s.boarders)
	if n == 0 || !now && n < boardBatch && s.SimTime-s.boarders[0].at < boardWaitSec {
		return
	}
	s.boardPlans = append(s.boardPlans[:0], make([]ai.Plan, n)...)
	chunks := max(min(workers.size()*2, n/boardersPerChunk), 1)
	workers.run(chunks, func(c int) {
		for i := n * c / chunks; i < n*(c+1)/chunks; i++ {
			b := s.boarders[i]
			if b.a.Party.Leader != nil {
				continue // followers copy the leader's plan, below
			}
			s.boardPlans[i] = s.Planner.StoredPlanForLookahead(b.a, b.lift.ID, s.World)
		}
	})
	// Leaders and guests alone first, so followers find the plans
	// their leaders made on the same chair, or one just ahead.
	for i, b := range s.boarders {
		if b.a.OnLiftID == b.lift.ID && !b.a.Removed && b.a.Party.Leader == nil {
			s.applyBoardPlan(b.a, b.lift, s.boardPlans[i])
		}
	}
	for _, b := range s.boarders {
		if b.a.OnLiftID == b.lift.ID && !b.a.Removed && b.a.Party.Leader != nil && !s.boardFollower(b.a, b.lift) {
			s.groupStats.Own++
			s.groupStats.OwnWhy[ownBoard]++
			s.replanOnBoard(b.a, b.lift)
		}
	}
	s.boarders = s.boarders[:0]
}

// boardersPerChunk is the fewest boarders a chunk of planning is cut into.
const boardersPerChunk = 2

// applyBoardPlan gives a guest on a chair of lift their post-ride plan.
func (s *Simulation) applyBoardPlan(agent *world.Guest, lift *world.Lift, lookahead ai.Plan) {
	s.setBlocked(agent, lookahead.Blocked)
	if lookahead.Done() {
		return
	}
	done := boardedDone(&agent.Plan, lift.ID)
	rideLiftStep := ai.PlanAction{Kind: ai.ActRideLift, LiftID: lift.ID}
	lookahead.Steps = append([]ai.PlanAction{rideLiftStep}, lookahead.Steps...)
	agent.Plan = lookahead
	agent.Party.PlanAt = s.SimTime
	if agent.Party.Leader != nil {
		agent.Party.Gen = 0
	}
	s.share(agent, world.PlanFrom{LiftTop: lift.ID}, lookahead, done)
}

// advancePlan moves the cursor to the next step and starts it; if the
// cursor walks off the end, the plan is done and we re-plan instead.
func (s *Simulation) advancePlan(a *world.Guest) {
	if a.Party.Leader != nil && s.followStep(a) {
		return
	}
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
		a.Plan.Target = lift.BackOfQueueWorldPos(mgl32.Vec2{a.Pos[0], a.Pos[2]}, w.Terrain)
		startCell := [2]int{
			int(math.Floor(float64(a.Pos[0] / CellSize))),
			int(math.Floor(float64(a.Pos[2] / CellSize))),
		}
		if path := s.Pathfinder.FindPath(startCell, lift.BackOfQueueCell(mgl32.Vec2{a.Pos[0], a.Pos[2]})); path != nil {
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
		if !lift.InService(s.World) {
			a.Plan.Steps = nil
			return
		}
		// Closed for the day: lifts load no one. A guest about to queue
		// heads straight for the parking lot instead, skiing or walking
		// down, rather than riding up for one more run on the way out.
		// Before opening they queue and wait for the first chair.
		if s.ClosedForDay() {
			s.setDepartReason(a, ai.DepartClosing)
			s.homeAtClosing(a)
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
		wait := lift.LineWait()
		// A follower whose leader is already in this line joins them.
		if a.Patience >= 0.05 && wait > world.MaxLineWaitFor(len(a.RidenLifts) > 0) && !leaderInLine(a, lift) {
			s.applyEvent(a, ai.ThoughtLineTooLong, lift.ID)
			a.Patience = 0
			s.replan(a)
			return
		}
		if wait >= world.LongLineWait {
			s.applyEvent(a, ai.ThoughtLongLine, lift.ID)
		}
		if lift.JoinLine(a, s.SimTime) {
			w.RebuildRopes()
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
		a.Plan.Target = lift.BackOfQueueWorldPos(mgl32.Vec2{a.Pos[0], a.Pos[2]}, w.Terrain)
	case ai.ActSkiToService:
		s.startRun(a)
		b := findBuildingByID(w, step.BldgID)
		if b == nil {
			return
		}
		a.TargetID = b.ID
		a.Plan.Goal = ai.GoalNone
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
	case ai.ActWalkToParking:
		b := findBuildingByID(w, step.BldgID)
		if b == nil {
			return
		}
		a.TargetID = b.ID
		a.Plan.Goal = ai.GoalDepart
		a.Plan.GoalID = b.ID
		a.Plan.Target = parkingWorldPos(w, b)
		startCell := [2]int{
			int(math.Floor(float64(a.Pos[0] / CellSize))),
			int(math.Floor(float64(a.Pos[2] / CellSize))),
		}
		a.Path = s.Pathfinder.FindPath(startCell, b.DoorCell())
		a.PathIdx = 0
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
			a.Plan.Target = lift.BackOfQueueWorldPos(mgl32.Vec2{a.Pos[0], a.Pos[2]}, w.Terrain)
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
			// Trail-to-trail: steer for where the trail skied meets
			// the next one.
			if t := w.FindTrail(step.TrailID); t != nil {
				c, _ := world.TrailJunction(w.FindTrail(step.Via), t, mgl32.Vec2{a.Pos[0], a.Pos[2]})
				y := w.Terrain.InterpolatedSurfaceElevationAt(c[0], c[1])
				a.Plan.Target = mgl32.Vec3{c[0], y, c[1]}
				a.Plan.Goal = ai.GoalNone
				a.Plan.GoalID = step.TrailID
				a.TargetID = 0
			}
		}
	case ai.ActWalkToService:
		b := findBuildingByID(w, step.BldgID)
		if b == nil {
			return
		}
		a.TargetID = b.ID
		a.Plan.Goal = ai.GoalNone
		a.Plan.GoalID = b.ID
		svc := visitService(w, a)
		a.Plan.Target = entranceWorldPos(w, b, a.Pos, svc)
		startCell := [2]int{
			int(math.Floor(float64(a.Pos[0] / CellSize))),
			int(math.Floor(float64(a.Pos[2] / CellSize))),
		}
		_, door := b.NearestServiceEntrance(svc, mgl32.Vec2{a.Pos[0], a.Pos[2]})
		a.Path = s.Pathfinder.FindPath(startCell, door)
		a.PathIdx = 0
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
		daysToClose := float64(world.GameDayIndex(closeDate) - world.GameDayIndex(now) + 1)
		if daysToClose < 1 {
			daysToClose = 1
		}
		a.SeasonPassExpiry = s.SimTime + daysToClose*secondsPerSimDay
		a.HasSeasonPass = true
		_ = b // building exists — purchase confirmed

	case ai.ActUseService:
		b := findBuildingByID(w, step.BldgID)
		if b == nil {
			return
		}
		a.Visit = world.Visit{}
		if step.Use == ai.OfferRentals && !a.NeedsGear {
			return // in the group that's renting, with skis of their own
		}
		if step.Use == ai.OfferLesson && (!a.WantsLesson || a.Lesson.SchoolID != 0) {
			return // in a group with someone signing up, or signed up already
		}
		s.stashOnEntry(a)
		if !b.HasRoomFor(step.Use) {
			// Full: line up at the door (serveLines lets them in).
			a.Visit.Waiting, a.Visit.WaitSince = true, s.SimTime
			a.Visit.Door = doorAt(b, b.UseDoor(step.Use), a.Pos)
			a.Speed, a.TargetID = 0, 0
			b.Waiting[b.UsePool(step.Use)]++
			return
		}
		s.beginVisit(a, b, step.Use)
	case ai.ActDepart:
		// Capture session stats (LastStars, LifetimeVisits, LastVisit,
		// VisitsThisSeason) on the persistent Guest record before the
		// reaper clears sim scratch fields, then flip Removed so
		// reapDeparted will splice this Guest out of OnMountain and into
		// their car.
		// Set the departure aside until the car drives off the map
		// (finishDeparture); a guest with no car leaves now.
		s.setDepartReason(a, s.departReasonFor(a))
		a.Leaving = world.Leaving{Pending: true, Review: w.FamilyReview(a), Reason: a.DepartReason}
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
	s.Demand.recordDeparture(s.World, g, l.Review.Stars, s.DateAt(s.SimTime))
	s.World.History.RecordDeparture(l.Review, l.Reason)
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
	case has(ai.ThoughtNoRentals):
		return ai.DepartNoRentals
	case has(ai.ThoughtNoTicketWindow):
		return ai.DepartNoTicket
	case has(ai.ThoughtLiftsClosed):
		return ai.DepartLiftsClosed
	case has(ai.ThoughtNothingForMe):
		return ai.DepartNothingToSki
	case has(ai.ThoughtLinesFull):
		return ai.DepartLines
	case has(ai.ThoughtBored), has(ai.ThoughtNotMySkiing):
		return ai.DepartBored
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
	s.kidsHomeWith(a)
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
	case ai.ActSkiToService:
		return snap.AtService == step.BldgID
	case ai.ActSkiToParking, ai.ActWalkToParking:
		return snap.AtParking == step.BldgID
	case ai.ActWalkToTicketOffice:
		return snap.AtTicketOffice == step.BldgID
	case ai.ActWalkToService:
		return snap.AtService == step.BldgID
	case ai.ActBuySeasonPass, ai.ActBuyDayTicket:
		return true // executed atomically in onPlanStepStart
	case ai.ActUseService:
		return a.RestTimer <= 0 && !a.Visit.Waiting
	case ai.ActDepart:
		// Terminal — the Removed flag is the real signal; this is
		// queried only when the agent hasn't been reaped yet.
		return false
	case ai.ActSkiTrail:
		if step.LiftID != 0 {
			return snap.AtLiftBase == step.LiftID
		}
		if step.BldgID != 0 {
			return snap.AtService == step.BldgID || snap.AtParking == step.BldgID
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
		if step.Kind == ai.ActRideLift && snap.Queued != step.LiftID && snap.OnLift != step.LiftID {
			// Riding needs them in its line or on it: a follower who
			// copied the leader's plan past the line stood waiting for
			// a chair for hours.
			return false
		}
		return l != nil && l.InService(w)
	case ai.ActSkiToService, ai.ActSkiToParking, ai.ActWalkToParking,
		ai.ActDepart, ai.ActWalkToTicketOffice, ai.ActWalkToService, ai.ActBuySeasonPass, ai.ActBuyDayTicket:
		b := findBuildingByID(w, step.BldgID)
		return b != nil && b.Usable()
	case ai.ActUseService:
		b := findBuildingByID(w, step.BldgID)
		return b != nil && b.OffersUse(step.Use)
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
		if head := a.Plan.Head(); head.Kind == ai.ActUseService {
			s.fulfilOffer(a, findBuildingByID(s.World, head.BldgID), head.Use)
		}
	}
}

// fulfilOffer is what using o at b does for guest a: the stats behind
// the needs it meets (world.OfferNeeds) fill, the building's quality
// counts toward their quality stars, and the visit's moments
// (Service Improvements, Scoreless Rating):
//
//   - the offer's own moment (a good meal, a drink, a rest)
//   - quality: a nice place, a shabby one, a packed one
//   - value: good value, overpriced
//   - the wait: a long one in the line at the door
func (s *Simulation) fulfilOffer(a *world.Guest, b *world.Building, o ai.Offer) {
	if o == ai.OfferWater {
		a.Thirst = 1 // free water: no moments, no quality, nothing paid
		return
	}
	switch o {
	case ai.OfferSeat:
		a.Patience = 1
		a.Energy = 1
		s.applyEvent(a, ai.ThoughtRested)
	case ai.OfferMeal:
		a.Hunger = 1
		a.Thirst = 1 // a meal comes with a drink
		s.applyEvent(a, ai.ThoughtGoodMeal)
	case ai.OfferDrink:
		a.Thirst = 1
		s.applyEvent(a, ai.ThoughtGoodDrink)
	case ai.OfferRentals:
		a.NeedsGear = false
		s.applyEvent(a, ai.ThoughtRentedGear)
	case ai.OfferApres:
		a.WantsApres, a.Apres = false, 0
		a.Thirst = 1
		s.applyEvent(a, ai.ThoughtGreatApres)
	case ai.OfferWarmUp:
		a.Chill = 0
		s.applyEvent(a, ai.ThoughtWarmedUp)
	case ai.OfferLesson:
		// Signed up: they wait at the meeting spot for a class (lessons.go).
		a.Lesson = world.Lesson{SchoolID: b.ID, Since: s.SimTime}
		s.applyEvent(a, ai.ThoughtSignedUp)
	}
	if b != nil {
		a.UseService(b.Quality)
		switch q := b.Quality; {
		case q >= 0.6:
			s.applyEvent(a, ai.ThoughtNicePlace, b.ID)
		case q <= 0.4:
			s.applyEvent(a, ai.ThoughtShabby, b.ID)
		}
		if b.Occupancy(o) >= 0.9 {
			s.applyEvent(a, ai.ThoughtPackedInside, b.ID)
		}
	}
	switch r := a.Visit.Ratio; {
	case r <= 0:
	case r <= world.GoodValueRatio:
		s.applyEvent(a, ai.ThoughtGoodValue)
	case r >= world.OverpricedRatio:
		s.applyEvent(a, ai.ThoughtOverpriced)
	}
	if a.Visit.Waited >= serviceLineThoughtSec {
		s.applyEvent(a, ai.ThoughtServiceLine)
	}
	a.Visit = world.Visit{}
}

// Rolled needs over the day (Service Improvements step 3).
const (
	// chillPerHourPerDegree is how fast a guest who feels the cold
	// (ColdSense 1) chills outdoors per clock hour per degree below
	// freezing: at −10 °C, half way to chilled through in an hour.
	chillPerHourPerDegree = float32(1.0 / 20)
	// warmPerHour is how fast being indoors (any visit) warms them.
	warmPerHour = float32(3)
	// apresFrom and apresFull are the clock hours après starts to
	// tempt and when it's as tempting as it gets before closing.
	apresFrom = 14.5
	apresFull = 16.0
)

// tickVisitNeeds moves the rolled needs along: chill builds outdoors on
// cold days for guests who feel the cold and goes indoors; the urge for
// après grows through the late afternoon, more after a good day, and is
// full once the lifts close.
func (s *Simulation) tickVisitNeeds(dt float64) {
	temp := s.TempNow()
	hours := float32(dt / world.SimSecondsPerHour)
	hour := float32(HourOfDay(s.SimTime))
	closed := s.ClosedForDay()
	for _, a := range s.World.OnMountain {
		if a.ColdSense > 0 {
			if a.RestTimer > 0 {
				a.Chill = max(a.Chill-warmPerHour*hours, 0)
			} else if temp < 0 {
				a.Chill = min(a.Chill+a.ColdSense*chillPerHourPerDegree*(-temp)*hours, 1)
			}
		}
		if a.WantsApres {
			switch {
			case closed && hour > apresFrom:
				a.Apres = 1
			default:
				ramp := clamp32((hour-apresFrom)/(apresFull-apresFrom), 0, 1)
				a.Apres = min(ramp*0.8*apresMood(a), 0.99)
			}
		}
	}
}

// apresMood is how much a good day adds to the urge for après: 0.5 on a
// day with no highlight, up to 1.5 with several.
func apresMood(a *world.Guest) float32 {
	var n uint16
	for _, m := range a.Moments {
		if ai.Effects[m.Kind].Class == ai.Highlight {
			n += m.N
		}
	}
	return clamp32(0.5+0.25*float32(n), 0.5, 1.5)
}

// anyRentals reports whether any building rents skis.
func anyRentals(w *world.World) bool {
	if !world.ServiceRentals.Available() {
		return false
	}
	for _, b := range w.Buildings {
		if b.Type == world.BuildingLodge && b.OffersUse(ai.OfferRentals) {
			return true
		}
	}
	return false
}

// homeAtClosing sends a guest home when the lifts close, unless they
// want après and there's a bar to go to: then they replan, and the après
// need (urgency 1 once closed) takes them there first.
func (s *Simulation) homeAtClosing(a *world.Guest) {
	if a.WantsApres {
		for _, b := range s.World.Buildings {
			if b.Type == world.BuildingLodge && b.OffersUse(ai.OfferApres) {
				a.Apres = 1
				a.Plan.Steps = nil
				return
			}
		}
	}
	s.directHomePlan(a)
}

// useRevenue is the revenue line using o at b falls under: meals and the
// food court's drinks are food, a bar's drinks and après are the bar's,
// rentals are rentals.
func useRevenue(b *world.Building, o ai.Offer) world.RevenueKind {
	switch {
	case o == ai.OfferRentals:
		return world.RevenueRentals
	case o == ai.OfferLesson:
		return world.RevenueLessons
	case o == ai.OfferApres, o == ai.OfferDrink && b.DrinkService() == world.ServiceBar:
		return world.RevenueBar
	}
	return world.RevenueFood
}

// reapDeparted removes agents flagged by ActDepart at the end of
// tickGuests. Defers removal out of the range loop so the slice header
// doesn't shift mid-iteration.
func (s *Simulation) reapDeparted() {
	w := s.World
	for i := len(w.OnMountain) - 1; i >= 0; i-- {
		if g := w.OnMountain[i]; g.Removed {
			leaveParty(g)
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
			return liftApproachWorldPos(w, l, a), true
		}
	case ai.ActSkiToService, ai.ActSkiToParking, ai.ActWalkToParking, ai.ActWalkToTicketOffice, ai.ActWalkToService:
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
	// A waypoint in a station's hut or under a top terminal can't be
	// reached: go on to the next.
	for agent.PathIdx < len(agent.Path)-1 && inStationHazard(w, agent.Path[agent.PathIdx]) {
		agent.PathIdx++
	}
	target := agent.Path[agent.PathIdx]
	tx := (float32(target[0]) + 0.5) * CellSize
	tz := (float32(target[1]) + 0.5) * CellSize
	ty := w.Terrain.SurfaceElevationAt(target[0], target[1])
	// Through a footpath's cells, walk the path itself.
	if w.FootpathCell(target[0], target[1]) {
		if q, ok := w.SnapToFootpath(mgl32.Vec2{tx, tz}); ok {
			tx, tz = q[0], q[1]
			ty = w.Terrain.InterpolatedSurfaceElevationAt(tx, tz)
		}
	}
	targetPos := mgl32.Vec3{tx, ty, tz}
	// The last stretch to a lift line heads for the line itself, not on
	// through the cells under it (a crowd, or the station in the last).
	// Close to the line, or to the station in the way, the route's done
	// with: it ends where the line's back was when it was laid, which a
	// growing line has left behind, and its cells run straight through
	// the station. From here they head for the line itself, or round the
	// station first (lineApproach), and the arrival check can see them
	// (goap.Extract: no arrival on a route).
	if s.nearLine(agent, walkToLineNear) {
		agent.Path, agent.PathIdx = nil, 0
		s.tickLocomote(agent, dt)
		return
	}
	s.recordWalkTick(agent, targetPos)

	dir := targetPos.Sub(agent.Pos)
	dist := dir.Len()

	speed := s.walkSpeed(agent)
	step := float32(speed * dt)
	if dist <= step {
		agent.Pos = targetPos
		agent.PathIdx++
		s.repathToMovedLine(agent)
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
	if flat := (mgl32.Vec2{dir[0], dir[2]}); flat.Len() > 1e-4 {
		ahead := flat.Normalize()
		if d := s.walkClear(agent.Pos, ahead, flat.Len()); d != ahead {
			// Round a tower or station part: on the level, then back to
			// heading for the path point.
			dirNorm = mgl32.Vec3{d[0], 0, d[1]}
		}
	}
	next := agent.Pos.Add(dirNorm.Mul(step))
	to := s.World.Terrain.KeepOutOfWater(mgl32.Vec2{agent.Pos[0], agent.Pos[2]}, mgl32.Vec2{next[0], next[2]})
	if to != (mgl32.Vec2{next[0], next[2]}) {
		next = mgl32.Vec3{to[0], s.World.Terrain.InterpolatedSurfaceElevationAt(to[0], to[1]), to[1]}
	}
	agent.Pos = next
	agent.Heading = float32(math.Atan2(float64(dirNorm[0]), float64(dirNorm[2])))
	agent.Speed = float32(speed)
}

// walkSpeed is how fast agent walks where they are: WalkSpeed, or
// FootpathSpeedup times it on a footpath.
func (s *Simulation) walkSpeed(agent *world.Guest) float64 {
	if s.World.OnFootpath(mgl32.Vec2{agent.Pos[0], agent.Pos[2]}) {
		return WalkSpeed * world.FootpathSpeedup
	}
	return WalkSpeed
}

// walkClearance is how wide a walker gives a lift tower or a station's
// legs or hut, in metres.
const walkClearance = 1.2

// walkClear turns a walker heading dir (unit XZ) toward a point dist
// away round the first lift tower or station part (s.towersScratch)
// they'd otherwise walk into: along its tangent, on the side the line
// already passes. One beyond the point, or right by it (a lift line runs
// between a station's legs), is left alone.
func (s *Simulation) walkClear(pos mgl32.Vec3, dir mgl32.Vec2, dist float32) mgl32.Vec2 {
	at := mgl32.Vec2{pos[0], pos[2]}
	goal := at.Add(dir.Mul(dist))
	for _, h := range s.towersScratch {
		to := h.Sub(at)
		along := to.Dot(dir)
		if along <= 0 || along > walkClearance+1 || along >= dist || h.Sub(goal).Len() < walkClearance {
			continue // behind, not close yet, past the point, or at it
		}
		cross := dir[0]*to[1] - dir[1]*to[0]
		if abs32(cross) >= walkClearance {
			continue // passes clear
		}
		// Step along the tangent on the side the line already passes.
		t := mgl32.Vec2{to[1], -to[0]}
		if cross < 0 {
			t = t.Mul(-1)
		}
		return t.Normalize()
	}
	// Nor across a rope (the player's, or a lift corral's): walk along it
	// toward whichever end makes the shorter way round to the goal.
	// A goal just over one is inside it (a waypoint in a corral): step
	// over, or they turn back and forth at the rope.
	if s.World.HasRopes() && dist > walkRopeGoalNear {
		if _, c, d, ok := s.World.RopeCrossingSeg(at, at.Add(dir.Mul(min(dist, walkRopeLook)))); ok {
			along := d.Sub(c).Normalize()
			if at.Sub(c).Len()+c.Sub(goal).Len() < at.Sub(d).Len()+d.Sub(goal).Len() {
				along = along.Mul(-1)
			}
			// In a corner, with a rope that way too, they step over:
			// a rope isn't a wall.
			if !s.World.RopeCrosses(at, at.Add(along.Mul(walkRopeLook))) {
				return along
			}
		}
	}
	// Nor under a top station or into a hut (stationHazards): if the way
	// ahead goes in, step aside on the side of the box the goal is on.
	// That side holds as they go along it (the box's bearing turns faster
	// than the goal's beyond it); choosing by which side is further from
	// the box flipped every tick for a walker heading straight at it, who
	// then stood shaking (Boreal Playtest). Once inside, they walk on out.
	ahead := at.Add(dir.Mul(min(dist, walkClearance+1)))
	for _, b := range stationHazards(s.World, at[0], at[1], walkClearance+2, nil) {
		if !b.Contains(ahead, walkStationMargin) || b.Contains(at, 0) || b.Contains(goal, walkStationMargin) {
			continue // a goal in the box: no way round to it
		}
		left, right := mgl32.Vec2{-dir[1], dir[0]}, mgl32.Vec2{dir[1], -dir[0]}
		to, g := b.Center.Sub(at), goal.Sub(at)
		if to[0]*g[1]-to[1]*g[0] >= 0 {
			return left
		}
		return right
	}
	return dir
}

// inStationHazard reports whether cell c's centre is in (or within
// walkStationMargin of) a station hut or top terminal (stationHazards).
func inStationHazard(w *world.World, c [2]int) bool {
	p := mgl32.Vec2{(float32(c[0]) + 0.5) * CellSize, (float32(c[1]) + 0.5) * CellSize}
	for _, b := range stationHazards(w, p[0], p[1], walkStationMargin, nil) {
		if b.Contains(p, walkStationMargin) {
			return true
		}
	}
	return false
}

// walkRopeLook is how far ahead a walker looks for a rope in the way,
// metres; within walkRopeGoalNear of their goal they don't go round.
const (
	walkRopeLook     = float32(1.5)
	walkRopeGoalNear = float32(4)
)

// walkStationMargin is how far outside a station's footprint a walker's
// way must stay, metres.
const walkStationMargin = float32(0.5)

// tickQueued walks the agent toward their place in line (in a crowd, or
// in a single file) and turns them to face the boarding spot.
func (s *Simulation) tickQueued(agent *world.Guest, dt float64) {
	w := s.World
	drain := float32(dt * patienceDrainPerSecQueuing)
	if !s.LiftsRunning() {
		drain = 0 // waiting for the lifts to open doesn't try anyone's patience
	}
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
		agent.Patience -= drain
		if agent.Patience < 0 {
			agent.Patience = 0
		}
		x, z := lift.LinePlaceXZ(agent, idx)
		slot := mgl32.Vec3{x, w.Terrain.InterpolatedSurfaceElevationAt(x, z), z}
		s.shuffleToward(agent, slot, dt)
		// Always face up the line, toward the load point and past it —
		// tickWalkToward sets heading to the walk direction, but skiers
		// who've reached their slot need to keep looking forward instead
		// of holding whatever heading they last walked in with.
		ahead := lift.LoadXZ().Add(lift.Top.Sub(lift.Base).Normalize().Mul(4))
		faceX := ahead[0] - agent.Pos[0]
		faceZ := ahead[1] - agent.Pos[2]
		if faceX*faceX+faceZ*faceZ > 0.01 {
			agent.Heading = float32(math.Atan2(float64(faceX), float64(faceZ)))
		}
		return
	}
}

// lineSnap is how close to their place in line a guest steps onto it,
// metres. tickWalkToward's ArrivalThreshold (2 m) is a whole place: with
// it, guests hopped a place at a time and zipped through a corral's lanes.
const lineSnap = float32(0.15)

// shuffleToward walks a guest in line toward target at walking pace.
func (s *Simulation) shuffleToward(agent *world.Guest, target mgl32.Vec3, dt float64) {
	d := mgl32.Vec2{target[0] - agent.Pos[0], target[2] - agent.Pos[2]}
	dist := d.Len()
	if dist < lineSnap {
		agent.Pos, agent.Speed = target, 0
		return
	}
	dir := s.walkClear(agent.Pos, d.Mul(1/dist), dist)
	speed := s.walkSpeed(agent)
	step := min(float32(speed*dt), dist)
	agent.Pos[0] += dir[0] * step
	agent.Pos[2] += dir[1] * step
	agent.Pos[1] = s.World.Terrain.InterpolatedSurfaceElevationAt(agent.Pos[0], agent.Pos[2])
	agent.Speed = float32(speed)
}

// tickRiding glues the agent to its current chair's position. Seat
// anchors come from the lift type's chair mesh slot metadata, which
// the model writer baked into chair.obj / chair_quad.obj from the
// slots the chair scripts declare. The slot Pos is in the chair-local
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
		if lift.Cableless() {
			// Heli passenger: glue position to the helicopter body.
			pos := world.HeliWorldPos(lift, w.Terrain)
			agent.Pos = pos
			agent.Heading = world.HeliHeading(lift)
			return
		}
		slots := world.SlotsFor(lift.Type.MeshID())
		for ci, chair := range lift.Chairs {
			for slotIdx, p := range chair.Passengers {
				if p != agent {
					continue
				}
				pos, heading := lift.PlacePos(ci, w.Terrain)
				agent.Pos = seatWorldPos(pos, heading, slotIdx, slots)
				agent.Heading = heading
				if slotIdx < len(slots) && slots[slotIdx].Back {
					agent.Heading = heading + math.Pi // facing the back of the cabin
				}
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
	if agent.Route.CutOff >= cutOffChecks && w.Terrain.NearOpenWater(agent.Pos[0], agent.Pos[2], cutOffNear) {
		s.cutOffByWater(agent)
		return
	}
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
	if agent.SkisOn && s.skiBatch != nil {
		// A skier's way round the trees, perception, and steering are
		// worked out for every skier at once, across cores
		// (decideSkiers); the walk or the run happens after, in order.
		l, why := s.follows(agent)
		*s.skiBatch = append(*s.skiBatch, skiJob{a: agent, goal: targetPos, seed: rng.Global().Uint64(), leader: l, off: why})
		return
	}
	// Around the trees, where the straight way crosses them: steer at
	// the next waypoint, while the destination stays targetPos.
	steer := s.routeTarget(agent, s.trailCarrot(agent, targetPos), &s.routeScratch)
	if !agent.SkisOn || (!shouldSki(w.Terrain, agent.Pos, steer) && agent.Speed <= skiWalkSpeed) {
		s.walkStep(agent, targetPos, steer, dt)
		return
	}
	s.tickSkier(agent, steer, dt)
}

// walkStep walks agent a step toward steer (on the way to targetPos):
// skis off, or terrain and momentum no longer calling for skiing.
func (s *Simulation) walkStep(agent *world.Guest, targetPos, steer mgl32.Vec3, dt float64) {
	// Remove skis when close to lodge or parking — guests shouldn't
	// shuffle the last few metres in full gear. Only trigger near the
	// destination so a departing guest still skis down from the lift
	// top rather than removing skis immediately after unloading.
	if agent.SkisOn && agent.SkiTransitionTimer == 0 {
		if d, ok := leavingDistance(agent, targetPos); ok && d < skisOffNearDest {
			agent.SkiTransitionTimer = skiChangeTime(agent)
			return
		}
	}
	s.recordWalkTick(agent, steer)
	s.tickWalkToward(agent, steer, dt)
	if !agent.SkisOn {
		agent.Patience -= float32(dt * patienceDrainPerSecWalking)
		if agent.Patience < 0 {
			agent.Patience = 0
		}
	}
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
	if head.Kind != ai.ActSkiToService && head.Kind != ai.ActSkiToParking && a.Plan.Goal != ai.GoalDepart {
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
	dirNorm := s.walkClear(agent.Pos, mgl32.Vec2{dx / distXZ, dz / distXZ}, distXZ)
	speed := s.walkSpeed(agent)
	step := float32(speed * dt)
	if step > distXZ {
		step = distXZ
	}
	to := s.World.Terrain.KeepOutOfWater(mgl32.Vec2{agent.Pos[0], agent.Pos[2]},
		mgl32.Vec2{agent.Pos[0] + dirNorm[0]*step, agent.Pos[2] + dirNorm[1]*step})
	agent.Pos[0], agent.Pos[2] = to[0], to[1]
	agent.Pos[1] = s.World.Terrain.InterpolatedSurfaceElevationAt(agent.Pos[0], agent.Pos[2])
	agent.Heading = float32(math.Atan2(float64(dirNorm[0]), float64(dirNorm[1])))
	agent.Speed = float32(speed)
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
			// peel off the front) — by way of round the line when
			// it's in the way.
			return liftApproachWorldPos(w, l, a), true
		}
	}
	for _, b := range w.Buildings {
		if b.ID == id {
			return entranceWorldPos(w, b, a.Pos, visitService(w, a)), true
		}
	}
	return mgl32.Vec3{}, false
}

// clipInToRide puts a boarding guest's skis on if they're not on yet,
// finishing a put-on still under way: nobody rides up carrying them.
func clipInToRide(a *world.Guest) {
	if !a.NeedsGear && (!a.SkisOn || a.SkiTransitionTimer < 0) {
		a.SkisOn, a.SkiTransitionTimer = true, 0
	}
}

// walkToLineNear is how close to their place in a lift line a walker
// leaves their route for it, metres.
const walkToLineNear = float32(20)

// nearLine reports whether a walker heading for a lift line is within
// reach metres of where they'd join it, or by the station when they have
// to go round it (lineApproach): close enough to leave their route and
// head straight there (reach walkToLineNear), or not far enough off to
// need a new one (repathToLine, further out, so the two don't take turns).
func (s *Simulation) nearLine(a *world.Guest, reach float32) bool {
	st := a.Plan.Head()
	if st.Kind != ai.ActWalkToLift {
		return false
	}
	l := findLiftByID(s.World, st.LiftID)
	if l == nil {
		return false
	}
	pos := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	if l.LineEndXZ(pos).Sub(pos).Len() < reach {
		return true
	}
	_, round := lineApproach(s.World, l, a)
	return round && pos.Sub(l.Base).Len() < world.StationExtent+reach
}

// repathToMovedLine re-routes a walker heading for a lift line whose
// route no longer ends at it: a crowd has grown past the route's end, or
// a corral's entrance has moved as it grew.
func (s *Simulation) repathToMovedLine(a *world.Guest) {
	st := a.Plan.Head()
	if st.Kind != ai.ActWalkToLift || a.PathIdx >= len(a.Path) {
		return
	}
	l := findLiftByID(s.World, st.LiftID)
	if l == nil {
		return
	}
	pos := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	end := l.BackOfQueueCell(pos)
	if last := a.Path[len(a.Path)-1]; max(last[0]-end[0], end[0]-last[0]) <= 1 && max(last[1]-end[1], end[1]-last[1]) <= 1 {
		return
	}
	cell := [2]int{int(math.Floor(float64(pos[0] / CellSize))), int(math.Floor(float64(pos[1] / CellSize)))}
	if path := s.Pathfinder.FindPath(cell, end); len(path) > 0 {
		a.Path, a.PathIdx = path, 0
	}
}

// repathToLine gives a walker heading for a lift line a new route when
// they've none and the line is well beyond where they left the last
// (a corral's entrance moves as it grows), and reports whether it did.
// Without one they walked straight at it, into the corral's ropes.
func (s *Simulation) repathToLine(a *world.Guest) bool {
	st := a.Plan.Head()
	if st.Kind != ai.ActWalkToLift || a.SkisOn || a.Queued {
		return false
	}
	l := findLiftByID(s.World, st.LiftID)
	if l == nil {
		return false
	}
	pos := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	if s.nearLine(a, walkToLineNear+10) {
		return false
	}
	cell := [2]int{int(math.Floor(float64(pos[0] / CellSize))), int(math.Floor(float64(pos[1] / CellSize)))}
	path := s.Pathfinder.FindPath(cell, l.BackOfQueueCell(pos))
	if len(path) == 0 {
		return false
	}
	a.Path, a.PathIdx = path, 0
	return true
}

// lineWayReach is how close to a point on the way round a lift line a
// guest must come before choosing the next, metres; lineWayKeep is how
// long one is kept at most, sim seconds.
const (
	lineWayReach = float32(1.2)
	lineWayKeep  = 60.0
)

// leaderInLine reports whether a's group leader is waiting in lift l's
// line.
func leaderInLine(a *world.Guest, l *world.Lift) bool {
	ld := a.Party.Leader
	if ld == nil || !ld.Queued {
		return false
	}
	for _, q := range l.Queue {
		if q == ld {
			return true
		}
	}
	return false
}

// liftApproachWorldPos is where a heads for lift l's line: the edge of a
// crowd nearest them, or the back of a single file by way of round it
// when it's in the way (QueueApproachXZ), and round the base station
// first when that's in the way (lineApproach).
func liftApproachWorldPos(w *world.World, l *world.Lift, a *world.Guest) mgl32.Vec3 {
	p, _ := lineApproach(w, l, a)
	return p
}

// lineApproach is liftApproachWorldPos, and whether it's a way round the
// base station rather than the line itself: the line runs back from under
// the station (out of a station house's door), so a guest coming at it
// from in front or beside goes round the outside to join it from behind,
// not through the station (world.Lift.StationDetour).
func lineApproach(w *world.World, l *world.Lift, a *world.Guest) (mgl32.Vec3, bool) {
	p := lineEndApproach(w, l, a)
	pos := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	if way, ok := l.StationDetour(pos, mgl32.Vec2{p[0], p[2]}); ok {
		return mgl32.Vec3{way[0], w.Terrain.InterpolatedSurfaceElevationAt(way[0], way[1]), way[1]}, true
	}
	return p, false
}

// lineEndApproach is where a heads for lift l's line itself.
// A point on the way round a single file is kept until reached
// (Guest.LineWay).
func lineEndApproach(w *world.World, l *world.Lift, a *world.Guest) mgl32.Vec3 {
	pos := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	at := func(p mgl32.Vec2) mgl32.Vec3 {
		return mgl32.Vec3{p[0], w.Terrain.InterpolatedSurfaceElevationAt(p[0], p[1]), p[1]}
	}
	if l.IsCrowd() || l.HasCorral() {
		// The edge of the crowd nearest them, or the corral's entrance:
		// its ropes steer them round it.
		return at(l.LineEndXZ(pos))
	}
	if a.LineWayLift == l.ID && w.SimTime-a.LineWayAt < lineWayKeep && a.LineWay.Sub(pos).Len() > lineWayReach {
		return at(a.LineWay)
	}
	a.LineWayLift = 0
	p := l.QueueApproachXZ(pos)
	bx, bz := l.QueueSlotXZ(len(l.Queue))
	if p == (mgl32.Vec2{bx, bz}) {
		return at(p)
	}
	// A way round through trees or a building: straight to the back.
	if cx, cz := int(p[0]/CellSize), int(p[1]/CellSize); !w.Terrain.InBounds(cx, cz) ||
		!w.Terrain.Cells[cx][cz].Walkable() || onBuildingFootprint(w, p[0], p[1]) {
		return l.BackOfQueueWorldPos(pos, w.Terrain)
	}
	a.LineWay, a.LineWayLift, a.LineWayAt = p, l.ID, w.SimTime
	return at(p)
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
