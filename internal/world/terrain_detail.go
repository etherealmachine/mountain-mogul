package world

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// DetailPerCell is how many detail samples span one cell edge: 1.25 m.
const DetailPerCell = 4

// TerrainDetail holds sub-cell height offsets on a lattice over the cell
// corner grid: sample (i, j) sits at corner-grid coordinate
// (i/DetailPerCell, j/DetailPerCell). Each offset is added to the
// terrain mesh's flat triangle surface at that point, so all-zero detail
// draws exactly the 5 m mesh. Offsets are relative to the corner heights
// the mesh is built from; anything that changes ground elevation makes
// them stale.
type TerrainDetail struct {
	W, H int       // samples: (Width-1)*DetailPerCell+1 × (Height-1)*DetailPerCell+1
	Off  []float32 // metres, row-major: Off[j*W+i]
}

// NewTerrainDetail allocates all-zero detail for a terrain of
// wCells × hCells.
func NewTerrainDetail(wCells, hCells int) *TerrainDetail {
	w := (wCells-1)*DetailPerCell + 1
	h := (hCells-1)*DetailPerCell + 1
	return &TerrainDetail{W: w, H: h, Off: make([]float32, w*h)}
}

// At is the offset at corner-grid coordinate (gx, gz), bilinearly
// interpolated between samples and clamped to the lattice.
func (d *TerrainDetail) At(gx, gz float32) float32 {
	fx := clampf(gx*DetailPerCell, 0, float32(d.W-1))
	fz := clampf(gz*DetailPerCell, 0, float32(d.H-1))
	i0, j0 := int(fx), int(fz)
	i1, j1 := min(i0+1, d.W-1), min(j0+1, d.H-1)
	tx, tz := fx-float32(i0), fz-float32(j0)
	a := d.Off[j0*d.W+i0]*(1-tx) + d.Off[j0*d.W+i1]*tx
	b := d.Off[j1*d.W+i0]*(1-tx) + d.Off[j1*d.W+i1]*tx
	return a*(1-tz) + b*tz
}

// DetailAt is the detail offset at world position (wx, wz), ignoring the
// mesh's corner jitter; 0 when the terrain has no detail.
func (t *Terrain) DetailAt(wx, wz float32) float32 {
	if t.Detail == nil {
		return 0
	}
	return t.Detail.At(wx/CellSize, wz/CellSize)
}

// MaxAbs is the largest offset magnitude, for padding bounds.
func (d *TerrainDetail) MaxAbs() float32 {
	var m float32
	for _, v := range d.Off {
		m = max(m, float32(math.Abs(float64(v))))
	}
	return m
}

// FillDetailTest replaces t.Detail with a synthetic pattern for checking
// that the renderer's seams, levels of detail, and VisualElevationAt
// agree: rolling bumps everywhere plus a sharp 6 m step across the map's
// middle.
func (t *Terrain) FillDetailTest() {
	d := NewTerrainDetail(t.Width, t.Height)
	mid := float32(t.Width-1) * CellSize / 2
	for j := 0; j < d.H; j++ {
		for i := 0; i < d.W; i++ {
			x := float32(i) * CellSize / DetailPerCell
			z := float32(j) * CellSize / DetailPerCell
			v := 1.2 * float32(math.Sin(float64(x)*2*math.Pi/9)*math.Sin(float64(z)*2*math.Pi/7))
			v += 6 * smoothstepf(-2, 2, x-mid)
			d.Off[j*d.W+i] = v
		}
	}
	t.Detail = d
}

// CornerGroundY is the mesh's ground height at corner (cx, cz): the
// average of the up to four cells around it, so a step between cells
// becomes a ramp.
func (t *Terrain) CornerGroundY(cx, cz int) float32 {
	var sum, n float32
	for x := max(cx-1, 0); x <= min(cx, t.Width-1); x++ {
		for z := max(cz-1, 0); z <= min(cz, t.Height-1); z++ {
			sum += t.Cells[x][z].GroundElevation
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

// MeshGroundAt is the mesh's flat ground surface, before snow and
// detail, at corner-grid coordinate (gx, gz): the cell's two triangles,
// split along a diagonal that alternates in a checkerboard.
func (t *Terrain) MeshGroundAt(gx, gz float32) float32 {
	gx = clampf(gx, 0, float32(t.Width-1))
	gz = clampf(gz, 0, float32(t.Height-1))
	x, z := min(int(gx), t.Width-2), min(int(gz), t.Height-2)
	fx, fz := gx-float32(x), gz-float32(z)
	e00, e10 := t.CornerGroundY(x, z), t.CornerGroundY(x+1, z)
	e01, e11 := t.CornerGroundY(x, z+1), t.CornerGroundY(x+1, z+1)
	if (x+z)%2 == 0 {
		if fz <= fx {
			return (1-fx)*e00 + (fx-fz)*e10 + fz*e11
		}
		return (1-fz)*e00 + fx*e11 + (fz-fx)*e01
	}
	if fx+fz <= 1 {
		return (1-fx-fz)*e00 + fx*e10 + fz*e01
	}
	return (1-fz)*e10 + (fx+fz-1)*e11 + (1-fx)*e01
}

// SetDetailFromHeights sets t.Detail so the drawn ground follows the
// given heights on the detail lattice (row-major, NaN where unknown,
// which keeps the plain mesh there).
func (t *Terrain) SetDetailFromHeights(w, h int, heights []float32) error {
	d := NewTerrainDetail(t.Width, t.Height)
	if w != d.W || h != d.H || len(heights) != w*h {
		return fmt.Errorf("detail heights are %d × %d; this terrain needs %d × %d", w, h, d.W, d.H)
	}
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			v := heights[j*w+i]
			if v != v {
				continue
			}
			d.Off[j*w+i] = v - t.MeshGroundAt(float32(i)/DetailPerCell, float32(j)/DetailPerCell)
		}
	}
	t.Detail = d
	return nil
}

// Bytes is the detail for saving: each offset in whole centimetres
// (clamped to ±327 m) as the little-endian int16 difference from the
// sample before it in its row, which gzip packs far tighter than floats.
func (d *TerrainDetail) Bytes() []byte {
	out := make([]byte, len(d.Off)*2)
	for j := 0; j < d.H; j++ {
		var prev int16
		for i := 0; i < d.W; i++ {
			k := j*d.W + i
			v := int16(clampf(float32(math.Round(float64(d.Off[k])*100)), -32767, 32767))
			delta := uint16(v - prev)
			out[2*k], out[2*k+1] = byte(delta), byte(delta>>8)
			prev = v
		}
	}
	return out
}

// LoadTerrainDetail rebuilds detail saved by Bytes for a terrain of
// wCells × hCells, or returns nil if b doesn't fit it.
func LoadTerrainDetail(wCells, hCells int, b []byte) *TerrainDetail {
	d := NewTerrainDetail(wCells, hCells)
	if len(b) != len(d.Off)*2 {
		return nil
	}
	for j := 0; j < d.H; j++ {
		var v int16
		for i := 0; i < d.W; i++ {
			k := j*d.W + i
			v += int16(uint16(b[2*k]) | uint16(b[2*k+1])<<8)
			d.Off[k] = float32(v) / 100
		}
	}
	return d
}

const detailHeightsMagic = "MMDH0001"

// WriteDetailHeights writes heights on a w × h detail lattice to path,
// gzipped: magic, w, h, then little-endian float32s, row-major.
func WriteDetailHeights(path string, w, h int, heights []float32) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(f)
	bw := bufio.NewWriter(gz)
	bw.WriteString(detailHeightsMagic)
	binary.Write(bw, binary.LittleEndian, [2]int32{int32(w), int32(h)})
	binary.Write(bw, binary.LittleEndian, heights)
	if err := bw.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := gz.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ReadDetailHeights reads a file written by WriteDetailHeights.
func ReadDetailHeights(path string) (w, h int, heights []float32, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, 0, nil, err
	}
	br := bufio.NewReader(gz)
	magic := make([]byte, len(detailHeightsMagic))
	if _, err := io.ReadFull(br, magic); err != nil || string(magic) != detailHeightsMagic {
		return 0, 0, nil, fmt.Errorf("%s: not a detail heights file", path)
	}
	var dims [2]int32
	if err := binary.Read(br, binary.LittleEndian, &dims); err != nil {
		return 0, 0, nil, err
	}
	w, h = int(dims[0]), int(dims[1])
	heights = make([]float32, w*h)
	if err := binary.Read(br, binary.LittleEndian, heights); err != nil {
		return 0, 0, nil, err
	}
	return w, h, heights, nil
}

func clampf(v, lo, hi float32) float32 {
	return min(max(v, lo), hi)
}

func smoothstepf(e0, e1, x float32) float32 {
	t := clampf((x-e0)/(e1-e0), 0, 1)
	return t * t * (3 - 2*t)
}
