package rng

import "math/rand"

var g *rand.Rand

// Init seeds the global RNG. Must be called once at game start with the
// session seed. All gameplay randomness flows through this source.
func Init(seed int64) {
	g = rand.New(rand.NewSource(seed))
}

// Global returns the shared game RNG. Init must have been called first.
func Global() *rand.Rand {
	return g
}
