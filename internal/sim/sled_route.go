package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Snowmobile routes: a patroller's snowmobile drives a route found on the
// cell grid round what it can't drive through (buildings, tree stands,
// lift towers and stations, open lakes, slopes too steep to climb or
// drop), cut to its corners, and dispatch times a call by it.
//
// The search costs are seconds of driving: snowmobileSpeed on groomed
// snow, slower in ungroomed snow, crawling over bare ground, and slower
// climbing and on steep ground. Crossing a rope or the ski area boundary
// costs a little (patrol ducks ropes when it has to).

const (
	// sledMaxClimb and sledMaxDrop are the steepest grade (rise over run)
	// a snowmobile drives up and down.
	sledMaxClimb = 0.65 // about 33°
	sledMaxDrop  = 0.75 // about 37°
	// sledUngroomed is the speed share in ungroomed snow, and
	// sledClimbSlow how much each unit of climbing grade slows it.
	sledUngroomed = 0.7
	sledClimbSlow = 0.8
	// sledTowerClear is how far a route keeps from a lift tower or
	// station part, metres.
	sledTowerClear = 3.0
	// sledRopeCost is what crossing a rope or the boundary adds, seconds.
	sledRopeCost = 4.0
	// sledWaypointReach is how near a waypoint the snowmobile gets before
	// heading for the next.
	sledWaypointReach = 4.0
)

// sledGrid is one route search's view of the map: which cells a
// snowmobile can't enter.
type sledGrid struct {
	w       *world.World
	t       *world.Terrain
	blocked []bool // per cell, x*Height+z
}

// newSledGrid marks the cells a snowmobile can't enter: buildings and
// thick trees (not Walkable), unfrozen lakes, and cells within
// sledTowerClear of a lift tower or station part.
func newSledGrid(w *world.World, towers []mgl32.Vec2) *sledGrid {
	t := w.Terrain
	g := &sledGrid{w: w, t: t, blocked: make([]bool, t.Width*t.Height)}
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			k := x*t.Height + z
			if !t.Cells[x][z].Walkable() {
				g.blocked[k] = true
				continue
			}
			if t.LakeOf != nil {
				if l := t.LakeOf[k]; l > 0 && int(l) <= len(w.Lakes) && !w.Lakes[l-1].Frozen() {
					g.blocked[k] = true
				}
			}
		}
	}
	r := int(math.Ceil(sledTowerClear/world.CellSize)) + 1
	for _, p := range towers {
		cx, cz := int(p[0]/world.CellSize), int(p[1]/world.CellSize)
		for x := cx - r; x <= cx+r; x++ {
			for z := cz - r; z <= cz+r; z++ {
				if !t.InBounds(x, z) {
					continue
				}
				c := mgl32.Vec2{(float32(x) + 0.5) * world.CellSize, (float32(z) + 0.5) * world.CellSize}
				if c.Sub(p).Len() < sledTowerClear+world.CellSize/2 {
					g.blocked[x*t.Height+z] = true
				}
			}
		}
	}
	return g
}

// open reports whether a snowmobile can be in cell c.
func (g *sledGrid) open(c [2]int) bool {
	return g.t.InBounds(c[0], c[1]) && !g.blocked[c[0]*g.t.Height+c[1]]
}

// speedAt is how fast a snowmobile goes over cell c, m/s, on the level.
func (g *sledGrid) speedAt(c [2]int) float32 {
	cell := &g.t.Cells[c[0]][c[1]]
	if cell.TopLayer() == nil {
		return snowmobileBareSpeed
	}
	return snowmobileSpeed * (sledUngroomed + (1-sledUngroomed)*cell.Grooming)
}

// stepTime is how long the drive from cell a to neighbouring cell b
// takes, seconds, or +Inf when it's too steep.
func (g *sledGrid) stepTime(a, b [2]int) float32 {
	t := g.t
	run := float32(math.Hypot(float64(b[0]-a[0]), float64(b[1]-a[1]))) * world.CellSize
	rise := t.SurfaceElevationAt(b[0], b[1]) - t.SurfaceElevationAt(a[0], a[1])
	grade := rise / run
	if grade > sledMaxClimb || -grade > sledMaxDrop {
		return float32(math.Inf(1))
	}
	speed := (g.speedAt(a) + g.speedAt(b)) / 2
	speed /= 1 + sledClimbSlow*max(grade, 0) + 0.3*max(-grade, 0)
	return run/speed + ropeStepCost(g.w, a, b)/routeRopeCost*sledRopeCost
}

// planSledRoute is the snowmobile's route from a to b, its corners and
// then b, and how long it takes to drive, seconds; ok false when there's
// no way. The start and end cells are allowed even when blocked (a
// garage door, a guest lying in the trees).
func (s *Simulation) planSledRoute(a, b mgl32.Vec2) (pts []mgl32.Vec2, secs float32, ok bool) {
	w := s.World
	t := w.Terrain
	from := [2]int{int(a[0] / world.CellSize), int(a[1] / world.CellSize)}
	to := [2]int{int(b[0] / world.CellSize), int(b[1] / world.CellSize)}
	if !t.InBounds(from[0], from[1]) || !t.InBounds(to[0], to[1]) {
		return nil, 0, false
	}
	if from == to {
		return []mgl32.Vec2{b}, b.Sub(a).Len() / snowmobileSpeed, true
	}
	s.refillTowersScratch()
	g := newSledGrid(w, s.towersScratch)
	open := func(c [2]int) bool { return c == from || c == to || g.open(c) }
	h := t.Height
	idx := func(c [2]int) int32 { return int32(c[0]*h + c[1]) }
	cellOf := func(k int32) [2]int { return [2]int{int(k) / h, int(k) % h} }
	heur := func(c [2]int) float32 {
		return float32(math.Hypot(float64(c[0]-to[0]), float64(c[1]-to[1]))) * world.CellSize / snowmobileSpeed
	}
	sc := &s.sledScratch
	sc.begin(t.Width * h)
	start, goal := idx(from), idx(to)
	sc.setG(start, 0, -1)
	sc.open.push(skiOpenNode{k: start, g: 0, f: heur(from)})
	dirs := [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	found := false
	for len(sc.open) > 0 {
		cur := sc.open.pop()
		if cur.g > sc.gAt(cur.k) {
			continue
		}
		if cur.k == goal {
			found = true
			break
		}
		pos := cellOf(cur.k)
		for _, d := range dirs {
			nb := [2]int{pos[0] + d[0], pos[1] + d[1]}
			if !open(nb) {
				continue
			}
			if d[0] != 0 && d[1] != 0 && (!open([2]int{pos[0] + d[0], pos[1]}) || !open([2]int{pos[0], pos[1] + d[1]})) {
				continue
			}
			step := g.stepTime(pos, nb)
			if math.IsInf(float64(step), 1) {
				continue
			}
			if ng, k := cur.g+step, idx(nb); ng < sc.gAt(k) {
				sc.setG(k, ng, cur.k)
				sc.open.push(skiOpenNode{k: k, g: ng, f: ng + heur(nb)})
			}
		}
	}
	if !found {
		return nil, 0, false
	}
	secs = sc.gAt(goal)
	var cells [][2]int
	for k := goal; k >= 0; k = sc.came[k] {
		cells = append(cells, cellOf(k))
		if k == start {
			break
		}
	}
	for i, j := 0, len(cells)-1; i < j; i, j = i+1, j-1 {
		cells[i], cells[j] = cells[j], cells[i]
	}
	centre := func(c [2]int) mgl32.Vec2 {
		return mgl32.Vec2{(float32(c[0]) + 0.5) * world.CellSize, (float32(c[1]) + 0.5) * world.CellSize}
	}
	// Keep only the corners: from each kept point, the furthest cell the
	// straight drive to which stays open, not too steep, and clear of
	// ropes; the drive starts at a and ends at b.
	at := a
	for i := 0; i < len(cells)-1; {
		j := i + 1
		for j+1 < len(cells) && g.lineClear(at, centre(cells[j+1]), open) {
			j++
		}
		if j == len(cells)-1 {
			break
		}
		at = centre(cells[j])
		pts = append(pts, at)
		i = j
	}
	return append(pts, b), secs, true
}

// lineClear reports whether a snowmobile can drive straight from a to b:
// every cell on the way open, no stretch too steep, and no rope crossed.
func (g *sledGrid) lineClear(a, b mgl32.Vec2, open func([2]int) bool) bool {
	t := g.t
	if !ropeClear(g.w, a, b) {
		return false
	}
	d := b.Sub(a)
	n := int(d.Len()/2.5) + 1
	prevY := t.InterpolatedSurfaceElevationAt(a[0], a[1])
	step := d.Len() / float32(n)
	for i := 1; i <= n; i++ {
		p := a.Add(d.Mul(float32(i) / float32(n)))
		if !open([2]int{int(p[0] / world.CellSize), int(p[1] / world.CellSize)}) {
			return false
		}
		y := t.InterpolatedSurfaceElevationAt(p[0], p[1])
		if grade := (y - prevY) / step; grade > sledMaxClimb || -grade > sledMaxDrop {
			return false
		}
		prevY = y
	}
	return true
}
