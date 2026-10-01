package world

import (
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
)

// Service buildings (Type BuildingLodge) are built tile by tile: every
// shell cell carries one Service, and the exterior is resolved from the
// footprint by ResolveLodgeShell. Doors are automatic — each connected
// run of same-service tiles gets one on the outside wall facing where
// its guests come from (RefreshDoors). Every shell cell blocks walking;
// guests path to a door.

// Service is what a building tile offers guests.
type Service uint8

const (
	ServiceNone    Service = iota
	ServiceLounge          // seating: guests rest here
	ServiceFood            // food court: meals, and seating
	ServiceBar             // drinks
	ServiceTickets         // day tickets and season passes
	ServiceCount
)

// Label is the service's player-facing name.
func (s Service) Label() string {
	switch s {
	case ServiceLounge:
		return "Lounge"
	case ServiceFood:
		return "Food court"
	case ServiceBar:
		return "Bar"
	case ServiceTickets:
		return "Tickets"
	}
	return "None"
}

// Accent tints a tile's walls by service so a building's layout reads
// from the camera. Lounge keeps the style palette unchanged.
func (s Service) Accent() mgl32.Vec3 {
	switch s {
	case ServiceFood:
		return mgl32.Vec3{1.00, 0.80, 0.55}
	case ServiceBar:
		return mgl32.Vec3{0.85, 0.60, 0.68}
	case ServiceTickets:
		return mgl32.Vec3{0.68, 0.80, 1.00}
	}
	return mgl32.Vec3{1, 1, 1}
}

// TileCost is what one tile of s costs to build.
func (s Service) TileCost() int {
	switch s {
	case ServiceFood:
		return 30_000
	case ServiceBar:
		return 30_000
	case ServiceTickets:
		return 25_000
	}
	return 20_000
}

// TileDailyCost is one open day's staffing and upkeep for a tile of s.
func (s Service) TileDailyCost() int {
	switch s {
	case ServiceFood:
		return 90 // kitchen and counter staff
	case ServiceBar:
		return 90
	case ServiceTickets:
		return 150 // a ticket window's clerk
	}
	return 40 // heating and cleaning
}

const (
	ServiceBuildingBaseCost      = 50_000 // foundations and utilities, charged with the first tile
	ServiceBuildingDailyBaseCost = 200    // manager and utilities per building

	// FoodCourtSeatsPerCell is seating per 5 × 5 m cell (~2 m² a seat
	// once counters and aisles are taken out).
	FoodCourtSeatsPerCell = 10
	DefaultMealPrice      = 18 // dollars per meal
	DefaultDrinkPrice     = 8  // dollars per drink

	// Legacy lodges (one fixed mesh) convert to a shell this many cells
	// on a side, matching the old 19 × 13 m mesh.
	legacyLodgeCellsX = 4
	legacyLodgeCellsZ = 3
)

// Door is an entrance on one outside wall of a shell tile. Dir points
// out of the building; Service is the tile's service.
type Door struct {
	Cell    [2]int
	Dir     [2]int
	Service Service
}

var cardinals = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// IsPainted reports whether b's footprint is a set of painted cells
// (parking lots and service buildings) rather than a mesh rectangle.
func (b *Building) IsPainted() bool {
	return len(b.Cells) > 0 && (b.Type == BuildingParking || b.Type == BuildingLodge)
}

// IsShell reports whether b is a service building with a tiled shell.
func (b *Building) IsShell() bool {
	return b.Type == BuildingLodge && len(b.Cells) > 0
}

// ServiceAt returns the service of shell cell c (ServiceNone off the shell).
func (b *Building) ServiceAt(c [2]int) Service {
	if !b.HasCell(c) {
		return ServiceNone
	}
	if s := b.Tiles[c]; s != ServiceNone {
		return s
	}
	return ServiceLounge
}

// TileCount returns how many tiles of s the building has.
func (b *Building) TileCount(s Service) int {
	n := 0
	for _, c := range b.Cells {
		if b.ServiceAt(c) == s {
			n++
		}
	}
	return n
}

// Offers reports whether guests can get s here: a tile of it and a way in.
func (b *Building) Offers(s Service) bool {
	return b.IsShell() && b.Usable() && b.TileCount(s) > 0
}

// OffersRest reports whether guests can sit down here: a lounge or a
// food court.
func (b *Building) OffersRest() bool {
	return b.Offers(ServiceLounge) || b.Offers(ServiceFood)
}

// HasFoodCourt reports whether cell c is a food-court tile.
func (b *Building) HasFoodCourt(c [2]int) bool {
	return b.ServiceAt(c) == ServiceFood
}

// Seats is the food court's seating capacity; 0 without one.
func (b *Building) Seats() int {
	return b.TileCount(ServiceFood) * FoodCourtSeatsPerCell
}

// ServesFood reports whether guests can eat here.
func (b *Building) ServesFood() bool {
	return b.Offers(ServiceFood)
}

// Usable reports whether guests can reach the building. Shells need at
// least one door; everything else is always usable.
func (b *Building) Usable() bool {
	return !b.IsShell() || len(b.Doors) > 0
}

// HasDoorFace reports whether cell c has a door on its wall facing dir.
func (b *Building) HasDoorFace(c, dir [2]int) bool {
	for _, d := range b.Doors {
		if d.Cell == c && d.Dir == dir {
			return true
		}
	}
	return false
}

// IsDoorCell reports whether cell c has a door on any wall.
func (b *Building) IsDoorCell(c [2]int) bool {
	for _, d := range b.Doors {
		if d.Cell == c {
			return true
		}
	}
	return false
}

// IsPerimeterCell reports whether shell cell c has an outside neighbour
// (4-connected).
func (b *Building) IsPerimeterCell(c [2]int) bool {
	if !b.HasCell(c) {
		return false
	}
	for _, d := range cardinals {
		if !b.HasCell([2]int{c[0] + d[0], c[1] + d[1]}) {
			return true
		}
	}
	return false
}

// Entrances returns the world XZ points guests walk to: each door cell's
// centre. Non-shell buildings have their anchor as the only entrance.
func (b *Building) Entrances() []mgl32.Vec2 {
	if !b.IsShell() {
		return []mgl32.Vec2{b.Pos}
	}
	out := make([]mgl32.Vec2, len(b.Doors))
	for i, d := range b.Doors {
		out[i] = cellCentre(d.Cell)
	}
	return out
}

// NearestEntrance returns the entrance closest to p and its door cell.
func (b *Building) NearestEntrance(p mgl32.Vec2) (mgl32.Vec2, [2]int) {
	return b.NearestServiceEntrance(ServiceNone, p)
}

// NearestServiceEntrance returns the door closest to p that opens onto
// service s, falling back to any door when no s tile has one (the
// service is reached through the building). ServiceNone takes any door.
func (b *Building) NearestServiceEntrance(s Service, p mgl32.Vec2) (mgl32.Vec2, [2]int) {
	if !b.IsShell() || len(b.Doors) == 0 {
		return b.Pos, b.DoorCell()
	}
	best, bestCell, found := mgl32.Vec2{}, [2]int{}, false
	for pass := 0; pass < 2 && !found; pass++ {
		for _, d := range b.Doors {
			if pass == 0 && s != ServiceNone && d.Service != s {
				continue
			}
			if c := cellCentre(d.Cell); !found || c.Sub(p).Len() < best.Sub(p).Len() {
				best, bestCell, found = c, d.Cell, true
			}
		}
	}
	return best, bestCell
}

func cellCentre(c [2]int) mgl32.Vec2 {
	return mgl32.Vec2{(float32(c[0]) + 0.5) * CellSize, (float32(c[1]) + 0.5) * CellSize}
}

// PaintedCellFree reports whether cell c can join the painted building
// with ID selfID (0 for one not yet created): in bounds, not part of
// another painted building, and not under another building's footprint.
func (w *World) PaintedCellFree(c [2]int, selfID uint64) bool {
	if !w.Terrain.InBounds(c[0], c[1]) {
		return false
	}
	cell := cellRect(c)
	for _, b := range w.Buildings {
		if b.ID == selfID {
			continue
		}
		if b.IsPainted() {
			if b.HasCell(c) {
				return false
			}
			continue
		}
		if b.Type == BuildingSnowGun {
			continue
		}
		if cell.Overlaps(b.Footprint()) {
			return false
		}
	}
	return true
}

// PaintedBuildingAt returns the parking lot or service building covering
// cell (cx, cz), or nil.
func (w *World) PaintedBuildingAt(cx, cz int) *Building {
	for _, b := range w.Buildings {
		if b.IsPainted() && b.HasCell([2]int{cx, cz}) {
			return b
		}
	}
	return nil
}

// PlaceLodgeShell creates a service building covering cells, all lounge.
// Cost gating and terrain grading live in the caller.
func (w *World) PlaceLodgeShell(cells [][2]int, seed uint32) *Building {
	tiles := make(map[[2]int]Service, len(cells))
	for _, c := range cells {
		tiles[c] = ServiceLounge
	}
	return w.PlaceServiceBuilding(tiles, seed)
}

// PreviewTile returns the kit tiles for a lone tile of s at c, for the
// build tool's ghost.
func PreviewTile(c [2]int, s Service, seed uint32) []ShellTile {
	b := &Building{Type: BuildingLodge, Cells: [][2]int{c}, Tiles: map[[2]int]Service{c: s}, StyleSeed: seed}
	return ResolveLodgeShell(b)
}

// PlaceServiceBuilding creates a service building from a tile map. The
// floor is fixed at the mean ground height under it; later tiles grade
// to that height.
func (w *World) PlaceServiceBuilding(tiles map[[2]int]Service, seed uint32) *Building {
	b := &Building{
		ID:         w.NextID(),
		Type:       BuildingLodge,
		StyleSeed:  seed,
		MealPrice:  DefaultMealPrice,
		DrinkPrice: DefaultDrinkPrice,
	}
	w.Buildings = append(w.Buildings, b)
	w.SetTiles(b, tiles)
	b.FloorY, b.FloorSet = w.meanGround(b.Cells), true
	return b
}

func (w *World) meanGround(cells [][2]int) float32 {
	if len(cells) == 0 {
		return 0
	}
	var s float32
	for _, c := range cells {
		s += w.Terrain.GroundElevationAt(c[0], c[1])
	}
	return s / float32(len(cells))
}

// SetTiles replaces a building's tiles and re-derives everything that
// depends on them: walking is blocked over the new shell and restored on
// cells it gave up, doors are re-placed, and Pos moves to the primary
// door (or the shell centre while there is none).
func (w *World) SetTiles(b *Building, tiles map[[2]int]Service) {
	old := b.Cells
	cells := make([][2]int, 0, len(tiles))
	b.Tiles = make(map[[2]int]Service, len(tiles))
	for c, s := range tiles {
		if s == ServiceNone {
			continue
		}
		cells = append(cells, c)
		b.Tiles[c] = s
	}
	b.Cells = sortedUniqueCells(cells)
	b.rebuildCellSet()

	t := w.Terrain
	for _, c := range old {
		if !b.HasCell(c) && t.InBounds(c[0], c[1]) {
			t.Cells[c[0]][c[1]].Passable = true
		}
	}
	for _, c := range b.Cells {
		if t.InBounds(c[0], c[1]) {
			t.Cells[c[0]][c[1]].Passable = false
			clearShellFloor(&t.Cells[c[0]][c[1]])
		}
	}
	w.RefreshDoors(b)
}

// SetShellCells replaces the footprint, keeping each surviving tile's
// service; new cells are lounge.
func (w *World) SetShellCells(b *Building, cells [][2]int) {
	tiles := make(map[[2]int]Service, len(cells))
	for _, c := range cells {
		tiles[c] = b.ServiceAt(c)
		if tiles[c] == ServiceNone {
			tiles[c] = ServiceLounge
		}
	}
	w.SetTiles(b, tiles)
}

// SetTileService sets (or, with ServiceNone, removes) the tile at c.
func (w *World) SetTileService(b *Building, c [2]int, s Service) {
	tiles := b.TileMap()
	if s == ServiceNone {
		delete(tiles, c)
	} else {
		tiles[c] = s
	}
	w.SetTiles(b, tiles)
}

// TileMap returns a copy of the building's cell → service map.
func (b *Building) TileMap() map[[2]int]Service {
	out := make(map[[2]int]Service, len(b.Cells))
	for _, c := range b.Cells {
		out[c] = b.ServiceAt(c)
	}
	return out
}

// SetFoodCourtCells makes exactly cells the food court: listed shell
// cells become food tiles and former food tiles off the list lounge.
func (w *World) SetFoodCourtCells(b *Building, cells [][2]int) {
	food := map[[2]int]bool{}
	for _, c := range cells {
		food[c] = true
	}
	tiles := b.TileMap()
	for c, s := range tiles {
		switch {
		case food[c]:
			tiles[c] = ServiceFood
		case s == ServiceFood:
			tiles[c] = ServiceLounge
		}
	}
	w.SetTiles(b, tiles)
}

// ClearLodgeFloors strips snow off every shell's cells — they're under
// a roof. Called after each snowfall.
func (w *World) ClearLodgeFloors() {
	t := w.Terrain
	for _, b := range w.Buildings {
		if !b.IsShell() {
			continue
		}
		for _, c := range b.Cells {
			if t.InBounds(c[0], c[1]) {
				clearShellFloor(&t.Cells[c[0]][c[1]])
			}
		}
	}
}

func clearShellFloor(c *Cell) {
	c.Base = 0
	c.Top = SnowLayer{}
	c.TreeDensity = 0
	c.MogulSize = 0
}

func (w *World) refreshLodgeAnchor(b *Building) {
	switch {
	case len(b.Doors) > 0:
		b.Pos = cellCentre(b.Doors[0].Cell)
	case len(b.Cells) > 0:
		b.Pos = parkingAnchor(b.Cells)
	}
}

// RefreshAllDoors re-places every service building's doors. Doors face
// lifts and parking, so this runs whenever those change.
func (w *World) RefreshAllDoors() {
	for _, b := range w.Buildings {
		if b.IsShell() {
			w.RefreshDoors(b)
		}
	}
}

// RefreshDoors gives each connected run of same-service tiles one door:
// on the outside wall, opening onto walkable ground that no other
// structure covers, closest to where that service's guests come from
// (serviceDoorTarget). A run with no such wall gets none and is reached
// through the building's other doors.
func (w *World) RefreshDoors(b *Building) {
	b.Doors = nil
	seen := map[[2]int]bool{}
	for _, start := range b.Cells {
		if seen[start] {
			continue
		}
		svc := b.ServiceAt(start)
		region := b.floodService(start, seen)
		target, hasTarget := w.serviceDoorTarget(b, svc)
		var best Door
		bestScore, found := float32(math.MaxFloat32), false
		for _, c := range region {
			for _, d := range cardinals {
				n := [2]int{c[0] + d[0], c[1] + d[1]}
				if b.HasCell(n) || !w.doorOpensOnto(b, n) {
					continue
				}
				score := w.Terrain.GroundElevationAt(n[0], n[1])
				if hasTarget {
					score = cellCentre(n).Sub(target).Len()
				}
				if score < bestScore {
					best, bestScore, found = Door{Cell: c, Dir: d, Service: svc}, score, true
				}
			}
		}
		if found {
			b.Doors = append(b.Doors, best)
		}
	}
	w.refreshLodgeAnchor(b)
}

// floodService returns the 4-connected run of tiles sharing start's
// service, marking them in seen. Cells are visited in sorted order so
// door choice is deterministic.
func (b *Building) floodService(start [2]int, seen map[[2]int]bool) [][2]int {
	svc := b.ServiceAt(start)
	var region [][2]int
	stack := [][2]int{start}
	seen[start] = true
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		region = append(region, c)
		for _, d := range cardinals {
			n := [2]int{c[0] + d[0], c[1] + d[1]}
			if !seen[n] && b.ServiceAt(n) == svc {
				seen[n] = true
				stack = append(stack, n)
			}
		}
	}
	return sortedUniqueCells(region)
}

// doorOpensOnto reports whether a door of b may open onto cell n.
func (w *World) doorOpensOnto(b *Building, n [2]int) bool {
	if !w.Terrain.InBounds(n[0], n[1]) || !w.Terrain.Cells[n[0]][n[1]].Passable {
		return false
	}
	return w.PaintedCellFree(n, b.ID)
}

// serviceDoorTarget is where a service's guests come from: parking for
// the ticket window (guests buy on arrival), the nearest lift base for
// everything else (guests come off the slopes). Each falls back to the
// other; false when the world has neither.
func (w *World) serviceDoorTarget(b *Building, s Service) (mgl32.Vec2, bool) {
	centre := parkingAnchor(b.Cells)
	nearestLift := func() (mgl32.Vec2, bool) {
		var best mgl32.Vec2
		bestD, found := float32(math.MaxFloat32), false
		for _, l := range w.Lifts {
			if d := l.Base.Sub(centre).Len(); d < bestD {
				best, bestD, found = l.Base, d, true
			}
		}
		return best, found
	}
	nearestLot := func() (mgl32.Vec2, bool) {
		var best mgl32.Vec2
		bestD, found := float32(math.MaxFloat32), false
		for _, o := range w.Buildings {
			if o.Type != BuildingParking {
				continue
			}
			if d := o.Pos.Sub(centre).Len(); d < bestD {
				best, bestD, found = o.Pos, d, true
			}
		}
		return best, found
	}
	first, second := nearestLift, nearestLot
	if s == ServiceTickets {
		first, second = nearestLot, nearestLift
	}
	if p, ok := first(); ok {
		return p, true
	}
	return second()
}

// ServiceBuildingCost returns what adding one s tile to a building with
// n tiles costs; the first tile carries the base cost.
func ServiceBuildingCost(s Service, n int) int {
	if n == 0 {
		return ServiceBuildingBaseCost + s.TileCost()
	}
	return s.TileCost()
}

// LodgeUpkeep is one open day's staffing and upkeep for service building b.
func LodgeUpkeep(b *Building) int {
	cost := ServiceBuildingDailyBaseCost
	for _, c := range b.Cells {
		cost += b.ServiceAt(c).TileDailyCost()
	}
	return cost
}

// ConvertLegacyBuilding turns a fixed-mesh lodge, bar or ticket office
// (no cells) into a service building covering the old mesh's footprint:
// lounge for a lodge, bar or tickets for the others. No-op for anything
// else or for buildings that already have tiles.
func (w *World) ConvertLegacyBuilding(b *Building) {
	if len(b.Cells) > 0 {
		return
	}
	var svc Service
	var cells [][2]int
	switch b.Type {
	case BuildingLodge:
		svc = ServiceLounge
		nx, nz := legacyLodgeCellsX, legacyLodgeCellsZ
		if math.Abs(math.Sin(float64(b.Rotation))) > 0.7 {
			nx, nz = nz, nx
		}
		cells = w.cellBlock(b.Pos, nx, nz)
	case BuildingBar, BuildingTicketOffice:
		svc = ServiceBar
		if b.Type == BuildingTicketOffice {
			svc = ServiceTickets
		}
		cells = w.footprintCells(b.Footprint())
		if len(cells) == 0 {
			cells = w.cellBlock(b.Pos, 1, 1)
		}
	default:
		return
	}
	// Mesh buildings could overlap painted ones; tiles can't.
	free := cells[:0]
	for _, c := range cells {
		if w.PaintedCellFree(c, b.ID) {
			free = append(free, c)
		}
	}
	cells = free
	if len(cells) == 0 {
		cells = w.nearestFreeCell(b)
	}
	if len(cells) == 0 {
		return
	}
	if b.Type != BuildingLodge {
		// The old anchor blocked one cell; SetTiles blocks the new shell.
		if c := b.DoorCell(); w.Terrain.InBounds(c[0], c[1]) {
			w.Terrain.Cells[c[0]][c[1]].Passable = true
		}
	}
	b.Type = BuildingLodge
	b.Rotation = 0
	if b.MealPrice == 0 {
		b.MealPrice = DefaultMealPrice
	}
	if b.DrinkPrice == 0 {
		b.DrinkPrice = DefaultDrinkPrice
	}
	if b.StyleSeed == 0 {
		b.StyleSeed = uint32(b.ID)
	}
	tiles := make(map[[2]int]Service, len(cells))
	for _, c := range cells {
		tiles[c] = svc
	}
	w.SetTiles(b, tiles)
	b.FloorY, b.FloorSet = w.meanGround(b.Cells), true
}

// nearestFreeCell returns the free cell closest to b's anchor within a
// few cells, or nil.
func (w *World) nearestFreeCell(b *Building) [][2]int {
	const reach = 6
	c0 := [2]int{int(b.Pos[0] / 5), int(b.Pos[1] / 5)}
	for r := 1; r <= reach; r++ {
		for dx := -r; dx <= r; dx++ {
			for dz := -r; dz <= r; dz++ {
				if dx != -r && dx != r && dz != -r && dz != r {
					continue
				}
				c := [2]int{c0[0] + dx, c0[1] + dz}
				if w.PaintedCellFree(c, b.ID) && w.Terrain.Cells[c[0]][c[1]].Passable {
					return [][2]int{c}
				}
			}
		}
	}
	return nil
}

// cellBlock returns the in-bounds nx × nz block of cells centred on p.
func (w *World) cellBlock(p mgl32.Vec2, nx, nz int) [][2]int {
	x0 := int(math.Round(float64(p[0]/CellSize - float32(nx)/2)))
	z0 := int(math.Round(float64(p[1]/CellSize - float32(nz)/2)))
	var cells [][2]int
	for cx := x0; cx < x0+nx; cx++ {
		for cz := z0; cz < z0+nz; cz++ {
			if w.Terrain.InBounds(cx, cz) {
				cells = append(cells, [2]int{cx, cz})
			}
		}
	}
	return cells
}

// footprintCells returns the in-bounds cells whose centres lie inside r.
func (w *World) footprintCells(r FootprintRect) [][2]int {
	minX, minZ, maxX, maxZ := r.Bounds()
	var cells [][2]int
	for cx := int(minX / CellSize); cx <= int(maxX/CellSize); cx++ {
		for cz := int(minZ / CellSize); cz <= int(maxZ/CellSize); cz++ {
			if w.Terrain.InBounds(cx, cz) && r.Contains(cellCentre([2]int{cx, cz}), 0) {
				cells = append(cells, [2]int{cx, cz})
			}
		}
	}
	return cells
}

func sortedUniqueCells(cells [][2]int) [][2]int {
	out := append([][2]int(nil), cells...)
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	n := 0
	for i, c := range out {
		if i == 0 || c != out[n-1] {
			out[n] = c
			n++
		}
	}
	return out[:n]
}
