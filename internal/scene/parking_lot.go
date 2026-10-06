package scene

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// Parking lots: grading, the shared drag tool used by the game and the
// editor, and teardown. The lot data model lives in
// world/parking.go; the asphalt and stripes in render/parking_mesh.go.

const (
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
func plowParkingCell(t *world.Terrain, x, z int) {
	c := &t.Cells[x][z]
	c.Base = 0
	c.Top = world.SnowLayer{}
	c.MogulSize = 0
	t.ClearTreesInCell(x, z)
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
		plowParkingCell(t, c[0], c[1])
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
				t.ClearTreesInCell(x, z)
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
			plowParkingCell(w.Terrain, c[0], c[1])
		}
	}
}

// lotTool is the parking tool's drag state, shared by the game and the
// editor. Dragging on open ground draws a new lot, as a rectangle turned
// to line up with the nearest road (R turns it further); dragging near
// an edge of an existing lot moves that edge, resizing the lot.
type lotTool struct {
	mode     lotDragMode
	start    mgl32.Vec2
	rotation float32
	turned   bool   // the player turned the next lot with R
	only     uint64 // when set (the popup's Resize), only this lot's edges can be grabbed
	lotID    uint64 // resizing: the lot
	side     int    // resizing: 0 +X, 1 −X, 2 +Z, 3 −Z (the lot's local axes)
	orig     world.FootprintRect
	rect     world.FootprintRect
	ok       bool
	why      string
}

type lotDragMode int

const (
	lotIdle lotDragMode = iota
	lotDrawing
	lotResizing
)

// lotEdgeGrab is how close to a lot's edge a press grabs it.
const lotEdgeGrab = float32(4)

// lotRoadAlignReach is how far from a road a new lot lines up with it.
const lotRoadAlignReach = float32(120)

// reset ends any drag; only is the lot the tool is limited to (0 for
// any).
func (t *lotTool) reset(only uint64) {
	*t = lotTool{rotation: t.rotation, turned: t.turned, only: only}
}

// press starts a drag at p: grabbing a lot's edge if one is within
// reach, else drawing a new lot (unless the tool is limited to resizing
// one lot). Reports whether a drag started.
func (t *lotTool) press(w *world.World, p mgl32.Vec2) bool {
	for _, b := range w.Buildings {
		if !b.IsRectLot() || (t.only != 0 && b.ID != t.only) {
			continue
		}
		r := b.LotRect()
		lx, lz := r.Local(p)
		if lotAbs(lx) > r.HalfX+lotEdgeGrab || lotAbs(lz) > r.HalfZ+lotEdgeGrab {
			continue
		}
		dx, dz := lotAbs(lotAbs(lx)-r.HalfX), lotAbs(lotAbs(lz)-r.HalfZ)
		if min(dx, dz) > lotEdgeGrab {
			continue
		}
		t.mode, t.lotID, t.orig, t.rect, t.ok = lotResizing, b.ID, r, r, true
		switch {
		case dx <= dz && lx >= 0:
			t.side = 0
		case dx <= dz:
			t.side = 1
		case lz >= 0:
			t.side = 2
		default:
			t.side = 3
		}
		return true
	}
	if t.only != 0 {
		return false
	}
	t.mode, t.lotID, t.start = lotDrawing, 0, p
	if !t.turned {
		t.rotation = roadAlignedRotation(w, p, t.rotation)
	}
	t.rect = world.FootprintRect{Center: p, Rotation: t.rotation}
	t.ok, t.why = false, ""
	return true
}

// roadAlignedRotation turns a lot drawn at p so its sides run along the
// nearest road within reach; fallback otherwise.
func roadAlignedRotation(w *world.World, p mgl32.Vec2, fallback float32) float32 {
	best := lotRoadAlignReach
	rot := fallback
	for _, e := range w.RoadEdges {
		a, b := w.RoadNodeByID(e.A), w.RoadNodeByID(e.B)
		if a == nil || b == nil {
			continue
		}
		q := world.ClosestPointOnRoadSegment(p, a.Pos, b.Pos)
		if d := q.Sub(p).Len(); d < best {
			dir := b.Pos.Sub(a.Pos)
			if dir.Len() < 1e-3 {
				continue
			}
			best = d
			// Local X is (cos R, −sin R): line it up with the road.
			rot = float32(math.Atan2(float64(-dir[1]), float64(dir[0])))
		}
	}
	return rot
}

// move updates the drag to p and re-checks the rectangle.
func (t *lotTool) move(w *world.World, p mgl32.Vec2) {
	switch t.mode {
	case lotDrawing:
		r := world.FootprintRect{Rotation: t.rotation}
		ax, az := r.Axes()
		d := p.Sub(t.start)
		dx, dz := d.Dot(ax), d.Dot(az)
		r.Center = t.start.Add(ax.Mul(dx / 2)).Add(az.Mul(dz / 2))
		r.HalfX, r.HalfZ = lotAbs(dx)/2, lotAbs(dz)/2
		t.rect = r
	case lotResizing:
		r := t.orig
		ax, az := r.Axes()
		n, half := ax, &r.HalfX
		switch t.side {
		case 1:
			n = ax.Mul(-1)
		case 2:
			n, half = az, &r.HalfZ
		case 3:
			n, half = az.Mul(-1), &r.HalfZ
		}
		s := p.Sub(r.Center).Dot(n) // where the grabbed edge goes
		length := max(s+*half, 1)   // from the fixed opposite edge
		r.Center = r.Center.Add(n.Mul((length - 2**half) / 2))
		*half = length / 2
		t.rect = r
	default:
		return
	}
	t.ok, t.why = w.LotRectFree(t.rect, t.lotID)
}

// rotate turns the next lot drawn (and the one being drawn).
func (t *lotTool) rotate(w *world.World, delta float32) {
	t.rotation = stepRotation(t.rotation, delta)
	t.turned = true
	if t.mode == lotDrawing {
		t.rect.Rotation = t.rotation
		t.move(w, t.rect.Center.Add(t.rect.Center.Sub(t.start)))
	}
}

// addedArea is the square metres a commit would add (0 when shrinking).
func (t *lotTool) addedArea() float32 {
	area := 4 * t.rect.HalfX * t.rect.HalfZ
	if t.mode == lotResizing {
		area -= 4 * t.orig.HalfX * t.orig.HalfZ
	}
	return max(area, 0)
}

// summary describes the rectangle being dragged, e.g. "120 × 64 m,
// 412 stalls".
func (t *lotTool) summary(w *world.World) string {
	var self *world.Building
	if t.lotID != 0 {
		self = w.BuildingByID(t.lotID)
	}
	return fmt.Sprintf("%.0f × %.0f m, %d stalls", 2*max(t.rect.HalfX, t.rect.HalfZ), 2*min(t.rect.HalfX, t.rect.HalfZ),
		w.LotStallCount(t.rect, self))
}

// commitLot places a new lot over rect (lotID 0) or resizes lot lotID to
// it, rebuilds its driveway to the nearest road, grades the terrain and
// rebuilds render state.
func commitLot(r *render.Renderer, w *world.World, lotID uint64, rect world.FootprintRect) *world.Building {
	var b *world.Building
	if lotID != 0 {
		b = w.BuildingByID(lotID)
		w.SetLotRect(b, rect)
	} else {
		b = w.PlaceRectLot(rect)
	}
	w.ConnectLotDriveway(b)
	applyParkingLotEffects(w, b)
	gradeLotDriveway(w, b)
	applyRoadCellState(w)
	r.FlushTerrainVerts(w.Terrain)
	r.RebuildStaticBatch(w)
	r.RebuildRoads(w)
	w.RebuildTrailGraph()
	return b
}

// gradeLotDriveway ramps the ground under lot b's driveway in a straight
// grade from the graded lot at its entrance to the road it meets, so the
// driveway neither climbs a cliff at the lot's edge nor sinks under the
// embankment. The road's own cells at the far end are left alone.
func gradeLotDriveway(w *world.World, b *world.Building) {
	if b.DriveEdge == 0 || len(b.DrivewayNodeIDs) == 0 {
		return
	}
	var far *world.RoadNode
	for _, e := range w.RoadEdges {
		if e.ID == b.DriveEdge {
			id := e.A
			if id == b.DrivewayNodeIDs[0] {
				id = e.B
			}
			far = w.RoadNodeByID(id)
		}
	}
	gate := w.RoadNodeByID(b.DrivewayNodeIDs[0])
	if far == nil || gate == nil {
		return
	}
	t := w.Terrain
	elevAt := func(p mgl32.Vec2) (float32, bool) {
		cx, cz := int(p[0]/world.CellSize), int(p[1]/world.CellSize)
		if !t.InBounds(cx, cz) {
			return 0, false
		}
		return t.Cells[cx][cz].GroundElevation, true
	}
	inward := b.Pos.Sub(gate.Pos)
	if l := inward.Len(); l > 1e-3 {
		inward = inward.Mul(world.CellSize / l)
	}
	e0, ok0 := elevAt(gate.Pos.Add(inward)) // the lot's graded plane just inside
	e1, ok1 := elevAt(far.Pos)
	if !ok0 || !ok1 {
		return
	}
	seg := far.Pos.Sub(gate.Pos)
	length := seg.Len()
	if length < 1 {
		return
	}
	dir := seg.Mul(1 / length)
	half := world.RoadHalfWidth + world.CellSize/2
	stop := 1 - (world.RoadHalfWidth+world.CellSize)/length // short of the road it meets
	const cs = world.CellSize
	minX, maxX := min(gate.Pos[0], far.Pos[0])-half, max(gate.Pos[0], far.Pos[0])+half
	minZ, maxZ := min(gate.Pos[1], far.Pos[1])-half, max(gate.Pos[1], far.Pos[1])+half
	for x := int(minX / cs); x <= int(maxX/cs); x++ {
		for z := int(minZ / cs); z <= int(maxZ/cs); z++ {
			if !t.InBounds(x, z) || b.HasCell([2]int{x, z}) {
				continue
			}
			c := mgl32.Vec2{(float32(x) + 0.5) * cs, (float32(z) + 0.5) * cs}
			d := c.Sub(gate.Pos)
			along := d.Dot(dir) / length
			if along < 0 || along > stop {
				continue
			}
			if perp := d.Sub(dir.Mul(along * length)).Len(); perp > half {
				continue
			}
			t.Cells[x][z].GroundElevation = e0 + (e1-e0)*along
		}
	}
	t.RecomputeSlopes()
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

// appendOverlay tints the cells under the lot being dragged: slate when
// it fits, red when it doesn't.
func (t *lotTool) appendOverlay(w *world.World, set func(cx, cz int, r, g, b, a uint8)) {
	if t.mode == lotIdle || t.rect.HalfX <= 0 || t.rect.HalfZ <= 0 {
		return
	}
	for _, c := range world.LotCells(w.Terrain, t.rect) {
		if t.ok {
			set(c[0], c[1], 70, 80, 110, 190)
		} else {
			set(c[0], c[1], 170, 50, 40, 190)
		}
	}
}

func lotAbs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
