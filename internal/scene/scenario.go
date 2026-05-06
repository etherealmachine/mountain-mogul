package scene

import (
	"fmt"
	"math"
	"math/rand"

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
// ray drops below the terrain surface.
func screenToCell(cam *render.Camera, terrain *world.Terrain, mousePos mgl32.Vec2) (gx, gz int) {
	const cellSize = float32(10.0)
	const maxElev = float32(1500.0)

	origin, dir := cam.ScreenToWorldRay(mousePos)
	if dir[1] >= 0 {
		return -1, -1
	}

	// Start t so that the ray is above the maximum terrain elevation.
	tCur := (maxElev - origin[1]) / dir[1]

	// Step size: half a cell width in XZ.
	xzLen := float32(math.Sqrt(float64(dir[0]*dir[0] + dir[2]*dir[2])))
	if xzLen < 1e-6 {
		return -1, -1
	}
	dt := (cellSize * 0.5) / xzLen

	for i := 0; i < 400; i++ {
		pos := origin.Add(dir.Mul(tCur))
		cx := int(pos[0] / cellSize)
		cz := int(pos[2] / cellSize)
		if terrain.InBounds(cx, cz) && pos[1] <= terrain.ElevationAt(cx, cz) {
			return cx, cz
		}
		tCur += dt
	}
	return -1, -1
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

// toolMode represents the active placement tool.
type toolMode int

const (
	toolNone        toolMode = iota
	toolBuilding    toolMode = iota
	toolLiftBase    toolMode = iota // waiting for first lift click
	toolLiftTop     toolMode = iota // waiting for second lift click
	toolGlade       toolMode = iota // reduce TreeDensity (brush)
	toolPlantTrees  toolMode = iota // increase TreeDensity (brush, editor only)
	toolRemove      toolMode = iota // remove building at clicked cell
)

// Scenario is the main gameplay scene.
type Scenario struct {
	app             *engine.App
	world           *world.World
	sim             *sim.Simulation
	menuBar         *ui.MenuBar
	escapeMenu      *EscapeMenu
	toolButtons     map[toolMode]*ui.Button
	activeTool      toolMode
	liftBase        [2]int // first click for lift placement
	scenarioPath    string
	time            float32
	rightDragging   bool
	hoverCell       [2]int // terrain cell currently under the mouse
	followAgentID   uint64 // 0 = free camera; >0 = ID of followed skier
}

// NewScenario creates a Scenario scene that loads from the given path.
func NewScenario(path string) *Scenario {
	return &Scenario{scenarioPath: path}
}

func (s *Scenario) Init(app *engine.App) error {
	s.app = app

	w, err := save.LoadScenario(s.scenarioPath)
	if err != nil {
		// Fall back to a blank world
		fmt.Printf("Scenario load error (%v), creating blank world\n", err)
		t := world.NewTerrain(32, 32)
		w = world.NewWorld(t)
	}
	s.world = w
	s.sim = sim.NewSimulation(w)

	// Build terrain mesh
	r := app.Renderer
	r.BuildTerrainMesh(w.Terrain)
	r.RebuildStaticBatch(w)

	// Generate cable meshes for existing lifts
	for _, lift := range w.Lifts {
		r.AddLiftCable(lift, w.Terrain)
	}

	s.escapeMenu = NewEscapeMenu(app, func() {
		if err := save.SaveScenario(s.scenarioPath, s.world); err != nil {
			fmt.Println("Save error:", err)
		} else {
			fmt.Println("Saved to", s.scenarioPath)
		}
	})

	// Build menu bar
	s.toolButtons = make(map[toolMode]*ui.Button)
	s.menuBar = ui.NewMenuBar(0, 32)
	s.toolButtons[toolBuilding] = s.menuBar.AddButton("Build Lodge", func() { s.setTool(toolBuilding) })
	s.toolButtons[toolLiftBase] = s.menuBar.AddButton("Place Lift", func() { s.setTool(toolLiftBase) })
	s.toolButtons[toolGlade] = s.menuBar.AddButton("Glade", func() { s.setTool(toolGlade) })
	s.toolButtons[toolRemove] = s.menuBar.AddButton("Remove", func() { s.setTool(toolRemove) })

	return nil
}

func (s *Scenario) Update(dt float64) {
	s.time += float32(dt)
	inp := s.app.Input
	r := s.app.Renderer

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

	// Menu bar input
	s.menuBar.HandleInput(inp, float32(r.ScreenHeight()))

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

	// Track terrain cell under mouse for brush preview.
	if !s.menuBar.ContainsY(inp.MousePos[1]) {
		gx, gz := screenToCell(r.Camera, s.world.Terrain, inp.MousePos)
		s.hoverCell = [2]int{gx, gz}
	} else {
		s.hoverCell = [2]int{-1, -1}
	}

	// Ghost preview for lift placement.
	s.updateLiftGhost(r)

	// World click / drag — glade supports held-down; placement tools use click-only.
	clickOrHeld := inp.LeftClick || (inp.LeftHeld && s.activeTool == toolGlade)
	if clickOrHeld && !s.menuBar.ContainsY(inp.MousePos[1]) {
		gx, gz := s.hoverCell[0], s.hoverCell[1]
		if s.world.Terrain.InBounds(gx, gz) {
			s.applyTool(gx, gz, r)
		}
	}

	// Tick simulation
	s.sim.Tick(dt)

	// Camera follow: track the selected agent using the freshest positions.
	if s.followAgentID != 0 {
		if agent := s.findFollowedAgent(); agent != nil {
			r.Camera.Target = agent.Pos
			r.Camera.Recalculate()
		} else {
			s.followAgentID = 0
		}
	}
}

const gladeRadius = 2 // cells

func (s *Scenario) applyTool(gx, gz int, r *render.Renderer) {
	defer s.syncToolButtons()
	w := s.world
	switch s.activeTool {
	case toolBuilding:
		w.PlaceBuilding(gx, gz)
		r.RebuildStaticBatch(w)
	case toolGlade:
		applyDensityBrush(w.Terrain, gx, gz, gladeRadius, -0.4)
		r.RebuildStaticBatch(w)
	case toolLiftBase:
		s.liftBase = [2]int{gx, gz}
		s.activeTool = toolLiftTop
		fmt.Printf("Lift base set at (%d,%d) — now click top\n", gx, gz)
	case toolLiftTop:
		lift := w.PlaceLift(s.liftBase[0], s.liftBase[1], gx, gz)
		r.AddLiftCable(lift, w.Terrain)
		r.RebuildStaticBatch(w)
		r.ClearAllGhosts()
		r.ClearGhostCable()
		s.activeTool = toolNone
	case toolRemove:
		s.removeAt(gx, gz, r)
	}
}

func (s *Scenario) removeAt(gx, gz int, r *render.Renderer) {
	w := s.world
	for _, b := range w.Buildings {
		if b.Pos[0] == gx && b.Pos[1] == gz {
			w.RemoveBuilding(b.ID)
			r.RebuildStaticBatch(w)
			return
		}
	}
}

func (s *Scenario) Render(r *render.Renderer) {
	const cellSize = float32(10.0)
	t := s.world.Terrain
	if s.activeTool == toolGlade && t.InBounds(s.hoverCell[0], s.hoverCell[1]) {
		gx, gz := s.hoverCell[0], s.hoverCell[1]
		center := mgl32.Vec2{float32(gx)*cellSize + cellSize/2, float32(gz)*cellSize + cellSize/2}
		r.SetBrush(center, (float32(gladeRadius)+0.5)*cellSize)
	} else {
		r.ClearBrush()
	}

	r.HighlightAgentID = s.followAgentID
	r.DrawWorld(s.world, s.time)
	r.ClearBrush()

	drawables := []render.UIDrawable{s.menuBar}
	if s.activeTool == toolLiftTop {
		drawables = append(drawables, &hintLabel{text: "Click to set lift top"})
	}
	for i, a := range s.world.Agents {
		if a.ID == s.followAgentID {
			drawables = append(drawables, &followLabel{agent: a, idx: i})
			break
		}
	}
	if s.escapeMenu.Visible() {
		drawables = append(drawables, s.escapeMenu)
	}
	r.DrawUI(drawables)
}

func (s *Scenario) Destroy() {}

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
	s.activeTool = toolNone
	s.syncToolButtons()
}

// syncToolButtons updates the active state of all tool buttons to match activeTool.
func (s *Scenario) syncToolButtons() {
	for mode, btn := range s.toolButtons {
		btn.SetActive(s.activeTool == mode ||
			(mode == toolLiftBase && s.activeTool == toolLiftTop))
	}
}

// updateLiftGhost drives the ghost preview during lift placement.
func (s *Scenario) updateLiftGhost(r *render.Renderer) {
	gx, gz := s.hoverCell[0], s.hoverCell[1]
	t := s.world.Terrain

	switch s.activeTool {
	case toolLiftBase:
		r.ClearGhostCable()
		r.SetGhosts(render.MeshTower, nil)
		if t.InBounds(gx, gz) {
			r.SetGhosts(render.MeshLiftStation, []render.StaticInstance{
				stationInstanceAt([2]int{gx, gz}, t),
			})
		} else {
			r.SetGhosts(render.MeshLiftStation, nil)
		}

	case toolLiftTop:
		if t.InBounds(gx, gz) {
			r.SetGhosts(render.MeshLiftStation, []render.StaticInstance{
				stationInstanceAt(s.liftBase, t),
				stationInstanceAt([2]int{gx, gz}, t),
			})
			r.SetGhosts(render.MeshTower, render.ComputeLiftTowerInstances(s.liftBase, [2]int{gx, gz}, t))
			r.SetGhostCable(s.liftBase, [2]int{gx, gz}, t)
		} else {
			r.ClearGhostCable()
		}

	default:
		r.ClearAllGhosts()
		r.ClearGhostCable()
	}
}

// stationInstanceAt builds a StaticInstance for a lift station at the given terrain cell.
func stationInstanceAt(cell [2]int, t *world.Terrain) render.StaticInstance {
	const cellSize = float32(10.0)
	x := float32(cell[0]) * cellSize
	z := float32(cell[1]) * cellSize
	y := t.ElevationAt(cell[0], cell[1])
	m := mgl32.Translate3D(x, y, z)
	inst := render.StaticInstance{ColorTint: [3]float32{1, 1, 1}}
	copy(inst.Transform[:], m[:])
	return inst
}

// cycleFollow advances the followed skier. First call picks a random skier;
// subsequent calls cycle forward through the agents slice.
func (s *Scenario) cycleFollow() {
	agents := s.world.Agents
	if len(agents) == 0 {
		s.followAgentID = 0
		return
	}
	if s.followAgentID == 0 {
		s.followAgentID = agents[rand.Intn(len(agents))].ID
		return
	}
	for i, a := range agents {
		if a.ID == s.followAgentID {
			s.followAgentID = agents[(i+1)%len(agents)].ID
			return
		}
	}
	// Followed agent no longer exists; wrap to first.
	s.followAgentID = agents[0].ID
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

// followLabel draws a HUD banner showing which skier the camera is following.
type followLabel struct {
	agent *world.Agent
	idx   int
}

func (f *followLabel) Draw(r *render.Renderer) {
	state := agentStateLabel(f.agent.State)
	text := fmt.Sprintf("Skier #%d  |  %s", f.idx+1, state)
	w := float32(len(text)*8 + 16)
	x := (float32(r.ScreenWidth()) - w) / 2
	r.DrawColorRect(x, 40, w, 20, mgl32.Vec4{0, 0, 0, 0.6})
	if r.Font != nil {
		r.Font.DrawText(r, text, x+8, 44, mgl32.Vec4{1, 0.95, 0.1, 1})
	}
}

func agentStateLabel(state world.AgentState) string {
	switch state {
	case world.StateWalking:
		return "Walking"
	case world.StateQueuing:
		return "Queuing"
	case world.StateRiding:
		return "On Lift"
	case world.StateSkiing:
		return "Skiing"
	case world.StateReturningToLodge:
		return "Heading Home"
	}
	return "Unknown"
}

// hintLabel draws a small text hint at the bottom of the screen.
type hintLabel struct {
	text string
}

func (h *hintLabel) Draw(r *render.Renderer) {
	r.DrawColorRect(4, float32(r.ScreenHeight())-24, float32(len(h.text)*8+8), 20, mgl32.Vec4{0, 0, 0, 0.7})
	if r.Font != nil {
		r.Font.DrawText(r, h.text, 8, float32(r.ScreenHeight())-20, mgl32.Vec4{1, 1, 0.5, 1})
	}
}
