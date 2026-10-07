package sim

import (
	"math"
	"time"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/rng"
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
//	              * visitPriceFactor(g, rating)
//
// The poll's winners from each entry share cars, one to four to a car
// (rollCarload), which drive in from the entry (traffic.go); the guests
// move into w.OnMountain when the car parks. On Depart a guest waits in
// the car, and when the carload is aboard it drives home and they
// return to AtHome (career stats incremented), ready to be rolled again
// on a future poll.
//
// The resort rating is the average final Satisfaction of the guests who
// left on the previous day (set at rollover from History.DayRating), so
// it reflects completed sessions — word-of-mouth from guests who
// finished their day — rather than whoever happens to be mid-run.

// demandPollInterval is the sim-time cadence of the per-Guest visit
// poll. Short enough that arrivals spread continuously through the day
// rather than landing all at once.
const demandPollInterval = 30.0

// The resort rating itself lives on the world (World.Rating), so it's
// saved; it bootstraps at world.InitialRating before any
// guests have departed — neutral, so demand picks up at 50% of the
// headline rate until real departures start folding in.

// seasonDaysApprox is the constant divisor for per-day visit rates.
// Real season length varies year-to-year as Memorial Day moves, but the
// difference is ~2% — not worth a per-poll recompute.
const seasonDaysApprox = 186.0

// Capacity formula constant. avgSessionSec is how long a typical guest
// occupies a lift seat across one cycle (queue + ride + descent).
const avgSessionSec = 800.0

// guestsPerTrailCell is the terrain-density tuning knob for TerrainCapacity.
// Each unique 5×5 m trail cell supports this many simultaneous guests —
// equivalent to ~16 guests per skiable acre at k=0.10. Adjust only this
// constant to retune the terrain-vs-lift balance.
const guestsPerTrailCell = float32(0.10)

// DemandSystem owns the resort rating + poll timers. The catchment
// itself lives on world.World.Guests; this struct only tracks the
// scalar rating and the timers that gate the per-guest walks.
type DemandSystem struct {
	LastPoll float64
	// Season is the close year (SeasonCloseYearFor) of the season the
	// last poll ran in; 0 until the first poll. Not persisted — a loaded
	// save re-seeds it from SimTime without triggering a reset.
	Season int
	// turnedAwayDay is 1 + the sim day index on which "no ticket office"
	// was last logged; 0 = never. Keeps the event to once per day.
	turnedAwayDay int
}

// NewDemandSystem bootstraps a fresh demand system with a neutral rating.
func NewDemandSystem() *DemandSystem {
	return &DemandSystem{}
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
	d.checkSeasonRollover(s)
	if !s.World.ResortOpen {
		return // closed: nobody comes, pass holders included
	}

	// Piggyback the slow cadence with a one-pass linear decay of skier
	// tracks in the surface-detail R channel. 0.985 per 30 s sim time
	// ≈ 30-min half-life — tracks linger but don't accumulate forever.
	if s.World != nil && s.World.Terrain != nil {
		s.World.Terrain.Surface.DecayTracks(0.985)
	}

	liftCap := resortCapacity(s.World)
	if liftCap <= 0 {
		return // no lifts → no guests want to come
	}
	cap := liftCap
	if tc := TerrainCapacity(s.World); tc > 0 && tc < liftCap {
		cap = tc
	}
	occupancy := float32(len(s.World.OnMountain)+arrivingGuests(s.World)) / cap
	if occupancy > 1 {
		occupancy = 1
	}

	// The share of the day's arrivals that fall in this poll window:
	// guests come from just before opening, mostly in the morning, and
	// stop an hour before closing (arrivalShare).
	h1 := HourOfDay(s.SimTime)
	pollFractionOfDay := float32(arrivalShare(s.World, math.Max(0, h1-elapsed/simSecondsPerHour), h1))
	if pollFractionOfDay <= 0 {
		return
	}
	if !s.World.AnyLiftRunning() {
		// Open, but every lift is stopped (new lifts start stopped):
		// say so once a day rather than letting guests come for nothing.
		d.logTurnedAway(s, "every lift is stopped")
		return
	}

	rating := clamp01(s.World.Rating)
	occFactor := 1 - occupancy
	hasOffice := hasTicketOffice(s.World)
	if !hasParking(s.World) {
		return // no lots → no arrivals
	}
	winners := map[uint64][]*world.Guest{} // by home entry

	for _, g := range s.World.Guests {
		if g.State != world.AtHome {
			continue
		}
		priceFactor := visitPriceFactor(s.World, g, s.SimTime, rating)
		if priceFactor == 0 {
			continue
		}
		match := terrainMatch(s.World, g.Traits.Skill)
		if match == 0 {
			continue
		}
		dailyRate := g.VisitsPerSeason / seasonDaysApprox
		p := dailyRate * pollFractionOfDay * rating * match * occFactor * priceFactor
		if p <= 0 || rng.Global().Float32() >= p {
			continue
		}
		// Day tickets are sold only at a ticket office; without one,
		// only pass holders come.
		if !hasOffice && !hasValidPass(g, s.SimTime) {
			d.logTurnedAway(s, "no ticket office")
			continue
		}
		winners[g.HomeEntryID] = append(winners[g.HomeEntryID], g)
	}
	entries := []uint64{0}
	for _, e := range s.World.Entries() {
		entries = append(entries, e.ID)
	}
	for _, e := range entries {
		gs := winners[e]
		for len(gs) > 0 {
			n := min(rollCarload(), len(gs))
			s.spawnCar(append([]*world.Guest(nil), gs[:n]...), e)
			gs = gs[n:]
		}
	}
}

// dayTicketCharge returns the day ticket guest g will pay for a visit
// starting at simTime and whether they can afford to come at all. Pass
// holders pay 0 and always can; everyone else pays w.DayTicketPrice if
// their DailyBudget covers it. spawnGuest sets the price aside from the
// guest's budget and the ticket office collects it (ActBuyDayTicket);
// the demand poll weighs it via visitPriceFactor.
func dayTicketCharge(w *world.World, g *world.Guest, simTime float64) (price int, ok bool) {
	if hasValidPass(g, simTime) {
		return 0, true
	}
	price = w.DayTicketPrice
	if price < 0 {
		price = 0
	}
	return price, float32(price) <= g.Traits.DailyBudget
}

// visitPriceFactor is the price-elasticity term in the visit
// probability, in [0, 1]. The price is the guest's day ticket (0 for pass
// holders) plus their share of the parking fee. It is 1 at or below a
// reference price, 0 once the price exceeds the guest's DailyBudget, and
// ((budget − price) / (budget − ref))^elasticity in between. The
// reference is the parking reference share plus, for guests who need a
// ticket, the rating-shifted ticket reference. See the DayTicket* and
// ParkingReferencePrice constants in world.go.
func visitPriceFactor(w *world.World, g *world.Guest, simTime float64, rating float32) float32 {
	price, ok := dayTicketCharge(w, g, simTime)
	if !ok {
		return 0
	}
	p := float32(price) + parkingShare(w)
	budget := g.Traits.DailyBudget
	if p > budget {
		return 0
	}
	if p == 0 {
		return 1 // free ticket (or pass) and free parking
	}
	ref := float32(world.ParkingReferencePrice) / world.MeanCarload
	if !hasValidPass(g, simTime) {
		ref += world.DayTicketReferencePrice * (1 + world.DayTicketRatingPremium*(rating-0.5))
	}
	if p <= ref {
		return 1
	}
	// ref < p ≤ budget here, so the denominator is positive.
	return float32(math.Pow(float64((budget-p)/(budget-ref)), float64(world.DayTicketElasticity)))
}

// parkingShare is one guest's average share of the per-car parking fee.
func parkingShare(w *world.World) float32 {
	if w.ParkingPrice <= 0 {
		return 0
	}
	return float32(w.ParkingPrice) / world.MeanCarload
}

// hasTicketOffice reports whether w has a ticket window guests can get
// into to buy day tickets.
func hasTicketOffice(w *world.World) bool {
	for _, b := range w.Buildings {
		if b.Offers(world.ServiceTickets) {
			return true
		}
	}
	return false
}

// logTurnedAway writes "Guests turned away: no ticket office" to the event
// feed at most once per sim day.
func (d *DemandSystem) logTurnedAway(s *Simulation, why string) {
	day := int(s.SimTime/secondsPerSimDay) + 1
	if d.turnedAwayDay == day {
		return
	}
	d.turnedAwayDay = day
	s.World.LogEvent(world.EventGuestsTurnedAway, s.SimTime, "Guests turned away: "+why)
}

// hasValidPass reports whether g holds a season pass that hasn't expired
// at simTime.
func hasValidPass(g *world.Guest, simTime float64) bool {
	return g.SeasonPassExpiry > 0 && simTime < g.SeasonPassExpiry
}

// recordDeparture is called once at the moment of ActDepart, before the
// guest's Removed flag is set. Captures the session Satisfaction as
// LastScore and bumps career stats. The day's departures set the rating
// at rollover (History.DayRating).
func (d *DemandSystem) recordDeparture(w *world.World, g *world.Guest, today time.Time) {
	g.LastScore = g.Satisfaction
	g.LifetimeVisits++
	g.VisitsThisSeason++
	g.LastVisit = today
}

// checkSeasonRollover detects the season boundary from SimTime and, when
// it has moved since the last poll, clears per-season guest counters.
// The first call only seeds d.Season.
func (d *DemandSystem) checkSeasonRollover(s *Simulation) {
	season := SeasonCloseYearFor(s.DateAt(s.SimTime))
	if d.Season != 0 && season != d.Season {
		resetSeasonCounters(s.World)
	}
	d.Season = season
}

// resetSeasonCounters zeroes every per-season field on every guest in the
// catchment. This is the single place a new season starts for guests;
// the derived-season work should call it rather than duplicating resets.
func resetSeasonCounters(w *world.World) {
	if w == nil {
		return
	}
	for _, g := range w.Guests {
		g.VisitsThisSeason = 0
	}
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

// TerrainCapacity returns the maximum simultaneous guests the resort's painted
// trail area can support. Deduplicates cells shared across overlapping trails.
// Returns 0 when the world has no trails — callers treat 0 as "no constraint".
func TerrainCapacity(w *world.World) float32 {
	seen := make(map[[2]int]struct{})
	for _, t := range w.Trails {
		for _, c := range t.Cells {
			seen[c] = struct{}{}
		}
	}
	return float32(len(seen)) * guestsPerTrailCell
}

// terrainMatch is how well the resort's terrain suits a guest: 1 with a
// painted trail at their skill level, easierTerrainMatch with only easier
// trails (they'll come, less often, and wish for more), else 0. Either
// way it needs a running lift they'd ride (one serving their level or
// easier, or any for advanced guests, as the planner allows): guests
// don't come to stand at a stopped lift or one with nothing for them.
func terrainMatch(w *world.World, skill float32) float32 {
	want := skillToDifficulty(skill)
	ride := want | (want - 1) // their level and everything easier
	if skill >= ai.SkillAdvancedThreshold {
		ride = 0
	}
	if !w.RunningLiftFor(ride) {
		return 0
	}
	match := float32(0)
	for _, t := range w.Trails {
		if len(t.Cells) == 0 {
			continue
		}
		if t.Difficulty == want {
			return 1
		}
		if t.Difficulty < want {
			match = easierTerrainMatch
		}
	}
	return match
}

// easierTerrainMatch is terrainMatch for a resort whose trails are all
// below a guest's level.
const easierTerrainMatch = float32(0.4)

func skillToDifficulty(skill float32) world.TerrainDifficulty {
	switch {
	case skill >= ai.SkillAdvancedThreshold:
		return world.DiffBlack
	case skill >= ai.SkillIntermediateThreshold:
		return world.DiffBlue
	default:
		return world.DiffGreen
	}
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

// hasParking reports whether w has a parking lot.
func hasParking(w *world.World) bool {
	for _, b := range w.Buildings {
		if b.Type == world.BuildingParking {
			return true
		}
	}
	return false
}
