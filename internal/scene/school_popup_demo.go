//go:build demo

package scene

import (
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// The demo has no ski school.
func (s *Scenario) addSchoolControls(*ui.Window, *world.Building) {}
