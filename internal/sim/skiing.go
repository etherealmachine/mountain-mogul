package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/ai/goap"
	"mountain-mogul/internal/rng"
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
	groomingBonus    = 0.5 // weight on Σ grooming along the candidate path — pulls skiers toward corduroy on clear slopes, outvoted by trees when present
	// groomEdgePenalty is added to a candidate's totalDensity (→ treePenalty
	// multiplier) each time the sample transitions from groomed to ungroomed
	// terrain. Only applied when self.Traits.PrefersGroomed is true. This
	// treats the piste edge as a soft obstacle and prevents the progressBonus
	// from pulling the skier off a curved groomed trail.
	groomEdgePenalty = 1.5

	// Width of the corridor the sampler treats as "the skier" when
	// integrating tree density along a candidate path. Each segment reads
	// density at the centre AND at ±corridorHalfWidth perpendicular, taking
	// the worst — so a path that grazes a tree edge scores as poorly as
	// one through the trunk. A 10 m half-width keeps the skier ~2 cells
	// off any dense cell.
	corridorHalfWidth = 10.0

	// Hazard avoidance: nearby lift towers and other skiers register a
	// density bump in the sampler so the controller routes around them
	// like it does for trees. The radii are how far from a hazard a
	// sample point starts seeing penalty — picked so the corridor sweep
	// catches them well before the skier brushes through.
	towerHazardRadius = 5.0 // lift towers are 0.6–0.9 m poles; the radius gives ~4 m of carving room
	skierHazardRadius = 2.5 // ski-width is ~0.6 m; the radius is "don't ski into someone's blind spot"


	// Fall-line attenuation. Identical to prior model — gentle terrain has
	// noisy gradients, so we ignore the fall direction below flatSlopeL.
	flatSlopeL  = 0.05
	steepSlopeL = 0.20

	// Balance / fall
	fallRecoverTime  = 4.0
	fallStartBalance = 0.7

	// Tree underfoot signal (display-only — controller doesn't branch on it)
	inTreesThreshold = 0.3

	// Snow wear from skier traffic.
	//   groomingWearRate: Grooming decays at this rate (1/s). ~100 passes wipe
	//     fresh corduroy; a 5 m cell at 10 m/s sees ~0.5 s per pass.
	//   trafficRate: SkierTraffic units accumulated per second while skiing.
	//   trafficThresh*: SkierTraffic thresholds that trigger a kind transition.
	//   mogulFormRate: mogul growth rate, scaled by (1 − Grooming).
	groomingWearRate    = 0.005
	trafficRate         = 1.0  // units/s; ~0.5 units per pass at 10 m/s
	trafficThreshPowder  = float32(40)  // ~40 passes
	trafficThreshWindSlab = float32(60)
	trafficThreshCrust   = float32(20)  // crust shatters quickly
	mogulFormRate        = 0.005
	mogulMinSnowDepth    = 0.3

	// patienceGainPerSecSkiing is patience restored per sim-second of
	// active downhill skiing. Offset against the drain from queuing —
	// a guest who skis freely without long waits stays patient all day.
	patienceGainPerSecSkiing = 1.0 / 1000.0

	// Energy drain rates. Normal skiing drains in ~2 hours of continuous
	// skiing. Falls cause a large one-shot hit. Ungroomed-snow penalties
	// are applied by energyDrainRate based on skill tier × snow kind.
	energyDrainPerSecSkiing = 1.0 / 7200.0
	energyFallDrain         = 0.30

	// Hunger drains at a fixed rate regardless of terrain.
	// Full drain in 1 in-game day (240 sim-seconds).
	hungerDrainPerSec = 1.0 / 240.0

	// Thirst base rate (1 in-game day full drain) scaled by altitude and exertion.
	thirstDrainPerSec      = 1.0 / 240.0
	thirstAltitudePerMetre = float32(0.0005) // +50% at 1000 m, ×2 at 2000 m

	// criticalStatThreshold mirrors goap.restTriggerThreshold: below this
	// level ThoughtHungry / ThoughtThirsty are emitted each tick (rate-
	// limited by ThoughtTTL) so the departure thought reflects the cause.
	criticalStatThreshold = float32(0.15)

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

// energyDrainRate returns the per-second energy drain for a guest skiing at
// the given skill level on the given surface. Groomed snow equalises all tiers
// at the base rate. On ungroomed snow the rate scales with the mismatch between
// skill and conditions:
//
//	               groomed   ungroomed   powder/boilerplate
//	Beginner         1×         6×              6×
//	Intermediate     1×         3×              6×
//	Advanced         1×         1×              3×
func energyDrainRate(skill float32, kind world.SnowKind, groomed bool) float32 {
	if groomed {
		return energyDrainPerSecSkiing
	}
	hard := kind == world.KindPowder || kind == world.KindBoilerplate
	switch {
	case skill >= ai.SkillAdvancedThreshold:
		if hard {
			return energyDrainPerSecSkiing * 3
		}
		return energyDrainPerSecSkiing
	case skill >= ai.SkillIntermediateThreshold:
		if hard {
			return energyDrainPerSecSkiing * 6
		}
		return energyDrainPerSecSkiing * 3
	default: // Beginner
		return energyDrainPerSecSkiing * 6
	}
}

// thirstExertionMultiplier returns how much harder the skier is sweating
// based on terrain difficulty. Uses the same tier logic as energyDrainRate
// but capped at 3× (thirst from sweating, not exhaustion).
func thirstExertionMultiplier(skill float32, kind world.SnowKind, groomed bool) float32 {
	if groomed {
		return 1.0
	}
	hard := kind == world.KindPowder || kind == world.KindBoilerplate
	switch {
	case skill >= ai.SkillAdvancedThreshold:
		if hard {
			return 2.0
		}
		return 1.0
	case skill >= ai.SkillIntermediateThreshold:
		if hard {
			return 3.0
		}
		return 2.0
	default: // Beginner
		return 3.0
	}
}

// =============================================================================
// SECTION 3 — Tick orchestration
// =============================================================================

// tickSkier runs one frame of the controller against `target`. Returns true
// when the agent has arrived (within ArrivalThreshold). a.Plan is set
// once at step-start by sim.onPlanStepStart; this function never re-reads
// strategic state.
func (s *Simulation) tickSkier(a *world.Guest, target mgl32.Vec3, dt float64) bool {
	delta := target.Sub(a.Pos)
	dist := delta.Len()
	if dist < ArrivalThreshold {
		a.Pos = target
		return true
	}

	perc := perceive(s.World.Terrain, a, target)

	dec := decide(s.World, s.towersScratch, s.spatial, a, perc, float32(dt))
	if dec.TurnSide != a.TurnSide {
		a.TurnDwell = 0
	} else {
		a.TurnDwell += float32(dt)
	}
	a.TurnSide = dec.TurnSide
	a.LastTactical = dec.TacticalOffset
	a.Sense = senseFrom(perc, dec)

	// Trait-driven terrain thoughts. Reads the cell under the guest and
	// emits thoughts based on glade and grooming preferences — these
	// thoughts count toward the session rating.
	cx := int(a.Pos[0] / CellSize)
	cz := int(a.Pos[2] / CellSize)
	var treeDensity, grooming float32
	var surfKind world.SnowKind
	if s.World.Terrain.InBounds(cx, cz) {
		cell := s.World.Terrain.Cells[cx][cz]
		treeDensity = cell.TreeDensity
		grooming = cell.Grooming
		if top := cell.TopLayer(); top != nil {
			surfKind = top.Kind
		}
	}
	const (
		treeDensityThreshold = 0.30
		groomingThreshold    = 0.50
	)
	inTrees := treeDensity >= treeDensityThreshold
	onGroomed := grooming >= groomingThreshold

	if inTrees {
		if a.Traits.LikesGlades {
			s.addThought(a, ai.ThoughtLovingGlades)
		} else {
			s.addThought(a, ai.ThoughtScaredInTrees)
		}
	}
	// Accumulate grooming for the run-end ThoughtLovingCorduroy check.
	a.RunGroomingSum += grooming
	a.RunGroomingSamples++

	// Satisfaction drift toward a terrain-quality target. The target is
	// derived from trait/terrain combinations; the drift rate is slow
	// enough that a brief bad patch doesn't tank the score but sustained
	// poor conditions (long tree run for a non-glade skier) do matter.
	{
		target := float32(0.5)
		if inTrees {
			if a.Traits.LikesGlades {
				target += 0.12
			} else {
				target -= 0.18
			}
		}
		if a.Traits.PrefersGroomed {
			if onGroomed {
				target += 0.15
			} else {
				target -= 0.08
			}
		}
		target = clamp32(target, 0.25, 0.80)
		const driftRate = float32(0.006)
		a.Satisfaction += (target - a.Satisfaction) * driftRate * float32(dt)
	}

	// Patience gain from active skiing.
	a.Patience += float32(dt * patienceGainPerSecSkiing)
	if a.Patience > 1 {
		a.Patience = 1
	}

	// Energy drain from skiing. Rate depends on skill tier × snow kind;
	// see energyDrainRate for the full table.
	a.Energy -= energyDrainRate(a.Traits.Skill, surfKind, onGroomed) * float32(dt)
	if a.Energy < 0 {
		a.Energy = 0
	}

	// Hunger: fixed-rate drain regardless of terrain.
	a.Hunger -= hungerDrainPerSec * float32(dt)
	if a.Hunger < 0 {
		a.Hunger = 0
	}

	// Thirst: altitude × exertion scaled drain.
	altFactor := 1 + a.Pos[1]*thirstAltitudePerMetre
	a.Thirst -= thirstDrainPerSec * altFactor * thirstExertionMultiplier(a.Traits.Skill, surfKind, onGroomed) * float32(dt)
	if a.Thirst < 0 {
		a.Thirst = 0
	}

	// Emit thoughts when critically low so the departure log reflects cause.
	// AddThought's TTL suppression prevents spam.
	if a.Hunger < criticalStatThreshold {
		s.addThought(a, ai.ThoughtHungry)
	}
	if a.Thirst < criticalStatThreshold {
		s.addThought(a, ai.ThoughtThirsty)
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
		a.Energy = clamp32(a.Energy-energyFallDrain, 0, 1)
		a.Events = append(a.Events, ai.GuestEvent{Kind: ai.EventFall, Time: s.SimTime})
		if step := a.Plan.Head(); step.Kind == ai.ActSkiTrail {
			s.addThought(a, ai.ThoughtFell, step.TrailID)
		} else {
			s.addThought(a, ai.ThoughtFell)
		}
		a.Satisfaction = clamp32(a.Satisfaction-0.10, 0, 1)
		recordFrame(s, a, target, dist, perc, dec)
		return false
	}

	prevPos := a.Pos
	apply(s.World.Terrain, a, dec, perc, dt)
	wearSnowUnderfoot(s.World.Terrain, a.Pos, dt)
	splatSkierTrack(s.World.Terrain, a, prevPos)
	recordFrame(s, a, target, dist, perc, dec)
	return false
}

// splatSkierTrack writes the agent's segment from prevPos to a.Pos into
// the surface-detail R channel. Gated on the same "actively skiing"
// conditions tickSkier itself enforces — the dispatcher already routes
// us here only when locomotion is live, but Fallen can flip mid-tick
// and Speed can dip below the splat threshold during slow turns.
//
// minSplatSpeed is set so a stationary-but-twitching skier (e.g. holding
// at the top of a queue spread) doesn't paint dots underfoot.
const minSplatSpeed = float32(1.0) // m/s

func splatSkierTrack(t *world.Terrain, a *world.Guest, prevPos mgl32.Vec3) {
	if t == nil || t.Surface == nil {
		return
	}
	if a.Fallen || a.OnLiftID != 0 || a.Queued || a.Speed < minSplatSpeed {
		// State that breaks the "actively carving down the hill" rule —
		// reset LastTrackPos so the next splat starts a fresh segment
		// rather than drawing a line through the lift/queue.
		a.LastTrackPos = mgl32.Vec3{}
		return
	}
	// First splat after a state reset — anchor on current pos so the
	// next substep extends a real segment.
	if a.LastTrackPos == (mgl32.Vec3{}) {
		a.LastTrackPos = prevPos
	}
	// Intensity 64 ≈ 25 % R per substep; with continuous skiing the
	// 3×3 disks overlap into a saturated line within a few ticks.
	const intensity = uint8(64)
	t.Surface.SplatTrackSegment(
		a.LastTrackPos[0], a.LastTrackPos[2],
		a.Pos[0], a.Pos[2], intensity,
	)
	a.LastTrackPos = a.Pos
}

// wearSnowUnderfoot mutates the cell beneath the agent to model skier
// traffic on the snow surface. Grooming decays (skis cut up the corduroy),
// Packed rises (boots and edges compact the column). SWE — held in
// Cell.SnowAccumulation — is conserved by construction; the visible snow
// depth therefore drops automatically as packing rises (depth =
// accumulation / density(packed)), matching how real snow behaves under
// traffic and groomer treads. MogulSize grows proportionally to
// (1 − Grooming) on cells with enough visible snow to form a bump.
func wearSnowUnderfoot(t *world.Terrain, pos mgl32.Vec3, dt float64) {
	xi := int(pos[0] / world.CellSize)
	zi := int(pos[2] / world.CellSize)
	if !t.InBounds(xi, zi) {
		return
	}
	c := &t.Cells[xi][zi]
	dirty := false
	if c.Grooming > 0 {
		c.Grooming -= float32(groomingWearRate * dt)
		if c.Grooming < 0 {
			c.Grooming = 0
		}
		dirty = true
	}
	// Accumulate traffic and trigger kind transitions at thresholds.
	c.SkierTraffic += float32(trafficRate * dt)
	if top := c.TopLayer(); top != nil {
		var thresh float32
		switch top.Kind {
		case world.KindPowder:
			thresh = trafficThreshPowder
		case world.KindWindSlab:
			thresh = trafficThreshWindSlab
		case world.KindCrust:
			thresh = trafficThreshCrust
		}
		if thresh > 0 && c.SkierTraffic >= thresh {
			top.Kind = world.KindPackedPowder
			c.SkierTraffic = 0
			dirty = true
		}
	}
	if c.VisibleSnowDepth() > mogulMinSnowDepth && c.MogulSize < 1 {
		c.MogulSize += float32(mogulFormRate*dt) * (1 - c.Grooming)
		if c.MogulSize > 1 {
			c.MogulSize = 1
		}
		dirty = true
	}
	if dirty {
		t.SnowDirty = true
	}
}

// tickFallen counts the agent down out of the fallen window and resumes.
func (s *Simulation) tickFallen(a *world.Guest, dt float64) {
	a.FallTimer -= float32(dt)
	if a.FallTimer <= 0 {
		a.Fallen = false
		a.Balance = float32(fallStartBalance)
		a.Speed = 0
		a.TurnSide = 0
	}
}

// =============================================================================
// SECTION 4 — Walking / ski transitions
// =============================================================================

// onBuildingFootprint returns true when world-space (x, z) is on or near any
// placed building's footprint. A half-cell margin is added on all sides so
// that pathfinder routes through cells adjacent to (but not the door cell of)
// a building also trigger the check — without the margin those cell centres
// can fall just outside the exact AABB edge.
func onBuildingFootprint(w *world.World, x, z float32) bool {
	const margin = world.CellSize / 2
	for _, b := range w.Buildings {
		minX, minZ, maxX, maxZ := world.FootprintAABB(b.Type, b.Pos[0], b.Pos[1])
		if x >= minX-margin && x <= maxX+margin && z >= minZ-margin && z <= maxZ+margin {
			return true
		}
	}
	return false
}

// noSnowUnderfoot returns true when the cell beneath world-space (x, z) has
// no snow layers at all (bare ground).
func noSnowUnderfoot(t *world.Terrain, x, z float32) bool {
	xi := int(x / world.CellSize)
	zi := int(z / world.CellSize)
	if !t.InBounds(xi, zi) {
		return false
	}
	return t.Cells[xi][zi].TopLayer() == nil
}

// mustWalk returns true when the agent is on terrain that requires walking
// rather than skiing: a building footprint or bare ground with no snow.
func mustWalk(w *world.World, x, z float32) bool {
	return onBuildingFootprint(w, x, z) || noSnowUnderfoot(w.Terrain, x, z)
}

// maybeStartSkiTransition triggers the 1-second ski equip/unequip pause when
// the agent crosses into or out of walk-required terrain.  Only called when
// no transition is already in progress.
func (s *Simulation) maybeStartSkiTransition(a *world.Guest) {
	walk := mustWalk(s.World, a.Pos[0], a.Pos[2])
	if walk && a.SkisOn {
		a.SkiTransitionTimer = 1.0 // removing skis
	} else if !walk && !a.SkisOn {
		a.SkiTransitionTimer = -1.0 // putting skis on
	}
}

// tickSkiTransition counts down the equip/unequip timer and flips SkisOn
// when it completes.  The agent stands still during the transition.
func (s *Simulation) tickSkiTransition(a *world.Guest, dt float64) {
	a.Speed = 0
	if a.SkiTransitionTimer > 0 {
		a.SkiTransitionTimer -= float32(dt)
		if a.SkiTransitionTimer <= 0 {
			a.SkiTransitionTimer = 0
			a.SkisOn = false
		}
	} else {
		a.SkiTransitionTimer += float32(dt)
		if a.SkiTransitionTimer >= 0 {
			a.SkiTransitionTimer = 0
			a.SkisOn = true
		}
	}
}

// =============================================================================
// SECTION 5 — Perception
// =============================================================================

func perceive(t *world.Terrain, a *world.Guest, target mgl32.Vec3) Perception {
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
func decide(w *world.World, towers []mgl32.Vec2, grid *spatialGrid, a *world.Guest, perc Perception, dt float32) Decision {
	axisHeading := composeAxis(perc)

	tactical, obstacleSeen, probeC, probeR, probeL := sampleTactical(w, towers, grid, a, perc, axisHeading, a.LastTactical)

	// Speed control. Base target from skill/traits, then reduce when trees
	// are visible in the forward fan — real skiers back off in glades to
	// give themselves more reaction time. Worst-probe density saturates the
	// reduction at 40% (target × 0.6) so even dense trees don't drop the
	// skier below half their cruising speed.
	targetSpeed := desiredSpeed(a.Traits, perc)
	worstProbe := probeC
	if probeR > worstProbe {
		worstProbe = probeR
	}
	if probeL > worstProbe {
		worstProbe = probeL
	}
	targetSpeed *= 1.0 - 0.4*clamp32(worstProbe/0.4, 0, 1)
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
		side = pickInitialSide(perc, deviation)
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
func desiredSpeed(traits ai.GuestTraits, perc Perception) float32 {
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
func pickInitialSide(perc Perception, deviation float32) int8 {
	// If heading already favours a side, commit to that — avoids an ugly
	// 180° flip in the first tick of the carve.
	if deviation > 0.05 {
		return +1
	}
	if deviation < -0.05 {
		return -1
	}
	if rng.Global().Float32() < 0.5 {
		return -1
	}
	return +1
}

// collectTowerXZs gathers the XZ positions of every lift tower in the
// world into a single slice. Computed once per sampleTactical call so
// the corridor-sample inner loop can iterate towers without re-walking
// the lift list (or re-allocating per-lift slices) at every probe point.
func collectTowerXZs(w *world.World) []mgl32.Vec2 {
	if len(w.Lifts) == 0 {
		return nil
	}
	var out []mgl32.Vec2
	for _, lift := range w.Lifts {
		out = append(out, lift.TowerXZs()...)
	}
	return out
}

// hazardDensityAt returns a [0, 1]-ish penalty at world (x, z), combining:
//
//   - terrain TreeDensity (the existing signal)
//   - linear falloff inside towerHazardRadius of any lift tower
//   - linear falloff inside skierHazardRadius of any other skier
//
// Combination is max(): one big hazard dominates. Tree-only paths score
// the same as before this function existed; tower/skier presence raises
// the penalty so the candidate-fan scorer routes around them.
//
// The agent term uses a spatial grid bucketed once per Tick — without
// it, this function dominated the profile (70% CPU) once active agent
// count crossed ~150 because every probe point iterated every other
// agent. With the grid, hazardDensityAt is O(towers + nearby) per call.
func hazardDensityAt(t *world.Terrain, towers []mgl32.Vec2, grid *spatialGrid, selfID uint64, x, z float32) float32 {
	d := t.TreeDensityAt(x, z)

	const towerR2 = towerHazardRadius * towerHazardRadius
	for _, p := range towers {
		dx := p[0] - x
		dz := p[1] - z
		r2 := dx*dx + dz*dz
		if r2 < towerR2 {
			f := 1 - r2/towerR2
			if f > d {
				d = f
			}
		}
	}

	const skierR2 = skierHazardRadius * skierHazardRadius
	if grid != nil {
		grid.forEachNear(x, z, func(other *world.Guest) {
			if other.ID == selfID {
				return
			}
			dx := other.Pos[0] - x
			dz := other.Pos[2] - z
			r2 := dx*dx + dz*dz
			if r2 < skierR2 {
				f := 1 - r2/skierR2
				if f > d {
					d = f
				}
			}
		})
	}
	return d
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
func sampleTactical(w *world.World, towers []mgl32.Vec2, grid *spatialGrid, self *world.Guest, perc Perception, axisHeading, prevTactical float32) (offset float32, obstacleSeen bool, probeC, probeR, probeL float32) {
	t := w.Terrain
	horizon := perc.Speed * float32(sampleHorizonSec)
	if horizon < float32(sampleMinDist) {
		horizon = float32(sampleMinDist)
	}
	if horizon > float32(sampleMaxDist) {
		horizon = float32(sampleMaxDist)
	}

	// `towers` is the shared per-Tick scratch built once in Simulation.
	// Self is excluded from the agent list so the skier doesn't avoid
	// itself.
	var selfID uint64
	if self != nil {
		selfID = self.ID
	}

	// Current-cell grooming used as the starting point for groom-edge
	// crossing detection. Only relevant when self.Traits.PrefersGroomed.
	var startGrooming float32
	prefersGroomed := self != nil && self.Traits.PrefersGroomed
	if prefersGroomed && t.InBoundsWorld(perc.Pos[0], perc.Pos[2]) {
		_, startGrooming, _, _ = t.SnowAt(perc.Pos[0], perc.Pos[2])
	}

	// Pass 1: integrate density, boundary hits, and grooming along each
	// candidate. Grooming reads the centre point only — corridor sampling
	// would double-count adjacent cells.
	type sampleData struct {
		ang                float32
		totalDensity       float32
		totalGrooming      float32
		boundaryHits       int
		groomEdgeCrossings int // groomed→ungroomed transitions (PrefersGroomed only)
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

		var totalDensity, totalGrooming float32
		var boundaryHits, groomEdgeCrossings int
		prevGrooming := startGrooming
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
			// Hazard includes trees, lift towers, and other skiers.
			density := hazardDensityAt(t, towers, grid, selfID, x, z)
			if dl := hazardDensityAt(t, towers, grid, selfID, x-rx*float32(corridorHalfWidth), z-rz*float32(corridorHalfWidth)); dl > density {
				density = dl
			}
			if dr := hazardDensityAt(t, towers, grid, selfID, x+rx*float32(corridorHalfWidth), z+rz*float32(corridorHalfWidth)); dr > density {
				density = dr
			}
			totalDensity += density
			_, grooming, _, _ := t.SnowAt(x, z)
			if prefersGroomed && prevGrooming > 0.5 && grooming < 0.5 {
				groomEdgeCrossings++
			}
			prevGrooming = grooming
			totalGrooming += grooming
		}
		samples[i] = sampleData{ang, totalDensity, totalGrooming, boundaryHits, groomEdgeCrossings}
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

	// Groom-preferring skiers (beginner/intermediate) weight corduroy 4× more
	// strongly than the default — enough to dominate direction choice on clear
	// slopes without being overridden by tree avoidance when hazards are present.
	groomWeight := float32(groomingBonus)
	if self != nil && self.Traits.PrefersGroomed {
		groomWeight *= 4
	}

	bestScore := float32(-1e9)
	for _, sd := range samples {
		score := float32(progressBonus) * float32(math.Cos(float64(sd.ang)))
		score -= float32(treePenalty) * sd.totalDensity
		score -= float32(boundaryPenalty) * float32(sd.boundaryHits)
		score += groomWeight * sd.totalGrooming
		score -= float32(treePenalty) * float32(groomEdgePenalty) * float32(sd.groomEdgeCrossings)
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
		score += (rng.Global().Float32() - 0.5) * 0.001
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

// effectiveFriction returns the (base, edge) kinetic friction coefficients
// at world position (wx, wz), modulated by snow kind, grooming, and moguls.
// (muBase, muEdge) are the groomed-corduroy baseline; each modifier shifts
// them toward the feel of the actual surface.
//
// Out-of-bounds positions return groomed PackedPowder defaults.
func effectiveFriction(t *world.Terrain, wx, wz float32) (base, edge float64) {
	depth, grooming, kind, mogul := t.SnowAt(wx, wz)
	base = muBase
	edge = muEdge

	// Grooming: lowers base (glide), raises edge (clean carve).
	base *= 1 - 0.30*float64(grooming)
	edge *= 1 + 0.20*float64(grooming)

	// Kind-based friction: each snow type has its own speed and grip character.
	base *= float64(world.KindBaseMult(kind))
	edge *= float64(world.KindEdgeMult(kind))

	// Powder: depth-dependent extra drag — shallow powder is floaty but
	// manageable; deep powder is slow and tiring. Gated on KindPowder so
	// Cement/Slush drag comes purely from their kind multipliers.
	if kind == world.KindPowder && depth > 0.5 {
		depthFactor := float64(clamp32(depth/2.5, 0, 1))
		base *= 1 + 0.80*depthFactor
		edge *= 1 - 0.50*depthFactor
	}

	// Moguls: each bump bleeds energy (uniform decel; geometric moguls not modelled).
	base *= 1 + 0.60*float64(mogul)
	edge *= 1 + 0.10*float64(mogul)

	// Numerical floors so multiplicative chain can't go negative.
	if base < 0.005 {
		base = 0.005
	}
	if edge < 0.02 {
		edge = 0.02
	}
	return base, edge
}

func apply(t *world.Terrain, a *world.Guest, dec Decision, perc Perception, dt float64) {
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

	// Snow-state-modulated friction. Snow type at the agent's current
	// position shifts both base (glide) and edge (carve) coefficients —
	// real consequences:
	//   - groomed corduroy: low base, high edge → fast and predictable carving
	//   - hard ice:         very low base, very LOW edge → fast, no edge hold,
	//                       so brake angle does little and skiers run away on
	//                       steep ice; matches the "skating out" experience
	//   - powder:           moderate-high base (sinking), low edge → slow,
	//                       skidded turns rather than carves
	//   - mogul field:      high base (banging into bumps), moderate edge →
	//                       fast scrub on the troughs but speed bleed overall
	//   - dense packed:     low-medium base, high edge → like groomed but
	//                       slightly grippier
	muB, muE := effectiveFriction(t, a.Pos[0], a.Pos[2])
	speed := float64(a.Speed)
	accel := gravity*sinTheta*cosOff -
		muB*gravity*cosTheta -
		muE*gravity*cosTheta*sinOffAbs -
		kDrag*speed*speed -
		float64(dec.Scrub)
	a.Speed = float32(math.Max(0, speed+accel*dt))
	if a.Speed < skiWalkSpeed {
		a.Speed = skiWalkSpeed
	}

	step := a.Speed * float32(dt)
	a.Pos[0] += hx * step
	a.Pos[2] += hz * step
	a.Pos[1] = t.InterpolatedSurfaceElevationAt(a.Pos[0], a.Pos[2])
}

// =============================================================================
// SECTION 7 — Stress / balance
// =============================================================================

// stressDelta returns the rate of change of Balance per second. Negative
// drains toward a fall, positive recovers. Clamped to keep numerical
// excursions bounded.
func stressDelta(traits ai.GuestTraits, perc Perception, dec Decision) float32 {
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
func ComputeSteeringDebug(w *world.World, a *world.Guest, target mgl32.Vec3) SteeringDebug {
	t := w.Terrain
	clone := *a
	perc := perceive(t, &clone, target)
	towers := collectTowerXZs(w)
	// One-off spatial grid for the debug pass — debug overlay only
	// fires for the followed agent so the construction cost is
	// negligible compared with the per-frame Simulation grid.
	grid := newSpatialGrid(float32(w.Terrain.Width)*CellSize, float32(w.Terrain.Height)*CellSize)
	grid.rebuild(w.OnMountain)
	dec := decide(w, towers, grid, &clone, perc, 0)

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
		out.Probes[i].Density = hazardDensityAt(t, towers, grid, a.ID, a.Pos[0]+d[0]*horizon, a.Pos[2]+d[1]*horizon)
	}
	return out
}

// =============================================================================
// SECTION 11 — Recorder hook
// =============================================================================

func recordFrame(s *Simulation, a *world.Guest, target mgl32.Vec3, dist float32, perc Perception, dec Decision) {
	if s.Recorder == nil {
		return
	}
	if id := s.Recorder.GuestID(); id != 0 && id != a.ID {
		return
	}
	s.Recorder.Record(RecorderFrame{
		SimTime:         s.SimTime,
		GuestID:         a.ID,
		Activity:        world.Activity(s.World, a),
		Pos:             a.Pos,
		Heading:         a.Heading,
		Target:          target,
		Dist:            dist,
		Speed:           a.Speed,
		PlanStep:        goap.PlanActionLabel(a.Plan.Head(), s.World),
		GoalName:        a.Plan.GoalName,
		PathLen:         len(a.Path),
		PathIdx:         a.PathIdx,
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
		TacticalOffset:  dec.TacticalOffset,
	})
}
