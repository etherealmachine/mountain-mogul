//go:build demo

package sim

import "mountain-mogul/internal/world"

// The demo has no heli; never reached, since Cableless is always false.
func tickHeliLift(*Simulation, *world.Lift, float64) {}
