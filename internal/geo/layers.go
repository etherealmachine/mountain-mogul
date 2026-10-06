package geo

import (
	"math"
	"slices"
	"time"

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
	// Run changes the w × ht heights, spacing metres apart, in place.
	Run func(h []float32, w, ht int, spacing float64, base *world.TerrainBase)
}

// Layers is every terrain layer, in the order they run.
var Layers = []Layer{
	{
		ID: "roads", Name: "Smooth roads", MovesGround: true,
		Run: func(h []float32, w, ht int, spacing float64, base *world.TerrainBase) {
			SmoothRoads(h, w, ht, boundsOf(base.Geo), spacing, roadsOf(base.Roads))
		},
	},
	{
		ID: "smooth", Name: "Smooth ground", MovesGround: true, DetailOnly: true,
		Run: func(h []float32, w, ht int, spacing float64, _ *world.TerrainBase) {
			SmoothGround(h, w, ht, spacing)
		},
	},
	{
		ID: "erode", Name: "Erode", MovesGround: true, DetailOnly: true,
		Run: func(h []float32, w, ht int, spacing float64, base *world.TerrainBase) {
			g := base.Geo
			Erode(h, w, ht, spacing, int64(math.Float64bits(g.MinLat)^math.Float64bits(g.MinLon)))
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
	}
	return true
}

// LayerStack runs the layers over a base and keeps each layer's output,
// so switching one layer only reruns it and the layers after it. Not
// safe for concurrent use.
type LayerStack struct {
	base *world.TerrainBase
	on   []bool      // what each kept output was made with
	out  [][]float32 // heights after each layer; shared with the one before when it was skipped
	// Took is how long each layer last took to run; 0 when it was skipped.
	Took []time.Duration
}

// NewLayerStack makes a stack for base.
func NewLayerStack(base *world.TerrainBase) *LayerStack {
	return &LayerStack{base: base, Took: make([]time.Duration, len(Layers))}
}

// Base is the base the stack runs on.
func (s *LayerStack) Base() *world.TerrainBase { return s.base }

// Run returns the base's ground with every layer applied except those
// in off, reusing outputs from the last run up to the first layer that
// changed. progress, when set, is called with each layer's name before
// it runs. The result must not be modified.
func (s *LayerStack) Run(off []string, progress func(name string)) []float32 {
	isOn := func(i int) bool { return !slices.Contains(off, Layers[i].ID) }
	from := 0
	for from < len(s.on) && s.on[from] == isOn(from) {
		from++
	}
	s.on, s.out = s.on[:from], s.out[:from]
	cur := s.base.Heights
	if from > 0 {
		cur = s.out[from-1]
	}
	spacing := world.CellSize
	if s.base.Detail {
		spacing /= world.DetailPerCell
	}
	for i := from; i < len(Layers); i++ {
		on := isOn(i)
		s.Took[i] = 0
		if on && Applies(i, s.base) {
			if progress != nil {
				progress(Layers[i].Name)
			}
			start := time.Now()
			next := append([]float32(nil), cur...)
			Layers[i].Run(next, s.base.W, s.base.H, spacing, s.base)
			s.Took[i] = time.Since(start)
			cur = next
		}
		s.on = append(s.on, on)
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
		out[k] = world.BaseRoad{Width: float32(r.Width), Tunnel: r.Tunnel, Path: make([][2]float64, len(r.Path))}
		for n, p := range r.Path {
			out[k].Path[n] = [2]float64{p.Lat, p.Lon}
		}
	}
	return out
}
