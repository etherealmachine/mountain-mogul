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
// cuts the corner of a blocked cell), over cells that are walkable and
// on land the resort can use; the destination may be blocked (a lift
// base, a door in a wall). A cell on a footpath costs 1/FootpathSpeedup,
// as it's walked that much faster, and the estimate to the goal assumes
// it's all path when there are paths, so the search finds the quickest
// way. The cell route is then cut down to its corners: from each kept
// cell, the furthest one the straight walk to which stays on walkable
// cells, and on a footpath when both ends are on one.
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
		return c == to || (t.IsAccessible(c[0], c[1]) && t.Cells[c[0]][c[1]].Walkable())
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

// corners cuts a cell route down to its start, its corners and its end:
// from each kept cell, the furthest one the straight walk to which is
// clear (walkClearLine).
func (p *Pathfinder) corners(cells [][2]int, open func([2]int) bool, paths bool) [][2]int {
	out := [][2]int{cells[0]}
	for i := 0; i < len(cells)-1; {
		j := i + 1
		for j+1 < len(cells) && p.walkClearLine(cells[i], cells[j+1], open, paths) {
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
// cell b's crosses only open cells, and, when both are on a footpath,
// stays on footpath cells.
func (p *Pathfinder) walkClearLine(a, b [2]int, open func([2]int) bool, paths bool) bool {
	centre := func(c [2]int) mgl32.Vec2 {
		return mgl32.Vec2{(float32(c[0]) + 0.5) * world.CellSize, (float32(c[1]) + 0.5) * world.CellSize}
	}
	pa, pb := centre(a), centre(b)
	d := pb.Sub(pa)
	length := d.Len()
	if length < 1e-3 {
		return true
	}
	side := mgl32.Vec2{-d[1], d[0]}.Mul(pathLineSpread / length)
	keepToPath := paths && p.world.FootpathCell(a[0], a[1]) && p.world.FootpathCell(b[0], b[1])
	n := int(length) + 1
	for i := 0; i <= n; i++ {
		q := pa.Add(d.Mul(float32(i) / float32(n)))
		for _, o := range [3]mgl32.Vec2{{}, side, side.Mul(-1)} {
			r := q.Add(o)
			c := [2]int{int(r[0] / world.CellSize), int(r[1] / world.CellSize)}
			if !open(c) {
				return false
			}
		}
		if keepToPath {
			c := [2]int{int(q[0] / world.CellSize), int(q[1] / world.CellSize)}
			if !p.world.FootpathCell(c[0], c[1]) {
				return false
			}
		}
	}
	return true
}
