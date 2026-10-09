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
//	p_per_poll(g) = (g.VisitsPerSeason / world.SeasonDays) * dayTypeDemand * pollFraction
//	              * clamp(ResortRating) * terrainMatch(g.Skill) * (1 - occupancy)
//	              * visitPriceFactor(g, rating)
//	              * (1 − RentalShare × NoRentalsStayHome, with no rental shop and no pass)
//
// Guests come in groups (world.FormGroups): the poll rolls each group's
// visit once, at its members' mean chance, and a group drives in from its
// entry in as few cars as hold it (traffic.go); the guests
// move into w.OnMountain when the car parks. On Depart a guest waits in
// the car, and when the carload is aboard it drives home and they
// return to AtHome (career stats incremented), ready to be rolled again
// on a future poll.
//
// The resort rating is the average stars of the guests who left on the
// previous day (set at rollover from History.DayRating), so
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

// A guest's VisitsPerSeason spread over the season's game days
// (world.SeasonDays) gives their visits a day, scaled by how busy the day
// is (dayTypeDemand).

// dayTypeDemand scales a day's visits by its kind (world.HolidayAt): a
// holiday brings two and a half times an ordinary day's crowd, a named
// one nearly seven times. Over a season (40 ordinary days, six holidays,
// four named) they average about 1, so a season still brings each guest
// VisitsPerSeason.
var dayTypeDemand = [...]float32{
	world.OrdinaryDay:  0.6,
	world.Holiday:      1.5,
	world.NamedHoliday: 4.0,
}

// Capacity formula constant. avgSessionSec is how long a typical guest
// takes over one lift cycle (line, ride, and descent): about 1,500 s on
// the Boreal Goals Test, where a guest rides about three times in five
// and a half clock hours (measured 2026-10-09).
const avgSessionSec = 1500.0

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
	h0 := math.Max(0, h1-elapsed/simSecondsPerHour)
	if a, b, ok := arrivalWindow(s.World); !ok || h1 <= a || h0 >= b {
		return // outside the hours guests arrive in
	}
	if !s.World.AnyLiftRunning() {
		// Open, but every lift is stopped (new lifts start stopped):
		// say so once a day rather than letting guests come for nothing.
		d.logTurnedAway(s, "every lift is stopped")
		return
	}

	rating := s.World.RatingShare()
	occFactor := 1 - occupancy
	dayType, _ := world.HolidayAt(s.DateAt(s.SimTime))
	busy := dayTypeDemand[dayType]
	hasOffice := hasTicketOffice(s.World)
	hasRentals := anyRentals(s.World)
	if !hasParking(s.World) {
		return // no lots → no arrivals
	}
	winners := map[uint64][][]*world.Guest{} // groups, by home entry

	// A group's visit is one roll, at the chance its members would
	// come on average, so a season brings each guest about as many
	// visits as when they came alone.
	for _, grp := range s.World.GuestGroups() {
		var p float32
		home, anyTicket := true, false
		for _, g := range grp {
			if g.State != world.AtHome {
				home = false
				break
			}
			p += visitChance(s, g, rating, busy, match(s.World, g), h0, h1, occFactor, hasRentals)
			anyTicket = anyTicket || !hasValidPass(g, s.SimTime)
		}
		if !home || p <= 0 {
			continue
		}
		p /= float32(len(grp))
		if rng.Global().Float32() >= p {
			continue
		}
		// Day tickets are sold only at a ticket office; without one,
		// only groups of pass holders come.
		if !hasOffice && anyTicket {
			d.logTurnedAway(s, "no ticket office")
			continue
		}
		winners[grp[0].HomeEntryID] = append(winners[grp[0].HomeEntryID], grp)
	}
	entries := []uint64{0}
	for _, e := range s.World.Entries() {
		entries = append(entries, e.ID)
	}
	for _, e := range entries {
		for _, grp := range winners[e] {
			// A group bigger than a car comes in as few cars as hold it,
			// filled evenly.
			cars := (len(grp) + carSeats - 1) / carSeats
			for i := 0; i < cars; i++ {
				lo, hi := len(grp)*i/cars, len(grp)*(i+1)/cars
				s.spawnCar(append([]*world.Guest(nil), grp[lo:hi]...), e)
			}
		}
	}
}

// carSeats is how many guests a car holds.
const carSeats = 4

// match is terrainMatch for g.
func match(w *world.World, g *world.Guest) float32 { return terrainMatch(w, g.Traits) }

// visitChance is the chance guest g would come in this poll window
// (h0 to h1, clock hours), alone: their visits a season spread over its
// days, how busy the day is, when they like to arrive, the rating, the
// terrain, how full the resort is, and the price.
func visitChance(s *Simulation, g *world.Guest, rating, busy, match float32, h0, h1 float64, occFactor float32, hasRentals bool) float32 {
	if match == 0 {
		return 0
	}
	priceFactor := visitPriceFactor(s.World, g, s.SimTime, rating)
	if priceFactor == 0 {
		return 0
	}
	dailyRate := g.VisitsPerSeason / world.SeasonDays * busy
	p := dailyRate * float32(arrivalShare(s.World, g, h0, h1)) * rating * match * occFactor * priceFactor
	if !hasRentals && !hasValidPass(g, s.SimTime) {
		// Some who'd have rented skis stay home from a resort with
		// no rental shop.
		p *= 1 - world.RentalShare(g.Traits.Skill)*world.NoRentalsStayHome
	}
	return p
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

// recordDeparture is called once when a guest's car leaves the map
// (finishDeparture). Captures the stars they left as LastStars and bumps
// career stats. The day's departures set the rating at rollover
// (History.DayRating).
func (d *DemandSystem) recordDeparture(w *world.World, g *world.Guest, stars float32, today time.Time) {
	g.LastStars = stars
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
// gate demand: each lift's riders a second (Lift.RidersPerSecond) times
// how long a typical guest takes over a cycle (avgSessionSec).
func resortCapacity(w *world.World) float32 {
	var total float32
	for _, l := range w.Lifts {
		total += l.RidersPerSecond() * avgSessionSec
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

// terrainMatch is how well the resort's marked runs suit a guest, 0..1.
// Each run cell counts by how its difficulty sits against the guest's
// range (runRange: from the level they're comfortable at to the level
// they want): fully in range, 0.3 one level easier, 0.1 two easier, not
// at all harder. The share of suitable terrain over terrainMatchFull
// counts in full, so a resort needn't be all one level to draw a guest,
// but an easy hill with a few black runs draws few experts looking for a
// challenge, and mostly those happy cruising. Only runs the player has
// marked count. It needs a running lift they'd ride (one serving their
// level or easier, or any for advanced guests, as the planner allows):
// guests don't come to stand at a stopped lift or one with nothing for
// them.
func terrainMatch(w *world.World, traits ai.GuestTraits) float32 {
	want := skillToDifficulty(traits.Skill)
	ride := want | (want - 1) // their level and everything easier
	if traits.Skill >= ai.SkillAdvancedThreshold {
		ride = 0
	}
	if !w.RunningLiftFor(ride) {
		return 0
	}
	lo, hi := runRange(traits)
	var suits, total float32
	for _, t := range w.Trails {
		d := diffIndex(t.Difficulty)
		if d < 0 || len(t.Cells) == 0 {
			continue
		}
		n := float32(len(t.Cells))
		total += n
		switch {
		case d > hi:
		case d >= lo:
			suits += n
		case d == lo-1:
			suits += n * 0.3
		default:
			suits += n * 0.1
		}
	}
	if total == 0 {
		return 0
	}
	return min(suits/total/terrainMatchFull, 1)
}

// terrainMatchFull is the share of suitable marked terrain at which a
// resort's terrain draws a guest as fully as it can.
const terrainMatchFull = float32(0.5)

// Steep tastes past these shift the run level a guest wants: one easier
// for those who'd rather cruise, one harder for those after a challenge.
const (
	cruiseSteepTaste    = float32(-0.3)
	challengeSteepTaste = float32(0.5)
)

// runRange is the run levels (0 green, 1 blue, 2 black) a guest is happy
// skiing: from the easier of their skill level and the level they want
// to the harder of the two. The level they want is their skill level,
// one easier if they dislike steeps (happy cruising), one harder if they
// love them (after a challenge).
func runRange(traits ai.GuestTraits) (lo, hi int) {
	level := diffIndex(skillToDifficulty(traits.Skill))
	want := level
	switch steep := traits.Tastes[ai.TasteSteep]; {
	case steep <= cruiseSteepTaste:
		want--
	case steep >= challengeSteepTaste:
		want++
	}
	want = min(max(want, 0), 2)
	return min(level, want), max(level, want)
}

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
