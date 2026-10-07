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
// apron in front of the lift post, the side skiers face: where they
// queue and load at the base, and ski off after unloading at the top.
// That's the bullwheel side, away from the cable (lift_station.scad's
// beam reaches 6.5 m that way). The apron is levelled to the natural
// ground at its middle. Banks ease back to natural ground on every side,
// the cable side included, since riders ski off down past the post; each
// station's banks are long enough to stay under liftApronMaxGrade, so a
// beginner can ski them. The flat part is flattened in the lidar detail
// too, so it's really level.
const (
	liftCorridorHalfWidth = float32(12.0) // → 24 m wide maintenance lane
	liftApronDepth        = float32(8.0)  // flat metres in front of the post, away from the cable
	liftApronHalfWidth    = float32(6.0)  // flat metres either side of the cable axis
	liftApronBank         = float32(8.0)  // shortest bank: metres over which the cut and fill ease back to natural
	liftApronMaxBank      = float32(24.0) // longest bank before the apron is lowered or raised instead
	liftApronMaxShift     = float32(4.0)  // most the apron may be raised or lowered from its target to fit
)

// liftApronMaxGrade is the steepest the earthwork may make the ground:
// the most a beginner holds their balance on, 1.5× their 10° comfort
// slope, where the balance model's slope drain matches its recovery
// (sim.stressDelta). A tighter cap can't be met on an ordinary hill: a
// flat apron cut into 11° ground needs banks steeper than the hill.
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
	clearLiftCorridor(t, lift.Base, lift.Top, liftCorridorHalfWidth)
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
	mid := axis.Mul(liftApronDepth / 2)
	topTarget := stationGroundElev(t, lift.Top.Add(mid))
	baseTarget := stationGroundElev(t, lift.Base.Sub(mid))
	carveStationApron(t, lift.Top, axis, +1, topTarget, claimed)
	carveStationApron(t, lift.Base, axis, -1, baseTarget, claimed)
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
}

// apronWeight is how fully the point d metres from a station (d in
// world XZ) is graded to the apron: 1 on the flat part, from the post to
// liftApronDepth beyond it, easing to 0 across a bank of the given length
// on every side. side is the direction the station faces along axis: -1
// for the base, +1 for the top.
func apronWeight(d, axis mgl32.Vec2, side, bank float32) float32 {
	along := (d[0]*axis[0] + d[1]*axis[1]) * side
	perp := float32(math.Abs(float64(d[0]*axis[1] - d[1]*axis[0])))
	outA := max(0, along-liftApronDepth, -along)
	outP := max(0, perp-liftApronHalfWidth)
	dist := float32(math.Hypot(float64(outA), float64(outP)))
	return 1 - smoothstep32(0, 1, dist/bank)
}

// fitApron returns the bank length and apron height for a station so no
// ground the earthwork touches ends up steeper than liftApronMaxGrade.
// It prefers the apron at target with the shortest bank, then longer
// banks, then the apron raised or lowered up to liftApronMaxShift in
// half-metre steps. Where the hill itself is too steep for any of those,
// it takes the one that oversteepens least.
func fitApron(t *world.Terrain, station, axis mgl32.Vec2, side, target float32, claimed map[[2]int]bool) (bank, height float32) {
	best := float32(math.Inf(1))
	for k := 0; k <= int(2*liftApronMaxShift); k++ {
		for _, sign := range [2]float32{-1, 1} {
			if k == 0 && sign > 0 {
				continue
			}
			h := target + sign*float32(k)/2
			for b := liftApronBank; b <= liftApronMaxBank; b += 4 {
				excess := apronExcess(t, station, axis, side, h, b, claimed)
				if excess <= 0 {
					return b, h
				}
				if excess < best {
					best, bank, height = excess, b, h
				}
			}
		}
	}
	return bank, height
}

// apronExcess is how far past liftApronMaxGrade the steepest cell the
// carve changes would be, after carving a station's apron at height with
// the given bank. Slope is measured as world.Terrain.GradientAt does: the
// gradient's magnitude from central differences. 0 when the carve fits.
func apronExcess(t *world.Terrain, station, axis mgl32.Vec2, side, height, bank float32, claimed map[[2]int]bool) float32 {
	const cellSize = float32(5.0)
	bound := float32(math.Hypot(float64(liftApronDepth), float64(liftApronHalfWidth))) + bank + 2*cellSize
	x0, x1 := max(int((station[0]-bound)/cellSize), 1), min(int((station[0]+bound)/cellSize), t.Width-2)
	z0, z1 := max(int((station[1]-bound)/cellSize), 1), min(int((station[1]+bound)/cellSize), t.Height-2)
	carved := func(x, z int) (elev, wgt float32) {
		g := t.Cells[x][z].GroundElevation
		if claimed[[2]int{x, z}] {
			return g, 0
		}
		c := mgl32.Vec2{(float32(x) + 0.5) * cellSize, (float32(z) + 0.5) * cellSize}
		wgt = apronWeight(c.Sub(station), axis, side, bank)
		return g + (height-g)*wgt, wgt
	}
	worst := float32(0)
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			_, w0 := carved(x, z)
			ex0, wx0 := carved(x-1, z)
			ex1, wx1 := carved(x+1, z)
			ez0, wz0 := carved(x, z-1)
			ez1, wz1 := carved(x, z+1)
			if w0+wx0+wx1+wz0+wz1 == 0 {
				continue // untouched ground keeps its slope
			}
			gx := (ex1 - ex0) / (2 * cellSize)
			gz := (ez1 - ez0) / (2 * cellSize)
			worst = max(worst, float32(math.Hypot(float64(gx), float64(gz)))-liftApronMaxGrade)
		}
	}
	return worst
}

// carveStationApron grades the ground around one station to target:
// cells (cut and fill), the lidar detail, and the material map, then
// packs and grooms the snow on the flat part. Claimed cells (other
// buildings' pads) keep their ground.
func carveStationApron(t *world.Terrain, station, axis mgl32.Vec2, side, target float32, claimed map[[2]int]bool) {
	const cellSize = float32(5.0)
	bank, target := fitApron(t, station, axis, side, target, claimed)
	bound := float32(math.Hypot(float64(liftApronDepth), float64(liftApronHalfWidth))) + bank + cellSize
	x0, x1 := int((station[0]-bound)/cellSize), int((station[0]+bound)/cellSize)
	z0, z1 := int((station[1]-bound)/cellSize), int((station[1]+bound)/cellSize)
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) || claimed[[2]int{x, z}] {
				continue
			}
			c := mgl32.Vec2{(float32(x) + 0.5) * cellSize, (float32(z) + 0.5) * cellSize}
			wgt := apronWeight(c.Sub(station), axis, side, bank)
			if wgt <= 0 {
				continue
			}
			cell := &t.Cells[x][z]
			cell.GroundElevation += (target - cell.GroundElevation) * wgt
			cell.MogulSize *= 1 - wgt
			if wgt > 0.5 {
				// Boarding pads and lift queues are foot-traffic compacted.
				if top := cell.TopLayer(); top != nil {
					top.Kind = world.KindPackedPowder
				}
				cell.Grooming = 1
				t.Groom.StampCell(x, z, axis[0], axis[1])
			}
		}
	}
	// The drawn ground is the cells' mesh plus the detail on top: level
	// the detail toward target by the same weight, so the flat part is
	// flat at 1.25 m too. Material under the flat part becomes ground.
	const per = cellSize / world.DetailPerCell
	i0, i1 := max(int((station[0]-bound)/per), 0), int((station[0]+bound)/per)
	j0, j1 := max(int((station[1]-bound)/per), 0), int((station[1]+bound)/per)
	if d := t.Detail; d != nil {
		for j := j0; j <= min(j1, d.H-1); j++ {
			for i := i0; i <= min(i1, d.W-1); i++ {
				p := mgl32.Vec2{float32(i) * per, float32(j) * per}
				if claimed[[2]int{int(p[0] / cellSize), int(p[1] / cellSize)}] {
					continue
				}
				wgt := apronWeight(p.Sub(station), axis, side, bank)
				if wgt <= 0 {
					continue
				}
				k := j*d.W + i
				mesh := t.MeshGroundAt(float32(i)/world.DetailPerCell, float32(j)/world.DetailPerCell)
				d.Off[k] = d.Off[k]*(1-wgt) + (target-mesh)*wgt
			}
		}
	}
	if m := t.Material; m != nil {
		changed := false
		for j := j0; j <= min(j1, m.H-1); j++ {
			for i := i0; i <= min(i1, m.W-1); i++ {
				p := mgl32.Vec2{float32(i) * per, float32(j) * per}
				if m.M[j*m.W+i].Bare() && apronWeight(p.Sub(station), axis, side, bank) > 0.5 {
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
