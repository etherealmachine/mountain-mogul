package scene

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// Editor is the scenario editor scene (no simulation).
type Editor struct {
	app               *engine.App
	world             *world.World
	menuBar           *ui.MenuBar
	topBar            *ui.TopBar       // editor-mode bar: just overlay + settings buttons
	overlayPanel      *ui.OverlayPanel // right-edge terrain-overlay toggles
	escapeMenu        *EscapeMenu
	toolButtons       map[toolMode]*ui.Button
	activeTool        toolMode
	scenarioPath      string
	hoverCell         [2]int      // terrain cell currently under the mouse
	hoverWorld        mgl32.Vec3  // continuous terrain hit under the mouse — for placement and ghost preview
	hoverValid        bool        // false when the cursor isn't on the terrain (or sits on chrome)
	radiusSlider      *ui.VSlider // shown for any brush tool
	densitySlider     *ui.VSlider // shown only for the plant tool — caps brush target density
	autoMaxSlider      *ui.VSlider // shown only in auto tool: max snow depth (m)
	autoSnowlineSlider *ui.VSlider // shown only in auto tool: snowline elevation (%)
	autoTreelineSlider *ui.VSlider // shown only in auto tool: treeline elevation (%) — drives both snow & forest
	autoCoverageSlider *ui.VSlider // shown only in auto tool: forest coverage (%)
	autoWindSlider     *ui.VSlider // shown only in auto tool: wind direction (deg)
	autoSeed           int64       // seed for the noise overlays; stable across slider tweaks
	autoFields         *elevFields // cached flow / curvature / minE-maxE; invalidated on elevation edits
	liftBase          mgl32.Vec2  // toolLiftBase → toolLiftTop two-click state: first click stored here
	pendingScreenshot bool        // captured at end of Render so the PNG matches what's on screen
}

// NewEditor creates an Editor scene loading from the given path.
func NewEditor(path string) *Editor {
	return &Editor{scenarioPath: path}
}

func (e *Editor) Init(app *engine.App) error {
	e.app = app

	w, savedCam, err := save.LoadScenario(e.scenarioPath)
	if err != nil {
		fmt.Printf("Editor load error (%v), creating blank world\n", err)
		t := world.NewTerrain(256, 256)
		w = world.NewWorld(t)
	}
	e.world = w

	r := app.Renderer
	r.ResetSceneState()
	r.BuildTerrainMesh(w.Terrain)
	r.RebuildStaticBatch(w)
	for _, lift := range w.Lifts {
		r.AddLiftCable(lift, w.Terrain)
	}

	if savedCam != nil {
		applyCameraSnapshot(r.Camera, savedCam)
	} else {
		const cellSize = float32(5.0)
		r.Camera.Target = mgl32.Vec3{
			float32(w.Terrain.Width) * cellSize / 2,
			0,
			float32(w.Terrain.Height) * cellSize / 2,
		}
		r.Camera.OrthoScale = 700
		r.Camera.Recalculate()
	}

	// Editor tool bar — same shape as the scenario's: centred icon buttons,
	// anchored to the bottom of the screen each frame in Update().
	e.toolButtons = make(map[toolMode]*ui.Button)
	e.menuBar = ui.NewMenuBar(0, 60)
	e.menuBar.Centered = true
	// Placement tools — same set the scenario exposes, but free in the
	// editor (no cost gating) since the editor's purpose is laying out
	// starting state for a scenario.
	e.toolButtons[toolParking] = e.menuBar.AddIconButton(render.IconUsers, "Parking", func() { e.setTool(toolParking) })
	e.toolButtons[toolBuilding] = e.menuBar.AddIconButton(render.IconHouse, "Lodge", func() { e.setTool(toolBuilding) })
	e.toolButtons[toolShed] = e.menuBar.AddIconButton(render.IconGarage, "Shed", func() { e.setTool(toolShed) })
	e.toolButtons[toolLiftBase] = e.menuBar.AddIconButton(render.IconCableCar, "Lift", func() { e.setTool(toolLiftBase) })
	e.toolButtons[toolPlantTrees] = e.menuBar.AddIconButton(render.IconTreeEvergreen, "Plant", func() { e.setTool(toolPlantTrees) })
	e.toolButtons[toolAuto] = e.menuBar.AddIconButton(render.IconSnowflake, "Auto", func() { e.setTool(toolAuto) })
	e.toolButtons[toolGlade] = e.menuBar.AddIconButton(render.IconAxe, "Glade", func() { e.setTool(toolGlade) })
	e.toolButtons[toolEditorRaise] = e.menuBar.AddIconButton(render.IconArrowFatUp, "Raise", func() { e.setTool(toolEditorRaise) })
	e.toolButtons[toolEditorLower] = e.menuBar.AddIconButton(render.IconArrowFatDown, "Lower", func() { e.setTool(toolEditorLower) })
	e.menuBar.AddIconButton(render.IconGlobe, "Import", func() {
		app.PushScene(NewTerrainImport(
			e.world.Terrain.Width,
			func(elevs [][]float32) { e.applyImportedTerrain(elevs, e.app.Renderer) },
		))
	})
	e.menuBar.AddIconButton(render.IconFloppyDisk, "Save", func() {
		if err := save.SaveScenario(e.scenarioPath, e.world, editorCameraSnapshot(e)); err != nil {
			fmt.Println("Save error:", err)
		} else {
			fmt.Println("Saved to", e.scenarioPath)
		}
	})

	e.escapeMenu = NewEscapeMenu(app, func() {
		if err := save.SaveScenario(e.scenarioPath, e.world, editorCameraSnapshot(e)); err != nil {
			fmt.Println("Save error:", err)
		} else {
			fmt.Println("Saved to", e.scenarioPath)
		}
	}, nil)

	// Top bar — editor-mode only has overlay-panel toggle + settings (gear).
	// No stats, no date/weather, no time controls, so the centre and left
	// stay empty by leaving the callbacks unset.
	const topBarH = float32(96)
	e.topBar = ui.NewTopBar(topBarH)
	e.topBar.SetSettingsButton(func() { e.escapeMenu.Toggle() })

	e.overlayPanel = ui.NewOverlayPanel()
	e.overlayPanel.Top = topBarH
	e.overlayPanel.Bottom = float32(app.Renderer.ScreenHeight()) - e.menuBar.H
	e.topBar.SetOverlayToggle(func() {
		visible := e.overlayPanel.Toggle()
		e.topBar.SetOverlayActive(visible)
	})

	// Brush sliders — repositioned each Render relative to screen size.
	// Radius shows for any brush tool; density only for plant. Range max is
	// generous on radius so users can paint a whole forest in one stroke.
	e.radiusSlider = ui.NewVSlider(0, 0, 18, 200, 1, 30, float32(defaultBrushRadius), "Radius")
	e.densitySlider = ui.NewVSlider(0, 0, 18, 200, 0, 100, 100, "Density")

	// Auto-gen sliders — drive both forest and snow generation. Persistent
	// across tool toggles so the player's last settings stick around.
	// Treeline is shared (forest cap + snow wind-exposure threshold); the
	// other sliders each affect one of the two layers.
	e.autoMaxSlider = ui.NewVSlider(0, 0, 18, 200, 0, 8, 4, "Max")
	e.autoMaxSlider.ValueFormat = "%.1f m"
	e.autoSnowlineSlider = ui.NewVSlider(0, 0, 18, 200, 0, 100, 30, "Snowline")
	e.autoSnowlineSlider.ValueFormat = "%.0f%%"
	e.autoTreelineSlider = ui.NewVSlider(0, 0, 18, 200, 0, 100, 70, "Treeline")
	e.autoTreelineSlider.ValueFormat = "%.0f%%"
	e.autoCoverageSlider = ui.NewVSlider(0, 0, 18, 200, 0, 100, 55, "Cover")
	e.autoCoverageSlider.ValueFormat = "%.0f%%"
	e.autoWindSlider = ui.NewVSlider(0, 0, 18, 200, 0, 360, 270, "Wind")
	e.autoWindSlider.ValueFormat = "%.0fdeg"

	return nil
}

const (
	toolEditorRaise = toolMode(100)
	toolEditorLower = toolMode(101)
	toolAuto        = toolMode(102)
)

func (e *Editor) Update(dt float64) {
	inp := e.app.Input
	r := e.app.Renderer

	// Coalesced snow-state flush: any tool that mutates SnowDepth /
	// Grooming / Packed / Ice / MogulSize sets Terrain.SnowDirty and we
	// push the result to the GPU once per frame. Matches the scenario's
	// per-frame check so tools can be shared without per-call wiring.
	if e.world != nil && e.world.Terrain != nil && e.world.Terrain.SnowDirty {
		r.FlushSnowState(e.world.Terrain)
		e.world.Terrain.SnowDirty = false
	}

	if inp.Pressed[glfw.KeyEscape] {
		if e.activeTool != toolNone {
			e.activeTool = toolNone
			e.syncToolButtons()
		} else {
			e.escapeMenu.Toggle()
		}
	}
	if e.escapeMenu.Visible() {
		e.escapeMenu.HandleInput(inp)
		return
	}

	// C: toggle the contour overlay via the panel so the hotkey and the
	// panel UI stay in sync (otherwise the panel's mask overwrites the
	// hotkey's change the same frame).
	if inp.Pressed[glfw.KeyC] {
		e.overlayPanel.ToggleBit(render.OverlayContour)
		r.TerrainOverlayMode = e.overlayPanel.Mask()
	}

	// F12: queue a screenshot — captured at the end of Render so the PNG
	// matches exactly what's on screen, including the menu bar and brush.
	if inp.Pressed[glfw.KeyF12] {
		e.pendingScreenshot = true
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
		r.Camera.Target = r.Camera.ScreenCenterOnHeightmap(e.world.Terrain.InterpolatedSurfaceElevationAt)
		r.Camera.Yaw += rotDelta
		r.Camera.Recalculate()
	}

	// Anchor to the bottom edge against the live screen height before
	// hit-testing — otherwise the buttons would float at the top.
	e.menuBar.Y = float32(r.ScreenHeight()) - e.menuBar.H
	e.menuBar.HandleInput(inp, float32(r.ScreenWidth()), float32(r.ScreenHeight()))
	e.topBar.HandleInput(inp, float32(r.ScreenWidth()))

	// Keep overlay panel sized to the live window and mirror its mask into
	// the renderer so toggles take effect this frame.
	e.overlayPanel.Bottom = float32(r.ScreenHeight()) - e.menuBar.H
	e.overlayPanel.HandleInput(inp, float32(r.ScreenWidth()))
	r.TerrainOverlayMode = e.overlayPanel.Mask()

	// Camera pan — right-drag or arrow keys
	if inp.RightClick || inp.RightHeld {
		if inp.MouseDelta.Len() > 0 {
			dx, dz := r.Camera.PanDelta(inp.MouseDelta)
			r.Camera.Target[0] += dx
			r.Camera.Target[2] += dz
			r.Camera.Recalculate()
		}
	}

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
		dx, dz := r.Camera.PanDelta(keyDelta)
		r.Camera.Target[0] += dx
		r.Camera.Target[2] += dz
		r.Camera.Recalculate()
	}

	// Zoom
	if inp.ScrollDelta != 0 {
		r.Camera.OrthoScale -= inp.ScrollDelta * 10
		if r.Camera.OrthoScale < 10 {
			r.Camera.OrthoScale = 10
		}
		if r.Camera.OrthoScale > 1500 {
			r.Camera.OrthoScale = 1500
		}
		r.Camera.Recalculate()
	}

	// Brush sliders take input before brush placement so dragging a thumb
	// doesn't also paint trees underneath it.
	sliderActive := false
	if e.toolUsesRadiusSlider() {
		e.layoutBrushSliders(r)
		if e.radiusSlider.HandleInput(inp.MousePos[0], inp.MousePos[1], inp.LeftClick, inp.LeftHeld) {
			sliderActive = true
		}
		if e.toolUsesDensitySlider() {
			if e.densitySlider.HandleInput(inp.MousePos[0], inp.MousePos[1], inp.LeftClick, inp.LeftHeld) {
				sliderActive = true
			}
		}
	}

	// Auto-gen sliders — adjusting any of them re-runs both the snow and
	// forest generators with the new values so the player gets live feedback.
	if e.activeTool == toolAuto {
		e.layoutAutoSliders(r)
		sliders := e.autoSliders()
		prev := make([]float32, len(sliders))
		for i, s := range sliders {
			prev[i] = s.Value
			if s.HandleInput(inp.MousePos[0], inp.MousePos[1], inp.LeftClick, inp.LeftHeld) {
				sliderActive = true
			}
		}
		changed := false
		for i, s := range sliders {
			if s.Value != prev[i] {
				changed = true
				break
			}
		}
		if changed {
			e.regenerateAuto()
		}
	}

	// Track terrain hit under the mouse — continuous Vec3 for the
	// placement ghost preview, plus the integer cell for the brush
	// tools. Both gated by the same on-chrome check so hovers over the
	// top bar / menu bar / overlay panel don't paint a ghost into the world.
	overChrome := e.menuBar.ContainsY(inp.MousePos[1]) ||
		e.topBar.ContainsY(inp.MousePos[1]) ||
		e.overlayPanel.ContainsXY(inp.MousePos[0], inp.MousePos[1], float32(r.ScreenWidth()))
	if !overChrome {
		if pos, ok := screenToWorld(r.Camera, e.world.Terrain, inp.MousePos); ok {
			e.hoverWorld = pos
			e.hoverValid = true
			const cellSize = float32(5.0)
			e.hoverCell = [2]int{int(pos[0] / cellSize), int(pos[2] / cellSize)}
		} else {
			e.hoverValid = false
			e.hoverCell = [2]int{-1, -1}
		}
	} else {
		e.hoverValid = false
		e.hoverCell = [2]int{-1, -1}
	}

	// Ghost preview for placement tools — editor placements are free so
	// the tint is always the affordable colour.
	updatePlacementGhost(r, e.world.Terrain, placementGhostState{
		activeTool: e.activeTool,
		hoverPos:   mgl32.Vec2{e.hoverWorld[0], e.hoverWorld[2]},
		hoverValid: e.hoverValid,
		liftBase:   e.liftBase,
		tint:       ghostTint(true),
	})

	// Tool application — placement tools fire on click only; brush tools
	// follow the cursor while held. Suppressed when a slider is grabbing
	// input or the cursor is over slider/menu chrome.
	if !sliderActive && !overChrome {
		overSlider := e.toolUsesRadiusSlider() && e.radiusSlider.Contains(inp.MousePos[0], inp.MousePos[1])
		overSlider = overSlider || (e.toolUsesDensitySlider() && e.densitySlider.Contains(inp.MousePos[0], inp.MousePos[1]))
		if e.activeTool == toolAuto {
			for _, s := range e.autoSliders() {
				if s.Contains(inp.MousePos[0], inp.MousePos[1]) {
					overSlider = true
					break
				}
			}
		}
		if !overSlider {
			if e.isPlacementTool() {
				if inp.LeftClick && e.hoverValid {
					e.applyPlacement(r)
				}
			} else if inp.LeftClick || inp.LeftHeld {
				gx, gz := e.hoverCell[0], e.hoverCell[1]
				if e.world.Terrain.InBounds(gx, gz) {
					e.applyEditorTool(gx, gz, r, float32(dt))
				}
			}
		}
	}
}

// isPlacementTool reports whether the active tool is a one-shot building
// or lift placement (click to commit) rather than a held brush.
func (e *Editor) isPlacementTool() bool {
	switch e.activeTool {
	case toolBuilding, toolShed, toolParking, toolLiftBase, toolLiftTop:
		return true
	}
	return false
}

// applyPlacement commits a one-shot building or lift placement at the
// current continuous hover position. No cost gating — editor placements
// are free. Mirrors the scenario's placement flow (apron, terrain flush,
// batch rebuild) so the resulting world is identical to one built by
// playing through the scenario.
func (e *Editor) applyPlacement(r *render.Renderer) {
	w := e.world
	wx := e.hoverWorld[0]
	wz := e.hoverWorld[2]
	switch e.activeTool {
	case toolBuilding:
		b := w.PlaceBuildingType(world.BuildingLodge, wx, wz)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
		e.autoFields = nil
	case toolShed:
		b := w.PlaceBuildingType(world.BuildingShed, wx, wz)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
		e.autoFields = nil
	case toolParking:
		b := w.PlaceBuildingType(world.BuildingParking, wx, wz)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
		e.autoFields = nil
	case toolLiftBase:
		e.liftBase = mgl32.Vec2{wx, wz}
		e.activeTool = toolLiftTop
		e.syncToolButtons()
	case toolLiftTop:
		lift := w.PlaceLift(e.liftBase[0], e.liftBase[1], wx, wz)
		applyLiftPlacementEffects(w.Terrain, lift)
		r.FlushTerrainVerts(w.Terrain)
		r.AddLiftCable(lift, w.Terrain)
		r.RebuildStaticBatch(w)
		e.activeTool = toolNone
		e.syncToolButtons()
		e.autoFields = nil
	}
}

// setTool toggles the given editor tool; re-clicking an active tool deactivates it.
// The Lift button is treated as "the lift tool" across both placement steps —
// clicking it again during the second click (toolLiftTop) cancels the flow.
// Activating auto re-rolls the noise seed and runs an initial generation
// so the player sees the result of the current slider values immediately.
func (e *Editor) setTool(t toolMode) {
	prev := e.activeTool
	isActive := e.activeTool == t || (t == toolLiftBase && e.activeTool == toolLiftTop)
	if isActive {
		e.activeTool = toolNone
	} else {
		e.activeTool = t
	}
	e.syncToolButtons()
	if e.activeTool == toolAuto && prev != toolAuto {
		e.autoSeed = time.Now().UnixNano()
		e.regenerateAuto()
	}
}

// autoSliders returns the slider set for the auto-gen tool in display order.
// Centralised so input handling, hit-testing, layout, and draw all walk the
// same list — adding a slider only requires touching this method.
func (e *Editor) autoSliders() []*ui.VSlider {
	return []*ui.VSlider{
		e.autoMaxSlider,
		e.autoSnowlineSlider,
		e.autoTreelineSlider,
		e.autoCoverageSlider,
		e.autoWindSlider,
	}
}

// regenerateAuto runs the snow and forest generators with the current slider
// values and pushes the results to the GPU. Snow goes via Terrain.SnowDirty
// (flushed at the top of Update), forest goes via RebuildStaticBatch.
//
// The elevation-derived passes (flow accumulation, curvature, min/max
// elevation) are cached on autoFields so live slider drag doesn't pay the
// O(N log N) sort every frame. The cache is invalidated whenever ground
// elevation changes (raise/lower brushes, terrain import).
func (e *Editor) regenerateAuto() {
	if e.world == nil || e.world.Terrain == nil {
		return
	}
	if e.autoFields == nil {
		e.autoFields = computeElevFields(e.world.Terrain)
	}
	treelineFrac := e.autoTreelineSlider.Value / 100
	e.autoFields.generateSnowCover(
		e.world.Terrain,
		e.autoMaxSlider.Value,
		e.autoSnowlineSlider.Value/100,
		treelineFrac,
		e.autoWindSlider.Value,
		e.autoSeed,
	)
	e.autoFields.generateTreeCover(
		e.world.Terrain,
		24,
		e.autoCoverageSlider.Value/100,
		treelineFrac,
		e.autoSeed,
	)
	if e.app != nil && e.app.Renderer != nil {
		e.app.Renderer.RebuildStaticBatch(e.world)
	}
}

// syncToolButtons updates the active state of all tool buttons to match
// activeTool. The Lift button stays highlighted during the two-click flow
// (toolLiftBase → toolLiftTop) so the player can see the placement is mid-flight.
func (e *Editor) syncToolButtons() {
	for mode, btn := range e.toolButtons {
		active := e.activeTool == mode || (mode == toolLiftBase && e.activeTool == toolLiftTop)
		btn.SetActive(active)
	}
}

const defaultBrushRadius = 2 // cells

// brushRadius returns the current radius for tools that use the slider, or
// the default for tools that don't. Centralising it keeps applyEditorTool
// and the hover preview in lockstep.
func (e *Editor) brushRadius() int {
	if e.toolUsesRadiusSlider() {
		r := int(e.radiusSlider.Value + 0.5)
		if r < 1 {
			r = 1
		}
		return r
	}
	return defaultBrushRadius
}

// toolUsesRadiusSlider reports whether the radius slider is relevant for
// the active tool — currently the two density brushes.
func (e *Editor) toolUsesRadiusSlider() bool {
	return e.activeTool == toolPlantTrees || e.activeTool == toolGlade
}

// toolUsesDensitySlider reports whether the density slider is relevant for
// the active tool. Glade always drives density toward zero, so it doesn't
// need a target.
func (e *Editor) toolUsesDensitySlider() bool {
	return e.activeTool == toolPlantTrees
}

func (e *Editor) applyEditorTool(gx, gz int, r *render.Renderer, dt float32) {
	w := e.world
	switch e.activeTool {
	case toolPlantTrees:
		target := e.densitySlider.Value / 100
		applyDensityBrushUpTo(w.Terrain, gx, gz, e.brushRadius(), 0.3, target)
		r.RebuildStaticBatch(w)
	case toolGlade:
		applyDensityBrush(w.Terrain, gx, gz, e.brushRadius(), -0.4)
		r.RebuildStaticBatch(w)
	case toolEditorRaise:
		w.Terrain.Cells[gx][gz].GroundElevation += 5.0 * dt
		r.FlushTerrainVerts(w.Terrain)
		e.autoFields = nil
	case toolEditorLower:
		w.Terrain.Cells[gx][gz].GroundElevation -= 5.0 * dt
		if w.Terrain.Cells[gx][gz].GroundElevation < 0 {
			w.Terrain.Cells[gx][gz].GroundElevation = 0
		}
		r.FlushTerrainVerts(w.Terrain)
		e.autoFields = nil
	}
}

// applyImportedTerrain replaces the entire scene with a fresh world built
// from the imported elevations. Buildings, lifts, agents, placed objects,
// and per-cell tree density are wiped — re-using terrain on top of an old
// layout would leave lifts dangling in mid-air and trees floating above
// new mountains. Snow depth starts at zero so the imported terrain reads
// as bare ground; the Auto-snow generator (and brushes, eventually) is the
// authoritative way to lay snow on top.
func (e *Editor) applyImportedTerrain(elevs [][]float32, r *render.Renderer) {
	rows := len(elevs)
	cols := 0
	if rows > 0 {
		cols = len(elevs[0])
	}
	if rows == 0 || cols == 0 {
		return
	}
	t := world.NewTerrain(cols, rows)
	for row := 0; row < t.Height; row++ {
		for col := 0; col < t.Width; col++ {
			if row < len(elevs) && col < len(elevs[row]) {
				t.Cells[col][row].GroundElevation = elevs[row][col]
			}
			t.Cells[col][row].SnowDepth = 0
		}
	}
	e.world = world.NewWorld(t)
	e.activeTool = toolNone
	e.autoFields = nil
	e.syncToolButtons()

	r.ResetSceneState()
	r.BuildTerrainMesh(t)
	r.RebuildStaticBatch(e.world)
}

func (e *Editor) Render(r *render.Renderer) {
	const cellSize = float32(5.0)
	t := e.world.Terrain
	if e.toolUsesRadiusSlider() && t.InBounds(e.hoverCell[0], e.hoverCell[1]) {
		gx, gz := e.hoverCell[0], e.hoverCell[1]
		center := mgl32.Vec2{float32(gx)*cellSize + cellSize/2, float32(gz)*cellSize + cellSize/2}
		r.SetBrush(center, (float32(e.brushRadius())+0.5)*cellSize)
	} else {
		r.ClearBrush()
	}

	r.DrawWorld(e.world, 0)
	r.ClearBrush()

	// Re-anchor before draw so menuBar.Y matches the live screen height.
	e.menuBar.Y = float32(r.ScreenHeight()) - e.menuBar.H
	e.overlayPanel.Bottom = float32(r.ScreenHeight()) - e.menuBar.H
	edDrawables := []render.UIDrawable{e.topBar, e.menuBar, e.overlayPanel}
	if e.toolUsesRadiusSlider() {
		e.layoutBrushSliders(r)
		edDrawables = append(edDrawables, e.radiusSlider)
		if e.toolUsesDensitySlider() {
			edDrawables = append(edDrawables, e.densitySlider)
		}
	}
	if e.activeTool == toolAuto {
		e.layoutAutoSliders(r)
		for _, s := range e.autoSliders() {
			edDrawables = append(edDrawables, s)
		}
	}
	if e.escapeMenu.Visible() {
		edDrawables = append(edDrawables, e.escapeMenu)
	}
	r.DrawUI(edDrawables)

	if e.pendingScreenshot {
		e.pendingScreenshot = false
		path := filepath.Join("debug", "screens", time.Now().Format("20060102-150405")+".png")
		if err := r.SaveScreenshot(path); err != nil {
			fmt.Println("Screenshot failed:", err)
		} else {
			fmt.Println("Screenshot saved →", path)
		}
	}
}

// layoutBrushSliders positions the brush sliders on the left edge, vertically
// centred below the menu bar. Density sits to the right of radius so both
// labels fit. Recomputed each frame so it tracks window resizes.
func (e *Editor) layoutBrushSliders(r *render.Renderer) {
	const trackW = float32(18)
	const trackH = float32(200)
	sh := float32(r.ScreenHeight())
	y := (sh-trackH)/2 + 20
	e.radiusSlider.X = 28
	e.radiusSlider.Y = y
	e.radiusSlider.W = trackW
	e.radiusSlider.H = trackH
	e.densitySlider.X = 80
	e.densitySlider.Y = y
	e.densitySlider.W = trackW
	e.densitySlider.H = trackH
}

// layoutAutoSliders positions the auto-gen sliders along the left edge,
// vertically centred. Wider column spacing than the brush sliders so the
// longer suffixes ("Snowline", "180deg") have room to read.
func (e *Editor) layoutAutoSliders(r *render.Renderer) {
	const trackW = float32(18)
	const trackH = float32(200)
	const col = float32(60)
	sh := float32(r.ScreenHeight())
	y := (sh-trackH)/2 + 20
	for i, s := range e.autoSliders() {
		s.X = 28 + float32(i)*col
		s.Y = y
		s.W = trackW
		s.H = trackH
	}
}

func (e *Editor) Destroy() {
	if e.app != nil && e.app.Renderer != nil {
		e.app.Renderer.ResetSceneState()
	}
}

// editorCameraSnapshot pulls the live camera state for save-time
// persistence. Mirrors Scenario.cameraSnapshot.
func editorCameraSnapshot(e *Editor) *save.CameraData {
	if e.app == nil || e.app.Renderer == nil || e.app.Renderer.Camera == nil {
		return nil
	}
	c := e.app.Renderer.Camera
	return &save.CameraData{
		TargetX:    c.Target[0],
		TargetY:    c.Target[1],
		TargetZ:    c.Target[2],
		Yaw:        c.Yaw,
		Pitch:      c.Pitch,
		OrthoScale: c.OrthoScale,
	}
}

func clampF(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
