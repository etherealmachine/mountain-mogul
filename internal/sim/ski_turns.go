package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Turning. A skier turns two ways:
//
//   - carving, at the rate their edge grip allows at speed (grip / speed,
//     a lateral acceleration), which costs nothing beyond edge friction;
//   - pivoting, up to pivotRate, by skidding the skis round: quick
//     manoeuvres, speed checks, and swerves around a trunk or another
//     skier. Turning faster than the carve rate skids, which sheds speed
//     (skidDecel) and, for the less skilled, balance (skidBalanceCost),
//     so a beginner who has to swerve hard may well fall.
//
// Linked turns carve, skidding more the faster the guest is going over
// their target (a speed check); swerves pivot.
const (
	carveGripBase  = 5.0 // m/s² lateral grip at skill 0
	carveGripSkill = 9.0 // added at skill 1
	pivotRateBase  = 60 * math.Pi / 180
	pivotRateSkill = 120 * math.Pi / 180
	// skidDecel is the speed shed a second while pivoting at the full
	// pivot rate beyond the carve rate.
	skidDecel = 6.0
	// skidBalanceCost is the balance a skill-0 skier loses a second while
	// pivoting at the full pivot rate; skill 1 loses none.
	skidBalanceCost = 1.2

	// Linked turns. On a slope (fall scale turnSlopeMin and up) a moving
	// skier links turns across the fall line. At target speed each turn
	// swings turnAmpBase off it, plus turnAmpUnskill for a beginner's
	// wide traverses; going faster widens it by turnAmpGain × overspeed
	// up to turnAmpMax, slower narrows it down to turnAmpMin. Each turn
	// holds at least turnDwell, shorter for the skilled.
	turnAmpBase    = 25 * math.Pi / 180
	turnAmpUnskill = 20 * math.Pi / 180
	turnAmpGain    = 1.5
	turnAmpMin     = 10 * math.Pi / 180
	turnAmpMax     = 75 * math.Pi / 180
	// turnOffMax caps the line picked round trees plus the turn's swing:
	// across the fall line, never back up the hill.
	turnOffMax = 90 * math.Pi / 180
	// skidTurnOver is the overspeed at which linked turns are fully
	// skidded (the pivot rate): a speed check.
	skidTurnOver   = 0.5
	turnMinSpeed   = 3.0 // m/s; below this they're skating, not turning
	turnSlopeMin   = 0.5
	turnDwellBase  = 1.5 // s at skill 0
	turnDwellSkill = 0.7 // taken off at skill 1

	// feltSlopeFloor: a skier across a slope still feels this share of
	// its full pitch.
	feltSlopeFloor = 0.6

	// Swerves. A trunk, tower or skier on the guest's line within
	// swerveLookSec (at least swerveMinDist) is dodged by pivoting to the
	// nearest heading that passes it at its clearance, checked against
	// every hazard in range; with no clear heading the guest stops hard
	// (swerveStopScrub, skill-scaled). Candidates are swerveSteps either
	// side of the current heading, up to swerveMaxOff.
	swerveLookSec   = 1.5
	swerveMinDist   = 5.0
	swerveMaxOff    = 80 * math.Pi / 180
	swerveSteps     = 8
	swerveStopScrub = 4.0 // m/s² at skill 0, ×2 at skill 1
	trunkClearance  = 1.0
	towerClearance  = 1.6
	skierClearance  = 1.5
)

// turnRates returns how fast the guest can carve at their speed and how
// fast they can pivot.
func turnRates(skill, speed float32) (carve, pivot float32) {
	skill = clamp32(skill, 0, 1)
	pivot = float32(pivotRateBase + pivotRateSkill*float64(skill))
	grip := float32(carveGripBase + carveGripSkill*float64(skill))
	carve = grip / max(speed, 1)
	return min(carve, pivot), pivot
}

// turnAmplitude is how far either side of their line the guest swings
// each turn: 0 on flat ground or when barely moving.
func turnAmplitude(skill float32, perc Perception, target float32) float32 {
	if perc.FallScale < turnSlopeMin || perc.Speed < turnMinSpeed || target < 0.01 {
		return 0
	}
	base := float32(turnAmpBase + turnAmpUnskill*float64(1-clamp32(skill, 0, 1)))
	return clamp32(base+float32(turnAmpGain)*(perc.Speed-target)/target, float32(turnAmpMin), float32(turnAmpMax))
}

// feltSlope is the pitch the guest feels: along their line, so a traverse
// is how a nervous skier copes with a steep pitch, though never less than
// feltSlopeFloor of the full pitch.
func feltSlope(perc Perception) float32 {
	if perc.FallScale <= 0 {
		return perc.SlopeAngle
	}
	hx, hz := float32(math.Sin(float64(perc.Heading))), float32(math.Cos(float64(perc.Heading)))
	along := abs32(hx*perc.FallDir[0] + hz*perc.FallDir[1])
	tanAlong := float32(math.Tan(float64(perc.SlopeAngle))) * along
	return max(float32(math.Atan(float64(tanAlong))), feltSlopeFloor*perc.SlopeAngle)
}

// turnDwell is the shortest the guest holds a turn before linking the
// next one.
func turnDwell(skill float32) float32 {
	return float32(turnDwellBase - turnDwellSkill*float64(clamp32(skill, 0, 1)))
}

// hazardPoint is something on the snow the guest must not ski into: its
// position and how far to pass it.
type hazardPoint struct {
	x, z, clear float32
}

// nearHazards gathers the trunks, towers and other skiers within reach
// of the guest's next swerveLookSec ahead (any direction they might
// swerve to) into buf.
func nearHazards(w *world.World, towers []mgl32.Vec2, grid *spatialGrid, a *world.Guest, reach float32, buf []hazardPoint) []hazardPoint {
	buf = buf[:0]
	px, pz := a.Pos[0], a.Pos[2]
	hx, hz := float32(math.Sin(float64(a.Heading))), float32(math.Cos(float64(a.Heading)))
	// Only what's not behind: a swerve can't reach past ±swerveMaxOff.
	behind := func(x, z float32) bool {
		return (x-px)*hx+(z-pz)*hz < -1
	}
	w.Terrain.ForEachTreeNear(px, pz, reach, func(tr world.Tree, _, _ int) {
		if a.HasTrunk && tr.X == a.Trunk[0] && tr.Z == a.Trunk[1] {
			return // the one they just hit; they're getting clear of it
		}
		if !behind(tr.X, tr.Z) {
			buf = append(buf, hazardPoint{tr.X, tr.Z, trunkClearance})
		}
	})
	r2 := reach * reach
	for _, p := range towers {
		dx, dz := p[0]-px, p[1]-pz
		if dx*dx+dz*dz <= r2 && !behind(p[0], p[1]) {
			buf = append(buf, hazardPoint{p[0], p[1], towerClearance})
		}
	}
	if grid != nil {
		// The grid answers within one bucket of a point: walk it along
		// the line ahead.
		n := len(buf)
		for d := float32(0); d <= reach; d += spatialCellSize {
			grid.forEachNear(px+hx*d, pz+hz*d, func(o *world.Guest) {
				if o.ID == a.ID {
					return
				}
				for _, h := range buf[n:] {
					if h.x == o.Pos[0] && h.z == o.Pos[2] {
						return
					}
				}
				if !behind(o.Pos[0], o.Pos[2]) {
					buf = append(buf, hazardPoint{o.Pos[0], o.Pos[2], skierClearance})
				}
			})
		}
	}
	return buf
}

// arcRun is how far the guest gets, turning from heading h0 toward h1
// at rate (rad/s) while moving at speed, before the corridor they sweep
// (each hazard's clearance either side) meets a hazard; reach when it
// doesn't within reach.
func arcRun(px, pz, h0, h1, rate, speed, reach float32, hazards []hazardPoint) float32 {
	const step = float32(0.75)
	turn := rate * step / max(speed, 1) // rad per step
	h := h0
	for d := float32(0); d < reach; d += step {
		h = wrapAngle(h + clamp32(wrapAngle(h1-h), -turn, turn))
		hx, hz := float32(math.Sin(float64(h))), float32(math.Cos(float64(h)))
		for _, p := range hazards {
			dx, dz := p.x-px, p.z-pz
			fwd := dx*hx + dz*hz
			if fwd < 0 || fwd >= step {
				continue
			}
			if lat := dx*hz - dz*hx; lat < p.clear && lat > -p.clear {
				return d + fwd
			}
		}
		px += hx * step
		pz += hz * step
	}
	return reach
}

// swerve checks the line the guest is turning onto (at carve rate) for a
// hazard within swerveLookSec. With it blocked it returns the heading
// whose arc, pivoting there, runs clear and ends nearest to the one they
// want (urgent), or, with no clear arc, the one that runs longest before
// a hazard and stop = true.
func swerve(w *world.World, towers []mgl32.Vec2, grid *spatialGrid, a *world.Guest, desired, carveRate, pivotRate float32, buf *[]hazardPoint) (heading float32, urgent, stop bool) {
	reach := max(float32(swerveMinDist), a.Speed*float32(swerveLookSec))
	*buf = nearHazards(w, towers, grid, a, reach, *buf)
	hazards := *buf
	if len(hazards) == 0 {
		return desired, false, false
	}
	px, pz := a.Pos[0], a.Pos[2]
	if arcRun(px, pz, a.Heading, desired, carveRate, a.Speed, reach, hazards) >= reach {
		return desired, false, false
	}
	best, bestScore := float32(0), float32(math.Inf(1))
	longest, longestRun := a.Heading, float32(-1)
	step := float32(swerveMaxOff) / swerveSteps
	for i := -swerveSteps; i <= swerveSteps; i++ {
		h := wrapAngle(a.Heading + float32(i)*step)
		run := arcRun(px, pz, a.Heading, h, pivotRate, a.Speed, reach, hazards)
		if run > longestRun {
			longest, longestRun = h, run
		}
		if run < reach {
			continue
		}
		// Nearest to where they want to go, then least turning.
		score := abs32(wrapAngle(h-desired)) + 0.5*abs32(float32(i)*step)
		if score < bestScore {
			best, bestScore = h, score
		}
	}
	if !math.IsInf(float64(bestScore), 1) {
		return best, true, false
	}
	return longest, true, true
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
