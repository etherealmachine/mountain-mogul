package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Footpaths are cleared walkways the player lays between buildings, lots
// and lifts: a line of nodes with a width at each, smoothed like a run's
// (smoothLine). Guests on foot walk them twice as fast (sim), and their
// routes prefer them. The derived index answers "is this point, or this
// cell, on a path" for the sim's walkers and route searches.

// Footpath is one path.
type Footpath struct {
	ID    uint64
	Nodes []TrailNode
}

// Footpath widths, metres, and price.
const (
	FootpathMinWidth     = 1.5
	FootpathMaxWidth     = 8
	FootpathDefaultWidth = 3
	// FootpathCostPerM2 is what a square metre of path costs to lay.
	FootpathCostPerM2 = 10
	// FootpathSpeedup is how much faster guests walk on a path.
	FootpathSpeedup = 2
)

// Centerline is the path's smoothed centre line.
func (f *Footpath) Centerline() []TrailSample { return smoothLine(f.Nodes) }

// FootpathCost is what laying a path through nodes costs: its area at
// FootpathCostPerM2.
func FootpathCost(nodes []TrailNode) int {
	line := smoothLine(nodes)
	var area float32
	for i := 1; i < len(line); i++ {
		area += line[i].Pos.Sub(line[i-1].Pos).Len() * (line[i].Width + line[i-1].Width) / 2
	}
	return int(area * FootpathCostPerM2)
}

// PlaceFootpath adds a path through nodes and returns it.
func (w *World) PlaceFootpath(nodes []TrailNode) *Footpath {
	f := &Footpath{ID: w.NextID(), Nodes: append([]TrailNode(nil), nodes...)}
	w.Footpaths = append(w.Footpaths, f)
	w.RebuildFootpaths()
	return f
}

// RemoveFootpath deletes path id.
func (w *World) RemoveFootpath(id uint64) {
	for i, f := range w.Footpaths {
		if f.ID == id {
			w.Footpaths = append(w.Footpaths[:i], w.Footpaths[i+1:]...)
			w.RebuildFootpaths()
			return
		}
	}
}

// FootpathByID is path id, or nil.
func (w *World) FootpathByID(id uint64) *Footpath {
	for _, f := range w.Footpaths {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// footpathSeg is one piece of a path's centre line, a to b, half width hw.
type footpathSeg struct {
	path uint64
	a, b mgl32.Vec2
	hw   float32
}

// footpathIndex buckets path pieces by terrain cell, and marks the cells
// a path crosses.
type footpathIndex struct {
	w, h  int
	segs  [][]footpathSeg // per cell (cx*h + cz): pieces within reach of it
	cells []bool          // per cell: a path crosses it
}

// footpathReach is how far beyond a path's edge a point or cell centre
// still counts as on it: walkers step a little wide.
const footpathReach = 0.5

// RebuildFootpaths rebuilds the path index after any path changes, and
// bumps FootpathsRev for the renderer.
func (w *World) RebuildFootpaths() {
	w.FootpathsRev++
	t := w.Terrain
	if t == nil {
		return
	}
	idx := footpathIndex{w: t.Width, h: t.Height}
	if len(w.Footpaths) > 0 {
		idx.segs = make([][]footpathSeg, t.Width*t.Height)
		idx.cells = make([]bool, t.Width*t.Height)
	}
	for _, f := range w.Footpaths {
		line := f.Centerline()
		for i := 1; i < len(line); i++ {
			seg := footpathSeg{path: f.ID, a: line[i-1].Pos, b: line[i].Pos, hw: max(line[i-1].Width, line[i].Width) / 2}
			// Every cell whose centre the piece passes within half a
			// cell plus its half width of is in reach of it.
			r := seg.hw + footpathReach + CellSize
			x0, x1 := int((min(seg.a[0], seg.b[0])-r)/CellSize), int((max(seg.a[0], seg.b[0])+r)/CellSize)
			z0, z1 := int((min(seg.a[1], seg.b[1])-r)/CellSize), int((max(seg.a[1], seg.b[1])+r)/CellSize)
			for cx := max(x0, 0); cx <= min(x1, t.Width-1); cx++ {
				for cz := max(z0, 0); cz <= min(z1, t.Height-1); cz++ {
					k := cx*t.Height + cz
					idx.segs[k] = append(idx.segs[k], seg)
					centre := mgl32.Vec2{(float32(cx) + 0.5) * CellSize, (float32(cz) + 0.5) * CellSize}
					if d, _ := segDist(centre, seg.a, seg.b); d <= seg.hw+CellSize/2 {
						idx.cells[k] = true
					}
				}
			}
		}
	}
	w.footpaths = idx
}

// segDist is p's distance from segment ab, and the nearest point on it.
func segDist(p, a, b mgl32.Vec2) (float32, mgl32.Vec2) {
	ab := b.Sub(a)
	l2 := ab.LenSqr()
	u := float32(0)
	if l2 > 1e-6 {
		u = min(max(p.Sub(a).Dot(ab)/l2, 0), 1)
	}
	q := a.Add(ab.Mul(u))
	return p.Sub(q).Len(), q
}

// FootpathCell reports whether a path crosses cell (cx, cz).
func (w *World) FootpathCell(cx, cz int) bool {
	idx := &w.footpaths
	if idx.cells == nil || cx < 0 || cz < 0 || cx >= idx.w || cz >= idx.h {
		return false
	}
	return idx.cells[cx*idx.h+cz]
}

// HasFootpaths reports whether there are any paths.
func (w *World) HasFootpaths() bool { return w.footpaths.cells != nil }

// OnFootpath reports whether p is on a path (within its half width,
// plus a little).
func (w *World) OnFootpath(p mgl32.Vec2) bool {
	for _, seg := range w.footpathSegsAt(p) {
		if d, _ := segDist(p, seg.a, seg.b); d <= seg.hw+footpathReach {
			return true
		}
	}
	return false
}

// SnapToFootpath is the nearest point on a path's centre line to p, if
// one is within reach (a path's half width plus a cell), so walkers
// routed through a path's cells keep to the path itself.
func (w *World) SnapToFootpath(p mgl32.Vec2) (mgl32.Vec2, bool) {
	best, found := float32(math.MaxFloat32), mgl32.Vec2{}
	for _, seg := range w.footpathSegsAt(p) {
		if d, q := segDist(p, seg.a, seg.b); d <= seg.hw+CellSize && d < best {
			best, found = d, q
		}
	}
	return found, best < math.MaxFloat32
}

// footpathSegsAt is the path pieces in reach of p's cell.
func (w *World) footpathSegsAt(p mgl32.Vec2) []footpathSeg {
	idx := &w.footpaths
	if idx.segs == nil {
		return nil
	}
	cx, cz := int(p[0]/CellSize), int(p[1]/CellSize)
	if p[0] < 0 || p[1] < 0 || cx >= idx.w || cz >= idx.h {
		return nil
	}
	return idx.segs[cx*idx.h+cz]
}

// FootpathAt is the path whose centre line passes nearest p within its
// half width plus reach, or nil; for picking a path to select or remove.
func (w *World) FootpathAt(p mgl32.Vec2, reach float32) *Footpath {
	var best *Footpath
	bestD := float32(math.MaxFloat32)
	for _, f := range w.Footpaths {
		line := f.Centerline()
		for i := 1; i < len(line); i++ {
			d, _ := segDist(p, line[i-1].Pos, line[i].Pos)
			if hw := max(line[i-1].Width, line[i].Width) / 2; d <= hw+reach && d < bestD {
				best, bestD = f, d
			}
		}
	}
	return best
}
