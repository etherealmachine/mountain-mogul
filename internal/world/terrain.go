package world

import (
	"github.com/go-gl/mathgl/mgl32"
)

// Cell represents a single grid cell in the terrain.
//
// The terrain is two layers stacked: ground (rock/dirt the player almost
// never edits) and a snow column on top whose accumulation and packing
// state evolve over time and which the player manipulates via grooming,
// snowmaking, etc.
//
// SnowAccumulation is the snow-water-equivalent of the column — the
// conserved quantity. Packing changes the density of that column but
// doesn't change how much "snow material" is there. The visible depth
// of the snow surface is therefore a derived quantity:
//
//	VisibleSnowDepth = SnowAccumulation / density(Packed)
//	SurfaceElevation = GroundElevation + VisibleSnowDepth
//
// where density runs from ~0.32 at fresh powder (Packed=0.2) to 1.0 at
// fully compacted (Packed=1.0). So a snowcat passing over fresh powder
// raises Packed without touching SnowAccumulation; the visible surface
// drops on its own as density rises. Same SWE, less depth — exactly how
// real snow compresses under traffic.
//
// Things that root in the earth (trees, building foundations) sit at
// GroundElevation. Skiers, the visible terrain mesh, lift cables, and
// snow-cat traffic operate on the snow surface.
//
// Snow type is represented by independent scalars rather than a discrete
// enum so transitions and gradients are continuous. "Type" labels (powder,
// corduroy, moguls, ice) are emergent from these fields:
//
//   - Grooming high + Packed moderate + MogulSize low → corduroy
//   - SnowAccumulation high + Packed low              → powder
//   - Grooming low + traffic over time                → MogulSize grows
//   - Ice high + Accumulation low                     → boilerplate
type Cell struct {
	GroundElevation  float32 // metres; the rock/dirt under the snow
	SnowAccumulation float32 // metres SWE; conserved under packing
	Grooming         float32 // 0..1; 1 = freshly groomed corduroy
	Packed           float32 // 0..1; density of the snow column (0 = fluffy powder, 1 = bulletproof packed)
	Ice              float32 // 0..1; ice fraction at the surface
	MogulSize        float32 // 0..1; mogul amplitude (visual + physics roughness)

	Passable    bool    // hard structural block (buildings, lift endpoints)
	TreeDensity float32 // 0.0 = clear, 1.0 = dense old-growth
}

// SnowDensity returns the relative density of a packed-snow column.
// 0.15 at Packed=0 (fresh fluff floor — even unpacked snow has some
// minimum cohesion) up to 1.0 at Packed=1.0 (bulletproof piste).
// Used by VisibleSnowDepth and by anywhere that needs to convert
// between SWE and visible depth.
func SnowDensity(packed float32) float32 {
	return 0.15 + 0.85*packed
}

// VisibleSnowDepth returns the height of the snow column above ground
// in metres, derived from accumulation and packing. Read this anywhere
// you need the physical depth on the surface (rendering, mogul-formation
// gating, agent collision); write to SnowAccumulation / Packed instead.
func (c Cell) VisibleSnowDepth() float32 {
	d := SnowDensity(c.Packed)
	if d <= 0 {
		return 0
	}
	return c.SnowAccumulation / d
}

// SurfaceElevation returns the snow-surface elevation for this cell
// (ground + visible snow column height).
func (c Cell) SurfaceElevation() float32 {
	return c.GroundElevation + c.VisibleSnowDepth()
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

	// SnowDirty signals to the renderer that a cell's snow state
	// (Grooming/Packed/Ice/MogulSize/SnowAccumulation) has changed and
	// the terrain VBO needs a re-upload. The sim sets this; the scene
	// flushes the mesh and clears the flag once per frame. We keep
	// rebuilds to one per frame max even when many cells change in a
	// single tick — the whole-mesh upload is the same cost.
	SnowDirty bool
}

// DefaultSnowAccumulation is the baseline SWE applied to fresh terrain
// (NewTerrain). At the also-default Packed=0.2 (fresh powder density
// ~0.32) it yields ~2.0 m of visible snow depth — roughly mid-season
// at a typical resort.
const DefaultSnowAccumulation = float32(0.64)

// NewTerrain creates a flat terrain with all cells passable, default
// snow accumulation, and lightly-packed (fresh-powder) snow. Skier
// traffic and snowcat grooming both raise Packed under conserved
// accumulation, so starting near zero gives the most room for visible
// compression as the resort gets tracked out.
func NewTerrain(w, h int) *Terrain {
	cells := make([][]Cell, w)
	for x := 0; x < w; x++ {
		cells[x] = make([]Cell, h)
		for z := 0; z < h; z++ {
			cells[x][z] = Cell{
				GroundElevation:  0,
				SnowAccumulation: DefaultSnowAccumulation,
				Packed:           0.2,
				Passable:         true,
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

// InBoundsWorld returns true if the given world-space XZ point falls within
// the terrain grid. Used by the skier controller to apply a boundary penalty
// to candidate trajectories that would leave the map.
func (t *Terrain) InBoundsWorld(wx, wz float32) bool {
	const cellSize = float32(5.0)
	maxX := float32(t.Width) * cellSize
	maxZ := float32(t.Height) * cellSize
	return wx >= 0 && wx < maxX && wz >= 0 && wz < maxZ
}

// GroundElevationAt returns the ground elevation (no snow) at the given
// grid cell. Use for tree rooting and building foundations.
func (t *Terrain) GroundElevationAt(x, z int) float32 {
	if !t.InBounds(x, z) {
		return 0
	}
	return t.Cells[x][z].GroundElevation
}

// SurfaceElevationAt returns the snow-surface elevation (ground + snow)
// at the given grid cell. Use for skiers, visible mesh, lift cable
// endpoints, lift station meshes.
func (t *Terrain) SurfaceElevationAt(x, z int) float32 {
	if !t.InBounds(x, z) {
		return 0
	}
	return t.Cells[x][z].SurfaceElevation()
}

// InterpolatedSurfaceElevationAt returns the bilinearly-interpolated
// snow-surface elevation at a world-space position. Smoother than the
// per-cell lookup for continuous motion (skier physics).
func (t *Terrain) InterpolatedSurfaceElevationAt(wx, wz float32) float32 {
	const cellSize = float32(5.0)
	xi, zi, fx, fz := t.bilinearIndices(wx, wz, cellSize)
	e00 := t.SurfaceElevationAt(xi, zi)
	e10 := t.SurfaceElevationAt(xi+1, zi)
	e01 := t.SurfaceElevationAt(xi, zi+1)
	e11 := t.SurfaceElevationAt(xi+1, zi+1)
	return (1-fz)*((1-fx)*e00+fx*e10) + fz*((1-fx)*e01+fx*e11)
}

// InterpolatedGroundElevationAt returns the bilinearly-interpolated
// ground elevation (no snow) at a world-space position. Currently only
// used by lift cable layout if/when the cable should clear ground rather
// than the snow surface; tree placement uses cell-aligned ground lookup.
func (t *Terrain) InterpolatedGroundElevationAt(wx, wz float32) float32 {
	const cellSize = float32(5.0)
	xi, zi, fx, fz := t.bilinearIndices(wx, wz, cellSize)
	e00 := t.GroundElevationAt(xi, zi)
	e10 := t.GroundElevationAt(xi+1, zi)
	e01 := t.GroundElevationAt(xi, zi+1)
	e11 := t.GroundElevationAt(xi+1, zi+1)
	return (1-fz)*((1-fx)*e00+fx*e10) + fz*((1-fx)*e01+fx*e11)
}

func (t *Terrain) bilinearIndices(wx, wz, cellSize float32) (xi, zi int, fx, fz float32) {
	xi = int(wx / cellSize)
	zi = int(wz / cellSize)
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
	fx = wx/cellSize - float32(xi)
	fz = wz/cellSize - float32(zi)
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
	return xi, zi, fx, fz
}

// TreeDensityAt returns the tree density at the given world-space XZ point
// using nearest-cell sampling. Out-of-bounds returns 0 (clear).
func (t *Terrain) TreeDensityAt(wx, wz float32) float32 {
	const cellSize = float32(5.0)
	xi := int(wx / cellSize)
	zi := int(wz / cellSize)
	if !t.InBounds(xi, zi) {
		return 0
	}
	return t.Cells[xi][zi].TreeDensity
}

// SnowAt returns the snow-state scalars at the given world-space XZ
// point. `depth` is the visible snow-column height (derived from
// accumulation and packing), not the raw SWE. Used by skier physics
// to derive effective friction. Out-of-bounds returns groomed-corduroy
// defaults.
func (t *Terrain) SnowAt(wx, wz float32) (depth, grooming, packed, ice, mogul float32) {
	const cellSize = float32(5.0)
	xi := int(wx / cellSize)
	zi := int(wz / cellSize)
	if !t.InBounds(xi, zi) {
		// At-bounds default mirrors a freshly-groomed apron: full SWE
		// converted to depth at Packed=0.5 density.
		return DefaultSnowAccumulation / SnowDensity(0.5), 1, 0.5, 0, 0
	}
	c := t.Cells[xi][zi]
	return c.VisibleSnowDepth(), c.Grooming, c.Packed, c.Ice, c.MogulSize
}

// NormalAt returns the snow-surface normal at the given (continuous)
// grid position by bilinear-sampling the surface elevation of neighbouring
// cells. Skiers ski on the snow, so the surface normal — not the ground
// normal — is what drives gravity-along-fall-line and the carving terms.
func (t *Terrain) NormalAt(x, z float32) mgl32.Vec3 {
	const cellSize = float32(5.0)

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

	// sample surface elevations around the point
	e00 := t.SurfaceElevationAt(xi, zi)
	e10 := t.SurfaceElevationAt(xi+1, zi)
	e01 := t.SurfaceElevationAt(xi, zi+1)

	// approximate normal using cross products of grid tangents.
	// Horizontal step is one cell = cellSize world units; using anything else
	// distorts the slope angle returned to skier physics.
	tx := mgl32.Vec3{cellSize, e10 - e00, 0.0}
	tz := mgl32.Vec3{0.0, e01 - e00, cellSize}

	normal := tz.Cross(tx)
	return normal.Normalize()
}
