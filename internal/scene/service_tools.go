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

// Service buildings go up in two steps. The building tool drags out a
// rectangle of empty floor on the building's own 5 m grid (turned to line
// up with the nearest road or lot; R turns it), shown as a ghost with its
// price, and builds it on a click inside or Enter; a drag that starts
// beside a building adds floor to it instead. The service tool then puts
// a service in a building's tiles, a click or a drag at a time. Doors are
// automatic (world.RefreshDoors), one per room. The data model lives in
// world/lodge.go; the tile kit in world/lodge_shell.go.

// serviceMenu is the Services menu, in the game and the editor.
var serviceMenu = []struct {
	svc  world.Service
	icon render.IconName
}{
	{world.ServiceLounge, render.IconHouse},
	{world.ServiceFood, render.IconUsers},
	{world.ServiceBar, render.IconCocktail},
	{world.ServiceTickets, render.IconCoin},
	{world.ServicePatrol, render.IconHeart},
	{world.ServiceGarage, render.IconGarage},
	{world.ServiceRentals, render.IconStack},
}

// maxShellSpan is the most cells a building rectangle spans each way.
const maxShellSpan = 12

// serviceTool is the build session for toolService: building mode drags
// out shells (kind); service mode puts svc in their tiles.
type serviceTool struct {
	building  bool // building mode; otherwise service mode
	svc       world.Service
	kind      world.ShellKind // what a new building is built as
	seed      uint32          // style for a new building, so the ghost matches
	rot       float32
	turned    bool       // the player turned new buildings with R
	rightDown mgl32.Vec2 // where the right button went down

	pick    tilePick // the tile under the cursor
	painted tilePick // service mode: the last tile painted this stroke

	rect     shellRect // building mode: the floor being laid out
	dragging bool      // the rectangle is following the mouse
	pending  bool      // it's laid out and waiting to be built
}

// tilePick is a tile of a service building.
type tilePick struct {
	bldg uint64
	cell [2]int
	ok   bool
}

// shellRect is a rectangle of cells, corners a and b inclusive, in one
// grid: a new building's (bldg 0) or an existing building's to extend.
type shellRect struct {
	bldg   uint64
	origin mgl32.Vec2
	rot    float32
	a, b   [2]int
	valid  bool
}

// cellAt is the cell of r's grid under world point p.
func (r shellRect) cellAt(p mgl32.Vec2) [2]int {
	return (&world.Building{Origin: r.origin, Rotation: r.rot}).TileAt(p)
}

// cells lists the rectangle's cells that aren't already in its building.
func (r shellRect) cells(w *world.World) [][2]int {
	if !r.valid {
		return nil
	}
	self := w.BuildingByID(r.bldg)
	var out [][2]int
	for x := min(r.a[0], r.b[0]); x <= max(r.a[0], r.b[0]); x++ {
		for z := min(r.a[1], r.b[1]); z <= max(r.a[1], r.b[1]); z++ {
			if c := [2]int{x, z}; self == nil || !self.HasCell(c) {
				out = append(out, c)
			}
		}
	}
	return out
}

// contains reports whether world point p falls in the rectangle.
func (r shellRect) contains(p mgl32.Vec2) bool {
	c := r.cellAt(p)
	return r.valid && c[0] >= min(r.a[0], r.b[0]) && c[0] <= max(r.a[0], r.b[0]) &&
		c[1] >= min(r.a[1], r.b[1]) && c[1] <= max(r.a[1], r.b[1])
}

// extendTo moves corner b to the cell under p, at most maxShellSpan
// cells from a each way.
func (r *shellRect) extendTo(p mgl32.Vec2) {
	c := r.cellAt(p)
	for i := range 2 {
		c[i] = min(max(c[i], r.a[i]-maxShellSpan+1), r.a[i]+maxShellSpan-1)
	}
	r.b = c
}

// anchorRect is a one-cell rectangle at world point g: a cell beside a
// building extends it; anywhere else starts a new building turned by rot
// with cell (0, 0) centred on g.
func anchorRect(w *world.World, g mgl32.Vec2, rot float32) shellRect {
	if b, c, ok := shellBeside(w, g); ok {
		return shellRect{bldg: b.ID, origin: b.Origin, rot: b.Rotation, a: c, b: c, valid: true}
	}
	ax, az := world.FootprintRect{Rotation: rot}.Axes()
	half := float32(world.CellSize) / 2
	return shellRect{origin: g.Sub(ax.Mul(half)).Sub(az.Mul(half)), rot: rot, valid: true}
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

// hitTile resolves the cursor ray to the nearest service-building tile
// in front of the terrain hit.
func hitTile(w *world.World, o, d, ground mgl32.Vec3, groundValid bool) tilePick {
	best := float32(math.Inf(1))
	if groundValid {
		best = ground.Sub(o).Len()
	}
	var pick tilePick
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
			if t, _, _, ok := rayBox(lo3, ld, lo, hi); ok && t < best {
				best, pick = t, tilePick{bldg: b.ID, cell: c, ok: true}
			}
		}
	}
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

// cellBuildable checks one new cell c of a grid at origin turned by rot
// for building self (nil for a new building); free skips the
// land-ownership rule (the editor).
func cellBuildable(w *world.World, self *world.Building, origin mgl32.Vec2, rot float32, c [2]int, free bool) (bool, string) {
	t := w.Terrain
	ground := world.TileGround(t, origin, rot, c)
	if len(ground) == 0 {
		return false, "Off the map"
	}
	var id uint64
	if self != nil {
		id = self.ID
	}
	for _, g := range ground {
		if !free && !t.IsAccessible(g[0], g[1]) {
			return false, "Can't build on land you don't own"
		}
		mine := self != nil && self.OnGround(g)
		if !w.PaintedCellFree(g, id) || !t.Cells[g[0]][g[1]].Passable && !mine {
			return false, "Something's already built there"
		}
	}
	return true, ""
}

// rectLegal checks every new cell of the rectangle.
func rectLegal(w *world.World, r shellRect, free bool) (bool, string) {
	cells := r.cells(w)
	if len(cells) == 0 {
		return false, ""
	}
	self := w.BuildingByID(r.bldg)
	for _, c := range cells {
		if ok, why := cellBuildable(w, self, r.origin, r.rot, c, free); !ok {
			return false, why
		}
	}
	return true, ""
}

// rectKind is what the rectangle builds as: its building's kind when
// extending, the tool's otherwise.
func rectKind(w *world.World, st *serviceTool) (world.ShellKind, int) {
	if b := w.BuildingByID(st.rect.bldg); b != nil {
		return b.Kind, b.Floors()
	}
	return st.kind, 1
}

// rectCost is what building the rectangle costs.
func rectCost(w *world.World, st *serviceTool) int {
	k, storeys := rectKind(w, st)
	return world.ShellCost(k, storeys, len(st.rect.cells(w)), st.rect.bldg == 0)
}

// fitLegal checks putting the tool's service in tile p.
func fitLegal(w *world.World, p tilePick, svc world.Service) (bool, string) {
	b := w.BuildingByID(p.bldg)
	if !p.ok || b == nil || b.ServiceAt(p.cell) == svc {
		return false, ""
	}
	if b.ServiceAt(p.cell) == world.ServiceGarage && !w.GarageCanLose(b, 1) {
		return false, "The garage is full: sell a vehicle first"
	}
	return true, ""
}

// fitCost is what putting svc in tile p costs.
func fitCost(w *world.World, p tilePick, svc world.Service) int {
	b := w.BuildingByID(p.bldg)
	if b == nil {
		return 0
	}
	return world.FitOutCost(b.Kind, b.Floors(), svc)
}

// turn turns new buildings by delta (R), including a rectangle being laid
// out for one.
func (st *serviceTool) turn(delta float32) {
	st.rot = stepRotation(st.rot, delta)
	st.turned = true
	if st.rect.bldg == 0 && (st.dragging || st.pending) {
		st.rect.rot = st.rot
	}
}

// laidOut reports whether a rectangle is being dragged or waiting to be
// built: Esc drops it rather than ending the tool.
func (st *serviceTool) laidOut() bool { return st.dragging || st.pending }

// dropRect forgets the rectangle, keeping the tool.
func (st *serviceTool) dropRect() {
	st.dragging, st.pending = false, false
	st.rect = shellRect{}
}

// buildingToolText is the building tool's hint for kind k.
func buildingToolText(k world.ShellKind) string {
	heat := "heated"
	if !k.Heated() {
		heat = "unheated"
	}
	return fmt.Sprintf("%s (%s): $%s a tile, plus $%s to start one. Drag out its floor (R turns it), then click inside or press Enter to build. Drag from a wall to extend; right-click a tile to remove it. Esc to finish.",
		k.Label(), heat, formatDollars(k.StructureTileCost()), formatDollars(k.BaseCost()))
}

// serviceToolText is the service tool's hint for svc.
func serviceToolText(svc world.Service) string {
	return fmt.Sprintf("%s: $%s a tile to fit out in a lodge (less in a tent or shed). Click or drag over a building's tiles; right-click empties a tile. Esc to finish.",
		svc.Label(), formatDollars(world.ShellLodge.FitOutTileCost(svc)))
}

// setNewShellKind starts the building tool for kind k; picking the
// active kind again ends it.
func (s *Scenario) setNewShellKind(k world.ShellKind) {
	if s.activeTool == toolService && s.serviceTool.building && s.serviceTool.kind == k {
		s.cancelTool()
		return
	}
	s.startServiceTool(serviceTool{building: true, kind: k, seed: rng.Global().Uint32()})
	s.setToast(buildingToolText(k))
}

// activateServiceTool starts putting svc in buildings' tiles; picking the
// active service again ends it.
func (s *Scenario) activateServiceTool(svc world.Service) {
	if s.activeTool == toolService && !s.serviceTool.building && s.serviceTool.svc == svc {
		s.cancelTool()
		return
	}
	s.startServiceTool(serviceTool{svc: svc})
	s.setToast(serviceToolText(svc))
}

func (s *Scenario) startServiceTool(st serviceTool) {
	if s.activeTool != toolNone {
		s.cancelTool()
	}
	s.roadEdit.clear()
	s.structureEdit.clear()
	s.serviceTool = st
	s.activeTool = toolService
	s.syncToolButtons()
	if s.popup != nil {
		s.popup.Visible = false
	}
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

// toolInput is one frame's mouse and keys for the build tool.
type toolInput struct {
	mouse        mgl32.Vec2
	covered      bool // the mouse is over UI
	ground       mgl32.Vec3
	groundValid  bool
	leftClick    bool
	leftHeld     bool
	rightClick   bool
	rightRelease bool
	enter        bool
}

// input runs one frame of the build tool.
func (env serviceEnv) input(st *serviceTool, in toolInput) {
	o, d := env.r.Camera.ScreenToWorldRay(in.mouse)
	st.pick = tilePick{}
	if !in.covered {
		st.pick = hitTile(env.w, o, d, in.ground, in.groundValid)
	}
	g := mgl32.Vec2{in.ground[0], in.ground[2]}
	onMap := in.groundValid && !in.covered

	if in.rightClick {
		st.rightDown = in.mouse
	}
	rightTap := in.rightRelease && in.mouse.Sub(st.rightDown).Len() < 4

	if st.building {
		switch {
		case st.dragging && in.leftHeld:
			if in.groundValid {
				st.rect.extendTo(g)
			}
		case st.dragging:
			st.dragging, st.pending = false, true
			env.toast(env.rectText(st))
		case in.leftClick && onMap && st.pending && st.rect.contains(g):
			env.buildRect(st)
		case in.leftClick && onMap:
			if !st.turned {
				st.rot = serviceRotation(env.w, g, st.rot)
			}
			st.rect, st.dragging, st.pending = anchorRect(env.w, g, st.rot), true, false
		case in.enter && st.pending:
			env.buildRect(st)
		case !st.pending:
			// Before a drag: a one-cell ghost follows the cursor.
			st.rect = shellRect{}
			if onMap {
				if !st.turned {
					st.rot = serviceRotation(env.w, g, st.rot)
				}
				st.rect = anchorRect(env.w, g, st.rot)
			}
		}
		if rightTap && !st.laidOut() {
			env.removeTile(st.pick)
		}
		return
	}

	// Service mode.
	if !in.leftHeld {
		st.painted = tilePick{}
	}
	switch {
	case in.leftClick && onMap && !st.pick.ok:
		env.toast("Put services in a building: build one first from the Buildings menu")
	case in.leftClick || in.leftHeld && st.painted.ok && st.pick.ok && st.pick != st.painted:
		if st.pick.ok {
			env.fit(st)
			st.painted = st.pick
		}
	}
	if rightTap {
		env.emptyTile(st.pick)
	}
}

// rectText is the toast for a laid-out rectangle: its size and price.
func (env serviceEnv) rectText(st *serviceTool) string {
	n := len(st.rect.cells(env.w))
	k, _ := rectKind(env.w, st)
	what := "a new " + strings.ToLower(k.Label())
	if b := env.w.BuildingByID(st.rect.bldg); b != nil {
		what = "more floor for " + b.Label()
	}
	price := ""
	if !env.free {
		price = fmt.Sprintf(" for $%s", formatDollars(rectCost(env.w, st)))
	}
	return fmt.Sprintf("%d tiles of %s%s. Click inside or press Enter to build; Esc to drop it.", n, what, price)
}

// buildRect builds the laid-out rectangle.
func (env serviceEnv) buildRect(st *serviceTool) {
	w := env.w
	if ok, why := rectLegal(w, st.rect, env.free); !ok {
		if why != "" {
			env.toast(why)
		}
		return
	}
	if !env.free {
		cost := rectCost(w, st)
		if !w.CanAfford(cost) {
			env.toast(fmt.Sprintf("Need $%s to build that — short by $%s", formatDollars(cost), formatDollars(cost-w.Available())))
			return
		}
		w.Cash -= cost
	}
	cells := st.rect.cells(w)
	if b := w.BuildingByID(st.rect.bldg); b != nil {
		w.AddShellCells(b, cells)
		env.finish(b, true)
	} else {
		tiles := make(map[[2]int]world.Service, len(cells))
		for _, c := range cells {
			tiles[c] = world.ServiceNone
		}
		b := w.PlaceServiceBuilding(st.rect.origin, st.rect.rot, tiles, st.seed)
		b.Kind = st.kind
		st.seed = rand.Uint32() // style only; the editor has no sim RNG
		if env.placed != nil {
			env.placed(b)
		}
		env.finish(b, true)
		env.toast(fmt.Sprintf("Built a %s. Put services in it from the Services menu.", strings.ToLower(b.Kind.Label())))
	}
	st.dropRect()
}

// fit puts the tool's service in the tile under the cursor.
func (env serviceEnv) fit(st *serviceTool) {
	w := env.w
	p := st.pick
	ok, why := fitLegal(w, p, st.svc)
	if !ok {
		if why != "" {
			env.toast(why)
		}
		return
	}
	if !env.free {
		cost := fitCost(w, p, st.svc)
		if !w.CanAfford(cost) {
			env.toast(fmt.Sprintf("Need $%s to fit out a %s tile — short by $%s",
				formatDollars(cost), strings.ToLower(st.svc.Label()), formatDollars(cost-w.Available())))
			return
		}
		w.Cash -= cost
	}
	b := w.BuildingByID(p.bldg)
	w.SetTileService(b, p.cell, st.svc)
	env.finish(b, false)
}

// emptyTile takes the service out of tile p, leaving empty floor.
func (env serviceEnv) emptyTile(p tilePick) {
	b := env.w.BuildingByID(p.bldg)
	if b == nil || b.ServiceAt(p.cell) == world.ServiceNone {
		return
	}
	if b.ServiceAt(p.cell) == world.ServiceGarage && !env.w.GarageCanLose(b, 1) {
		env.toast("The garage is full: sell a vehicle first")
		return
	}
	env.w.SetTileService(b, p.cell, world.ServiceNone)
	env.finish(b, false)
}

// removeTile takes tile p out of its building; the last tile takes the
// building with it.
func (env serviceEnv) removeTile(p tilePick) {
	b := env.w.BuildingByID(p.bldg)
	if b == nil {
		return
	}
	if b.ServiceAt(p.cell) == world.ServiceGarage && !env.w.GarageCanLose(b, 1) {
		env.toast("The garage is full: sell a vehicle first")
		return
	}
	if len(b.Cells) == 1 {
		env.delete(b.ID)
		return
	}
	env.w.RemoveTile(b, p.cell)
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

// ghost previews the tool: the rectangle of floor as a shell, or the tile
// under the cursor with the service in it; red when it can't go ahead.
func (env serviceEnv) ghost(st *serviceTool) {
	w := env.w
	green, red := [3]float32{0.6, 1.0, 0.6}, [3]float32{1.0, 0.4, 0.4}
	if st.building {
		cells := st.rect.cells(w)
		if len(cells) == 0 {
			return
		}
		k, storeys := rectKind(w, st)
		seed, floor := st.seed, float32(0)
		if b := w.BuildingByID(st.rect.bldg); b != nil {
			seed, floor = b.StyleSeed, w.ShellFloorY(b)
		} else if g := world.TileGround(w.Terrain, st.rect.origin, st.rect.rot, cells[0]); len(g) > 0 {
			floor = w.Terrain.GroundElevationAt(g[0][0], g[0][1])
		}
		tint := green
		if ok, _ := rectLegal(w, st.rect, env.free); !ok || !env.free && !w.CanAfford(rectCost(w, st)) {
			tint = red
		}
		env.r.SetShellGhost(world.PreviewShell(st.rect.origin, st.rect.rot, k, storeys, cells, seed), floor, 1, tint)
		return
	}
	b := w.BuildingByID(st.pick.bldg)
	if !st.pick.ok || b == nil {
		return
	}
	tint := green
	if ok, _ := fitLegal(w, st.pick, st.svc); !ok || !env.free && !w.CanAfford(fitCost(w, st.pick, st.svc)) {
		tint = red
	}
	env.r.SetShellGhost(world.PreviewTile(b.Origin, b.Rotation, b.Kind, b.Floors(), st.pick.cell, st.svc, b.StyleSeed), w.ShellFloorY(b), 1.04, tint)
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

func (s *Scenario) serviceGhost(r *render.Renderer) {
	s.serviceEnv().ghost(&s.serviceTool)
}

// serviceOverlayColor is a service's floor-plan colour in the overlay.
func serviceOverlayColor(sv world.Service) (r, g, b uint8) {
	c := sv.Info().Overlay
	return c[0], c[1], c[2]
}

// appendServiceOverlay tints b's floor plan by service, doors brighter,
// and the selected room (its tiles in b's grid) brighter still.
func appendServiceOverlay(b *world.Building, selected [][2]int, set func(cx, cz int, r, g, b, a uint8)) {
	if b == nil {
		return
	}
	sel := map[[2]int]bool{}
	for _, c := range selected {
		sel[c] = true
	}
	for _, c := range b.Ground {
		centre := mgl32.Vec2{(float32(c[0]) + 0.5) * world.CellSize, (float32(c[1]) + 0.5) * world.CellSize}
		tile := b.TileAt(centre)
		r, g, bl := serviceOverlayColor(b.ServiceAt(tile))
		a := uint8(170)
		switch {
		case sel[tile]:
			r, g, bl, a = lighten(r), lighten(g), lighten(bl), 235
		case len(sel) > 0:
			a = 90 // dim the rest of the building
		}
		set(c[0], c[1], r, g, bl, a)
	}
	for _, d := range b.Doors {
		n := b.TileCentre([2]int{d.Cell[0] + d.Dir[0], d.Cell[1] + d.Dir[1]})
		set(int(n[0]/world.CellSize), int(n[1]/world.CellSize), 80, 210, 120, 200)
	}
}

// lighten moves a colour channel halfway to white.
func lighten(v uint8) uint8 { return v + (255-v)/2 }

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

// roomSel is the room whose popup is open: the room holding cell of
// building bldg.
type roomSel struct {
	bldg uint64
	cell [2]int
	ok   bool
}

// buildLodgePopup builds (or rebuilds) a service building's popup: what
// it is, its storeys and floor, its rooms (each opens its own popup), and
// the building's own actions. confirmDelete swaps the Delete button for
// Confirm / Cancel.
func (s *Scenario) buildLodgePopup(b *world.Building, confirmDelete bool, screenW, screenH int) {
	s.selectedRoom = roomSel{}
	reopen := func(confirm bool) func() {
		return func() { s.buildLodgePopup(b, confirm, screenW, screenH) }
	}
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
	w.AddLabel("Floor", func() string {
		if n := b.EmptyTiles(); n > 0 {
			return fmt.Sprintf("%d tiles, %d empty", len(b.Cells), n)
		}
		return fmt.Sprintf("%d tiles", len(b.Cells))
	})
	rooms := b.Rooms()
	if len(rooms) == 0 {
		w.AddLabel("Services", func() string { return "none yet: add them from the Services menu" })
	}
	for _, room := range rooms {
		cell := room.Cells[0]
		w.AddActionButton(fmt.Sprintf("%s: %d tiles", room.Service.Label(), len(room.Cells)), func() {
			s.buildRoomPopup(b, cell, false, screenW, screenH)
		})
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
	w.AddActionButton("New style", func() {
		b.StyleSeed = rng.Global().Uint32()
		s.app.Renderer.RebuildStaticBatch(s.world)
	})
	if confirmDelete {
		w.AddLabel("Confirm", func() string { return "Delete this building and everything in it?" })
		w.AddActionButton("Confirm", func() { s.deletePaintedBuilding(b.ID) })
		w.AddActionButton("Cancel", reopen(false))
	} else {
		w.AddActionButton("Delete building", reopen(true))
	}
	w.Visible = true
	w.Center(screenW, screenH)
	s.popup = w
}

// buildRoomPopup builds (or rebuilds) the popup of the room holding cell
// of b: what it does, its own controls, and taking it out. confirmRemove
// swaps the Remove button for Confirm / Cancel.
func (s *Scenario) buildRoomPopup(b *world.Building, cell [2]int, confirmRemove bool, screenW, screenH int) {
	room, ok := b.RoomAt(cell)
	if !ok {
		s.buildLodgePopup(b, false, screenW, screenH)
		return
	}
	s.selectedBuildingID = b.ID
	s.selectedRoom = roomSel{bldg: b.ID, cell: cell, ok: true}
	back := func() { s.buildLodgePopup(b, false, screenW, screenH) }
	reopen := func(confirm bool) func() {
		return func() { s.buildRoomPopup(b, cell, confirm, screenW, screenH) }
	}
	sv := room.Service
	w := ui.NewWindow(sv.Label(), 0, 0)
	w.AddLabel("In", func() string { return b.Label() })
	w.AddLabel("Tiles", func() string {
		if b.Floors() > 1 {
			return fmt.Sprintf("%d on each of %d storeys", len(room.Cells), b.Floors())
		}
		return fmt.Sprintf("%d", len(room.Cells))
	})
	w.AddLabel("Door", func() string {
		if _, ok := b.RoomDoor(room); ok {
			return "yes"
		}
		return "none (no open outside wall): guests use another door"
	})
	switch sv {
	case world.ServiceFood:
		w.AddLabel("Diners", func() string {
			return fmt.Sprintf("%d / %d seats, %d waiting", b.InUse[world.PoolFoodSeats], b.Seats(), b.Waiting[world.PoolFoodSeats])
		})
		w.AddIntStepper("Meal price ($)", &b.MealPrice, 1, 0, 100)
		if !b.Offers(world.ServiceBar) {
			w.AddBoolToggle("Free water", func() bool { return b.FreeWater }, func(v bool) { b.FreeWater = v })
		}
	case world.ServiceBar:
		w.AddIntStepper("Drink price ($)", &b.DrinkPrice, 1, 0, 50)
		w.AddBoolToggle("Free water", func() bool { return b.FreeWater }, func(v bool) { b.FreeWater = v })
	case world.ServiceRentals:
		w.AddIntStepper("Rental price ($)", &b.RentalPrice, 1, 0, 150)
	case world.ServicePatrol:
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
		s.addFallReport(w)
	case world.ServiceGarage:
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
	case world.ServiceTickets:
		s.addResortControls(w, reopen(false))
	}
	w.AddLabel("Daily cost", func() string {
		return fmt.Sprintf("$%s/day", formatDollars(world.RoomUpkeep(b, room)))
	})
	if confirmRemove {
		w.AddLabel("Confirm", func() string { return "Take this out, leaving empty floor? No refund." })
		w.AddActionButton("Confirm", func() { s.removeRoom(b, room); back() })
		w.AddActionButton("Cancel", reopen(false))
	} else {
		w.AddActionButton("Remove "+strings.ToLower(sv.Label()), reopen(true))
	}
	w.AddActionButton("Back to building", back)
	w.Visible = true
	w.Center(screenW, screenH)
	s.popup = w
}

// addFallReport adds the day's falls to a patrol popup: how many, and
// the three runs with the most.
func (s *Scenario) addFallReport(w *ui.Window) {
	w.AddSection("Falls today")
	w.AddLabel("All", func() string {
		r := s.world.History.FallReport()
		return fmt.Sprintf("%d (%d off any run, %d getting off lifts)", r.Total, r.OffRun, r.Unloading)
	})
	for i, label := range []string{"Most", "2nd", "3rd"} {
		w.AddLabel(label, func() string {
			r := s.world.History.FallReport()
			if i >= len(r.Runs) {
				return "-"
			}
			name := "a deleted run"
			if t := s.world.FindTrail(r.Runs[i].TrailID); t != nil {
				name = t.Name
				if name == "" {
					name = "Unnamed"
				}
			}
			return fmt.Sprintf("%s: %d", name, r.Runs[i].Count)
		})
	}
}

// removeRoom takes room's service out of b, leaving empty floor.
func (s *Scenario) removeRoom(b *world.Building, room world.Room) {
	if room.Service == world.ServiceGarage && !s.world.GarageCanLose(b, len(room.Cells)) {
		s.setToast("The garage is full: sell vehicles first")
		return
	}
	tiles := b.TileMap()
	for _, c := range room.Cells {
		tiles[c] = world.ServiceNone
	}
	s.world.SetTiles(b, tiles)
	s.serviceEnv().finish(b, false)
}

// selectedRoomCells is the open room popup's room in the building drawn
// as a cutaway, for the floor-plan highlight.
func (s *Scenario) selectedRoomCells(b *world.Building) [][2]int {
	if b == nil || !s.selectedRoom.ok || s.selectedRoom.bldg != b.ID {
		return nil
	}
	room, ok := b.RoomAt(s.selectedRoom.cell)
	if !ok {
		return nil
	}
	return room.Cells
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
