package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Cars carry guests between their home entry (a RoadNodeEdgeConnection)
// and a parking lot. The sim (sim/traffic.go) drives them along the road
// graph; the renderer draws each one at Pos.

// CarState is where a car is in its trip.
type CarState uint8

const (
	// CarQueued: spawned at its entry, waiting for room on the road.
	CarQueued CarState = iota
	// CarArriving: driving from the entry to its lot and stall.
	CarArriving
	// CarParked: in its stall while its guests are on the mountain.
	CarParked
	// CarLeaving: driving from its stall back to its entry.
	CarLeaving
	// CarTurnedAway: found no stall anywhere and is driving home with
	// its guests still aboard.
	CarTurnedAway
)

// Car is one carload of guests (one to four).
type Car struct {
	ID     uint64
	Guests []*Guest
	// Entry is the home entry's road node ID; 0 when the map has no
	// entries (the car appears in its stall and vanishes when it leaves).
	Entry uint64
	// Lot is the parking lot the car is heading to or parked in, and
	// Stall the index into its Stalls (-1 until one is claimed).
	Lot   uint64
	Stall int
	State CarState
	// Route is the road nodes the car drives through: entry to the lot's
	// driveway when arriving, driveway to entry when leaving. Leg indexes
	// the car's legs (see sim.carLegCount) and D is metres along that leg.
	Route []uint64
	Leg   int
	D     float32
	Speed float32 // m/s
	// Pos, Heading and InLot are the car's pose for the renderer, set by
	// the sim every step. Heading is yaw about +Y (0 faces +Z).
	Pos     mgl32.Vec2
	Heading float32
	InLot   bool
}

// Moving reports whether the car is on the road or in a lot aisle.
func (c *Car) Moving() bool {
	return c.State == CarArriving || c.State == CarLeaving || c.State == CarTurnedAway
}

// ParkedCars is how many cars are parked in lot id.
func (w *World) ParkedCars(id uint64) int {
	n := 0
	for _, c := range w.Cars {
		if c.Lot == id && c.State == CarParked {
			n++
		}
	}
	return n
}

// MeanCarload is the average number of guests per car, for anything that
// thinks per guest about a per-car price (the parking fee).
const MeanCarload = 2.4

// ResettleParkedCars puts the cars parked in lot b back in its stalls
// after the lot moved or changed shape: each in its own stall if it still
// exists, otherwise in a free one; a car with no stall left is dropped
// (its guests leave from any lot).
func (w *World) ResettleParkedCars(b *Building) {
	used := make([]bool, len(b.Stalls))
	var homeless []*Car
	for _, c := range w.Cars {
		if c.Lot != b.ID || c.State != CarParked {
			continue
		}
		if c.Stall >= 0 && c.Stall < len(used) && !used[c.Stall] {
			used[c.Stall] = true
			continue
		}
		homeless = append(homeless, c)
	}
	for _, c := range homeless {
		c.Stall = -1
		for i, u := range used {
			if !u {
				c.Stall, used[i] = i, true
				break
			}
		}
	}
	keep := w.Cars[:0]
	for _, c := range w.Cars {
		if c.Lot == b.ID && c.State == CarParked {
			if c.Stall < 0 {
				for _, g := range c.Guests {
					if g.CarID == c.ID {
						g.CarID, g.CarLot = 0, 0
						if g.State == InCar {
							g.State = AtHome
						}
					}
				}
				continue
			}
			st := b.Stalls[c.Stall]
			c.Pos, c.Heading = st.Pos, st.Heading+math.Pi
		}
		keep = append(keep, c)
	}
	w.Cars = keep
}
