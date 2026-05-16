package world

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Cost constants. Tuned so a starting balance of StartingCash buys a
// lodge plus two ~600 m lifts with no padding — players hit the wall
// quickly and have to think about layout.
const (
	LodgeCost       = 50000  // single fixed cost per lodge (no spawn behavior yet — reserved)
	ShedCost        = 30000  // grooming equipment storage; cheaper than a lodge — no plumbing, no kitchen, just bays
	ParkingCost     = 40000  // base parking lot — capacity is fixed for now, scaling via multiple lots
	LiftStationCost = 50000  // fixed cost for both stations of a lift (you always need two)
	LiftPerMeter    = 100    // cost per metre of cable run, covers towers + cable
	StartingCash    = 250000 // 1× parking + 2× ~600 m lifts (40K + 2 × (50K + 60K) = 260K) — slight stretch on lift length

	DefaultTicketPrice = 10 // dollars per lift ride; player adjusts via the lift popup
)

// BuildingCost returns the up-front cost of placing a building of the
// given type.
func BuildingCost(t BuildingType) int {
	switch t {
	case BuildingShed:
		return ShedCost
	case BuildingParking:
		return ParkingCost
	}
	return LodgeCost
}

// World owns all simulation state.
type World struct {
	Terrain   *Terrain
	Objects   []*PlacedObject
	Buildings []*Building
	Lifts     []*Lift
	Trails    []*Trail

	// TrailGraph is the derived connectivity graph built from trail cell data.
	// Rebuilt by RebuildTrailGraph whenever trails are added, removed, or edited.
	// Nil until the first trail is placed.
	TrailGraph *TrailGraph

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

	Snowcats  []*Snowcat
	RoadNodes []*RoadNode
	RoadEdges []*RoadEdge
	nextID    uint64

	// History is a daily ring of stats (guest count, cash, arrivals,
	// departures) feeding the in-game charts window. Nil on a freshly
	// constructed world; the scenario load path / new-game path
	// allocates an empty one so the sim immediately begins recording.
	History *History

	// Cash is the resort's bank balance in dollars. PlaceBuilding /
	// PlaceLift deduct from this and refuse the placement when the
	// balance can't cover the cost.
	Cash int
}

// NewWorld creates a World with the given terrain and the default
// starting balance.
func NewWorld(terrain *Terrain) *World {
	return &World{
		Terrain: terrain,
		nextID:  1,
		Cash:    StartingCash,
	}
}

// LiftCost returns what it costs to build a lift between two world XZ
// positions: a fixed station-pair fee plus per-metre run cost.
func LiftCost(base, top mgl32.Vec2) int {
	length := base.Sub(top).Len()
	return LiftStationCost + int(length*LiftPerMeter)
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
// Passability is not affected — trees use TreeDensity, rocks/stumps are decorative.
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
// Parking lots are passive containers — MaxCars caps the visible
// population, CurrentCars is driven by the (future) demand system.
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
		if layout, ok := ParkingLotLayout(typ); ok {
			b.MaxCars = layout.Capacity()
		}
	case BuildingLodge:
		// Lodges are reserved for future rest/lunch features.
	case BuildingShed:
		// Sheds start with one cat included. Additional cats are
		// purchased via the shed popup. SpawnSnowcat is called after
		// the building is appended so the cat reads the shed's ID
		// for ownership.
		b.Cats = 1
	}
	w.Buildings = append(w.Buildings, b)
	cell := b.DoorCell()
	if w.Terrain.InBounds(cell[0], cell[1]) {
		w.Terrain.Cells[cell[0]][cell[1]].Passable = false
	}
	if typ == BuildingShed {
		w.SpawnSnowcat(b)
	}
	return b
}

// RemoveBuilding removes a building and restores cell passability.
// Sheds also evict their snowcats — the cats have nowhere to go home
// to once the shed is gone. Parking lots also drop their driveway
// node (and any road edges the player attached to it) since those
// references can't survive the lot's removal.
func (w *World) RemoveBuilding(id uint64) {
	for i, b := range w.Buildings {
		if b.ID == id {
			cell := b.DoorCell()
			if w.Terrain.InBounds(cell[0], cell[1]) {
				w.Terrain.Cells[cell[0]][cell[1]].Passable = true
			}
			if b.Type == BuildingShed {
				w.RemoveSnowcatsOwnedBy(b.ID)
			}
			if b.Type == BuildingParking {
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
		Speed:       2.5, // m/s — realistic chairlift speed
		TicketPrice: DefaultTicketPrice,
	}

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

	w.Lifts = append(w.Lifts, lift)
	if base := lift.QueueCell(); w.Terrain.InBounds(base[0], base[1]) {
		w.Terrain.Cells[base[0]][base[1]].Passable = false
	}
	if top := lift.TopCell(); w.Terrain.InBounds(top[0], top[1]) {
		w.Terrain.Cells[top[0]][top[1]].Passable = false
	}
	return lift
}

// LiftUpgradeCost is the price to convert a fixed-grip double into a
// fixed-grip quad — covers replacement of both station bullwheels and
// the full chair set. Towers and cable stay in place, so it's cheaper
// than building a fresh lift from scratch.
const LiftUpgradeCost = LiftStationCost

// UpgradeLift converts a lift to the given chair variant, deducting the
// upgrade cost from World.Cash. Returns true on success, false if the
// target type doesn't represent a valid upgrade or the player can't
// afford it. Existing passengers are preserved (they keep their current
// seat indices in the resized chair).
//
// Currently only Double → FixedQuad is supported; other transitions are
// rejected. Cable, towers, queue, and chair positions are unchanged.
func (w *World) UpgradeLift(l *Lift, target LiftType) bool {
	if l == nil || l.Type != LiftDouble || target != LiftFixedQuad {
		return false
	}
	if w.Cash < LiftUpgradeCost {
		return false
	}
	w.Cash -= LiftUpgradeCost
	l.Type = target
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
