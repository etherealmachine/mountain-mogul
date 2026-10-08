package scene

import (
	"fmt"
	"math/rand"

	"github.com/go-gl/glfw/v3.3/glfw"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// The editor's building and service tools: the game's (service_tools.go),
// free and on any land. The Buildings menu drags out empty buildings
// (lodge, tent, shed) and the Services menu puts services in them;
// clicking a building with no tool opens its popup.

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

// activateServiceTool starts putting svc in buildings' tiles; picking
// the active service again ends it.
func (e *Editor) activateServiceTool(svc world.Service) {
	if e.activeTool == toolService && !e.serviceTool.building && e.serviceTool.svc == svc {
		e.setTool(toolService) // toggles off
		return
	}
	e.startServiceTool(serviceTool{svc: svc})
	e.setToast(fmt.Sprintf("%s: click or drag over a building's tiles; right-click empties a tile.", svc.Label()))
}

// setNewShellKind starts the building tool for kind k; picking the
// active kind again ends it.
func (e *Editor) setNewShellKind(k world.ShellKind) {
	if e.activeTool == toolService && e.serviceTool.building && e.serviceTool.kind == k {
		e.setTool(toolService) // toggles off
		return
	}
	e.startServiceTool(serviceTool{building: true, kind: k, seed: rand.Uint32()})
	e.setToast(fmt.Sprintf("%s: drag out its floor (R turns it), then click inside or press Enter to build. Drag from a wall to extend; right-click a tile to remove it.", k.Label()))
}

func (e *Editor) startServiceTool(st serviceTool) {
	if e.activeTool != toolService {
		e.setTool(toolService)
	}
	e.serviceTool = st
	e.syncToolButtons()
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
	for _, room := range b.Rooms() {
		n := len(room.Cells)
		win.AddLabel(room.Service.Label(), func() string { return fmt.Sprintf("%d tiles", n) })
	}
	if n := b.EmptyTiles(); n > 0 {
		win.AddLabel("Empty floor", func() string { return fmt.Sprintf("%d tiles", n) })
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

// updateServiceTool runs the building and service tools for a frame.
func (e *Editor) updateServiceTool(inp *engine.Input, covered bool) {
	if e.activeTool != toolService {
		return
	}
	env := e.serviceEnv()
	env.input(&e.serviceTool, toolInput{
		mouse: inp.MousePos, covered: covered,
		ground: e.hoverWorld, groundValid: e.hoverValid,
		leftClick: inp.LeftClick && !inp.LeftClickConsumed, leftHeld: inp.LeftHeld,
		rightClick: inp.RightClick, rightRelease: inp.RightRelease,
		enter: inp.Pressed[glfw.KeyEnter] || inp.Pressed[glfw.KeyKPEnter],
	})
	env.ghost(&e.serviceTool)
}
