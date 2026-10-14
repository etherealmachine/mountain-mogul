//go:build demo

package sim

// The demo has no SkiFree Mode: World.Player is never set.
func (s *Simulation) tickPlayer(float64) {}

// PlayerInput is SkiFree Mode's keys; unused in the demo.
type PlayerInput struct{ Left, Right, Sharp, Brake, Stop, Push bool }
