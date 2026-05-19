package world

import (
	"github.com/go-gl/mathgl/mgl32"
)

// LayerKind describes how a snow layer was deposited.
type LayerKind uint8

const (
	LayerFreshSnow LayerKind = iota // dry cold storm; low initial density
	LayerWetSnow                    // warm storm or rain-on-snow; higher density
	LayerIceRain                    // rain + freeze cycle; high ice fraction
	LayerWindSlab                   // wind-packed / consolidated slab
)

// LayerKindName returns a display name for a layer kind.
func LayerKindName(k LayerKind) string {
	switch k {
	case LayerFreshSnow:
		return "Fresh Snow"
	case LayerWetSnow:
		return "Wet Snow"
	case LayerIceRain:
		return "Ice Rain"
	case LayerWindSlab:
		return "Wind Slab"
	default:
		return "Snow"
	}
}

// SnowLayer is one depositional event in a cell's snow column.
// Layers are stored oldest-first (index 0 = deepest/oldest; last = surface).
// Each layer conserves its own SWE under packing: visible depth = Accumulation / density(Packed).
type SnowLayer struct {
	Accumulation float32   // SWE metres, conserved under packing
	Packed       float32   // 0..1 column density (0=fluffy powder, 1=bulletproof)
	Ice          float32   // 0..1 ice fraction in this layer
	Kind         LayerKind
}

// Cell represents a single grid cell in the terrain.
//
// Snow is stored as a stack of SnowLayer values, oldest at index 0, newest
// (surface) last. This represents the depositional history: one layer per
// storm or weather event. Skier traffic and grooming affect only the surface
// (top) layer. Snowfall pushes new layers.
//
// Grooming and MogulSize are cell-level surface modifiers that sit above the
// layer stack: Grooming is set by the snowcat fleet and decays under skier
// traffic; MogulSize grows from ungroomed traffic.
//
// Derived surface properties (SurfaceIce, SurfacePacked, VisibleSnowDepth)
// are computed from the layer stack. All physics and rendering read these
// derived values rather than individual layer fields.
type Cell struct {
	GroundElevation float32
	Layers          []SnowLayer // [0]=oldest/deepest, [len-1]=surface
	Grooming        float32     // 0..1; 1 = freshly groomed corduroy
	MogulSize       float32     // 0..1; mogul amplitude (visual + physics roughness)

	Passable    bool    // hard structural block (buildings, lift endpoints)
	TreeDensity float32 // 0.0 = clear, 1.0 = dense old-growth
}

// SnowDensity returns the relative density of a packed-snow column.
// 0.15 at Packed=0 (fresh fluff floor) up to 1.0 at Packed=1.0 (bulletproof piste).
func SnowDensity(packed float32) float32 {
	return 0.15 + 0.85*packed
}

// TotalSWE returns the total snow-water-equivalent across all layers (metres).
func (c Cell) TotalSWE() float32 {
	var total float32
	for _, l := range c.Layers {
		total += l.Accumulation
	}
	return total
}

// VisibleSnowDepth returns the total visible snow column height in metres,
// summing per-layer depth (each layer's accumulation / its density).
func (c Cell) VisibleSnowDepth() float32 {
	var depth float32
	for _, l := range c.Layers {
		d := SnowDensity(l.Packed)
		if d > 0 {
			depth += l.Accumulation / d
		}
	}
	return depth
}

// SurfacePacked returns the Packed value of the top (surface) layer, or 0 if there are no layers.
func (c Cell) SurfacePacked() float32 {
	if len(c.Layers) == 0 {
		return 0
	}
	return c.Layers[len(c.Layers)-1].Packed
}

// SurfaceIce returns the Ice value of the top (surface) layer, or 0 if there are no layers.
func (c Cell) SurfaceIce() float32 {
	if len(c.Layers) == 0 {
		return 0
	}
	return c.Layers[len(c.Layers)-1].Ice
}

// TopLayer returns a pointer to the surface layer, or nil if there are no layers.
func (c *Cell) TopLayer() *SnowLayer {
	if len(c.Layers) == 0 {
		return nil
	}
	return &c.Layers[len(c.Layers)-1]
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
	// (Layers/Grooming/MogulSize) has changed and the terrain VBO needs
	// a re-upload. The sim sets this; the scene flushes the mesh and
	// clears the flag once per frame.
	SnowDirty bool

	// Surface is a 1 m-resolution RGBA8 buffer mirroring the cell grid,
	// carrying sub-cell features the 5 m mesh can't (skier tracks,
	// tree wells, groom edges). See world/surface_detail.go.
	Surface *SurfaceDetail
}

// DefaultSnowAccumulation is the baseline SWE applied to fresh terrain
// (NewTerrain). At the also-default Packed=0.2 (fresh powder density
// ~0.32) it yields ~2.0 m of visible snow depth — roughly mid-season
// at a typical resort.
const DefaultSnowAccumulation = float32(0.64)

// NewTerrain creates a flat terrain with all cells passable and a single
// default fresh-snow layer (SWE=DefaultSnowAccumulation, Packed=0.2).
// Skier traffic and snowcat grooming raise the top layer's Packed under
// conserved SWE, so starting near zero gives the most room for visible
// compression as the resort gets tracked out.
func NewTerrain(w, h int) *Terrain {
	cells := make([][]Cell, w)
	for x := 0; x < w; x++ {
		cells[x] = make([]Cell, h)
		for z := 0; z < h; z++ {
			cells[x][z] = Cell{
				Layers: []SnowLayer{{
					Accumulation: DefaultSnowAccumulation,
					Packed:       0.2,
					Kind:         LayerFreshSnow,
				}},
				Passable: true,
			}
		}
	}
	return &Terrain{
		Width:   w,
		Height:  h,
		Cells:   cells,
		Surface: NewSurfaceDetail(w, h),
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
	return c.VisibleSnowDepth(), c.Grooming, c.SurfacePacked(), c.SurfaceIce(), c.MogulSize
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
