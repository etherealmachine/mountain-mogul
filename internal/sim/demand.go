package sim

import (
	"math/rand"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// =============================================================================
// Demand system — per-Guest visit poll + resort rating
// =============================================================================
//
// Every potential visitor lives as a *Guest in world.World.Guests with a
// per-guest VisitsPerSeason drawn at world init. Every demandPollInterval
// sim-seconds the system walks the catchment and rolls a Bernoulli per
// AtHome guest:
//
//	p_per_poll(g) = (g.VisitsPerSeason / seasonDays) * pollFraction
//	              * clamp(ResortRating) * terrainMatch(g.Skill) * (1 - occupancy)
//
// On a hit the guest spawns at a uniform-random parking lot, moves into
// w.OnMountain, and their State flips to OnMountain. On Depart the same
// guest returns to AtHome (career stats incremented), ready to be
// rolled again on a future poll.
//
// Once per in-game day, maybePollRating averages Guest.Rating over every
// on-mountain guest and sets ResortRating to that mean. The daily
// cadence makes the rating read like a "guest survey" rather than a
// continuous EMA — easier for the player to reason about — and
// aggregating across the whole population (rather than a small sample)
// crushes day-to-day variance without needing temporal smoothing. This
// is what RCT2 / Parkitect / Planet Coaster all converged on.

// GuestsPerCar matches the renderer's "one car ≈ four people" mental
// model. Each spawn bumps CurrentCars by 1/GuestsPerCar (and each
// departure decrements by the same) so the visible car count tracks
// guest population at quarter resolution.
const GuestsPerCar = 4

// demandPollInterval is the sim-time cadence of the per-Guest visit
// poll. Short enough that arrivals spread continuously through the day
// rather than landing all at once.
const demandPollInterval = 30.0

// ratingPollInterval is the sim-time cadence of the rating sample —
// once per in-game day. Aliased to secondsPerSimDay so retuning the
// calendar tempo automatically retunes the rating cadence.
const ratingPollInterval = secondsPerSimDay

// initialResortRating bootstraps the score on opening day before any
// guests have been polled — neutral, so demand picks up at 50% of the
// headline rate until the first rating poll fires.
const initialResortRating = 0.5

// seasonDaysApprox is the constant divisor for per-day visit rates.
// Real season length varies year-to-year as Memorial Day moves, but the
// difference is ~2% — not worth a per-poll recompute.
const seasonDaysApprox = 186.0

// Capacity formula constant. avgSessionSec is how long a typical guest
// occupies a lift seat across one cycle (queue + ride + descent).
const avgSessionSec = 800.0

// DemandSystem owns the resort rating + poll timers. The catchment
// itself lives on world.World.Guests; this struct only tracks the
// scalar rating and the timers that gate the per-guest walks.
type DemandSystem struct {
	ResortRating   float32
	LastPoll       float64
	LastRatingPoll float64
}

// NewDemandSystem bootstraps a fresh demand system with a neutral rating.
func NewDemandSystem() *DemandSystem {
	return &DemandSystem{ResortRating: initialResortRating}
}

// maybePoll walks the catchment and rolls one Bernoulli per AtHome
// guest, spawning the winners. Fires every demandPollInterval sim
// seconds; cheap when called more often because the timer gates the
// whole pass.
//
// The per-guest probability is calibrated so that a guest with
// VisitsPerSeason = N hits roughly N times across the season (with
// noise dominated by the rating and terrainMatch factors).
func (d *DemandSystem) maybePoll(s *Simulation) {
	if s.SimTime-d.LastPoll < demandPollInterval {
		return
	}
	elapsed := s.SimTime - d.LastPoll
	d.LastPoll = s.SimTime

	// Piggyback the slow cadence with a one-pass linear decay of skier
	// tracks in the surface-detail R channel. 0.985 per 30 s sim time
	// ≈ 30-min half-life — tracks linger but don't accumulate forever.
	if s.World != nil && s.World.Terrain != nil {
		s.World.Terrain.Surface.DecayTracks(0.985)
	}

	cap := resortCapacity(s.World)
	if cap <= 0 {
		return // no lifts → no guests want to come
	}
	occupancy := float32(len(s.World.OnMountain)) / cap
	if occupancy > 1 {
		occupancy = 1
	}

	// Convert poll-window length into a per-day fraction. At
	// demandPollInterval = 30s and secondsPerSimDay ≈ 77s, this is
	// ~0.39 of a sim day per poll — most polls land inside one day.
	pollFractionOfDay := float32(elapsed / secondsPerSimDay)

	rating := clamp01(d.ResortRating)
	occFactor := 1 - occupancy

	for _, g := range s.World.Guests {
		if g.State != world.AtHome {
			continue
		}
		match := terrainMatch(s.World, g.Traits.Skill)
		if match == 0 {
			continue
		}
		dailyRate := g.VisitsPerSeason / seasonDaysApprox
		p := dailyRate * pollFractionOfDay * rating * match * occFactor
		if p <= 0 || s.Rng.Float32() >= p {
			continue
		}
		lot := uniformParking(s.World, s.Rng)
		if lot == nil {
			return // no lots → no spawns this poll
		}
		if s.spawnGuest(lot, g) {
			lot.CurrentCars += 1.0 / float32(GuestsPerCar)
			if max := float32(lot.MaxCars); max > 0 && lot.CurrentCars > max {
				lot.CurrentCars = max
			}
		}
	}
}

// recordDeparture is called once at the moment of ActDepart, before the
// guest's Removed flag is set. Captures session score + bumps career
// stats on the persistent Guest record. The resort rating itself is
// sampled separately by maybePollRating.
func (d *DemandSystem) recordDeparture(g *world.Guest, simTime float64) {
	g.LastScore = g.Rating()
	g.LifetimeVisits++
	g.VisitsThisSeason++
	g.LastVisit = DateAt(simTime)
}

// maybePollRating sets ResortRating to the mean Guest.Rating over every
// on-mountain guest. Fires once per ratingPollInterval sim-seconds.
// With zero on-mountain guests the rating is left at its previous
// value.
func (d *DemandSystem) maybePollRating(s *Simulation) {
	if s.SimTime-d.LastRatingPoll < ratingPollInterval {
		return
	}
	d.LastRatingPoll = s.SimTime

	var sum float32
	var n int
	for _, g := range s.World.OnMountain {
		if g.Removed {
			continue
		}
		sum += g.Rating()
		n++
	}
	if n == 0 {
		return
	}
	d.ResortRating = sum / float32(n)
}

// =============================================================================
// Pure helpers
// =============================================================================

// resortCapacity is the "comfortable guests-at-once" estimate used to
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
// if any lift in the resort serves the guest's skill bit, else 0.
func terrainMatch(w *world.World, skill ai.SkillLevel) float32 {
	want := skillToDifficulty(skill)
	for _, l := range w.Lifts {
		if w.ServicesForLift(l.ID).Has(want) {
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

// visitProbability is retained for the rating-vs-match-vs-occupancy
// shape; callers compose with their own scalars on top.
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
// if the world has none.
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
