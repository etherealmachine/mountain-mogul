package sim

import (
	"math"

	"mountain-mogul/internal/world"
)

// tickPatrollers runs the ski-patrol state machine for every patroller once
// per sim substep. Patrollers are spawned by PlaceBuildingType when a patrol
// hut is placed and despawned when the hut is removed.
func (s *Simulation) tickPatrollers(dt float64) {
	for _, p := range s.World.Patrollers {
		switch p.State {
		case world.PatrollerAtHut:
			s.patrollerIdle(p)
		case world.PatrollerEnRoute:
			s.patrollerEnRoute(p, dt)
		case world.PatrollerOnScene:
			s.patrollerOnScene(p, dt)
		case world.PatrollerReturning:
			s.patrollerReturning(p, dt)
		}
	}
}

// patrollerIdle scans for injured unclaimed guests. Claims the nearest one.
func (s *Simulation) patrollerIdle(p *world.Patroller) {
	var best *world.Guest
	var bestD2 float32
	for _, g := range s.World.OnMountain {
		if !g.Injured || g.OnPatrollerID != 0 {
			continue
		}
		dx := g.Pos[0] - p.Pos[0]
		dz := g.Pos[2] - p.Pos[2]
		d2 := dx*dx + dz*dz
		if best == nil || d2 < bestD2 {
			best = g
			bestD2 = d2
		}
	}
	if best == nil {
		return
	}
	p.TargetGuestID = best.ID
	p.State = world.PatrollerEnRoute
}

// patrollerEnRoute drives toward the target guest. If the guest is no longer
// injured (gave up and left), return to hut.
func (s *Simulation) patrollerEnRoute(p *world.Patroller, dt float64) {
	var target *world.Guest
	for _, g := range s.World.OnMountain {
		if g.ID == p.TargetGuestID {
			target = g
			break
		}
	}
	if target == nil || !target.Injured {
		p.TargetGuestID = 0
		p.State = world.PatrollerAtHut
		return
	}
	if noSnowUnderfoot(s.World.Terrain, p.Pos[0], p.Pos[2]) {
		return // snowmobile can't cross bare ground
	}
	if p.DriveToward(target.Pos[0], target.Pos[2], dt) {
		target.OnPatrollerID = p.ID
		p.ActionTimer = world.PatrollerOnSceneSeconds
		p.State = world.PatrollerOnScene
	}
}

// patrollerOnScene counts down while loading the patient onto the snowmobile.
func (s *Simulation) patrollerOnScene(p *world.Patroller, dt float64) {
	p.ActionTimer -= float32(dt)
	if p.ActionTimer > 0 {
		return
	}
	// Find the nearest parking lot.
	w := s.World
	var best *world.Building
	bestD2 := float32(math.MaxFloat32)
	for _, b := range w.Buildings {
		if b.Type != world.BuildingParking {
			continue
		}
		dx := b.Pos[0] - p.Pos[0]
		dz := b.Pos[1] - p.Pos[2]
		d2 := dx*dx + dz*dz
		if d2 < bestD2 {
			best = b
			bestD2 = d2
		}
	}
	if best == nil {
		// No parking — drop the patient and go idle.
		s.patrollerDropPatient(p, false)
		return
	}
	p.TargetPos[0] = best.Pos[0]
	p.TargetPos[2] = best.Pos[1] // Building.Pos is Vec2 (X, Z)
	p.State = world.PatrollerReturning
}

// patrollerReturning drives to the parking lot with the patient aboard.
func (s *Simulation) patrollerReturning(p *world.Patroller, dt float64) {
	// Keep the patient glued to the snowmobile.
	for _, g := range s.World.OnMountain {
		if g.ID == p.TargetGuestID {
			g.Pos[0] = p.Pos[0]
			g.Pos[2] = p.Pos[2]
			break
		}
	}
	if noSnowUnderfoot(s.World.Terrain, p.Pos[0], p.Pos[2]) {
		return // snowmobile can't cross bare ground
	}
	if p.DriveToward(p.TargetPos[0], p.TargetPos[2], dt) {
		s.patrollerDropPatient(p, true)
	}
}

// patrollerDropPatient detaches the patient and, when depart==true, marks
// them removed so reapDeparted sends them home. The patroller resets to its
// hut position and returns to PatrollerAtHut.
func (s *Simulation) patrollerDropPatient(p *world.Patroller, depart bool) {
	w := s.World
	for _, g := range w.OnMountain {
		if g.ID == p.TargetGuestID {
			g.OnPatrollerID = 0
			g.Injured = false
			g.Fallen = false
			if depart {
				s.Demand.recordDeparture(g, s.SimTime)
				w.History.RecordDeparture()
				w.History.RecordExitThought(g.LastThought().Kind)
				g.Removed = true
			}
			break
		}
	}
	p.TargetGuestID = 0

	// Return patroller to its hut door.
	for _, b := range w.Buildings {
		if b.ID == p.HutID {
			cell := b.DoorCell()
			p.Pos[0] = float32(cell[0]) * world.CellSize
			p.Pos[2] = float32(cell[1]) * world.CellSize
			break
		}
	}
	p.State = world.PatrollerAtHut
}
