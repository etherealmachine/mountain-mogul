package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// World owns all simulation state.
type World struct {
	Terrain   *Terrain
	Objects   []*PlacedObject
	Buildings []*Building
	Lifts     []*Lift
	Agents    []*Agent
	nextID    uint64

	// Cash is the resort's bank balance in dollars. Decoration today: nothing
	// in the simulation reads or writes it. Wired into the top-bar HUD so the
	// economy work has a home to grow into.
	Cash int
}

// NewWorld creates a World with the given terrain.
func NewWorld(terrain *Terrain) *World {
	return &World{
		Terrain: terrain,
		nextID:  1,
		Cash:    50000,
	}
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

// PlaceBuilding places a lodge and marks the cell as impassable.
func (w *World) PlaceBuilding(x, z int) *Building {
	b := &Building{
		ID:            w.NextID(),
		Pos:           [2]int{x, z},
		MeanSpawnRate: 0.5, // mean: 1 skier per 2 seconds
		SkierCount:    100,
	}
	w.Buildings = append(w.Buildings, b)
	if w.Terrain.InBounds(x, z) {
		w.Terrain.Cells[x][z].Passable = false
	}
	return b
}

// RemoveBuilding removes a building and marks the cell as passable.
func (w *World) RemoveBuilding(id uint64) {
	for i, b := range w.Buildings {
		if b.ID == id {
			if w.Terrain.InBounds(b.Pos[0], b.Pos[1]) {
				w.Terrain.Cells[b.Pos[0]][b.Pos[1]].Passable = true
			}
			w.Buildings = append(w.Buildings[:i], w.Buildings[i+1:]...)
			return
		}
	}
}

// PlaceLift creates a lift and marks its endpoint cells as impassable.
func (w *World) PlaceLift(bx, bz, tx, tz int) *Lift {
	lift := &Lift{
		ID:    w.NextID(),
		Base:  [2]int{bx, bz},
		Top:   [2]int{tx, tz},
		Speed: 2.5, // m/s — realistic chairlift speed
	}

	// Initialise chairs evenly spaced around the loop.
	const cellSize = 10.0
	dx := float64(tx-bx) * cellSize
	dz := float64(tz-bz) * cellSize
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
	if w.Terrain.InBounds(bx, bz) {
		w.Terrain.Cells[bx][bz].Passable = false
	}
	if w.Terrain.InBounds(tx, tz) {
		w.Terrain.Cells[tx][tz].Passable = false
	}
	return lift
}

// RemoveLift removes a lift and restores passability.
func (w *World) RemoveLift(id uint64) {
	for i, lift := range w.Lifts {
		if lift.ID == id {
			if w.Terrain.InBounds(lift.Base[0], lift.Base[1]) {
				w.Terrain.Cells[lift.Base[0]][lift.Base[1]].Passable = true
			}
			if w.Terrain.InBounds(lift.Top[0], lift.Top[1]) {
				w.Terrain.Cells[lift.Top[0]][lift.Top[1]].Passable = true
			}
			w.Lifts = append(w.Lifts[:i], w.Lifts[i+1:]...)
			return
		}
	}
}

// SpawnAgent creates a new agent at the given building's position.
func (w *World) SpawnAgent(b *Building) *Agent {
	const cellSize = 10.0
	x := b.Pos[0]
	z := b.Pos[1]
	elev := w.Terrain.ElevationAt(x, z)
	agent := &Agent{
		ID:  w.NextID(),
		Pos: mgl32.Vec3{float32(x) * cellSize, elev, float32(z) * cellSize},
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

// NearestLift returns the nearest lift to the given grid position, or nil.
func (w *World) NearestLift(pos [2]int) *Lift {
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
