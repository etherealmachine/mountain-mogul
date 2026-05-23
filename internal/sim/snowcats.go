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

// tickSnowcats advances the grooming fleet one step. Standby cats park at
// their shed. Active cats follow their assigned section route, starting a
// new pass whenever the section average drops below the grooming threshold.
func (s *Simulation) tickSnowcats(dt float64) {
	w := s.World

	if s.sectionsStale {
		reassignAllSections(w)
		s.sectionsStale = false
	}


	for _, cat := range w.Snowcats {
		shed := findBuilding(w, cat.ShedID)
		if shed == nil {
			continue
		}

		if cat.Status == world.CatStandby {
			driveToDoor(w, cat, shed, dt)
			continue
		}

		// Active: follow the current route or decide what to do next.
		if len(cat.Route) > 0 {
			advanceCat(w, cat, dt)
			continue
		}

		if len(cat.Section) == 0 {
			driveToDoor(w, cat, shed, dt)
			continue
		}

		if sectionAvgGrooming(w, cat) < sectionGroomThreshold {
			planRoute(w, cat)
			advanceCat(w, cat, dt)
		} else {
			driveToDoor(w, cat, shed, dt)
		}
	}
}

// advanceCat moves the cat one step along its pre-planned route.
// Each cell in Route is already the correct next position — no BFS at
// runtime. The cat grooms every cell it arrives at (column cells and
// BFS connector cells alike).
func advanceCat(w *world.World, cat *world.Snowcat, dt float64) {
	if cat.RouteIdx >= len(cat.Route) {
		cat.Route = nil
		return
	}
	target := cat.Route[cat.RouteIdx]
	tx := (float32(target[0]) + 0.5) * world.CellSize
	tz := (float32(target[1]) + 0.5) * world.CellSize
	arrived := cat.DriveToward(tx, tz, dt, arriveCellSlack)
	cat.Pos[1] = w.Terrain.InterpolatedSurfaceElevationAt(cat.Pos[0], cat.Pos[2])
	if arrived {
		groomCell(w, target)
		cat.RouteIdx++
	}
}

// planRoute builds the full flat cell sequence for cat's next grooming
// pass and stores it in cat.Route. The algorithm:
//  1. Sort the cat's section columns by nearest-neighbour from its
//     current position (minimises initial transit).
//  2. Traverse columns in a serpentine (boustrophedon) pattern —
//     odd columns top-to-bottom, even columns bottom-to-top.
//  3. Between the last cell of one column and the first cell of the
//     next, insert a BFS path through groomed trail cells so the cat
//     never drives off-piste.
//  4. For columns whose cells are non-adjacent in Z (diagonal trails),
//     insert BFS bridges within the column too.
//
// All BFS uses a single walkable set built once for the whole plan.
func planRoute(w *world.World, cat *world.Snowcat) {
	cols := make([]world.CatColumn, len(cat.Section))
	copy(cols, cat.Section)
	sortRouteNearestNeighbor(w, cols, cat.Pos[0], cat.Pos[2])

	walkable := buildWalkable(w)

	var route [][2]int
	goingDown := true
	var prev [2]int
	hasPrev := false

	for _, col := range cols {
		trail := findTrail(w, col.TrailID)
		if trail == nil || !trail.Groomed {
			continue
		}
		cells := sliceCellsSorted(trail, col.X)
		if len(cells) == 0 {
			continue
		}

		// Serpentine: alternate direction per column.
		colCells := make([][2]int, len(cells))
		copy(colCells, cells)
		if !goingDown {
			for i, j := 0, len(colCells)-1; i < j; i, j = i+1, j-1 {
				colCells[i], colCells[j] = colCells[j], colCells[i]
			}
		}
		goingDown = !goingDown

		entry := colCells[0]

		// Connect from previous position to this column's entry via BFS.
		from := prev
		if !hasPrev {
			from = [2]int{int(cat.Pos[0] / world.CellSize), int(cat.Pos[2] / world.CellSize)}
		}
		if from != entry {
			if path := bfsPath(walkable, from, entry); len(path) > 1 {
				route = append(route, path[1:]...) // omit 'from' (already in route)
			}
			// BFS failure: cat will drive straight to entry — acceptable
			// for the initial approach from the shed.
		}

		// Append column cells, inserting BFS bridges across any gaps.
		for i, cell := range colCells {
			if i > 0 {
				p := colCells[i-1]
				dz := cell[1] - p[1]
				if dz < 0 {
					dz = -dz
				}
				if dz > 1 {
					if path := bfsPath(walkable, p, cell); len(path) > 1 {
						route = append(route, path[1:]...)
						continue
					}
				}
			}
			route = append(route, cell)
		}

		prev = colCells[len(colCells)-1]
		hasPrev = true
	}

	cat.Route = route
	cat.RouteIdx = 0
}

// buildWalkable returns the set of all cells belonging to any groomed trail.
func buildWalkable(w *world.World) map[[2]int]bool {
	walkable := map[[2]int]bool{}
	for _, trail := range w.Trails {
		if !trail.Groomed {
			continue
		}
		for _, c := range trail.Cells {
			walkable[c] = true
		}
	}
	return walkable
}

// bfsPath returns a path through walkable cells from `from` to `to` using
// 4-connected BFS. Returns nil if either endpoint is not walkable or no
// path exists.
func bfsPath(walkable map[[2]int]bool, from, to [2]int) [][2]int {
	if !walkable[from] || !walkable[to] {
		return nil
	}
	if from == to {
		return [][2]int{from}
	}
	parent := map[[2]int][2]int{}
	visited := map[[2]int]bool{from: true}
	queue := [][2]int{from}
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	found := false
outer:
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range dirs {
			next := [2]int{cur[0] + d[0], cur[1] + d[1]}
			if visited[next] || !walkable[next] {
				continue
			}
			visited[next] = true
			parent[next] = cur
			if next == to {
				found = true
				break outer
			}
			queue = append(queue, next)
		}
	}
	if !found {
		return nil
	}
	var path [][2]int
	for cur := to; cur != from; cur = parent[cur] {
		path = append(path, cur)
	}
	path = append(path, from)
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// driveToDoor steers cat toward its shed door cell.
func driveToDoor(w *world.World, cat *world.Snowcat, shed *world.Building, dt float64) {
	door := shed.DoorCell()
	tx := (float32(door[0]) + 0.5) * world.CellSize
	tz := (float32(door[1]) + 0.5) * world.CellSize
	cat.DriveToward(tx, tz, dt, arriveCellSlack)
	cat.Pos[1] = w.Terrain.InterpolatedSurfaceElevationAt(cat.Pos[0], cat.Pos[2])
}

// reassignAllSections performs a capacity-weighted Voronoi partition of groomed
// trail columns across all active cats. Each (trail, x-column) pair is assigned
// to the shed with the best score = distance² / catCount², so a shed with N
// active cats has N× the effective pull radius of a single-cat shed. Within
// each shed the assigned columns are divided evenly among the active cats.
// Standby cats receive no section. Called when sectionsStale is set.
func reassignAllSections(w *world.World) {
	// Clear all existing assignments and routes.
	for _, cat := range w.Snowcats {
		cat.Section = nil
		cat.Route = nil
		cat.RouteIdx = 0
	}

	// Collect active cats and the set of sheds that have them.
	var activeCats []*world.Snowcat
	for _, cat := range w.Snowcats {
		if cat.Status == world.CatActive {
			activeCats = append(activeCats, cat)
		}
	}
	if len(activeCats) == 0 {
		return
	}

	// Count active cats per shed for capacity weighting.
	shedCatCount := map[uint64]int{}
	for _, cat := range activeCats {
		shedCatCount[cat.ShedID]++
	}

	// Build shed door world-positions for sheds with active cats.
	type shedSite struct {
		id    uint64
		wx    float32
		wz    float32
		nCats float32 // active cat count, precast to float for scoring
	}
	shedByID := map[uint64]*world.Building{}
	for _, b := range w.Buildings {
		if b.Type == world.BuildingShed {
			shedByID[b.ID] = b
		}
	}
	sites := make([]shedSite, 0, len(shedCatCount))
	for shedID, n := range shedCatCount {
		shed := shedByID[shedID]
		if shed == nil {
			continue
		}
		door := shed.DoorCell()
		sites = append(sites, shedSite{
			id:    shedID,
			wx:    (float32(door[0]) + 0.5) * world.CellSize,
			wz:    (float32(door[1]) + 0.5) * world.CellSize,
			nCats: float32(n),
		})
	}
	// Sort sites by ID for deterministic tie-breaking.
	sort.Slice(sites, func(i, j int) bool { return sites[i].id < sites[j].id })

	// Precompute centroid Z per trail (all columns share the same trail centroid Z
	// for the proximity metric; column X is used directly for the X distance).
	trailCentZ := map[uint64]float32{}
	for _, trail := range w.Trails {
		if !trail.Groomed || len(trail.Cells) == 0 {
			continue
		}
		var sumZ float32
		for _, c := range trail.Cells {
			sumZ += (float32(c[1]) + 0.5) * world.CellSize
		}
		trailCentZ[trail.ID] = sumZ / float32(len(trail.Cells))
	}

	// Voronoi: assign each (trail, xCol) to the nearest shed site.
	shedCols := map[uint64][]world.CatColumn{}
	for _, trail := range w.Trails {
		if !trail.Groomed || len(trail.Cells) == 0 {
			continue
		}
		centZ := trailCentZ[trail.ID]
		xSet := map[int]bool{}
		for _, c := range trail.Cells {
			xSet[c[0]] = true
		}
		for x := range xSet {
			colWX := (float32(x) + 0.5) * world.CellSize
			var bestID uint64
			var bestScore float32
			for _, site := range sites {
				dx := colWX - site.wx
				dz := centZ - site.wz
				// score = d² / n²: a shed with N cats wins columns up to N×
				// farther away than a single-cat shed at the same distance.
				score := (dx*dx + dz*dz) / (site.nCats * site.nCats)
				if bestID == 0 || score < bestScore {
					bestID = site.id
					bestScore = score
				}
			}
			if bestID != 0 {
				shedCols[bestID] = append(shedCols[bestID], world.CatColumn{TrailID: trail.ID, X: x})
			}
		}
	}

	// Within each shed sort columns then divide evenly among active cats.
	shedActiveCats := map[uint64][]*world.Snowcat{}
	for _, cat := range activeCats {
		shedActiveCats[cat.ShedID] = append(shedActiveCats[cat.ShedID], cat)
	}

	for shedID, cols := range shedCols {
		cats := shedActiveCats[shedID]
		if len(cats) == 0 {
			continue
		}
		sort.Slice(cols, func(i, j int) bool {
			if cols[i].TrailID != cols[j].TrailID {
				return cols[i].TrailID < cols[j].TrailID
			}
			return cols[i].X < cols[j].X
		})
		sort.Slice(cats, func(i, j int) bool { return cats[i].ID < cats[j].ID })

		n := len(cats)
		for i, cat := range cats {
			lo := i * len(cols) / n
			hi := (i + 1) * len(cols) / n
			if lo >= hi {
				hi = lo + 1
			}
			if hi > len(cols) {
				hi = len(cols)
			}
			if lo >= len(cols) {
				continue // more cats than columns; this cat sits idle
			}
			section := make([]world.CatColumn, hi-lo)
			copy(section, cols[lo:hi])
			cat.Section = section
		}
	}

}

// sortRouteNearestNeighbor reorders cols in-place using a greedy
// nearest-neighbor heuristic starting from (startWX, startWZ). Each column's
// representative position is its world X centre and the Z centroid of its
// trail's cells. This minimises transit distance between columns when the cat
// begins a new grooming pass.
func sortRouteNearestNeighbor(w *world.World, cols []world.CatColumn, startWX, startWZ float32) {
	n := len(cols)
	if n <= 1 {
		return
	}

	// Precompute trail centroid Z for each unique trail in the route.
	centZ := make(map[uint64]float32, n)
	for _, col := range cols {
		if _, ok := centZ[col.TrailID]; ok {
			continue
		}
		trail := findTrail(w, col.TrailID)
		if trail == nil || len(trail.Cells) == 0 {
			continue
		}
		var sumZ float32
		for _, c := range trail.Cells {
			sumZ += (float32(c[1]) + 0.5) * world.CellSize
		}
		centZ[col.TrailID] = sumZ / float32(len(trail.Cells))
	}

	visited := make([]bool, n)
	result := make([]world.CatColumn, 0, n)
	curX, curZ := startWX, startWZ

	for len(result) < n {
		best, bestD2 := -1, float32(0)
		for i, col := range cols {
			if visited[i] {
				continue
			}
			cx := (float32(col.X) + 0.5) * world.CellSize
			cz := centZ[col.TrailID]
			dx, dz := cx-curX, cz-curZ
			d2 := dx*dx + dz*dz
			if best < 0 || d2 < bestD2 {
				best, bestD2 = i, d2
			}
		}
		if best < 0 {
			break
		}
		visited[best] = true
		result = append(result, cols[best])
		curX = (float32(cols[best].X) + 0.5) * world.CellSize
		curZ = centZ[cols[best].TrailID]
	}
	copy(cols, result)
}

// sectionAvgGrooming returns the average Grooming value across all cells
// in cat's assigned section. Returns 1.0 if the section is empty.
func sectionAvgGrooming(w *world.World, cat *world.Snowcat) float32 {
	var sum float32
	var n int
	for _, col := range cat.Section {
		trail := findTrail(w, col.TrailID)
		if trail == nil {
			continue
		}
		for _, c := range trail.Cells {
			if c[0] != col.X || !w.Terrain.InBounds(c[0], c[1]) {
				continue
			}
			sum += w.Terrain.Cells[c[0]][c[1]].Grooming
			n++
		}
	}
	if n == 0 {
		return 1.0
	}
	return sum / float32(n)
}

// sliceCellsSorted returns all cells in trail at x-column xCol, ordered
// top-to-bottom (highest terrain elevation first).
func sliceCellsSorted(trail *world.Trail, xCol int) [][2]int {
	var cells [][2]int
	for _, c := range trail.Cells {
		if c[0] == xCol {
			cells = append(cells, c)
		}
	}
	sort.Slice(cells, func(i, j int) bool {
		return cells[i][1] < cells[j][1]
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
