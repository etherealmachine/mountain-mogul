package world

import (
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
)

// Parking lots are procedural: the player paints a set of grid cells
// (Building.Cells) and everything else — the stall grid, capacity, the
// driveway attach point, the anchor Pos — is derived from that set by
// RefreshParkingLot. The renderer draws asphalt and stall stripes from
// the same cells and stalls, so MaxCars never drifts from what the
// screen shows.

const (
	// CarLength and CarWidth mirror models-src/car.scad — keep in sync
	// when the car mesh changes size. body_len = 4.0, body_w = 1.7.
	CarLength = float32(4.00)
	CarWidth  = float32(1.70)

	// Stall pitch = car footprint + clearance. Width gets a door-gap
	// margin between neighbours; length gets bumper clearance on each
	// end. Lands on the real-world 2.5 × 5.0 m metric stall.
	ParkingStallWidth  = CarWidth + 0.80  // 2.50 m
	ParkingStallLength = CarLength + 1.00 // 5.00 m

	// parkingAisleWidth is the drive aisle each stall row faces. Rows
	// are laid out as double-loaded modules: stall row, aisle, stall
	// row, with neighbouring modules back-to-back.
	parkingAisleWidth = float32(6.0)

	// parkingEdgeInset keeps stall paint off the very edge of the lot.
	parkingEdgeInset = float32(0.5)

	// Lots placed by a single point (testbeds, saves from before lots
	// were painted) get a default rectangle matching the old 40 × 30 m pad.
	defaultParkingCellsX = 8
	defaultParkingCellsZ = 6

	// ParkingCostPerCell is what the player pays per painted lot cell.
	ParkingCostPerCell = 2_500
)

// ParkingStall is one painted parking space. Heading is the car's yaw
// about +Y; 0 means the car's length runs along world Z.
type ParkingStall struct {
	Pos     mgl32.Vec2
	Heading float32
}

// IsCellLot reports whether b is a parking lot with a painted footprint.
func (b *Building) IsCellLot() bool {
	return b.Type == BuildingParking && len(b.Cells) > 0
}

// HasCell reports whether cell c belongs to the lot's footprint.
func (b *Building) HasCell(c [2]int) bool {
	if len(b.Cells) == 0 {
		return false
	}
	if b.cellSet == nil {
		b.rebuildCellSet()
	}
	_, ok := b.cellSet[c]
	return ok
}

func (b *Building) rebuildCellSet() {
	b.cellSet = make(map[[2]int]struct{}, len(b.Cells))
	for _, c := range b.Cells {
		b.cellSet[c] = struct{}{}
	}
}

// FootprintContains reports whether world XZ (x, z) lies on the
// building's footprint, grown by margin metres on every side.
func (b *Building) FootprintContains(x, z, margin float32) bool {
	if b.IsCellLot() {
		for _, p := range [5]mgl32.Vec2{
			{x, z}, {x - margin, z - margin}, {x + margin, z - margin},
			{x - margin, z + margin}, {x + margin, z + margin},
		} {
			if b.HasCell(cellOf(p)) {
				return true
			}
		}
		return false
	}
	return b.Footprint().Contains(mgl32.Vec2{x, z}, margin)
}

// cellRect is terrain cell c as a footprint rectangle.
func cellRect(c [2]int) FootprintRect {
	return FootprintRect{
		Center: mgl32.Vec2{(float32(c[0]) + 0.5) * CellSize, (float32(c[1]) + 0.5) * CellSize},
		HalfX:  CellSize / 2,
		HalfZ:  CellSize / 2,
	}
}

// cellsOverlapRect reports whether any lot cell square intersects fp.
func (b *Building) cellsOverlapRect(fp FootprintRect) bool {
	minX, minZ, maxX, maxZ := fp.Bounds()
	x0 := int(math.Floor(float64(minX / CellSize)))
	x1 := int(math.Floor(float64(maxX / CellSize)))
	z0 := int(math.Floor(float64(minZ / CellSize)))
	z1 := int(math.Floor(float64(maxZ / CellSize)))
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if c := [2]int{x, z}; b.HasCell(c) && fp.Overlaps(cellRect(c)) {
				return true
			}
		}
	}
	return false
}

// ParkingLotAt returns the parking lot whose footprint contains cell
// (cx, cz), or nil.
func (w *World) ParkingLotAt(cx, cz int) *Building {
	for _, b := range w.Buildings {
		if b.IsCellLot() && b.HasCell([2]int{cx, cz}) {
			return b
		}
	}
	return nil
}

// ParkingCellFree reports whether cell c can join the lot with ID lotID
// (0 for a lot that doesn't exist yet): in bounds, not part of another
// lot, and not under another building's footprint.
func (w *World) ParkingCellFree(c [2]int, lotID uint64) bool {
	if !w.Terrain.InBounds(c[0], c[1]) {
		return false
	}
	cell := cellRect(c)
	for _, b := range w.Buildings {
		if b.ID == lotID {
			continue
		}
		if b.IsCellLot() {
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

// PlaceParkingLot creates a parking lot covering cells. The caller
// grades the terrain and rebuilds render state; cost gating lives in
// the caller too.
func (w *World) PlaceParkingLot(cells [][2]int) *Building {
	b := &Building{
		ID:    w.NextID(),
		Type:  BuildingParking,
		Cells: append([][2]int(nil), cells...),
	}
	w.Buildings = append(w.Buildings, b)
	w.RefreshParkingLot(b, true)
	return b
}

// AddParkingCells adds cells to a lot, ignoring ones it already has.
// Call RefreshParkingLot once the stroke is done.
func (w *World) AddParkingCells(b *Building, cells [][2]int) {
	if b.cellSet == nil {
		b.rebuildCellSet()
	}
	for _, c := range cells {
		if !b.HasCell(c) {
			b.Cells = append(b.Cells, c)
			b.cellSet[c] = struct{}{}
		}
	}
}

// RemoveParkingCells removes cells from a lot. Call RefreshParkingLot
// once the stroke is done (or RemoveBuilding if the lot is now empty).
func (w *World) RemoveParkingCells(b *Building, cells [][2]int) {
	if len(b.Cells) == 0 {
		return
	}
	rm := make(map[[2]int]bool, len(cells))
	for _, c := range cells {
		rm[c] = true
	}
	out := b.Cells[:0]
	for _, c := range b.Cells {
		if !rm[c] {
			out = append(out, c)
		}
	}
	b.Cells = out
	b.rebuildCellSet()
}

// SetParkingLotCells replaces a lot's footprint. nil cells restore the
// default rectangle around the lot's Pos — used when loading saves from
// before lots were painted. The caller runs RefreshParkingLot once the
// road graph is in place.
func (w *World) SetParkingLotCells(b *Building, cells [][2]int) {
	if len(cells) == 0 {
		cells = defaultParkingCells(w.Terrain, b.Pos[0], b.Pos[1], b.Rotation)
	}
	b.Cells = append([][2]int(nil), cells...)
	b.rebuildCellSet()
}

// defaultParkingCells returns the default rectangular footprint for a
// lot anchored at world XZ (x, z), clipped to the map. Rotation of a
// quarter turn swaps the rectangle's sides.
func defaultParkingCells(t *Terrain, x, z, rotation float32) [][2]int {
	nx, nz := defaultParkingCellsX, defaultParkingCellsZ
	if math.Abs(math.Sin(float64(rotation))) > 0.7 {
		nx, nz = nz, nx
	}
	x0 := int(math.Floor(float64(x/CellSize))) - nx/2
	z0 := int(math.Floor(float64(z/CellSize))) - nz/2
	cells := make([][2]int, 0, nx*nz)
	for cx := x0; cx < x0+nx; cx++ {
		for cz := z0; cz < z0+nz; cz++ {
			if t.InBounds(cx, cz) {
				cells = append(cells, [2]int{cx, cz})
			}
		}
	}
	return cells
}

// RefreshParkingLot re-derives everything that depends on a lot's cells:
// cell order, the anchor Pos (re-centred when recenter is set or when Pos
// has fallen off the lot), the blocked door cell, the driveway node, the
// stall layout and MaxCars. Terrain grading is the scene's job.
func (w *World) RefreshParkingLot(b *Building, recenter bool) {
	if b.Type != BuildingParking || len(b.Cells) == 0 {
		return
	}
	sort.Slice(b.Cells, func(i, j int) bool {
		if b.Cells[i][0] != b.Cells[j][0] {
			return b.Cells[i][0] < b.Cells[j][0]
		}
		return b.Cells[i][1] < b.Cells[j][1]
	})
	b.rebuildCellSet()

	t := w.Terrain
	hadPos := b.Pos != (mgl32.Vec2{}) // a fresh lot has no door to release yet
	oldDoor := b.DoorCell()
	if recenter || !b.HasCell(oldDoor) {
		b.Pos = parkingAnchor(b.Cells)
	}
	if door := b.DoorCell(); hadPos && door != oldDoor && t.InBounds(oldDoor[0], oldDoor[1]) {
		t.Cells[oldDoor[0]][oldDoor[1]].Passable = true
	}
	if door := b.DoorCell(); t.InBounds(door[0], door[1]) {
		t.Cells[door[0]][door[1]].Passable = false
	}

	w.settleParkingDriveway(b)
	entrance := b.Pos
	if len(b.DrivewayNodeIDs) > 0 {
		if n := w.RoadNodeByID(b.DrivewayNodeIDs[0]); n != nil {
			entrance = n.Pos
		}
	}
	b.Stalls = layoutParkingStalls(b.Cells, entrance)
	b.MaxCars = len(b.Stalls)
	if b.CurrentCars > float32(b.MaxCars) {
		b.CurrentCars = float32(b.MaxCars)
	}
}

// parkingAnchor returns the centre of the lot cell nearest the lot's
// centroid, so the anchor always sits on the lot even for L-shapes.
func parkingAnchor(cells [][2]int) mgl32.Vec2 {
	var sx, sz float64
	for _, c := range cells {
		sx += float64(c[0]) + 0.5
		sz += float64(c[1]) + 0.5
	}
	n := float64(len(cells))
	mx, mz := sx/n, sz/n
	best := cells[0]
	bestD := math.MaxFloat64
	for _, c := range cells {
		dx := float64(c[0]) + 0.5 - mx
		dz := float64(c[1]) + 0.5 - mz
		if d := dx*dx + dz*dz; d < bestD {
			best, bestD = c, d
		}
	}
	return mgl32.Vec2{(float32(best[0]) + 0.5) * CellSize, (float32(best[1]) + 0.5) * CellSize}
}

// parkingEntrance picks where the lot's driveway sits: the midpoint of
// the lot's outer edge closest to the existing road network, or the
// lowest outer edge when there are no roads yet (roads usually arrive
// from the valley).
func (w *World) parkingEntrance(b *Building) mgl32.Vec2 {
	own := make(map[uint64]bool, len(b.DrivewayNodeIDs))
	for _, id := range b.DrivewayNodeIDs {
		own[id] = true
	}
	var roads []mgl32.Vec2
	for _, n := range w.RoadNodes {
		if !own[n.ID] {
			roads = append(roads, n.Pos)
		}
	}
	best := b.Pos
	bestScore := float32(math.MaxFloat32)
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for _, c := range b.Cells {
		for _, d := range dirs {
			nb := [2]int{c[0] + d[0], c[1] + d[1]}
			if b.HasCell(nb) || !w.Terrain.InBounds(nb[0], nb[1]) {
				continue
			}
			mid := mgl32.Vec2{
				(float32(c[0]) + 0.5 + float32(d[0])*0.5) * CellSize,
				(float32(c[1]) + 0.5 + float32(d[1])*0.5) * CellSize,
			}
			var score float32
			if len(roads) > 0 {
				score = float32(math.MaxFloat32)
				for _, p := range roads {
					if dd := mid.Sub(p).Len(); dd < score {
						score = dd
					}
				}
			} else {
				score = w.Terrain.GroundElevationAt(nb[0], nb[1])
			}
			if score < bestScore {
				best, bestScore = mid, score
			}
		}
	}
	return best
}

// parkingDrivewayReach is how far a driveway node may sit from the lot
// and still count as attached. Generous enough for the fixed-pad era's
// slots (1 m past a pad that isn't cell-aligned).
const parkingDrivewayReach = CellSize * 1.5

// settleParkingDriveway keeps the lot's first driveway node attached to
// the lot. A node within reach stays put so the player's roads don't
// move under them; a first node left stranded by a reshape moves to a
// fresh entrance. IDs that no longer resolve are dropped. Call only once
// road nodes are loaded.
func (w *World) settleParkingDriveway(b *Building) {
	var live []uint64
	for _, id := range b.DrivewayNodeIDs {
		if n := w.RoadNodeByID(id); n != nil {
			live = append(live, id)
		}
	}
	b.DrivewayNodeIDs = live
	if len(live) == 0 {
		return // EnsureParkingDriveway creates the first one
	}
	if n := w.RoadNodeByID(live[0]); !b.FootprintContains(n.Pos[0], n.Pos[1], parkingDrivewayReach) {
		n.Pos = w.parkingEntrance(b)
	}
}

// lotMask is a dense membership grid over a lot's cell bounding box so
// the stall search can test points without map lookups.
type lotMask struct {
	x0, z0, w, h int
	in           []bool
}

func newLotMask(cells [][2]int) *lotMask {
	m := &lotMask{x0: cells[0][0], z0: cells[0][1]}
	x1, z1 := m.x0, m.z0
	for _, c := range cells {
		if c[0] < m.x0 {
			m.x0 = c[0]
		}
		if c[1] < m.z0 {
			m.z0 = c[1]
		}
		if c[0] > x1 {
			x1 = c[0]
		}
		if c[1] > z1 {
			z1 = c[1]
		}
	}
	m.w, m.h = x1-m.x0+1, z1-m.z0+1
	m.in = make([]bool, m.w*m.h)
	for _, c := range cells {
		m.in[(c[0]-m.x0)*m.h+(c[1]-m.z0)] = true
	}
	return m
}

func (m *lotMask) has(px, pz float32) bool {
	cx := int(math.Floor(float64(px/CellSize))) - m.x0
	cz := int(math.Floor(float64(pz/CellSize))) - m.z0
	if cx < 0 || cx >= m.w || cz < 0 || cz >= m.h {
		return false
	}
	return m.in[cx*m.h+cz]
}

// rectInside samples a 3×3 grid over the rect (u0..u1 along the row
// axis, v0..v1 across it). Sample spacing stays under a cell width for
// every rect we test, so no whole missing cell can hide between samples.
func (m *lotMask) rectInside(alongX bool, u0, u1, v0, v1 float32) bool {
	for i := 0; i < 3; i++ {
		u := u0 + (u1-u0)*float32(i)/2
		for j := 0; j < 3; j++ {
			v := v0 + (v1-v0)*float32(j)/2
			x, z := u, v
			if !alongX {
				x, z = v, u
			}
			if !m.has(x, z) {
				return false
			}
		}
	}
	return true
}

// layoutParkingStalls fits the densest stall grid it can into the lot.
// Rows are double-loaded modules (stall row, aisle, stall row) tiled
// across the lot; every stall must sit fully on the lot and face an
// aisle that is also on the lot, so each car can drive out. Both row
// directions and a range of phase offsets are tried; ties keep rows
// along X. Stalls are ordered nearest-entrance first so a partly full
// lot fills from the driveway.
func layoutParkingStalls(cells [][2]int, entrance mgl32.Vec2) []ParkingStall {
	if len(cells) == 0 {
		return nil
	}
	m := newLotMask(cells)
	minX := float32(m.x0) * CellSize
	minZ := float32(m.z0) * CellSize
	maxX := float32(m.x0+m.w) * CellSize
	maxZ := float32(m.z0+m.h) * CellSize

	const (
		sw     = ParkingStallWidth
		sl     = ParkingStallLength
		inset  = parkingEdgeInset
		period = 2*ParkingStallLength + parkingAisleWidth
	)
	var best []ParkingStall
	for _, alongX := range []bool{true, false} {
		uMin, uMax, vMin, vMax := minX, maxX, minZ, maxZ
		heading := float32(0) // along-X rows: car length runs along Z
		if !alongX {
			uMin, uMax, vMin, vMax = minZ, maxZ, minX, maxX
			heading = math.Pi / 2
		}
		for phaseV := float32(0); phaseV < period; phaseV++ {
			for phaseU := float32(0); phaseU < sw; phaseU += 0.5 {
				var stalls []ParkingStall
				for vs := vMin - period + phaseV; vs < vMax; vs += period {
					// Row A: stalls [vs, vs+sl], aisle beyond on +v.
					// Row B: aisle before it on -v, stalls [vs+sl+aisle, vs+period].
					rows := [2]struct {
						s0, a0, a1 float32
						flip       bool
					}{
						{vs, vs + sl, vs + sl + parkingAisleWidth, false},
						{vs + sl + parkingAisleWidth, vs + sl, vs + sl + parkingAisleWidth, true},
					}
					for _, row := range rows {
						for u := uMin + phaseU; u+sw <= uMax; u += sw {
							if !m.rectInside(alongX, u+inset, u+sw-inset, row.s0+inset, row.s0+sl-inset) {
								continue
							}
							if !m.rectInside(alongX, u, u+sw, row.a0, row.a1) {
								continue
							}
							cu, cv := u+sw/2, row.s0+sl/2
							pos := mgl32.Vec2{cu, cv}
							if !alongX {
								pos = mgl32.Vec2{cv, cu}
							}
							h := heading
							if row.flip {
								h += math.Pi
							}
							stalls = append(stalls, ParkingStall{Pos: pos, Heading: h})
						}
					}
				}
				if len(stalls) > len(best) {
					best = stalls
				}
			}
		}
	}
	sortStallsFrom(best, entrance)
	return best
}

// sortStallsFrom orders stalls nearest-first from p.
func sortStallsFrom(stalls []ParkingStall, p mgl32.Vec2) {
	sort.SliceStable(stalls, func(i, j int) bool {
		return stalls[i].Pos.Sub(p).Len() < stalls[j].Pos.Sub(p).Len()
	})
}
