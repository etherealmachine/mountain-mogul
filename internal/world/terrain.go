package world

import (
	"github.com/go-gl/mathgl/mgl32"
)

// Cell represents a single grid cell in the terrain.
type Cell struct {
	Elevation   float32
	Passable    bool    // hard structural block (buildings, lift endpoints)
	Groomed     bool
	SnowDepth   float32
	TreeDensity float32 // 0.0 = clear, 1.0 = dense old-growth
}

// Walkable returns true if an agent on foot can enter this cell.
// Dense forest (density >= 0.5) is treated as impenetrable on foot;
// hard structural obstacles (Passable == false) always block.
func (c Cell) Walkable() bool {
	return c.Passable && c.TreeDensity < 0.5
}

// Terrain is the heightmap grid used for both simulation and rendering.
type Terrain struct {
	Width, Height int
	Cells         [][]Cell // [x][z]
}

// NewTerrain creates a flat terrain with all cells passable and full snow depth.
func NewTerrain(w, h int) *Terrain {
	cells := make([][]Cell, w)
	for x := 0; x < w; x++ {
		cells[x] = make([]Cell, h)
		for z := 0; z < h; z++ {
			cells[x][z] = Cell{
				Elevation: 0,
				Passable:  true,
				SnowDepth: 1.0,
			}
		}
	}
	return &Terrain{
		Width:  w,
		Height: h,
		Cells:  cells,
	}
}

// InBounds returns true if the given grid coordinates are within the terrain.
func (t *Terrain) InBounds(x, z int) bool {
	return x >= 0 && x < t.Width && z >= 0 && z < t.Height
}

// ElevationAt returns the elevation at the given grid cell.
func (t *Terrain) ElevationAt(x, z int) float32 {
	if !t.InBounds(x, z) {
		return 0
	}
	return t.Cells[x][z].Elevation
}

// InterpolatedElevationAt returns the bilinearly-interpolated terrain elevation
// at a world-space position. Smoother than ElevationAt for continuous motion.
func (t *Terrain) InterpolatedElevationAt(wx, wz float32) float32 {
	const cellSize = float32(10.0)
	xi := int(wx / cellSize)
	zi := int(wz / cellSize)
	if xi < 0 {
		xi = 0
	}
	if xi >= t.Width-1 {
		xi = t.Width - 2
	}
	if zi < 0 {
		zi = 0
	}
	if zi >= t.Height-1 {
		zi = t.Height - 2
	}
	fx := wx/cellSize - float32(xi)
	fz := wz/cellSize - float32(zi)
	if fx < 0 {
		fx = 0
	}
	if fx > 1 {
		fx = 1
	}
	if fz < 0 {
		fz = 0
	}
	if fz > 1 {
		fz = 1
	}
	e00 := t.ElevationAt(xi, zi)
	e10 := t.ElevationAt(xi+1, zi)
	e01 := t.ElevationAt(xi, zi+1)
	e11 := t.ElevationAt(xi+1, zi+1)
	return (1-fz)*((1-fx)*e00+fx*e10) + fz*((1-fx)*e01+fx*e11)
}

// TreeDensityAt returns the tree density at the given world-space XZ point
// using nearest-cell sampling. Out-of-bounds returns 0 (clear).
func (t *Terrain) TreeDensityAt(wx, wz float32) float32 {
	const cellSize = float32(10.0)
	xi := int(wx / cellSize)
	zi := int(wz / cellSize)
	if !t.InBounds(xi, zi) {
		return 0
	}
	return t.Cells[xi][zi].TreeDensity
}

// NormalAt returns the surface normal at the given (continuous) grid position
// by bilinear-sampling the elevation of neighboring cells.
func (t *Terrain) NormalAt(x, z float32) mgl32.Vec3 {
	xi := int(x)
	zi := int(z)

	// clamp to valid range
	if xi < 0 {
		xi = 0
	}
	if xi >= t.Width-1 {
		xi = t.Width - 2
	}
	if zi < 0 {
		zi = 0
	}
	if zi >= t.Height-1 {
		zi = t.Height - 2
	}

	// sample elevations around the point
	e00 := t.ElevationAt(xi, zi)
	e10 := t.ElevationAt(xi+1, zi)
	e01 := t.ElevationAt(xi, zi+1)

	// approximate normal using cross products of grid tangents
	// tangent in X direction
	tx := mgl32.Vec3{10.0, e10 - e00, 0.0}
	// tangent in Z direction
	tz := mgl32.Vec3{0.0, e01 - e00, 10.0}

	normal := tz.Cross(tx)
	return normal.Normalize()
}
