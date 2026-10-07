package scene

import (
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// Terrain brushes in the scenario editor: smooth, flatten, raise, and
// lower the ground (world.Terrain.ApplyBrush) under the cursor while the
// mouse is held, at the radius and strength sliders' settings.
//
// They edit the ground the map has now. Rebuilding the terrain layers
// (the Layers panel) starts again from the import and loses these edits.

// terrainStroke is a brush stroke in progress: from mouse-down to
// mouse-up.
type terrainStroke struct {
	active bool
	target float32 // flatten: the ground height where the stroke began
	// dirty marks ground changed since the renderer last saw it, and
	// sinceFlush the seconds since it did: the whole terrain is
	// re-uploaded, so mid-stroke that happens a few times a second, and
	// once more at the end.
	dirty      bool
	sinceFlush float32
}

// strokeFlushSec is how often the terrain is re-uploaded mid-stroke.
const strokeFlushSec = 0.2

// brushStrengthPerSec scales the strength slider to a rate, so a stroke
// does as much at any frame rate: at 100% the brush goes about this many
// times the way to its target each second at its centre.
const brushStrengthPerSec = 4.0

func (e *Editor) isTerrainBrush() bool {
	switch e.activeTool {
	case toolSmooth, toolFlatten, toolRaise, toolLower:
		return true
	}
	return false
}

// applyTerrainBrush applies the active brush at the cursor for one frame
// of a held stroke.
func (e *Editor) applyTerrainBrush(r *render.Renderer, dt float32) {
	t := e.world.Terrain
	pos := e.hoverWorld
	if !e.stroke.active {
		e.stroke = terrainStroke{active: true, target: t.GroundHeightAt(pos[0], pos[2])}
	}
	brush := map[toolMode]world.TerrainBrush{
		toolSmooth: world.BrushSmooth, toolFlatten: world.BrushFlatten,
		toolRaise: world.BrushRaise, toolLower: world.BrushLower,
	}[e.activeTool]
	strength := min(e.strengthSlider.Value/100*brushStrengthPerSec*dt, 1)
	_, _, _, _, ok := t.ApplyBrush(world.BrushStroke{
		Brush:    brush,
		X:        pos[0],
		Z:        pos[2],
		Radius:   (float32(e.brushRadius()) + 0.5) * world.CellSize,
		Strength: strength,
		Target:   e.stroke.target,
	})
	if !ok {
		return
	}
	e.stroke.dirty = true
	e.stroke.sinceFlush += dt
	if e.stroke.sinceFlush >= strokeFlushSec {
		e.flushGround(r, false)
	}
}

// endTerrainStroke finishes a stroke when the mouse comes up: the ground
// goes to the renderer once more, with everything that sits on it.
func (e *Editor) endTerrainStroke(r *render.Renderer) {
	if !e.stroke.active {
		return
	}
	if e.stroke.dirty {
		e.flushGround(r, true)
	}
	e.stroke = terrainStroke{}
}

// flushGround sends changed ground to the renderer; final also rebuilds
// what sits on the ground (trees, roads, structures) and the editor's
// cached elevation fields.
func (e *Editor) flushGround(r *render.Renderer, final bool) {
	e.markDirty()
	e.stroke.dirty, e.stroke.sinceFlush = false, 0
	if r == nil {
		return
	}
	t := e.world.Terrain
	r.FlushTerrainVerts(t)
	if final {
		e.layerCache = layerCache{}
		r.BuildMogulTex(t)
		r.RebuildStaticBatch(e.world)
		r.RebuildRoads(e.world)
	}
}
