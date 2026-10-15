//go:build !demo

package scene

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// Railways in the scenario editor: the game's Rail and Station tools and
// track editing (rail_tool.go), free; a track's popup sets its style.

// railEnv is the editor's railway tools' env.
func (e *Editor) railEnv() railEnv {
	r := e.app.Renderer
	return railEnv{
		w: e.world, r: r, tool: &e.railTool, edit: &e.railEdit, active: e.activeTool,
		hover: mgl32.Vec2{e.hoverWorld[0], e.hoverWorld[2]}, hoverValid: e.hoverValid,
		toast: e.setToast, free: true,
		changed: func() {
			e.layerCache.fields = nil
			e.markDirty()
		},
		show: func(win *ui.Window) {
			e.closePopups()
			win.Center(r.ScreenWidth(), r.ScreenHeight())
			e.railPopup = win
		},
		up: func(win *ui.Window) bool { return e.railPopup == win },
	}
}

// closePopups hides the editor's popups.
func (e *Editor) closePopups() {
	for _, p := range []*ui.Window{e.parcelPopup, e.lotPopup, e.entryPopup, e.trailPopup, e.railPopup, e.liftPopup} {
		if p != nil {
			p.Visible = false
		}
	}
}

// startRailTool picks style and turns the Rail tool on (off if it's on
// with that style).
func (e *Editor) startRailTool(style world.RailStyle) {
	if e.activeTool == toolRail && e.railTool.style == style {
		e.setTool(toolRail) // off
		return
	}
	if e.activeTool != toolRail {
		e.setTool(toolRail)
	}
	e.railTool = railTool{style: style}
	e.syncToolButtons()
	e.setToast("Click to lay track; finish on a joint or another track, at the map's edge (click off the map), or click the last point again.")
}

// updateRailTools runs a frame of the railway tools: the ghosts, clicks
// with the Rail and Station tools, and with no tool the selected track's
// handles; whether the mouse went to them.
func (e *Editor) updateRailTools(inp *engine.Input, overChrome bool) bool {
	env := e.railEnv()
	switch e.activeTool {
	case toolRail:
		if !overChrome {
			env.offMap(inp.MousePos) // off the map: at its edge
		}
		env.updateGhost()
		if inp.LeftClick && !inp.LeftClickConsumed && env.hoverValid && !overChrome {
			env.click(env.hover)
			inp.LeftClickConsumed = true
		}
	case toolStation:
		env.updateStationGhost()
		if inp.LeftClick && !inp.LeftClickConsumed && e.hoverValid && !overChrome {
			env.stationClick(env.hover)
			inp.LeftClickConsumed = true
		}
	case toolNone:
		if e.hoverValid && !overChrome || e.railEdit.drag.ok {
			return env.editInput(inp)
		}
	}
	return false
}

// endRailTool drops the track being laid and its ghost.
func (e *Editor) endRailTool(r *render.Renderer) {
	e.railTool = railTool{style: e.railTool.style}
	r.SetRailGhost(nil, nil, [3]float32{})
}
