package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// Unloading is a rider getting off a chair at the top station: they stand
// up where their seat is, under the front of the terminal's hood, find
// their feet for a moment, then peel off sideways (toward the trail
// they're about to ski, or their seat's side) in front of the portal
// frame, out past its legs and the hut, and round onto the level apron
// beside and behind the station (scene: carveStationApron), where they
// turn toward their run and hand over to normal skiing. Ground below the
// front of a top station is often a steep face; leaving from the apron
// gives a skier the run of the top before they pick a way down. The path
// is scripted and never reaches under the frame, drive or hut
// (world.Lift.StationBoxes), so balance stays full and nothing steers
// them through the station. Falls getting off are a roll when they stand
// up (unloadFallChance), not the balance model.

const (
	unloadSpeed    = float32(2.5) // m/s down the unload ramp
	unloadStraight = float32(0.2) // metres straight ahead, finding their feet
	// The peel turns unloadPeelRate rad a metre: 1 m radius, so a
	// quarter turn sideways ends 1.2 m back from the bullwheel, in front
	// of the frame's legs and the hut (2.2 m back).
	unloadPeelRate = float32(1)
	unloadClear    = float32(7) // metres across the line from the bullwheel: past the legs and the hut
	// Crossing out they angle unloadCrossDown down the line from
	// sideways, so a rider from the seat furthest back, or one crossing
	// the whole width to the far side, keeps in front of the frame.
	unloadCrossDown = float32(15 * math.Pi / 180)
	// Out beside the hut they head back along the line, unloadApronTurn
	// off it and away from the station, until unloadApronBack metres
	// back from the bullwheel: on the apron, level with the hut's far end.
	unloadApronTurn = float32(20 * math.Pi / 180)
	unloadApronBack = float32(6)
	unloadMaxGone   = float32(40)                 // the path ends by here whatever: a rider never stays scripted
	unloadSubstep   = float32(0.2)                // most metres moved at once along the path
	unloadFallSpan  = float32(3)                  // falls come within this many metres past the first
	unloadTurnSeat  = float32(90 * math.Pi / 180) // heading at the end with no trail to aim for: across the slope
	// Aiming for a trail: the heading at the end is the trail's direction
	// from where they stand, between unloadAimMin and unloadAimMax off
	// the line (the way back past the station, so 0 is straight on and
	// π straight down the line). The aim is unloadAimAlong metres down
	// the trail's centre line from its point nearest the top.
	unloadAimMin   = float32(30 * math.Pi / 180)
	unloadAimMax   = float32(150 * math.Pi / 180)
	unloadAimAlong = float32(20)
)

// startUnloading takes a rider off the chair at the top of lift: placed
// on the ground under their seat, facing up the lift line, with the side
// they'll peel to from where the seat sits across the chair.
func (s *Simulation) startUnloading(a *world.Guest, lift *world.Lift, slotIdx int) {
	w := s.World
	chairPos, heading := lift.ChairPos(lift.UnloadProgress(), w.Terrain)
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
	// By default, toward the seat's side. ChairPos's perpendicular is
	// (-cos h, sin h): turning toward +side means a smaller heading. Turn
	// is the heading at the end against the one they stood up with.
	side := a.Unload.Side
	turn := unloadTurnSeat
	if side > 0 || side == 0 && slotIdx%2 == 0 {
		turn = -turn
	}
	a.Unload.Turn = turn
	if rng.Global().Float32() < unloadFallChance(a.Traits.Skill, lift.Type) {
		// Somewhere between finding their feet and getting clear.
		a.Unload.FallAt = 1 + rng.Global().Float32()*unloadFallSpan
	}
}

// aimUnload turns a rider's peel toward where their plan sends them
// first, once the plan has moved on from the ride: into the trail they'll
// ski, else the next step's target. The side is the aim's side of where
// they stand; how far they turn, the aim's direction, kept between
// unloadAimMin and unloadAimMax.
func (s *Simulation) aimUnload(a *world.Guest) {
	aim, ok := s.unloadAim(a)
	if !ok {
		return
	}
	want := float32(math.Atan2(float64(aim[0]-a.Pos[0]), float64(aim[1]-a.Pos[2])))
	d := wrapAngle(want - a.Heading)
	mag := clamp32(float32(math.Abs(float64(d))), unloadAimMin, unloadAimMax)
	if d < 0 {
		mag = -mag
	}
	a.Unload.Turn = mag
}

// unloadAim is the point a rider getting off makes for.
func (s *Simulation) unloadAim(a *world.Guest) (mgl32.Vec2, bool) {
	pos := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	if id := plannedTrail(a); id != 0 {
		if t := s.World.FindTrail(id); t != nil {
			if line := t.Centerline(); len(line) > 0 {
				near, best := 0, float32(math.Inf(1))
				for i, p := range line {
					if d := p.Pos.Sub(pos).Len(); d < best {
						near, best = i, d
					}
				}
				// Down the trail: toward whichever end is further from here.
				step := 1
				if line[0].Pos.Sub(pos).Len() > line[len(line)-1].Pos.Sub(pos).Len() {
					step = -1
				}
				i, gone := near, float32(0)
				for gone < unloadAimAlong && i+step >= 0 && i+step < len(line) {
					gone += line[i+step].Pos.Sub(line[i].Pos).Len()
					i += step
				}
				return line[i].Pos, true
			}
			if x, z, ok := t.NearestCellCenter(pos[0], pos[1]); ok {
				return mgl32.Vec2{x, z}, true
			}
		}
	}
	if p, ok := planTargetWorldPos(s.World, a); ok {
		return mgl32.Vec2{p[0], p[2]}, true
	}
	return mgl32.Vec2{}, false
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
// straight ahead to find their feet, then peeling toward their side at
// unloadPeelRate to sideways (a little down the line, unloadCrossDown)
// until they're unloadClear across the line, back beside the station
// until unloadApronBack behind the bullwheel, then round to their final
// heading (Turn), ending in normal skiing at that heading and speed. The step is walked in pieces of at most
// unloadSubstep, so a long tick still follows the path.
func (s *Simulation) tickUnloading(a *world.Guest, dt float64) {
	u := &a.Unload
	lift := findLiftByID(s.World, u.LiftID)
	if lift == nil {
		a.Unload = world.Unloading{}
		return
	}
	at, back, side, ok := lift.StationFrame(true)
	if !ok {
		a.Unload = world.Unloading{}
		return
	}
	backHeading := float32(math.Atan2(float64(back[0]), float64(back[1])))
	t := s.World.Terrain
	defer func() { a.Pos[1] = t.InterpolatedSurfaceElevationAt(a.Pos[0], a.Pos[2]) }()
	a.Speed = unloadSpeed
	a.Balance = 1
	sign := float32(1)
	if u.Turn < 0 {
		sign = -1
	}
	for left := unloadSpeed * float32(dt); left > 0; {
		if u.FallAt > 0 && u.Gone >= u.FallAt {
			s.fallUnloading(a)
			return
		}
		step := min(left, unloadSubstep)
		left -= step
		d := mgl32.Vec2{a.Pos[0], a.Pos[2]}.Sub(at)
		clear := abs32(d.Dot(side)) >= unloadClear
		onApron := clear && d.Dot(back) >= unloadApronBack
		turned := wrapAngle(a.Heading - backHeading)
		if onApron && abs32(u.Turn-turned) < 0.05 || u.Gone >= unloadMaxGone {
			a.Unload = world.Unloading{}
			return
		}
		if u.Gone >= unloadStraight || s.fallenAhead(a) {
			want := u.Turn
			switch {
			case !clear:
				want = sign * (math.Pi/2 + unloadCrossDown) // out sideways first
			case !onApron:
				want = sign * unloadApronTurn
			}
			turn := unloadPeelRate * step
			a.Heading = wrapAngle(a.Heading + clamp32(want-turned, -turn, turn))
		}
		a.Pos[0] += float32(math.Sin(float64(a.Heading))) * step
		a.Pos[2] += float32(math.Cos(float64(a.Heading))) * step
		u.Gone += step
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
