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

	// In-trees signal. Above this underfoot density the HUD shows an
	// "IN TREES" badge for the followed skier. Trees are a soft cost —
	// they drain balance and scrub speed via physics — so the AI doesn't
	// branch on this; it just keeps skiing toward the goal. A future
	// GladeTolerance trait may modulate the badge / cost per skier.
	inTreesThreshold = 0.3

	// Path planner. When an obstacle blocks the direct line to the goal,
	// the planner reads a wide lateral profile across the forward axis,
	// enumerates distinct clear gaps wider than the skier's MinGapWidth
	// trait, and emits one waypoint at the chosen gap's centre. The skier
	// commits to that waypoint until it's consumed (reach / bypass /
	// below) — see waypointConsumed.
	planLookDepth        = 200.0 // m forward; deep enough to pierce a typical patch
	planLateralRange     = 150.0 // m; ±range scanned around the axis
	planLateralStep      = 5.0   // m; sample spacing across the lateral range
	planDensityThreshold = 0.15  // cell density above which a sample is "blocked"
	planMinDist          = 30.0  // m; below this, no plan (skier is close to goal — head straight)

	// Waypoint consumption thresholds. A waypoint is dropped once any of:
	//   reach: skier within waypointReachM of the waypoint
	//   bypass: skier closer to final goal than waypoint by waypointBypassM
	//   below: skier descended waypointDescentBufferM past wp.y
	// Three OR'd rules grounded in geometry/physics; no timer.
	waypointReachM         = 8.0
	waypointBypassM        = 5.0
	waypointDescentBufferM = 1.0

	// Confidence drift model. Confidence is a per-agent multiplier on
	// target speed that varies with forward outlook + recent state; it
	// captures the anticipation behaviour real skiers exhibit (straighten
	// out before a flat to ride momentum in; back off after a near-miss).
	confidenceDriftRate     = 0.25 // /s; half-convergence ~2.8 s — slow enough that the "warming-up wide-arc" phase lasts several seconds
	confidenceMin           = 0.5  // 50% of baseline target speed
	confidenceMax           = 1.5  // 150% of baseline target speed
	confSlopeGain           = 2.0  // /rad; gentler ahead → confidence up
	confHazardGain          = 0.4  // density 1.0 → -0.4 confidence target

	// Tuck anticipation: dispatcher widens the gentle-threshold when the
	// 10m-forward probe (perc.SlopeAhead) sees flatter terrain. This is
	// what makes the skier "straighten out before the runout" — independent
	// of overall confidence so a tentative skier on a uniform steep doesn't
	// tuck.
	tuckAnticipation = 1.0  // multiplier on (slopeNow - slopeAhead) added to gentle
	confBalanceGain         = 1.0  // balance shortfall below 0.7 → confidence down
	confBalanceShelf        = 0.7  // balance below this starts eroding confidence
	confBalanceGrowth       = 0.3  // bonus to target when balance is at confBalanceCeiling+
	confBalanceCeiling      = 0.9  // balance above this contributes to growth
	confPostFallReset       = 0.5  // confidence after recovery from a fall

	// Parallel-turn shaping. Confidence narrows the arc and lengthens the
	// dwell so a confident skier carves longer, tighter turns and a
	// tentative skier swings wider with more frequent direction changes.
	confArcOffsetMax    = 25 * math.Pi / 180 // rad; ±arc shift across [0.5, 1.5] confidence
	confDwellExtension  = 1.5                // s/conf; per +1.0 confidence above 1.0, dwell grows by 1.5 s

	// Spawn confidence — slightly cautious. A real skier doesn't start a
	// run at peak confidence; they warm up over the first few seconds.
	// Keeps initial arcs visibly wider, then narrows as the run goes
	// well. balance growth pulls confidence up to ~1.3 within a few
	// seconds on clear terrain.
	spawnConfidence = 0.7

	// Per-turn jitter magnitudes — applied as random offsets each time
	// the parallel turn phase flips. Real skiers don't make identical
	// turns; each carve is a little different in width and rhythm. The
	// dwell jitter is the bigger lever for visible "wavelength"
	// variation: cycle period ≈ 2 × dwell, so a ±0.7 s jitter on a
	// ~1 s baseline produces 0.6 s–3.4 s cycles → 8 m–44 m wavelengths
	// at 13 m/s. Arc jitter perturbs the amplitude/heading peaks.
	arcJitterMag     = 18 * math.Pi / 180 // ±18° random arc shift per turn
	dwellJitterMag   = 1.0                // ±1.0 s random dwell shift per turn
	jitterDwellFloor = 0.35               // s; absolute floor when jitter dips dwell low. Jitter is random per turn so a single 0.35 s carve doesn't ping; sustained sub-second turns are still prevented by the dwell baseline

	// Hockey stop pulse
	hockeyDurationS = 0.6
	hockeyScrub     = 8.0 // m/s² extra deceleration during the pulse
	hockeyBalCost   = 0.4 // balance drain per second while engaged

	// Route freshness
	routePlanInterval = 2.0 // sim-seconds between route refreshes

	// Energy budget. The fresh value (Energy=1.0) drains over this many
	// sim-seconds of *active skiing* (lift rides and walks don't count).
	// Calibrated so a typical agent gets ~20 descents at ~40 s of ski-time
	// each before depleting and routing home. Tune downward if scenarios
	// have shorter descents.
	energyBudgetSec = 800.0

	// energyLowThreshold gates the "head home" decision at lift-top
	// selection and lift-base arrival. Below this fraction the agent
	// picks a lodge instead of another lift — they don't have a confident
	// descent left in them. Roughly one descent's drain (~40 s ÷
	// energyBudgetSec ≈ 0.05). Strict ≤0 would let a skier commit to a
	// final ride they can't physically afford; this gives a soft margin.
	energyLowThreshold = 0.05
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

	// AxisDir / AxisDist point at whichever target the skier is currently
	// heading at: the front waypoint if the route has any, otherwise
	// GoalPos. Computed in perceive() via nextAxisTarget(a).
	AxisDir  mgl32.Vec2
	AxisDist float32

	Trees      []Hazard
	TreeCenter float32 // density at the centre forward probe

	// "I'm currently inside trees" — read from the agent's actual cell,
	// not the cone ahead. Display-only: the steering layer treats trees
	// as a soft cost and doesn't branch on this.
	AtCellDensity float32 // TreeDensityAt(Pos.X, Pos.Z) — density underfoot
	InTrees       bool    // AtCellDensity > inTreesThreshold

	// Confidence multiplier on target speed (Agent.Confidence). Synced in
	// tickSkier each tick after the Confidence drift step so steer() sees
	// the fresh value.
	Confidence float32

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
	// FlatSkis suppresses the muEdge perpendicular-friction term in the
	// physics step. Edge-friction models the energy cost of gripping the
	// slope during a carved turn — but a TechStraight glide rides the skis
	// flat, with no carving and no edge engagement, so the only friction
	// should be muBase + drag. Without this, a committed cross-slope
	// traverse decelerates as if the skier were carving the whole way,
	// losing speed they have no real-world reason to lose.
	FlatSkis bool
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

	// L1: route management.
	//   1. Brand-new or goal-changed → wipe and replan from scratch.
	//   2. Pop consumed waypoints (reach / bypass / below) — three OR'd
	//      rules grounded in geometry and physics, no timer.
	//   3. Stale refresh: replan only if the queue is empty. Sticky-once-
	//      chosen prevents the rng-driven planner from re-rolling a different
	//      gap each interval.
	if a.Route.Goal == ai.GoalNone || a.Route.GoalPos != target {
		a.Route = ai.Route{
			Goal:      routeGoalFor(s.World, a),
			GoalPos:   target,
			StaleAt:   s.SimTime + routePlanInterval,
			Waypoints: planWaypoints(s.World.Terrain, a.Pos, target, a.Traits.MinGapWidth, s.Rng),
		}
	}
	for len(a.Route.Waypoints) > 0 && waypointConsumed(a.Pos, target, a.Route.Waypoints[0]) {
		a.Route.Waypoints = a.Route.Waypoints[1:]
	}
	if a.Route.StaleAt < s.SimTime {
		if len(a.Route.Waypoints) == 0 {
			a.Route.Waypoints = planWaypoints(s.World.Terrain, a.Pos, target, a.Traits.MinGapWidth, s.Rng)
		}
		a.Route.StaleAt = s.SimTime + routePlanInterval
	}

	// L2: perceive
	perc := perceive(s.World.Terrain, a)

	// Confidence drift — slow per-tick adjustment of the speed multiplier
	// based on forward outlook and current state. Mutates a.Confidence;
	// we then sync into perc so this tick's steer() sees the fresh value.
	updateConfidence(a, perc, float32(dt))
	perc.Confidence = a.Confidence

	// L3: steer
	intent, avoid := steer(a.Traits, a.Avoid, perc, float32(dt), s.Rng)
	a.Avoid = avoid

	// L4: motor / technique
	cmd, motor := selectTechnique(a.Traits, a.Motor, intent, perc, float32(dt), s.Rng)
	a.Motor = motor

	// Snapshot perception/intent for the follow HUD and perception-cone
	// shader. Stale outside skiing — readers gate on Activity.
	a.Sense = senseFrom(a, perc, intent, cmd)

	// Energy drain — only ticks during active skiing, since this is the
	// only path that calls tickSkier. Reroute to a lodge at the next
	// decision boundary is handled in the dispatcher (pickTopTarget /
	// onArrive), not here, so a depleting skier finishes their current
	// descent cleanly.
	if a.Energy > 0 {
		a.Energy -= float32(dt / energyBudgetSec)
		if a.Energy < 0 {
			a.Energy = 0
		}
	}

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
		a.Confidence = confPostFallReset // skis cautious for a while after getting up
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

	// Axis target: front waypoint if the route has any, else GoalPos.
	// The route layer (tickSkier) is responsible for keeping the queue
	// trimmed to live waypoints — perceive is just a reader.
	axisTarget := nextAxisTarget(a)
	goalDelta := axisTarget.Sub(pos)
	axisXZ := mgl32.Vec2{goalDelta[0], goalDelta[2]}
	axisDist := axisXZ.Len()
	var axisDir mgl32.Vec2
	if axisDist > 1e-3 {
		axisDir = axisXZ.Mul(1.0 / axisDist)
	}

	trees, treeCenter := scanTrees(t, pos, a.Heading, a.Speed)

	atCell := t.TreeDensityAt(pos[0], pos[2])
	inTrees := atCell > inTreesThreshold

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
		InArrival:     axisDist < ArrivalRadius,
	}
}

// nextAxisTarget returns the world-space point the skier should currently
// head at: the front waypoint if the route has any, else the final goal.
func nextAxisTarget(a *world.Agent) mgl32.Vec3 {
	if len(a.Route.Waypoints) > 0 {
		return a.Route.Waypoints[0]
	}
	return a.Route.GoalPos
}

// planWaypoints reads the lateral density profile across the forward axis
// to the goal, enumerates clear runs wider than minGapWidth, and returns
// either an empty slice (no obstacle in the way, or the goal is too close
// for a meaningful scan) or a one-element slice containing the chosen gap
// centre. Pure function — no per-skier persistence; the caller (route
// refresh in tickSkier) is responsible for keeping the chosen waypoint
// "sticky" across refreshes.
//
// Algorithm:
//  1. Compute axis to goal. If goal is within planMinDist, no plan.
//  2. Sample tree density at planLookDepth m forward, across
//     ±planLateralRange laterally, every planLateralStep m.
//  3. Mark each lateral sample "clear" when density < planDensityThreshold.
//  4. Enumerate contiguous clear runs as candidate gaps.
//  5. Drop gaps narrower than minGapWidth (per-skier preference).
//  6. If the lateral profile is fully clear → no waypoint (skier heads
//     straight at the goal).
//  7. Otherwise pick one gap uniformly at random and return its centre,
//     elevation sampled from the terrain so the descent-past invalidation
//     rule has a real altitude to compare.
func planWaypoints(t *world.Terrain, pos, goal mgl32.Vec3, minGapWidth float32, rng *rand.Rand) []mgl32.Vec3 {
	delta := goal.Sub(pos)
	axisXZ := mgl32.Vec2{delta[0], delta[2]}
	axisDist := axisXZ.Len()
	if axisDist < 1e-3 {
		return nil
	}
	axisDir := axisXZ.Mul(1.0 / axisDist)

	lookDepth := float32(planLookDepth)
	if axisDist < lookDepth*1.4 {
		// Closer to goal: scan less far so the look point doesn't overshoot.
		lookDepth = axisDist * 0.7
	}
	if lookDepth < planMinDist {
		return nil
	}

	fx := pos[0] + axisDir[0]*lookDepth
	fz := pos[2] + axisDir[1]*lookDepth
	rx := axisDir[1]
	rz := -axisDir[0]

	nSamples := int(2*planLateralRange/planLateralStep) + 1
	clear := make([]bool, nSamples)
	anyBlocked := false
	for i := 0; i < nSamples; i++ {
		offset := -planLateralRange + float32(i)*planLateralStep
		sx := fx + rx*offset
		sz := fz + rz*offset
		clear[i] = t.TreeDensityAt(sx, sz) < planDensityThreshold
		if !clear[i] {
			anyBlocked = true
		}
	}
	if !anyBlocked {
		return nil
	}

	type gap struct {
		startIdx int
		endIdx   int // inclusive
	}
	gaps := make([]gap, 0, 4)
	i := 0
	for i < nSamples {
		if !clear[i] {
			i++
			continue
		}
		j := i
		for j < nSamples && clear[j] {
			j++
		}
		widthM := float32(j-i-1) * planLateralStep
		if widthM >= minGapWidth {
			gaps = append(gaps, gap{i, j - 1})
		}
		i = j
	}
	if len(gaps) == 0 {
		return nil
	}

	chosen := gaps[rng.Intn(len(gaps))]
	centerIdx := (chosen.startIdx + chosen.endIdx) / 2
	centerOffset := -planLateralRange + float32(centerIdx)*planLateralStep
	wpX := fx + rx*centerOffset
	wpZ := fz + rz*centerOffset

	// Sample terrain elevation at the chosen point so the descent-past
	// invalidation rule (waypointConsumed) has a real altitude to compare
	// against.
	wpY := t.InterpolatedElevationAt(wpX, wpZ)

	return []mgl32.Vec3{{wpX, wpY, wpZ}}
}

// waypointConsumed reports whether the skier has cleared the waypoint and
// the route layer should drop it. Three OR'd rules grounded in geometry
// and physics, no timer:
//
//   reach:  dist2D(pos, wp) <= waypointReachM
//           — skier is physically at the waypoint.
//   bypass: dist2D(pos, goal) + waypointBypassM <= dist2D(wp, goal)
//           — skier has gotten meaningfully closer to the final goal than
//             the waypoint is.
//   below:  pos.y < wp.y - waypointDescentBufferM
//           — skier has descended past the waypoint's altitude. Skiers
//             don't climb back up: once below, the waypoint is provably
//             unreachable, so a new plan is in order. This is the
//             unconditional safety net for the lateral-overshoot pathology
//             where the skier passes wide of the wp without either reach
//             or bypass firing.
func waypointConsumed(pos, goal, wp mgl32.Vec3) bool {
	dx := pos[0] - wp[0]
	dz := pos[2] - wp[2]
	if dx*dx+dz*dz <= waypointReachM*waypointReachM {
		return true
	}
	skierDist := mgl32.Vec2{goal[0] - pos[0], goal[2] - pos[2]}.Len()
	gapDist := mgl32.Vec2{goal[0] - wp[0], goal[2] - wp[2]}.Len()
	if skierDist+waypointBypassM <= gapDist {
		return true
	}
	if pos[1] < wp[1]-waypointDescentBufferM {
		return true
	}
	return false
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

func steer(traits ai.SkierTraits, avoid ai.AvoidState, perc Perception, dt float32, rng *rand.Rand) (Intent, ai.AvoidState) {
	// Axis: blend seek-toward-target (waypoint or goal) with fall-line bias,
	// attenuated by slope. On flats fallScale ≈ 0 → axis = seek; on steeps
	// the axis bends downhill.
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
	// Fully-boxed escape: every probed direction is blocked, so
	// bending picks an arbitrary side that's just as bad — and the
	// commit ends up oscillating as the skier crosses cell boundaries
	// and the "best" side flips. Drop the commit and let the skier
	// plow forward toward the target. Trees are a soft cost: the
	// existing TreeCenter speed scrub and physics balance drain
	// already extract the right penalty for skiing through them.
	centre, right, left := perceptionProbeDensities(perc)
	fullyBoxed := centre > 0.3 && right > 0.3 && left > 0.3
	switch {
	case maxSev > 0.3 && !fullyBoxed:
		avoid.Clear = 0
		if avoid.Side == 0 {
			hx := float32(math.Sin(float64(perc.Heading)))
			hz := float32(math.Cos(float64(perc.Heading)))
			avoid.Side = pickAvoidSide(perc.Trees, hx, hz, rng)
		}
		// Bend factor 1.0 means a fully-dense hazard rotates the axis
		// ~57° off the fall line — strong enough to skirt a wide
		// circular patch once committed early. Lower factors (0.35-0.6)
		// failed to pull the skier clear of a 60 m-radius grove in
		// headless tests.
		bend := 1.0 * maxSev
		axisHeading = wrapAngle(axisHeading + float32(avoid.Side)*bend)
	case fullyBoxed:
		avoid.Side = 0
		avoid.Clear = 0
	default:
		avoid.Clear += dt
		if avoid.Clear > 1.0 {
			avoid.Side = 0
		}
	}

	// Desired speed: comfort × aggression × dynamic Confidence, scaled
	// down on steeper terrain or when trees are dense ahead. Confidence
	// is the anticipation multiplier — raises the target when lookahead
	// is clear and gentler, drops it when steep/hazardous terrain is
	// coming or balance is shaky.
	conf := perc.Confidence
	if conf <= 0 {
		conf = 1.0 // legacy/saved agents that didn't init Confidence
	}
	speed := traits.ComfortSpeed * (0.7 + 0.6*traits.Aggression) * conf
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
// the heading-perpendicular (right of heading) to extract the lateral sign.
//
// Heading-perp (not axis-perp) matters: scanTrees lays its 5 probes
// symmetrically about heading, so projecting onto axis-perpendicular when
// heading ≠ axis introduces a sin(heading − axis) bias term that
// systematically pulls the skier to one side regardless of which side has
// trees.
//
// On a true symmetric tie, coin-flip via rng so we never silently default to
// one side. The committed side persists in AvoidState until the probes go
// clear.
func pickAvoidSide(hs []Hazard, headingX, headingZ float32, rng *rand.Rand) int8 {
	var ax, az float32
	for _, h := range hs {
		ax -= h.Dir[0] * h.Severity
		az -= h.Dir[1] * h.Severity
	}
	perpX, perpZ := headingZ, -headingX
	lateral := ax*perpX + az*perpZ
	if lateral > 1e-3 {
		return +1
	}
	if lateral < -1e-3 {
		return -1
	}
	if rng != nil && rng.Float32() < 0.5 {
		return -1
	}
	return +1
}

// =============================================================================
// SECTION 6 — Layer 4: Motor / Technique dispatch
// =============================================================================

// selectTechnique picks the best available technique for the current intent
// and skill, then computes a MotorCmd from that technique's profile. Returns
// the new MotorState so the caller can persist phase across ticks.
func selectTechnique(traits ai.SkierTraits, prev ai.MotorState, intent Intent, perc Perception, dt float32, rng *rand.Rand) (MotorCmd, ai.MotorState) {
	tech := pickTechnique(traits, prev, intent, perc)

	next := prev
	if next.Active != tech {
		next.Active = tech
		next.PhaseTime = 0
		// Re-pick a fresh phase when entering Parallel from another tech.
		// Direction is coin-flipped so the first arc isn't always to the
		// right — that tiny initial east drift was enough to bias outcomes
		// on a symmetric obstacle (skier ended up just east of patch center
		// when strategic engaged, then asymmetric reinforcement kicked in).
		if tech == ai.TechParallel && next.TurnPhase == 0 {
			if rng != nil && rng.Float32() < 0.5 {
				next.TurnPhase = -1
			} else {
				next.TurnPhase = 1
			}
		}
		if tech != ai.TechParallel {
			next.TurnPhase = 0
		}
	}
	next.PhaseTime += dt

	switch tech {
	case ai.TechStraight:
		return MotorCmd{Heading: intent.AxisHeading, Scrub: 0, BalanceCost: 0, FlatSkis: true}, next

	case ai.TechPizza:
		// Wedge: heading on axis, constant scrub. Higher balance cost than
		// parallel because plowing under load is tiring.
		return MotorCmd{Heading: intent.AxisHeading, Scrub: 4.5, BalanceCost: 0.05}, next

	case ai.TechWedgeTurn:
		return MotorCmd{Heading: intent.AxisHeading, Scrub: 3.0, BalanceCost: 0.06}, next

	case ai.TechParallel:
		heading, motor := parallelHeading(traits, next, intent, perc, rng)
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

	// Default cruise. Parallel turns only make sense when the slope is
	// steep enough to push the skier toward overspeed without active
	// scrub. Below half the skier's comfort slope, straight-running is
	// what a real skier does ("tuck through the runout"). The overspeed
	// branch above still grabs Parallel back if straight-running actually
	// builds too much speed, so it's the *default* that changes.
	//
	// Lookahead anticipation widens the gentle threshold when the 10m
	// forward probe sees gentler terrain. This is what makes the skier
	// "straighten out before the runout" — driven by the terrain feature,
	// not by overall confidence, so a tentative skier on a uniform steep
	// doesn't tuck and a confident skier in a tree patch doesn't either.
	gentle := traits.ComfortSlope * 0.5
	if perc.SlopeAhead > 0 {
		anticipation := perc.SlopeAngle - perc.SlopeAhead
		if anticipation > 0 {
			gentle += anticipation * float32(tuckAnticipation)
		}
	}
	if perc.SlopeAngle <= gentle {
		return ai.TechStraight
	}
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
func parallelHeading(traits ai.SkierTraits, state ai.MotorState, intent Intent, perc Perception, rng *rand.Rand) (float32, ai.MotorState) {
	if state.TurnPhase == 0 {
		// Coin-flip the first arc direction: a fixed +1 default biased the
		// initial pre-strategic drift east on every seed.
		if rng != nil && rng.Float32() < 0.5 {
			state.TurnPhase = -1
		} else {
			state.TurnPhase = 1
		}
		// Initial jitter for the first turn.
		state.ArcJitter = rng.Float32()*2 - 1
		state.DwellJitter = rng.Float32()*2 - 1
	}

	// Slope-driven arc width baseline. ratio=1 means the agent is at
	// their comfort slope; below comfort still gets a healthy arc, well
	// above comfort pushes toward the max.
	comfortSlope := traits.ComfortSlope
	if comfortSlope < 1e-3 {
		comfortSlope = float32(20 * math.Pi / 180)
	}
	slopeRatio := perc.SlopeAngle / comfortSlope
	t := clamp32((slopeRatio-0.3)/1.0, 0, 1) // 0 at 30% of comfort, 1 at 130%
	arc := float32(parallelMinArc) + (float32(parallelMaxArc)-float32(parallelMinArc))*t

	// Confidence offset: high conf narrows the arc (tight short-radius
	// turns; less scrub, more committed line); low conf widens it
	// (sweeping turns; lots of scrub, slow descent). Maps confidence
	// across [0.5, 1.5] to arc shifts of [+confArcOffsetMax, -confArcOffsetMax].
	conf := perc.Confidence
	if conf <= 0 {
		conf = 1.0
	}
	confT := clamp32((conf-0.5)/1.0, 0, 1) // 0 at conf=0.5, 1 at conf=1.5
	arc -= float32(confArcOffsetMax) * (confT*2 - 1)

	// Per-turn jitter: each carve gets a small random width offset so
	// successive turns aren't dimensionally identical.
	arc += float32(arcJitterMag) * state.ArcJitter

	if arc < float32(parallelMinArc) {
		arc = float32(parallelMinArc)
	}
	if arc > float32(parallelMaxArc) {
		arc = float32(parallelMaxArc)
	}

	// Confidence-extended dwell: high-conf skiers carve longer, more
	// committed turns; baseline at conf=1.0 is parallelMinDwell. We only
	// extend (never shorten below the existing safety floor) — the floor
	// exists to prevent sub-second pinging across the fall line.
	dwell := float32(parallelMinDwell)
	if conf > 1.0 {
		dwell += float32(confDwellExtension) * (conf - 1.0)
	}

	// Per-turn jitter on dwell as well — some turns last a beat longer,
	// others snap back early. We allow jitter to dip below
	// parallelMinDwell since the variation is random across turns
	// (not sustained), but cap the absolute minimum at jitterDwellFloor
	// so the agent can't ping mid-axis tick after tick.
	dwell += float32(dwellJitterMag) * state.DwellJitter
	if dwell < jitterDwellFloor {
		dwell = jitterDwellFloor
	}

	desired := wrapAngle(intent.AxisHeading + float32(state.TurnPhase)*arc)

	// Phase flip: heading reached the arc edge AND the carve has had time to
	// develop. The dwell guard is what stops the squiggle.
	deviation := wrapAngle(perc.Heading-intent.AxisHeading) * float32(state.TurnPhase)
	if deviation > arc*0.85 && state.PhaseTime >= dwell {
		state.TurnPhase = -state.TurnPhase
		state.PhaseTime = 0
		// Re-roll jitter for the next turn.
		state.ArcJitter = rng.Float32()*2 - 1
		state.DwellJitter = rng.Float32()*2 - 1
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
	edgeFriction := muEdge * gravity * cosTheta * sinOffAbs
	if cmd.FlatSkis {
		edgeFriction = 0
	}
	accel := gravity*sinTheta*cosOff -
		muBase*gravity*cosTheta -
		edgeFriction -
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
	intent, _ := steer(clone.Traits, clone.Avoid, perc, 0, nil)

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

		AvoidSide:    a.Avoid.Side,
		NextWaypoint: nextAxisTarget(a),
		WaypointsLeft: int8(len(a.Route.Waypoints)),
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

// updateConfidence drifts the agent's Confidence multiplier toward a target
// computed from forward outlook + balance state. Models the anticipation
// behaviour: a real skier raises their target speed when they read clear,
// gentler terrain ahead and lowers it when balance is shaky or hazards
// loom. The drift is slow (~1.4 s to half-converge) so short fluctuations
// don't whipsaw the speed target.
func updateConfidence(a *world.Agent, perc Perception, dt float32) {
	target := float32(1.0)

	// Outlook 1: slope ahead vs current. Gentler ahead → confidence up.
	// Uses the local 10m forward probe (perc.SlopeAhead). The previous
	// 300m strategic scan was dropped along with the rest of the
	// strategic-layer state; the local probe keeps the "tuck through the
	// runout" anticipation working at shorter range.
	if perc.SlopeAhead > 0 {
		slopeDelta := perc.SlopeAngle - perc.SlopeAhead
		target += confSlopeGain * slopeDelta
	}

	// Outlook 2: hazards ahead → confidence down. Uses the centre-probe
	// density (perc.TreeCenter) since the long-range PeakDensity was
	// dropped with the strategic scan. The signal is shorter-range now
	// (12-40m forward instead of 300m) but still drains confidence when
	// the immediate path has trees.
	target -= confHazardGain * perc.TreeCenter

	// Current state: low balance erodes confidence.
	if a.Balance < confBalanceShelf {
		target -= confBalanceGain * (confBalanceShelf - a.Balance)
	}

	// Warming up: high balance feeds confidence growth, gated by clear
	// outlook. "I feel good" requires both a healthy run-so-far AND a
	// benign-looking immediate path.
	if a.Balance > confBalanceCeiling {
		t := (a.Balance - confBalanceCeiling) / (1.0 - confBalanceCeiling)
		if t > 1 {
			t = 1
		}
		clearFactor := 1.0 - perc.TreeCenter
		if clearFactor < 0 {
			clearFactor = 0
		}
		target += confBalanceGrowth * t * clearFactor
	}

	// Slow drift toward target.
	a.Confidence += (target - a.Confidence) * dt * confidenceDriftRate
	if a.Confidence < confidenceMin {
		a.Confidence = confidenceMin
	}
	if a.Confidence > confidenceMax {
		a.Confidence = confidenceMax
	}
}

// senseFrom builds a display snapshot of the current tick's perception/intent/
// motor decisions. The AI never reads this back; it exists for the renderer
// and follow HUD to pick up between ticks.
func senseFrom(a *world.Agent, perc Perception, intent Intent, cmd MotorCmd) ai.Sense {
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
		InTrees:       perc.InTrees,
		AtCellDensity: perc.AtCellDensity,
		Confidence:    a.Confidence,
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
