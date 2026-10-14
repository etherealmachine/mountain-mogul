package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Pathfinder finds walking routes on the terrain grid for guests heading
// to a lift line or a door, and for patrollers on foot. A route is the
// cells to walk to in turn, the last the destination: only its corners,
// so walkers go straight between them rather than cell by cell.
//
// The search is A* in eight directions (a diagonal costs √2 and never
// cuts the corner of a blocked cell), over walkable cells out of open
// water; the
// destination may be blocked (a lift base, a door in a wall). Crossing a
// rope or the ski area boundary costs extra (ropeStepCost), so routes go
// round or stay inside when they can, but aren't stopped. A cell on a footpath costs 1/FootpathSpeedup,
// as it's walked that much faster, and the estimate to the goal assumes
// it's all path when there are paths, so the search finds the quickest
// way. The cell route is then cut down to its corners: from each kept
// cell, the furthest one the straight walk to which stays on walkable
// cells, on a footpath when both ends are on one, and on the same side
// of every rope and the boundary.
//
// It keeps its working arrays between searches, so it isn't safe to use
// from more than one goroutine at once (the sim calls it serially).
type Pathfinder struct {
	terrain *world.Terrain
	world   *world.World
	sc      routeScratch
}

// NewPathfinder creates a new Pathfinder for w's terrain.
func NewPathfinder(w *world.World) *Pathfinder {
	return &Pathfinder{terrain: w.Terrain, world: w}
}

// FindPath returns the walking route from `from` to `to`: the start, the
// corners, and the destination; nil if there's no way.
func (p *Pathfinder) FindPath(from, to [2]int) [][2]int {
	t := p.terrain
	if !t.InBounds(from[0], from[1]) || !t.InBounds(to[0], to[1]) {
		return nil
	}
	if from == to {
		return [][2]int{to}
	}
	w, h := t.Width, t.Height
	idx := func(c [2]int) int32 { return int32(c[0]*h + c[1]) }
	cellOf := func(k int32) [2]int { return [2]int{int(k) / h, int(k) % h} }
	paths := p.world != nil && p.world.HasFootpaths()
	open := func(c [2]int) bool {
		if !t.InBounds(c[0], c[1]) {
			return false
		}
		return c == to || t.Cells[c[0]][c[1]].Walkable() && !t.OpenWaterCell(c[0], c[1])
	}
	cost := func(c [2]int) float32 {
		if paths && p.world.FootpathCell(c[0], c[1]) {
			return 1.0 / world.FootpathSpeedup
		}
		return 1
	}
	hScale := float32(1)
	if paths {
		hScale = 1.0 / world.FootpathSpeedup
	}
	heur := func(c [2]int) float32 {
		dx, dz := abs32(float32(c[0]-to[0])), abs32(float32(c[1]-to[1]))
		return hScale * (max(dx, dz) + (math.Sqrt2-1)*min(dx, dz))
	}

	sc := &p.sc
	sc.begin(w * h)
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
			step := cost(nb)
			if d[0] != 0 && d[1] != 0 {
				// No cutting a blocked corner.
				if !open([2]int{pos[0] + d[0], pos[1]}) || !open([2]int{pos[0], pos[1] + d[1]}) {
					continue
				}
				step *= math.Sqrt2
			}
			step += ropeStepCost(p.world, pos, nb)
			if ng, k := cur.g+step, idx(nb); ng < sc.gAt(k) {
				sc.setG(k, ng, cur.k)
				sc.open.push(skiOpenNode{k: k, g: ng, f: ng + heur(nb)})
			}
		}
	}
	if !found {
		return nil
	}
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
	return p.corners(cells, open, paths)
}

// cellCentreXZ is cell c's centre in world XZ metres.
func cellCentreXZ(c [2]int) mgl32.Vec2 {
	return mgl32.Vec2{(float32(c[0]) + 0.5) * world.CellSize, (float32(c[1]) + 0.5) * world.CellSize}
}

// footWalkCost is the time to walk straight from a to b, in metres of
// snow: footpath stretches count 1/FootpathSpeedup of their length.
func footWalkCost(w *world.World, a, b mgl32.Vec2) float32 {
	d := b.Sub(a)
	length := d.Len()
	n := int(length) + 1
	step := length / float32(n)
	var c float32
	for i := 0; i < n; i++ {
		q := a.Add(d.Mul((float32(i) + 0.5) / float32(n)))
		// On the path itself, as walking speed reads it (not just in a
		// path's cell), so a line along a path's edge isn't priced as on it.
		if w.OnFootpath(q) {
			c += step / world.FootpathSpeedup
		} else {
			c += step
		}
	}
	return c
}

// footShortcutSlack is how much slower than the route a straight walk
// may be and still replace it, metres of snow: room for rounding, not
// enough to leave a path for the snow beside it.
const footShortcutSlack = 0.5

// footShortcutOK reports whether walking straight from a to b is no
// slower than the route between them, which takes route (time) to walk.
// With footpaths about, this keeps corner cutting from straightening a
// route off the path the search chose (Boreal Playtest).
func footShortcutOK(w *world.World, a, b mgl32.Vec2, route float32) bool {
	return footWalkCost(w, a, b) <= route+footShortcutSlack
}

// corners cuts a cell route down to its start, its corners and its end:
// from each kept cell, the furthest one the straight walk to which is
// clear (walkClearLine) and, with footpaths about, no slower than the
// route it replaces.
func (p *Pathfinder) corners(cells [][2]int, open func([2]int) bool, paths bool) [][2]int {
	var cum []float32
	if paths {
		cum = make([]float32, len(cells))
		for k := 1; k < len(cells); k++ {
			cum[k] = cum[k-1] + footWalkCost(p.world, cellCentreXZ(cells[k-1]), cellCentreXZ(cells[k]))
		}
	}
	out := [][2]int{cells[0]}
	for i := 0; i < len(cells)-1; {
		j := i + 1
		for j+1 < len(cells) && p.walkClearLine(cells[i], cells[j+1], open) &&
			(!paths || footShortcutOK(p.world, cellCentreXZ(cells[i]), cellCentreXZ(cells[j+1]), cum[j+1]-cum[i])) {
			j++
		}
		out = append(out, cells[j])
		i = j
	}
	return out
}

// pathLineSpread is how far either side of a straight walk between two
// cell centres the line is checked, metres: a walker's shoulders, so a
// line doesn't graze a blocked cell's corner.
const pathLineSpread = 1.0

// walkClearLine reports whether the straight walk from cell a's centre to
// cell b's crosses only open cells.
func (p *Pathfinder) walkClearLine(a, b [2]int, open func([2]int) bool) bool {
	pa, pb := cellCentreXZ(a), cellCentreXZ(b)
	d := pb.Sub(pa)
	length := d.Len()
	if length < 1e-3 {
		return true
	}
	side := mgl32.Vec2{-d[1], d[0]}.Mul(pathLineSpread / length)
	// Nor across a rope, or out of (or into) the ski area, that the
	// cells went round.
	t := p.terrain
	if p.world != nil && p.world.RopeCrosses(pa, pb) {
		return false
	}
	inside := t.IsAccessible(a[0], a[1])
	keepSide := inside == t.IsAccessible(b[0], b[1])
	n := int(length) + 1
	for i := 0; i <= n; i++ {
		q := pa.Add(d.Mul(float32(i) / float32(n)))
		for _, o := range [3]mgl32.Vec2{{}, side, side.Mul(-1)} {
			r := q.Add(o)
			c := [2]int{int(r[0] / world.CellSize), int(r[1] / world.CellSize)}
			if !open(c) || keepSide && t.InBounds(c[0], c[1]) && t.IsAccessible(c[0], c[1]) != inside {
				return false
			}
		}
	}
	return true
}
