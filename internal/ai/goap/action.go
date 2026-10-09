package goap

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Action is one step in a plan. Preconditions read the snapshot and the
// live world (lift queues, building positions); effects mutate a *copy*
// of the snapshot the planner expands during search. Costs are positive
// reals summed by A*; per-agent multipliers stay in the same shape so
// goalWeight / actionCost are the only two surfaces a designer touches.
type Action interface {
	Name() string
	Precondition(s *WorldSnapshot, w *world.World) bool
	Apply(s *WorldSnapshot, w *world.World)
	Cost(s *WorldSnapshot, w *world.World) float32
}

// =============================================================================
// Tunables
// =============================================================================

const (
	// Cost-per-second baseline. Costs are in "seconds-equivalent" so the
	// planner can compare walking, queuing, riding, and skiing uniformly.
	walkSpeedMps = 0.67
	skiSpeedMps  = 10.0 // average descent speed for cost estimation; the
	// L1 controller ultimately decides actual speed

	// Lift-novelty bonus. First ride of a lift is "free"; each repeat ride
	// adds repeatPenaltyPerRide to RideLift's cost, capped so a much-ridden
	// lift never becomes prohibitive. Geometric decay would also work and
	// produces a softer falloff, but linear is easier to reason about and
	// the cap saturates quickly enough that the difference is academic.
	repeatPenaltyPerRide = 12.0
	repeatPenaltyCap     = 60.0

	// belowLevelPenaltySec makes a lift with no trail at the guest's own
	// level cost about four extra minutes, so they ride it only when
	// nothing at their level is running.
	belowLevelPenaltySec = 240.0

	// freeRoamTaste is how much an advanced guest has to like powder,
	// trees or steeps to leave the trails and ski the open mountain from
	// a lift top (freeRoams). Everyone else skis trails where there are
	// any.
	freeRoamTaste = 0.4

	// tasteMissSec is the most a lift's terrain can add to riding it for
	// not suiting the guest: its full amount when the trails off the top
	// are everything they dislike, none when they're everything they
	// love (Snow Tastes).
	tasteMissSec = 300.0

	// Minimum vertical drop for a SkiTo* action to be applicable. Below
	// this, the destination is effectively at the same elevation as the
	// lift top, and skiing-to-it is degenerate. Keeps the action graph
	// from generating "ski to the lift you just unloaded at."
	minDescentMeters = 20.0
)

// =============================================================================
// Action types
// =============================================================================

// WalkToLift moves the agent from a ground anchor (parking or lodge) to a
// lift base. Walking — used when there's no useful descent line to the
// lift base from the agent's current position.
type WalkToLift struct{ LiftID uint64 }

func (a *WalkToLift) Name() string {
	return fmt.Sprintf("WalkToLift(%d)", a.LiftID)
}

func (a *WalkToLift) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.OnLift != 0 || s.Queued != 0 || s.AtLiftBase != 0 || s.AtLiftTop != 0 {
		return false
	}
	l := findLift(w, a.LiftID)
	if l == nil {
		return false
	}
	// Beginners and intermediates won't walk to a lift with no trail at or
	// below their level (novices: with any terrain that isn't green).
	// Advanced+ are willing to free-roam from any lift.
	return liftAccessible(l, s.Skill, w)
}

func (a *WalkToLift) Apply(s *WorldSnapshot, w *world.World) {
	l := findLift(w, a.LiftID)
	if l == nil {
		return
	}
	s.Pos = mgl32.Vec3{l.Base[0], s.Pos[1], l.Base[1]}
	s.AtService = 0
	s.AtParking = 0
	s.AtTicketOffice = 0
	s.AtTrailEnd = 0
	s.AtLiftBase = l.ID
}

func (a *WalkToLift) Cost(s *WorldSnapshot, w *world.World) float32 {
	l := findLift(w, a.LiftID)
	if l == nil {
		return math.MaxFloat32
	}
	return distXZ(s.Pos, l.Base[0], l.Base[1]) / walkSpeedMps
}

// JoinQueue transitions the agent from AtLiftBase to Queued. Cost grows
// with queue length so the planner avoids long lines when alternatives
// exist
type JoinQueue struct{ LiftID uint64 }

func (a *JoinQueue) Name() string {
	return fmt.Sprintf("JoinQueue(%d)", a.LiftID)
}

func (a *JoinQueue) Precondition(s *WorldSnapshot, w *world.World) bool {
	l := findLift(w, a.LiftID)
	if !(!s.Removed && s.AtLiftBase == a.LiftID && l != nil && l.Open && !l.OnHold) {
		return false
	}
	if !liftAccessible(l, s.Skill, w) {
		return false
	}
	// Reject if the queue is too long, unless patience is already exhausted.
	// The exhausted exception keeps GoHome routing functional: a guest leaving
	// the mountain still needs to join a queue and ride up to exit a lift base.
	// No riding without a season pass or a day ticket from the window,
	// or without skis: a guest who came without rents first.
	if !hasTicket(s) || s.Need[ai.NeedRentals] > 0 {
		return false
	}
	// A line with a wait over world.MaxLineWait sends the planner to
	// another lift; the cap is bypassed when Patience < 0.05 so GoHome
	// routing can still ride up and exit a lift base.
	if s.Patience >= 0.05 && l.LineWait() > world.MaxLineWait {
		return false
	}
	// Reject if the guest can't afford this lift. Pass holders skip the budget
	// check — their rides are free. Budget-exhausted guests still need to ride
	// up to exit — same exception as the patience check above.
	if !s.HasSeasonPass && s.RemainingBudget > 0 && s.RemainingBudget < float32(l.RideFare()) {
		return false
	}
	return true
}

func (a *JoinQueue) Apply(s *WorldSnapshot, w *world.World) {
	s.AtLiftBase = 0
	s.Queued = a.LiftID
}

func (a *JoinQueue) Cost(s *WorldSnapshot, w *world.World) float32 {
	l := findLift(w, a.LiftID)
	if l == nil {
		return math.MaxFloat32
	}
	return l.LineWait()
}

// RideLift is folded board + ride + unload: the planner doesn't see the
// boarding step separately because BoardChair has no game-state effect
// beyond what RideLift's apply already captures. Effects: AtLiftTop set,
// Queued/OnLift cleared, RidenLifts incremented (which is the only
// novelty signal in the MVP).
type RideLift struct{ LiftID uint64 }

func (a *RideLift) Name() string {
	return fmt.Sprintf("RideLift(%d)", a.LiftID)
}

func (a *RideLift) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed {
		return false
	}
	return s.Queued == a.LiftID || s.OnLift == a.LiftID
}

func (a *RideLift) Apply(s *WorldSnapshot, w *world.World) {
	s.Queued = 0
	s.OnLift = 0
	s.AtLiftTop = a.LiftID
	s.RidenLifts = ai.AddRide(s.RidenLifts, a.LiftID)
}

func (a *RideLift) Cost(s *WorldSnapshot, w *world.World) float32 {
	l := findLift(w, a.LiftID)
	if l == nil {
		return math.MaxFloat32
	}
	ride := l.LoopLength() / (2 * l.Speed)
	// A lift with nothing at the guest's own level, off its top or
	// branching off those runs, is a fallback: they'll ride it when
	// nothing better runs, and wish for harder terrain.
	if !w.TerrainForLift(a.LiftID).Has(skillLevel(s.Skill)) {
		ride += belowLevelPenaltySec
	}
	// Terrain that suits the guest's tastes makes a lift the better ride.
	if c, ok := w.LiftConditions(a.LiftID); ok {
		// The line is how crowded the lift is.
		c[ai.TasteCrowds] = min(1, l.LineWait()/world.MaxLineWait)
		var m float32
		for k := range c {
			m += s.Tastes[k] * c[k]
		}
		m = max(-1, min(1, m))
		ride += tasteMissSec * (1 - m) / 2
	}
	// Repeat penalty: 0 for the first ride, ramps to repeatPenaltyCap.
	count := ai.RideCountOf(s.RidenLifts, a.LiftID)
	penalty := float32(count) * repeatPenaltyPerRide
	if penalty > repeatPenaltyCap {
		penalty = repeatPenaltyCap
	}
	return ride + penalty
}

// SkiToLift descends from a lift top to another lift's base. The base
// must be at least minDescentMeters below the source lift top — gravity-
// gated reachability stands in for explicit trails in the MVP.
type SkiToLift struct{ LiftID uint64 }

func (a *SkiToLift) Name() string {
	return fmt.Sprintf("SkiToLift(%d)", a.LiftID)
}

func (a *SkiToLift) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.AtLiftTop == 0 {
		return false
	}
	src := findLift(w, s.AtLiftTop)
	dst := findLift(w, a.LiftID)
	if src == nil || dst == nil {
		return false
	}
	return liftTopElev(w, src)-liftBaseElev(w, dst) >= minDescentMeters
}

func (a *SkiToLift) Apply(s *WorldSnapshot, w *world.World) {
	l := findLift(w, a.LiftID)
	if l == nil {
		return
	}
	s.AtLiftTop = 0
	s.AtService = 0
	s.AtParking = 0
	s.AtTicketOffice = 0
	s.AtTrailEnd = 0
	s.AtLiftBase = l.ID
	s.Pos = mgl32.Vec3{l.Base[0], s.Pos[1], l.Base[1]}
}

func (a *SkiToLift) Cost(s *WorldSnapshot, w *world.World) float32 {
	src := findLift(w, s.AtLiftTop)
	dst := findLift(w, a.LiftID)
	if src == nil || dst == nil {
		return math.MaxFloat32
	}
	return distXZ(mgl32.Vec3{src.Top[0], 0, src.Top[1]}, dst.Base[0], dst.Base[1]) / skiSpeedMps
}

// SkiToService descends from a lift top to a building that offers
// something to use (a seat, a meal, a drink). Used in plans that fulfil
// a need.
type SkiToService struct{ BldgID uint64 }

func (a *SkiToService) Name() string {
	return fmt.Sprintf("SkiToService(%d)", a.BldgID)
}

func (a *SkiToService) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.AtLiftTop == 0 {
		return false
	}
	src := findLift(w, s.AtLiftTop)
	dst := findBuilding(w, a.BldgID, world.BuildingLodge)
	if src == nil || dst == nil || !dst.OffersAnyUse() {
		return false
	}
	return liftTopElev(w, src)-buildingElev(w, dst) >= minDescentMeters
}

func (a *SkiToService) Apply(s *WorldSnapshot, w *world.World) {
	b := findBuilding(w, a.BldgID, world.BuildingLodge)
	if b == nil {
		return
	}
	s.AtLiftTop = 0
	s.AtTrailEnd = 0
	setAtBuilding(s, b)
	s.Pos = mgl32.Vec3{b.Pos[0], s.Pos[1], b.Pos[1]}
}

func (a *SkiToService) Cost(s *WorldSnapshot, w *world.World) float32 {
	src := findLift(w, s.AtLiftTop)
	dst := findBuilding(w, a.BldgID, world.BuildingLodge)
	if src == nil || dst == nil {
		return math.MaxFloat32
	}
	return distXZ(mgl32.Vec3{src.Top[0], 0, src.Top[1]}, dst.Pos[0], dst.Pos[1]) / skiSpeedMps
}

// SkiToParking descends from a lift top to a parking lot. The terminal
// SkiTo* used by GoHome plans before Depart.
type SkiToParking struct{ LotID uint64 }

func (a *SkiToParking) Name() string {
	return fmt.Sprintf("SkiToParking(%d)", a.LotID)
}

func (a *SkiToParking) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.AtLiftTop == 0 || (s.CarLot != 0 && a.LotID != s.CarLot) {
		return false
	}
	src := findLift(w, s.AtLiftTop)
	dst := findBuilding(w, a.LotID, world.BuildingParking)
	if src == nil || dst == nil {
		return false
	}
	return liftTopElev(w, src)-buildingElev(w, dst) >= minDescentMeters
}

func (a *SkiToParking) Apply(s *WorldSnapshot, w *world.World) {
	b := findBuilding(w, a.LotID, world.BuildingParking)
	if b == nil {
		return
	}
	s.AtLiftTop = 0
	s.AtService = 0
	s.AtTicketOffice = 0
	s.AtTrailEnd = 0
	s.AtParking = b.ID
	s.Pos = mgl32.Vec3{b.Pos[0], s.Pos[1], b.Pos[1]}
}

func (a *SkiToParking) Cost(s *WorldSnapshot, w *world.World) float32 {
	src := findLift(w, s.AtLiftTop)
	dst := findBuilding(w, a.LotID, world.BuildingParking)
	if src == nil || dst == nil {
		return math.MaxFloat32
	}
	return distXZ(mgl32.Vec3{src.Top[0], 0, src.Top[1]}, dst.Pos[0], dst.Pos[1]) / skiSpeedMps
}

// WalkToParking walks from the base area to a parking lot: after a
// trail down to a lift base, the way home. Ends at the lot like
// SkiToParking.
type WalkToParking struct{ LotID uint64 }

func (a *WalkToParking) Name() string {
	return fmt.Sprintf("WalkToParking(%d)", a.LotID)
}

func (a *WalkToParking) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.OnLift != 0 || s.Queued != 0 || s.AtLiftTop != 0 || s.AtParking != 0 ||
		(s.CarLot != 0 && a.LotID != s.CarLot) {
		return false
	}
	return findBuilding(w, a.LotID, world.BuildingParking) != nil
}

func (a *WalkToParking) Apply(s *WorldSnapshot, w *world.World) {
	b := findBuilding(w, a.LotID, world.BuildingParking)
	if b == nil {
		return
	}
	s.AtLiftBase = 0
	s.AtTrailEnd = 0
	s.AtService = 0
	s.AtTicketOffice = 0
	s.AtParking = b.ID
	s.Pos = mgl32.Vec3{b.Pos[0], s.Pos[1], b.Pos[1]}
}

func (a *WalkToParking) Cost(s *WorldSnapshot, w *world.World) float32 {
	b := findBuilding(w, a.LotID, world.BuildingParking)
	if b == nil {
		return math.MaxFloat32
	}
	return distXZ(s.Pos, b.Pos[0], b.Pos[1]) / walkSpeedMps
}

// freeRoams reports whether the guest would rather ski the open mountain
// than the trails: advanced, and keen on powder, trees or steeps.
// Beginners and intermediates never do, so a run beyond them is never
// the planner's doing.
func freeRoams(s *WorldSnapshot) bool {
	return FreeRoams(s.Skill, s.Tastes)
}

// FreeRoams is freeRoams for a guest's skill and tastes.
func FreeRoams(skill float32, tastes ai.Tastes) bool {
	if skill < ai.SkillAdvancedThreshold {
		return false
	}
	return max(tastes[ai.TastePowder], tastes[ai.TasteTrees], tastes[ai.TasteSteep]) >= freeRoamTaste
}

// TrailLevels is the trail difficulties a guest keeps to on the snow:
// their level and below for beginners and intermediates, any trail for
// an advanced guest, and 0 (anywhere) for one who free-roams.
func TrailLevels(skill float32, tastes ai.Tastes) world.TerrainDifficulty {
	switch {
	case FreeRoams(skill, tastes):
		return 0
	case skill >= ai.SkillAdvancedThreshold:
		return world.DiffGreen | world.DiffBlue | world.DiffBlack
	}
	return skillDiff(skill)
}

// WalkToTicketOffice moves the agent from any ground position to a ticket
// office. Applicable when the guest still needs a day ticket, or holds one
// and has enough budget left to upgrade to a season pass.
type WalkToTicketOffice struct{ OfficeID uint64 }

func (a *WalkToTicketOffice) Name() string {
	return fmt.Sprintf("WalkToTicketOffice(%d)", a.OfficeID)
}

func (a *WalkToTicketOffice) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.OnLift != 0 || s.Queued != 0 || s.AtLiftBase != 0 || s.AtLiftTop != 0 {
		return false
	}
	if s.HasSeasonPass {
		return false
	}
	if s.HasDayTicket && s.RemainingBudget < passCost(s, w)+passReserve {
		return false
	}
	b := findBuilding(w, a.OfficeID, world.BuildingLodge)
	return b != nil && b.Offers(world.ServiceTickets)
}

func (a *WalkToTicketOffice) Apply(s *WorldSnapshot, w *world.World) {
	b := findBuilding(w, a.OfficeID, world.BuildingLodge)
	if b == nil {
		return
	}
	s.AtTrailEnd = 0
	setAtBuilding(s, b)
	p, _ := b.NearestServiceEntrance(world.ServiceTickets, mgl32.Vec2{s.Pos[0], s.Pos[2]})
	s.Pos = mgl32.Vec3{p[0], s.Pos[1], p[1]}
}

func (a *WalkToTicketOffice) Cost(s *WorldSnapshot, w *world.World) float32 {
	b := findBuilding(w, a.OfficeID, world.BuildingLodge)
	if b == nil {
		return math.MaxFloat32
	}
	return distXZ(s.Pos, b.Pos[0], b.Pos[1]) / walkSpeedMps
}

// WalkToService walks from the base area (a lot, a building, the foot
// of a lift) to a building that offers something to use: rentals on
// arrival, après after the lifts close, or lunch between laps.
type WalkToService struct{ BldgID uint64 }

func (a *WalkToService) Name() string {
	return fmt.Sprintf("WalkToService(%d)", a.BldgID)
}

func (a *WalkToService) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.OnLift != 0 || s.Queued != 0 || s.AtLiftTop != 0 || s.AtService == a.BldgID {
		return false
	}
	b := findBuilding(w, a.BldgID, world.BuildingLodge)
	return b != nil && b.OffersAnyUse()
}

func (a *WalkToService) Apply(s *WorldSnapshot, w *world.World) {
	b := findBuilding(w, a.BldgID, world.BuildingLodge)
	if b == nil {
		return
	}
	s.AtLiftBase = 0
	s.AtTrailEnd = 0
	setAtBuilding(s, b)
	s.Pos = mgl32.Vec3{b.Pos[0], s.Pos[1], b.Pos[1]}
}

func (a *WalkToService) Cost(s *WorldSnapshot, w *world.World) float32 {
	b := findBuilding(w, a.BldgID, world.BuildingLodge)
	if b == nil {
		return math.MaxFloat32
	}
	return distXZ(s.Pos, b.Pos[0], b.Pos[1]) / walkSpeedMps
}

// BuyDayTicket is an atomic action executed at a ticket office: the guest
// pays the day ticket priced at arrival. RemainingBudget already excludes
// it, so only the ticket flag changes here; the simulation moves the cash
// when the step starts.
type BuyDayTicket struct{ OfficeID uint64 }

func (a *BuyDayTicket) Name() string {
	return fmt.Sprintf("BuyDayTicket(%d)", a.OfficeID)
}

func (a *BuyDayTicket) Precondition(s *WorldSnapshot, w *world.World) bool {
	return !s.Removed && s.AtTicketOffice == a.OfficeID && !hasTicket(s)
}

func (a *BuyDayTicket) Apply(s *WorldSnapshot, w *world.World) {
	s.HasDayTicket = true
	s.AtTicketOffice = 0
}

func (a *BuyDayTicket) Cost(s *WorldSnapshot, w *world.World) float32 {
	return 5.0 // brief transaction, same as BuySeasonPass
}

// BuySeasonPass is an atomic action executed at a ticket office. The guest
// pays the season pass fee and receives free lift access for the rest of the
// season. The actual SimTime expiry and cash transfer are applied by the
// simulation when the step starts.
type BuySeasonPass struct{ OfficeID uint64 }

func (a *BuySeasonPass) Name() string {
	return fmt.Sprintf("BuySeasonPass(%d)", a.OfficeID)
}

func (a *BuySeasonPass) Precondition(s *WorldSnapshot, w *world.World) bool {
	return !s.Removed && s.AtTicketOffice == a.OfficeID && !s.HasSeasonPass &&
		s.RemainingBudget >= passCost(s, w)+passReserve
}

// passReserve is what a guest keeps back for the day's lunch and drinks
// before buying a season pass, so a pass doesn't leave them unable to
// buy a drink.
const passReserve = float32(world.DefaultMealPrice + 2*world.DefaultDrinkPrice)

func (a *BuySeasonPass) Apply(s *WorldSnapshot, w *world.World) {
	s.RemainingBudget -= passCost(s, w)
	s.PassCredit = 0
	s.HasSeasonPass = true
	s.AtTicketOffice = 0
}

func (a *BuySeasonPass) Cost(s *WorldSnapshot, w *world.World) float32 {
	return 5.0 // brief transaction; planner sees minimal queue cost
}

// UseService uses something a building offers (a seat, a meal, a
// drink): it fulfils the needs that offer meets (world.OfferNeeds) and
// costs its price, its time, and the expected wait at the door. A guest
// won't join a line longer than the place holds, or pay far more than
// they expect (world.RefuseRatio).
type UseService struct {
	BldgID uint64
	Use    ai.Offer
}

func (a *UseService) Name() string {
	return fmt.Sprintf("Use%s(%d)", ai.OfferLabels[a.Use], a.BldgID)
}

func (a *UseService) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed || s.AtService != a.BldgID {
		return false
	}
	b := findBuilding(w, a.BldgID, world.BuildingLodge)
	if b == nil || !b.OffersUse(a.Use) || !b.LineOpen(a.Use) {
		return false
	}
	return affordable(b, a.Use, s)
}

func (a *UseService) Apply(s *WorldSnapshot, w *world.World) {
	needs := world.OfferNeeds(a.Use)
	for k := ai.NeedKind(0); k < ai.NeedCount; k++ {
		if needs.Has(k) {
			s.Need[k] = 0
		}
	}
	if needs.Has(ai.NeedRest) {
		// The skiing goals read these directly.
		s.Patience, s.Energy = 1, 1
	}
	if b := findBuilding(w, a.BldgID, world.BuildingLodge); b != nil {
		s.RemainingBudget -= float32(b.UsePrice(a.Use))
	}
}

func (a *UseService) Cost(s *WorldSnapshot, w *world.World) float32 {
	c := world.OfferDuration(a.Use)
	if a.Use == ai.OfferWater {
		c += waterPenalty
	}
	if b := findBuilding(w, a.BldgID, world.BuildingLodge); b != nil {
		c += b.ExpectedWait(a.Use)
	}
	return c
}

// waterPenalty makes free water dearer to plan than a drink (a drink
// takes ten clock minutes, water one), so a guest who'll buy a drink
// buys one unless the counter's line is long, and water is for the rest.
const waterPenalty = world.SimSecondsPerHour / 6

// affordable reports whether the guest will pay for o at b: free, or
// within what they have left and under world.RefuseRatio of what they
// expect it to cost.
func affordable(b *world.Building, o ai.Offer, s *WorldSnapshot) bool {
	price := b.UsePrice(o)
	return price == 0 || (s.RemainingBudget >= float32(price) && b.PriceRatio(o, s.Budget) < world.RefuseRatio)
}

// pricedOut reports whether need k could be met somewhere but every
// place that offers it charges more than the guest will pay
// (world.RefuseRatio): the planner's reason for "everything here costs
// too much".
func pricedOut(s *WorldSnapshot, w *world.World, k ai.NeedKind) bool {
	offered := false
	for _, b := range w.Buildings {
		if b.Type != world.BuildingLodge {
			continue
		}
		for o := ai.OfferNone + 1; o < ai.OfferCount; o++ {
			if !world.OfferNeeds(o).Has(k) || !b.OffersUse(o) {
				continue
			}
			offered = true
			if b.UsePrice(o) == 0 || b.PriceRatio(o, s.Budget) < world.RefuseRatio {
				return false
			}
		}
	}
	return offered
}

// Depart is the terminal action that removes the agent from the sim.
// Precondition is AtParking; effect is Removed = true.
type Depart struct{ LotID uint64 }

func (a *Depart) Name() string {
	return fmt.Sprintf("Depart(%d)", a.LotID)
}

func (a *Depart) Precondition(s *WorldSnapshot, w *world.World) bool {
	return !s.Removed && s.AtParking == a.LotID && (s.CarLot == 0 || s.CarLot == a.LotID)
}

func (a *Depart) Apply(s *WorldSnapshot, w *world.World) {
	s.Removed = true
}

func (a *Depart) Cost(s *WorldSnapshot, w *world.World) float32 {
	return 0
}

// =============================================================================
// Action enumeration
// =============================================================================

// ApplicableActions enumerates every action whose precondition holds at
// snapshot s. The planner expands the current frontier by calling this
// and applying each result's Apply to a Cloned snapshot. Action count is
// O(lifts + lodges + parking + trail edges) — fine for small resort scales.
func ApplicableActions(s *WorldSnapshot, w *world.World) []Action {
	out := make([]Action, 0, 8)
	// Walk to any lift base from a ground anchor.
	if !s.Removed && s.OnLift == 0 && s.Queued == 0 && s.AtLiftBase == 0 && s.AtLiftTop == 0 {
		for _, l := range w.Lifts {
			a := &WalkToLift{LiftID: l.ID}
			if a.Precondition(s, w) {
				out = append(out, a)
			}
		}
	}
	// Queue / ride at the lift base, top, or while queued/on-lift.
	if s.AtLiftBase != 0 {
		jq := &JoinQueue{LiftID: s.AtLiftBase}
		if jq.Precondition(s, w) {
			out = append(out, jq)
		}
	}
	if s.Queued != 0 {
		out = append(out, &RideLift{LiftID: s.Queued})
	}
	if s.OnLift != 0 {
		out = append(out, &RideLift{LiftID: s.OnLift})
	}

	// Trail-based descents from any anchor that has trail edges.
	out = trailActions(out, s, w)

	// Free-roam ski-down from a lift top: only for guests who'd rather be
	// off the trails, or from a top with no trail down at all.
	if s.AtLiftTop != 0 && (freeRoams(s) || len(w.TrailGraph.EdgesFrom(s.AtLiftTop)) == 0) {
		for _, l := range w.Lifts {
			a := &SkiToLift{LiftID: l.ID}
			if a.Precondition(s, w) {
				out = append(out, a)
			}
		}
		for _, b := range w.Buildings {
			switch b.Type {
			case world.BuildingLodge:
				if a := (&SkiToService{BldgID: b.ID}); a.Precondition(s, w) {
					out = append(out, a)
				}
			case world.BuildingParking:
				a := &SkiToParking{LotID: b.ID}
				if a.Precondition(s, w) {
					out = append(out, a)
				}
			}
		}
	}

	// At a service building — use what it offers; at parking — depart.
	if s.AtService != 0 {
		for o := ai.OfferNone + 1; o < ai.OfferCount; o++ {
			if a := (&UseService{BldgID: s.AtService, Use: o}); a.Precondition(s, w) {
				out = append(out, a)
			}
		}
	}
	if s.AtParking != 0 {
		a := &Depart{LotID: s.AtParking}
		if a.Precondition(s, w) {
			out = append(out, a)
		}
	}
	// Walk to the car from the base area: how a guest who skied a trail
	// down to a lift base gets home.
	if !s.Removed && s.OnLift == 0 && s.Queued == 0 && s.AtLiftTop == 0 && s.AtParking == 0 {
		for _, b := range w.Buildings {
			if b.Type != world.BuildingParking {
				continue
			}
			if a := (&WalkToParking{LotID: b.ID}); a.Precondition(s, w) {
				out = append(out, a)
			}
		}
	}
	// Walk to a service building from the base area.
	if !s.Removed && s.OnLift == 0 && s.Queued == 0 && s.AtLiftTop == 0 {
		for _, b := range w.Buildings {
			if b.Type != world.BuildingLodge {
				continue
			}
			if a := (&WalkToService{BldgID: b.ID}); a.Precondition(s, w) {
				out = append(out, a)
			}
		}
	}
	// Walk to ticket office from any ground position (not on a lift or in a queue).
	if !s.Removed && !s.HasSeasonPass && s.OnLift == 0 && s.Queued == 0 && s.AtLiftBase == 0 && s.AtLiftTop == 0 {
		for _, b := range w.Buildings {
			if !b.Offers(world.ServiceTickets) {
				continue
			}
			a := &WalkToTicketOffice{OfficeID: b.ID}
			if a.Precondition(s, w) {
				out = append(out, a)
			}
		}
	}
	// Buy a day ticket or a pass when already at the ticket office.
	if s.AtTicketOffice != 0 {
		if a := (&BuyDayTicket{OfficeID: s.AtTicketOffice}); a.Precondition(s, w) {
			out = append(out, a)
		}
		if a := (&BuySeasonPass{OfficeID: s.AtTicketOffice}); a.Precondition(s, w) {
			out = append(out, a)
		}
	}
	return out
}

// =============================================================================
// Storage handoff — translate concrete Actions to plain-data ai.PlanAction
// =============================================================================

// ToPlanActions translates a planner-emitted Action slice into the leaf
// ai package's PlanAction records so the result can live on
// world.Guest.Plan without forcing world to import goap. Walks the plan
// applying each step to a snapshot copy so the per-step Cost reflects
// the state it was costed against during search.
func ToPlanActions(actions []Action, snap WorldSnapshot, w *world.World) []ai.PlanAction {
	if len(actions) == 0 {
		return nil
	}
	out := make([]ai.PlanAction, 0, len(actions))
	step := snap.Clone()
	for _, a := range actions {
		pa := ai.PlanAction{Cost: a.Cost(&step, w)}
		switch t := a.(type) {
		case *WalkToLift:
			pa.Kind = ai.ActWalkToLift
			pa.LiftID = t.LiftID
		case *JoinQueue:
			pa.Kind = ai.ActJoinQueue
			pa.LiftID = t.LiftID
		case *RideLift:
			pa.Kind = ai.ActRideLift
			pa.LiftID = t.LiftID
		case *SkiToLift:
			pa.Kind = ai.ActSkiToLift
			pa.LiftID = t.LiftID
		case *SkiToService:
			pa.Kind = ai.ActSkiToService
			pa.BldgID = t.BldgID
		case *WalkToService:
			pa.Kind = ai.ActWalkToService
			pa.BldgID = t.BldgID
		case *SkiToParking:
			pa.Kind = ai.ActSkiToParking
			pa.BldgID = t.LotID
		case *WalkToParking:
			pa.Kind = ai.ActWalkToParking
			pa.BldgID = t.LotID
		case *UseService:
			pa.Kind = ai.ActUseService
			pa.BldgID = t.BldgID
			pa.Use = t.Use
		case *Depart:
			pa.Kind = ai.ActDepart
			pa.BldgID = t.LotID
		case *WalkToTicketOffice:
			pa.Kind = ai.ActWalkToTicketOffice
			pa.BldgID = t.OfficeID
		case *BuyDayTicket:
			pa.Kind = ai.ActBuyDayTicket
			pa.BldgID = t.OfficeID
		case *BuySeasonPass:
			pa.Kind = ai.ActBuySeasonPass
			pa.BldgID = t.OfficeID
		case *SkiTrail:
			pa.Kind = ai.ActSkiTrail
			pa.TrailID = t.TrailID // via trail (display); overridden for trail-to-trail
			pa.Via = t.TrailID
			switch t.ToKind {
			case world.KindLiftBase:
				pa.LiftID = t.ToID
			case world.KindBuilding:
				pa.BldgID = t.ToID
			case world.KindTrail:
				pa.TrailID = t.ToID // destination trail — used by planActionComplete
			}
		}
		out = append(out, pa)
		a.Apply(&step, w)
	}
	return out
}

// PlanActionLabel renders an ai.PlanAction with the same name + entity
// label formatting goap.DisplayName uses for concrete Actions. Used by
// the HUD which now reads plan steps from the agent rather than from
// fresh planner output.
func PlanActionLabel(pa ai.PlanAction, w *world.World) string {
	switch pa.Kind {
	case ai.ActWalkToLift:
		return "WalkToLift(" + liftLabel(w, pa.LiftID) + ")"
	case ai.ActJoinQueue:
		return "JoinQueue(" + liftLabel(w, pa.LiftID) + ")"
	case ai.ActRideLift:
		return "RideLift(" + liftLabel(w, pa.LiftID) + ")"
	case ai.ActSkiToLift:
		return "SkiToLift(" + liftLabel(w, pa.LiftID) + ")"
	case ai.ActSkiToService:
		return "SkiToService(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActWalkToService:
		return "WalkToService(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActSkiToParking:
		return "SkiToParking(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActWalkToParking:
		return "WalkToParking(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActUseService:
		return ai.OfferLabels[pa.Use] + "(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActDepart:
		return "Depart(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActWalkToTicketOffice:
		return "WalkToTicketOffice(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActBuyDayTicket:
		return "BuyDayTicket(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActBuySeasonPass:
		return "BuySeasonPass(" + buildingLabel(w, pa.BldgID) + ")"
	case ai.ActSkiTrail:
		return skiTrailPlanActionLabel(pa, w)
	}
	return "—"
}

// =============================================================================
// Display
// =============================================================================

// DisplayName returns a human-readable form of an action's Name() with
// entity IDs swapped for labels: lift names where set (PlaceLift auto-
// assigns Lift1, Lift2, ...), building positions otherwise. The HUD
// uses this; Name() stays raw so logs and recorder traces remain stable
// even when lifts get renamed mid-session.
func DisplayName(a Action, w *world.World) string {
	switch act := a.(type) {
	case *WalkToLift:
		return "WalkToLift(" + liftLabel(w, act.LiftID) + ")"
	case *JoinQueue:
		return "JoinQueue(" + liftLabel(w, act.LiftID) + ")"
	case *RideLift:
		return "RideLift(" + liftLabel(w, act.LiftID) + ")"
	case *SkiToLift:
		return "SkiToLift(" + liftLabel(w, act.LiftID) + ")"
	case *SkiToService:
		return "SkiToService(" + buildingLabel(w, act.BldgID) + ")"
	case *WalkToService:
		return "WalkToService(" + buildingLabel(w, act.BldgID) + ")"
	case *SkiToParking:
		return "SkiToParking(" + buildingLabel(w, act.LotID) + ")"
	case *UseService:
		return ai.OfferLabels[act.Use] + "(" + buildingLabel(w, act.BldgID) + ")"
	case *Depart:
		return "Depart(" + buildingLabel(w, act.LotID) + ")"
	case *WalkToTicketOffice:
		return "WalkToTicketOffice(" + buildingLabel(w, act.OfficeID) + ")"
	case *BuyDayTicket:
		return "BuyDayTicket(" + buildingLabel(w, act.OfficeID) + ")"
	case *BuySeasonPass:
		return "BuySeasonPass(" + buildingLabel(w, act.OfficeID) + ")"
	case *SkiTrail:
		return skiTrailDisplayName(act, w)
	}
	return a.Name()
}

func liftLabel(w *world.World, id uint64) string {
	if l := findLift(w, id); l != nil && l.Name != "" {
		return l.Name
	}
	return fmt.Sprintf("#%d", id)
}

func buildingLabel(w *world.World, id uint64) string {
	for _, b := range w.Buildings {
		if b.ID != id {
			continue
		}
		// Lodges and parking lots don't have names yet; identify by type
		// + ID so the HUD reads "Lodge #4" / "Parking #2".
		switch b.Type {
		case world.BuildingLodge:
			return fmt.Sprintf("Lodge#%d", id)
		case world.BuildingParking:
			return fmt.Sprintf("Lot#%d", id)
		}
	}
	return fmt.Sprintf("#%d", id)
}

// =============================================================================
// Helpers
// =============================================================================

func findLift(w *world.World, id uint64) *world.Lift {
	for _, l := range w.Lifts {
		if l.ID == id {
			return l
		}
	}
	return nil
}

func findBuilding(w *world.World, id uint64, typ world.BuildingType) *world.Building {
	for _, b := range w.Buildings {
		if b.ID == id && b.Type == typ {
			return b
		}
	}
	return nil
}

func liftTopElev(w *world.World, l *world.Lift) float32 {
	return w.Terrain.InterpolatedSurfaceElevationAt(l.Top[0], l.Top[1])
}

func liftBaseElev(w *world.World, l *world.Lift) float32 {
	return w.Terrain.InterpolatedSurfaceElevationAt(l.Base[0], l.Base[1])
}

func buildingElev(w *world.World, b *world.Building) float32 {
	return w.Terrain.InterpolatedSurfaceElevationAt(b.Pos[0], b.Pos[1])
}

func distXZ(p mgl32.Vec3, x, z float32) float32 {
	dx := p[0] - x
	dz := p[2] - z
	return float32(math.Sqrt(float64(dx*dx + dz*dz)))
}
