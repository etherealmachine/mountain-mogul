package world

import (
	"fmt"
	"math"
	"time"

	"github.com/go-gl/mathgl/mgl32"
)

// Cost constants (VISION §7 magnitudes, dollars).
//
// Calibration target — a Season 1 pod: one ~1 km fixed quad with its
// trails, plus parking, ticket office, lodge, shed (first cat), and a
// patrol hut. At the VISION S1 attendance (~170 guests/day averaged over
// weekdays and weekends) and the $60 day ticket, a 186-day season grosses
// ~$1.9M. Daily opex for that resort (see DailyOperatingCost) is ~$3.8k,
// ~$0.71M a season ≈ 37% of gross, so the pod ($900k) nets back roughly
// its own build cost in one season. Lift build prices are per LiftType
// (LiftType.StationCost / PerMeterCost); a 2.5 km gondola (~$10M) is ~6×
// a typical 700 m high-speed quad (~$1.64M).
const (
	LodgeCost    = 150_000 // single fixed cost per lodge (VISION: shell $100k + per cell; flat until shells land)
	ParkingCost  = 150_000 // base parking lot
	StartingCash = 1_000_000

	// Credit line (VISION §7). Cash may go negative down to −CreditLimit;
	// the negative balance is the drawn amount. Interest accrues daily on
	// the drawn amount and is charged to Cash when the calendar month turns.
	// Cash below −CreditLimit for BankruptcyGraceDays consecutive day
	// rollovers is bankruptcy.
	DefaultCreditLimit  = 1_000_000 // dollars
	DefaultCreditRate   = 0.12      // a year, on 365 calendar days; simple interest, accrued daily
	BankruptcyGraceDays = 30        // consecutive day rollovers below the credit floor

	DefaultParcelPrice = 250_000 // dollars; editor default for a newly painted purchasable parcel

	GladeCostPerTree = 100 // per tree the glade brush removes; a full cell (MaxTreesPerCell) costs $200

	DefaultTicketPrice    = 10 // dollars per heli ride; only heli charges per ride, player adjusts via the lift popup
	DefaultDayTicketPrice = 60 // dollars per visit, paid once at the ticket window (VISION §7: $60–90); pass holders pay nothing

	// Day-ticket price elasticity in the demand poll. A guest's price factor
	// is 1 at or below the reference price and falls to 0 as the price rises
	// to their DailyBudget: ((budget − price) / (budget − ref))^elasticity.
	// The reference scales with resort rating, ref × (1 + premium × (rating − 0.5)),
	// so a well-rated resort can charge more before guests balk.
	DayTicketReferencePrice = 60           // dollars; the no-penalty price at rating 0.5
	DayTicketRatingPremium  = float32(1.0) // unitless; reference spans 0.5×–1.5× across rating 0..1
	DayTicketElasticity     = float32(0.5) // unitless exponent; <1 bows the curve so guests hold on until price nears budget

	// ParkingReferencePrice is the per-car fee guests accept without
	// complaint; each guest's share (÷ MeanCarload) adds to the day
	// ticket in the demand poll, on both the price and the reference.
	ParkingReferencePrice = 20 // dollars per car

	TicketOfficeCost       = 80_000
	DefaultSeasonPassPrice = 150                          // one-time fee per guest per season; guests with sufficient budget buy it on arrival
	HelipadCost            = 2_000_000                    // flat cost for a heli-ski operation (two pads + helicopter); the post-gondola unlock
	SnowGunCost            = 40_000                       // snowmaking cannon; operating cost below
	SnowGunActiveCostDay   = 400                          // dollars per game-day while enabled (water + power)
	SnowGunRangeCells      = 3                            // spray radius in terrain cells
	SnowGunRangeM          = SnowGunRangeCells * CellSize // = 15 metres
	SnowGunMinTempC        = float32(-2.0)                // air must be at or below this to make snow

	// Daily operational costs, dollars per in-game day, charged at rollover
	// (see DailyOperatingCost). Lift running costs are per LiftType
	// (LiftType.RunningCostDay) on top of the staff (Lift.Headcount).
	LiftStaffDailyCost = 250 // per lift worker: the two operators, and any line attendant

	// Standby (closed-resort) costs, dollars per in-game day. What the
	// resort pays while nothing is open: lifts idle with no attendants,
	// cats parked (CatStandbyCostDay), buildings heated but unstaffed.
	// See DailyStandbyCost. Charged at rollover on days the resort was
	// closed all day (World.ResortOpen).
	LiftStandbyCostDay     = 100 // per lift: inspections, idle power
	BuildingStandbyCostDay = 40  // per service building
	// Snowcat daily costs live in world/snowcat.go (CatActiveCostDay, CatStandbyCostDay).
)

// BuildingCost returns the up-front cost of placing a building of the
// given type.
func BuildingCost(t BuildingType) int {
	switch t {
	case BuildingParking:
		return ParkingCost
	case BuildingSnowGun:
		return SnowGunCost
	case BuildingTicketOffice:
		return TicketOfficeCost
	}
	return LodgeCost
}

// The clock runs 4× faster than guest movement: one clock minute is 15
// sim seconds, so a 9:00–16:00 ski day is 6300 sim s: a 2.5-minute chair
// ride is 10 clock minutes, and a guest gets a dozen or more runs. Day and
// night run at the same rate. Movement and other physical durations stay
// in sim seconds; anything meant as clock time (needs, waits, meals) is
// written in SimSecondsPerHour. It was 180 (20×) until 2026-10-07, which
// left guests two or three runs a day and lifts carrying a fifth of what
// they should; saves rescale on load.
const (
	SimSecondsPerHour = 900.0
	SecondsPerSimDay  = 24 * SimSecondsPerHour

	// LegacySecondsPerSimDay is the day length of saves written before the
	// clock had hours; save load rescales their clocks.
	LegacySecondsPerSimDay = 240.0

	// NewGameStartHour is the clock hour a fresh game or scenario starts
	// at, so the first thing the player sees is the morning.
	NewGameStartHour = 8.0
)

// Default lift operating hours (clock hours, local solar time).
const (
	DefaultOpenHour  = 9.0
	DefaultCloseHour = 16.0
)

// DefaultStartDate is the calendar date SimTime 0 maps to in a world that
// doesn't set its own: Nov 25, 2026, opening day of the 2026-27 season.
// Scenarios set World.StartDate in the editor.
var DefaultStartDate = time.Date(2026, time.November, 25, 0, 0, 0, 0, time.UTC)

// World owns all simulation state.
type World struct {
	Terrain   *Terrain
	Objects   []*PlacedObject
	Buildings []*Building
	Lifts     []*Lift
	Trails    []*Trail
	// Footpaths are the walkways (footpath.go); FootpathsRev counts
	// changes to them, for the renderer, and footpaths is their index.
	Footpaths    []*Footpath
	FootpathsRev int
	footpaths    footpathIndex

	// TrailGraph is the derived connectivity graph built from trail cell data.
	// Rebuilt by RebuildTrailGraph whenever trails are added, removed, or edited.
	// Nil until the first trail is placed.
	TrailGraph *TrailGraph
	// TrailVersion counts trail-graph rebuilds, so drawn trails know
	// when to redraw.
	TrailVersion uint64

	// trailAt maps each terrain cell (z*Width+x) to 1 + its index in
	// Trails, 0 for none; where trails overlap, the hardest wins.
	// Derived, rebuilt with TrailGraph; read through TrailAt.
	trailAt []uint16
	// trailDiffs is each cell's difficulties: every trail on it, so a
	// green under a blue still counts for a beginner. Read through
	// TrailDiffsAt.
	trailDiffs []TerrainDifficulty
	// trailLines caches each run's centre line (TrailLine).
	trailLines map[uint64]*TrailLine

	// Guests is the master catchment — every potential visitor the resort
	// could ever attract, ~10k entries seeded at world init. Identity +
	// career stats live here forever; the slice header never shrinks. The
	// demand poll walks this slice once per day deciding who shows up.
	Guests []*Guest

	// OnMountain is the hot-path active subset of Guests — the pointers
	// the sim ticks every frame. A guest appears here from arrival until
	// reapDeparted splices them out and flips State back to AtHome (the
	// Guest itself stays in Guests).
	OnMountain []*Guest

	Snowcats []*Snowcat
	// Snowmobiles are the patrol snowmobiles, each housed in a garage.
	Snowmobiles []*Snowmobile
	Patrollers  []*Patroller
	RoadNodes   []*RoadNode
	RoadEdges   []*RoadEdge
	// Cars are the carloads of guests on the roads and in the lots
	// (sim/traffic.go).
	Cars   []*Car
	nextID uint64

	// History is a daily ring of stats (guest count, cash, arrivals,
	// departures) feeding the in-game charts window. Nil on a freshly
	// constructed world; the scenario load path / new-game path
	// allocates an empty one so the sim immediately begins recording.
	History *History

	// Events is the bounded feed of notable happenings (avalanches,
	// rescues, lift status changes, builds, day summaries) shown in the
	// UI event panel. Zero value is an empty log.
	Events EventLog

	// Cash is the resort's bank balance in dollars. PlaceBuilding /
	// PlaceLift deduct from this and refuse the placement when the
	// balance can't cover the cost.
	Cash int

	// CreditLimit is the size of the credit line in dollars. Cash may be
	// spent down to −CreditLimit (the credit floor); see CanAfford.
	CreditLimit int

	// CreditRate is the credit line's yearly interest rate (0.12 is 12%).
	CreditRate float32

	// AccruedInterest is interest in dollars accrued on the drawn balance
	// since the last monthly charge. Fractional so small daily amounts
	// don't round away; rounded when charged to Cash.
	AccruedInterest float64

	// DaysBelowFloor counts consecutive day rollovers that ended with Cash
	// below −CreditLimit. Resets to 0 on any rollover at or above the floor.
	DaysBelowFloor int

	// Bankrupt is set once DaysBelowFloor reaches BankruptcyGraceDays and
	// never cleared by the sim. The UI reads it for the game-over screen.
	Bankrupt bool

	// ResortOpen is the player's resort-wide open/closed switch, set from
	// any ticket office popup. Closed: no arrivals, standby costs, lifts
	// load only guests heading home. Snowcats and snow guns run either
	// way. New games and scenarios start closed; testbeds start open.
	ResortOpen bool

	// ClosedForDay mirrors sim.ClosedForDay for the planner: the mountain
	// is done for the day, so guests on it should head home. Set by the
	// sim every tick; not saved.
	ClosedForDay bool

	// OpenHour and CloseHour are the daily lift operating hours in clock
	// hours (9.5 = 9:30). While the resort is open, lifts load, guests
	// arrive and snowcats stay in the shed only between them.
	OpenHour, CloseHour float32

	// SeasonPassPrice is the one-time fee guests pay at the ticket office for
	// a season pass. Pass holders ride any lift for free for the remainder of
	// the season. Defaults to DefaultSeasonPassPrice; the player can adjust it
	// via the ticket office popup (future).
	SeasonPassPrice int

	// DayTicketPrice is the per-visit fee in dollars. It is fixed when a
	// guest arrives and paid when they reach a ticket office; with no office
	// only pass holders come. Guests holding a valid season pass pay
	// nothing; guests whose DailyBudget can't cover it stay home. Defaults
	// to DefaultDayTicketPrice; the player adjusts it via the ticket
	// office popup.
	DayTicketPrice int

	// ParkingPrice is the per-car fee in dollars for every lot, set from
	// the parking lot popup. The car's guests split it and each pays their
	// share on arrival, pass holders included. 0 (the default) is free.
	ParkingPrice int

	// Parcels is the scenario-authored land-ownership registry. An empty
	// slice means no parcel system: all cells are accessible. When non-empty,
	// only ParcelOwned parcels are accessible; call ApplyParcels to derive
	// Terrain.accessible from this list.
	Parcels []Parcel

	// SkiArea is the ski-area boundary (ski_area.go); skiAreaCells caches
	// the cells inside it, -1 when stale.
	SkiArea      []SkiAreaOutline
	skiAreaCells int
	skiAreaMask  []bool

	// Goals and Rules are what the scenario asks of the player (goals.go);
	// GoalProgress holds how each goal stands, in the same order.
	Goals        []Goal
	Rules        []string
	GoalProgress []GoalProgress
	Outcome      Outcome
	// OutcomeDay is the day index the scenario was won or lost.
	OutcomeDay int

	// Rating is the resort rating in stars, 1–5: the average stars the
	// guests who left the last day with departures gave (Review), shown
	// as the HUD's heart. RatingShare puts it on 0–1 for demand.
	Rating float32

	// Lakes are the map's lakes and ponds and their ice; their cells are
	// in Terrain.LakeOf. Set by the Auto material terrain layer.
	Lakes []Lake

	// Seed is the RNG seed used when this world's Simulation was created.
	// Saved and reloaded so SeedGuests on load produces the same guest pool.
	Seed int64

	// SimTime is the sim clock in seconds. Simulation owns the live clock
	// and mirrors it here every sub-tick so saves capture it; on load,
	// NewSimulationWithSeed starts the clock from this value.
	SimTime float64

	// StartDate is the calendar date SimTime 0 maps to (midnight UTC). The
	// sim's calendar (sim.DateAt) counts one day per sim day from here.
	// Authored per scenario in the editor; defaults to DefaultStartDate.
	StartDate time.Time

	// Scenario is the scenario's name, description, and campaign placing.
	Scenario ScenarioInfo

	// GroupMix is how the scenario's guests come in groups (group.go);
	// zero for DefaultGroupMix. GroupsRev counts regroupings, for
	// GuestGroups' cache.
	GroupMix  GroupMix
	GroupsRev int
	groups    groupCache

	// Geo is the real-world area the terrain was imported from, or nil
	// for drawn maps and imports from before it was recorded.
	Geo *GeoBounds
	// BaseAltitude is the height above sea level, in metres, of ground
	// elevation 0. Zero for drawn maps, which sit at sea level.
	BaseAltitude float32
	// TimeZone is the IANA zone of the place, e.g. "America/Los_Angeles".
	// With Geo it makes the clock local time; empty means the clock is
	// solar time.
	TimeZone string
	// Climate is the place's typical weather, or nil for the generic one.
	Climate *Climate
	// TerrainBase is the imported ground before terrain layers, for the
	// editor; nil for drawn maps, older imports, and player games.
	TerrainBase *TerrainBase

	// FocusedGuestID is the ID of the guest currently being followed by the
	// camera (0 = none). Written by the scene layer; exposed to the query
	// system so "WHERE followed = 1" works in live SQL queries.
	FocusedGuestID uint64
}

// NewWorld creates a World with the given terrain and the default
// starting balance.
func NewWorld(terrain *Terrain) *World {
	return &World{
		Terrain:         terrain,
		nextID:          1,
		skiAreaCells:    -1,
		Cash:            StartingCash,
		CreditLimit:     DefaultCreditLimit,
		CreditRate:      DefaultCreditRate,
		StartDate:       DefaultStartDate,
		History:         NewHistory(),
		SeasonPassPrice: DefaultSeasonPassPrice,
		DayTicketPrice:  DefaultDayTicketPrice,
		OpenHour:        DefaultOpenHour,
		CloseHour:       DefaultCloseHour,
		Rating:          InitialRating,
	}
}

// InitialRating is a new resort's rating in stars, before any guest has
// left.
const InitialRating = 3.0

// RatingShare is the rating on 0–1 (1★ is 0, 5★ is 1), for demand and
// the ticket price.
func (w *World) RatingShare() float32 {
	return min(max((w.Rating-1)/(MaxStars-1), 0), 1)
}

// Available returns what the player can spend right now: cash plus the
// undrawn part of the credit line. Negative when below the credit floor.
func (w *World) Available() int {
	return w.Cash + w.CreditLimit
}

// CanAfford reports whether spending cost keeps Cash at or above the
// credit floor (−CreditLimit).
func (w *World) CanAfford(cost int) bool {
	return cost <= w.Available()
}

// CreditDrawn returns the drawn part of the credit line in dollars: the
// negative part of Cash, 0 when Cash is non-negative.
func (w *World) CreditDrawn() int {
	if w.Cash < 0 {
		return -w.Cash
	}
	return 0
}

// LiftCost returns what it costs to build a lift of the given type
// between two world XZ positions: the type's station-pair fee plus its
// per-metre run cost. Heli has no cable; use HelipadCost.
func LiftCost(typ LiftType, base, top mgl32.Vec2) int {
	length := base.Sub(top).Len()
	return typ.StationCost() + int(length*float32(typ.PerMeterCost()))
}

// DailyOperatingCost returns the resort's operating cost for one open
// in-game day in dollars. See OperatingCosts for the breakdown.
func (w *World) DailyOperatingCost() int { return w.OperatingCosts().Total() }

// OperatingCosts is one open day's operating cost by category: lift
// staff and running costs, cats by status, staffed buildings, and
// enabled snow guns.
func (w *World) OperatingCosts() CostBreakdown {
	var c CostBreakdown
	for _, l := range w.Lifts {
		c[CostLifts] += l.Headcount()*LiftStaffDailyCost + l.Type.RunningCostDay()
	}
	for _, cat := range w.Snowcats {
		if cat.Status == CatActive {
			c[CostSnowcats] += CatActiveCostDay
		} else {
			c[CostSnowcats] += CatStandbyCostDay
		}
	}
	for _, b := range w.Buildings {
		switch b.Type {
		case BuildingLodge:
			c[CostBuildings] += LodgeUpkeep(b)
		case BuildingSnowGun:
			if b.SnowGunEnabled {
				c[CostSnowGuns] += SnowGunActiveCostDay
			}
		}
	}
	return c
}

// DailyStandbyCost returns what the resort costs per in-game day while
// closed, in dollars. See StandbyCosts for the breakdown.
func (w *World) DailyStandbyCost() int { return w.StandbyCosts().Total() }

// StandbyCosts is one closed day's cost by category: idle lifts, every
// cat parked, staffed buildings unstaffed. Snow guns still cost their
// active rate when enabled, since snowmaking before opening is the point
// of having them.
func (w *World) StandbyCosts() CostBreakdown {
	var c CostBreakdown
	c[CostLifts] = len(w.Lifts) * LiftStandbyCostDay
	c[CostSnowcats] = len(w.Snowcats) * CatStandbyCostDay
	for _, b := range w.Buildings {
		switch b.Type {
		case BuildingLodge:
			c[CostBuildings] += BuildingStandbyCostDay
		case BuildingSnowGun:
			if b.SnowGunEnabled {
				c[CostSnowGuns] += SnowGunActiveCostDay
			}
		}
	}
	return c
}

// NextID returns the next unique entity ID.
func (w *World) NextID() uint64 {
	id := w.nextID
	w.nextID++
	return id
}

// SetMinNextID raises the internal ID counter to at least n+1 so that
// subsequent NextID() calls won't collide with `n`. Used by the save
// loader after restoring entities with their original IDs.
func (w *World) SetMinNextID(n uint64) {
	if n+1 > w.nextID {
		w.nextID = n + 1
	}
}

// PlaceObject places a decorative natural object (rock, stump, lone tree).
// Passability is not affected; rocks and stumps are decorative. Forest trees are stored on the Terrain (trees.go).
func (w *World) PlaceObject(t ObjectType, x, z int) *PlacedObject {
	obj := &PlacedObject{
		ID:   w.NextID(),
		Type: t,
		Pos:  [2]int{x, z},
	}
	w.Objects = append(w.Objects, obj)
	return obj
}

// RemoveObject removes a placed decorative object.
func (w *World) RemoveObject(id uint64) {
	for i, obj := range w.Objects {
		if obj.ID == id {
			w.Objects = append(w.Objects[:i], w.Objects[i+1:]...)
			return
		}
	}
}

// PlaceBuilding places a lodge at world XZ position (x, z) and marks
// the cell containing the anchor as impassable. Equivalent to
// PlaceBuildingType(BuildingLodge, x, z).
func (w *World) PlaceBuilding(x, z float32) *Building {
	return w.PlaceBuildingType(BuildingLodge, x, z)
}

// PlaceBuildingType places a building of the given type. Cost /
// affordability gating lives in the caller so save load and testbed
// setup can construct entities without re-deducting from the player's
// balance.
//
// Parking lots placed this way get the default rectangle centred on
// (x, z); the player-facing path draws one with PlaceRectLot.
//
// Multi-cell footprints with rotated AABB rasterisation are a future
// extension.
func (w *World) PlaceBuildingType(typ BuildingType, x, z float32) *Building {
	b := &Building{
		ID:   w.NextID(),
		Type: typ,
		Pos:  mgl32.Vec2{x, z},
	}
	switch typ {
	case BuildingParking:
		r := DefaultLotRect(b.Pos)
		b.LotSize = mgl32.Vec2{2 * r.HalfX, 2 * r.HalfZ}
	case BuildingLodge, BuildingBar, BuildingTicketOffice:
		b.MealPrice = DefaultMealPrice
		b.DrinkPrice = DefaultDrinkPrice
		b.RentalPrice = DefaultRentalPrice
		b.Quality = DefaultQuality
	case BuildingSnowGun:
		b.SnowGunEnabled = true
	}
	w.Buildings = append(w.Buildings, b)
	if typ == BuildingLodge || typ == BuildingBar || typ == BuildingTicketOffice {
		// Point-placed lodges, bars and ticket offices (tests, testbeds,
		// the editor's ticket office) become small service buildings.
		w.placePointService(b)
		return b
	}
	// Snow guns are narrow pole-mounted devices — don't block any cell.
	if typ != BuildingSnowGun {
		cell := b.DoorCell()
		if w.Terrain.InBounds(cell[0], cell[1]) {
			w.Terrain.Cells[cell[0]][cell[1]].Passable = false
		}
	}
	if typ == BuildingParking {
		w.RefreshParkingLot(b, false)
	}
	return b
}

// ClearBuilt removes everything built on the ground: buildings (and the
// snowcats and patrollers they house), lifts, and roads, leaving every
// cell passable. Trails, trees, snow, and parcels stay. For when the
// ground itself is replaced.
func (w *World) ClearBuilt() {
	w.Buildings, w.Lifts = nil, nil
	w.RoadNodes, w.RoadEdges = nil, nil
	w.Snowcats, w.Patrollers = nil, nil
	for x := range w.Terrain.Cells {
		for z := range w.Terrain.Cells[x] {
			w.Terrain.Cells[x][z].Passable = true
		}
	}
	w.RebuildTrailGraph()
}

// RemoveBuilding removes a building and restores cell passability.
// Sheds also evict their snowcats — the cats have nowhere to go home
// to once the shed is gone. Parking lots also drop their driveway
// node (and any road edges the player attached to it) since those
// references can't survive the lot's removal.
func (w *World) RemoveBuilding(id uint64) {
	for i, b := range w.Buildings {
		if b.ID == id {
			if b.IsShell() {
				w.RemoveSnowcatsOwnedBy(b.ID)
				w.RemoveSnowmobilesIn(b.ID)
				w.RemovePatrollersOwnedBy(b.ID)
				for _, c := range b.Ground {
					if w.Terrain.InBounds(c[0], c[1]) {
						w.Terrain.Cells[c[0]][c[1]].Passable = true
					}
				}
			} else if b.Type != BuildingSnowGun {
				cell := b.DoorCell()
				if w.Terrain.InBounds(cell[0], cell[1]) {
					w.Terrain.Cells[cell[0]][cell[1]].Passable = true
				}
			}
			if b.Type == BuildingParking {
				w.disconnectLotDriveway(b)
				for _, id := range b.DrivewayNodeIDs {
					w.RemoveRoadNode(id)
				}
			}
			w.Buildings = append(w.Buildings[:i], w.Buildings[i+1:]...)
			return
		}
	}
}

// PlaceLift creates a lift between two world XZ positions and marks
// the containing cells as impassable. Cost / affordability gating
// lives in the caller (the placement tool path) so save load and
// testbed setup can construct entities without re-deducting from the
// player's balance.
//
// New lifts get an auto-generated display name ("Lift1", "Lift2", ...)
// derived from the highest existing "Lift%d" suffix + 1, so the F4
// debug panel and lift popup show something meaningful out of the box.
// Players can rename via the popup; the auto-namer skips renamed lifts
// when computing the next suffix so renames don't create collisions.
func (w *World) PlaceLift(typ LiftType, bx, bz, tx, tz float32) *Lift {
	lift := &Lift{
		ID:          w.NextID(),
		Type:        typ,
		Name:        w.nextLiftDefaultName(),
		Base:        mgl32.Vec2{bx, bz},
		Top:         mgl32.Vec2{tx, tz},
		Speed:       typ.DefaultSpeed(),
		TicketPrice: DefaultTicketPrice,
		Open:        false,
	}

	if typ == LiftHeli {
		// Helicopters use a state-machine rather than a chair loop. No
		// chairs are created; HeliState drives the simulation.
		lift.HeliState = &HeliData{
			Phase:      HeliAtBase,
			Passengers: make([]*Guest, 0, HeliCapacity),
		}
	} else {
		// Initialise chairs evenly spaced around the loop, each pre-sized
		// to the lift type's per-chair capacity.
		dx := float64(tx - bx)
		dz := float64(tz - bz)
		cableLen := math.Sqrt(dx*dx + dz*dz)
		loopLen := cableLen * 2
		numChairs := int(loopLen / ChairSpacingM)
		if numChairs < 2 {
			numChairs = 2
		}
		cap := typ.Capacity()
		lift.Chairs = make([]Chair, numChairs)
		for i := range lift.Chairs {
			lift.Chairs[i] = Chair{
				Progress:   float32(i) / float32(numChairs),
				Passengers: make([]*Guest, cap),
			}
		}
	}

	w.Lifts = append(w.Lifts, lift)
	if base := lift.QueueCell(); w.Terrain.InBounds(base[0], base[1]) {
		w.Terrain.Cells[base[0]][base[1]].Passable = false
	}
	if top := lift.TopCell(); w.Terrain.InBounds(top[0], top[1]) {
		w.Terrain.Cells[top[0]][top[1]].Passable = false
	}
	// A lift on an existing trail connects to it now, not at the next
	// trail edit.
	w.RebuildTrailGraph()
	return lift
}

// LiftUpgradeCost is the price to convert a lift from one chair variant
// to the next: the difference in station cost between the two types.
// Towers and cable stay in place, so it's cheaper than building fresh.
// Returns 0 if from → to isn't a supported upgrade.
func LiftUpgradeCost(from, to LiftType) int {
	switch {
	case from == LiftDouble && to == LiftFixedTriple,
		from == LiftDouble && to == LiftFixedQuad,
		from == LiftFixedTriple && to == LiftFixedQuad,
		from == LiftFixedQuad && to == LiftHSQuad,
		from == LiftHSQuad && to == LiftHS6Pack:
		return to.StationCost() - from.StationCost()
	}
	return 0
}

// UpgradeLift converts a lift to the given chair variant, deducting the
// upgrade cost from World.Cash. Returns true on success, false if the
// target type doesn't represent a valid upgrade or the player can't
// afford it. Existing passengers are preserved (they keep their current
// seat indices in the resized chair).
//
// Supported transitions: Double → Triple or FixedQuad, Triple →
// FixedQuad, FixedQuad → HSQuad, HSQuad → HS6Pack.
// Cable, towers, queue, and chair positions are unchanged.
func (w *World) UpgradeLift(l *Lift, target LiftType) bool {
	if l == nil {
		return false
	}
	cost := LiftUpgradeCost(l.Type, target)
	if cost == 0 {
		return false
	}
	if !w.CanAfford(cost) {
		return false
	}
	w.Cash -= cost
	l.Type = target
	l.Speed = target.DefaultSpeed()
	cap := target.Capacity()
	for i := range l.Chairs {
		fresh := make([]*Guest, cap)
		copy(fresh, l.Chairs[i].Passengers)
		l.Chairs[i].Passengers = fresh
	}
	return true
}

// RemoveLift removes a lift and restores passability on both endpoint cells.
func (w *World) RemoveLift(id uint64) {
	for i, lift := range w.Lifts {
		if lift.ID == id {
			if base := lift.QueueCell(); w.Terrain.InBounds(base[0], base[1]) {
				w.Terrain.Cells[base[0]][base[1]].Passable = true
			}
			if top := lift.TopCell(); w.Terrain.InBounds(top[0], top[1]) {
				w.Terrain.Cells[top[0]][top[1]].Passable = true
			}
			w.Lifts = append(w.Lifts[:i], w.Lifts[i+1:]...)
			w.RebuildTrailGraph()
			return
		}
	}
}

// RemoveFromOnMountain splices the guest with the given ID out of
// w.OnMountain and clears their sim scratch fields. The Guest record
// itself stays in w.Guests with State = AtHome — identity and career
// stats persist for the next visit.
func (w *World) RemoveFromOnMountain(id uint64) {
	for i, g := range w.OnMountain {
		if g.ID == id {
			g.ResetForDeparture()
			w.OnMountain = append(w.OnMountain[:i], w.OnMountain[i+1:]...)
			return
		}
	}
}

// nextLiftDefaultName returns the next "Lift%d" suffix to assign to a
// freshly-placed lift, by scanning existing lifts for the highest used
// suffix and adding 1. Lifts the player has renamed (e.g. "Accelerator")
// are skipped, so renames don't open up gaps that later collide. Empty
// names from old saves predating the auto-name change count as zero and
// also get skipped — call EnsureLiftNames to backfill those.
func (w *World) nextLiftDefaultName() string {
	max := 0
	for _, l := range w.Lifts {
		var n int
		if _, err := fmt.Sscanf(l.Name, "Lift%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("Lift%d", max+1)
}

// EnsureLiftNames assigns a default "Lift%d" name to every lift whose
// Name is currently empty. Idempotent — re-running it is a no-op. Used
// by the save loader to backfill names on lifts saved before the auto-
// naming change.
func (w *World) EnsureLiftNames() {
	for _, l := range w.Lifts {
		if l.Name == "" {
			l.Name = w.nextLiftDefaultName()
		}
	}
}

// NearestLift returns the nearest lift to the given world XZ position,
// or nil. Distance is measured in world metres against each lift base.
func (w *World) NearestLift(pos mgl32.Vec2) *Lift {
	var nearest *Lift
	bestDist := math.MaxFloat64
	for _, lift := range w.Lifts {
		dx := float64(lift.Base[0] - pos[0])
		dz := float64(lift.Base[1] - pos[1])
		dist := math.Sqrt(dx*dx + dz*dz)
		if dist < bestDist {
			bestDist = dist
			nearest = lift
		}
	}
	return nearest
}
