package sim

import (
	"image"
	"sort"

	"mountain-mogul/internal/world"
)

const (
	groomMogulDecay = 0.5

	arriveCellSlack = world.CellSize * 0.5

	// sectionGroomThreshold is the average grooming level below which a cat
	// will head out to re-groom its section. 0.5 means "half the corduroy
	// has faded" — the section needs another pass.
	sectionGroomThreshold = 0.5
)

// tickSnowcats advances the grooming fleet one step. Each cat either:
//   - follows its active zig-zag route through its assigned section, or
//   - starts a new pass when section average grooming drops below 50%, or
//   - drives back to its shed and waits.
func (s *Simulation) tickSnowcats(dt float64) {
	w := s.World

	// Assign sections to sheds whose cats don't have one yet.
	assignMissingSections(w)

	for _, cat := range w.Snowcats {
		shed := findBuilding(w, cat.ShedID)
		if shed == nil {
			continue
		}

		// If the assigned trail is gone or no longer groomed, clear the section
		// so it gets reassigned next tick.
		if cat.SectionTrailID != 0 {
			trail := findTrail(w, cat.SectionTrailID)
			if trail == nil || !trail.Groomed {
				cat.SectionTrailID = 0
				cat.SectionXs = nil
				cat.TrailID = 0
				cat.SliceXs = nil
			}
		}

		// No section yet — wait by the shed.
		if cat.SectionTrailID == 0 {
			driveToDoor(w, cat, shed, dt)
			continue
		}

		// Actively routing: advance one step along the zig-zag.
		if cat.TrailID != 0 {
			advanceCat(w, cat, shed, dt)
			continue
		}

		// No active route. Start a new pass if section needs work.
		if sectionAvgGrooming(w, cat) < sectionGroomThreshold {
			cat.TrailID = cat.SectionTrailID
			cat.SliceXs = cat.SectionXs
			cat.SliceIdx = 0
			cat.GoingDown = true
			cat.CellIdx = 0
			advanceCat(w, cat, shed, dt)
		} else {
			driveToDoor(w, cat, shed, dt)
		}
	}
}

// advanceCat moves cat one step along its current zig-zag route. Clears
// the route when all slices are done.
func advanceCat(w *world.World, cat *world.Snowcat, shed *world.Building, dt float64) {
	trail := findTrail(w, cat.TrailID)
	if trail == nil {
		cat.TrailID = 0
		cat.SliceXs = nil
		return
	}

	for {
		if cat.SliceIdx >= len(cat.SliceXs) {
			cat.TrailID = 0
			cat.SliceXs = nil
			return
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
		return
	}
}

// driveToDoor steers cat toward its shed door cell.
func driveToDoor(w *world.World, cat *world.Snowcat, shed *world.Building, dt float64) {
	door := shed.DoorCell()
	tx := (float32(door[0]) + 0.5) * world.CellSize
	tz := (float32(door[1]) + 0.5) * world.CellSize
	cat.DriveToward(tx, tz, dt, arriveCellSlack)
	cat.Pos[1] = w.Terrain.InterpolatedSurfaceElevationAt(cat.Pos[0], cat.Pos[2])
}

// assignMissingSections assigns permanent section columns to every cat in
// any shed where at least one cat has no section. All cats in a shed are
// assigned atomically so sections are non-overlapping by construction.
func assignMissingSections(w *world.World) {
	// Group cats by shed.
	shedCats := map[uint64][]*world.Snowcat{}
	for _, cat := range w.Snowcats {
		shedCats[cat.ShedID] = append(shedCats[cat.ShedID], cat)
	}

	for shedID, cats := range shedCats {
		needsAssign := false
		for _, cat := range cats {
			if cat.SectionXs == nil {
				needsAssign = true
				break
			}
		}
		if !needsAssign {
			continue
		}

		shed := findBuilding(w, shedID)
		if shed == nil {
			continue
		}
		trail := nearestGroomedTrail(w, shed)
		if trail == nil {
			continue
		}

		// Collect unique x-columns sorted ascending.
		xSet := map[int]bool{}
		for _, c := range trail.Cells {
			xSet[c[0]] = true
		}
		allCols := make([]int, 0, len(xSet))
		for x := range xSet {
			allCols = append(allCols, x)
		}
		sort.Ints(allCols)

		if len(allCols) == 0 {
			continue
		}

		// Sort cats by ID for stable, deterministic assignment.
		sort.Slice(cats, func(i, j int) bool { return cats[i].ID < cats[j].ID })
		n := len(cats)
		for i, cat := range cats {
			lo := i * len(allCols) / n
			hi := (i + 1) * len(allCols) / n
			if lo >= hi {
				// Degenerate: more cats than columns — give this cat the last column.
				hi = lo + 1
				if hi > len(allCols) {
					hi = len(allCols)
					lo = hi - 1
				}
			}
			section := make([]int, hi-lo)
			copy(section, allCols[lo:hi])
			cat.SectionTrailID = trail.ID
			cat.SectionXs = section
		}
	}
}

// nearestGroomedTrail returns the groomed trail whose cell centroid is
// closest to the shed's door cell. Returns nil if no groomed trail exists.
func nearestGroomedTrail(w *world.World, shed *world.Building) *world.Trail {
	door := shed.DoorCell()
	doorX := (float32(door[0]) + 0.5) * world.CellSize
	doorZ := (float32(door[1]) + 0.5) * world.CellSize

	var best *world.Trail
	var bestDist2 float32
	for _, trail := range w.Trails {
		if !trail.Groomed || len(trail.Cells) == 0 {
			continue
		}
		var sumX, sumZ float32
		for _, c := range trail.Cells {
			sumX += (float32(c[0]) + 0.5) * world.CellSize
			sumZ += (float32(c[1]) + 0.5) * world.CellSize
		}
		n := float32(len(trail.Cells))
		cx, cz := sumX/n, sumZ/n
		dx, dz := cx-doorX, cz-doorZ
		d2 := dx*dx + dz*dz
		if best == nil || d2 < bestDist2 {
			best = trail
			bestDist2 = d2
		}
	}
	return best
}

// sectionAvgGrooming returns the average Grooming value across all cells
// in cat's assigned section. Returns 1.0 if the section is empty.
func sectionAvgGrooming(w *world.World, cat *world.Snowcat) float32 {
	trail := findTrail(w, cat.SectionTrailID)
	if trail == nil {
		return 1.0
	}
	sectionSet := make(map[int]bool, len(cat.SectionXs))
	for _, x := range cat.SectionXs {
		sectionSet[x] = true
	}
	var sum float32
	var n int
	for _, c := range trail.Cells {
		if !sectionSet[c[0]] || !w.Terrain.InBounds(c[0], c[1]) {
			continue
		}
		sum += w.Terrain.Cells[c[0]][c[1]].Grooming
		n++
	}
	if n == 0 {
		return 1.0
	}
	return sum / float32(n)
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
