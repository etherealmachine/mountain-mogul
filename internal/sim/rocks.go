package sim

import (
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// Rock shows through snow thinner than world.ThinSnowDepth. A skier on it
// hits rocks now and then, more the thinner it is and the further they
// go: a scrape that annoys them ("hit rocks, the snow's too thin", at most
// once in rockThoughtGap), and sometimes a fall. Routes keep off thin
// snow where they can (thinSnowCost).
const (
	// rockHitsPerMetre is the chance per metre skied of hitting rock on
	// bare ground, falling to none at ThinSnowDepth.
	rockHitsPerMetre = float32(0.01)
	// rockFallShare is the share of rock hits that knock the skier down,
	// less for a good skier.
	rockFallShare = float32(0.2)
	// rockThoughtGap is the sim seconds between a guest's rock thoughts.
	rockThoughtGap = 600.0
)

// rockHit rolls a skier's chance of hitting rock on cell this tick, and
// reports whether it knocked them down.
func (s *Simulation) rockHit(a *world.Guest, cell *world.Cell, dt float64) bool {
	depth := cell.VisibleSnowDepth()
	if depth >= world.ThinSnowDepth || a.Speed < 1 {
		return false
	}
	thin := 1 - depth/world.ThinSnowDepth
	if rng.Global().Float32() >= rockHitsPerMetre*thin*a.Speed*float32(dt) {
		return false
	}
	if s.SimTime-a.LastRockHit >= rockThoughtGap || a.LastRockHit == 0 {
		s.applyEvent(a, ai.ThoughtHitRocks)
	}
	a.LastRockHit = s.SimTime
	if rng.Global().Float32() < rockFallShare*(1-0.7*clamp32(a.Traits.Skill, 0, 1)) {
		s.knockDown(a, world.FallForward)
		return true
	}
	return false
}
