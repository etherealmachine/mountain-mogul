//go:build demo

package scene

import (
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// The demo has no snowmaking (trail_snowmaking.go).

func addSnowmakingRows(*ui.Window, *world.World, *world.Trail, trailHooks) {}

func (s *Scenario) snowmakingStatus(*world.Trail) string { return "" }
