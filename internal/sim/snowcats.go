package sim

import (
	"image"
	"sort"

	"mountain-mogul/internal/world"
)

// Grooming impact a single cat arrival applies to a cell.
const (
	groomMogulDecay = 0.5

	// arriveCellSlack: how close the cat must get to count as arrived.
	arriveCellSlack = world.CellSize * 0.5

	// fullyGroomed: threshold above which a cell is considered done.
	// Slightly under 1.0 so natural decay (moguls, ice) can re-qualify it.
	fullyGroomed = 0.99

	// slicesPerReservation: how many x-column slices a cat claims at once.
	// Each slice is one cell wide (SnowcatTillerWidth) and spans the full
	// trail length in that column. Tunable later.
	slicesPerReservation = 3
)

// sliceKey uniquely identifies one vertical slice within a trail.
type sliceKey struct {
	trailID uint64
	xCol    int
}

// tickSnowcats advances the grooming fleet one step. Each cat either:
//   - follows its reserved zig-zag route (up one column, down the next), or
//   - claims new slices from any groomed trail that has ungroomed cells, or
//   - drives back to its shed door when no work is available.
//
// Multiple cats can work the same trail simultaneously by holding
// non-overlapping column reservations.
func (s *Simulation) tickSnowcats(dt float64) {
	w := s.World

	// Build the global reservation set from cats that already have routes.
	reserved := map[sliceKey]bool{}
	for _, cat := range w.Snowcats {
		if cat.TrailID != 0 {
			for _, x := range cat.SliceXs {
				reserved[sliceKey{cat.TrailID, x}] = true
			}
		}
	}

	for _, cat := range w.Snowcats {
		shed := findBuilding(w, cat.ShedID)
		if shed == nil {
			continue
		}

		// No active route: try to claim slices from any groomed trail.
		if cat.TrailID == 0 {
			reserveSlices(w, cat, reserved)
		}

		// Still no work: drive to shed.
		if cat.TrailID == 0 {
			door := shed.DoorCell()
			tx := (float32(door[0]) + 0.5) * world.CellSize
			tz := (float32(door[1]) + 0.5) * world.CellSize
			cat.DriveToward(tx, tz, dt, arriveCellSlack)
			cat.Pos[1] = w.Terrain.InterpolatedSurfaceElevationAt(cat.Pos[0], cat.Pos[2])
			continue
		}

		trail := findTrail(w, cat.TrailID)
		if trail == nil {
			cat.TrailID = 0
			cat.SliceXs = nil
			continue
		}

		// Advance along the zig-zag. The inner loop only iterates more than
		// once when a slice is exhausted and we step to the next one — it
		// never spins longer than len(SliceXs).
		for {
			if cat.SliceIdx >= len(cat.SliceXs) {
				// All reserved slices done; release route.
				cat.TrailID = 0
				cat.SliceXs = nil
				break
			}

			cells := sliceCellsSorted(w, trail, cat.SliceXs[cat.SliceIdx])

			sliceDone := len(cells) == 0 ||
				(cat.GoingDown && cat.CellIdx >= len(cells)) ||
				(!cat.GoingDown && cat.CellIdx < 0)

			if sliceDone {
				cat.SliceIdx++
				cat.GoingDown = !cat.GoingDown
				if cat.SliceIdx < len(cat.SliceXs) {
					next := sliceCellsSorted(w, trail, cat.SliceXs[cat.SliceIdx])
					if cat.GoingDown {
						cat.CellIdx = 0
					} else {
						cat.CellIdx = len(next) - 1
					}
				}
				continue
			}

			target := cells[cat.CellIdx]
			tx := (float32(target[0]) + 0.5) * world.CellSize
			tz := (float32(target[1]) + 0.5) * world.CellSize
			arrived := cat.DriveToward(tx, tz, dt, arriveCellSlack)
			cat.Pos[1] = w.Terrain.InterpolatedSurfaceElevationAt(cat.Pos[0], cat.Pos[2])
			if arrived {
				groomCell(w, target)
				if cat.GoingDown {
					cat.CellIdx++
				} else {
					cat.CellIdx--
				}
			}
			break
		}
	}
}

// reserveSlices scans groomed trails for available x-column slices and
// assigns up to slicesPerReservation of them to cat. Updates reserved so
// subsequent cats in the same tick don't claim the same columns.
func reserveSlices(w *world.World, cat *world.Snowcat, reserved map[sliceKey]bool) {
	for _, trail := range w.Trails {
		if !trail.Groomed || len(trail.Cells) == 0 {
			continue
		}
		cols := availableColumns(w, trail, reserved)
		if len(cols) == 0 {
			continue
		}
		n := slicesPerReservation
		if n > len(cols) {
			n = len(cols)
		}
		cat.TrailID = trail.ID
		cat.SliceXs = cols[:n]
		cat.SliceIdx = 0
		cat.GoingDown = true
		cat.CellIdx = 0 // top of first slice (GoingDown=true → start at index 0)
		for _, x := range cat.SliceXs {
			reserved[sliceKey{trail.ID, x}] = true
		}
		return
	}
}

// availableColumns returns the x-column indices in trail that (a) have at
// least one cell below the fullyGroomed threshold and (b) are not already
// reserved by another cat. Columns are ordered by ascending average grooming
// so the most degraded strips are worked first.
func availableColumns(w *world.World, trail *world.Trail, reserved map[sliceKey]bool) []int {
	xSet := map[int]bool{}
	for _, c := range trail.Cells {
		xSet[c[0]] = true
	}
	type colScore struct {
		x   int
		avg float32
	}
	var scored []colScore
	for x := range xSet {
		if reserved[sliceKey{trail.ID, x}] {
			continue
		}
		var sum float32
		var n int
		needsWork := false
		for _, c := range trail.Cells {
			if c[0] != x || !w.Terrain.InBounds(c[0], c[1]) {
				continue
			}
			g := w.Terrain.Cells[c[0]][c[1]].Grooming
			sum += g
			n++
			if g < fullyGroomed {
				needsWork = true
			}
		}
		if !needsWork {
			continue
		}
		avg := float32(1)
		if n > 0 {
			avg = sum / float32(n)
		}
		scored = append(scored, colScore{x, avg})
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].avg < scored[j].avg
	})
	out := make([]int, len(scored))
	for i, s := range scored {
		out[i] = s.x
	}
	return out
}

// sliceCellsSorted returns all cells in trail at x-column xCol, ordered
// top-to-bottom (highest terrain elevation first).
func sliceCellsSorted(w *world.World, trail *world.Trail, xCol int) [][2]int {
	var cells [][2]int
	for _, c := range trail.Cells {
		if c[0] == xCol {
			cells = append(cells, c)
		}
	}
	sort.Slice(cells, func(i, j int) bool {
		return w.Terrain.SurfaceElevationAt(cells[i][0], cells[i][1]) >
			w.Terrain.SurfaceElevationAt(cells[j][0], cells[j][1])
	})
	return cells
}

// findTrail returns the trail with the given ID, or nil.
func findTrail(w *world.World, id uint64) *world.Trail {
	for _, t := range w.Trails {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// groomCell applies a single cat pass to cell c.
func groomCell(w *world.World, c [2]int) {
	if !w.Terrain.InBounds(c[0], c[1]) {
		return
	}
	cell := &w.Terrain.Cells[c[0]][c[1]]
	if top := cell.TopLayer(); top != nil {
		top.Kind = world.KindPackedPowder
	}
	cell.Grooming = 1.0
	cell.SkierTraffic = 0
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
