package sim

import (
	"math"
	"math/rand"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// =============================================================================
// SKIER CONTROLLER — Plan A
//
// Continuous steering, no technique enum. Every tick:
//
//   perceive → small typed bundle: slope, fall-line, axis to target
//   decide   → desired heading + scrub (the controller; no per-mode state)
//   apply    → rate-cap heading, integrate physics
//
// S-turns emerge from the controller, not from a programmed oscillator: when
// the skier exceeds comfort speed, we command a heading rotated off the fall
// line by an angle that grows with overspeed; this engages the existing
// edge-friction term in the physics step and scrubs energy. A persistent
// TurnSide flips when heading reaches the committed-side arc edge, producing
// linked turns whose amplitude and period are dynamic functions of speed
// error rather than tuned constants.
//
// Tree and boundary avoidance is predictive: the controller samples a small
// fan of candidate trajectories ~2 s ahead, scores each by tree density and
// off-map penalty, and picks the lateral offset that wins. There is no side-
// commit state machine — the scoring is monotonic in the projected world.
//
// Persistent per-agent state: Plan (where), Balance (when do I fall),
// TurnSide (carving left/right), Energy (session fatigue). That's it.
// =============================================================================

// =============================================================================
// SECTION 1 — Tunables
// =============================================================================

const (
	// Slope physics
	gravity = 9.81
	muBase  = 0.04 // base kinetic friction (groomed snow)
	muEdge  = 0.20 // perpendicular (carving) friction — the brake mechanism
	kDrag   = 0.01 // air drag per unit mass

	// Motion floors / arrival
	skiWalkSpeed     = 2.0 // m/s; minimum forward motion (skating/poling)
	ArrivalRadius    = 6.0 // m; switch to direct-pointing inside this
	ArrivalThreshold = 2.0 // m; sim considers the agent "there"

	// Heading rotation. Single rate cap, applied uniformly. Capped at the
	// rate a real skier can transfer their weight from one edge to the
	// other — turns much faster than this read as "robotic" because no
	// human could initiate that kind of direction change without losing
	// balance.
	headingRateMax = 40 * math.Pi / 180 // rad/s

	// Minimum dwell on a committed turn side before the controller is
	// allowed to flip. Models the body-weight commitment phase of a carve:
	// you can't keep flipping edges every 0.3 s without falling. Combined
	// with the heading rate cap, this puts each carve at ~1.2 s minimum
	// and a full S-cycle at ~2.4 s — cruising rhythm, not slalom.
	turnDwellMin = 1.2 // sec

	// Speed-control braking. brakeAngle = clamp(overspeed × gain, 0, max),
	// where overspeed = (speed − target)/target. A skier at 33% above target
	// reaches brakeAngleMax; above that, scrub kicks in too.
	brakeAngleMax     = 40 * math.Pi / 180 // rad; max heading offset for braking
	brakeAngleGain    = 1.5
	brakeMinForCommit = 8 * math.Pi / 180 // below this, drop turn-side commit and let heading relax to axis

	// Forward sampling for trees + boundaries. Horizon is generous because
	// real obstacles (a 60 m grove, a wall) are wider than the skier's
	// reactive distance — at 14 m/s with horizon=28 m the skier sees the
	// patch with only 2 s to maneuver, not enough lateral budget. 3.5 s of
	// lookahead with a 70 m cap gives the controller time to actually
	// commit to a side.
	sampleHorizonSec = 3.5
	sampleMinDist    = 22.0
	sampleMaxDist    = 70.0
	sampleSegments   = 8
	sampleAngleMax   = 60 * math.Pi / 180
	sampleCount      = 7
	treePenalty      = 4.0
	boundaryPenalty  = 8.0
	progressBonus    = 0.3 // weight on cos(offset) — small so wider clearances aren't outvoted by "stay on axis"
	sideCommitBonus  = 0.4 // weight on sign(prevTactical)·sign(offset) — biases toward the side already chosen so symmetric obstacles don't flip-flop

	// Width of the corridor the sampler treats as "the skier" when
	// integrating tree density along a candidate path. Each segment reads
	// density at the centre AND at ±corridorHalfWidth perpendicular, taking
	// the worst — so a path that grazes a tree edge scores as poorly as
	// one through the trunk. A 10 m half-width keeps the skier ~2 cells
	// off any dense cell.
	corridorHalfWidth = 10.0


	// Fall-line attenuation. Identical to prior model — gentle terrain has
	// noisy gradients, so we ignore the fall direction below flatSlopeL.
	flatSlopeL  = 0.05
	steepSlopeL = 0.20

	// Balance / fall
	fallRecoverTime  = 4.0
	fallStartBalance = 0.7

	// Tree underfoot signal (display-only — controller doesn't branch on it)
	inTreesThreshold = 0.3

	// Energy budget. Drains at a flat rate per sim-second of active skiing
	// (lift rides + walks don't count). Below energyLowThreshold, decision
	// boundaries (lift unload, lift base arrival) reroute to a lodge.
	// Calibrated for ~20 descents at ~40 s each.
	energyBudgetSec    = 800.0
	energyLowThreshold = 0.05
)

// =============================================================================
// SECTION 2 — Per-tick types
// =============================================================================

// Perception is the small typed bundle decide() reads. Fresh per tick;
// never stored.
type Perception struct {
	Pos     mgl32.Vec3
	Heading float32
	Speed   float32

	Normal     mgl32.Vec3
	SlopeAngle float32 // rad at the agent's position
	FallDir    mgl32.Vec2
	FallScale  float32 // smoothstepped slope strength [0, 1]

	AxisDir   mgl32.Vec2 // unit XZ vector toward the current target
	AxisDist  float32
	InArrival bool

	AtCellDensity float32
	InTrees       bool
}

// Decision is what the controller emits each tick. Consumed by apply().
type Decision struct {
	DesiredHeading float32
	Scrub          float32 // m/s² active deceleration (beyond passive friction)

	// Diagnostics — propagated to Sense / RecorderFrame.
	AxisHeading    float32
	TacticalOffset float32
	Brake          float32 // commanded brakeAngle (rad)
	TurnSide       int8
	TargetSpeed    float32
	Mode           string

	ProbeC, ProbeR, ProbeL float32
}

// =============================================================================
// SECTION 3 — Tick orchestration
// =============================================================================

// tickSkier runs one frame of the controller against `target`. Returns true
// when the agent has arrived (within ArrivalThreshold).
func (s *Simulation) tickSkier(a *world.Agent, target mgl32.Vec3, dt float64) bool {
	delta := target.Sub(a.Pos)
	dist := delta.Len()
	if dist < ArrivalThreshold {
		a.Pos = target
		return true
	}

	// Plan refresh: only on goal change. The strategic layer is intentionally
	// thin in Plan A — this is where future "explore a new run / weigh
	// conditions" logic will hook in, not the per-tick controller.
	if a.Plan.Target != target {
		a.Plan = ai.Plan{
			Goal:   planGoalFor(s.World, a),
			GoalID: a.TargetID,
			Target: target,
		}
	}

	perc := perceive(s.World.Terrain, a, target)
	dec := decide(s.World.Terrain, a, perc, float32(dt), s.Rng)
	if dec.TurnSide != a.TurnSide {
		a.TurnDwell = 0
	} else {
		a.TurnDwell += float32(dt)
	}
	a.TurnSide = dec.TurnSide
	a.LastTactical = dec.TacticalOffset
	a.Sense = senseFrom(perc, dec)

	// Energy drain (active skiing only).
	if a.Energy > 0 {
		a.Energy -= float32(dt / energyBudgetSec)
		if a.Energy < 0 {
			a.Energy = 0
		}
	}

	// Balance + fall.
	a.Balance += stressDelta(a.Traits, perc, dec) * float32(dt)
	if a.Balance > 1 {
		a.Balance = 1
	}
	if a.Balance <= 0 {
		a.Balance = 0
		a.Fallen = true
		a.FallTimer = float32(fallRecoverTime)
		a.Speed = 0
		recordFrame(s, a, target, dist, perc, dec)
		return false
	}

	apply(s.World.Terrain, a, dec, perc, dt)
	recordFrame(s, a, target, dist, perc, dec)
	return false
}

// tickFallen counts the agent down out of the fallen window and resumes.
func (s *Simulation) tickFallen(a *world.Agent, dt float64) {
	a.FallTimer -= float32(dt)
	if a.FallTimer <= 0 {
		a.Fallen = false
		a.Balance = float32(fallStartBalance)
		a.Speed = 0
		a.TurnSide = 0
	}
}

// planGoalFor returns the GoalKind for whatever entity TargetID resolves to.
func planGoalFor(w *world.World, a *world.Agent) ai.GoalKind {
	for _, b := range w.Buildings {
		if b.ID == a.TargetID {
			return ai.GoalLodge
		}
	}
	return ai.GoalLift
}

// =============================================================================
// SECTION 4 — Perception
// =============================================================================

func perceive(t *world.Terrain, a *world.Agent, target mgl32.Vec3) Perception {
	pos := a.Pos
	n := t.NormalAt(pos[0]/CellSize, pos[2]/CellSize)
	fall, fallScale := fallDirAndScale(n)
	slope := float32(math.Acos(math.Min(1, math.Max(-1, float64(n[1])))))

	axisXZ := mgl32.Vec2{target[0] - pos[0], target[2] - pos[2]}
	axisDist := axisXZ.Len()
	var axisDir mgl32.Vec2
	if axisDist > 1e-3 {
		axisDir = axisXZ.Mul(1.0 / axisDist)
	}

	atCell := t.TreeDensityAt(pos[0], pos[2])
	return Perception{
		Pos:           pos,
		Heading:       a.Heading,
		Speed:         a.Speed,
		Normal:        n,
		SlopeAngle:    slope,
		FallDir:       fall,
		FallScale:     fallScale,
		AxisDir:       axisDir,
		AxisDist:      axisDist,
		InArrival:     axisDist < ArrivalRadius,
		AtCellDensity: atCell,
		InTrees:       atCell > inTreesThreshold,
	}
}

// =============================================================================
// SECTION 5 — Controller (decide)
// =============================================================================

// decide is the entire steering logic. Every output is a continuous function
// of perception + small persistent state (TurnSide).
//
// Heading composition:
//
//	axis     = blend(target-direction, fall-line)  (slope-attenuated)
//	tactical = forward-sampling lateral offset      (trees/boundaries)
//	brake    = TurnSide × brakeAngle                (speed control → S-turns)
//	desired  = axis + tactical + brake
//
// The brake offset is what produces emergent S-turns: while overspeed,
// brakeAngle > 0 → desired heading is off the fall line → edge friction
// scrubs speed → speed drops → brakeAngle shrinks → if heading has reached
// the arc edge on the committed side, flip TurnSide and carve back.
func decide(t *world.Terrain, a *world.Agent, perc Perception, dt float32, rng *rand.Rand) Decision {
	axisHeading := composeAxis(perc)

	tactical, obstacleSeen, probeC, probeR, probeL := sampleTactical(t, perc, axisHeading, a.LastTactical, rng)

	// Speed control.
	targetSpeed := desiredSpeed(a.Traits, perc)
	overspeed := float32(0)
	if perc.Speed > targetSpeed && targetSpeed > 0.01 {
		overspeed = (perc.Speed - targetSpeed) / targetSpeed
	}
	brakeAngle := clamp32(overspeed*float32(brakeAngleGain), 0, float32(brakeAngleMax))

	// Persistent turn-side commit. Below the commit threshold the skier
	// runs straight; above it, they're carving on a committed side.
	//
	// While avoiding an obstacle (obstacleSeen) we suppress the brake
	// oscillation entirely. The tactical offset already takes the heading
	// off the fall line, so cross-fall friction still scrubs speed — and
	// the oscillation would otherwise fight the lateral commitment by
	// swinging heading back through axis every cycle. Real skiers don't
	// S-turn through trees; they pick a line and hold it.
	side := a.TurnSide
	deviation := wrapAngle(perc.Heading - axisHeading)
	dwellSatisfied := a.TurnDwell >= float32(turnDwellMin)
	switch {
	case obstacleSeen:
		side = 0
	case brakeAngle < float32(brakeMinForCommit):
		side = 0
	case side == 0:
		side = pickInitialSide(perc, deviation, rng)
	case side > 0 && deviation > brakeAngle*0.85 && dwellSatisfied:
		side = -1
	case side < 0 && deviation < -brakeAngle*0.85 && dwellSatisfied:
		side = +1
	}

	desired := wrapAngle(axisHeading + tactical + float32(side)*brakeAngle)

	// Active scrub: only when way overspeed. Below 60% over comfort, edge
	// friction alone handles the brake. Above, we add a wedge-style scrub.
	var scrub float32
	if overspeed > 0.6 {
		scrub = 4.0 * (overspeed - 0.6)
		if scrub > 6.0 {
			scrub = 6.0
		}
	}

	mode := "straight"
	if brakeAngle > 0 {
		mode = "carve"
	}
	if scrub > 0 {
		mode = "brake"
	}

	return Decision{
		DesiredHeading: desired,
		Scrub:          scrub,
		AxisHeading:    axisHeading,
		TacticalOffset: tactical,
		Brake:          brakeAngle,
		TurnSide:       side,
		TargetSpeed:    targetSpeed,
		Mode:           mode,
		ProbeC:         probeC,
		ProbeR:         probeR,
		ProbeL:         probeL,
	}
}

// composeAxis blends the seek-target direction with the fall-line, attenuated
// by slope. On flats (fallScale ~ 0) the axis is pure seek; on steeps the
// axis bends downhill so the skier doesn't fight gravity sideways.
func composeAxis(perc Perception) float32 {
	bx := perc.AxisDir[0] + perc.FallDir[0]*perc.FallScale
	bz := perc.AxisDir[1] + perc.FallDir[1]*perc.FallScale
	bl := float32(math.Sqrt(float64(bx*bx + bz*bz)))
	if bl < 1e-4 {
		bx, bz = perc.AxisDir[0], perc.AxisDir[1]
		bl = 1
	}
	bx /= bl
	bz /= bl
	return float32(math.Atan2(float64(bx), float64(bz)))
}

// desiredSpeed is the per-skier target speed before braking kicks in. Maps
// ComfortSpeed × Aggression × arrival modulation. No confidence multiplier
// (Plan A drops the drift state).
func desiredSpeed(traits ai.SkierTraits, perc Perception) float32 {
	s := traits.ComfortSpeed * (0.7 + 0.6*traits.Aggression)
	if perc.InArrival {
		s *= 0.5
	}
	if s < skiWalkSpeed {
		s = skiWalkSpeed
	}
	return s
}

// pickInitialSide chooses which way to start a carve when entering the brake
// regime. Coin-flipped — neither side is privileged. Future work could bias
// away from terrain boundaries or worse-scoring tactical samples.
func pickInitialSide(perc Perception, deviation float32, rng *rand.Rand) int8 {
	// If heading already favours a side, commit to that — avoids an ugly
	// 180° flip in the first tick of the carve.
	if deviation > 0.05 {
		return +1
	}
	if deviation < -0.05 {
		return -1
	}
	if rng != nil && rng.Float32() < 0.5 {
		return -1
	}
	return +1
}

// sampleTactical scores a fan of candidate forward arcs and returns the
// best lateral offset (relative to axis), a flag indicating whether an
// obstacle is in view (so the controller can suppress S-turn oscillation
// while avoiding), and centre/right/left density readings for the HUD.
//
// Score = progressBonus × cos(offset)
//       − Σ treePenalty × density(point along projected line)
//       − boundaryPenalty × (off-map sample count)
//       + sideCommitBonus × sign(prevTactical) × sign(offset)   [conditional]
//
// The progress term keeps the skier on axis when nothing obstructs. The
// side-commit term breaks the symmetry of obstacles centred on axis so
// the skier doesn't flip-flop tick-to-tick when both lateral options
// score equally — but it's gated on actually seeing an obstacle in the
// fan. Without that gate the commit bonus would slowly drift the skier
// off-axis even on a clear slope, since prevTactical is self-perpetuating.
func sampleTactical(t *world.Terrain, perc Perception, axisHeading, prevTactical float32, rng *rand.Rand) (offset float32, obstacleSeen bool, probeC, probeR, probeL float32) {
	horizon := perc.Speed * float32(sampleHorizonSec)
	if horizon < float32(sampleMinDist) {
		horizon = float32(sampleMinDist)
	}
	if horizon > float32(sampleMaxDist) {
		horizon = float32(sampleMaxDist)
	}

	// Pass 1: integrate density and boundary hits along each candidate.
	type sampleData struct {
		ang          float32
		totalDensity float32
		boundaryHits int
	}
	samples := make([]sampleData, sampleCount)
	var maxDensity float32
	for i := 0; i < sampleCount; i++ {
		f := float32(2*i)/float32(sampleCount-1) - 1 // -1 .. +1
		ang := f * float32(sampleAngleMax)
		head := axisHeading + ang
		hx := float32(math.Sin(float64(head)))
		hz := float32(math.Cos(float64(head)))
		// Perpendicular-to-path unit vector for the corridor checks below.
		rx, rz := hz, -hx

		var totalDensity float32
		var boundaryHits int
		for sIdx := 1; sIdx <= sampleSegments; sIdx++ {
			d := horizon * float32(sIdx) / float32(sampleSegments)
			x := perc.Pos[0] + hx*d
			z := perc.Pos[2] + hz*d
			if !t.InBoundsWorld(x, z) {
				boundaryHits++
				continue
			}
			// Worst-of-three: centre + ±corridorHalfWidth perpendicular.
			// Treats the candidate path as a corridor, so the skier
			// avoids brushing the patch edge instead of grazing it.
			density := t.TreeDensityAt(x, z)
			if dl := t.TreeDensityAt(x-rx*float32(corridorHalfWidth), z-rz*float32(corridorHalfWidth)); dl > density {
				density = dl
			}
			if dr := t.TreeDensityAt(x+rx*float32(corridorHalfWidth), z+rz*float32(corridorHalfWidth)); dr > density {
				density = dr
			}
			totalDensity += density
		}
		samples[i] = sampleData{ang, totalDensity, boundaryHits}
		if totalDensity > maxDensity {
			maxDensity = totalDensity
		}

		switch i {
		case sampleCount / 2:
			probeC = totalDensity / float32(sampleSegments)
		case sampleCount - 1:
			probeR = totalDensity / float32(sampleSegments)
		case 0:
			probeL = totalDensity / float32(sampleSegments)
		}
	}

	// Pass 2: score. Side-commit bonus is gated on an obstacle being
	// visible — without that gate, prevTactical keeps re-electing itself
	// even on perfectly clear terrain.
	obstacleSeen = maxDensity > 0.5
	prevSign := float32(0)
	if obstacleSeen {
		switch {
		case prevTactical > 0.05:
			prevSign = +1
		case prevTactical < -0.05:
			prevSign = -1
		}
	}

	bestScore := float32(-1e9)
	for _, sd := range samples {
		score := float32(progressBonus) * float32(math.Cos(float64(sd.ang)))
		score -= float32(treePenalty) * sd.totalDensity
		score -= float32(boundaryPenalty) * float32(sd.boundaryHits)
		if prevSign != 0 && sd.ang != 0 {
			angSign := float32(+1)
			if sd.ang < 0 {
				angSign = -1
			}
			score += float32(sideCommitBonus) * prevSign * angSign
		}
		// Tiny RNG jitter so a symmetric obstacle (centre patch, identical
		// scores either side) gets resolved by the simulation's RNG instead
		// of by iteration order — otherwise the "first encountered tie wins"
		// rule biases every run to the same side regardless of seed.
		if rng != nil {
			score += (rng.Float32() - 0.5) * 0.001
		}
		if score > bestScore {
			bestScore = score
			offset = sd.ang
		}
	}
	return
}

// =============================================================================
// SECTION 6 — Apply (physics integration)
// =============================================================================

func apply(t *world.Terrain, a *world.Agent, dec Decision, perc Perception, dt float64) {
	a.Heading = rotateToward(a.Heading, dec.DesiredHeading, float32(headingRateMax), dt)

	hx := float32(math.Sin(float64(a.Heading)))
	hz := float32(math.Cos(float64(a.Heading)))

	cosTheta := float64(perc.Normal[1])
	sinTheta := math.Sqrt(math.Max(0, 1-cosTheta*cosTheta))

	cosOff := 1.0
	sinOffAbs := 0.0
	if perc.FallScale > 0 {
		cosOff = float64(hx*perc.FallDir[0] + hz*perc.FallDir[1])
		sinOffAbs = math.Abs(float64(hx*perc.FallDir[1] - hz*perc.FallDir[0]))
	}

	speed := float64(a.Speed)
	accel := gravity*sinTheta*cosOff -
		muBase*gravity*cosTheta -
		muEdge*gravity*cosTheta*sinOffAbs -
		kDrag*speed*speed -
		float64(dec.Scrub)
	a.Speed = float32(math.Max(0, speed+accel*dt))
	if a.Speed < skiWalkSpeed {
		a.Speed = skiWalkSpeed
	}

	step := a.Speed * float32(dt)
	a.Pos[0] += hx * step
	a.Pos[2] += hz * step
	a.Pos[1] = t.InterpolatedElevationAt(a.Pos[0], a.Pos[2])
}

// =============================================================================
// SECTION 7 — Stress / balance
// =============================================================================

// stressDelta returns the rate of change of Balance per second. Negative
// drains toward a fall, positive recovers. Clamped to keep numerical
// excursions bounded.
func stressDelta(traits ai.SkierTraits, perc Perception, dec Decision) float32 {
	d := float32(0.15) // base recovery

	if traits.ComfortSpeed > 0 && perc.Speed > traits.ComfortSpeed*1.2 {
		excess := perc.Speed/traits.ComfortSpeed - 1.2
		d -= excess * 0.3
	}
	if traits.ComfortSlope > 0 && perc.SlopeAngle > traits.ComfortSlope*1.2 {
		excess := perc.SlopeAngle/traits.ComfortSlope - 1.2
		d -= excess * 0.5
	}
	// Hard scrub costs balance — wedging under load is tiring.
	if dec.Scrub > 0 {
		d -= dec.Scrub * 0.02
	}
	if perc.AtCellDensity > inTreesThreshold {
		d -= (perc.AtCellDensity - inTreesThreshold) * 0.4
	}

	if d < -1 {
		d = -1
	}
	if d > 0.4 {
		d = 0.4
	}
	return d
}

// =============================================================================
// SECTION 8 — Geometry helpers
// =============================================================================

// fallDirAndScale returns the unit downhill direction and a smoothstepped
// strength in [0, 1]. On near-flat terrain, strength is zero so steering
// ignores the noisy gradient.
func fallDirAndScale(normal mgl32.Vec3) (mgl32.Vec2, float32) {
	v := mgl32.Vec2{normal[0], normal[2]}
	l := v.Len()
	if l < 1e-4 {
		return mgl32.Vec2{}, 0
	}
	dir := v.Mul(1.0 / l)
	if l <= flatSlopeL {
		return dir, 0
	}
	if l >= steepSlopeL {
		return dir, 1
	}
	tt := (l - flatSlopeL) / (steepSlopeL - flatSlopeL)
	return dir, tt * tt * (3 - 2*tt)
}

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

func wrapAngle(a float32) float32 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// =============================================================================
// SECTION 9 — Snapshots (HUD / recorder)
// =============================================================================

// senseFrom builds the per-tick HUD/renderer snapshot. Display-only.
func senseFrom(perc Perception, dec Decision) ai.Sense {
	horizon := perc.Speed * float32(sampleHorizonSec)
	if horizon < float32(sampleMinDist) {
		horizon = float32(sampleMinDist)
	}
	if horizon > float32(sampleMaxDist) {
		horizon = float32(sampleMaxDist)
	}
	return ai.Sense{
		ProbeDist:      horizon,
		ProbeHalfAngle: float32(sampleAngleMax),
		ProbeC:         dec.ProbeC,
		ProbeR:         dec.ProbeR,
		ProbeL:         dec.ProbeL,
		AxisHeading:    dec.AxisHeading,
		DesiredHeading: dec.DesiredHeading,
		TargetSpeed:    dec.TargetSpeed,
		Brake:          dec.Brake,
		TurnSide:       dec.TurnSide,
		Mode:           dec.Mode,
		InTrees:        perc.InTrees,
		AtCellDensity:  perc.AtCellDensity,
	}
}

// =============================================================================
// SECTION 10 — F3 debug overlay
// =============================================================================

// SteeringDebug is the debug bundle consumed by scene/scenario.go's F3
// overlay. Re-derived from a non-mutating perception+decide pass against
// `target` so the overlay shows what the controller would output RIGHT NOW.
type SteeringDebug struct {
	Pos         mgl32.Vec3
	FallLine    mgl32.Vec2
	DesiredHead float32
	ProbeDist   float32
	Probes      [3]struct {
		Dir     mgl32.Vec2
		Density float32
	}
}

// ComputeSteeringDebug runs a non-mutating perception + decide pass for
// rendering. The TurnSide on the agent is read but not mutated.
func ComputeSteeringDebug(t *world.Terrain, a *world.Agent, target mgl32.Vec3) SteeringDebug {
	clone := *a
	perc := perceive(t, &clone, target)
	// nil rng → no jitter, debug shows the deterministic part of the score.
	dec := decide(t, &clone, perc, 0, nil)

	horizon := perc.Speed * float32(sampleHorizonSec)
	if horizon < float32(sampleMinDist) {
		horizon = float32(sampleMinDist)
	}
	if horizon > float32(sampleMaxDist) {
		horizon = float32(sampleMaxDist)
	}

	out := SteeringDebug{
		Pos:         a.Pos,
		FallLine:    perc.FallDir,
		DesiredHead: dec.DesiredHeading,
		ProbeDist:   horizon,
	}

	hx := float32(math.Sin(float64(a.Heading)))
	hz := float32(math.Cos(float64(a.Heading)))
	rx, rz := hz, -hx
	angles := [3]float64{0, float64(sampleAngleMax), -float64(sampleAngleMax)}
	for i, ang := range angles {
		c := float32(math.Cos(ang))
		s := float32(math.Sin(ang))
		d := mgl32.Vec2{c*hx + s*rx, c*hz + s*rz}
		out.Probes[i].Dir = d
		out.Probes[i].Density = t.TreeDensityAt(a.Pos[0]+d[0]*horizon, a.Pos[2]+d[1]*horizon)
	}
	return out
}

// =============================================================================
// SECTION 11 — Recorder hook
// =============================================================================

func recordFrame(s *Simulation, a *world.Agent, target mgl32.Vec3, dist float32, perc Perception, dec Decision) {
	if s.Recorder == nil {
		return
	}
	if id := s.Recorder.AgentID(); id != 0 && id != a.ID {
		return
	}
	s.Recorder.Record(RecorderFrame{
		SimTime:         s.SimTime,
		AgentID:         a.ID,
		Activity:        world.Activity(s.World, a),
		Pos:             a.Pos,
		Heading:         a.Heading,
		Target:          target,
		Dist:            dist,
		Speed:           a.Speed,
		FallLine:        perc.FallDir,
		AxisHeading:     dec.AxisHeading,
		DesiredHeading:  dec.DesiredHeading,
		TargetSpeed:     dec.TargetSpeed,
		Brake:           dec.Brake,
		TurnSide:        dec.TurnSide,
		Mode:            dec.Mode,
		Balance:         a.Balance,
		ProbeC:          dec.ProbeC,
		ProbeR:          dec.ProbeR,
		ProbeL:          dec.ProbeL,
		SlopeCos:        perc.Normal[1],
		InArrivalRadius: perc.InArrival,
	})
}
