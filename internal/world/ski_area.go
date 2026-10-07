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
	w.skiAreaCells = -1
}

// RemoveSkiAreaAt removes the outline around world (x, z), reporting
// whether there was one.
func (w *World) RemoveSkiAreaAt(x, z float32) bool {
	for i, o := range w.SkiArea {
		if o.Contains(x, z) {
			w.SkiArea = append(w.SkiArea[:i], w.SkiArea[i+1:]...)
			w.skiAreaCells = -1
			return true
		}
	}
	return false
}

// SetSkiArea replaces the boundary (save loading).
func (w *World) SetSkiArea(outlines []SkiAreaOutline) {
	w.SkiArea = outlines
	w.skiAreaCells = -1
}

// SkiAreaCells is how many terrain cells have their centre inside the
// boundary: the skiable terrain. Cached until the boundary changes.
func (w *World) SkiAreaCells() int {
	if len(w.SkiArea) == 0 || w.Terrain == nil {
		return 0
	}
	if w.skiAreaCells >= 0 {
		return w.skiAreaCells
	}
	n := 0
	for x := 0; x < w.Terrain.Width; x++ {
		for z := 0; z < w.Terrain.Height; z++ {
			if w.InSkiArea((float32(x)+0.5)*CellSize, (float32(z)+0.5)*CellSize) {
				n++
			}
		}
	}
	w.skiAreaCells = n
	return n
}
