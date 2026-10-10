package world

import (
	"github.com/go-gl/mathgl/mgl32"
)

const (
	PatrollerOnSceneSeconds = float32(4.0) // s — countdown while loading patient
)

// PatrollerState is the active phase of a ski patroller's work cycle.
type PatrollerState uint8

const (
	PatrollerAtHut       PatrollerState = iota // on duty, waiting in the patrol room
	PatrollerEnRoute                           // driving to an injured guest
	PatrollerOnScene                           // on scene, loading the patient (timer)
	PatrollerReturning                         // driving the patient to first aid
	PatrollerOffDuty                           // off shift, in the patrol room
	PatrollerToGarage                          // walking to a garage for a snowmobile
	PatrollerFetching                          // driving the snowmobile to park by the patrol room
	PatrollerToSled                            // walking to the parked snowmobile (to respond, or to stow it)
	PatrollerStowing                           // driving the snowmobile back to its garage at close
	PatrollerWalkingBack                       // walking back to the patrol room from the garage
	PatrollerToLift                            // walking to a lift to ski to an injury
	PatrollerRiding                            // riding that lift up
	PatrollerSkiing                            // skiing down to the injured guest
	PatrollerToboggan                          // skiing the guest down in a toboggan
	PatrollerSkiingBack                        // skiing back to the patrol room after a call that fell through
	PatrollerHiking                            // hiking up to an injured guest
)

// OnFoot reports whether the patroller is walking.
func (s PatrollerState) OnFoot() bool {
	return s == PatrollerToGarage || s == PatrollerToSled || s == PatrollerWalkingBack || s == PatrollerToLift ||
		s == PatrollerHiking
}

// OnSkis reports whether the patroller is on a lift or skiing.
func (s PatrollerState) OnSkis() bool {
	return s == PatrollerRiding || s == PatrollerSkiing || s == PatrollerToboggan || s == PatrollerSkiingBack
}

// Driving reports whether the patroller is on a snowmobile.
func (s PatrollerState) Driving() bool {
	return s == PatrollerEnRoute || s == PatrollerOnScene || s == PatrollerReturning ||
		s == PatrollerFetching || s == PatrollerStowing
}

// Responding reports whether the patroller is out on a call.
func (s PatrollerState) Responding() bool {
	switch s {
	case PatrollerEnRoute, PatrollerOnScene, PatrollerReturning,
		PatrollerToLift, PatrollerRiding, PatrollerSkiing, PatrollerToboggan, PatrollerHiking:
		return true
	}
	return false
}

// Label describes the state for popups.
func (s PatrollerState) Label() string {
	switch s {
	case PatrollerAtHut:
		return "on duty"
	case PatrollerEnRoute:
		return "on the way to an injury"
	case PatrollerOnScene:
		return "on scene"
	case PatrollerReturning:
		return "bringing a patient in"
	case PatrollerToGarage:
		return "fetching a snowmobile"
	case PatrollerFetching:
		return "bringing a snowmobile up"
	case PatrollerToSled:
		return "heading to the snowmobile"
	case PatrollerStowing:
		return "putting the snowmobile away"
	case PatrollerWalkingBack:
		return "walking back"
	case PatrollerToLift:
		return "heading to a lift"
	case PatrollerRiding:
		return "riding up to an injury"
	case PatrollerSkiing:
		return "skiing to an injury"
	case PatrollerToboggan:
		return "bringing a patient down by toboggan"
	case PatrollerSkiingBack:
		return "skiing back"
	case PatrollerHiking:
		return "hiking to an injury"
	}
	return "off duty"
}

// Patroller is one ski patroller, based at a building's patrol service
// (HutID). Their day: off duty overnight; in the morning they walk to a
// garage, take a snowmobile (SnowmobileID), and park it on the snow by
// the patrol room; they answer injuries from there, by snowmobile, by
// lift and skis, or by hiking, whichever is faster, bringing the guest down by
// snowmobile or toboggan; after close they put the snowmobile back and
// walk in (notes/next/Patrol Day.md). The state machine runs in
// sim/patrol.go.
type Patroller struct {
	ID            uint64
	HutID         uint64 // the building whose patrol service bases them; despawns with it
	Pos           mgl32.Vec3
	Heading       float32
	State         PatrollerState
	SnowmobileID  uint64     // the snowmobile they've taken out today; 0 for none
	TargetGuestID uint64     // guest being rescued; 0 when idle
	TargetPos     mgl32.Vec3 // current drive destination
	ActionTimer   float32    // counts down during PatrollerOnScene
	OnSkis        bool       // answering this call on skis (by lift), not by snowmobile
	LiftID        uint64     // the lift they're riding or walking to
	LiftProgress  float32    // 0–1 up that lift
	Path          [][2]int   // walking route (cells), not saved
	PathIdx       int
	// SledRoute is the snowmobile's route to SledGoal (sim/sled_route.go):
	// its corners, then the goal; SledIdx the next. Not saved.
	SledRoute []mgl32.Vec2
	SledIdx   int
	SledGoal  mgl32.Vec2
}

// SpawnPatroller creates a new patroller parked at hut's patrol door
// (hut is the building whose patrol service bases them).
func (w *World) SpawnPatroller(hut *Building) *Patroller {
	p := &Patroller{
		ID:    w.NextID(),
		HutID: hut.ID,
		Pos:   w.PatrollerHutPos(hut),
		State: PatrollerOffDuty,
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
		if m := w.SnowmobileByID(p.SnowmobileID); m != nil {
			w.returnSnowmobile(m)
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
	var gone []uint64
	for _, p := range w.Patrollers {
		if p.HutID == hutID {
			gone = append(gone, p.ID)
		}
	}
	for _, id := range gone {
		w.removePatroller(id)
	}
}
