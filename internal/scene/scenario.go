package scene

import (
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/ai/goap"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// screenToCell projects a screen position to a terrain grid cell using DDA
// ray-marching. Steps half a cell at a time from above max terrain until the
// ray drops below the terrain surface. Used for the editor's terrain
// brushes (Plant, Glade, Raise, Lower) which are inherently per-cell.
func screenToCell(cam *render.Camera, terrain *world.Terrain, mousePos mgl32.Vec2) (gx, gz int) {
	const cellSize = float32(5.0)
	pos, ok := screenToWorld(cam, terrain, mousePos)
	if !ok {
		return -1, -1
	}
	return int(pos[0] / cellSize), int(pos[2] / cellSize)
}

// screenToWorld projects a screen position to a continuous point on the
// terrain mesh. Returns the refined intersection plus ok=true on success.
//
// The DDA marches in half-cell steps to find the *cell* where the ray
// drops below terrain (~5 m quantisation). For freeform placement that
// quantisation is visible — so we follow up with a few iterations of
// bisection between the last "above terrain" and first "below terrain"
// sample, refining against `InterpolatedElevationAt` to get a sub-metre
// hit suitable for placing buildings or lift endpoints anywhere.
func screenToWorld(cam *render.Camera, terrain *world.Terrain, mousePos mgl32.Vec2) (mgl32.Vec3, bool) {
	const cellSize = float32(5.0)
	const maxElev = float32(1500.0)

	origin, dir := cam.ScreenToWorldRay(mousePos)
	if dir[1] >= 0 {
		return mgl32.Vec3{}, false
	}

	// Start t so the ray is above the maximum terrain elevation.
	tCur := (maxElev - origin[1]) / dir[1]
	xzLen := float32(math.Sqrt(float64(dir[0]*dir[0] + dir[2]*dir[2])))
	if xzLen < 1e-6 {
		return mgl32.Vec3{}, false
	}
	dt := (cellSize * 0.5) / xzLen

	// Iteration cap is sized so we can descend the full maxElev range
	// at half-cell steps (with safety margin). Tied to cellSize so
	// changing cell resolution doesn't break the picker.
	maxSteps := int(maxElev/(cellSize*0.5)) + 200

	tPrev := tCur
	for i := 0; i < maxSteps; i++ {
		pos := origin.Add(dir.Mul(tCur))
		cx := int(pos[0] / cellSize)
		cz := int(pos[2] / cellSize)
		if terrain.InBounds(cx, cz) && pos[1] <= terrain.SurfaceElevationAt(cx, cz) {
			// Bisect between tPrev (above terrain) and tCur (below terrain)
			// using the smooth surface elevation so the returned point
			// sits within ~cm of the actual terrain mesh — small enough to
			// be invisible at game scale.
			lo, hi := tPrev, tCur
			for k := 0; k < 6; k++ {
				mid := (lo + hi) * 0.5
				p := origin.Add(dir.Mul(mid))
				if p[1] > terrain.InterpolatedSurfaceElevationAt(p[0], p[2]) {
					lo = mid
				} else {
					hi = mid
				}
			}
			return origin.Add(dir.Mul(hi)), true
		}
		tPrev = tCur
		tCur += dt
	}
	return mgl32.Vec3{}, false
}

// applyDensityBrush modifies TreeDensity within `radius` cells of (cx, cz) by `delta`.
// Clamps each cell's density to [0, 1].
func applyDensityBrush(t *world.Terrain, cx, cz, radius int, delta float32) {
	r2 := radius * radius
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dz*dz > r2 {
				continue
			}
			x, z := cx+dx, cz+dz
			if !t.InBounds(x, z) {
				continue
			}
			d := t.Cells[x][z].TreeDensity + delta
			if d < 0 {
				d = 0
			} else if d > 1 {
				d = 1
			}
			t.Cells[x][z].TreeDensity = d
		}
	}
}

// gladeStrokeCost returns the cost of one glade-brush application centred at
// (cx, cz) with the given radius and removal strength. Charges
// GladeCostPerCell × density_actually_removed per cell, so the total cost
// across all strokes to fully clear a cell equals exactly GladeCostPerCell
// regardless of how many strokes it takes. Zero when no trees are in range.
func gladeStrokeCost(t *world.Terrain, cx, cz, radius int, strength float32) int {
	r2 := radius * radius
	var total float32
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dz*dz > r2 {
				continue
			}
			x, z := cx+dx, cz+dz
			if !t.InBounds(x, z) {
				continue
			}
			d := t.Cells[x][z].TreeDensity
			if d <= 0 {
				continue
			}
			removed := strength
			if removed > d {
				removed = d
			}
			total += removed
		}
	}
	return int(total*float32(world.GladeCostPerCell) + 0.5)
}

// applyLiftPlacementEffects, applyLiftStationApron and clearLiftCorridor
// live in lift_apron.go — lift terminal grading is its own concern and
// we expect it to iterate.

// stationGroundElev returns the ground elevation of the cell containing
// the given world XZ position, or 0 if the position is off-terrain.
func stationGroundElev(t *world.Terrain, pos mgl32.Vec2) float32 {
	const cellSize = float32(5.0)
	xi := int(pos[0] / cellSize)
	zi := int(pos[1] / cellSize)
	if !t.InBounds(xi, zi) {
		return 0
	}
	return t.Cells[xi][zi].GroundElevation
}

// buildingFootprint returns the building's approximate physical
// half-extents in (halfX, halfZ) metres. The authoritative source is the
// `# footprint` line baked into the building's .obj by tools/scad2obj
// from `echo("MOGUL_META", "footprint", halfX, halfY)` in the .scad — so
// model dimensions are declared once, in the source of truth. The
// fallback below only kicks in when the OBJ hasn't been built (the
// renderer is showing the magenta marker cube in that case), so its
// half-extents just match that cube.
func buildingFootprint(typ world.BuildingType) (halfX, halfZ float32) {
	if fp, ok := world.FootprintFor(typ.MeshID()); ok {
		halfX, halfZ = fp.HalfX, fp.HalfZ
		if typ == world.BuildingParking {
			const cellSize = float32(5.0)
			halfX += cellSize
			halfZ += cellSize
		}
		return halfX, halfZ
	}
	return 1, 1 // matches the 2 m magenta fallback cube
}

// buildingApronBareGround is the width of the visible plowed-clearing
// band between the building footprint and the start of the smoothstep
// blend back to natural snow, in metres. The apron is sized per-axis so
// this band is uniform on all four sides of the pad — without that,
// rectangular buildings like the 40 × 30 m parking lot get visibly
// different bare-ground widths on their long vs. short edges (one side
// bleeds toward the snow, the other reads as a sharp boundary).
const buildingApronBareGround = float32(8.0)

// apronInnerFraction is the smoothstep threshold inside buildStationApron
// — the inner core stays at full target until perpDist/halfWidth (or
// signedAlong/depth) passes this fraction, then blends back to natural.
// Keep in sync with the smoothstep32(0.7, 1.0, …) calls below.
const apronInnerFraction = float32(0.7)

// applyBuildingPlacementEffects grooms a rectangular apron around a
// placed building and clears trees inside the same zone. The apron is
// sized from the building's footprint (halfX, halfZ) plus a clearance
// margin so each building gets visual breathing room. Unlike lifts the
// apron is raise-only with a small buildup — buildings fit the natural
// slope but won't sink into mogul fields or ungroomed powder.
func applyBuildingPlacementEffects(t *world.Terrain, b *world.Building) {
	halfX, halfZ := buildingFootprint(b.Type)
	// Size each axis of the apron so its inner flat zone extends exactly
	// buildingApronBareGround beyond the pad on every side. Solving
	// apronHalf * apronInnerFraction = footprintHalf + bareGround for
	// apronHalf gives the formula below.
	apronHalfX := (halfX + buildingApronBareGround) / apronInnerFraction
	apronHalfZ := (halfZ + buildingApronBareGround) / apronInnerFraction

	// Building apron: flatten the ground to the anchor's elevation plus
	// a small buildup, and plow off all the snow. Inner cells have
	// SnowAccumulation = 0 (bare ground / asphalt visible); outer edge
	// smoothsteps back up to the surrounding snowpack so the clearing
	// has snowdrift sides instead of a hard cliff. Result: parking
	// lots, lodges and sheds sit in a plowed graded clearing, neither
	// raised above nor buried beneath the snow.
	//
	// Reads the live cell elevation — fine because each structure is
	// stamped exactly once at placement (or drag) when the ground is
	// still natural. The apron leaves a permanent footprint; re-stamping
	// is not part of the model.
	const apronBuildup = float32(0.5)
	target := stationGroundElev(t, b.Pos) + apronBuildup

	// Two passes along the world-X axis give a 2*apronHalfX × 2*apronHalfZ
	// rectangle with side falloff on all four edges (full weight in the
	// inner core, smoothstep down to nothing at the edge). The two passes'
	// "back edges" meet flush at the building centre, producing a single
	// uniform pad with no internal seam.
	buildStationApron(t, b.Pos, mgl32.Vec2{1, 0}, +1, apronHalfZ, apronHalfX, target, true)
	buildStationApron(t, b.Pos, mgl32.Vec2{1, 0}, -1, apronHalfZ, apronHalfX, target, true)
	// Buildings plow off all the snow under their footprint — parking
	// lots, lodges and sheds sit on bare asphalt / dirt, not a snowdrift.
	// Lift aprons (in lift_apron.go) skip this step; their packed=1.0
	// makes the visible snow column shrink without removing the snow.
	plowApronSnow(t, b.Pos, mgl32.Vec2{1, 0}, +1, apronHalfZ, apronHalfX)
	plowApronSnow(t, b.Pos, mgl32.Vec2{1, 0}, -1, apronHalfZ, apronHalfX)

	// Trees inside the apron zone go to zero density so the building isn't
	// rendered with trunks pressing through its walls. Same extents as the
	// apron — the visible clear pad matches the tree-free footprint.
	clearBuildingTrees(t, b.Pos, apronHalfX, apronHalfZ)

	// Stamp the door cell as impassable so the stamp path produces the
	// same blocked-cell state as the original PlaceBuilding call.
	door := b.DoorCell()
	if t.InBounds(door[0], door[1]) {
		t.Cells[door[0]][door[1]].Passable = false
	}
	// Trees inside the apron were zeroed above; refresh the surface-
	// detail G channel so the well texture matches the new tree set.
	t.RestampTreeWells()
}

// clearBuildingTrees zeros TreeDensity in cells inside the axis-aligned
// rectangle ±(halfX, halfZ) around `pos`. Matches the apron rectangle so
// the visible clear pad lines up with the tree-cleared zone.
func clearBuildingTrees(t *world.Terrain, pos mgl32.Vec2, halfX, halfZ float32) {
	const cellSize = float32(5.0)
	x0 := int((pos[0] - halfX) / cellSize)
	x1 := int((pos[0]+halfX)/cellSize) + 1
	z0 := int((pos[1] - halfZ) / cellSize)
	z1 := int((pos[1]+halfZ)/cellSize) + 1
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			// Cell CENTER, not corner — matches the renderer's tree
			// anchor at ((x+0.5)*cellSize). Corner-based testing biased
			// the clear toward +X / +Z by a half cell.
			cx := (float32(x)+0.5)*cellSize - pos[0]
			cz := (float32(z)+0.5)*cellSize - pos[1]
			if cx < 0 {
				cx = -cx
			}
			if cz < 0 {
				cz = -cz
			}
			if cx <= halfX && cz <= halfZ {
				t.Cells[x][z].TreeDensity = 0
			}
		}
	}
}

// buildStationApron grades a rectangular pad whose back edge sits flush
// against `station`, extending `depth` metres along `axis * side` (the
// apron-forward direction) and ±`halfWidth` metres along the perpendicular.
// The pad's GroundElevation is set to `target`; the caller decides what
// that means.
//
// Used by the building apron (called as a pair of back-to-back passes to
// make a single rectangular pad). The lift apron has its own grading
// path in lift_apron.go.
//
// `cut` toggles between two grading modes:
//
//   - cut = false: raise-only. Cells whose natural elevation already
//     exceeds the target are left alone.
//
//   - cut = true (buildings): cut-and-fill. Cells above the target are
//     pulled down to the pad level; cells below are pushed up. Produces
//     a true flat platform on sloped sites.
//
// Falloff is applied at the front edge and the two side edges so the
// transition between the pad and natural terrain blends smoothly, but
// NOT at the back edge against the station/building anchor, which keeps
// a sharp transition where the two passes meet at the centre.
//
// `axis` is a unit vector along the apron's primary axis. `side = +1`
// flips the apron to extend in the +axis direction; `side = -1` extends
// the apron in the -axis direction.
// buildStationApron grades the ground under a station footprint and
// raises Packed in the inner zone. Snow plowing (zeroing
// SnowAccumulation) is a separate concern handled by plowApronSnow —
// buildings call it, lift terminals don't (they get the "packed pad"
// look from Packed=1.0 alone, since visible depth = accumulation /
// density(Packed) automatically shrinks).
func buildStationApron(t *world.Terrain, station, axis mgl32.Vec2, side, halfWidth, depth, target float32, cut bool) {
	const cellSize = float32(5.0)
	stationCell := [2]int{
		int(station[0] / cellSize),
		int(station[1] / cellSize),
	}
	if !t.InBounds(stationCell[0], stationCell[1]) {
		return
	}
	bound := halfWidth
	if depth > bound {
		bound = depth
	}
	x0 := int((station[0] - bound) / cellSize)
	x1 := int((station[0]+bound)/cellSize) + 1
	z0 := int((station[1] - bound) / cellSize)
	z1 := int((station[1]+bound)/cellSize) + 1
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			// Cell CENTER, not corner — same fix as road clearance and
			// clearBuildingTrees. Corner-based decomposition pushed the
			// apron half a cell to the +X / +Z side relative to where
			// the cell's visual contents (tree, snow shading) sit.
			cx := (float32(x) + 0.5) * cellSize
			cz := (float32(z) + 0.5) * cellSize
			dx := cx - station[0]
			dz := cz - station[1]
			// Decompose into along-axis and perpendicular components.
			alongRaw := dx*axis[0] + dz*axis[1]
			signedAlong := alongRaw * side
			if signedAlong < 0 || signedAlong > depth {
				continue
			}
			perpX := dx - alongRaw*axis[0]
			perpZ := dz - alongRaw*axis[1]
			perpDist := float32(math.Sqrt(float64(perpX*perpX + perpZ*perpZ)))
			if perpDist > halfWidth {
				continue
			}
			// Forward falloff at the far edge — full weight through the
			// inner 70 %, smoothstepped to zero at depth so the apron
			// blends into natural terrain ahead. NO falloff at the back
			// edge (signedAlong = 0) — sharp cut into the hillside if
			// needed.
			forwardWeight := 1 - smoothstep32(0.7, 1.0, signedAlong/depth)
			// Side falloff — same logic on the cross-axis: full through
			// the inner 70 %, fading to zero at halfWidth so skiers can
			// peel off either side without hitting a step.
			sideWeight := 1 - smoothstep32(0.7, 1.0, perpDist/halfWidth)
			w := forwardWeight * sideWeight
			if w <= 0 {
				continue
			}
			cur := t.Cells[x][z].GroundElevation
			if !cut && target <= cur {
				// Raise-only mode: cells whose natural elevation is
				// already at or above the target pad height keep what
				// they have.
				continue
			}
			// Blend toward the target by the smoothstep weight. When
			// cut = false this only ever raises (the guard above
			// filters cells where target <= cur). When cut = true it
			// can also lower cells above the target, producing a true
			// flat platform.
			t.Cells[x][z].GroundElevation = cur + (target-cur)*w
			// Set the top layer to Packed Powder in the inner zone —
			// boarding pads, building approaches, and lift queues are
			// foot-traffic compacted. KindPackedPowder has higher density
			// (0.50 vs 0.15 for powder), so visible depth drops automatically.
			cell := &t.Cells[x][z]
			if top := cell.TopLayer(); top != nil && w > 0.5 {
				top.Kind = world.KindPackedPowder
			}
			cell.MogulSize *= 1 - w
		}
	}
}

// plowApronSnow zeros SnowAccumulation across the inner zone of a
// rectangular apron and smoothsteps it back to natural at the outer
// edge. Same geometry as buildStationApron (axis / side / halfWidth /
// depth) so the plowed footprint lines up with the graded footprint.
// Buildings call this to clear their pad to bare ground; lift terminals
// do NOT — their visual "packed apron" comes from Packed=1.0 alone.
func plowApronSnow(t *world.Terrain, station, axis mgl32.Vec2, side, halfWidth, depth float32) {
	const cellSize = float32(5.0)
	bound := halfWidth
	if depth > bound {
		bound = depth
	}
	x0 := int((station[0] - bound) / cellSize)
	x1 := int((station[0]+bound)/cellSize) + 1
	z0 := int((station[1] - bound) / cellSize)
	z1 := int((station[1]+bound)/cellSize) + 1
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			cx := (float32(x) + 0.5) * cellSize
			cz := (float32(z) + 0.5) * cellSize
			dx := cx - station[0]
			dz := cz - station[1]
			alongRaw := dx*axis[0] + dz*axis[1]
			signedAlong := alongRaw * side
			if signedAlong < 0 || signedAlong > depth {
				continue
			}
			perpX := dx - alongRaw*axis[0]
			perpZ := dz - alongRaw*axis[1]
			perpDist := float32(math.Sqrt(float64(perpX*perpX + perpZ*perpZ)))
			if perpDist > halfWidth {
				continue
			}
			forwardWeight := 1 - smoothstep32(0.7, 1.0, signedAlong/depth)
			sideWeight := 1 - smoothstep32(0.7, 1.0, perpDist/halfWidth)
			w := forwardWeight * sideWeight
			if w <= 0 {
				continue
			}
			// Blend toward 0 at the inner zone; outer edge keeps natural.
			t.Cells[x][z].Base *= 1 - w
			t.Cells[x][z].Top.Accumulation *= 1 - w
		}
	}
}

// pointSegmentDistSq returns the squared distance from p to the line
// segment ab. Cheap helper for clearLiftCorridor.
func pointSegmentDistSq(p, a, b mgl32.Vec2) float32 {
	abx := b[0] - a[0]
	abz := b[1] - a[1]
	abLen2 := abx*abx + abz*abz
	if abLen2 < 1e-6 {
		dx := p[0] - a[0]
		dz := p[1] - a[1]
		return dx*dx + dz*dz
	}
	t := ((p[0]-a[0])*abx + (p[1]-a[1])*abz) / abLen2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	cx := a[0] + abx*t
	cz := a[1] + abz*t
	dx := p[0] - cx
	dz := p[1] - cz
	return dx*dx + dz*dz
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// applyDensityBrushUpTo ramps TreeDensity within `radius` cells of (cx, cz)
// upward by `step`, but caps each cell at `target` (so the slider acts as a
// ceiling). Cells already at or above target are left alone, so reducing the
// slider after painting doesn't erase existing forest — use the glade tool
// for that.
func applyDensityBrushUpTo(t *world.Terrain, cx, cz, radius int, step, target float32) {
	r2 := radius * radius
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dz*dz > r2 {
				continue
			}
			x, z := cx+dx, cz+dz
			if !t.InBounds(x, z) {
				continue
			}
			cur := t.Cells[x][z].TreeDensity
			if cur >= target {
				continue
			}
			d := cur + step
			if d > target {
				d = target
			}
			t.Cells[x][z].TreeDensity = d
		}
	}
}

// toolMode represents the active placement tool.
type toolMode int

const (
	toolNone        toolMode = iota
	toolBuilding      toolMode = iota // place a lodge
	toolTicketOffice  toolMode = iota // place a ticket office (season pass sales)
	toolShed        toolMode = iota // place an equipment shed
	toolParking     toolMode = iota // place a parking lot (skier spawn/despawn)
	toolLiftBase    toolMode = iota // waiting for first lift click
	toolLiftTop     toolMode = iota // waiting for second lift click
	toolRoadStart   toolMode = iota // waiting for first road click
	toolRoadEnd     toolMode = iota // waiting for second road click
	toolEdgeConnect toolMode = iota // place a map-edge road connection node (editor only)
	toolPatrolHut   toolMode = iota // place a ski patrol hut
	toolSnowGun     toolMode = iota // place a snowmaking cannon
	toolGlade       toolMode = iota // reduce TreeDensity (brush)
	toolPlantTrees  toolMode = iota // increase TreeDensity (brush, editor only)
	toolRemove     toolMode = iota // remove building at clicked cell
	toolTrailPaint toolMode = iota // paint/erase cells on the active trail
	toolLandBuy    toolMode = iota // click to purchase a land parcel
)

// Scenario is the main gameplay scene.
type Scenario struct {
	app             *engine.App
	world           *world.World
	sim             *sim.Simulation
	toolBar         *ui.MenuBar      // bottom-of-screen tool palette
	topBar          *ui.TopBar       // resort-management HUD strip
	overlayPanel    *ui.OverlayPanel // right-side terrain-overlay toggles
	chartWindow     *ui.ChartWindow  // resort-stats charts (line + grouped bar)
	escapeMenu      *EscapeMenu
	settingsMenu    *SettingsMenu
	debugConsole    *DebugConsole
	toolButtons      map[toolMode]*ui.Button
	liftDoubleBtn    *ui.Button        // toolbar button for the double-chair lift variant
	liftQuadBtn      *ui.Button        // toolbar button for the fixed-quad lift variant
	liftHSQuadBtn    *ui.Button        // toolbar button for the high-speed quad lift variant
	liftHS6PackBtn   *ui.Button        // toolbar button for the high-speed 6-pack lift variant
	liftGondolaBtn   *ui.Button        // toolbar button for the MDG gondola
	liftHeliBtn      *ui.Button        // toolbar button for the helicopter heli-ski lift
	liftsSubmenu     *ui.SubmenuButton // Lifts group (all chair/gondola/heli variants)
	opsSubmenu       *ui.SubmenuButton // Operations group (shed, patrol)
	amenitiesSubmenu  *ui.SubmenuButton // Amenities group (lodge)
	transportSubmenu  *ui.SubmenuButton // Transport group (parking, road)
	activeTool      toolMode
	liftType        world.LiftType // chair variant the toolLiftBase/Top flow will place
	liftBase        mgl32.Vec2     // first click world position for lift placement
	roadStart       mgl32.Vec2 // first click world position for road placement (post-snap)
	scenarioPath    string
	time            float32
	rightDragging   bool
	hoverCell        [2]int     // terrain cell under the mouse — for cell-based tools
	hoverWorld       mgl32.Vec3 // continuous terrain hit under the mouse — for placement
	hoverMouseScreen mgl32.Vec2 // screen-space mouse position, updated with hoverValid
	hoverValid       bool       // false when the mouse is off-terrain or over the menu bars
	followGuestID   uint64 // 0 = free camera; >0 = ID of followed skier
	firstPerson     bool   // V: first-person camera at the followed skier's head
	debugSteering   bool   // F3: render steering forces on the followed skier
	debugPlanner    bool   // F4: show goal weights, full plan, snapshot anchors for the followed skier
	debugTerrainIns bool   // F5: dump cell snow/terrain state under the mouse cursor

	// fpsSmoothed is an EMA of the wall-clock frame rate, updated every
	// Update tick. Surfaced in the F5 inspector panel so the player can
	// monitor frame time while looking at sim state. Initialised lazily
	// from the first dt so the first frame doesn't read as 0 fps.
	fpsSmoothed float32
	paused          bool
	popup           *ui.Window
	saveAllowed     bool   // false in testbed mode; gates the Save prompt
	saveName        string // last name used for Save; pre-fills the prompt next time
	savePrompt      *savePrompt
	prebuiltWorld   *world.World
	simSeed         int64                          // 0 = wall-clock; nonzero forces deterministic RNG
	rebuild         func(seed int64) *world.World // non-nil ⇒ "New Seed" button shown
	tickHook        func(s *sim.Simulation)       // optional testbed hook; called each frame before Tick
	queryServer     *sim.QueryServer              // non-nil when -live-query is set; wired into sim on installWorld

	// Glade-tool sliders (radius in cells, thin = % density removed per
	// application; slider 0–10 → 0.00–0.10 density delta). Visible only
	// while toolGlade is active; mirrors the editor's two-slider layout.
	// lastGladeCell is the cell most recently glade-applied this stroke;
	// drag-paint fires only when the cursor crosses into a new cell so
	// stationary holding doesn't pulse and a slow careful click doesn't
	// double-apply.
	gladeRadiusSlider *ui.VSlider
	gladeThinSlider   *ui.VSlider
	lastGladeCell     [2]int

	// Trail-paint mode: while toolTrailPaint is active, the player
	// drag-paints (left) or erases (right) cells on the active trail.
	activeTrailID    uint64
	trailDifficulty  world.TerrainDifficulty // difficulty for the next new trail
	lastTrailPaintCell [2]int
	trailEraseMode   bool // true = left-drag removes cells; false = left-drag adds cells

	selectedCatID      uint64 // 0 = none; >0 = ID of cat whose popup is open
	selectedBuildingID uint64 // 0 = none; >0 = ID of building whose popup is open
	showCatPath   bool   // true = draw cat's route + transit as debug lines

	// hoverParcel is the parcel under the mouse cursor when toolLandBuy is
	// active. Nil when the cursor is over owned/off-limits land or no parcel
	// system is defined.
	hoverParcel *world.Parcel

	// parcelBoundaryDirty is set to true when the parcel state changes
	// (world load or purchase). The render loop rebuilds and uploads the
	// fence line geometry once and clears the flag.
	parcelBoundaryDirty bool

	// Click-to-edit road state. Active only while activeTool == toolNone:
	// a toolNone left click on a road node/edge populates this and the
	// player can drag the node or press Delete to remove it.
	roadEdit roadEditSelection

	// Click-to-edit building / lift state. Same toolNone affordance as
	// road edit but for placed structures.
	structureEdit structureEditSelection

	// Debug instrumentation (see plan: orbiting-skier debug aids).
	csvRecorder       *sim.CSVRecorder
	pendingScreenshot bool
	toastText         string
	toastExpiry       float32 // s.time at which toast disappears

	// GIF capture state. gifRecording is true while recording; gifFrames
	// accumulates paletted frames; gifFrameCount counts rendered frames so we
	// can subsample (capture every gifFrameInterval-th frame).
	// gifResult receives a single message from the background encode goroutine.
	gifRecording  bool
	gifFrames     []*image.Paletted
	gifFrameCount int
	gifResult     chan string

	// Trail of world-space positions for the currently followed skier.
	// Reset when followGuestID changes; appended when at least
	// trackMinSpacing metres past the last sample.
	track       []mgl32.Vec3
	trackOwner  uint64
}

const (
	trackMinSpacing = 0.5  // m; minimum distance from last sample before appending
	trackMaxPoints  = 6000 // hard cap; old points dropped when exceeded
)

// speedOptions lists the time-scale presets shown in the top bar.
// 4× is the default (~1 real hour per ~186-day ski season). 1× drags a
// season out to ~4 hours for granular debugging; 20× compresses it to
// ~12 minutes for fast-forward. The simulation substeps internally (see
// Simulation.Tick) so the L1 controller still sees a small dt at the
// upper preset. Pause is its own button — not in this list.
var speedOptions = []float64{1, 4, 20}


// NewScenarioFromFile creates a Scenario that loads its initial world from
// `path`. Used for both New Game (asset scenarios) and Load Game (named
// saves). Save is enabled; the first Save click prompts for a name. If the
// loaded path lives inside the user's saves directory, that name is used as
// the default for subsequent saves so re-saving overwrites the same file.
func NewScenarioFromFile(path string) *Scenario {
	s := &Scenario{
		scenarioPath: path,
		saveAllowed:  true,
	}
	// If the file came from the user's saves directory, seed saveName with
	// its basename so Save defaults to overwriting the same slot.
	dir, file := filepath.Split(path)
	cleanDir := filepath.Clean(dir)
	if cleanDir == filepath.Clean(save.SavesDir()) && strings.HasSuffix(file, ".json") {
		s.saveName = strings.TrimSuffix(file, ".json")
	}
	return s
}

// TerrainSize returns the loaded world's terrain grid dimensions. Returns
// (0, 0) before Init has run. Used by the -screenshot harness in main to
// frame the camera around the whole map.
// SetOverlay forces the terrain-overlay bitmask on the scenario's
// overlay panel. The panel is the source of truth — Update writes its
// mask to r.TerrainOverlayMode every frame — so a CLI caller that
// just sets the renderer field gets clobbered. This setter installs
// the mask on the panel itself. Used by -screenshot -overlay-mode for
// rendering specific debug overlays without driving the keyboard.
func (s *Scenario) SetOverlay(mask int) {
	if s.overlayPanel != nil {
		s.overlayPanel.SetMask(mask)
	}
}

func (s *Scenario) TerrainSize() (int, int) {
	if s.world == nil || s.world.Terrain == nil {
		return 0, 0
	}
	return s.world.Terrain.Width, s.world.Terrain.Height
}

// RegenForest re-runs the auto-forest generator on the loaded terrain and
// rebuilds the static batch so the new tree placement renders. Exposed
// mainly for the -screenshot harness; in normal use the editor's
// Auto-forest button drives this.
func (s *Scenario) RegenForest(seed int64) {
	if s.world == nil || s.world.Terrain == nil {
		return
	}
	GenerateTreeCover(s.world.Terrain, 24, 0.55, 0.75, seed)
	s.world.Terrain.RestampTreeWells()
	if s.app != nil && s.app.Renderer != nil {
		s.app.Renderer.RebuildStaticBatch(s.world)
	}
}

// NewScenarioFromWorld creates a Scenario backed by a programmatically-built
// world (e.g. a sim.Testbed). seed is forwarded to NewSimulationWithSeed for
// reproducibility; pass 0 for wall-clock seeding. Save/Load are disabled in
// this mode so a debug run can't clobber the user save slot.
func NewScenarioFromWorld(w *world.World, seed int64) *Scenario {
	return &Scenario{
		prebuiltWorld: w,
		simSeed:       seed,
	}
}

// NewScenarioFromTestbed creates a Scenario from a sim.Testbed and remembers
// how to rebuild the world from a seed, so the menu bar can offer a
// "New Seed" button that re-rolls the run without leaving the scene.
// globalQueryServer, if set, is attached to every Scenario on installWorld.
var globalQueryServer *sim.QueryServer

// globalSimSeed, if non-zero, overrides the RNG seed for every Scenario.
var globalSimSeed int64

// UseQueryServer registers a query server to be wired into every Scenario
// that installs a world, regardless of how it was created. Call once from
// main before app.Run().
func UseQueryServer(qs *sim.QueryServer) { globalQueryServer = qs }

// UseSimSeed sets a global RNG seed applied to every Scenario on installWorld.
func UseSimSeed(seed int64) { globalSimSeed = seed }

// SetQueryServer attaches a live-query server to this specific scenario.
// Overrides the global server for this instance.
func (s *Scenario) SetQueryServer(qs *sim.QueryServer) { s.queryServer = qs }

// SetSeed overrides the RNG seed for this scenario's simulation.
func (s *Scenario) SetSeed(seed int64) { s.simSeed = seed }

func NewScenarioFromTestbed(tb *sim.Testbed) *Scenario {
	rebuild := func(seed int64) *world.World {
		return tb.Build()
	}
	return &Scenario{
		prebuiltWorld: rebuild(tb.Seed),
		simSeed:       tb.Seed,
		rebuild:       rebuild,
		tickHook:      tb.TickHook,
	}
}

func (s *Scenario) Init(app *engine.App) error {
	s.app = app

	var w *world.World
	var cam *save.CameraData
	if s.prebuiltWorld != nil {
		w = s.prebuiltWorld
	} else {
		loaded, loadedCam, err := save.LoadScenario(s.scenarioPath)
		if err != nil {
			// Fall back to a blank world
			fmt.Printf("Scenario load error (%v), creating blank world\n", err)
			t := world.NewTerrain(32, 32)
			loaded = world.NewWorld(t)
		}
		w = loaded
		cam = loadedCam
	}
	s.installWorld(w)
	if cam != nil {
		applyCameraSnapshot(app.Renderer.Camera, cam)
	}

	s.settingsMenu = NewSettingsMenu(app, func() { s.escapeMenu.Show() })
	openSettings := func() {
		s.escapeMenu.Hide()
		s.settingsMenu.Show()
	}
	if s.saveAllowed {
		s.escapeMenu = NewEscapeMenu(app, s.openSavePrompt, s.gotoLoadMenu, openSettings)
	} else {
		s.escapeMenu = NewEscapeMenu(app, nil, nil, openSettings)
	}
	s.debugConsole = newDebugConsole(s.world, s.setToast)
	r := s.app.Renderer
	s.debugConsole.SetSim(s.sim, func() { r.FlushTerrainVerts(s.world.Terrain) })

	// Bottom tool bar — palette of construction tools, centred along the
	// bottom edge. Y is set each frame in Update() based on the current
	// screen height. Tall enough to fit a 24-px icon plus a label row.
	const toolBarH = float32(60)
	s.toolButtons = make(map[toolMode]*ui.Button)
	s.toolBar = ui.NewMenuBar(0, toolBarH)
	s.toolBar.Centered = true

	// Amenities submenu: Lodge, Ticket Office
	s.amenitiesSubmenu = s.toolBar.AddSubmenu(render.IconHouse, "Amenities")
	s.toolButtons[toolBuilding] = s.amenitiesSubmenu.AddChild(render.IconHouse, "Lodge", func() { s.setTool(toolBuilding) })
	s.toolButtons[toolTicketOffice] = s.amenitiesSubmenu.AddChild(render.IconCoin, "Tickets", func() { s.setTool(toolTicketOffice) })

	// Operations submenu: Shed, Patrol
	s.opsSubmenu = s.toolBar.AddSubmenu(render.IconGarage, "Operations")
	s.toolButtons[toolShed] = s.opsSubmenu.AddChild(render.IconGarage, "Shed", func() { s.setTool(toolShed) })
	s.toolButtons[toolPatrolHut] = s.opsSubmenu.AddChild(render.IconHeart, "Patrol", func() { s.setTool(toolPatrolHut) })

	// Transport submenu: Parking, Road
	s.transportSubmenu = s.toolBar.AddSubmenu(render.IconRoad, "Transport")
	s.toolButtons[toolParking] = s.transportSubmenu.AddChild(render.IconUsers, "Parking", func() { s.setTool(toolParking) })
	s.toolButtons[toolRoadStart] = s.transportSubmenu.AddChild(render.IconRoad, "Road", func() { s.setTool(toolRoadStart) })

	// Lifts submenu: all chair/gondola/heli variants
	s.liftsSubmenu = s.toolBar.AddSubmenu(render.IconCableCar, "Lifts")
	s.liftDoubleBtn = s.liftsSubmenu.AddChild(render.IconCableCar, "Double", func() { s.activateLiftTool(world.LiftDouble) })
	s.liftQuadBtn = s.liftsSubmenu.AddChild(render.IconCableCar, "Quad", func() { s.activateLiftTool(world.LiftFixedQuad) })
	s.liftHSQuadBtn = s.liftsSubmenu.AddChild(render.IconCableCar, "HSQuad", func() { s.activateLiftTool(world.LiftHSQuad) })
	s.liftHS6PackBtn = s.liftsSubmenu.AddChild(render.IconCableCar, "6-Pack", func() { s.activateLiftTool(world.LiftHS6Pack) })
	s.liftGondolaBtn = s.liftsSubmenu.AddChild(render.IconCableCar, "Gondola", func() { s.activateLiftTool(world.LiftGondola) })
	s.liftHeliBtn = s.liftsSubmenu.AddChild(render.IconCableCar, "Heli", func() { s.activateLiftTool(world.LiftHeli) })

	// Flat tools
	s.toolButtons[toolSnowGun] = s.toolBar.AddIconButton(render.IconSnowflake, "Snow Gun", func() { s.setTool(toolSnowGun) })
	s.toolButtons[toolGlade] = s.toolBar.AddIconButton(render.IconAxe, "Glade", func() { s.setTool(toolGlade) })
	s.toolButtons[toolTrailPaint] = s.toolBar.AddIconButton(render.IconFlag, "Trail", func() { s.activateTrailTool() })
	s.toolButtons[toolRemove] = s.toolBar.AddIconButton(render.IconTrash, "Remove", func() { s.setTool(toolRemove) })
	s.toolButtons[toolLandBuy] = s.toolBar.AddIconButton(render.IconGlobe, "Land", func() {
		s.setTool(toolLandBuy)
		if s.activeTool == toolLandBuy {
			s.setToast("Click a purchasable parcel to buy it. Esc to exit.")
		}
	})
	if s.rebuild != nil {
		s.toolBar.AddButton("New Seed", func() {
			s.restartWithSeed(rand.Int63n(1_000_000))
		})
	}

	// Top resort-management bar — stats / date / weather / speed / settings.
	// Three text rows on the left at 26 px glyph height + padding need ~90 px;
	// the speed cluster wants icon + label which also lands in that range.
	const topBarH = float32(96)
	s.topBar = ui.NewTopBar(topBarH)
	s.topBar.GetCash = func() int { return s.world.Cash }
	s.topBar.GetGuests = func() int { return len(s.world.OnMountain) }
	s.topBar.GetHappiness = func() float32 {
		if s.sim == nil || s.sim.Demand == nil {
			return 0
		}
		return s.sim.Demand.ResortRating
	}
	s.topBar.GetDate = func() (int, string, int) {
		d := sim.CalendarAt(s.sim.SimTime)
		return d.Day, d.Month, d.Year
	}
	s.topBar.GetWeather = func() []ui.ForecastDay {
		today := s.sim.Weather.Today()
		from := sim.DateAt(s.sim.SimTime)
		forecast := s.sim.Weather.Forecast(from, 4)
		days := make([]ui.ForecastDay, 1+len(forecast))
		days[0] = ui.ForecastDay{
			Weather:  weatherToUI(today.State),
			TempHigh: today.TempHigh,
			TempLow:  today.TempLow,
		}
		for i, d := range forecast {
			days[i+1] = ui.ForecastDay{
				Weather:  weatherToUI(d.State),
				TempHigh: d.TempHigh,
				TempLow:  d.TempLow,
			}
		}
		return days
	}

	// Glade-tool sliders. Default radius matches the previous fixed
	// gladeRadius; Thin range 1–5 % per application keeps drag-painting
	// gradual at the high end (~20 cells across a stand to fully clear)
	// and lets the player do fine selective work at the low end.
	s.gladeRadiusSlider = ui.NewVSlider(0, 0, 18, 200, 1, 30, float32(gladeRadius), "Radius")
	s.gladeThinSlider = ui.NewVSlider(0, 0, 18, 200, 1, 5, 2, "Thin")
	s.lastGladeCell = [2]int{-1, -1}

	onSpeed := make([]func(), len(speedOptions))
	for i, mult := range speedOptions {
		mult := mult
		idx := i
		onSpeed[idx] = func() {
			s.sim.TimeScale = mult
			s.paused = false
			s.syncSpeedButtons()
		}
	}
	s.topBar.SetSpeedControls(
		func() {
			s.paused = !s.paused
			s.syncSpeedButtons()
		},
		onSpeed,
	)
	s.topBar.SetSettingsButton(func() { s.escapeMenu.Toggle() })
	s.syncSpeedButtons()

	// Overlay panel — vertical strip on the right edge. Toggle button in
	// the top bar drives visibility; the panel itself owns hit-testing
	// for its rows.
	s.overlayPanel = ui.NewOverlayPanel()
	s.overlayPanel.Top = topBarH
	s.overlayPanel.Bottom = float32(app.Renderer.ScreenHeight()) - toolBarH
	s.topBar.SetOverlayToggle(func() {
		visible := s.overlayPanel.Toggle()
		s.topBar.SetOverlayActive(visible)
	})

	// Charts window — three tabs: guest population (line), arrivals /
	// departures (grouped bar), cash on hand (line). Data is pulled
	// from world.History each frame so retuning the simulation doesn't
	// require touching the UI.
	s.chartWindow = ui.NewChartWindow("Resort Overview", 0, 0, []ui.Chart{
		{
			Title:   "Guests on mountain",
			Icon:    render.IconUsers,
			Kind:    ui.ChartLine,
			Series:  []ui.ChartSeries{{Name: "Guests", Color: mgl32.Vec4{0.55, 0.85, 1.0, 1}}},
			GetData: func() []ui.ChartPoint { return historyToChart(s.world, guestsField) },
		},
		{
			Title: "Arrivals & departures",
			Icon:  render.IconChartBar,
			Kind:  ui.ChartGroupedBar,
			Series: []ui.ChartSeries{
				{Name: "Arrivals", Color: mgl32.Vec4{0.35, 0.85, 0.45, 1}},
				{Name: "Departures", Color: mgl32.Vec4{0.95, 0.55, 0.45, 1}},
			},
			GetData: func() []ui.ChartPoint { return historyToChart(s.world, arrDepField) },
		},
		{
			Title:   "Cash on hand",
			Icon:    render.IconCoin,
			Kind:    ui.ChartLine,
			Series:  []ui.ChartSeries{{Name: "Cash", Color: mgl32.Vec4{1.0, 0.85, 0.30, 1}}},
			GetData: func() []ui.ChartPoint { return historyToChart(s.world, cashField) },
		},
		{
			Title: "Daily revenue & costs",
			Icon:  render.IconChartLine,
			Kind:  ui.ChartLine,
			Series: []ui.ChartSeries{
				{Name: "Revenue", Color: mgl32.Vec4{0.35, 0.85, 0.45, 1}},
				{Name: "Costs", Color: mgl32.Vec4{0.95, 0.45, 0.35, 1}},
				{Name: "Profit", Color: mgl32.Vec4{1.0, 0.85, 0.30, 1}},
			},
			GetData: func() []ui.ChartPoint { return historyToChart(s.world, pnlField) },
		},
		{
			Title:   "Guest thoughts (at resort)",
			Icon:    render.IconHeart,
			Kind:    ui.ChartThoughtRank,
			Series:  thoughtChartSeries(),
			GetData: func() []ui.ChartPoint { return thoughtsToDistribution(s.world) },
		},
		{
			Title:   "Exit thoughts",
			Icon:    render.IconFlag,
			Kind:    ui.ChartThoughtRank,
			Series:  thoughtChartSeries(),
			GetData: func() []ui.ChartPoint { return exitThoughtsToDistribution(s.world) },
		},
		{
			Title: "Resort overview",
			Icon:  render.IconGridFour,
			Kind:  ui.ChartStats,
			Series: []ui.ChartSeries{
				{Name: "Easy terrain", Color: mgl32.Vec4{0.55, 1.0, 0.55, 1}},
				{Name: "Intermediate terrain", Color: mgl32.Vec4{0.55, 0.75, 1.0, 1}},
				{Name: "Advanced terrain", Color: mgl32.Vec4{0.85, 0.85, 0.85, 1}},
				{Name: "Total skiable terrain", Color: mgl32.Vec4{0.55, 0.85, 1.0, 1}},
			},
			GetData: func() []ui.ChartPoint {
				var green, blue, black int
				seenAll := make(map[[2]int]bool)
				for _, t := range s.world.Trails {
					seen := make(map[[2]int]bool)
					for _, c := range t.Cells {
						if !seen[c] {
							seen[c] = true
							switch t.Difficulty {
							case world.DiffGreen:
								green++
							case world.DiffBlue:
								blue++
							default:
								black++
							}
						}
						seenAll[c] = true
					}
				}
				return []ui.ChartPoint{{Values: []float64{
					float64(green),
					float64(blue),
					float64(black),
					float64(len(seenAll)),
				}}}
			},
			FormatValue: func(v float64) string { return settings.FormatArea(int(v)) },
		},
		{
			Title: "Mountain capacity",
			Icon:  render.IconUsers,
			Kind:  ui.ChartStats,
			Series: []ui.ChartSeries{
				{Name: "Terrain cap (guests at once)", Color: mgl32.Vec4{1.0, 0.75, 0.35, 1}},
			},
			GetData: func() []ui.ChartPoint {
				return []ui.ChartPoint{{Values: []float64{float64(sim.TerrainCapacity(s.world))}}}
			},
			FormatValue: func(v float64) string { return fmt.Sprintf("%.0f guests", v) },
		},
	})
	s.chartWindow.Center(app.Renderer.ScreenWidth(), app.Renderer.ScreenHeight())
	s.topBar.SetChartsToggle(func() {
		s.chartWindow.Visible = !s.chartWindow.Visible
		s.topBar.SetChartsActive(s.chartWindow.Visible)
	})

	s.trailDifficulty = world.DiffGreen

	return nil
}

// historyField selects which DailySample columns get pulled into a
// chart. Lets the three Chart definitions share one converter.
type historyField int

const (
	guestsField historyField = iota
	arrDepField
	cashField
	pnlField
)

// historyToChart turns world.History.Ordered() into []ui.ChartPoint with
// the values for the requested field. Cheap (one alloc per Draw); the
// chart window only calls this while visible.
func historyToChart(w *world.World, field historyField) []ui.ChartPoint {
	if w == nil || w.History == nil {
		return nil
	}
	samples := w.History.Ordered()
	out := make([]ui.ChartPoint, len(samples))
	for i, s := range samples {
		switch field {
		case guestsField:
			out[i] = ui.ChartPoint{Day: s.Day, Values: []float64{float64(s.GuestsOnMountain)}}
		case arrDepField:
			out[i] = ui.ChartPoint{Day: s.Day, Values: []float64{float64(s.ArrivalsToday), float64(s.DeparturesToday)}}
		case cashField:
			out[i] = ui.ChartPoint{Day: s.Day, Values: []float64{float64(s.Cash)}}
		case pnlField:
			out[i] = ui.ChartPoint{Day: s.Day, Values: []float64{
				float64(s.Revenue),
				float64(s.Costs),
				float64(s.Revenue - s.Costs),
			}}
		}
	}
	return out
}

// thoughtChartSeries builds the ChartSeries slice for thought charts by
// iterating over ai.ThoughtLabel in enum order. Any ThoughtKind with a
// non-empty label is included automatically — adding a new thought type
// only requires filling in ThoughtLabel and ThoughtChartColor in ai/types.go.
func thoughtChartSeries() []ui.ChartSeries {
	var out []ui.ChartSeries
	for k := ai.ThoughtKind(1); int(k) < ai.ThoughtKindCount; k++ {
		if ai.ThoughtLabel[k] == "" {
			continue
		}
		c := ai.ThoughtChartColor[k]
		out = append(out, ui.ChartSeries{
			Name:  ai.ThoughtLabel[k],
			Color: mgl32.Vec4{c[0], c[1], c[2], c[3]},
		})
	}
	return out
}

// thoughtValuesFor builds the Values slice for a single ChartPoint from
// counts, iterating in the same order as thoughtChartSeries.
func thoughtValuesFor(counts [ai.ThoughtKindCount]int) []float64 {
	var vals []float64
	for k := ai.ThoughtKind(1); int(k) < ai.ThoughtKindCount; k++ {
		if ai.ThoughtLabel[k] == "" {
			continue
		}
		wt := ai.ThoughtSatisfactionWeight[k]
		if wt < 0 {
			wt = -wt
		}
		vals = append(vals, float64(counts[k])*wt)
	}
	return vals
}

// thoughtsToDistribution returns a single ChartPoint holding the most
// recently completed day's thought counts, or today's in-progress counts
// if no day has been pushed yet.
func thoughtsToDistribution(w *world.World) []ui.ChartPoint {
	if w == nil || w.History == nil {
		return nil
	}
	if samples := w.History.Ordered(); len(samples) > 0 {
		last := samples[len(samples)-1]
		return []ui.ChartPoint{{Day: last.Day, Values: thoughtValuesFor(last.ThoughtCounts)}}
	}
	return []ui.ChartPoint{{Values: thoughtValuesFor(w.History.ThoughtCountsToday)}}
}

// exitThoughtsToDistribution returns a ChartPoint weighted by satisfaction
// impact for exit thoughts — the last thought each departing guest had.
func exitThoughtsToDistribution(w *world.World) []ui.ChartPoint {
	if w == nil || w.History == nil {
		return nil
	}
	if samples := w.History.Ordered(); len(samples) > 0 {
		last := samples[len(samples)-1]
		return []ui.ChartPoint{{Day: last.Day, Values: thoughtValuesFor(last.ExitThoughtCounts)}}
	}
	return []ui.ChartPoint{{Values: thoughtValuesFor(w.History.ExitThoughtCountsToday)}}
}

// weatherToUI maps sim.WeatherState to the UI icon enum.
func weatherToUI(s sim.WeatherState) ui.WeatherKind {
	switch s {
	case sim.WeatherClear:
		return ui.WKSunny
	case sim.WeatherOvercast:
		return ui.WKCloudy
	case sim.WeatherLightSnow:
		return ui.WKSnow
	case sim.WeatherHeavySnow:
		return ui.WKStorm
	case sim.WeatherRain:
		return ui.WKRain
	}
	return ui.WKSunny
}

// cameraSnapshot captures the live orthographic-camera state into a
// save.CameraData so the scene can be reloaded framed exactly as the
// player left it. First-person (perspective) state is excluded — it
// follows a transient skier and resets on load.
func (s *Scenario) cameraSnapshot() *save.CameraData {
	if s.app == nil || s.app.Renderer == nil || s.app.Renderer.Camera == nil {
		return nil
	}
	c := s.app.Renderer.Camera
	return &save.CameraData{
		TargetX:    c.Target[0],
		TargetY:    c.Target[1],
		TargetZ:    c.Target[2],
		Yaw:        c.Yaw,
		Pitch:      c.Pitch,
		OrthoScale: c.OrthoScale,
	}
}

// applyCameraSnapshot writes a saved camera state back onto the
// renderer's live camera. Disables perspective mode in case the save
// was taken before first-person was added; orthographic fields are
// applied verbatim. OrthoScale guard keeps a corrupt or zero-valued
// scale from making the world invisible.
func applyCameraSnapshot(c *render.Camera, cam *save.CameraData) {
	if c == nil || cam == nil {
		return
	}
	c.Perspective = false
	c.Target = mgl32.Vec3{cam.TargetX, cam.TargetY, cam.TargetZ}
	c.Yaw = cam.Yaw
	c.Pitch = cam.Pitch
	if cam.OrthoScale > 0 {
		c.OrthoScale = cam.OrthoScale
	}
	c.Recalculate()
}

// installWorld swaps in a new world and rebuilds renderer state. Tears down
// per-lift meshes for the previous world (if any) before bringing up the new.
func (s *Scenario) installWorld(w *world.World) {
	r := s.app.Renderer
	r.ResetSceneState()
	s.world = w
	seed := globalSimSeed // -seed flag takes priority over testbed default
	if seed == 0 {
		seed = s.simSeed
	}
	if seed != 0 {
		s.simSeed = seed // keep display and rebuild in sync
		s.sim = sim.NewSimulationWithSeed(w, seed)
	} else {
		s.sim = sim.NewSimulation(w)
	}
	// Plow roads and parking lots at each day rollover so freshly-fallen
	// snow doesn't accumulate on asphalt.
	s.sim.OnDayRollover = applyRoadCellState
	if s.queryServer != nil {
		s.sim.QueryServer = s.queryServer
	} else {
		s.sim.QueryServer = globalQueryServer
	}
	if s.debugConsole != nil {
		s.debugConsole.SetSim(s.sim, func() { r.FlushTerrainVerts(s.world.Terrain) })
	}
	// Saved cells already carry every apron / road / corridor stamp that
	// was in effect at save time. Testbeds set their own cell state
	// directly in the builder (no aprons by default). Either way the
	// terrain is authoritative as loaded — no global re-stamp needed.
	r.BuildTerrainMesh(w.Terrain)
	// Stamp sub-cell features into the surface-detail buffer before the
	// initial GPU upload, so tree wells + groom edges are already in
	// the texture by the time the first frame samples it.
	w.Terrain.RestampTreeWells()
	w.Terrain.RecomputeGroomEdges()
	r.BuildSnowSurfaceTex(w.Terrain)
	r.RebuildStaticBatch(w)
	r.RebuildRoads(w)
	for _, lift := range w.Lifts {
		r.AddLiftCable(lift, w.Terrain)
	}
	s.parcelBoundaryDirty = true
}

// openSavePrompt is hooked into the escape menu's Save button. Pops up a
// modal text-input pre-filled with the current saveName (or a fresh
// timestamp when the session has not saved yet). Disabled in testbed mode.
func (s *Scenario) openSavePrompt() {
	if !s.saveAllowed {
		s.setToast("Save disabled in testbed mode")
		return
	}
	initial := s.saveName
	if initial == "" {
		initial = save.DefaultSaveName()
	}
	s.savePrompt = newSavePrompt(initial,
		func(name string) {
			s.savePrompt = nil
			s.commitSave(name)
		},
		func() {
			s.savePrompt = nil
		},
	)
}

// commitSave writes the world to {SavesDir}/{name}.json and updates
// saveName so the next prompt defaults to the same slot.
func (s *Scenario) commitSave(name string) {
	clean := save.SanitizeSaveName(name)
	if clean == "" {
		s.setToast("Save name cannot be empty")
		return
	}
	path, err := save.SaveAs(clean, s.world, s.cameraSnapshot())
	if err != nil {
		s.setToast("Save error: " + err.Error())
		return
	}
	s.saveName = clean
	s.setToast("Saved to " + filepath.Base(path))
}

// gotoLoadMenu is hooked into the escape menu's Load button. Pops back to
// the start menu and pushes the SaveList scene so the user can pick which
// save to resume — uniform with the main-menu Load Game flow.
func (s *Scenario) gotoLoadMenu() {
	app := s.app
	app.PopScene() // pop this scenario back to the start menu
	app.PushScene(NewSaveList())
}

// restartWithSeed rebuilds the testbed world with a fresh RNG seed and
// resets transient scene state (tool, popup, follow, debug overlays) so the
// run starts clean. No-op if rebuild is nil (non-testbed scenarios).
func (s *Scenario) restartWithSeed(seed int64) {
	if s.rebuild == nil {
		return
	}
	s.simSeed = seed
	s.installWorld(s.rebuild(seed))
	s.cancelTool()
	if s.popup != nil {
		s.popup.Visible = false
	}
	s.followGuestID = 0
	s.app.Renderer.SetDebugLines(nil)
	s.syncSpeedButtons()
	s.setToast(fmt.Sprintf("Restarted with seed=%d", seed))
}

// syncSpeedButtons highlights the active speed/pause button on the top bar.
func (s *Scenario) syncSpeedButtons() {
	if s.topBar == nil {
		return
	}
	if s.paused {
		s.topBar.SetPauseActive(true)
		return
	}
	active := -1
	for i, mult := range speedOptions {
		if s.sim.TimeScale == mult {
			active = i
			break
		}
	}
	s.topBar.SetSpeedActive(active)
}

func (s *Scenario) Update(dt float64) {
	s.time += float32(dt)
	inp := s.app.Input
	r := s.app.Renderer

	// Wall-clock FPS, exponentially smoothed. α=0.08 gives a ~0.2 s
	// time constant — steady enough that the displayed number doesn't
	// flicker on a single hitched frame but responsive enough to show
	// real dips. Seeded on first frame so the readout doesn't start at 0.
	if dt > 0 {
		instant := float32(1.0 / dt)
		if s.fpsSmoothed == 0 {
			s.fpsSmoothed = instant
		} else {
			s.fpsSmoothed = s.fpsSmoothed*0.92 + instant*0.08
		}
	}

	// Save prompt is the topmost modal — it captures all input and even
	// swallows Escape (handled inside its TextInput's Cancel binding).
	if s.savePrompt != nil {
		s.savePrompt.HandleInput(inp, float32(r.ScreenWidth()), float32(r.ScreenHeight()))
		return
	}

	// When the lift popup (or any popup with a text-input row) is open,
	// keyboard chars belong to its field — suppress single-letter
	// hotkeys so typing a lift name doesn't also toggle FPV / contour /
	// CSV logging.
	typing := s.popup.WantsKeyboard()

	if inp.Pressed[glfw.KeyGraveAccent] {
		s.debugConsole.Toggle()
		inp.CharInput = inp.CharInput[:0] // swallow the ~ so it isn't typed into the field
	}
	if s.debugConsole.Visible() {
		s.debugConsole.HandleInput(inp)
		return
	}

	if inp.Pressed[glfw.KeyEscape] {
		switch {
		case s.settingsMenu.Visible():
			s.settingsMenu.Hide()
			s.escapeMenu.Show()
		case s.roadEdit.active() || s.structureEdit.active():
			s.roadEdit.clear()
			s.structureEdit.clear()
		case s.activeTool != toolNone:
			s.cancelTool()
		default:
			s.escapeMenu.Toggle()
		}
	}
	if !typing && (inp.Pressed[glfw.KeyDelete] || inp.Pressed[glfw.KeyBackspace]) {
		switch {
		case s.roadEdit.active():
			deleteSelectedRoad(r, s.world, &s.roadEdit)
		case s.structureEdit.active():
			deleteSelectedStructure(r, s.world, &s.structureEdit)
		}
	}
	if s.settingsMenu.Visible() {
		s.settingsMenu.HandleInput(inp)
		return
	}
	if s.escapeMenu.Visible() {
		s.escapeMenu.HandleInput(inp)
		return
	}

	// Tab: cycle the followed skier (first press picks random, subsequent press advances).
	if !typing && inp.Pressed[glfw.KeyTab] {
		s.cycleFollow()
	}

	// V: toggle first-person camera at the followed skier's head. No-op
	// when nobody is followed — FPV without a target would have no
	// anchor.
	if !typing && inp.Pressed[glfw.KeyV] && s.followGuestID != 0 {
		s.firstPerson = !s.firstPerson
	}

	// C: quick toggle for the contour overlay. The full overlay panel is
	// behind the top-bar stack button; this hotkey is kept for muscle
	// memory since contour was the most-used legacy overlay.
	if !typing && inp.Pressed[glfw.KeyC] {
		s.overlayPanel.ToggleBit(render.OverlayContour)
		r.TerrainOverlayMode = s.overlayPanel.Mask()
	}

	// B: toggle the bump-normal debug overlay — paints the perturbed
	// shading normal directly as RGB so the procedural powder/mogul
	// bump map is visible from the testbed without other overlays
	// blending in.
	if !typing && inp.Pressed[glfw.KeyB] {
		s.overlayPanel.ToggleBit(render.OverlayBumpNormal)
		r.TerrainOverlayMode = s.overlayPanel.Mask()
	}

	// N: toggle the surface-detail debug overlay — paints the raw
	// uSnowSurface texture (R=tracks, G=tree wells, B=groom edges) so
	// the 1 m sub-cell buffer is directly visible.
	if !typing && inp.Pressed[glfw.KeyN] {
		s.overlayPanel.ToggleBit(render.OverlaySurfaceDetail)
		r.TerrainOverlayMode = s.overlayPanel.Mask()
	}

	// F3: toggle steering debug overlay (visualises forces for followed skier).
	if inp.Pressed[glfw.KeyF3] {
		s.debugSteering = !s.debugSteering
		if !s.debugSteering {
			r.SetDebugLines(nil)
		}
	}

	// F4: toggle planner debug panel (goal weights, full plan, snapshot
	// anchors). Independent of F3 — the user can run either or both.
	if inp.Pressed[glfw.KeyF4] {
		s.debugPlanner = !s.debugPlanner
	}

	// F5: toggle terrain/snow inspector — reads the cell under the mouse
	// cursor each frame and dumps GroundElevation, SnowAccumulation,
	// Packed, derived VisibleSnowDepth, and the rest of the snow-state
	// scalars. Used to verify save/load round-trips and to spot-check
	// what the wear loop and grooming pass are doing to specific cells.
	if inp.Pressed[glfw.KeyF5] {
		s.debugTerrainIns = !s.debugTerrainIns
	}

	// L: toggle CSV log of the followed skier (debug instrumentation).
	if !typing && inp.Pressed[glfw.KeyL] {
		s.toggleSkierLog()
	}

	// F12: screenshot. Shift+F12: toggle GIF recording.
	if inp.Pressed[glfw.KeyF12] {
		if inp.Held[glfw.KeyLeftShift] || inp.Held[glfw.KeyRightShift] {
			s.toggleGIFRecording(r)
		} else {
			s.pendingScreenshot = true
		}
	}

	// Auto-stop CSV log if the followed skier changed or no longer exists.
	if s.csvRecorder != nil {
		a := s.findFollowedGuest()
		if a == nil || a.ID != s.csvRecorder.GuestID() {
			s.stopSkierLog("Logging stopped: skier no longer followed")
		}
	}

	// Q/E: rotate camera.
	const rotSpeed = float32(90.0) // degrees per second
	rotDelta := float32(0)
	if inp.Held[glfw.KeyQ] {
		rotDelta -= rotSpeed * float32(dt)
	}
	if inp.Held[glfw.KeyE] {
		rotDelta += rotSpeed * float32(dt)
	}
	if rotDelta != 0 {
		// Pivot the rotation around what's currently at the centre of the
		// screen, not Camera.Target. Otherwise rotation visibly drifts when
		// Target sits at Y=0 but the visible terrain is hundreds of metres
		// above — the screen-centre world point moves under the cursor as
		// yaw changes. Snapping Target to the on-screen pivot point each
		// frame keeps the view pinned.
		r.Camera.Target = r.Camera.ScreenCenterOnHeightmap(s.world.Terrain.InterpolatedSurfaceElevationAt)
		r.Camera.Yaw += rotDelta
		r.Camera.Recalculate()
	}

	// Position the bottom tool bar against the current screen height before
	// it handles input — so its hit-tests use the live Y.
	s.toolBar.Y = float32(r.ScreenHeight()) - s.toolBar.H
	s.toolBar.HandleInput(inp, float32(r.ScreenWidth()), float32(r.ScreenHeight()))
	s.topBar.HandleInput(inp, float32(r.ScreenWidth()))

	// Overlay panel — keep its Bottom in sync with the live screen height
	// in case of resize, then route input. The panel owns the bitmask;
	// we mirror it into the renderer each frame so the shader sees the
	// current set.
	s.overlayPanel.Bottom = float32(r.ScreenHeight()) - s.toolBar.H
	s.overlayPanel.HandleInput(inp, float32(r.ScreenWidth()))
	r.TerrainOverlayMode = s.overlayPanel.Mask()

	// Camera pan with right-click drag
	if inp.RightClick {
		s.rightDragging = true
	}
	if !inp.LeftHeld && !inp.RightClick {
		// if no button held this frame reset
	}
	if s.rightDragging {
		if inp.MouseDelta.Len() > 0 {
			s.followGuestID = 0
			dx, dz := r.Camera.PanDelta(inp.MouseDelta)
			r.Camera.Target[0] += dx
			r.Camera.Target[2] += dz
			r.Camera.Recalculate()
		}
	}
	// Stop drag when no buttons held (approximate)
	if inp.MouseDelta.Len() == 0 && !inp.LeftHeld {
		s.rightDragging = false
	}

	// Arrow key pan
	const keyPanSpeed = float32(300) // pixels/sec equivalent
	var keyDelta mgl32.Vec2
	if inp.Held[glfw.KeyLeft] {
		keyDelta[0] -= keyPanSpeed * float32(dt)
	}
	if inp.Held[glfw.KeyRight] {
		keyDelta[0] += keyPanSpeed * float32(dt)
	}
	if inp.Held[glfw.KeyUp] {
		keyDelta[1] += keyPanSpeed * float32(dt)
	}
	if inp.Held[glfw.KeyDown] {
		keyDelta[1] -= keyPanSpeed * float32(dt)
	}
	if keyDelta[0] != 0 || keyDelta[1] != 0 {
		s.followGuestID = 0
		dx, dz := r.Camera.PanDelta(keyDelta)
		r.Camera.Target[0] += dx
		r.Camera.Target[2] += dz
		r.Camera.Recalculate()
	}

	// Zoom with scroll
	if inp.ScrollDelta != 0 {
		r.Camera.OrthoScale -= inp.ScrollDelta * 10
		if r.Camera.OrthoScale < 10 {
			r.Camera.OrthoScale = 10
		}
		if r.Camera.OrthoScale > 500 {
			r.Camera.OrthoScale = 500
		}
		r.Camera.Recalculate()
	}

	// Track terrain hit under mouse — both as a continuous Vec3 for
	// placement and as a grid cell for the cell-based tools (glade brush,
	// remove). hoverValid gates both.
	if !s.barsContain(inp.MousePos[1]) {
		if pos, ok := screenToWorld(r.Camera, s.world.Terrain, inp.MousePos); ok {
			s.hoverWorld = pos
			s.hoverMouseScreen = inp.MousePos
			s.hoverValid = true
			const cellSize = float32(5.0)
			s.hoverCell = [2]int{int(pos[0] / cellSize), int(pos[2] / cellSize)}
		} else {
			s.hoverValid = false
			s.hoverCell = [2]int{-1, -1}
		}
	} else {
		s.hoverValid = false
		s.hoverCell = [2]int{-1, -1}
	}

	// Track hovered parcel for the land-buy tool.
	if s.activeTool == toolLandBuy && s.hoverValid {
		p := s.world.ParcelAt(s.hoverCell[0], s.hoverCell[1])
		if p != nil && p.State == world.ParcelPurchasable {
			s.hoverParcel = p
		} else {
			s.hoverParcel = nil
		}
	} else {
		s.hoverParcel = nil
	}

	// Ghost preview for placement tools — uses the continuous hover so
	// the preview tracks the cursor without snapping.
	updatePlacementGhost(r, s.world.Terrain, placementGhostState{
		activeTool: s.activeTool,
		hoverPos:   mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]},
		hoverValid: s.hoverValid,
		liftBase:   s.liftBase,
		liftType:   s.liftType,
		roadStart:  s.roadStart,
		tint:       s.placementTint(),
	})
	// Highlight every existing road node while the road tool is active so
	// the player can see snap targets. The snap-target node (or the
	// projected snap point on an edge) is rendered brighter. While
	// editing an existing road / structure (toolNone selection), draw
	// the full handle layer instead so the player can see the move
	// target.
	if s.activeTool == toolRoadStart || s.activeTool == toolRoadEnd {
		emitRoadNodeMarkers(r, s.world, mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]}, s.hoverValid)
	} else if s.activeTool == toolNone && s.roadEdit.active() {
		emitRoadEditMarkers(r, s.world, &s.roadEdit)
	} else if s.activeTool == toolNone && s.structureEdit.active() {
		emitStructureEditMarkers(r, s.world, &s.structureEdit)
	}

	// Popup window input — handle before world clicks so its buttons can
	// mark inp.LeftClickConsumed.
	if s.chartWindow != nil && s.chartWindow.Visible {
		s.chartWindow.HandleInput(inp)
		s.topBar.SetChartsActive(s.chartWindow.Visible)
	}
	if s.popup != nil && s.popup.Visible {
		s.popup.HandleInput(inp)
	}

	// World click / drag — glade supports held-down; placement tools use click-only.
	screenW := float32(r.ScreenWidth())
	if !inp.LeftClickConsumed && inp.LeftClick && s.activeTool == toolNone && !s.uiCovers(inp.MousePos[0], inp.MousePos[1], screenW) {
		// Skier pick takes priority over toolNone-level edits and popups.
		if a := s.pickGuest(r.Camera, inp.MousePos); a != nil {
			s.followGuestID = a.ID
			s.activeTrailID = 0
			s.selectedCatID = 0
			s.selectedBuildingID = 0
			s.showCatPath = false
			if s.popup != nil {
				s.popup.Visible = false
			}
			inp.LeftClickConsumed = true
		} else if s.hoverValid {
			// toolNone hit-test cascade: road handle → structure
			// (building / lift) → fall through to the existing popup
			// flow. Each selection mode clears the other so only one
			// thing is highlighted at a time.
			pos := mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]}
			if tryStartRoadEdit(s.world, pos, &s.roadEdit) {
				s.structureEdit.clear()
				if s.popup != nil {
					s.popup.Visible = false
				}
				inp.LeftClickConsumed = true
			} else if tryStartStructureEdit(s.world, pos, &s.structureEdit) {
				s.roadEdit.clear()
				// Let the existing popup flow still open for buildings
				// and lifts — structure edit overlays drag-to-move and
				// Delete on top of the informational popup.
			} else {
				if s.roadEdit.active() {
					s.roadEdit.clear()
				}
				if s.structureEdit.active() {
					s.structureEdit.clear()
				}
			}
		}
	}
	// toolNone drag handling — road or structure depending on which
	// selection is dragging. Release commits the deferred static-batch
	// rebuild.
	if s.activeTool == toolNone {
		if s.roadEdit.dragging {
			if inp.LeftHeld {
				if s.hoverValid {
					dragRoadNode(r, s.world, &s.roadEdit, mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]})
				}
			} else {
				commitRoadDrag(r, s.world)
				s.roadEdit.dragging = false
			}
		}
		if s.structureEdit.dragging {
			if inp.LeftHeld {
				if s.hoverValid {
					dragStructure(r, s.world, &s.structureEdit, mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]})
				}
			} else {
				commitStructureDrag(r, s.world)
				s.structureEdit.dragging = false
			}
		}
	}
	// Glade sliders take input before the brush so dragging the thumb
	// doesn't also paint underneath it.
	sliderActive := false
	if s.activeTool == toolGlade {
		s.layoutGladeSliders(r)
		if s.gladeRadiusSlider.HandleInput(inp) {
			sliderActive = true
		}
		if s.gladeThinSlider.HandleInput(inp) {
			sliderActive = true
		}
	}

	// End-of-stroke reset for drag-paint tools. As soon as the mouse is
	// up, forget the last cell so the next click starts a fresh stroke.
	if !inp.LeftHeld {
		if s.lastGladeCell != [2]int{-1, -1} {
			s.lastGladeCell = [2]int{-1, -1}
		}
		if s.activeTool == toolTrailPaint && s.lastTrailPaintCell != [2]int{-1, -1} {
			s.finishTrailPaintStroke()
			s.lastTrailPaintCell = [2]int{-1, -1}
		}
	}

	// Right-held erase for trail paint.
	if s.activeTool == toolTrailPaint && inp.RightHeld && s.hoverValid {
		gx, gz := s.hoverCell[0], s.hoverCell[1]
		if s.world.Terrain.InBounds(gx, gz) {
			s.applyTrailErase(gx, gz)
			s.lastTrailPaintCell = [2]int{gx, gz}
		}
	}

	if !inp.LeftClickConsumed && !sliderActive {
		// Drag-painting: apply once when the cursor moves into a new cell
		// while held. Requires a prior valid last*Cell — so holding LMB
		// from a toolbar click doesn't auto-apply on entering terrain.
		gladeDragged := s.activeTool == toolGlade && inp.LeftHeld &&
			s.lastGladeCell != [2]int{-1, -1} &&
			s.hoverCell != s.lastGladeCell
		trailDragged := s.activeTool == toolTrailPaint && inp.LeftHeld &&
			s.lastTrailPaintCell != [2]int{-1, -1} &&
			s.hoverCell != s.lastTrailPaintCell
		clickOrDrag := inp.LeftClick || gladeDragged || trailDragged
		if clickOrDrag && !s.uiCovers(inp.MousePos[0], inp.MousePos[1], screenW) && s.hoverValid {
			overSlider := s.activeTool == toolGlade &&
				(s.gladeRadiusSlider.Contains(inp.MousePos[0], inp.MousePos[1]) ||
					s.gladeThinSlider.Contains(inp.MousePos[0], inp.MousePos[1]))
			gx, gz := s.hoverCell[0], s.hoverCell[1]
			if !overSlider && s.world.Terrain.InBounds(gx, gz) {
				if s.activeTool == toolNone && inp.LeftClick {
					s.tryOpenPopup(s.hoverWorld, r.ScreenWidth(), r.ScreenHeight())
				} else if s.activeTool == toolTrailPaint && inp.LeftClick && !trailDragged {
					// In edit mode the active tool captures clicks; no trail-picking.
					s.lastTrailPaintCell = s.hoverCell
					s.applyTool(r)
				} else {
					if s.activeTool == toolGlade {
						s.lastGladeCell = s.hoverCell
					}
					if s.activeTool == toolTrailPaint {
						s.lastTrailPaintCell = s.hoverCell
					}
					s.applyTool(r)
				}
			}
		}
	}

	// Tick simulation (skipped while paused).
	if !s.paused {
		if s.tickHook != nil {
			s.tickHook(s.sim)
		}
		s.sim.Tick(dt)
	}
	if s.sim != nil && s.sim.QueryServer != nil {
		s.sim.QueryServer.Tick(s.world, s.sim)
	}

	// If the sim mutated any cell's snow state (cats grooming, eventually
	// snowfall/decay too), re-upload the terrain vertex buffer so the
	// fragment shader sees the new values. Coalesced to one flush per
	// frame regardless of how many cells changed.
	//
	// FlushSnowState only rewrites the snow-state floats per vertex on
	// the cached vert array — much cheaper than FlushTerrainVerts,
	// which recomputes AO and smoothY for the whole map and was
	// stalling the frame budget every time a cat groomed.
	if s.world.Terrain.SnowDirty {
		r.FlushSnowState(s.world.Terrain)
		// Grooming may have changed (cat passes, brush apply) — recompute
		// the groom-edge mask on the same dirty signal. Cheap enough to
		// run on any SnowDirty flush; the alternative is a separate
		// GroomingDirty flag and the savings aren't worth the bookkeeping.
		s.world.Terrain.RecomputeGroomEdges()
		s.world.Terrain.SnowDirty = false
	}

	// Sub-cell surface-detail texture — uploads only the dirty
	// sub-region, so the per-frame cost is proportional to how much
	// the sim actually touched (typically a handful of pixels per
	// active skier).
	if sd := s.world.Terrain.Surface; sd != nil && sd.Dirty {
		r.FlushSnowSurface(s.world.Terrain)
	}

	// Camera follow: track the selected agent using the freshest positions.
	// In first-person mode, drive the perspective camera to the skier's
	// head and hide their mesh so the camera isn't sitting inside the
	// torso; otherwise stay in the default isometric ortho follow.
	r.HiddenGuestID = 0
	r.HiddenRadius = 0
	if s.followGuestID != 0 {
		if agent := s.findFollowedGuest(); agent != nil {
			if s.firstPerson {
				applyFirstPersonCamera(r.Camera, agent)
				r.HiddenGuestID = agent.ID
				// Lift queues stack all queuers at the same cell, so a
				// queued skier's neighbours sit right in the camera.
				// Hide anyone within ~1.5 m of the followed skier so
				// the FPV doesn't get a green box stuck to its face.
				r.HiddenGuestPos = agent.Pos
				r.HiddenRadius = 1.5
			} else {
				if r.Camera.Perspective {
					r.Camera.Perspective = false
				}
				r.Camera.Target = agent.Pos
				r.Camera.Recalculate()
			}
		} else {
			s.followGuestID = 0
		}
	}
	// Follow dropped (despawn / right-drag pan / etc.) — exit FPV so
	// the free camera comes back in ortho.
	if s.followGuestID == 0 && (s.firstPerson || r.Camera.Perspective) {
		s.firstPerson = false
		r.Camera.Perspective = false
		r.Camera.Recalculate()
	}

	// Track + optional steering overlay for the followed skier.
	s.updateOverlay(r)
}

// updateOverlay maintains the followed skier's track and (optionally) the
// steering-debug visualisation in the debug-line buffer, and separately
// pushes the cell overlay texture (trails + grooming routes) to the renderer.
func (s *Scenario) updateOverlay(r *render.Renderer) {
	// Rebuild parcel fence geometry when parcels change.
	if s.parcelBoundaryDirty {
		postVerts, ropeLines := buildParcelFence(s.world)
		r.SetFencePostVerts(postVerts)
		r.SetBoundaryLines(ropeLines)
		s.parcelBoundaryDirty = false
	}

	// Cell overlay texture (trails + grooming routes) — always updated.
	pix, ow, oh := s.buildCellOverlay()
	r.SetCellOverlay(pix, ow, oh)

	a := s.findFollowedGuest()

	// Reset the track when follow target changes (including dropped follow).
	if a == nil || a.ID != s.trackOwner {
		s.track = s.track[:0]
		s.trackOwner = 0
	}

	var lines []render.DebugLine

	if a != nil {
		if s.trackOwner == 0 {
			s.trackOwner = a.ID
		}

		// Append the latest position when the skier has moved meaningfully and
		// the sim is running. Skipping while paused avoids piling up duplicates.
		if !s.paused {
			add := len(s.track) == 0
			if !add {
				last := s.track[len(s.track)-1]
				dx := a.Pos[0] - last[0]
				dy := a.Pos[1] - last[1]
				dz := a.Pos[2] - last[2]
				if dx*dx+dy*dy+dz*dz >= trackMinSpacing*trackMinSpacing {
					add = true
				}
			}
			if add {
				s.track = append(s.track, a.Pos)
				if len(s.track) > trackMaxPoints {
					s.track = s.track[len(s.track)-trackMaxPoints:]
				}
			}
		}

		lines = make([]render.DebugLine, 0, len(s.track)+8)
		const trackHover = 0.4 // m above terrain so the line is not buried
		const trackR, trackG, trackB = 1.0, 0.55, 0.1 // warm orange
		for i := 1; i < len(s.track); i++ {
			p, q := s.track[i-1], s.track[i]
			lines = append(lines, render.DebugLine{
				A:     mgl32.Vec3{p[0], p[1] + trackHover, p[2]},
				B:     mgl32.Vec3{q[0], q[1] + trackHover, q[2]},
				Color: [3]float32{trackR, trackG, trackB},
			})
		}

		if s.debugSteering && a.TargetID != 0 && !a.Fallen && a.OnLiftID == 0 && !a.Queued {
			if target, ok := skierTarget(s.world, a); ok {
				lines = append(lines, steeringLines(s.world, a, target)...)
			}
		}
	}

	r.SetDebugLines(lines)

	if s.selectedCatID != 0 && s.showCatPath && s.popup != nil && s.popup.Visible {
		r.SetCatPathLines(s.catPathLines())
	} else {
		r.SetCatPathLines(nil)
	}
}

// buildCellOverlay returns an RGBA8 pixel array (w×h, one texel per terrain
// cell) encoding trail, grooming-route, and land-ownership colours. Trails
// are shown when the Trails overlay bit is set, or always when actively
// painting a trail. Land ownership is shown when the land-buy tool is active
// or when parcels exist (so the player always knows their boundary).
// Returns nil when there is nothing to paint.
func (s *Scenario) buildCellOverlay() (pixels []uint8, w, h int) {
	if s.world == nil || s.world.Terrain == nil {
		return nil, 0, 0
	}
	tw := s.world.Terrain.Width
	th := s.world.Terrain.Height

	trailOverlayOn := s.overlayPanel != nil && (s.overlayPanel.Mask()&render.OverlayTrails) != 0
	hasActiveTrail := s.activeTrailID != 0
	hasTrails := len(s.world.Trails) > 0 && (trailOverlayOn || hasActiveTrail)
	hasLandOverlay := s.hoverParcel != nil // hover highlight when buying land
	if !hasTrails && !hasLandOverlay {
		return nil, 0, 0
	}

	pix := make([]uint8, tw*th*4)
	set := func(cx, cz int, r, g, b, a uint8) {
		if cx < 0 || cx >= tw || cz < 0 || cz >= th {
			return
		}
		i := (cz*tw + cx) * 4
		// Blend on top of existing: simple alpha-over compositing.
		srcA := float32(a) / 255.0
		dstA := float32(pix[i+3]) / 255.0
		outA := srcA + dstA*(1-srcA)
		if outA < 1e-4 {
			return
		}
		pix[i+0] = uint8((float32(r)/255.0*srcA + float32(pix[i+0])/255.0*dstA*(1-srcA)) / outA * 255)
		pix[i+1] = uint8((float32(g)/255.0*srcA + float32(pix[i+1])/255.0*dstA*(1-srcA)) / outA * 255)
		pix[i+2] = uint8((float32(b)/255.0*srcA + float32(pix[i+2])/255.0*dstA*(1-srcA)) / outA * 255)
		pix[i+3] = uint8(outA * 255)
	}

	// Hover highlight for land-buy tool — brighter version of the parcel's
	// own palette color so the player knows what they're about to buy.
	if hasLandOverlay {
		pr, pg, pb := parcelPurchasableColor(s.hoverParcel.ID)
		for _, c := range s.hoverParcel.Cells {
			set(c[0], c[1], pr, pg, pb, 200)
		}
	}

	// Trails — shown when overlay is on, or always for the selected/active trail.
	for _, t := range s.world.Trails {
		active := t.ID == s.activeTrailID
		if !trailOverlayOn && !active {
			continue
		}
		var r, g, b, a uint8
		switch t.Difficulty {
		case world.DiffGreen:
			r, g, b = 55, 160, 55
		case world.DiffBlue:
			r, g, b = 55, 120, 210
		default: // DiffBlack
			r, g, b = 60, 60, 60
		}
		if active {
			a = 200 // brighter while editing
		} else {
			a = 130
		}
		for _, c := range t.Cells {
			set(c[0], c[1], r, g, b, a)
		}
		if t.Groomed && !active {
			for _, c := range t.Cells {
				set(c[0], c[1], 255, 255, 255, 40)
			}
		}
	}

	return pix, tw, th
}

// parcelPurchasableColor returns a distinct RGBA overlay color for a purchasable
// parcel, cycling through a small palette keyed by the parcel's ID so that
// adjacent parcels are visually distinguishable.
func parcelPurchasableColor(id uint16) (r, g, b uint8) {
	// Six perceptually-distinct warm/cool hues that contrast with the green
	// owned tint and the red off-limits tint.
	palette := [6][3]uint8{
		{220, 160, 50},  // amber
		{80, 160, 220},  // sky blue
		{200, 100, 200}, // lavender
		{80, 200, 160},  // teal
		{230, 120, 60},  // orange
		{160, 200, 80},  // yellow-green
	}
	c := palette[id%6]
	return c[0], c[1], c[2]
}

// parcelCentroidWorld returns the world-space XZ centroid of a parcel's cells.
func parcelCentroidWorld(p *world.Parcel, t *world.Terrain) (wx, wz float32, ok bool) {
	if len(p.Cells) == 0 {
		return 0, 0, false
	}
	const cs = float32(world.CellSize)
	var sx, sz float64
	for _, c := range p.Cells {
		sx += float64(c[0])
		sz += float64(c[1])
	}
	n := float64(len(p.Cells))
	cx := float32(sx/n+0.5) * cs
	cz := float32(sz/n+0.5) * cs
	return cx, cz, true
}

// buildParcelFence generates square-post triangle geometry and drooping rope
// line segments for the ski-area boundary fence. Returns post vertices (flat
// pos+color triangle list, 6 floats per vertex) and rope DebugLines.
//
// The terrain mesh renders (Width-1)×(Height-1) quads; corner grid runs from
// (0,0) to (Width-1, Height-1). All fence geometry is clamped to this grid.
func buildParcelFence(w *world.World) (postVerts []float32, ropeLines []render.DebugLine) {
	if len(w.Parcels) == 0 {
		return nil, nil
	}
	t := w.Terrain
	const cs = float32(world.CellSize)

	// Build an explicit owned grid: true only for cells in a ParcelOwned parcel.
	// We use this instead of IsAccessible so the fence traces owned land, not
	// "everything that isn't purchasable" (which would surround unowned islands).
	ownedGrid := make([][]bool, t.Width)
	for x := range ownedGrid {
		ownedGrid[x] = make([]bool, t.Height)
	}
	for _, p := range w.Parcels {
		if p.State != world.ParcelOwned {
			continue
		}
		for _, c := range p.Cells {
			if t.InBounds(c[0], c[1]) {
				ownedGrid[c[0]][c[1]] = true
			}
		}
	}
	isOwned := func(cx, cz int) bool {
		if cx < 0 || cx >= t.Width || cz < 0 || cz >= t.Height {
			return false
		}
		return ownedGrid[cx][cz]
	}

	// Post geometry constants.
	const postHW = float32(0.08) // half-width of square cross-section (0.16 m post)
	const postTop = float32(1.5) // height above terrain
	const postBase = float32(0.0)

	// Rope geometry constants.
	const ropeAttach = float32(1.2) // height on post where rope ties
	const ropeSag = float32(0.30)   // max droop at rope centre

	// Two brown shades: slightly lighter for +Z/-Z faces, darker for +X/-X.
	brown1 := [3]float32{0.42, 0.24, 0.10} // front/back
	brown2 := [3]float32{0.32, 0.18, 0.07} // sides
	ropeCol := [3]float32{0.50, 0.22, 0.12}

	elevAt := func(gx, gz int) float32 {
		return t.InterpolatedSurfaceElevationAt(float32(gx)*cs, float32(gz)*cs)
	}
	elevAtWorld := func(wx, wz float32) float32 {
		return t.InterpolatedSurfaceElevationAt(wx, wz)
	}

	// cornerHash returns a stable pseudo-random value in [0,1) for a grid corner.
	// Using the corner's integer grid indices as the seed ensures the same corner
	// always produces the same offset regardless of which fence edge adds it.
	cornerHash := func(gx, gz int) float32 {
		h := uint32(gx)*1234567 + uint32(gz)*7654321
		h ^= h >> 16
		h *= 0x45d9f3b
		h ^= h >> 16
		return float32(h&0xFFFF) / float32(0x10000)
	}

	// quad emits two triangles for a planar quad v0..v3 (v0,v1,v2 and v0,v2,v3).
	quad := func(v0, v1, v2, v3 [3]float32, c [3]float32) {
		for _, v := range [6][3]float32{v0, v1, v2, v0, v2, v3} {
			postVerts = append(postVerts, v[0], v[1], v[2], c[0], c[1], c[2])
		}
	}

	// addPost emits the 4 side faces of a square prism at (px, py, pz).
	addPost := func(px, py, pz float32) {
		hw := postHW
		bot := py + postBase
		top := py + postTop
		// +Z face
		quad([3]float32{px - hw, top, pz + hw}, [3]float32{px + hw, top, pz + hw},
			[3]float32{px + hw, bot, pz + hw}, [3]float32{px - hw, bot, pz + hw}, brown1)
		// -Z face
		quad([3]float32{px + hw, top, pz - hw}, [3]float32{px - hw, top, pz - hw},
			[3]float32{px - hw, bot, pz - hw}, [3]float32{px + hw, bot, pz - hw}, brown1)
		// +X face
		quad([3]float32{px + hw, top, pz - hw}, [3]float32{px + hw, top, pz + hw},
			[3]float32{px + hw, bot, pz + hw}, [3]float32{px + hw, bot, pz - hw}, brown2)
		// -X face
		quad([3]float32{px - hw, top, pz + hw}, [3]float32{px - hw, top, pz - hw},
			[3]float32{px - hw, bot, pz - hw}, [3]float32{px - hw, bot, pz + hw}, brown2)
	}

	// jitteredPost returns the world-space XZ position of a post at grid corner
	// (gx, gz). The offset is driven purely by the corner index so every edge
	// that shares the corner places its post at the same spot (corners line up).
	// Two independent hashes give uncorrelated X and Z displacements of ±maxJitter.
	const maxJitter = float32(1.50)
	jitteredPost := func(gx, gz int, basePx, basePz float32) (wx, wz float32) {
		dx := (cornerHash(gx, gz) - 0.5) * 2 * maxJitter
		dz := (cornerHash(gx*997+1, gz*991+1) - 0.5) * 2 * maxJitter
		return basePx + dx, basePz + dz
	}

	// addEdge emits posts at both ends and a 3-segment drooping rope between them.
	// gxA/gzA and gxB/gzB are the integer grid corner indices used for jitter.
	addEdge := func(ax, ay, az, bx, by, bz float32, gxA, gzA, gxB, gzB int) {
		pAx, pAz := jitteredPost(gxA, gzA, ax, az)
		pAy := elevAtWorld(pAx, pAz)
		pBx, pBz := jitteredPost(gxB, gzB, bx, bz)
		pBy := elevAtWorld(pBx, pBz)

		addPost(pAx, pAy, pAz)
		addPost(pBx, pBy, pBz)

		// Rope: attach at ropeAttach height, 3 segments with parabolic sag.
		ry0 := pAy + ropeAttach
		ry1 := pBy + ropeAttach
		lerp := func(a, b, t float32) float32 { return a + (b-a)*t }
		sag := func(t float32) float32 { return ropeSag * 4 * t * (1 - t) }

		rA := mgl32.Vec3{pAx, ry0, pAz}
		rM1 := mgl32.Vec3{lerp(pAx, pBx, 1.0/3), lerp(ry0, ry1, 1.0/3) - sag(1.0/3), lerp(pAz, pBz, 1.0/3)}
		rM2 := mgl32.Vec3{lerp(pAx, pBx, 2.0/3), lerp(ry0, ry1, 2.0/3) - sag(2.0/3), lerp(pAz, pBz, 2.0/3)}
		rB := mgl32.Vec3{pBx, ry1, pBz}

		ropeLines = append(ropeLines,
			render.DebugLine{A: rA, B: rM1, Color: ropeCol},
			render.DebugLine{A: rM1, B: rM2, Color: ropeCol},
			render.DebugLine{A: rM2, B: rB, Color: ropeCol},
		)
	}

	// Main pass with direction-specific bounds guards (see inline comments).
	for cx := 0; cx < t.Width; cx++ {
		for cz := 0; cz < t.Height; cz++ {
			if !isOwned(cx, cz) {
				continue
			}
			if cx < t.Width-1 && cz < t.Height-1 && !isOwned(cx+1, cz) {
				ex := float32(cx+1) * cs
				addEdge(ex, elevAt(cx+1, cz), float32(cz)*cs,
					ex, elevAt(cx+1, cz+1), float32(cz+1)*cs,
					cx+1, cz, cx+1, cz+1)
			}
			if cz < t.Height-1 && !isOwned(cx-1, cz) {
				ex := float32(cx) * cs
				addEdge(ex, elevAt(cx, cz), float32(cz)*cs,
					ex, elevAt(cx, cz+1), float32(cz+1)*cs,
					cx, cz, cx, cz+1)
			}
			if cz < t.Height-1 && cx < t.Width-1 && !isOwned(cx, cz+1) {
				ez := float32(cz+1) * cs
				addEdge(float32(cx)*cs, elevAt(cx, cz+1), ez,
					float32(cx+1)*cs, elevAt(cx+1, cz+1), ez,
					cx, cz+1, cx+1, cz+1)
			}
			if cx < t.Width-1 && !isOwned(cx, cz-1) {
				ez := float32(cz) * cs
				addEdge(float32(cx)*cs, elevAt(cx, cz), ez,
					float32(cx+1)*cs, elevAt(cx+1, cz), ez,
					cx, cz, cx+1, cz)
			}
		}
	}

	// Right map-edge.
	for cz := 0; cz < t.Height-1; cz++ {
		if isOwned(t.Width-1, cz) {
			ex := float32(t.Width-1) * cs
			addEdge(ex, elevAt(t.Width-1, cz), float32(cz)*cs,
				ex, elevAt(t.Width-1, cz+1), float32(cz+1)*cs,
				t.Width-1, cz, t.Width-1, cz+1)
		}
	}

	// Bottom map-edge.
	for cx := 0; cx < t.Width-1; cx++ {
		if isOwned(cx, t.Height-1) {
			ez := float32(t.Height-1) * cs
			addEdge(float32(cx)*cs, elevAt(cx, t.Height-1), ez,
				float32(cx+1)*cs, elevAt(cx+1, t.Height-1), ez,
				cx, t.Height-1, cx+1, t.Height-1)
		}
	}

	return postVerts, ropeLines
}

// steeringLines builds the F3 steering-debug visualisation for the agent.
func steeringLines(w *world.World, a *world.Guest, target mgl32.Vec3) []render.DebugLine {
	d := sim.ComputeSteeringDebug(w, a, target)
	origin := mgl32.Vec3{a.Pos[0], a.Pos[1] + 1.5, a.Pos[2]}
	mk := func(dir mgl32.Vec2, length float32, color [3]float32) render.DebugLine {
		return render.DebugLine{
			A:     origin,
			B:     mgl32.Vec3{origin[0] + dir[0]*length, origin[1], origin[2] + dir[1]*length},
			Color: color,
		}
	}
	lines := make([]render.DebugLine, 0, 5)
	if d.FallLine.Len() > 1e-4 {
		lines = append(lines, mk(d.FallLine, 10, [3]float32{0.1, 0.9, 1.0}))
	}
	hx := float32(math.Sin(float64(d.DesiredHead)))
	hz := float32(math.Cos(float64(d.DesiredHead)))
	lines = append(lines, mk(mgl32.Vec2{hx, hz}, 10, [3]float32{1.0, 0.95, 0.1}))
	for _, p := range d.Probes {
		length := d.ProbeDist * (0.4 + 0.6*p.Density)
		shade := 0.4 + 0.6*p.Density
		lines = append(lines, mk(p.Dir, length, [3]float32{shade, 0.15, 0.15}))
	}
	return lines
}

// skierTarget returns the world-space target the agent is currently
// steering toward, resolved from the agent's TargetID against either the
// world's lifts or buildings.
func skierTarget(w *world.World, a *world.Guest) (mgl32.Vec3, bool) {
	if a.TargetID == 0 {
		return mgl32.Vec3{}, false
	}
	for _, lift := range w.Lifts {
		if lift.ID == a.TargetID {
			cell := lift.QueueCell()
			return mgl32.Vec3{lift.Base[0], w.Terrain.SurfaceElevationAt(cell[0], cell[1]), lift.Base[1]}, true
		}
	}
	for _, b := range w.Buildings {
		if b.ID == a.TargetID {
			cell := b.DoorCell()
			return mgl32.Vec3{b.Pos[0], w.Terrain.SurfaceElevationAt(cell[0], cell[1]), b.Pos[1]}, true
		}
	}
	return mgl32.Vec3{}, false
}

const (
	gladeRadius        = 2     // cells
	buildingPickRadius = 7.0   // metres — ~one cell width, matches default lodge footprint
	liftPickRadius     = 5.0   // metres — clicks within this of the base register as a hit
)

// applyTool dispatches the active placement / editing tool. Building and
// lift placement read the continuous hover position; brush tools read the
// cell version. Caller guarantees s.hoverValid before invoking.
func (s *Scenario) applyTool(r *render.Renderer) {
	defer s.syncToolButtons()
	w := s.world
	gx, gz := s.hoverCell[0], s.hoverCell[1]
	wx, wz := s.hoverWorld[0], s.hoverWorld[2]
	switch s.activeTool {
	case toolBuilding:
		if !w.Terrain.IsAccessible(gx, gz) {
			s.setToast("Can't build on land you don't own")
			return
		}
		if w.Cash < world.LodgeCost {
			s.setToast(fmt.Sprintf("Need $%d for a lodge — short by $%d",
				world.LodgeCost, world.LodgeCost-w.Cash))
			return
		}
		if w.BuildingOverlap(world.BuildingLodge, wx, wz) {
			s.setToast("Can't place a lodge here — overlaps another building")
			return
		}
		w.Cash -= world.LodgeCost
		b := w.PlaceBuildingType(world.BuildingLodge, wx, wz)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
	case toolTicketOffice:
		if !w.Terrain.IsAccessible(gx, gz) {
			s.setToast("Can't build on land you don't own")
			return
		}
		if w.Cash < world.TicketOfficeCost {
			s.setToast(fmt.Sprintf("Need $%d for a ticket office — short by $%d",
				world.TicketOfficeCost, world.TicketOfficeCost-w.Cash))
			return
		}
		if w.BuildingOverlap(world.BuildingTicketOffice, wx, wz) {
			s.setToast("Can't place a ticket office here — overlaps another building")
			return
		}
		w.Cash -= world.TicketOfficeCost
		b := w.PlaceBuildingType(world.BuildingTicketOffice, wx, wz)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
	case toolShed:
		if !w.Terrain.IsAccessible(gx, gz) {
			s.setToast("Can't build on land you don't own")
			return
		}
		if w.Cash < world.ShedCost {
			s.setToast(fmt.Sprintf("Need $%d for a shed — short by $%d",
				world.ShedCost, world.ShedCost-w.Cash))
			return
		}
		if w.BuildingOverlap(world.BuildingShed, wx, wz) {
			s.setToast("Can't place a shed here — overlaps another building")
			return
		}
		w.Cash -= world.ShedCost
		b := w.PlaceBuildingType(world.BuildingShed, wx, wz)
		s.sim.InvalidateSections()
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
	case toolParking:
		if !w.Terrain.IsAccessible(gx, gz) {
			s.setToast("Can't build on land you don't own")
			return
		}
		if w.Cash < world.ParkingCost {
			s.setToast(fmt.Sprintf("Need $%d for a parking lot — short by $%d",
				world.ParkingCost, world.ParkingCost-w.Cash))
			return
		}
		if w.BuildingOverlap(world.BuildingParking, wx, wz) {
			s.setToast("Can't place a parking lot here — overlaps another building")
			return
		}
		w.Cash -= world.ParkingCost
		b := w.PlaceBuildingType(world.BuildingParking, wx, wz)
		w.EnsureParkingDriveway(b)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
		r.RebuildRoads(w)
	case toolPatrolHut:
		if !w.Terrain.IsAccessible(gx, gz) {
			s.setToast("Can't build on land you don't own")
			return
		}
		if w.Cash < world.PatrolHutCost {
			s.setToast(fmt.Sprintf("Need $%d for a patrol hut — short by $%d",
				world.PatrolHutCost, world.PatrolHutCost-w.Cash))
			return
		}
		if w.BuildingOverlap(world.BuildingPatrolHut, wx, wz) {
			s.setToast("Can't place a patrol hut here — overlaps another building")
			return
		}
		w.Cash -= world.PatrolHutCost
		b := w.PlaceBuildingType(world.BuildingPatrolHut, wx, wz)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
	case toolSnowGun:
		if !w.Terrain.IsAccessible(gx, gz) {
			s.setToast("Can't build on land you don't own")
			return
		}
		if w.Cash < world.SnowGunCost {
			s.setToast(fmt.Sprintf("Need $%d for a snow gun — short by $%d",
				world.SnowGunCost, world.SnowGunCost-w.Cash))
			return
		}
		if w.BuildingOverlap(world.BuildingSnowGun, wx, wz) {
			s.setToast("Can't place a snow gun here — overlaps another building")
			return
		}
		w.Cash -= world.SnowGunCost
		w.PlaceBuildingType(world.BuildingSnowGun, wx, wz)
		r.RebuildStaticBatch(w)
	case toolGlade:
		// Slider value 0–10 = % density delta per application. Each click
		// or drag-into-new-cell event fires one application; stationary
		// holding does nothing further (see lastGladeCell gate in Update).
		strength := s.gladeThinSlider.Value / 100
		cost := gladeStrokeCost(w.Terrain, gx, gz, s.gladeBrushRadius(), strength)
		if cost > 0 && w.Cash < cost {
			s.setToast(fmt.Sprintf("Need $%d to clear here — short by $%d", cost, cost-w.Cash))
			return
		}
		w.Cash -= cost
		applyDensityBrush(w.Terrain, gx, gz, s.gladeBrushRadius(), -strength)
		w.Terrain.RestampTreeWells()
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
	case toolLiftBase:
		if !w.Terrain.IsAccessible(gx, gz) {
			s.setToast("Can't place a lift on land you don't own")
			return
		}
		minCost := world.LiftStationCost
		if s.liftType == world.LiftHeli {
			minCost = world.HelipadCost / 2
		}
		if w.Cash < minCost {
			s.setToast(fmt.Sprintf("Need $%d — short by $%d", minCost, minCost-w.Cash))
			return
		}
		s.liftBase = mgl32.Vec2{wx, wz}
		s.activeTool = toolLiftTop
	case toolLiftTop:
		if !w.Terrain.IsAccessible(gx, gz) {
			s.setToast("Can't place a lift on land you don't own")
			return
		}
		top := mgl32.Vec2{wx, wz}
		var cost int
		if s.liftType == world.LiftHeli {
			cost = world.HelipadCost
		} else {
			cost = world.LiftCost(s.liftBase, top)
		}
		if w.Cash < cost {
			s.setToast(fmt.Sprintf("Need $%d — short by $%d", cost, cost-w.Cash))
			return
		}
		w.Cash -= cost
		lift := w.PlaceLift(s.liftType, s.liftBase[0], s.liftBase[1], wx, wz)
		if lift.IsHeli() {
			applyHelipadPlacementEffects(w.Terrain, lift)
		} else {
			applyLiftPlacementEffects(w.Terrain, lift)
		}
		r.FlushTerrainVerts(w.Terrain)
		r.AddLiftCable(lift, w.Terrain) // no-op for helilifts
		r.RebuildStaticBatch(w)
		r.ClearAllGhosts()
		r.ClearGhostCable()
		s.activeTool = toolNone
	case toolRoadStart:
		// Cheapest a road can be is the base cost — gate entry so the
		// player can't waste a click on a start they couldn't afford
		// the cheapest follow-up from.
		if w.Cash < world.RoadBaseCost {
			s.setToast(fmt.Sprintf("Need $%d to start a road — short by $%d",
				world.RoadBaseCost, world.RoadBaseCost-w.Cash))
			return
		}
		s.roadStart = resolveRoadEndpoint(w, mgl32.Vec2{wx, wz}).pos
		s.activeTool = toolRoadEnd
	case toolRoadEnd:
		// Re-resolve the start so node/edge context is recovered before
		// committing — the stored s.roadStart is just the post-snap
		// position (deterministic since the graph hasn't changed mid-flow).
		start := resolveRoadEndpoint(w, s.roadStart)
		end := resolveRoadEndpoint(w, mgl32.Vec2{wx, wz})
		// Refuse a degenerate same-spot click before charging the player.
		if end.pos == start.pos {
			s.setToast("Click somewhere else to finish the road")
			return
		}
		cost := world.RoadCost(start.pos, end.pos)
		if w.Cash < cost {
			s.setToast(fmt.Sprintf("Need $%d for this road — short by $%d",
				cost, cost-w.Cash))
			return
		}
		if placeRoadSegment(w, start, end) == nil {
			// Endpoints collapsed to the same node — refunded by not
			// deducting cost.
			s.setToast("Road endpoints snapped together — no segment placed")
			return
		}
		w.Cash -= cost
		applyRoadCellState(w)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
		r.RebuildRoads(w)
		// Continue the chain from the just-placed endpoint — the player
		// stays in toolRoadEnd and the next click extends the road. Esc
		// or re-clicking the Road button drops them out. The ghost
		// preview will redraw with the new roadStart on the next frame's
		// updatePlacementGhost call, so we don't clear it here.
		s.roadStart = end.pos
	case toolRemove:
		s.removeAt(s.hoverWorld, r)
	case toolTrailPaint:
		if s.trailEraseMode {
			s.applyTrailErase(gx, gz)
		} else {
			s.applyTrailPaint(gx, gz)
		}
		s.lastTrailPaintCell = [2]int{gx, gz}
	case toolLandBuy:
		s.openLandBuyPopup(gx, gz, r.ScreenWidth(), r.ScreenHeight())
	}
}

// trailPaintBrushRadius is the half-width of the trail-paint brush in cells.
const trailPaintBrushRadius = 2

// activateTrailTool enters trail-paint mode for a new trail. Re-clicking
// the Trail button while already in paint mode deactivates the tool.
func (s *Scenario) activateTrailTool() {
	if s.activeTool == toolTrailPaint {
		s.cancelTool()
		return
	}
	t := s.world.PlaceTrail("", s.trailDifficulty)
	s.activeTrailID = t.ID
	s.trailEraseMode = false
	s.lastTrailPaintCell = [2]int{-1, -1}
	s.activeTool = toolTrailPaint
	s.syncToolButtons()
	s.setToast("Drag to add cells. Right-drag to remove. Esc to finish.")
}

// applyTrailPaint adds cells under the brush to the active trail.
func (s *Scenario) applyTrailPaint(gx, gz int) {
	if s.activeTrailID == 0 {
		return
	}
	cells := world.BrushCells(gx, gz, trailPaintBrushRadius)
	var valid [][2]int
	for _, c := range cells {
		if s.world.Terrain.InBounds(c[0], c[1]) {
			valid = append(valid, c)
		}
	}
	if len(valid) > 0 {
		s.world.AddTrailCells(s.activeTrailID, valid)
	}
}

// applyTrailErase removes cells under the brush from the active trail.
func (s *Scenario) applyTrailErase(gx, gz int) {
	if s.activeTrailID == 0 {
		return
	}
	cells := world.BrushCells(gx, gz, trailPaintBrushRadius)
	s.world.RemoveTrailCells(s.activeTrailID, cells)
}

// finishTrailPaintStroke rebuilds the TrailGraph after a drag-paint stroke ends
// and invalidates section assignments since column counts may have changed.
func (s *Scenario) finishTrailPaintStroke() {
	s.world.RebuildTrailGraph()
	s.sim.InvalidateSections()
}


// findBuilding returns the building with the given ID, or nil.
func (s *Scenario) findBuilding(id uint64) *world.Building {
	for _, b := range s.world.Buildings {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// gladeBrushRadius reads the glade radius slider, clamped to a sane min.
func (s *Scenario) gladeBrushRadius() int {
	if s.gladeRadiusSlider == nil {
		return gladeRadius
	}
	v := int(s.gladeRadiusSlider.Value + 0.5)
	if v < 1 {
		v = 1
	}
	return v
}

// layoutGladeSliders positions the glade-tool sliders on the left edge,
// vertically centred below the top bar. Recomputed each frame so they
// track window resizes. Mirrors the editor's layoutBrushSliders.
func (s *Scenario) layoutGladeSliders(r *render.Renderer) {
	const trackW = float32(18)
	const trackH = float32(200)
	sh := float32(r.ScreenHeight())
	y := (sh-trackH)/2 + 20
	s.gladeRadiusSlider.X = 28
	s.gladeRadiusSlider.Y = y
	s.gladeRadiusSlider.W = trackW
	s.gladeRadiusSlider.H = trackH
	s.gladeThinSlider.X = 80
	s.gladeThinSlider.Y = y
	s.gladeThinSlider.W = trackW
	s.gladeThinSlider.H = trackH
}

// removeAt removes the building or lift closest to clickPos within the
// pick radius. Lift removal isn't wired here yet — the existing tool
// only deletes buildings. Once `world.RemoveLift` is plumbed through the
// renderer it should fall under the same proximity test.
func (s *Scenario) removeAt(clickPos mgl32.Vec3, r *render.Renderer) {
	w := s.world
	pick := mgl32.Vec2{clickPos[0], clickPos[2]}
	for _, b := range w.Buildings {
		if b.Pos.Sub(pick).Len() <= buildingPickRadius {
			// Parking lots own a driveway road node + possibly incident
			// edges; RemoveBuilding drops both, so the road mesh has to
			// be regenerated when one comes out.
			wasParking := b.Type == world.BuildingParking
			wasShed := b.Type == world.BuildingShed
			w.RemoveBuilding(b.ID)
			if wasShed {
				s.sim.InvalidateSections()
			}
			// RemoveBuilding restores Passable on the door cell; we
			// intentionally do NOT revert the apron's ground / snow / tree
			// effects (no Natural baseline to revert TO). The graded pad
			// just stays where the building was — the player can paint
			// trees back or re-stamp later.
			r.FlushTerrainVerts(w.Terrain)
			r.RebuildStaticBatch(w)
			if wasParking {
				r.RebuildRoads(w)
			}
			return
		}
	}
}

func (s *Scenario) Render(r *render.Renderer) {
	const cellSize = float32(5.0)
	t := s.world.Terrain
	switch {
	case s.activeTool == toolGlade && t.InBounds(s.hoverCell[0], s.hoverCell[1]):
		gx, gz := s.hoverCell[0], s.hoverCell[1]
		center := mgl32.Vec2{float32(gx)*cellSize + cellSize/2, float32(gz)*cellSize + cellSize/2}
		r.SetBrush(center, (float32(s.gladeBrushRadius())+0.5)*cellSize)
	default:
		r.ClearBrush()
	}

	// Snow gun range ring — shown during placement hover and when the gun's popup is open.
	rangeShown := false
	if s.activeTool == toolSnowGun && s.hoverValid {
		r.SetRange(mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]}, world.SnowGunRangeM)
		rangeShown = true
	} else if s.popup != nil && s.popup.Visible && s.selectedBuildingID != 0 {
		for _, b := range s.world.Buildings {
			if b.ID == s.selectedBuildingID && b.Type == world.BuildingSnowGun {
				r.SetRange(b.Pos, world.SnowGunRangeM)
				rangeShown = true
				break
			}
		}
	}
	if !rangeShown {
		r.ClearRange()
	}

	r.HighlightGuestID = s.followGuestID
	if s.selectedCatID != 0 && s.popup != nil && s.popup.Visible {
		r.HighlightCatID = s.selectedCatID
	} else {
		r.HighlightCatID = 0
	}
	s.applyPerceptionCone(r)
	r.WeatherOverlay = int(s.sim.Weather.Today().State)
	r.DrawWorld(s.world, s.time)
	r.ClearBrush()
	r.ClearRange()
	r.ClearPerceptionCone()

	// Re-anchor the bottom tool bar to the live screen height before draw.
	s.toolBar.Y = float32(r.ScreenHeight()) - s.toolBar.H
	if s.overlayPanel != nil {
		s.overlayPanel.Bottom = float32(r.ScreenHeight()) - s.toolBar.H
	}
	drawables := []render.UIDrawable{s.topBar, s.toolBar, s.overlayPanel}

	// Parcel price labels — shown when the land-buy tool is active so the
	// player can see what each purchasable parcel costs before clicking.
	if s.activeTool == toolLandBuy && len(s.world.Parcels) > 0 && r.Font != nil {
		w := s.world
		drawables = append(drawables, uiDrawFunc(func(rr *render.Renderer) {
			for i := range w.Parcels {
				p := &w.Parcels[i]
				if p.State != world.ParcelPurchasable {
					continue
				}
				cx, cz, ok := parcelCentroidWorld(p, w.Terrain)
				if !ok {
					continue
				}
				elev := w.Terrain.InterpolatedSurfaceElevationAt(cx, cz)
				sx, sy, vis := rr.WorldToScreen(mgl32.Vec3{cx, elev + 4, cz})
				if !vis {
					continue
				}
				label := fmt.Sprintf("$%d", p.Price)
				tw := rr.Font.TextWidth(label)
				pr, pg, pb := parcelPurchasableColor(p.ID)
				col := mgl32.Vec4{float32(pr) / 255, float32(pg) / 255, float32(pb) / 255, 1}
				rr.Font.DrawText(rr, label, sx-tw/2, sy-float32(render.GlyphH)/2, col)
			}
		}))
	}

	if s.simSeed != 0 {
		drawables = append(drawables, &seedLabel{seed: s.simSeed})
	}
	if s.activeTool == toolGlade {
		s.layoutGladeSliders(r)
		drawables = append(drawables, s.gladeRadiusSlider, s.gladeThinSlider)
	}
	if s.activeTool == toolLiftTop {
		drawables = append(drawables, &hintLabel{text: "Click to set lift top"})
		if s.hoverValid {
			drawables = append(drawables, &liftDropLabel{
				terrain: s.world.Terrain,
				base:    s.liftBase,
				hover:   mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]},
			})
		}
	}
	if cost, affordable, _, valid := s.placementCost(); valid {
		drawables = append(drawables, &costLabel{cost: cost, affordable: affordable})
	}
	for _, a := range s.world.OnMountain {
		if a.ID == s.followGuestID {
			drawables = append(drawables, &followLabel{
				world:   s.world,
				agent:   a,
				simTime: s.sim.SimTime,
			})
			if s.debugPlanner {
				drawables = append(drawables, &plannerDebugPanel{
					world: s.world,
					agent: a,
				})
			}
			break
		}
	}
	if s.overlayPanel != nil && (s.overlayPanel.Mask()&render.OverlaySnowDepth) != 0 &&
		s.hoverValid && s.world != nil && s.world.Terrain != nil &&
		s.world.Terrain.InBounds(s.hoverCell[0], s.hoverCell[1]) {
		cell := s.world.Terrain.Cells[s.hoverCell[0]][s.hoverCell[1]]
		drawables = append(drawables, &snowLayerTooltip{
			base:    cell.Base,
			top:     cell.Top,
			mouseX:  s.hoverMouseScreen[0],
			mouseY:  s.hoverMouseScreen[1],
			screenW: r.ScreenWidth(),
			screenH: r.ScreenHeight(),
		})
	}
	if s.overlayPanel != nil && (s.overlayPanel.Mask()&render.OverlayGrooming) != 0 &&
		s.hoverValid && s.world != nil && s.world.Terrain != nil &&
		s.world.Terrain.InBounds(s.hoverCell[0], s.hoverCell[1]) {
		cell := s.world.Terrain.Cells[s.hoverCell[0]][s.hoverCell[1]]
		drawables = append(drawables, &groomingTooltip{
			grooming: cell.Grooming,
			mouseX:   s.hoverMouseScreen[0],
			mouseY:   s.hoverMouseScreen[1],
			screenW:  r.ScreenWidth(),
			screenH:  r.ScreenHeight(),
		})
	}
	if s.debugTerrainIns && s.world != nil && s.world.Terrain != nil &&
		s.world.Terrain.InBounds(s.hoverCell[0], s.hoverCell[1]) {
		drawables = append(drawables, &terrainInspectPanel{
			terrain:  s.world.Terrain,
			cell:     s.hoverCell,
			world:    s.hoverWorld,
			fps:      s.fpsSmoothed,
			updateMs: s.app.LastUpdateMs,
			renderMs: s.app.LastRenderMs,
		})
	}
	if s.popup != nil && s.popup.Visible {
		drawables = append(drawables, s.popup)
	}
	if s.chartWindow != nil && s.chartWindow.Visible {
		drawables = append(drawables, s.chartWindow)
	}
	if s.settingsMenu.Visible() {
		drawables = append(drawables, s.settingsMenu)
	}
	if s.debugConsole.Visible() {
		drawables = append(drawables, s.debugConsole)
	}
	if s.escapeMenu.Visible() {
		drawables = append(drawables, s.escapeMenu)
	}
	if s.toastText != "" && s.time < s.toastExpiry {
		drawables = append(drawables, &toastLabel{text: s.toastText})
	}
	if s.savePrompt != nil {
		drawables = append(drawables, s.savePrompt)
	}
	if s.world != nil && len(s.world.Trails) > 0 {
		showAll := s.overlayPanel != nil && (s.overlayPanel.Mask()&render.OverlayTrails) != 0
		drawables = append(drawables, &trailLabels{
			world:         s.world,
			activeTrailID: s.activeTrailID,
			showAll:       showAll,
		})
	}
	// GIF capture happens before UI so toasts and overlays don't appear in
	// recorded frames. Subsample every gifFrameInterval rendered frames.
	if s.gifRecording {
		s.gifFrameCount++
		if s.gifFrameCount%gifFrameInterval == 0 {
			if frame := r.ReadFrame(); frame != nil {
				// Scale to half resolution before dithering — 4× fewer pixels.
				b := frame.Bounds()
				half := image.Rect(0, 0, b.Dx()/2, b.Dy()/2)
				small := image.NewNRGBA(half)
				xdraw.BiLinear.Scale(small, half, frame, b, xdraw.Over, nil)
				p := image.NewPaletted(half, palette.Plan9)
				draw.FloydSteinberg.Draw(p, half, small, image.Point{})
				s.gifFrames = append(s.gifFrames, p)
			}
			if len(s.gifFrames) >= gifMaxFrames {
				s.finishGIF()
			}
		}
	}

	r.DrawUI(drawables)

	// Screenshot is taken AFTER UI is drawn so the captured frame includes UI.
	// The "saved" toast is set after capture so it appears only on subsequent
	// frames, never in the screenshot itself.
	if s.pendingScreenshot {
		s.pendingScreenshot = false
		path := filepath.Join("debug", "screens", time.Now().Format("20060102-150405")+".png")
		if err := r.SaveScreenshot(path); err != nil {
			s.setToast("Screenshot failed: " + err.Error())
		} else {
			s.setToast("Screenshot saved → " + path)
		}
	}

	// Drain result from background GIF encode goroutine.
	if s.gifResult != nil {
		select {
		case msg := <-s.gifResult:
			s.setToast(msg)
			s.gifResult = nil
		default:
		}
	}
}

func (s *Scenario) Destroy() {
	if s.app != nil && s.app.Renderer != nil {
		s.app.Renderer.ResetSceneState()
	}
}

const (
	gifFrameInterval = 3   // capture every Nth rendered frame (~20fps at 60fps)
	gifMaxFrames     = 600 // hard cap: ~30s at 20fps before auto-stop
	gifCentiseconds  = 5   // delay per frame in 1/100s (gifFrameInterval/60fps ≈ 5cs)
)

// toggleGIFRecording starts or stops a GIF capture session.
func (s *Scenario) toggleGIFRecording(r *render.Renderer) {
	if s.gifRecording {
		s.finishGIF()
	} else {
		s.gifRecording = true
		s.gifFrames = nil
		s.gifFrameCount = 0
		s.gifResult = make(chan string, 1)
		s.setToast("GIF recording — Shift+F12 to stop")
	}
}

// finishGIF stops recording and encodes the accumulated frames to disk in a
// background goroutine so the game doesn't freeze. The result is reported via
// s.gifResult which Draw() drains each frame.
func (s *Scenario) finishGIF() {
	s.gifRecording = false
	frames := s.gifFrames
	s.gifFrames = nil
	if len(frames) == 0 {
		s.setToast("GIF: no frames captured")
		return
	}

	s.setToast(fmt.Sprintf("GIF encoding %d frames…", len(frames)))
	result := s.gifResult

	go func() {
		delays := make([]int, len(frames))
		for i := range delays {
			delays[i] = gifCentiseconds
		}

		path := filepath.Join("debug", "screens", time.Now().Format("20060102-150405")+".gif")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			result <- "GIF failed: " + err.Error()
			return
		}
		f, err := os.Create(path)
		if err != nil {
			result <- "GIF failed: " + err.Error()
			return
		}
		defer f.Close()

		if err := gif.EncodeAll(f, &gif.GIF{Image: frames, Delay: delays}); err != nil {
			result <- "GIF encode failed: " + err.Error()
			return
		}
		result <- fmt.Sprintf("GIF saved (%d frames) → %s", len(frames), path)
	}()
}

// tryOpenPopup opens a popup window if the click is within the pick
// radius of a building or any part of a lift (base, top, or anywhere
// along the cable line). Buildings have priority — clicks inside a
// building's footprint open the lodge panel even if a lift happens to
// be near. The cable check uses point-to-segment distance so towers
// (which sit on the cable line) and stations are all selectable.
func (s *Scenario) tryOpenPopup(clickPos mgl32.Vec3, screenW, screenH int) {
	pick := mgl32.Vec2{clickPos[0], clickPos[2]}
	const catPickRadius2 = float32(5.0 * 5.0)
	for _, cat := range s.world.Snowcats {
		dx := cat.Pos[0] - pick[0]
		dz := cat.Pos[2] - pick[1]
		if dx*dx+dz*dz <= catPickRadius2 {
			s.openSnowcatPopup(cat, screenW, screenH)
			return
		}
	}
	for _, b := range s.world.Buildings {
		if b.Pos.Sub(pick).Len() <= buildingPickRadius {
			s.openBuildingPopup(b, screenW, screenH)
			return
		}
	}
	cableHalfWidth2 := float32(liftPickRadius * liftPickRadius)
	for _, lift := range s.world.Lifts {
		if lift.Base.Sub(pick).Len() <= liftPickRadius ||
			lift.Top.Sub(pick).Len() <= liftPickRadius ||
			pointSegmentDistSq(pick, lift.Base, lift.Top) <= cableHalfWidth2 {
			s.openLiftPopup(lift, screenW, screenH)
			return
		}
	}
	// Trail pick: check if click cell belongs to any trail.
	cx := int(clickPos[0] / world.CellSize)
	cz := int(clickPos[2] / world.CellSize)
	for _, t := range s.world.Trails {
		for _, cell := range t.Cells {
			if cell[0] == cx && cell[1] == cz {
				s.openTrailPopup(t, screenW, screenH)
				return
			}
		}
	}
	// Nothing matched — clear trail selection since user clicked on empty ground.
	if s.activeTool != toolTrailPaint {
		s.activeTrailID = 0
	}
	if s.popup != nil {
		s.popup.Visible = false
	}
}

func (s *Scenario) openTrailPopup(trail *world.Trail, screenW, screenH int) {
	s.selectedCatID = 0
	s.selectedBuildingID = 0
	s.showCatPath = false
	s.activeTrailID = trail.ID // show overlay for selected trail even without overlay panel
	s.buildTrailPopup(trail, false, screenW, screenH)
}

// buildTrailPopup builds (or rebuilds) the trail popup. confirmClear true shows
// Confirm/Cancel instead of the normal Clear button.
func (s *Scenario) buildTrailPopup(trail *world.Trail, confirmClear bool, screenW, screenH int) {
	t := trail
	w := ui.NewWindow("Trail", 0, 0)
	w.AddTextInput("Name", t.Name, func(text string) { t.Name = text })
	w.AddDifficultyToggles("Difficulty",
		func(bit uint8) bool { return t.Difficulty == world.TerrainDifficulty(bit) },
		func(bit uint8) {
			t.Difficulty = world.TerrainDifficulty(bit)
			s.world.RebuildTrailGraph()
		},
	)
	w.AddBoolToggle("Groomed", func() bool { return t.Groomed }, func(v bool) {
		t.Groomed = v
		s.sim.InvalidateSections()
	})
	w.AddLabel("Cells", func() string { return fmt.Sprintf("%d", len(t.Cells)) })
	w.AddLabel("Groom", func() string {
		if len(t.Cells) == 0 {
			return "—"
		}
		var sum float32
		for _, c := range t.Cells {
			sum += s.world.Terrain.Cells[c[0]][c[1]].Grooming
		}
		return fmt.Sprintf("%.0f%%", sum/float32(len(t.Cells))*100)
	})

	enterEdit := func(erase bool) {
		s.trailEraseMode = erase
		s.activeTrailID = t.ID
		s.trailDifficulty = t.Difficulty
		s.lastTrailPaintCell = [2]int{-1, -1}
		s.activeTool = toolTrailPaint
		s.syncToolButtons()
		if s.popup != nil {
			s.popup.Visible = false
		}
	}
	w.AddActionButton("Add", func() {
		enterEdit(false)
		s.setToast("Drag to add cells. Right-drag to remove. Esc to finish.")
	})
	w.AddActionButton("Remove", func() {
		enterEdit(true)
		s.setToast("Drag to remove cells. Esc to finish.")
	})

	if confirmClear {
		w.AddLabel("Confirm", func() string { return "Delete this trail?" })
		w.AddActionButton("Confirm", func() {
			for _, a := range s.world.OnMountain {
				for _, step := range a.Plan.Steps {
					if step.TrailID == t.ID {
						a.Plan.Steps = nil
						break
					}
				}
			}
			s.world.DeleteTrail(t.ID)
			s.world.RebuildTrailGraph()
			s.activeTrailID = 0
			if s.popup != nil {
				s.popup.Visible = false
			}
		})
		w.AddActionButton("Cancel", func() {
			s.buildTrailPopup(t, false, screenW, screenH)
		})
	} else {
		w.AddActionButton("Clear", func() {
			s.buildTrailPopup(t, true, screenW, screenH)
		})
	}

	w.Visible = true
	w.Center(screenW, screenH)
	s.popup = w
}

func (s *Scenario) openBuildingPopup(b *world.Building, screenW, screenH int) {
	if s.activeTool != toolTrailPaint {
		s.activeTrailID = 0
	}
	s.selectedCatID = 0
	s.selectedBuildingID = b.ID
	s.showCatPath = false
	bldg := b
	switch bldg.Type {
	case world.BuildingShed:
		w := ui.NewWindow("Equipment Shed", 0, 0)
		w.AddLabel("Fleet", func() string {
			cats := s.world.CatsOwnedBy(bldg.ID)
			active := 0
			for _, c := range cats {
				if c.Status == world.CatActive {
					active++
				}
			}
			return fmt.Sprintf("%d cats (%d active)", len(cats), active)
		})
		w.AddIntStepperFn("Active",
			func() string {
				cats := s.world.CatsOwnedBy(bldg.ID)
				active := 0
				for _, c := range cats {
					if c.Status == world.CatActive {
						active++
					}
				}
				return fmt.Sprintf("%d / %d", active, len(cats))
			},
			func() { // minus: put one active cat on standby
				cats := s.world.CatsOwnedBy(bldg.ID)
				for i := len(cats) - 1; i >= 0; i-- {
					if cats[i].Status == world.CatActive {
						cats[i].Status = world.CatStandby
						s.sim.InvalidateSections()
						return
					}
				}
			},
			func() { // plus: activate one standby cat
				cats := s.world.CatsOwnedBy(bldg.ID)
				for _, c := range cats {
					if c.Status == world.CatStandby {
						c.Status = world.CatActive
						s.sim.InvalidateSections()
						return
					}
				}
			})
		w.AddLabel("Daily cost", func() string {
			cats := s.world.CatsOwnedBy(bldg.ID)
			cost := 0
			for _, c := range cats {
				if c.Status == world.CatActive {
					cost += world.CatActiveCostDay
				} else {
					cost += world.CatStandbyCostDay
				}
			}
			return fmt.Sprintf("$%d/day", cost)
		})
		w.AddActionButton(fmt.Sprintf("Buy cat  $%d", world.CatPurchasePrice), func() {
			if s.world.Cash < world.CatPurchasePrice {
				s.setToast(fmt.Sprintf("Need $%d for another cat", world.CatPurchasePrice))
				return
			}
			s.world.Cash -= world.CatPurchasePrice
			s.world.SpawnSnowcat(bldg)
			s.sim.InvalidateSections()
		})
		w.AddActionButton("Release cat", func() {
			cats := s.world.CatsOwnedBy(bldg.ID)
			if len(cats) == 0 {
				return
			}
			// Prefer releasing a standby cat; fall back to any.
			var target *world.Snowcat
			for _, c := range cats {
				if c.Status == world.CatStandby {
					target = c
					break
				}
			}
			if target == nil {
				target = cats[len(cats)-1]
			}
			s.world.RemoveSnowcat(target.ID)
			s.sim.InvalidateSections()
		})
		w.Visible = true
		w.Center(screenW, screenH)
		s.popup = w
		return
	case world.BuildingParking:
		w := ui.NewWindow("Parking Lot", 0, 0)
		w.AddLabel("Cars", func() string {
			return fmt.Sprintf("%d / %d", int(bldg.CurrentCars), bldg.MaxCars)
		})
		w.Visible = true
		w.Center(screenW, screenH)
		s.popup = w
		return
	case world.BuildingSnowGun:
		w := ui.NewWindow("Snow Gun", 0, 0)
		w.AddBoolToggle("Active", func() bool { return bldg.SnowGunEnabled }, func(v bool) {
			bldg.SnowGunEnabled = v
		})
		w.AddLabel("Daily cost", func() string {
			if bldg.SnowGunEnabled {
				return fmt.Sprintf("$%d/day", world.SnowGunActiveCostDay)
			}
			return "$0/day (off)"
		})
		w.AddLabel("Status", func() string {
			if !bldg.SnowGunEnabled {
				return "Off"
			}
			low := s.sim.Weather.Today().TempLow
			if low <= world.SnowGunMinTempC {
				return fmt.Sprintf("Producing snow (low %s)", settings.FormatTemp(low))
			}
			return fmt.Sprintf("Too warm (low %s)", settings.FormatTemp(low))
		})
		w.Visible = true
		w.Center(screenW, screenH)
		s.popup = w
		return
	case world.BuildingTicketOffice:
		w := ui.NewWindow("Ticket Office", 0, 0)
		w.AddIntStepper("Pass price ($)", &s.world.SeasonPassPrice, 10, 0, 1000)
		w.AddLabel("Pass holders", func() string {
			count := 0
			for _, a := range s.world.OnMountain {
				if a.HasSeasonPass {
					count++
				}
			}
			return fmt.Sprintf("%d on mountain", count)
		})
		w.AddLabel("Inbound", func() string {
			count := 0
			for _, a := range s.world.OnMountain {
				if a.TargetID == bldg.ID {
					count++
				}
			}
			return fmt.Sprintf("%d", count)
		})
		w.Visible = true
		w.Center(screenW, screenH)
		s.popup = w
		return
	case world.BuildingPatrolHut:
		w := ui.NewWindow("Patrol Hut", 0, 0)
		w.AddLabel("Patroller", func() string {
			for _, p := range s.world.Patrollers {
				if p.HutID != bldg.ID {
					continue
				}
				switch p.State {
				case world.PatrollerAtHut:
					return "At hut"
				case world.PatrollerEnRoute:
					return "En route to injured skier"
				case world.PatrollerOnScene:
					return "On scene"
				case world.PatrollerReturning:
					return "Returning with patient"
				}
			}
			return "None"
		})
		w.Visible = true
		w.Center(screenW, screenH)
		s.popup = w
		return
	case world.BuildingLodge:
		w := ui.NewWindow("Lodge", 0, 0)
		w.AddLabel("Inbound", func() string {
			count := 0
			for _, a := range s.world.OnMountain {
				if a.TargetID == bldg.ID {
					count++
				}
			}
			return fmt.Sprintf("%d", count)
		})
		w.Visible = true
		w.Center(screenW, screenH)
		s.popup = w
		return
	default:
		panic(fmt.Sprintf("openBuildingPopup: unhandled building type %d", bldg.Type))
	}
}

func (s *Scenario) openLiftPopup(lift *world.Lift, screenW, screenH int) {
	if s.activeTool != toolTrailPaint {
		s.activeTrailID = 0
	}
	s.selectedCatID = 0
	s.selectedBuildingID = 0
	s.showCatPath = false
	l := lift
	w := ui.NewWindow("Ski Lift", 0, 0)
	w.AddTextInput("Name", l.Name, func(text string) { l.Name = text })
	w.AddLabel("Services", func() string {
		svc := s.world.ServicesForLift(l.ID)
		switch {
		case svc.Has(world.DiffGreen) && svc.Has(world.DiffBlue) && svc.Has(world.DiffBlack):
			return "Green / Blue / Black"
		case svc.Has(world.DiffGreen) && svc.Has(world.DiffBlue):
			return "Green / Blue"
		case svc.Has(world.DiffGreen) && svc.Has(world.DiffBlack):
			return "Green / Black"
		case svc.Has(world.DiffBlue) && svc.Has(world.DiffBlack):
			return "Blue / Black"
		case svc.Has(world.DiffGreen):
			return "Green"
		case svc.Has(world.DiffBlue):
			return "Blue"
		case svc.Has(world.DiffBlack):
			return "Black"
		default:
			return "None"
		}
	})
	w.AddLabel("Type", func() string {
		return l.Type.Label()
	})
	w.AddLabel("Queue", func() string {
		return fmt.Sprintf("%d skiers", len(l.Queue))
	})
	w.AddLabel("On lift", func() string {
		return fmt.Sprintf("%d skiers", l.PassengerCount())
	})
	w.AddLabel("Chairs", func() string {
		return fmt.Sprintf("%d × %d-seat", len(l.Chairs), l.Type.Capacity())
	})
	w.AddStepper("Speed (m/s)", &l.Speed, 0.5, 0.5, 8.0)
	w.AddIntStepper("Ticket ($)", &l.TicketPrice, 5, 0, 200)
	if l.OnHold {
		w.AddLabel("Status", func() string { return "On Hold (no snow at base)" })
	}
	if l.Open {
		w.AddActionButton("Close Lift", func() {
			for _, g := range l.Queue {
				g.Plan.Steps = nil
			}
			l.Queue = l.Queue[:0]
			l.Open = false
			s.openLiftPopup(l, screenW, screenH)
		})
	} else {
		w.AddActionButton("Open Lift", func() {
			l.Open = true
			s.openLiftPopup(l, screenW, screenH)
		})
	}
	if l.Type == world.LiftDouble {
		label := fmt.Sprintf("Upgrade to Quad ($%d)", world.LiftUpgradeCost)
		w.AddActionButton(label, func() {
			if !s.world.UpgradeLift(l, world.LiftFixedQuad) {
				if s.world.Cash < world.LiftUpgradeCost {
					s.setToast(fmt.Sprintf("Need $%d to upgrade — short by $%d",
						world.LiftUpgradeCost, world.LiftUpgradeCost-s.world.Cash))
				}
				return
			}
			s.openLiftPopup(l, screenW, screenH)
		})
	}
	if l.Type == world.LiftFixedQuad {
		label := fmt.Sprintf("Upgrade to HS Quad ($%d)", world.LiftHSUpgradeCost)
		w.AddActionButton(label, func() {
			if !s.world.UpgradeLift(l, world.LiftHSQuad) {
				if s.world.Cash < world.LiftHSUpgradeCost {
					s.setToast(fmt.Sprintf("Need $%d to upgrade — short by $%d",
						world.LiftHSUpgradeCost, world.LiftHSUpgradeCost-s.world.Cash))
				}
				return
			}
			s.openLiftPopup(l, screenW, screenH)
		})
	}
	if l.Type == world.LiftHSQuad {
		label := fmt.Sprintf("Upgrade to HS 6-Pack ($%d)", world.LiftHS6PackUpgradeCost)
		w.AddActionButton(label, func() {
			if !s.world.UpgradeLift(l, world.LiftHS6Pack) {
				if s.world.Cash < world.LiftHS6PackUpgradeCost {
					s.setToast(fmt.Sprintf("Need $%d to upgrade — short by $%d",
						world.LiftHS6PackUpgradeCost, world.LiftHS6PackUpgradeCost-s.world.Cash))
				}
				return
			}
			s.openLiftPopup(l, screenW, screenH)
		})
	}
	w.Visible = true
	w.Center(screenW, screenH)
	s.popup = w
}

func (s *Scenario) openSnowcatPopup(cat *world.Snowcat, screenW, screenH int) {
	if s.activeTool != toolTrailPaint {
		s.activeTrailID = 0
	}
	s.selectedCatID = cat.ID
	s.selectedBuildingID = 0
	s.showCatPath = false
	s.followGuestID = 0
	c := cat
	w := ui.NewWindow("Snowcat", 0, 0)
	w.AddLabel("Status", func() string {
		if c.Status == world.CatActive {
			return "Active"
		}
		return "Standby"
	})
	w.AddLabel("Section", func() string {
		return fmt.Sprintf("%d columns", len(c.Section))
	})
	w.AddLabel("Route", func() string {
		if len(c.Route) == 0 {
			return "idle"
		}
		return fmt.Sprintf("%d / %d cells", c.RouteIdx, len(c.Route))
	})
	w.AddActionButton("Go to shed", func() {
		r := s.app.Renderer
		for _, b := range s.world.Buildings {
			if b.ID == c.ShedID {
				r.Camera.Target[0] = b.Pos[0]
				r.Camera.Target[2] = b.Pos[1]
				r.Camera.Recalculate()
				return
			}
		}
	})
	w.AddActionButton("Show path", func() {
		s.showCatPath = !s.showCatPath
	})
	w.Visible = true
	w.Center(screenW, screenH)
	s.popup = w
}

// openLandBuyPopup shows a purchase confirmation popup for the parcel at
// terrain cell (gx, gz). Does nothing if the cell is not in a purchasable
// parcel.
func (s *Scenario) openLandBuyPopup(gx, gz, screenW, screenH int) {
	p := s.world.ParcelAt(gx, gz)
	if p == nil || p.State != world.ParcelPurchasable {
		if p != nil && p.State == world.ParcelOffLimits {
			s.setToast("This land is off-limits and cannot be purchased")
		}
		return
	}
	parcel := p
	w := ui.NewWindow(parcel.Name, 0, 0)
	w.AddLabel("Price", func() string { return fmt.Sprintf("$%d", parcel.Price) })
	w.AddLabel("Status", func() string { return "Available for purchase" })
	w.AddActionButton(fmt.Sprintf("Buy for $%d", parcel.Price), func() {
		if !s.world.BuyParcel(parcel.ID) {
			s.setToast(fmt.Sprintf("Need $%d — short by $%d", parcel.Price, parcel.Price-s.world.Cash))
		} else {
			s.parcelBoundaryDirty = true
		}
		w.Visible = false
		s.popup = nil
	})
	w.AddActionButton("Cancel", func() {
		w.Visible = false
		s.popup = nil
	})
	w.Visible = true
	w.Center(screenW, screenH)
	s.popup = w
}

// catPathLines builds debug lines for the selected cat's remaining route,
// hovering slightly above the terrain surface.
func (s *Scenario) catPathLines() []render.DebugLine {
	var cat *world.Snowcat
	for _, c := range s.world.Snowcats {
		if c.ID == s.selectedCatID {
			cat = c
			break
		}
	}
	if cat == nil {
		s.selectedCatID = 0
		return nil
	}

	const hover = float32(0.5)
	color := [3]float32{0.0, 0.0, 0.0}

	var lines []render.DebugLine
	for i := cat.RouteIdx; i+1 < len(cat.Route); i++ {
		a, b := cat.Route[i], cat.Route[i+1]
		ax := (float32(a[0]) + 0.5) * world.CellSize
		az := (float32(a[1]) + 0.5) * world.CellSize
		bx := (float32(b[0]) + 0.5) * world.CellSize
		bz := (float32(b[1]) + 0.5) * world.CellSize
		ay := s.world.Terrain.SurfaceElevationAt(a[0], a[1]) + hover
		by := s.world.Terrain.SurfaceElevationAt(b[0], b[1]) + hover
		lines = append(lines, render.DebugLine{
			A:     mgl32.Vec3{ax, ay, az},
			B:     mgl32.Vec3{bx, by, bz},
			Color: color,
		})
	}
	return lines
}

// setTool toggles the given tool on; if it is already active, deactivates it.
// The Lift and Road tools are two-click flows — clicking their button while
// the second click is pending cancels the in-progress placement.
func (s *Scenario) setTool(t toolMode) {
	r := s.app.Renderer
	isActive := s.activeTool == t ||
		(t == toolLiftBase && s.activeTool == toolLiftTop) ||
		(t == toolRoadStart && s.activeTool == toolRoadEnd)
	r.ClearAllGhosts()
	r.ClearGhostCable()
	r.ClearGhostRoad()
	if isActive {
		s.activeTool = toolNone
	} else {
		s.activeTool = t
	}
	// Activating any tool ends a toolNone-level edit session so the
	// selection markers disappear and any in-flight drag is dropped.
	if s.roadEdit.active() || s.roadEdit.dragging {
		s.roadEdit.clear()
	}
	if s.structureEdit.active() || s.structureEdit.dragging {
		s.structureEdit.clear()
	}
	s.syncToolButtons()
}

// activateLiftTool starts (or switches) the two-click lift placement
// flow for the given chair variant. Clicking the same variant's
// toolbar button while it's already active toggles the tool off;
// clicking the other variant's button mid-flow swaps the chair type
// without cancelling the placement, so the in-progress base position
// is preserved.
func (s *Scenario) activateLiftTool(typ world.LiftType) {
	inLiftFlow := s.activeTool == toolLiftBase || s.activeTool == toolLiftTop
	if inLiftFlow && s.liftType == typ {
		s.cancelTool()
		return
	}
	s.liftType = typ
	if inLiftFlow {
		// Keep the current step (base or top); only the chair type changes.
		s.syncToolButtons()
		return
	}
	s.setTool(toolLiftBase)
}

// cancelTool deactivates whatever tool is currently active.
func (s *Scenario) cancelTool() {
	s.app.Renderer.ClearAllGhosts()
	s.app.Renderer.ClearGhostCable()
	s.app.Renderer.ClearGhostRoad()
	if s.activeTool == toolTrailPaint {
		s.activeTrailID = 0
		s.lastTrailPaintCell = [2]int{-1, -1}
		s.trailEraseMode = false
	}
	s.activeTool = toolNone
	s.syncToolButtons()
}

// barsContain reports whether a screen Y is inside either the top HUD bar
// or the bottom tool bar — i.e. should be excluded from world hit-testing.
func (s *Scenario) barsContain(y float32) bool {
	if s.topBar != nil && s.topBar.ContainsY(y) {
		return true
	}
	if s.toolBar != nil && s.toolBar.ContainsY(y) {
		return true
	}
	return false
}

// uiCovers reports whether a screen point is inside any HUD element
// that should suppress world hit-testing. Used for hover updates and
// held-mouse drag suppression — initial-click consumption lives on
// inp.LeftClickConsumed instead, since that survives a button callback
// that hides its own popup mid-frame.
func (s *Scenario) uiCovers(x, y float32, screenW float32) bool {
	if s.barsContain(y) {
		return true
	}
	if s.overlayPanel != nil && s.overlayPanel.ContainsXY(x, y, screenW) {
		return true
	}
	if s.popup != nil && s.popup.ContainsPoint(x, y) {
		return true
	}
	if s.chartWindow != nil && s.chartWindow.ContainsPoint(x, y) {
		return true
	}
	return false
}

// syncToolButtons updates the active state of all tool buttons to match activeTool.
// The lift buttons live outside toolButtons because two of them share the
// toolLiftBase/Top tool modes — only the one whose chair type matches the
// current liftType selection should highlight.
func (s *Scenario) syncToolButtons() {
	for mode, btn := range s.toolButtons {
		btn.SetActive(s.activeTool == mode ||
			(mode == toolRoadStart && s.activeTool == toolRoadEnd))
	}
	liftActive := s.activeTool == toolLiftBase || s.activeTool == toolLiftTop
	if s.liftDoubleBtn != nil {
		s.liftDoubleBtn.SetActive(liftActive && s.liftType == world.LiftDouble)
	}
	if s.liftQuadBtn != nil {
		s.liftQuadBtn.SetActive(liftActive && s.liftType == world.LiftFixedQuad)
	}
	if s.liftHSQuadBtn != nil {
		s.liftHSQuadBtn.SetActive(liftActive && s.liftType == world.LiftHSQuad)
	}
	if s.liftHS6PackBtn != nil {
		s.liftHS6PackBtn.SetActive(liftActive && s.liftType == world.LiftHS6Pack)
	}
	if s.liftGondolaBtn != nil {
		s.liftGondolaBtn.SetActive(liftActive && s.liftType == world.LiftGondola)
	}
	if s.liftHeliBtn != nil {
		s.liftHeliBtn.SetActive(liftActive && s.liftType == world.LiftHeli)
	}
	// Sync submenu parent active state so the group button highlights when any
	// child tool is selected, giving the user a visual cue of the active group.
	if s.liftsSubmenu != nil {
		s.liftsSubmenu.Btn.SetActive(s.liftsSubmenu.HasActiveChild())
	}
	if s.opsSubmenu != nil {
		s.opsSubmenu.Btn.SetActive(s.opsSubmenu.HasActiveChild())
	}
	if s.amenitiesSubmenu != nil {
		s.amenitiesSubmenu.Btn.SetActive(s.amenitiesSubmenu.HasActiveChild())
	}
	if s.transportSubmenu != nil {
		s.transportSubmenu.Btn.SetActive(s.transportSubmenu.HasActiveChild())
	}
}

// placementGhostState bundles the inputs that drive the placement ghost
// preview for one frame. Centralised so scenario and editor share the
// same ghost-rendering code; affordability tinting is the caller's job
// (the editor is always free; the scenario red-tints when over-budget).
type placementGhostState struct {
	activeTool toolMode
	hoverPos   mgl32.Vec2     // continuous world XZ under the cursor
	hoverValid bool           // false when the cursor isn't on the terrain
	liftBase   mgl32.Vec2     // first-click position for the two-step lift placement (toolLiftTop only)
	liftType   world.LiftType // which lift variant is being placed (for ghost style selection)
	roadStart  mgl32.Vec2     // first-click position for the two-step road placement (toolRoadEnd only)
	tint       [3]float32     // colour multiplier for the ghost — typically affordable / unaffordable
}

// updatePlacementGhost drives the translucent preview that follows the
// cursor while a placement tool is active. Resets every ghost up-front
// each frame, then opts the active tool's geometry back in — keeps the
// state of the ghost world in lockstep with the active tool without
// per-tool teardown logic.
//
// Reads the continuous hoverPos so previews track the cursor at freeform
// precision, not snapped to a cell.
func updatePlacementGhost(r *render.Renderer, t *world.Terrain, st placementGhostState) {
	r.ClearAllGhosts()
	r.ClearGhostCable()
	r.ClearGhostRoad()

	if !st.hoverValid {
		return
	}

	switch st.activeTool {
	case toolBuilding, toolTicketOffice:
		r.SetGhosts(render.MeshBuilding, []render.StaticInstance{
			buildingInstance(st.hoverPos, t, st.tint),
		})

	case toolShed:
		r.SetGhosts(render.MeshShed, []render.StaticInstance{
			buildingInstance(st.hoverPos, t, st.tint),
		})

	case toolPatrolHut:
		r.SetGhosts(render.MeshShed, []render.StaticInstance{
			buildingInstance(st.hoverPos, t, st.tint),
		})

	case toolSnowGun:
		r.SetGhosts(render.MeshSnowGun, []render.StaticInstance{
			buildingInstance(st.hoverPos, t, st.tint),
		})

	case toolParking:
		r.SetGhosts(render.MeshParkingPad, []render.StaticInstance{
			buildingInstance(st.hoverPos, t, st.tint),
		})

	case toolLiftBase:
		if st.liftType == world.LiftHeli {
			r.SetGhosts(render.MeshHelipad, []render.StaticInstance{
				helipadInstance(st.hoverPos, st.hoverPos, t, st.tint),
			})
		} else {
			// No top yet — render at default orientation by passing
			// otherEnd == pos. Rotation kicks in in toolLiftTop below.
			r.SetGhosts(render.MeshLiftStation, []render.StaticInstance{
				stationInstance(st.hoverPos, st.hoverPos, t, st.tint),
			})
		}

	case toolLiftTop:
		base := st.liftBase
		top := st.hoverPos
		if st.liftType == world.LiftHeli {
			r.SetGhosts(render.MeshHelipad, []render.StaticInstance{
				helipadInstance(base, top, t, st.tint),
				helipadInstance(top, base, t, st.tint),
			})
			// No cable ghost for helicopter lifts.
		} else {
			r.SetGhosts(render.MeshLiftStation, []render.StaticInstance{
				stationInstance(base, top, t, st.tint), // base faces top
				stationInstance(top, base, t, st.tint), // top faces base
			})
			r.SetGhostCable(base, top, t)
		}

	case toolRoadEnd:
		// Ghost a road segment between the stored start and the cursor.
		// SetGhostRoad mirrors SetGhostCable: a fresh single-segment
		// mesh regenerated each frame as the cursor moves.
		r.SetGhostRoad(st.roadStart, st.hoverPos, t, st.tint)

	case toolEdgeConnect:
		// Show the post at the perimeter and a road stub extending
		// inland. When the click is too far inland for any edge,
		// previewPos falls back to the raw cursor and placementLegal
		// red-tints the whole preview.
		previewPos := st.hoverPos
		inland := st.hoverPos
		if snapped, inward, ok := projectToMapEdge(t, st.hoverPos, edgeConnectTolerance); ok {
			previewPos = snapped
			inland = mgl32.Vec2{
				snapped[0] + inward[0]*edgeConnectStubLength,
				snapped[1] + inward[1]*edgeConnectStubLength,
			}
		}
		r.SetGhosts(render.MeshRoadConnect, []render.StaticInstance{
			roadConnectInstance(previewPos, t, st.tint),
		})
		r.SetGhostRoad(previewPos, inland, t, st.tint)
	}
}

// roadConnectInstance wraps render.RoadConnectTransform into a
// StaticInstance for the ghost-preview path. Live placements bypass
// this and go through RebuildStaticBatch like any other batched static.
func roadConnectInstance(pos mgl32.Vec2, t *world.Terrain, tint [3]float32) render.StaticInstance {
	m := render.RoadConnectTransform(pos, t)
	inst := render.StaticInstance{ColorTint: tint}
	copy(inst.Transform[:], m[:])
	return inst
}

// placementCost returns the cost of placing whatever the active tool
// will create at the current hover position, a flag indicating whether
// the player can afford it, and a flag indicating whether the placement
// is legal (e.g., footprint doesn't overlap another building). Returns
// (0, false, true, false) if no cost-bearing tool is active or the
// hover is off-terrain.
func (s *Scenario) placementCost() (cost int, affordable, legal, valid bool) {
	if !s.hoverValid {
		return 0, false, true, false
	}
	pos := mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]}
	legal = true
	gx, gz := s.hoverCell[0], s.hoverCell[1]
	cellOwned := s.world.Terrain.IsAccessible(gx, gz)
	switch s.activeTool {
	case toolBuilding:
		cost = world.LodgeCost
		legal = cellOwned && !s.world.BuildingOverlap(world.BuildingLodge, pos[0], pos[1])
	case toolTicketOffice:
		cost = world.TicketOfficeCost
		legal = cellOwned && !s.world.BuildingOverlap(world.BuildingTicketOffice, pos[0], pos[1])
	case toolShed:
		cost = world.ShedCost
		legal = cellOwned && !s.world.BuildingOverlap(world.BuildingShed, pos[0], pos[1])
	case toolPatrolHut:
		cost = world.PatrolHutCost
		legal = cellOwned && !s.world.BuildingOverlap(world.BuildingPatrolHut, pos[0], pos[1])
	case toolSnowGun:
		cost = world.SnowGunCost
		legal = cellOwned && !s.world.BuildingOverlap(world.BuildingSnowGun, pos[0], pos[1])
	case toolParking:
		cost = world.ParkingCost
		legal = cellOwned && !s.world.BuildingOverlap(world.BuildingParking, pos[0], pos[1])
	case toolLiftBase:
		if s.liftType == world.LiftHeli {
			cost = world.HelipadCost / 2
		} else {
			cost = world.LiftStationCost
		}
		legal = cellOwned
	case toolLiftTop:
		if s.liftType == world.LiftHeli {
			cost = world.HelipadCost
		} else {
			cost = world.LiftCost(s.liftBase, pos)
		}
		legal = cellOwned
	case toolRoadStart:
		cost = world.RoadBaseCost
	case toolRoadEnd:
		end := resolveRoadEndpoint(s.world, pos)
		cost = world.RoadCost(s.roadStart, end.pos)
	default:
		return 0, false, true, false
	}
	return cost, s.world.Cash >= cost, legal, true
}

// placementTint resolves the ghost tint for the active placement tool:
// white when OK, warm red when unaffordable, saturated red when illegal.
// No-op tint when no placement is active.
func (s *Scenario) placementTint() [3]float32 {
	_, affordable, legal, valid := s.placementCost()
	if !valid {
		return ghostTint(true, true)
	}
	return ghostTint(affordable, legal)
}

// ghostTint returns the per-instance ColorTint for ghost previews:
// saturated red when the placement is illegal (overlap, etc.), warm red
// when the player can't afford it, white otherwise. Illegal beats
// unaffordable since you can't place at all.
func ghostTint(affordable, legal bool) [3]float32 {
	if !legal {
		return [3]float32{1.0, 0.25, 0.25}
	}
	if !affordable {
		return [3]float32{1.0, 0.45, 0.45}
	}
	return [3]float32{1, 1, 1}
}

// stationInstance wraps render.LiftStationTransform into a StaticInstance
// for the ghost-preview path. Live placements bypass this and call the
// transform helper directly inside RebuildStaticBatch.
func stationInstance(pos, otherEnd mgl32.Vec2, t *world.Terrain, tint [3]float32) render.StaticInstance {
	m := render.LiftStationTransform(pos, otherEnd, t)
	inst := render.StaticInstance{ColorTint: tint}
	copy(inst.Transform[:], m[:])
	return inst
}

// helipadInstance wraps render.HelipadTransform into a StaticInstance
// for the ghost-preview path.
func helipadInstance(pos, otherEnd mgl32.Vec2, t *world.Terrain, tint [3]float32) render.StaticInstance {
	m := render.HelipadTransform(pos, otherEnd, t)
	inst := render.StaticInstance{ColorTint: tint}
	copy(inst.Transform[:], m[:])
	return inst
}

// buildingInstance wraps render.BuildingTransform into a StaticInstance
// for the ghost-preview path. New buildings have rotation 0 by default;
// once a placement-rotation control exists, route it through here.
func buildingInstance(pos mgl32.Vec2, t *world.Terrain, tint [3]float32) render.StaticInstance {
	m := render.BuildingTransform(pos, 0, t)
	inst := render.StaticInstance{ColorTint: tint}
	copy(inst.Transform[:], m[:])
	return inst
}

// cycleFollow jumps the camera to a random skier. When already following a
// skier, the current one is excluded so each Tab press visibly changes target.
func (s *Scenario) cycleFollow() {
	agents := s.world.OnMountain
	if len(agents) == 0 {
		s.followGuestID = 0
		return
	}
	current := s.followGuestID
	candidates := agents
	if current != 0 && len(agents) > 1 {
		candidates = make([]*world.Guest, 0, len(agents)-1)
		for _, a := range agents {
			if a.ID != current {
				candidates = append(candidates, a)
			}
		}
	}
	s.followGuestID = candidates[rand.Intn(len(candidates))].ID
}

// pickGuest returns the front-most agent under the cursor, or nil if none.
// Treats each skier as a sphere of pickRadius around its world position.
func (s *Scenario) pickGuest(cam *render.Camera, mousePos mgl32.Vec2) *world.Guest {
	const pickRadius = float32(2.5)
	const pickRadius2 = pickRadius * pickRadius
	origin, dir := cam.ScreenToWorldRay(mousePos)
	var best *world.Guest
	bestT := float32(math.Inf(1))
	for _, a := range s.world.OnMountain {
		oc := a.Pos.Sub(origin)
		t := oc.Dot(dir)
		if t < 0 {
			continue
		}
		closest := origin.Add(dir.Mul(t))
		diff := closest.Sub(a.Pos)
		if diff.Dot(diff) <= pickRadius2 && t < bestT {
			bestT = t
			best = a
		}
	}
	return best
}

// applyFirstPersonCamera puts the camera at the followed skier's head,
// looking forward along Heading with a slight downward tilt so the
// slope ahead fills more of the frame. Heading uses the
// atan2(dx, dz) convention shared with the motor / sense code.
func applyFirstPersonCamera(cam *render.Camera, a *world.Guest) {
	const (
		headHeight = 1.6  // metres above the agent's foot position
		lookDist   = 10.0 // metres ahead of the eye
		pitchDown  = 5.0  // degrees below horizontal
	)
	eye := a.Pos
	eye[1] += headHeight

	h := float64(a.Heading)
	p := pitchDown * math.Pi / 180
	cosP := math.Cos(p)
	fwd := mgl32.Vec3{
		float32(math.Sin(h) * cosP),
		float32(-math.Sin(p)),
		float32(math.Cos(h) * cosP),
	}

	cam.Perspective = true
	cam.FOVDeg = 70
	cam.EyePos = eye
	cam.LookAt = eye.Add(fwd.Mul(lookDist))
	cam.Recalculate()
}

// findFollowedGuest returns the agent being followed, or nil if it no longer exists.
func (s *Scenario) findFollowedGuest() *world.Guest {
	for _, a := range s.world.OnMountain {
		if a.ID == s.followGuestID {
			return a
		}
	}
	return nil
}

// applyPerceptionCone forwards the followed skier's perception fan to the
// renderer. Only emits a cone for agents that are actively skiing — when the
// agent is walking, queuing, on a lift, or fallen, Sense is stale and we
// hide the highlight. Match against Activity rather than introspecting Sense
// because the snapshot has no "fresh" flag.
func (s *Scenario) applyPerceptionCone(r *render.Renderer) {
	if s.followGuestID == 0 {
		r.ClearPerceptionCone()
		return
	}
	a := s.findFollowedGuest()
	if a == nil {
		r.ClearPerceptionCone()
		return
	}
	switch world.Activity(s.world, a) {
	case "To Lodge", "To Lift", "Traveling":
		// active skiing — fall through
	default:
		r.ClearPerceptionCone()
		return
	}
	if a.Sense.ProbeDist <= 0 {
		r.ClearPerceptionCone()
		return
	}
	hx := float32(math.Sin(float64(a.Heading)))
	hz := float32(math.Cos(float64(a.Heading)))
	cosHalf := float32(math.Cos(float64(a.Sense.ProbeHalfAngle)))
	r.SetPerceptionCone(a.Pos, mgl32.Vec2{hx, hz}, cosHalf, a.Sense.ProbeDist)
}

// followLabel is the compact HUD banner shown for the currently followed
// skier. Stays narrow: identity, speed/energy/fun, selected goal name,
// and the next action only. Deep planner introspection — full plan,
// goal weights, snapshot anchors — moves to plannerDebugPanel behind F4.
//
// Reads agent.Plan directly — the simulation's tickPlanning updates it
// only when a replan trigger fires (plan empty, head done, precondition
// broken, periodic safety check), so the displayed action is stable
// across frames.
type followLabel struct {
	world   *world.World
	agent   *world.Guest
	simTime float64
}

func (f *followLabel) Draw(r *render.Renderer) {
	activity := world.Activity(f.world, f.agent)
	patiencePct := int(f.agent.Patience*100 + 0.5)
	energyPct := int(f.agent.Energy*100 + 0.5)
	hungerPct := int(f.agent.Hunger*100 + 0.5)
	thirstPct := int(f.agent.Thirst*100 + 0.5)
	mode := f.agent.Sense.Mode
	if mode == "" {
		mode = "—"
	}
	// Trait badges — short flags that show the player which preferences
	// are driving this guest's terrain reactions.
	badges := ai.SkillTierName(f.agent.Traits.Skill)
	if f.agent.Traits.LikesGlades {
		badges += " · glades"
	}
	if f.agent.Traits.PrefersGroomed {
		badges += " · corduroy"
	}
	satisfactionPct := int(f.agent.Satisfaction * 100)
	passStr := ""
	if f.agent.HasSeasonPass {
		passStr = "  · season pass"
	}
	rows := []string{
		fmt.Sprintf("%s #%d (%s)  |  %s  |  %s", f.agent.Name, f.agent.ID, badges, activity, mode),
		fmt.Sprintf("%s    patience %d%%    energy %d%%    hunger %d%%    thirst %d%%    satisfaction %d%%", settings.FormatSpeed(f.agent.Speed), patiencePct, energyPct, hungerPct, thirstPct, satisfactionPct),
		fmt.Sprintf("budget $%d%s", int(f.agent.RemainingBudget), passStr),
	}
	resolve := entityName(f.world)
	if t := f.agent.CurrentThought(f.simTime); t.Kind != ai.ThoughtNone {
		rows = append(rows, fmt.Sprintf("\"%s\"", t.Display(resolve)))
	} else if t := f.agent.LastThought(); t.Kind != ai.ThoughtNone {
		rows = append(rows, fmt.Sprintf("(earlier: \"%s\")", t.Display(resolve)))
	}
	// Read the stored plan rather than running the planner per frame.
	// Plan is updated by sim.tickPlanning at the four MD-spec replan
	// triggers; HUD reads what the sim is actually executing.
	plan := &f.agent.Plan
	switch {
	case plan.GoalName == "":
		rows = append(rows, "goal: —")
	case plan.Done():
		rows = append(rows, fmt.Sprintf("goal: %s   (idle)", plan.GoalName))
	default:
		rows = append(rows, fmt.Sprintf("goal: %s   →  %s",
			plan.GoalName, goap.PlanActionLabel(plan.Head(), f.world)))
	}
	drawHUDBox(r, rows, 106, mgl32.Vec4{1, 0.95, 0.1, 1}, true)
}

// plannerDebugPanel is the F4-toggled deep readout of the L0 GOAP
// planner for the followed skier. Reads the *stored* plan on
// agent.Plan rather than recomputing — keeps the panel in sync with
// what the sim is actually executing. Snapshot anchors and goal
// weights are recomputed each frame from a fresh Extract since they
// reflect live state (one agent per frame is cheap).
type plannerDebugPanel struct {
	world *world.World
	agent *world.Guest
}

// debugPanelMaxPlan caps how many plan rows the panel shows. Plans can
// run to Planner.MaxPlanLen (16) but the panel is for spot-checking,
// not full plan archaeology — overflow gets "… +N more".
const debugPanelMaxPlan = 12

func (p *plannerDebugPanel) Draw(r *render.Renderer) {
	snap := goap.Extract(p.agent, p.world)
	plan := &p.agent.Plan

	var rows []string
	rows = append(rows, fmt.Sprintf("─── Planner: Skier #%d ───", p.agent.ID))

	// Ranked goal weights — the actual decision audit, computed off the
	// live snapshot so weights reflect what the next replan would see.
	rows = append(rows, "Goal weights:")
	winnerMarked := false
	for _, gr := range goap.RankedGoals(&snap, p.world) {
		marker := "  "
		if !gr.Satisfied && !winnerMarked {
			marker = "> "
			winnerMarked = true
		}
		label := gr.Goal.Name()
		if gr.Satisfied {
			label += " (satisfied)"
		}
		rows = append(rows, fmt.Sprintf("%s%-22s  w=%.2f", marker, label, gr.Weight))
	}

	// Snapshot anchors — surfaces Extract() output so we can spot mis-
	// derived positions.
	rows = append(rows, "Snapshot:")
	rows = append(rows, fmt.Sprintf("  base=%s  top=%s  q=%s",
		liftRef(p.world, snap.AtLiftBase),
		liftRef(p.world, snap.AtLiftTop),
		liftRef(p.world, snap.Queued)))
	rows = append(rows, fmt.Sprintf("  onLift=%s  lodge=%s  lot=%s",
		liftRef(p.world, snap.OnLift),
		buildingRef(p.world, snap.AtLodge),
		buildingRef(p.world, snap.AtParking)))

	// Trail graph summary — anchor edges + total edge count.
	rows = append(rows, "Trail graph:")
	if p.world.TrailGraph == nil || len(p.world.TrailGraph.Edges) == 0 {
		rows = append(rows, "  (no edges — paint trails over stations)")
	} else {
		anchorID := goap.CurrentAnchorID(&snap)
		edges := p.world.TrailGraph.EdgesFrom(anchorID)
		rows = append(rows, fmt.Sprintf("  total=%d  anchor=%d  from-anchor=%d",
			len(p.world.TrailGraph.Edges), anchorID, len(edges)))
		for _, e := range edges {
			t := p.world.FindTrail(e.TrailID)
			tName := fmt.Sprintf("#%d", e.TrailID)
			if t != nil && t.Name != "" {
				tName = t.Name
			}
			rows = append(rows, fmt.Sprintf("  via %s → #%d (%.0fm)", tName, e.ToID, e.Distance))
		}
	}

	// Per-lift ride counts driving Explore + RideLift.Cost.
	rows = append(rows, "RidenLifts:")
	rows = append(rows, ridenLiftsLine(p.world, snap.RidenLifts))

	// Plan body — read straight from the stored agent.Plan, with the
	// current step marked. Steps that have already executed sit above
	// it (cosmetically dimmed via "·"), upcoming steps below.
	if plan.GoalName != "" {
		rows = append(rows, fmt.Sprintf("Goal: %s", plan.GoalName))
	}
	rows = append(rows, "Plan:")
	switch {
	case plan.Done():
		rows = append(rows, "  (no plan)")
	default:
		for i, step := range plan.Steps {
			if i >= debugPanelMaxPlan {
				rows = append(rows, fmt.Sprintf("  … +%d more", len(plan.Steps)-debugPanelMaxPlan))
				break
			}
			marker := "  "
			if i == plan.Step {
				marker = "→ "
			} else if i < plan.Step {
				marker = "· "
			}
			rows = append(rows, fmt.Sprintf("%s%2d. %-32s  c=%5.1fs",
				marker, i+1, goap.PlanActionLabel(step, p.world), step.Cost))
		}
	}

	drawHUDBox(r, rows, 200, mgl32.Vec4{0.85, 0.95, 1, 1}, false)
}

// ridenLiftsLine formats the per-lift ride count map as a single row
// like "Lift1×2, Accelerator×1". Sorted by descending count so the most-
// ridden lift comes first.
func ridenLiftsLine(w *world.World, rides []ai.RideCount) string {
	if len(rides) == 0 {
		return "  (none)"
	}
	type entry struct {
		label string
		count int
	}
	var entries []entry
	for _, r := range rides {
		if r.Count == 0 {
			continue
		}
		entries = append(entries, entry{liftRef(w, r.LiftID), r.Count})
	}
	if len(entries) == 0 {
		return "  (none)"
	}
	// Sort by descending count, ties broken by label.
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0; j-- {
			if entries[j-1].count >= entries[j].count {
				if entries[j-1].count > entries[j].count || entries[j-1].label <= entries[j].label {
					break
				}
			}
			entries[j-1], entries[j] = entries[j], entries[j-1]
		}
	}
	out := "  "
	for i, e := range entries {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%s×%d", e.label, e.count)
	}
	return out
}

// terrainInspectPanel is the F5-toggled per-cell readout. Renders the
// stored fields (GroundElevation, SnowAccumulation, Packed, ...) plus
// the derived VisibleSnowDepth / SurfaceElevation / SnowDensity so it's
// easy to verify the SWE → visible-depth chain, spot-check save/load
// round-trips, and see what skier wear / grooming is doing to a
// specific cell.
type terrainInspectPanel struct {
	terrain  *world.Terrain
	cell     [2]int
	world    mgl32.Vec3
	fps      float32 // smoothed wall-clock FPS, 0 = not yet sampled
	updateMs float32 // previous frame's Update() wall time
	renderMs float32 // previous frame's Render() wall time
}

func (p *terrainInspectPanel) Draw(r *render.Renderer) {
	c := p.terrain.Cells[p.cell[0]][p.cell[1]]
	fpsRow := "FPS        = —"
	if p.fps > 0 {
		fpsRow = fmt.Sprintf("FPS        = %5.1f  (%.2f ms)", p.fps, 1000.0/p.fps)
	}
	rows := []string{
		fpsRow,
		fmt.Sprintf("  update   = %5.2f ms", p.updateMs),
		fmt.Sprintf("  render   = %5.2f ms", p.renderMs),
		fmt.Sprintf("─── Cell (%d, %d) ───", p.cell[0], p.cell[1]),
		fmt.Sprintf("world      = (%.1f, %.1f)", p.world[0], p.world[2]),
		fmt.Sprintf("Ground     = %.3f m", c.GroundElevation),
		fmt.Sprintf("Surface    = %.3f m", c.SurfaceElevation()),
		fmt.Sprintf("VisDepth   = %.3f m  (derived)", c.VisibleSnowDepth()),
		"",
		fmt.Sprintf("TotalSWE   = %.3f m", c.TotalSWE()),
		fmt.Sprintf("Grooming   = %.3f", c.Grooming),
		fmt.Sprintf("MogulSize  = %.3f", c.MogulSize),
	}
	if c.Top.Accumulation > 0 {
		rows = append(rows, fmt.Sprintf("  Top  %-13s %.2fm",
			world.KindName(c.Top.Kind), c.Top.Accumulation/world.KindDensity(c.Top.Kind)))
	}
	if c.Base > 0 {
		rows = append(rows, fmt.Sprintf("  Base              %.2fm",
			c.Base/world.KindDensity(world.KindBase)))
	}
	rows = append(rows, "",
		fmt.Sprintf("TreeDens   = %.3f", c.TreeDensity),
		fmt.Sprintf("Passable   = %v", c.Passable),
	)
	drawHUDBox(r, rows, 460, mgl32.Vec4{0.85, 0.95, 1, 1}, false)
}

// snowLayerTooltip renders a hover tooltip showing the two-layer snow state for
// the cell under the cursor when the Snow overlay is active.
type snowLayerTooltip struct {
	base    float32      // KindBase SWE (metres)
	top     world.SnowLayer
	mouseX  float32
	mouseY  float32
	screenW int
	screenH int
}

func (t *snowLayerTooltip) Draw(r *render.Renderer) {
	const padX = float32(8)
	const padY = float32(5)
	const lineGap = float32(2)
	lineH := float32(render.GlyphH) + lineGap

	var rows []string
	if t.top.Accumulation == 0 && t.base == 0 {
		rows = []string{"No snow"}
	} else {
		rows = append(rows, "Snow")
		if t.top.Accumulation > 0 {
			visD := t.top.Accumulation / world.KindDensity(t.top.Kind)
			rows = append(rows, fmt.Sprintf("%-13s %s", world.KindName(t.top.Kind), settings.FormatDepth(visD)))
		}
		if t.base > 0 {
			visD := t.base / world.KindDensity(world.KindBase)
			rows = append(rows, fmt.Sprintf("Base          %s", settings.FormatDepth(visD)))
		}
	}

	maxLen := 0
	for _, row := range rows {
		if len(row) > maxLen {
			maxLen = len(row)
		}
	}
	bw := float32(maxLen*render.GlyphAdvance) + 2*padX
	bh := lineH*float32(len(rows)) + 2*padY - lineGap

	const offsetX = float32(14)
	const offsetY = float32(14)
	x := t.mouseX + offsetX
	y := t.mouseY + offsetY
	if x+bw > float32(t.screenW)-2 {
		x = t.mouseX - bw - offsetX
	}
	if y+bh > float32(t.screenH)-2 {
		y = t.mouseY - bh - offsetY
	}

	r.DrawColorRect(x, y, bw, bh, mgl32.Vec4{0.04, 0.08, 0.14, 0.90})
	if r.Font == nil {
		return
	}
	headerCol := mgl32.Vec4{0.55, 0.80, 1.0, 1}
	layerCol := mgl32.Vec4{0.85, 0.93, 1.0, 1}
	for i, row := range rows {
		col := layerCol
		if i == 0 {
			col = headerCol
		}
		r.Font.DrawText(r, row, x+padX, y+padY+float32(i)*lineH, col)
	}
}

type groomingTooltip struct {
	grooming float32
	mouseX   float32
	mouseY   float32
	screenW  int
	screenH  int
}

func (t *groomingTooltip) Draw(r *render.Renderer) {
	const padX = float32(8)
	const padY = float32(5)
	pct := int(t.grooming * 100)
	text := fmt.Sprintf("Grooming  %d%%", pct)

	bw := float32(len(text)*render.GlyphAdvance) + 2*padX
	bh := float32(render.GlyphH) + 2*padY

	const offsetX = float32(14)
	const offsetY = float32(14)
	x := t.mouseX + offsetX
	y := t.mouseY + offsetY
	if x+bw > float32(t.screenW)-2 {
		x = t.mouseX - bw - offsetX
	}
	if y+bh > float32(t.screenH)-2 {
		y = t.mouseY - bh - offsetY
	}

	r.DrawColorRect(x, y, bw, bh, mgl32.Vec4{0.04, 0.08, 0.14, 0.90})
	if r.Font == nil {
		return
	}
	r.Font.DrawText(r, text, x+padX, y+padY, mgl32.Vec4{0.55, 0.95, 0.65, 1})
}

func liftRef(w *world.World, id uint64) string {
	if id == 0 {
		return "—"
	}
	for _, l := range w.Lifts {
		if l.ID == id {
			if l.Name != "" {
				return l.Name
			}
			return fmt.Sprintf("#%d", id)
		}
	}
	return fmt.Sprintf("#%d", id)
}

func buildingRef(w *world.World, id uint64) string {
	if id == 0 {
		return "—"
	}
	for _, b := range w.Buildings {
		if b.ID != id {
			continue
		}
		switch b.Type {
		case world.BuildingLodge:
			return fmt.Sprintf("Lodge#%d", id)
		case world.BuildingParking:
			return fmt.Sprintf("Lot#%d", id)
		}
	}
	return fmt.Sprintf("#%d", id)
}

// drawHUDBox renders a translucent black box with centred text rows at
// vertical offset y. col selects the text colour; centred=true centres
// each row in the box, otherwise rows are left-aligned (used by the
// debug panel where alignment matters more than centring).
func drawHUDBox(r *render.Renderer, rows []string, y float32, col mgl32.Vec4, centred bool) {
	const padX = 10
	const padY = 4
	const lineGap = 2
	maxLen := 0
	for _, row := range rows {
		if len(row) > maxLen {
			maxLen = len(row)
		}
	}
	bw := float32(maxLen*render.GlyphAdvance + 2*padX)
	boxH := float32(render.GlyphH*len(rows)+lineGap*(len(rows)-1)) + float32(2*padY)
	var x float32
	if centred {
		x = (float32(r.ScreenWidth()) - bw) / 2
	} else {
		x = float32(r.ScreenWidth()) - bw - 8 // right-anchored panel
	}
	r.DrawColorRect(x, y, bw, boxH, mgl32.Vec4{0, 0, 0, 0.65})
	if r.Font == nil {
		return
	}
	for i, row := range rows {
		var rowX float32
		if centred {
			rowX = x + (bw-r.Font.TextWidth(row))/2
		} else {
			rowX = x + float32(padX)
		}
		rowY := y + float32(padY) + float32(i*(render.GlyphH+lineGap))
		r.Font.DrawText(r, row, rowX, rowY, col)
	}
}

// seedLabel shows the deterministic RNG seed used by the current run.
// Rendered only in testbed mode (Scenario.simSeed != 0) so the player can read
// off the seed for reproducing a session.
type seedLabel struct {
	seed int64
}

func (sl *seedLabel) Draw(r *render.Renderer) {
	text := fmt.Sprintf("seed=%d", sl.seed)
	boxH := float32(render.GlyphH + 8)
	boxW := float32(len(text)*render.GlyphAdvance + 16)
	x := float32(r.ScreenWidth()) - boxW - 4
	y := float32(102) // just below the 96-px top HUD bar
	textY := y + (boxH-float32(render.GlyphH))/2
	r.DrawColorRect(x, y, boxW, boxH, mgl32.Vec4{0.07, 0.09, 0.15, 0.82})
	r.DrawColorRectOutline(x, y, boxW, boxH, mgl32.Vec4{0.30, 0.44, 0.72, 0.50})
	if r.Font != nil {
		r.Font.DrawText(r, text, x+8, textY, mgl32.Vec4{0.60, 0.75, 1.00, 0.85})
	}
}

// costLabel draws a price tag near the top-left, just below the HUD,
// while a placement tool is active. Green when the player can cover
// the cost, red when they can't.
type costLabel struct {
	cost       int
	affordable bool
}

func (c *costLabel) Draw(r *render.Renderer) {
	text := fmt.Sprintf("$%d", c.cost)
	boxH := float32(render.GlyphH + 8)
	boxW := float32(len(text)*render.GlyphAdvance + 16)
	x := float32(4)
	y := float32(102) // just below the 96-px top HUD bar
	textY := y + (boxH-float32(render.GlyphH))/2
	r.DrawColorRect(x, y, boxW, boxH, mgl32.Vec4{0, 0, 0, 0.7})
	colour := mgl32.Vec4{0.6, 0.95, 0.55, 1}
	if !c.affordable {
		colour = mgl32.Vec4{0.95, 0.45, 0.45, 1}
	}
	if r.Font != nil {
		r.Font.DrawText(r, text, x+8, textY, colour)
	}
}

// hintLabel draws a small text hint at the bottom of the screen.
type hintLabel struct {
	text string
}

func (h *hintLabel) Draw(r *render.Renderer) {
	boxH := float32(render.GlyphH + 8)
	const toolBarReserve = float32(60) // matches Scenario.toolBar.H
	boxW := float32(len(h.text)*render.GlyphAdvance + 16)
	y := float32(r.ScreenHeight()) - boxH - toolBarReserve - 6
	textY := y + (boxH-float32(render.GlyphH))/2
	r.DrawColorRect(4, y, boxW, boxH, mgl32.Vec4{0, 0, 0, 0.7})
	if r.Font != nil {
		r.Font.DrawText(r, h.text, 8+4, textY, mgl32.Vec4{1, 1, 0.5, 1})
	}
}

// liftDropLabel draws the vertical rise of a lift-in-progress as a floating
// pill at the cable midpoint. Shown during the toolLiftTop placement step.
type liftDropLabel struct {
	terrain *world.Terrain
	base    mgl32.Vec2 // XZ of the first-click base station
	hover   mgl32.Vec2 // XZ of the cursor (future top station)
}

func (l *liftDropLabel) Draw(r *render.Renderer) {
	if r.Font == nil {
		return
	}
	baseY := l.terrain.InterpolatedSurfaceElevationAt(l.base[0], l.base[1])
	topY := l.terrain.InterpolatedSurfaceElevationAt(l.hover[0], l.hover[1])
	dx := l.hover[0] - l.base[0]
	dz := l.hover[1] - l.base[1]
	dy := topY - baseY
	cableLenM := float32(math.Sqrt(float64(dx*dx + dz*dz + dy*dy)))

	midX := (l.base[0] + l.hover[0]) * 0.5
	midZ := (l.base[1] + l.hover[1]) * 0.5
	midY := (baseY + topY) * 0.5
	sx, sy, visible := r.WorldToScreen(mgl32.Vec3{midX, midY, midZ})
	if !visible {
		return
	}

	text := settings.FormatElevation(cableLenM)
	tw := r.Font.TextWidth(text) + 14
	th := float32(render.GlyphH) + 6
	bx := sx - tw/2
	by := sy - th/2
	r.DrawColorRect(bx, by, tw, th, mgl32.Vec4{0, 0, 0, 0.55})
	r.Font.DrawText(r, text, bx+7, by+(th-float32(render.GlyphH))/2, mgl32.Vec4{1, 1, 1, 1})
}

// toastLabel draws a transient status message near the bottom-centre of the
// screen. Used for screenshot/log toggle confirmations.
type toastLabel struct {
	text string
}

func (t *toastLabel) Draw(r *render.Renderer) {
	boxH := float32(render.GlyphH + 8)
	const toolBarReserve = float32(60) // matches Scenario.toolBar.H
	var boxW float32
	if r.Font != nil {
		boxW = r.Font.TextWidth(t.text) + 20
	} else {
		boxW = float32(len(t.text)*render.GlyphAdvance + 20)
	}
	x := (float32(r.ScreenWidth()) - boxW) / 2
	y := float32(r.ScreenHeight()) - boxH - toolBarReserve - 30
	textY := y + (boxH-float32(render.GlyphH))/2
	r.DrawColorRect(x, y, boxW, boxH, mgl32.Vec4{0, 0, 0, 0.75})
	if r.Font != nil {
		r.Font.DrawText(r, t.text, x+10, textY, mgl32.Vec4{1, 1, 1, 1})
	}
}

// trailLabels draws each trail's name as a floating world-space label.
// Shown when showAll (overlay bit on) or for the selected/active trail.
type trailLabels struct {
	world         *world.World
	activeTrailID uint64
	showAll       bool
}

func (tl *trailLabels) Draw(r *render.Renderer) {
	if r.Font == nil {
		return
	}
	for _, t := range tl.world.Trails {
		if len(t.Cells) == 0 {
			continue
		}
		isActive := t.ID == tl.activeTrailID
		if !tl.showAll && !isActive {
			continue
		}

		// Centroid world XZ → look up terrain Y for the label anchor.
		c := t.Centroid()
		terrainY := tl.world.Terrain.InterpolatedSurfaceElevationAt(c[0], c[1])
		worldPos := mgl32.Vec3{c[0], terrainY + 8, c[1]} // 8 m above surface

		sx, sy, visible := r.WorldToScreen(worldPos)
		if !visible {
			continue
		}

		label := t.Name
		if label == "" {
			label = "Unnamed"
		}
		tw := r.Font.TextWidth(label) + 14
		th := float32(render.GlyphH) + 6
		bx := sx - tw/2
		by := sy - th/2

		// Background pill.
		r.DrawColorRect(bx, by, tw, th, mgl32.Vec4{0, 0, 0, 0.55})

		// Text colour by difficulty.
		var col mgl32.Vec4
		switch t.Difficulty {
		case world.DiffGreen:
			col = mgl32.Vec4{0.55, 1.0, 0.55, 1}
		case world.DiffBlue:
			col = mgl32.Vec4{0.55, 0.75, 1.0, 1}
		default: // DiffBlack — use light gray so it reads on dark backgrounds
			col = mgl32.Vec4{0.85, 0.85, 0.85, 1}
		}
		r.Font.DrawText(r, label, bx+7, by+(th-float32(render.GlyphH))/2, col)
	}
}

// toggleSkierLog starts or stops a CSV recorder for the followed skier.
func (s *Scenario) toggleSkierLog() {
	if s.csvRecorder != nil {
		s.stopSkierLog("Logging stopped")
		return
	}
	a := s.findFollowedGuest()
	if a == nil {
		s.setToast("Press Tab to follow a skier first")
		return
	}
	if err := os.MkdirAll("debug", 0o755); err != nil {
		s.setToast("Log dir error: " + err.Error())
		return
	}
	ts := time.Now().Format("20060102-150405")
	path := filepath.Join("debug", fmt.Sprintf("skier-%d-%s.csv", a.ID, ts))
	rec, err := sim.NewCSVRecorder(path, a.ID)
	if err != nil {
		s.setToast("Log open error: " + err.Error())
		return
	}
	s.csvRecorder = rec
	s.sim.Recorder = rec
	s.setToast(fmt.Sprintf("Logging skier #%d → %s", a.ID, path))
}

// stopSkierLog flushes and closes the active recorder, if any. msg is shown
// in the toast (along with the path) so the user knows where the file landed.
func (s *Scenario) stopSkierLog(msg string) {
	if s.csvRecorder == nil {
		return
	}
	path := s.csvRecorder.Path()
	if err := s.csvRecorder.Close(); err != nil {
		fmt.Println("CSV close error:", err)
	}
	s.csvRecorder = nil
	s.sim.Recorder = nil
	if msg == "" {
		msg = "Logging stopped"
	}
	s.setToast(msg + " → " + path)
}

// setToast displays a transient status message for ~3 seconds.
func (s *Scenario) setToast(text string) {
	s.toastText = text
	s.toastExpiry = s.time + 3.0
}

// savePrompt is the modal "Save As" widget. Owns its own TextInput plus
// OK / Cancel buttons; the parent Scenario routes input to it whenever
// non-nil and draws it as the topmost UI element.
type savePrompt struct {
	input    *ui.TextInput
	okBtn    *ui.Button
	cancelBtn *ui.Button
	onSubmit func(string)
	onCancel func()
}

func newSavePrompt(initial string, onSubmit func(string), onCancel func()) *savePrompt {
	p := &savePrompt{onSubmit: onSubmit, onCancel: onCancel}
	p.input = ui.NewTextInput(0, 0, 0, 32, initial)
	p.input.OnSubmit = func(text string) { p.onSubmit(text) }
	p.input.OnCancel = func() { p.onCancel() }
	p.okBtn = ui.NewButton(0, 0, 90, 32, "Save", func() { p.onSubmit(p.input.Text) })
	p.cancelBtn = ui.NewButton(0, 0, 90, 32, "Cancel", func() { p.onCancel() })
	return p
}

const savePromptW = 420
const savePromptH = 150

func (p *savePrompt) layout(sw, sh float32) {
	x := (sw - savePromptW) / 2
	y := (sh - savePromptH) / 2
	const pad = 16
	p.input.X = x + pad
	p.input.Y = y + 50
	p.input.W = savePromptW - 2*pad
	p.okBtn.X = x + savePromptW - pad - p.okBtn.W
	p.okBtn.Y = y + savePromptH - pad - p.okBtn.H
	p.cancelBtn.X = p.okBtn.X - 12 - p.cancelBtn.W
	p.cancelBtn.Y = p.okBtn.Y
}

func (p *savePrompt) HandleInput(inp *engine.Input, sw, sh float32) {
	p.layout(sw, sh)
	// Modal: any click while the prompt is up belongs to the prompt — the
	// dimmed background is part of the modal, not a passthrough.
	if inp.LeftClick {
		inp.LeftClickConsumed = true
	}
	p.input.HandleInput(inp)
	mx, my := inp.MousePos[0], inp.MousePos[1]
	for _, b := range []*ui.Button{p.okBtn, p.cancelBtn} {
		b.SetHovered(b.Contains(mx, my))
		if inp.LeftClick && b.Contains(mx, my) {
			b.Click()
			return
		}
	}
}

func (p *savePrompt) Draw(r *render.Renderer) {
	sw := float32(r.ScreenWidth())
	sh := float32(r.ScreenHeight())
	p.layout(sw, sh)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	defer gl.Disable(gl.BLEND)
	r.DrawColorRect(0, 0, sw, sh, mgl32.Vec4{0, 0, 0, 0.55})
	x := (sw - savePromptW) / 2
	y := (sh - savePromptH) / 2
	r.DrawColorRect(x, y, savePromptW, savePromptH, mgl32.Vec4{0.08, 0.12, 0.22, 0.98})
	if r.Font != nil {
		r.Font.DrawText(r, "Save As", x+16, y+16, mgl32.Vec4{1, 0.95, 0.8, 1})
	}
	p.input.Draw(r)
	p.okBtn.Draw(r)
	p.cancelBtn.Draw(r)
}

// entityName returns a closure that resolves an entity ID to a human-readable
// name for use in Thought.Display. Checks lifts, then trails, then buildings.
func entityName(w *world.World) func(uint64) string {
	return func(id uint64) string {
		for _, l := range w.Lifts {
			if l.ID == id {
				if l.Name != "" {
					return l.Name
				}
				return fmt.Sprintf("Lift #%d", id)
			}
		}
		if t := w.FindTrail(id); t != nil {
			if t.Name != "" {
				return t.Name
			}
			return fmt.Sprintf("trail #%d", id)
		}
		for _, b := range w.Buildings {
			if b.ID == id {
				return fmt.Sprintf("#%d", id)
			}
		}
		return ""
	}
}
