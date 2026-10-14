package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// SnowKind describes the physical character of a snow layer.
type SnowKind uint8

const (
	KindPowder          SnowKind = iota // cold dry storm; light, deep, floaty
	KindPackedPowder                    // groomed or skied-in; fast and predictable
	KindCement                          // warm storm; dense, wet, heavy
	KindWindSlab                        // wind-consolidated; hollow feel, can shatter
	KindCrust                           // sun/wind surface glaze; breakable, edge-catching
	KindBoilerplate                     // hard frozen surface; very fast, no edge
	KindSlush                           // saturated wet snow; slow, heavy, poor edge
	KindFrozenGranular                  // refrozen slush; icy grains, some texture
	KindCorn                            // spring granular; buttery, fast, great grip
	KindBase                            // compacted season base; firm, dense, not icy
	KindAvalancheDebris                 // tumbled runout snow mixed with rock and soil
	KindMachine                         // man-made: dense, firm, durable (snowmaking.go)
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
	case KindMachine:
		return "Machine Made"
	default:
		return "Snow"
	}
}

// KindDensity is the density snow of kind k is laid down at (fresh
// snowfall, machine snow), and the guess for a layer saved before layers
// stored their own. A layer's density after that is its own: it settles
// day by day (SettleSnow) and cats pack it (GroomCompact), but changing
// its kind (wind, sun, skiers) doesn't change it.
func KindDensity(k SnowKind) float32 {
	switch k {
	case KindPowder:
		return 0.12
	case KindPackedPowder:
		return 0.40
	case KindCement:
		return 0.40
	case KindWindSlab:
		return 0.35
	case KindCrust:
		return 0.35
	case KindBoilerplate:
		return 0.45
	case KindSlush:
		return 0.50
	case KindFrozenGranular:
		return 0.45
	case KindCorn:
		return 0.45
	case KindBase:
		return BaseDensityDefault
	case KindAvalancheDebris:
		return 0.50 // tumbled chunks
	case KindMachine:
		return 0.45 // small round grains, packed tight
	default:
		return 0.40
	}
}

// Snow settles under its own weight: each day a layer's density closes
// a share of the gap to where it settles to, faster on a warm day. Fresh
// powder at 0.12 is about 0.27 after a cold week; the season base
// settles toward 0.45 (real midwinter snowpacks are 0.30 to 0.45).
const (
	BaseDensityDefault = float32(0.40) // a base with none stored
	topSettledDensity  = float32(0.38)
	baseSettledDensity = float32(0.45)
	settleColdPerDay   = float32(0.12)
	settleWarmPerDay   = float32(0.25)
	// A cat packs the top GroomPackDepth metres of snow to
	// GroomPackDensity.
	GroomPackDepth   = float32(0.35)
	GroomPackDensity = float32(0.45)
)

// EffDensity is the layer's density: its own, or for a layer with none
// stored, its kind's (KindDensity).
func (l SnowLayer) EffDensity() float32 {
	if l.Density > 0 {
		return l.Density
	}
	return KindDensity(l.Kind)
}

// SetKind changes the layer's kind, keeping its density.
func (l *SnowLayer) SetKind(k SnowKind) {
	l.Density = l.EffDensity()
	l.Kind = k
}

// AddSnow lays swe metres of snow at density on the layer, the two
// densities blending by water.
func (l *SnowLayer) AddSnow(swe, density float32) {
	l.Density = blendDensity(l.Accumulation, l.EffDensity(), swe, density)
	l.Accumulation += swe
}

// blendDensity is the density of a metres of water at density da and b
// at db together: total water over total depth.
func blendDensity(a, da, b, db float32) float32 {
	if a <= 0 {
		return db
	}
	if b <= 0 {
		return da
	}
	return (a + b) / (a/da + b/db)
}

// BaseEffDensity is the season base's density: its own, or
// BaseDensityDefault for one with none stored.
func (c *Cell) BaseEffDensity() float32 {
	if c.BaseDensity > 0 {
		return c.BaseDensity
	}
	return BaseDensityDefault
}

// AddToBase folds swe metres of snow at density into the season base.
func (c *Cell) AddToBase(swe, density float32) {
	c.BaseDensity = blendDensity(c.Base, c.BaseEffDensity(), swe, density)
	c.Base += swe
}

// FoldTopIntoBase moves the whole top layer into the season base, which
// takes its density as it is: snow isn't packed by being buried.
func (c *Cell) FoldTopIntoBase() {
	if c.Top.Accumulation > 0 {
		c.AddToBase(c.Top.Accumulation, c.Top.EffDensity())
	}
	c.Top = SnowLayer{}
}

// SettleSnow is a day of settling: the top layer and the base close a
// share of the gap to their settled densities (none for a layer already
// denser: ice, slush and the like stay as they are).
func (c *Cell) SettleSnow(warm bool) {
	k := settleColdPerDay
	if warm {
		k = settleWarmPerDay
	}
	if c.Top.Accumulation > 0 {
		if d := c.Top.EffDensity(); d < topSettledDensity {
			c.Top.Density = d + (topSettledDensity-d)*k
		} else {
			c.Top.Density = d
		}
	}
	if c.Base > 0 {
		if d := c.BaseEffDensity(); d < baseSettledDensity {
			c.BaseDensity = d + (baseSettledDensity-d)*k
		} else {
			c.BaseDensity = d
		}
	}
}

// GroomCompact is a cat's pass over the top layer: its top
// GroomPackDepth metres packed to GroomPackDensity, the snow under that
// as it was. A metre of fresh powder grooms down to about 70 cm.
func (c *Cell) GroomCompact() {
	l := &c.Top
	d := l.EffDensity()
	if l.Accumulation <= 0 || d >= GroomPackDensity {
		return
	}
	depth := l.Accumulation / d
	packed := min(depth, GroomPackDepth)
	newDepth := depth - packed + packed*d/GroomPackDensity
	l.Density = l.Accumulation / newDepth
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
	case KindMachine:
		return 0.80
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
	case KindMachine:
		return 0.05
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
	case KindMachine:
		return 0.95 // firm and fast, a touch grabby until it's groomed
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
	case KindMachine:
		return 1.05
	default:
		return 1.00
	}
}

// SnowLayer is the active surface snow stratum. Visible depth =
// Accumulation / EffDensity.
type SnowLayer struct {
	Accumulation float32 // SWE metres, conserved under kind transitions
	Kind         SnowKind
	// Density is the layer's own (water over depth); 0 for its kind's
	// (EffDensity).
	Density float32
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
// Snow has two components: Base is consolidated season-base SWE, at
// BaseDensity; Top is the active surface layer whose Kind varies with
// weather, grooming, and skier traffic. When Top melts away, Base is
// promoted to a new KindBase Top. When a new storm's kind differs from Top,
// Top folds into Base (at its own density) and a new Top begins. Both
// settle day by day (SettleSnow).
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
	Base            float32   // consolidated season-base SWE (metres)
	BaseDensity     float32   // the base's density; 0 for BaseDensityDefault
	Top             SnowLayer // active surface; weather and skiing act here
	Grooming        float32   // 0..1; 1 = freshly groomed corduroy
	MogulSize       float32   // 0..1; mogul amplitude (visual + physics roughness)
	SkierTraffic    float32   // accumulated traffic; drives kind transitions

	Passable  bool    // hard structural block (buildings, lift endpoints)
	TreeCount uint8   // stored trees in this cell; kept in step by Terrain's tree methods (trees.go)
	Slope     float32 // rise/run magnitude; set by Terrain.RecomputeSlopes

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
	treeAnchor := c.TreeCover() * 0.4
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
		depth += c.Top.Accumulation / c.Top.EffDensity()
	}
	if c.Base > 0 {
		depth += c.Base / c.BaseEffDensity()
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

// WalkBlockTrees is how many trunks in one cell make it impassable on
// foot. A single trunk leaves room to walk around it.
const WalkBlockTrees = 2

// Walkable returns true if an agent on foot can enter this cell.
// A cell with WalkBlockTrees or more trunks is treated as impenetrable
// on foot; hard structural obstacles (Passable == false) always block.
func (c Cell) Walkable() bool {
	return c.Passable && c.TreeCount < WalkBlockTrees
}

// Terrain is the heightmap grid used for both simulation and rendering.
type Terrain struct {
	Width, Height int
	Cells         [][]Cell // [x][z]

	// snowDirty tracks which tiles' snow state (layers, grooming,
	// moguls) changed since the renderer's last flush: writers call
	// MarkSnowDirty for the cells they touch, or MarkAllSnowDirty; the
	// scene flushes those tiles and clears the marks once per frame
	// (snow_dirty.go).
	snowDirty snowDirty

	// trunks caches each trunk's steering hazard at 1 m (trunk_field.go).
	trunks trunkField

	// Surface is a 1 m-resolution RGBA8 buffer mirroring the cell grid,
	// carrying sub-cell features the 5 m mesh can't (skier tracks,
	// tree wells, groom edges). See world/surface_detail.go.
	Surface *SurfaceDetail

	// Groom records where snowcats groomed, at 1 m. See groom_map.go.
	Groom *GroomMap

	// Moguls is how big the moguls are at each metre; each cell's
	// MogulSize is its average (mogul_map.go).
	Moguls *MogulMap

	// Detail is sub-cell height detail drawn on top of the 5 m mesh, or
	// nil for none. See terrain_detail.go.
	Detail *TerrainDetail

	// Material is what the ground is made of, on the detail lattice, or
	// nil when nothing has set it. See material.go.
	Material *TerrainMaterial
	// openWater is the per-cell open-water grid (water_cells.go).
	openWater openWaterCells

	// LakeOf is each grid point's lake, indexed x*Height+z: 1 + its index
	// in World.Lakes, or 0 for none. Like a cell's elevation and snow, it's
	// the value at the cell's corner, world (x, z) × CellSize, so the snow
	// a lake clears is the snow on its water. LakeDepth is the water's
	// depth there, metres, estimated from the shore. Both nil when there
	// are no lakes. LakeNear finds a material sample's lake.
	LakeOf    []uint8
	LakeDepth []float32

	// shed caches what ShedSnow derives from the ground and material map;
	// nil when the ground has changed since. See snowshed.go.
	shed *shedCache

	// accessible is a derived per-cell land-ownership grid. nil means all
	// in-bounds cells are accessible (no parcel system). Maintained by
	// World.ApplyParcels and World.BuyParcel.
	accessible [][]bool

	// trees holds each cell's stored trees, indexed x*Height+z. Written
	// only through trees.go so Cell.TreeCount stays in step.
	trees [][]Tree

	// horizon is the derived terrain horizon map (see horizon.go);
	// horizonStale is set by RecomputeSlopes after elevation changes.
	horizon      *HorizonMap
	horizonStale bool
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
		Groom:   NewGroomMap(w, h),
		Moguls:  NewMogulMap(w, h),
		trees:   make([][]Tree, w*h),
	}
}

// RecomputeSlopes recomputes Cell.Slope for every cell from the current
// ground elevations using central differences. Call this once after any
// batch change to GroundElevation (terrain import, save load, lift grading,
// testbed setup). The sim's avalanche logic reads Cell.Slope directly.
func (t *Terrain) RecomputeSlopes() {
	t.horizonStale = true
	t.shed = nil
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

// CellAtWorld returns the cell holding world XZ (wx, wz), or nil off the
// map.
func (t *Terrain) CellAtWorld(wx, wz float32) *Cell {
	x, z := int(wx/CellSize), int(wz/CellSize)
	if wx < 0 || wz < 0 || !t.InBounds(x, z) {
		return nil
	}
	return &t.Cells[x][z]
}

// TreeCoverAt returns Cell.TreeCover for the cell holding world XZ
// (wx, wz). Out-of-bounds returns 0 (clear).
func (t *Terrain) TreeCoverAt(wx, wz float32) float32 {
	x, z, ok := t.treeCellOf(wx, wz)
	if !ok {
		return 0
	}
	return t.Cells[x][z].TreeCover()
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

// IsAccessible returns true if terrain cell (x, z) is owned by the player
// and available for building, skiing, and grooming. Returns false for
// out-of-bounds cells. When no parcel system is defined (accessible == nil),
// all in-bounds cells are accessible.
func (t *Terrain) IsAccessible(x, z int) bool {
	if !t.InBounds(x, z) {
		return false
	}
	if t.accessible == nil {
		return true
	}
	return t.accessible[x][z]
}

// IsAccessibleWorld returns true if the world-space position (wx, wz) maps to
// an owned terrain cell.
func (t *Terrain) IsAccessibleWorld(wx, wz float32) bool {
	const cs = float32(CellSize)
	xi := int(wx / cs)
	zi := int(wz / cs)
	return t.IsAccessible(xi, zi)
}
