package sim

import (
	"image"
	"math"

	"mountain-mogul/internal/world"
)

// Grooming impact a single arrival at a cell applies. Tuned so a cat
// can convert a moderately mogul-y cell back to corduroy in one pass:
//   - Grooming jumps to full
//   - MogulSize and Ice are knocked back; not zeroed so multiple passes
//     compound (more passes → progressively fresher cord)
//   - Packed jumps to 1.0 (cat tread compresses the column completely)
//     and SnowDepth drops proportionally to conserve mass
const (
	groomMogulDecay = 0.5
	groomIceDecay   = 0.5

	// arriveCellSlack is how close to the target cell's centre the cat
	// has to get before we count it as "arrived" and groom the cell.
	// Half a cell is the natural choice — once we're inside the cell
	// boundary we're standing on it.
	arriveCellSlack = world.CellSize * 0.5

	// fullyGroomed is the threshold above which a cell is considered
	// "done" by the route picker. Slightly under 1.0 to leave room for
	// natural decay (mogul re-formation, ice from traffic) to shave
	// Grooming a touch below 1 and re-qualify the cell.
	fullyGroomed = 0.99
)

// tickSnowcats runs the grooming fleet one step. Each cat picks the
// least-groomed cell in its shed's route, drives toward it at
// SnowcatSpeed, and corduroys the cell on arrival. When the route is
// empty or fully serviced the cat parks at its shed's door.
func (s *Simulation) tickSnowcats(dt float64) {
	w := s.World
	for _, cat := range w.Snowcats {
		shed := findBuilding(w, cat.ShedID)
		if shed == nil {
			// Orphaned cat — the shed was just removed. Skip; the
			// world's RemoveBuilding hook already calls
			// RemoveSnowcatsOwnedBy, so we'll never actually run here
			// unless something else gets unlinked.
			continue
		}

		// Re-pick the target if we don't have one, or if the cell we
		// were heading to has been groomed already (e.g. another cat
		// got there first, or the player re-painted the route).
		needPick := cat.TargetCell == world.NoCellTarget ||
			!cellInRoute(shed.RouteCells, cat.TargetCell) ||
			cellGrooming(w, cat.TargetCell) >= fullyGroomed
		if needPick {
			cat.TargetCell = pickNextRouteCell(w, shed, cat.Pos)
		}

		// Compute world-space target. No route cell available → park
		// at the shed door instead of just stalling in place; reads as
		// "cat returned to base" visually.
		var targetWX, targetWZ float32
		grooming := false
		if cat.TargetCell == world.NoCellTarget {
			door := shed.DoorCell()
			targetWX = (float32(door[0]) + 0.5) * world.CellSize
			targetWZ = (float32(door[1]) + 0.5) * world.CellSize
		} else {
			targetWX = (float32(cat.TargetCell[0]) + 0.5) * world.CellSize
			targetWZ = (float32(cat.TargetCell[1]) + 0.5) * world.CellSize
			grooming = true
		}

		arrived := cat.DriveToward(targetWX, targetWZ, dt, arriveCellSlack)
		// Keep the cat sitting on the snow surface visually even when
		// moving. Reading per-frame is cheap compared with the rest of
		// the tick.
		cat.Pos[1] = w.Terrain.InterpolatedSurfaceElevationAt(cat.Pos[0], cat.Pos[2])

		if arrived && grooming {
			groomCell(w, cat.TargetCell)
			cat.TargetCell = world.NoCellTarget // re-pick next tick
		}
	}
}

// pickNextRouteCell scans the shed's route for the worst-groomed cell,
// breaking ties with distance from `pos` (so a cat sweeps a region
// rather than darting across the route to pick at random equally-bad
// cells). Returns NoCellTarget if no route cell qualifies — either the
// route is empty or every cell is already fully groomed.
func pickNextRouteCell(w *world.World, shed *world.Building, pos [3]float32) [2]int {
	bestCell := world.NoCellTarget
	bestScore := float32(math.MaxFloat32)
	for _, c := range shed.RouteCells {
		if !w.Terrain.InBounds(c[0], c[1]) {
			continue
		}
		g := w.Terrain.Cells[c[0]][c[1]].Grooming
		if g >= fullyGroomed {
			continue
		}
		// Score: grooming dominates (low = bad = worth driving to),
		// distance a small tiebreaker so a cat doesn't jump back and
		// forth between equally-degraded cells on opposite ends of
		// the route.
		dx := (float32(c[0])+0.5)*world.CellSize - pos[0]
		dz := (float32(c[1])+0.5)*world.CellSize - pos[2]
		distSq := dx*dx + dz*dz
		score := g + distSq*1e-4
		if score < bestScore {
			bestScore = score
			bestCell = c
		}
	}
	return bestCell
}

// cellInRoute returns true if `c` appears in the route. Linear scan;
// route sizes are bounded by MaxRouteCells (~720 cells max with a
// fully kitted-out shed) and the call is once per cat per tick, so a
// hash map isn't worth the bookkeeping yet.
func cellInRoute(route [][2]int, c [2]int) bool {
	for _, r := range route {
		if r == c {
			return true
		}
	}
	return false
}

// cellGrooming reads the cell's current Grooming value; returns 1.0
// (treated as fully groomed → skip) for out-of-bounds inputs so a
// stale target cell can never wedge a cat in place.
func cellGrooming(w *world.World, c [2]int) float32 {
	if !w.Terrain.InBounds(c[0], c[1]) {
		return 1.0
	}
	return w.Terrain.Cells[c[0]][c[1]].Grooming
}

// groomCell applies a single cat-pass to the cell at `c`. Grooming
// goes to full; mogul roughness and surface ice are knocked back; the
// snow column compresses to fully packed (Packed → 1) and SnowDepth
// drops proportionally under snow-water-equivalent conservation. A
// fresh-powder cell at Packed=0 settles to ~40 % of its original depth
// in one pass; a moderately-packed default cell (Packed=0.5) drops to
// ~70 %. The visible result is that groomed lanes sit visibly lower
// than the adjacent untracked snow — exactly what real corduroy looks
// like next to a powder shoulder.
//
// Marks the terrain dirty so the scene re-uploads the mesh on the
// next frame.
func groomCell(w *world.World, c [2]int) {
	if !w.Terrain.InBounds(c[0], c[1]) {
		return
	}
	cell := &w.Terrain.Cells[c[0]][c[1]]

	// Packed → 1.0 raises the column density; SnowAccumulation (SWE) is
	// conserved automatically, so the visible depth drops to ~accumulation
	// metres without us touching it. Grooming/MogulSize/Ice are the other
	// fields the cat actually rewrites.
	cell.Packed = 1.0
	cell.Grooming = 1.0
	cell.MogulSize *= groomMogulDecay
	cell.Ice *= groomIceDecay

	// Real grooming destroys tracks — zero the R channel inside the
	// cell's pixel footprint. Matches the player expectation that a
	// freshly groomed lane reads as untouched corduroy.
	if w.Terrain.Surface != nil {
		px0 := c[0] * world.PxPerCell
		pz0 := c[1] * world.PxPerCell
		w.Terrain.Surface.ClearTrackBox(image.Rect(
			px0, pz0,
			px0+world.PxPerCell, pz0+world.PxPerCell,
		))
	}

	w.Terrain.SnowDirty = true
}

