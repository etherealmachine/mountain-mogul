package sim

import (
	"container/heap"
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Routes around forest. A guest skiing (or walking) freely to a lift, a
// building, or the car heads straight for it; where that line crosses
// trees, they follow a route around them instead (skiRoute), planned on
// the 5 m cell grid: tree cover costs extra by how much the guest minds
// trees (standCoverScale, so a glade lover still cuts through a glade
// when it's shorter), climbing costs extra since skiers can't go uphill
// well, and buildings block. The route is cut down to the corners where
// the line between waypoints would cross trees, and the guest steers at
// the next one, skipping ahead whenever the way to the one after is
// clear.

const (
	// routeTreeCost is the extra cost of a cell of full tree cover, in
	// cells of open ground, for a guest who minds trees fully.
	routeTreeCost = 20.0
	// routeClimbCost is the extra cost per metre climbed, in cells.
	routeClimbCost = 2.0
	// routeMargin is how many cells past the start and goal's box the
	// search may wander.
	routeMargin = 40
	// routeWaypointReach is how near a waypoint counts as reached, in
	// metres: well outside ArrivalRadius, so waypoints don't slow anyone.
	routeWaypointReach = float32(12)
	// routeRecheckSec is how often a guest off their route plans again.
	routeRecheckSec = 5.0
	// routeGoalMoved is how far the goal must move, in metres, before
	// the route is planned afresh.
	routeGoalMoved = float32(25)
	// routeLookSec is how often a guest checks whether they can cut to a
	// later waypoint.
	routeLookSec = 0.5
	// routeStraySq is how far (squared metres) a guest may be from the
	// waypoint they're steering at before they plan again.
	routeStraySq = float32(45 * 45)
)

// routeTarget is where a guest heading for goal steers this tick: goal
// itself when the way is clear, else the next waypoint of a route around
// the trees.
func (s *Simulation) routeTarget(a *world.Guest, goal mgl32.Vec3) mgl32.Vec3 {
	t := s.World.Terrain
	g := mgl32.Vec2{goal[0], goal[2]}
	pos := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	// A lift's target is the back of its line, which moves as the line
	// grows: keep the route unless the goal has really moved.
	if a.Route.Goal.Sub(g).Len() > routeGoalMoved {
		a.Route = world.SkiRoute{Goal: g}
	}
	r := &a.Route
	stray := len(r.Points) > 0 && r.Index < len(r.Points) && r.Points[r.Index].Sub(pos).LenSqr() > routeStraySq
	if (!r.Checked || stray) && s.SimTime >= r.NextCheck {
		r.Checked, r.NextCheck = true, s.SimTime+routeRecheckSec
		r.Points, r.Index = nil, 0
		scale := standCoverScale(a.Traits.Tastes)
		if !lineClear(t, pos, g, scale) {
			r.Points = planSkiRoute(t, pos, g, scale)
		}
	}
	if len(r.Points) == 0 {
		return goal
	}
	// Skip ahead past reached waypoints, and (twice a second, as it's the
	// costly part) past any the way beyond is clear of.
	look := s.SimTime >= r.NextLook
	if look {
		r.NextLook = s.SimTime + routeLookSec
	}
	scale := standCoverScale(a.Traits.Tastes)
	for r.Index < len(r.Points) {
		if r.Points[r.Index].Sub(pos).Len() < routeWaypointReach {
			r.Index++
			continue
		}
		next := g
		if r.Index+1 < len(r.Points) {
			next = r.Points[r.Index+1]
		}
		if look && lineClear(t, pos, next, scale) {
			r.Index++
			continue
		}
		break
	}
	if r.Index >= len(r.Points) {
		return goal
	}
	p := r.Points[r.Index]
	return mgl32.Vec3{p[0], t.InterpolatedSurfaceElevationAt(p[0], p[1]), p[1]}
}

// lineClear reports whether the straight way from a to b keeps out of
// trees the guest minds (cover × scale under inTreesThreshold) and off
// buildings.
func lineClear(t *world.Terrain, a, b mgl32.Vec2, scale float32) bool {
	d := b.Sub(a)
	n := int(d.Len()/2.5) + 1
	for i := 1; i <= n; i++ {
		p := a.Add(d.Mul(float32(i) / float32(n)))
		cx, cz := int(p[0]/world.CellSize), int(p[1]/world.CellSize)
		if !t.InBounds(cx, cz) {
			continue
		}
		if t.TreeCoverAt(p[0], p[1])*scale > inTreesThreshold {
			return false
		}
		if i < n && !t.Cells[cx][cz].Walkable() {
			return false
		}
	}
	return true
}

// planSkiRoute finds a route from a to b around trees on the cell grid
// and cuts it down to the waypoints where the straight line would cross
// trees; nil when there's none.
func planSkiRoute(t *world.Terrain, a, b mgl32.Vec2, scale float32) []mgl32.Vec2 {
	start := [2]int{int(a[0] / world.CellSize), int(a[1] / world.CellSize)}
	goal := [2]int{int(b[0] / world.CellSize), int(b[1] / world.CellSize)}
	if !t.InBounds(start[0], start[1]) || !t.InBounds(goal[0], goal[1]) || start == goal {
		return nil
	}
	x0 := max(min(start[0], goal[0])-routeMargin, 0)
	z0 := max(min(start[1], goal[1])-routeMargin, 0)
	x1 := min(max(start[0], goal[0])+routeMargin, t.Width-1)
	z1 := min(max(start[1], goal[1])+routeMargin, t.Height-1)
	w, h := x1-x0+1, z1-z0+1
	idx := func(c [2]int) int { return (c[1]-z0)*w + (c[0] - x0) }
	g := make([]float64, w*h)
	came := make([]int32, w*h)
	for i := range g {
		g[i] = math.Inf(1)
		came[i] = -1
	}
	cost := func(c [2]int) float64 {
		cell := &t.Cells[c[0]][c[1]]
		if c != goal && (!cell.Walkable() || !t.IsAccessible(c[0], c[1])) {
			return math.Inf(1)
		}
		return 1 + routeTreeCost*float64(cell.TreeCover()*scale)
	}
	heur := func(c [2]int) float64 {
		return math.Hypot(float64(c[0]-goal[0]), float64(c[1]-goal[1]))
	}
	elev := func(c [2]int) float32 { return t.SurfaceElevationAt(c[0], c[1]) }
	open := &nodeHeap{}
	heap.Push(open, &node{pos: start, g: 0, f: heur(start)})
	g[idx(start)] = 0
	found := false
	dirs := [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	for open.Len() > 0 {
		cur := heap.Pop(open).(*node)
		if cur.g > g[idx(cur.pos)] {
			continue
		}
		if cur.pos == goal {
			found = true
			break
		}
		for _, d := range dirs {
			nb := [2]int{cur.pos[0] + d[0], cur.pos[1] + d[1]}
			if nb[0] < x0 || nb[0] > x1 || nb[1] < z0 || nb[1] > z1 {
				continue
			}
			step := cost(nb)
			if math.IsInf(step, 1) {
				continue
			}
			if d[0] != 0 && d[1] != 0 {
				step *= math.Sqrt2
			}
			step += routeClimbCost * float64(max(elev(nb)-elev(cur.pos), 0)) / world.CellSize
			if ng := cur.g + step; ng < g[idx(nb)] {
				g[idx(nb)] = ng
				came[idx(nb)] = int32(idx(cur.pos))
				heap.Push(open, &node{pos: nb, g: ng, f: ng + heur(nb)})
			}
		}
	}
	if !found {
		return nil
	}
	var cells []mgl32.Vec2
	for k := idx(goal); k >= 0; k = int(came[k]) {
		cells = append(cells, mgl32.Vec2{(float32(k%w+x0) + 0.5) * world.CellSize, (float32(k/w+z0) + 0.5) * world.CellSize})
		if k == idx(start) {
			break
		}
	}
	for i, j := 0, len(cells)-1; i < j; i, j = i+1, j-1 {
		cells[i], cells[j] = cells[j], cells[i]
	}
	// Keep only the corners: from each kept point, the furthest one the
	// line to which stays clear.
	var out []mgl32.Vec2
	from := a
	for i := 0; i < len(cells); {
		j := i
		for j+1 < len(cells) && lineClear(t, from, cells[j+1], scale) {
			j++
		}
		out = append(out, cells[j])
		from = cells[j]
		i = j + 1
	}
	return out
}
