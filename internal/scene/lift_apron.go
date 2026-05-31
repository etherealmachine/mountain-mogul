package scene

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// ── Lift terminal ground effects ──────────────────────────────────────
//
// Lives in its own file so it can iterate without touching the building
// apron or the generic pad helper in scenario.go. Tuning constants are
// package-level so they're easy to find.

// Apron and corridor sizing. Aprons are symmetric for now (top and
// base use the same rectangle); the lift-station mesh is smaller than
// a single cell, so the apron is what gives a lift visual mass on the
// map. Tuned against the 5 m terrain grid.
const (
	liftCorridorHalfWidth = float32(12.0) // → 24 m wide maintenance lane
	liftApronHalfWidth    = float32(20.0) // → 40 m total cross-axis width
	liftApronDepth        = float32(24.0)
	liftApronBuildup      = float32(2.5) // metres above station footing (base station only)

	// Top-station cut-and-fill tuning. The top apron is shelved into
	// the hillside: we sample the max natural ground elevation within
	// liftStationFootprintRadius of the anchor (which covers the column
	// + beam + bullwheel in local space) and set the platform target to
	// that maximum plus a low lip. cut=true on this apron then pulls
	// uphill cells down to the shelf and fills downhill cells up. Real
	// top stations are almost always built this way; raise-only doesn't
	// work because the beam extends 6.5 m horizontally over terrain
	// that's still rising past the anchor.
	liftStationFootprintRadius = float32(7.0)
	liftTopApronLip            = float32(0.5)
)

// applyLiftPlacementEffects applies the ground-side consequences of
// putting down a lift: a tree-free maintenance corridor under the
// cable, raised raise-only aprons at each station, and a grooming
// pass over each apron rectangle. Real loading and unloading lanes
// are flattened daily, so the apron starts groomed.
//
// Lives outside world.PlaceLift so save loading and testbed setup can
// reconstruct lifts without re-applying ground edits the player may
// have made afterward.
func applyLiftPlacementEffects(t *world.Terrain, lift *world.Lift) {
	clearLiftCorridor(t, lift.Base, lift.Top, liftCorridorHalfWidth)
	axis := mgl32.Vec2{
		lift.Top[0] - lift.Base[0],
		lift.Top[1] - lift.Base[1],
	}
	if l := axis.Len(); l > 0 {
		axis = axis.Mul(1 / l)
	}
	// Top extends forward (along axis), base extends backward (against
	// axis). Asymmetric grading: the top is shelved (cut-and-fill) into
	// the hillside so the column doesn't clip uphill terrain, while the
	// base stays raise-only — bottom-of-lift sites are typically flatter
	// (parking, day lodge) and carving there would read as wrong.
	// Reads live cell elevations — fine because each lift is stamped
	// exactly once at placement (or drag) when the cells are still
	// natural. The apron leaves a permanent footprint; re-stamping is
	// not part of the model.
	topTarget := maxGroundIn(t, lift.Top, liftStationFootprintRadius) + liftTopApronLip
	baseTarget := stationGroundElev(t, lift.Base) + liftApronBuildup
	// No snow plow on lift aprons — Packed=1.0 (set by buildStationApron's
	// inner zone) shrinks the visible column on its own, which reads as
	// the packed-down boarding pad without removing snow under the skiers.
	buildStationApron(t, lift.Top, axis, +1, liftApronHalfWidth, liftApronDepth, topTarget, true)
	buildStationApron(t, lift.Base, axis, -1, liftApronHalfWidth, liftApronDepth, baseTarget, false)
	groomLiftApron(t, lift.Top, axis, +1)
	groomLiftApron(t, lift.Base, axis, -1)
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

// maxGroundIn returns the maximum GroundElevation across cells whose
// centres lie within `radius` metres of pos. Used to size the top
// station's cut-and-fill platform so the shelf clears the highest
// ground under the mesh footprint. Falls back to the anchor cell's
// elevation if no in-bounds cells are sampled.
func maxGroundIn(t *world.Terrain, pos mgl32.Vec2, radius float32) float32 {
	const cellSize = float32(5.0)
	x0 := int((pos[0] - radius) / cellSize)
	x1 := int((pos[0]+radius)/cellSize) + 1
	z0 := int((pos[1] - radius) / cellSize)
	z1 := int((pos[1]+radius)/cellSize) + 1
	r2 := radius * radius
	have := false
	var hi float32
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			cx := (float32(x) + 0.5) * cellSize
			cz := (float32(z) + 0.5) * cellSize
			dx := cx - pos[0]
			dz := cz - pos[1]
			if dx*dx+dz*dz > r2 {
				continue
			}
			e := t.Cells[x][z].GroundElevation
			if !have || e > hi {
				hi = e
				have = true
			}
		}
	}
	if !have {
		return stationGroundElev(t, pos)
	}
	return hi
}

// groomLiftApron sets Grooming=1 across the apron rectangle for one
// station. Separate pass from buildStationApron (which is shared with
// buildings) so grooming is a lift-only concern — building aprons are
// plowed bare ground where the Grooming field is meaningless.
//
// Matches the apron rectangle exactly (same axis, side, halfWidth,
// depth) so the groomed zone lines up with the graded zone. No
// smoothstep falloff — the entire apron reads as freshly groomed and
// surrounding cells stay at their natural grooming level.
func groomLiftApron(t *world.Terrain, station, axis mgl32.Vec2, side float32) {
	const cellSize = float32(5.0)
	bound := liftApronHalfWidth
	if liftApronDepth > bound {
		bound = liftApronDepth
	}
	x0 := int((station[0] - bound) / cellSize)
	x1 := int((station[0]+bound)/cellSize) + 1
	z0 := int((station[1] - bound) / cellSize)
	z1 := int((station[1]+bound)/cellSize) + 1
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			cx := (float32(x) + 0.5) * cellSize
			cz := (float32(z) + 0.5) * cellSize
			dx := cx - station[0]
			dz := cz - station[1]
			alongRaw := dx*axis[0] + dz*axis[1]
			signedAlong := alongRaw * side
			if signedAlong < 0 || signedAlong > liftApronDepth {
				continue
			}
			perpX := dx - alongRaw*axis[0]
			perpZ := dz - alongRaw*axis[1]
			perpDistSq := perpX*perpX + perpZ*perpZ
			if perpDistSq > liftApronHalfWidth*liftApronHalfWidth {
				continue
			}
			t.Cells[x][z].Grooming = 1
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

// clearCircle zeros TreeDensity within radius metres of pos.
func clearCircle(t *world.Terrain, pos mgl32.Vec2, radius float32) {
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
				t.Cells[x][z].TreeDensity = 0
			}
		}
	}
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
			}
		}
	}
}

// clearLiftCorridor zeros TreeDensity in cells within `halfWidth` metres
// of the line segment between two world XZ points. Models the standard
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
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			cx := (float32(x) + 0.5) * cellSize
			cz := (float32(z) + 0.5) * cellSize
			if pointSegmentDistSq(mgl32.Vec2{cx, cz}, base, top) <= hw2 {
				t.Cells[x][z].TreeDensity = 0
			}
		}
	}
}

