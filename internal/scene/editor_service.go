package scene

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"math/rand"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// The editor's building tool: the game's (service_tools.go), free and on
// any land. The Buildings menu picks what a new building is built as
// (lodge, tent, shed) and the service to paint; clicking a building with
// no tool opens its popup.

// serviceEnv is the editor's build-tool surroundings.
func (e *Editor) serviceEnv() serviceEnv {
	return serviceEnv{
		w: e.world, r: e.app.Renderer, free: true, toast: e.setToast,
		changed: func() {
			e.markDirty()
			e.layerCache.fields = nil // grading moved ground
		},
		delete: func(id uint64) {
			removePaintedBuilding(e.app.Renderer, e.world, id)
			e.markDirty()
		},
	}
}

// activateServiceTool starts painting tiles of svc; picking the active
// service again ends it.
func (e *Editor) activateServiceTool(svc world.Service) {
	if e.activeTool == toolService && e.serviceTool.svc == svc {
		e.setTool(toolService) // toggles off
		return
	}
	if e.activeTool != toolService {
		e.setTool(toolService)
	}
	e.serviceTool = serviceTool{svc: svc, kind: e.newShellKind, seed: rand.Uint32()}
	e.syncToolButtons()
	e.setToast(fmt.Sprintf("%s in a %s: click ground to start a building (R turns it), a wall to extend, a roof to switch; right-click removes.",
		svc.Label(), e.newShellKind.Label()))
}

// setNewShellKind picks what new buildings are built as, keeping the
// service being painted (lounge if none).
func (e *Editor) setNewShellKind(k world.ShellKind) {
	e.newShellKind = k
	svc := world.ServiceLounge
	if e.activeTool == toolService {
		svc = e.serviceTool.svc
	}
	if e.activeTool == toolService {
		e.serviceTool.kind = k
		e.syncToolButtons()
		return
	}
	e.activateServiceTool(svc)
}

// openShellPopup opens a service building's popup: what it's built as,
// storeys for a lodge, its tiles, garage vehicles, and delete.
func (e *Editor) openShellPopup(id uint64, confirmDelete bool, screenW, screenH int) {
	b := e.world.BuildingByID(id)
	if b == nil {
		return
	}
	w := e.world
	win := ui.NewWindow(b.Label(), 0, 0)
	win.AddLabel("Built as", func() string { return b.Kind.Label() })
	if b.Kind.MaxStoreys() > 1 {
		win.AddIntStepperFn("Storeys", func() string { return fmt.Sprintf("%d", b.Floors()) },
			func() { e.setStoreys(b, b.Floors()-1) }, func() { e.setStoreys(b, b.Floors()+1) })
	}
	for sv := world.ServiceLounge; sv < world.ServiceCount; sv++ {
		if n := b.TileCount(sv); n > 0 {
			win.AddLabel(sv.Label(), func() string { return fmt.Sprintf("%d tiles", n) })
		}
	}
	if b.TileCount(world.ServiceGarage) > 0 {
		win.AddLabel("Garage", func() string {
			used, total := w.GarageSpace(b)
			return fmt.Sprintf("%d snowcats, %d snowmobiles; %s of %s tiles", len(w.CatsOwnedBy(b.ID)), len(w.SnowmobilesIn(b)), halfTiles(used), halfTiles(total))
		})
		win.AddActionButton("Add snowcat", func() { e.addVehicle(b, true) })
		win.AddActionButton("Add snowmobile", func() { e.addVehicle(b, false) })
		win.AddActionButton("Remove snowcat", func() {
			if cats := w.CatsOwnedBy(b.ID); len(cats) > 0 {
				w.RemoveSnowcat(cats[len(cats)-1].ID)
				e.markDirty()
			}
		})
		win.AddActionButton("Remove snowmobile", func() {
			if ms := w.SnowmobilesIn(b); len(ms) > 0 {
				w.RemoveSnowmobile(ms[len(ms)-1].ID)
				e.markDirty()
			}
		})
	}
	win.AddActionButton("New style", func() {
		b.StyleSeed = rand.Uint32()
		e.app.Renderer.RebuildStaticBatch(w)
		e.markDirty()
	})
	if confirmDelete {
		win.AddActionButton("Confirm delete", func() {
			removePaintedBuilding(e.app.Renderer, w, id)
			e.markDirty()
			win.Visible = false
		})
		win.AddActionButton("Cancel", func() { e.openShellPopup(id, false, screenW, screenH) })
	} else {
		win.AddActionButton("Delete building", func() { e.openShellPopup(id, true, screenW, screenH) })
	}
	win.Visible = true
	win.Center(screenW, screenH)
	e.lotPopup = win // the editor's one building popup
}

// addVehicle puts a snowcat (cat) or snowmobile in b's garage if it fits.
func (e *Editor) addVehicle(b *world.Building, cat bool) {
	w := e.world
	half, name := world.SnowmobileGarageHalfTiles, "snowmobile"
	if cat {
		half, name = world.CatGarageHalfTiles, "snowcat"
	}
	if !w.GarageFits(b, half) {
		used, total := w.GarageSpace(b)
		e.setToast(fmt.Sprintf("No room: a %s needs %s free tiles, and this garage has %s", name, halfTiles(half), halfTiles(total-used)))
		return
	}
	if cat {
		w.SpawnSnowcat(b)
	} else {
		w.AddSnowmobile(b)
	}
	e.markDirty()
}

// setStoreys sets a lodge's storeys (free in the editor).
func (e *Editor) setStoreys(b *world.Building, n int) {
	n = max(1, min(n, b.Kind.MaxStoreys()))
	if n == b.Floors() {
		return
	}
	b.Storeys = n
	e.app.Renderer.RebuildStaticBatch(e.world)
	e.markDirty()
}

// updateServiceTool runs the building tool for a frame: the pick under
// the mouse and its ghost.
func (e *Editor) updateServiceTool(mouse mgl32.Vec2, covered bool) {
	if e.activeTool != toolService {
		return
	}
	env := e.serviceEnv()
	env.updatePick(&e.serviceTool, mouse, covered, e.hoverWorld, e.hoverValid)
	env.ghost(&e.serviceTool)
}
