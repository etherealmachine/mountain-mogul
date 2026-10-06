package world

import (
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
)

// Parking lots are rectangles: Pos is the centre, Rotation turns the
// rectangle about Y (FootprintRect's convention) and LotSize is its
// extent along the local X and Z axes. Everything else — the cells under
// it (for grading, plowing and overlap), the entrance (Gate), the stall
// layout and MaxCars — is derived by RefreshParkingLot. The renderer
// draws the asphalt and stall stripes from the same rectangle and
// stalls, so MaxCars never drifts from what the screen shows.
//
// Stall rows run along the long side in double-loaded modules (stall
// row, aisle, stall row), with a cross aisle at each end. The entrance
// is on the side facing the nearest road; on a long side the stalls in
// front of it are left out so it opens onto an aisle.

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

	// ParkingAisleWidth is the drive aisle each stall row faces, and the
	// cross aisle at each end of the lot.
	ParkingAisleWidth = float32(6.0)

	// LotCornerRadius rounds the asphalt's corners.
	LotCornerRadius = float32(4.0)

	// MinLotWidth fits one stall row and its aisle; MinLotLength the end
	// aisles and a couple of stalls.
	MinLotWidth  = ParkingStallLength + ParkingAisleWidth
	MinLotLength = 2*ParkingAisleWidth + 2*ParkingStallWidth
	// MaxLotSide caps either side of a lot.
	MaxLotSide = float32(400)

	// Lots placed by a single point (testbeds) get this size.
	defaultLotX = float32(40)
	defaultLotZ = float32(30)

	// ParkingCostPerM2 is what the player pays per square metre of lot.
	ParkingCostPerM2 = 100

	// lotGateCornerMargin keeps the entrance off the rounded corners.
	lotGateCornerMargin = LotCornerRadius + ParkingAisleWidth/2
	// MaxDrivewayLength is the longest driveway built automatically to
	// the nearest road; past it the entrance waits for the player's road.
	MaxDrivewayLength = float32(150)
)

// ParkingStall is one painted parking space. Heading is the yaw about +Y
// of the direction from the stall out to its aisle; 0 means the car's
// length runs along world Z. Aisle is the point in the middle of the
// aisle in front of the stall.
type ParkingStall struct {
	Pos     mgl32.Vec2
	Heading float32
	Aisle   mgl32.Vec2
}

// IsCellLot reports whether b is a parking lot with a footprint.
func (b *Building) IsCellLot() bool {
	return b.Type == BuildingParking && len(b.Cells) > 0
}

// IsRectLot reports whether b is a rectangular parking lot.
func (b *Building) IsRectLot() bool {
	return b.Type == BuildingParking && b.LotSize[0] > 0 && b.LotSize[1] > 0
}

// LotRect is a parking lot's rectangle.
func (b *Building) LotRect() FootprintRect {
	return FootprintRect{Center: b.Pos, HalfX: b.LotSize[0] / 2, HalfZ: b.LotSize[1] / 2, Rotation: b.Rotation}
}

// LotArea is the lot's area in square metres.
func (b *Building) LotArea() float32 { return b.LotSize[0] * b.LotSize[1] }

// LotAxes are a lot rectangle's long axis U and short axis V as world
// directions, with the half extents along each.
func LotAxes(r FootprintRect) (u, v mgl32.Vec2, halfU, halfV float32) {
	ax, az := r.Axes()
	if r.HalfX >= r.HalfZ {
		return ax, az, r.HalfX, r.HalfZ
	}
	return az, ax, r.HalfZ, r.HalfX
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
	if b.IsRectLot() {
		return b.LotRect().Contains(mgl32.Vec2{x, z}, margin)
	}
	if b.IsPainted() {
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
	return w.PaintedCellFree(c, lotID)
}

// LotCells are the cells a lot rectangle covers: those whose centre lies
// within a metre of it. The graded shoulder ring around them catches the
// rest of the asphalt.
func LotCells(t *Terrain, r FootprintRect) [][2]int {
	minX, minZ, maxX, maxZ := r.Bounds()
	var cells [][2]int
	for x := int(math.Floor(float64(minX / CellSize))); x <= int(math.Floor(float64(maxX/CellSize))); x++ {
		for z := int(math.Floor(float64(minZ / CellSize))); z <= int(math.Floor(float64(maxZ/CellSize))); z++ {
			if !t.InBounds(x, z) {
				continue
			}
			if r.Contains(cellRect([2]int{x, z}).Center, 1) {
				cells = append(cells, [2]int{x, z})
			}
		}
	}
	return cells
}

// LotRectFree reports whether a lot rectangle fits: on the map, within
// the size limits, and clear of every other building (selfID excepted).
// Returns a reason when it doesn't.
func (w *World) LotRectFree(r FootprintRect, selfID uint64) (bool, string) {
	long, short := 2*max(r.HalfX, r.HalfZ), 2*min(r.HalfX, r.HalfZ)
	switch {
	case short < MinLotWidth || long < MinLotLength:
		return false, "Too small for a row of stalls"
	case long > MaxLotSide:
		return false, "Too big"
	}
	minX, minZ, maxX, maxZ := r.Bounds()
	mapW := float32(w.Terrain.Width) * CellSize
	mapH := float32(w.Terrain.Height) * CellSize
	if minX < 0 || minZ < 0 || maxX > mapW || maxZ > mapH {
		return false, "Off the map"
	}
	for _, c := range LotCells(w.Terrain, r) {
		if !w.PaintedCellFree(c, selfID) {
			return false, "Overlaps another building"
		}
	}
	return true, ""
}

// PlaceRectLot creates a parking lot over rectangle r. The caller checks
// LotRectFree, connects the driveway, grades the terrain and rebuilds
// render state; cost gating lives in the caller too.
func (w *World) PlaceRectLot(r FootprintRect) *Building {
	b := &Building{ID: w.NextID(), Type: BuildingParking}
	w.Buildings = append(w.Buildings, b)
	w.SetLotRect(b, r)
	return b
}

// PlaceParkingLot creates a lot over the bounding box of cells.
func (w *World) PlaceParkingLot(cells [][2]int) *Building {
	return w.PlaceRectLot(CellsBoundingRect(cells))
}

// CellsBoundingRect is the axis-aligned rectangle around cells: how lots
// painted cell by cell (older saves) become rectangles.
func CellsBoundingRect(cells [][2]int) FootprintRect {
	x0, z0, x1, z1 := cells[0][0], cells[0][1], cells[0][0], cells[0][1]
	for _, c := range cells {
		x0, z0 = min(x0, c[0]), min(z0, c[1])
		x1, z1 = max(x1, c[0]), max(z1, c[1])
	}
	minX, minZ := float32(x0)*CellSize, float32(z0)*CellSize
	maxX, maxZ := float32(x1+1)*CellSize, float32(z1+1)*CellSize
	return FootprintRect{
		Center: mgl32.Vec2{(minX + maxX) / 2, (minZ + maxZ) / 2},
		HalfX:  (maxX - minX) / 2,
		HalfZ:  (maxZ - minZ) / 2,
	}
}

// SetLotRect moves or resizes a lot to rectangle r and re-derives it.
// Call ConnectLotDriveway afterwards to rebuild its driveway.
func (w *World) SetLotRect(b *Building, r FootprintRect) {
	if door := b.DoorCell(); len(b.Cells) > 0 && w.Terrain.InBounds(door[0], door[1]) {
		w.Terrain.Cells[door[0]][door[1]].Passable = true
	}
	b.Pos, b.Rotation = r.Center, r.Rotation
	b.LotSize = mgl32.Vec2{2 * r.HalfX, 2 * r.HalfZ}
	w.RefreshParkingLot(b, false)
}

// LotStallCount is how many stalls lot rectangle r would hold (b is the
// lot being resized, or nil).
func (w *World) LotStallCount(r FootprintRect, b *Building) int {
	stalls, _ := layoutLot(r, w.PlanLotGate(r, b).Gate)
	return len(stalls)
}

// DefaultLotRect is the default lot rectangle centred on c, for lots
// placed by a single point (testbeds).
func DefaultLotRect(c mgl32.Vec2) FootprintRect {
	return FootprintRect{Center: c, HalfX: defaultLotX / 2, HalfZ: defaultLotZ / 2}
}

// RefreshParkingLot re-derives everything that depends on a lot's
// rectangle: its cells, the blocked door cell under its centre, the
// entrance, the stall layout and MaxCars. A live driveway node on the
// lot's edge keeps the entrance where it is; otherwise the entrance goes
// on the side facing the nearest road. recenter is unused (the centre is
// the rectangle's). Terrain grading is the scene's job.
func (w *World) RefreshParkingLot(b *Building, recenter bool) {
	if b.Type != BuildingParking {
		return
	}
	if !b.IsRectLot() {
		r := DefaultLotRect(b.Pos)
		b.LotSize = mgl32.Vec2{2 * r.HalfX, 2 * r.HalfZ}
	}
	t := w.Terrain
	oldDoor, hadCells := b.DoorCell(), len(b.Cells) > 0
	r := b.LotRect()
	b.Cells = LotCells(t, r)
	sort.Slice(b.Cells, func(i, j int) bool {
		if b.Cells[i][0] != b.Cells[j][0] {
			return b.Cells[i][0] < b.Cells[j][0]
		}
		return b.Cells[i][1] < b.Cells[j][1]
	})
	b.rebuildCellSet()
	if door := b.DoorCell(); hadCells && door != oldDoor && t.InBounds(oldDoor[0], oldDoor[1]) {
		t.Cells[oldDoor[0]][oldDoor[1]].Passable = true
	}
	if door := b.DoorCell(); t.InBounds(door[0], door[1]) {
		t.Cells[door[0]][door[1]].Passable = false
	}

	b.Gate = w.PlanLotGate(r, b).Gate
	if len(b.DrivewayNodeIDs) > 0 {
		if n := w.RoadNodeByID(b.DrivewayNodeIDs[0]); n != nil {
			if g, ok := snapToLotEdge(r, n.Pos, 3); ok {
				b.Gate = g
			}
		}
	}
	b.Stalls, b.lotAisles = layoutLot(r, b.Gate)
	b.MaxCars = len(b.Stalls)
}

// snapToLotEdge projects p onto the nearest side of r, reporting whether
// p was within reach metres of it.
func snapToLotEdge(r FootprintRect, p mgl32.Vec2, reach float32) (mgl32.Vec2, bool) {
	lx, lz := r.Local(p)
	dx, dz := abs32(abs32(lx)-r.HalfX), abs32(abs32(lz)-r.HalfZ)
	ax, az := r.Axes()
	if dx <= dz {
		if dx > reach || abs32(lz) > r.HalfZ+reach {
			return p, false
		}
		lx = sign32(lx) * r.HalfX
		lz = max(-r.HalfZ, min(r.HalfZ, lz))
	} else {
		if dz > reach || abs32(lx) > r.HalfX+reach {
			return p, false
		}
		lz = sign32(lz) * r.HalfZ
		lx = max(-r.HalfX, min(r.HalfX, lx))
	}
	return r.Center.Add(ax.Mul(lx)).Add(az.Mul(lz)), true
}

func sign32(v float32) float32 {
	if v < 0 {
		return -1
	}
	return 1
}

// LotGatePlan is where a lot's entrance goes and the driveway to the
// nearest road: from Gate straight to RoadPt on Edge (or to the existing
// node Snap). Road is false when no road is within MaxDrivewayLength;
// Gate is then the middle of the lowest side.
type LotGatePlan struct {
	Gate   mgl32.Vec2
	Road   bool
	RoadPt mgl32.Vec2
	Edge   *RoadEdge
	Snap   *RoadNode
}

// DrivewayCost is what building the plan's driveway costs.
func (p LotGatePlan) DrivewayCost() int {
	if !p.Road {
		return 0
	}
	return RoadCost(p.Gate, p.RoadPt)
}

// PlanLotGate picks the entrance for lot rectangle r: the point on its
// edge (clear of the corners) nearest a road in front of it. Roads that
// belong to lot b (its driveway, or anything attached to its entrance)
// don't count; b may be nil for a lot not placed yet.
func (w *World) PlanLotGate(r FootprintRect, b *Building) LotGatePlan {
	own := map[uint64]bool{}
	if b != nil {
		for _, id := range b.DrivewayNodeIDs {
			own[id] = true
		}
	}
	type seg struct {
		e    *RoadEdge
		a, b mgl32.Vec2
	}
	var segs []seg
	for _, e := range w.RoadEdges {
		if own[e.A] || own[e.B] || (b != nil && e.ID == b.DriveEdge) {
			continue
		}
		na, nb := w.RoadNodeByID(e.A), w.RoadNodeByID(e.B)
		if na == nil || nb == nil {
			continue
		}
		segs = append(segs, seg{e, na.Pos, nb.Pos})
	}

	ax, az := r.Axes()
	type side struct {
		n          mgl32.Vec2 // outward normal
		mid, along mgl32.Vec2
		half       float32
	}
	sides := [4]side{
		{ax, r.Center.Add(ax.Mul(r.HalfX)), az, r.HalfZ},
		{ax.Mul(-1), r.Center.Sub(ax.Mul(r.HalfX)), az, r.HalfZ},
		{az, r.Center.Add(az.Mul(r.HalfZ)), ax, r.HalfX},
		{az.Mul(-1), r.Center.Sub(az.Mul(r.HalfZ)), ax, r.HalfX},
	}
	best := LotGatePlan{}
	bestD := MaxDrivewayLength
	for _, s := range sides {
		reach := max(s.half-lotGateCornerMargin, 0)
		for t := -reach; t <= reach; t += 1 {
			p := s.mid.Add(s.along.Mul(t))
			for _, sg := range segs {
				q := ClosestPointOnRoadSegment(p, sg.a, sg.b)
				d := q.Sub(p)
				dist := d.Len()
				if dist >= bestD || d.Dot(s.n) < dist*0.7 {
					continue // farther, or not out in front of this side
				}
				bestD = dist
				best = LotGatePlan{Gate: p, Road: true, RoadPt: q, Edge: sg.e}
			}
		}
	}
	if !best.Road {
		lowest := float32(math.MaxFloat32)
		for _, s := range sides {
			c := cellOf(s.mid)
			if !w.Terrain.InBounds(c[0], c[1]) {
				continue
			}
			if e := w.Terrain.Cells[c[0]][c[1]].GroundElevation; e < lowest {
				lowest, best.Gate = e, s.mid
			}
		}
		if lowest == float32(math.MaxFloat32) {
			best.Gate = sides[0].mid
		}
		return best
	}
	// Meet the road at an existing node when one is close, rather than
	// splitting the road a few metres from it.
	for _, id := range [2]uint64{best.Edge.A, best.Edge.B} {
		if n := w.RoadNodeByID(id); n != nil && n.Pos.Sub(best.RoadPt).Len() < 8 {
			best.Snap, best.RoadPt = n, n.Pos
		}
	}
	return best
}

// ConnectLotDriveway rebuilds lot b's entrance and driveway: the entrance
// node moves to the planned gate (roads the player attached to it
// follow), the old driveway comes out (rejoining the road it split), and
// a new one runs straight to the nearest road, meeting it at a T.
// Returns the plan built.
func (w *World) ConnectLotDriveway(b *Building) LotGatePlan {
	w.disconnectLotDriveway(b)
	plan := w.PlanLotGate(b.LotRect(), b)
	w.EnsureParkingDriveway(b)
	if len(b.DrivewayNodeIDs) == 0 {
		return plan
	}
	gate := w.RoadNodeByID(b.DrivewayNodeIDs[0])
	gate.Pos = plan.Gate
	if plan.Road {
		j := plan.Snap
		if j == nil {
			j = w.SplitRoadEdge(plan.Edge, plan.RoadPt)
		}
		if j != nil {
			b.DriveEdge = w.AddRoadEdge(gate.ID, j.ID).ID
		}
	}
	w.RefreshParkingLot(b, false)
	return plan
}

// disconnectLotDriveway removes the driveway ConnectLotDriveway built,
// and the node it split the road at when nothing else uses it.
func (w *World) disconnectLotDriveway(b *Building) {
	if b.DriveEdge == 0 {
		return
	}
	var e *RoadEdge
	for _, x := range w.RoadEdges {
		if x.ID == b.DriveEdge {
			e = x
		}
	}
	b.DriveEdge = 0
	if e == nil {
		return
	}
	j := e.A
	if len(b.DrivewayNodeIDs) > 0 && j == b.DrivewayNodeIDs[0] {
		j = e.B
	}
	w.removeRoadEdge(e.ID)
	n := w.RoadNodeByID(j)
	if n == nil || n.Kind != RoadNodeIntersection {
		return
	}
	var rest []*RoadEdge
	for _, x := range w.RoadEdges {
		if x.A == j || x.B == j {
			rest = append(rest, x)
		}
	}
	switch len(rest) {
	case 0:
		w.RemoveRoadNode(j)
	case 2:
		a, c := otherEndpoint(rest[0], j), otherEndpoint(rest[1], j)
		w.RemoveRoadNode(j)
		w.AddRoadEdge(a, c)
	}
}

func (w *World) removeRoadEdge(id uint64) {
	for i, e := range w.RoadEdges {
		if e.ID == id {
			w.RoadEdges = append(w.RoadEdges[:i], w.RoadEdges[i+1:]...)
			return
		}
	}
}

// layoutLot lays out the stalls of lot rectangle r with its entrance at
// gate, and returns them nearest-entrance first, with the aisles' local
// V offsets.
func layoutLot(r FootprintRect, gate mgl32.Vec2) ([]ParkingStall, []float32) {
	u, v, halfU, halfV := LotAxes(r)
	const (
		sw     = ParkingStallWidth
		sl     = ParkingStallLength
		aisle  = ParkingAisleWidth
		period = 2*sl + aisle
	)
	at := func(lu, lv float32) mgl32.Vec2 { return r.Center.Add(u.Mul(lu)).Add(v.Mul(lv)) }
	heading := func(dir float32) float32 { // dir: +1 aisle toward +V
		d := v.Mul(dir)
		return float32(math.Atan2(float64(d[0]), float64(d[1])))
	}

	// Rows across V, centred.
	width := 2 * halfV
	n := int(width / period)
	single := width-float32(n)*period >= sl+aisle
	used := float32(n) * period
	if single {
		used += sl + aisle
	}
	v0 := -halfV + (width-used)/2
	type row struct {
		v   float32 // stall centre, local V
		dir float32 // toward its aisle
	}
	var rows []row
	var aisles []float32
	for k := 0; k < n; k++ {
		base := v0 + float32(k)*period
		rows = append(rows, row{base + sl/2, 1}, row{base + sl + aisle + sl/2, -1})
		aisles = append(aisles, base+sl+aisle/2)
	}
	if single {
		base := v0 + float32(n)*period
		aisles = append(aisles, base+aisle/2)
		rows = append(rows, row{base + aisle + sl/2, -1})
	}

	// Stalls along U between the end aisles, centred.
	m := int((2*halfU - 2*aisle) / sw)
	u0 := -float32(m)*sw/2 + sw/2

	// The entrance's corridor: on a long side, the stalls between it and
	// the nearest aisle are left out.
	d := gate.Sub(r.Center)
	gu, gv := d.Dot(u), d.Dot(v)
	onLong := abs32(abs32(gv)-halfV) < abs32(abs32(gu)-halfU)
	var nearAisle float32
	if len(aisles) > 0 {
		nearAisle = aisles[0]
		for _, a := range aisles {
			if abs32(a-gv) < abs32(nearAisle-gv) {
				nearAisle = a
			}
		}
	}

	var stalls []ParkingStall
	for _, rw := range rows {
		for i := 0; i < m; i++ {
			su := u0 + float32(i)*sw
			if onLong && abs32(su-gu) < aisle/2+sw/2 && (rw.v-nearAisle)*(gv-nearAisle) > 0 {
				continue
			}
			stalls = append(stalls, ParkingStall{
				Pos:     at(su, rw.v),
				Heading: heading(rw.dir),
				Aisle:   at(su, rw.v+rw.dir*(sl/2+aisle/2)),
			})
		}
	}
	sortStallsFrom(stalls, gate)
	return stalls, aisles
}

// LotDrive is the path a car takes inside lot b from its entrance to
// stall i: in to the nearest aisle (or the end aisle), along the aisles
// to the one in front of the stall, then into the stall.
func (b *Building) LotDrive(i int) []mgl32.Vec2 {
	st := b.Stalls[i]
	if !b.IsRectLot() || len(b.lotAisles) == 0 {
		return []mgl32.Vec2{b.Gate, st.Aisle, st.Pos}
	}
	r := b.LotRect()
	u, v, halfU, halfV := LotAxes(r)
	local := func(p mgl32.Vec2) (float32, float32) {
		d := p.Sub(r.Center)
		return d.Dot(u), d.Dot(v)
	}
	at := func(lu, lv float32) mgl32.Vec2 { return r.Center.Add(u.Mul(lu)).Add(v.Mul(lv)) }
	gu, gv := local(b.Gate)
	au, av := local(st.Aisle)
	endU := halfU - ParkingAisleWidth/2
	pts := []mgl32.Vec2{b.Gate}
	if abs32(abs32(gv)-halfV) >= abs32(abs32(gu)-halfU) {
		// Entrance on a short side: into the end aisle.
		ue := sign32(gu) * endU
		pts = append(pts, at(ue, gv), at(ue, av))
	} else {
		// Entrance on a long side: through the gap to the nearest aisle.
		near := b.lotAisles[0]
		for _, a := range b.lotAisles {
			if abs32(a-gv) < abs32(near-gv) {
				near = a
			}
		}
		pts = append(pts, at(gu, near))
		if abs32(av-near) > 0.5 {
			ue := sign32(gu+au) * endU
			pts = append(pts, at(ue, near), at(ue, av))
		}
	}
	pts = append(pts, st.Aisle, st.Pos)
	out := pts[:1]
	for _, p := range pts[1:] {
		if p.Sub(out[len(out)-1]).Len() > 0.05 {
			out = append(out, p)
		}
	}
	return out
}

// sortStallsFrom orders stalls nearest-first from p.
func sortStallsFrom(stalls []ParkingStall, p mgl32.Vec2) {
	sort.SliceStable(stalls, func(i, j int) bool {
		return stalls[i].Pos.Sub(p).Len() < stalls[j].Pos.Sub(p).Len()
	})
}

// parkingAnchor returns the centre of the cell nearest the cells'
// centroid, so the anchor always sits on the footprint even for
// L-shapes. Used for service buildings.
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
