//go:build !demo

package scene

import (
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	xdraw "golang.org/x/image/draw"

	"github.com/go-gl/glfw/v3.3/glfw"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

// scenarioDev is a Scenario's developer state in the full game: the debug
// console, the F3–F5 inspectors, the skier CSV log, GIF capture, and the
// testbed and live-query hooks. The demo's is empty (scenario_demo.go).
type scenarioDev struct {
	debugConsole    *DebugConsole
	debugSteering   bool // F3: render steering forces on the followed skier
	debugPlanner    bool // F4: show goal weights, full plan, snapshot anchors for the followed skier
	debugTerrainIns bool // F5: dump cell snow/terrain state under the mouse cursor

	csvRecorder *sim.CSVRecorder // L: CSV log of the followed skier

	// GIF capture state. gifRecording is true while recording; gifFrames
	// accumulates paletted frames; gifFrameCount counts rendered frames so we
	// can subsample (capture every gifFrameInterval-th frame).
	// gifResult receives a single message from the background encode goroutine.
	gifRecording  bool
	gifFrames     []*image.Paletted
	gifFrameCount int
	gifResult     chan string

	rebuild     func(seed int64) *world.World // non-nil ⇒ "New Seed" button shown
	tickHook    func(s *sim.Simulation)       // optional testbed hook; called each frame before Tick
	queryServer *sim.QueryServer              // non-nil when -live-query is set
}

// NewScenarioFromTestbed creates a Scenario from a sim.Testbed and remembers
// how to rebuild the world from a seed, so the menu bar can offer a
// "New Seed" button that re-rolls the run without leaving the scene.
func NewScenarioFromTestbed(tb *sim.Testbed) *Scenario {
	rebuild := func(seed int64) *world.World {
		return tb.NewWorld()
	}
	return &Scenario{
		prebuiltWorld: rebuild(tb.Seed),
		simSeed:       tb.Seed,
		scenarioDev: scenarioDev{
			rebuild:  rebuild,
			tickHook: tb.TickHook,
		},
	}
}

// globalQueryServer, if set, is attached to every Scenario on installWorld.
var globalQueryServer *sim.QueryServer

// UseQueryServer registers a query server to be wired into every Scenario
// that installs a world, regardless of how it was created. Call once from
// main before app.Run().
func UseQueryServer(qs *sim.QueryServer) { globalQueryServer = qs }

// SetQueryServer attaches a live-query server to this specific scenario.
// Overrides the global server for this instance.
func (s *Scenario) SetQueryServer(qs *sim.QueryServer) { s.queryServer = qs }

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

// initDev sets up the debug console and, for a testbed, the New Seed
// button. Called from Init once the tool bar exists.
func (s *Scenario) initDev() {
	s.debugConsole = newDebugConsole(s.world, s.setToast)
	r := s.app.Renderer
	s.debugConsole.SetSim(s.sim, func() { r.FlushTerrainVerts(s.world.Terrain) }, func() { r.RebuildStaticBatch(s.world) })
	if s.rebuild != nil {
		s.toolBar.AddButton("New Seed", func() {
			s.restartWithSeed(rand.Int63n(1_000_000))
		})
	}
}

// installDev points the live-query server and debug console at a newly
// installed simulation.
func (s *Scenario) installDev() {
	if s.queryServer == nil {
		s.queryServer = globalQueryServer
	}
	if s.debugConsole != nil {
		r := s.app.Renderer
		s.debugConsole.SetSim(s.sim, func() { r.FlushTerrainVerts(s.world.Terrain) }, func() { r.RebuildStaticBatch(s.world) })
	}
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
	s.setFollowGuest(0)
	s.app.Renderer.SetDebugLines(nil)
	s.syncSpeedButtons()
	s.setToast(fmt.Sprintf("Restarted with seed=%d", seed))
}

// devTick runs the testbed's hook before each sim tick.
func (s *Scenario) devTick() {
	if s.tickHook != nil {
		s.tickHook(s.sim)
	}
}

// devServeQueries answers pending live SQL queries against this frame's state.
func (s *Scenario) devServeQueries() {
	if s.sim != nil && s.queryServer != nil {
		s.queryServer.Tick(s.world, s.sim)
	}
}

// devConsoleInput toggles the debug console on ~ and, while it's open,
// gives it the keyboard. Reports whether it took the input.
func (s *Scenario) devConsoleInput(inp *engine.Input) bool {
	if inp.Pressed[glfw.KeyGraveAccent] {
		s.debugConsole.Toggle()
		inp.CharInput = inp.CharInput[:0] // swallow the ~ so it isn't typed into the field
	}
	if s.debugConsole.Visible() {
		s.debugConsole.HandleInput(inp)
		return true
	}
	return false
}

// devHotkeys handles the debug overlays and instrumentation keys.
func (s *Scenario) devHotkeys(inp *engine.Input, r *render.Renderer, typing bool) {
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

	// F6: colour guests by what they're doing (walking, queuing, on a
	// lift, ...) over their outfits.
	if inp.Pressed[glfw.KeyF6] {
		r.ActivityTint = !r.ActivityTint
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

	// Shift+F12: toggle GIF recording (F12 alone is a screenshot).
	if inp.Pressed[glfw.KeyF12] && (inp.Held[glfw.KeyLeftShift] || inp.Held[glfw.KeyRightShift]) {
		s.pendingScreenshot = false
		s.toggleGIFRecording()
	}

	// Auto-stop CSV log if the followed skier changed or no longer exists.
	if s.csvRecorder != nil {
		a := s.findFollowedGuest()
		if a == nil || a.ID != s.csvRecorder.GuestID() {
			s.stopSkierLog("Logging stopped: skier no longer followed")
		}
	}
}

// devSteeringLines adds the F3 steering forces of the followed skier a
// (nil when nobody is followed) to lines.
func (s *Scenario) devSteeringLines(a *world.Guest, lines []render.DebugLine) []render.DebugLine {
	if s.debugSteering && a != nil && a.TargetID != 0 && !a.Fallen && a.OnLiftID == 0 && !a.Queued {
		if target, ok := skierTarget(s.world, a); ok {
			lines = append(lines, steeringLines(s.world, a, target)...)
		}
	}
	return lines
}

// devPlannerPanel adds the F4 planner panel for the followed guest a.
func (s *Scenario) devPlannerPanel(drawables []render.UIDrawable, a *world.Guest) []render.UIDrawable {
	if s.debugPlanner {
		drawables = append(drawables, &plannerDebugPanel{
			world: s.world,
			agent: a,
		})
	}
	return drawables
}

// devInspector adds the F5 terrain inspector for the cell under the mouse.
func (s *Scenario) devInspector(drawables []render.UIDrawable) []render.UIDrawable {
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
	return drawables
}

// devConsoleDrawable adds the debug console when it's open.
func (s *Scenario) devConsoleDrawable(drawables []render.UIDrawable) []render.UIDrawable {
	if s.debugConsole.Visible() {
		drawables = append(drawables, s.debugConsole)
	}
	return drawables
}

const (
	gifFrameInterval = 3   // capture every Nth rendered frame (~20fps at 60fps)
	gifMaxFrames     = 600 // hard cap: ~30s at 20fps before auto-stop
	gifCentiseconds  = 5   // delay per frame in 1/100s (gifFrameInterval/60fps ≈ 5cs)
)

// devCaptureFrame grabs a GIF frame while recording. It runs before the
// UI is drawn so toasts and overlays don't appear in recorded frames, and
// subsamples every gifFrameInterval rendered frames.
func (s *Scenario) devCaptureFrame(r *render.Renderer) {
	if !s.gifRecording {
		return
	}
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

// devAfterUI drains the result from the background GIF encode goroutine.
func (s *Scenario) devAfterUI() {
	if s.gifResult != nil {
		select {
		case msg := <-s.gifResult:
			s.setToast(msg)
			s.gifResult = nil
		default:
		}
	}
}

// toggleGIFRecording starts or stops a GIF capture session.
func (s *Scenario) toggleGIFRecording() {
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
