package goap

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// SkiTrail is a GOAP action that moves the agent along a player-defined
// trail from one entity anchor to another. FromID/FromKind identify the
// guest's current anchor (a lift top, building, or trail junction); ToID/
// ToKind identify the destination (a lift base, building, or trail junction).
// TrailID is the trail being traversed — used for plan display and completion
// detection when the destination is another trail.
type SkiTrail struct {
	TrailID  uint64
	FromID   uint64
	FromKind world.EdgeKind
	ToID     uint64
	ToKind   world.EdgeKind
	Distance float32
}

func (a *SkiTrail) Name() string {
	return fmt.Sprintf("SkiTrail(%d,%d→%d)", a.TrailID, a.FromID, a.ToID)
}

func (a *SkiTrail) Precondition(s *WorldSnapshot, w *world.World) bool {
	if s.Removed {
		return false
	}
	switch a.FromKind {
	case world.KindLiftTop:
		return s.AtLiftTop == a.FromID
	case world.KindTrail:
		return s.AtTrailEnd == a.FromID
	case world.KindBuilding:
		return s.AtLodge == a.FromID || s.AtParking == a.FromID
	}
	return false
}

func (a *SkiTrail) Apply(s *WorldSnapshot, w *world.World) {
	// Clear the current anchor.
	s.AtLiftTop = 0
	s.AtTrailEnd = 0
	s.AtLodge = 0
	s.AtParking = 0

	switch a.ToKind {
	case world.KindLiftBase:
		s.AtLiftBase = a.ToID
		if l := findLift(w, a.ToID); l != nil {
			s.Pos = mgl32.Vec3{l.Base[0], s.Pos[1], l.Base[1]}
		}
	case world.KindBuilding:
		b := findAnyBuilding(w, a.ToID)
		if b != nil {
			s.Pos = mgl32.Vec3{b.Pos[0], s.Pos[1], b.Pos[1]}
			switch b.Type {
			case world.BuildingLodge:
				s.AtLodge = a.ToID
			case world.BuildingParking:
				s.AtParking = a.ToID
			}
		}
	case world.KindTrail:
		// Arriving at a trail junction. AtTrailEnd is set to the destination
		// trail so the next tick can generate SkiTrail actions from that trail.
		s.AtTrailEnd = a.ToID
		if t := w.FindTrail(a.ToID); t != nil {
			c := t.Centroid()
			s.Pos = mgl32.Vec3{c[0], s.Pos[1], c[1]}
		}
	}
}

func (a *SkiTrail) Cost(s *WorldSnapshot, w *world.World) float32 {
	return a.Distance / skiSpeedMps
}

// =============================================================================
// ApplicableActions extension — trail-based descents
// =============================================================================

// trailActions appends SkiTrail actions reachable from the guest's current
// anchor via the world's TrailGraph. Returns the (possibly grown) slice.
func trailActions(out []Action, s *WorldSnapshot, w *world.World) []Action {
	if w.TrailGraph == nil {
		return out
	}
	anchor := currentAnchorID(s)
	if anchor == 0 {
		return out
	}
	for _, edge := range w.TrailGraph.EdgesFrom(anchor) {
		a := &SkiTrail{
			TrailID:  edge.TrailID,
			FromID:   edge.FromID,
			FromKind: edge.FromKind,
			ToID:     edge.ToID,
			ToKind:   edge.ToKind,
			Distance: edge.Distance,
		}
		if a.Precondition(s, w) {
			out = append(out, a)
		}
	}
	return out
}

// CurrentAnchorID returns the entity ID the guest is currently anchored at,
// or 0 if in transit. Checks all At* fields in priority order.
func CurrentAnchorID(s *WorldSnapshot) uint64 {
	return currentAnchorID(s)
}

// currentAnchorID returns the entity ID the guest is currently anchored at,
// or 0 if in transit. Checks all At* fields in priority order.
func currentAnchorID(s *WorldSnapshot) uint64 {
	if s.AtLiftTop != 0 {
		return s.AtLiftTop
	}
	if s.AtTrailEnd != 0 {
		return s.AtTrailEnd
	}
	if s.AtLodge != 0 {
		return s.AtLodge
	}
	if s.AtParking != 0 {
		return s.AtParking
	}
	return 0
}

// skillDiff maps a skill level to the terrain difficulty a guest requires.
// Returns 0 for Advanced+ — they're willing to free-roam and don't filter on
// difficulty when choosing a lift.
func skillDiff(skill ai.SkillLevel) world.TerrainDifficulty {
	switch skill {
	case ai.SkillBeginner:
		return world.DiffGreen
	case ai.SkillIntermediate:
		return world.DiffBlue
	default:
		return 0
	}
}

// findAnyBuilding returns the building with the given ID regardless of type.
func findAnyBuilding(w *world.World, id uint64) *world.Building {
	for _, b := range w.Buildings {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// trailLabel returns a display label for a trail by ID.
func trailLabel(w *world.World, id uint64) string {
	if t := w.FindTrail(id); t != nil && t.Name != "" {
		return t.Name
	}
	return fmt.Sprintf("Trail#%d", id)
}

// skiTrailPlanActionLabel returns the HUD label for an ActSkiTrail step.
func skiTrailPlanActionLabel(pa ai.PlanAction, w *world.World) string {
	via := trailLabel(w, pa.TrailID)
	switch {
	case pa.LiftID != 0:
		return "SkiTrail(" + via + "→" + liftLabel(w, pa.LiftID) + ")"
	case pa.BldgID != 0:
		return "SkiTrail(" + via + "→" + buildingLabel(w, pa.BldgID) + ")"
	default:
		return "SkiTrail(" + via + "→" + trailLabel(w, pa.TrailID) + ")"
	}
}

// skiTrailDisplayName returns the display name for a SkiTrail action.
func skiTrailDisplayName(a *SkiTrail, w *world.World) string {
	via := trailLabel(w, a.TrailID)
	switch a.ToKind {
	case world.KindLiftBase:
		return "SkiTrail(" + via + "→" + liftLabel(w, a.ToID) + ")"
	case world.KindBuilding:
		return "SkiTrail(" + via + "→" + buildingLabel(w, a.ToID) + ")"
	case world.KindTrail:
		return "SkiTrail(" + via + "→" + trailLabel(w, a.ToID) + ")"
	}
	return fmt.Sprintf("SkiTrail(%s→#%d)", via, a.ToID)
}

