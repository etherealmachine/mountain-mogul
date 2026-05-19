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
	liftDoubleBtn     *ui.Button     // toolbar button for the double-chair lift variant
	liftQuadBtn       *ui.Button     // toolbar button for the fixed-quad lift variant
	liftType          world.LiftType // chair variant the toolLiftBase/Top flow will place
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
	roadStart         mgl32.Vec2  // toolRoadStart → toolRoadEnd two-click state: first click stored here (post-snap)
	roadEdit          roadEditSelection // toolNone click-to-edit road handle / dragged node
	structureEdit     structureEditSelection // toolNone click-to-edit building / lift handle
	addStormBtn       *ui.Button  // shown in toolAuto: push a fresh-snow layer onto the stack
	clearLayersBtn    *ui.Button  // shown in toolAuto: remove all snow layers
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
	// Saved cells already carry every apron / road / corridor stamp that
	// was in effect at save time — no rebuild needed on load. New
	// structures get their footprint stamped at placement time.
	r.BuildTerrainMesh(w.Terrain)
	r.BuildSnowSurfaceTex(w.Terrain)
	r.RebuildStaticBatch(w)
	r.RebuildRoads(w)
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
	e.liftDoubleBtn = e.menuBar.AddIconButton(render.IconCableCar, "Double", func() { e.activateLiftTool(world.LiftDouble) })
	e.liftQuadBtn = e.menuBar.AddIconButton(render.IconCableCar, "Quad", func() { e.activateLiftTool(world.LiftFixedQuad) })
	e.toolButtons[toolRoadStart] = e.menuBar.AddIconButton(render.IconRoad, "Road", func() { e.setTool(toolRoadStart) })
	e.toolButtons[toolEdgeConnect] = e.menuBar.AddIconButton(render.IconFlag, "Edge", func() { e.setTool(toolEdgeConnect) })
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
	e.autoMaxSlider = ui.NewVSlider(0, 0, 18, 200, 0, 4, 2, "Max")
	e.autoMaxSlider.ValueFormat = "%.1f m"
	e.autoSnowlineSlider = ui.NewVSlider(0, 0, 18, 200, 0, 100, 30, "Snow")
	e.autoSnowlineSlider.ValueFormat = "%.0f%%"
	e.autoTreelineSlider = ui.NewVSlider(0, 0, 18, 200, 0, 100, 70, "Tree")
	e.autoTreelineSlider.ValueFormat = "%.0f%%"
	e.autoCoverageSlider = ui.NewVSlider(0, 0, 18, 200, 0, 100, 55, "Cover")
	e.autoCoverageSlider.ValueFormat = "%.0f%%"
	e.autoWindSlider = ui.NewVSlider(0, 0, 18, 200, 0, 360, 270, "Wind")
	e.autoWindSlider.ValueFormat = "%.0fdeg"

	e.addStormBtn = ui.NewButton(0, 0, 120, 28, "+ Add Storm", func() {
		e.pushSnowLayer(world.LayerFreshSnow, 0.2)
	})
	e.clearLayersBtn = ui.NewButton(0, 0, 100, 28, "Clear Snow", func() {
		e.clearAllLayers()
	})

	return nil
}

const (
	toolEditorRaise = toolMode(100)
	toolEditorLower = toolMode(101)
	toolAuto        = toolMode(102)
)

// uiDrawFunc adapts a bare function to the render.UIDrawable interface.
type uiDrawFunc func(*render.Renderer)

func (f uiDrawFunc) Draw(r *render.Renderer) { f(r) }

func (e *Editor) Update(dt float64) {
	inp := e.app.Input
	r := e.app.Renderer

	// Coalesced snow-state flush: any tool that mutates SnowAccumulation /
	// Grooming / Packed / Ice / MogulSize sets Terrain.SnowDirty and we
	// push the result to the GPU once per frame. Matches the scenario's
	// per-frame check so tools can be shared without per-call wiring.
	if e.world != nil && e.world.Terrain != nil && e.world.Terrain.SnowDirty {
		r.FlushSnowState(e.world.Terrain)
		e.world.Terrain.SnowDirty = false
	}

	if inp.Pressed[glfw.KeyEscape] {
		switch {
		case e.roadEdit.active() || e.structureEdit.active():
			e.roadEdit.clear()
			e.structureEdit.clear()
		case e.activeTool != toolNone:
			e.activeTool = toolNone
			e.syncToolButtons()
		default:
			e.escapeMenu.Toggle()
		}
	}
	if (inp.Pressed[glfw.KeyDelete] || inp.Pressed[glfw.KeyBackspace]) {
		switch {
		case e.roadEdit.active():
			deleteSelectedRoad(r, e.world, &e.roadEdit)
			e.autoFields = nil
		case e.structureEdit.active():
			deleteSelectedStructure(r, e.world, &e.structureEdit)
			e.autoFields = nil
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
		if e.radiusSlider.HandleInput(inp) {
			sliderActive = true
		}
		if e.toolUsesDensitySlider() {
			if e.densitySlider.HandleInput(inp) {
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
			if s.HandleInput(inp) {
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
		// Layer management buttons — positioned below the sliders.
		e.layoutLayerButtons(r)
		mx, my := inp.MousePos[0], inp.MousePos[1]
		e.addStormBtn.SetHovered(e.addStormBtn.Contains(mx, my))
		e.clearLayersBtn.SetHovered(e.clearLayersBtn.Contains(mx, my))
		if inp.LeftClick {
			if e.addStormBtn.Contains(mx, my) {
				e.addStormBtn.Click()
			} else if e.clearLayersBtn.Contains(mx, my) {
				e.clearLayersBtn.Click()
			}
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
		roadStart:  e.roadStart,
		tint:       ghostTint(true, e.placementLegal()),
	})
	// Editor mirrors the scenario's node-highlight behaviour while a
	// road tool is active — same snap rules, same visual cue. While
	// editing an existing road (toolNone selection), draw the full node
	// handle layer instead so the player can see what they're moving.
	// Structure edit (building / lift handles) gets a single marker on
	// the selected anchor.
	if e.activeTool == toolRoadStart || e.activeTool == toolRoadEnd {
		emitRoadNodeMarkers(r, e.world, mgl32.Vec2{e.hoverWorld[0], e.hoverWorld[2]}, e.hoverValid)
	} else if e.activeTool == toolNone && e.roadEdit.active() {
		emitRoadEditMarkers(r, e.world, &e.roadEdit)
	} else if e.activeTool == toolNone && e.structureEdit.active() {
		emitStructureEditMarkers(r, e.world, &e.structureEdit)
	}

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
			} else if e.activeTool == toolNone {
				e.handleToolNoneMouse(r, inp.LeftClick, inp.LeftHeld)
			} else if inp.LeftClick || inp.LeftHeld {
				gx, gz := e.hoverCell[0], e.hoverCell[1]
				if e.world.Terrain.InBounds(gx, gz) {
					e.applyEditorTool(gx, gz, r, float32(dt))
				}
			}
		}
	}
}

// handleToolNoneMouse processes click / drag / release for both the
// road-edit and structure-edit affordances. Hit priority on click is
// road first (small, precise targets), then building / lift. Drag and
// release route to whichever selection is active.
func (e *Editor) handleToolNoneMouse(r *render.Renderer, leftClick, leftHeld bool) {
	// Drag-release handling runs even when the cursor is off-terrain so
	// a player who drags onto the menu bar can still get a clean commit.
	if !leftHeld {
		if e.roadEdit.dragging {
			commitRoadDrag(r, e.world)
			e.roadEdit.dragging = false
			e.autoFields = nil
		}
		if e.structureEdit.dragging {
			commitStructureDrag(r, e.world)
			e.structureEdit.dragging = false
			e.autoFields = nil
		}
	}
	if !e.hoverValid {
		return
	}
	pos := mgl32.Vec2{e.hoverWorld[0], e.hoverWorld[2]}
	switch {
	case leftClick:
		if tryStartRoadEdit(e.world, pos, &e.roadEdit) {
			e.structureEdit.clear()
		} else if tryStartStructureEdit(e.world, pos, &e.structureEdit) {
			e.roadEdit.clear()
		} else {
			e.roadEdit.clear()
			e.structureEdit.clear()
		}
	case leftHeld && e.roadEdit.dragging:
		dragRoadNode(r, e.world, &e.roadEdit, pos)
	case leftHeld && e.structureEdit.dragging:
		dragStructure(r, e.world, &e.structureEdit, pos)
	}
}

// isPlacementTool reports whether the active tool is a one-shot building
// or lift placement (click to commit) rather than a held brush.
func (e *Editor) isPlacementTool() bool {
	switch e.activeTool {
	case toolBuilding, toolShed, toolParking, toolLiftBase, toolLiftTop, toolRoadStart, toolRoadEnd, toolEdgeConnect:
		return true
	}
	return false
}

// placementLegal reports whether the current hover position is a legal
// spot for the active building tool. Lift placement is unconstrained for
// now — only building footprint overlap is checked.
func (e *Editor) placementLegal() bool {
	if !e.hoverValid {
		return true
	}
	wx, wz := e.hoverWorld[0], e.hoverWorld[2]
	switch e.activeTool {
	case toolBuilding:
		return !e.world.BuildingOverlap(world.BuildingLodge, wx, wz)
	case toolShed:
		return !e.world.BuildingOverlap(world.BuildingShed, wx, wz)
	case toolParking:
		return !e.world.BuildingOverlap(world.BuildingParking, wx, wz)
	case toolEdgeConnect:
		_, _, ok := projectToMapEdge(e.world.Terrain, mgl32.Vec2{wx, wz}, edgeConnectTolerance)
		return ok
	}
	return true
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
		if w.BuildingOverlap(world.BuildingLodge, wx, wz) {
			return
		}
		b := w.PlaceBuildingType(world.BuildingLodge, wx, wz)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
		e.autoFields = nil
	case toolShed:
		if w.BuildingOverlap(world.BuildingShed, wx, wz) {
			return
		}
		b := w.PlaceBuildingType(world.BuildingShed, wx, wz)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
		e.autoFields = nil
	case toolParking:
		if w.BuildingOverlap(world.BuildingParking, wx, wz) {
			return
		}
		b := w.PlaceBuildingType(world.BuildingParking, wx, wz)
		w.EnsureParkingDriveway(b)
		applyBuildingPlacementEffects(w.Terrain, b)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
		r.RebuildRoads(w)
		e.autoFields = nil
	case toolLiftBase:
		e.liftBase = mgl32.Vec2{wx, wz}
		e.activeTool = toolLiftTop
		e.syncToolButtons()
	case toolLiftTop:
		lift := w.PlaceLift(e.liftType, e.liftBase[0], e.liftBase[1], wx, wz)
		applyLiftPlacementEffects(w.Terrain, lift)
		r.FlushTerrainVerts(w.Terrain)
		r.AddLiftCable(lift, w.Terrain)
		r.RebuildStaticBatch(w)
		e.activeTool = toolNone
		e.syncToolButtons()
		e.autoFields = nil
	case toolRoadStart:
		e.roadStart = resolveRoadEndpoint(w, mgl32.Vec2{wx, wz}).pos
		e.activeTool = toolRoadEnd
		e.syncToolButtons()
	case toolRoadEnd:
		start := resolveRoadEndpoint(w, e.roadStart)
		end := resolveRoadEndpoint(w, mgl32.Vec2{wx, wz})
		if placeRoadSegment(w, start, end) != nil {
			applyRoadCellState(w)
			r.FlushTerrainVerts(w.Terrain)
			r.RebuildStaticBatch(w)
			// Continue chaining from the just-placed endpoint. Esc or
			// re-clicking the Road button drops out of the tool.
			e.roadStart = end.pos
		}
		r.RebuildRoads(w)
		e.autoFields = nil
	case toolEdgeConnect:
		snapped, inward, ok := projectToMapEdge(w.Terrain, mgl32.Vec2{wx, wz}, edgeConnectTolerance)
		if !ok {
			return
		}
		// Quantise to the cell grid (same rule as road-tool clicks) so
		// the perimeter post + inland end both land on vertex
		// coordinates and the clearance pass stays symmetric.
		snapped = snapToCellGrid(snapped)
		inland := mgl32.Vec2{
			snapped[0] + inward[0]*edgeConnectStubLength,
			snapped[1] + inward[1]*edgeConnectStubLength,
		}
		// Two nodes + an edge between them. The post-bearing edge node
		// is the spawn/despawn anchor; the inland end is a plain
		// freestanding node the player will hook their network onto
		// via the road tool's snap-to-edge/node logic.
		edgeNode := w.AddRoadNode(snapped, world.RoadNodeEdgeConnection)
		inlandNode := w.AddRoadNode(inland, world.RoadNodeFreestanding)
		w.AddRoadEdge(edgeNode.ID, inlandNode.ID)
		applyRoadCellState(w)
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
		r.RebuildRoads(w)
		// Stay in the tool — designers usually place several edge
		// connections in a row.
	}
}

// setTool toggles the given editor tool; re-clicking an active tool deactivates it.
// The Lift button is treated as "the lift tool" across both placement steps —
// clicking it again during the second click (toolLiftTop) cancels the flow.
// Activating auto re-rolls the noise seed and runs an initial generation
// so the player sees the result of the current slider values immediately.
func (e *Editor) setTool(t toolMode) {
	prev := e.activeTool
	isActive := e.activeTool == t ||
		(t == toolLiftBase && e.activeTool == toolLiftTop) ||
		(t == toolRoadStart && e.activeTool == toolRoadEnd)
	if isActive {
		e.activeTool = toolNone
	} else {
		e.activeTool = t
	}
	// Activating any tool ends a toolNone-level edit session — the
	// selection markers should disappear and a half-finished drag must
	// not continue across a tool switch.
	if e.roadEdit.active() || e.roadEdit.dragging {
		e.roadEdit.clear()
	}
	if e.structureEdit.active() || e.structureEdit.dragging {
		e.structureEdit.clear()
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

// pushSnowLayer adds a new snow layer to every cell using the current
// auto-gen slider settings. Unlike regenerateAuto, this does not
// replace existing layers — it stacks on top of them.
func (e *Editor) pushSnowLayer(kind world.LayerKind, packed float32) {
	if e.world == nil || e.world.Terrain == nil {
		return
	}
	if e.autoFields == nil {
		e.autoFields = computeElevFields(e.world.Terrain)
	}
	addSnowLayerCached(
		e.autoFields,
		e.world.Terrain,
		kind, packed,
		e.autoMaxSlider.Value,
		e.autoSnowlineSlider.Value/100,
		e.autoTreelineSlider.Value/100,
		e.autoWindSlider.Value,
		e.autoSeed,
	)
	if e.app != nil && e.app.Renderer != nil {
		e.app.Renderer.FlushTerrainVerts(e.world.Terrain)
	}
}

// clearAllLayers removes all snow layers from every terrain cell.
func (e *Editor) clearAllLayers() {
	if e.world == nil || e.world.Terrain == nil {
		return
	}
	t := e.world.Terrain
	for x := range t.Cells {
		for z := range t.Cells[x] {
			t.Cells[x][z].Layers = nil
		}
	}
	t.SnowDirty = true
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
		e.app.Renderer.FlushTerrainVerts(e.world.Terrain)
		e.app.Renderer.RebuildStaticBatch(e.world)
	}
}

// syncToolButtons updates the active state of all tool buttons to match
// activeTool. Lift buttons live outside toolButtons because two of them
// share the toolLiftBase/Top tool modes — only the one whose chair type
// matches the current liftType selection should highlight.
func (e *Editor) syncToolButtons() {
	for mode, btn := range e.toolButtons {
		active := e.activeTool == mode ||
			(mode == toolRoadStart && e.activeTool == toolRoadEnd)
		btn.SetActive(active)
	}
	liftActive := e.activeTool == toolLiftBase || e.activeTool == toolLiftTop
	if e.liftDoubleBtn != nil {
		e.liftDoubleBtn.SetActive(liftActive && e.liftType == world.LiftDouble)
	}
	if e.liftQuadBtn != nil {
		e.liftQuadBtn.SetActive(liftActive && e.liftType == world.LiftFixedQuad)
	}
}

// activateLiftTool starts (or switches) the two-click lift placement
// flow for the given chair variant. Clicking the same variant again
// toggles the tool off; clicking the other variant mid-flow swaps the
// chair type without cancelling the placement.
func (e *Editor) activateLiftTool(typ world.LiftType) {
	inLiftFlow := e.activeTool == toolLiftBase || e.activeTool == toolLiftTop
	if inLiftFlow && e.liftType == typ {
		e.activeTool = toolNone
		e.syncToolButtons()
		return
	}
	e.liftType = typ
	if inLiftFlow {
		e.syncToolButtons()
		return
	}
	e.setTool(toolLiftBase)
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
		w.Terrain.RestampTreeWells()
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
	case toolGlade:
		applyDensityBrush(w.Terrain, gx, gz, e.brushRadius(), -0.4)
		w.Terrain.RestampTreeWells()
		r.FlushTerrainVerts(w.Terrain)
		r.RebuildStaticBatch(w)
	case toolEditorRaise:
		w.Terrain.Cells[gx][gz].GroundElevation += 5.0 * dt
		r.FlushTerrainVerts(w.Terrain)
		e.autoFields = nil
	case toolEditorLower:
		newE := w.Terrain.Cells[gx][gz].GroundElevation - 5.0*dt
		if newE < 0 {
			newE = 0
		}
		w.Terrain.Cells[gx][gz].GroundElevation = newE
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

	// Find the minimum elevation so we can normalise: terrain floor → Y=0.
	minElev := elevs[0][0]
	for _, row := range elevs {
		for _, v := range row {
			if v < minElev {
				minElev = v
			}
		}
	}

	t := world.NewTerrain(cols, rows)
	for row := 0; row < t.Height; row++ {
		for col := 0; col < t.Width; col++ {
			if row < len(elevs) && col < len(elevs[row]) {
				t.Cells[col][row].GroundElevation = elevs[row][col] - minElev
			}
			t.Cells[col][row].Layers = nil
		}
	}
	e.world = world.NewWorld(t)
	e.activeTool = toolNone
	e.autoFields = nil
	e.syncToolButtons()

	r.ResetSceneState()
	r.BuildTerrainMesh(t)
	r.BuildSnowSurfaceTex(t)
	r.RebuildStaticBatch(e.world)

	// Centre the camera on the imported terrain.
	const cellSize = float32(5.0)
	cx := float32(t.Width) * cellSize / 2
	cz := float32(t.Height) * cellSize / 2
	cy := t.SurfaceElevationAt(t.Width/2, t.Height/2)
	r.Camera.Target = mgl32.Vec3{cx, cy, cz}
	r.Camera.OrthoScale = float32(t.Width) * cellSize / 2
	r.Camera.Recalculate()
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
		e.layoutLayerButtons(r)
		sliders := e.autoSliders()
		first := sliders[0]
		last := sliders[len(sliders)-1]
		const panelPad = float32(10)
		panelX := first.X - panelPad
		panelY := first.Y - float32(render.GlyphH) - 4 - float32(render.GlyphH) - 8
		panelW := (last.X + last.W + panelPad) - panelX
		btnBottom := e.addStormBtn.Y + e.addStormBtn.H
		panelH := (btnBottom + 8) - panelY
		panel := uiDrawFunc(func(r *render.Renderer) {
			r.DrawColorRect(panelX, panelY, panelW, panelH, mgl32.Vec4{0.08, 0.10, 0.14, 0.95})
			titleH := float32(render.GlyphH + 8)
			r.DrawColorRect(panelX, panelY, panelW, titleH, mgl32.Vec4{0.15, 0.20, 0.35, 1.0})
			if r.Font != nil {
				r.Font.DrawText(r, "Snow Setup", panelX+8, panelY+4, mgl32.Vec4{0.9, 0.95, 1, 1})
			}
		})
		edDrawables = append(edDrawables, panel)
		for _, s := range sliders {
			edDrawables = append(edDrawables, s)
		}
		edDrawables = append(edDrawables, e.addStormBtn, e.clearLayersBtn)
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

// layoutAutoSliders positions the auto-gen sliders inside the snow panel.
// col=80 gives each slider 80 px of horizontal space; the widest label
// ("Cover", 5 chars × 15 px = 75 px) fits with a 5 px gap on each side.
func (e *Editor) layoutAutoSliders(r *render.Renderer) {
	const trackW = float32(18)
	const trackH = float32(200)
	const col = float32(80)
	sh := float32(r.ScreenHeight())
	y := (sh-trackH)/2 + 20
	for i, s := range e.autoSliders() {
		s.X = 20 + float32(i)*col
		s.Y = y
		s.W = trackW
		s.H = trackH
	}
}

// layoutLayerButtons positions the Add Storm and Clear Snow buttons below
// the auto-gen sliders. The gap includes one glyph height to clear the
// value text that VSlider draws at track_bottom+4.
func (e *Editor) layoutLayerButtons(r *render.Renderer) {
	const trackH = float32(200)
	sh := float32(r.ScreenHeight())
	btnY := (sh-trackH)/2 + 20 + trackH + float32(render.GlyphH) + 12
	e.addStormBtn.X = 20
	e.addStormBtn.Y = btnY
	e.clearLayersBtn.X = 20 + e.addStormBtn.W + 8
	e.clearLayersBtn.Y = btnY
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
