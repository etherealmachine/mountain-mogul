package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Gear: skis come off and go on with a pause, and stay outside while a
// guest is indoors. A guest carrying skis on the way into a building
// (world.Guest.CarriesSkis) takes them to a rack near the door with room
// (world.SkiRackReach), or, with none, sticks them in the snow just short
// of the door; when they next head anywhere but into a building they walk
// back for them. Each stop to leave or pick up skis is gearStopTime.

const (
	// skiChangeBase is how long, in sim seconds, a middling skier takes to
	// take skis off or put them on; beginners take longer, experts less
	// (skiChangeTime). About 4 real seconds at normal speed.
	skiChangeBase = 15.0
	// gearStopTime is the stop to put skis in a rack or the snow, or take
	// them out, in sim seconds.
	gearStopTime = 3.0
	// skisInSnowAt is how far from the door a guest with no rack sticks
	// their skis in the snow: short of where they count as arrived
	// (8 m, goap's proximity radius).
	skisInSnowAt = 12.0
	// rackStandOff is how far in front of their slot a guest stands to
	// put skis in or take them out.
	rackStandOff = 0.6
)

// skiChangeTime is how long guest a takes to take skis off or put them on,
// sim seconds: 1.3× the base for a beginner down to 0.7× for an expert.
func skiChangeTime(a *world.Guest) float32 {
	return skiChangeBase * (1.3 - 0.6*clamp32(a.Traits.Skill, 0, 1))
}

// goingIn reports whether a's plan is taking them into a building now:
// on the way to use something there, or using it.
func goingIn(a *world.Guest) bool {
	switch a.Plan.Head().Kind {
	case ai.ActSkiToService, ai.ActWalkToService, ai.ActUseService:
		return true
	}
	return false
}

// gearErrand runs a guest's ski errand this tick, if they have one, and
// reports whether it took the tick: stopping to leave or pick up skis,
// walking to a rack to leave them, sticking them in the snow short of
// the door, or walking back to them on the way out.
func (s *Simulation) gearErrand(a *world.Guest, dt float64) bool {
	if a.GearTimer > 0 {
		a.GearTimer -= float32(dt)
		a.Speed = 0
		if a.GearTimer < 0 {
			a.GearTimer = 0
		}
		return true
	}
	switch {
	case a.CarriesSkis() && goingIn(a) && a.Plan.Head().Kind != ai.ActUseService:
		return s.leaveSkis(a, dt)
	case a.Stash.Out && !goingIn(a):
		return s.fetchSkis(a, dt)
	}
	return false
}

// leaveSkis walks a carrying guest to a rack near the door they're going
// in by, or sticks their skis in the snow short of it.
func (s *Simulation) leaveSkis(a *world.Guest, dt float64) bool {
	door, ok := liveTarget(s.World, a)
	if !ok {
		return false
	}
	at := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	doorXZ := mgl32.Vec2{door[0], door[2]}
	if at.Sub(doorXZ).Len() > world.SkiRackReach {
		return false // not near yet
	}
	rack := s.World.BuildingByID(a.Stash.RackID)
	if rack == nil || rack.Type != world.BuildingSkiRack {
		a.Stash.RackID = 0
		if rack, a.Stash.Slot = s.rackFor(doorXZ, a); rack != nil {
			a.Stash.RackID = rack.ID
		}
	}
	if rack == nil {
		if at.Sub(doorXZ).Len() > skisInSnowAt {
			return false // carry on toward the door
		}
		s.skisInSnow(a)
		return true
	}
	slot, yaw := rack.SkiRackSlot(a.Stash.Slot)
	front := mgl32.Vec2{float32(math.Sin(float64(yaw))), float32(math.Cos(float64(yaw)))}
	stand := slot.Add(front.Mul(rackStandOff))
	y := s.World.Terrain.InterpolatedSurfaceElevationAt(stand[0], stand[1])
	if !s.tickWalkToward(a, mgl32.Vec3{stand[0], y, stand[1]}, dt) {
		return true
	}
	a.Heading = yaw + math.Pi // facing the rack
	a.Stash = world.SkiStash{Out: true, RackID: rack.ID, Slot: a.Stash.Slot, Pos: slot, Yaw: yaw}
	a.GearTimer = gearStopTime
	a.Speed = 0
	return true
}

// skisInSnow sticks a's skis in the snow beside them and stops a moment.
func (s *Simulation) skisInSnow(a *world.Guest) {
	right := mgl32.Vec2{float32(math.Cos(float64(a.Heading))), -float32(math.Sin(float64(a.Heading)))}
	a.Stash = world.SkiStash{Out: true, Pos: mgl32.Vec2{a.Pos[0], a.Pos[2]}.Add(right.Mul(0.5)), Yaw: a.Heading}
	a.GearTimer = gearStopTime
	a.Speed = 0
}

// fetchSkis walks a back to their skis and picks them up.
func (s *Simulation) fetchSkis(a *world.Guest, dt float64) bool {
	to := a.Stash.Pos
	if a.Stash.RackID != 0 {
		front := mgl32.Vec2{float32(math.Sin(float64(a.Stash.Yaw))), float32(math.Cos(float64(a.Stash.Yaw)))}
		to = to.Add(front.Mul(rackStandOff))
	}
	y := s.World.Terrain.InterpolatedSurfaceElevationAt(to[0], to[1])
	if !s.tickWalkToward(a, mgl32.Vec3{to[0], y, to[1]}, dt) {
		return true
	}
	a.Heading = a.Stash.Yaw + math.Pi
	a.Stash = world.SkiStash{}
	a.GearTimer = gearStopTime
	a.Speed = 0
	return true
}

// rackFor is the rack nearest door with a free slot, within
// world.SkiRackReach of it, and that slot; nil when there's none. a is
// the guest choosing, whose own reservation doesn't count against it.
func (s *Simulation) rackFor(door mgl32.Vec2, a *world.Guest) (*world.Building, int) {
	var best *world.Building
	bestD, bestSlot := float32(world.SkiRackReach), 0
	for _, b := range s.World.Buildings {
		if b.Type != world.BuildingSkiRack {
			continue
		}
		d := b.Pos.Sub(door).Len()
		if d >= bestD {
			continue
		}
		if slot, ok := s.freeRackSlot(b, a); ok {
			best, bestD, bestSlot = b, d, slot
		}
	}
	return best, bestSlot
}

// freeRackSlot is a slot in rack b nobody's skis are in or headed for.
func (s *Simulation) freeRackSlot(b *world.Building, self *world.Guest) (int, bool) {
	var taken [world.SkiRackPairs]bool
	for _, g := range s.World.OnMountain {
		if g != self && g.Stash.RackID == b.ID && g.Stash.Slot < world.SkiRackPairs {
			taken[g.Stash.Slot] = true
		}
	}
	for i, t := range taken {
		if !t {
			return i, true
		}
	}
	return 0, false
}

// stashOnEntry leaves the skis of a guest who got to the door still
// holding them (they were already close when the visit came up): in the
// rack slot they were heading for, else in the snow where they stand.
func (s *Simulation) stashOnEntry(a *world.Guest) {
	if !a.CarriesSkis() {
		return
	}
	if rack := s.World.BuildingByID(a.Stash.RackID); rack != nil && rack.Type == world.BuildingSkiRack {
		slot, yaw := rack.SkiRackSlot(a.Stash.Slot)
		a.Stash = world.SkiStash{Out: true, RackID: rack.ID, Slot: a.Stash.Slot, Pos: slot, Yaw: yaw}
		return
	}
	s.skisInSnow(a)
	a.GearTimer = 0
}

// SkisInRack is how many pairs of skis are in rack id now.
func (s *Simulation) SkisInRack(id uint64) int {
	n := 0
	for _, g := range s.World.OnMountain {
		if g.Stash.Out && g.Stash.RackID == id {
			n++
		}
	}
	return n
}
