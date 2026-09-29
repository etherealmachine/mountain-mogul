package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Lodge shell resolution (VISION §8, "Shell resolution"): the exterior is
// a kit of tiles authored in OpenSCAD (models-src/lodge_*.scad), chosen
// from the painted footprint at half-cell resolution.
//
//   - Walls: one tile per half-cell edge between shell and outside.
//     Door cells get door tiles, food-court cells glazed tiles, the rest
//     plain or windowed in a rhythm the style seed picks.
//   - Corners: marching squares over the four half-cells around each
//     grid vertex — one inside is an outer corner, three is an inner one.
//   - Roof: height is the Chebyshev (L∞) distance to the outside, capped.
//     For rectilinear footprints that field is exactly an equal-pitch hip
//     roof, valleys at inside corners included, and across one half-cell
//     tile it takes one of five shapes: flat, slope, hip, valley, saddle.
//
// Tiles are in a canonical frame — outward (or downhill) is +Z — turned
// by Rot about +Y with the renderer's HomogRotate3DY convention.

// Shell kit dimensions. Keep in sync with models-src/lib/lodge_kit.scad.
const (
	ShellTileSize     = CellSize / 2 // 2.5 m: one half-cell
	ShellWallHeight   = float32(5.0)
	ShellRoofRise     = float32(2.1) // per tile of run (~40° pitch)
	ShellRoofMaxLevel = 4            // roof flattens past this many tiles in from the eaves
)

// ShellTileKind selects a tile mesh from the lodge kit.
type ShellTileKind uint8

const (
	TileWall ShellTileKind = iota
	TileWallWindow
	TileWallWindowAlt
	TileWallGlazed
	TileDoor
	TileCornerOuter
	TileCornerInner
	TileRoofFlat
	TileRoofSlope
	TileRoofHip
	TileRoofValley
	TileRoofSaddle
	TileEave
	TileEaveCorner
	TileChimney
	ShellTileKindCount
)

// IsRoof reports whether the tile takes the roof colour.
func (k ShellTileKind) IsRoof() bool {
	switch k {
	case TileRoofFlat, TileRoofSlope, TileRoofHip, TileRoofValley, TileRoofSaddle, TileEave, TileEaveCorner:
		return true
	}
	return false
}

// ShellTile is one placed kit tile. Pos is world X/Z and height above
// the lodge floor.
type ShellTile struct {
	Kind ShellTileKind
	Pos  mgl32.Vec3
	Rot  float32
}

// shellGrid answers inside/outside at half-cell resolution.
type shellGrid struct {
	b *Building
}

func (g shellGrid) in(sx, sz int) bool {
	return g.b.HasCell([2]int{floorDiv2(sx), floorDiv2(sz)})
}

func floorDiv2(v int) int {
	if v < 0 {
		return (v - 1) / 2
	}
	return v / 2
}

// rotFor returns the Y rotation turning canonical +Z onto (dx, dz).
func rotFor(dx, dz int) float32 {
	return float32(math.Atan2(float64(dx), float64(dz)))
}

// ResolveLodgeShell returns the kit tiles for lodge b.
func ResolveLodgeShell(b *Building) []ShellTile {
	if !b.IsShell() {
		return nil
	}
	g := shellGrid{b}
	x0, z0 := b.Cells[0][0]*2, b.Cells[0][1]*2
	x1, z1 := x0, z0
	for _, c := range b.Cells {
		x0, z0 = min(x0, c[0]*2), min(z0, c[1]*2)
		x1, z1 = max(x1, c[0]*2+1), max(z1, c[1]*2+1)
	}
	const T = ShellTileSize
	var tiles []ShellTile
	style := styleHash(b.StyleSeed)
	pattern := int(style % 3)
	altWindows := style&4 != 0

	// Walls and eaves.
	dirs := [4][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}}
	for sx := x0; sx <= x1; sx++ {
		for sz := z0; sz <= z1; sz++ {
			if !g.in(sx, sz) {
				continue
			}
			cell := [2]int{floorDiv2(sx), floorDiv2(sz)}
			for _, d := range dirs {
				if g.in(sx+d[0], sz+d[1]) {
					continue
				}
				pos := mgl32.Vec3{
					(float32(sx) + 0.5 + float32(d[0])*0.5) * T, 0,
					(float32(sz) + 0.5 + float32(d[1])*0.5) * T,
				}
				rot := rotFor(d[0], d[1])
				along := sx
				if d[0] != 0 {
					along = sz
				}
				tiles = append(tiles,
					ShellTile{Kind: wallKind(b, cell, along, pattern, altWindows), Pos: pos, Rot: rot},
					ShellTile{Kind: TileEave, Pos: pos.Add(mgl32.Vec3{0, ShellWallHeight, 0}), Rot: rot})
			}
		}
	}

	// Corners: marching squares over the four half-cells at each vertex.
	for vx := x0; vx <= x1+1; vx++ {
		for vz := z0; vz <= z1+1; vz++ {
			quads := [4][2]int{{-1, -1}, {0, -1}, {-1, 0}, {0, 0}}
			var inside [4]bool
			n := 0
			for i, q := range quads {
				if g.in(vx+q[0], vz+q[1]) {
					inside[i] = true
					n++
				}
			}
			pos := mgl32.Vec3{float32(vx) * T, 0, float32(vz) * T}
			eave := pos.Add(mgl32.Vec3{0, ShellWallHeight, 0})
			for i, q := range quads {
				// Outward diagonal from quadrant q is away from it.
				ox, oz := -(2*q[0] + 1), -(2*q[1] + 1)
				switch {
				case n == 3 && !inside[i]:
					tiles = append(tiles, ShellTile{Kind: TileCornerInner, Pos: pos, Rot: diagRot(-ox, -oz)})
				case (n == 1 || n == 2 && inside[i] && inside[3-i]) && inside[i]:
					r := diagRot(ox, oz)
					tiles = append(tiles,
						ShellTile{Kind: TileCornerOuter, Pos: pos, Rot: r},
						ShellTile{Kind: TileEaveCorner, Pos: eave, Rot: r})
				}
			}
		}
	}

	// Roof.
	levels := map[[2]int]int{}
	level := func(vx, vz int) int {
		k := [2]int{vx, vz}
		if l, ok := levels[k]; ok {
			return l
		}
		l := chebyshevToOutside(g, vx, vz)
		levels[k] = l
		return l
	}
	top := -1
	var tops []mgl32.Vec3
	for sx := x0; sx <= x1; sx++ {
		for sz := z0; sz <= z1; sz++ {
			if !g.in(sx, sz) {
				continue
			}
			// Corner order: (-,-), (+,-), (+,+), (-,+).
			h := [4]int{level(sx, sz), level(sx+1, sz), level(sx+1, sz+1), level(sx, sz+1)}
			m := min(h[0], h[1], h[2], h[3])
			var o [4]bool
			for i := range h {
				o[i] = h[i] > m
			}
			kind, rot := roofTile(o)
			pos := mgl32.Vec3{(float32(sx) + 0.5) * T, ShellWallHeight + float32(m)*ShellRoofRise, (float32(sz) + 0.5) * T}
			tiles = append(tiles, ShellTile{Kind: kind, Pos: pos, Rot: rot})
			if kind == TileRoofSlope || kind == TileRoofFlat {
				if m > top {
					top, tops = m, tops[:0]
				}
				if m == top {
					tops = append(tops, pos)
				}
			}
		}
	}
	if len(b.Cells) >= 4 && len(tops) > 0 {
		p := tops[int(style>>3)%len(tops)]
		tiles = append(tiles, ShellTile{Kind: TileChimney, Pos: p})
	}
	return tiles
}

// wallKind picks the facade tile for a wall edge of shell cell cell.
// along is the half-cell index along the wall, which sets the window
// rhythm: pattern 0 windows every tile, 1 alternates, 2 pairs them.
func wallKind(b *Building, cell [2]int, along, pattern int, alt bool) ShellTileKind {
	switch {
	case b.HasDoor(cell):
		return TileDoor
	case b.HasFoodCourt(cell):
		return TileWallGlazed
	}
	window := TileWallWindow
	if alt {
		window = TileWallWindowAlt
	}
	a := along & 3
	switch pattern {
	case 1:
		if a%2 == 1 {
			return TileWall
		}
	case 2:
		if a == 3 {
			return TileWall
		}
	}
	return window
}

// diagRot returns the rotation turning the canonical (+1, +1) diagonal
// onto (dx, dz).
func diagRot(dx, dz int) float32 {
	switch {
	case dx > 0 && dz > 0:
		return 0
	case dx > 0:
		return math.Pi / 2
	case dz < 0:
		return math.Pi
	}
	return -math.Pi / 2
}

// chebyshevToOutside is the L∞ distance, in tiles, from grid vertex
// (vx, vz) to the nearest outside half-cell, capped at ShellRoofMaxLevel.
func chebyshevToOutside(g shellGrid, vx, vz int) int {
	for k := 0; k < ShellRoofMaxLevel; k++ {
		for x := vx - k - 1; x <= vx+k; x++ {
			for z := vz - k - 1; z <= vz+k; z++ {
				// Only the ring added at radius k is new.
				if k > 0 && x > vx-k-1 && x < vx+k && z > vz-k-1 && z < vz+k {
					continue
				}
				if !g.in(x, z) {
					return k
				}
			}
		}
	}
	return ShellRoofMaxLevel
}

// Canonical roof patterns: which corners, in (-,-), (+,-), (+,+), (-,+)
// order, sit one level above the tile's lowest corner. Slopes fall
// toward +Z, hips peak at (-,-), valleys dip at (+,+), saddles ridge
// from (-,-) to (+,+).
var roofCanon = []struct {
	kind ShellTileKind
	high [4]bool
}{
	{TileRoofSlope, [4]bool{true, true, false, false}},
	{TileRoofHip, [4]bool{true, false, false, false}},
	{TileRoofValley, [4]bool{true, true, false, true}},
	{TileRoofSaddle, [4]bool{true, false, true, false}},
}

// roofTile matches a corner pattern against the canonical tiles under
// each quarter turn.
func roofTile(o [4]bool) (ShellTileKind, float32) {
	if o == [4]bool{} {
		return TileRoofFlat, 0
	}
	corners := [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}
	for _, c := range roofCanon {
		for q := 0; q < 4; q++ {
			rot := float32(q) * math.Pi / 2
			cos, sin := float32(math.Cos(float64(rot))), float32(math.Sin(float64(rot)))
			var got [4]bool
			for i, p := range corners {
				// HomogRotate3DY: (x, z) → (x cos + z sin, −x sin + z cos).
				rx := p[0]*cos + p[1]*sin
				rz := -p[0]*sin + p[1]*cos
				for j, t := range corners {
					if abs32(rx-t[0]) < 0.5 && abs32(rz-t[1]) < 0.5 {
						got[j] = c.high[i]
					}
				}
			}
			if got == o {
				return c.kind, rot
			}
		}
	}
	return TileRoofFlat, 0
}

// ShellFloorY is the lodge floor elevation: the mean ground height over
// the shell (the pad is graded flat when painted).
func (w *World) ShellFloorY(b *Building) float32 {
	if len(b.Cells) == 0 {
		return 0
	}
	var s float32
	for _, c := range b.Cells {
		s += w.Terrain.GroundElevationAt(c[0], c[1])
	}
	return s / float32(len(b.Cells))
}

// ShellPalette returns the wall and roof tints for a style seed.
func ShellPalette(seed uint32) (wall, roof mgl32.Vec3) {
	walls := []mgl32.Vec3{
		{1.00, 0.93, 0.82}, // honey pine
		{0.86, 0.68, 0.52}, // cedar
		{0.95, 0.95, 0.92}, // whitewash
		{0.62, 0.44, 0.33}, // dark timber
	}
	roofs := []mgl32.Vec3{
		{0.56, 0.62, 0.68}, // slate
		{0.78, 0.30, 0.24}, // barn red
		{0.34, 0.52, 0.38}, // forest green
		{0.44, 0.42, 0.44}, // charcoal
	}
	h := styleHash(seed)
	return walls[(h>>12)%uint32(len(walls))], roofs[(h>>16)%uint32(len(roofs))]
}

// styleHash spreads a style seed's bits so neighbouring seeds pick
// unrelated facade and palette variants.
func styleHash(seed uint32) uint32 {
	h := seed*0x9E3779B1 + 0x7F4A7C15
	h ^= h >> 15
	h *= 0x85EBCA6B
	h ^= h >> 13
	return h
}
