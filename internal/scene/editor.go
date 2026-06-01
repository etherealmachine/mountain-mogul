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
	"mountain-mogul/internal/settings"
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
	settingsMenu      *SettingsMenu
	toolButtons       map[toolMode]*ui.Button
	// Submenu groups
	buildingsSubmenu *ui.SubmenuButton
	transportSubmenu *ui.SubmenuButton
	liftsSubmenu     *ui.SubmenuButton
	terrainSubmenu   *ui.SubmenuButton
	// Lift variant buttons (outside toolButtons — multiple share toolLiftBase/Top)
	liftDoubleBtn  *ui.Button
	liftQuadBtn    *ui.Button
	liftHSQuadBtn  *ui.Button
	liftHS6PackBtn *ui.Button
	liftGondolaBtn *ui.Button
	liftHeliBtn    *ui.Button
	liftType       world.LiftType
	activeTool     toolMode
	scenarioPath   string
	hoverCell        [2]int
	hoverWorld       mgl32.Vec3
	hoverMouseScreen mgl32.Vec2
	hoverValid       bool
	radiusSlider  *ui.VSlider // shown for any brush tool
	densitySlider *ui.VSlider // plant tool only
	// Parcel rect-selection state
	parcelRectStart  [2]int
	parcelRectActive bool
	parcelRectIntent parcelRectIntent
	parcelEditID     uint16      // ID of parcel being modified via rect or popup
	parcelPopup      *ui.Window
	autoMaxSlider      *ui.VSlider
	autoSnowlineSlider *ui.VSlider
	autoTreelineSlider *ui.VSlider
	autoCoverageSlider *ui.VSlider
	autoWindSlider     *ui.VSlider
	autoSeed           int64
	autoFields         *elevFields
	liftBase           mgl32.Vec2
	roadStart          mgl32.Vec2
	roadEdit           roadEditSelection
	structureEdit      structureEditSelection
	addStormBtn        *ui.Button
	clearLayersBtn     *ui.Button
	parcelBoundaryDirty        bool // fence geometry needs rebuild
	suppressBrushUntilRelease  bool // set when a brush tool is activated via toolbar click; cleared on mouse-up
	pendingScreenshot          bool
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
	e.parcelBoundaryDirty = true

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

	// Editor tool bar — grouped submenus matching the gameplay toolbar style.
	e.toolButtons = make(map[toolMode]*ui.Button)
	e.menuBar = ui.NewMenuBar(0, 60)
	e.menuBar.Centered = true

	// Buildings submenu: Lodge, Shed
	e.buildingsSubmenu = e.menuBar.AddSubmenu(render.IconHouse, "Buildings")
	e.toolButtons[toolBuilding] = e.buildingsSubmenu.AddChild(render.IconHouse, "Lodge", func() { e.setTool(toolBuilding) })
	e.toolButtons[toolShed] = e.buildingsSubmenu.AddChild(render.IconGarage, "Shed", func() { e.setTool(toolShed) })

	// Transport submenu: Parking, Road, Edge Connect
	e.transportSubmenu = e.menuBar.AddSubmenu(render.IconRoad, "Transport")
	e.toolButtons[toolParking] = e.transportSubmenu.AddChild(render.IconUsers, "Parking", func() { e.setTool(toolParking) })
	e.toolButtons[toolRoadStart] = e.transportSubmenu.AddChild(render.IconRoad, "Road", func() { e.setTool(toolRoadStart) })
	e.toolButtons[toolEdgeConnect] = e.transportSubmenu.AddChild(render.IconFlag, "Edge", func() { e.setTool(toolEdgeConnect) })

	// Lifts submenu
	e.liftsSubmenu = e.menuBar.AddSubmenu(render.IconCableCar, "Lifts")
	e.liftDoubleBtn = e.liftsSubmenu.AddChild(render.IconCableCar, "Double", func() { e.activateLiftTool(world.LiftDouble) })
	e.liftQuadBtn = e.liftsSubmenu.AddChild(render.IconCableCar, "Quad", func() { e.activateLiftTool(world.LiftFixedQuad) })
	e.liftHSQuadBtn = e.liftsSubmenu.AddChild(render.IconCableCar, "HSQuad", func() { e.activateLiftTool(world.LiftHSQuad) })
	e.liftHS6PackBtn = e.liftsSubmenu.AddChild(render.IconCableCar, "6-Pack", func() { e.activateLiftTool(world.LiftHS6Pack) })
	e.liftGondolaBtn = e.liftsSubmenu.AddChild(render.IconCableCar, "Gondola", func() { e.activateLiftTool(world.LiftGondola) })
	e.liftHeliBtn = e.liftsSubmenu.AddChild(render.IconCableCar, "Heli", func() { e.activateLiftTool(world.LiftHeli) })

	// Terrain submenu: Plant Trees, Auto-gen, Glade
	e.terrainSubmenu = e.menuBar.AddSubmenu(render.IconTreeEvergreen, "Terrain")
	e.toolButtons[toolPlantTrees] = e.terrainSubmenu.AddChild(render.IconTreeEvergreen, "Plant", func() { e.setTool(toolPlantTrees) })
	e.toolButtons[toolAuto] = e.terrainSubmenu.AddChild(render.IconSnowflake, "Auto", func() { e.setTool(toolAuto) })
	e.toolButtons[toolGlade] = e.terrainSubmenu.AddChild(render.IconAxe, "Glade", func() { e.setTool(toolGlade) })

	// Land: single button to start drawing a new parcel rectangle
	e.toolButtons[toolParcelRect] = e.menuBar.AddIconButton(render.IconGlobe, "Add Parcel", func() {
		e.parcelRectIntent = parcelIntentNew
		e.parcelRectActive = false
		e.parcelEditID = 0
		e.setTool(toolParcelRect)
	})

	// Flat utility buttons
	e.toolButtons[toolRemove] = e.menuBar.AddIconButton(render.IconTrash, "Remove", func() { e.setTool(toolRemove) })
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

	e.settingsMenu = NewSettingsMenu(app, func() { e.escapeMenu.Show() })
	openSettings := func() {
		e.escapeMenu.Hide()
		e.settingsMenu.Show()
	}
	e.escapeMenu = NewEscapeMenu(app, func() {
		if err := save.SaveScenario(e.scenarioPath, e.world, editorCameraSnapshot(e)); err != nil {
			fmt.Println("Save error:", err)
		} else {
			fmt.Println("Saved to", e.scenarioPath)
		}
	}, nil, openSettings)

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
	e.autoMaxSlider.DisplayFunc = func(v float32) string { return settings.FormatDepth(v) }
	e.autoSnowlineSlider = ui.NewVSlider(0, 0, 18, 200, 0, 100, 70, "Elev")
	e.autoSnowlineSlider.ValueFormat = "%.0f%%"
	e.autoTreelineSlider = ui.NewVSlider(0, 0, 18, 200, 0, 100, 70, "Tree")
	e.autoTreelineSlider.ValueFormat = "%.0f%%"
	e.autoCoverageSlider = ui.NewVSlider(0, 0, 18, 200, 0, 100, 55, "Cover")
	e.autoCoverageSlider.ValueFormat = "%.0f%%"
	e.autoWindSlider = ui.NewVSlider(0, 0, 18, 200, 0, 360, 270, "Wind")
	e.autoWindSlider.ValueFormat = "%.0fdeg"

	e.addStormBtn = ui.NewButton(0, 0, 120, 28, "+ Add Storm", func() {
		e.pushSnowLayer(world.KindPowder)
	})
	e.clearLayersBtn = ui.NewButton(0, 0, 100, 28, "Clear Snow", func() {
		e.clearAllLayers()
	})

	return nil
}

const (
	toolAuto       = toolMode(102)
	toolParcelRect = toolMode(103) // two-click rectangle selection for parcel authoring
)

// parcelRectIntent describes what a committed rect selection should do.
type parcelRectIntent int

const (
	parcelIntentNew    parcelRectIntent = iota // create a new parcel from the rect
	parcelIntentAdd    parcelRectIntent = iota // add rect cells to parcelEditID
	parcelIntentRemove parcelRectIntent = iota // remove rect cells from parcelEditID
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
		case e.settingsMenu.Visible():
			e.settingsMenu.Hide()
			e.escapeMenu.Show()
		case e.roadEdit.active() || e.structureEdit.active():
			e.roadEdit.clear()
			e.structureEdit.clear()
		case e.activeTool == toolParcelRect && e.parcelRectActive:
			// Cancel the in-progress selection but stay in the tool.
			e.parcelRectActive = false
		case e.activeTool != toolNone:
			e.activeTool = toolNone
			e.parcelRectActive = false
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
	if e.settingsMenu.Visible() {
		e.settingsMenu.HandleInput(inp)
		return
	}
	if e.escapeMenu.Visible() {
		e.escapeMenu.HandleInput(inp)
		return
	}
	if e.parcelPopup != nil && e.parcelPopup.Visible {
		e.parcelPopup.HandleInput(inp)
		// Close popup on Escape while it's open.
		if inp.Pressed[glfw.KeyEscape] {
			e.parcelPopup.Visible = false
		}
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
	popupCoversClick := e.parcelPopup != nil && e.parcelPopup.Visible &&
		e.parcelPopup.ContainsPoint(inp.MousePos[0], inp.MousePos[1])
	overChrome := e.menuBar.ContainsY(inp.MousePos[1]) ||
		e.topBar.ContainsY(inp.MousePos[1]) ||
		e.overlayPanel.ContainsXY(inp.MousePos[0], inp.MousePos[1], float32(r.ScreenWidth())) ||
		popupCoversClick
	if !overChrome {
		if pos, ok := screenToWorld(r.Camera, e.world.Terrain, inp.MousePos); ok {
			e.hoverWorld = pos
			e.hoverMouseScreen = inp.MousePos
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
		liftType:   e.liftType,
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
	// Clear the suppress flag once the mouse button is fully released.
	if !inp.LeftClick && !inp.LeftHeld {
		e.suppressBrushUntilRelease = false
	}

	if !sliderActive && !overChrome && !inp.LeftClickConsumed && !e.suppressBrushUntilRelease {
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
				shiftHeld := inp.Held[glfw.KeyLeftShift] || inp.Held[glfw.KeyRightShift]
				// toolParcelRect can fire off-terrain (first or second click):
				// clamp to map bounds so the rect can reach the edges.
				placementReady := inp.LeftClick && (e.hoverValid ||
					e.activeTool == toolParcelRect)
				if placementReady {
					if !e.hoverValid && e.activeTool == toolParcelRect {
						e.hoverCell = e.clampedTerrainCell(r, inp.MousePos)
					}
					e.applyPlacement(r, shiftHeld)
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
		} else if p := e.world.ParcelAt(e.hoverCell[0], e.hoverCell[1]); p != nil {
			e.roadEdit.clear()
			e.structureEdit.clear()
			e.openParcelPopup(p.ID, r.ScreenWidth(), r.ScreenHeight())
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
	case toolBuilding, toolShed, toolParking, toolLiftBase, toolLiftTop, toolRoadStart, toolRoadEnd, toolEdgeConnect, toolRemove, toolParcelRect:
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
// are free. shiftHeld enables flood-fill mode for toolParcelRect.
func (e *Editor) applyPlacement(r *render.Renderer, shiftHeld bool) {
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
		if lift.IsHeli() {
			applyHelipadPlacementEffects(w.Terrain, lift)
		} else {
			applyLiftPlacementEffects(w.Terrain, lift)
			r.AddLiftCable(lift, w.Terrain)
		}
		r.FlushTerrainVerts(w.Terrain)
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
	case toolRemove:
		pick := mgl32.Vec2{wx, wz}
		for _, b := range w.Buildings {
			if b.Pos.Sub(pick).Len() <= buildingPickRadius {
				wasParking := b.Type == world.BuildingParking
				w.RemoveBuilding(b.ID)
				r.FlushTerrainVerts(w.Terrain)
				r.RebuildStaticBatch(w)
				if wasParking {
					r.RebuildRoads(w)
				}
				return
			}
		}
	case toolParcelRect:
		gx, gz := e.hoverCell[0], e.hoverCell[1]
		if !w.Terrain.InBounds(gx, gz) {
			return
		}
		if shiftHeld && !e.parcelRectActive {
			// Shift + first click: flood fill to roads / map boundary.
			e.commitParcelFloodFill(gx, gz, r)
		} else if !e.parcelRectActive {
			// First click: record start corner.
			e.parcelRectStart = [2]int{gx, gz}
			e.parcelRectActive = true
		} else if shiftHeld {
			// Shift + second click: full unclipped rectangle.
			e.commitParcelRect(gx, gz, r)
		} else if e.parcelRectIntent == parcelIntentRemove {
			// Remove: plain rectangle — road-clipping would block on the
			// target parcel's own cells, preventing any selection.
			e.commitParcelRect(gx, gz, r)
		} else {
			// Add / new: rectangle clipped to road boundaries.
			cells := e.rectCellsClippedToRoads(
				e.parcelRectStart[0], e.parcelRectStart[1], gx, gz)
			e.parcelRectActive = false
			e.commitCells(cells, r)
		}
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
		e.parcelRectActive = false
	} else {
		e.activeTool = t
		e.suppressBrushUntilRelease = true
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
func (e *Editor) pushSnowLayer(kind world.SnowKind) {
	if e.world == nil || e.world.Terrain == nil {
		return
	}
	if e.autoFields == nil {
		e.autoFields = computeElevFields(e.world.Terrain)
	}
	addSnowLayerCached(
		e.autoFields,
		e.world.Terrain,
		kind,
		e.autoMaxSlider.Value,
		1.0-e.autoSnowlineSlider.Value/100,
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
			t.Cells[x][z].Base = 0
			t.Cells[x][z].Top = world.SnowLayer{}
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
		1.0-e.autoSnowlineSlider.Value/100,
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
	// Re-stamp clearances that generateTreeCover would otherwise overwrite.
	for _, b := range e.world.Buildings {
		halfX, halfZ := buildingFootprint(b.Type)
		clearBuildingTrees(e.world.Terrain, b.Pos, halfX, halfZ)
	}
	for _, lift := range e.world.Lifts {
		clearLiftCorridor(e.world.Terrain, lift.Base, lift.Top, liftCorridorHalfWidth)
	}
	applyRoadCellState(e.world)
	if e.app != nil && e.app.Renderer != nil {
		e.app.Renderer.FlushTerrainVerts(e.world.Terrain)
		e.app.Renderer.RebuildStaticBatch(e.world)
	}
}

// syncToolButtons updates the active state of all tool buttons to match activeTool.
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
	if e.liftHSQuadBtn != nil {
		e.liftHSQuadBtn.SetActive(liftActive && e.liftType == world.LiftHSQuad)
	}
	if e.liftHS6PackBtn != nil {
		e.liftHS6PackBtn.SetActive(liftActive && e.liftType == world.LiftHS6Pack)
	}
	if e.liftGondolaBtn != nil {
		e.liftGondolaBtn.SetActive(liftActive && e.liftType == world.LiftGondola)
	}
	if e.liftHeliBtn != nil {
		e.liftHeliBtn.SetActive(liftActive && e.liftType == world.LiftHeli)
	}
	// Highlight submenu parent when any child is active.
	for _, sub := range []*ui.SubmenuButton{
		e.buildingsSubmenu, e.transportSubmenu, e.liftsSubmenu, e.terrainSubmenu,
	} {
		if sub != nil {
			sub.Btn.SetActive(sub.HasActiveChild())
		}
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
// the active tool.
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
			t.Cells[col][row].Base = 0
			t.Cells[col][row].Top = world.SnowLayer{}
		}
	}
	t.RecomputeSlopes()
	e.world = world.NewWorld(t)
	e.activeTool = toolNone
	e.autoFields = nil
	e.parcelBoundaryDirty = true
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

// parcelFloodFill returns all terrain cells reachable from (startX, startZ) by
// 4-connected BFS, stopping at map edges, impassable cells, and cells within
// the road inner-snow band (same radius used by applyRoadCellState). This
// lets the designer flood-fill a natural region bounded by roads and the
// terrain perimeter rather than drawing a rectangle.
func (e *Editor) parcelFloodFill(startX, startZ int) [][2]int {
	t := e.world.Terrain
	if !t.InBounds(startX, startZ) {
		return nil
	}

	// Build a per-cell "blocked" grid: impassable structures + cells near roads.
	const cs = float32(world.CellSize)
	const innerR2 = roadSnowInnerRadius * roadSnowInnerRadius
	blocked := make([][]bool, t.Width)
	for x := range blocked {
		blocked[x] = make([]bool, t.Height)
		for z := range blocked[x] {
			if !t.Cells[x][z].Passable {
				blocked[x][z] = true
			}
		}
	}
	for _, chain := range e.world.FindRoadChains() {
		samples := world.SampleRoadChain(chain, t, world.RoadChainSamplesPerSegment)
		if len(samples) < 2 {
			continue
		}
		// Restrict to a bbox around the chain to avoid scanning the whole map.
		minX, maxX := samples[0][0], samples[0][0]
		minZ, maxZ := samples[0][1], samples[0][1]
		for _, s := range samples[1:] {
			if s[0] < minX {
				minX = s[0]
			}
			if s[0] > maxX {
				maxX = s[0]
			}
			if s[1] < minZ {
				minZ = s[1]
			}
			if s[1] > maxZ {
				maxZ = s[1]
			}
		}
		x0 := int((minX-roadSnowInnerRadius)/cs) - 1
		x1 := int((maxX+roadSnowInnerRadius)/cs) + 1
		z0 := int((minZ-roadSnowInnerRadius)/cs) - 1
		z1 := int((maxZ+roadSnowInnerRadius)/cs) + 1
		for x := x0; x <= x1; x++ {
			for z := z0; z <= z1; z++ {
				if !t.InBounds(x, z) || blocked[x][z] {
					continue
				}
				cx := (float32(x) + 0.5) * cs
				cz := (float32(z) + 0.5) * cs
				cell := mgl32.Vec2{cx, cz}
				for i := 0; i < len(samples)-1; i++ {
					cp := world.ClosestPointOnRoadSegment(cell, samples[i], samples[i+1])
					dx := cx - cp[0]
					dz := cz - cp[1]
					if dx*dx+dz*dz <= innerR2 {
						blocked[x][z] = true
						break
					}
				}
			}
		}
	}

	// Also block cells that already belong to any parcel, so the fill
	// respects existing parcel boundaries as walls.
	for _, p := range e.world.Parcels {
		for _, c := range p.Cells {
			if t.InBounds(c[0], c[1]) {
				blocked[c[0]][c[1]] = true
			}
		}
	}

	if blocked[startX][startZ] {
		return nil
	}

	// 4-connected BFS.
	visited := make([][]bool, t.Width)
	for x := range visited {
		visited[x] = make([]bool, t.Height)
	}
	visited[startX][startZ] = true
	queue := [][2]int{{startX, startZ}}
	var result [][2]int
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		result = append(result, cur)
		for _, d := range dirs {
			nx, nz := cur[0]+d[0], cur[1]+d[1]
			if !t.InBounds(nx, nz) || blocked[nx][nz] || visited[nx][nz] {
				continue
			}
			visited[nx][nz] = true
			queue = append(queue, [2]int{nx, nz})
		}
	}
	return result
}

// clampedTerrainCell projects screenPos onto the terrain base plane (y = 0)
// and returns the nearest in-bounds cell, clamped to the terrain edges.
// Used so off-map clicks can still commit a parcel rect to the map boundary.
func (e *Editor) clampedTerrainCell(r *render.Renderer, screenPos mgl32.Vec2) [2]int {
	t := e.world.Terrain
	const cs = float32(world.CellSize)
	world3 := r.Camera.ScreenToTerrain(screenPos, 0)
	cx := int(world3[0] / cs)
	cz := int(world3[2] / cs)
	if cx < 0 {
		cx = 0
	} else if cx >= t.Width {
		cx = t.Width - 1
	}
	if cz < 0 {
		cz = 0
	} else if cz >= t.Height {
		cz = t.Height - 1
	}
	return [2]int{cx, cz}
}

// rectCellsClippedToRoads returns the subset of cells in the axis-aligned rect
// that are reachable from (ax, az) without crossing roads, existing parcels, or
// impassable cells. It reuses parcelFloodFill for the connectivity check and
// then intersects the result with the rect bounds.
func (e *Editor) rectCellsClippedToRoads(ax, az, bx, bz int) [][2]int {
	x0, x1 := ax, bx
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	z0, z1 := az, bz
	if z0 > z1 {
		z0, z1 = z1, z0
	}
	flooded := e.parcelFloodFill(ax, az)
	var result [][2]int
	for _, c := range flooded {
		if c[0] >= x0 && c[0] <= x1 && c[1] >= z0 && c[1] <= z1 {
			result = append(result, c)
		}
	}
	return result
}

// rectCells returns all in-bounds terrain cells within the axis-aligned rect
// defined by two grid corners (inclusive on both ends).
func (e *Editor) rectCells(ax, az, bx, bz int) [][2]int {
	x0, x1 := ax, bx
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	z0, z1 := az, bz
	if z0 > z1 {
		z0, z1 = z1, z0
	}
	var out [][2]int
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if e.world.Terrain.InBounds(x, z) {
				out = append(out, [2]int{x, z})
			}
		}
	}
	return out
}

// commitParcelRect finalises a two-click rect selection for parcel authoring.
func (e *Editor) commitParcelRect(ex, ez int, r *render.Renderer) {
	cells := e.rectCells(e.parcelRectStart[0], e.parcelRectStart[1], ex, ez)
	e.parcelRectActive = false
	e.commitCells(cells, r)
}

// commitParcelFloodFill runs a flood fill from (startX, startZ) bounded by
// roads and the map edge, then commits the resulting cell set using the same
// intent logic as commitParcelRect.
func (e *Editor) commitParcelFloodFill(startX, startZ int, r *render.Renderer) {
	cells := e.parcelFloodFill(startX, startZ)
	if len(cells) == 0 {
		return
	}
	e.commitCells(cells, r)
}

// commitCells is the shared finaliser for both rect and flood-fill parcel
// commits. It applies the current parcelRectIntent using the provided cell set.
func (e *Editor) commitCells(cells [][2]int, r *render.Renderer) {
	switch e.parcelRectIntent {
	case parcelIntentNew:
		var maxID uint16
		for _, p := range e.world.Parcels {
			if p.ID > maxID {
				maxID = p.ID
			}
		}
		newID := maxID + 1
		e.world.Parcels = append(e.world.Parcels, world.Parcel{
			ID:    newID,
			State: world.ParcelPurchasable,
			Price: 50000,
			Cells: cells,
		})
		e.parcelEditID = newID
		e.world.ApplyParcels()
		e.parcelBoundaryDirty = true
		e.activeTool = toolNone
		e.syncToolButtons()
		e.openParcelPopup(newID, r.ScreenWidth(), r.ScreenHeight())

	case parcelIntentAdd:
		for _, c := range cells {
			e.removeCellFromAllParcels(c[0], c[1])
		}
		for i := range e.world.Parcels {
			if e.world.Parcels[i].ID == e.parcelEditID {
				e.world.Parcels[i].Cells = append(e.world.Parcels[i].Cells, cells...)
				break
			}
		}
		e.world.ApplyParcels()
		e.parcelBoundaryDirty = true
		e.activeTool = toolNone
		e.syncToolButtons()
		e.openParcelPopup(e.parcelEditID, r.ScreenWidth(), r.ScreenHeight())

	case parcelIntentRemove:
		for i := range e.world.Parcels {
			if e.world.Parcels[i].ID != e.parcelEditID {
				continue
			}
			for _, c := range cells {
				cs := e.world.Parcels[i].Cells
				for j := range cs {
					if cs[j] == c {
						cs[j] = cs[len(cs)-1]
						e.world.Parcels[i].Cells = cs[:len(cs)-1]
						break
					}
				}
			}
			break
		}
		e.world.ApplyParcels()
		e.parcelBoundaryDirty = true
		e.activeTool = toolNone
		e.syncToolButtons()
		e.openParcelPopup(e.parcelEditID, r.ScreenWidth(), r.ScreenHeight())
	}
}

// openParcelPopup opens (or replaces) the parcel detail popup for the parcel
// with the given ID.
func (e *Editor) openParcelPopup(id uint16, screenW, screenH int) {
	e.parcelEditID = id

	// Capture ID for callbacks — avoids stale pointer if Parcels slice moves.
	parcel := func() *world.Parcel {
		for i := range e.world.Parcels {
			if e.world.Parcels[i].ID == id {
				return &e.world.Parcels[i]
			}
		}
		return nil
	}
	stateName := func(s world.ParcelState) string {
		switch s {
		case world.ParcelOwned:
			return "Owned"
		case world.ParcelPurchasable:
			return "Purchasable"
		case world.ParcelOffLimits:
			return "Off-limits"
		}
		return "?"
	}

	win := ui.NewWindow("Parcel", 0, 0)

	win.AddLabel("Type", func() string {
		if p := parcel(); p != nil {
			return stateName(p.State)
		}
		return "?"
	})
	win.AddActionButton("→ Owned", func() {
		if p := parcel(); p != nil {
			p.State = world.ParcelOwned
			e.world.ApplyParcels()
			e.parcelBoundaryDirty = true
		}
	})
	win.AddActionButton("→ Purchasable", func() {
		if p := parcel(); p != nil {
			p.State = world.ParcelPurchasable
			e.world.ApplyParcels()
			e.parcelBoundaryDirty = true
		}
	})
	win.AddActionButton("→ Off-limits", func() {
		if p := parcel(); p != nil {
			p.State = world.ParcelOffLimits
			e.world.ApplyParcels()
			e.parcelBoundaryDirty = true
		}
	})
	win.AddIntStepperFn("Price", func() string {
		if p := parcel(); p != nil {
			return fmt.Sprintf("$%d", p.Price)
		}
		return "$0"
	}, func() {
		if p := parcel(); p != nil && p.Price >= 5000 {
			p.Price -= 5000
		}
	}, func() {
		if p := parcel(); p != nil {
			p.Price += 5000
		}
	})
	win.AddLabel("Cells", func() string {
		if p := parcel(); p != nil {
			return fmt.Sprintf("%d", len(p.Cells))
		}
		return "0"
	})
	win.AddActionButton("Add Cells", func() {
		win.Visible = false
		e.parcelRectIntent = parcelIntentAdd
		e.parcelRectActive = false
		e.setTool(toolParcelRect)
	})
	win.AddActionButton("Remove Cells", func() {
		win.Visible = false
		e.parcelRectIntent = parcelIntentRemove
		e.parcelRectActive = false
		e.setTool(toolParcelRect)
	})
	win.AddActionButton("Delete Parcel", func() {
		for i, p := range e.world.Parcels {
			if p.ID == id {
				e.world.Parcels = append(e.world.Parcels[:i], e.world.Parcels[i+1:]...)
				break
			}
		}
		e.world.ApplyParcels()
		e.parcelBoundaryDirty = true
		win.Visible = false
		e.parcelPopup = nil
		e.parcelEditID = 0
	})

	win.Visible = true
	win.Center(screenW, screenH)
	e.parcelPopup = win
}

// paintParcelBrush assigns all cells within the current brush radius to a
// parcel of the given state, creating the parcel if it doesn't exist yet.
func (e *Editor) paintParcelBrush(cx, cz int, state world.ParcelState) {
	for _, c := range world.BrushCells(cx, cz, e.brushRadius()) {
		if !e.world.Terrain.InBounds(c[0], c[1]) {
			continue
		}
		e.setCellParcelState(c[0], c[1], state)
	}
	e.world.ApplyParcels()
	e.parcelBoundaryDirty = true
}

// eraseParcelBrush removes all cells within the brush radius from every parcel,
// reverting them to the default-accessible (unassigned) state.
func (e *Editor) eraseParcelBrush(cx, cz int) {
	for _, c := range world.BrushCells(cx, cz, e.brushRadius()) {
		e.removeCellFromAllParcels(c[0], c[1])
	}
	// Drop empty parcels.
	var keep []world.Parcel
	for _, p := range e.world.Parcels {
		if len(p.Cells) > 0 {
			keep = append(keep, p)
		}
	}
	e.world.Parcels = keep
	e.world.ApplyParcels()
	e.parcelBoundaryDirty = true
}

// setCellParcelState moves cell (cx, cz) into the parcel of the given state.
// It removes the cell from any other parcel first.
func (e *Editor) setCellParcelState(cx, cz int, state world.ParcelState) {
	e.removeCellFromAllParcels(cx, cz)
	p := e.findOrCreateParcel(state)
	p.Cells = append(p.Cells, [2]int{cx, cz})
}

// findOrCreateParcel returns the first parcel with the given state, creating
// one if none exists. Purchasable parcels use the current price slider value.
func (e *Editor) findOrCreateParcel(state world.ParcelState) *world.Parcel {
	for i := range e.world.Parcels {
		if e.world.Parcels[i].State == state {
			return &e.world.Parcels[i]
		}
	}
	var maxID uint16
	for _, p := range e.world.Parcels {
		if p.ID > maxID {
			maxID = p.ID
		}
	}
	e.world.Parcels = append(e.world.Parcels, world.Parcel{
		ID:    maxID + 1,
		State: state,
	})
	return &e.world.Parcels[len(e.world.Parcels)-1]
}

// removeCellFromAllParcels removes cell (cx, cz) from every parcel's cell list.
func (e *Editor) removeCellFromAllParcels(cx, cz int) {
	for i := range e.world.Parcels {
		cells := e.world.Parcels[i].Cells
		for j := 0; j < len(cells); j++ {
			if cells[j][0] == cx && cells[j][1] == cz {
				cells[j] = cells[len(cells)-1]
				e.world.Parcels[i].Cells = cells[:len(cells)-1]
				break
			}
		}
	}
}

// buildEditorParcelOverlay returns an RGBA8 cell overlay showing parcel states,
// the currently-being-edited parcel (brighter), and the live rect selection preview.
// rectEnd is the current second corner for the preview (may be clamped off-terrain).
func (e *Editor) buildEditorParcelOverlay(rectEnd [2]int) ([]uint8, int, int) {
	t := e.world.Terrain
	hasParcels := len(e.world.Parcels) > 0
	hasRect := e.activeTool == toolParcelRect && e.parcelRectActive
	if !hasParcels && !hasRect {
		return nil, 0, 0
	}
	tw, th := t.Width, t.Height
	pix := make([]uint8, tw*th*4)
	set := func(cx, cz int, rv, gv, bv, av uint8) {
		if cx < 0 || cx >= tw || cz < 0 || cz >= th {
			return
		}
		i := (cz*tw + cx) * 4
		pix[i], pix[i+1], pix[i+2], pix[i+3] = rv, gv, bv, av
	}
	// Parcels: dim tint for others, brighter for the one being edited.
	for _, p := range e.world.Parcels {
		editing := p.ID == e.parcelEditID && e.parcelEditID != 0
		alpha := uint8(100)
		if editing {
			alpha = 180
		}
		var rv, gv, bv uint8
		switch p.State {
		case world.ParcelOwned:
			rv, gv, bv = 55, 200, 55
		case world.ParcelPurchasable:
			rv, gv, bv = parcelPurchasableColor(p.ID)
		case world.ParcelOffLimits:
			rv, gv, bv = 220, 60, 60
		}
		for _, c := range p.Cells {
			set(c[0], c[1], rv, gv, bv, alpha)
		}
	}
	// Live rect selection preview — use the same cell set that would be
	// committed on click: road-clipped by default, full rect when shift held.
	if hasRect {
		var rv, gv, bv uint8
		switch e.parcelRectIntent {
		case parcelIntentRemove:
			rv, gv, bv = 240, 80, 80
		default:
			rv, gv, bv = 80, 200, 240
		}
		shiftHeld := e.app.Input.Held[glfw.KeyLeftShift] || e.app.Input.Held[glfw.KeyRightShift]
		var cells [][2]int
		if shiftHeld || e.parcelRectIntent == parcelIntentRemove {
			cells = e.rectCells(e.parcelRectStart[0], e.parcelRectStart[1],
				rectEnd[0], rectEnd[1])
		} else {
			cells = e.rectCellsClippedToRoads(e.parcelRectStart[0], e.parcelRectStart[1],
				rectEnd[0], rectEnd[1])
		}
		for _, c := range cells {
			set(c[0], c[1], rv, gv, bv, 170)
		}
	}
	return pix, tw, th
}

func (e *Editor) Render(r *render.Renderer) {
	const cellSize = float32(5.0)
	t := e.world.Terrain

	// Rebuild parcel fence geometry when parcels change.
	if e.parcelBoundaryDirty {
		postVerts, ropeLines := buildParcelFence(e.world)
		r.SetFencePostVerts(postVerts)
		r.SetBoundaryLines(ropeLines)
		e.parcelBoundaryDirty = false
	}

	// Parcel cell overlay — always visible when parcels are defined.
	// When drawing a rect off-terrain, clamp to the map boundary for the preview.
	rectEndCell := e.hoverCell
	if e.activeTool == toolParcelRect && e.parcelRectActive && !e.hoverValid {
		rectEndCell = e.clampedTerrainCell(r, e.hoverMouseScreen)
	}
	pix, ow, oh := e.buildEditorParcelOverlay(rectEndCell)
	r.SetCellOverlay(pix, ow, oh)

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

	// Parcel labels — price tag at the centroid of each purchasable parcel,
	// colored to match its palette entry so it reads as a caption for the tint.
	if len(e.world.Parcels) > 0 && r.Font != nil {
		w := e.world
		edDrawables = append(edDrawables, uiDrawFunc(func(rr *render.Renderer) {
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
	if e.activeTool == toolLiftTop && e.hoverValid {
		edDrawables = append(edDrawables, &liftDropLabel{
			terrain: e.world.Terrain,
			base:    e.liftBase,
			hover:   mgl32.Vec2{e.hoverWorld[0], e.hoverWorld[2]},
		})
	}
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
	if e.overlayPanel != nil && (e.overlayPanel.Mask()&render.OverlaySnowDepth) != 0 &&
		e.hoverValid && e.world.Terrain.InBounds(e.hoverCell[0], e.hoverCell[1]) {
		cell := e.world.Terrain.Cells[e.hoverCell[0]][e.hoverCell[1]]
		edDrawables = append(edDrawables, &snowLayerTooltip{
			base:    cell.Base,
			top:     cell.Top,
			mouseX:  e.hoverMouseScreen[0],
			mouseY:  e.hoverMouseScreen[1],
			screenW: r.ScreenWidth(),
			screenH: r.ScreenHeight(),
		})
	}
	if e.parcelPopup != nil && e.parcelPopup.Visible {
		edDrawables = append(edDrawables, e.parcelPopup)
	}
	if e.settingsMenu.Visible() {
		edDrawables = append(edDrawables, e.settingsMenu)
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
