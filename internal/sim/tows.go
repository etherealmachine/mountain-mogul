package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// Riding a tow (a rope tow or a T-bar): riders slide up a track on their
// skis holding the rope or the bar, packing the snow under them, and some
// fall off on the way. A
// rider who falls lets go, goes down beside the track, and once up again
// plans afresh from there: ski down, or back to the line.

// Rope tow and T-bar riders fall off on a ride with these chances, by
// skill tier: beginner, intermediate, advanced, anywhere along the track
// (a long T-bar isn't many times worse than a short one: whoever's going
// to come off mostly does). Novices don't ride tows (goap
// liftAccessible).
var (
	ropeFallPerRide = [3]float32{0.20, 0.04, 0.005}
	tbarFallPerRide = [3]float32{0.25, 0.05, 0.01}
)

// towSafeEnds is how many metres past where riders take hold, and short
// of where they let go, nobody falls: finding their feet, and the flat
// at the top.
const towSafeEnds = 4

// towFallChance is the chance a rider at skill falls off lift over
// metres of a track track metres long.
func towFallChance(lift *world.Lift, skill, metres, track float32) float32 {
	per := ropeFallPerRide
	if lift.Type.Hangers() {
		per = tbarFallPerRide
	}
	tier := 0
	switch {
	case skill >= ai.SkillAdvancedThreshold:
		tier = 2
	case skill >= ai.SkillIntermediateThreshold:
		tier = 1
	}
	return per[tier] * metres / max(track, 1)
}

// tickTowRiders packs the track under a tow's riders on chair, which
// moved metres this tick, and has some of them fall off.
func (s *Simulation) tickTowRiders(lift *world.Lift, chair *world.Chair, metres float32, dt float64) {
	if metres <= 0 {
		return
	}
	dir := lift.Top.Sub(lift.Base)
	length := dir.Len()
	if length < 1 {
		return
	}
	dir = dir.Mul(1 / length)
	safe := lift.Station().LoadAhead + towSafeEnds
	for j, a := range chair.Passengers {
		if a == nil {
			continue
		}
		packUnderfoot(s.World.Terrain, a.Pos, dt)
		along := mgl32.Vec2{a.Pos[0], a.Pos[2]}.Sub(lift.Base).Dot(dir)
		if along < safe || along > length-safe {
			continue
		}
		if rng.Global().Float32() < towFallChance(lift, a.Traits.Skill, metres, length-2*safe) {
			s.fallOffTow(a, lift, chair, j, dir)
		}
	}
}

// fallOffTow takes a from seat j of chair, a place on tow lift, and
// puts them down beside the track, up-line dir, with no plan: they make
// a fresh one when they're up.
func (s *Simulation) fallOffTow(a *world.Guest, lift *world.Lift, chair *world.Chair, j int, dir mgl32.Vec2) {
	chair.Passengers[j] = nil
	a.OnLiftID = 0
	s.dropBoarder(a)
	// Out from the rope: riders ride on ChairPos's +perp side.
	perp := mgl32.Vec2{-dir[1], dir[0]}
	a.Pos[0] += perp[0] * 0.6
	a.Pos[2] += perp[1] * 0.6
	a.Pos[1] = s.World.Terrain.InterpolatedSurfaceElevationAt(a.Pos[0], a.Pos[2])
	a.Heading = float32(math.Atan2(float64(dir[0]), float64(dir[1])))
	a.Speed = lift.Speed
	a.TurnSide = 0
	a.Plan.Steps = nil
	s.knockDown(a, world.FallSide, world.FallLift)
	s.applyEvent(a, ai.ThoughtFellOffTow, lift.ID)
}

// dropBoarder takes a off the boarders waiting for a post-ride plan.
func (s *Simulation) dropBoarder(a *world.Guest) {
	for i := 0; i < len(s.boarders); {
		if s.boarders[i].a == a {
			s.boarders = append(s.boarders[:i], s.boarders[i+1:]...)
			continue
		}
		i++
	}
}
