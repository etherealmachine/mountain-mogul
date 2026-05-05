package sim

import (
	"math"
	"math/rand"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

const (
	SkiSpeed      = 15.0 // m/s open slope
	WalkSpeed     = 2.0  // m/s
	CellSize      = 10.0 // metres per grid cell
	treeCollisionRate = 2.0 // expected collisions/sec at density=1, skillFactor=1
)

// Simulation drives all agent and building behaviour.
type Simulation struct {
	World      *world.World
	Pathfinder *Pathfinder
}

// NewSimulation creates a Simulation wrapping the given world.
func NewSimulation(w *world.World) *Simulation {
	return &Simulation{
		World:      w,
		Pathfinder: NewPathfinder(w.Terrain),
	}
}

// Tick advances the simulation by dt seconds.
func (s *Simulation) Tick(dt float64) {
	s.tickBuildings(dt)
	s.tickLifts(dt)
	s.tickAgents(dt)
}

func (s *Simulation) tickBuildings(dt float64) {
	w := s.World
	// Only spawn if there's at least one lift
	if len(w.Lifts) == 0 {
		return
	}
	for _, b := range w.Buildings {
		if b.AdvanceTimer(dt) {
			nearest := w.NearestLift(b.Pos)
			if nearest == nil {
				continue
			}
			agent := w.SpawnAgent(b)
			agent.TargetLiftID = nearest.ID
			path := s.Pathfinder.FindPath(b.Pos, nearest.Base)
			if path != nil {
				agent.Path = path
				agent.PathIdx = 0
				agent.State = world.StateWalking
			} else {
				// No path found; skip spawning this agent
				w.RemoveAgent(agent.ID)
			}
		}
	}
}

func (s *Simulation) tickLifts(dt float64) {
	w := s.World
	for _, lift := range w.Lifts {
		// Board queuing agents
		for len(lift.Queue) > 0 && len(lift.Riders) < world.MaxRiders {
			agent := lift.Queue[0]
			lift.Queue = lift.Queue[1:]
			lift.Riders = append(lift.Riders, world.LiftRider{Agent: agent, Progress: 0})
			agent.State = world.StateRiding
		}

		// Advance riders
		toDeposit := make([]int, 0)
		for i := range lift.Riders {
			lift.Riders[i].Progress += float32(float64(lift.Speed) * dt)
			if lift.Riders[i].Progress >= 1.0 {
				toDeposit = append(toDeposit, i)
			}
		}

		// Deposit riders in reverse order to preserve indices
		for j := len(toDeposit) - 1; j >= 0; j-- {
			i := toDeposit[j]
			rider := lift.Riders[i]
			lift.Riders = append(lift.Riders[:i], lift.Riders[i+1:]...)

			// Deposit at top
			agent := rider.Agent
			tx := float32(lift.Top[0]) * CellSize
			tz := float32(lift.Top[1]) * CellSize
			ty := w.Terrain.ElevationAt(lift.Top[0], lift.Top[1])
			agent.Pos = mgl32.Vec3{tx, ty, tz}
			agent.State = world.StateSkiing
			agent.TargetLiftID = lift.ID
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
			// Handled in tickLifts; update visual position along cable
			s.tickRiding(agent, dt)
		case world.StateSkiing:
			s.tickSkiing(agent, dt)
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

func (s *Simulation) joinLiftQueue(agent *world.Agent) {
	w := s.World
	for _, lift := range w.Lifts {
		if lift.ID == agent.TargetLiftID {
			lift.Queue = append(lift.Queue, agent)
			return
		}
	}
	// Lift not found; re-walk
	agent.State = world.StateWalking
}

func (s *Simulation) tickRiding(agent *world.Agent, dt float64) {
	// Visual interpolation along cable — find the rider's progress
	w := s.World
	for _, lift := range w.Lifts {
		if lift.ID != agent.TargetLiftID {
			continue
		}
		for _, rider := range lift.Riders {
			if rider.Agent == agent {
				bx := float32(lift.Base[0]) * CellSize
				bz := float32(lift.Base[1]) * CellSize
				tx := float32(lift.Top[0]) * CellSize
				tz := float32(lift.Top[1]) * CellSize
				by := w.Terrain.ElevationAt(lift.Base[0], lift.Base[1])
				ty := w.Terrain.ElevationAt(lift.Top[0], lift.Top[1])
				p := rider.Progress
				agent.Pos = mgl32.Vec3{
					bx + (tx-bx)*p,
					by + (ty-by)*p + 5.0, // 5m above terrain
					bz + (tz-bz)*p,
				}
				// heading along lift
				dx := tx - bx
				dz := tz - bz
				agent.Heading = float32(math.Atan2(float64(dx), float64(dz)))
				return
			}
		}
	}
}

func (s *Simulation) tickSkiing(agent *world.Agent, dt float64) {
	w := s.World
	// Find target lift base
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
		// Lift gone; find nearest
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
		// Reached lift base — join queue
		agent.Pos = liftBase
		agent.State = world.StateQueuing
		s.joinLiftQueue(agent)
		return
	}

	// Sample tree density at agent's current cell.
	gx := int(agent.Pos[0] / CellSize)
	gz := int(agent.Pos[2] / CellSize)
	density := float32(0)
	if w.Terrain.InBounds(gx, gz) {
		density = w.Terrain.Cells[gx][gz].TreeDensity
	}

	// Tree collision — probability scales with density, dt, and skill.
	// TODO: replace constant skillFactor with agent.SkillFactor when that field exists.
	if density > 0 {
		const skillFactor = 1.0
		p := float64(density) * dt * treeCollisionRate / skillFactor
		if rand.Float64() < p {
			w.RemoveAgent(agent.ID) // placeholder for a proper injury/removal event
			return
		}
	}

	// Speed reduced in proportion to tree density (dense trees = ~40% of open speed).
	speed := float64(SkiSpeed) * float64(1.0-0.6*density)

	dirNorm := dir.Normalize()
	step := float32(speed * dt)
	agent.Pos = agent.Pos.Add(dirNorm.Mul(step))
	agent.Heading = float32(math.Atan2(float64(dirNorm[0]), float64(dirNorm[2])))
}
