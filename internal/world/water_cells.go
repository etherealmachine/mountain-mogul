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
// bridge carries them over). So is ground built on, and builtMargin
// cells round it: lots, buildings, roads and footpaths stand on solid
// ground, whatever a lake outline drawn under them says (on the Boreal
// Goals Test a lake reached under a lot, and the thaw of its shallows on
// the second day cut the base off). The grid is kept up to date by
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
	built      uint64 // builtSig when made
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
	sig := w.builtSig()
	if o.made && o.material == m && (m == nil || o.version == m.Version) && o.bridgesRev == w.BridgesRev && o.built == sig {
		return
	}
	o.made, o.material, o.bridgesRev, o.built = true, m, w.BridgesRev, sig
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
	// Built-on ground is solid.
	open := func(cx, cz int) {
		for x := cx - builtMargin; x <= cx+builtMargin; x++ {
			for z := cz - builtMargin; z <= cz+builtMargin; z++ {
				if t.InBounds(x, z) {
					cells[x*t.Height+z] = false
				}
			}
		}
	}
	for _, b := range w.Buildings {
		if len(b.Ground) > 0 {
			for _, c := range b.Ground {
				open(c[0], c[1])
			}
			continue
		}
		x0, z0, x1, z1 := b.Footprint().Bounds()
		for x := int(x0 / CellSize); x <= int(x1/CellSize); x++ {
			for z := int(z0 / CellSize); z <= int(z1/CellSize); z++ {
				open(x, z)
			}
		}
	}
	for _, n := range w.RoadNodes {
		open(int(n.Pos[0]/CellSize), int(n.Pos[1]/CellSize))
	}
	for _, c := range w.FindRoadChains() {
		for _, p := range SampleRoadChain(c, t, RoadChainSamplesPerSegment) {
			open(int(p[0]/CellSize), int(p[1]/CellSize))
		}
	}
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			if cells[x*t.Height+z] && w.FootpathCell(x, z) {
				cells[x*t.Height+z] = false
			}
		}
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

// builtMargin is how many cells round ground built on are solid too:
// the door steps, the lot's edge, a road's shoulders.
const builtMargin = 2

// builtSig changes when the buildings, roads or footpaths do, so the
// open-water grid is remade (RefreshOpenWater).
func (w *World) builtSig() uint64 {
	h := uint64(len(w.Buildings))*1_000_003 + uint64(len(w.RoadEdges))*7_919 + uint64(w.FootpathsRev)*104_729
	for _, b := range w.Buildings {
		h = h*31 + b.ID + uint64(len(b.Ground))
	}
	for _, n := range w.RoadNodes {
		h = h*31 + n.ID + uint64(n.Pos[0]*8) + uint64(n.Pos[1]*8)<<20
	}
	return h
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
