package geo

import (
	"math"

	"mountain-mogul/internal/world"
)

// Layer is one terrain pass run on an import's base ground, in a fixed
// order, which the editor's Layers panel switches on and off.
type Layer struct {
	ID, Name string
	// MovesGround says the layer changes heights, so switching it clears
	// everything built on the ground (lifts, buildings, roads), which
	// would otherwise float or be buried.
	MovesGround bool
	// DetailOnly layers need the 1.25 m lidar detail and do nothing on
	// a tile-only import.
	DetailOnly bool
	// Fixed layers have no strength slider; Run ignores the strength.
	Fixed bool
	// Run changes the w × ht heights, spacing metres apart, in place, at
	// strength s (0–1, world.DefaultLayerStrength the standard pass).
	Run func(h []float32, w, ht int, spacing float64, base *world.TerrainBase, s float32)
}

// Layers is every terrain layer, in the order they run.
var Layers = []Layer{
	{
		ID: "roads", Name: "Smooth roads", MovesGround: true,
		// Stronger: a corridor up to twice as wide, with a quarter of the
		// bumps put back.
		Run: func(h []float32, w, ht int, spacing float64, base *world.TerrainBase, s float32) {
			SmoothRoads(h, w, ht, boundsOf(base.Geo), spacing, roadsOf(base.Roads), RoadSmoothing{
				Width:  world.LayerScale(s, 0.5, 2),
				Rough:  world.LayerScale(s, 1, 0.25),
				Amount: world.LayerScale(s, 0, 1),
			})
		},
	},
	{
		ID: "smooth", Name: "Smooth ground", MovesGround: true, DetailOnly: true,
		// Stronger: a blur radius up to three times as wide, enough to
		// round off moguls and small knolls.
		Run: func(h []float32, w, ht int, spacing float64, _ *world.TerrainBase, s float32) {
			SmoothGround(h, w, ht, spacing, world.LayerScale(s, 0.5, 3), world.LayerScale(s, 0, 1))
		},
	},
	{
		ID: "erode", Name: "Erode", MovesGround: true, DetailOnly: true,
		// Stronger: up to three times the drops, each carrying up to twice
		// as much, so gullies cut deeper and reach further up.
		Run: func(h []float32, w, ht int, spacing float64, base *world.TerrainBase, s float32) {
			g := base.Geo
			Erode(h, w, ht, spacing, world.LayerScale(s, 0, 3), world.LayerScale(s, 0.6, 2),
				int64(math.Float64bits(g.MinLat)^math.Float64bits(g.MinLon)))
		},
	},
	{
		// After erosion, so the channels it cuts stay crisp.
		ID: "creeks", Name: "Creeks", MovesGround: true, Fixed: true,
		Run: func(h []float32, w, ht int, spacing float64, base *world.TerrainBase, _ float32) {
			CarveCreeks(h, w, ht, spacing, CreekStreams(h, w, ht, spacing, base))
		},
	},
	{
		// Last, so nothing roughens the levelled water afterwards.
		ID: "lakes", Name: "Lakes", MovesGround: true, Fixed: true,
		Run: func(h []float32, w, ht int, _ float64, base *world.TerrainBase, _ float32) {
			FlattenLakes(h, w, ht, boundsOf(base.Geo), base.Lakes)
		},
	},
}

// Applies reports whether layer i does anything on this base.
func Applies(i int, base *world.TerrainBase) bool {
	l := Layers[i]
	switch {
	case l.DetailOnly && !base.Detail:
		return false
	case l.ID == "roads" && len(base.Roads) == 0:
		return false
	case l.ID == "lakes" && len(base.Lakes) == 0:
		return false
	case l.ID == "creeks" && len(base.Streams) == 0:
		return false
	}
	return true
}

// Setting is how a ground layer is set: on or off, and its strength.
type Setting struct {
	On       bool
	Strength float32
}

// Settings is how each of Layers is set on base, in order.
func Settings(base *world.TerrainBase) []Setting {
	set := make([]Setting, len(Layers))
	for i, l := range Layers {
		set[i] = Setting{On: base.LayerOn(l.ID), Strength: base.Strength(l.ID)}
	}
	return set
}

// LayerStack runs the layers over a base and keeps each layer's output,
// so switching one layer only reruns it and the layers after it. Not
// safe for concurrent use.
type LayerStack struct {
	base *world.TerrainBase
	set  []Setting   // what each kept output was made with
	out  [][]float32 // heights after each layer; shared with the one before when it was skipped
}

// NewLayerStack makes a stack for base.
func NewLayerStack(base *world.TerrainBase) *LayerStack {
	return &LayerStack{base: base}
}

// Base is the base the stack runs on.
func (s *LayerStack) Base() *world.TerrainBase { return s.base }

// Run returns the base's ground with each layer applied as set (from
// Settings), reusing outputs from the last run up to the first layer
// whose setting changed. A layer that's off, at strength 0, or does
// nothing here is skipped. progress, when set, is called with
// each layer's name before it runs. The result must not be modified.
func (s *LayerStack) Run(set []Setting, progress func(name string)) []float32 {
	runs := func(i int, st Setting) bool { return st.On && st.Strength > 0 && Applies(i, s.base) }
	same := func(i int) bool {
		a, b := s.set[i], set[i]
		return runs(i, a) == runs(i, b) && (!runs(i, a) || a.Strength == b.Strength)
	}
	from := 0
	for from < len(s.set) && same(from) {
		from++
	}
	s.set, s.out = s.set[:from], s.out[:from]
	cur := s.base.Heights
	if from > 0 {
		cur = s.out[from-1]
	}
	spacing := world.CellSize
	if s.base.Detail {
		spacing /= world.DetailPerCell
	}
	for i := from; i < len(Layers); i++ {
		if runs(i, set[i]) {
			if progress != nil {
				progress(Layers[i].Name)
			}
			next := append([]float32(nil), cur...)
			Layers[i].Run(next, s.base.W, s.base.H, spacing, s.base, set[i].Strength)
			cur = next
		}
		s.set = append(s.set, set[i])
		s.out = append(s.out, cur)
	}
	return cur
}

// ApplyHeights sets t's ground from heights on base's lattice: the cells
// directly, or, with detail, the cells as the lattice's average over
// each cell and the detail as what's left.
func ApplyHeights(t *world.Terrain, base *world.TerrainBase, heights []float32) error {
	if !base.Detail {
		for z := 0; z < t.Height; z++ {
			for x := 0; x < t.Width; x++ {
				t.Cells[x][z].GroundElevation = heights[z*base.W+x]
			}
		}
		t.Detail = nil
		t.RecomputeSlopes()
		return nil
	}
	cells := cellsFromLattice(heights, base.W, base.H, t.Width, t.Height, world.DetailPerCell)
	for z, row := range cells {
		for x, v := range row {
			t.Cells[x][z].GroundElevation = v
		}
	}
	t.RecomputeSlopes()
	return t.SetDetailFromHeights(base.W, base.H, heights)
}

// BoundsOf is g as a Bounds.
func BoundsOf(g world.GeoBounds) Bounds { return boundsOf(g) }

func boundsOf(g world.GeoBounds) Bounds {
	return Bounds{MinLat: g.MinLat, MaxLat: g.MaxLat, MinLon: g.MinLon, MaxLon: g.MaxLon}
}

func roadsOf(rs []world.BaseRoad) []Road {
	out := make([]Road, len(rs))
	for k, r := range rs {
		out[k] = Road{Width: float64(r.Width), Tunnel: r.Tunnel, Path: make([]LatLon, len(r.Path))}
		for n, p := range r.Path {
			out[k].Path[n] = LatLon{Lat: p[0], Lon: p[1]}
		}
	}
	return out
}

func baseRoads(rs []Road) []world.BaseRoad {
	out := make([]world.BaseRoad, len(rs))
	for k, r := range rs {
		out[k] = world.BaseRoad{Name: r.Name, Kind: r.Kind, Width: float32(r.Width), Tunnel: r.Tunnel, Path: basePath(r.Path)}
	}
	return out
}

func basePath(ps []LatLon) [][2]float64 {
	out := make([][2]float64, len(ps))
	for n, p := range ps {
		out[n] = [2]float64{p.Lat, p.Lon}
	}
	return out
}

func baseLifts(ls []SkiLift) []world.BaseLift {
	var out []world.BaseLift
	for _, l := range ls {
		out = append(out, world.BaseLift{Name: l.Name, Kind: l.Kind, Seats: l.Seats, Path: basePath(l.Path)})
	}
	return out
}

func baseRuns(rs []SkiRun) []world.BaseRun {
	var out []world.BaseRun
	for _, r := range rs {
		out = append(out, world.BaseRun{Name: r.Name, Difficulty: r.Difficulty, Area: r.Area, Path: basePath(r.Path)})
	}
	return out
}

func baseStreams(ws []Waterway) []world.BaseStream {
	var out []world.BaseStream
	for _, w := range ws {
		out = append(out, world.BaseStream{Name: w.Name, Kind: w.Kind, Intermittent: w.Intermittent, Path: basePath(w.Path)})
	}
	return out
}

func baseLakes(ls []WaterArea) []world.BaseLake {
	var out []world.BaseLake
	for _, l := range ls {
		bl := world.BaseLake{Name: l.Name, Kind: l.Kind}
		for _, p := range l.Paths {
			bl.Paths = append(bl.Paths, basePath(p))
		}
		out = append(out, bl)
	}
	return out
}

func baseAreas(as []SkiAreaOutline) []world.BaseArea {
	var out []world.BaseArea
	for _, a := range as {
		ba := world.BaseArea{Name: a.Name}
		for _, p := range a.Paths {
			ba.Paths = append(ba.Paths, basePath(p))
		}
		out = append(out, ba)
	}
	return out
}
