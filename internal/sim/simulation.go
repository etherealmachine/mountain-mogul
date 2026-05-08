package sim

import (
	"math"
	"math/rand"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

const (
	WalkSpeed = 2.0  // m/s
	CellSize  = 10.0 // metres per grid cell
)

// Simulation drives all agent and building behaviour.
type Simulation struct {
	World      *world.World
	Pathfinder *Pathfinder
	TimeScale  float64    // simulation speed multiplier (default 5)
	SimTime    float64    // accumulated sim seconds (post-TimeScale)
	Rng        *rand.Rand // single source for all gameplay randomness; testbeds seed this for determinism

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
func NewSimulationWithSeed(w *world.World, seed int64) *Simulation {
	return &Simulation{
		World:      w,
		Pathfinder: NewPathfinder(w.Terrain),
		TimeScale:  5.0,
		Rng:        rand.New(rand.NewSource(seed)),
	}
}

// Tick advances the simulation by dt real seconds.
func (s *Simulation) Tick(dt float64) {
	dt *= s.TimeScale
	s.SimTime += dt
	s.tickBuildings(dt)
	s.tickLifts(dt)
	s.tickAgents(dt)
}

func (s *Simulation) tickBuildings(dt float64) {
	w := s.World
	if len(w.Lifts) == 0 {
		return
	}
	for _, b := range w.Buildings {
		if b.AdvanceTimer(dt, s.Rng) {
			nearest := w.NearestLift(b.Pos)
			if nearest == nil {
				continue
			}
			b.SkierCount--
			agent := w.SpawnAgent(b)
			agent.TargetID = nearest.ID
			agent.Traits = ai.TraitsFor(rollSkillLevel(s.Rng))
			agent.Balance = 1.0
			agent.Confidence = spawnConfidence
			agent.Energy = 1.0
			path := s.Pathfinder.FindPath(b.Pos, nearest.Base)
			if path != nil {
				agent.Path = path
				agent.PathIdx = 0
			} else {
				w.RemoveAgent(agent.ID)
				b.SkierCount++ // failed spawn; return to pool
			}
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
					agent.Motor = ai.MotorState{}
					agent.Route = ai.Route{}
					if agent.Balance < 0.5 {
						agent.Balance = 1.0 // ride up restored balance
					}
					tx := float32(lift.Top[0]) * CellSize
					tz := float32(lift.Top[1]) * CellSize
					ty := w.Terrain.ElevationAt(lift.Top[0], lift.Top[1])
					agent.Pos = mgl32.Vec3{tx, ty, tz}

					// Pick a fresh target so we can orient the skier downhill.
					// Heading must be reset: while riding, Heading points up the lift
					// cable, which would make the new tickSkier physics treat gravity
					// as decelerating (cos θ_off < 0) and the skier would never start.
					targetID, targetPos := pickTopTarget(w, agent, s.Rng)
					agent.TargetID = targetID
					dx := targetPos[0] - agent.Pos[0]
					dz := targetPos[2] - agent.Pos[2]
					agent.Heading = float32(math.Atan2(float64(dx), float64(dz)))
				}
			}

			// At base (progress wraps past 1.0): load up to 2 skiers from queue.
			if chair.Progress >= 1.0 {
				chair.Progress -= 1.0
				for j := 0; j < 2 && len(lift.Queue) > 0; j++ {
					agent := lift.Queue[0]
					lift.Queue = lift.Queue[1:]
					chair.Passengers[j] = agent
					agent.OnLiftID = lift.ID
					agent.Queued = false
				}
			}
		}
	}
}

// pickTopTarget chooses where a skier goes after unloading at a lift top.
// Low-energy skiers head to a randomly-chosen lodge to despawn; otherwise
// they pick a uniform-random lift across the whole resort and ski to its
// base — the resort-spanning "I'll do whichever lift next" behaviour. With
// a single-lift scenario this just keeps picking the same lift, matching
// prior behaviour. Returns the entity ID and its world-space position.
func pickTopTarget(w *world.World, agent *world.Agent, rng *rand.Rand) (uint64, mgl32.Vec3) {
	if agent != nil && agent.Energy <= energyLowThreshold && len(w.Buildings) > 0 {
		lodge := w.Buildings[rng.Intn(len(w.Buildings))]
		return lodge.ID, mgl32.Vec3{
			(float32(lodge.Pos[0]) + 0.5) * CellSize,
			w.Terrain.ElevationAt(lodge.Pos[0], lodge.Pos[1]),
			(float32(lodge.Pos[1]) + 0.5) * CellSize,
		}
	}
	next := w.Lifts[rng.Intn(len(w.Lifts))]
	return next.ID, mgl32.Vec3{
		(float32(next.Base[0]) + 0.5) * CellSize,
		w.Terrain.ElevationAt(next.Base[0], next.Base[1]),
		(float32(next.Base[1]) + 0.5) * CellSize,
	}
}

// tickAgents dispatches each agent to the appropriate handler based on its
// implicit state. Order of checks matters: fallen short-circuits everything,
// then on-lift, then queued, then path-walking, then goal locomotion.
func (s *Simulation) tickAgents(dt float64) {
	w := s.World
	for _, agent := range w.Agents {
		switch {
		case agent.Fallen:
			s.tickFallen(agent, dt)
		case agent.OnLiftID != 0:
			s.tickRiding(agent, dt)
		case agent.Queued:
			// Waiting in lift.Queue — boarding is driven by tickLifts.
		case len(agent.Path) > 0 && agent.PathIdx < len(agent.Path):
			s.tickPath(agent, dt)
		default:
			s.tickLocomote(agent, dt)
		}
	}
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
	ty := w.Terrain.ElevationAt(target[0], target[1])
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
			s.onPathComplete(agent)
		}
		return
	}
	dirNorm := dir.Normalize()
	agent.Pos = agent.Pos.Add(dirNorm.Mul(step))
	agent.Heading = float32(math.Atan2(float64(dirNorm[0]), float64(dirNorm[2])))
}

// onPathComplete handles the transition out of pathfinder-walking. If the
// target is a lift, the agent joins its queue; otherwise locomotion takes
// over on the next tick.
func (s *Simulation) onPathComplete(agent *world.Agent) {
	for _, lift := range s.World.Lifts {
		if lift.ID == agent.TargetID {
			lift.Queue = append(lift.Queue, agent)
			agent.Queued = true
			return
		}
	}
}

// tickRiding glues the agent to its current chair's position. Resolved by
// scanning the named lift's chair passenger lists.
func (s *Simulation) tickRiding(agent *world.Agent, dt float64) {
	w := s.World
	for _, lift := range w.Lifts {
		if lift.ID != agent.OnLiftID {
			continue
		}
		for _, chair := range lift.Chairs {
			for _, p := range chair.Passengers {
				if p == agent {
					pos, heading := lift.ChairPos(chair.Progress, w.Terrain)
					agent.Pos = pos
					agent.Heading = heading
					return
				}
			}
		}
	}
}

// tickLocomote moves the agent toward TargetID, choosing ski or walk based
// on local slope and goal direction. Steep downhill toward the goal → ski;
// flat or uphill → walk straight.
func (s *Simulation) tickLocomote(agent *world.Agent, dt float64) {
	w := s.World
	if agent.TargetID == 0 {
		return
	}
	targetPos, kind, ok := resolveTarget(w, agent.TargetID)
	if !ok {
		// Target vanished — drop it; next tick this agent is idle.
		agent.TargetID = 0
		return
	}

	if shouldSki(w.Terrain, agent.Pos, targetPos) {
		if s.tickSkier(agent, targetPos, dt) {
			s.onArrive(agent, kind)
		}
		return
	}
	if s.tickWalkToward(agent, targetPos, dt) {
		s.onArrive(agent, kind)
	}
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
	agent.Pos[1] = s.World.Terrain.InterpolatedElevationAt(agent.Pos[0], agent.Pos[2])
	agent.Heading = float32(math.Atan2(float64(dirNorm[0]), float64(dirNorm[1])))
	agent.Speed = WalkSpeed
	return false
}

// onArrive handles arrival at the agent's TargetID. For lift targets the
// agent queues up — unless their Energy is depleted, in which case they're
// rerouted to a lodge to despawn. For building (lodge) targets the agent
// is absorbed.
func (s *Simulation) onArrive(agent *world.Agent, kind targetKind) {
	switch kind {
	case targetLift:
		if agent.Energy <= energyLowThreshold && s.routeHome(agent) {
			return
		}
		for _, lift := range s.World.Lifts {
			if lift.ID == agent.TargetID {
				lift.Queue = append(lift.Queue, agent)
				agent.Queued = true
				return
			}
		}
	case targetBuilding:
		for _, b := range s.World.Buildings {
			if b.ID == agent.TargetID {
				b.SkierCount++
				s.World.RemoveAgent(agent.ID)
				return
			}
		}
	}
}

// routeHome retargets a depleted agent at a randomly-chosen lodge and lays
// down a pathfinder route from the agent's current cell. Used when a skier
// arrives at a lift base with no energy left: instead of queueing, they
// walk home and despawn in onArrive(targetBuilding). Returns false (and
// does nothing) if there are no lodges to route to — caller falls through
// to the normal queueing behaviour. The path may be nil if the pathfinder
// can't find a route; tickLocomote handles that with direct walk/ski.
func (s *Simulation) routeHome(agent *world.Agent) bool {
	w := s.World
	if len(w.Buildings) == 0 {
		return false
	}
	lodge := w.Buildings[s.Rng.Intn(len(w.Buildings))]
	agent.TargetID = lodge.ID
	startCell := [2]int{int(agent.Pos[0] / CellSize), int(agent.Pos[2] / CellSize)}
	if path := s.Pathfinder.FindPath(startCell, lodge.Pos); path != nil {
		agent.Path = path
		agent.PathIdx = 0
	}
	return true
}

// targetKind discriminates an Agent.TargetID lookup result.
type targetKind uint8

const (
	targetNone targetKind = iota
	targetLift
	targetBuilding
)

// resolveTarget looks up an agent's TargetID against the world's lifts and
// buildings, returning the target's world-space position and which kind it
// was. ok=false when the ID matches nothing (e.g. the entity was removed).
//
// Uses CELL-CENTER world coordinates ((gx+0.5)*cs) so the seeking axis from
// any cell-center spawned agent runs through cell centers — important for
// the strategic L/R probes, whose ±50 m lateral offsets land at cell-edge
// world positions and otherwise pick up quantization asymmetry from probes
// that fall in different patch-radius cells depending on the agent's
// fractional cell position.
func resolveTarget(w *world.World, id uint64) (mgl32.Vec3, targetKind, bool) {
	for _, l := range w.Lifts {
		if l.ID == id {
			return mgl32.Vec3{
				(float32(l.Base[0]) + 0.5) * CellSize,
				w.Terrain.ElevationAt(l.Base[0], l.Base[1]),
				(float32(l.Base[1]) + 0.5) * CellSize,
			}, targetLift, true
		}
	}
	for _, b := range w.Buildings {
		if b.ID == id {
			return mgl32.Vec3{
				(float32(b.Pos[0]) + 0.5) * CellSize,
				w.Terrain.ElevationAt(b.Pos[0], b.Pos[1]),
				(float32(b.Pos[1]) + 0.5) * CellSize,
			}, targetBuilding, true
		}
	}
	return mgl32.Vec3{}, targetNone, false
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
