package world

// Building represents a guest building that spawns agents.
type Building struct {
	ID         uint64
	Pos        [2]int
	Rotation   float32
	SpawnRate  float64 // agents spawned per second
	spawnTimer float64
}

// SpawnTimer returns the current spawn timer value.
func (b *Building) SpawnTimer() float64 { return b.spawnTimer }

// AdvanceTimer advances the spawn timer by dt seconds and returns
// true if a spawn should occur (and resets the timer).
func (b *Building) AdvanceTimer(dt float64) bool {
	if b.SpawnRate <= 0 {
		return false
	}
	b.spawnTimer += dt
	if b.spawnTimer >= 1.0/b.SpawnRate {
		b.spawnTimer = 0
		return true
	}
	return false
}
