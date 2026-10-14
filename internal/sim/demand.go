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
//	p_per_poll(g) = g.VisitsPerSeason * dailyVisitShare * dayTypeDemand * pollFraction
//	              * clamp(ResortRating) * terrainDraw(g) * (1 - occupancy)
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

// dailyVisitShare turns a guest's VisitsPerSeason into their chance of
// coming on a day, before how busy the day is (dayTypeDemand) and the
// resort's draw. It's a tuning constant, deliberately not 1 /
// world.SeasonDays: the season's length is chosen for play time, and
// changing it shouldn't move every day's crowd. VisitsPerSeason ranks
// guests from casual to regular rather than counting visits exactly.
const dailyVisitShare = 0.2

// dayTypeDemand scales a day's visits by its kind (world.HolidayAt): a
// holiday brings two and a half times an ordinary day's crowd, a named
// one nearly seven times.
var dayTypeDemand = [...]float32{
	world.OrdinaryDay:  0.6,
	world.Holiday:      1.5,
	world.NamedHoliday: 4.0,
}

// familyDemand scales a family's chance of coming by the kind of day:
// school holidays bring them, and ordinary days have the kids in school.
// familySchoolDraw is how much more a family comes to a resort with a
// ski school.
var familyDemand = [...]float32{
	world.OrdinaryDay:  0.8,
	world.Holiday:      1.2,
	world.NamedHoliday: 1.6,
}

const familySchoolDraw = 1.15

// afternoonDraw is how much more likely an arrival from the afternoon
// ticket's start is, per share of the adult price it saves: at half
// price, 75% more.
const afternoonDraw = 1.5

// hasKid reports whether a kid is in grp.
func hasKid(grp []*world.Guest) bool {
	for _, g := range grp {
		if g.Child {
			return true
		}
	}
	return false
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

	if resortCapacity(s.World) <= 0 {
		return // no lifts → no guests want to come
	}
	occupancy := Occupancy(s.World)

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
	offer := terrainOn(s.World)
	occFactor := 1 - occupancy
	dayType, _ := world.HolidayAt(s.DateAt(s.SimTime))
	busy := dayTypeDemand[dayType]
	school := anySchool(s.World)
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
			p += visitChance(s, g, rating, busy, terrainDraw(s.World, offer, g.Traits), h0, h1, occFactor, hasRentals)
			anyTicket = anyTicket || !hasValidPass(g, s.SimTime)
		}
		if !home || p <= 0 {
			continue
		}
		p /= float32(len(grp))
		if hasKid(grp) {
			// Families come on holidays more, school days less, and more
			// to a resort with a ski school.
			p *= familyDemand[dayType]
			if school {
				p *= familySchoolDraw
			}
		}
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
			// filled evenly: a van for five to seven (world.RollCar).
			cars := (len(grp) + world.MaxCarSeats - 1) / world.MaxCarSeats
			for i := 0; i < cars; i++ {
				lo, hi := len(grp)*i/cars, len(grp)*(i+1)/cars
				s.spawnCar(append([]*world.Guest(nil), grp[lo:hi]...), e)
			}
		}
	}
}

// visitChance is the chance guest g would come in this poll window
// (h0 to h1, clock hours), alone: their visits a season as a daily
// chance, how busy the day is, when they like to arrive, the rating, the
// terrain, how full the resort is, and the price.
func visitChance(s *Simulation, g *world.Guest, rating, busy, match float32, h0, h1 float64, occFactor float32, hasRentals bool) float32 {
	if match == 0 {
		return 0
	}
	priceFactor := visitPriceFactor(s.World, g, s.SimTime, rating)
	if priceFactor == 0 {
		return 0
	}
	dailyRate := g.VisitsPerSeason * dailyVisitShare * busy
	p := dailyRate * float32(arrivalShare(s.World, g, h0, h1)) * rating * match * occFactor * priceFactor
	if t := s.World.Tickets; t.Afternoon && h0 >= world.AfternoonFrom && s.World.DayTicketPrice > 0 && !hasValidPass(g, s.SimTime) {
		// A cheaper afternoon brings half-day skiers who'd not have come.
		off := 1 - min(max(float32(t.AfternoonPrice)/float32(s.World.DayTicketPrice), 0), 1)
		p *= 1 + afternoonDraw*off
	}
	p += eveningChance(s, g, dailyRate, h0, h1) * rating * match * occFactor * priceFactor
	if !hasRentals && !hasValidPass(g, s.SimTime) {
		// Some who'd have rented skis stay home from a resort with
		// no rental shop.
		p *= 1 - world.RentalShare(g.Traits.Skill)*world.NoRentalsStayHome
	}
	return p
}

// dayTicketCharge returns the day ticket guest g will pay for a visit
// starting at simTime and whether they can afford to come at all. Pass
// holders pay 0 and always can; everyone else pays their ticket (the
// adult price, or a kids', a child's or an afternoon ticket's) if
// their DailyBudget covers it. spawnGuest sets the price aside from the
// guest's budget and the ticket office collects it (ActBuyDayTicket);
// the demand poll weighs it via visitPriceFactor.
func dayTicketCharge(w *world.World, g *world.Guest, simTime float64) (price int, ok bool) {
	if hasValidPass(g, simTime) {
		return 0, true
	}
	// The adult price, or a kid's or the afternoon's (world.TicketPrice).
	price = w.TicketPrice(g, HourOfDay(simTime))
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

// Occupancy is how full the resort feels to guests deciding whether to
// come, 0..1: the fuller of the lifts and the runs. The lifts count
// everyone on the mountain or driving in except those indoors (a lunch
// in the lodge doesn't crowd the lifts) against resortCapacity; the runs
// count only the guests out on the snow against TerrainCapacity, since
// that's a measure of skiers per acre. Counting the lodge against the
// runs held a one-lift resort with a lodge to about two thirds of the
// arrivals of one without, because guests with lunch stay all day
// (Boreal Tutorial, 2026-10-10).
func Occupancy(w *world.World) float32 {
	liftCap := resortCapacity(w)
	if liftCap <= 0 {
		return 1
	}
	cycling := float32(arrivingGuests(w))
	var onSnow float32
	for _, g := range w.OnMountain {
		if g.Indoors() {
			continue
		}
		cycling++
		if onTheSnow(g) {
			onSnow++
		}
	}
	occ := cycling / liftCap
	if tc := TerrainCapacity(w); tc > 0 {
		occ = max(occ, onSnow/tc)
	}
	return min(occ, 1)
}

// onTheSnow reports whether g is out on the runs: not on a lift, in a
// line, fallen and waiting for patrol, or walking a path at the base.
func onTheSnow(g *world.Guest) bool {
	return g.OnLiftID == 0 && g.Unload.LiftID == 0 && !g.Queued && !g.Visit.Waiting &&
		g.OnPatrollerID == 0 && !(len(g.Path) > 0 && g.PathIdx < len(g.Path))
}

// TerrainCapacity returns the maximum simultaneous guests the trails the
// running lifts serve (World.ReachableTrails) can support. Deduplicates
// cells shared across overlapping trails. Returns 0 when no trails are
// served — callers treat 0 as "no constraint".
func TerrainCapacity(w *world.World) float32 {
	seen := make(map[[2]int]struct{})
	for _, t := range w.ReachableTrails() {
		for _, c := range t.Cells {
			seen[c] = struct{}{}
		}
	}
	return float32(len(seen)) * guestsPerTrailCell
}

// terrainOffer is the terrain the running lifts serve
// (World.ReachableTrails), by run level (0 green, 1 blue, 2 black): its
// trail cells and how many trails. Worked out once a poll.
type terrainOffer struct {
	cells, trails [3]float32
}

// terrainOn sums the terrain w's running lifts serve.
func terrainOn(w *world.World) terrainOffer {
	var o terrainOffer
	for _, t := range w.ReachableTrails() {
		d := diffIndex(t.Difficulty)
		if d < 0 || len(t.Cells) == 0 {
			continue
		}
		o.cells[d] += float32(len(t.Cells))
		o.trails[d]++
	}
	return o
}

// terrainDraw is how strongly the terrain on offer pulls a guest, 0..1:
// how much of it suits them, and how many runs. Each level counts by how
// it sits against the guest's range (runRange: from the level they're
// comfortable at to the level they want): fully in range, 0.3 one level
// easier, 0.1 two easier, not at all harder. The amount saturates
// (terrainDrawCells of suitable cells draws half as well as unlimited
// terrain), and so does the choice of runs (terrainDrawTrails suitable
// runs, half). So a beginner hill draws beginners and few experts, and
// each lift that adds terrain or runs at a level draws more of that
// level, less so the more there already is. It needs a running lift
// they'd ride (one serving their level or easier, or any for advanced
// guests, as the planner allows): guests don't come to stand at a
// stopped lift or one with nothing for them.
func terrainDraw(w *world.World, o terrainOffer, traits ai.GuestTraits) float32 {
	want := skillToDifficulty(traits.Skill)
	ride := want | (want - 1) // their level and everything easier
	if traits.Skill >= ai.SkillAdvancedThreshold {
		ride = 0
	}
	if !w.RunningLiftFor(ride) {
		return 0
	}
	lo, hi := runRange(traits)
	var cells, trails float32
	for d := range 3 {
		var k float32
		switch {
		case d > hi:
		case d >= lo:
			k = 1
		case d == lo-1:
			k = 0.3
		default:
			k = 0.1
		}
		cells += k * o.cells[d]
		trails += k * o.trails[d]
	}
	if cells == 0 {
		return 0
	}
	return cells / (cells + terrainDrawCells) * trails / (trails + terrainDrawTrails)
}

// The amount of suitable terrain (5 m trail cells) and number of
// suitable runs at which each draws half as well as it could.
const (
	terrainDrawCells  = float32(3000)
	terrainDrawTrails = float32(6)
)

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

// Evening guests: a night session draws locals after work and the like,
// arriving from eveningFrom to eveningTo (or an hour before closing), on
// top of the day's guests, up to nightDraw of a day's visits with half
// the runs lit or more.
const (
	eveningFrom = world.NightFrom + 0.5
	eveningTo   = 19.0
	nightDraw   = float32(0.35)
	// eveningEnergy is the energy an evening guest arrives with: a day's
	// work behind them, they ski two or three hours.
	eveningEnergy = float32(0.6)
)

// eveningChance is guest g's chance, before the rating and the rest, of
// coming for the night session in the poll window h0 to h1.
func eveningChance(s *Simulation, g *world.Guest, dailyRate float32, h0, h1 float64) float32 {
	w := s.World
	if !w.NightSession() {
		return 0
	}
	from, to := eveningFrom, min(eveningTo, float64(w.CloseHour)-arrivalLastHourBeforeClose)
	if to <= from {
		return 0
	}
	share := (math.Min(h1, to) - math.Max(h0, from)) / (to - from)
	if share <= 0 {
		return 0
	}
	lit := min(w.LitShare()*2, 1)
	return dailyRate * nightDraw * lit * float32(share)
}
