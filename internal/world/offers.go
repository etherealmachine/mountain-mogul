package world

import "mountain-mogul/internal/ai"

// offerInfo is what using an offer does for a guest and how long it
// takes. The sim applies the stat changes (sim.fulfilOffer); the planner
// only sees the needs it fulfils.
var offerInfo = [ai.OfferCount]struct {
	needs    ai.NeedMask
	duration float32 // sim seconds
}{
	ai.OfferSeat:    {ai.NeedRest.Mask(), SimSecondsPerHour / 3},                             // twenty clock minutes
	ai.OfferMeal:    {ai.NeedHunger.Mask() | ai.NeedThirst.Mask(), SimSecondsPerHour / 2},    // a meal comes with a drink; half an hour at a seat
	ai.OfferDrink:   {ai.NeedThirst.Mask(), SimSecondsPerHour / 6},                           // ten clock minutes
	ai.OfferRentals: {ai.NeedRentals.Mask(), SimSecondsPerHour / 4},                          // a quarter hour to fit boots and skis
	ai.OfferApres:   {ai.NeedApres.Mask() | ai.NeedThirst.Mask(), SimSecondsPerHour * 3 / 4}, // three-quarters of an hour at the bar
	ai.OfferWarmUp:  {ai.NeedWarmth.Mask(), SimSecondsPerHour / 4},                           // a quarter hour by the fire
	ai.OfferWater:   {ai.NeedThirst.Mask(), SimSecondsPerHour / 60},                          // a clock minute at the fountain
}

// OfferNeeds is the set of needs using o fulfils.
func OfferNeeds(o ai.Offer) ai.NeedMask {
	if o >= ai.OfferCount {
		return 0
	}
	return offerInfo[o].needs
}

// OfferDuration is how long using o takes, in sim seconds.
func OfferDuration(o ai.Offer) float32 {
	if o >= ai.OfferCount {
		return 0
	}
	return offerInfo[o].duration
}

// OffersUse reports whether guests can use o here: a seat in a lounge or
// a food court, a meal at a food court, a drink at a bar or a food court.
func (b *Building) OffersUse(o ai.Offer) bool {
	switch o {
	case ai.OfferSeat:
		return b.OffersRest()
	case ai.OfferMeal:
		return b.ServesFood()
	case ai.OfferDrink:
		return b.ServesDrinks()
	case ai.OfferRentals:
		return b.Offers(ServiceRentals)
	case ai.OfferApres:
		return b.Offers(ServiceBar)
	case ai.OfferWarmUp:
		return b.Offers(ServiceLounge)
	case ai.OfferWater:
		return b.FreeWater && b.ServesDrinks()
	}
	return false
}

// OffersAnyUse reports whether guests can use anything here.
func (b *Building) OffersAnyUse() bool {
	for o := ai.OfferNone + 1; o < ai.OfferCount; o++ {
		if b.OffersUse(o) {
			return true
		}
	}
	return false
}

// UsePrice is what using o here costs a guest.
func (b *Building) UsePrice(o ai.Offer) int {
	switch o {
	case ai.OfferMeal:
		return b.MealPrice
	case ai.OfferDrink:
		return b.DrinkPrice
	case ai.OfferApres:
		return 2 * b.DrinkPrice // a round or two
	case ai.OfferRentals:
		return b.RentalPrice
	}
	return 0
}

// UsePool is what using an offer takes up in a building: a seat in the
// food court (meals, and rests where there's no lounge), a lounge seat
// (rests, warming up), a turn at the counter (drinks, at the bar or the
// food court), a seat at the bar (après), or a place at the rental
// bench.
type UsePool uint8

const (
	PoolNone UsePool = iota
	PoolFoodSeats
	PoolLoungeSeats
	PoolCounter
	PoolBarSeats
	PoolRentals
	PoolCount
)

const (
	// DefaultQuality is every service building's quality until there's
	// a way to raise it: guests expect the reference price.
	DefaultQuality = float32(0.5)
	// LoungeSeatsPerCell is lounge seating per 5 × 5 m cell, as a food
	// court's.
	LoungeSeatsPerCell = 10
	// CounterPerCell is how many guests a bar or food-court tile serves
	// drinks to at once.
	CounterPerCell = 4
	// BarSeatsPerCell is après seating per bar tile per floor.
	BarSeatsPerCell = 8
	// RentalsPerCell is how many guests a rental tile fits at once.
	RentalsPerCell = 3
)

// UsePool is the pool using o here takes from.
func (b *Building) UsePool(o ai.Offer) UsePool {
	switch o {
	case ai.OfferSeat:
		if b.Offers(ServiceLounge) {
			return PoolLoungeSeats
		}
		return PoolFoodSeats
	case ai.OfferMeal:
		return PoolFoodSeats
	case ai.OfferDrink:
		return PoolCounter
	case ai.OfferWater:
		return PoolNone // help yourself: no turn at the counter
	case ai.OfferWarmUp:
		return PoolLoungeSeats
	case ai.OfferApres:
		return PoolBarSeats
	case ai.OfferRentals:
		return PoolRentals
	}
	return PoolNone
}

// PoolCapacity is how many guests pool p holds at once.
func (b *Building) PoolCapacity(p UsePool) int {
	switch p {
	case PoolFoodSeats:
		return b.Seats()
	case PoolLoungeSeats:
		return b.TileCount(ServiceLounge) * LoungeSeatsPerCell * b.Floors()
	case PoolCounter:
		if b.Offers(ServiceBar) {
			return b.TileCount(ServiceBar) * CounterPerCell
		}
		return b.TileCount(ServiceFood) * CounterPerCell
	case PoolBarSeats:
		return b.TileCount(ServiceBar) * BarSeatsPerCell * b.Floors()
	case PoolRentals:
		return b.TileCount(ServiceRentals) * RentalsPerCell
	}
	return 0
}

// HasRoomFor reports whether a guest can use o now, without waiting.
func (b *Building) HasRoomFor(o ai.Offer) bool {
	p := b.UsePool(o)
	return p == PoolNone || b.InUse[p] < b.PoolCapacity(p)
}

// LineOpen reports whether a guest arriving for o would join the line at
// the door: shorter than the pool holds, so the wait is at most about
// one visit.
func (b *Building) LineOpen(o ai.Offer) bool {
	p := b.UsePool(o)
	return p == PoolNone || b.Waiting[p] < b.PoolCapacity(p)
}

// ExpectedWait is about how long a guest arriving for o would wait to be
// served, in sim seconds: the line ahead of them over the pool's size,
// times how long a visit takes.
func (b *Building) ExpectedWait(o ai.Offer) float32 {
	p := b.UsePool(o)
	c := b.PoolCapacity(p)
	if c == 0 || b.InUse[p] < c {
		return 0
	}
	return float32(b.Waiting[p]+1) / float32(c) * OfferDuration(o)
}

// Occupancy is how full the pool o uses is, 0..1.
func (b *Building) Occupancy(o ai.Offer) float32 {
	p := b.UsePool(o)
	c := b.PoolCapacity(p)
	if c == 0 {
		return 0
	}
	return min(float32(b.InUse[p])/float32(c), 1)
}

// Value for money (Service Improvements). A guest expects to pay the
// offer's reference price (today's defaults), scaled by how much they
// have to spend (half the reference plus half their DailyBudget against
// valueRefBudget, 0.75× to 1.3×: a beginner on the smallest budget
// still finds the default price fair) and by the building's quality
// (0.5× at 0 to 1.5× at 1). Crowding doesn't change the price they
// expect; it scores on its own (sim.fulfilOffer).
// Paying under GoodValueRatio of that is good value, over
// OverpricedRatio overpriced, and at RefuseRatio they won't buy.
var offerRefPrice = [ai.OfferCount]float32{
	ai.OfferMeal:    DefaultMealPrice,
	ai.OfferDrink:   DefaultDrinkPrice,
	ai.OfferApres:   2 * DefaultDrinkPrice,
	ai.OfferRentals: DefaultRentalPrice,
}

const (
	valueRefBudget  = float32(150)
	GoodValueRatio  = float32(0.75)
	OverpricedRatio = float32(1.5)
	RefuseRatio     = float32(2.5)
)

// ExpectedPrice is what a guest with daily budget budget expects to pay
// for o at quality q; 0 for something free.
func ExpectedPrice(o ai.Offer, budget, q float32) float32 {
	if o >= ai.OfferCount {
		return 0
	}
	wealth := min(max(0.5+0.5*budget/valueRefBudget, 0.75), 1.3)
	return offerRefPrice[o] * wealth * (0.5 + q)
}

// PriceRatio is the price of o here over what a guest with daily budget
// budget expects to pay; 0 when it's free.
func (b *Building) PriceRatio(o ai.Offer, budget float32) float32 {
	price := b.UsePrice(o)
	if price <= 0 {
		return 0
	}
	exp := ExpectedPrice(o, budget, b.Quality)
	if exp <= 0 {
		return RefuseRatio
	}
	return float32(price) / exp
}

// UseDoor is the service whose door a guest goes in by to use o: a seat
// in the lounge (or the food court without one), a meal at the food
// court, a drink at the bar (or the food court without one).
func (b *Building) UseDoor(o ai.Offer) Service {
	switch o {
	case ai.OfferSeat:
		if !b.Offers(ServiceLounge) {
			return ServiceFood
		}
		return ServiceLounge
	case ai.OfferMeal:
		return ServiceFood
	case ai.OfferDrink, ai.OfferWater:
		return b.DrinkService()
	case ai.OfferRentals:
		return ServiceRentals
	case ai.OfferApres:
		return ServiceBar
	case ai.OfferWarmUp:
		return ServiceLounge
	}
	return ServiceNone
}

// NeedUrgency is how urgent need k is for g, 0..1: how far the stat
// behind it has drained (rest takes the lower of energy and patience).
func (g *Guest) NeedUrgency(k ai.NeedKind) float32 {
	switch k {
	case ai.NeedHunger:
		return 1 - g.Hunger
	case ai.NeedThirst:
		return 1 - g.Thirst
	case ai.NeedRest:
		return 1 - min(g.Energy, g.Patience)
	case ai.NeedRentals:
		if g.NeedsGear {
			return 1
		}
	case ai.NeedApres:
		return g.Apres
	case ai.NeedWarmth:
		return g.Chill
	}
	return 0
}
