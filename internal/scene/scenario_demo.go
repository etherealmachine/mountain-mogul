//go:build demo

package scene

import (
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// scenarioDev is empty in the demo: no debug console, inspectors, logs,
// or testbed and live-query hooks (scenario_dev.go has the full game's).
type scenarioDev struct{}

func (s *Scenario) initDev()                                         {}
func (s *Scenario) installDev()                                      {}
func (s *Scenario) devTick()                                         {}
func (s *Scenario) devServeQueries()                                 {}
func (s *Scenario) devConsoleInput(*engine.Input, bool) bool         { return false }
func (s *Scenario) devHotkeys(*engine.Input, *render.Renderer, bool) {}
func (s *Scenario) devCaptureFrame(*render.Renderer)                 {}
func (s *Scenario) devAfterUI()                                      {}

func (s *Scenario) devSteeringLines(_ *world.Guest, lines []render.DebugLine) []render.DebugLine {
	return lines
}

func (s *Scenario) devPlannerPanel(d []render.UIDrawable, _ *world.Guest) []render.UIDrawable {
	return d
}
func (s *Scenario) devInspector(d []render.UIDrawable) []render.UIDrawable       { return d }
func (s *Scenario) devConsoleDrawable(d []render.UIDrawable) []render.UIDrawable { return d }
