package world

import "github.com/go-gl/mathgl/mgl32"

// AgentState represents the current state of an agent.
type AgentState int

const (
	StateWalking AgentState = iota
	StateQueuing
	StateRiding
	StateSkiing
)

// Agent is a skier/guest in the simulation.
type Agent struct {
	ID           uint64
	Pos          mgl32.Vec3
	Heading      float32
	State        AgentState
	Path         [][2]int
	PathIdx      int
	TargetLiftID uint64 // which lift to walk/ski toward
}
