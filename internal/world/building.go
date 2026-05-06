package world

import (
	"math"
	"math/rand"
)

// Building represents a lodge that holds skiers and spawns them into the world.
type Building struct {
	ID            uint64
	Pos           [2]int
	Rotation      float32
	MeanSpawnRate float64 // mean spawns per second (Poisson process)
	SkierCount    int     // skiers currently in the lodge pool
	spawnTimer    float64
	nextSpawnIn   float64 // random interval until next spawn (exponential)
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
