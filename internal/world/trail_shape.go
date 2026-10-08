package world

import (
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
)

// A trail is drawn, not painted: a run is a line of nodes, each with its
// own width, from a lift top (or a building, or another trail) to a lift
// base (or a building or trail); an area is an outline. Trail.Cells are
// derived from the shape (ShapeTrail) so the graph, grooming, capacity,
// and everything else that reads cells keep working.

// TrailKind is what a trail is: a run, or an ungroomed area.
type TrailKind uint8

const (
	TrailRun         TrailKind = iota // a line of nodes with widths; may be groomed
	TrailGlade                        // an outlined area in the trees; never groomed
	TrailBowl                         // an outlined open area; never groomed
	TrailBackcountry                  // an outlined area beyond the boundary; never groomed
	TrailKindCount
)

// Label is the kind's name.
func (k TrailKind) Label() string {
	switch k {
	case TrailGlade:
		return "Glade"
	case TrailBowl:
		return "Bowl"
	case TrailBackcountry:
		return "Backcountry"
	}
	return "Run"
}

// IsArea reports whether the kind is drawn as an outline.
func (k TrailKind) IsArea() bool { return k != TrailRun }

// TrailNode is one node of a run: where its centre line passes, and how
// wide the run is there (metres).
type TrailNode struct {
	Pos   mgl32.Vec2
	Width float32
}

// Run widths, metres.
const (
	TrailMinWidth     = 8
	TrailMaxWidth     = 80
	TrailDefaultWidth = 25
)

// TrailEnd is what a run's end node is attached to: a lift's top or base,
// a building, or another trail (Kind and ID as in a TrailEdge), or
// nothing (Set false).
type TrailEnd struct {
	Set  bool
	Kind EdgeKind
	ID   uint64
}

// trailSampleStep is the spacing of a run's centre line samples.
const trailSampleStep = 2.5

// TrailSample is a point on a run's smoothed centre line.
type TrailSample struct {
	Pos   mgl32.Vec2
	Width float32
}

// Centerline is the run's centre line smoothed through its nodes (a
// Catmull-Rom curve), sampled about every trailSampleStep metres, with
// the width interpolated along it. Nil for an area or a run with fewer
// than two nodes.
func (t *Trail) Centerline() []TrailSample {
	n := len(t.Nodes)
	if t.Kind.IsArea() || n < 2 {
		return nil
	}
	at := func(i int) TrailNode { return t.Nodes[min(max(i, 0), n-1)] }
	out := []TrailSample{{Pos: t.Nodes[0].Pos, Width: t.Nodes[0].Width}}
	for i := 0; i < n-1; i++ {
		p0, p1, p2, p3 := at(i-1).Pos, at(i).Pos, at(i+1).Pos, at(i+2).Pos
		w1, w2 := at(i).Width, at(i+1).Width
		steps := max(int(math.Ceil(float64(p2.Sub(p1).Len()/trailSampleStep))), 1)
		for s := 1; s <= steps; s++ {
			u := float32(s) / float32(steps)
			out = append(out, TrailSample{Pos: catmullRom(p0, p1, p2, p3, u), Width: w1 + (w2-w1)*u})
		}
	}
	return out
}

// catmullRom is the uniform Catmull-Rom curve through p1 (u=0) and p2
// (u=1) with neighbours p0 and p3.
func catmullRom(p0, p1, p2, p3 mgl32.Vec2, u float32) mgl32.Vec2 {
	u2, u3 := u*u, u*u*u
	return p0.Mul(-0.5*u3 + u2 - 0.5*u).
		Add(p1.Mul(1.5*u3 - 2.5*u2 + 1)).
		Add(p2.Mul(-1.5*u3 + 2*u2 + 0.5*u)).
		Add(p3.Mul(0.5*u3 - 0.5*u2))
}

// Length is the run's centre line length in metres (0 for an area).
func (t *Trail) Length() float32 {
	var l float32
	c := t.Centerline()
	for i := 1; i < len(c); i++ {
		l += c[i].Pos.Sub(c[i-1].Pos).Len()
	}
	return l
}

// endTarget is where an attached end sits and the cell it must cover:
// a lift's top or base station; for a building or trail, the node's own
// spot (placed there when it was attached). ok is false when the thing
// it's attached to is gone.
func (w *World) endTarget(e TrailEnd, at mgl32.Vec2) (pos mgl32.Vec2, cell [2]int, ok bool) {
	switch e.Kind {
	case KindLiftTop, KindLiftBase:
		for _, l := range w.Lifts {
			if l.ID != e.ID {
				continue
			}
			if e.Kind == KindLiftTop {
				return l.Top, l.TopCell(), true
			}
			return l.Base, l.QueueCell(), true
		}
		return at, [2]int{}, false
	case KindBuilding:
		return at, cellOf(at), w.BuildingByID(e.ID) != nil
	case KindTrail:
		return at, cellOf(at), w.FindTrail(e.ID) != nil
	}
	return at, [2]int{}, false
}

// ShapeTrail re-derives t's cells from its shape: a run's ends follow
// the lift stations they're attached to (an end whose lift or building
// is gone comes loose), then every cell whose centre lies within the
// run's half-width of its centre line, every cell the line passes
// through (so a narrow run never breaks), and the cells under attached
// ends (so it connects); an area takes every cell whose centre is inside
// its outline.
func (w *World) ShapeTrail(t *Trail) {
	cells := map[[2]int]bool{}
	if t.Kind.IsArea() {
		rasterPolygon(t.Outline, cells)
	} else {
		for i, e := range [2]*TrailEnd{&t.Start, &t.End} {
			if !e.Set || len(t.Nodes) == 0 {
				continue
			}
			ni := 0
			if i == 1 {
				ni = len(t.Nodes) - 1
			}
			pos, cell, ok := w.endTarget(*e, t.Nodes[ni].Pos)
			if !ok {
				*e = TrailEnd{}
				continue
			}
			t.Nodes[ni].Pos = pos
			cells[cell] = true
		}
		rasterRun(t.Centerline(), cells)
	}
	t.Cells = t.Cells[:0]
	for c := range cells {
		if w.Terrain == nil || w.Terrain.InBounds(c[0], c[1]) {
			t.Cells = append(t.Cells, c)
		}
	}
	sort.Slice(t.Cells, func(i, j int) bool {
		if t.Cells[i][0] != t.Cells[j][0] {
			return t.Cells[i][0] < t.Cells[j][0]
		}
		return t.Cells[i][1] < t.Cells[j][1]
	})
}

// rasterRun adds the cells a run's centre line covers.
func rasterRun(line []TrailSample, cells map[[2]int]bool) {
	for _, s := range line {
		cells[cellOf(s.Pos)] = true
	}
	for i := 1; i < len(line); i++ {
		a, b := line[i-1], line[i]
		half := max(a.Width, b.Width) / 2
		lo := cellOf(mgl32.Vec2{min(a.Pos[0], b.Pos[0]) - half, min(a.Pos[1], b.Pos[1]) - half})
		hi := cellOf(mgl32.Vec2{max(a.Pos[0], b.Pos[0]) + half, max(a.Pos[1], b.Pos[1]) + half})
		ab := b.Pos.Sub(a.Pos)
		l2 := ab.Dot(ab)
		for x := lo[0]; x <= hi[0]; x++ {
			for z := lo[1]; z <= hi[1]; z++ {
				c := mgl32.Vec2{(float32(x) + 0.5) * CellSize, (float32(z) + 0.5) * CellSize}
				u := float32(0)
				if l2 > 0 {
					u = min(max(c.Sub(a.Pos).Dot(ab)/l2, 0), 1)
				}
				h := (a.Width + (b.Width-a.Width)*u) / 2
				if c.Sub(a.Pos.Add(ab.Mul(u))).Len() <= h {
					cells[[2]int{x, z}] = true
				}
			}
		}
	}
}

// rasterPolygon adds the cells whose centres are inside poly.
func rasterPolygon(poly []mgl32.Vec2, cells map[[2]int]bool) {
	if len(poly) < 3 {
		return
	}
	lo, hi := poly[0], poly[0]
	for _, p := range poly {
		lo = mgl32.Vec2{min(lo[0], p[0]), min(lo[1], p[1])}
		hi = mgl32.Vec2{max(hi[0], p[0]), max(hi[1], p[1])}
	}
	c0, c1 := cellOf(lo), cellOf(hi)
	for x := c0[0]; x <= c1[0]; x++ {
		for z := c0[1]; z <= c1[1]; z++ {
			if PointInPolygon(mgl32.Vec2{(float32(x) + 0.5) * CellSize, (float32(z) + 0.5) * CellSize}, poly) {
				cells[[2]int{x, z}] = true
			}
		}
	}
}

// PointInPolygon reports whether p is inside poly (even-odd rule).
func PointInPolygon(p mgl32.Vec2, poly []mgl32.Vec2) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if (a[1] > p[1]) != (b[1] > p[1]) && p[0] < (b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0] {
			in = !in
		}
	}
	return in
}

// PlaceRun adds a run along nodes, its ends attached as given, and
// shapes it. Callers rebuild the trail graph.
func (w *World) PlaceRun(name string, diff TerrainDifficulty, nodes []TrailNode, start, end TrailEnd) *Trail {
	t := w.PlaceTrail(name, diff)
	t.Kind = TrailRun
	t.Nodes = append([]TrailNode(nil), nodes...)
	t.Start, t.End = start, end
	w.ShapeTrail(t)
	return t
}

// PlaceArea adds an area of kind k inside outline and shapes it. Callers
// rebuild the trail graph.
func (w *World) PlaceArea(name string, k TrailKind, diff TerrainDifficulty, outline []mgl32.Vec2) *Trail {
	t := w.PlaceTrail(name, diff)
	t.Kind = k
	t.Outline = append([]mgl32.Vec2(nil), outline...)
	w.ShapeTrail(t)
	return t
}
