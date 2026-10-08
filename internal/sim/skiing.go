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
// S-turns emerge from the controller, not from a programmed oscillator: near
// comfort speed, we command a heading rotated off the fall line by an angle
// that grows with overspeed; this engages the existing
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
	kDrag   = 0.01 // air drag per unit mass

	// Motion floors / arrival
	skiWalkSpeed     = 2.0 // m/s; minimum forward motion (skating/poling)
	skiCreepSpeed    = 1.0 // m/s; edging past a hazard (Decision.Stop), slower than a trunk hit
	ArrivalRadius    = 6.0 // m; switch to direct-pointing inside this
	ArrivalThreshold = 2.0 // m; sim considers the agent "there"

	// Turning, linked turns and swerves: ski_turns.go.

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
	// tasteSteerWeight weighs Σ taste × feature (cellFeatures) along a
	// candidate path: guests drift toward the snow they like (corduroy,
	// powder, bumps, glades) and away from what they don't, outvoted by
	// hazards when present. 2.5 keeps a Cruiser on corduroy about as hard
	// as the old fixed grooming bonus did.
	tasteSteerWeight = 2.5
	// mogulSteerAversion × (1 − taste) is taken off every guest's moguls
	// taste when steering: everyone but a bump skier heads for the
	// smoother side of a mogul run, unless that side is icy or treed (both
	// disliked too), while a guest who loves moguls outright keeps the
	// full pull.
	mogulSteerAversion = 0.6
	// gladeCoverRelief is how much a love of trees lowers the tree-stand
	// part of the hazard: a trees taste of 1 avoids stands at
	// 1 − gladeCoverRelief strength. Individual trunks are avoided by
	// everyone.
	gladeCoverRelief = 0.8
	// groomEdgePenalty is added to a candidate's totalDensity (→ treePenalty
	// multiplier) each time the sample transitions from groomed to ungroomed
	// terrain. Only applied when self.Traits.Tastes.PrefersGroomed() is true. This
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
	towerHazardRadius = 5.0                     // lift towers are 0.6–0.9 m poles; the radius gives ~4 m of carving room
	skierHazardRadius = 2.5                     // ski-width is ~0.6 m; the radius is "don't ski into someone's blind spot"
	trunkHazardRadius = world.TrunkHazardRadius // 3 m; a trunk is ~0.4 m across; the radius steers a skier around one specific tree

	// Tree collisions: a skier whose position comes within trunkHitRadius
	// of a trunk while moving toward it faster than trunkHitMinSpeed hits
	// it and falls. Injury chance scales with speed, reaching
	// trunkInjuryChanceMax at injuryMaxSpeed.
	trunkHitRadius       = float32(0.6)
	trunkHitMinSpeed     = float32(2.0)
	trunkInjuryChanceMax = float32(0.15)
	trunkSeriousShare    = float32(0.6) // hitting a tree is the classic serious injury

	// Fall-line attenuation. Identical to prior model — gentle terrain has
	// noisy gradients, so we ignore the fall direction below flatSlopeL.
	flatSlopeL  = 0.05
	steepSlopeL = 0.20

	// Balance / fall
	fallStartBalance = 0.7

	// Injury on a fall. Chance = (speedFactor + slopeFactor) / 2 *
	// fallInjuryChanceMax: 0.6% at max speed (20 m/s) on max slope (40°),
	// about 0.15% for a gentle tumble. Real resorts see roughly two to
	// four injuries per thousand skier days with a couple of falls each,
	// and most injured skiers get themselves down: fallSeriousShare of
	// injuries need patrol, the rest go home on their own.
	injuryMaxSpeed      = float32(20.0)
	injuryMaxSlope      = float32(40 * math.Pi / 180)
	fallInjuryChanceMax = float32(0.006)
	fallSeriousShare    = float32(0.3)
	// injuryWaitTime is how long a seriously hurt guest waits for patrol
	// before giving up: ten minutes of movement (600 sim seconds, 40
	// minutes on the clock, which runs 4× faster; see
	// world.SimSecondsPerHour). Long enough for a patroller to walk to a
	// lift, ride it, and ski down, so abandonment is rare.
	injuryWaitTime = float32(600.0)

	// Tree underfoot signal (display-only — controller doesn't branch on it)
	inTreesThreshold = 0.3

	// Snow wear from skier traffic.
	//   groomingWearRate: Grooming decays at this rate (1/s). ~100 passes wipe
	//     fresh corduroy; a 5 m cell at 10 m/s sees ~0.5 s per pass.
	//   trafficRate: SkierTraffic units accumulated per second while skiing.
	//   trafficThresh*: SkierTraffic thresholds that trigger a kind transition.
	//   mogulFormRate: mogul growth per second of a skier over a metre of
	//   snow, at full turning, slope, and softness (growMoguls).
	groomingWearRate      = 0.005
	trafficRate           = 1.0         // units/s; ~0.5 units per pass at 10 m/s
	trafficThreshPowder   = float32(40) // ~40 passes
	trafficThreshWindSlab = float32(60)
	trafficThreshCrust    = float32(20) // crust shatters quickly
	mogulFormRate         = 0.03
	// mogulMinSnowSWE is the snow, in metres of water, needed for moguls:
	// about 30 cm of settled snow. Gated on water rather than visible
	// depth, which the snowpack's settling understates.
	mogulMinSnowSWE = float32(0.1)

	// patienceGainPerSecSkiing is patience restored per sim-second of
	// active downhill skiing: full in about 5.6 clock hours. Offset
	// against the drain from queuing — a guest who skis freely without
	// long waits stays patient all day.
	patienceGainPerSecSkiing = 1.0 / (5.56 * world.SimSecondsPerHour)

	// Energy drain rates. Normal skiing drains in ~2 hours of continuous
	// skiing (real time, sim seconds: about 8 clock hours). Falls cause a
	// large one-shot hit. Ungroomed-snow penalties are applied by
	// energyDrainRate based on skill tier × snow kind.
	energyDrainPerSecSkiing = 1.0 / 7200.0
	energyFallDrain         = 0.30

	// Each fall costs a guest fallPatienceDrain of patience. A guest
	// down for the fallGiveUpCount-th time on one descent stops trying:
	// skis off, they wait for patrol like an injured guest and walk home
	// if none comes.
	fallPatienceDrain = 0.10
	fallGiveUpCount   = 4

	// Hunger drains at a fixed rate regardless of terrain: full to empty
	// in five clock hours of skiing, so a guest arriving fed gets hungry
	// around lunchtime.
	hungerDrainPerSec = 1.0 / (5 * world.SimSecondsPerHour)

	// Thirst base rate (five clock hours to empty) scaled by altitude and exertion.
	thirstDrainPerSec      = 1.0 / (5 * world.SimSecondsPerHour)
	thirstAltitudePerMetre = float32(0.0005) // +50% at 1000 m, ×2 at 2000 m

	// criticalStatThreshold mirrors goap.restTriggerThreshold: below it
	// the hungry, thirsty, impatient, and tired conditions start.
	criticalStatThreshold = float32(0.15)
	// needClearThreshold is where those conditions clear again: the 0.25
	// at which a guest starts looking for the service.
	needClearThreshold = float32(0.25)
	// exhaustedThreshold mirrors GoHome's 0.05 cut-off.
	exhaustedThreshold = float32(0.05)

	// scoreStart is every guest's satisfaction on arrival: the ledger of
	// their day starts here.
	scoreStart = float32(0.5)
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
	MogulSize     float32 // underfoot, from the 1 m mogul map
	Skid          float32 // last tick's skid, 0–1 of the pivot rate (apply)
}

// Decision is what the controller emits each tick. Consumed by apply().
type Decision struct {
	DesiredHeading float32
	Scrub          float32 // m/s² active deceleration (beyond passive friction)
	// Stop: no way past a hazard ahead, so the guest slows to
	// skiCreepSpeed and edges on rather than skating at skiWalkSpeed.
	Stop bool
	// TurnRate caps how fast apply turns the heading this tick: the carve
	// rate, or the pivot rate for a swerve. CarveRate is the rate beyond
	// which turning skids.
	TurnRate, CarveRate float32

	// Diagnostics — propagated to Sense / RecorderFrame.
	AxisHeading    float32
	TacticalOffset float32
	Brake          float32 // commanded turn amplitude (rad)
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
	seed := rng.Global().Uint64()
	if s.skiBatch != nil {
		// Steering for every skier is decided at once, across cores
		// (decideSkiers); the rest of the tick runs after, in order.
		*s.skiBatch = append(*s.skiBatch, skiJob{a: a, target: target, dist: dist, perc: perc, seed: seed})
		return false
	}
	r := stepRand(seed)
	dec := decide(s.World, s.towersScratch, s.spatial, a, perc, float32(dt), &s.steerScratch, &r)
	return s.applySkier(a, target, dist, perc, dec, dt)
}

// applySkier runs the rest of a skier's tick once their steering is
// decided: needs, falls, movement, and the snow underfoot.
func (s *Simulation) applySkier(a *world.Guest, target mgl32.Vec3, dist float32, perc Perception, dec Decision, dt float64) bool {
	if dec.TurnSide != a.TurnSide {
		a.TurnDwell = 0
	} else {
		a.TurnDwell += float32(dt)
	}
	a.TurnSide = dec.TurnSide
	a.LastTactical = dec.TacticalOffset
	a.Sense = senseFrom(perc, dec)

	// The cell under the guest: snow underfoot against their tastes, and
	// the run record.
	cx := int(a.Pos[0] / CellSize)
	cz := int(a.Pos[2] / CellSize)
	var cell *world.Cell
	var grooming float32
	var surfKind world.SnowKind
	if s.World.Terrain.InBounds(cx, cz) {
		cell = &s.World.Terrain.Cells[cx][cz]
		grooming = cell.Grooming
		if top := cell.TopLayer(); top != nil {
			surfKind = top.Kind
		}
	}
	const groomingThreshold = 0.50
	onGroomed := grooming >= groomingThreshold
	dislike, match, powder := s.tickUnderfoot(a, cell, perc.SlopeAngle, float32(dt))
	s.recordRun(a, cell, cx, cz, grooming, perc.SlopeAngle, match, powder, float32(dt))

	a.SkiedThisTick = true

	// Patience gain from active skiing.
	a.Patience += float32(dt * patienceGainPerSecSkiing)
	if a.Patience > 1 {
		a.Patience = 1
	}

	// Energy drain from skiing. Rate depends on skill tier × snow kind;
	// see energyDrainRate for the full table.
	a.Energy -= energyDrainRate(a.Traits.Skill, surfKind, onGroomed) * (1 + underfootTiring*dislike) * float32(dt)
	if a.Energy < 0 {
		a.Energy = 0
	}

	// Hunger: fixed-rate drain regardless of terrain.
	a.Hunger -= hungerDrainPerSec * float32(dt)
	if a.Hunger < 0 {
		a.Hunger = 0
	}

	// Thirst: altitude × exertion scaled drain.
	altFactor := 1 + (s.World.BaseAltitude+a.Pos[1])*thirstAltitudePerMetre
	a.Thirst -= thirstDrainPerSec * altFactor * thirstExertionMultiplier(a.Traits.Skill, surfKind, onGroomed) * float32(dt)
	if a.Thirst < 0 {
		a.Thirst = 0
	}

	// Avalanche hit — if active debris is flowing through this cell, force a
	// high-probability injury fall before the normal balance update.
	{
		xi := int(a.Pos[0] / CellSize)
		zi := int(a.Pos[2] / CellSize)
		if s.World.Terrain.InBounds(xi, zi) && s.World.Terrain.Cells[xi][zi].AvySnow > avyMinSnow && !a.Fallen {
			s.applyEvent(a, ai.ThoughtCaughtInAvalanche)
			s.knockDown(a, world.FallForward)
			const avyInjuryChance = float32(0.70)
			if rng.Global().Float32() < avyInjuryChance {
				s.injure(a, true)
			} else {
				s.applyTrailEvent(a, ai.ThoughtFell)
			}
			recordFrame(s, a, target, dist, perc, dec)
			return false
		}
	}

	if !a.Fallen && hitsTrunk(s.World.Terrain, a) {
		s.treeHit(a)
		recordFrame(s, a, target, dist, perc, dec)
		return false
	}

	// Balance + fall.
	a.Balance += stressDelta(a.Traits, perc, dec) * float32(dt)
	if a.Balance > 1 {
		a.Balance = 1
	}
	if a.Balance <= 0 {
		speedFactor := clamp32(a.Speed/injuryMaxSpeed, 0, 1)
		slopeFactor := clamp32(perc.SlopeAngle/injuryMaxSlope, 0, 1)
		injuryChance := (speedFactor + slopeFactor) / 2 * fallInjuryChanceMax
		s.knockDown(a, fallDirection(a.Traits, perc, dec))
		if rng.Global().Float32() < injuryChance {
			s.injure(a, rng.Global().Float32() < fallSeriousShare)
		} else {
			s.applyTrailEvent(a, ai.ThoughtFell)
		}
		recordFrame(s, a, target, dist, perc, dec)
		return false
	}

	prevPos := a.Pos
	apply(s.World.Terrain, a, dec, perc, dt)
	wearSnowUnderfoot(s.World.Terrain, a, dt)
	splatSkierTrack(s.World.Terrain, a, prevPos, world.TrackClock(s.SimTime))
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

func splatSkierTrack(t *world.Terrain, a *world.Guest, prevPos mgl32.Vec3, now uint16) {
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
		a.Pos[0], a.Pos[2], intensity, now,
	)
	a.LastTrackPos = a.Pos
}

// wearSnowUnderfoot mutates the cell beneath the agent to model skier
// traffic on the snow surface. Grooming decays (skis cut up the corduroy),
// Packed rises (boots and edges compact the column). SWE — held in
// Cell.SnowAccumulation — is conserved by construction; the visible snow
// depth therefore drops automatically as packing rises (depth =
// accumulation / density(packed)), matching how real snow behaves under
// traffic and groomer treads. Moguls grow along the skier's line
// (growMoguls).
func wearSnowUnderfoot(t *world.Terrain, a *world.Guest, dt float64) {
	pos := a.Pos
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
	growMoguls(t, a, xi, zi, dt)
	if dirty {
		t.MarkSnowDirty(xi, zi)
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
		if b.FootprintContains(x, z, margin) {
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
	} else if !walk && !a.SkisOn && !a.OnFoot {
		// Don't put skis back on when close to the destination — the guest
		// is about to arrive and shouldn't re-equip for the last few metres.
		// Further away (e.g. at the lift top after a heatwave stripped the
		// base snow) they should still ski down rather than walking the whole
		// mountain.
		if target, ok := liveTarget(s.World, a); ok {
			if d, leaving := leavingDistance(a, target); leaving && d <= skisOnNearDest {
				return
			}
		}
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

	atCell := t.TreeCoverAt(pos[0], pos[2])
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
		MogulSize:     t.MogulSizeAt(pos[0], pos[2]),
		Skid:          a.Skid,
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
//	turn     = TurnSide × amplitude                 (speed control → S-turns)
//	desired  = axis + tactical + turn
//
// then a swerve overrides desired when a trunk, tower or skier is on the
// line within swerveLookSec (ski_turns.go).
//
// The turn offset is what produces S-turns: near target speed on a slope
// the guest turns across the fall line by an amplitude that grows with
// overspeed; edge friction scrubs speed; when heading reaches the arc edge
// on the committed side (after turnDwell), TurnSide flips and they carve
// back. Linked turns carve at the guest's carve rate; swerves pivot faster
// and skid (turnRates).
//
// decide only reads the world and a, so many skiers can decide at once
// (decideSkiers); r is the decision's own random stream for the same
// reason.
func decide(w *world.World, towers []mgl32.Vec2, grid *spatialGrid, a *world.Guest, perc Perception, dt float32, sc *steerScratch, r *stepRand) Decision {
	if sc == nil {
		sc = new(steerScratch)
	}
	axisHeading := composeAxis(perc)

	tactical, _, probeC, probeR, probeL := sampleTactical(w, towers, grid, a, perc, axisHeading, a.LastTactical, r, &sc.near)

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
	targetSpeed = min(targetSpeed, treeSpeedAhead(w.Terrain, a, perc))
	targetSpeed *= mogulSpeedScale(a.Traits, perc.MogulSize)
	overspeed := float32(0)
	if perc.Speed > targetSpeed && targetSpeed > 0.01 {
		overspeed = (perc.Speed - targetSpeed) / targetSpeed
	}
	// Linked turns on any real slope, wider the faster the guest is going
	// against their target.
	carveRate, pivotRate := turnRates(a.Traits.Skill, perc.Speed)
	side := a.TurnSide
	amp := turnAmplitude(a.Traits.Skill, perc, targetSpeed)

	// Picking a way past trees is still done with linked turns: the
	// tactical offset moves the line they turn about, and the turns are
	// what keep their speed down among the trunks. A turn ends once the
	// heading is within 15% of the swing of its edge.
	edge := func(side int8) float32 {
		return clamp32(tactical+float32(side)*amp, -turnOffMax, turnOffMax)
	}
	deviation := wrapAngle(perc.Heading - axisHeading)
	dwellSatisfied := a.TurnDwell >= turnDwell(a.Traits.Skill)
	switch {
	case amp == 0:
		side = 0
	case side == 0:
		side = pickInitialSide(perc, wrapAngle(deviation-tactical), r)
	case side > 0 && deviation > edge(+1)-0.15*amp && dwellSatisfied:
		side = -1
	case side < 0 && deviation < edge(-1)+0.15*amp && dwellSatisfied:
		side = +1
	}

	desired := wrapAngle(axisHeading + edge(side))
	if side == 0 {
		desired = wrapAngle(axisHeading + tactical)
	}

	// Overspeed skids the turns round faster: a speed check.
	turnRate := carveRate + (pivotRate-carveRate)*clamp32(overspeed/skidTurnOver, 0, 1)

	// Active scrub: a skidded speed check when way overspeed.
	var scrub float32
	if overspeed > 0.6 {
		scrub = min(4.0*(overspeed-0.6), 6.0)
	}

	// Anything on the line within the next second and a half: swerve
	// round it at the pivot rate, or stop hard when there's no way past.
	stopping, swerving := false, false
	if h, urgent, stop := swerve(w, towers, grid, a, desired, turnRate, pivotRate, &sc.hazards); urgent {
		desired = h
		turnRate = pivotRate
		swerving = true
		if stop {
			scrub = max(scrub, float32(swerveStopScrub)*(1+clamp32(a.Traits.Skill, 0, 1)))
			stopping = true
		}
	}
	mode := "straight"
	if amp > 0 {
		mode = "carve"
	}
	if scrub > 0 {
		mode = "brake"
	}
	if turnRate > carveRate {
		mode = "skid"
	}
	if swerving {
		mode = "swerve"
	}

	return Decision{
		DesiredHeading: desired,
		Scrub:          scrub,
		Stop:           stopping,
		AxisHeading:    axisHeading,
		TacticalOffset: tactical,
		Brake:          amp,
		TurnRate:       turnRate,
		CarveRate:      carveRate,
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
func pickInitialSide(perc Perception, deviation float32, r *stepRand) int8 {
	// If heading already favours a side, commit to that — avoids an ugly
	// 180° flip in the first tick of the carve.
	if deviation > 0.05 {
		return +1
	}
	if deviation < -0.05 {
		return -1
	}
	if r.Float32() < 0.5 {
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
//   - terrain tree cover (Cell.TreeCover) × coverScale, so whole stands are
//     avoided, less by guests who love glades (standCoverScale)
//   - falloff inside trunkHazardRadius of any trunk, read from the
//     terrain's cached trunk field (world.TrunkHazardAt)
//   - falloff inside towerHazardRadius of any lift tower
//   - falloff inside skierHazardRadius of any other skier
//
// Combination is max(): one big hazard dominates. Cover keeps skiers out
// of stands; the trunk, tower, and skier terms route them around single
// obstacles the cover is too coarse to see. Towers and skiers come from
// near, gathered once per decision around the whole fan (steerNear), so
// each of the fan's sample points only checks the few that could reach it.
func hazardDensityAt(t *world.Terrain, near *steerNear, x, z, coverScale float32) float32 {
	d := t.TreeCoverAt(x, z) * coverScale
	if d < 1 {
		d = max(d, t.TrunkHazardAt(x, z))
	}
	if near == nil {
		return d
	}
	const towerR2 = towerHazardRadius * towerHazardRadius
	for _, p := range near.towers {
		dx, dz := p[0]-x, p[1]-z
		if r2 := dx*dx + dz*dz; r2 < towerR2 {
			d = max(d, 1-r2/towerR2)
		}
	}
	const skierR2 = skierHazardRadius * skierHazardRadius
	for _, p := range near.skiers {
		dx, dz := p[0]-x, p[1]-z
		if r2 := dx*dx + dz*dz; r2 < skierR2 {
			d = max(d, 1-r2/skierR2)
		}
	}
	return d
}

// steerNear is what a skier's fan of sample points could run into this
// step: the lift towers and other skiers within reach of their position.
type steerNear struct {
	towers []mgl32.Vec2
	skiers []mgl32.Vec2
}

// gather fills n with the towers and skiers (other than selfID) within
// reach of (x, z), plus each kind's hazard radius.
func (n *steerNear) gather(towers []mgl32.Vec2, grid *spatialGrid, selfID uint64, x, z, reach float32) {
	n.towers, n.skiers = n.towers[:0], n.skiers[:0]
	tr := reach + towerHazardRadius
	for _, p := range towers {
		if dx, dz := p[0]-x, p[1]-z; dx*dx+dz*dz <= tr*tr {
			n.towers = append(n.towers, p)
		}
	}
	if grid == nil {
		return
	}
	sr := reach + skierHazardRadius
	grid.forEachWithin(x, z, sr, func(o *world.Guest) {
		if o.ID == selfID {
			return
		}
		if dx, dz := o.Pos[0]-x, o.Pos[2]-z; dx*dx+dz*dz <= sr*sr {
			n.skiers = append(n.skiers, mgl32.Vec2{o.Pos[0], o.Pos[2]})
		}
	})
}

// standCoverScale is how hard a guest avoids tree stands: 1 for anyone
// who doesn't love trees, down to 1 − gladeCoverRelief for a trees taste
// of 1, so glade lovers go into the woods.
func standCoverScale(t ai.Tastes) float32 {
	return 1 - gladeCoverRelief*max(0, t[ai.TasteTrees])
}

// sampleTactical scores a fan of candidate forward arcs and returns the
// best lateral offset (relative to axis), a flag indicating whether an
// obstacle is in view, and centre/right/left density readings for the
// HUD.
//
// Score = progressBonus × cos(offset)
//
//   - tasteSteerWeight × Σ taste × feature (point along projected line)
//     − Σ treePenalty × density(point along projected line)
//     − boundaryPenalty × (off-map sample count)
//   - sideCommitBonus × sign(prevTactical) × sign(offset)   [conditional]
//
// The progress term keeps the skier on axis when nothing obstructs. The
// side-commit term breaks the symmetry of obstacles centred on axis so
// the skier doesn't flip-flop tick-to-tick when both lateral options
// score equally — but it's gated on actually seeing an obstacle in the
// fan. Without that gate the commit bonus would slowly drift the skier
// off-axis even on a clear slope, since prevTactical is self-perpetuating.
func sampleTactical(w *world.World, towers []mgl32.Vec2, grid *spatialGrid, self *world.Guest, perc Perception, axisHeading, prevTactical float32, r *stepRand, near *steerNear) (offset float32, obstacleSeen bool, probeC, probeR, probeL float32) {
	t := w.Terrain
	horizon := perc.Speed * float32(sampleHorizonSec)
	if horizon < float32(sampleMinDist) {
		horizon = float32(sampleMinDist)
	}
	if horizon > float32(sampleMaxDist) {
		horizon = float32(sampleMaxDist)
	}

	// The towers and other skiers any sample point could reach: the
	// farthest sample is horizon ahead and corridorHalfWidth aside. Self
	// is left out so the skier doesn't avoid itself.
	var selfID uint64
	if self != nil {
		selfID = self.ID
	}
	near.gather(towers, grid, selfID, perc.Pos[0], perc.Pos[2], horizon+float32(corridorHalfWidth))

	// The guest's tastes steer the line (tasteSteerWeight), and their
	// love of trees eases how hard they avoid stands.
	var tastes ai.Tastes
	coverScale := float32(1)
	if self != nil {
		tastes = self.Traits.Tastes
		coverScale = standCoverScale(tastes)
	}
	// Moguls are hard work: only a guest who really likes them steers in
	// (mogulSteerAversion, fading out as the taste nears 1).
	tastes[ai.TasteMoguls] -= mogulSteerAversion * (1 - tastes[ai.TasteMoguls])

	// Current-cell grooming used as the starting point for groom-edge
	// crossing detection. Only relevant when self.Traits.Tastes.PrefersGroomed().
	var startGrooming float32
	prefersGroomed := self != nil && self.Traits.Tastes.PrefersGroomed()
	if prefersGroomed && t.InBoundsWorld(perc.Pos[0], perc.Pos[2]) {
		_, startGrooming, _, _ = t.SnowAt(perc.Pos[0], perc.Pos[2])
	}

	// Pass 1: integrate density, boundary hits, and grooming along each
	// candidate. Grooming reads the centre point only — corridor sampling
	// would double-count adjacent cells.
	type sampleData struct {
		ang                float32
		totalDensity       float32
		totalTaste         float32
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

		var totalDensity, totalTaste float32
		var boundaryHits, groomEdgeCrossings int
		prevGrooming := startGrooming
		for sIdx := 1; sIdx <= sampleSegments; sIdx++ {
			d := horizon * float32(sIdx) / float32(sampleSegments)
			x := perc.Pos[0] + hx*d
			z := perc.Pos[2] + hz*d
			if !t.IsAccessibleWorld(x, z) {
				boundaryHits++
				continue
			}
			// Worst-of-three: centre + ±corridorHalfWidth perpendicular.
			// Treats the candidate path as a corridor, so the skier
			// avoids brushing the patch edge instead of grazing it.
			// Hazard includes trees, lift towers, and other skiers.
			density := hazardDensityAt(t, near, x, z, coverScale)
			if dl := hazardDensityAt(t, near, x-rx*float32(corridorHalfWidth), z-rz*float32(corridorHalfWidth), coverScale); dl > density {
				density = dl
			}
			if dr := hazardDensityAt(t, near, x+rx*float32(corridorHalfWidth), z+rz*float32(corridorHalfWidth), coverScale); dr > density {
				density = dr
			}
			totalDensity += density
			_, grooming, _, _ := t.SnowAt(x, z)
			if prefersGroomed && prevGrooming > 0.5 && grooming < 0.5 {
				groomEdgeCrossings++
			}
			prevGrooming = grooming
			if cell := t.CellAtWorld(x, z); cell != nil {
				f := cellFeatures(cell, cellSlope(cell))
				// Moguls at 1 m, so the less skied-out edge of a run
				// shows up.
				f[ai.TasteMoguls] = t.MogulSizeAt(x, z)
				totalTaste += tasteMatch(tastes, f)
			}
		}
		samples[i] = sampleData{ang, totalDensity, totalTaste, boundaryHits, groomEdgeCrossings}
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
		score += float32(tasteSteerWeight) * sd.totalTaste
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
		score += (r.Float32() - 0.5) * 0.001
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

func apply(t *world.Terrain, a *world.Guest, dec Decision, perc Perception, dt float64) {
	before := a.Heading
	a.Heading = rotateToward(a.Heading, dec.DesiredHeading, dec.TurnRate, dt)
	// Turning faster than the carve rate skids the skis round.
	skid := 0.0
	if dt > 0 && dec.TurnRate > dec.CarveRate {
		rate := math.Abs(float64(wrapAngle(a.Heading-before))) / dt
		skid = math.Max(0, rate-float64(dec.CarveRate)) / float64(dec.TurnRate)
	}
	a.Skid = float32(skid)

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
	xi := int(a.Pos[0] / world.CellSize)
	zi := int(a.Pos[2] / world.CellSize)
	if xi < 0 {
		xi = 0
	} else if xi >= t.Width {
		xi = t.Width - 1
	}
	if zi < 0 {
		zi = 0
	} else if zi >= t.Height {
		zi = t.Height - 1
	}
	muB, muE := t.Cells[xi][zi].EffectiveFriction()
	speed := float64(a.Speed)
	accel := gravity*sinTheta*cosOff -
		muB*gravity*cosTheta -
		muE*gravity*cosTheta*sinOffAbs -
		kDrag*speed*speed -
		float64(dec.Scrub) -
		skidDecel*skid
	a.Speed = float32(math.Max(0, speed+accel*dt))
	if floor := float32(skiWalkSpeed); dec.Stop {
		a.Speed = max(a.Speed, skiCreepSpeed)
	} else if a.Speed < floor {
		a.Speed = floor
	}

	step := a.Speed * float32(dt)
	a.Pos[0] += hx * step
	a.Pos[2] += hz * step
	a.Pos[1] = t.InterpolatedSurfaceElevationAt(a.Pos[0], a.Pos[2])
}

// hitsTrunk reports whether the guest is within trunkHitRadius of a trunk
// while closing on it at trunkHitMinSpeed or more: their speed toward
// it, not just their speed, so glancing past or pushing off doesn't
// count. The trunk they last hit doesn't count until they're clear of it
// (trunkClearDist), so a skier who fell against a tree can't keep
// hitting it while getting going again.
func hitsTrunk(t *world.Terrain, a *world.Guest) bool {
	if a.HasTrunk {
		dx, dz := a.Pos[0]-a.Trunk[0], a.Pos[2]-a.Trunk[1]
		if dx*dx+dz*dz > trunkClearDist*trunkClearDist {
			a.HasTrunk = false
		}
	}
	if a.Speed < trunkHitMinSpeed {
		return false
	}
	_, ok := trunkAhead(t, a)
	return ok
}

// trunkAhead is the nearest trunk within trunkHitRadius that the guest
// is closing on at trunkHitMinSpeed or more, other than the one they
// last hit.
func trunkAhead(t *world.Terrain, a *world.Guest) (world.Tree, bool) {
	hx := float32(math.Sin(float64(a.Heading)))
	hz := float32(math.Cos(float64(a.Heading)))
	var best world.Tree
	bestD := float32(math.Inf(1))
	t.ForEachTreeNear(a.Pos[0], a.Pos[2], trunkHitRadius, func(tr world.Tree, _, _ int) {
		if a.HasTrunk && tr.X == a.Trunk[0] && tr.Z == a.Trunk[1] {
			return
		}
		dx, dz := tr.X-a.Pos[0], tr.Z-a.Pos[2]
		d := float32(math.Hypot(float64(dx), float64(dz)))
		if d < 1e-3 {
			return
		}
		if a.Speed*(dx*hx+dz*hz)/d >= trunkHitMinSpeed && d < bestD {
			best, bestD = tr, d
		}
	})
	return best, bestD < float32(math.Inf(1))
}

// trunkClearDist is how far a guest must get from the trunk they hit
// before it can knock them down again, in metres.
const trunkClearDist = float32(3)

// getUpClearOfTrunk turns a guest who fell against a trunk to ski on
// past it: their heading goes to whichever side of the trunk is nearer
// their way, angled away from it, and they stand at least a metre from
// it.
func getUpClearOfTrunk(a *world.Guest) {
	if !a.HasTrunk {
		return
	}
	away := mgl32.Vec2{a.Pos[0] - a.Trunk[0], a.Pos[2] - a.Trunk[1]}
	if away.Len() < 1e-3 {
		away = mgl32.Vec2{-float32(math.Sin(float64(a.Heading))), -float32(math.Cos(float64(a.Heading)))}
	}
	away = away.Normalize()
	if d := (mgl32.Vec2{a.Pos[0] - a.Trunk[0], a.Pos[2] - a.Trunk[1]}).Len(); d < 1 {
		a.Pos[0] += away[0] * (1 - d)
		a.Pos[2] += away[1] * (1 - d)
	}
	// Past the trunk on the side nearer where they're going.
	want := mgl32.Vec2{a.Plan.Target[0] - a.Pos[0], a.Plan.Target[2] - a.Pos[2]}
	side := mgl32.Vec2{-away[1], away[0]}
	if want.Dot(side) < 0 {
		side = side.Mul(-1)
	}
	dir := side.Add(away.Mul(0.5)).Normalize()
	a.Heading = float32(math.Atan2(float64(dir[0]), float64(dir[1])))
}

// treeHit knocks the guest down after skiing into a trunk: a fall, and an
// injury with a chance that grows with speed.
func (s *Simulation) treeHit(a *world.Guest) {
	injuryChance := clamp32(a.Speed/injuryMaxSpeed, 0, 1) * trunkInjuryChanceMax
	if tr, ok := trunkAhead(s.World.Terrain, a); ok {
		a.Trunk, a.HasTrunk = [2]float32{tr.X, tr.Z}, true
	}
	s.knockDown(a, world.FallThrown)
	s.applyEvent(a, ai.ThoughtHitTree)
	if rng.Global().Float32() < injuryChance {
		s.injure(a, rng.Global().Float32() < trunkSeriousShare)
	}
}

// injure hurts a guest who has just fallen. A serious injury leaves them
// where they lie waiting for patrol (up to injuryWaitTime); a minor one
// lets them get up after the usual fall and head home on their own.
// Either costs satisfaction through its ai.Effects row.
func (s *Simulation) injure(a *world.Guest, serious bool) {
	a.Events = append(a.Events, ai.GuestEvent{Kind: ai.EventInjury, Time: s.SimTime})
	if !serious {
		a.HurtGoHome = true
		s.applyEvent(a, ai.ThoughtHurtGoingHome)
		return
	}
	a.Injured = true
	a.InjuryWaitTimer = injuryWaitTime
	s.applyTrailEvent(a, ai.ThoughtInjured)
}

// giveUpRun ends a descent the guest can't manage: down for the
// fallGiveUpCount-th time, they take their skis off and wait for patrol
// as an injured guest does (Stranded marks them as unhurt).
func (s *Simulation) giveUpRun(a *world.Guest) {
	a.Injured, a.Stranded = true, true
	a.InjuryWaitTimer = injuryWaitTime
	a.SkisOn = false
	a.Speed, a.TurnSide = 0, 0
	s.applyTrailEvent(a, ai.ThoughtGaveUp)
}

// applyTrailEvent applies an event naming the trail the guest is on,
// when they're on one.
func (s *Simulation) applyTrailEvent(a *world.Guest, kind ai.ThoughtKind) {
	if id := plannedTrail(a); id != 0 {
		s.applyEvent(a, kind, id)
		return
	}
	s.applyEvent(a, kind)
}

// plannedTrail is the trail the guest's plan has them skiing, 0 when it
// has them on none.
func plannedTrail(a *world.Guest) uint64 {
	step := a.Plan.Head()
	if step.Kind != ai.ActSkiTrail {
		return 0
	}
	if step.Via != 0 {
		return step.Via
	}
	return step.TrailID
}

// recordFall logs a guest going down: on the guest, and in the day's
// falls under the run they were skiing (their plan's, or the one
// underfoot) or the lift they were getting off.
func (s *Simulation) recordFall(a *world.Guest) {
	a.Events = append(a.Events, ai.GuestEvent{Kind: ai.EventFall, Time: s.SimTime})
	f := world.FallRecord{X: a.Pos[0], Z: a.Pos[2], LiftID: a.Unload.LiftID}
	if f.LiftID == 0 {
		f.TrailID = plannedTrail(a)
		if f.TrailID == 0 {
			if t := s.World.TrailAt(int(a.Pos[0]/CellSize), int(a.Pos[2]/CellSize)); t != nil {
				f.TrailID = t.ID
			}
		}
	}
	s.World.History.RecordFall(f)
}

// tickMood updates a guest's conditions and charges the active ones to
// their score, whatever they're doing: each condition's ai.Effects rate
// per clock hour, for as long as it holds. Snow-underfoot conditions end
// when the guest didn't ski since the last update.
func (s *Simulation) tickMood(a *world.Guest, dt float64) {
	s.tickNeedConditions(a)
	if !a.SkiedThisTick {
		for _, k := range underfootConditions {
			s.setCondition(a, k, false)
		}
	}
	a.SkiedThisTick = false
	if a.Conditions == 0 {
		return
	}
	var rate float32
	for k := ai.ThoughtKind(1); int(k) < ai.ThoughtKindCount; k++ {
		if a.Conditions.Has(k) {
			rate += ai.Effects[k].Satisfaction
		}
	}
	a.Satisfaction = clamp32(a.Satisfaction+rate*float32(dt)/world.SimSecondsPerHour, 0, 1)
}

// tickNeedConditions turns the need conditions on and off from the
// guest's stats. Each starts at its threshold and clears once the stat
// recovers past needClearThreshold, or for the budget, once it can pay.
func (s *Simulation) tickNeedConditions(a *world.Guest) {
	has := a.Conditions.Has
	s.setCondition(a, ai.ThoughtHungry, holds(has(ai.ThoughtHungry), a.Hunger, criticalStatThreshold, needClearThreshold))
	s.setCondition(a, ai.ThoughtThirsty, holds(has(ai.ThoughtThirsty), a.Thirst, criticalStatThreshold, needClearThreshold))
	s.setCondition(a, ai.ThoughtImpatient, holds(has(ai.ThoughtImpatient), a.Patience, criticalStatThreshold, needClearThreshold))
	combined := min(a.Patience, a.Energy)
	exhausted := holds(has(ai.ThoughtExhausted), combined, exhaustedThreshold, criticalStatThreshold)
	s.setCondition(a, ai.ThoughtExhausted, exhausted)
	s.setCondition(a, ai.ThoughtTired, !exhausted && holds(has(ai.ThoughtTired), a.Energy, criticalStatThreshold, needClearThreshold))
	s.setCondition(a, ai.ThoughtCold, holds(has(ai.ThoughtCold), 1-a.Chill, criticalStatThreshold, 0.4))
	cheap := cheapestLiftTicket(s.World)
	s.setCondition(a, ai.ThoughtTooExpensive, cheap > 0 && a.RemainingBudget < float32(cheap))
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
	if felt := feltSlope(perc); traits.ComfortSlope > 0 && felt > traits.ComfortSlope*1.2 {
		excess := felt/traits.ComfortSlope - 1.2
		d -= excess * 0.5
	}
	// Hard scrub costs balance — wedging under load is tiring.
	if dec.Scrub > 0 {
		d -= dec.Scrub * 0.02
	}
	if perc.AtCellDensity > inTreesThreshold {
		d -= (perc.AtCellDensity - inTreesThreshold) * 0.4
	}
	d -= mogulStress(traits, perc)
	// Skidding round a sharp swerve: hard work for the less skilled.
	d -= float32(skidBalanceCost) * perc.Skid * (1 - clamp32(traits.Skill, 0, 1))

	if d < -1 {
		d = -1
	}
	if d > 0.4 {
		d = 0.4
	}
	return d
}

// Moguls knock skiers about: balance drains by mogulBalanceCost a second
// on full moguls at mogulRefSpeed, in proportion to size and speed, and
// by (1 − skill)² of that, so skill absorbs it fast: a beginner on big
// moguls at 5 m/s goes down in about seven seconds, an intermediate at
// 8 m/s about breaks even with recovery (0.15 a second), and an expert
// barely feels them. A love of bumps eases it by up to mogulTasteRelief.
const (
	mogulBalanceCost = float32(0.6)
	mogulRefSpeed    = float32(6)
	mogulTasteRelief = float32(0.5)
)

// Among trees guests ski no faster than their skill lets them pick a
// line between trunks: treeSpeedBase plus treeSpeedSkill × skill in an
// open glade (about 7.5 m/s for an expert, 5.5 for an intermediate),
// half that in a tight stand. Glade lovers go in; they don't go in at
// full speed.
const (
	treeSpeedBase  = float32(3)
	treeSpeedSkill = float32(5)
	// Slowing for trees ahead (treeSpeedAhead): cover is read every
	// treeLookStep along the line out to treeLookDist, assuming the guest
	// can brake at treeBrakeDecel.
	treeLookStep   = float32(5)
	treeLookDist   = float32(40)
	treeBrakeDecel = float32(2)
)

// treeSpeedAhead is the most the guest will ski now so they can still
// slow to the tree cap of the cover along their line within
// treeLookDist, braking at treeBrakeDecel: they start slowing for a glade
// before they're in it.
func treeSpeedAhead(t *world.Terrain, a *world.Guest, perc Perception) float32 {
	limit := treeSpeedCap(a.Traits, perc.AtCellDensity)
	hx, hz := float32(math.Sin(float64(a.Heading))), float32(math.Cos(float64(a.Heading)))
	for d := float32(treeLookStep); d <= treeLookDist; d += treeLookStep {
		cover := t.TreeCoverAt(perc.Pos[0]+d*hx, perc.Pos[2]+d*hz)
		if cover < 0.05 {
			continue
		}
		c := treeSpeedCap(a.Traits, cover)
		limit = min(limit, float32(math.Sqrt(float64(c*c+2*treeBrakeDecel*d))))
	}
	return limit
}

// treeSpeedCap is the most a guest will ski with tree cover density
// around them (no cap below 0.05).
func treeSpeedCap(traits ai.GuestTraits, density float32) float32 {
	if density <= 0.05 {
		return float32(math.Inf(1))
	}
	c := (treeSpeedBase + treeSpeedSkill*clamp32(traits.Skill, 0, 1)) * (1 - 0.5*clamp32(density, 0, 1))
	return max(c, skiWalkSpeed)
}

// Guests back off in moguls: the speed they aim for drops by up to
// mogulSlowMax on full moguls, less with skill (an expert by
// 1 − mogulSlowSkill of it) and less again the more they love bumps.
const (
	mogulSlowMax   = float32(0.5)
	mogulSlowSkill = float32(0.6)
)

// mogulSpeedScale is what the moguls underfoot do to a guest's target
// speed.
func mogulSpeedScale(traits ai.GuestTraits, size float32) float32 {
	if size <= 0.01 {
		return 1
	}
	return 1 - mogulSlowMax*size*
		(1-mogulSlowSkill*clamp32(traits.Skill, 0, 1))*
		(1-clamp32(traits.Tastes[ai.TasteMoguls], 0, 1))
}

// mogulStress is how fast the moguls underfoot drain balance.
func mogulStress(traits ai.GuestTraits, perc Perception) float32 {
	if perc.MogulSize <= 0.01 || perc.Speed <= 0 {
		return 0
	}
	unskill := 1 - clamp32(traits.Skill, 0, 1)
	relief := unskill * unskill * (1 - mogulTasteRelief*max(traits.Tastes[ai.TasteMoguls], 0))
	return mogulBalanceCost * perc.MogulSize * perc.Speed / mogulRefSpeed * relief
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
	// A fixed stream per guest, so drawing the overlay doesn't consume
	// the game's random numbers.
	r := stepRand(a.ID)
	dec := decide(w, towers, grid, &clone, perc, 0, nil, &r)

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
	var near steerNear
	near.gather(towers, grid, a.ID, a.Pos[0], a.Pos[2], horizon)
	angles := [3]float64{0, float64(sampleAngleMax), -float64(sampleAngleMax)}
	for i, ang := range angles {
		c := float32(math.Cos(ang))
		s := float32(math.Sin(ang))
		d := mgl32.Vec2{c*hx + s*rx, c*hz + s*rz}
		out.Probes[i].Dir = d
		out.Probes[i].Density = hazardDensityAt(t, &near, a.Pos[0]+d[0]*horizon, a.Pos[2]+d[1]*horizon, standCoverScale(a.Traits.Tastes))
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

// cheapestLiftTicket returns the minimum per-ride fare across all lifts,
// or 0 when there are no lifts or any lift is free to ride (every cable
// lift, since the day ticket covers them).
func cheapestLiftTicket(w *world.World) int {
	min := 0
	for i, l := range w.Lifts {
		if fare := l.RideFare(); i == 0 || fare < min {
			min = fare
		}
	}
	return min
}

// Moguls form where skiers turn on steep, soft, deep snow: growth per
// second at the skier's spot is mogulFormRate × turning × slope × snow ×
// (1 − grooming), stamped into the mogul map along their line, up to a
// size that grows with the slope.
var (
	mogulTurnFull  = float32(math.Pi / 4)        // off the fall line by this much counts as full turning
	mogulSlopeFrom = float32(5 * math.Pi / 180)  // no moguls below this slope
	mogulSlopeTo   = float32(20 * math.Pi / 180) // full growth, and full-size moguls, from this slope
	// mogulGentleCap is the biggest moguls get on the gentlest slope that
	// grows them: a green skied for a week gets bumpy, not a bump run.
	// The cap rises with the slope to 1 at mogulSlopeTo.
	mogulGentleCap = float32(0.25)
	mogulIcySnow   = float32(0.3) // growth on a hard, icy surface
)

// growMoguls grows the moguls under a skier at cell (xi, zi): more the
// more their line crosses the fall line (turning), the steeper the slope,
// the softer the snow (given enough of it), and the less groomed the cell.
func growMoguls(t *world.Terrain, a *world.Guest, xi, zi int, dt float64) {
	c := &t.Cells[xi][zi]
	if c.TotalSWE() < mogulMinSnowSWE {
		return
	}
	fx, fz := t.FallLineAt(xi, zi)
	fl := float32(math.Hypot(float64(fx), float64(fz)))
	hx, hz := float32(math.Sin(float64(a.Heading))), float32(math.Cos(float64(a.Heading)))
	off := float32(math.Abs(float64(hx*fz-hz*fx))) / fl // sin of the angle off the fall line
	turn := clamp32(float32(math.Asin(float64(clamp32(off, 0, 1))))/mogulTurnFull, 0, 1)
	slope := float32(math.Atan(float64(c.Slope)))
	steep := clamp32((slope-mogulSlopeFrom)/(mogulSlopeTo-mogulSlopeFrom), 0, 1)
	snow := float32(1)
	if top := c.TopLayer(); top != nil {
		switch top.Kind {
		case world.KindBoilerplate, world.KindFrozenGranular, world.KindCrust:
			snow = mogulIcySnow
		}
	}
	grow := float32(mogulFormRate*dt) * turn * steep * snow * (1 - c.Grooming)
	t.StampMoguls(a.Pos[0], a.Pos[2], grow, mogulGentleCap+(1-mogulGentleCap)*steep)
}
