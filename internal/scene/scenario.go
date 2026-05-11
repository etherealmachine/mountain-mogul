package scene

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/save"
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

// applyLiftPlacementEffects applies the ground-side consequences of putting
// down a lift: a tree-free maintenance corridor under the cable line and
// raised boarding/exit aprons at the base and top stations.
//
// Both aprons are raise-only — real ski areas typically build up an
// earthwork pad at each station rather than carve a notch into the
// hillside, so cells whose natural elevation is already higher than the
// station footing keep their elevation untouched and only lower-lying
// cells get raised toward the station. Sizing is fixed at 40 × 24 m
// (8 × 5 cells at 5 m granularity); the lift-station mesh itself is
// smaller than a single cell, so the apron is what gives the lift
// visual mass on the map.
//
// Lives in scenario.go (not world.PlaceLift) so save loading and testbed
// setup can reconstruct lifts without re-flattening or re-clearing terrain
// the player may have intentionally edited.
func applyLiftPlacementEffects(t *world.Terrain, lift *world.Lift) {
	const corridorHalfWidth = float32(12.0) // → 24 m wide maintenance lane
	const apronHalfWidth = float32(20.0)    // → 40 m total cross-axis width
	const apronDepth = float32(24.0)
	const apronBuildup = float32(2.5) // metres above station footing
	clearLiftCorridor(t, lift.Base, lift.Top, corridorHalfWidth)
	axis := mgl32.Vec2{
		lift.Top[0] - lift.Base[0],
		lift.Top[1] - lift.Base[1],
	}
	if l := axis.Len(); l > 0 {
		axis = axis.Mul(1 / l)
	}
	// Top: extends forward (along axis). Base: extends backward (against
	// axis). Both raise-only — see buildStationApron docs.
	buildStationApron(t, lift.Top, axis, +1, apronHalfWidth, apronDepth, apronBuildup)
	buildStationApron(t, lift.Base, axis, -1, apronHalfWidth, apronDepth, apronBuildup)
}

// applyBuildingPlacementEffects grooms a small square apron around a
// lodge so the lodge doesn't appear to be perched on a mogul field or
// surrounded by ungroomed powder. Smaller than lift aprons (lodges have
// less of a footprint) and unlike lifts it doesn't raise the ground —
// lodges fit the natural slope. Same falloff math, axis-aligned, applied
// once at placement (same contract as applyLiftPlacementEffects).
func applyBuildingPlacementEffects(t *world.Terrain, b *world.Building) {
	const apronHalfWidth = float32(12.0) // → 24 m square apron
	const apronBuildup = float32(0.5)    // a small graded pad so doorsteps don't sink into the hill
	// Two passes along orthogonal axes give a square apron with side
	// falloff on all four edges (full weight in the inner core, smoothstep
	// down to nothing at the edge).
	buildStationApron(t, b.Pos, mgl32.Vec2{1, 0}, +1, apronHalfWidth, apronHalfWidth, apronBuildup)
	buildStationApron(t, b.Pos, mgl32.Vec2{1, 0}, -1, apronHalfWidth, apronHalfWidth, apronBuildup)
}

// clearLiftCorridor zeros TreeDensity in cells within `halfWidth` metres of
// the line segment between two world XZ points. Models the standard
// chairlift maintenance lane — trees would otherwise foul cables, towers,
// and the over-snow grooming machines that service the line.
func clearLiftCorridor(t *world.Terrain, base, top mgl32.Vec2, halfWidth float32) {
	const cellSize = float32(5.0)
	minX := minF(base[0], top[0]) - halfWidth
	maxX := maxF(base[0], top[0]) + halfWidth
	minZ := minF(base[1], top[1]) - halfWidth
	maxZ := maxF(base[1], top[1]) + halfWidth
	x0 := int(minX / cellSize)
	x1 := int(maxX/cellSize) + 1
	z0 := int(minZ / cellSize)
	z1 := int(maxZ/cellSize) + 1
	hw2 := halfWidth * halfWidth
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			cx := (float32(x) + 0.5) * cellSize
			cz := (float32(z) + 0.5) * cellSize
			if pointSegmentDistSq(mgl32.Vec2{cx, cz}, base, top) <= hw2 {
				t.Cells[x][z].TreeDensity = 0
			}
		}
	}
}

// buildStationApron raises terrain in a rectangular pad whose back edge
// sits flush against `station`, extending `depth` metres along `axis * side`
// (the apron-forward direction) and ±`halfWidth` metres along the perpendicular.
// The pad is raised to `stationElev + buildup` so it sits visibly elevated
// above the natural ground even on flat terrain — without a buildup, lifts
// placed in nearly-flat spots get an apron with nothing to raise *to* and
// look unmodified.
//
// Raise-only: only cells whose natural elevation is below the target get
// pulled up; cells already higher are left alone. Real ski areas grade
// these areas as built-up earthwork pads rather than carving notches into
// the hillside, so this is closer to how a boarding/exit area actually
// looks at a real lift.
//
// Falloff is applied at the front edge and the two side edges — where skiers
// exit off the chair, in any of the forward / left / right directions — but
// NOT at the back edge against the lift, which keeps a sharp transition.
//
// `axis` is a unit vector along the cable (base → top). `side = +1` flips
// the apron to extend in the +axis direction (top station: forward toward
// the front of the platform), `side = -1` extends the apron in the -axis
// direction (base station: back toward the boarding queue).
func buildStationApron(t *world.Terrain, station, axis mgl32.Vec2, side, halfWidth, depth, buildup float32) {
	const cellSize = float32(5.0)
	stationCell := [2]int{
		int(station[0] / cellSize),
		int(station[1] / cellSize),
	}
	if !t.InBounds(stationCell[0], stationCell[1]) {
		return
	}
	target := t.Cells[stationCell[0]][stationCell[1]].GroundElevation + buildup
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
			if target <= cur {
				// Raise-only: cells whose natural elevation is already
				// at or above the target pad height keep what they have.
				// Real lift aprons are built-up earthwork pads, not
				// notches carved into a hillside.
				continue
			}
			t.Cells[x][z].GroundElevation = cur + (target-cur)*w
			// Apron snow: a thin packed layer. Building pads aren't
			// corduroyed by snowcats — they're foot-tracked and
			// machine-compacted, so they read as packed flat snow
			// rather than tilled stripes. SnowDepth tapers along the
			// falloff so apron edges blend into the surrounding
			// snowpack. Grooming is explicitly NOT raised here; if
			// the player grooms over the apron later, fine, but the
			// default state of an apron is packed-not-groomed.
			const apronSnow = float32(0.05)
			deepSnow := t.Cells[x][z].SnowDepth
			if deepSnow > apronSnow {
				t.Cells[x][z].SnowDepth = apronSnow + (deepSnow-apronSnow)*(1-w)
			}
			if w > t.Cells[x][z].Packed {
				t.Cells[x][z].Packed = w
			}
			t.Cells[x][z].MogulSize *= 1 - w
			t.Cells[x][z].Ice *= 1 - w
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
	toolBuilding    toolMode = iota // place a lodge
	toolShed        toolMode = iota // place an equipment shed
	toolLiftBase    toolMode = iota // waiting for first lift click
	toolLiftTop     toolMode = iota // waiting for second lift click
	toolGlade       toolMode = iota // reduce TreeDensity (brush)
	toolPlantTrees  toolMode = iota // increase TreeDensity (brush, editor only)
	toolRemove      toolMode = iota // remove building at clicked cell
	toolRoute       toolMode = iota // paint snowcat route cells onto a selected shed
)

// Scenario is the main gameplay scene.
type Scenario struct {
	app             *engine.App
	world           *world.World
	sim             *sim.Simulation
	toolBar         *ui.MenuBar      // bottom-of-screen tool palette
	topBar          *ui.TopBar       // resort-management HUD strip
	overlayPanel    *ui.OverlayPanel // right-side terrain-overlay toggles
	escapeMenu      *EscapeMenu
	toolButtons     map[toolMode]*ui.Button
	activeTool      toolMode
	liftBase        mgl32.Vec2 // first click world position for lift placement
	scenarioPath    string
	time            float32
	rightDragging   bool
	hoverCell       [2]int     // terrain cell under the mouse — for cell-based tools
	hoverWorld      mgl32.Vec3 // continuous terrain hit under the mouse — for placement
	hoverValid      bool       // false when the mouse is off-terrain or over the menu bars
	followAgentID   uint64 // 0 = free camera; >0 = ID of followed skier
	firstPerson     bool   // V: first-person camera at the followed skier's head
	debugSteering   bool   // F3: render steering forces on the followed skier
	paused          bool
	popup           *ui.Window
	saveAllowed     bool   // false in testbed mode; gates the Save prompt
	saveName        string // last name used for Save; pre-fills the prompt next time
	savePrompt      *savePrompt
	prebuiltWorld   *world.World
	simSeed         int64                          // 0 = wall-clock; nonzero forces deterministic RNG
	rebuild         func(seed int64) *world.World // non-nil ⇒ "New Seed" button shown

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

	// Route-paint mode: while toolRoute is active, the player drag-
	// paints route cells onto the shed identified by routeShedID. 0 =
	// no shed selected (the tool is a no-op).
	routeShedID  uint64
	lastRouteCell [2]int

	// Debug instrumentation (see plan: orbiting-skier debug aids).
	csvRecorder       *sim.CSVRecorder
	pendingScreenshot bool
	toastText         string
	toastExpiry       float32 // s.time at which toast disappears

	// Trail of world-space positions for the currently followed skier.
	// Reset when followAgentID changes; appended when at least
	// trackMinSpacing metres past the last sample.
	track       []mgl32.Vec3
	trackOwner  uint64
}

const (
	trackMinSpacing = 0.5  // m; minimum distance from last sample before appending
	trackMaxPoints  = 6000 // hard cap; old points dropped when exceeded
)

// speedOptions lists the time-scale presets shown in the top bar.
// 1× is real-time, 2× and 4× are the two fast-forward steps. Pause is its
// own button — not in this list.
var speedOptions = []float64{1, 2, 4}


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
	GenerateTreeCover(s.world.Terrain, 24, 0.55, seed)
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
func NewScenarioFromTestbed(tb *sim.Testbed) *Scenario {
	rebuild := func(seed int64) *world.World {
		return tb.Build(rand.New(rand.NewSource(seed)))
	}
	return &Scenario{
		prebuiltWorld: rebuild(tb.Seed),
		simSeed:       tb.Seed,
		rebuild:       rebuild,
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

	if s.saveAllowed {
		s.escapeMenu = NewEscapeMenu(app, s.openSavePrompt, s.gotoLoadMenu)
	} else {
		s.escapeMenu = NewEscapeMenu(app, nil, nil)
	}

	// Bottom tool bar — palette of construction tools, centred along the
	// bottom edge. Y is set each frame in Update() based on the current
	// screen height. Tall enough to fit a 24-px icon plus a label row.
	const toolBarH = float32(60)
	s.toolButtons = make(map[toolMode]*ui.Button)
	s.toolBar = ui.NewMenuBar(0, toolBarH)
	s.toolBar.Centered = true
	s.toolButtons[toolBuilding] = s.toolBar.AddIconButton(render.IconHouse, "Lodge", func() { s.setTool(toolBuilding) })
	s.toolButtons[toolShed] = s.toolBar.AddIconButton(render.IconGarage, "Shed", func() { s.setTool(toolShed) })
	s.toolButtons[toolLiftBase] = s.toolBar.AddIconButton(render.IconCableCar, "Lift", func() { s.setTool(toolLiftBase) })
	s.toolButtons[toolGlade] = s.toolBar.AddIconButton(render.IconAxe, "Glade", func() { s.setTool(toolGlade) })
	s.toolButtons[toolRemove] = s.toolBar.AddIconButton(render.IconTrash, "Remove", func() { s.setTool(toolRemove) })
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
	s.topBar.GetGuests = func() int { return len(s.world.Agents) }
	s.topBar.GetHappiness = func() float32 { return resortHappiness(s.world) }
	s.topBar.GetDate = func() (int, string, int) {
		d := sim.CalendarAt(s.sim.SimTime)
		return d.Day, d.Month, d.Year
	}
	s.topBar.GetWeather = func() (ui.WeatherKind, ui.WeatherKind, int) {
		ws := sim.WeatherAt(s.sim.SimTime)
		return weatherToUI(ws.Now), weatherToUI(ws.Next), ws.TempF
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
		func() { s.escapeMenu.Toggle() },
	)
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

	return nil
}

// weatherToUI maps the sim-side weather enum to the UI-side enum. The two
// packages can't import each other so the scene does the translation.
func weatherToUI(w sim.WeatherKind) ui.WeatherKind {
	switch w {
	case sim.WeatherSunny:
		return ui.WKSunny
	case sim.WeatherCloudy:
		return ui.WKCloudy
	case sim.WeatherSnowing:
		return ui.WKSnow
	case sim.WeatherStormy:
		return ui.WKStorm
	}
	return ui.WKSunny
}

// resortHappiness is a placeholder readout for the top-bar happiness bar.
// Stays at 0.80 today; the eventual model will derive this from per-agent
// satisfaction (lift waits, terrain match, etc.). Wired up to a function
// rather than a constant so swapping in the real signal is a one-line change.
func resortHappiness(w *world.World) float32 {
	if w == nil {
		return 0.0
	}
	return 0.80
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
	if s.simSeed != 0 {
		s.sim = sim.NewSimulationWithSeed(w, s.simSeed)
	} else {
		s.sim = sim.NewSimulation(w)
	}
	r.BuildTerrainMesh(w.Terrain)
	r.RebuildStaticBatch(w)
	for _, lift := range w.Lifts {
		r.AddLiftCable(lift, w.Terrain)
	}
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
	s.followAgentID = 0
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

	// Save prompt is the topmost modal — it captures all input and even
	// swallows Escape (handled inside its TextInput's Cancel binding).
	if s.savePrompt != nil {
		s.savePrompt.HandleInput(inp, float32(r.ScreenWidth()), float32(r.ScreenHeight()))
		return
	}

	if inp.Pressed[glfw.KeyEscape] {
		if s.activeTool != toolNone {
			s.cancelTool()
		} else {
			s.escapeMenu.Toggle()
		}
	}
	if s.escapeMenu.Visible() {
		s.escapeMenu.HandleInput(inp)
		return
	}

	// Tab: cycle the followed skier (first press picks random, subsequent press advances).
	if inp.Pressed[glfw.KeyTab] {
		s.cycleFollow()
	}

	// V: toggle first-person camera at the followed skier's head. No-op
	// when nobody is followed — FPV without a target would have no
	// anchor.
	if inp.Pressed[glfw.KeyV] && s.followAgentID != 0 {
		s.firstPerson = !s.firstPerson
	}

	// C: quick toggle for the contour overlay. The full overlay panel is
	// behind the top-bar stack button; this hotkey is kept for muscle
	// memory since contour was the most-used legacy overlay.
	if inp.Pressed[glfw.KeyC] {
		s.overlayPanel.ToggleBit(render.OverlayContour)
		r.TerrainOverlayMode = s.overlayPanel.Mask()
	}

	// F3: toggle steering debug overlay (visualises forces for followed skier).
	if inp.Pressed[glfw.KeyF3] {
		s.debugSteering = !s.debugSteering
		if !s.debugSteering {
			r.SetDebugLines(nil)
		}
	}

	// L: toggle CSV log of the followed skier (debug instrumentation).
	if inp.Pressed[glfw.KeyL] {
		s.toggleSkierLog()
	}

	// F12: capture a screenshot of the current frame to debug/screens/.
	if inp.Pressed[glfw.KeyF12] {
		s.pendingScreenshot = true
	}

	// Auto-stop CSV log if the followed skier changed or no longer exists.
	if s.csvRecorder != nil {
		a := s.findFollowedAgent()
		if a == nil || a.ID != s.csvRecorder.AgentID() {
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
			s.followAgentID = 0
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
		s.followAgentID = 0
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

	// Ghost preview for lift placement.
	s.updatePlacementGhost(r)

	// Popup window input — handle before world clicks so buttons consume the event.
	popupConsumed := false
	if s.popup != nil && s.popup.Visible {
		s.popup.HandleInput(inp)
		if inp.LeftClick && s.popup.ContainsPoint(inp.MousePos[0], inp.MousePos[1]) {
			popupConsumed = true
		}
	}

	// World click / drag — glade supports held-down; placement tools use click-only.
	clickConsumed := popupConsumed
	screenW := float32(r.ScreenWidth())
	if !clickConsumed && inp.LeftClick && s.activeTool == toolNone && !s.uiCovers(inp.MousePos[0], inp.MousePos[1], screenW) {
		// Skier pick takes priority over popups when no tool is active.
		if a := s.pickAgent(r.Camera, inp.MousePos); a != nil {
			s.followAgentID = a.ID
			if s.popup != nil {
				s.popup.Visible = false
			}
			clickConsumed = true
		}
	}
	// Glade sliders take input before the brush so dragging the thumb
	// doesn't also paint underneath it.
	sliderActive := false
	if s.activeTool == toolGlade {
		s.layoutGladeSliders(r)
		if s.gladeRadiusSlider.HandleInput(inp.MousePos[0], inp.MousePos[1], inp.LeftClick, inp.LeftHeld) {
			sliderActive = true
		}
		if s.gladeThinSlider.HandleInput(inp.MousePos[0], inp.MousePos[1], inp.LeftClick, inp.LeftHeld) {
			sliderActive = true
		}
	}

	// End-of-stroke reset for glade and route drag-paint. As soon as
	// the mouse is up, forget the last cell we applied to so the next
	// click starts a fresh stroke (and won't drag-apply on its own
	// initial frame).
	if !inp.LeftHeld {
		s.lastGladeCell = [2]int{-1, -1}
		s.lastRouteCell = [2]int{-1, -1}
	}

	if !clickConsumed && !sliderActive {
		// Glade and route drag-painting: apply once when the cursor moves
		// into a new cell while held. Requires a prior valid last*Cell —
		// so holding LMB from a toolbar click doesn't auto-apply on
		// entering terrain, only a real click on terrain starts a stroke.
		gladeDragged := s.activeTool == toolGlade && inp.LeftHeld &&
			s.lastGladeCell != [2]int{-1, -1} &&
			s.hoverCell != s.lastGladeCell
		routeDragged := s.activeTool == toolRoute && inp.LeftHeld &&
			s.lastRouteCell != [2]int{-1, -1} &&
			s.hoverCell != s.lastRouteCell
		clickOrDrag := inp.LeftClick || gladeDragged || routeDragged
		if clickOrDrag && !s.uiCovers(inp.MousePos[0], inp.MousePos[1], screenW) && s.hoverValid {
			overSlider := s.activeTool == toolGlade &&
				(s.gladeRadiusSlider.Contains(inp.MousePos[0], inp.MousePos[1]) ||
					s.gladeThinSlider.Contains(inp.MousePos[0], inp.MousePos[1]))
			gx, gz := s.hoverCell[0], s.hoverCell[1]
			if !overSlider && s.world.Terrain.InBounds(gx, gz) {
				if s.activeTool == toolNone && inp.LeftClick {
					s.tryOpenPopup(s.hoverWorld, r.ScreenWidth(), r.ScreenHeight())
				} else {
					if s.activeTool == toolGlade {
						s.lastGladeCell = s.hoverCell
					}
					if s.activeTool == toolRoute {
						s.lastRouteCell = s.hoverCell
					}
					s.applyTool(r)
				}
			}
		}
	}

	// Tick simulation (skipped while paused).
	if !s.paused {
		s.sim.Tick(dt)
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
		s.world.Terrain.SnowDirty = false
	}

	// Camera follow: track the selected agent using the freshest positions.
	// In first-person mode, drive the perspective camera to the skier's
	// head and hide their mesh so the camera isn't sitting inside the
	// torso; otherwise stay in the default isometric ortho follow.
	r.HiddenAgentID = 0
	r.HiddenRadius = 0
	if s.followAgentID != 0 {
		if agent := s.findFollowedAgent(); agent != nil {
			if s.firstPerson {
				applyFirstPersonCamera(r.Camera, agent)
				r.HiddenAgentID = agent.ID
				// Lift queues stack all queuers at the same cell, so a
				// queued skier's neighbours sit right in the camera.
				// Hide anyone within ~1.5 m of the followed skier so
				// the FPV doesn't get a green box stuck to its face.
				r.HiddenAgentPos = agent.Pos
				r.HiddenRadius = 1.5
			} else {
				if r.Camera.Perspective {
					r.Camera.Perspective = false
				}
				r.Camera.Target = agent.Pos
				r.Camera.Recalculate()
			}
		} else {
			s.followAgentID = 0
		}
	}
	// Follow dropped (despawn / right-drag pan / etc.) — exit FPV so
	// the free camera comes back in ortho.
	if s.followAgentID == 0 && (s.firstPerson || r.Camera.Perspective) {
		s.firstPerson = false
		r.Camera.Perspective = false
		r.Camera.Recalculate()
	}

	// Track + optional steering overlay for the followed skier.
	s.updateOverlay(r)
}

// updateOverlay maintains the followed skier's track and (optionally) the
// steering-debug visualisation, then pushes both to the renderer in a single
// SetDebugLines batch. The track resets whenever the followed skier changes
// or follow is disabled.
//
// Also draws route cell outlines for any shed whose route should be
// visible — every shed while toolRoute is active, and just the
// selected shed while in route-paint mode. Route lines run alongside
// the skier track in the same batch so we keep one buffer upload per
// frame.
func (s *Scenario) updateOverlay(r *render.Renderer) {
	a := s.findFollowedAgent()

	// Reset the track when follow target changes (including dropped follow).
	if a == nil || a.ID != s.trackOwner {
		s.track = s.track[:0]
		s.trackOwner = 0
	}
	if a == nil {
		// No skier followed — but route cells may still need drawing
		// during route paint, so push just those.
		r.SetDebugLines(s.routeOverlayLines())
		return
	}
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

	lines := make([]render.DebugLine, 0, len(s.track)+8)
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

	lines = append(lines, s.routeOverlayLines()...)
	r.SetDebugLines(lines)
}

// routeOverlayLines builds the cell-outline overlay for shed routes.
// While the route-paint tool is active the selected shed's cells are
// drawn in bright cyan; other sheds' routes are drawn in a dimmer
// teal so the player can see how their resort's grooming coverage
// distributes. Outside paint mode no route lines are drawn — the
// terrain stays unobstructed for normal play.
//
// Each cell is drawn as a perimeter + diagonal cross so the marks read
// as filled-ish cells rather than thin outlines that disappear against
// busy terrain. The lines hover 2 m above the surface so vertex jitter
// and snow piles don't clip them.
func (s *Scenario) routeOverlayLines() []render.DebugLine {
	if s.activeTool != toolRoute {
		return nil
	}
	const hover = float32(2.0)
	const cellSize = float32(5.0)
	bright := [3]float32{0.40, 1.00, 1.00}
	dim := [3]float32{0.18, 0.55, 0.65}

	out := make([]render.DebugLine, 0, 64)
	addCell := func(c [2]int, col [3]float32) {
		x0 := float32(c[0]) * cellSize
		x1 := x0 + cellSize
		z0 := float32(c[1]) * cellSize
		z1 := z0 + cellSize
		y00 := s.world.Terrain.InterpolatedSurfaceElevationAt(x0, z0) + hover
		y10 := s.world.Terrain.InterpolatedSurfaceElevationAt(x1, z0) + hover
		y01 := s.world.Terrain.InterpolatedSurfaceElevationAt(x0, z1) + hover
		y11 := s.world.Terrain.InterpolatedSurfaceElevationAt(x1, z1) + hover
		p00 := mgl32.Vec3{x0, y00, z0}
		p10 := mgl32.Vec3{x1, y10, z0}
		p11 := mgl32.Vec3{x1, y11, z1}
		p01 := mgl32.Vec3{x0, y01, z1}
		out = append(out,
			// perimeter
			render.DebugLine{A: p00, B: p10, Color: col},
			render.DebugLine{A: p10, B: p11, Color: col},
			render.DebugLine{A: p11, B: p01, Color: col},
			render.DebugLine{A: p01, B: p00, Color: col},
			// diagonals — makes cells read as fills at a glance
			render.DebugLine{A: p00, B: p11, Color: col},
			render.DebugLine{A: p10, B: p01, Color: col},
		)
	}
	for _, b := range s.world.Buildings {
		if b.Type != world.BuildingShed {
			continue
		}
		col := dim
		if b.ID == s.routeShedID {
			col = bright
		}
		for _, c := range b.RouteCells {
			addCell(c, col)
		}
	}
	return out
}

// steeringLines builds the F3 steering-debug visualisation for the agent.
func steeringLines(w *world.World, a *world.Agent, target mgl32.Vec3) []render.DebugLine {
	d := sim.ComputeSteeringDebug(w.Terrain, a, target)
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
func skierTarget(w *world.World, a *world.Agent) (mgl32.Vec3, bool) {
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
		if w.Cash < world.LodgeCost {
			s.setToast(fmt.Sprintf("Need $%d for a lodge — short by $%d",
				world.LodgeCost, world.LodgeCost-w.Cash))
			return
		}
		w.Cash -= world.LodgeCost
		b := w.PlaceBuildingType(world.BuildingLodge, wx, wz)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
	case toolShed:
		if w.Cash < world.ShedCost {
			s.setToast(fmt.Sprintf("Need $%d for a shed — short by $%d",
				world.ShedCost, world.ShedCost-w.Cash))
			return
		}
		w.Cash -= world.ShedCost
		b := w.PlaceBuildingType(world.BuildingShed, wx, wz)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
	case toolGlade:
		// Slider value 0–10 = % density delta per application. Each click
		// or drag-into-new-cell event fires one application; stationary
		// holding does nothing further (see lastGladeCell gate in Update).
		strength := s.gladeThinSlider.Value / 100
		applyDensityBrush(w.Terrain, gx, gz, s.gladeBrushRadius(), -strength)
		r.RebuildStaticBatch(w)
	case toolLiftBase:
		// Cheapest a lift can be is the station-pair fee — gate
		// entry to the two-click flow so the player can't waste a
		// click setting a base they could never afford a top from.
		if w.Cash < world.LiftStationCost {
			s.setToast(fmt.Sprintf("Need $%d for a lift — short by $%d",
				world.LiftStationCost, world.LiftStationCost-w.Cash))
			return
		}
		s.liftBase = mgl32.Vec2{wx, wz}
		s.activeTool = toolLiftTop
	case toolLiftTop:
		top := mgl32.Vec2{wx, wz}
		cost := world.LiftCost(s.liftBase, top)
		if w.Cash < cost {
			s.setToast(fmt.Sprintf("Need $%d for this lift — short by $%d",
				cost, cost-w.Cash))
			return
		}
		w.Cash -= cost
		lift := w.PlaceLift(s.liftBase[0], s.liftBase[1], wx, wz)
		applyLiftPlacementEffects(w.Terrain, lift)
		r.FlushTerrainVerts(w.Terrain)
		r.AddLiftCable(lift, w.Terrain)
		r.RebuildStaticBatch(w)
		r.ClearAllGhosts()
		r.ClearGhostCable()
		s.activeTool = toolNone
	case toolRemove:
		s.removeAt(s.hoverWorld, r)
	case toolRoute:
		s.applyRoutePaint(gx, gz)
	}
}

// routeBrushRadius is the half-width of the route-paint brush in cells.
// At radius 2 the brush is a 5×5-cell disc; the player drag-paints
// roughly 5 m wide strips per stroke. Matches the cat's tiller width
// (one cell) but lets a single stroke cover more ground.
const routeBrushRadius = 2

// applyRoutePaint adds every in-bounds cell within `routeBrushRadius`
// of (gx, gz) to the route owned by s.routeShedID, up to the per-shed
// cap. Idempotent on already-painted cells; stops mid-brush if the cap
// is hit so the player gets a clear "ran out of capacity" toast.
func (s *Scenario) applyRoutePaint(gx, gz int) {
	if s.routeShedID == 0 {
		return
	}
	shed := s.findBuilding(s.routeShedID)
	if shed == nil || shed.Type != world.BuildingShed {
		return
	}
	cap := world.MaxRouteCells(shed.Cats)
	hit := make(map[[2]int]bool, len(shed.RouteCells))
	for _, c := range shed.RouteCells {
		hit[c] = true
	}
	r2 := routeBrushRadius * routeBrushRadius
	for dz := -routeBrushRadius; dz <= routeBrushRadius; dz++ {
		for dx := -routeBrushRadius; dx <= routeBrushRadius; dx++ {
			if dx*dx+dz*dz > r2 {
				continue
			}
			x, z := gx+dx, gz+dz
			if !s.world.Terrain.InBounds(x, z) {
				continue
			}
			cell := [2]int{x, z}
			if hit[cell] {
				continue
			}
			if len(shed.RouteCells) >= cap {
				s.setToast(fmt.Sprintf("Route is full (%d cells). Add a cat to extend.", cap))
				return
			}
			shed.RouteCells = append(shed.RouteCells, cell)
			hit[cell] = true
		}
	}
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
			w.RemoveBuilding(b.ID)
			r.RebuildStaticBatch(w)
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
	case s.activeTool == toolRoute && t.InBounds(s.hoverCell[0], s.hoverCell[1]):
		gx, gz := s.hoverCell[0], s.hoverCell[1]
		center := mgl32.Vec2{float32(gx)*cellSize + cellSize/2, float32(gz)*cellSize + cellSize/2}
		r.SetBrush(center, (float32(routeBrushRadius)+0.5)*cellSize)
	default:
		r.ClearBrush()
	}

	r.HighlightAgentID = s.followAgentID
	s.applyPerceptionCone(r)
	r.DrawWorld(s.world, s.time)
	r.ClearBrush()
	r.ClearPerceptionCone()

	// Re-anchor the bottom tool bar to the live screen height before draw.
	s.toolBar.Y = float32(r.ScreenHeight()) - s.toolBar.H
	if s.overlayPanel != nil {
		s.overlayPanel.Bottom = float32(r.ScreenHeight()) - s.toolBar.H
	}
	drawables := []render.UIDrawable{s.topBar, s.toolBar, s.overlayPanel}
	if s.simSeed != 0 {
		drawables = append(drawables, &seedLabel{seed: s.simSeed})
	}
	if s.activeTool == toolGlade {
		s.layoutGladeSliders(r)
		drawables = append(drawables, s.gladeRadiusSlider, s.gladeThinSlider)
	}
	if s.activeTool == toolLiftTop {
		drawables = append(drawables, &hintLabel{text: "Click to set lift top"})
	}
	if s.activeTool == toolRoute {
		if shed := s.findBuilding(s.routeShedID); shed != nil {
			cap := world.MaxRouteCells(shed.Cats)
			used := len(shed.RouteCells)
			drawables = append(drawables, &hintLabel{
				text: fmt.Sprintf("Route: %d / %d cells (%d cats) — drag to paint, Esc to finish",
					used, cap, shed.Cats),
			})
		}
	}
	if cost, affordable, valid := s.placementCost(); valid {
		drawables = append(drawables, &costLabel{cost: cost, affordable: affordable})
	}
	for _, a := range s.world.Agents {
		if a.ID == s.followAgentID {
			drawables = append(drawables, &followLabel{world: s.world, agent: a})
			break
		}
	}
	if s.popup != nil && s.popup.Visible {
		drawables = append(drawables, s.popup)
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
	r.DrawUI(drawables)

	// Screenshot is taken AFTER UI is drawn so the captured frame matches what
	// the user sees. The "saved" toast is set after capture so it appears only
	// on subsequent frames, never in the screenshot itself.
	if s.pendingScreenshot {
		s.pendingScreenshot = false
		path := filepath.Join("debug", "screens", time.Now().Format("20060102-150405")+".png")
		if err := r.SaveScreenshot(path); err != nil {
			s.setToast("Screenshot failed: " + err.Error())
		} else {
			s.setToast("Screenshot saved → " + path)
		}
	}
}

func (s *Scenario) Destroy() {
	if s.app != nil && s.app.Renderer != nil {
		s.app.Renderer.ResetSceneState()
	}
}

// tryOpenPopup opens a popup window if the click is within the pick
// radius of a building or any part of a lift (base, top, or anywhere
// along the cable line). Buildings have priority — clicks inside a
// building's footprint open the lodge panel even if a lift happens to
// be near. The cable check uses point-to-segment distance so towers
// (which sit on the cable line) and stations are all selectable.
func (s *Scenario) tryOpenPopup(clickPos mgl32.Vec3, screenW, screenH int) {
	pick := mgl32.Vec2{clickPos[0], clickPos[2]}
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
	if s.popup != nil {
		s.popup.Visible = false
	}
}

func (s *Scenario) openBuildingPopup(b *world.Building, screenW, screenH int) {
	bldg := b
	switch bldg.Type {
	case world.BuildingShed:
		w := ui.NewWindow("Equipment Shed", 0, 0)
		// Cat count with side-effect spawning/despawning. +/- both clamp
		// (1..MaxCatsPerShed); buying a cat deducts CatCost and spawns
		// one at the shed door.
		w.AddIntStepperFn("Cats",
			func() string { return fmt.Sprintf("%d/%d", bldg.Cats, world.MaxCatsPerShed) },
			func() { // minus: sell a cat (refund half its cost)
				if bldg.Cats <= 1 {
					return // never empty the shed; first cat is included
				}
				cats := s.world.CatsOwnedBy(bldg.ID)
				if len(cats) == 0 {
					return
				}
				s.world.RemoveSnowcat(cats[len(cats)-1].ID)
				bldg.Cats--
				s.world.Cash += world.CatCost / 2
			},
			func() { // plus: buy a cat
				if bldg.Cats >= world.MaxCatsPerShed {
					return
				}
				if s.world.Cash < world.CatCost {
					s.setToast(fmt.Sprintf("Need $%d for another cat", world.CatCost))
					return
				}
				s.world.Cash -= world.CatCost
				s.world.SpawnSnowcat(bldg)
				bldg.Cats++
			})
		w.AddLabel("Route", func() string {
			return fmt.Sprintf("%d / %d cells", len(bldg.RouteCells), world.MaxRouteCells(bldg.Cats))
		})
		w.AddActionButton("Paint Route", func() {
			s.routeShedID = bldg.ID
			s.lastRouteCell = [2]int{-1, -1}
			s.activeTool = toolRoute
			s.syncToolButtons()
			if s.popup != nil {
				s.popup.Visible = false
			}
			s.setToast("Drag to paint route. Press Esc to finish.")
		})
		w.AddActionButton("Clear Route", func() {
			bldg.RouteCells = nil
		})
		w.Visible = true
		w.Center(screenW, screenH)
		s.popup = w
		return
	}
	w := ui.NewWindow("Lodge", 0, 0)
	w.AddLabel("Skiers inside", func() string {
		return fmt.Sprintf("%d", bldg.SkierCount)
	})
	w.AddLabel("Spawn rate", func() string {
		return fmt.Sprintf("%.2f/s", bldg.MeanSpawnRate)
	})
	w.AddLabel("Inbound", func() string {
		count := 0
		for _, a := range s.world.Agents {
			if a.TargetID == bldg.ID {
				count++
			}
		}
		return fmt.Sprintf("%d", count)
	})
	w.Visible = true
	w.Center(screenW, screenH)
	s.popup = w
}

func (s *Scenario) openLiftPopup(lift *world.Lift, screenW, screenH int) {
	l := lift
	w := ui.NewWindow("Ski Lift", 0, 0)
	w.AddLabel("Queue", func() string {
		return fmt.Sprintf("%d skiers", len(l.Queue))
	})
	w.AddLabel("On lift", func() string {
		return fmt.Sprintf("%d skiers", l.PassengerCount())
	})
	w.AddLabel("Chairs", func() string {
		return fmt.Sprintf("%d", len(l.Chairs))
	})
	w.AddStepper("Speed (m/s)", &l.Speed, 0.5, 0.5, 8.0)
	w.AddIntStepper("Ticket ($)", &l.TicketPrice, 5, 0, 200)
	w.Visible = true
	w.Center(screenW, screenH)
	s.popup = w
}

// setTool toggles the given tool on; if it is already active, deactivates it.
func (s *Scenario) setTool(t toolMode) {
	r := s.app.Renderer
	isActive := s.activeTool == t || (t == toolLiftBase && s.activeTool == toolLiftTop)
	r.ClearAllGhosts()
	r.ClearGhostCable()
	if isActive {
		s.activeTool = toolNone
	} else {
		s.activeTool = t
	}
	s.syncToolButtons()
}

// cancelTool deactivates whatever tool is currently active.
func (s *Scenario) cancelTool() {
	s.app.Renderer.ClearAllGhosts()
	s.app.Renderer.ClearGhostCable()
	if s.activeTool == toolRoute {
		s.routeShedID = 0
		s.lastRouteCell = [2]int{-1, -1}
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
// that should suppress world hit-testing. Extends barsContain to also
// consider the right-side overlay panel (which is only present at
// specific X coordinates, so the bar-style Y-only check isn't enough).
func (s *Scenario) uiCovers(x, y float32, screenW float32) bool {
	if s.barsContain(y) {
		return true
	}
	if s.overlayPanel != nil && s.overlayPanel.ContainsXY(x, y, screenW) {
		return true
	}
	return false
}

// syncToolButtons updates the active state of all tool buttons to match activeTool.
func (s *Scenario) syncToolButtons() {
	for mode, btn := range s.toolButtons {
		btn.SetActive(s.activeTool == mode ||
			(mode == toolLiftBase && s.activeTool == toolLiftTop))
	}
}

// updatePlacementGhost drives the translucent preview that follows the
// cursor while a placement tool is active. Resets every ghost up-front
// each frame, then opts the active tool's geometry back in — keeps the
// state of the ghost world in lockstep with the active tool without
// per-tool teardown logic. The ghost tints red when the player can't
// afford the placement.
//
// Reads the continuous hoverWorld so previews track the cursor at
// freeform precision, not snapped to a cell.
func (s *Scenario) updatePlacementGhost(r *render.Renderer) {
	t := s.world.Terrain

	r.ClearAllGhosts()
	r.ClearGhostCable()

	if !s.hoverValid {
		return
	}
	pos := mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]}
	tint := ghostTint(s.placementAffordable())

	switch s.activeTool {
	case toolBuilding:
		r.SetGhosts(render.MeshBuilding, []render.StaticInstance{
			buildingInstance(pos, t, tint),
		})

	case toolShed:
		r.SetGhosts(render.MeshShed, []render.StaticInstance{
			buildingInstance(pos, t, tint),
		})

	case toolLiftBase:
		// No top yet — render at default orientation by passing
		// otherEnd == pos. Rotation kicks in in toolLiftTop below.
		r.SetGhosts(render.MeshLiftStation, []render.StaticInstance{
			stationInstance(pos, pos, t, tint),
		})

	case toolLiftTop:
		base := s.liftBase
		top := pos
		r.SetGhosts(render.MeshLiftStation, []render.StaticInstance{
			stationInstance(base, top, t, tint), // base faces top
			stationInstance(top, base, t, tint), // top faces base
		})
		r.SetGhostCable(base, top, t)
	}
}

// placementCost returns the cost of placing whatever the active tool
// will create at the current hover position, plus a flag indicating
// whether the player can afford it. Returns (0, false, false) if no
// cost-bearing tool is active or the hover is off-terrain.
func (s *Scenario) placementCost() (cost int, affordable bool, valid bool) {
	if !s.hoverValid {
		return 0, false, false
	}
	pos := mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]}
	switch s.activeTool {
	case toolBuilding:
		cost = world.LodgeCost
	case toolShed:
		cost = world.ShedCost
	case toolLiftBase:
		cost = world.LiftStationCost
	case toolLiftTop:
		cost = world.LiftCost(s.liftBase, pos)
	default:
		return 0, false, false
	}
	return cost, s.world.Cash >= cost, true
}

func (s *Scenario) placementAffordable() bool {
	_, ok, valid := s.placementCost()
	return !valid || ok // no active placement → don't tint anything
}

// ghostTint returns the per-instance ColorTint for ghost previews:
// white when affordable, warm red when not.
func ghostTint(affordable bool) [3]float32 {
	if affordable {
		return [3]float32{1, 1, 1}
	}
	return [3]float32{1.0, 0.45, 0.45}
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
	agents := s.world.Agents
	if len(agents) == 0 {
		s.followAgentID = 0
		return
	}
	current := s.followAgentID
	candidates := agents
	if current != 0 && len(agents) > 1 {
		candidates = make([]*world.Agent, 0, len(agents)-1)
		for _, a := range agents {
			if a.ID != current {
				candidates = append(candidates, a)
			}
		}
	}
	s.followAgentID = candidates[rand.Intn(len(candidates))].ID
}

// pickAgent returns the front-most agent under the cursor, or nil if none.
// Treats each skier as a sphere of pickRadius around its world position.
func (s *Scenario) pickAgent(cam *render.Camera, mousePos mgl32.Vec2) *world.Agent {
	const pickRadius = float32(2.5)
	const pickRadius2 = pickRadius * pickRadius
	origin, dir := cam.ScreenToWorldRay(mousePos)
	var best *world.Agent
	bestT := float32(math.Inf(1))
	for _, a := range s.world.Agents {
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
func applyFirstPersonCamera(cam *render.Camera, a *world.Agent) {
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

// findFollowedAgent returns the agent being followed, or nil if it no longer exists.
func (s *Scenario) findFollowedAgent() *world.Agent {
	for _, a := range s.world.Agents {
		if a.ID == s.followAgentID {
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
	if s.followAgentID == 0 {
		r.ClearPerceptionCone()
		return
	}
	a := s.findFollowedAgent()
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

// followLabel draws a HUD banner showing which skier the camera is following.
type followLabel struct {
	world *world.World
	agent *world.Agent
}

func (f *followLabel) Draw(r *render.Renderer) {
	activity := world.Activity(f.world, f.agent)
	energyPct := int(f.agent.Energy*100 + 0.5)
	mode := f.agent.Sense.Mode
	if mode == "" {
		mode = "—"
	}
	rows := []string{
		fmt.Sprintf("Skier #%d  |  %s  |  %s", f.agent.ID, activity, mode),
		fmt.Sprintf("%.1f m/s    energy %d%%", f.agent.Speed, energyPct),
	}

	const padX = 10
	const padY = 4
	const lineGap = 2
	maxLen := 0
	for _, row := range rows {
		if len(row) > maxLen {
			maxLen = len(row)
		}
	}
	w := float32(maxLen*render.GlyphAdvance + 2*padX)
	boxH := float32(render.GlyphH*len(rows)+lineGap*(len(rows)-1)) + float32(2*padY)
	x := (float32(r.ScreenWidth()) - w) / 2
	y := float32(106) // sits just below the 96-px top HUD bar
	r.DrawColorRect(x, y, w, boxH, mgl32.Vec4{0, 0, 0, 0.6})
	if r.Font != nil {
		col := mgl32.Vec4{1, 0.95, 0.1, 1}
		for i, row := range rows {
			rowX := x + (w-float32(len(row)*render.GlyphAdvance))/2
			rowY := y + float32(padY) + float32(i*(render.GlyphH+lineGap))
			r.Font.DrawText(r, row, rowX, rowY, col)
		}
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
	const boxH = float32(render.GlyphH + 8)
	boxW := float32(len(text)*render.GlyphAdvance + 16)
	x := float32(r.ScreenWidth()) - boxW - 4
	y := float32(102) // just below the 96-px top HUD bar
	textY := y + (boxH-float32(render.GlyphH))/2
	r.DrawColorRect(x, y, boxW, boxH, mgl32.Vec4{0, 0, 0, 0.6})
	if r.Font != nil {
		r.Font.DrawText(r, text, x+8, textY, mgl32.Vec4{0.85, 0.85, 0.85, 1})
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
	const boxH = float32(render.GlyphH + 8)
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
	const boxH = float32(render.GlyphH + 8)
	const toolBarReserve = float32(60) // matches Scenario.toolBar.H
	boxW := float32(len(h.text)*render.GlyphAdvance + 16)
	y := float32(r.ScreenHeight()) - boxH - toolBarReserve - 6
	textY := y + (boxH-float32(render.GlyphH))/2
	r.DrawColorRect(4, y, boxW, boxH, mgl32.Vec4{0, 0, 0, 0.7})
	if r.Font != nil {
		r.Font.DrawText(r, h.text, 8+4, textY, mgl32.Vec4{1, 1, 0.5, 1})
	}
}

// toastLabel draws a transient status message near the bottom-centre of the
// screen. Used for screenshot/log toggle confirmations.
type toastLabel struct {
	text string
}

func (t *toastLabel) Draw(r *render.Renderer) {
	const boxH = float32(render.GlyphH + 8)
	const toolBarReserve = float32(60) // matches Scenario.toolBar.H
	boxW := float32(len(t.text)*render.GlyphAdvance + 20)
	x := (float32(r.ScreenWidth()) - boxW) / 2
	y := float32(r.ScreenHeight()) - boxH - toolBarReserve - 30
	textY := y + (boxH-float32(render.GlyphH))/2
	r.DrawColorRect(x, y, boxW, boxH, mgl32.Vec4{0, 0, 0, 0.75})
	if r.Font != nil {
		r.Font.DrawText(r, t.text, x+10, textY, mgl32.Vec4{1, 1, 1, 1})
	}
}

// toggleSkierLog starts or stops a CSV recorder for the followed skier.
func (s *Scenario) toggleSkierLog() {
	if s.csvRecorder != nil {
		s.stopSkierLog("Logging stopped")
		return
	}
	a := s.findFollowedAgent()
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
