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
	WalkSpeed = 2.0  // m/s
	CellSize  = 5.0 // metres per grid cell
)

// Simulation drives all agent and building behaviour.
type Simulation struct {
	World      *world.World
	Pathfinder *Pathfinder
	TimeScale  float64    // simulation speed multiplier (default 5)
	SimTime    float64    // accumulated sim seconds (post-TimeScale)
	Rng        *rand.Rand // single source for all gameplay randomness; testbeds seed this for determinism

	// Planner is the L0 GOAP planner. Currently observe-only — the
	// follow HUD renders its decision for the watched skier but agent
	// behaviour still flows through pickTopTarget. Phase 2 will replace
	// pickTopTarget and the lodge-routing branch with PlanForAgent.
	Planner *goap.Planner

	// Recorder, if non-nil, receives one RecorderFrame per skiing tick.
	// Used by the debug CSV log; default nil.
	Recorder Recorder
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
	sim := &Simulation{
		World:      w,
		Pathfinder: NewPathfinder(w.Terrain),
		TimeScale:  5.0,
		Rng:        rand.New(rand.NewSource(seed)),
		Planner:    goap.NewPlanner(),
	}
	for _, a := range w.Agents {
		if !a.Plan.Done() {
			sim.onPlanStepStart(a)
		}
	}
	return sim
}

// Tick advances the simulation by dt real seconds.
func (s *Simulation) Tick(dt float64) {
	dt *= s.TimeScale
	s.SimTime += dt
	s.tickBuildings(dt)
	s.tickLifts(dt)
	s.tickAgents(dt)
	s.tickSnowcats(dt)
}

func (s *Simulation) tickBuildings(dt float64) {
	w := s.World
	if len(w.Lifts) == 0 {
		return
	}
	for _, b := range w.Buildings {
		if !b.AdvanceTimer(dt, s.Rng) {
			continue
		}
		b.SkierCount--
		b.ArrivalDeparture(+1) // a skier spawning = a car arriving
		agent := w.SpawnAgent(b)
		agent.Traits = ai.TraitsFor(rollSkillLevel(s.Rng))
		agent.Balance = 1.0
		agent.Energy = 1.0
		agent.TurnSide = 0
		// Planner picks the first lift and onPlanStepStart lays the
		// pathfinder route. If no viable plan or no walkable path
		// exists, undo the spawn so the parking lot pool stays
		// consistent.
		s.replan(agent)
		head := agent.Plan.Head()
		bad := agent.Plan.Done() ||
			(head.Kind == ai.ActWalkToLift && len(agent.Path) == 0)
		if bad {
			w.RemoveAgent(agent.ID)
			b.SkierCount++
			b.ArrivalDeparture(-1)
		}
	}
}

func (s *Simulation) tickLifts(dt float64) {
	w := s.World
	for _, lift := range w.Lifts {
		loopLen := lift.LoopLength()
		if loopLen < 1 {
			continue
		}
		fracPerSec := float64(lift.Speed) / float64(loopLen)
		for i := range lift.Chairs {
			chair := &lift.Chairs[i]
			prev := chair.Progress
			chair.Progress += float32(fracPerSec * dt)

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

					// Novelty-driven Fun bump. First ride of this lift is the
					// biggest gain; subsequent rides taper geometrically so
					// the planner's Explore goal naturally drives skiers to
					// unridden lifts before everything else.
					bumpFunAndRideCount(agent, lift)
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
				}
			}
		}
	}
}

// bumpFunAndRideCount applies the per-agent novelty bonus at lift unload:
// first ride of this lift is the biggest Fun gain (≈0.15), subsequent
// rides taper geometrically with factor 0.55 so a 4th repeat is barely
// rewarded. RidenLifts is incremented after the bonus so count=0 maps
// to the unridden case. Allocates RidenLifts on first call.
//
// Mirrored shape in goap.RideLift.Cost — keep the two in sync if the
// constants move so the planner's preference for unridden lifts matches
// the actual Fun outcome.
func bumpFunAndRideCount(agent *world.Agent, lift *world.Lift) {
	if agent.RidenLifts == nil {
		agent.RidenLifts = make(map[uint64]int)
	}
	count := agent.RidenLifts[lift.ID]
	bonus := float32(0.15)
	for i := 0; i < count; i++ {
		bonus *= 0.55
	}
	agent.Fun += bonus
	if agent.Fun > 1 {
		agent.Fun = 1
	}
	agent.RidenLifts[lift.ID] = count + 1
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

// tickAgents dispatches each agent to the appropriate handler based on its
// implicit state. tickPlanning runs first per agent so any replan / step
// advance settles the implicit state (Queued / TargetID / RestTimer /
// Removed) before the switch picks a handler. Order of checks matters:
// fallen short-circuits everything, then on-lift, then queued, then
// resting, then path-walking, then goal locomotion. Removed agents are
// reaped from w.Agents after the loop so range iteration isn't shifted
// mid-pass.
func (s *Simulation) tickAgents(dt float64) {
	w := s.World
	for _, agent := range w.Agents {
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
func (s *Simulation) tickPlanning(a *world.Agent) {
	if a.Plan.Done() {
		s.replan(a)
		return
	}
	snap := goap.Extract(a, s.World)
	head := a.Plan.Head()
	if planActionComplete(head, a, snap) {
		s.advancePlan(a)
		return
	}
	if !planActionPreconditionHolds(head, snap, s.World) {
		s.replan(a)
		return
	}
}

// replan generates a fresh plan and starts its head step. Called at
// spawn, when the plan exhausts, and when a precondition breaks.
func (s *Simulation) replan(a *world.Agent) {
	a.Plan = s.Planner.StoredPlanFor(a, s.World)
	if !a.Plan.Done() {
		s.onPlanStepStart(a)
	}
}

// advancePlan moves the cursor to the next step and starts it; if the
// cursor walks off the end, the plan is done and we re-plan instead.
func (s *Simulation) advancePlan(a *world.Agent) {
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
func (s *Simulation) onPlanStepStart(a *world.Agent) {
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
		if path := s.Pathfinder.FindPath(startCell, lift.BackOfQueueCell()); path != nil {
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
	case ai.ActRestAtLodge:
		a.RestTimer = restAtLodgeSec
		a.Speed = 0
		a.TargetID = 0
	case ai.ActDepart:
		b := findBuildingByID(w, step.BldgID)
		if b != nil {
			b.SkierCount++
			b.ArrivalDeparture(-1) // a skier leaving = a car driving off
		}
		a.Removed = true
	}
}

// planActionComplete returns true when the head step's post-state is
// observable in snap. Drives advancePlan.
func planActionComplete(step ai.PlanAction, a *world.Agent, snap goap.WorldSnapshot) bool {
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
	}
	return true
}

// tickResting counts down the atomic RestAtLodge timer. On expiry
// Energy resets to 1; tickPlanning on the next frame advances the plan.
func (s *Simulation) tickResting(a *world.Agent, dt float64) {
	if a.RestTimer <= 0 {
		return
	}
	a.RestTimer -= float32(dt)
	if a.RestTimer <= 0 {
		a.RestTimer = 0
		a.Energy = 1
	}
}

// reapDeparted removes agents flagged by ActDepart at the end of
// tickAgents. Defers removal out of the range loop so the slice header
// doesn't shift mid-iteration.
func (s *Simulation) reapDeparted() {
	w := s.World
	for i := len(w.Agents) - 1; i >= 0; i-- {
		if w.Agents[i].Removed {
			w.RemoveAgent(w.Agents[i].ID)
		}
	}
}

// planTargetWorldPos returns the world-space position the agent's head
// step is steering toward, if any. Lift / building targets resolve via
// the same routes resolveTarget uses, so heading orientation at lift
// unload matches what tickLocomote will steer at next tick.
func planTargetWorldPos(w *world.World, a *world.Agent) (mgl32.Vec3, bool) {
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

// rollSkillLevel samples the lodge-population skill distribution: 60% beginner,
// 30% intermediate, 10% advanced. Per-lodge tuning is a future extension.
func rollSkillLevel(rng *rand.Rand) ai.SkillLevel {
	r := rng.Float64()
	switch {
	case r < 0.6:
		return ai.SkillBeginner
	case r < 0.9:
		return ai.SkillIntermediate
	default:
		return ai.SkillAdvanced
	}
}

// tickPath walks the agent along their pathfinder route at WalkSpeed. When
// the path ends and the target is a lift, they queue up; otherwise they fall
// through to goal-based locomotion next tick.
func (s *Simulation) tickPath(agent *world.Agent, dt float64) {
	w := s.World
	target := agent.Path[agent.PathIdx]
	tx := (float32(target[0]) + 0.5) * CellSize
	tz := (float32(target[1]) + 0.5) * CellSize
	ty := w.Terrain.SurfaceElevationAt(target[0], target[1])
	targetPos := mgl32.Vec3{tx, ty, tz}

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
// and they shuffle forward on subsequent ticks.
func (s *Simulation) tickQueued(agent *world.Agent, dt float64) {
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
// anchor.
func (s *Simulation) tickRiding(agent *world.Agent, dt float64) {
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
func (s *Simulation) tickLocomote(agent *world.Agent, dt float64) {
	w := s.World
	if agent.TargetID == 0 {
		return
	}
	targetPos, ok := resolveTarget(w, agent.TargetID)
	if !ok {
		// Target vanished — drop it; tickPlanning's precondition check
		// will re-plan on the next tick.
		agent.TargetID = 0
		return
	}

	if shouldSki(w.Terrain, agent.Pos, targetPos) {
		s.tickSkier(agent, targetPos, dt)
		return
	}
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
func (s *Simulation) tickWalkToward(agent *world.Agent, target mgl32.Vec3, dt float64) bool {
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
