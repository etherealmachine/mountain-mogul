package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// SnowKind describes the physical character of a snow layer.
type SnowKind uint8

const (
	KindPowder         SnowKind = iota // cold dry storm; light, deep, floaty
	KindPackedPowder                   // groomed or skied-in; fast and predictable
	KindCement                         // warm storm; dense, wet, heavy
	KindWindSlab                       // wind-consolidated; hollow feel, can shatter
	KindCrust                          // sun/wind surface glaze; breakable, edge-catching
	KindBoilerplate                    // hard frozen surface; very fast, no edge
	KindSlush                          // saturated wet snow; slow, heavy, poor edge
	KindFrozenGranular                 // refrozen slush; icy grains, some texture
	KindCorn                           // spring granular; buttery, fast, great grip
	KindBase                           // compacted season base; firm, dense, not icy
	KindAvalancheDebris               // tumbled runout snow mixed with rock and soil
)

// KindName returns a display name for a snow kind.
func KindName(k SnowKind) string {
	switch k {
	case KindPowder:
		return "Powder"
	case KindPackedPowder:
		return "Packed Powder"
	case KindCement:
		return "Cement"
	case KindWindSlab:
		return "Wind Slab"
	case KindCrust:
		return "Crust"
	case KindBoilerplate:
		return "Boilerplate"
	case KindSlush:
		return "Slush"
	case KindFrozenGranular:
		return "Frozen Granular"
	case KindCorn:
		return "Corn"
	case KindBase:
		return "Base"
	case KindAvalancheDebris:
		return "Debris"
	default:
		return "Snow"
	}
}

// KindDensity returns the relative density for a snow kind.
// Used to convert SWE (accumulation) to visible depth: depth = acc / density.
func KindDensity(k SnowKind) float32 {
	switch k {
	case KindPowder:
		return 0.15
	case KindPackedPowder:
		return 0.50
	case KindCement:
		return 0.65
	case KindWindSlab:
		return 0.55
	case KindCrust:
		return 0.60
	case KindBoilerplate:
		return 0.90
	case KindSlush:
		return 0.55
	case KindFrozenGranular:
		return 0.80
	case KindCorn:
		return 0.52
	case KindBase:
		return 0.75
	case KindAvalancheDebris:
		return 0.65 // dense chunks, similar to cement
	default:
		return 0.50
	}
}

// KindShaderPacked returns the value written into the terrain vertex's packed
// slot for a snow kind. Used by the renderer; not for physics.
func KindShaderPacked(k SnowKind) float32 {
	switch k {
	case KindPowder:
		return 0.05
	case KindPackedPowder:
		return 0.85
	case KindCement:
		return 0.55
	case KindWindSlab:
		return 0.50
	case KindCrust:
		return 0.65
	case KindBoilerplate:
		return 0.95
	case KindSlush:
		return 0.40
	case KindFrozenGranular:
		return 0.80
	case KindCorn:
		return 0.70
	case KindBase:
		return 0.90
	case KindAvalancheDebris:
		return 0.20 // paired with ShaderIce=0.45; ice>packed is the debris shader signal
	default:
		return 0.50
	}
}

// KindShaderIce returns the value written into the terrain vertex's ice slot
// for a snow kind. Drives specular sparkle in the shader.
func KindShaderIce(k SnowKind) float32 {
	switch k {
	case KindWindSlab:
		return 0.10
	case KindCrust:
		return 0.40
	case KindBoilerplate:
		return 0.90
	case KindFrozenGranular:
		return 0.55
	case KindCorn:
		return 0.05
	case KindBase:
		return 0.00
	case KindAvalancheDebris:
		return 0.45 // ice>packed flags debris in the shader (no other kind has this)
	default:
		return 0.00
	}
}

// KindBaseMult returns the base-friction multiplier for a snow kind, applied
// on top of the groomed-corduroy baseline in effectiveFriction.
// Values > 1 mean more drag (slower); < 1 mean less drag (faster).
func KindBaseMult(k SnowKind) float32 {
	switch k {
	case KindPowder:
		return 1.00 // depth gate in effectiveFriction adds drag proportional to depth
	case KindPackedPowder:
		return 1.00
	case KindCement:
		return 1.30
	case KindWindSlab:
		return 0.90
	case KindCrust:
		return 0.80
	case KindBoilerplate:
		return 0.50
	case KindSlush:
		return 1.50
	case KindFrozenGranular:
		return 0.65
	case KindCorn:
		return 0.82
	case KindBase:
		return 0.90
	case KindAvalancheDebris:
		return 1.20 // rough, chunky surface — more drag than groomed snow
	default:
		return 1.00
	}
}

// KindEdgeMult returns the edge-friction multiplier for a snow kind.
// Values < 1 mean less grip; > 1 mean more grip.
func KindEdgeMult(k SnowKind) float32 {
	switch k {
	case KindPowder:
		return 0.60
	case KindPackedPowder:
		return 1.00
	case KindCement:
		return 0.90
	case KindWindSlab:
		return 0.80
	case KindCrust:
		return 0.50
	case KindBoilerplate:
		return 0.15
	case KindSlush:
		return 0.75
	case KindFrozenGranular:
		return 0.35
	case KindCorn:
		return 0.95
	case KindBase:
		return 1.10
	case KindAvalancheDebris:
		return 0.75 // uneven chunks reduce edge control
	default:
		return 1.00
	}
}

// SnowLayer is the active surface snow stratum. Visible depth = Accumulation / KindDensity(Kind).
type SnowLayer struct {
	Accumulation float32  // SWE metres, conserved under kind transitions
	Kind         SnowKind
}

// Avalanche instability thresholds (rise/run, SWE metres).
const (
	AvySlopeMin  = float32(0.55) // below this slope, never releases
	AvySlopeMax  = float32(1.40) // above this, slopeExcess is clamped to 1
	avyMinSWE    = float32(0.05) // minimum top-layer SWE for a cell to be avalanche-prone
	avyLoadScale = float32(0.20) // top-layer SWE that saturates the load factor at 1.0
)

// avyKindMult returns a snow-instability multiplier for the given kind.
func avyKindMult(k SnowKind) float32 {
	switch k {
	case KindWindSlab, KindCrust:
		return 1.5
	case KindFrozenGranular:
		return 1.2
	case KindPowder, KindCement:
		return 1.0
	case KindSlush, KindCorn:
		return 0.8
	default: // PackedPowder, Boilerplate, Base, AvalancheDebris
		return 0.2
	}
}

// Cell represents a single grid cell in the terrain.
//
// Snow has two components: Base is consolidated season-base SWE (always
// KindBase density); Top is the active surface layer whose Kind varies with
// weather, grooming, and skier traffic. When Top melts away, Base is
// promoted to a new KindBase Top. When a new storm's kind differs from Top,
// Top's SWE folds into Base and a new Top begins.
//
// Grooming and MogulSize are surface modifiers: Grooming is set by the
// snowcat fleet and decays under skier traffic; MogulSize grows from
// ungroomed traffic.
//
// SkierTraffic accumulates while skiers cross the cell and resets after a
// kind transition (e.g. Powder → Packed Powder). Decays daily.
//
// AvySnow, AvyMomentum, and AvyTick are transient avalanche-wave state.
// They are not persisted to save files and reset to zero on load.
type Cell struct {
	GroundElevation float32
	Base            float32   // consolidated season-base SWE (metres); always KindBase density
	Top             SnowLayer // active surface; weather and skiing act here
	Grooming        float32   // 0..1; 1 = freshly groomed corduroy
	MogulSize       float32   // 0..1; mogul amplitude (visual + physics roughness)
	SkierTraffic    float32   // accumulated traffic; drives kind transitions

	Passable    bool    // hard structural block (buildings, lift endpoints)
	TreeDensity float32 // 0.0 = clear, 1.0 = dense old-growth
	Slope       float32 // rise/run magnitude; set by Terrain.RecomputeSlopes

	AvySnow     float32 // SWE of snow in transit through this cell (0 = inactive)
	AvyMomentum float32 // wave momentum at this cell
	AvyTick     int     // generation index when this cell entered the active front
}

// Baseline friction coefficients for an ideally groomed packed-powder surface.
// EffectiveFriction scales these by snow kind, grooming state, and mogul size.
const (
	MuKineticBase = float64(0.04) // base kinetic friction
	MuKineticEdge = float64(0.20) // perpendicular (carving) friction
)

// EffectiveFriction returns the (base, edge) kinetic friction coefficients for
// this cell, modulated by snow kind, grooming level, powder depth, and mogul
// size. base is the along-fall-line drag term; edge is the cross-fall brake.
func (c Cell) EffectiveFriction() (base, edge float64) {
	depth := c.VisibleSnowDepth()
	kind := KindPowder
	if top := c.TopLayer(); top != nil {
		kind = top.Kind
	}
	base = MuKineticBase
	edge = MuKineticEdge

	// Grooming: lowers base (glide), raises edge (clean carve).
	base *= 1 - 0.30*float64(c.Grooming)
	edge *= 1 + 0.20*float64(c.Grooming)

	// Kind-based multipliers.
	base *= float64(KindBaseMult(kind))
	edge *= float64(KindEdgeMult(kind))

	// Powder: depth-dependent extra drag.
	if kind == KindPowder && depth > 0.5 {
		depthFactor := float64(depth / 2.5)
		if depthFactor > 1 {
			depthFactor = 1
		}
		base *= 1 + 0.80*depthFactor
		edge *= 1 - 0.50*depthFactor
	}

	// Moguls bleed energy on every bump.
	base *= 1 + 0.60*float64(c.MogulSize)
	edge *= 1 + 0.10*float64(c.MogulSize)

	if base < 0.005 {
		base = 0.005
	}
	if edge < 0.02 {
		edge = 0.02
	}
	return base, edge
}

// InstabilityScore returns an avalanche instability score using the cell's
// cached Slope. Returns ≥1.0 when the cell qualifies for release, 0 when
// stable or below the minimum snow threshold. Requires RecomputeSlopes to
// have been called after any ground-elevation change.
func (c *Cell) InstabilityScore() float32 {
	if c.Slope < AvySlopeMin || c.Top.Accumulation < avyMinSWE {
		return 0
	}
	slopeExcess := (c.Slope - AvySlopeMin) / (AvySlopeMax - AvySlopeMin)
	if slopeExcess > 1 {
		slopeExcess = 1
	}
	kindMult := avyKindMult(c.Top.Kind)
	treeAnchor := c.TreeDensity * 0.4
	return (c.Top.Accumulation / avyLoadScale) * slopeExcess * kindMult * (1 - treeAnchor)
}

// TotalSWE returns the total snow-water-equivalent across both layers (metres).
func (c Cell) TotalSWE() float32 {
	return c.Base + c.Top.Accumulation
}

// VisibleSnowDepth returns the total visible snow column height in metres.
func (c Cell) VisibleSnowDepth() float32 {
	var depth float32
	if c.Top.Accumulation > 0 {
		depth += c.Top.Accumulation / KindDensity(c.Top.Kind)
	}
	if c.Base > 0 {
		depth += c.Base / KindDensity(KindBase)
	}
	return depth
}

// SurfacePacked returns the shader packed value for the surface layer.
func (c Cell) SurfacePacked() float32 {
	if top := c.TopLayer(); top != nil {
		return KindShaderPacked(top.Kind)
	}
	return 0
}

// SurfaceIce returns the shader ice value for the surface layer.
func (c Cell) SurfaceIce() float32 {
	if top := c.TopLayer(); top != nil {
		return KindShaderIce(top.Kind)
	}
	return 0
}

// TopLayer returns a pointer to the active surface layer, or nil if bare ground.
// Invariant: if any snow exists, Top.Accumulation > 0 (Base is only promoted
// to a KindBase Top during melt, never left exposed as a raw float).
func (c *Cell) TopLayer() *SnowLayer {
	if c.Top.Accumulation > 0 {
		return &c.Top
	}
	return nil
}

// GradientAt returns the ground-elevation gradient (∂e/∂x, ∂e/∂z) at cell
// (x, z) using central differences with edge clamping. The magnitude of the
// returned vector is the rise-over-run slope (dimensionless).
func (t *Terrain) GradientAt(x, z int) (gx, gz float32) {
	const cellSize = float32(5.0)
	x0, x1 := x-1, x+1
	if x0 < 0 {
		x0 = 0
	}
	if x1 >= t.Width {
		x1 = t.Width - 1
	}
	z0, z1 := z-1, z+1
	if z0 < 0 {
		z0 = 0
	}
	if z1 >= t.Height {
		z1 = t.Height - 1
	}
	dxRun := float32(x1-x0) * cellSize
	dzRun := float32(z1-z0) * cellSize
	if dxRun > 0 {
		gx = (t.Cells[x1][z].GroundElevation - t.Cells[x0][z].GroundElevation) / dxRun
	}
	if dzRun > 0 {
		gz = (t.Cells[x][z1].GroundElevation - t.Cells[x][z0].GroundElevation) / dzRun
	}
	return gx, gz
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
// (NewTerrain). At KindPowder density (0.15) it yields ~4.3 m of visible
// snow depth — deep mid-season powder. Traffic and grooming compact it.
const DefaultSnowAccumulation = float32(0.64)

// NewTerrain creates a flat terrain with all cells passable and a single
// default Powder layer (SWE=DefaultSnowAccumulation).
func NewTerrain(w, h int) *Terrain {
	cells := make([][]Cell, w)
	for x := 0; x < w; x++ {
		cells[x] = make([]Cell, h)
		for z := 0; z < h; z++ {
			cells[x][z] = Cell{
				Top:      SnowLayer{Accumulation: DefaultSnowAccumulation, Kind: KindBase},
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

// RecomputeSlopes recomputes Cell.Slope for every cell from the current
// ground elevations using central differences. Call this once after any
// batch change to GroundElevation (terrain import, save load, lift grading,
// testbed setup). The sim's avalanche logic reads Cell.Slope directly.
func (t *Terrain) RecomputeSlopes() {
	for x := range t.Cells {
		for z := range t.Cells[x] {
			gx, gz := t.GradientAt(x, z)
			t.Cells[x][z].Slope = float32(math.Sqrt(float64(gx*gx + gz*gz)))
		}
	}
}

// InBounds returns true if the given grid coordinates are within the terrain.
func (t *Terrain) InBounds(x, z int) bool {
	return x >= 0 && x < t.Width && z >= 0 && z < t.Height
}

// InBoundsWorld returns true if the given world-space XZ point falls within
// the terrain grid.
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
// at the given grid cell.
func (t *Terrain) SurfaceElevationAt(x, z int) float32 {
	if !t.InBounds(x, z) {
		return 0
	}
	return t.Cells[x][z].SurfaceElevation()
}

// InterpolatedSurfaceElevationAt returns the bilinearly-interpolated
// snow-surface elevation at a world-space position.
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
// ground elevation (no snow) at a world-space position.
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

// SnowAt returns the snow-state at the given world-space XZ point.
// depth is the visible snow column height (metres). kind is the top layer's
// snow kind. Out-of-bounds returns groomed-PackedPowder defaults.
func (t *Terrain) SnowAt(wx, wz float32) (depth, grooming float32, kind SnowKind, mogul float32) {
	const cellSize = float32(5.0)
	xi := int(wx / cellSize)
	zi := int(wz / cellSize)
	if !t.InBounds(xi, zi) {
		return DefaultSnowAccumulation / KindDensity(KindPackedPowder), 1, KindPackedPowder, 0
	}
	c := t.Cells[xi][zi]
	k := KindPowder
	if top := c.TopLayer(); top != nil {
		k = top.Kind
	}
	return c.VisibleSnowDepth(), c.Grooming, k, c.MogulSize
}

// NormalAt returns the snow-surface normal at the given (continuous)
// grid position by bilinear-sampling the surface elevation of neighbouring
// cells.
func (t *Terrain) NormalAt(x, z float32) mgl32.Vec3 {
	const cellSize = float32(5.0)

	xi := int(x)
	zi := int(z)

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

	e00 := t.SurfaceElevationAt(xi, zi)
	e10 := t.SurfaceElevationAt(xi+1, zi)
	e01 := t.SurfaceElevationAt(xi, zi+1)

	tx := mgl32.Vec3{cellSize, e10 - e00, 0.0}
	tz := mgl32.Vec3{0.0, e01 - e00, cellSize}

	normal := tz.Cross(tx)
	return normal.Normalize()
}
