package sim

import (
	"math/rand"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// =============================================================================
// Demand system — global skier pool + resort rating
// =============================================================================
//
// The simulation holds a fixed-size pool of potential skiers partitioned
// by skill level (the "catchment"). Every demandPollInterval sim-seconds
// the system asks each group "do you visit right now?" via a Bernoulli
// draw against
//
//	P(visit) = clamp(ResortRating) × terrainMatch × clamp(1 − occupancy)
//
// On success the system draws one carload (SkiersPerCar agents) from the
// group's pool, spawns them at a uniform-random parking lot, and bumps
// that lot's CurrentCars by 1 (visible-population bookkeeping).
//
// On Depart the agent's session is scored from (Fun, Energy, falls vs
// runs) and EMA'd into ResortRating; the skier is returned to their
// group's pool. The pool itself only ever shrinks via spawns and grows
// via departs, so over the long run the catchment is conserved.
//
// Future hooks the shape leaves room for:
//   - per-group preferences / loyalty (currently only skill partitions)
//   - time-of-day / weather modulation of the poll interval and P(visit)
//   - per-lot draw weighting (queue length, distance to lifts)
//   - richer terrainMatch once trails are first-class

// SkiersPerCar matches the renderer's "one car ≈ four people" mental
// model. Departures decrement CurrentCars by 1/SkiersPerCar each so a
// full carload leaving the resort drops one car from the render pad.
const SkiersPerCar = 4

// demandPollInterval is the sim-time cadence of the demand poll. The
// time/weather system will eventually drive this instead of a fixed
// interval; for now it's an unconditional 30 s tick.
const demandPollInterval = 30.0

// ratingEMAλ controls how fast new departures move ResortRating. With
// λ = 1/70 the EMA half-life is ~50 departures — slow enough that one
// bad guest doesn't tank the score, fast enough that a player's edit
// shows up in arrivals within a session.
const ratingEMAλ = 1.0 / 70.0

// initialResortRating bootstraps the EMA — neutral, so an empty resort
// gets some arrivals on opening day but only at 50% of the headline
// rate.
const initialResortRating = 0.5

// Capacity formula constant. avgSessionSec is how long a typical skier
// occupies a lift seat across one cycle (queue + ride + descent). Tied
// to the L1 controller's Energy budget so capacity grows alongside the
// "skiers complete N descents before fatigue" calibration.
const avgSessionSec = 800.0

// Score equation weights. α + β + γ should sum to 1 so a perfect
// session lands at score=1 and a disastrous one at score=0.
const (
	scoreWeightFun    = 0.4
	scoreWeightEnergy = 0.3
	scoreWeightClean  = 0.3
)

// SkierGroup is one partition of the global pool. Currently keyed by
// skill level only.
type SkierGroup struct {
	Pool int
}

// DemandSystem owns the catchment and the resort rating. Lives on
// Simulation; only its maybePoll method runs on the tick path.
type DemandSystem struct {
	Groups       [3]SkierGroup // indexed by ai.SkillLevel (Beginner=0, ...)
	ResortRating float32
	LastPoll     float64
}

// NewDemandSystem seeds the catchment with a beginner-heavy
// distribution at thousand-skier scale and bootstraps the rating EMA.
func NewDemandSystem() *DemandSystem {
	return &DemandSystem{
		Groups: [3]SkierGroup{
			{Pool: 6000}, // Beginner
			{Pool: 3000}, // Intermediate
			{Pool: 1000}, // Advanced
		},
		ResortRating: initialResortRating,
	}
}

// maybePoll fires one demand decision per group when at least
// demandPollInterval sim-seconds have elapsed since the last poll.
func (d *DemandSystem) maybePoll(s *Simulation) {
	if s.SimTime-d.LastPoll < demandPollInterval {
		return
	}
	d.LastPoll = s.SimTime

	// Piggyback the slow cadence with a one-pass linear decay of skier
	// tracks in the surface-detail R channel. 0.985 per 30 s sim time
	// ≈ 30-min half-life — tracks linger but don't accumulate forever,
	// and we don't pay for a separate timer or a per-tick walk.
	if s.World != nil && s.World.Terrain != nil {
		s.World.Terrain.Surface.DecayTracks(0.985)
	}

	cap := resortCapacity(s.World)
	if cap <= 0 {
		return // no lifts → no skiers want to come
	}
	occupancy := float32(len(s.World.Agents)) / cap

	for skill := ai.SkillLevel(0); skill < 3; skill++ {
		group := &d.Groups[skill]
		if group.Pool < SkiersPerCar {
			continue
		}
		match := terrainMatch(s.World, skill)
		if match == 0 {
			continue
		}
		p := visitProbability(d.ResortRating, match, occupancy)
		if s.Rng.Float32() >= p {
			continue
		}
		lot := uniformParking(s.World, s.Rng)
		if lot == nil {
			return // no lots at all — nothing to do for any group
		}
		spawned := 0
		for i := 0; i < SkiersPerCar; i++ {
			if s.spawnSkier(lot, skill) {
				spawned++
			}
		}
		if spawned > 0 {
			group.Pool -= spawned
			lot.CurrentCars += 1
			if max := float32(lot.MaxCars); max > 0 && lot.CurrentCars > max {
				lot.CurrentCars = max
			}
		}
	}
}

// scoreDeparture computes the 0..1 rating contribution of one ending
// session from its event log + final Fun/Energy. Cleanness penalises
// falls per completed run — a session with zero falls scores 1 on
// that axis regardless of how few runs; many falls per run scores 0.
func scoreDeparture(a *world.Agent) float32 {
	falls, runs := 0, 0
	for _, e := range a.Events {
		switch e.Kind {
		case ai.EventFall:
			falls++
		case ai.EventRun:
			runs++
		}
	}
	clean := float32(1)
	if runs > 0 {
		ratio := float32(falls) / float32(runs)
		if ratio > 1 {
			ratio = 1
		}
		clean = 1 - ratio
	}
	score := scoreWeightFun*a.Fun +
		scoreWeightEnergy*a.Energy +
		scoreWeightClean*clean
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}

// recordDeparture is called once at the moment of ActDepart, before the
// agent's Removed flag is set. Updates the rating EMA and returns the
// skier to their group's pool.
func (d *DemandSystem) recordDeparture(a *world.Agent) {
	score := scoreDeparture(a)
	d.ResortRating = (1-ratingEMAλ)*d.ResortRating + ratingEMAλ*score
	skill := int(a.Traits.Skill)
	if skill < 0 || skill >= len(d.Groups) {
		return
	}
	d.Groups[skill].Pool++
}

// =============================================================================
// Pure functions — easy to unit-test, no Simulation reference needed
// =============================================================================

// resortCapacity is the "comfortable skiers-at-once" estimate used to
// gate demand. Sum over lifts of (chairs × seats per chair) × session
// length / loop time — i.e. how many skier-seats turn over within one
// typical session.
func resortCapacity(w *world.World) float32 {
	var total float32
	for _, l := range w.Lifts {
		if len(l.Chairs) == 0 {
			continue
		}
		seats := len(l.Chairs[0].Passengers)
		if seats == 0 {
			continue
		}
		loop := l.LoopLength()
		if loop <= 0 || l.Speed <= 0 {
			continue
		}
		loopTime := loop / (2 * l.Speed)
		if loopTime <= 0 {
			continue
		}
		total += float32(len(l.Chairs)*seats) / loopTime * avgSessionSec
	}
	return total
}

// terrainMatch is the binary skill-vs-lift-services check. Returns 1
// if any lift in the resort serves the group's skill bit, else 0. A
// smoother form (fraction of lifts matching) is the obvious next
// iteration once `Services` becomes derived from trails.
func terrainMatch(w *world.World, skill ai.SkillLevel) float32 {
	want := skillToDifficulty(skill)
	for _, l := range w.Lifts {
		if l.Services.Has(want) {
			return 1
		}
	}
	return 0
}

func skillToDifficulty(skill ai.SkillLevel) world.TerrainDifficulty {
	switch skill {
	case ai.SkillBeginner:
		return world.DiffGreen
	case ai.SkillIntermediate:
		return world.DiffBlue
	case ai.SkillAdvanced:
		return world.DiffBlack
	}
	return world.DiffGreen
}

// visitProbability is the Bernoulli p the poll draws against.
// Multiplicative: any factor near zero kills demand. Rating and
// occupancy are clamped to [0,1]; terrainMatch is already binary.
func visitProbability(rating, match, occupancy float32) float32 {
	r := clamp01(rating)
	o := clamp01(occupancy)
	p := r * match * (1 - o)
	return clamp01(p)
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// uniformParking returns a uniformly-chosen Parking building, or nil
// if the world has none. Sheds and lodges are skipped — only parking
// lots are valid drop-off points.
func uniformParking(w *world.World, rng *rand.Rand) *world.Building {
	lots := make([]*world.Building, 0, len(w.Buildings))
	for _, b := range w.Buildings {
		if b.Type == world.BuildingParking {
			lots = append(lots, b)
		}
	}
	if len(lots) == 0 {
		return nil
	}
	return lots[rng.Intn(len(lots))]
}
