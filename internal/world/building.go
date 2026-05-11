package world

import (
	"math"
	"math/rand"

	"github.com/go-gl/mathgl/mgl32"
)

// BuildingType selects what a Building represents and which mesh
// renders it. New types added here also need a mesh ID in
// world/objects.go (mirrored in render/mesh.go), a `.scad` source in
// models-src/, a per-type cost in world.go, and a toolbar button in
// scene/scenario.go.
type BuildingType uint8

const (
	BuildingLodge   BuildingType = 0
	BuildingShed    BuildingType = 1
	BuildingParking BuildingType = 2
)

// Building represents a structure placed on the terrain. Parking lots are
// the primary skier spawn/despawn point (a car arrives, four skiers walk
// to the lifts; later they walk back and the car drives off). Lodges are
// reserved for future rest/lunch buildings and currently have no spawn
// behavior. Sheds garage snowcat / snowmobile equipment.
//
// Pos is the building's anchor in continuous world XZ coordinates (metres).
// Y is derived from terrain elevation at use time. Footprints are still
// effectively a single cell for passability rasterisation; oriented-AABB
// footprints are a future extension.
type Building struct {
	ID            uint64
	Type          BuildingType
	Pos           mgl32.Vec2
	Rotation      float32
	MeanSpawnRate float64 // mean spawns per second (Poisson process); Parking only
	SkierCount    int     // skiers currently in the lot's pool; Parking only
	spawnTimer    float64
	nextSpawnIn   float64 // random interval until next spawn (exponential)

	// Parking-only state. MaxCars caps how many cars the lot can hold;
	// CurrentCars is a continuous estimate (incremented per spawn,
	// decremented per despawn) and the render path floors it to instance
	// N car models in a grid pattern. Roughly 4 skiers per car.
	MaxCars     int
	CurrentCars float32

	// Shed-only state. Cats is the number of grooming machines this
	// shed dispatches (1..MaxCatsPerShed). RouteCells holds the cells
	// the player painted as this shed's grooming route — cats pick the
	// least-groomed cell in the list, drive to it, and corduroy it.
	// The route is a *set* of cells (drag-painted), not an ordered
	// path; the cat picks a next target each time it arrives.
	Cats       int
	RouteCells [][2]int
}

// SkiersPerCar is the average number of skiers that arrive together in one
// car. Drives the visual CurrentCars increment/decrement on each spawn/despawn.
const SkiersPerCar = 4

// ArrivalDeparture nudges CurrentCars based on a +1 (arrival = spawn) or -1
// (departure = despawn) event, clamped to [0, MaxCars]. Centralised so
// spawn and despawn callsites can't drift out of sync.
func (b *Building) ArrivalDeparture(sign int) {
	if b.Type != BuildingParking {
		return
	}
	delta := float32(sign) / float32(SkiersPerCar)
	b.CurrentCars += delta
	if b.CurrentCars < 0 {
		b.CurrentCars = 0
	}
	if max := float32(b.MaxCars); max > 0 && b.CurrentCars > max {
		b.CurrentCars = max
	}
}

// DoorCell returns the grid cell containing the building's anchor — the
// pathfinder destination for skiers walking to this lodge. Floor (not
// round) so a Pos exactly on a cell boundary lands in the cell whose
// indices match its floor coordinates, consistent with how skiers map
// their own continuous Pos to a cell elsewhere in the sim.
func (b *Building) DoorCell() [2]int {
	return cellOf(b.Pos)
}

// SpawnTimer returns the current spawn timer value.
func (b *Building) SpawnTimer() float64 { return b.spawnTimer }

// AdvanceTimer advances the spawn timer by dt seconds and returns true if a
// spawn should occur. Inter-arrival times are exponentially distributed with
// mean 1/MeanSpawnRate (Poisson process). The first interval is drawn lazily
// on the first call so callers don't need to know about RNG init order.
// Returns false if the pool is empty.
func (b *Building) AdvanceTimer(dt float64, rng *rand.Rand) bool {
	if b.MeanSpawnRate <= 0 || b.SkierCount <= 0 {
		return false
	}
	if b.nextSpawnIn == 0 {
		b.nextSpawnIn = randExp(b.MeanSpawnRate, rng)
		return false
	}
	b.spawnTimer += dt
	if b.spawnTimer >= b.nextSpawnIn {
		b.spawnTimer = 0
		b.nextSpawnIn = randExp(b.MeanSpawnRate, rng)
		return true
	}
	return false
}

// randExp returns an exponential random variate with the given rate parameter.
func randExp(rate float64, rng *rand.Rand) float64 {
	if rate <= 0 {
		return math.MaxFloat64
	}
	return -math.Log(1-rng.Float64()) / rate
}
