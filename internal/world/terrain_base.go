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

	LidarCoverage float32
	LidarNote     string

	// LayersOff lists the IDs of switched-off layers, so layers added
	// later start on.
	LayersOff []string
}

// BaseRoad is one OpenStreetMap road: its width in metres, and its
// path as (lat, lon) pairs.
type BaseRoad struct {
	Width  float32
	Tunnel bool
	Path   [][2]float64
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
