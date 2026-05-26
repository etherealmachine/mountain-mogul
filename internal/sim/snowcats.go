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

// planRoute builds the full cell sequence for cat's next grooming pass.
// Section trail cells form a graph; Prim's MST with straight=0/turn=1 edge
// costs produces a spanning tree; DFS with explicit backtrack cells
// serialises that into the route so every cell is groomed.
func planRoute(w *world.World, cat *world.Snowcat) {
	sectionCells := buildSectionCells(w, cat)
	if len(sectionCells) == 0 {
		return
	}

	catCX := int(cat.Pos[0] / world.CellSize)
	catCZ := int(cat.Pos[2] / world.CellSize)

	comps := sectionComponents(sectionCells)
	comps = orderByNearest(comps, catCX, catCZ)

	var route [][2]int
	curX, curZ := catCX, catCZ
	for _, comp := range comps {
		start := nearestCell(comp, curX, curZ)
		par, parDir := mstPrim(comp, start)
		seg := mstDFS(comp, par, parDir, start)
		route = append(route, seg...)
		if len(seg) > 0 {
			last := seg[len(seg)-1]
			curX, curZ = last[0], last[1]
		}
	}

	cat.Route = route
	cat.RouteIdx = 0
}

// buildSectionCells returns the set of all trail cells assigned to cat's section.
func buildSectionCells(w *world.World, cat *world.Snowcat) map[[2]int]bool {
	cells := map[[2]int]bool{}
	for _, col := range cat.Section {
		trail := findTrail(w, col.TrailID)
		if trail == nil || !trail.Groomed {
			continue
		}
		for _, c := range trail.Cells {
			if c[0] == col.X {
				cells[c] = true
			}
		}
	}
	return cells
}

// nearestCell returns the cell in set with minimum Manhattan distance to (cx, cz).
func nearestCell(cells map[[2]int]bool, cx, cz int) [2]int {
	best, bestD, first := [2]int{}, int(^uint(0)>>1), true
	for c := range cells {
		dx, dz := c[0]-cx, c[1]-cz
		if dx < 0 {
			dx = -dx
		}
		if dz < 0 {
			dz = -dz
		}
		if d := dx + dz; first || d < bestD {
			best, bestD, first = c, d, false
		}
	}
	return best
}

// sectionComponents finds 4-connected components within the cell set.
func sectionComponents(cells map[[2]int]bool) []map[[2]int]bool {
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	visited := map[[2]int]bool{}
	var out []map[[2]int]bool
	for seed := range cells {
		if visited[seed] {
			continue
		}
		comp := map[[2]int]bool{}
		queue := [][2]int{seed}
		for len(queue) > 0 {
			c := queue[0]
			queue = queue[1:]
			if visited[c] {
				continue
			}
			visited[c] = true
			comp[c] = true
			for _, d := range dirs {
				nb := [2]int{c[0] + d[0], c[1] + d[1]}
				if cells[nb] && !visited[nb] {
					queue = append(queue, nb)
				}
			}
		}
		out = append(out, comp)
	}
	return out
}

// orderByNearest reorders components by greedy nearest-neighbour from (startX, startZ).
func orderByNearest(comps []map[[2]int]bool, startX, startZ int) []map[[2]int]bool {
	n := len(comps)
	used := make([]bool, n)
	ordered := make([]map[[2]int]bool, 0, n)
	curX, curZ := startX, startZ
	for len(ordered) < n {
		best, bestD := -1, int(^uint(0)>>1)
		for i, comp := range comps {
			if used[i] {
				continue
			}
			near := nearestCell(comp, curX, curZ)
			dx, dz := near[0]-curX, near[1]-curZ
			if dx < 0 {
				dx = -dx
			}
			if dz < 0 {
				dz = -dz
			}
			if d := dx + dz; best < 0 || d < bestD {
				best, bestD = i, d
			}
		}
		if best < 0 {
			break
		}
		used[best] = true
		ordered = append(ordered, comps[best])
		near := nearestCell(comps[best], curX, curZ)
		curX, curZ = near[0], near[1]
	}
	return ordered
}

// mstPrim builds a spanning tree over cells rooted at start using Prim's
// algorithm. Edge cost is 0 for straight movement (same direction as arrival),
// 1 for a turn. Returns parent and parentDir maps describing the tree.
func mstPrim(cells map[[2]int]bool, start [2]int) (parent, parentDir map[[2]int][2]int) {
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	maxInt := int(^uint(0) >> 1)

	parent = map[[2]int][2]int{}
	parentDir = map[[2]int][2]int{}
	arriveDir := map[[2]int][2]int{}
	key := make(map[[2]int]int, len(cells))
	for c := range cells {
		key[c] = maxInt
	}
	key[start] = 0

	type entry struct {
		cell [2]int
		cost int
	}
	open := []entry{{start, 0}}
	inMST := map[[2]int]bool{}

	for len(open) > 0 {
		bi := 0
		for i := 1; i < len(open); i++ {
			if open[i].cost < open[bi].cost {
				bi = i
			}
		}
		cur := open[bi]
		open[bi] = open[len(open)-1]
		open = open[:len(open)-1]

		if inMST[cur.cell] {
			continue
		}
		inMST[cur.cell] = true
		from := arriveDir[cur.cell]

		for _, d := range dirs {
			nb := [2]int{cur.cell[0] + d[0], cur.cell[1] + d[1]}
			if !cells[nb] || inMST[nb] {
				continue
			}
			cost := 0
			if from != ([2]int{}) && d != from {
				cost = 1
			}
			if cost < key[nb] {
				key[nb] = cost
				parent[nb] = cur.cell
				parentDir[nb] = d
				arriveDir[nb] = d
				open = append(open, entry{nb, cost})
			}
		}
	}
	return parent, parentDir
}

// mstDFS traverses the spanning tree depth-first and returns a flat cell
// sequence. Children are ordered straight-ahead first to minimise turns.
// When the DFS must backtrack to a fork after finishing a branch, it walks
// back through the tree edges so every intermediate cell is included and
// groomed by the cat.
func mstDFS(cells map[[2]int]bool, parent, parentDir map[[2]int][2]int, start [2]int) [][2]int {
	children := map[[2]int][][2]int{}
	for node := range cells {
		if node == start {
			continue
		}
		p := parent[node]
		children[p] = append(children[p], node)
	}

	route := make([][2]int, 0, len(cells))

	var walk func(node [2]int, inDir [2]int)
	walk = func(node [2]int, inDir [2]int) {
		route = append(route, node)
		kids := children[node]
		sort.Slice(kids, func(i, j int) bool {
			// straight-ahead children first
			si := parentDir[kids[i]] == inDir
			sj := parentDir[kids[j]] == inDir
			if si != sj {
				return si
			}
			return false
		})
		for i, kid := range kids {
			walk(kid, parentDir[kid])
			if i < len(kids)-1 {
				// Backtrack from current position to node along tree edges.
				cur := route[len(route)-1]
				for cur != node {
					cur = parent[cur]
					route = append(route, cur)
				}
			}
		}
	}

	walk(start, [2]int{})
	return route
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
