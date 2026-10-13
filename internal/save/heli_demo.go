//go:build demo

package save

import "mountain-mogul/internal/world"

// The demo has no heli (heli.go); never reached, since Cableless is
// always false.

type liftDataExtras struct{}

func saveHeli(*world.Lift, *LiftData) {}
func loadHeli(*world.Lift, LiftData)  {}
