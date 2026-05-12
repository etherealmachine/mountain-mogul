package scene

import (
	"mountain-mogul/internal/world"
)

// rebuildTerrainFromNatural is the single source of truth for keeping
// the cell-level display state consistent with the structures in the
// world. It resets every cell's display fields from its Natural shadow
// (the "no structures present" baseline), then re-stamps:
//
//  1. Road clearance (snow falloff + tree clear inside the carriageway
//     band)
//  2. Building footprints (apron flattening + plowed pad + tree clear)
//  3. Lift station aprons + cable corridor
//
// Order matters only on overlap: later passes overwrite earlier ones.
// The current order (road → building → lift) lets a building or lift
// flatten the cells under a road that crosses its pad — which is what
// the renderer expects today.
//
// Sim-only fields (Grooming, Packed, Ice, MogulSize) are NOT reset —
// they're runtime accumulations that the natural layer doesn't track,
// and zeroing them on every structure edit would erase mid-game state.
//
// Called by every code path that mutates the structure graph: road
// placement, road edit (drag / delete), building placement & removal,
// lift placement & removal, and the non-structure paint paths after
// they've already updated the Natural fields.
func rebuildTerrainFromNatural(w *world.World) {
	if w == nil || w.Terrain == nil {
		return
	}
	w.Terrain.ResetDisplayFromNatural()
	applyRoadCellState(w)
	for _, b := range w.Buildings {
		applyBuildingPlacementEffects(w.Terrain, b)
	}
	for _, l := range w.Lifts {
		applyLiftPlacementEffects(w.Terrain, l)
	}
}
