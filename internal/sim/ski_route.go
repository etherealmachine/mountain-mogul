package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/ai/goap"
	"mountain-mogul/internal/world"
)

// Routes. A guest skiing (or walking) to a lift, a building, the car or
// the next trail heads straight for it when the line suits them; where
// it doesn't, they follow a route of their own instead (skiRoute),
// planned on the 5 m cell grid by what they mind (routeProfile): tree
// cover by how much they mind trees (standCoverScale, so a glade lover
// still cuts through a glade when it's shorter); pitch past their
// comfort along the line, so a nervous skier traverses a steep face or
// goes round it; for guests who keep to trails (all but the advanced
// who'd rather roam), ground off any trail at their level; and climbing,
// since skiers can't go uphill well. Buildings block. The route is cut
// down to the corners where the line between waypoints stops suiting
// them, and the guest steers at the next one, skipping ahead whenever
// the way to the one after suits them.

const (
	// routeTreeCost is the extra cost of a cell of full tree cover, in
	// cells of open ground, for a guest who minds trees fully.
	routeTreeCost = 20.0
	// routeClimbCost is the extra cost per metre climbed, in cells.
	routeClimbCost = 2.0
	// routeHeuristicWeight scales the search's distance estimate past an
	// exact lower bound: the search reaches the goal expanding a fraction
	// of the cells, for routes a little longer than the best (weighted
	// A*). At 1 a busy day's route searches were the sim's biggest cost.
	routeHeuristicWeight = float32(2.5)
	// routeMargin is how many cells past the start and goal's box the
	// search may wander.
	routeMargin = 40
	// routeWaypointReach is how near a waypoint counts as reached, in
	// metres: well outside ArrivalRadius, so waypoints don't slow anyone.
	routeWaypointReach = float32(12)
	// routeRecheckSec is how often a guest off their route plans again.
	routeRecheckSec = 10.0
	// routeGoalMoved is how far the goal must move, in metres, before
	// the route is planned afresh.
	routeGoalMoved = float32(50)
	// routeLookSec is how often a guest checks whether they can cut to a
	// later waypoint.
	routeLookSec = 0.5
	// routeStraySq is how far (squared metres) a guest may be from the
	// waypoint they're steering at before they plan again.
	routeStraySq = float32(45 * 45)
	// routeSteepCost is the extra cost of a cell, in cells, per unit of
	// pitch past the guest's comfort along the step (felt ÷ comfort − 1):
	// a beginner pointing down twice their comfortable pitch pays about
	// twelve cells a cell, so traversing it, or going round, wins.
	routeSteepCost = 12.0
	// routeSteepClear is how far past comfort a straight line may feel
	// before the guest plans a route instead.
	routeSteepClear = float32(1.15)
	// routeOffTrailCost is the extra cost of a cell off any trail the
	// guest keeps to, in cells.
	routeOffTrailCost = 4.0
	// routeTrailSlack is how near either end of a straight line, in
	// metres, it may leave the guest's trails and still suit them: lift
	// lines and doors sit just off the trail.
	routeTrailSlack = float32(20)
)

// routeProfile is what a guest's route weighs besides distance.
type routeProfile struct {
	w       *world.World
	cover   float32                 // standCoverScale
	comfort float32                 // tan of the comfort slope; 0 when pitch doesn't matter (on foot)
	levels  world.TerrainDifficulty // trails they keep to; 0 for anywhere
}

// routeProfileFor is a's routeProfile now.
func (s *Simulation) routeProfileFor(a *world.Guest) routeProfile {
	p := routeProfile{w: s.World, cover: standCoverScale(a.Traits.Tastes)}
	if !a.SkisOn {
		return p
	}
	if a.Traits.ComfortSlope > 0 {
		p.comfort = float32(math.Tan(float64(a.Traits.ComfortSlope)))
	}
	if len(s.World.Trails) > 0 {
		p.levels = goap.TrailLevels(a.Traits.Skill, a.Traits.Tastes)
	}
	return p
}

// feltPitch is the pitch (rise over run) a guest feels going from p to
// q, with elevations ep and eq, ending on cell c: along their line, but
// never less than feltSlopeFloor of the fall line's (feltSlope).
func feltPitch(c *world.Cell, p, q mgl32.Vec2, ep, eq float32) float32 {
	along := abs32(eq-ep) / max(q.Sub(p).Len(), 0.01)
	return max(along, float32(feltSlopeFloor)*c.Slope)
}

// offTrail reports whether cell (cx, cz) is off every trail the profile
// keeps to.
func (p *routeProfile) offTrail(cx, cz int) bool {
	return p.levels != 0 && p.w.TrailDiffsAt(cx, cz)&p.levels == 0
}

// routeTarget is where a guest heading for goal steers this tick: goal
// itself when the way is clear, else the next waypoint of a route around
// the trees.
func (s *Simulation) routeTarget(a *world.Guest, goal mgl32.Vec3, sc *routeScratch) mgl32.Vec3 {
	t := s.World.Terrain
	g := mgl32.Vec2{goal[0], goal[2]}
	pos := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	s.prepareRoute(a, goal, sc)
	r := &a.Route
	if len(r.Points) == 0 {
		return goal
	}
	// Skip ahead past reached waypoints, and (twice a second, as it's the
	// costly part) past any the way beyond is clear of.
	look := s.SimTime >= r.NextLook
	if look {
		r.NextLook = s.SimTime + routeLookSec
	}
	prof := s.routeProfileFor(a)
	for r.Index < len(r.Points) {
		if r.Points[r.Index].Sub(pos).Len() < routeWaypointReach {
			r.Index++
			continue
		}
		next := g
		if r.Index+1 < len(r.Points) {
			next = r.Points[r.Index+1]
		}
		if look && lineClear(t, pos, next, &prof) {
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

// routeDue reports whether a's route to goal is due to be (re)planned:
// the goal has moved, it was never checked, or the guest has strayed from
// it, and the recheck time has come.
func (s *Simulation) routeDue(a *world.Guest, goal mgl32.Vec3) bool {
	g := mgl32.Vec2{goal[0], goal[2]}
	pos := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	r := a.Route
	if r.Goal.Sub(g).Len() > routeGoalMoved {
		r = world.SkiRoute{Goal: g}
	}
	stray := len(r.Points) > 0 && r.Index < len(r.Points) && r.Points[r.Index].Sub(pos).LenSqr() > routeStraySq
	return (!r.Checked || stray) && s.SimTime >= r.NextCheck
}

// prepareRoute (re)plans a's route round the trees to goal when it's due.
// It reads only the terrain and a, and writes only a.Route, so many
// guests' routes can be prepared at once (planRoutes).
func (s *Simulation) prepareRoute(a *world.Guest, goal mgl32.Vec3, sc *routeScratch) {
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
		prof := s.routeProfileFor(a)
		if !lineClear(t, pos, g, &prof) {
			if pts, ok := s.trailRoute(a, pos, g); ok {
				r.Points = pts
			} else {
				r.Points = planSkiRoute(t, pos, g, &prof, sc)
			}
		}
	}
}

// lineClear reports whether the straight way from a to b suits the
// guest: out of trees they mind (cover × scale under inTreesThreshold),
// off buildings, never much steeper along it than they're comfortable
// with, and, away from its ends, on their trails.
func lineClear(t *world.Terrain, a, b mgl32.Vec2, prof *routeProfile) bool {
	d := b.Sub(a)
	length := d.Len()
	n := int(length/2.5) + 1
	prev, prevY := a, t.InterpolatedSurfaceElevationAt(a[0], a[1])
	for i := 1; i <= n; i++ {
		f := float32(i) / float32(n)
		p := a.Add(d.Mul(f))
		cx, cz := int(p[0]/world.CellSize), int(p[1]/world.CellSize)
		if !t.InBounds(cx, cz) {
			continue
		}
		if t.TreeCoverAt(p[0], p[1])*prof.cover > inTreesThreshold {
			return false
		}
		cell := &t.Cells[cx][cz]
		if i < n && !cell.Walkable() {
			return false
		}
		y := t.InterpolatedSurfaceElevationAt(p[0], p[1])
		if prof.comfort > 0 && feltPitch(cell, prev, p, prevY, y) > prof.comfort*routeSteepClear {
			return false
		}
		prev, prevY = p, y
		if along := f * length; along > routeTrailSlack && length-along > routeTrailSlack && prof.offTrail(cx, cz) {
			return false
		}
	}
	return true
}

// planSkiRoute finds the route from a to b on the cell grid that suits
// the guest best (routeProfile) and cuts it down to the waypoints where
// the straight line would stop suiting them; nil when there's none.
func planSkiRoute(t *world.Terrain, a, b mgl32.Vec2, prof *routeProfile, sc *routeScratch) []mgl32.Vec2 {
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
	idx := func(c [2]int) int32 { return int32((c[1]-z0)*w + (c[0] - x0)) }
	posOf := func(k int32) [2]int { return [2]int{int(k)%w + x0, int(k)/w + z0} }
	sc.begin(w * h)
	cost := func(c [2]int) float32 {
		cell := &t.Cells[c[0]][c[1]]
		if c != goal && (!cell.Walkable() || !t.IsAccessible(c[0], c[1])) {
			return float32(math.Inf(1))
		}
		c2 := 1 + routeTreeCost*cell.TreeCover()*prof.cover
		if prof.offTrail(c[0], c[1]) {
			c2 += routeOffTrailCost
		}
		return c2
	}
	centre := func(c [2]int) mgl32.Vec2 {
		return mgl32.Vec2{(float32(c[0]) + 0.5) * world.CellSize, (float32(c[1]) + 0.5) * world.CellSize}
	}
	heur := func(c [2]int) float32 {
		return routeHeuristicWeight * float32(math.Hypot(float64(c[0]-goal[0]), float64(c[1]-goal[1])))
	}
	elev := func(c [2]int) float32 { return t.SurfaceElevationAt(c[0], c[1]) }
	sc.setG(idx(start), 0, -1)
	sc.open.push(skiOpenNode{k: idx(start), g: 0, f: heur(start)})
	found := false
	dirs := [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	for len(sc.open) > 0 {
		cur := sc.open.pop()
		if cur.g > sc.gAt(cur.k) {
			continue
		}
		pos := posOf(cur.k)
		if pos == goal {
			found = true
			break
		}
		curElev, curCentre := elev(pos), centre(pos)
		for _, d := range dirs {
			nb := [2]int{pos[0] + d[0], pos[1] + d[1]}
			if nb[0] < x0 || nb[0] > x1 || nb[1] < z0 || nb[1] > z1 {
				continue
			}
			step := cost(nb)
			if math.IsInf(float64(step), 1) {
				continue
			}
			if d[0] != 0 && d[1] != 0 {
				step *= math.Sqrt2
			}
			nbElev := elev(nb)
			step += routeClimbCost * max(nbElev-curElev, 0) / world.CellSize
			if prof.comfort > 0 {
				felt := feltPitch(&t.Cells[nb[0]][nb[1]], curCentre, centre(nb), curElev, nbElev)
				if over := felt/prof.comfort - 1; over > 0 {
					step += routeSteepCost * over
				}
			}
			if ng, k := cur.g+step, idx(nb); ng < sc.gAt(k) {
				sc.setG(k, ng, cur.k)
				sc.open.push(skiOpenNode{k: k, g: ng, f: ng + heur(nb)})
			}
		}
	}
	if !found {
		return nil
	}
	cells := sc.cells[:0]
	for k := idx(goal); k >= 0; k = sc.came[k] {
		cells = append(cells, centre(posOf(k)))
		if k == idx(start) {
			break
		}
	}
	sc.cells = cells
	for i, j := 0, len(cells)-1; i < j; i, j = i+1, j-1 {
		cells[i], cells[j] = cells[j], cells[i]
	}
	// Keep only the corners: from each kept point, the furthest one the
	// line to which stays clear.
	var out []mgl32.Vec2
	from := a
	for i := 0; i < len(cells); {
		j := i
		for j+1 < len(cells) && lineClear(t, from, cells[j+1], prof) {
			j++
		}
		out = append(out, cells[j])
		from = cells[j]
		i = j + 1
	}
	return out
}

// trailAhead is how far down the trail a guest skiing one aims, in
// metres: near enough that the route there follows the run's shape, far
// enough to pick a line.
const trailAhead = float32(50)

// trailCarrot is where a guest skiing a trail aims: trailAhead down the
// trail's centre line from where they are, toward goal (the step's end),
// rather than at goal itself, which on a winding run can lie across
// other trails. goal once it's nearer than that, and for anyone not
// skiing a run with a centre line.
func (s *Simulation) trailCarrot(a *world.Guest, goal mgl32.Vec3) mgl32.Vec3 {
	if !a.SkisOn || a.Plan.Head().Kind != ai.ActSkiTrail {
		return goal
	}
	line := s.World.TrailLine(plannedTrail(a))
	if line == nil {
		return goal
	}
	here := line.Nearest(mgl32.Vec2{a.Pos[0], a.Pos[2]})
	end := line.Nearest(mgl32.Vec2{goal[0], goal[2]})
	left := line.Along[end] - line.Along[here]
	if abs32(left) <= trailAhead {
		return goal
	}
	d := line.Along[here] + trailAhead
	if left < 0 {
		d = line.Along[here] - trailAhead
	}
	p := line.Samples[line.AtAlong(d)].Pos
	return mgl32.Vec3{p[0], s.World.Terrain.InterpolatedSurfaceElevationAt(p[0], p[1]), p[1]}
}

// trailRouteSpacing is the spacing of a trail route's waypoints, in
// metres, and trailRouteNear how near the centre line a guest must be
// to follow it rather than search.
const (
	trailRouteSpacing = float32(10)
	trailRouteNear    = float32(80)
)

// trailRoute is the way along the centre line of the trail a's plan is
// skiing, from near pos to goal (on the line: trailCarrot's aim), when
// the straight way there isn't clear: the run's own shape, with no
// search. Only guests off a trail, or well off its line, search around
// the trees (planSkiRoute), which was most of the sim's cost in a crowd.
// ok is false when a isn't skiing a run with a centre line or is too far
// from it.
func (s *Simulation) trailRoute(a *world.Guest, pos, goal mgl32.Vec2) ([]mgl32.Vec2, bool) {
	if !a.SkisOn || a.Plan.Head().Kind != ai.ActSkiTrail {
		return nil, false
	}
	line := s.World.TrailLine(plannedTrail(a))
	if line == nil {
		return nil, false
	}
	here, end := line.Nearest(pos), line.Nearest(goal)
	if line.Samples[here].Pos.Sub(pos).Len() > trailRouteNear {
		return nil, false
	}
	dir := 1
	if end < here {
		dir = -1
	}
	var pts []mgl32.Vec2
	last := line.Along[here]
	for i := here + dir; (dir > 0 && i < end) || (dir < 0 && i > end); i += dir {
		if abs32(line.Along[i]-last) >= trailRouteSpacing {
			pts = append(pts, line.Samples[i].Pos)
			last = line.Along[i]
		}
	}
	return append(pts, goal), true
}

// routeScratch is one goroutine's reusable state for planSkiRoute: the
// cost so far and the parent of each cell of the search box, valid where
// its stamp is the search's gen (so nothing is cleared between searches),
// the open list as plain values, and the path's cells. A route search
// used to allocate all of these, tens of kilobytes, ten thousand times a
// busy game hour.
type routeScratch struct {
	g     []float32
	came  []int32
	stamp []uint32
	gen   uint32
	open  skiOpenList
	cells []mgl32.Vec2
}

// begin readies the scratch for a search over n cells.
func (sc *routeScratch) begin(n int) {
	if len(sc.stamp) < n {
		sc.g = make([]float32, n)
		sc.came = make([]int32, n)
		sc.stamp = make([]uint32, n)
		sc.gen = 0
	}
	sc.gen++
	if sc.gen == 0 { // wrapped: old stamps could match again
		clear(sc.stamp)
		sc.gen = 1
	}
	sc.open = sc.open[:0]
}

// gAt is cell k's cost so far, infinite if this search hasn't reached it.
func (sc *routeScratch) gAt(k int32) float32 {
	if sc.stamp[k] != sc.gen {
		return float32(math.Inf(1))
	}
	return sc.g[k]
}

func (sc *routeScratch) setG(k int32, g float32, from int32) {
	sc.stamp[k], sc.g[k], sc.came[k] = sc.gen, g, from
}

// skiOpenNode is a cell on the open list: its index in the search box, its
// cost so far, and that plus the estimate to the goal.
type skiOpenNode struct {
	k    int32
	g, f float32
}

// skiOpenList is a binary min-heap of skiOpenNodes by f.
type skiOpenList []skiOpenNode

func (h *skiOpenList) push(n skiOpenNode) {
	*h = append(*h, n)
	s := *h
	for i := len(s) - 1; i > 0; {
		p := (i - 1) / 2
		if s[p].f <= s[i].f {
			break
		}
		s[p], s[i] = s[i], s[p]
		i = p
	}
}

func (h *skiOpenList) pop() skiOpenNode {
	s := *h
	top := s[0]
	last := len(s) - 1
	s[0] = s[last]
	s = s[:last]
	for i := 0; ; {
		l, r, m := 2*i+1, 2*i+2, i
		if l < len(s) && s[l].f < s[m].f {
			m = l
		}
		if r < len(s) && s[r].f < s[m].f {
			m = r
		}
		if m == i {
			break
		}
		s[i], s[m] = s[m], s[i]
		i = m
	}
	*h = s
	return top
}
