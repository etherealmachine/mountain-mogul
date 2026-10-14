package scene

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// ── Lift terminal ground effects ──────────────────────────────────────
//
// Lives in its own file so it can iterate without touching the building
// apron or the generic pad helper in scenario.go. Tuning constants are
// package-level so they're easy to find.

// Lift station ground. Nothing is built up: each station gets a flat
// apron from its bullwheel back away from the cable, under the terminal,
// its portal frame and hut (lift_station.py; world.Lift.StationBoxes)
// and either side of them: where guests queue between the legs and load
// at the base, and at the top where riders, having stood up under the
// hood's front and peeled off sideways in front of the frame, come round
// beside the hut (sim: tickUnloading) to the runs that start there
// (world.Lift.TopRunEnd). Around the apron the earthwork moves only
// the ground it must: anything steeper than liftApronMaxGrade from the
// apron's edge is cut down or filled up to that grade, and ground already
// within it is left alone, so the banks end where they meet the hill.
// The apron's height is the one, within liftApronMaxShift of the natural
// ground at its middle, that moves the least earth. It works on the
// lidar detail where there is one, so the apron is really level.
const (
	liftCorridorHalfWidth = float32(12.0) // → 24 m wide maintenance lane
	liftApronDepth        = float32(8.0)  // flat metres back from the bullwheel, away from the cable
	liftApronHalfWidth    = float32(6.0)  // flat metres either side of the cable axis (a detachable's: its roof and hut, apronShape)
	liftApronMaxBank      = float32(24.0) // furthest the grade is held from the apron's edge
	liftApronToe          = float32(8.0)  // metres past that over which the bank fades back to the hill
	liftApronMaxShift     = float32(4.0)  // most the apron may sit above or below the natural ground at its middle
)

// apronShape is a station's flat apron in its frame: from front metres
// in front of the bullwheel (under the terminal, round the load point) to
// depth behind it, and half either side of the cable axis.
type apronShape struct{ front, depth, half float32 }

// liftApron is lift l's stations' apron: under the whole terminal.
func liftApron(l *world.Lift) apronShape {
	a := apronShape{front: l.Station().ApronFront, depth: liftApronDepth, half: liftApronHalfWidth}
	switch {
	case l.Station().Enclosed:
		// The station house, and room behind it for riders walking out.
		a.half, a.depth = max(6.5, l.Station().HalfWidth()+1.4), max(a.depth, l.Station().Rear()+7)
	case l.Type == world.LiftHS8Pack:
		a.half = 8.5 // its wider roof, posts and hut
	case l.Type.Detachable():
		a.half = 6.5 // the roof, its posts and the hut
	case l.Type.Hangers():
		// The mast behind the wheel, the control post, and room to take
		// a bar and let it go.
		a = apronShape{front: l.Station().ApronFront, depth: 5, half: 4.5}
	case l.Type.Surface():
		// The belt's end or the rope's bullwheel and motor, the control
		// post, and room to step off.
		a = apronShape{front: l.Station().ApronFront, depth: 5, half: 3.5}
	}
	return a
}

// liftApronMaxGrade is the steepest the earthwork may make the ground:
// the most a beginner holds their balance on, 1.5× their 10° comfort
// slope, where the balance model's slope drain matches its recovery
// (sim.stressDelta). Natural ground steeper than this is left as it is
// unless the earthwork has to move it anyway.
var liftApronMaxGrade = float32(math.Tan(15 * math.Pi / 180))

// applyLiftPlacementEffects applies the ground-side consequences of
// putting down a lift: a tree-free maintenance corridor under the
// cable, a carved apron at each station, and a grooming pass over each
// apron. Real loading and unloading lanes are flattened daily, so the
// apron starts groomed.
//
// Lives outside world.PlaceLift so save loading and testbed setup can
// reconstruct lifts without re-applying ground edits the player may
// have made afterward.
func applyLiftPlacementEffects(w *world.World, lift *world.Lift) {
	t := w.Terrain
	corridor := liftCorridorHalfWidth
	if lift.Type.Surface() {
		corridor = 4 // no towers to service: the belt or rope and a lane beside it
		if lift.Type.Hangers() {
			corridor = 5 // its towers, the track and the return
		}
	}
	clearLiftCorridor(t, lift.Base, lift.Top, corridor)
	axis := mgl32.Vec2{
		lift.Top[0] - lift.Base[0],
		lift.Top[1] - lift.Base[1],
	}
	if l := axis.Len(); l > 0 {
		axis = axis.Mul(1 / l)
	}
	// Each apron faces away from the cable: the base's against axis
	// (downhill of the base), the top's along it (beyond the top). Each
	// is levelled to the natural ground at its middle, read before either
	// is carved.
	claimed := claimedGround(w, nil, lift)
	a := liftApron(lift)
	mid := axis.Mul((a.depth - a.front) / 2)
	topTarget := stationGroundElev(t, lift.Top.Add(mid))
	baseTarget := stationGroundElev(t, lift.Base.Sub(mid))
	carveStationApron(t, lift.Top, axis, +1, a, topTarget, claimed, true)
	carveStationApron(t, lift.Base, axis, -1, a, baseTarget, claimed, true)
	// Stamp queue + top cells impassable so the structure-stamp path
	// matches PlaceLift's blocking.
	queue := lift.QueueCell()
	if t.InBounds(queue[0], queue[1]) {
		t.Cells[queue[0]][queue[1]].Passable = false
	}
	top := lift.TopCell()
	if t.InBounds(top[0], top[1]) {
		t.Cells[top[0]][top[1]].Passable = false
	}
	// Trees were zeroed under the corridor and apron; refresh the
	// surface-detail G channel so the well texture matches the new
	// (smaller) tree set.
	t.RestampTreeWells()
	t.RecomputeSlopes()
	// Its cable is laid out from the ground as it is now.
	lift.ResetTowerCache()
}

// apronDist is how far the point d metres from a station (d in world
// XZ) is outside the station's flat apron a: 0 on it, from a.front in
// front of the bullwheel to a.depth behind it and a.half either side. side is
// the direction the station faces along axis: -1 for the base, +1 for
// the top.
func apronDist(d, axis mgl32.Vec2, side float32, a apronShape) float32 {
	dist, _ := apronEdge(d, axis, side, a)
	return dist
}

// apronEdge is apronDist and the unit direction from the nearest point of
// the apron out to d (zero on the apron).
func apronEdge(d, axis mgl32.Vec2, side float32, a apronShape) (float32, mgl32.Vec2) {
	along := (d[0]*axis[0] + d[1]*axis[1]) * side
	perpSigned := d[0]*axis[1] - d[1]*axis[0]
	perp := float32(math.Abs(float64(perpSigned)))
	outA := max(0, along-a.depth, -along-a.front)
	outP := max(0, perp-a.half)
	dist := float32(math.Hypot(float64(outA), float64(outP)))
	if dist == 0 {
		return 0, mgl32.Vec2{}
	}
	// Back to world XZ: along runs side·axis, perp the axis turned a
	// quarter (positive where perpSigned is).
	oa := outA
	if along < 0 {
		oa = -outA
	}
	pp := outP
	if perpSigned < 0 {
		pp = -outP
	}
	dir := axis.Mul(oa * side).Add(mgl32.Vec2{axis[1], -axis[0]}.Mul(pp))
	return dist, dir.Mul(1 / dist)
}

// apronGrade is the ground dist metres from an apron at height that
// stood at natural: the apron itself, or the hill held to within
// liftApronMaxGrade of the apron's edge, cut where it's above and, where
// fill is allowed, filled where it's below; the cut fades back to the
// hill past liftApronMaxBank. fill is how much filling this direction
// takes, 0..1 (apronFill).
func apronGrade(height, natural, dist, fill float32) float32 {
	if dist <= 0 {
		return height
	}
	rise := liftApronMaxGrade * dist
	v := min(natural, height+rise)
	if low := height - rise; natural < low {
		v = natural + (low-natural)*fill
	}
	if over := dist - liftApronMaxBank; over > 0 {
		v += (natural - v) * smoothstep32(0, 1, over/liftApronToe)
	}
	return v
}

// apronFill is how much fill the hill takes in the direction a point
// sits from the apron: all of it where the grade down from the apron
// meets the hill within liftApronMaxBank, none where the hill falls away
// faster (any fill there would end in a drop steeper than the hill),
// easing over the last two metres. farNatural is the hill liftApronMaxBank
// out from the apron's edge that way.
func apronFill(height, farNatural float32) float32 {
	short := height - liftApronMaxGrade*liftApronMaxBank - farNatural
	return 1 - smoothstep32(0, 1, short/2)
}

// carveStationApron grades the ground around one station (cells, the
// lidar detail, and the material map), then packs and grooms the snow
// on the flat part. With search, the apron's height is the one within
// liftApronMaxShift of target, in half-metre steps, that moves the least
// earth (nearest target on a tie); without, it's target. Claimed cells
// (other buildings' pads) keep their ground.
func carveStationApron(t *world.Terrain, station, axis mgl32.Vec2, side float32, a apronShape, target float32, claimed map[[2]int]bool, search bool) {
	const cellSize = float32(5.0)
	bound := float32(math.Hypot(float64(max(a.depth, a.front)), float64(a.half))) + liftApronMaxBank + liftApronToe + cellSize
	x0, x1 := int((station[0]-bound)/cellSize), int((station[0]+bound)/cellSize)
	z0, z1 := int((station[1]-bound)/cellSize), int((station[1]+bound)/cellSize)
	p := t.GroundPatch(x0, z0, x1, z1, 0)
	keep := func(x, z int) bool { return claimed[[2]int{x, z}] }
	// Each sample's distance from the apron (-1 for one on claimed ground
	// or outside the block, which stays as it is), and the hill
	// liftApronMaxBank out from the apron's edge in its direction, for
	// whether that way can be filled.
	dist := make([]float32, len(p.Before))
	far := make([]float32, len(p.Before))
	for j := 0; j < p.H; j++ {
		for i := 0; i < p.W; i++ {
			k := j*p.W + i
			wx, wz := p.Pos(i, j)
			cx, cz := int(math.Floor(float64(wx/cellSize))), int(math.Floor(float64(wz/cellSize)))
			if cx < x0 || cx > x1 || cz < z0 || cz > z1 || keep(cx, cz) {
				dist[k] = -1
				continue
			}
			d, dir := apronEdge(mgl32.Vec2{wx, wz}.Sub(station), axis, side, a)
			dist[k] = d
			if d > 0 {
				edge := mgl32.Vec2{wx, wz}.Sub(dir.Mul(d))
				q := edge.Add(dir.Mul(liftApronMaxBank))
				far[k] = p.BeforeAtWorld(q[0], q[1])
			}
		}
	}
	grade := func(h float32, k int) float32 {
		return apronGrade(h, p.Before[k], dist[k], apronFill(h, far[k]))
	}
	earth := func(h float32) float32 {
		var sum float32
		for k, d := range dist {
			if d >= 0 {
				sum += float32(math.Abs(float64(grade(h, k) - p.Before[k])))
			}
		}
		return sum
	}
	height := target
	if search {
		best := earth(target)
		for k := 1; k <= int(2*liftApronMaxShift); k++ {
			for _, sign := range [2]float32{-1, 1} {
				h := target + sign*float32(k)/2
				if e := earth(h); e < best {
					best, height = e, h
				}
			}
		}
	}
	for k, d := range dist {
		if d >= 0 {
			p.After[k] = grade(height, k)
		}
	}
	before := make(map[[2]int]float32)
	for x := max(x0, 0); x <= min(x1, t.Width-1); x++ {
		for z := max(z0, 0); z <= min(z1, t.Height-1); z++ {
			before[[2]int{x, z}] = t.Cells[x][z].GroundElevation
		}
	}
	p.Commit(keep)
	// Snow: moguls go wherever the ground moved; the flat part is packed
	// and groomed, as foot traffic and the daily grooming of real loading
	// lanes leave it.
	for c, old := range before {
		if keep(c[0], c[1]) {
			continue
		}
		cell := &t.Cells[c[0]][c[1]]
		if math.Abs(float64(cell.GroundElevation-old)) > 0.05 {
			cell.MogulSize = 0
		}
		centre := mgl32.Vec2{(float32(c[0]) + 0.5) * cellSize, (float32(c[1]) + 0.5) * cellSize}
		if apronDist(centre.Sub(station), axis, side, a) <= 0 {
			if top := cell.TopLayer(); top != nil {
				top.Kind = world.KindPackedPowder
			}
			cell.Grooming = 1
			t.Groom.StampCell(c[0], c[1], axis[0], axis[1])
		}
	}
	// Bare material under the flat part becomes ground.
	if m := t.Material; m != nil {
		const per = cellSize / world.DetailPerCell
		i0, i1 := max(int((station[0]-bound)/per), 0), min(int((station[0]+bound)/per), m.W-1)
		j0, j1 := max(int((station[1]-bound)/per), 0), min(int((station[1]+bound)/per), m.H-1)
		changed := false
		for j := j0; j <= j1; j++ {
			for i := i0; i <= i1; i++ {
				q := mgl32.Vec2{float32(i) * per, float32(j) * per}
				if m.M[j*m.W+i].Bare() && apronDist(q.Sub(station), axis, side, a) <= 0 {
					m.M[j*m.W+i] = world.MatMeadow
					changed = true
				}
			}
		}
		if changed {
			m.Version++
		}
	}
}

// applyHelipadPlacementEffects applies the ground-side consequences of placing
// a heli-ski lift: flatten and groom a circular pad at each endpoint, clear
// trees in a generous radius around both pads.  No corridor is cleared between
// the two points — the helicopter flies over the terrain, not along it.
func applyHelipadPlacementEffects(t *world.Terrain, lift *world.Lift) {
	const padRadius = float32(10.0) // flat landing-zone radius around each pad
	const clearRadius = float32(18.0)
	for _, pos := range []mgl32.Vec2{lift.Base, lift.Top} {
		target := stationGroundElev(t, pos)
		flattenCircle(t, pos, padRadius, target)
		clearCircle(t, pos, clearRadius)
		groomCircle(t, pos, padRadius)
	}
	t.RestampTreeWells()
	t.RecomputeSlopes()
}

// flattenCircle levels all cells within radius metres of pos to targetElev.
func flattenCircle(t *world.Terrain, pos mgl32.Vec2, radius, targetElev float32) {
	const cellSize = float32(5.0)
	r2 := radius * radius
	x0 := int((pos[0] - radius) / cellSize)
	x1 := int((pos[0]+radius)/cellSize) + 1
	z0 := int((pos[1] - radius) / cellSize)
	z1 := int((pos[1]+radius)/cellSize) + 1
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			cx := (float32(x) + 0.5) * cellSize
			cz := (float32(z) + 0.5) * cellSize
			dx := cx - pos[0]
			dz := cz - pos[1]
			if dx*dx+dz*dz <= r2 {
				t.Cells[x][z].GroundElevation = targetElev
			}
		}
	}
}

// clearCircle removes the trees within radius metres of pos.
func clearCircle(t *world.Terrain, pos mgl32.Vec2, radius float32) {
	const cellSize = float32(5.0)
	r2 := radius * radius
	x0 := int((pos[0] - radius) / cellSize)
	x1 := int((pos[0]+radius)/cellSize) + 1
	z0 := int((pos[1] - radius) / cellSize)
	z1 := int((pos[1]+radius)/cellSize) + 1
	t.RemoveTreesIn(x0, z0, x1, z1, func(tr world.Tree) bool {
		dx, dz := tr.X-pos[0], tr.Z-pos[1]
		return dx*dx+dz*dz <= r2
	})
}

// groomCircle sets Grooming=1 within radius metres of pos.
func groomCircle(t *world.Terrain, pos mgl32.Vec2, radius float32) {
	const cellSize = float32(5.0)
	r2 := radius * radius
	x0 := int((pos[0] - radius) / cellSize)
	x1 := int((pos[0]+radius)/cellSize) + 1
	z0 := int((pos[1] - radius) / cellSize)
	z1 := int((pos[1]+radius)/cellSize) + 1
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			cx := (float32(x) + 0.5) * cellSize
			cz := (float32(z) + 0.5) * cellSize
			dx := cx - pos[0]
			dz := cz - pos[1]
			if dx*dx+dz*dz <= r2 {
				t.Cells[x][z].Grooming = 1
				fx, fz := t.FallLineAt(x, z)
				t.Groom.StampCell(x, z, fx, fz)
			}
		}
	}
}

// clearLiftCorridor removes the trees within `halfWidth` metres of the
// line segment between two world XZ points. Models the standard
// chairlift maintenance lane — trees would otherwise foul cables, towers,
// and the over-snow grooming machines that service the line.
func clearLiftCorridor(t *world.Terrain, base, top mgl32.Vec2, halfWidth float32) {
	const cellSize = float32(5.0)
	minX := minF(base[0], top[0]) - halfWidth
	maxX := maxF(base[0], top[0]) + halfWidth
	minZ := minF(base[1], top[1]) - halfWidth
	maxZ := maxF(base[1], top[1]) + halfWidth
	x0 := int(minX / cellSize)
	x1 := int(maxX/cellSize) + 1
	z0 := int(minZ / cellSize)
	z1 := int(maxZ/cellSize) + 1
	hw2 := halfWidth * halfWidth
	t.RemoveTreesIn(x0, z0, x1, z1, func(tr world.Tree) bool {
		return pointSegmentDistSq(mgl32.Vec2{tr.X, tr.Z}, base, top) <= hw2
	})
}

// apronLevelSlack is how far, metres, the ground under a station's apron
// may stray from level before levelStationAprons regrades it.
const apronLevelSlack = float32(0.15)

// levelStationAprons regrades the ground under any lift station whose
// apron isn't level, at its bullwheel's height: saves made before the
// terminals reached in front of the wheel levelled only behind it, so the
// longer terminals stood over a slope. A level apron is left alone.
func levelStationAprons(w *world.World) {
	t := w.Terrain
	changed := false
	for _, l := range w.Lifts {
		if l.Cableless() {
			continue
		}
		axis := l.Top.Sub(l.Base)
		if n := axis.Len(); n > 0 {
			axis = axis.Mul(1 / n)
		}
		a := liftApron(l)
		for _, end := range []struct {
			at   mgl32.Vec2
			side float32
		}{{l.Base, -1}, {l.Top, +1}} {
			claimed := claimedGround(w, nil, l)
			if apronIsLevel(t, end.at, axis, end.side, a, claimed) {
				continue
			}
			carveStationApron(t, end.at, axis, end.side, a, t.GroundHeightAt(end.at[0], end.at[1]), claimed, false)
			l.ResetTowerCache()
			changed = true
		}
	}
	if changed {
		t.RestampTreeWells()
		t.RecomputeSlopes()
	}
}

// apronIsLevel reports whether the ground over apron a of the station at
// station is within apronLevelSlack of the ground at the bullwheel,
// sampled every metre; claimed ground (other buildings' pads), which the
// carve leaves alone, doesn't count.
func apronIsLevel(t *world.Terrain, station, axis mgl32.Vec2, side float32, a apronShape, claimed map[[2]int]bool) bool {
	h := t.GroundHeightAt(station[0], station[1])
	across := mgl32.Vec2{axis[1], -axis[0]}
	for along := -a.front; along <= a.depth; along++ {
		for off := -a.half; off <= a.half; off++ {
			p := station.Add(axis.Mul(along * side)).Add(across.Mul(off))
			if claimed[[2]int{int(p[0] / world.CellSize), int(p[1] / world.CellSize)}] {
				continue
			}
			if d := t.GroundHeightAt(p[0], p[1]) - h; d > apronLevelSlack || d < -apronLevelSlack {
				return false
			}
		}
	}
	return true
}
