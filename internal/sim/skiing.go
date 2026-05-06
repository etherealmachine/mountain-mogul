package sim

import (
	"math"

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

	// Parallel-turn arc width range
	parallelMinArc = 18 * math.Pi / 180
	parallelMaxArc = 65 * math.Pi / 180

	// Tree perception
	probeForwardDist = 12.0                // m
	probeAngle       = 35 * math.Pi / 180  // rad; ±offset for side probes

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
}

// =============================================================================
// SECTION 3 — Tick orchestration
// =============================================================================

// tickSkier runs one frame of the AI pipeline against `target`. Returns true
// when the agent has arrived (within ArrivalThreshold) — caller transitions
// state. Used by both StateSkiing (target = lift base) and
// StateReturningToLodge (target = lodge).
func (s *Simulation) tickSkier(a *world.Agent, target mgl32.Vec3, dt float64) bool {
	// Hard arrival check up front so we don't run a full pipeline for a
	// last-tick that just snaps to target.
	delta := target.Sub(a.Pos)
	dist := delta.Len()
	if dist < ArrivalThreshold {
		a.Pos = target
		return true
	}

	// L1: refresh route if stale or pointing somewhere else.
	if a.Route.Goal == ai.GoalNone || a.Route.GoalPos != target || a.Route.StaleAt < s.SimTime {
		a.Route = ai.Route{
			Goal:    routeGoalFor(a),
			GoalPos: target,
			StaleAt: s.SimTime + routePlanInterval,
		}
	}

	// L2: perceive
	perc := perceive(s.World.Terrain, a)

	// L3: steer
	intent := steer(a.Traits, perc)

	// L4: motor / technique
	cmd, motor := selectTechnique(a.Traits, a.Motor, intent, perc, float32(dt))
	a.Motor = motor

	// Balance drain & fall trigger.
	a.Balance += stressDelta(a.Traits, perc, intent, cmd) * float32(dt)
	if a.Balance > 1 {
		a.Balance = 1
	}
	if a.Balance <= 0 {
		a.Balance = 0
		a.ResumeState = a.State
		a.State = world.StateFallen
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

// tickFallen counts the agent down out of StateFallen and resumes. The
// caller (tickAgents) routes here when state == StateFallen.
func (s *Simulation) tickFallen(a *world.Agent, dt float64) {
	a.FallTimer -= float32(dt)
	if a.FallTimer <= 0 {
		a.State = a.ResumeState
		if a.State == world.StateFallen { // safety; shouldn't happen
			a.State = world.StateSkiing
		}
		a.Balance = 0.7
		a.Speed = 0
		a.Motor = ai.MotorState{}
	}
}

// routeGoalFor picks the GoalKind matching the agent's current state.
func routeGoalFor(a *world.Agent) ai.GoalKind {
	if a.State == world.StateReturningToLodge {
		return ai.GoalLodge
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

	trees, treeCenter := scanTrees(t, pos, a.Heading, a.Traits.SightRange)

	return Perception{
		Pos:        pos,
		Heading:    a.Heading,
		Speed:      a.Speed,
		Normal:     n,
		SlopeAngle: slope,
		SlopeAhead: slopeAhead,
		FallDir:    fall,
		FallScale:  fallScale,
		AxisDir:    axisDir,
		AxisDist:   axisDist,
		Trees:      trees,
		TreeCenter: treeCenter,
		InArrival:  axisDist < ArrivalRadius,
	}
}

// scanTrees samples tree density at five forward probes and returns hazards
// for any probe whose density exceeds a noise threshold. Centre density is
// returned separately so the steering layer can quickly modulate target speed.
func scanTrees(t *world.Terrain, pos mgl32.Vec3, heading, sightRange float32) ([]Hazard, float32) {
	hx := float32(math.Sin(float64(heading)))
	hz := float32(math.Cos(float64(heading)))
	rx := hz
	rz := -hx

	probeDist := sightRange * 0.6
	if probeDist < probeForwardDist {
		probeDist = probeForwardDist
	}

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

func steer(traits ai.SkierTraits, perc Perception) Intent {
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

	// Tree avoidance: bend axis away from the worst hazard within view.
	if h := worstHazard(perc.Trees); h.Severity > 0.3 {
		// Cross product sign determines which side the hazard is on.
		cross := bx*h.Dir[1] - bz*h.Dir[0]
		side := float32(1)
		if cross > 0 {
			side = -1
		}
		bend := 0.35 * h.Severity // up to ~20°
		axisHeading = wrapAngle(axisHeading + side*bend)
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
	if h := worstHazard(perc.Trees); h.Severity > 0.6 && h.Distance < traits.SightRange*0.4 {
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
	}
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
		heading, motor := parallelHeading(next, intent, perc)
		return MotorCmd{Heading: heading, Scrub: 0, BalanceCost: 0.02}, motor

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

// parallelHeading drives the linked-turn oscillation. Arc width grows with
// the speed-over-target ratio so a fast skier turns wider arcs to scrub
// (carving friction in physics translates wider arcs into more deceleration);
// a controlled skier holds tighter arcs for cleaner progress along the axis.
func parallelHeading(state ai.MotorState, intent Intent, perc Perception) (float32, ai.MotorState) {
	if state.TurnPhase == 0 {
		state.TurnPhase = 1
	}

	speedRatio := perc.Speed / max32(intent.Speed, 1)
	t := clamp32((speedRatio-0.6)/0.6, 0, 1) // 0 below comfort, 1 at 1.2x
	arc := float32(parallelMinArc) + (float32(parallelMaxArc)-float32(parallelMinArc))*t

	desired := wrapAngle(intent.AxisHeading + float32(state.TurnPhase)*arc)

	// Phase flip: heading has rotated through ~85% of the current side's arc.
	deviation := wrapAngle(perc.Heading-intent.AxisHeading) * float32(state.TurnPhase)
	if deviation > arc*0.85 {
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
	if h := worstHazard(perc.Trees); h.Severity > 0.5 && h.Distance < traits.SightRange*0.3 {
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
	a.Heading = rotateToward(a.Heading, cmd.Heading, float32(maxAngularSpeed), dt)

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

// ProbeDistance is the forward distance used when drawing perception probes
// in the debug overlay. Exposed so the renderer can scale line lengths.
const ProbeDistance = probeForwardDist

// SteeringDebug is the legacy debug bundle used by the F3 overlay. Re-derived
// from a fresh perception+steer pass against `target` so the overlay shows
// what the agent would decide RIGHT NOW.
type SteeringDebug struct {
	Pos         mgl32.Vec3
	FallLine    mgl32.Vec2
	DesiredHead float32 // axis heading from the steering layer
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
	intent := steer(clone.Traits, perc)

	hx := float32(math.Sin(float64(a.Heading)))
	hz := float32(math.Cos(float64(a.Heading)))
	rx, rz := hz, -hx
	probeDir := func(angle float64) mgl32.Vec2 {
		c := float32(math.Cos(angle))
		s := float32(math.Sin(angle))
		return mgl32.Vec2{c*hx + s*rx, c*hz + s*rz}
	}

	out := SteeringDebug{
		Pos:         a.Pos,
		FallLine:    perc.FallDir,
		DesiredHead: intent.AxisHeading,
	}
	angles := [3]float64{0, probeAngle, -probeAngle}
	for i, ang := range angles {
		d := probeDir(ang)
		out.Probes[i].Dir = d
		out.Probes[i].Density = t.TreeDensityAt(a.Pos[0]+d[0]*probeForwardDist, a.Pos[2]+d[1]*probeForwardDist)
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
		State:           a.State,
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
