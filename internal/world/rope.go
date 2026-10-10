package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Rope lines: ropes on bamboo poles the player strings across the snow
// to guide guests (notes/next/Rope Lines.md). A rope isn't a wall: skiers
// steer to keep from crossing one and walking routes go round where
// they can, but anyone who ends up crossing just does, with no
// collision. The ski area boundary (the edge of owned land, drawn as a
// rope fence) follows the same rule. The derived index answers "does
// this cell have a rope in it" and "where does this line first cross a
// rope" for the sim.

// Rope is one rope line, straight between its nodes.
type Rope struct {
	ID    uint64
	Nodes []mgl32.Vec2
}

const (
	// RopeCostPerM is what a metre of rope and poles costs.
	RopeCostPerM = 4
	// RopePolePitch is the spacing of the poles along a rope, metres.
	RopePolePitch = 3.0
)

// RopeLength is how long a rope through nodes is, metres.
func RopeLength(nodes []mgl32.Vec2) float32 {
	var l float32
	for i := 1; i < len(nodes); i++ {
		l += nodes[i].Sub(nodes[i-1]).Len()
	}
	return l
}

// RopeCost is what stringing a rope through nodes costs.
func RopeCost(nodes []mgl32.Vec2) int {
	return int(math.Ceil(float64(RopeLength(nodes) * RopeCostPerM)))
}

// PlaceRope adds a rope through nodes and returns it.
func (w *World) PlaceRope(nodes []mgl32.Vec2) *Rope {
	r := &Rope{ID: w.NextID(), Nodes: append([]mgl32.Vec2(nil), nodes...)}
	w.Ropes = append(w.Ropes, r)
	w.RebuildRopes()
	return r
}

// RemoveRope takes rope id down.
func (w *World) RemoveRope(id uint64) {
	for i, r := range w.Ropes {
		if r.ID == id {
			w.Ropes = append(w.Ropes[:i], w.Ropes[i+1:]...)
			w.RebuildRopes()
			return
		}
	}
}

// RopeByID is rope id, or nil.
func (w *World) RopeByID(id uint64) *Rope {
	for _, r := range w.Ropes {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// RopeAt is the rope passing nearest p within reach, or nil; for picking
// one to edit or remove.
func (w *World) RopeAt(p mgl32.Vec2, reach float32) *Rope {
	var best *Rope
	bestD := reach
	for _, r := range w.Ropes {
		for i := 1; i < len(r.Nodes); i++ {
			if d, _ := segDist(p, r.Nodes[i-1], r.Nodes[i]); d <= bestD {
				best, bestD = r, d
			}
		}
	}
	return best
}

// ropeSeg is one straight stretch of a rope.
type ropeSeg struct{ a, b mgl32.Vec2 }

// ropeIndex buckets rope stretches by terrain cell: each cell lists the
// stretches passing through it or its neighbours, and marks whether one
// passes through it.
type ropeIndex struct {
	w, h  int
	segs  [][]ropeSeg
	cells []bool
}

// RebuildRopes rebuilds the rope index after any rope changes, and bumps
// RopesRev for the renderer.
func (w *World) RebuildRopes() {
	w.RopesRev++
	t := w.Terrain
	if t == nil {
		return
	}
	idx := ropeIndex{w: t.Width, h: t.Height}
	if len(w.Ropes) > 0 {
		idx.segs = make([][]ropeSeg, t.Width*t.Height)
		idx.cells = make([]bool, t.Width*t.Height)
	}
	for _, r := range w.Ropes {
		for i := 1; i < len(r.Nodes); i++ {
			seg := ropeSeg{r.Nodes[i-1], r.Nodes[i]}
			x0 := int(min(seg.a[0], seg.b[0])/CellSize) - 1
			x1 := int(max(seg.a[0], seg.b[0])/CellSize) + 1
			z0 := int(min(seg.a[1], seg.b[1])/CellSize) - 1
			z1 := int(max(seg.a[1], seg.b[1])/CellSize) + 1
			for cx := max(x0, 0); cx <= min(x1, t.Width-1); cx++ {
				for cz := max(z0, 0); cz <= min(z1, t.Height-1); cz++ {
					k := cx*t.Height + cz
					idx.segs[k] = append(idx.segs[k], seg)
					if segCrossesCell(seg, cx, cz) {
						idx.cells[k] = true
					}
				}
			}
		}
	}
	w.ropes = idx
}

// segCrossesCell reports whether stretch s passes through cell (cx, cz).
func segCrossesCell(s ropeSeg, cx, cz int) bool {
	lo := mgl32.Vec2{float32(cx) * CellSize, float32(cz) * CellSize}
	hi := lo.Add(mgl32.Vec2{CellSize, CellSize})
	// Clip the stretch to the cell's box (Liang–Barsky).
	t0, t1 := float32(0), float32(1)
	d := s.b.Sub(s.a)
	for axis := range 2 {
		if math.Abs(float64(d[axis])) < 1e-6 {
			if s.a[axis] < lo[axis] || s.a[axis] > hi[axis] {
				return false
			}
			continue
		}
		u0 := (lo[axis] - s.a[axis]) / d[axis]
		u1 := (hi[axis] - s.a[axis]) / d[axis]
		if u0 > u1 {
			u0, u1 = u1, u0
		}
		t0, t1 = max(t0, u0), min(t1, u1)
		if t0 > t1 {
			return false
		}
	}
	return true
}

// HasRopes reports whether any ropes are up.
func (w *World) HasRopes() bool { return w.ropes.cells != nil }

// RopeCell reports whether a rope passes through cell (cx, cz).
func (w *World) RopeCell(cx, cz int) bool {
	idx := &w.ropes
	if idx.cells == nil || cx < 0 || cz < 0 || cx >= idx.w || cz >= idx.h {
		return false
	}
	return idx.cells[cx*idx.h+cz]
}

// RopeCrossing is how far along the straight line from a to b (0 to 1)
// it first crosses a rope, and whether it does.
func (w *World) RopeCrossing(a, b mgl32.Vec2) (float32, bool) {
	idx := &w.ropes
	if idx.segs == nil {
		return 0, false
	}
	d := b.Sub(a)
	length := d.Len()
	best, found := float32(2), false
	// Each cell lists the stretches through it and its neighbours, so
	// checking the cells under every few metres of the line finds every
	// stretch it can cross.
	n := int(length/CellSize) + 1
	lastK := -1
	for i := 0; i <= n; i++ {
		p := a.Add(d.Mul(float32(i) / float32(n)))
		cx, cz := int(p[0]/CellSize), int(p[1]/CellSize)
		if p[0] < 0 || p[1] < 0 || cx >= idx.w || cz >= idx.h {
			continue
		}
		k := cx*idx.h + cz
		if k == lastK {
			continue
		}
		lastK = k
		for _, s := range idx.segs[k] {
			if u, ok := segIntersect(a, b, s.a, s.b); ok && u < best {
				best, found = u, true
			}
		}
		if found && float32(i)/float32(n) > best {
			break // nothing nearer lies further along
		}
	}
	return best, found
}

// RopeCrosses reports whether the straight line from a to b crosses a
// rope.
func (w *World) RopeCrosses(a, b mgl32.Vec2) bool {
	_, ok := w.RopeCrossing(a, b)
	return ok
}

// segIntersect is where segment ab crosses segment cd, as a fraction of
// the way from a to b, and whether it does.
func segIntersect(a, b, c, d mgl32.Vec2) (float32, bool) {
	r, s := b.Sub(a), d.Sub(c)
	den := r[0]*s[1] - r[1]*s[0]
	if math.Abs(float64(den)) < 1e-9 {
		return 0, false // parallel
	}
	ca := c.Sub(a)
	u := (ca[0]*s[1] - ca[1]*s[0]) / den
	v := (ca[0]*r[1] - ca[1]*r[0]) / den
	if u < 0 || u > 1 || v < 0 || v > 1 {
		return 0, false
	}
	return u, true
}
