package world

import "math"

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

	hCells  int
	cellAvg []float32 // the MogulSize the map last wrote to each cell (x*hCells+z)
}

// MogulPxPerCell is the mogul map's resolution: one pixel per metre.
const MogulPxPerCell = 5

// mogulFull is a pixel's value at full moguls.
const mogulFull = 65535

// NewMogulMap allocates an empty map for a terrain of wCells × hCells.
func NewMogulMap(wCells, hCells int) *MogulMap {
	w, h := wCells*MogulPxPerCell, hCells*MogulPxPerCell
	return &MogulMap{W: w, H: h, Px: make([]uint16, w*h), hCells: hCells, cellAvg: make([]float32, wCells*hCells)}
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
		t.SnowDirty = true
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
