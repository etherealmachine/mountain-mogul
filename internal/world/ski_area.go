package world

import "github.com/go-gl/mathgl/mgl32"

// SkiAreaOutline is one closed outline of the resort's ski-area
// boundary, in world XZ metres. A scenario's author draws them in the
// editor, over OpenStreetMap's boundary where the import has one. The
// ground inside any outline is the resort's skiable terrain.
type SkiAreaOutline struct {
	Points []mgl32.Vec2
}

// Contains reports whether world (x, z) is inside the outline (even-odd).
func (o SkiAreaOutline) Contains(x, z float32) bool {
	in := false
	n := len(o.Points)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		a, b := o.Points[i], o.Points[j]
		if (a[1] > z) != (b[1] > z) && x < (b[0]-a[0])*(z-a[1])/(b[1]-a[1])+a[0] {
			in = !in
		}
	}
	return in
}

// InSkiArea reports whether world (x, z) is inside the ski-area
// boundary.
func (w *World) InSkiArea(x, z float32) bool {
	for _, o := range w.SkiArea {
		if o.Contains(x, z) {
			return true
		}
	}
	return false
}

// AddSkiArea adds a closed outline to the boundary; fewer than three
// points does nothing.
func (w *World) AddSkiArea(points []mgl32.Vec2) {
	if len(points) < 3 {
		return
	}
	w.SkiArea = append(w.SkiArea, SkiAreaOutline{Points: append([]mgl32.Vec2(nil), points...)})
	w.skiAreaCells, w.skiAreaMask = -1, nil
}

// RemoveSkiArea removes outline i.
func (w *World) RemoveSkiArea(i int) {
	w.SkiArea = append(w.SkiArea[:i], w.SkiArea[i+1:]...)
	w.skiAreaCells, w.skiAreaMask = -1, nil
}

// SkiAreaAt is the index of the outline around world (x, z), or -1.
func (w *World) SkiAreaAt(x, z float32) int {
	for i, o := range w.SkiArea {
		if o.Contains(x, z) {
			return i
		}
	}
	return -1
}

// SetSkiArea replaces the boundary (save loading).
func (w *World) SetSkiArea(outlines []SkiAreaOutline) {
	w.SkiArea = outlines
	w.skiAreaCells, w.skiAreaMask = -1, nil
}

// SkiAreaMask is, for each terrain cell (z*Width + x), whether its
// centre is inside the boundary; nil without one. Cached until the
// boundary changes, so overlays can draw it every frame.
func (w *World) SkiAreaMask() []bool {
	if len(w.SkiArea) == 0 || w.Terrain == nil {
		return nil
	}
	if w.skiAreaMask != nil {
		return w.skiAreaMask
	}
	t := w.Terrain
	m := make([]bool, t.Width*t.Height)
	n := 0
	for _, o := range w.SkiArea {
		if len(o.Points) < 3 {
			continue
		}
		lo, hi := o.Points[0], o.Points[0]
		for _, p := range o.Points {
			lo = mgl32.Vec2{min(lo[0], p[0]), min(lo[1], p[1])}
			hi = mgl32.Vec2{max(hi[0], p[0]), max(hi[1], p[1])}
		}
		x0, z0 := max(int(lo[0]/CellSize), 0), max(int(lo[1]/CellSize), 0)
		x1, z1 := min(int(hi[0]/CellSize), t.Width-1), min(int(hi[1]/CellSize), t.Height-1)
		for z := z0; z <= z1; z++ {
			for x := x0; x <= x1; x++ {
				if k := z*t.Width + x; !m[k] && o.Contains((float32(x)+0.5)*CellSize, (float32(z)+0.5)*CellSize) {
					m[k] = true
					n++
				}
			}
		}
	}
	w.skiAreaMask, w.skiAreaCells = m, n
	return m
}

// SkiAreaCells is how many terrain cells have their centre inside the
// boundary: the skiable terrain.
func (w *World) SkiAreaCells() int {
	if w.SkiAreaMask() == nil {
		return 0
	}
	return w.skiAreaCells
}

// MoveSkiAreaPoint moves corner i of outline o to p.
func (w *World) MoveSkiAreaPoint(o, i int, p mgl32.Vec2) {
	w.SkiArea[o].Points[i] = p
	w.skiAreaCells, w.skiAreaMask = -1, nil
}

// InsertSkiAreaPoint makes p corner i of outline o, before the corner
// that was i.
func (w *World) InsertSkiAreaPoint(o, i int, p mgl32.Vec2) {
	pts := w.SkiArea[o].Points
	w.SkiArea[o].Points = append(pts[:i:i], append([]mgl32.Vec2{p}, pts[i:]...)...)
	w.skiAreaCells, w.skiAreaMask = -1, nil
}

// RemoveSkiAreaPoint removes corner i of outline o, unless that would
// leave fewer than three; reports whether it did.
func (w *World) RemoveSkiAreaPoint(o, i int) bool {
	pts := w.SkiArea[o].Points
	if len(pts) <= 3 {
		return false
	}
	w.SkiArea[o].Points = append(pts[:i:i], pts[i+1:]...)
	w.skiAreaCells, w.skiAreaMask = -1, nil
	return true
}
