package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

const (
	patrollerSpeed          = float32(12.0) // m/s — fast enough to reach an injured skier quickly
	patrollerArriveRadius   = float32(5.0)  // m — close enough to act
	PatrollerOnSceneSeconds = float32(4.0)  // s — countdown while loading patient
)

// PatrollerState is the active phase of a ski patroller's work cycle.
type PatrollerState uint8

const (
	PatrollerAtHut     PatrollerState = iota // waiting at patrol hut
	PatrollerEnRoute                         // driving to injured guest
	PatrollerOnScene                         // on-scene loading patient (timer)
	PatrollerReturning                       // driving to parking lot with patient
)

// Patroller is a ski-patrol unit (person + snowmobile). One is spawned per
// patrol hut. Its state machine is driven by sim/patrol.go every tick.
type Patroller struct {
	ID            uint64
	HutID         uint64 // owning patrol hut; despawns when hut is removed
	Pos           mgl32.Vec3
	Heading       float32
	State         PatrollerState
	TargetGuestID uint64     // guest being rescued; 0 when idle
	TargetPos     mgl32.Vec3 // current drive destination
	ActionTimer   float32    // counts down during PatrollerOnScene
}

// SpawnPatroller creates a new patroller parked at hut's patrol door
// (hut is the building whose patrol service bases them).
func (w *World) SpawnPatroller(hut *Building) *Patroller {
	p := &Patroller{
		ID:    w.NextID(),
		HutID: hut.ID,
		Pos:   w.PatrollerHutPos(hut),
		State: PatrollerAtHut,
	}
	w.Patrollers = append(w.Patrollers, p)
	return p
}

// PatrollerHutPos is where a patroller waits at its base: just outside
// the building's patrol door.
func (w *World) PatrollerHutPos(hut *Building) mgl32.Vec3 {
	return w.ServiceHome(hut, ServicePatrol)
}

// removePatroller drops one patroller, letting go of any patient.
func (w *World) removePatroller(id uint64) {
	for i, p := range w.Patrollers {
		if p.ID != id {
			continue
		}
		if p.TargetGuestID != 0 {
			for _, g := range w.OnMountain {
				if g.ID == p.TargetGuestID {
					g.OnPatrollerID = 0
					break
				}
			}
		}
		w.Patrollers = append(w.Patrollers[:i], w.Patrollers[i+1:]...)
		return
	}
}

// RemovePatrollersOwnedBy drops every patroller whose HutID matches hutID.
// Called when a patrol hut is demolished.
func (w *World) RemovePatrollersOwnedBy(hutID uint64) {
	// Clear OnPatrollerID on any guest the patroller was carrying.
	for _, p := range w.Patrollers {
		if p.HutID == hutID && p.TargetGuestID != 0 {
			for _, g := range w.OnMountain {
				if g.ID == p.TargetGuestID {
					g.OnPatrollerID = 0
					break
				}
			}
		}
	}
	out := w.Patrollers[:0]
	for _, p := range w.Patrollers {
		if p.HutID != hutID {
			out = append(out, p)
		}
	}
	w.Patrollers = out
}

// DriveToward moves the patroller one step toward (targetX, targetZ) at
// patrollerSpeed. Returns true when within patrollerArriveRadius.
func (p *Patroller) DriveToward(targetX, targetZ float32, dt float64) bool {
	dx := targetX - p.Pos[0]
	dz := targetZ - p.Pos[2]
	dist := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if dist < patrollerArriveRadius {
		return true
	}
	p.Heading = float32(math.Atan2(float64(dx), float64(dz)))
	step := patrollerSpeed * float32(dt)
	if step > dist {
		step = dist
	}
	p.Pos[0] += dx / dist * step
	p.Pos[2] += dz / dist * step
	return false
}
