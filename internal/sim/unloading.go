package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// Unloading is a rider getting off a chair at the top station: they stand
// up where their seat is, glide straight ahead onto the apron (away from
// the cable), peel off to their seat's side, and only then hand over to
// normal skiing. The path is scripted, so balance stays full and nothing
// steers them over the station. Falls getting off are a roll when they
// stand up (unloadFallChance), not the balance model.

const (
	unloadSpeed     = float32(2.5)                // m/s down the unload ramp
	unloadStraight  = float32(4)                  // metres straight ahead before peeling off
	unloadPeel      = float32(6)                  // metres of arc away from the lift line
	unloadPeelOuter = float32(70 * math.Pi / 180) // turn by the end of the arc, outermost seats
	unloadPeelInner = float32(30 * math.Pi / 180) // turn for seats next to the chair's middle
)

// startUnloading takes a rider off the chair at the top of lift: placed
// on the ground under their seat, facing up the lift line, with the side
// they'll peel to from where the seat sits across the chair.
func (s *Simulation) startUnloading(a *world.Guest, lift *world.Lift, slotIdx int) {
	w := s.World
	chairPos, heading := lift.ChairPos(0.4999, w.Terrain)
	slots := world.SlotsFor(lift.Type.MeshID())
	seat := seatWorldPos(chairPos, heading, slotIdx, slots)
	ground := w.Terrain.InterpolatedSurfaceElevationAt(seat[0], seat[2])
	a.Pos = mgl32.Vec3{seat[0], ground, seat[2]}
	a.Heading = heading
	a.Speed = unloadSpeed
	a.TurnSide = 0
	a.Balance = 1
	a.Unload = world.Unloading{
		LiftID: lift.ID,
		Side:   seatSide(slotIdx, len(lift.Chairs[0].Passengers), slots),
		SeatY:  max(seat[1]-ground, 0),
	}
	if rng.Global().Float32() < unloadFallChance(a.Traits.Skill, lift.Type) {
		// Somewhere between finding their feet and the middle of the peel.
		a.Unload.FallAt = 1 + rng.Global().Float32()*(unloadStraight+unloadPeel/2-1)
	}
}

// unloadFallChance is the chance a rider falls getting off: beginners
// most, and fixed-grip chairs, which don't slow at the top, more than
// detachables. Gondola riders walk off.
func unloadFallChance(skill float32, t world.LiftType) float32 {
	var chance [3]float32 // beginner, intermediate, advanced
	switch t {
	case world.LiftDouble, world.LiftFixedTriple, world.LiftFixedQuad:
		chance = [3]float32{0.03, 0.005, 0.001}
	case world.LiftHSQuad, world.LiftHS6Pack:
		chance = [3]float32{0.01, 0.002, 0}
	default:
		return 0
	}
	switch {
	case skill >= ai.SkillAdvancedThreshold:
		return chance[2]
	case skill >= ai.SkillIntermediateThreshold:
		return chance[1]
	}
	return chance[0]
}

// seatSide is where a seat sits across its chair, from -1 (one edge) to
// +1 (the other), in the same sense as ChairPos's perpendicular. From the
// mesh's seat slots when the renderer has registered them (their local Z
// runs across the chair), else from the seat's index.
func seatSide(slotIdx, seats int, slots []world.MeshSlot) float32 {
	if slotIdx < len(slots) {
		widest := float32(0)
		for _, sl := range slots {
			widest = max(widest, float32(math.Abs(float64(sl.Pos[2]))))
		}
		if widest > 0 {
			return slots[slotIdx].Pos[2] / widest
		}
	}
	if seats < 2 {
		return 0
	}
	return 2*float32(slotIdx)/float32(seats-1) - 1
}

// tickUnloading moves an unloading rider one step along their path:
// straight ahead, then an arc that turns them toward their side, ending
// in normal skiing at the arc's heading and speed.
func (s *Simulation) tickUnloading(a *world.Guest, dt float64) {
	u := &a.Unload
	if u.FallAt > 0 && u.Gone >= u.FallAt {
		s.fallUnloading(a)
		return
	}
	step := unloadSpeed * float32(dt)
	if u.Gone >= unloadStraight || s.fallenAhead(a) {
		side := u.Side
		turn := unloadPeelInner + (unloadPeelOuter-unloadPeelInner)*float32(math.Abs(float64(side)))
		if side < 0 {
			turn = -turn
		}
		// ChairPos's perpendicular is (-cos h, sin h): turning toward +side
		// means a smaller heading.
		a.Heading = wrapAngle(a.Heading - turn*step/unloadPeel)
	}
	a.Pos[0] += float32(math.Sin(float64(a.Heading))) * step
	a.Pos[2] += float32(math.Cos(float64(a.Heading))) * step
	a.Pos[1] = s.World.Terrain.InterpolatedSurfaceElevationAt(a.Pos[0], a.Pos[2])
	a.Speed = unloadSpeed
	a.Balance = 1
	u.Gone += step
	if u.Gone >= unloadStraight+unloadPeel {
		a.Unload = world.Unloading{}
	}
}

// fallUnloading puts a rider down in the unload zone. They lie there for
// the usual fall, through tickFallen, then finish getting off: the
// unloading state stays, with the fall spent.
func (s *Simulation) fallUnloading(a *world.Guest) {
	a.Unload.FallAt = 0
	a.Speed = 0
	s.knockDown(a, world.FallSide)
	s.applyEvent(a, ai.ThoughtFellUnloading, a.Unload.LiftID)
}

// fallenAhead reports whether someone is down in the unload zone just
// ahead of a rider, who then peels off early to get around them.
func (s *Simulation) fallenAhead(a *world.Guest) bool {
	const reach = float32(4)
	fwd := mgl32.Vec2{float32(math.Sin(float64(a.Heading))), float32(math.Cos(float64(a.Heading)))}
	ahead := false
	s.spatial.forEachNear(a.Pos[0], a.Pos[2], func(o *world.Guest) {
		if o == a || !o.Fallen || o.Unload.LiftID == 0 {
			return
		}
		d := mgl32.Vec2{o.Pos[0] - a.Pos[0], o.Pos[2] - a.Pos[2]}
		if along := d.Dot(fwd); along > 0 && along < reach && d.Len() < reach {
			ahead = true
		}
	})
	return ahead
}
