package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Open water as guests see it (notes/next/Water Crossings.md): a cell is
// open water when any of its material samples is open water or thin ice,
// so a creek's open stretch or an unfrozen lake is a wall to the route
// searches and something skiers steer clear of. Snow bridges and lake
// ice are crossable, and a cell a bridge crosses is open to walk (the
// bridge carries them over). The grid is kept up to date by
// World.RefreshOpenWater, run serially (the sim's Tick) since route
// searches read it in parallel.

// Open reports whether m can't be stood on: open water, or ice too thin.
func (m Material) Open() bool { return m == MatOpenWater || m == MatThinIce }

// openWaterCells is the per-cell open-water grid and what it was made
// from.
type openWaterCells struct {
	cells      []bool // x*Height+z; nil when there's no open water
	material   *TerrainMaterial
	version    int
	bridgesRev int
	made       bool
}

// RefreshOpenWater remakes the open-water grid when the material map or
// the bridges have changed.
func (w *World) RefreshOpenWater() {
	t := w.Terrain
	if t == nil {
		return
	}
	o := &t.openWater
	m := t.Material
	if o.made && o.material == m && (m == nil || o.version == m.Version) && o.bridgesRev == w.BridgesRev {
		return
	}
	o.made, o.material, o.bridgesRev = true, m, w.BridgesRev
	o.cells = nil
	if m == nil {
		return
	}
	o.version = m.Version
	any := false
	cells := make([]bool, t.Width*t.Height)
	for j := 0; j < m.H; j++ {
		for i := 0; i < m.W; i++ {
			if !m.M[j*m.W+i].Open() {
				continue
			}
			cx, cz := min(i/DetailPerCell, t.Width-1), min(j/DetailPerCell, t.Height-1)
			cells[cx*t.Height+cz] = true
			any = true
		}
	}
	if !any {
		return
	}
	// A bridge opens the cells along its deck.
	for _, b := range w.Bridges {
		for s := 1; s < len(b.Points); s++ {
			a, d := b.Points[s-1], b.Points[s].Sub(b.Points[s-1])
			n := int(d.Len()) + 1
			for k := 0; k <= n; k++ {
				p := a.Add(d.Mul(float32(k) / float32(n)))
				cx, cz := int(p[0]/CellSize), int(p[1]/CellSize)
				if t.InBounds(cx, cz) {
					cells[cx*t.Height+cz] = false
				}
			}
		}
	}
	o.cells = cells
}

// OpenWaterCell reports whether cell (cx, cz) is open water with no
// bridge over it (RefreshOpenWater).
func (t *Terrain) OpenWaterCell(cx, cz int) bool {
	c := t.openWater.cells
	return c != nil && t.InBounds(cx, cz) && c[cx*t.Height+cz]
}

// HasOpenWater reports whether any cell is open water.
func (t *Terrain) HasOpenWater() bool { return t.openWater.cells != nil }

// NearOpenWater reports whether an open-water cell is within r cells of
// world (x, z).
func (t *Terrain) NearOpenWater(x, z float32, r int) bool {
	if t.openWater.cells == nil {
		return false
	}
	cx, cz := int(x/CellSize), int(z/CellSize)
	for i := cx - r; i <= cx+r; i++ {
		for j := cz - r; j <= cz+r; j++ {
			if t.OpenWaterCell(i, j) {
				return true
			}
		}
	}
	return false
}

// OpenWaterOnLine reports whether the straight way from a to b passes
// through an open-water cell, not counting the cells it starts and ends
// in (a guest at the water's edge, or a goal beside it).
func (t *Terrain) OpenWaterOnLine(a, b mgl32.Vec2) bool {
	if t.openWater.cells == nil {
		return false
	}
	ax, az := float64(a[0])/CellSize, float64(a[1])/CellSize
	bx, bz := float64(b[0])/CellSize, float64(b[1])/CellSize
	cx, cz := int(math.Floor(ax)), int(math.Floor(az))
	ex, ez := int(math.Floor(bx)), int(math.Floor(bz))
	// Walk the cells the line passes through (Amanatides–Woo).
	dx, dz := bx-ax, bz-az
	stepX, stepZ := 1, 1
	if dx < 0 {
		stepX = -1
	}
	if dz < 0 {
		stepZ = -1
	}
	next := func(p float64, c, step int, d float64) (float64, float64) {
		if d == 0 {
			return math.Inf(1), math.Inf(1)
		}
		edge := float64(c)
		if step > 0 {
			edge++
		}
		return (edge - p) / d, math.Abs(1 / d)
	}
	tMaxX, tDeltaX := next(ax, cx, stepX, dx)
	tMaxZ, tDeltaZ := next(az, cz, stepZ, dz)
	steps := abs(ex-cx) + abs(ez-cz) + 2
	for n := 0; n < steps; n++ {
		if cx == ex && cz == ez {
			return false
		}
		if tMaxX < tMaxZ {
			cx += stepX
			tMaxX += tDeltaX
		} else {
			cz += stepZ
			tMaxZ += tDeltaZ
		}
		if (cx != ex || cz != ez) && t.OpenWaterCell(cx, cz) {
			return true
		}
	}
	return false
}

// KeepOutOfWater is where a guest moving from from to to ends up: to,
// unless that steps from dry ground into open water, when they slide
// along the bank (to with one axis held), or stay put.
func (t *Terrain) KeepOutOfWater(from, to mgl32.Vec2) mgl32.Vec2 {
	if t.openWater.cells == nil {
		return to
	}
	cell := func(p mgl32.Vec2) bool { return t.OpenWaterCell(int(p[0]/CellSize), int(p[1]/CellSize)) }
	if !cell(to) || cell(from) {
		return to
	}
	if p := (mgl32.Vec2{to[0], from[1]}); !cell(p) {
		return p
	}
	if p := (mgl32.Vec2{from[0], to[1]}); !cell(p) {
		return p
	}
	return from
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
