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

// Service is what a building tile is for. Every service works the same
// way (serviceInfo): its tiles in a building, a door, a cost per tile to
// build and per day to run, and what it does — for guests (rest, meals,
// drinks, tickets) or for the mountain (patrol, grooming).
type Service uint8

const (
	ServiceNone    Service = iota
	ServiceLounge          // seating: guests rest here
	ServiceFood            // food court: meals, and seating
	ServiceBar             // drinks
	ServiceTickets         // day tickets and season passes
	ServicePatrol          // ski patrol: a patroller and snowmobile per tile
	ServiceGarage          // snowcat garage: space for snowcats and snowmobiles (garage.go)
	ServiceCount
)

// ServiceInfo is what a service is: its name and look, what a tile costs
// to build and to run for a day, and whether guests walk in for it.
type ServiceInfo struct {
	Label         string
	Accent        mgl32.Vec3 // wall tint, so a building's layout reads from the camera
	Overlay       [3]uint8   // floor-plan colour in the cutaway
	TileCost      int
	TileDailyCost int  // staffing and upkeep per open day
	ForGuests     bool // guests walk to its door for it
}

const (
	// PatrollersPerTile is how many patrollers (each with a snowmobile)
	// a patrol tile bases.
	PatrollersPerTile = 1
)

var serviceInfo = [ServiceCount]ServiceInfo{
	ServiceNone: {Label: "None", Accent: mgl32.Vec3{1, 1, 1}},
	ServiceLounge: {Label: "Lounge", Accent: mgl32.Vec3{1, 1, 1}, Overlay: [3]uint8{150, 120, 90},
		TileCost: 20_000, TileDailyCost: 40, ForGuests: true}, // heating and cleaning
	ServiceFood: {Label: "Food court", Accent: mgl32.Vec3{1.00, 0.80, 0.55}, Overlay: [3]uint8{235, 150, 50},
		TileCost: 30_000, TileDailyCost: 90, ForGuests: true}, // kitchen and counter staff
	ServiceBar: {Label: "Bar", Accent: mgl32.Vec3{0.85, 0.60, 0.68}, Overlay: [3]uint8{190, 80, 120},
		TileCost: 30_000, TileDailyCost: 90, ForGuests: true},
	ServiceTickets: {Label: "Tickets", Accent: mgl32.Vec3{0.68, 0.80, 1.00}, Overlay: [3]uint8{80, 140, 230},
		TileCost: 25_000, TileDailyCost: 150, ForGuests: true}, // a ticket window's clerk
	ServicePatrol: {Label: "Ski patrol", Accent: mgl32.Vec3{1.00, 0.62, 0.58}, Overlay: [3]uint8{220, 60, 50},
		TileCost: 40_000, TileDailyCost: 250}, // a patroller and their snowmobile
	ServiceGarage: {Label: "Snowcat garage", Accent: mgl32.Vec3{0.82, 0.84, 0.86}, Overlay: [3]uint8{120, 130, 140},
		TileCost: 15_000, TileDailyCost: 30}, // the floor and doors; vehicles are bought into it
}

// Info is the service's entry in the registry.
func (s Service) Info() ServiceInfo {
	if s >= ServiceCount {
		return serviceInfo[ServiceNone]
	}
	return serviceInfo[s]
}

// Label is the service's player-facing name.
func (s Service) Label() string { return s.Info().Label }

// Accent tints a tile's walls by service. Lounge keeps the style palette
// unchanged.
func (s Service) Accent() mgl32.Vec3 { return s.Info().Accent }

// TileCost is what one tile of s costs to build (in a lodge; see
// ShellKind.TileCost).
func (s Service) TileCost() int { return s.Info().TileCost }

// TileDailyCost is one open day's staffing and upkeep for a tile of s.
func (s Service) TileDailyCost() int { return s.Info().TileDailyCost }

const (
	ServiceBuildingBaseCost      = 50_000 // foundations and utilities, charged with the first tile
	ServiceBuildingDailyBaseCost = 200    // manager and utilities per building

	// FoodCourtSeatsPerCell is seating per 5 × 5 m cell (~2 m² a seat
	// once counters and aisles are taken out).
	FoodCourtSeatsPerCell = 10
	DefaultMealPrice      = 18 // dollars per meal
	DefaultDrinkPrice     = 8  // dollars per drink

	// A lodge placed by a single point (testbeds, tests) is this many
	// cells on a side.
	pointLodgeCellsX = 4
	pointLodgeCellsZ = 3
)

// Door is an entrance on one outside wall of a shell tile. Cell and Dir
// are in the building's own grid; Dir points out of the building.
// Service is the tile's service.
type Door struct {
	Cell    [2]int
	Dir     [2]int
	Service Service
}

var cardinals = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// IsPainted reports whether b's footprint is a set of cells under it
// (parking lots and service buildings) rather than a mesh rectangle.
func (b *Building) IsPainted() bool {
	return len(b.Ground) > 0 && (b.Type == BuildingParking || b.Type == BuildingLodge)
}

// GridToWorld maps a point in a service building's own grid (metres from
// Origin along the grid's X and Z) to world XZ.
func (b *Building) GridToWorld(gx, gz float32) mgl32.Vec2 {
	ax, az := FootprintRect{Rotation: b.Rotation}.Axes()
	return b.Origin.Add(ax.Mul(gx)).Add(az.Mul(gz))
}

// WorldToGrid maps world XZ p into the building's grid, in metres.
func (b *Building) WorldToGrid(p mgl32.Vec2) (gx, gz float32) {
	ax, az := FootprintRect{Rotation: b.Rotation}.Axes()
	d := p.Sub(b.Origin)
	return d.Dot(ax), d.Dot(az)
}

// TileCentre is the world XZ centre of cell c of the building's grid.
func (b *Building) TileCentre(c [2]int) mgl32.Vec2 {
	return b.GridToWorld((float32(c[0])+0.5)*CellSize, (float32(c[1])+0.5)*CellSize)
}

// TileAt is the cell of the building's grid under world XZ p.
func (b *Building) TileAt(p mgl32.Vec2) [2]int {
	gx, gz := b.WorldToGrid(p)
	return [2]int{int(math.Floor(float64(gx / CellSize))), int(math.Floor(float64(gz / CellSize)))}
}

// TileGround is the map cells one tile at cell c of a grid at origin
// turned by rot would cover.
func TileGround(t *Terrain, origin mgl32.Vec2, rot float32, c [2]int) [][2]int {
	return shellGround(t, &Building{Origin: origin, Rotation: rot, Cells: [][2]int{c}})
}

// shellGround is the map cells under a service building's tiles: a map
// cell counts when its centre, or a point 1.5 m in from any corner, lies
// on a tile. With no rotation and a grid-aligned origin it's exactly the
// tiles.
func shellGround(t *Terrain, b *Building) [][2]int {
	if len(b.Cells) == 0 {
		return nil
	}
	minX, minZ := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxZ := -minX, -minZ
	for _, c := range b.Cells {
		for _, k := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
			p := b.GridToWorld(float32(c[0]+k[0])*CellSize, float32(c[1]+k[1])*CellSize)
			minX, minZ = min(minX, p[0]), min(minZ, p[1])
			maxX, maxZ = max(maxX, p[0]), max(maxZ, p[1])
		}
	}
	const in = CellSize/2 - 1
	var out [][2]int
	for x := int(math.Floor(float64(minX / CellSize))); x <= int(math.Floor(float64(maxX/CellSize))); x++ {
		for z := int(math.Floor(float64(minZ / CellSize))); z <= int(math.Floor(float64(maxZ/CellSize))); z++ {
			if !t.InBounds(x, z) {
				continue
			}
			c := cellCentre([2]int{x, z})
			for _, o := range [5][2]float32{{0, 0}, {-in, -in}, {in, -in}, {-in, in}, {in, in}} {
				if b.HasCell(b.TileAt(c.Add(mgl32.Vec2{o[0], o[1]}))) {
					out = append(out, [2]int{x, z})
					break
				}
			}
		}
	}
	return out
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

// ServesGuests reports whether guests have a reason to come here: a
// service they walk in for (not just patrol or a garage).
func (b *Building) ServesGuests() bool {
	for s := ServiceLounge; s < ServiceCount; s++ {
		if s.Info().ForGuests && b.Offers(s) {
			return true
		}
	}
	return false
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
	return b.TileCount(ServiceFood) * FoodCourtSeatsPerCell * b.Floors()
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
		out[i] = b.TileCentre(d.Cell)
	}
	return out
}

// NearestEntrance returns the entrance closest to p and its door cell.
func (b *Building) NearestEntrance(p mgl32.Vec2) (mgl32.Vec2, [2]int) {
	return b.NearestServiceEntrance(ServiceNone, p)
}

// NearestServiceEntrance returns the door closest to p that opens onto
// service s, falling back to any door when no s tile has one (the
// service is reached through the building), and the map cell it's in.
// ServiceNone takes any door.
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
			if c := b.TileCentre(d.Cell); !found || c.Sub(p).Len() < best.Sub(p).Len() {
				best, bestCell, found = c, cellOf(c), true
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
			if b.OnGround(c) {
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
		if b.IsPainted() && b.OnGround([2]int{cx, cz}) {
			return b
		}
	}
	return nil
}

// PlaceLodgeShell creates a service building covering cells of a grid at
// origin turned by rot, all lounge. Cost gating and terrain grading live
// in the caller.
func (w *World) PlaceLodgeShell(origin mgl32.Vec2, rot float32, cells [][2]int, seed uint32) *Building {
	tiles := make(map[[2]int]Service, len(cells))
	for _, c := range cells {
		tiles[c] = ServiceLounge
	}
	return w.PlaceServiceBuilding(origin, rot, tiles, seed)
}

// PreviewTile returns the kit tiles for a lone tile of s at cell c of a
// grid at origin turned by rot, in a building of kind k with storeys
// storeys, for the build tool's ghost.
func PreviewTile(origin mgl32.Vec2, rot float32, k ShellKind, storeys int, c [2]int, s Service, seed uint32) []ShellTile {
	b := &Building{Type: BuildingLodge, Origin: origin, Rotation: rot, Kind: k, Storeys: storeys, Cells: [][2]int{c}, Tiles: map[[2]int]Service{c: s}, StyleSeed: seed}
	return ResolveLodgeShell(b)
}

// PlaceServiceBuilding creates a service building from a tile map in a
// grid at origin turned by rot. The floor is fixed at the mean ground
// height under it; later tiles grade to that height.
func (w *World) PlaceServiceBuilding(origin mgl32.Vec2, rot float32, tiles map[[2]int]Service, seed uint32) *Building {
	b := &Building{
		ID:         w.NextID(),
		Type:       BuildingLodge,
		Origin:     origin,
		Rotation:   rot,
		StyleSeed:  seed,
		MealPrice:  DefaultMealPrice,
		DrinkPrice: DefaultDrinkPrice,
	}
	w.Buildings = append(w.Buildings, b)
	w.SetTiles(b, tiles)
	b.FloorY, b.FloorSet = w.meanGround(b.Ground), true
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
// depends on them: the map cells under it, walking blocked over the new
// shell and restored on cells it gave up, doors re-placed, and Pos moved
// to the primary door (or the shell centre while there is none).
func (w *World) SetTiles(b *Building, tiles map[[2]int]Service) {
	old := b.Ground
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
	b.Ground = shellGround(t, b)
	b.rebuildGroundSet()
	for _, c := range old {
		if !b.OnGround(c) && t.InBounds(c[0], c[1]) {
			t.Cells[c[0]][c[1]].Passable = true
		}
	}
	for _, c := range b.Ground {
		if t.InBounds(c[0], c[1]) {
			t.Cells[c[0]][c[1]].Passable = false
			clearShellFloor(t, c[0], c[1])
		}
	}
	w.RefreshDoors(b)
	w.SyncFleet(b)
}

// ServiceHome is where the vehicles of service s based at b wait and
// return to: just outside the service's door (or the building's nearest
// door when s has none of its own), on the snow; b's anchor without a
// door.
func (w *World) ServiceHome(b *Building, s Service) mgl32.Vec3 {
	p := b.Pos
	found := false
	for pass := 0; pass < 2 && !found; pass++ {
		for _, d := range b.Doors {
			if pass == 0 && d.Service != s {
				continue
			}
			// The centre of the cell outside the door; a snowcat, being
			// long, parks half a cell further out.
			out := float32(1)
			if s == ServiceGarage {
				out = 1.5
			}
			gx := (float32(d.Cell[0]) + 0.5 + out*float32(d.Dir[0])) * CellSize
			gz := (float32(d.Cell[1]) + 0.5 + out*float32(d.Dir[1])) * CellSize
			p, found = b.GridToWorld(gx, gz), true
			break
		}
	}
	var y float32
	if c := cellOf(p); w.Terrain.InBounds(c[0], c[1]) {
		y = w.Terrain.SurfaceElevationAt(c[0], c[1])
	}
	return mgl32.Vec3{p[0], y, p[1]}
}

// SyncFleet keeps the patrollers based at b in step with its patrol
// tiles: PatrollersPerTile per patrol tile. New ones start at the door;
// surplus ones go, idle ones first. (Snowcats and snowmobiles are bought
// into a garage's space instead; see garage.go.)
func (w *World) SyncFleet(b *Building) {
	wantP := b.TileCount(ServicePatrol) * PatrollersPerTile
	var mine []*Patroller
	for _, p := range w.Patrollers {
		if p.HutID == b.ID {
			mine = append(mine, p)
		}
	}
	for n := len(mine); n < wantP; n++ {
		w.SpawnPatroller(b)
	}
	for pass := 0; pass < 2; pass++ { // idle ones first, then any
		for i := len(mine) - 1; i >= 0 && len(mine) > wantP; i-- {
			if pass == 1 || mine[i].State == PatrollerAtHut {
				w.removePatroller(mine[i].ID)
				mine = append(mine[:i], mine[i+1:]...)
			}
		}
	}
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
		for _, c := range b.Ground {
			if t.InBounds(c[0], c[1]) {
				clearShellFloor(t, c[0], c[1])
			}
		}
	}
}

func clearShellFloor(t *Terrain, x, z int) {
	c := &t.Cells[x][z]
	c.Base = 0
	c.Top = SnowLayer{}
	c.MogulSize = 0
	t.ClearTreesInCell(x, z)
}

func (w *World) refreshLodgeAnchor(b *Building) {
	switch {
	case len(b.Doors) > 0:
		b.Pos = b.TileCentre(b.Doors[0].Cell)
	case len(b.Cells) > 0:
		b.Pos = b.shellCentre()
	}
}

// shellCentre is the world XZ centre of the tile nearest the shell's
// centroid.
func (b *Building) shellCentre() mgl32.Vec2 {
	a := parkingAnchor(b.Cells) // grid metres
	return b.GridToWorld(a[0], a[1])
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
				out := cellOf(b.TileCentre(n))
				score := w.Terrain.GroundElevationAt(out[0], out[1])
				if hasTarget {
					score = b.TileCentre(n).Sub(target).Len()
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

// doorOpensOnto reports whether a door of b may open onto cell n of its
// grid: the map cell there is walkable, off the building, and free of
// other structures.
func (w *World) doorOpensOnto(b *Building, n [2]int) bool {
	m := cellOf(b.TileCentre(n))
	if !w.Terrain.InBounds(m[0], m[1]) || !w.Terrain.Cells[m[0]][m[1]].Passable || b.OnGround(m) {
		return false
	}
	return w.PaintedCellFree(m, b.ID)
}

// serviceDoorTarget is where a service's guests come from: parking for
// the ticket window (guests buy on arrival), the nearest lift base for
// everything else (guests come off the slopes). Each falls back to the
// other; false when the world has neither.
func (w *World) serviceDoorTarget(b *Building, s Service) (mgl32.Vec2, bool) {
	centre := b.shellCentre()
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

// ServiceBuildingCost returns what adding one s tile to a building of
// kind k with storeys storeys and n tiles costs: the tile on every
// storey, and the kind's base cost with the first tile.
func ServiceBuildingCost(k ShellKind, storeys int, s Service, n int) int {
	cost := k.TileCost(s) * max(storeys, 1)
	if n == 0 {
		cost += k.BaseCost()
	}
	return cost
}

// LodgeUpkeep is one open day's staffing and upkeep for service building b.
func LodgeUpkeep(b *Building) int {
	cost := b.Kind.DailyBaseCost()
	tiles := 0
	for _, c := range b.Cells {
		tiles += b.ServiceAt(c).TileDailyCost()
	}
	return cost + int(float32(tiles*b.Floors())*b.Kind.dailyScale())
}

// placePointService turns a lodge, bar or ticket office placed by a
// single point (tests, testbeds, the editor's ticket office) into a
// small service building there on the map's grid: a lounge block for a
// lodge, one bar or ticket tile for the others. Cells another building
// covers are left out. No-op for anything else.
func (w *World) placePointService(b *Building) {
	var svc Service
	nx, nz := 1, 1
	switch b.Type {
	case BuildingLodge:
		svc, nx, nz = ServiceLounge, pointLodgeCellsX, pointLodgeCellsZ
	case BuildingBar:
		svc = ServiceBar
	case BuildingTicketOffice:
		svc = ServiceTickets
	default:
		return
	}
	x0 := int(math.Round(float64(b.Pos[0]/CellSize - float32(nx)/2)))
	z0 := int(math.Round(float64(b.Pos[1]/CellSize - float32(nz)/2)))
	tiles := map[[2]int]Service{}
	for cx := x0; cx < x0+nx; cx++ {
		for cz := z0; cz < z0+nz; cz++ {
			if c := [2]int{cx, cz}; w.PaintedCellFree(c, b.ID) {
				tiles[c] = svc
			}
		}
	}
	if len(tiles) == 0 {
		return
	}
	b.Type, b.Origin, b.Rotation = BuildingLodge, mgl32.Vec2{}, 0
	b.MealPrice, b.DrinkPrice = DefaultMealPrice, DefaultDrinkPrice
	if b.StyleSeed == 0 {
		b.StyleSeed = uint32(b.ID)
	}
	w.SetTiles(b, tiles)
	b.FloorY, b.FloorSet = w.meanGround(b.Ground), true
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
