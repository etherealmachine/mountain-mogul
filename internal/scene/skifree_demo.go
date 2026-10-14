//go:build demo

package scene

import (
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// The demo has no SkiFree Mode.
type skiMode struct{ scroll float32 }

func (s *Scenario) skiInput(*engine.Input, bool) (bool, bool)                             { return false, false }
func (s *Scenario) skiItButton(interface{ AddActionButton(string, func()) }, *world.Lift) {}
func (s *Scenario) chaseCamera(*render.Camera, *world.Guest, float64, float32)            {}
