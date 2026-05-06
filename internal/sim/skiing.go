package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// Steering / skiing-behavior tuning constants.
const (
	ComfortSpeed     = 12.0                 // m/s; skiers want to stay near this
	SkiWalkSpeed     = 2.0                  // m/s; floor — skiers skate/pole rather than stopping
	MuEdge           = 0.4                  // carving friction when traversing
	MaxAngularSpeed  = 90.0 * math.Pi / 180 // rad/s; cap on heading rotation
	ProbeDistance    = 8.0                  // m; forward probe length for tree avoidance
	ProbeAngle       = 35.0 * math.Pi / 180 // rad; ±offset for side probes
	TurnAngle        = 60.0 * math.Pi / 180 // rad; how far off the fall line an S-turn pushes
	ArrivalRadius    = 5.0                  // m; within this distance, steer pure-seek (no fall/avoid)
	ArrivalThreshold = 2.0                  // m; once inside, the agent has reached the target
)

// fallLine returns the unit horizontal direction of steepest descent
// derived from the surface normal. If the slope is too flat to derive
// a meaningful direction, returns the zero vector.
func fallLine(normal mgl32.Vec3) mgl32.Vec2 {
	v := mgl32.Vec2{normal[0], normal[2]}
	l := v.Len()
	if l < 1e-4 {
		return mgl32.Vec2{}
	}
	return v.Mul(1.0 / l)
}

// treeAvoidance computes a steering vector that pushes the agent away
// from forested terrain. It samples TreeDensity at three forward probes
// (centre, ±ProbeAngle) and contributes lateral steering toward the
// less-dense side. Returns a 2D world-XZ vector; magnitude grows with
// how dense the trees are.
//
// Convention: heading angle h satisfies (x, z) = (sin h, cos h). Increasing
// h rotates from +Z toward +X. With Y up and +Z forward, +X is the agent's
// right. So a probe at +ProbeAngle samples to the right of the agent, and
// the "right" perpendicular is (hz, -hx).
func treeAvoidance(t *world.Terrain, pos mgl32.Vec3, heading float32) mgl32.Vec2 {
	hx := float32(math.Sin(float64(heading)))
	hz := float32(math.Cos(float64(heading)))
	rx := hz  // right perpendicular
	rz := -hx

	probeDensity := func(angle float64) float32 {
		c := float32(math.Cos(angle))
		s := float32(math.Sin(angle))
		// rotate heading by +angle (toward +X = right)
		dx := c*hx + s*rx
		dz := c*hz + s*rz
		return t.TreeDensityAt(pos[0]+dx*ProbeDistance, pos[2]+dz*ProbeDistance)
	}

	dC := probeDensity(0)
	dR := probeDensity(ProbeAngle)
	dL := probeDensity(-ProbeAngle)

	// (dL - dR) > 0  → trees denser on left → push right.
	// (dL - dR) < 0  → trees denser on right → push left.
	push := mgl32.Vec2{rx, rz}.Mul(dL - dR)

	// If the centre probe is in trees, add lateral push toward the clearer side.
	if dC > 0.1 {
		var sideMul float32
		if dL > dR {
			sideMul = 1 // push right
		} else if dR > dL {
			sideMul = -1 // push left
		}
		push = push.Add(mgl32.Vec2{rx, rz}.Mul(dC * sideMul))
	}
	return push
}

// desiredHeading combines the steering forces into a target heading angle.
// `seek`, `fall`, `avoid` are 2D XZ vectors (need not be unit). `turnPhase`
// is the agent's current S-turn side, mutated when a phase flip is needed.
// Returns the desired heading angle in radians (atan2 convention used by
// the rest of the sim: angle from +Z, increasing toward +X).
func desiredHeading(seek, fall, avoid mgl32.Vec2, speed, comfort float32, turnPhase *int8) float32 {
	const (
		wSeek  = 0.5
		wFall  = 0.6
		wAvoid = 1.2
		wTurn  = 1.4
	)

	seekN := normalize2(seek)
	fallN := normalize2(fall)

	desired := seekN.Mul(wSeek).Add(fallN.Mul(wFall)).Add(avoid.Mul(wAvoid))

	// S-turn: if speeding, rotate desired off the fall line by TurnAngle.
	if speed > comfort && fallN.Len() > 0 {
		if *turnPhase == 0 {
			*turnPhase = 1
		}
		// Rotate the fall line by ±TurnAngle (lateral perpendicular).
		// Perpendicular of fallN (in XZ) for left/right.
		px, pz := -fallN[1], fallN[0] // 90° CCW
		side := float32(*turnPhase)
		turnVec := mgl32.Vec2{px * side, pz * side}
		desired = desired.Add(turnVec.Mul(wTurn))

		// Phase flip: when current heading aligns with current desired off-fall direction
		// (i.e. the skier has reached the cross-fall traverse), kick the phase to the
		// other side. Simple proxy: flip when seek direction has rotated past fall line
		// on the current side. We use a cheap test: dot(seek, perp) sign change.
		seekDotPerp := seekN[0]*px + seekN[1]*pz
		if seekDotPerp*float32(*turnPhase) < -0.2 {
			*turnPhase = -*turnPhase
		}
	} else {
		*turnPhase = 0
	}

	if desired.Len() < 1e-4 {
		// Nothing to steer toward; fall back to seek.
		desired = seekN
		if desired.Len() < 1e-4 {
			return 0
		}
	}
	return float32(math.Atan2(float64(desired[0]), float64(desired[1])))
}

// rotateToward rotates `current` toward `desired` by at most maxRate*dt radians.
// Both angles use the sim convention: atan2(x, z).
func rotateToward(current, desired, maxRate float32, dt float64) float32 {
	diff := wrapAngle(desired - current)
	step := float32(float64(maxRate) * dt)
	if diff > step {
		diff = step
	} else if diff < -step {
		diff = -step
	}
	return wrapAngle(current + diff)
}

// wrapAngle wraps a radian angle into [-π, π].
func wrapAngle(a float32) float32 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

// SteeringDebug describes the steering forces acting on a skiing agent
// for a single frame. Returned by ComputeSteeringDebug for the debug overlay.
type SteeringDebug struct {
	Pos         mgl32.Vec3
	FallLine    mgl32.Vec2 // unit XZ vector down the fall line (zero if flat)
	DesiredHead float32    // desired heading angle in radians
	Probes      [3]struct {
		Dir     mgl32.Vec2 // unit direction of probe (centre, right, left)
		Density float32
	}
}

// ComputeSteeringDebug runs the steering calculation for `agent` toward
// `target` without mutating the agent. Used by the debug overlay so the
// renderer can visualise fall line, desired heading, and probe density.
func ComputeSteeringDebug(w *world.Terrain, agent *world.Agent, target mgl32.Vec3) SteeringDebug {
	delta := target.Sub(agent.Pos)
	normal := w.NormalAt(agent.Pos[0]/CellSize, agent.Pos[2]/CellSize)
	fall := fallLine(normal)
	avoid := treeAvoidance(w, agent.Pos, agent.Heading)

	// Read TurnPhase non-mutating: copy and discard.
	phase := agent.TurnPhase
	desired := desiredHeading(
		mgl32.Vec2{delta[0], delta[2]},
		fall,
		avoid,
		agent.Speed, ComfortSpeed,
		&phase,
	)

	// Re-derive the three probe directions in world XZ.
	hx := float32(math.Sin(float64(agent.Heading)))
	hz := float32(math.Cos(float64(agent.Heading)))
	rx := hz
	rz := -hx
	probeDir := func(angle float64) mgl32.Vec2 {
		c := float32(math.Cos(angle))
		s := float32(math.Sin(angle))
		return mgl32.Vec2{c*hx + s*rx, c*hz + s*rz}
	}

	out := SteeringDebug{
		Pos:         agent.Pos,
		FallLine:    fall,
		DesiredHead: desired,
	}
	angles := [3]float64{0, ProbeAngle, -ProbeAngle}
	for i, ang := range angles {
		d := probeDir(ang)
		out.Probes[i].Dir = d
		out.Probes[i].Density = w.TreeDensityAt(
			agent.Pos[0]+d[0]*ProbeDistance,
			agent.Pos[2]+d[1]*ProbeDistance,
		)
	}
	return out
}

func normalize2(v mgl32.Vec2) mgl32.Vec2 {
	l := v.Len()
	if l < 1e-4 {
		return mgl32.Vec2{}
	}
	return v.Mul(1.0 / l)
}
