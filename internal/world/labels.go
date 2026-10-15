package world

import "github.com/go-gl/mathgl/mgl32"

// LabelKind is what a map label names, which sets how it's drawn.
type LabelKind uint8

const (
	// LabelPeak is a mountain top: a summit mark, the name and height.
	LabelPeak LabelKind = iota
	LabelKindCount
)

// String is the kind's name in the editor.
func (k LabelKind) String() string {
	switch k {
	case LabelPeak:
		return "Peak"
	}
	return "Label"
}

// Label is a name on the map, shown in play: a mountain top for now.
// An imported map starts with OpenStreetMap's named peaks
// (TerrainBase.Peaks); the scenario editor adds, moves, renames and
// deletes them. Kept in every save, player games too.
type Label struct {
	ID   uint64
	Kind LabelKind
	Name string
	// Ele is a peak's height above sea level in metres; 0 shows the
	// ground's.
	Ele float32
	Pos mgl32.Vec2 // world XZ
}

// GeoToWorld is (lat, lon)'s world XZ on a terrain t covering g, and
// whether it's on the map: column 0 is MinLon and the last MaxLon, row 0
// MaxLat and the last MinLat.
func GeoToWorld(g GeoBounds, t *Terrain, lat, lon float64) (mgl32.Vec2, bool) {
	maxX := float32(t.Width-1) * CellSize
	maxZ := float32(t.Height-1) * CellSize
	x := float32((lon - g.MinLon) / (g.MaxLon - g.MinLon) * float64(maxX))
	z := float32((g.MaxLat - lat) / (g.MaxLat - g.MinLat) * float64(maxZ))
	return mgl32.Vec2{x, z}, x >= 0 && z >= 0 && x <= maxX && z <= maxZ
}

// AddLabel adds l with a new ID and returns it.
func (w *World) AddLabel(l Label) *Label {
	l.ID = w.NextID()
	w.Labels = append(w.Labels, &l)
	return &l
}

// RemoveLabel takes the label with this ID off the map.
func (w *World) RemoveLabel(id uint64) {
	for i, l := range w.Labels {
		if l.ID == id {
			w.Labels = append(w.Labels[:i], w.Labels[i+1:]...)
			return
		}
	}
}

// PeaksFromBase adds a peak label for each of the terrain base's peaks
// on the map that isn't labelled already (a peak label of the same
// name), and returns how many it added. Labels the editor moved or
// renamed stay as they are; one it deleted comes back.
func (w *World) PeaksFromBase() int {
	b := w.TerrainBase
	if b == nil || w.Terrain == nil {
		return 0
	}
	have := map[string]bool{}
	for _, l := range w.Labels {
		if l.Kind == LabelPeak {
			have[l.Name] = true
		}
	}
	n := 0
	for _, p := range b.Peaks {
		pos, ok := GeoToWorld(b.Geo, w.Terrain, p.At[0], p.At[1])
		if !ok || have[p.Name] {
			continue
		}
		w.AddLabel(Label{Kind: LabelPeak, Name: p.Name, Ele: p.Ele, Pos: pos})
		have[p.Name] = true
		n++
	}
	return n
}
