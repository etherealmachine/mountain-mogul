package scene

import (
	"fmt"
	"math"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// structureEditSelection holds the toolNone click-to-edit state for a
// building or a lift endpoint. Mutually exclusive: at most one of
// building or lift is non-nil. liftEnd identifies which lift station is
// being edited (0 = base, 1 = top).
type structureEditSelection struct {
	building   *world.Building
	lift       *world.Lift
	liftEnd    int // 0 = base, 1 = top
	dragging   bool
	dragOffset mgl32.Vec2 // (building.Pos or station.Pos) minus the cursor at click — preserves cursor-on-grab feel
}

func (s *structureEditSelection) active() bool {
	return s.building != nil || s.lift != nil
}

func (s *structureEditSelection) clear() {
	s.building = nil
	s.lift = nil
	s.liftEnd = 0
	s.dragging = false
	s.dragOffset = mgl32.Vec2{}
}

// tryStartStructureEdit hit-tests buildings and lift stations at `pos`.
// Returns true on hit and populates sel; pickRadius mirrors the existing
// removeAt / tryOpenPopup rule (~one cell width). Buildings take
// priority — a click inside a building's footprint always picks the
// building even if a lift cable happens to be near.
func tryStartStructureEdit(w *world.World, pos mgl32.Vec2, sel *structureEditSelection) bool {
	for _, b := range w.Buildings {
		if b.IsPainted() {
			continue // painted lots and lodges are reshaped from their popup, not dragged
		}
		if b.Pos.Sub(pos).Len() <= buildingPickRadius {
			sel.building = b
			sel.lift = nil
			sel.dragOffset = b.Pos.Sub(pos)
			sel.dragging = true
			return true
		}
	}
	// Closer of base / top wins — players reach for whichever station
	// they intend to move, not the cable midpoint.
	var bestLift *world.Lift
	bestEnd := 0
	bestD2 := float32(liftPickRadius * liftPickRadius)
	for _, l := range w.Lifts {
		for end, p := range [2]mgl32.Vec2{l.Base, l.Top} {
			d := p.Sub(pos)
			d2 := d.Dot(d)
			if d2 <= bestD2 {
				bestLift = l
				bestEnd = end
				bestD2 = d2
			}
		}
	}
	if bestLift != nil {
		sel.building = nil
		sel.lift = bestLift
		sel.liftEnd = bestEnd
		var anchor mgl32.Vec2
		if bestEnd == 0 {
			anchor = bestLift.Base
		} else {
			anchor = bestLift.Top
		}
		sel.dragOffset = anchor.Sub(pos)
		sel.dragging = true
		return true
	}
	return false
}

// dragStructure follows the cursor with the selected building or lift
// endpoint and triggers a full rebuild so terrain restoration / new-
// footprint stamping happen each frame. Heavy per-frame cost for lifts
// (cable mesh regen via AddLiftCable) but the structure count is low.
//
// Overlap check on buildings: refuses moves that would collide with
// another building. The selected building is excluded from the check
// since its current Pos still occupies its old footprint.
func dragStructure(r *render.Renderer, w *world.World, sel *structureEditSelection, cursor mgl32.Vec2) {
	target := cursor.Add(sel.dragOffset)
	switch {
	case sel.building != nil:
		newPos := target
		if w.BuildingOverlapExcept(sel.building.Type, newPos[0], newPos[1], sel.building.Rotation, sel.building.ID) {
			return
		}
		if sel.building.Pos == newPos {
			return
		}
		sel.building.Pos = newPos
		// Parking lots own driveway nodes that anchor the road graph.
		// Slide them along so the player's road network follows.
		if sel.building.Type == world.BuildingParking {
			refreshParkingDriveways(w, sel.building)
		}
		applyBuildingPlacementEffects(w, sel.building)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildRoads(w)
	case sel.lift != nil:
		if sel.liftEnd == 0 {
			if sel.lift.Base == target {
				return
			}
			sel.lift.Base = target
		} else {
			if sel.lift.Top == target {
				return
			}
			sel.lift.Top = target
		}
		applyLiftPlacementEffects(w, sel.lift)
		w.RebuildTrailGraph() // the moved end may now touch other trails
		r.FlushTerrainVerts(w.Terrain)
		r.RemoveLiftCable(sel.lift.ID)
		r.AddLiftCable(sel.lift, w.Terrain)
	}
}

// Building rotation steps: R turns a quarter, Shift+R a fine step.
const (
	buildingRotateSteps    = 24 // fine steps per full turn (15°)
	buildingRotateFineStep = float32(2 * math.Pi / buildingRotateSteps)
	buildingRotateStep     = float32(math.Pi / 2)
)

// rotateKeyDelta returns the rotation requested by R this frame (0 if R
// wasn't pressed).
func rotateKeyDelta(inp *engine.Input) float32 {
	if !inp.Pressed[glfw.KeyR] {
		return 0
	}
	if inp.Held[glfw.KeyLeftShift] || inp.Held[glfw.KeyRightShift] {
		return buildingRotateFineStep
	}
	return buildingRotateStep
}

// stepRotation adds delta to rot, snapped to the fine step and wrapped
// into [0, 2π) so repeated turns don't drift or grow without bound.
func stepRotation(rot, delta float32) float32 {
	steps := int(math.Round(float64((rot + delta) / buildingRotateFineStep)))
	steps = ((steps % buildingRotateSteps) + buildingRotateSteps) % buildingRotateSteps
	return float32(steps) * buildingRotateFineStep
}

// rotationDegrees formats a rotation for toasts.
func rotationDegrees(rot float32) string {
	return fmt.Sprintf("%d°", int(math.Round(float64(rot)*180/math.Pi)))
}

// isBuildingPlacementTool reports whether t places a single rotatable
// building.
func isBuildingPlacementTool(t toolMode) bool {
	switch t {
	case toolBuilding, toolTicketOffice, toolSnowGun:
		return true
	}
	return false
}

// placeBuilding places a building of typ at (x, z) turned by rotation.
// The caller checks overlap and applies placement effects.
func placeBuilding(w *world.World, typ world.BuildingType, x, z, rotation float32) *world.Building {
	b := w.PlaceBuildingType(typ, x, z)
	b.Rotation = rotation
	return b
}

// rotateSelectedBuilding turns the selected building by delta in place
// and regrades its pad. Returns false (leaving it untouched) when the
// turned footprint would overlap another building.
func rotateSelectedBuilding(r *render.Renderer, w *world.World, sel *structureEditSelection, delta float32) bool {
	b := sel.building
	if b == nil || b.IsPainted() {
		return false
	}
	rot := stepRotation(b.Rotation, delta)
	if w.BuildingOverlapExcept(b.Type, b.Pos[0], b.Pos[1], rot, b.ID) {
		return false
	}
	b.Rotation = rot
	if b.Type == world.BuildingParking {
		refreshParkingDriveways(w, b)
		r.RebuildRoads(w)
	}
	applyBuildingPlacementEffects(w, b)
	r.FlushTerrainVerts(w.Terrain)
	r.RebuildStaticBatch(w)
	return true
}

// commitStructureDrag finalises a drag — pushes the static-batch +
// roads rebuilds that were deferred during the move for cost reasons.
func commitStructureDrag(r *render.Renderer, w *world.World) {
	r.RebuildStaticBatch(w)
}

// deleteSelectedStructure removes the selected building or lift and
// clears the selection. RemoveBuilding / RemoveLift restore Passable on
// the freed cells; the apron's ground / snow / tree effects stay where
// the structure was, since there's no Natural baseline to revert to.
// The player accepts a left-behind apron until a future rebuild lands.
func deleteSelectedStructure(r *render.Renderer, w *world.World, sel *structureEditSelection) {
	switch {
	case sel.building != nil:
		wasParking := sel.building.Type == world.BuildingParking
		w.RemoveBuilding(sel.building.ID)
		sel.clear()
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
		if wasParking {
			r.RebuildRoads(w)
		}
	case sel.lift != nil:
		liftID := sel.lift.ID
		sel.clear()
		w.RemoveLift(liftID)
		r.RemoveLiftCable(liftID)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
	}
}

// refreshParkingDriveways updates a moved parking lot's driveway road
// nodes so they track the new pad position. Each DrivewayNodeIDs[i]
// resolves to a live road node whose Pos is rewritten to the
// corresponding slot's new world position. Edges incident to the node
// stay attached — the player's road network "flexes" with the move.
func refreshParkingDriveways(w *world.World, b *world.Building) {
	positions := w.DrivewayPositions(b)
	for i, id := range b.DrivewayNodeIDs {
		if i >= len(positions) {
			break
		}
		if n := w.RoadNodeByID(id); n != nil {
			n.Pos = positions[i]
		}
	}
}

// emitStructureEditMarkers renders selection feedback for the
// structure edit tool. A muted ghost marker sits at every building
// and lift station, the selected one in the hot edit tint. Mirrors
// the road-edit markers so the player has consistent visual language
// for "this is a draggable handle".
//
// Uses the MeshLiftStation ghost batch for lift stations and an
// adapted marker for buildings — keeps it lightweight. Actually we
// piggy-back on MeshRoadNode for both: it's a generic cylinder
// already wired into the ghost path, and using the same mesh keeps
// the user's mental model simple ("yellow / orange handles = drag").
func emitStructureEditMarkers(r *render.Renderer, w *world.World, sel *structureEditSelection) {
	if !sel.active() {
		// Don't clear the road-node ghost batch here — road edit owns
		// that channel. Structure edit instead reuses the lift-station
		// ghost batch (already a single instance per station in the
		// active road tool flow) but we only set markers when
		// something's selected. No-op otherwise.
		return
	}
	// For now selection is purely positional — the road-edit marker
	// scaling already lives in road_edit.go and produces oversized
	// posts. Reuse it: a single marker at the building / station Pos.
	if sel.building != nil {
		inst := roadEditMarkerInstance(sel.building.Pos, w.Terrain, roadEditSelectionTint, roadEditSelectionRadiusScale, roadEditSelectionHeightScale)
		r.SetGhosts(render.MeshRoadNode, []render.StaticInstance{inst})
		return
	}
	if sel.lift != nil {
		anchor := sel.lift.Base
		if sel.liftEnd == 1 {
			anchor = sel.lift.Top
		}
		inst := roadEditMarkerInstance(anchor, w.Terrain, roadEditSelectionTint, roadEditSelectionRadiusScale, roadEditSelectionHeightScale)
		r.SetGhosts(render.MeshRoadNode, []render.StaticInstance{inst})
	}
}
