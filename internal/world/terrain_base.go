package world

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
)

// TerrainBase is an imported map's ground before any terrain layer ran,
// kept in scenarios so the editor can switch layers on and off and
// reopen the import on the same square. Player saves drop it.
type TerrainBase struct {
	Geo GeoBounds
	// Heights is the ground on a W × H lattice, row-major, north row
	// first, in metres above World.BaseAltitude: the 1.25 m detail
	// lattice when the import found lidar (Detail), else the cell grid.
	W, H    int
	Detail  bool
	Heights []float32

	Roads    []BaseRoad // OpenStreetMap roads in Geo, for the road layer
	RoadNote string     // why there are none, when there aren't

	// The OpenStreetMap ski features in Geo, drawn by the editor's
	// OpenStreetMap overlay. Data © OpenStreetMap contributors, ODbL.
	Lifts []BaseLift
	Runs  []BaseRun
	Areas []BaseArea
	// The OpenStreetMap water in Geo: streams and rivers, and lakes.
	Streams []BaseStream
	Lakes   []BaseLake

	LidarCoverage float32
	LidarNote     string

	// LayersOff lists the IDs of switched-off layers, so layers added
	// later start on.
	LayersOff []string
	// Strengths is each layer's strength, by ID, where it isn't
	// DefaultLayerStrength.
	Strengths map[string]float32
}

// DefaultLayerStrength is a layer's strength until it's changed: each
// layer's standard pass. 0 does next to nothing and 1 goes as far past
// the standard pass as the layer usefully can.
const DefaultLayerStrength = 0.5

// LayerScale is a multiplier for a layer's standard settings at
// strength s: atZero at 0, 1 at DefaultLayerStrength, atFull at 1,
// linear in between.
func LayerScale(s float32, atZero, atFull float64) float64 {
	t := float64(min(max(s, 0), 1)) / DefaultLayerStrength
	if t <= 1 {
		return atZero + (1-atZero)*t
	}
	return 1 + (atFull-1)*(t-1)
}

// BaseRoad is one OpenStreetMap road: its name, highway class
// (motorway, residential...), width in metres, and its path as
// (lat, lon) pairs.
type BaseRoad struct {
	Name, Kind string
	Width      float32
	Tunnel     bool
	Path       [][2]float64
}

// BaseLift is one OpenStreetMap lift: its aerialway kind (chair_lift,
// gondola, t-bar, magic_carpet...), seats per chair (0 when untagged),
// and its line from one station to the other as (lat, lon) pairs.
type BaseLift struct {
	Name, Kind string
	Seats      int
	Path       [][2]float64
}

// BaseRun is one OpenStreetMap downhill run: its piste:difficulty
// (novice, easy, intermediate, advanced, expert, freeride, extreme),
// and its centre line, or its outline when Area.
type BaseRun struct {
	Name, Difficulty string
	Area             bool
	Path             [][2]float64
}

// BaseStream is one OpenStreetMap waterway: its kind (river, stream,
// canal, ditch, drain), whether it dries up in summer, and its line in
// the direction the water flows, as (lat, lon) pairs.
type BaseStream struct {
	Name, Kind   string
	Intermittent bool
	Path         [][2]float64
}

// BaseLake is a lake, pond, or reservoir: its water kind ("" when
// untagged) and one or more outlines.
type BaseLake struct {
	Name, Kind string
	Paths      [][][2]float64
}

// BaseArea is a ski area's mapped boundary, as one or more outlines.
type BaseArea struct {
	Name  string
	Paths [][][2]float64
}

// LayerOn reports whether the layer with this ID is switched on.
func (b *TerrainBase) LayerOn(id string) bool {
	return !slices.Contains(b.LayersOff, id)
}

// SetLayer switches the layer with this ID on or off.
func (b *TerrainBase) SetLayer(id string, on bool) {
	b.LayersOff = slices.DeleteFunc(b.LayersOff, func(s string) bool { return s == id })
	if !on {
		b.LayersOff = append(b.LayersOff, id)
	}
}

// Strength is the layer with this ID's strength, 0–1.
func (b *TerrainBase) Strength(id string) float32 {
	if s, ok := b.Strengths[id]; ok {
		return s
	}
	return DefaultLayerStrength
}

// SetStrength sets the layer with this ID's strength, 0–1.
func (b *TerrainBase) SetStrength(id string, s float32) {
	s = min(max(s, 0), 1)
	if s == DefaultLayerStrength {
		delete(b.Strengths, id)
		return
	}
	if b.Strengths == nil {
		b.Strengths = map[string]float32{}
	}
	b.Strengths[id] = s
}

// HeightsBytes is Heights for saving: whole centimetres, each row as
// zigzag varints of the difference from the sample before it, which
// gzip packs tightly.
func (b *TerrainBase) HeightsBytes() []byte {
	out := make([]byte, 0, len(b.Heights)*2)
	var buf [binary.MaxVarintLen64]byte
	for j := 0; j < b.H; j++ {
		var prev int64
		for _, v := range b.Heights[j*b.W : (j+1)*b.W] {
			cm := int64(math.Round(float64(v) * 100))
			n := binary.PutVarint(buf[:], cm-prev)
			out = append(out, buf[:n]...)
			prev = cm
		}
	}
	return out
}

// SetHeightsBytes reads heights written by HeightsBytes for a W × H base.
func (b *TerrainBase) SetHeightsBytes(data []byte) error {
	h := make([]float32, b.W*b.H)
	for j := 0; j < b.H; j++ {
		var v int64
		for i := 0; i < b.W; i++ {
			d, n := binary.Varint(data)
			if n <= 0 {
				return fmt.Errorf("terrain base heights end early at row %d", j)
			}
			data = data[n:]
			v += d
			h[j*b.W+i] = float32(v) / 100
		}
	}
	b.Heights = h
	return nil
}
