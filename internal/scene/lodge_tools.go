package scene

import (
	"fmt"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// Painted lodges: the player paints the shell cell by cell, marks doors
// on its outer cells and paints a food court inside it. The data model
// lives in world/lodge.go; the tile kit in world/lodge_shell.go.

type lodgeEditMode uint8

const (
	lodgeEditShell lodgeEditMode = iota // paint / erase shell cells
	lodgeEditDoors                      // click outer cells to toggle doors
	lodgeEditFood                       // paint / erase food-court cells
)

// lodgePaint is the edit session for toolLodge. lodgeID is 0 until the
// first shell cell is painted, so an abandoned session leaves nothing.
type lodgePaint struct {
	lodgeID  uint64
	mode     lodgeEditMode
	erase    bool
	lastCell [2]int // {-1,-1} between strokes
	// shellDirty means the footprint changed (regrade on commit);
	// detailDirty means only doors or the food court did.
	shellDirty, detailDirty bool
}

func (p *lodgePaint) start(id uint64, mode lodgeEditMode, erase bool) {
	*p = lodgePaint{lodgeID: id, mode: mode, erase: erase, lastCell: [2]int{-1, -1}}
}

func (p *lodgePaint) lodge(w *world.World) *world.Building {
	if p.lodgeID == 0 {
		return nil
	}
	for _, b := range w.Buildings {
		if b.ID == p.lodgeID {
			return b
		}
	}
	return nil
}

// shellAddable reports whether cell c can join the session's shell.
func (p *lodgePaint) shellAddable(w *world.World, c [2]int) bool {
	if b := p.lodge(w); b != nil && b.HasCell(c) {
		return false
	}
	return w.PaintedCellFree(c, p.lodgeID)
}

// addShellCell grows the shell by c, creating the lodge on first use.
// Returns the new lodge when this call created it.
func (p *lodgePaint) addShellCell(w *world.World, c [2]int) (created *world.Building) {
	p.shellDirty = true
	if b := p.lodge(w); b != nil {
		w.SetShellCells(b, append(append([][2]int(nil), b.Cells...), c))
		return nil
	}
	b := w.PlaceLodgeShell([][2]int{c}, rng.Global().Uint32())
	p.lodgeID = b.ID
	return b
}

func (p *lodgePaint) removeShellCell(w *world.World, c [2]int) {
	b := p.lodge(w)
	if b == nil || !b.HasCell(c) {
		return
	}
	keep := make([][2]int, 0, len(b.Cells))
	for _, k := range b.Cells {
		if k != c {
			keep = append(keep, k)
		}
	}
	w.SetShellCells(b, keep)
	p.shellDirty = true
}

// setFoodCourt adds or removes food-court cell c; false when nothing changed.
func (p *lodgePaint) setFoodCourt(w *world.World, c [2]int, on bool) bool {
	b := p.lodge(w)
	if b == nil || !b.HasCell(c) || b.HasFoodCourt(c) == on {
		return false
	}
	cells := make([][2]int, 0, len(b.FoodCourtCells)+1)
	for _, k := range b.FoodCourtCells {
		if k != c {
			cells = append(cells, k)
		}
	}
	if on {
		cells = append(cells, c)
	}
	b.SetFoodCourtCells(cells)
	p.detailDirty = true
	return true
}

// finishStroke commits the session's edits: a changed footprint regrades
// the pad; any change rebuilds the tiles and the trail graph (doors are
// trail endpoints). A shell erased to nothing is deleted and finishStroke
// returns true.
func (p *lodgePaint) finishStroke(r *render.Renderer, w *world.World) (deleted bool) {
	p.lastCell = [2]int{-1, -1}
	if !p.shellDirty && !p.detailDirty {
		return false
	}
	shell := p.shellDirty
	p.shellDirty, p.detailDirty = false, false
	b := p.lodge(w)
	if b == nil {
		return false
	}
	if len(b.Cells) == 0 {
		removePaintedBuilding(r, w, b.ID)
		p.lodgeID = 0
		return true
	}
	if shell {
		gradePaintedPad(w.Terrain, b, 0)
		applyRoadCellState(w)
		r.FlushTerrainVerts(w.Terrain)
	}
	r.RebuildStaticBatch(w)
	w.RebuildTrailGraph()
	return false
}

// lodgeHoverOK reports whether a click at c would do something in the
// current mode — drives the hover tint in the overlay.
func (p *lodgePaint) lodgeHoverOK(w *world.World, c [2]int) bool {
	b := p.lodge(w)
	switch p.mode {
	case lodgeEditShell:
		if p.erase {
			return b != nil && b.HasCell(c)
		}
		return p.shellAddable(w, c) && w.Terrain.IsAccessible(c[0], c[1])
	case lodgeEditDoors:
		return b != nil && b.IsPerimeterCell(c)
	case lodgeEditFood:
		return b != nil && b.HasCell(c) && b.HasFoodCourt(c) == p.erase
	}
	return false
}

// appendLodgeOverlay tints the lodge being edited — shell, food court and
// doors — plus the hovered cell, green where a click would apply.
func appendLodgeOverlay(p *lodgePaint, w *world.World, hover [2]int, hoverValid bool, set func(cx, cz int, r, g, b, a uint8)) {
	if b := p.lodge(w); b != nil {
		for _, c := range b.Cells {
			set(c[0], c[1], 150, 120, 90, 150)
		}
		for _, c := range b.FoodCourtCells {
			set(c[0], c[1], 235, 150, 50, 190)
		}
		for _, c := range b.DoorCells {
			set(c[0], c[1], 80, 210, 120, 220)
		}
	}
	if !hoverValid {
		return
	}
	if p.lodgeHoverOK(w, hover) {
		set(hover[0], hover[1], 120, 230, 140, 170)
	} else {
		set(hover[0], hover[1], 230, 80, 70, 150)
	}
}

// editedLodge is the lodge being painted, or the one whose popup is open —
// drawn as a cutaway with its floor plan overlaid.
func (s *Scenario) editedLodge() *world.Building {
	if s.activeTool == toolLodge {
		return s.lodgePaint.lodge(s.world)
	}
	if s.popup == nil || !s.popup.Visible || s.selectedBuildingID == 0 {
		return nil
	}
	for _, b := range s.world.Buildings {
		if b.ID == s.selectedBuildingID && b.IsShell() {
			return b
		}
	}
	return nil
}

// activateLodgeTool starts a lodge edit session. id 0 paints a new lodge;
// otherwise the session edits that lodge in the given mode. Re-clicking
// the toolbar button while painting a new lodge ends the session.
func (s *Scenario) activateLodgeTool(id uint64, mode lodgeEditMode, erase bool) {
	if s.activeTool == toolLodge && id == 0 {
		s.cancelTool()
		return
	}
	if s.activeTool != toolNone {
		s.cancelTool()
	}
	s.roadEdit.clear()
	s.structureEdit.clear()
	s.lodgePaint.start(id, mode, erase)
	s.activeTool = toolLodge
	s.syncToolButtons()
	if s.popup != nil {
		s.popup.Visible = false
	}
	switch {
	case mode == lodgeEditDoors:
		s.setToast("Click the lodge's outer cells to add or remove doors. Esc to finish.")
	case mode == lodgeEditFood && erase:
		s.setToast("Drag to remove food court. Esc to finish.")
	case mode == lodgeEditFood:
		s.setToast(fmt.Sprintf("Drag inside the lodge to paint a food court ($%s + $%s per cell). Right-drag to erase. Esc to finish.",
			formatDollars(world.FoodCourtBaseCost), formatDollars(world.FoodCourtCostPerCell)))
	case erase:
		s.setToast("Drag to remove lodge area. Esc to finish.")
	case id == 0:
		s.setToast(fmt.Sprintf("Drag to paint the lodge ($%s + $%s per cell). Right-drag to erase. Esc to finish.",
			formatDollars(world.LodgeShellBaseCost), formatDollars(world.LodgeCostPerCell)))
	default:
		s.setToast(fmt.Sprintf("Drag to add lodge area ($%s per cell). Right-drag to erase. Esc to finish.",
			formatDollars(world.LodgeCostPerCell)))
	}
}

// finishLodgeSession commits the stroke in progress and warns when the
// lodge still has no way in.
func (s *Scenario) finishLodgeSession() {
	p := &s.lodgePaint
	if p.finishStroke(s.app.Renderer, s.world) {
		return
	}
	if b := p.lodge(s.world); b != nil && !b.Usable() {
		s.setToast("This lodge has no door yet — open it and use Mark doors so guests can get in.")
	}
}

// applyLodgePaint runs one click or drag step of the lodge tool at
// (gx, gz). Painting charges per cell, with the base cost on the first.
func (s *Scenario) applyLodgePaint(gx, gz int, erase bool) {
	w := s.world
	p := &s.lodgePaint
	c := [2]int{gx, gz}
	p.lastCell = c
	b := p.lodge(w)
	switch p.mode {
	case lodgeEditShell:
		if erase {
			p.removeShellCell(w, c)
			return
		}
		if !w.Terrain.IsAccessible(gx, gz) {
			s.setToast("Can't build on land you don't own")
			return
		}
		if !p.shellAddable(w, c) {
			return
		}
		cost := world.LodgeCostPerCell
		if b == nil {
			cost = world.LodgeShellCost(1)
		}
		if !s.chargeLodge(cost, "lodge") {
			return
		}
		if created := p.addShellCell(w, c); created != nil {
			s.sim.LogBuildingPlaced(created)
		}
	case lodgeEditDoors:
		if b == nil {
			return
		}
		if !w.ToggleDoor(b, c) {
			s.setToast("Doors go on the lodge's outer cells")
			return
		}
		p.detailDirty = true
		p.finishStroke(s.app.Renderer, w)
		p.lastCell = c
	case lodgeEditFood:
		if b == nil || !b.HasCell(c) {
			return
		}
		if erase {
			p.setFoodCourt(w, c, false)
			return
		}
		if b.HasFoodCourt(c) {
			return
		}
		cost := world.FoodCourtCostPerCell
		if len(b.FoodCourtCells) == 0 {
			cost += world.FoodCourtBaseCost
		}
		if !s.chargeLodge(cost, "food court") {
			return
		}
		p.setFoodCourt(w, c, true)
	}
}

// chargeLodge deducts cost for more of what, toasting when short.
func (s *Scenario) chargeLodge(cost int, what string) bool {
	w := s.world
	if !w.CanAfford(cost) {
		s.setToast(fmt.Sprintf("Need $%s for more %s — short by $%s", formatDollars(cost), what, formatDollars(cost-w.Available())))
		return false
	}
	w.Cash -= cost
	return true
}

// buildLodgePopup builds (or rebuilds) the lodge popup. confirmDelete swaps
// the Delete button for Confirm / Cancel.
func (s *Scenario) buildLodgePopup(b *world.Building, confirmDelete bool, screenW, screenH int) {
	w := ui.NewWindow("Lodge", 0, 0)
	w.AddLabel("Area", func() string { return fmt.Sprintf("%d cells", len(b.Cells)) })
	w.AddLabel("Doors", func() string {
		if len(b.DoorCells) == 0 {
			return "none — guests can't get in"
		}
		return fmt.Sprintf("%d", len(b.DoorCells))
	})
	w.AddLabel("Food court", func() string {
		if len(b.FoodCourtCells) == 0 {
			return "none"
		}
		return fmt.Sprintf("%d / %d seats", b.Diners, b.Seats())
	})
	if len(b.FoodCourtCells) > 0 {
		w.AddIntStepper("Meal price ($)", &b.MealPrice, 1, 0, 100)
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
	w.AddActionButton("Add area", func() { s.activateLodgeTool(b.ID, lodgeEditShell, false) })
	w.AddActionButton("Remove area", func() { s.activateLodgeTool(b.ID, lodgeEditShell, true) })
	w.AddActionButton("Mark doors", func() { s.activateLodgeTool(b.ID, lodgeEditDoors, false) })
	w.AddActionButton("Paint food court", func() { s.activateLodgeTool(b.ID, lodgeEditFood, false) })
	if len(b.FoodCourtCells) > 0 {
		w.AddActionButton("Remove food court", func() { s.activateLodgeTool(b.ID, lodgeEditFood, true) })
	}
	w.AddActionButton("New style", func() {
		b.StyleSeed = rng.Global().Uint32()
		s.app.Renderer.RebuildStaticBatch(s.world)
	})
	if confirmDelete {
		w.AddLabel("Confirm", func() string { return "Delete this lodge?" })
		w.AddActionButton("Confirm", func() { s.deletePaintedBuilding(b.ID) })
		w.AddActionButton("Cancel", func() { s.buildLodgePopup(b, false, screenW, screenH) })
	} else {
		w.AddActionButton("Delete lodge", func() { s.buildLodgePopup(b, true, screenW, screenH) })
	}
	w.Visible = true
	w.Center(screenW, screenH)
	s.popup = w
}
