package sim

import (
	"fmt"
	"math"
	"mountain-mogul/internal/ai"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// Ski patrol (notes/next/Patrol Day.md). Patrollers are based at a
// building's patrol service, one per patrol tile (world.SyncFleet). Their
// day: off duty in the patrol room overnight; when the resort's day
// starts they walk to the nearest garage with a free snowmobile, drive it
// out, and park it on the nearest snow to the patrol room. On duty they
// wait inside; an injury sends one out to the snowmobile, to the guest,
// and back to first aid at the patrol room. Once the resort has closed
// and the last guest is off the mountain, they drive the snowmobile back
// to its garage and walk in. A patroller with no snowmobile, or for whom
// skiing is faster, rides a running lift (no queue) and skis straight
// down to the guest, then brings them down in a toboggan. Failing both
// (or when it's faster) they hike out from the patrol room; a guest above
// the lift's top is hiked up to from there. Someone always goes. Skiing
// and hiking here are a straight glide at a steady speed, not the
// guests' skiing.
//
// Snowmobiles run at snowmobileSpeed on snow and crawl at
// snowmobileBareSpeed over bare ground: the garage apron and the patrol
// room's pad are plowed, so a snowmobile that refused bare ground (as it
// once did) never got out.

const (
	snowmobileSpeed     = float32(12.0) // m/s on snow
	snowmobileBareSpeed = float32(2.0)  // m/s over bare ground: driven over carefully
	snowmobileArrive    = float32(3.0)  // m

	// patrollerWalkSpeed is a patroller on foot, in boots but not skis.
	patrollerWalkSpeed = float32(1.3)

	// patrolParkReach is how far (cells) from the patrol door to look
	// for snow to park on.
	patrolParkReach = 12

	// A patroller on skis: down to an injury at patrolSkiSpeed, and back
	// with the guest in a toboggan at patrolTobogganSpeed; both slow to
	// patrolBareSpeed off the snow. patrolLiftHeight lifts a rider above
	// the ground along the lift line for drawing.
	patrolSkiSpeed      = float32(8.0)
	patrolTobogganSpeed = float32(4.0)
	patrolBareSpeed     = float32(1.0)
	patrolLiftHeight    = float32(8.0)
	// patrolClimbRate is how fast a hiking patroller gains height (about
	// 900 m an hour), on top of the walk across.
	patrolClimbRate = float32(0.25)
)

// tickPatrollers runs every patroller's state machine once per substep.
// Patrol is on duty from the start of the resort's day until it closes
// and the last guest is off the mountain (the sweep).
func (s *Simulation) tickPatrollers(dt float64) {
	onDuty := !s.ClosedForDay() || len(s.World.OnMountain) > 0
	for _, p := range s.World.Patrollers {
		base := findBuilding(s.World, p.HutID)
		if base == nil {
			continue
		}
		switch p.State {
		case world.PatrollerOffDuty:
			if onDuty {
				s.patrollerStartShift(p, base)
			}
		case world.PatrollerAtHut:
			s.patrollerOnDuty(p, base, onDuty)
		case world.PatrollerToLift:
			if s.patrollerWalk(p, dt) {
				p.State, p.LiftProgress = world.PatrollerRiding, 0
			}
		case world.PatrollerRiding:
			s.patrollerRide(p, dt)
		case world.PatrollerSkiing, world.PatrollerHiking:
			s.patrollerToInjury(p, base, dt)
		case world.PatrollerToboggan, world.PatrollerSkiingBack:
			s.patrollerToboggan(p, base, dt)
		case world.PatrollerToGarage:
			if s.patrollerWalk(p, dt) {
				s.patrollerTakeSled(p, base)
			}
		case world.PatrollerFetching:
			if s.patrollerDrive(p, dt) {
				p.State = world.PatrollerAtHut // parked by the patrol room; inside
				p.Pos = s.World.PatrollerHutPos(base)
			}
		case world.PatrollerToSled:
			if s.patrollerWalk(p, dt) {
				s.patrollerAtSled(p, base)
			}
		case world.PatrollerEnRoute:
			s.patrollerEnRoute(p, base, dt)
		case world.PatrollerOnScene:
			s.patrollerOnScene(p, base, dt)
		case world.PatrollerReturning:
			s.patrollerReturning(p, base, dt)
		case world.PatrollerStowing:
			if s.patrollerDrive(p, dt) {
				s.patrollerStowSled(p, base)
			}
		case world.PatrollerWalkingBack:
			if s.patrollerWalk(p, dt) {
				p.State = world.PatrollerOffDuty
				p.Pos = s.World.PatrollerHutPos(base)
			}
		}
	}
}

// patrollerStartShift sends an off-duty patroller to the nearest garage
// with a free snowmobile, claiming it; with none free they go on duty
// without one.
func (s *Simulation) patrollerStartShift(p *world.Patroller, base *world.Building) {
	w := s.World
	home := w.PatrollerHutPos(base)
	var best *world.Snowmobile
	var bestGarage *world.Building
	bestD := float32(math.MaxFloat32)
	for _, m := range w.Snowmobiles {
		if m.TakenBy != 0 || !m.InGarage {
			continue
		}
		g := findBuilding(w, m.GarageID)
		if g == nil {
			continue
		}
		gp := w.ServiceHome(g, world.ServiceGarage)
		if d := gp.Sub(home).Len(); d < bestD {
			best, bestGarage, bestD = m, g, d
		}
	}
	p.Pos = home
	if best == nil {
		p.State = world.PatrollerAtHut
		return
	}
	best.TakenBy, p.SnowmobileID = p.ID, best.ID
	s.patrollerWalkTo(p, w.ServiceHome(bestGarage, world.ServiceGarage))
	p.State = world.PatrollerToGarage
}

// patrollerTakeSled rolls the claimed snowmobile out of its garage and
// sets off to park it by the patrol room.
func (s *Simulation) patrollerTakeSled(p *world.Patroller, base *world.Building) {
	w := s.World
	m := w.SnowmobileByID(p.SnowmobileID)
	if m == nil {
		p.State = world.PatrollerAtHut
		p.Pos = w.PatrollerHutPos(base)
		return
	}
	if g := findBuilding(w, m.GarageID); g != nil {
		m.Pos = w.ServiceHome(g, world.ServiceGarage)
	}
	m.InGarage = false
	p.Pos = m.Pos
	p.TargetPos = s.patrolParkingSpot(base)
	p.State = world.PatrollerFetching
}

// patrollerOnDuty waits in the patrol room. An injury nobody has claimed
// sends the patroller out the faster way: to their snowmobile and over
// the snow, or to a running lift, up it, and down on skis. At the end of
// the day (closed, everyone off the mountain), a patroller with a
// snowmobile out goes to put it away.
func (s *Simulation) patrollerOnDuty(p *world.Patroller, base *world.Building, onDuty bool) {
	w := s.World
	m := w.SnowmobileByID(p.SnowmobileID)
	if !onDuty {
		if m == nil {
			p.State = world.PatrollerOffDuty
			return
		}
		s.patrollerWalkTo(p, m.Pos)
		p.State = world.PatrollerToSled
		return
	}
	injured := s.injuredByDistance(p.Pos)
	if len(injured) == 0 {
		return
	}
	g := injured[0]
	p.TargetGuestID = g.ID
	sledT := float32(math.MaxFloat32)
	if m != nil {
		sledT = p.Pos.Sub(m.Pos).Len()/patrollerWalkSpeed + s.sledTime(m.Pos, g.Pos)
	}
	lift, skiT := s.bestSkiRoute(p.Pos, g.Pos)
	if lift == nil {
		skiT = math.MaxFloat32
	}
	hikeT := patrolTravelTime(p.Pos, g.Pos)
	switch {
	case hikeT <= sledT && hikeT <= skiT:
		p.OnSkis = true
		p.State = world.PatrollerHiking
	case skiT < sledT:
		p.OnSkis, p.LiftID = true, lift.ID
		s.patrollerWalkTo(p, mgl32.Vec3{lift.Base[0], 0, lift.Base[1]})
		p.State = world.PatrollerToLift
	default:
		p.OnSkis = false
		s.patrollerWalkTo(p, m.Pos)
		p.State = world.PatrollerToSled
	}
}

// patrolTravelTime is how long a patroller on skis takes to get from a
// to b in a straight line: skiing down, or hiking across and up.
func patrolTravelTime(a, b mgl32.Vec3) float32 {
	across := mgl32.Vec2{b[0] - a[0], b[2] - a[2]}.Len()
	if rise := b[1] - a[1]; rise > 0 {
		return across/patrollerWalkSpeed + rise/patrolClimbRate
	}
	return across / patrolSkiSpeed
}

// injuredByDistance is the injured guests nobody is helping, nearest
// first.
func (s *Simulation) injuredByDistance(from mgl32.Vec3) []*world.Guest {
	var out []*world.Guest
	for _, g := range s.World.OnMountain {
		if g.Injured && g.OnPatrollerID == 0 && !s.guestClaimed(g.ID) {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Pos.Sub(from).Len() < out[j].Pos.Sub(from).Len()
	})
	return out
}

// sledTime estimates a snowmobile drive in a straight line from a to b:
// full speed over snow, a crawl over bare ground (sampled along the way).
func (s *Simulation) sledTime(a, b mgl32.Vec3) float32 {
	d := mgl32.Vec2{b[0] - a[0], b[2] - a[2]}
	dist := d.Len()
	const samples = 10
	bare := 0
	for i := 0; i <= samples; i++ {
		t := float32(i) / samples
		if noSnowUnderfoot(s.World.Terrain, a[0]+d[0]*t, a[2]+d[1]*t) {
			bare++
		}
	}
	f := float32(bare) / (samples + 1)
	return dist * (f/snowmobileBareSpeed + (1-f)/snowmobileSpeed)
}

// bestSkiRoute is the running lift that gets a patroller at from to an
// injured guest at to soonest (walk to its base, ride up, then ski down or
// hike up to them in a straight line), and how long that takes; nil when
// no lift is running.
func (s *Simulation) bestSkiRoute(from, to mgl32.Vec3) (*world.Lift, float32) {
	w := s.World
	if !s.LiftsRunning() {
		return nil, 0
	}
	var best *world.Lift
	bestT := float32(math.MaxFloat32)
	for _, l := range w.Lifts {
		if l.IsHeli() || !l.Open || l.OnHold || l.Speed <= 0 {
			continue
		}
		top, base := s.liftEnd(l.Top), s.liftEnd(l.Base)
		t := mgl32.Vec2{base[0] - from[0], base[2] - from[2]}.Len()/patrollerWalkSpeed +
			top.Sub(base).Len()/l.Speed +
			patrolTravelTime(top, to)
		if t < bestT {
			best, bestT = l, t
		}
	}
	return best, bestT
}

// liftEnd is a lift station's position on the ground.
func (s *Simulation) liftEnd(p mgl32.Vec2) mgl32.Vec3 {
	return mgl32.Vec3{p[0], s.World.Terrain.InterpolatedSurfaceElevationAt(p[0], p[1]), p[1]}
}

// patrollerRide carries the patroller up the lift (no queue: patrol goes
// to the front), then sets them skiing down to the guest.
func (s *Simulation) patrollerRide(p *world.Patroller, dt float64) {
	l := findLiftByID(s.World, p.LiftID)
	if l == nil {
		p.State = world.PatrollerSkiing // the lift's gone; ski from here
		return
	}
	base, top := s.liftEnd(l.Base), s.liftEnd(l.Top)
	length := top.Sub(base).Len()
	if length > 0 {
		p.LiftProgress += l.Speed * float32(dt) / length
	} else {
		p.LiftProgress = 1
	}
	if p.LiftProgress >= 1 {
		p.Pos = top
		p.State = world.PatrollerSkiing
		return
	}
	p.Pos = base.Add(top.Sub(base).Mul(p.LiftProgress))
	p.Pos[1] += patrolLiftHeight
	d := top.Sub(base)
	p.Heading = float32(math.Atan2(float64(d[0]), float64(d[2])))
}

// patrollerToInjury skis down, or hikes up, to the injured guest. If
// they're no longer waiting (gave up, or someone else got them), the
// patroller skis back.
func (s *Simulation) patrollerToInjury(p *world.Patroller, base *world.Building, dt float64) {
	target := s.patrollerTarget(p)
	if target == nil || !target.Injured {
		p.TargetGuestID = 0
		p.TargetPos = s.patrolParkingSpot(base)
		p.State = world.PatrollerSkiingBack
		return
	}
	p.TargetPos = target.Pos
	speed := patrolSkiSpeed
	p.State = world.PatrollerSkiing
	if rise := target.Pos[1] - p.Pos[1]; rise > 0 {
		across := mgl32.Vec2{target.Pos[0] - p.Pos[0], target.Pos[2] - p.Pos[2]}.Len()
		if t := patrolTravelTime(p.Pos, target.Pos); t > 0 {
			speed = across / t
		}
		p.State = world.PatrollerHiking
	}
	if s.patrollerGlide(p, speed, dt) {
		target.OnPatrollerID = p.ID
		s.patrolReached(target)
		p.ActionTimer = world.PatrollerOnSceneSeconds
		p.State = world.PatrollerOnScene
	}
}

// patrollerToboggan skis the patient down to first aid at the patrol
// room's snowmobile spot (or, skiing back, just the patroller).
func (s *Simulation) patrollerToboggan(p *world.Patroller, base *world.Building, dt float64) {
	if g := s.patrollerTarget(p); g != nil {
		g.Pos[0], g.Pos[2] = p.Pos[0], p.Pos[2]
	}
	if s.patrollerGlide(p, patrolTobogganSpeed, dt) {
		p.OnSkis = false
		s.patrollerDropPatient(p, base)
	}
}

// patrollerGlide moves a patroller on skis toward TargetPos at speed,
// slowing to a walk off the snow. Reports arrival.
func (s *Simulation) patrollerGlide(p *world.Patroller, speed float32, dt float64) bool {
	d := mgl32.Vec2{p.TargetPos[0] - p.Pos[0], p.TargetPos[2] - p.Pos[2]}
	dist := d.Len()
	if dist < snowmobileArrive {
		return true
	}
	if noSnowUnderfoot(s.World.Terrain, p.Pos[0], p.Pos[2]) {
		speed = min(speed, patrolBareSpeed)
	}
	step := min(speed*float32(dt), dist)
	p.Pos[0] += d[0] / dist * step
	p.Pos[2] += d[1] / dist * step
	p.Pos[1] = s.World.Terrain.InterpolatedSurfaceElevationAt(p.Pos[0], p.Pos[2])
	p.Heading = float32(math.Atan2(float64(d[0]), float64(d[1])))
	return false
}

// guestClaimed reports whether some patroller is already on the way to
// guest id.
func (s *Simulation) guestClaimed(id uint64) bool {
	for _, p := range s.World.Patrollers {
		if p.TargetGuestID == id {
			return true
		}
	}
	return false
}

// patrollerAtSled starts the drive the patroller walked to the snowmobile
// for: to the injured guest, or (at close, or if the call fell through)
// back to the garage.
func (s *Simulation) patrollerAtSled(p *world.Patroller, base *world.Building) {
	w := s.World
	m := w.SnowmobileByID(p.SnowmobileID)
	if m == nil {
		p.TargetGuestID = 0
		p.State = world.PatrollerAtHut
		p.Pos = w.PatrollerHutPos(base)
		return
	}
	p.Pos = m.Pos
	if p.TargetGuestID != 0 {
		p.State = world.PatrollerEnRoute
		return
	}
	p.TargetGuestID = 0
	if g := findBuilding(w, m.GarageID); g != nil {
		p.TargetPos = w.ServiceHome(g, world.ServiceGarage)
		p.State = world.PatrollerStowing
		return
	}
	p.State = world.PatrollerAtHut
}

// patrollerEnRoute drives to the injured guest. If the guest is no longer
// injured (gave up and left), the patroller parks the snowmobile back by
// the patrol room.
func (s *Simulation) patrollerEnRoute(p *world.Patroller, base *world.Building, dt float64) {
	target := s.patrollerTarget(p)
	if target == nil || !target.Injured {
		p.TargetGuestID = 0
		p.TargetPos = s.patrolParkingSpot(base)
		p.State = world.PatrollerFetching
		return
	}
	p.TargetPos = target.Pos
	if s.patrollerDrive(p, dt) {
		target.OnPatrollerID = p.ID
		s.patrolReached(target)
		p.ActionTimer = world.PatrollerOnSceneSeconds
		p.State = world.PatrollerOnScene
	}
}

// patrolReached is the injured guest's verdict on how long help took,
// from the injury to a patroller at their side.
func (s *Simulation) patrolReached(g *world.Guest) {
	waited := injuryWaitTime - g.InjuryWaitTimer
	switch {
	case waited <= patrolFastSec:
		s.applyEvent(g, ai.ThoughtPatrolFast)
	case waited >= patrolSlowSec:
		s.applyEvent(g, ai.ThoughtPatrolSlow)
	default:
		s.applyEvent(g, ai.ThoughtPatrolCame)
	}
}

// Patrol response times, in sim seconds from injury to a patroller at the
// guest's side: about 40 and 100 minutes on the clock. A snowmobile from
// a hut near the lift base answers a mid-mountain call in about the
// first; a lift ride and ski down takes about the second.
const (
	patrolFastSec = 120.0
	patrolSlowSec = 300.0
)

func (s *Simulation) patrollerTarget(p *world.Patroller) *world.Guest {
	for _, g := range s.World.OnMountain {
		if g.ID == p.TargetGuestID {
			return g
		}
	}
	return nil
}

// patrollerOnScene counts down while loading the patient, then heads for
// first aid: the snowmobile's spot by the patrol room.
func (s *Simulation) patrollerOnScene(p *world.Patroller, base *world.Building, dt float64) {
	p.ActionTimer -= float32(dt)
	if p.ActionTimer > 0 {
		return
	}
	p.TargetPos = s.patrolParkingSpot(base)
	if p.OnSkis {
		p.State = world.PatrollerToboggan
		return
	}
	p.State = world.PatrollerReturning
}

// patrollerReturning drives the patient in.
func (s *Simulation) patrollerReturning(p *world.Patroller, base *world.Building, dt float64) {
	if g := s.patrollerTarget(p); g != nil {
		g.Pos[0], g.Pos[2] = p.Pos[0], p.Pos[2] // the patient rides along
	}
	if s.patrollerDrive(p, dt) {
		s.patrollerDropPatient(p, base)
	}
}

// patrollerDropPatient hands the patient over at first aid (patched up,
// they walk to their car), leaves the snowmobile parked where it is, and goes
// back on duty inside.
func (s *Simulation) patrollerDropPatient(p *world.Patroller, base *world.Building) {
	w := s.World
	if g := s.patrollerTarget(p); g != nil {
		g.OnPatrollerID = 0
		g.Injured = false
		g.Fallen = false
		g.SkisOn = false
		g.Speed, g.TurnSide, g.Patience = 0, 0, 0
		g.Balance = float32(fallStartBalance)
		g.Pos = mgl32.Vec3{p.Pos[0], w.Terrain.InterpolatedSurfaceElevationAt(p.Pos[0], p.Pos[2]), p.Pos[2]}
		w.LogEventAt(world.EventRescue, s.SimTime,
			fmt.Sprintf("Patrol brought %s down to first aid", g.Name),
			mgl32.Vec2{p.Pos[0], p.Pos[2]}, g.ID)
		// Patched up, they walk to their car; the departure counts when
		// they drive off, as for any guest.
		s.setDepartReason(g, ai.DepartHurt)
		s.directHomePlan(g)
	}
	p.TargetGuestID = 0
	p.State = world.PatrollerAtHut
	p.Pos = w.PatrollerHutPos(base)
}

// patrollerStowSled parks the snowmobile in its garage and walks back.
func (s *Simulation) patrollerStowSled(p *world.Patroller, base *world.Building) {
	w := s.World
	if m := w.SnowmobileByID(p.SnowmobileID); m != nil {
		if g := findBuilding(w, m.GarageID); g != nil {
			m.Pos = w.GarageSpot(g)
		}
		m.InGarage, m.TakenBy = true, 0
	}
	p.SnowmobileID = 0
	s.patrollerWalkTo(p, w.PatrollerHutPos(base))
	p.State = world.PatrollerWalkingBack
}

// patrollerDrive moves the patroller's snowmobile toward TargetPos, fast
// on snow and slowly over bare ground; the patroller rides it. Reports
// arrival. Without a snowmobile the patroller simply arrives.
func (s *Simulation) patrollerDrive(p *world.Patroller, dt float64) bool {
	m := s.World.SnowmobileByID(p.SnowmobileID)
	if m == nil {
		return true
	}
	d := mgl32.Vec2{p.TargetPos[0] - m.Pos[0], p.TargetPos[2] - m.Pos[2]}
	dist := d.Len()
	if dist < snowmobileArrive {
		p.Pos = m.Pos
		return true
	}
	speed := snowmobileSpeed
	if noSnowUnderfoot(s.World.Terrain, m.Pos[0], m.Pos[2]) {
		speed = snowmobileBareSpeed
	}
	step := min(speed*float32(dt), dist)
	m.Pos[0] += d[0] / dist * step
	m.Pos[2] += d[1] / dist * step
	m.Pos[1] = s.World.Terrain.InterpolatedSurfaceElevationAt(m.Pos[0], m.Pos[2])
	m.Heading = float32(math.Atan2(float64(d[0]), float64(d[1])))
	p.Pos, p.Heading = m.Pos, m.Heading
	return false
}

// patrollerWalkTo sets a walking route to dest: the pathfinder's, or a
// straight line when it finds none.
func (s *Simulation) patrollerWalkTo(p *world.Patroller, dest mgl32.Vec3) {
	p.TargetPos = dest
	from := [2]int{int(p.Pos[0] / CellSize), int(p.Pos[2] / CellSize)}
	to := [2]int{int(dest[0] / CellSize), int(dest[2] / CellSize)}
	p.Path, p.PathIdx = s.Pathfinder.FindPath(from, to), 0
}

// patrollerWalk walks the patroller along its route, then the last few
// metres to TargetPos. Reports arrival.
func (s *Simulation) patrollerWalk(p *world.Patroller, dt float64) bool {
	step := patrollerWalkSpeed * float32(dt)
	for step > 0 {
		var goal mgl32.Vec2
		last := p.PathIdx >= len(p.Path)
		if last {
			goal = mgl32.Vec2{p.TargetPos[0], p.TargetPos[2]}
		} else {
			c := p.Path[p.PathIdx]
			goal = mgl32.Vec2{(float32(c[0]) + 0.5) * CellSize, (float32(c[1]) + 0.5) * CellSize}
		}
		d := goal.Sub(mgl32.Vec2{p.Pos[0], p.Pos[2]})
		dist := d.Len()
		if dist <= step {
			p.Pos[0], p.Pos[2] = goal[0], goal[1]
			step -= dist
			if last {
				p.Pos[1] = s.World.Terrain.InterpolatedSurfaceElevationAt(p.Pos[0], p.Pos[2])
				return true
			}
			p.PathIdx++
			continue
		}
		p.Pos[0] += d[0] / dist * step
		p.Pos[2] += d[1] / dist * step
		p.Heading = float32(math.Atan2(float64(d[0]), float64(d[1])))
		step = 0
	}
	p.Pos[1] = s.World.Terrain.InterpolatedSurfaceElevationAt(p.Pos[0], p.Pos[2])
	return false
}

// patrolParkingSpot is where base's snowmobiles wait: the centre of the
// nearest snow-covered, walkable cell to the patrol door that isn't under
// a building, within patrolParkReach cells; the door itself when there's
// none.
func (s *Simulation) patrolParkingSpot(base *world.Building) mgl32.Vec3 {
	w := s.World
	t := w.Terrain
	home := w.ServiceHome(base, world.ServicePatrol)
	c0 := [2]int{int(home[0] / CellSize), int(home[2] / CellSize)}
	for r := 0; r <= patrolParkReach; r++ {
		best, bestD := [2]int{}, float32(math.MaxFloat32)
		for dx := -r; dx <= r; dx++ {
			for dz := -r; dz <= r; dz++ {
				if max(intAbs(dx), intAbs(dz)) != r {
					continue
				}
				c := [2]int{c0[0] + dx, c0[1] + dz}
				if !t.InBounds(c[0], c[1]) || !t.Cells[c[0]][c[1]].Passable || t.Cells[c[0]][c[1]].TopLayer() == nil {
					continue
				}
				if w.PaintedBuildingAt(c[0], c[1]) != nil {
					continue
				}
				cx, cz := (float32(c[0])+0.5)*CellSize, (float32(c[1])+0.5)*CellSize
				if d := (mgl32.Vec2{cx - home[0], cz - home[2]}).Len(); d < bestD {
					best, bestD = c, d
				}
			}
		}
		if bestD < math.MaxFloat32 {
			x, z := (float32(best[0])+0.5)*CellSize, (float32(best[1])+0.5)*CellSize
			return mgl32.Vec3{x, t.SurfaceElevationAt(best[0], best[1]), z}
		}
	}
	return home
}

func intAbs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
