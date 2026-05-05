package world

// LiftRider tracks an agent's progress along the lift.
type LiftRider struct {
	Agent    *Agent
	Progress float32 // 0.0 (base) -> 1.0 (top)
}

// Lift represents a ski lift connecting a base to a top station.
type Lift struct {
	ID    uint64
	Base  [2]int
	Top   [2]int
	Speed float32 // fraction of lift length completed per second

	Queue  []*Agent
	Riders []LiftRider
}

// MaxRiders is the maximum number of riders on a lift at once.
const MaxRiders = 4
