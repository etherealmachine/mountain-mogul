package scene

import (
	"math"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// Painted parking lots: grading, the shared drag-paint session used by the
// game and the editor, and teardown. The lot data model lives in
// world/parking.go; the asphalt and stripes in render/parking_mesh.go.

const (
	parkingBrushRadius = 2 // cells; matches the trail brush

	// parkingShoulderCells is the ring around the lot graded to the same
	// plane. Terrain corners average the four cells around them, so the
	// lot's edge corners only sit on the plane if this ring does too.
	parkingShoulderCells = 1
	// parkingBlendCells is the smoothstep ramp from the shoulder back to
	// natural terrain.
	parkingBlendCells = 2
	// parkingMaxGrade caps the fitted pad slope (rise / run). Real lots
	// are graded to a few percent for drainage; a steeper best fit is
	// scaled back so the lot cuts into the hillside instead of tilting.
	parkingMaxGrade = float32(0.05)
)

// gradePlane is a best-fit ground plane in cell units, centred on the
// lot's mean cell centre so the tilt never shifts the average elevation.
type gradePlane struct {
	mx, mz, e float32 // centroid (cell coords) and elevation there
	gx, gz    float32 // metres of rise per cell along x and z
}

func (p gradePlane) at(c [2]int) float32 {
	return p.e + p.gx*(float32(c[0])+0.5-p.mx) + p.gz*(float32(c[1])+0.5-p.mz)
}

// fitParkingPlane least-squares fits GroundElevation over the lot cells,
// degrading to a 1-D fit for single-row lots and to flat for a lone cell.
func fitParkingPlane(t *world.Terrain, cells [][2]int, maxGrade float32) gradePlane {
	var p gradePlane
	n := float32(len(cells))
	for _, c := range cells {
		p.mx += float32(c[0]) + 0.5
		p.mz += float32(c[1]) + 0.5
		p.e += t.Cells[c[0]][c[1]].GroundElevation
	}
	p.mx /= n
	p.mz /= n
	p.e /= n
	var sxx, sxz, szz, sxe, sze float32
	for _, c := range cells {
		dx := float32(c[0]) + 0.5 - p.mx
		dz := float32(c[1]) + 0.5 - p.mz
		de := t.Cells[c[0]][c[1]].GroundElevation - p.e
		sxx += dx * dx
		sxz += dx * dz
		szz += dz * dz
		sxe += dx * de
		sze += dz * de
	}
	const eps = 1e-4
	switch det := sxx*szz - sxz*sxz; {
	case det > eps:
		p.gx = (sxe*szz - sze*sxz) / det
		p.gz = (sze*sxx - sxe*sxz) / det
	case sxx > eps:
		p.gx = sxe / sxx
	case szz > eps:
		p.gz = sze / szz
	}
	maxPerCell := maxGrade * world.CellSize
	if g := float32(math.Hypot(float64(p.gx), float64(p.gz))); g > maxPerCell {
		p.gx *= maxPerCell / g
		p.gz *= maxPerCell / g
	}
	return p
}

// parkingPadCells returns the lot cells plus the graded shoulder ring.
func parkingPadCells(t *world.Terrain, b *world.Building) map[[2]int]bool {
	pad := make(map[[2]int]bool, len(b.Cells)*2)
	for _, c := range b.Cells {
		for dx := -parkingShoulderCells; dx <= parkingShoulderCells; dx++ {
			for dz := -parkingShoulderCells; dz <= parkingShoulderCells; dz++ {
				if n := [2]int{c[0] + dx, c[1] + dz}; t.InBounds(n[0], n[1]) {
					pad[n] = true
				}
			}
		}
	}
	return pad
}

// plowParkingCell strips a pad cell to bare, tree-free ground.
func plowParkingCell(c *world.Cell) {
	c.Base = 0
	c.Top = world.SnowLayer{}
	c.TreeDensity = 0
	c.MogulSize = 0
}

// applyParkingLotEffects grades a painted lot: the lot and its shoulder
// are cut-and-filled onto a best-fit plane and plowed bare; a ramp
// beyond blends ground, snow and trees back to natural. Re-running after
// a reshape regrades the whole lot to the new fit. Like the building
// apron it is one-way — erased cells keep their graded ground.
func applyParkingLotEffects(w *world.World, b *world.Building) {
	gradePaintedPad(w, b, parkingMaxGrade)
}

// gradePaintedPad cuts and fills a painted footprint and its shoulder onto
// a best-fit plane no steeper than maxGrade (0 for a level pad), plows it
// bare and ramps the ground beyond onto an embankment (leaving other
// painted pads and structure cells alone), blending snow and trees back
// to natural.
func gradePaintedPad(w *world.World, b *world.Building, maxGrade float32) {
	if len(b.Cells) == 0 {
		return
	}
	t := w.Terrain
	plane := fitParkingPlane(t, b.Cells, maxGrade)
	if b.IsShell() && b.FloorSet {
		plane = gradePlane{e: b.FloorY}
	}
	pad := parkingPadCells(t, b)
	var edge [][2]int // pad cells bordering non-pad ground
	x0, z0, x1, z1 := t.Width, t.Height, -1, -1
	for c := range pad {
		cell := &t.Cells[c[0]][c[1]]
		cell.GroundElevation = plane.at(c)
		plowParkingCell(cell)
		x0, z0 = min(x0, c[0]), min(z0, c[1])
		x1, z1 = max(x1, c[0]), max(z1, c[1])
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			if !pad[[2]int{c[0] + d[0], c[1] + d[1]}] {
				edge = append(edge, c)
				break
			}
		}
	}

	claimed := claimedGround(w, b, nil)
	const reach = parkingBlendCells + 1
	const cellSize = float32(5.0)
	span := int(embankmentReach / cellSize)
	for x := x0 - span; len(edge) > 0 && x <= x1+span; x++ {
		for z := z0 - span; z <= z1+span; z++ {
			c := [2]int{x, z}
			if pad[c] || !t.InBounds(x, z) {
				continue
			}
			nearest, bestD2 := edge[0], math.MaxInt
			for _, e := range edge {
				dx, dz := e[0]-x, e[1]-z
				if d2 := dx*dx + dz*dz; d2 < bestD2 {
					nearest, bestD2 = e, d2
				}
			}
			d := float32(math.Sqrt(float64(bestD2)))
			if d*cellSize > embankmentReach {
				continue
			}
			cell := &t.Cells[x][z]
			padElev := plane.at(nearest)
			if cell.Passable && !claimed[c] {
				cell.GroundElevation = clampToEmbankment(cell.GroundElevation, padElev, d*cellSize, true, true)
			}
			if d >= reach {
				continue
			}
			blend := 1 - smoothstep32(0, reach, d)
			cell.Base *= 1 - blend
			cell.Top.Accumulation *= 1 - blend
			cell.MogulSize *= 1 - blend
			if blend > 0.5 {
				cell.TreeDensity = 0
			}
		}
	}
	t.RecomputeSlopes()
	t.RestampTreeWells()
}

// replowParkingLots clears snow and trees back off every painted pad (lots
// and lodge shells) without regrading — for editor passes that regenerate
// snow and forest cover.
func replowParkingLots(w *world.World) {
	for _, b := range w.Buildings {
		if !b.IsPainted() {
			continue
		}
		for c := range parkingPadCells(w.Terrain, b) {
			plowParkingCell(&w.Terrain.Cells[c[0]][c[1]])
		}
	}
}

// parkingPaint is the drag-paint state for the parking tool, shared by
// the game and the editor. lotID is 0 until the first cells are painted,
// so an abandoned session leaves nothing behind.
type parkingPaint struct {
	lotID    uint64
	erase    bool   // left-drag removes cells instead of adding them
	lastCell [2]int // cell painted most recently this stroke; {-1,-1} between strokes
	dirty    bool   // cells changed since the last finishStroke
}

func (p *parkingPaint) start(lotID uint64, erase bool) {
	*p = parkingPaint{lotID: lotID, erase: erase, lastCell: [2]int{-1, -1}}
}

func (p *parkingPaint) lot(w *world.World) *world.Building {
	if p.lotID == 0 {
		return nil
	}
	for _, b := range w.Buildings {
		if b.ID == p.lotID {
			return b
		}
	}
	return nil
}

// addableCells returns the cells under a brush at (gx, gz) that the
// session's lot can take and doesn't already have. allowed adds the
// caller's own rules (land ownership in the game).
func (p *parkingPaint) addableCells(w *world.World, gx, gz, radius int, allowed func(c [2]int) bool) [][2]int {
	lot := p.lot(w)
	var out [][2]int
	for _, c := range world.BrushCells(gx, gz, radius) {
		if lot != nil && lot.HasCell(c) {
			continue
		}
		if !w.ParkingCellFree(c, p.lotID) || (allowed != nil && !allowed(c)) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// paint adds cells to the session's lot, creating the lot on first use.
// Returns the new lot when this call created it, else nil.
func (p *parkingPaint) paint(w *world.World, cells [][2]int) (created *world.Building) {
	if len(cells) == 0 {
		return nil
	}
	p.dirty = true
	if lot := p.lot(w); lot != nil {
		w.AddParkingCells(lot, cells)
		return nil
	}
	lot := w.PlaceParkingLot(cells)
	p.lotID = lot.ID
	return lot
}

// eraseAt removes the lot's cells under a brush at (gx, gz).
func (p *parkingPaint) eraseAt(w *world.World, gx, gz, radius int) {
	lot := p.lot(w)
	if lot == nil {
		return
	}
	before := len(lot.Cells)
	w.RemoveParkingCells(lot, world.BrushCells(gx, gz, radius))
	if len(lot.Cells) != before {
		p.dirty = true
	}
}

// finishStroke commits a paint or erase stroke: re-derives the lot,
// regrades the terrain and rebuilds render state. A lot erased down to
// nothing is deleted; finishStroke then returns true.
func (p *parkingPaint) finishStroke(r *render.Renderer, w *world.World) (deleted bool) {
	p.lastCell = [2]int{-1, -1}
	if !p.dirty {
		return false
	}
	p.dirty = false
	lot := p.lot(w)
	if lot == nil {
		return false
	}
	if len(lot.Cells) == 0 {
		removePaintedBuilding(r, w, lot.ID)
		p.lotID = 0
		return true
	}
	w.RefreshParkingLot(lot, true)
	w.EnsureParkingDriveway(lot)
	applyParkingLotEffects(w, lot)
	applyRoadCellState(w)
	r.FlushTerrainVerts(w.Terrain)
	r.RebuildStaticBatch(w)
	r.RebuildRoads(w)
	w.RebuildTrailGraph()
	return false
}

// removePaintedBuilding removes a lot (with its driveway node and any road
// edges attached to it) or a lodge shell. The graded pad stays, like every
// other building.
func removePaintedBuilding(r *render.Renderer, w *world.World, id uint64) {
	w.RemoveBuilding(id)
	r.RebuildStaticBatch(w)
	r.RebuildRoads(w)
	w.RebuildTrailGraph()
}

// appendParkingOverlay tints the cells of the lot being painted so the
// player sees the footprint before the stroke commits.
func appendParkingOverlay(lot *world.Building, set func(cx, cz int, r, g, b, a uint8)) {
	if lot == nil {
		return
	}
	for _, c := range lot.Cells {
		set(c[0], c[1], 70, 80, 110, 190)
	}
}
