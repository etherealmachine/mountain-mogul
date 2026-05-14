package world

import (
	"github.com/go-gl/mathgl/mgl32"
)

// Cell represents a single grid cell in the terrain.
//
// The terrain is two layers stacked: ground (rock/dirt the player almost
// never edits) and a snow layer on top whose depth and state evolve over
// time and which the player manipulates via grooming, snowmaking, etc.
//
//	SurfaceElevation = GroundElevation + SnowDepth
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
//   - SnowDepth high + Packed low                       → powder
//   - Grooming low + traffic over time                  → MogulSize grows
//   - Ice high + SnowDepth low                          → boilerplate
type Cell struct {
	GroundElevation float32 // metres; the rock/dirt under the snow
	SnowDepth       float32 // metres; snow on top of ground
	Grooming        float32 // 0..1; 1 = freshly groomed corduroy
	Packed          float32 // 0..1; density of the snow column (0 = fluffy powder, 1 = bulletproof packed)
	Ice             float32 // 0..1; ice fraction at the surface
	MogulSize       float32 // 0..1; mogul amplitude (visual + physics roughness)

	Passable    bool    // hard structural block (buildings, lift endpoints)
	TreeDensity float32 // 0.0 = clear, 1.0 = dense old-growth

	// Natural-state shadow fields. These hold the cell's "no structures
	// present" baseline so that placing / moving / removing a building,
	// lift, or road can restore the original terrain. Player non-sim
	// edits (auto-gen, glade / plant brushes, raise / lower, terrain
	// import) write the natural values; structure stamps overwrite the
	// display fields above on top of the natural baseline.
	//
	// Sim writes (snowfall, decay, grooming) intentionally hit the
	// display fields only — the natural layer is design-time intent and
	// shouldn't drift with runtime weather. The cost is that snow that
	// fell on a road's footprint at runtime won't reappear when the
	// road later moves; only what was originally painted does.
	//
	// Passable has no natural shadow — no natural terrain in this
	// codebase is impassable, so the rebuild path resets Display.Passable
	// to true and lets each structure's stamp re-mark the cells it owns.
	NaturalElev  float32
	NaturalSnow  float32
	NaturalTrees float32
}

// SurfaceElevation returns the snow-surface elevation for this cell
// (ground + snow).
func (c Cell) SurfaceElevation() float32 {
	return c.GroundElevation + c.SnowDepth
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
	// (Grooming/Packed/Ice/MogulSize/SnowDepth) has changed and the
	// terrain VBO needs a re-upload. The sim sets this; the scene
	// flushes the mesh and clears the flag once per frame. We keep
	// rebuilds to one per frame max even when many cells change in a
	// single tick — the whole-mesh upload is the same cost.
	SnowDirty bool
}

// DefaultSnowDepth is the baseline snow thickness applied to fresh
// terrain (NewTerrain) and to old saves that didn't store snow depth.
// Roughly mid-season packed depth at a typical resort.
const DefaultSnowDepth = float32(2.0)

// NewTerrain creates a flat terrain with all cells passable, default
// snow depth, and lightly-packed (close to fresh powder) snow. Skier
// traffic and snowcat grooming both compress the column toward Packed=1
// under SWE conservation, so starting near zero gives the most room
// for visible compression as the resort gets tracked out.
func NewTerrain(w, h int) *Terrain {
	cells := make([][]Cell, w)
	for x := 0; x < w; x++ {
		cells[x] = make([]Cell, h)
		for z := 0; z < h; z++ {
			cells[x][z] = Cell{
				GroundElevation: 0,
				SnowDepth:       DefaultSnowDepth,
				Packed:          0.2,
				Passable:        true,
				NaturalElev:     0,
				NaturalSnow:     DefaultSnowDepth,
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

// SnapshotNatural copies every cell's display fields into its Natural
// shadow. Call after the world's "no structures yet" terrain state has
// been set up — i.e. once after generation, or once after save load
// (since the save format may not yet carry Natural fields), before any
// structure stamping has run.
func (t *Terrain) SnapshotNatural() {
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			c := &t.Cells[x][z]
			c.NaturalElev = c.GroundElevation
			c.NaturalSnow = c.SnowDepth
			c.NaturalTrees = c.TreeDensity
		}
	}
}

// ResetDisplayFromNatural restores every cell's display fields from
// its Natural shadow. The caller is expected to re-stamp current
// structure footprints (roads, buildings, lifts) afterwards. Sim-side
// fields (Grooming, Packed, Ice, MogulSize) are not part of the
// natural layer and are left untouched. Passable is force-reset to
// true here so structure stamps own the "blocked" cells exclusively.
func (t *Terrain) ResetDisplayFromNatural() {
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			c := &t.Cells[x][z]
			c.GroundElevation = c.NaturalElev
			c.SnowDepth = c.NaturalSnow
			c.TreeDensity = c.NaturalTrees
			c.Passable = true
		}
	}
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
	c := t.Cells[x][z]
	return c.GroundElevation + c.SnowDepth
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
// point. Used by skier physics to derive effective friction. Out-of-bounds
// returns groomed-corduroy defaults.
func (t *Terrain) SnowAt(wx, wz float32) (depth, grooming, packed, ice, mogul float32) {
	const cellSize = float32(5.0)
	xi := int(wx / cellSize)
	zi := int(wz / cellSize)
	if !t.InBounds(xi, zi) {
		return DefaultSnowDepth, 1, 0.5, 0, 0
	}
	c := t.Cells[xi][zi]
	return c.SnowDepth, c.Grooming, c.Packed, c.Ice, c.MogulSize
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
