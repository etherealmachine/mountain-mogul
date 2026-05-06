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

	lodgeReturnProb = 0.25 // probability a skier returns to lodge at the top instead of skiing down
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
			agent.TargetLiftID = nearest.ID
			agent.TargetBuildingID = b.ID
			agent.Traits = ai.TraitsFor(rollSkillLevel(s.Rng))
			agent.Balance = 1.0
			path := s.Pathfinder.FindPath(b.Pos, nearest.Base)
			if path != nil {
				agent.Path = path
				agent.PathIdx = 0
				agent.State = world.StateWalking
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

					// Pick state and a target so we can orient the skier downhill.
					// Heading must be reset: while riding, Heading points up the lift
					// cable, which would make the new tickSkier physics treat gravity
					// as decelerating (cos θ_off < 0) and the skier would never start.
					var target mgl32.Vec3
					if s.Rng.Float64() < lodgeReturnProb && findBuilding(w, agent.TargetBuildingID) != nil {
						agent.State = world.StateReturningToLodge
						lodge := findBuilding(w, agent.TargetBuildingID)
						target = mgl32.Vec3{
							float32(lodge.Pos[0]) * CellSize,
							w.Terrain.ElevationAt(lodge.Pos[0], lodge.Pos[1]),
							float32(lodge.Pos[1]) * CellSize,
						}
					} else {
						agent.State = world.StateSkiing
						agent.TargetLiftID = lift.ID
						target = mgl32.Vec3{
							float32(lift.Base[0]) * CellSize,
							w.Terrain.ElevationAt(lift.Base[0], lift.Base[1]),
							float32(lift.Base[1]) * CellSize,
						}
					}
					dx := target[0] - agent.Pos[0]
					dz := target[2] - agent.Pos[2]
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
					agent.State = world.StateRiding
				}
			}
		}
	}
}

func (s *Simulation) tickAgents(dt float64) {
	w := s.World
	for _, agent := range w.Agents {
		switch agent.State {
		case world.StateWalking:
			s.tickWalking(agent, dt)
		case world.StateQueuing:
			// Waiting; handled in tickLifts
		case world.StateRiding:
			s.tickRiding(agent, dt)
		case world.StateSkiing:
			s.tickSkiing(agent, dt)
		case world.StateReturningToLodge:
			s.tickReturning(agent, dt)
		case world.StateFallen:
			s.tickFallen(agent, dt)
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

func (s *Simulation) tickWalking(agent *world.Agent, dt float64) {
	w := s.World
	if len(agent.Path) == 0 || agent.PathIdx >= len(agent.Path) {
		agent.State = world.StateQueuing
		s.joinLiftQueue(agent)
		return
	}

	target := agent.Path[agent.PathIdx]
	tx := float32(target[0]) * CellSize
	tz := float32(target[1]) * CellSize
	ty := w.Terrain.ElevationAt(target[0], target[1])
	targetPos := mgl32.Vec3{tx, ty, tz}

	dir := targetPos.Sub(agent.Pos)
	dist := dir.Len()

	step := float32(WalkSpeed * dt)
	if dist <= step {
		agent.Pos = targetPos
		agent.PathIdx++
		if agent.PathIdx >= len(agent.Path) {
			agent.State = world.StateQueuing
			s.joinLiftQueue(agent)
		}
	} else {
		dirNorm := dir.Normalize()
		agent.Pos = agent.Pos.Add(dirNorm.Mul(step))
		agent.Heading = float32(math.Atan2(float64(dirNorm[0]), float64(dirNorm[2])))
	}
}

func (s *Simulation) tickReturning(agent *world.Agent, dt float64) {
	w := s.World
	lodge := findBuilding(w, agent.TargetBuildingID)
	if lodge == nil {
		w.RemoveAgent(agent.ID)
		return
	}

	lx := float32(lodge.Pos[0]) * CellSize
	lz := float32(lodge.Pos[1]) * CellSize
	ly := w.Terrain.ElevationAt(lodge.Pos[0], lodge.Pos[1])
	lodgePos := mgl32.Vec3{lx, ly, lz}

	if s.tickSkier(agent, lodgePos, dt) {
		lodge.SkierCount++
		w.RemoveAgent(agent.ID)
	}
}

func (s *Simulation) joinLiftQueue(agent *world.Agent) {
	w := s.World
	for _, lift := range w.Lifts {
		if lift.ID == agent.TargetLiftID {
			lift.Queue = append(lift.Queue, agent)
			return
		}
	}
	agent.State = world.StateWalking
}

func (s *Simulation) tickRiding(agent *world.Agent, dt float64) {
	w := s.World
	for _, lift := range w.Lifts {
		if lift.ID != agent.TargetLiftID {
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

func (s *Simulation) tickSkiing(agent *world.Agent, dt float64) {
	w := s.World
	var liftBase mgl32.Vec3
	found := false
	for _, lift := range w.Lifts {
		if lift.ID == agent.TargetLiftID {
			bx := float32(lift.Base[0]) * CellSize
			bz := float32(lift.Base[1]) * CellSize
			by := w.Terrain.ElevationAt(lift.Base[0], lift.Base[1])
			liftBase = mgl32.Vec3{bx, by, bz}
			found = true
			break
		}
	}
	if !found {
		nearest := w.NearestLift([2]int{
			int(agent.Pos[0] / CellSize),
			int(agent.Pos[2] / CellSize),
		})
		if nearest == nil {
			return
		}
		agent.TargetLiftID = nearest.ID
		return
	}

	if s.tickSkier(agent, liftBase, dt) {
		agent.State = world.StateQueuing
		s.joinLiftQueue(agent)
	}
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
