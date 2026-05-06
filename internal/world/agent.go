package world

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
)

// AgentState represents the current state of an agent.
type AgentState int

const (
	StateWalking         AgentState = iota
	StateQueuing                    // waiting in lift queue
	StateRiding                     // on the chairlift
	StateSkiing                     // skiing down toward target lift
	StateReturningToLodge           // skiing back to home lodge after reaching top
	StateFallen                     // briefly immobilised after a fall; resumes on timer
)

// Agent is a skier/guest in the simulation.
type Agent struct {
	ID               uint64
	Pos              mgl32.Vec3
	Heading          float32
	State            AgentState
	Path             [][2]int
	PathIdx          int
	TargetLiftID     uint64
	TargetBuildingID uint64
	Speed            float32

	// AI state — populated by sim package. The persistent types live in
	// internal/ai to avoid a sim ↔ world import cycle.
	Traits      ai.SkierTraits
	Route       ai.Route
	Motor       ai.MotorState
	Balance     float32 // 1.0 fresh; ≤0 triggers a fall
	FallTimer   float32 // seconds remaining in StateFallen
	ResumeState AgentState
}
