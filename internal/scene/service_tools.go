package scene

import (
	"fmt"
	"math"
	"math/rand"
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
// and right-click a tile to remove it. Each building has its own 5 m
// grid: a new one is turned to line up with the nearest road or lot
// (R turns it), and its tiles follow that grid. Doors are automatic
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
	cell   [2]int     // in bldg's grid, or the new building's
	origin mgl32.Vec2 // pickNew: the new building's grid
	rot    float32
	legal  bool
	reason string // why a pick is illegal, for the toast

	hitBldg uint64 // building whose tile the cursor is over (0 for none)
	hitCell [2]int
}

// serviceTool is the build session for toolService.
type serviceTool struct {
	svc       world.Service
	kind      world.ShellKind // what a new building is built as
	seed      uint32          // style for a new building, so the ghost matches
	rot       float32
	turned    bool // the player turned new buildings with R
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

// tileBox is the pick volume of shell tile c in its building's grid: the
// cell up to the ridge of a one-tile roof.
func tileBox(floor, height float32, c [2]int) (lo, hi mgl32.Vec3) {
	const cell = float32(5)
	lo = mgl32.Vec3{float32(c[0]) * cell, floor, float32(c[1]) * cell}
	hi = mgl32.Vec3{lo[0] + cell, floor + height, lo[2] + cell}
	return lo, hi
}

// pickService resolves the cursor ray against the shells first, falling
// back to the terrain hit (ground, groundValid) for a new tile, in a new
// building turned by rot.
func pickService(w *world.World, o, d mgl32.Vec3, svc world.Service, ground mgl32.Vec3, groundValid bool, rot float32, free bool) servicePick {
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
		height := float32(b.Floors())*b.Kind.WallHeight() + b.Kind.RoofRise()
		// The ray in the building's grid; lengths are unchanged.
		ax, az := world.FootprintRect{Rotation: b.Rotation}.Axes()
		gx, gz := b.WorldToGrid(mgl32.Vec2{o[0], o[2]})
		lo3 := mgl32.Vec3{gx, o[1], gz}
		ld := mgl32.Vec3{d[0]*ax[0] + d[2]*ax[1], d[1], d[0]*az[0] + d[2]*az[1]}
		for _, c := range b.Cells {
			lo, hi := tileBox(floor, height, c)
			t, axis, sign, ok := rayBox(lo3, ld, lo, hi)
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
		g := mgl32.Vec2{ground[0], ground[2]}
		if b, c, ok := shellBeside(w, g); ok {
			pick = servicePick{kind: pickAdd, bldg: b.ID, cell: c}
		} else {
			// A new building whose cell (0, 0) is centred on the cursor.
			ax, az := world.FootprintRect{Rotation: rot}.Axes()
			half := float32(world.CellSize) / 2
			pick = servicePick{kind: pickNew, origin: g.Sub(ax.Mul(half)).Sub(az.Mul(half)), rot: rot}
		}
	}
	pick.legal, pick.reason = pickLegal(w, pick, svc, free)
	return pick
}

// shellBeside returns a service building with a tile 4-adjacent to the
// cell of its grid under world point p, and that cell.
func shellBeside(w *world.World, p mgl32.Vec2) (*world.Building, [2]int, bool) {
	for _, b := range w.Buildings {
		if !b.IsShell() {
			continue
		}
		c := b.TileAt(p)
		if b.HasCell(c) {
			continue
		}
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			if b.HasCell([2]int{c[0] + d[0], c[1] + d[1]}) {
				return b, c, true
			}
		}
	}
	return nil, [2]int{}, false
}

// pickGrid is the grid a pick's cell is in: its building's, or the new
// building's.
func pickGrid(w *world.World, p servicePick) (mgl32.Vec2, float32) {
	if b := w.BuildingByID(p.bldg); b != nil {
		return b.Origin, b.Rotation
	}
	return p.origin, p.rot
}

// serviceRotation is the turn for a new building at p: lined up with
// the nearest road or parking lot within reach, or fallback.
func serviceRotation(w *world.World, p mgl32.Vec2, fallback float32) float32 {
	rot := roadAlignedRotation(w, p, fallback)
	best := lotRoadAlignReach
	for _, e := range w.RoadEdges {
		a, b := w.RoadNodeByID(e.A), w.RoadNodeByID(e.B)
		if a != nil && b != nil {
			best = min(best, world.ClosestPointOnRoadSegment(p, a.Pos, b.Pos).Sub(p).Len())
		}
	}
	for _, b := range w.Buildings {
		if !b.IsRectLot() {
			continue
		}
		r := b.LotRect()
		lx, lz := r.Local(p)
		d := max(lotAbs(lx)-r.HalfX, lotAbs(lz)-r.HalfZ, 0)
		if d < best {
			best, rot = d, b.Rotation
		}
	}
	return rot
}

// pickLegal checks a pick; free skips the land-ownership rule (the
// editor).
func pickLegal(w *world.World, p servicePick, svc world.Service, free bool) (bool, string) {
	switch p.kind {
	case pickRepaint:
		b := w.BuildingByID(p.bldg)
		if b == nil || b.ServiceAt(p.cell) == svc {
			return false, ""
		}
		if b.ServiceAt(p.cell) == world.ServiceGarage && !w.GarageCanLose(b, 1) {
			return false, "The garage is full: sell a vehicle first"
		}
		return true, ""
	case pickNew, pickAdd:
		t := w.Terrain
		self := w.BuildingByID(p.bldg)
		if self != nil && self.HasCell(p.cell) {
			return false, ""
		}
		origin, rot := pickGrid(w, p)
		ground := world.TileGround(t, origin, rot, p.cell)
		if len(ground) == 0 {
			return false, ""
		}
		for _, c := range ground {
			if !free && !t.IsAccessible(c[0], c[1]) {
				return false, "Can't build on land you don't own"
			}
			mine := self != nil && self.OnGround(c)
			if !w.PaintedCellFree(c, p.bldg) || !t.Cells[c[0]][c[1]].Passable && !mine {
				return false, "Something's already built there"
			}
		}
		return true, ""
	}
	return false, ""
}

// pickCost is what applying p with service svc charges.
func pickCost(w *world.World, p servicePick, st *serviceTool) int {
	b := w.BuildingByID(p.bldg)
	if p.kind == pickNew || b == nil {
		return world.ServiceBuildingCost(st.kind, 1, st.svc, 0)
	}
	return world.ServiceBuildingCost(b.Kind, b.Floors(), st.svc, len(b.Cells))
}

// setNewShellKind picks what new buildings are built as. With a service
// already picked, the tool switches over at once; otherwise lounge.
func (s *Scenario) setNewShellKind(k world.ShellKind) {
	s.newShellKind = k
	svc := world.ServiceLounge
	if s.activeTool == toolService {
		svc = s.serviceTool.svc
		s.cancelTool()
	}
	s.activateServiceTool(svc)
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
	s.serviceTool = serviceTool{svc: svc, kind: s.newShellKind, seed: rng.Global().Uint32()}
	s.activeTool = toolService
	s.syncToolButtons()
	if s.popup != nil {
		s.popup.Visible = false
	}
	k := s.newShellKind
	s.setToast(fmt.Sprintf("%s: $%s a tile in a %s. Click ground to start a %s ($%s, R turns it), a wall to extend, a roof to switch; right-click removes. Esc to finish.",
		svc.Label(), formatDollars(k.TileCost(svc)), strings.ToLower(k.Label()), strings.ToLower(k.Label()), formatDollars(world.ServiceBuildingCost(k, 1, svc, 0))))
}

// serviceEnv is what the build tool works in: the game charges for
// tiles, checks land, and logs new buildings; the editor builds for free
// anywhere.
type serviceEnv struct {
	w       *world.World
	r       *render.Renderer
	free    bool                  // the editor: no cost, any land
	toast   func(string)          // tell the player something
	placed  func(*world.Building) // a new building went up (game: the event feed)
	changed func()                // after any change (snowcat sections, dirty flags)
	delete  func(id uint64)       // tear a building down
}

// updateServicePick refreshes the tool's pick under the mouse; covered
// means the mouse is over UI.
func (env serviceEnv) updatePick(st *serviceTool, mouse mgl32.Vec2, covered bool, hover mgl32.Vec3, hoverValid bool) {
	if covered {
		st.pick = servicePick{}
		return
	}
	o, d := env.r.Camera.ScreenToWorldRay(mouse)
	if !st.turned && hoverValid {
		st.rot = serviceRotation(env.w, mgl32.Vec2{hover[0], hover[2]}, st.rot)
	}
	st.pick = pickService(env.w, o, d, st.svc, hover, hoverValid, st.rot, env.free)
}

// apply runs the left-click action under the cursor.
func (env serviceEnv) apply(st *serviceTool) {
	w := env.w
	p := st.pick
	if p.kind == pickNone {
		return
	}
	if !p.legal {
		if p.reason != "" {
			env.toast(p.reason)
		}
		return
	}
	if !env.free {
		cost := pickCost(w, p, st)
		if !w.CanAfford(cost) {
			env.toast(fmt.Sprintf("Need $%s for a %s tile — short by $%s",
				formatDollars(cost), strings.ToLower(st.svc.Label()), formatDollars(cost-w.Available())))
			return
		}
		w.Cash -= cost
	}
	switch p.kind {
	case pickNew:
		b := w.PlaceServiceBuilding(p.origin, p.rot, map[[2]int]world.Service{p.cell: st.svc}, st.seed)
		b.Kind = st.kind
		st.seed = rand.Uint32() // style only; the editor has no sim RNG
		if env.placed != nil {
			env.placed(b)
		}
		env.finish(b, true)
	case pickAdd:
		b := w.BuildingByID(p.bldg)
		w.SetTileService(b, p.cell, st.svc)
		env.finish(b, true)
	case pickRepaint:
		b := w.BuildingByID(p.bldg)
		w.SetTileService(b, p.cell, st.svc)
		env.finish(b, false)
	}
}

// remove removes the tile under the cursor; the last tile takes the
// building with it.
func (env serviceEnv) remove(st *serviceTool) {
	p := st.pick
	b := env.w.BuildingByID(p.hitBldg)
	if b == nil {
		return
	}
	if b.ServiceAt(p.hitCell) == world.ServiceGarage && !env.w.GarageCanLose(b, 1) {
		env.toast("The garage is full: sell a vehicle first")
		return
	}
	if len(b.Cells) == 1 {
		env.delete(b.ID)
		return
	}
	env.w.SetTileService(b, p.hitCell, world.ServiceNone)
	env.finish(b, false)
}

// finish regrades a grown footprint to the building's floor and rebuilds
// the tiles and the trail graph, which refreshes the doors.
func (env serviceEnv) finish(b *world.Building, regrade bool) {
	w, r := env.w, env.r
	if regrade {
		gradePaintedPad(w, b, 0)
		applyRoadCellState(w)
		r.FlushTerrainVerts(w.Terrain)
	}
	w.RebuildTrailGraph()
	r.RebuildStaticBatch(w)
	if env.changed != nil {
		env.changed()
	}
}

// ghost previews the pick: a lone tile of the tool's service at the
// target cell, red when the click can't go ahead.
func (env serviceEnv) ghost(st *serviceTool) {
	w := env.w
	p := st.pick
	if p.kind == pickNone {
		return
	}
	seed, kind, floors := st.seed, st.kind, 1
	origin, rot := pickGrid(w, p)
	floor := float32(0)
	if g := world.TileGround(w.Terrain, origin, rot, p.cell); len(g) > 0 {
		floor = w.Terrain.GroundElevationAt(g[0][0], g[0][1])
	}
	if b := w.BuildingByID(p.bldg); b != nil {
		seed, floor, kind, floors = b.StyleSeed, w.ShellFloorY(b), b.Kind, b.Floors()
	}
	tint := [3]float32{0.6, 1.0, 0.6}
	if !p.legal || (!env.free && !w.CanAfford(pickCost(w, p, st))) {
		tint = [3]float32{1.0, 0.4, 0.4}
	}
	scale := float32(1)
	if p.kind == pickRepaint {
		scale = 1.04
	}
	env.r.SetShellGhost(world.PreviewTile(origin, rot, kind, floors, p.cell, st.svc, seed), floor, scale, tint)
}

// serviceEnv is the game's build-tool surroundings.
func (s *Scenario) serviceEnv() serviceEnv {
	return serviceEnv{
		w: s.world, r: s.app.Renderer, toast: s.setToast,
		placed:  func(b *world.Building) { s.sim.LogBuildingPlaced(b) },
		changed: func() { s.sim.InvalidateSections() }, // garage tiles may have changed the fleet
		delete:  s.deletePaintedBuilding,
	}
}

// updateServicePick refreshes the tool's pick under the mouse.
func (s *Scenario) updateServicePick(r *render.Renderer, mouse mgl32.Vec2) {
	covered := s.barsContain(mouse[1]) || s.uiCovers(mouse[0], mouse[1], float32(r.ScreenWidth()))
	s.serviceEnv().updatePick(&s.serviceTool, mouse, covered, s.hoverWorld, s.hoverValid)
}

func (s *Scenario) applyServicePick()  { s.serviceEnv().apply(&s.serviceTool) }
func (s *Scenario) removeServiceTile() { s.serviceEnv().remove(&s.serviceTool) }
func (s *Scenario) serviceGhost(r *render.Renderer) {
	s.serviceEnv().ghost(&s.serviceTool)
}

// serviceOverlayColor is a service's floor-plan colour in the overlay.
func serviceOverlayColor(sv world.Service) (r, g, b uint8) {
	c := sv.Info().Overlay
	return c[0], c[1], c[2]
}

// appendServiceOverlay tints b's floor plan by service, doors brighter.
func appendServiceOverlay(b *world.Building, set func(cx, cz int, r, g, b, a uint8)) {
	if b == nil {
		return
	}
	for _, c := range b.Ground {
		centre := mgl32.Vec2{(float32(c[0]) + 0.5) * world.CellSize, (float32(c[1]) + 0.5) * world.CellSize}
		r, g, bl := serviceOverlayColor(b.ServiceAt(b.TileAt(centre)))
		set(c[0], c[1], r, g, bl, 170)
	}
	for _, d := range b.Doors {
		n := b.TileCentre([2]int{d.Cell[0] + d.Dir[0], d.Cell[1] + d.Dir[1]})
		set(int(n[0]/world.CellSize), int(n[1]/world.CellSize), 80, 210, 120, 200)
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

// halfTiles formats a count of half-tiles as tiles: "3" or "3.5".
func halfTiles(n int) string {
	if n%2 == 0 {
		return fmt.Sprintf("%d", n/2)
	}
	return fmt.Sprintf("%.1f", float32(n)/2)
}

// buyVehicle buys a snowcat (cat) or a snowmobile into b's garage, if it
// fits and the resort can pay.
func (s *Scenario) buyVehicle(b *world.Building, cat bool) {
	w := s.world
	price, half, name := world.SnowmobilePrice, world.SnowmobileGarageHalfTiles, "snowmobile"
	if cat {
		price, half, name = world.CatPurchasePrice, world.CatGarageHalfTiles, "snowcat"
	}
	if !w.GarageFits(b, half) {
		used, total := w.GarageSpace(b)
		s.setToast(fmt.Sprintf("No room: a %s needs %s free tiles, and this garage has %s", name, halfTiles(half), halfTiles(total-used)))
		return
	}
	if !w.CanAfford(price) {
		s.setToast(fmt.Sprintf("Need $%s for a %s — short by $%s", formatDollars(price), name, formatDollars(price-w.Available())))
		return
	}
	w.Cash -= price
	if cat {
		w.SpawnSnowcat(b)
		s.sim.InvalidateSections()
	} else {
		w.AddSnowmobile(b)
	}
	s.setToast(fmt.Sprintf("Bought a %s: $%s", name, formatDollars(price)))
}

// sellVehicle sells one of b's snowcats (cat) or snowmobiles for half
// what it cost: a parked one, standby cats first.
func (s *Scenario) sellVehicle(b *world.Building, cat bool) {
	w := s.world
	if cat {
		cats := w.CatsOwnedBy(b.ID)
		if len(cats) == 0 {
			return
		}
		pick := cats[len(cats)-1]
		for _, c := range cats {
			if c.Status == world.CatStandby {
				pick = c
				break
			}
		}
		w.RemoveSnowcat(pick.ID)
		w.Cash += world.CatPurchasePrice / 2
		s.sim.InvalidateSections()
		s.setToast(fmt.Sprintf("Sold a snowcat: $%s", formatDollars(world.CatPurchasePrice/2)))
		return
	}
	ms := w.SnowmobilesIn(b)
	if len(ms) == 0 {
		return
	}
	w.RemoveSnowmobile(ms[len(ms)-1].ID)
	w.Cash += world.SnowmobilePrice / 2
	s.setToast(fmt.Sprintf("Sold a snowmobile: $%s", formatDollars(world.SnowmobilePrice/2)))
}

// setCatsActive puts one of b's snowcats on standby (parked, cheaper) or
// back to work.
func (s *Scenario) setCatsActive(b *world.Building, active bool) {
	from, to := world.CatActive, world.CatStandby
	if active {
		from, to = to, from
	}
	for _, c := range s.world.CatsOwnedBy(b.ID) {
		if c.Status == from {
			c.Status = to
			s.sim.InvalidateSections()
			return
		}
	}
}

// setStoreys changes a lodge's storey count: adding one pays for every
// tile again; taking one off refunds nothing.
func (s *Scenario) setStoreys(b *world.Building, n int) {
	w := s.world
	n = max(1, min(n, b.Kind.MaxStoreys()))
	if n == b.Floors() {
		return
	}
	if n > b.Floors() {
		cost := b.AddStoreyCost()
		if !w.CanAfford(cost) {
			s.setToast(fmt.Sprintf("Need $%s for another storey — short by $%s", formatDollars(cost), formatDollars(cost-w.Available())))
			return
		}
		w.Cash -= cost
		s.setToast(fmt.Sprintf("Added a storey: $%s", formatDollars(cost)))
	}
	b.Storeys = n
	s.app.Renderer.RebuildStaticBatch(w)
}

// buildLodgePopup builds (or rebuilds) a service building's popup.
// confirmDelete swaps the Delete button for Confirm / Cancel.
func (s *Scenario) buildLodgePopup(b *world.Building, confirmDelete bool, screenW, screenH int) {
	w := ui.NewWindow(b.Label(), 0, 0)
	w.AddLabel("Built as", func() string {
		if b.Kind.Heated() {
			return b.Kind.Label() + ", heated"
		}
		return b.Kind.Label() + ", unheated"
	})
	if b.Kind.MaxStoreys() > 1 {
		w.AddIntStepperFn("Storeys", func() string { return fmt.Sprintf("%d", b.Floors()) },
			func() { s.setStoreys(b, b.Floors()-1) }, func() { s.setStoreys(b, b.Floors()+1) })
	}
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
	if b.TileCount(world.ServicePatrol) > 0 {
		w.AddLabel("Patrollers", func() string {
			n, out, sleds := 0, 0, 0
			for _, p := range s.world.Patrollers {
				if p.HutID == b.ID {
					n++
					if p.State.Responding() {
						out++
					}
					if p.SnowmobileID != 0 {
						sleds++
					}
				}
			}
			return fmt.Sprintf("%d (%d on a call, %d with a snowmobile)", n, out, sleds)
		})
	}
	if b.TileCount(world.ServiceGarage) > 0 {
		w.AddLabel("Space", func() string {
			used, total := s.world.GarageSpace(b)
			return fmt.Sprintf("%s of %s tiles used", halfTiles(used), halfTiles(total))
		})
		w.AddLabel("Vehicles", func() string {
			return fmt.Sprintf("%d snowcats, %d snowmobiles", len(s.world.CatsOwnedBy(b.ID)), len(s.world.SnowmobilesIn(b)))
		})
		w.AddActionButton(fmt.Sprintf("Buy snowcat ($%s)", formatDollars(world.CatPurchasePrice)), func() { s.buyVehicle(b, true) })
		w.AddActionButton(fmt.Sprintf("Buy snowmobile ($%s)", formatDollars(world.SnowmobilePrice)), func() { s.buyVehicle(b, false) })
		w.AddActionButton("Sell snowcat", func() { s.sellVehicle(b, true) })
		w.AddActionButton("Sell snowmobile", func() { s.sellVehicle(b, false) })
		w.AddIntStepperFn("Snowcats active",
			func() string {
				cats := s.world.CatsOwnedBy(b.ID)
				active := 0
				for _, c := range cats {
					if c.Status == world.CatActive {
						active++
					}
				}
				return fmt.Sprintf("%d / %d", active, len(cats))
			},
			func() { s.setCatsActive(b, false) },
			func() { s.setCatsActive(b, true) })
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
