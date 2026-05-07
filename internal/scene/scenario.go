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
	debugSteering   bool   // F3: render steering forces on the followed skier
	paused          bool
	pauseBtn        *ui.Button
	speedBtns       []*ui.Button // index aligned with speedOptions
	popup           *ui.Window
	saveAllowed     bool   // false in testbed mode; gates the Save prompt
	saveName        string // last name used for Save; pre-fills the prompt next time
	savePrompt      *savePrompt
	prebuiltWorld   *world.World
	simSeed         int64                          // 0 = wall-clock; nonzero forces deterministic RNG
	rebuild         func(seed int64) *world.World // non-nil ⇒ "New Seed" button shown

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

// speedOptions lists the time-scale presets shown in the menu bar.
var speedOptions = []float64{1, 5, 10}

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
	if s.prebuiltWorld != nil {
		w = s.prebuiltWorld
	} else {
		loaded, err := save.LoadScenario(s.scenarioPath)
		if err != nil {
			// Fall back to a blank world
			fmt.Printf("Scenario load error (%v), creating blank world\n", err)
			t := world.NewTerrain(32, 32)
			loaded = world.NewWorld(t)
		}
		w = loaded
	}
	s.installWorld(w)

	if s.saveAllowed {
		s.escapeMenu = NewEscapeMenu(app, s.openSavePrompt, s.gotoLoadMenu)
	} else {
		s.escapeMenu = NewEscapeMenu(app, nil, nil)
	}

	// Build menu bar
	s.toolButtons = make(map[toolMode]*ui.Button)
	s.menuBar = ui.NewMenuBar(0, render.GlyphH+10)
	s.toolButtons[toolBuilding] = s.menuBar.AddButton("Build Lodge", func() { s.setTool(toolBuilding) })
	s.toolButtons[toolLiftBase] = s.menuBar.AddButton("Place Lift", func() { s.setTool(toolLiftBase) })
	s.toolButtons[toolGlade] = s.menuBar.AddButton("Glade", func() { s.setTool(toolGlade) })
	s.toolButtons[toolRemove] = s.menuBar.AddButton("Remove", func() { s.setTool(toolRemove) })
	if s.rebuild != nil {
		s.menuBar.AddButton("New Seed", func() {
			s.restartWithSeed(rand.Int63n(1_000_000))
		})
	}

	// Speed / pause controls — right-aligned. The menu bar packs right-buttons
	// right-to-left in insertion order, so insert Pause first to make it the
	// leftmost of the cluster: [Pause] [1x] [5x] [10x] [right edge].
	s.pauseBtn = s.menuBar.AddRightButton("Pause", func() {
		s.paused = !s.paused
		s.syncSpeedButtons()
	})
	s.speedBtns = make([]*ui.Button, len(speedOptions))
	for i, mult := range speedOptions {
		mult := mult
		idx := i
		label := fmt.Sprintf("%.0fx", mult)
		s.speedBtns[idx] = s.menuBar.AddRightButton(label, func() {
			s.sim.TimeScale = mult
			s.paused = false
			s.syncSpeedButtons()
		})
	}
	s.syncSpeedButtons()

	return nil
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
	path, err := save.SaveAs(clean, s.world)
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

// syncSpeedButtons highlights the active speed/pause button.
func (s *Scenario) syncSpeedButtons() {
	for i, btn := range s.speedBtns {
		btn.SetActive(!s.paused && s.sim.TimeScale == speedOptions[i])
	}
	if s.pauseBtn != nil {
		s.pauseBtn.SetActive(s.paused)
	}
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

	// C: toggle slope + contour overlay.
	if inp.Pressed[glfw.KeyC] {
		if r.TerrainOverlayMode == 0 {
			r.TerrainOverlayMode = 1
		} else {
			r.TerrainOverlayMode = 0
		}
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
		r.Camera.Yaw += rotDelta
		r.Camera.Recalculate()
	}

	// Menu bar input
	s.menuBar.HandleInput(inp, float32(r.ScreenWidth()), float32(r.ScreenHeight()))

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
	if !clickConsumed && inp.LeftClick && s.activeTool == toolNone && !s.menuBar.ContainsY(inp.MousePos[1]) {
		// Skier pick takes priority over popups when no tool is active.
		if a := s.pickAgent(r.Camera, inp.MousePos); a != nil {
			s.followAgentID = a.ID
			if s.popup != nil {
				s.popup.Visible = false
			}
			clickConsumed = true
		}
	}
	if !clickConsumed {
		clickOrHeld := inp.LeftClick || (inp.LeftHeld && s.activeTool == toolGlade)
		if clickOrHeld && !s.menuBar.ContainsY(inp.MousePos[1]) {
			gx, gz := s.hoverCell[0], s.hoverCell[1]
			if s.world.Terrain.InBounds(gx, gz) {
				if s.activeTool == toolNone && inp.LeftClick {
					s.tryOpenPopup(gx, gz, r.ScreenWidth(), r.ScreenHeight())
				} else {
					s.applyTool(gx, gz, r)
				}
			}
		}
	}

	// Tick simulation (skipped while paused).
	if !s.paused {
		s.sim.Tick(dt)
	}

	// Camera follow: track the selected agent using the freshest positions.
	if s.followAgentID != 0 {
		if agent := s.findFollowedAgent(); agent != nil {
			r.Camera.Target = agent.Pos
			r.Camera.Recalculate()
		} else {
			s.followAgentID = 0
		}
	}

	// Track + optional steering overlay for the followed skier.
	s.updateOverlay(r)
}

// updateOverlay maintains the followed skier's track and (optionally) the
// steering-debug visualisation, then pushes both to the renderer in a single
// SetDebugLines batch. The track resets whenever the followed skier changes
// or follow is disabled.
func (s *Scenario) updateOverlay(r *render.Renderer) {
	a := s.findFollowedAgent()

	// Reset the track when follow target changes (including dropped follow).
	if a == nil || a.ID != s.trackOwner {
		s.track = s.track[:0]
		s.trackOwner = 0
	}
	if a == nil {
		r.SetDebugLines(nil)
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

	r.SetDebugLines(lines)
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
		length := float32(sim.ProbeDistance) * (0.4 + 0.6*p.Density)
		shade := 0.4 + 0.6*p.Density
		lines = append(lines, mk(p.Dir, length, [3]float32{shade, 0.15, 0.15}))
	}
	return lines
}

// skierTarget returns the world-space target the agent is currently
// steering toward, resolved from the agent's TargetID against either the
// world's lifts or buildings.
func skierTarget(w *world.World, a *world.Agent) (mgl32.Vec3, bool) {
	const cellSize = float32(10.0)
	if a.TargetID == 0 {
		return mgl32.Vec3{}, false
	}
	for _, lift := range w.Lifts {
		if lift.ID == a.TargetID {
			return mgl32.Vec3{
				float32(lift.Base[0]) * cellSize,
				w.Terrain.ElevationAt(lift.Base[0], lift.Base[1]),
				float32(lift.Base[1]) * cellSize,
			}, true
		}
	}
	for _, b := range w.Buildings {
		if b.ID == a.TargetID {
			return mgl32.Vec3{
				float32(b.Pos[0]) * cellSize,
				w.Terrain.ElevationAt(b.Pos[0], b.Pos[1]),
				float32(b.Pos[1]) * cellSize,
			}, true
		}
	}
	return mgl32.Vec3{}, false
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
	if s.simSeed != 0 {
		drawables = append(drawables, &seedLabel{seed: s.simSeed})
	}
	if s.activeTool == toolLiftTop {
		drawables = append(drawables, &hintLabel{text: "Click to set lift top"})
	}
	for i, a := range s.world.Agents {
		if a.ID == s.followAgentID {
			drawables = append(drawables, &followLabel{world: s.world, agent: a, idx: i})
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

// tryOpenPopup opens a popup window if a building or lift is at the clicked cell.
func (s *Scenario) tryOpenPopup(gx, gz, screenW, screenH int) {
	// Check buildings first.
	for _, b := range s.world.Buildings {
		if b.Pos[0] == gx && b.Pos[1] == gz {
			s.openBuildingPopup(b, screenW, screenH)
			return
		}
	}
	// Check lift bases (within 1 cell).
	for _, lift := range s.world.Lifts {
		dx := lift.Base[0] - gx
		dz := lift.Base[1] - gz
		if dx*dx+dz*dz <= 1 {
			s.openLiftPopup(lift, screenW, screenH)
			return
		}
	}
	// Nothing hit — close any open popup.
	if s.popup != nil {
		s.popup.Visible = false
	}
}

func (s *Scenario) openBuildingPopup(b *world.Building, screenW, screenH int) {
	bldg := b
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
	world *world.World
	agent *world.Agent
	idx   int
}

func (f *followLabel) Draw(r *render.Renderer) {
	activity := world.Activity(f.world, f.agent)
	row1 := fmt.Sprintf("Skier #%d  |  %s", f.idx+1, activity)
	row2 := fmt.Sprintf("%.1f m/s", f.agent.Speed)
	const padX = 10
	const padY = 4
	const lineGap = 2
	maxLen := len(row1)
	if len(row2) > maxLen {
		maxLen = len(row2)
	}
	w := float32(maxLen*render.GlyphAdvance + 2*padX)
	boxH := float32(render.GlyphH*2 + lineGap + 2*padY)
	x := (float32(r.ScreenWidth()) - w) / 2
	y := float32(40)
	r.DrawColorRect(x, y, w, boxH, mgl32.Vec4{0, 0, 0, 0.6})
	if r.Font != nil {
		row1X := x + (w-float32(len(row1)*render.GlyphAdvance))/2
		row2X := x + (w-float32(len(row2)*render.GlyphAdvance))/2
		r.Font.DrawText(r, row1, row1X, y+padY, mgl32.Vec4{1, 0.95, 0.1, 1})
		r.Font.DrawText(r, row2, row2X, y+padY+float32(render.GlyphH+lineGap), mgl32.Vec4{1, 0.95, 0.1, 1})
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
	y := float32(render.GlyphH + 14) // just below the menu bar
	textY := y + (boxH-float32(render.GlyphH))/2
	r.DrawColorRect(x, y, boxW, boxH, mgl32.Vec4{0, 0, 0, 0.6})
	if r.Font != nil {
		r.Font.DrawText(r, text, x+8, textY, mgl32.Vec4{0.85, 0.85, 0.85, 1})
	}
}

// hintLabel draws a small text hint at the bottom of the screen.
type hintLabel struct {
	text string
}

func (h *hintLabel) Draw(r *render.Renderer) {
	const boxH = float32(render.GlyphH + 8)
	boxW := float32(len(h.text)*render.GlyphAdvance + 16)
	y := float32(r.ScreenHeight()) - boxH - 4
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
	boxW := float32(len(t.text)*render.GlyphAdvance + 20)
	x := (float32(r.ScreenWidth()) - boxW) / 2
	y := float32(r.ScreenHeight()) - boxH - 30
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
