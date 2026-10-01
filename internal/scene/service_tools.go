package scene

import (
	"fmt"
	"math"
	"strings"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// Service buildings are built a tile at a time: pick a service, click
// the ground to start a building with one tile of it, click a wall to
// add a tile on that side, click a roof to switch that tile's service,
// and right-click a tile to remove it. Doors are automatic
// (world.RefreshDoors). The data model lives in world/lodge.go; the tile
// kit in world/lodge_shell.go.

type servicePickKind uint8

const (
	pickNone    servicePickKind = iota
	pickNew                     // start a building on the ground
	pickAdd                     // add a tile to bldg at cell
	pickRepaint                 // switch bldg's tile at cell to the tool's service
)

// servicePick is what a left click would do under the cursor, plus the
// tile under it for right-click removal.
type servicePick struct {
	kind   servicePickKind
	bldg   uint64
	cell   [2]int
	legal  bool
	reason string // why a pick is illegal, for the toast

	hitBldg uint64 // building whose tile the cursor is over (0 for none)
	hitCell [2]int
}

// serviceTool is the build session for toolService.
type serviceTool struct {
	svc       world.Service
	seed      uint32 // style for a new building, so the ghost matches
	pick      servicePick
	rightDown mgl32.Vec2 // where the right button went down
}

// rayBox intersects a ray with an axis-aligned box. It returns the entry
// distance and the outward normal axis of the entry face (0 x, 1 y, 2 z)
// and its sign.
func rayBox(o, d, lo, hi mgl32.Vec3) (t float32, axis int, sign float32, ok bool) {
	tmin, tmax := float32(math.Inf(-1)), float32(math.Inf(1))
	for i := 0; i < 3; i++ {
		if d[i] == 0 {
			if o[i] < lo[i] || o[i] > hi[i] {
				return 0, 0, 0, false
			}
			continue
		}
		t1, t2 := (lo[i]-o[i])/d[i], (hi[i]-o[i])/d[i]
		s := float32(-1)
		if t1 > t2 {
			t1, t2 = t2, t1
			s = 1
		}
		if t1 > tmin {
			tmin, axis, sign = t1, i, s
		}
		tmax = min(tmax, t2)
	}
	if tmin > tmax || tmax < 0 {
		return 0, 0, 0, false
	}
	return tmin, axis, sign, true
}

// tileBox is the pick volume of shell tile c: the cell up to the ridge
// of a one-tile roof.
func tileBox(floor float32, c [2]int) (lo, hi mgl32.Vec3) {
	const cell = float32(5)
	lo = mgl32.Vec3{float32(c[0]) * cell, floor, float32(c[1]) * cell}
	hi = mgl32.Vec3{lo[0] + cell, floor + world.ShellWallHeight + world.ShellRoofRise, lo[2] + cell}
	return lo, hi
}

// pickService resolves the cursor ray against the shells first, falling
// back to the terrain hit (ground, groundValid) for a new tile.
func pickService(w *world.World, o, d mgl32.Vec3, svc world.Service, ground mgl32.Vec3, groundCell [2]int, groundValid bool) servicePick {
	best := float32(math.Inf(1))
	if groundValid {
		best = ground.Sub(o).Len()
	}
	var pick servicePick
	for _, b := range w.Buildings {
		if !b.IsShell() {
			continue
		}
		floor := w.ShellFloorY(b)
		for _, c := range b.Cells {
			lo, hi := tileBox(floor, c)
			t, axis, sign, ok := rayBox(o, d, lo, hi)
			if !ok || t >= best {
				continue
			}
			best = t
			pick = servicePick{bldg: b.ID, cell: c, hitBldg: b.ID, hitCell: c}
			switch axis {
			case 1:
				pick.kind = pickRepaint
			case 0:
				pick.kind, pick.cell = pickAdd, [2]int{c[0] + int(sign), c[1]}
			default:
				pick.kind, pick.cell = pickAdd, [2]int{c[0], c[1] + int(sign)}
			}
		}
	}
	if pick.kind == pickNone {
		if !groundValid {
			return pick
		}
		pick = servicePick{kind: pickNew, cell: groundCell}
		if b := shellBeside(w, groundCell); b != nil {
			pick.kind, pick.bldg = pickAdd, b.ID
		}
	}
	pick.legal, pick.reason = pickLegal(w, pick, svc)
	return pick
}

// shellBeside returns a service building with a tile 4-adjacent to c.
func shellBeside(w *world.World, c [2]int) *world.Building {
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		if b := w.PaintedBuildingAt(c[0]+d[0], c[1]+d[1]); b != nil && b.IsShell() {
			return b
		}
	}
	return nil
}

func pickLegal(w *world.World, p servicePick, svc world.Service) (bool, string) {
	switch p.kind {
	case pickRepaint:
		b := w.BuildingByID(p.bldg)
		if b == nil || b.ServiceAt(p.cell) == svc {
			return false, ""
		}
		return true, ""
	case pickNew, pickAdd:
		c := p.cell
		t := w.Terrain
		if !t.InBounds(c[0], c[1]) {
			return false, ""
		}
		if !t.IsAccessible(c[0], c[1]) {
			return false, "Can't build on land you don't own"
		}
		if !w.PaintedCellFree(c, p.bldg) || !t.Cells[c[0]][c[1]].Passable {
			return false, "Something's already built there"
		}
		if p.kind == pickAdd {
			if b := w.BuildingByID(p.bldg); b != nil && b.HasCell(c) {
				return false, ""
			}
		}
		return true, ""
	}
	return false, ""
}

// pickCost is what applying p with service svc charges.
func pickCost(p servicePick, svc world.Service) int {
	if p.kind == pickNew {
		return world.ServiceBuildingCost(svc, 1)
	}
	return svc.TileCost()
}

// activateServiceTool starts building tiles of svc. Picking the active
// service again ends the session.
func (s *Scenario) activateServiceTool(svc world.Service) {
	if s.activeTool == toolService && s.serviceTool.svc == svc {
		s.cancelTool()
		return
	}
	if s.activeTool != toolNone {
		s.cancelTool()
	}
	s.roadEdit.clear()
	s.structureEdit.clear()
	s.serviceTool = serviceTool{svc: svc, seed: rng.Global().Uint32()}
	s.activeTool = toolService
	s.syncToolButtons()
	if s.popup != nil {
		s.popup.Visible = false
	}
	s.setToast(fmt.Sprintf("%s: $%s a tile. Click ground to start a building ($%s), a wall to extend, a roof to switch; right-click removes. Esc to finish.",
		svc.Label(), formatDollars(svc.TileCost()), formatDollars(world.ServiceBuildingCost(svc, 1))))
}

// updateServicePick refreshes the tool's pick under the mouse.
func (s *Scenario) updateServicePick(r *render.Renderer, mouse mgl32.Vec2) {
	st := &s.serviceTool
	if s.barsContain(mouse[1]) || s.uiCovers(mouse[0], mouse[1], float32(r.ScreenWidth())) {
		st.pick = servicePick{}
		return
	}
	o, d := r.Camera.ScreenToWorldRay(mouse)
	st.pick = pickService(s.world, o, d, st.svc, s.hoverWorld, s.hoverCell, s.hoverValid)
}

// applyServicePick runs the left-click action under the cursor.
func (s *Scenario) applyServicePick() {
	w := s.world
	st := &s.serviceTool
	p := st.pick
	if p.kind == pickNone {
		return
	}
	if !p.legal {
		if p.reason != "" {
			s.setToast(p.reason)
		}
		return
	}
	cost := pickCost(p, st.svc)
	if !w.CanAfford(cost) {
		s.setToast(fmt.Sprintf("Need $%s for a %s tile — short by $%s",
			formatDollars(cost), strings.ToLower(st.svc.Label()), formatDollars(cost-w.Available())))
		return
	}
	w.Cash -= cost
	switch p.kind {
	case pickNew:
		b := w.PlaceServiceBuilding(map[[2]int]world.Service{p.cell: st.svc}, st.seed)
		st.seed = rng.Global().Uint32()
		s.sim.LogBuildingPlaced(b)
		s.finishServiceEdit(b, true)
	case pickAdd:
		b := w.BuildingByID(p.bldg)
		w.SetTileService(b, p.cell, st.svc)
		s.finishServiceEdit(b, true)
	case pickRepaint:
		b := w.BuildingByID(p.bldg)
		w.SetTileService(b, p.cell, st.svc)
		s.finishServiceEdit(b, false)
	}
}

// removeServiceTile removes the tile under the cursor; the last tile
// takes the building with it.
func (s *Scenario) removeServiceTile() {
	p := s.serviceTool.pick
	b := s.world.BuildingByID(p.hitBldg)
	if b == nil {
		return
	}
	if len(b.Cells) == 1 {
		s.deletePaintedBuilding(b.ID)
		return
	}
	s.world.SetTileService(b, p.hitCell, world.ServiceNone)
	s.finishServiceEdit(b, false)
}

// finishServiceEdit regrades a grown footprint to the building's floor
// and rebuilds the tiles and the trail graph, which refreshes the doors.
func (s *Scenario) finishServiceEdit(b *world.Building, regrade bool) {
	r := s.app.Renderer
	w := s.world
	if regrade {
		gradePaintedPad(w, b, 0)
		applyRoadCellState(w)
		r.FlushTerrainVerts(w.Terrain)
	}
	w.RebuildTrailGraph()
	r.RebuildStaticBatch(w)
}

// serviceGhost previews the pick: a lone tile of the tool's service at
// the target cell, red when the click can't go ahead.
func (s *Scenario) serviceGhost(r *render.Renderer) {
	w := s.world
	st := &s.serviceTool
	p := st.pick
	if p.kind == pickNone {
		return
	}
	seed := st.seed
	floor := w.Terrain.GroundElevationAt(p.cell[0], p.cell[1])
	if b := w.BuildingByID(p.bldg); b != nil {
		seed, floor = b.StyleSeed, w.ShellFloorY(b)
	}
	tint := [3]float32{0.6, 1.0, 0.6}
	if !p.legal || !w.CanAfford(pickCost(p, st.svc)) {
		tint = [3]float32{1.0, 0.4, 0.4}
	}
	scale := float32(1)
	if p.kind == pickRepaint {
		scale = 1.04
	}
	r.SetShellGhost(world.PreviewTile(p.cell, st.svc, seed), floor, scale, tint)
}

// serviceOverlayColor is a service's floor-plan colour in the overlay.
func serviceOverlayColor(sv world.Service) (r, g, b uint8) {
	switch sv {
	case world.ServiceFood:
		return 235, 150, 50
	case world.ServiceBar:
		return 190, 80, 120
	case world.ServiceTickets:
		return 80, 140, 230
	}
	return 150, 120, 90
}

// appendServiceOverlay tints b's floor plan by service, doors brighter.
func appendServiceOverlay(b *world.Building, set func(cx, cz int, r, g, b, a uint8)) {
	if b == nil {
		return
	}
	for _, c := range b.Cells {
		r, g, bl := serviceOverlayColor(b.ServiceAt(c))
		set(c[0], c[1], r, g, bl, 170)
	}
	for _, d := range b.Doors {
		n := [2]int{d.Cell[0] + d.Dir[0], d.Cell[1] + d.Dir[1]}
		set(n[0], n[1], 80, 210, 120, 200)
	}
}

// editedLodge is the service building whose popup is open — drawn as a
// cutaway with its floor plan overlaid.
func (s *Scenario) editedLodge() *world.Building {
	if s.popup == nil || !s.popup.Visible || s.selectedBuildingID == 0 {
		return nil
	}
	if b := s.world.BuildingByID(s.selectedBuildingID); b != nil && b.IsShell() {
		return b
	}
	return nil
}

// buildLodgePopup builds (or rebuilds) a service building's popup.
// confirmDelete swaps the Delete button for Confirm / Cancel.
func (s *Scenario) buildLodgePopup(b *world.Building, confirmDelete bool, screenW, screenH int) {
	w := ui.NewWindow(b.Label(), 0, 0)
	for sv := world.ServiceLounge; sv < world.ServiceCount; sv++ {
		if n := b.TileCount(sv); n > 0 {
			w.AddLabel(sv.Label(), func() string { return fmt.Sprintf("%d tiles", n) })
		}
	}
	w.AddLabel("Doors", func() string {
		if len(b.Doors) == 0 {
			return "none — no open wall"
		}
		return fmt.Sprintf("%d", len(b.Doors))
	})
	if b.TileCount(world.ServiceFood) > 0 {
		w.AddLabel("Diners", func() string { return fmt.Sprintf("%d / %d seats", b.Diners, b.Seats()) })
		w.AddIntStepper("Meal price ($)", &b.MealPrice, 1, 0, 100)
	}
	if b.TileCount(world.ServiceBar) > 0 {
		w.AddIntStepper("Drink price ($)", &b.DrinkPrice, 1, 0, 50)
	}
	w.AddLabel("Daily cost", func() string {
		return fmt.Sprintf("$%s/day", formatDollars(world.LodgeUpkeep(b)))
	})
	w.AddLabel("Inbound", func() string {
		count := 0
		for _, a := range s.world.OnMountain {
			if a.TargetID == b.ID {
				count++
			}
		}
		return fmt.Sprintf("%d", count)
	})
	if b.Offers(world.ServiceTickets) {
		s.addResortControls(w, func() { s.buildLodgePopup(b, false, screenW, screenH) })
	}
	w.AddActionButton("New style", func() {
		b.StyleSeed = rng.Global().Uint32()
		s.app.Renderer.RebuildStaticBatch(s.world)
	})
	if confirmDelete {
		w.AddLabel("Confirm", func() string { return "Delete this building?" })
		w.AddActionButton("Confirm", func() { s.deletePaintedBuilding(b.ID) })
		w.AddActionButton("Cancel", func() { s.buildLodgePopup(b, false, screenW, screenH) })
	} else {
		w.AddActionButton("Delete building", func() { s.buildLodgePopup(b, true, screenW, screenH) })
	}
	w.Visible = true
	w.Center(screenW, screenH)
	s.popup = w
}

// addResortControls adds the resort-wide controls sold from a ticket
// window: open/close, lift hours and ticket prices. rebuild reopens the
// popup so the open/close label tracks the state.
func (s *Scenario) addResortControls(w *ui.Window, rebuild func()) {
	toggleLabel := "Open resort"
	if s.world.ResortOpen {
		toggleLabel = "Close resort"
	}
	w.AddActionButton(toggleLabel, func() {
		s.sim.SetResortOpen(!s.world.ResortOpen)
		rebuild()
	})
	const hourStep, minOpenSpan = 0.5, 2
	w.AddIntStepperFn("Lifts open",
		func() string { return settings.FormatClock(float64(s.world.OpenHour)) },
		func() { s.world.OpenHour = max(s.world.OpenHour-hourStep, 5) },
		func() { s.world.OpenHour = min(s.world.OpenHour+hourStep, s.world.CloseHour-minOpenSpan) })
	w.AddIntStepperFn("Lifts close",
		func() string { return settings.FormatClock(float64(s.world.CloseHour)) },
		func() { s.world.CloseHour = max(s.world.CloseHour-hourStep, s.world.OpenHour+minOpenSpan) },
		func() { s.world.CloseHour = min(s.world.CloseHour+hourStep, 23) })
	for _, l := range s.world.Lifts {
		l := l
		w.AddLabel(l.Name+" base", func() string { return liftBaseSnowText(s.world, l) })
	}
	w.AddIntStepper("Day ticket ($)", &s.world.DayTicketPrice, 5, 0, 500)
	w.AddIntStepper("Pass price ($)", &s.world.SeasonPassPrice, 10, 0, 1000)
	w.AddLabel("Pass holders", func() string {
		count := 0
		for _, a := range s.world.OnMountain {
			if a.HasSeasonPass {
				count++
			}
		}
		return fmt.Sprintf("%d on mountain", count)
	})
}
