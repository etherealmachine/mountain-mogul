package sim

import (
	"math"
	"math/rand"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// =============================================================================
// SKIER AI
//
// Five-layer pipeline driven once per agent per tick:
//
//   Route      — slow-changing strategic plan (where am I going)
//   Perception — eyesight cone over the world (what do I see right now)
//   Steering   — Perception + Route → Intent (what do I want to do)
//   Motor      — Intent + Traits + MotorState → MotorCmd (which technique now)
//   Physics    — MotorCmd + dt → integrate heading, speed, position
//
// Each layer is a near-pure function with a small typed input and output, so
// individual layers are testable in isolation. Persistent per-agent AI state
// (Traits, Route, MotorState, Balance) lives in internal/ai; transient
// per-tick types (Perception, Intent, MotorCmd, Hazard) are sim-internal.
// =============================================================================

// =============================================================================
// SECTION 1 — Tunables
// =============================================================================

const (
	// Slope physics
	gravity = 9.81
	muBase  = 0.04 // base kinetic friction (groomed snow)
	muEdge  = 0.20 // carving friction (cross-fall component); was 0.40 in old model
	kDrag   = 0.01 // air drag per unit mass

	// Steering / motion limits
	maxAngularSpeed = 120 * math.Pi / 180 // rad/s; cap on heading rotation per tick
	skiWalkSpeed    = 2.0                 // m/s; minimum forward motion (skating/poling floor)

	// Arrival
	ArrivalRadius    = 6.0 // m; switch to direct-pointing inside this
	ArrivalThreshold = 2.0 // m; sim considers the agent "there"

	// Fall-line attenuation (smoothstep on slope strength)
	flatSlopeL  = 0.05 // sin(~2.9°)
	steepSlopeL = 0.20 // sin(~11.5°)

	// Parallel-turn arc width range. The motor anchors arc width on slope
	// (steeper terrain → wider arcs to scrub), not on speed-vs-comfort, so
	// even on a gentle slope where the skier never reaches comfort speed the
	// turns still have visible amplitude.
	parallelMinArc = 35 * math.Pi / 180
	parallelMaxArc = 70 * math.Pi / 180

	// Minimum sim-time the skier dwells on one side of the fall line before
	// allowing a phase flip. Stops sub-second pinging across the axis and
	// gives each carve room to develop.
	parallelMinDwell = 0.7 // seconds

	// Heading-rotation cap while the parallel motor is engaged. Lower than
	// the global maxAngularSpeed so carved arcs read as turns rather than
	// snaps. Other techniques (hockey, sideslip) keep the global cap.
	parallelTurnRate = 55 * math.Pi / 180 // rad/s

	// Tree perception. Distances scale with the skier's current speed
	// (time-to-impact, not skill) so a slow skier at the top of a run plans
	// calmly while a fast skier looks farther ahead but reacts later. All
	// tiers share these constants — glade tolerance will be a separate trait.
	treeLookahead   = 3.0                  // s; trajectory projected ahead for probes
	treeProbeMin    = 12.0                 // m; floor — even idle skiers see nearby trees
	treeProbeMax    = 40.0                 // m; cap — beyond this individual trees stop mattering
	treeUrgencyTime = 1.0                  // s; "I'd hit this within a second" → scrub
	treeStressTime  = 0.6                  // s; "I'd hit this very soon" → drain balance
	probeAngle      = 35 * math.Pi / 180   // rad; ±offset for side probes

	// In-trees aversion. Above this underfoot density the skier gives up
	// goal-seeking and steers for the nearest gap. A future GladeTolerance
	// trait will shift this per-skier; for now everyone is averse.
	inTreesThreshold = 0.3
	clearSampleDist  = 15.0 // m; radius of the 8-point gradient sample
	inTreesSpeedCap  = 2.4  // m/s; hard target-speed cap while in trees

	// Walking escape — when in-trees aversion fails to make progress, give
	// up on skiing and walk out to the nearest clear cell. We use a pure
	// "time-in-trees" gate (not a speed gate) because the skiing pipeline
	// has a hard skiWalkSpeed = 2.0 m/s floor — an agent can be making
	// negligible-but-nonzero progress and never look "stuck" by speed
	// alone. The trigger is set well above a typical 0.8-density patch
	// transit (~10 s) so Phase 2 fires only when Phase 1 has really
	// failed (very dense patch, pinned in a corridor, etc.).
	stuckTriggerS     = 12.0 // sim-seconds of in-trees before escape kicks in
	clearCellDensity  = 0.3  // destination must have density below this
	clearSearchRadius = 20   // cells; max spiral-search radius (~200 m)

	// Strategic vision. Long-range read of the run ahead, run once per
	// Route refresh, that biases the skier's line toward the cleaner side
	// of distant tree masses. Far longer than the tactical perception cone
	// (12-40 m) — a real skier can see hundreds of metres of slope below
	// them and picks a line from the top.
	strategicRange           = 300.0 // m; "almost-infinite vision" horizon
	strategicLateralOffset   = 50.0  // m; near typical patch radius — straddles edges
	strategicNSamples        = 12    // forward samples (every ~25 m at full range)
	strategicMinDist         = 30.0  // m; below this, no strategic plan (close to goal)
	strategicCentreThreshold = 0.15  // average centre density to engage at all
	strategicSymmetryFloor   = 0.05  // |L−R| below this → symmetric → use default
	strategicSymmetryDefault = 1.0   // default right bias on a symmetric obstacle (full commit)
	strategicBiasScale       = 0.3   // diff → bias scaling (smaller ⇒ more sensitive)
	strategicBendMax         = 0.30  // rad ≈ 17°; max axis bend at full bias

	// Hockey stop pulse
	hockeyDurationS = 0.6
	hockeyScrub     = 8.0 // m/s² extra deceleration during the pulse
	hockeyBalCost   = 0.4 // balance drain per second while engaged

	// Route freshness
	routePlanInterval = 2.0 // sim-seconds between route refreshes
)

// =============================================================================
// SECTION 2 — Per-tick types
// =============================================================================

// Hazard is a perceived thing the steering layer wants to avoid. Direction is
// a unit XZ vector from the agent toward the hazard.
type Hazard struct {
	Dir      mgl32.Vec2
	Distance float32
	Severity float32 // 0..1
}

// Perception captures everything the agent can sense this tick. Computed
// fresh each frame; never stored on the agent.
type Perception struct {
	Pos     mgl32.Vec3
	Heading float32
	Speed   float32

	Normal     mgl32.Vec3
	SlopeAngle float32 // radians at the agent's position
	SlopeAhead float32 // averaged slope angle 10 m forward
	FallDir    mgl32.Vec2
	FallScale  float32 // smoothstepped slope strength in [0, 1]

	AxisDir  mgl32.Vec2 // unit vector toward route goal in XZ
	AxisDist float32    // metres to the goal in XZ

	Trees      []Hazard
	TreeCenter float32 // density at the centre forward probe

	// "I'm currently inside trees" — read from the agent's actual cell, not
	// the cone ahead. Drives a different steering branch focused on getting
	// out, not on dodging incoming hazards.
	AtCellDensity float32    // TreeDensityAt(Pos.X, Pos.Z) — density underfoot
	InTrees       bool       // AtCellDensity > inTreesThreshold
	ClearDir      mgl32.Vec2 // unit XZ vector toward least-dense neighbourhood; zero when InTrees=false

	// Long-range strategic side bias copied from Route. Drives a small
	// constant axis bend in steer(); refreshed at Route cadence (2 s).
	StrategicBias float32

	InArrival bool // AxisDist < ArrivalRadius
}

// Intent is what the steering layer wants the skier to do this tick. The
// motor layer turns it into a specific technique + heading.
type Intent struct {
	AxisHeading float32 // desired travel direction (axis), radians
	Speed       float32 // desired speed, m/s
	Urgency     float32 // 0..1; "I need to scrub speed NOW"
}

// MotorCmd is the motor layer's output, consumed by physics.
type MotorCmd struct {
	Heading     float32 // commanded heading; physics rotates toward this
	Scrub       float32 // m/s² extra deceleration beyond passive friction
	BalanceCost float32 // balance drain per second
	// MaxTurnRate caps heading rotation this tick (rad/s). 0 = use the global
	// maxAngularSpeed default. Techniques use this to express that a carved
	// arc shouldn't rotate as fast as an emergency hockey stop.
	MaxTurnRate float32
}

// =============================================================================
// SECTION 3 — Tick orchestration
// =============================================================================

// tickSkier runs one frame of the AI pipeline against `target`. Returns true
// when the agent has arrived (within ArrivalThreshold) — the caller decides
// what arrival means (queue a lift, enter a lodge, etc.).
func (s *Simulation) tickSkier(a *world.Agent, target mgl32.Vec3, dt float64) bool {
	// Hard arrival check up front so we don't run a full pipeline for a
	// last-tick that just snaps to target.
	delta := target.Sub(a.Pos)
	dist := delta.Len()
	if dist < ArrivalThreshold {
		a.Pos = target
		return true
	}

	// Stuck-in-trees detection. Accumulates while the agent is classified
	// in-trees; if Phase 1's slow-and-steer-out hasn't gotten them clear
	// after stuckTriggerS sim-seconds, we fall back to walking. We read
	// a.Sense.InTrees from the *previous* tick because Perception isn't
	// computed yet — that's fine, the timer only needs to be approximately
	// correct over many ticks.
	if a.Sense.InTrees {
		a.StuckTimer += float32(dt)
	} else {
		a.StuckTimer = 0
	}
	if a.StuckTimer >= stuckTriggerS {
		if cell, ok := findClearCell(s.World.Terrain, a.Pos); ok {
			a.Path = [][2]int{cell}
			a.PathIdx = 0
			a.StuckTimer = 0
			return false
		}
		// No clear cell anywhere — keep skiing; nothing better to do.
		a.StuckTimer = 0
	}

	// L1: refresh route if stale or pointing somewhere else.
	if a.Route.Goal == ai.GoalNone || a.Route.GoalPos != target || a.Route.StaleAt < s.SimTime {
		// Compute axis to goal for the strategic scan. (steer() will
		// re-derive this from Perception.AxisDir each tick; we need it
		// here, before perceive runs, to feed strategicScan.)
		axisDist := mgl32.Vec2{delta[0], delta[2]}.Len()
		var axisDir mgl32.Vec2
		if axisDist > 1e-3 {
			axisDir = mgl32.Vec2{delta[0] / axisDist, delta[2] / axisDist}
		}
		a.Route = ai.Route{
			Goal:          routeGoalFor(s.World, a),
			GoalPos:       target,
			StaleAt:       s.SimTime + routePlanInterval,
			StrategicBias: strategicScan(s.World.Terrain, a.Pos, axisDir, axisDist, s.Rng),
		}
	}

	// L2: perceive
	perc := perceive(s.World.Terrain, a)

	// L3: steer
	intent, avoid := steer(a.Traits, a.Avoid, perc, float32(dt))
	a.Avoid = avoid

	// L4: motor / technique
	cmd, motor := selectTechnique(a.Traits, a.Motor, intent, perc, float32(dt))
	a.Motor = motor

	// Snapshot perception/intent for the follow HUD and perception-cone
	// shader. Stale outside skiing — readers gate on Activity.
	a.Sense = senseFrom(perc, intent, cmd)

	// Balance drain & fall trigger.
	a.Balance += stressDelta(a.Traits, perc, intent, cmd) * float32(dt)
	if a.Balance > 1 {
		a.Balance = 1
	}
	if a.Balance <= 0 {
		a.Balance = 0
		a.Fallen = true
		a.FallTimer = 4
		a.Speed = 0
		recordFrame(s, a, target, dist, perc, intent, cmd)
		return false
	}

	// L5: physics
	applyMotor(s.World.Terrain, a, cmd, perc, dt)

	recordFrame(s, a, target, dist, perc, intent, cmd)
	return false
}

// tickFallen counts the agent down out of the fallen window and resumes.
// The agent's TargetID and other goal fields are untouched, so the next
// dispatch resumes locomotion (ski or walk) toward the same target.
func (s *Simulation) tickFallen(a *world.Agent, dt float64) {
	a.FallTimer -= float32(dt)
	if a.FallTimer <= 0 {
		a.Fallen = false
		a.Balance = 0.7
		a.Speed = 0
		a.Motor = ai.MotorState{}
	}
}

// routeGoalFor returns the route's goal kind based on what kind of entity
// the agent's TargetID resolves to. Used to label the route for the AI's
// internal bookkeeping (the kind doesn't currently affect steering).
func routeGoalFor(w *world.World, a *world.Agent) ai.GoalKind {
	for _, b := range w.Buildings {
		if b.ID == a.TargetID {
			return ai.GoalLodge
		}
	}
	return ai.GoalLift
}

// =============================================================================
// SECTION 4 — Layer 2: Perception
// =============================================================================

func perceive(t *world.Terrain, a *world.Agent) Perception {
	pos := a.Pos
	n := t.NormalAt(pos[0]/CellSize, pos[2]/CellSize)
	fall, fallScale := fallDirAndScale(n)

	slope := float32(math.Acos(math.Min(1, math.Max(-1, float64(n[1])))))

	hx := float32(math.Sin(float64(a.Heading)))
	hz := float32(math.Cos(float64(a.Heading)))

	nAhead := t.NormalAt((pos[0]+hx*10)/CellSize, (pos[2]+hz*10)/CellSize)
	slopeAhead := float32(math.Acos(math.Min(1, math.Max(-1, float64(nAhead[1])))))

	goalDelta := a.Route.GoalPos.Sub(pos)
	axisXZ := mgl32.Vec2{goalDelta[0], goalDelta[2]}
	axisDist := axisXZ.Len()
	var axisDir mgl32.Vec2
	if axisDist > 1e-3 {
		axisDir = axisXZ.Mul(1.0 / axisDist)
	}

	trees, treeCenter := scanTrees(t, pos, a.Heading, a.Speed)

	atCell := t.TreeDensityAt(pos[0], pos[2])
	inTrees := atCell > inTreesThreshold
	var clearDir mgl32.Vec2
	if inTrees {
		clearDir = sampleClearDir(t, pos)
	}

	return Perception{
		Pos:           pos,
		Heading:       a.Heading,
		Speed:         a.Speed,
		Normal:        n,
		SlopeAngle:    slope,
		SlopeAhead:    slopeAhead,
		FallDir:       fall,
		FallScale:     fallScale,
		AxisDir:       axisDir,
		AxisDist:      axisDist,
		Trees:         trees,
		TreeCenter:    treeCenter,
		AtCellDensity: atCell,
		InTrees:       inTrees,
		ClearDir:      clearDir,
		StrategicBias: a.Route.StrategicBias,
		InArrival:     axisDist < ArrivalRadius,
	}
}

// findClearCell spiral-searches outward from the agent's current cell for
// the nearest grid cell with TreeDensity below clearCellDensity. Returns the
// cell coordinate and true on success; false if nothing within
// clearSearchRadius qualifies. Used by the walking-escape branch in
// tickSkier when the skier has been stuck in trees too long.
//
// Search pattern: rings of increasing Chebyshev distance from the centre.
// Within each ring, prefer cells closer to the fall-line (lower Y) — but
// since the testbed-scale slope is uniform downhill, this falls out
// naturally from the iteration order (we visit -dz before +dz at small dz,
// which on a "south is downhill" slope tends downhill first; close enough
// for an escape heuristic).
func findClearCell(t *world.Terrain, pos mgl32.Vec3) ([2]int, bool) {
	cx := int(pos[0] / CellSize)
	cz := int(pos[2] / CellSize)
	for r := 1; r <= clearSearchRadius; r++ {
		// Iterate the ring of Chebyshev-distance r around (cx, cz).
		for dx := -r; dx <= r; dx++ {
			for dz := -r; dz <= r; dz++ {
				if dx != -r && dx != r && dz != -r && dz != r {
					continue // interior — already visited at smaller r
				}
				x, z := cx+dx, cz+dz
				if !t.InBounds(x, z) {
					continue
				}
				if t.Cells[x][z].TreeDensity < clearCellDensity {
					return [2]int{x, z}, true
				}
			}
		}
	}
	return [2]int{}, false
}

// strategicScan reads the run far ahead and returns a side-bias scalar in
// [-1, +1]: negative leans left, positive leans right. Sampled along the
// straight axis to the goal at strategicNSamples forward points, with
// ±strategicLateralOffset perpendicular probes at each. The forward
// (centre) average decides whether the path is dense enough to engage at
// all; the L-R differential decides which side to lean.
//
// On a perfectly-symmetric obstacle the L-R differential collapses to
// ~0 (both lateral probes hit the disk equally). When that happens we
// flip a coin from rng — random per-scan, drawing from the deterministic
// per-sim Rng so testbed seeds stay reproducible. Once the skier has
// drifted any meaningful distance toward a chosen side, the diff signal
// dominates and locks the direction in (the patch is now off-axis on
// the opposite side, so left-vs-right density is no longer symmetric).
// A future PreferredSide personality trait can replace the coin flip
// with a per-skier preference.
//
// Cost: 3 × strategicNSamples = 36 TreeDensityAt calls per call + at
// most one rng.Float32(). Called once per Route refresh (every 2 s)
// per agent.
func strategicScan(t *world.Terrain, pos mgl32.Vec3, axisDir mgl32.Vec2, axisDist float32, rng *rand.Rand) float32 {
	scanRange := float32(strategicRange)
	if axisDist < scanRange {
		scanRange = axisDist
	}
	if scanRange < strategicMinDist {
		return 0
	}

	// Right-perpendicular to axis: when axis is (sin h, cos h), perpRight
	// is (cos h, -sin h) — i.e. (axis.z, -axis.x).
	rx := axisDir[1]
	rz := -axisDir[0]

	var centre, left, right, totalW float32
	for i := 1; i <= strategicNSamples; i++ {
		f := float32(i) / float32(strategicNSamples)
		d := scanRange * f
		cx := pos[0] + axisDir[0]*d
		cz := pos[2] + axisDir[1]*d
		w := 1 - 0.5*f // closer matters more; far end at 0.5×
		totalW += w
		centre += t.TreeDensityAt(cx, cz) * w
		right += t.TreeDensityAt(cx+rx*strategicLateralOffset, cz+rz*strategicLateralOffset) * w
		left += t.TreeDensityAt(cx-rx*strategicLateralOffset, cz-rz*strategicLateralOffset) * w
	}
	if totalW <= 0 {
		return 0
	}
	centre /= totalW
	left /= totalW
	right /= totalW

	if centre < strategicCentreThreshold {
		return 0
	}

	diff := left - right // positive ⇒ right is denser ⇒ lean left? No: positive ⇒ left side has *more* trees ⇒ lean *right*.
	// Wait — we sum density (more trees = bigger number). diff = left -
	// right > 0 means LEFT is denser → lean RIGHT (away from trees) →
	// bias > 0. Correct.
	abs := diff
	if abs < 0 {
		abs = -abs
	}
	if abs < strategicSymmetryFloor {
		if rng.Float32() < 0.5 {
			return -strategicSymmetryDefault
		}
		return strategicSymmetryDefault
	}
	bias := diff / strategicBiasScale
	if bias > 1 {
		bias = 1
	}
	if bias < -1 {
		bias = -1
	}
	return bias
}

// sampleClearDir returns a unit XZ vector pulling toward the least-dense
// neighbourhood around pos. Eight cardinal+diagonal samples at clearSampleDist
// metres; each contributes (1 - density) weight in its direction. The sum is
// the gradient of "openness" — the direction that gets you out fastest.
//
// Returns the zero vector when the skier is in a uniform high-density area
// and there's no usable signal; the caller falls back to fall-line.
func sampleClearDir(t *world.Terrain, pos mgl32.Vec3) mgl32.Vec2 {
	// Eight directions on the unit circle.
	dirs := [8][2]float32{
		{1, 0}, {0.7071, 0.7071}, {0, 1}, {-0.7071, 0.7071},
		{-1, 0}, {-0.7071, -0.7071}, {0, -1}, {0.7071, -0.7071},
	}
	var sum mgl32.Vec2
	for _, d := range dirs {
		sx := pos[0] + d[0]*clearSampleDist
		sz := pos[2] + d[1]*clearSampleDist
		w := 1 - t.TreeDensityAt(sx, sz) // clearer = stronger pull
		if w < 0 {
			w = 0
		}
		sum[0] += d[0] * w
		sum[1] += d[1] * w
	}
	l := float32(math.Sqrt(float64(sum[0]*sum[0] + sum[1]*sum[1])))
	if l < 1e-3 {
		return mgl32.Vec2{}
	}
	return mgl32.Vec2{sum[0] / l, sum[1] / l}
}

// scanTrees samples tree density at five forward probes and returns hazards
// for any probe whose density exceeds a noise threshold. Centre density is
// returned separately so the steering layer can quickly modulate target speed.
// Probe distance is speed-scaled (lookahead time) and clamped — every skier
// uses the same look-ahead behaviour regardless of skill.
func scanTrees(t *world.Terrain, pos mgl32.Vec3, heading, speed float32) ([]Hazard, float32) {
	hx := float32(math.Sin(float64(heading)))
	hz := float32(math.Cos(float64(heading)))
	rx := hz
	rz := -hx

	probeDist := computeProbeDist(speed)

	angles := []float64{0, probeAngle * 0.5, -probeAngle * 0.5, probeAngle, -probeAngle}
	hazards := make([]Hazard, 0, len(angles))
	var centre float32

	for i, ang := range angles {
		c := float32(math.Cos(ang))
		s := float32(math.Sin(ang))
		dx := c*hx + s*rx
		dz := c*hz + s*rz
		density := t.TreeDensityAt(pos[0]+dx*probeDist, pos[2]+dz*probeDist)
		if i == 0 {
			centre = density
		}
		if density > 0.15 {
			hazards = append(hazards, Hazard{
				Dir:      mgl32.Vec2{dx, dz},
				Distance: probeDist,
				Severity: density,
			})
		}
	}
	return hazards, centre
}

// =============================================================================
// SECTION 5 — Layer 3: Steering
// =============================================================================

func steer(traits ai.SkierTraits, avoid ai.AvoidState, perc Perception, dt float32) (Intent, ai.AvoidState) {
	// In-trees override: when the skier is *inside* a dense cell, goal-seek
	// is the wrong objective. Steer for the gap instead. Skip the tactical
	// tree-bend below — that's for incoming hazards we still might dodge,
	// not for the situation we're already in.
	if perc.InTrees {
		return steerInTrees(traits, avoid, perc, dt)
	}

	// Axis: blend seek-toward-goal with fall-line bias, attenuated by slope.
	// On flats fallScale ≈ 0 → axis = seek; on steeps the axis bends downhill.
	bx := perc.AxisDir[0] + perc.FallDir[0]*perc.FallScale
	bz := perc.AxisDir[1] + perc.FallDir[1]*perc.FallScale
	bl := float32(math.Sqrt(float64(bx*bx + bz*bz)))
	if bl < 1e-4 {
		bx, bz = perc.AxisDir[0], perc.AxisDir[1]
		bl = 1
	}
	bx /= bl
	bz /= bl
	axisHeading := float32(math.Atan2(float64(bx), float64(bz)))

	// Strategic bias: small constant rotation toward the cleaner side of
	// the long-range run-ahead, as judged at the last Route refresh.
	// Always-on (no commit gate); deliberately small so it's a hint, not
	// a force. Self-decaying — as the skier passes the obstacle, the next
	// strategicScan sees the centre line clear and bias drops to 0.
	if perc.StrategicBias != 0 {
		axisHeading = wrapAngle(axisHeading + strategicBendMax*perc.StrategicBias)
	}

	// Tree avoidance: pick a side once, hold it until the patch clears.
	//
	// Why a commit: the perception field is symmetric when a skier is mid-
	// patch (every probe equally dense), so a stateless rule has no signal
	// to lean on and the per-tick bend alternates with the carve cycle —
	// netting zero. Real skiers pick a side at the patch boundary, commit,
	// and ride it out. We approximate that with a tiny state machine.
	maxSev := float32(0)
	for _, h := range perc.Trees {
		if h.Severity > maxSev {
			maxSev = h.Severity
		}
	}
	if maxSev > 0.3 {
		avoid.Clear = 0
		if avoid.Side == 0 {
			avoid.Side = pickAvoidSide(perc.Trees, bx, bz)
		}
		// Bend factor 1.0 means a fully-dense hazard rotates the axis ~57°
		// off the fall line — strong enough to skirt a wide circular patch
		// once committed early. Lower factors (0.35-0.6) failed to pull the
		// skier clear of a 60 m-radius grove in headless tests.
		bend := 1.0 * maxSev
		axisHeading = wrapAngle(axisHeading + float32(avoid.Side)*bend)
	} else {
		avoid.Clear += dt
		if avoid.Clear > 1.0 {
			avoid.Side = 0
		}
	}

	// Desired speed: comfort × aggression, scaled down on steeper terrain or
	// when trees are dense ahead.
	speed := traits.ComfortSpeed * (0.7 + 0.6*traits.Aggression)
	if perc.SlopeAhead > traits.ComfortSlope {
		excess := perc.SlopeAhead/traits.ComfortSlope - 1
		speed *= 1 - clamp32(excess*0.6, 0, 0.6)
	}
	if perc.TreeCenter > 0.3 {
		speed *= 0.6
	}
	if perc.InArrival {
		speed *= 0.5
	}
	if speed < skiWalkSpeed {
		speed = skiWalkSpeed
	}

	// Urgency: how badly do I need to slow down right now?
	urgency := float32(0)
	if traits.ComfortSpeed > 0 && perc.Speed > traits.ComfortSpeed*1.4 {
		urgency = (perc.Speed/traits.ComfortSpeed - 1.4) * 1.5
	}
	if traits.ComfortSlope > 0 && perc.SlopeAhead > traits.ComfortSlope*1.4 {
		u2 := (perc.SlopeAhead/traits.ComfortSlope - 1.4) * 1.0
		if u2 > urgency {
			urgency = u2
		}
	}
	if h := worstHazard(perc.Trees); h.Severity > 0.6 && perc.Speed > 0.1 &&
		h.Distance/perc.Speed < treeUrgencyTime {
		u2 := h.Severity
		if u2 > urgency {
			urgency = u2
		}
	}
	if urgency < 0 {
		urgency = 0
	}
	if urgency > 1 {
		urgency = 1
	}

	return Intent{
		AxisHeading: axisHeading,
		Speed:       speed,
		Urgency:     urgency,
	}, avoid
}

// steerInTrees handles the "I'm inside a dense patch, get me out" branch.
// Goal-seek is suspended while in trees — the skier just wants to be on
// clearer terrain. ClearDir already points toward the local minimum of
// density; we blend it with the fall line so the skier prefers to ski
// *down*-and-out rather than back uphill, then hard-clamp speed and pin
// urgency high so the motor layer engages a scrub-heavy technique.
//
// Tree-avoidance commitment (`avoid`) is left untouched here; once the
// agent emerges to clear ground the regular steer() will resume managing
// it. We don't want a long stay in trees to flip-flop the commit.
func steerInTrees(traits ai.SkierTraits, avoid ai.AvoidState, perc Perception, dt float32) (Intent, ai.AvoidState) {
	// Axis: ClearDir (gradient out) + fall-line bias. ClearDir may be zero
	// in a uniformly dense neighbourhood — fall back to fall-line in that
	// case so the skier still drifts downhill rather than freezing.
	bx := perc.ClearDir[0] + perc.FallDir[0]*perc.FallScale
	bz := perc.ClearDir[1] + perc.FallDir[1]*perc.FallScale
	bl := float32(math.Sqrt(float64(bx*bx + bz*bz)))
	if bl < 1e-4 {
		bx, bz = perc.FallDir[0], perc.FallDir[1]
		bl = float32(math.Sqrt(float64(bx*bx + bz*bz)))
		if bl < 1e-4 {
			// flat + uniform trees: can't pick a direction. Hold heading.
			bx = float32(math.Sin(float64(perc.Heading)))
			bz = float32(math.Cos(float64(perc.Heading)))
			bl = 1
		}
	}
	bx /= bl
	bz /= bl
	axisHeading := float32(math.Atan2(float64(bx), float64(bz)))

	// Hard cap on target speed regardless of comfort/aggression. The motor
	// layer sees this is well below the current actual speed and engages
	// pizza/sideslip/hockey via the existing overshoot dispatch.
	speed := float32(inTreesSpeedCap)

	// Pin urgency high so the technique dispatcher prefers scrub-heavy
	// moves. 0.85 keeps it under the 0.8-strict Hockey threshold but well
	// over the parallel-vs-pizza fork at 1.05× target speed.
	urgency := float32(0.85)

	_ = traits
	_ = dt
	return Intent{
		AxisHeading: axisHeading,
		Speed:       speed,
		Urgency:     urgency,
	}, avoid
}

func worstHazard(hs []Hazard) Hazard {
	var w Hazard
	for _, h := range hs {
		if h.Severity > w.Severity {
			w = h
		}
	}
	return w
}

// pickAvoidSide chooses which way to bend when the skier first detects trees.
// Strategy: sum -h.Dir × h.Severity to get an "away" vector, then project onto
// the axis-perpendicular (right of axis) to extract the lateral sign. If the
// patch is dead-on symmetric, projection ≈ 0; in that case we default to +1
// (right) so the agent always commits to *some* side rather than alternating.
// The committed side persists in AvoidState until the probes go clear.
func pickAvoidSide(hs []Hazard, axisX, axisZ float32) int8 {
	var ax, az float32
	for _, h := range hs {
		ax -= h.Dir[0] * h.Severity
		az -= h.Dir[1] * h.Severity
	}
	// Right-of-axis perpendicular (when axis is +z, perpRight = +x).
	perpX, perpZ := axisZ, -axisX
	lateral := ax*perpX + az*perpZ
	if lateral > 1e-3 {
		return +1
	}
	if lateral < -1e-3 {
		return -1
	}
	return +1 // dead-on symmetric: pick a side and commit
}

// =============================================================================
// SECTION 6 — Layer 4: Motor / Technique dispatch
// =============================================================================

// selectTechnique picks the best available technique for the current intent
// and skill, then computes a MotorCmd from that technique's profile. Returns
// the new MotorState so the caller can persist phase across ticks.
func selectTechnique(traits ai.SkierTraits, prev ai.MotorState, intent Intent, perc Perception, dt float32) (MotorCmd, ai.MotorState) {
	tech := pickTechnique(traits, prev, intent, perc)

	next := prev
	if next.Active != tech {
		next.Active = tech
		next.PhaseTime = 0
		// Re-pick a fresh phase when entering Parallel from another tech.
		if tech == ai.TechParallel && next.TurnPhase == 0 {
			next.TurnPhase = 1
		}
		if tech != ai.TechParallel {
			next.TurnPhase = 0
		}
	}
	next.PhaseTime += dt

	switch tech {
	case ai.TechStraight:
		return MotorCmd{Heading: intent.AxisHeading, Scrub: 0, BalanceCost: 0}, next

	case ai.TechPizza:
		// Wedge: heading on axis, constant scrub. Higher balance cost than
		// parallel because plowing under load is tiring.
		return MotorCmd{Heading: intent.AxisHeading, Scrub: 4.5, BalanceCost: 0.05}, next

	case ai.TechWedgeTurn:
		return MotorCmd{Heading: intent.AxisHeading, Scrub: 3.0, BalanceCost: 0.06}, next

	case ai.TechParallel:
		heading, motor := parallelHeading(traits, next, intent, perc)
		return MotorCmd{Heading: heading, Scrub: 0, BalanceCost: 0.02, MaxTurnRate: float32(parallelTurnRate)}, motor

	case ai.TechHockey:
		// Hard 90° edge-set for a brief pulse. The hockey state ends when
		// PhaseTime exceeds hockeyDurationS — selectTechnique on next tick
		// will repick a different technique.
		side := float32(1)
		if perc.Speed > 0 && perc.FallScale > 0 {
			// Bias the stop side away from the fall line on the side already
			// being travelled toward.
			cross := math.Sin(float64(perc.Heading))*float64(perc.FallDir[1]) -
				math.Cos(float64(perc.Heading))*float64(perc.FallDir[0])
			if cross < 0 {
				side = -1
			}
		}
		heading := wrapAngle(intent.AxisHeading + side*float32(math.Pi/2))
		return MotorCmd{Heading: heading, Scrub: hockeyScrub, BalanceCost: hockeyBalCost}, next

	case ai.TechSideslip:
		heading := wrapAngle(intent.AxisHeading + float32(math.Pi/2))
		return MotorCmd{Heading: heading, Scrub: 5.0, BalanceCost: 0.08}, next
	}
	// Fallback (should be unreachable given the switch above).
	return MotorCmd{Heading: intent.AxisHeading}, next
}

// pickTechnique encodes the situational dispatch policy.
func pickTechnique(traits ai.SkierTraits, prev ai.MotorState, intent Intent, perc Perception) ai.Technique {
	t := traits.Techniques

	// Hockey stop is brief and re-evaluated when its pulse ends.
	if prev.Active == ai.TechHockey && prev.PhaseTime < hockeyDurationS {
		return ai.TechHockey
	}

	// Inside arrival radius or on near-flat terrain: just point and shoot.
	if perc.InArrival || perc.SlopeAngle < 0.05 {
		return ai.TechStraight
	}

	// Emergency: hockey stop if available and warranted.
	if intent.Urgency > 0.8 && t.Has(ai.TechHockey) {
		return ai.TechHockey
	}

	// High urgency without Hockey: fall through to a scrub-heavy technique.
	// Pizza is the universal "stop now" tool — wedge both edges, drag.
	// Without this clause, intermediate/beginner skiers in tree-emergency
	// situations would stay on Parallel turns (zero scrub) and never
	// actually slow down, because Sideslip's slope gate (below) is too
	// strict to fire on moderate terrain.
	if intent.Urgency > 0.8 {
		if t.Has(ai.TechPizza) {
			return ai.TechPizza
		}
		if t.Has(ai.TechSideslip) {
			return ai.TechSideslip
		}
	}

	// Sideslip on steeps when the agent is well over their comfort slope and
	// clearly speed-overloaded.
	if perc.SlopeAngle > traits.ComfortSlope*1.4 && perc.Speed > intent.Speed*1.3 && t.Has(ai.TechSideslip) {
		return ai.TechSideslip
	}

	// Speed control: prefer parallel turns when going too fast; fall back to
	// pizza for beginners. The parallel block also serves as the default
	// cruising technique because linked turns are how real skiers move.
	if perc.Speed > intent.Speed*1.05 {
		if t.Has(ai.TechParallel) {
			return ai.TechParallel
		}
		if t.Has(ai.TechPizza) {
			return ai.TechPizza
		}
	}

	// Default cruise.
	if t.Has(ai.TechParallel) {
		return ai.TechParallel
	}
	if t.Has(ai.TechPizza) {
		return ai.TechPizza
	}
	return ai.TechStraight
}

// parallelHeading drives the linked-turn oscillation. Arc width is anchored
// on slope-vs-comfort, not speed-vs-comfort: even on gentle terrain where the
// skier never reaches comfort speed, arcs stay wide enough to read as turns.
// On steep terrain the arcs widen further so cross-fall scrub does more work.
//
// A minimum dwell time per phase (parallelMinDwell) prevents sub-second
// pinging across the fall line — real carves carry the skier past axis and
// the next turn initiates only after the previous one has developed.
func parallelHeading(traits ai.SkierTraits, state ai.MotorState, intent Intent, perc Perception) (float32, ai.MotorState) {
	if state.TurnPhase == 0 {
		state.TurnPhase = 1
	}

	// Slope-driven arc width. ratio=1 means the agent is at their comfort
	// slope; below comfort still gets a healthy arc, well above comfort
	// pushes toward the max.
	comfortSlope := traits.ComfortSlope
	if comfortSlope < 1e-3 {
		comfortSlope = float32(20 * math.Pi / 180)
	}
	slopeRatio := perc.SlopeAngle / comfortSlope
	t := clamp32((slopeRatio-0.3)/1.0, 0, 1) // 0 at 30% of comfort, 1 at 130%
	arc := float32(parallelMinArc) + (float32(parallelMaxArc)-float32(parallelMinArc))*t

	desired := wrapAngle(intent.AxisHeading + float32(state.TurnPhase)*arc)

	// Phase flip: heading reached the arc edge AND the carve has had time to
	// develop. The dwell guard is what stops the squiggle.
	deviation := wrapAngle(perc.Heading-intent.AxisHeading) * float32(state.TurnPhase)
	if deviation > arc*0.85 && state.PhaseTime >= float32(parallelMinDwell) {
		state.TurnPhase = -state.TurnPhase
		state.PhaseTime = 0
	}

	return desired, state
}

// =============================================================================
// SECTION 7 — Stress / balance
// =============================================================================

// stressDelta is the rate of change of Balance per second. Negative values
// drain (toward a fall); positive values recover. Clamped to a sane range so
// numerical excursions can't yank Balance out from under the agent.
func stressDelta(traits ai.SkierTraits, perc Perception, intent Intent, cmd MotorCmd) float32 {
	d := float32(0.15) // base recovery

	if traits.ComfortSpeed > 0 && perc.Speed > traits.ComfortSpeed*1.2 {
		excess := perc.Speed/traits.ComfortSpeed - 1.2
		d -= excess * 0.3
	}
	if traits.ComfortSlope > 0 && perc.SlopeAhead > traits.ComfortSlope*1.2 {
		excess := perc.SlopeAhead/traits.ComfortSlope - 1.2
		d -= excess * 0.5
	}
	if h := worstHazard(perc.Trees); h.Severity > 0.5 && perc.Speed > 0.1 &&
		h.Distance/perc.Speed < treeStressTime {
		d -= h.Severity * 0.4
	}
	d -= cmd.BalanceCost

	if d < -1 {
		d = -1
	}
	if d > 0.4 {
		d = 0.4
	}
	return d
}

// =============================================================================
// SECTION 8 — Layer 5: Physics integration
// =============================================================================

func applyMotor(t *world.Terrain, a *world.Agent, cmd MotorCmd, perc Perception, dt float64) {
	turnRate := float32(maxAngularSpeed)
	if cmd.MaxTurnRate > 0 {
		turnRate = cmd.MaxTurnRate
	}
	a.Heading = rotateToward(a.Heading, cmd.Heading, turnRate, dt)

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
		float64(cmd.Scrub)
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
// SECTION 9 — Geometry helpers
// =============================================================================

// fallDirAndScale returns the unit downhill direction and a smoothstepped
// strength in [0, 1] reflecting slope steepness. On near-flat terrain (where
// the gradient is dominated by tile-quantization noise), strength is zero so
// steering ignores the noisy direction.
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

// rotateToward rotates `current` toward `desired` by at most maxRate*dt
// radians. Angles in radians using atan2(x, z) convention.
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

func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// =============================================================================
// SECTION 10 — Debug API (consumed by F3 overlay in scene/scenario.go)
// =============================================================================

// SteeringDebug is the legacy debug bundle used by the F3 overlay. Re-derived
// from a fresh perception+steer pass against `target` so the overlay shows
// what the agent would decide RIGHT NOW. ProbeDist is the speed-scaled
// look-ahead the simulation is currently using, exposed so the renderer can
// scale debug line lengths to match.
type SteeringDebug struct {
	Pos         mgl32.Vec3
	FallLine    mgl32.Vec2
	DesiredHead float32 // axis heading from the steering layer
	ProbeDist   float32 // m; current forward-probe distance
	Probes      [3]struct {
		Dir     mgl32.Vec2 // unit XZ; centre, right, left
		Density float32
	}
}

// ComputeSteeringDebug runs a non-mutating perception + steering pass for
// rendering. Used by scene/scenario.go's F3 overlay to visualise fall line,
// axis, and forward probes.
func ComputeSteeringDebug(t *world.Terrain, a *world.Agent, target mgl32.Vec3) SteeringDebug {
	// Synthesize a route just for the perception step.
	clone := *a
	clone.Route.GoalPos = target
	if clone.Traits.SightRange == 0 {
		clone.Traits.SightRange = 25 // sensible default for the overlay
	}
	perc := perceive(t, &clone)
	intent, _ := steer(clone.Traits, clone.Avoid, perc, 0)

	hx := float32(math.Sin(float64(a.Heading)))
	hz := float32(math.Cos(float64(a.Heading)))
	rx, rz := hz, -hx
	probeDir := func(angle float64) mgl32.Vec2 {
		c := float32(math.Cos(angle))
		s := float32(math.Sin(angle))
		return mgl32.Vec2{c*hx + s*rx, c*hz + s*rz}
	}

	probeDist := float32(treeLookahead) * a.Speed
	if probeDist < treeProbeMin {
		probeDist = treeProbeMin
	}
	if probeDist > treeProbeMax {
		probeDist = treeProbeMax
	}
	out := SteeringDebug{
		Pos:         a.Pos,
		FallLine:    perc.FallDir,
		DesiredHead: intent.AxisHeading,
		ProbeDist:   probeDist,
	}
	angles := [3]float64{0, probeAngle, -probeAngle}
	for i, ang := range angles {
		d := probeDir(ang)
		out.Probes[i].Dir = d
		out.Probes[i].Density = t.TreeDensityAt(a.Pos[0]+d[0]*probeDist, a.Pos[2]+d[1]*probeDist)
	}
	return out
}

// =============================================================================
// SECTION 11 — Recorder hook
// =============================================================================

// recordFrame is the bridge between the AI tick and the optional Recorder.
// Off the hot path unless a Recorder is attached and matches this agent.
func recordFrame(s *Simulation, a *world.Agent, target mgl32.Vec3, dist float32, perc Perception, intent Intent, cmd MotorCmd) {
	if s.Recorder == nil {
		return
	}
	if id := s.Recorder.AgentID(); id != 0 && id != a.ID {
		return
	}
	probeC, probeR, probeL := perceptionProbeDensities(perc)
	s.Recorder.Record(RecorderFrame{
		SimTime:         s.SimTime,
		AgentID:         a.ID,
		Activity:        world.Activity(s.World, a),
		Pos:             a.Pos,
		Heading:         a.Heading,
		Target:          target,
		Dist:            dist,
		Speed:           a.Speed,
		Technique:       a.Motor.Active,
		TurnPhase:       a.Motor.TurnPhase,
		FallLine:        perc.FallDir,
		AxisHeading:     intent.AxisHeading,
		DesiredHeading:  cmd.Heading,
		TargetSpeed:     intent.Speed,
		Urgency:         intent.Urgency,
		Balance:         a.Balance,
		ProbeC:          probeC,
		ProbeR:          probeR,
		ProbeL:          probeL,
		SlopeCos:        perc.Normal[1],
		InArrivalRadius: perc.InArrival,
	})
}

// computeProbeDist returns the speed-scaled tree-perception lookahead used
// by scanTrees, clamped to [treeProbeMin, treeProbeMax]. Exposed so the
// follow HUD / perception-cone shader can render the same fan the AI sees.
func computeProbeDist(speed float32) float32 {
	d := float32(treeLookahead) * speed
	if d < treeProbeMin {
		d = treeProbeMin
	}
	if d > treeProbeMax {
		d = treeProbeMax
	}
	return d
}

// senseFrom builds a display snapshot of the current tick's perception/intent/
// motor decisions. The AI never reads this back; it exists for the renderer
// and follow HUD to pick up between ticks.
func senseFrom(perc Perception, intent Intent, cmd MotorCmd) ai.Sense {
	probeC, probeR, probeL := perceptionProbeDensities(perc)
	hx := float32(math.Sin(float64(perc.Heading)))
	hz := float32(math.Cos(float64(perc.Heading)))
	var worst Hazard
	for _, h := range perc.Trees {
		if h.Severity > worst.Severity {
			worst = h
		}
	}
	var worstSide int8
	if worst.Severity > 0 {
		cross := hx*worst.Dir[1] - hz*worst.Dir[0]
		switch {
		case cross > 0.05:
			worstSide = +1
		case cross < -0.05:
			worstSide = -1
		}
	}
	return ai.Sense{
		ProbeDist:      computeProbeDist(perc.Speed),
		ProbeHalfAngle: probeAngle,
		ProbeC:         probeC,
		ProbeR:         probeR,
		ProbeL:         probeL,
		WorstSeverity:  worst.Severity,
		WorstDist:      worst.Distance,
		WorstSide:      worstSide,
		Urgency:        intent.Urgency,
		AxisHeading:    intent.AxisHeading,
		DesiredHeading: cmd.Heading,
		TargetSpeed:    intent.Speed,
		InTrees:        perc.InTrees,
		AtCellDensity:  perc.AtCellDensity,
		ClearDir:       perc.ClearDir,
		StrategicBias:  perc.StrategicBias,
	}
}

// perceptionProbeDensities re-derives the centre/right/left probe densities
// from the cone hazards. Cheap and lets the recorder stay synchronous with
// the previous CSV layout for analysis tools.
func perceptionProbeDensities(perc Perception) (centre, right, left float32) {
	centre = perc.TreeCenter
	for _, h := range perc.Trees {
		// Decide right/left by cross with current heading. A hazard whose
		// Dir × heading is positive is on the right, by our (sin, cos)
		// heading convention.
		hx := float32(math.Sin(float64(perc.Heading)))
		hz := float32(math.Cos(float64(perc.Heading)))
		cross := hx*h.Dir[1] - hz*h.Dir[0]
		if cross > 0 && h.Severity > right {
			right = h.Severity
		}
		if cross < 0 && h.Severity > left {
			left = h.Severity
		}
	}
	return
}
