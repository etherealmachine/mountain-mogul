package sim

import (
	"math"
	"sort"

	"mountain-mogul/internal/world"
)

// Pass planning lays grooming lanes over a trail the way a cat crew would:
// side-by-side passes down the fall line, or along the trail on narrow
// runs that cross the slope (cat tracks). Lanes are evenly spaced
// streamlines of a per-cell direction field: each new pass is seeded one
// lane over from an existing one and stops where it closes on another,
// so lanes follow curves and converge in gullies without piling up.
const (
	passStep      = float32(1.0)                     // metres between pass points
	passTestDist  = world.SnowcatLaneSpacing * 0.5   // a pass stops this close to another
	passCoverDist = world.SnowcatTillerWidth/2 + 0.1 // a cell centre this close to a pass is groomed by it
	passMinLen    = 6                                // points; shorter lane seeds are dropped
	passMaxPts    = 6000
	extentMax     = float32(80) // metres probed each way when sizing a trail's width
	laneDirs      = 16          // candidate lane directions over a half turn
	fallBias      = float32(0.5)
	latReach      = world.SnowcatLaneSpacing * 1.5 // a lane takes its lateral coordinate from one this close
)

type vec2 = [2]float32

func norm2(v vec2) vec2 {
	l := float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1])))
	if l == 0 {
		return vec2{0, 1}
	}
	return vec2{v[0] / l, v[1] / l}
}

func dot2(a, b vec2) float32 { return a[0]*b[0] + a[1]*b[1] }

// passPt is an accepted pass point with its lateral coordinate and the
// direction that coordinate grows in.
type passPt struct {
	p, up vec2
	lat   float32
}

// passPlanner holds one trail's cell mask and direction field.
type passPlanner struct {
	t         *world.Terrain
	x0, z0    int // mask origin in cells
	w, h      int
	in        []bool
	field     []vec2 // doubled-angle direction per mask cell
	cells     [][2]int
	pointGrid map[[2]int][]passPt // accepted pass points, bucketed by 2 m
	passes    []world.GroomPass
	trailID   uint64
}

func newPassPlanner(t *world.Terrain, trail *world.Trail) *passPlanner {
	p := &passPlanner{t: t, trailID: trail.ID, pointGrid: map[[2]int][]passPt{}}
	first := true
	var x1, z1 int
	for _, c := range trail.Cells {
		if !t.InBounds(c[0], c[1]) || t.Cells[c[0]][c[1]].TopLayer() == nil {
			continue
		}
		p.cells = append(p.cells, c)
		if first {
			p.x0, p.z0, x1, z1 = c[0], c[1], c[0], c[1]
			first = false
			continue
		}
		p.x0, p.z0 = min(p.x0, c[0]), min(p.z0, c[1])
		x1, z1 = max(x1, c[0]), max(z1, c[1])
	}
	if first {
		return p
	}
	p.w, p.h = x1-p.x0+1, z1-p.z0+1
	p.in = make([]bool, p.w*p.h)
	for _, c := range p.cells {
		p.in[p.idx(c[0], c[1])] = true
	}
	p.buildField()
	return p
}

func (p *passPlanner) idx(cx, cz int) int { return (cx-p.x0)*p.h + (cz - p.z0) }

func (p *passPlanner) inCell(cx, cz int) bool {
	if cx < p.x0 || cz < p.z0 || cx >= p.x0+p.w || cz >= p.z0+p.h {
		return false
	}
	return p.in[p.idx(cx, cz)]
}

func (p *passPlanner) inside(pt vec2) bool {
	return p.inCell(int(math.Floor(float64(pt[0]/world.CellSize))), int(math.Floor(float64(pt[1]/world.CellSize))))
}

// extent is how far the trail runs from pt along ±dir, in metres.
func (p *passPlanner) extent(pt, dir vec2) float32 {
	var total float32
	for _, s := range [2]float32{1, -1} {
		for d := passStep; d <= extentMax; d += passStep {
			if !p.inside(vec2{pt[0] + dir[0]*d*s, pt[1] + dir[1]*d*s}) {
				break
			}
			total += passStep
		}
	}
	return total
}

// buildField picks each cell's lane direction, then smooths it so lanes
// bend gently where a trail turns from the fall line onto a cat track.
func (p *passPlanner) buildField() {
	raw := make([]vec2, p.w*p.h)
	for _, c := range p.cells {
		// Fall line from the 3×3 mean gradient, to ride over cell noise.
		var gx, gz float32
		for dx := -1; dx <= 1; dx++ {
			for dz := -1; dz <= 1; dz++ {
				x, z := min(max(c[0]+dx, 0), p.t.Width-1), min(max(c[1]+dz, 0), p.t.Height-1)
				a, b := p.t.GradientAt(x, z)
				gx, gz = gx+a, gz+b
			}
		}
		fall := norm2(vec2{-gx, -gz})
		centre := vec2{(float32(c[0]) + 0.5) * world.CellSize, (float32(c[1]) + 0.5) * world.CellSize}
		// Lanes run along the trail's longest reach through the cell,
		// favouring the fall line: open slopes reach the probe limit
		// every way and get fall-line lanes; a cat track reaches far
		// only along itself; a bend reaches farthest diagonally.
		dir, best := fall, float32(-1)
		for k := 0; k < laneDirs; k++ {
			a := float64(k) * math.Pi / laneDirs
			d := vec2{float32(math.Cos(a)), float32(math.Sin(a))}
			score := p.extent(centre, d) * (1 + fallBias*float32(math.Abs(float64(dot2(d, fall)))))
			if score > best {
				dir, best = d, score
			}
		}
		raw[p.idx(c[0], c[1])] = vec2{dir[0]*dir[0] - dir[1]*dir[1], 2 * dir[0] * dir[1]}
	}
	p.field = raw
	for iter := 0; iter < 4; iter++ {
		next := make([]vec2, len(raw))
		for _, c := range p.cells {
			var sum vec2
			for dx := -1; dx <= 1; dx++ {
				for dz := -1; dz <= 1; dz++ {
					if p.inCell(c[0]+dx, c[1]+dz) {
						v := p.field[p.idx(c[0]+dx, c[1]+dz)]
						sum[0], sum[1] = sum[0]+v[0], sum[1]+v[1]
					}
				}
			}
			next[p.idx(c[0], c[1])] = sum
		}
		p.field = next
	}
}

// dirAt is the lane direction at pt, flipped to agree with prev.
func (p *passPlanner) dirAt(pt, prev vec2) vec2 {
	fx, fz := pt[0]/world.CellSize-0.5, pt[1]/world.CellSize-0.5
	cx, cz := int(math.Floor(float64(fx))), int(math.Floor(float64(fz)))
	tx, tz := fx-float32(cx), fz-float32(cz)
	var sum vec2
	for _, o := range [4][3]float32{{0, 0, (1 - tx) * (1 - tz)}, {1, 0, tx * (1 - tz)}, {0, 1, (1 - tx) * tz}, {1, 1, tx * tz}} {
		x, z := cx+int(o[0]), cz+int(o[1])
		if p.inCell(x, z) {
			v := p.field[p.idx(x, z)]
			sum[0], sum[1] = sum[0]+v[0]*o[2], sum[1]+v[1]*o[2]
		}
	}
	if sum[0] == 0 && sum[1] == 0 {
		return prev
	}
	half := math.Atan2(float64(sum[1]), float64(sum[0])) / 2
	d := vec2{float32(math.Cos(half)), float32(math.Sin(half))}
	if dot2(d, prev) < 0 {
		d = vec2{-d[0], -d[1]}
	}
	return d
}

func bucketOf(pt vec2) [2]int {
	return [2]int{int(math.Floor(float64(pt[0] / 2))), int(math.Floor(float64(pt[1] / 2)))}
}

// near reports whether an accepted pass point lies within r of pt.
func (p *passPlanner) near(pt vec2, r float32) bool {
	b := bucketOf(pt)
	reach := int(r/2) + 1
	for dx := -reach; dx <= reach; dx++ {
		for dz := -reach; dz <= reach; dz++ {
			for _, q := range p.pointGrid[[2]int{b[0] + dx, b[1] + dz}] {
				ex, ez := q.p[0]-pt[0], q.p[1]-pt[1]
				if ex*ex+ez*ez < r*r {
					return true
				}
			}
		}
	}
	return false
}

// nearest is the closest accepted pass point within r of pt.
func (p *passPlanner) nearest(pt vec2, r float32) (passPt, bool) {
	b := bucketOf(pt)
	reach := int(r/2) + 1
	var best passPt
	bestD, found := r*r, false
	for dx := -reach; dx <= reach; dx++ {
		for dz := -reach; dz <= reach; dz++ {
			for _, q := range p.pointGrid[[2]int{b[0] + dx, b[1] + dz}] {
				ex, ez := q.p[0]-pt[0], q.p[1]-pt[1]
				if d := ex*ex + ez*ez; d < bestD {
					best, bestD, found = q, d, true
				}
			}
		}
	}
	return best, found
}

// traceHalf follows the field from start along heading until the pass
// leaves the trail, closes on another pass, or doubles back.
func (p *passPlanner) traceHalf(start, heading vec2) []vec2 {
	var pts []vec2
	pt, prev := start, heading
	for len(pts) < passMaxPts/2 {
		d := p.dirAt(pt, prev)
		if dot2(d, prev) < 0.7 {
			break
		}
		next := vec2{pt[0] + d[0]*passStep, pt[1] + d[1]*passStep}
		if !p.inside(next) || p.near(next, passTestDist) {
			break
		}
		pts = append(pts, next)
		pt, prev = next, d
	}
	return pts
}

// trace runs a lane through seed both ways, returning its points and the
// seed's index among them.
func (p *passPlanner) trace(seed, heading vec2) ([]vec2, int) {
	h := p.dirAt(seed, heading)
	fwd := p.traceHalf(seed, h)
	back := p.traceHalf(seed, vec2{-h[0], -h[1]})
	pts := make([]vec2, 0, len(back)+1+len(fwd))
	for i := len(back) - 1; i >= 0; i-- {
		pts = append(pts, back[i])
	}
	pts = append(pts, seed)
	return append(pts, fwd...), len(back)
}

// accept adds a lane. Its lateral coordinate continues the nearest
// lane's at every point it can, so the two agree where their swaths meet
// and corduroy carries across the seam; past any neighbour it holds the
// last value. A first lane starts at 0.
func (p *passPlanner) accept(pts []vec2, seedIdx int) {
	lats := make([]float32, len(pts))
	sign := float32(1)
	if n, ok := p.nearest(pts[seedIdx], latReach); ok && dot2(perpAt(pts, seedIdx), n.up) < 0 {
		sign = -1
	}
	known := make([]bool, len(pts))
	for k, q := range pts {
		if n, ok := p.nearest(q, latReach); ok {
			lats[k] = n.lat + dot2(vec2{q[0] - n.p[0], q[1] - n.p[1]}, n.up)
			known[k] = true
		}
	}
	fillGaps(lats, known)
	smoothLats(lats)
	for k, q := range pts {
		perp := perpAt(pts, k)
		pp := passPt{p: q, up: vec2{perp[0] * sign, perp[1] * sign}, lat: lats[k]}
		b := bucketOf(q)
		p.pointGrid[b] = append(p.pointGrid[b], pp)
	}
	p.passes = append(p.passes, world.GroomPass{TrailID: p.trailID, Pts: pts, Lats: lats, Sign: sign})
}

// fillGaps carries the nearest known lateral coordinate into runs of
// points with no neighbouring lane.
func fillGaps(lats []float32, known []bool) {
	last := -1
	for k := range lats {
		if known[k] {
			if last < 0 {
				for j := 0; j < k; j++ {
					lats[j] = lats[k]
				}
			}
			last = k
		} else if last >= 0 {
			lats[k] = lats[last]
		}
	}
}

// smoothLats evens out the coordinate along a lane, so a switch between
// neighbours doesn't kink the ridges.
func smoothLats(lats []float32) {
	const r = 3
	src := append([]float32(nil), lats...)
	for k := range lats {
		var sum float32
		n := 0
		for j := max(k-r, 0); j <= min(k+r, len(src)-1); j++ {
			sum += src[j]
			n++
		}
		lats[k] = sum / float32(n)
	}
}

// perpAt is the left-hand normal of pts at index j.
func perpAt(pts []vec2, j int) vec2 {
	a, b := pts[max(j-1, 0)], pts[min(j+1, len(pts)-1)]
	tan := norm2(vec2{b[0] - a[0], b[1] - a[1]})
	return vec2{-tan[1], tan[0]}
}

// grow seeds lanes one spacing to either side of every accepted pass,
// working outward until no more fit.
func (p *passPlanner) grow(from int) {
	for i := from; i < len(p.passes); i++ {
		pts := p.passes[i].Pts
		for j := 0; j < len(pts); j += 2 {
			perp := perpAt(pts, j)
			tan := vec2{perp[1], -perp[0]}
			for _, s := range [2]float32{1, -1} {
				c := vec2{pts[j][0] + perp[0]*world.SnowcatLaneSpacing*s, pts[j][1] + perp[1]*world.SnowcatLaneSpacing*s}
				if !p.inside(c) || p.near(c, world.SnowcatLaneSpacing*0.95) {
					continue
				}
				if lane, seedIdx := p.trace(c, tan); len(lane) >= passMinLen {
					p.accept(lane, seedIdx)
				}
			}
		}
	}
}

// plan lays out the trail's passes.
func (p *passPlanner) plan() []world.GroomPass {
	if len(p.cells) == 0 {
		return nil
	}
	// Highest cells first: the first lane starts at the top, and filler
	// seeds for missed corners are tried top-down.
	order := append([][2]int(nil), p.cells...)
	sort.Slice(order, func(i, j int) bool {
		ei := p.t.GroundElevationAt(order[i][0], order[i][1])
		ej := p.t.GroundElevationAt(order[j][0], order[j][1])
		if ei != ej {
			return ei > ej
		}
		if order[i][0] != order[j][0] {
			return order[i][0] < order[j][0]
		}
		return order[i][1] < order[j][1]
	})
	for _, c := range order {
		centre := vec2{(float32(c[0]) + 0.5) * world.CellSize, (float32(c[1]) + 0.5) * world.CellSize}
		if p.near(centre, passCoverDist) {
			continue
		}
		from := len(p.passes)
		pts, seedIdx := p.trace(centre, vec2{0, 1})
		if len(pts) < 2 {
			// A corner too tight for a lane still gets a short stab.
			h := p.dirAt(centre, vec2{0, 1})
			pts, seedIdx = []vec2{{centre[0] - h[0]*0.5, centre[1] - h[1]*0.5}, {centre[0] + h[0]*0.5, centre[1] + h[1]*0.5}}, 0
		}
		p.accept(pts, seedIdx)
		p.grow(from)
	}
	return p.passes
}

// planTrailPasses lays out grooming passes covering a trail's snow cells.
func planTrailPasses(t *world.Terrain, trail *world.Trail) []world.GroomPass {
	return newPassPlanner(t, trail).plan()
}

// passCells returns the cells, among those keep accepts, whose centres
// lie within a swath of passes.
func passCells(t *world.Terrain, passes []world.GroomPass, keep func(c [2]int) bool) [][2]int {
	seen := map[[2]int]bool{}
	var out [][2]int
	r := int(math.Ceil(float64(passCoverDist / world.CellSize)))
	for _, ps := range passes {
		for _, q := range ps.Pts {
			cx, cz := int(q[0]/world.CellSize), int(q[1]/world.CellSize)
			for dx := -r; dx <= r; dx++ {
				for dz := -r; dz <= r; dz++ {
					c := [2]int{cx + dx, cz + dz}
					if seen[c] || !t.InBounds(c[0], c[1]) || !keep(c) {
						continue
					}
					ex := (float32(c[0])+0.5)*world.CellSize - q[0]
					ez := (float32(c[1])+0.5)*world.CellSize - q[1]
					if ex*ex+ez*ez <= passCoverDist*passCoverDist {
						seen[c] = true
						out = append(out, c)
					}
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}
