package world

import (
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
)

// Lodges are painted shells: the player paints the footprint cells
// (Building.Cells), marks perimeter cells as doors (DoorCells) and paints
// interior modules onto shell cells (FoodCourtCells). The exterior is
// resolved from that shape by ResolveLodgeShell. Every shell cell blocks
// walking; guests path to the nearest door, so door placement matters.

const (
	LodgeShellBaseCost   = 100_000 // VISION §7: shell $100k + per cell
	LodgeCostPerCell     = 4_000
	FoodCourtBaseCost    = 50_000 // fit-out charged with the first food-court cell
	FoodCourtCostPerCell = 5_000

	LodgeDailyCostPerCell     = 20 // heating and upkeep on top of LodgeDailyCost
	FoodCourtDailyCostPerCell = 60 // kitchen and counter staff

	// FoodCourtSeatsPerCell is seating per 5 × 5 m cell (~2 m² a seat
	// once counters and aisles are taken out).
	FoodCourtSeatsPerCell = 10
	DefaultMealPrice      = 18 // dollars per meal

	// Legacy lodges (one fixed mesh) convert to a shell this many cells
	// on a side, matching the old 19 × 13 m mesh.
	legacyLodgeCellsX = 4
	legacyLodgeCellsZ = 3
)

// IsPainted reports whether b's footprint is a set of painted cells
// (parking lots and lodge shells) rather than a mesh rectangle.
func (b *Building) IsPainted() bool {
	return len(b.Cells) > 0 && (b.Type == BuildingParking || b.Type == BuildingLodge)
}

// IsShell reports whether b is a lodge with a painted shell.
func (b *Building) IsShell() bool {
	return b.Type == BuildingLodge && len(b.Cells) > 0
}

// HasDoor reports whether cell c is one of the lodge's doors.
func (b *Building) HasDoor(c [2]int) bool {
	for _, d := range b.DoorCells {
		if d == c {
			return true
		}
	}
	return false
}

// HasFoodCourt reports whether cell c is part of the lodge's food court.
func (b *Building) HasFoodCourt(c [2]int) bool {
	for _, f := range b.FoodCourtCells {
		if f == c {
			return true
		}
	}
	return false
}

// Seats is the food court's seating capacity; 0 without one.
func (b *Building) Seats() int {
	return len(b.FoodCourtCells) * FoodCourtSeatsPerCell
}

// ServesFood reports whether guests can eat here: a lodge with a food
// court and a door to reach it by.
func (b *Building) ServesFood() bool {
	return b.Type == BuildingLodge && len(b.FoodCourtCells) > 0 && b.Usable()
}

// Usable reports whether guests can reach the building. Lodge shells need
// at least one door; everything else is always usable.
func (b *Building) Usable() bool {
	return !b.IsShell() || len(b.DoorCells) > 0
}

// IsPerimeterCell reports whether shell cell c has an outside neighbour
// (4-connected) — the cells a door can go on.
func (b *Building) IsPerimeterCell(c [2]int) bool {
	if !b.HasCell(c) {
		return false
	}
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
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
	out := make([]mgl32.Vec2, len(b.DoorCells))
	for i, d := range b.DoorCells {
		out[i] = cellCentre(d)
	}
	return out
}

// NearestEntrance returns the entrance closest to p and its door cell.
func (b *Building) NearestEntrance(p mgl32.Vec2) (mgl32.Vec2, [2]int) {
	if !b.IsShell() || len(b.DoorCells) == 0 {
		return b.Pos, b.DoorCell()
	}
	best, bestCell := cellCentre(b.DoorCells[0]), b.DoorCells[0]
	for _, d := range b.DoorCells[1:] {
		if c := cellCentre(d); c.Sub(p).Len() < best.Sub(p).Len() {
			best, bestCell = c, d
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

// PaintedBuildingAt returns the parking lot or lodge shell covering cell
// (cx, cz), or nil.
func (w *World) PaintedBuildingAt(cx, cz int) *Building {
	for _, b := range w.Buildings {
		if b.IsPainted() && b.HasCell([2]int{cx, cz}) {
			return b
		}
	}
	return nil
}

// PlaceLodgeShell creates a lodge covering cells, with no doors or
// modules yet. Cost gating and terrain grading live in the caller.
func (w *World) PlaceLodgeShell(cells [][2]int, seed uint32) *Building {
	b := &Building{
		ID:        w.NextID(),
		Type:      BuildingLodge,
		StyleSeed: seed,
		MealPrice: DefaultMealPrice,
	}
	w.Buildings = append(w.Buildings, b)
	w.SetShellCells(b, cells)
	return b
}

// SetShellCells replaces a lodge's footprint and re-derives everything
// that depends on it: doors and food-court cells that fell off the shell
// (or off its perimeter, for doors) are dropped, walking is blocked over
// the new shell and restored on cells it gave up, and Pos moves to the
// primary door (or the shell centre while there is none).
func (w *World) SetShellCells(b *Building, cells [][2]int) {
	old := b.Cells
	b.Cells = sortedUniqueCells(cells)
	b.rebuildCellSet()

	var doors [][2]int
	for _, d := range b.DoorCells {
		if b.IsPerimeterCell(d) {
			doors = append(doors, d)
		}
	}
	b.DoorCells = doors
	var food [][2]int
	for _, f := range b.FoodCourtCells {
		if b.HasCell(f) {
			food = append(food, f)
		}
	}
	b.FoodCourtCells = sortedUniqueCells(food)

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
	w.refreshLodgeAnchor(b)
}

// ClearLodgeFloors strips snow off every lodge shell's cells — they're
// under a roof. Called after each snowfall.
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
	case len(b.DoorCells) > 0:
		b.Pos = cellCentre(b.DoorCells[0])
	case len(b.Cells) > 0:
		b.Pos = parkingAnchor(b.Cells)
	}
}

// ToggleDoor adds or removes a door on shell cell c. Only perimeter cells
// take doors. Returns false when c can't hold a door.
func (w *World) ToggleDoor(b *Building, c [2]int) bool {
	if !b.IsPerimeterCell(c) {
		return false
	}
	for i, d := range b.DoorCells {
		if d == c {
			b.DoorCells = append(b.DoorCells[:i], b.DoorCells[i+1:]...)
			w.refreshLodgeAnchor(b)
			return true
		}
	}
	b.DoorCells = append(b.DoorCells, c)
	w.refreshLodgeAnchor(b)
	return true
}

// SetFoodCourtCells replaces the food court, keeping only shell cells.
func (b *Building) SetFoodCourtCells(cells [][2]int) {
	var keep [][2]int
	for _, c := range cells {
		if b.HasCell(c) {
			keep = append(keep, c)
		}
	}
	b.FoodCourtCells = sortedUniqueCells(keep)
}

// LodgeShellCost returns what a shell of n cells costs to build.
func LodgeShellCost(n int) int {
	return LodgeShellBaseCost + n*LodgeCostPerCell
}

// LodgeUpkeep is one open day's staffing and upkeep for lodge b.
func LodgeUpkeep(b *Building) int {
	return LodgeDailyCost + len(b.Cells)*LodgeDailyCostPerCell +
		len(b.FoodCourtCells)*FoodCourtDailyCostPerCell
}

// ConvertLegacyLodge turns a fixed-mesh lodge (no cells) into a shell
// covering the old mesh's footprint, with one door on the side facing
// the nearest lift base (or the downhill side when there are no lifts).
// No-op for lodges that already have a shell.
func (w *World) ConvertLegacyLodge(b *Building) {
	if b.Type != BuildingLodge || len(b.Cells) > 0 {
		return
	}
	nx, nz := legacyLodgeCellsX, legacyLodgeCellsZ
	if math.Abs(math.Sin(float64(b.Rotation))) > 0.7 {
		nx, nz = nz, nx
	}
	x0 := int(math.Round(float64(b.Pos[0]/CellSize - float32(nx)/2)))
	z0 := int(math.Round(float64(b.Pos[1]/CellSize - float32(nz)/2)))
	var cells [][2]int
	for cx := x0; cx < x0+nx; cx++ {
		for cz := z0; cz < z0+nz; cz++ {
			if w.Terrain.InBounds(cx, cz) {
				cells = append(cells, [2]int{cx, cz})
			}
		}
	}
	if len(cells) == 0 {
		return
	}
	if b.MealPrice == 0 {
		b.MealPrice = DefaultMealPrice
	}
	if b.StyleSeed == 0 {
		b.StyleSeed = uint32(b.ID)
	}
	b.Rotation = 0
	w.SetShellCells(b, cells)
	w.ResetDefaultDoor(b)
}

// ResetDefaultDoor replaces a lodge's doors with the single default door
// (see defaultDoorCell). Save loading calls it for converted legacy
// lodges once lifts exist, so the door faces the right way.
func (w *World) ResetDefaultDoor(b *Building) {
	if !b.IsShell() {
		return
	}
	var sx, sz float32
	for _, c := range b.Cells {
		p := cellCentre(c)
		sx += p[0]
		sz += p[1]
	}
	n := float32(len(b.Cells))
	b.DoorCells = nil
	w.ToggleDoor(b, w.defaultDoorCell(b, mgl32.Vec2{sx / n, sz / n}))
}

// defaultDoorCell picks the perimeter cell facing the nearest lift base,
// or the lowest perimeter cell when there are no lifts.
func (w *World) defaultDoorCell(b *Building, centre mgl32.Vec2) [2]int {
	var target *mgl32.Vec2
	bestD := float32(math.MaxFloat32)
	for _, l := range w.Lifts {
		if d := l.Base.Sub(centre).Len(); d < bestD {
			base := l.Base
			target, bestD = &base, d
		}
	}
	best := b.Cells[0]
	bestScore := float32(math.MaxFloat32)
	for _, c := range b.Cells {
		if !b.IsPerimeterCell(c) {
			continue
		}
		var score float32
		if target != nil {
			score = cellCentre(c).Sub(*target).Len()
		} else {
			score = w.Terrain.GroundElevationAt(c[0], c[1])
		}
		if score < bestScore {
			best, bestScore = c, score
		}
	}
	return best
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
