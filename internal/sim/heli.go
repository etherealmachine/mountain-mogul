//go:build !demo

package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// tickHeliLift drives the state machine for a heli-ski helicopter lift.
// Called by tickLifts for every LiftHeli entry; cable-lift logic is
// handled by the regular tickLifts path.
func tickHeliLift(s *Simulation, lift *world.Lift, dt float64) {
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
			lift.LeaveLine(agent)
			h.Passengers = append(h.Passengers, agent)
			agent.OnLiftID = lift.ID
			agent.Queued = false
			clipInToRide(agent)
			agent.UseService(world.LiftQuality)
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
