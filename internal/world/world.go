package world

import (
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
	Agents    []*Agent
	Snowcats  []*Snowcat
	nextID    uint64

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
// Lodge defaults: spawn 1 skier every ~2 s from a pool of 100.
// Shed defaults: no spawning — sheds are equipment storage and don't
// hold a skier pool.
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
		// Parking lots are the primary skier spawn/despawn point. Each
		// arriving car drops off ~4 skiers; departures reverse it.
		b.MeanSpawnRate = 0.5 // mean: 1 skier per 2 seconds
		b.SkierCount = 100
		b.MaxCars = 40
	case BuildingLodge:
		// Lodges are reserved for future rest/lunch features and don't
		// currently spawn skiers. The spawn fields stay zero so the
		// sim's spawn picker ignores them.
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
// to once the shed is gone.
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
func (w *World) PlaceLift(bx, bz, tx, tz float32) *Lift {
	lift := &Lift{
		ID:          w.NextID(),
		Base:        mgl32.Vec2{bx, bz},
		Top:         mgl32.Vec2{tx, tz},
		Speed:       2.5, // m/s — realistic chairlift speed
		TicketPrice: DefaultTicketPrice,
	}

	// Initialise chairs evenly spaced around the loop.
	dx := float64(tx - bx)
	dz := float64(tz - bz)
	cableLen := math.Sqrt(dx*dx + dz*dz)
	loopLen := cableLen * 2
	numChairs := int(loopLen / ChairSpacingM)
	if numChairs < 2 {
		numChairs = 2
	}
	lift.Chairs = make([]Chair, numChairs)
	for i := range lift.Chairs {
		lift.Chairs[i] = Chair{Progress: float32(i) / float32(numChairs)}
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

// SpawnAgent creates a new agent at the building's anchor position.
//
// Y is taken from the terrain mesh under the lodge cell. Pre-migration
// this used the cell *corner* as the spawn XZ, producing a half-cell
// offset from where the lodge actually sat — fixed alongside the move
// to continuous Pos so agents now spawn exactly under the lodge anchor.
func (w *World) SpawnAgent(b *Building) *Agent {
	cell := b.DoorCell()
	elev := w.Terrain.SurfaceElevationAt(cell[0], cell[1])
	agent := &Agent{
		ID:  w.NextID(),
		Pos: mgl32.Vec3{b.Pos[0], elev, b.Pos[1]},
	}
	w.Agents = append(w.Agents, agent)
	return agent
}

// RemoveAgent removes an agent by ID.
func (w *World) RemoveAgent(id uint64) {
	for i, a := range w.Agents {
		if a.ID == id {
			w.Agents = append(w.Agents[:i], w.Agents[i+1:]...)
			return
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
