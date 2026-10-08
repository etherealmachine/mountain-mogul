package world

import (
	"image"
	"math"
)

// MogulMap is how big the moguls are at each metre of the mountain, 0..1,
// grown along the lines skiers actually take (sim's wearSnowUnderfoot).
// Each cell's MogulSize is the average of its pixels: physics, tastes,
// and trail conditions read that, the way they always have.
//
// Anything else may still set a cell's MogulSize directly (grooming,
// melt-out, avalanches, earthworks). The map notices the next time it
// touches the cell, or before a save (SyncMoguls), and scales the cell's
// pixels to match: down to zero when the cell was cleared, or filled
// evenly when moguls appear with no map behind them (an older save).
//
// Saved with the game.
type MogulMap struct {
	W, H int      // pixels, one per metre
	Px   []uint16 // 0..mogulFull; 16 bits so a tick's sliver of growth isn't rounded away

	// Dirty marks pixels changed since the renderer last uploaded them.
	Dirty    bool
	DirtyBox image.Rectangle

	// Dir is each cell's smoothed fall line, (dx, dz) mapped to 0..255,
	// two bytes per cell in rows of cells (z*wCells+x): the direction the
	// drawn mogul field runs (mogul_field.go). Filled by
	// RefreshMogulFallLines.
	Dir []uint8

	wCells, hCells int
	cellAvg        []float32 // the MogulSize the map last wrote to each cell (x*hCells+z)
}

// MogulPxPerCell is the mogul map's resolution: one pixel per metre.
const MogulPxPerCell = 5

// mogulFull is a pixel's value at full moguls.
const mogulFull = 65535

// NewMogulMap allocates an empty map for a terrain of wCells × hCells.
func NewMogulMap(wCells, hCells int) *MogulMap {
	w, h := wCells*MogulPxPerCell, hCells*MogulPxPerCell
	return &MogulMap{W: w, H: h, Px: make([]uint16, w*h), wCells: wCells, hCells: hCells, cellAvg: make([]float32, wCells*hCells)}
}

func (m *MogulMap) markDirty(r image.Rectangle) {
	r = r.Intersect(image.Rect(0, 0, m.W, m.H))
	if r.Empty() {
		return
	}
	if m.Dirty {
		m.DirtyBox = m.DirtyBox.Union(r)
	} else {
		m.DirtyBox = r
	}
	m.Dirty = true
}

// mogulCellRect is cell (cx, cz)'s pixels.
func mogulCellRect(cx, cz int) image.Rectangle {
	return image.Rect(cx*MogulPxPerCell, cz*MogulPxPerCell, (cx+1)*MogulPxPerCell, (cz+1)*MogulPxPerCell)
}

// StampMoguls grows the moguls around world (wx, wz) by amount (0..1 at
// the centre pixel, half on its eight neighbours), no pixel past limit
// (0..1), and updates the cells it touched.
func (t *Terrain) StampMoguls(wx, wz, amount, limit float32) {
	m := t.Moguls
	if m == nil || amount <= 0 {
		return
	}
	px, pz := int(math.Floor(float64(wx))), int(math.Floor(float64(wz)))
	touched := [4][2]int{}
	nTouched := 0
	for dz := -1; dz <= 1; dz++ {
		for dx := -1; dx <= 1; dx++ {
			x, z := px+dx, pz+dz
			if x < 0 || z < 0 || x >= m.W || z >= m.H {
				continue
			}
			cx, cz := x/MogulPxPerCell, z/MogulPxPerCell
			seen := false
			for i := 0; i < nTouched; i++ {
				if touched[i] == [2]int{cx, cz} {
					seen = true
				}
			}
			if !seen && nTouched < len(touched) {
				t.reconcileMoguls(cx, cz)
				touched[nTouched] = [2]int{cx, cz}
				nTouched++
			}
			a := amount
			if dx != 0 || dz != 0 {
				a *= 0.5
			}
			i := z*m.W + x
			if top := min(limit, 1) * mogulFull; float32(m.Px[i]) < top {
				m.Px[i] = uint16(min(float32(m.Px[i])+a*mogulFull+0.5, top))
			}
		}
	}
	for i := 0; i < nTouched; i++ {
		t.writeMogulAverage(touched[i][0], touched[i][1])
	}
	m.markDirty(image.Rect(px-1, pz-1, px+2, pz+2))
}

// reconcileMoguls scales cell (cx, cz)'s pixels to its MogulSize if
// something other than the map changed it.
func (t *Terrain) reconcileMoguls(cx, cz int) {
	m := t.Moguls
	if !t.InBounds(cx, cz) {
		return
	}
	k := cx*m.hCells + cz
	want, had := t.Cells[cx][cz].MogulSize, m.cellAvg[k]
	if want == had {
		return
	}
	for z := cz * MogulPxPerCell; z < (cz+1)*MogulPxPerCell; z++ {
		for x := cx * MogulPxPerCell; x < (cx+1)*MogulPxPerCell; x++ {
			i := z*m.W + x
			switch {
			case want <= 0:
				m.Px[i] = 0
			case had <= 0:
				m.Px[i] = uint16(min(want, 1)*mogulFull + 0.5)
			default:
				m.Px[i] = uint16(min(float32(m.Px[i])*want/had, mogulFull) + 0.5)
			}
		}
	}
	m.cellAvg[k] = want
	m.markDirty(mogulCellRect(cx, cz))
}

// writeMogulAverage sets cell (cx, cz)'s MogulSize to its pixels' average.
func (t *Terrain) writeMogulAverage(cx, cz int) {
	m := t.Moguls
	if !t.InBounds(cx, cz) {
		return
	}
	var sum int
	for z := cz * MogulPxPerCell; z < (cz+1)*MogulPxPerCell; z++ {
		for x := cx * MogulPxPerCell; x < (cx+1)*MogulPxPerCell; x++ {
			sum += int(m.Px[z*m.W+x])
		}
	}
	avg := float32(sum) / float32(MogulPxPerCell*MogulPxPerCell*mogulFull)
	if t.Cells[cx][cz].MogulSize != avg {
		t.MarkSnowDirty(cx, cz)
	}
	t.Cells[cx][cz].MogulSize = avg
	m.cellAvg[cx*m.hCells+cz] = avg
}

// SyncMoguls brings every cell's pixels in line with its MogulSize, for a
// save.
func (t *Terrain) SyncMoguls() {
	if t.Moguls == nil {
		return
	}
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			t.reconcileMoguls(x, z)
		}
	}
}

// Bytes is the map's pixels at 8 bits, for saving; nil when there are no
// moguls.
func (m *MogulMap) Bytes() []byte {
	if m == nil {
		return nil
	}
	out := make([]byte, len(m.Px))
	any := false
	for i, v := range m.Px {
		out[i] = byte(v >> 8)
		any = any || out[i] != 0
	}
	if !any {
		return nil
	}
	return out
}

// LoadMoguls copies saved pixels in and takes each cell's average as what
// the map last wrote, reporting false if they don't fit. Cells whose saved
// MogulSize differs are reconciled the next time the map touches them.
func (t *Terrain) LoadMoguls(b []byte) bool {
	m := t.Moguls
	if m == nil || len(b) != len(m.Px) {
		return false
	}
	for i, v := range b {
		m.Px[i] = uint16(v)<<8 | uint16(v)
	}
	m.markDirty(image.Rect(0, 0, m.W, m.H))
	for cx := 0; cx < t.Width; cx++ {
		for cz := 0; cz < t.Height; cz++ {
			var sum int
			for z := cz * MogulPxPerCell; z < (cz+1)*MogulPxPerCell; z++ {
				for x := cx * MogulPxPerCell; x < (cx+1)*MogulPxPerCell; x++ {
					sum += int(m.Px[z*m.W+x])
				}
			}
			m.cellAvg[cx*m.hCells+cz] = float32(sum) / float32(MogulPxPerCell*MogulPxPerCell*mogulFull)
		}
	}
	return true
}

// ScaleMoguls multiplies every mogul by factor (0..1): snow filling them
// in, or a thaw rounding them off.
func (t *Terrain) ScaleMoguls(factor float32) {
	m := t.Moguls
	if m == nil || factor >= 1 {
		return
	}
	factor = max(factor, 0)
	for cx := 0; cx < t.Width; cx++ {
		for cz := 0; cz < t.Height; cz++ {
			t.reconcileMoguls(cx, cz)
			if t.Cells[cx][cz].MogulSize == 0 {
				continue
			}
			for z := cz * MogulPxPerCell; z < (cz+1)*MogulPxPerCell; z++ {
				for x := cx * MogulPxPerCell; x < (cx+1)*MogulPxPerCell; x++ {
					i := z*m.W + x
					m.Px[i] = uint16(float32(m.Px[i])*factor + 0.5)
				}
			}
			m.markDirty(mogulCellRect(cx, cz))
			t.writeMogulAverage(cx, cz)
		}
	}
}

// FlattenMogulSwath levels the moguls a cat's tiller drove over from
// (x0, z0) to (x1, z1): gone within halfWidth of the line, easing back to
// untouched over a metre past it, so the edges of a narrow pass stay
// bumpy.
func (t *Terrain) FlattenMogulSwath(x0, z0, x1, z1, halfWidth float32) {
	m := t.Moguls
	if m == nil {
		return
	}
	const soft = 1.0
	reach := halfWidth + soft
	x0p, x1p := int(math.Floor(float64(min(x0, x1)-reach))), int(math.Ceil(float64(max(x0, x1)+reach)))
	z0p, z1p := int(math.Floor(float64(min(z0, z1)-reach))), int(math.Ceil(float64(max(z0, z1)+reach)))
	x0p, z0p = max(x0p, 0), max(z0p, 0)
	x1p, z1p = min(x1p, m.W), min(z1p, m.H)
	if x0p >= x1p || z0p >= z1p {
		return
	}
	for cx := x0p / MogulPxPerCell; cx <= (x1p-1)/MogulPxPerCell; cx++ {
		for cz := z0p / MogulPxPerCell; cz <= (z1p-1)/MogulPxPerCell; cz++ {
			t.reconcileMoguls(cx, cz)
		}
	}
	dx, dz := x1-x0, z1-z0
	l2 := dx*dx + dz*dz
	for pz := z0p; pz < z1p; pz++ {
		for px := x0p; px < x1p; px++ {
			i := pz*m.W + px
			if m.Px[i] == 0 {
				continue
			}
			cx, cz := float32(px)+0.5, float32(pz)+0.5
			s := float32(0)
			if l2 > 0 {
				s = min(max(((cx-x0)*dx+(cz-z0)*dz)/l2, 0), 1)
			}
			ex, ez := cx-(x0+dx*s), cz-(z0+dz*s)
			d := float32(math.Sqrt(float64(ex*ex + ez*ez)))
			if d >= reach {
				continue
			}
			keep := max(d-halfWidth, 0) / soft
			m.Px[i] = uint16(float32(m.Px[i])*keep + 0.5)
		}
	}
	for cx := x0p / MogulPxPerCell; cx <= (x1p-1)/MogulPxPerCell; cx++ {
		for cz := z0p / MogulPxPerCell; cz <= (z1p-1)/MogulPxPerCell; cz++ {
			t.writeMogulAverage(cx, cz)
		}
	}
	m.markDirty(image.Rect(x0p, z0p, x1p, z1p))
}
