package sim

import (
	"math"
	"sort"

	"mountain-mogul/internal/world"
)

const (
	arriveCellSlack = world.CellSize * 0.5

	// sectionGroomThreshold: a cat heads out for its nightly pass when any
	// snow-covered cell in its section has Grooming below this. Skiers wear
	// down only the lanes they use, so a section average can stay high
	// while the skied line is scraped bare; any cell skied since the last
	// pass trips it, untouched corduroy is left alone.
	sectionGroomThreshold = 0.9
)

// nightIndex identifies the night containing simTime: the evening after
// day d's opening hour through the next morning share index d.
func nightIndex(w *world.World, simTime float64) int {
	return dayIndex(simTime - float64(w.OpenHour)*simSecondsPerHour)
}

// tickSnowcats advances the grooming fleet one step. Standby cats park at
// their shed, as does the whole fleet while the mountain is open. After
// hours, each active cat makes one pass of its section per night if the
// section's grooming has dropped below sectionGroomThreshold.
func (s *Simulation) tickSnowcats(dt float64) {
	w := s.World

	if s.sectionsStale {
		s.reassignAllSections()
		s.sectionsStale = false
	}

	// Cats groom only while the mountain is closed for the day, and head
	// back to the shed before the morning's first guests arrive.
	offShift := !s.ClosedForDay()
	night := nightIndex(w, s.SimTime)
	w.Terrain.Groom.Night = uint16(night + 1)
	if s.catPassNight == nil {
		s.catPassNight = map[uint64]int{}
	}

	for _, cat := range w.Snowcats {
		shed := findBuilding(w, cat.ShedID)
		if shed == nil {
			continue
		}

		if cat.Status == world.CatStandby || offShift {
			cat.Route = nil
			driveToDoor(w, cat, shed, dt)
			continue
		}

		// Active: follow the current route or decide what to do next.
		if len(cat.Route) > 0 {
			s.advanceCat(cat, dt)
			if len(cat.Route) == 0 {
				s.catPassNight[cat.ID] = night
			}
			continue
		}

		if len(cat.Section) == 0 {
			driveToDoor(w, cat, shed, dt)
			continue
		}

		done, ok := s.catPassNight[cat.ID]
		if (!ok || done != night) && sectionNeedsGrooming(w, cat) {
			s.planRoute(cat)
			s.advanceCat(cat, dt)
		} else {
			driveToDoor(w, cat, shed, dt)
		}
	}
}

// advanceCat drives the cat along its route for dt seconds, through as
// many waypoints as its speed covers. Tiller-down steps stamp the groom
// map, wipe skier tracks under the swath, and groom the cells they cross.
func (s *Simulation) advanceCat(cat *world.Snowcat, dt float64) {
	w := s.World
	budget := float32(world.SnowcatSpeed * dt)
	for budget > 0 && cat.RouteIdx < len(cat.Route) {
		step := cat.Route[cat.RouteIdx]
		dx, dz := step.P[0]-cat.Pos[0], step.P[1]-cat.Pos[2]
		dist := float32(math.Sqrt(float64(dx*dx + dz*dz)))
		if dist > 0 {
			cat.Heading = float32(math.Atan2(float64(dx), float64(dz)))
		}
		if dist > budget {
			cat.Pos[0] += dx / dist * budget
			cat.Pos[2] += dz / dist * budget
			break
		}
		cat.Pos[0], cat.Pos[2] = step.P[0], step.P[1]
		budget -= dist
		if step.Groom && cat.RouteIdx > 0 {
			s.groomAlong(cat, cat.Route[cat.RouteIdx-1].P, step)
		}
		cat.RouteIdx++
	}
	cat.Pos[1] = w.Terrain.InterpolatedSurfaceElevationAt(cat.Pos[0], cat.Pos[2])
	if cat.RouteIdx >= len(cat.Route) {
		cat.Route = nil
	}
}

// groomAlong applies one tiller-down step from a to b.
func (s *Simulation) groomAlong(cat *world.Snowcat, a [2]float32, step world.RouteStep) {
	t := s.World.Terrain
	b := step.P
	t.Groom.StampSegment(a[0], a[1], b[0], b[1], step.LatFrom, step.Lat, step.Sign)
	t.Surface.ClearTrackSwath(a[0], a[1], b[0], b[1], world.SnowcatTillerWidth/2)
	t.FlattenMogulSwath(a[0], a[1], b[0], b[1], world.SnowcatTillerWidth/2)
	done := s.catGroomed[cat.ID]
	cx, cz := int(b[0]/world.CellSize), int(b[1]/world.CellSize)
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			c := [2]int{cx + dx, cz + dz}
			if done[c] || !s.groomable[c] {
				continue
			}
			ex := (float32(c[0])+0.5)*world.CellSize - b[0]
			ez := (float32(c[1])+0.5)*world.CellSize - b[1]
			if ex*ex+ez*ez <= passCoverDist*passCoverDist {
				groomCell(s.World, c)
				if done != nil {
					done[c] = true
				}
			}
		}
	}
}

// Turns between neighbouring passes: a pass end this close to the next
// pass start is joined by a looping turn with the tiller down (where it
// stays on the trail), leaving turn marks; farther ones are transits.
const (
	turnMaxGap = float32(15)
	turnReach  = float32(4)
)

// planRoute chains the section's passes into tonight's route: from the
// cat's position, always on to the nearest free pass end, so neighbouring
// lanes are driven back and forth.
func (s *Simulation) planRoute(cat *world.Snowcat) {
	if len(cat.Section) == 0 {
		return
	}
	used := make([]bool, len(cat.Section))
	pos := vec2{cat.Pos[0], cat.Pos[2]}
	var lastDir vec2
	havePrev := false
	var route []world.RouteStep
	for range cat.Section {
		best, rev, bestD := -1, false, float32(math.MaxFloat32)
		for i, ps := range cat.Section {
			if used[i] || len(ps.Pts) == 0 {
				continue
			}
			for _, end := range [2]int{0, len(ps.Pts) - 1} {
				q := ps.Pts[end]
				d := (q[0]-pos[0])*(q[0]-pos[0]) + (q[1]-pos[1])*(q[1]-pos[1])
				if d < bestD {
					best, rev, bestD = i, end != 0, d
				}
			}
		}
		if best < 0 {
			break
		}
		used[best] = true
		pts := append([][2]float32(nil), cat.Section[best].Pts...)
		lats := append([]float32(nil), cat.Section[best].Lats...)
		if rev {
			for l, r := 0, len(pts)-1; l < r; l, r = l+1, r-1 {
				pts[l], pts[r] = pts[r], pts[l]
				lats[l], lats[r] = lats[r], lats[l]
			}
		}
		sign := cat.Section[best].Sign
		if rev {
			sign = -sign
		}
		start := pts[0]
		dir0 := lastDir
		if len(pts) > 1 {
			dir0 = norm2(vec2{pts[1][0] - start[0], pts[1][1] - start[1]})
		}
		if havePrev && bestD <= turnMaxGap*turnMaxGap {
			route = append(route, s.turnSteps(pos, lastDir, start, dir0)...)
		} else {
			route = append(route, world.RouteStep{P: start})
		}
		for k := 1; k < len(pts); k++ {
			route = append(route, world.RouteStep{P: pts[k], Groom: true, LatFrom: lats[k-1], Lat: lats[k], Sign: sign})
		}
		pos = pts[len(pts)-1]
		if len(pts) > 1 {
			prev := pts[len(pts)-2]
			lastDir = norm2(vec2{pos[0] - prev[0], pos[1] - prev[1]})
		} else {
			lastDir = dir0
		}
		havePrev = true
	}
	cat.Route = route
	cat.RouteIdx = 0
	if s.catGroomed == nil {
		s.catGroomed = map[uint64]map[[2]int]bool{}
	}
	s.catGroomed[cat.ID] = map[[2]int]bool{}
}

// turnSteps is a cubic curve from e (heading de) to st (heading ds), in
// steps of about a metre, with the tiller down where it's over a trail.
func (s *Simulation) turnSteps(e, de, st, ds vec2) []world.RouteStep {
	p1 := vec2{e[0] + de[0]*turnReach, e[1] + de[1]*turnReach}
	p2 := vec2{st[0] - ds[0]*turnReach, st[1] - ds[1]*turnReach}
	approx := float32(math.Hypot(float64(st[0]-e[0]), float64(st[1]-e[1]))) + 2*turnReach
	n := max(int(approx/passStep), 2)
	steps := make([]world.RouteStep, 0, n)
	prev := e
	for i := 1; i <= n; i++ {
		u := float32(i) / float32(n)
		a, b, c, d := (1-u)*(1-u)*(1-u), 3*(1-u)*(1-u)*u, 3*(1-u)*u*u, u*u*u
		q := vec2{a*e[0] + b*p1[0] + c*p2[0] + d*st[0], a*e[1] + b*p1[1] + c*p2[1] + d*st[1]}
		mid := [2]int{int((prev[0] + q[0]) / 2 / world.CellSize), int((prev[1] + q[1]) / 2 / world.CellSize)}
		steps = append(steps, world.RouteStep{P: q, Groom: s.groomable[mid], Sign: 1})
		prev = q
	}
	return steps
}

// driveToDoor steers cat toward its shed door cell.
func driveToDoor(w *world.World, cat *world.Snowcat, shed *world.Building, dt float64) {
	home := w.SnowcatParkPos(shed)
	cat.DriveToward(home[0], home[2], dt, arriveCellSlack)
	cat.Pos[1] = w.Terrain.InterpolatedSurfaceElevationAt(cat.Pos[0], cat.Pos[2])
}

// GroomAllNow gives every active cat's section a full pass at once and
// parks the cats again. Used by the debug console and screenshots.
func (s *Simulation) GroomAllNow() {
	s.reassignAllSections()
	s.sectionsStale = false
	s.World.Terrain.Groom.Night++
	for _, cat := range s.World.Snowcats {
		shed := findBuilding(s.World, cat.ShedID)
		if shed == nil || len(cat.Section) == 0 {
			continue
		}
		s.planRoute(cat)
		s.advanceCat(cat, math.MaxFloat32)
		cat.Route = nil
		cat.Pos = s.World.SnowcatParkPos(shed)
	}
}

// reassignAllSections plans passes for every groomed trail and shares
// them out. Each pass goes to the shed with the best score =
// distance² / catCount² from its midpoint, so a shed with N active cats
// has N× the pull radius of a single-cat shed. Within a shed, passes are
// ordered across each trail and split among its cats by length. Standby
// cats receive no section. Called when sectionsStale is set.
func (s *Simulation) reassignAllSections() {
	w := s.World
	for _, cat := range w.Snowcats {
		cat.Section = nil
		cat.SectionCells = nil
		cat.Route = nil
		cat.RouteIdx = 0
	}

	type planned struct {
		pass   world.GroomPass
		mid    vec2
		key    float32 // position across the trail, for ordering lanes
		length float32
	}
	s.groomable = map[[2]int]bool{}
	var all []planned
	for _, trail := range w.Trails {
		if !trail.Groomed || len(trail.Cells) == 0 {
			continue
		}
		for _, c := range trail.Cells {
			s.groomable[c] = true
		}
		passes := planTrailPasses(w.Terrain, trail)
		// The trail's mean lane direction, as a doubled angle.
		var axis vec2
		for _, ps := range passes {
			if n := len(ps.Pts); n > 1 {
				d := norm2(vec2{ps.Pts[n-1][0] - ps.Pts[0][0], ps.Pts[n-1][1] - ps.Pts[0][1]})
				axis[0] += d[0]*d[0] - d[1]*d[1]
				axis[1] += 2 * d[0] * d[1]
			}
		}
		half := math.Atan2(float64(axis[1]), float64(axis[0])) / 2
		across := vec2{-float32(math.Sin(half)), float32(math.Cos(half))}
		for _, ps := range passes {
			mid := ps.Pts[len(ps.Pts)/2]
			all = append(all, planned{pass: ps, mid: mid, key: dot2(mid, across), length: max(ps.Length(), passStep)})
		}
	}

	var activeCats []*world.Snowcat
	for _, cat := range w.Snowcats {
		if cat.Status == world.CatActive {
			activeCats = append(activeCats, cat)
		}
	}
	if len(activeCats) == 0 {
		return
	}

	shedCatCount := map[uint64]int{}
	for _, cat := range activeCats {
		shedCatCount[cat.ShedID]++
	}
	type shedSite struct {
		id     uint64
		wx, wz float32
		nCats  float32
	}
	shedByID := map[uint64]*world.Building{}
	for _, b := range w.Buildings {
		if b.Offers(world.ServiceGarage) {
			shedByID[b.ID] = b
		}
	}
	sites := make([]shedSite, 0, len(shedCatCount))
	for shedID, n := range shedCatCount {
		shed := shedByID[shedID]
		if shed == nil {
			continue
		}
		home := w.SnowcatParkPos(shed)
		sites = append(sites, shedSite{
			id:    shedID,
			wx:    home[0],
			wz:    home[2],
			nCats: float32(n),
		})
	}
	sort.Slice(sites, func(i, j int) bool { return sites[i].id < sites[j].id })

	shedPasses := map[uint64][]planned{}
	for _, p := range all {
		var bestID uint64
		var bestScore float32
		for _, site := range sites {
			dx, dz := p.mid[0]-site.wx, p.mid[1]-site.wz
			score := (dx*dx + dz*dz) / (site.nCats * site.nCats)
			if bestID == 0 || score < bestScore {
				bestID, bestScore = site.id, score
			}
		}
		if bestID != 0 {
			shedPasses[bestID] = append(shedPasses[bestID], p)
		}
	}

	shedActiveCats := map[uint64][]*world.Snowcat{}
	for _, cat := range activeCats {
		shedActiveCats[cat.ShedID] = append(shedActiveCats[cat.ShedID], cat)
	}
	keep := func(c [2]int) bool { return s.groomable[c] }
	for shedID, passes := range shedPasses {
		cats := shedActiveCats[shedID]
		if len(cats) == 0 {
			continue
		}
		sort.SliceStable(passes, func(i, j int) bool {
			if passes[i].pass.TrailID != passes[j].pass.TrailID {
				return passes[i].pass.TrailID < passes[j].pass.TrailID
			}
			return passes[i].key < passes[j].key
		})
		sort.Slice(cats, func(i, j int) bool { return cats[i].ID < cats[j].ID })
		var total float32
		for _, p := range passes {
			total += p.length
		}
		var cum float32
		for _, p := range passes {
			i := min(int((cum+p.length/2)/total*float32(len(cats))), len(cats)-1)
			cats[i].Section = append(cats[i].Section, p.pass)
			cum += p.length
		}
		for _, cat := range cats {
			cat.SectionCells = passCells(w.Terrain, cat.Section, keep)
		}
	}
}

// sectionNeedsGrooming reports whether any snow-covered cell under cat's
// passes has Grooming below sectionGroomThreshold. Bare cells are
// ignored: the cat can't groom them.
func sectionNeedsGrooming(w *world.World, cat *world.Snowcat) bool {
	for _, c := range cat.SectionCells {
		cell := &w.Terrain.Cells[c[0]][c[1]]
		if cell.TopLayer() != nil && cell.Grooming < sectionGroomThreshold {
			return true
		}
	}
	return false
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
	w.Terrain.SnowDirty = true
}
