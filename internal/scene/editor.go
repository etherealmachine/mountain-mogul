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
	escapeMenu        *EscapeMenu
	toolButtons       map[toolMode]*ui.Button
	activeTool        toolMode
	scenarioPath      string
	hoverCell         [2]int      // terrain cell currently under the mouse
	radiusSlider      *ui.VSlider // shown for any brush tool
	densitySlider     *ui.VSlider // shown only for the plant tool — caps brush target density
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
	e.toolButtons[toolPlantTrees] = e.menuBar.AddIconButton(render.IconTreeEvergreen, "Plant", func() { e.setTool(toolPlantTrees) })
	e.menuBar.AddIconButton(render.IconTreeEvergreen, "Auto-forest", func() {
		GenerateTreeCover(e.world.Terrain, 24, 0.55, time.Now().UnixNano())
		e.app.Renderer.RebuildStaticBatch(e.world)
	})
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

	// Brush sliders — repositioned each Render relative to screen size.
	// Radius shows for any brush tool; density only for plant. Range max is
	// generous on radius so users can paint a whole forest in one stroke.
	e.radiusSlider = ui.NewVSlider(0, 0, 18, 200, 1, 30, float32(defaultBrushRadius), "Radius")
	e.densitySlider = ui.NewVSlider(0, 0, 18, 200, 0, 100, 100, "Density")

	return nil
}

const (
	toolEditorRaise = toolMode(100)
	toolEditorLower = toolMode(101)
)

func (e *Editor) Update(dt float64) {
	inp := e.app.Input
	r := e.app.Renderer

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

	// C: cycle terrain overlay (off → contour → slope debug → off).
	if inp.Pressed[glfw.KeyC] {
		r.TerrainOverlayMode = (r.TerrainOverlayMode + 1) % 3
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

	// Track terrain cell under mouse for brush preview.
	if !e.menuBar.ContainsY(inp.MousePos[1]) {
		gx, gz := screenToCell(r.Camera, e.world.Terrain, inp.MousePos)
		e.hoverCell = [2]int{gx, gz}
	} else {
		e.hoverCell = [2]int{-1, -1}
	}

	// Tool application — suppressed when a slider is grabbing input or
	// the cursor is over slider/menu chrome.
	if !sliderActive && (inp.LeftClick || inp.LeftHeld) && !e.menuBar.ContainsY(inp.MousePos[1]) {
		overSlider := e.toolUsesRadiusSlider() && e.radiusSlider.Contains(inp.MousePos[0], inp.MousePos[1])
		overSlider = overSlider || (e.toolUsesDensitySlider() && e.densitySlider.Contains(inp.MousePos[0], inp.MousePos[1]))
		if !overSlider {
			gx, gz := e.hoverCell[0], e.hoverCell[1]
			if e.world.Terrain.InBounds(gx, gz) {
				e.applyEditorTool(gx, gz, r, float32(dt))
			}
		}
	}
}

// setTool toggles the given editor tool; re-clicking an active tool deactivates it.
func (e *Editor) setTool(t toolMode) {
	if e.activeTool == t {
		e.activeTool = toolNone
	} else {
		e.activeTool = t
	}
	e.syncToolButtons()
}

// syncToolButtons updates the active state of all tool buttons to match activeTool.
func (e *Editor) syncToolButtons() {
	for mode, btn := range e.toolButtons {
		btn.SetActive(e.activeTool == mode)
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
	case toolEditorLower:
		w.Terrain.Cells[gx][gz].GroundElevation -= 5.0 * dt
		if w.Terrain.Cells[gx][gz].GroundElevation < 0 {
			w.Terrain.Cells[gx][gz].GroundElevation = 0
		}
		r.FlushTerrainVerts(w.Terrain)
	}
}

// applyImportedTerrain replaces the entire scene with a fresh world built
// from the imported elevations. Buildings, lifts, agents, placed objects,
// and per-cell tree density are wiped — re-using terrain on top of an old
// layout would leave lifts dangling in mid-air and trees floating above
// new mountains. Cell defaults (snow depth, passability) match NewTerrain.
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
		}
	}
	e.world = world.NewWorld(t)
	e.activeTool = toolNone
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
	edDrawables := []render.UIDrawable{e.menuBar}
	if e.toolUsesRadiusSlider() {
		e.layoutBrushSliders(r)
		edDrawables = append(edDrawables, e.radiusSlider)
		if e.toolUsesDensitySlider() {
			edDrawables = append(edDrawables, e.densitySlider)
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
