package sim

import (
	"image"
	"math"

	"github.com/go-gl/mathgl/mgl32"

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
// least-groomed cell among the groomed trails nearest its shed, drives
// toward it at SnowcatSpeed, and corduroys the cell on arrival. When no
// qualifying cell exists the cat parks at its shed's door.
func (s *Simulation) tickSnowcats(dt float64) {
	w := s.World

	// Pre-compute groomed trail cells per shed once per tick so every
	// cat owned by the same shed shares the same assignment slice.
	shedCells := make(map[uint64][][2]int)
	for _, b := range w.Buildings {
		if b.Type == world.BuildingShed {
			shedCells[b.ID] = groomedCellsForShed(b.ID, w)
		}
	}

	for _, cat := range w.Snowcats {
		shed := findBuilding(w, cat.ShedID)
		if shed == nil {
			continue
		}
		cells := shedCells[cat.ShedID]

		needPick := cat.TargetCell == world.NoCellTarget ||
			!cellInSlice(cells, cat.TargetCell) ||
			cellGrooming(w, cat.TargetCell) >= fullyGroomed
		if needPick {
			cat.TargetCell = pickNextRouteCell(w, cells, cat.Pos)
		}

		// Compute world-space target. No cell available → park at the
		// shed door instead of stalling in place.
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
		cat.Pos[1] = w.Terrain.InterpolatedSurfaceElevationAt(cat.Pos[0], cat.Pos[2])

		if arrived && grooming {
			groomCell(w, cat.TargetCell)
			cat.TargetCell = world.NoCellTarget
		}
	}
}

// groomedCellsForShed returns all cells from trails marked Groomed whose
// centroid is closer to shed shedID than to any other shed.
func groomedCellsForShed(shedID uint64, w *world.World) [][2]int {
	var sheds []*world.Building
	for _, b := range w.Buildings {
		if b.Type == world.BuildingShed {
			sheds = append(sheds, b)
		}
	}
	if len(sheds) == 0 {
		return nil
	}
	var out [][2]int
	for _, trail := range w.Trails {
		if !trail.Groomed || len(trail.Cells) == 0 {
			continue
		}
		c := trail.Centroid()
		nearest := sheds[0]
		bestDist := shedSqDist(c, nearest.Pos)
		for _, sh := range sheds[1:] {
			if d := shedSqDist(c, sh.Pos); d < bestDist {
				bestDist = d
				nearest = sh
			}
		}
		if nearest.ID == shedID {
			out = append(out, trail.Cells...)
		}
	}
	return out
}

func shedSqDist(a, b mgl32.Vec2) float32 {
	dx, dz := a[0]-b[0], a[1]-b[1]
	return dx*dx + dz*dz
}

// pickNextRouteCell scans cells for the worst-groomed one, breaking ties
// with distance from pos so a cat sweeps a region rather than darting
// randomly. Returns NoCellTarget if no cell qualifies.
func pickNextRouteCell(w *world.World, cells [][2]int, pos [3]float32) [2]int {
	bestCell := world.NoCellTarget
	bestScore := float32(math.MaxFloat32)
	for _, c := range cells {
		if !w.Terrain.InBounds(c[0], c[1]) {
			continue
		}
		g := w.Terrain.Cells[c[0]][c[1]].Grooming
		if g >= fullyGroomed {
			continue
		}
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

// cellInSlice returns true if c appears in cells.
func cellInSlice(cells [][2]int, c [2]int) bool {
	for _, r := range cells {
		if r == c {
			return true
		}
	}
	return false
}

// cellGrooming reads the cell's current Grooming value; returns 1.0 for
// out-of-bounds inputs so a stale target never wedges a cat in place.
func cellGrooming(w *world.World, c [2]int) float32 {
	if !w.Terrain.InBounds(c[0], c[1]) {
		return 1.0
	}
	return w.Terrain.Cells[c[0]][c[1]].Grooming
}

// groomCell applies a single cat-pass to the cell at c.
func groomCell(w *world.World, c [2]int) {
	if !w.Terrain.InBounds(c[0], c[1]) {
		return
	}
	cell := &w.Terrain.Cells[c[0]][c[1]]
	if top := cell.TopLayer(); top != nil {
		top.Packed = 1.0
		top.Ice *= groomIceDecay
	}
	cell.Grooming = 1.0
	cell.MogulSize *= groomMogulDecay

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
