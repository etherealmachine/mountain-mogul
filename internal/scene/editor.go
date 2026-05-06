package scene

import (
	"fmt"

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
	app          *engine.App
	world        *world.World
	menuBar      *ui.MenuBar
	escapeMenu   *EscapeMenu
	toolButtons  map[toolMode]*ui.Button
	activeTool   toolMode
	scenarioPath string
	hoverCell    [2]int // terrain cell currently under the mouse
}

// NewEditor creates an Editor scene loading from the given path.
func NewEditor(path string) *Editor {
	return &Editor{scenarioPath: path}
}

func (e *Editor) Init(app *engine.App) error {
	e.app = app

	w, err := save.LoadScenario(e.scenarioPath)
	if err != nil {
		fmt.Printf("Editor load error (%v), creating blank world\n", err)
		t := world.NewTerrain(256, 256)
		w = world.NewWorld(t)
	}
	e.world = w

	r := app.Renderer
	r.BuildTerrainMesh(w.Terrain)
	r.RebuildStaticBatch(w)
	for _, lift := range w.Lifts {
		r.AddLiftCable(lift, w.Terrain)
	}

	r.Camera.Target = mgl32.Vec3{
		float32(w.Terrain.Width) * 5,
		0,
		float32(w.Terrain.Height) * 5,
	}
	r.Camera.OrthoScale = 700
	r.Camera.Recalculate()

	e.toolButtons = make(map[toolMode]*ui.Button)
	e.menuBar = ui.NewMenuBar(0, render.GlyphH+10)
	e.toolButtons[toolPlantTrees] = e.menuBar.AddButton("Plant Trees", func() { e.setTool(toolPlantTrees) })
	e.toolButtons[toolGlade] = e.menuBar.AddButton("Glade", func() { e.setTool(toolGlade) })
	e.toolButtons[toolEditorRaise] = e.menuBar.AddButton("Raise", func() { e.setTool(toolEditorRaise) })
	e.toolButtons[toolEditorLower] = e.menuBar.AddButton("Lower", func() { e.setTool(toolEditorLower) })
	e.menuBar.AddButton("Import Terrain", func() {
		app.PushScene(NewTerrainImport(
			e.world.Terrain.Width,
			e.world.Terrain.Height,
			func(elevs [][]float32) { e.applyImportedTerrain(elevs, e.app.Renderer) },
		))
	})
	e.menuBar.AddButton("Save", func() {
		if err := save.SaveScenario(e.scenarioPath, e.world); err != nil {
			fmt.Println("Save error:", err)
		} else {
			fmt.Println("Saved to", e.scenarioPath)
		}
	})

	e.escapeMenu = NewEscapeMenu(app, func() {
		if err := save.SaveScenario(e.scenarioPath, e.world); err != nil {
			fmt.Println("Save error:", err)
		} else {
			fmt.Println("Saved to", e.scenarioPath)
		}
	}, nil)

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

	// C: toggle slope + contour overlay.
	if inp.Pressed[glfw.KeyC] {
		if r.TerrainOverlayMode == 0 {
			r.TerrainOverlayMode = 1
		} else {
			r.TerrainOverlayMode = 0
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
		r.Camera.Yaw += rotDelta
		r.Camera.Recalculate()
	}

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

	// Track terrain cell under mouse for brush preview.
	if !e.menuBar.ContainsY(inp.MousePos[1]) {
		gx, gz := screenToCell(r.Camera, e.world.Terrain, inp.MousePos)
		e.hoverCell = [2]int{gx, gz}
	} else {
		e.hoverCell = [2]int{-1, -1}
	}

	// Tool application
	if (inp.LeftClick || inp.LeftHeld) && !e.menuBar.ContainsY(inp.MousePos[1]) {
		gx, gz := e.hoverCell[0], e.hoverCell[1]
		if e.world.Terrain.InBounds(gx, gz) {
			e.applyEditorTool(gx, gz, r, float32(dt))
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

const editorBrushRadius = 2 // cells

func (e *Editor) applyEditorTool(gx, gz int, r *render.Renderer, dt float32) {
	w := e.world
	switch e.activeTool {
	case toolPlantTrees:
		applyDensityBrush(w.Terrain, gx, gz, editorBrushRadius, 0.3)
		r.RebuildStaticBatch(w)
	case toolGlade:
		applyDensityBrush(w.Terrain, gx, gz, editorBrushRadius, -0.4)
		r.RebuildStaticBatch(w)
	case toolEditorRaise:
		w.Terrain.Cells[gx][gz].Elevation += 5.0 * dt
		r.FlushTerrainVerts(w.Terrain)
	case toolEditorLower:
		w.Terrain.Cells[gx][gz].Elevation -= 5.0 * dt
		if w.Terrain.Cells[gx][gz].Elevation < 0 {
			w.Terrain.Cells[gx][gz].Elevation = 0
		}
		r.FlushTerrainVerts(w.Terrain)
	}
}

func (e *Editor) applyImportedTerrain(elevs [][]float32, r *render.Renderer) {
	t := e.world.Terrain
	for row := 0; row < t.Height; row++ {
		for col := 0; col < t.Width; col++ {
			if row < len(elevs) && col < len(elevs[row]) {
				t.Cells[col][row].Elevation = elevs[row][col]
			}
		}
	}
	r.BuildTerrainMesh(t)
}

func (e *Editor) Render(r *render.Renderer) {
	const cellSize = float32(10.0)
	t := e.world.Terrain
	if (e.activeTool == toolGlade || e.activeTool == toolPlantTrees) && t.InBounds(e.hoverCell[0], e.hoverCell[1]) {
		gx, gz := e.hoverCell[0], e.hoverCell[1]
		center := mgl32.Vec2{float32(gx)*cellSize + cellSize/2, float32(gz)*cellSize + cellSize/2}
		r.SetBrush(center, (float32(editorBrushRadius)+0.5)*cellSize)
	} else {
		r.ClearBrush()
	}

	r.DrawWorld(e.world, 0)
	r.ClearBrush()

	edDrawables := []render.UIDrawable{e.menuBar}
	if e.escapeMenu.Visible() {
		edDrawables = append(edDrawables, e.escapeMenu)
	}
	r.DrawUI(edDrawables)
}

func (e *Editor) Destroy() {}

func clampF(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
