package sim

import (
	"math"
	"math/rand"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

const (
	WalkSpeed = 2.0  // m/s
	CellSize  = 10.0 // metres per grid cell

	// Skiing physics constants
	g     = 9.81  // gravitational acceleration (m/s²)
	mu    = 0.05  // kinetic friction coefficient (groomed snow)
	kDrag = 0.01  // air resistance per unit mass (m⁻¹)

	lodgeReturnProb = 0.25 // probability a skier returns to lodge at the top instead of skiing down
)

// Simulation drives all agent and building behaviour.
type Simulation struct {
	World      *world.World
	Pathfinder *Pathfinder
	TimeScale  float64 // simulation speed multiplier (default 5)
}

// NewSimulation creates a Simulation wrapping the given world.
func NewSimulation(w *world.World) *Simulation {
	return &Simulation{
		World:      w,
		Pathfinder: NewPathfinder(w.Terrain),
		TimeScale:  5.0,
	}
}

// Tick advances the simulation by dt real seconds.
func (s *Simulation) Tick(dt float64) {
	dt *= s.TimeScale
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
		if b.AdvanceTimer(dt) {
			nearest := w.NearestLift(b.Pos)
			if nearest == nil {
				continue
			}
			b.SkierCount--
			agent := w.SpawnAgent(b)
			agent.TargetLiftID = nearest.ID
			agent.TargetBuildingID = b.ID
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
					tx := float32(lift.Top[0]) * CellSize
					tz := float32(lift.Top[1]) * CellSize
					ty := w.Terrain.ElevationAt(lift.Top[0], lift.Top[1])
					agent.Pos = mgl32.Vec3{tx, ty, tz}
					if rand.Float64() < lodgeReturnProb && findBuilding(w, agent.TargetBuildingID) != nil {
						agent.State = world.StateReturningToLodge
					} else {
						agent.State = world.StateSkiing
						agent.TargetLiftID = lift.ID
					}
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
		}
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

	dir := lodgePos.Sub(agent.Pos)
	dist := dir.Len()

	if dist < 1.0 {
		agent.Pos = lodgePos
		lodge.SkierCount++
		w.RemoveAgent(agent.ID)
		return
	}

	// Physics: same slope/friction/drag model as tickSkiing
	normal := w.Terrain.NormalAt(agent.Pos[0]/CellSize, agent.Pos[2]/CellSize)
	cosTheta := float64(normal[1])
	sinTheta := math.Sqrt(math.Max(0, 1-cosTheta*cosTheta))
	a := g*sinTheta - mu*g*cosTheta - kDrag*float64(agent.Speed)*float64(agent.Speed)
	agent.Speed = float32(math.Max(0, float64(agent.Speed)+a*dt))

	dirNorm := dir.Normalize()
	agent.Pos = agent.Pos.Add(dirNorm.Mul(agent.Speed * float32(dt)))
	agent.Heading = float32(math.Atan2(float64(dirNorm[0]), float64(dirNorm[2])))
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

	dir := liftBase.Sub(agent.Pos)
	dist := dir.Len()

	if dist < 1.0 {
		agent.Pos = liftBase
		agent.State = world.StateQueuing
		s.joinLiftQueue(agent)
		return
	}

	// Physics: derive slope angle from terrain normal at agent position.
	normal := w.Terrain.NormalAt(agent.Pos[0]/CellSize, agent.Pos[2]/CellSize)
	cosTheta := float64(normal[1])
	sinTheta := math.Sqrt(math.Max(0, 1-cosTheta*cosTheta))

	// Net acceleration: gravity along slope - friction - air drag
	a := g*sinTheta - mu*g*cosTheta - kDrag*float64(agent.Speed)*float64(agent.Speed)
	agent.Speed = float32(math.Max(0, float64(agent.Speed)+a*dt))

	dirNorm := dir.Normalize()
	step := agent.Speed * float32(dt)
	agent.Pos = agent.Pos.Add(dirNorm.Mul(step))
	agent.Heading = float32(math.Atan2(float64(dirNorm[0]), float64(dirNorm[2])))
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
