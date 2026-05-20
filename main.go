package main

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/scene"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

func init() { runtime.LockOSThread() }

func main() {
	testbed := flag.String("testbed", "", "load a registered testbed by name prefix (e.g. \"10 degree slope\", or 'list'). Default: open in the UI. Pair with -headless to drive the trace runner, or -screenshot to capture a PNG.")
	headless := flag.Bool("headless", false, "with -testbed: skip the UI and run the trace/aggregate runner that prints to stdout")
	simSeconds := flag.Float64("sim-seconds", 240, "max sim seconds for -testbed -headless runs")
	samplePeriod := flag.Float64("sample", 0.5, "sim seconds between trace rows in -testbed -headless runs")
	seed := flag.Int64("seed", 0, "override testbed seed (0 = use the testbed's recommended seed)")
	runs := flag.Int("runs", 1, "number of -headless iterations; >1 switches to aggregate mode (silent per-run, summary at end). Per-run seed = base seed + iteration index.")
	screenshot := flag.String("screenshot", "", "render a few frames and write a PNG to this path, then exit. Source: -testbed if set, else -load, else most recent save.")
	loadPath := flag.String("load", "", "save file path to use with -screenshot (default: most recent save)")
	warmupFrames := flag.Int("warmup", 30, "frames to render before capturing -screenshot (so the world settles)")
	regenForest := flag.Bool("regen-forest", false, "with -screenshot: regenerate auto-forest on the loaded terrain before capture")
	forestSeed := flag.Int64("forest-seed", 0, "with -regen-forest: seed for the regen (0 = wall-clock)")
	camTargetX := flag.Float64("camera-target-x", math.NaN(), "initial camera target X (world units) for -screenshot or -testbed UI mode. Default: terrain centre.")
	camTargetZ := flag.Float64("camera-target-z", math.NaN(), "initial camera target Z (world units) for -screenshot or -testbed UI mode. Default: terrain centre.")
	camYaw := flag.Float64("camera-yaw", math.NaN(), "initial camera yaw in degrees for -screenshot or -testbed UI mode. Default: 225.")
	camPitch := flag.Float64("camera-pitch", math.NaN(), "initial camera pitch in degrees for -screenshot or -testbed UI mode. Default: 45.")
	camZoom := flag.Float64("camera-zoom", math.NaN(), "initial camera OrthoScale (world units per half-viewport-height) for -screenshot or -testbed UI mode. Default: auto-fit terrain.")
	overlayMode := flag.Int("overlay-mode", 0, "-screenshot terrain overlay bitmask (render.Overlay*: contour=1, slope=2, snow-depth=4, grooming=8, packed=16, ice=32, mogul=64, bump-normal=128)")
	skipIntro := flag.Bool("skip-intro", false, "skip the Minty Fresh splash and jump straight to the start menu")
	profile := flag.Bool("profile", false, "run a headless 50× sim profile (representative resort, demand on) → cpu.prof + mem.prof")
	profileSeconds := flag.Float64("profile-seconds", 10, "wall-clock seconds to run with -profile")
	profileScale := flag.Float64("profile-scale", 50, "TimeScale to run with -profile")
	trace := flag.Bool("trace", false, "start a localhost pprof endpoint on :6060 for live profiling (curl http://localhost:6060/debug/pprof/profile?seconds=10 > out.prof during a stall)")
	cpuProfile := flag.String("cpuprofile", "", "write a CPU profile of the interactive session to FILE; play normally and Cmd-Q to flush. Inspect with: go tool pprof -top FILE")
	memProfile := flag.String("memprofile", "", "write a heap profile of the interactive session to FILE on exit. Inspect with: go tool pprof -alloc_space -top FILE")
	flag.Parse()

	if *trace {
		go func() {
			fmt.Fprintln(os.Stderr, "trace: pprof endpoint on http://localhost:6060/debug/pprof/")
			if err := http.ListenAndServe("localhost:6060", nil); err != nil {
				fmt.Fprintln(os.Stderr, "trace: pprof server exited:", err)
			}
		}()
	}

	if *profile {
		runProfile(*profileSeconds, *profileScale)
		return
	}

	// -screenshot takes precedence over -testbed: if both are set, the
	// testbed is rendered (not run headless) and captured to PNG. This is
	// the iterate-on-shader workflow — pick a testbed that exercises the
	// surface you care about, dial in a camera angle, and re-render after
	// every shader tweak without leaving the terminal.
	if *screenshot != "" {
		runScreenshot(screenshotOpts{
			outPath:      *screenshot,
			testbedName:  *testbed,
			loadPath:     *loadPath,
			warmupFrames: *warmupFrames,
			regenForest:  *regenForest,
			forestSeed:   *forestSeed,
			camTargetX:   *camTargetX,
			camTargetZ:   *camTargetZ,
			camYaw:       *camYaw,
			camPitch:     *camPitch,
			camZoom:      *camZoom,
			seed:         *seed,
			overlayMode:  *overlayMode,
		})
		return
	}

	if *testbed != "" {
		// `-testbed list` always prints the registry, regardless of mode —
		// it's a discoverability shortcut, not a sim run.
		if *testbed == "list" {
			fmt.Println("registered testbeds:")
			for _, t := range sim.Testbeds {
				fmt.Printf("  %s\n", t.Name)
			}
			return
		}
		if *headless {
			if *runs > 1 {
				runHeadlessAggregate(*testbed, *simSeconds, *seed, *runs)
			} else {
				runHeadless(*testbed, *simSeconds, *samplePeriod, *seed)
			}
			return
		}
		stopProfile := startInteractiveProfiles(*cpuProfile, *memProfile)
		defer stopProfile()
		runTestbedUI(*testbed, *trace, cameraOverrides{
			targetX: *camTargetX,
			targetZ: *camTargetZ,
			yaw:     *camYaw,
			pitch:   *camPitch,
			zoom:    *camZoom,
		})
		return
	}
	if *headless {
		fmt.Fprintln(os.Stderr, "-headless requires -testbed=<name>")
		os.Exit(1)
	}

	stopProfile := startInteractiveProfiles(*cpuProfile, *memProfile)
	defer stopProfile()

	app := engine.NewApp("Mountain Mogul", 1280, 720, "assets")
	defer app.Destroy()
	app.LogSlowFrames = *trace

	if *skipIntro {
		app.PushScene(scene.NewStartMenu())
	} else {
		app.PushScene(scene.NewIntroScene())
	}
	app.Run()
}

// startInteractiveProfiles begins a CPU and/or heap pprof capture for the
// lifetime of the interactive session. Returns a cleanup func that the
// caller defers — it stops the CPU profile and writes the heap profile
// after the GLFW loop returns. Either path is a no-op when its flag is
// empty so the same setup is safe to call unconditionally.
//
// Workflow: run with `-cpuprofile cpu.prof`, play normally, quit the
// window. Inspect with `go tool pprof -top cpu.prof` or
// `go tool pprof -http=:8080 cpu.prof` for the flamegraph.
func startInteractiveProfiles(cpuPath, memPath string) func() {
	var cpuFile *os.File
	if cpuPath != "" {
		f, err := os.Create(cpuPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cpuprofile: create:", err)
		} else if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintln(os.Stderr, "cpuprofile: start:", err)
			f.Close()
		} else {
			cpuFile = f
			fmt.Fprintln(os.Stderr, "cpuprofile: capturing to", cpuPath)
		}
	}
	return func() {
		if cpuFile != nil {
			pprof.StopCPUProfile()
			cpuFile.Close()
			fmt.Fprintln(os.Stderr, "cpuprofile: wrote", cpuPath)
		}
		if memPath != "" {
			f, err := os.Create(memPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "memprofile: create:", err)
				return
			}
			defer f.Close()
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				fmt.Fprintln(os.Stderr, "memprofile: write:", err)
				return
			}
			fmt.Fprintln(os.Stderr, "memprofile: wrote", memPath)
		}
	}
}

// runTestbedUI launches the regular GUI but skips the splash and start
// menu, pushing the requested testbed straight onto the scene stack so
// the user lands inside the scenario ready to play. Mirrors what the
// in-game testbed menu does on button click. Any -camera-* overrides
// set the *initial* framing — the user can pan/zoom from there as
// normal, so the flags are a starting view, not a lock.
func runTestbedUI(name string, logSlowFrames bool, cam cameraOverrides) {
	tb, err := sim.FindTestbed(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testbed:", err)
		os.Exit(1)
	}
	app := engine.NewApp("Mountain Mogul", 1280, 720, "assets")
	defer app.Destroy()
	app.LogSlowFrames = logSlowFrames
	sc := scene.NewScenarioFromTestbed(tb)
	app.PushScene(sc)
	applyCameraOverrides(app.Renderer.Camera, sc, cam, "testbed")
	app.Run()
}

// screenshotOpts bundles the configurable knobs for runScreenshot so the
// call site stays readable as we add camera-override flags.
type screenshotOpts struct {
	outPath      string
	testbedName  string // if set, build the world from the matching sim.Testbed
	loadPath     string // else load a save (empty → most recent)
	warmupFrames int
	regenForest  bool
	forestSeed   int64
	// Camera overrides — NaN means "use the auto default" so partial
	// overrides (e.g. just zoom) Just Work without clobbering the rest.
	camTargetX, camTargetZ float64
	camYaw, camPitch       float64
	camZoom                float64
	seed                   int64 // forwarded to NewSimulationWithSeed when loading a testbed
	overlayMode            int   // render.Overlay* bitmask applied before capture
}

// runScreenshot opens a window, loads either a registered testbed (when
// opt.testbedName is set) or a save file (opt.loadPath, defaulting to the
// most recent save), optionally re-runs the auto-forest generator,
// applies any camera overrides, runs the scene loop for warmupFrames, and
// writes the back buffer to opt.outPath before exiting. Used to inspect
// rendering and procgen changes from CI / agents that can't drive the
// GUI — and for shader iteration where you want the same testbed,
// camera angle, and zoom every run so only the visual changes.
func runScreenshot(opt screenshotOpts) {
	if opt.warmupFrames < 1 {
		opt.warmupFrames = 1
	}

	app := engine.NewApp("Mountain Mogul (screenshot)", 1280, 720, "assets")
	defer app.Destroy()

	var sc *scene.Scenario
	switch {
	case opt.testbedName != "":
		tb, err := sim.FindTestbed(opt.testbedName)
		if err != nil {
			fmt.Fprintln(os.Stderr, "screenshot:", err)
			os.Exit(1)
		}
		fmt.Printf("screenshot: loading testbed %q (warmup=%d frames) → %s\n",
			tb.Name, opt.warmupFrames, opt.outPath)
		sc = scene.NewScenarioFromTestbed(tb)
	default:
		path := opt.loadPath
		if path == "" {
			p, ok := save.MostRecentSave()
			if !ok {
				fmt.Fprintln(os.Stderr, "screenshot: no saves found, supply -testbed=<name> or -load=<path>")
				os.Exit(1)
			}
			path = p
		}
		fmt.Printf("screenshot: loading %s (warmup=%d frames) → %s\n",
			path, opt.warmupFrames, opt.outPath)
		sc = scene.NewScenarioFromFile(path)
	}
	app.PushScene(sc)

	if opt.regenForest {
		seed := opt.forestSeed
		if seed == 0 {
			seed = time.Now().UnixNano()
		}
		fmt.Printf("screenshot: regenerating auto-forest (seed=%d)\n", seed)
		sc.RegenForest(seed)
	}

	applyCameraOverrides(app.Renderer.Camera, sc, cameraOverrides{
		targetX: opt.camTargetX,
		targetZ: opt.camTargetZ,
		yaw:     opt.camYaw,
		pitch:   opt.camPitch,
		zoom:    opt.camZoom,
	}, "screenshot")

	if opt.overlayMode != 0 {
		sc.SetOverlay(opt.overlayMode)
		fmt.Printf("screenshot: terrain overlay mask=%d\n", opt.overlayMode)
	}

	// Run a manual scene loop. Mirrors engine.App.Run but exits after
	// warmup, capturing the back buffer between Render and SwapBuffers so
	// the PNG matches exactly what the user would see on screen. Timing
	// the steady-state portion (after a few warmup frames the GPU pipeline
	// fills) gives a rough FPS reading we can report.
	const benchSkip = 3
	prev := time.Now()
	var benchStart time.Time
	for frame := 0; frame < opt.warmupFrames; frame++ {
		now := time.Now()
		dt := now.Sub(prev).Seconds()
		prev = now

		if frame == benchSkip {
			benchStart = now
		}

		app.Input.BeginFrame()
		glfw.PollEvents()

		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		sc.Update(dt)
		sc.Render(app.Renderer)

		if frame == opt.warmupFrames-1 {
			if err := app.Renderer.SaveScreenshot(opt.outPath); err != nil {
				fmt.Fprintln(os.Stderr, "screenshot: write failed:", err)
				os.Exit(1)
			}
			fmt.Println("screenshot: wrote", opt.outPath)
		}

		app.Window.SwapBuffers()
	}
	if opt.warmupFrames > benchSkip {
		elapsed := time.Since(benchStart).Seconds()
		measured := opt.warmupFrames - benchSkip
		fps := float64(measured) / elapsed
		fmt.Printf("screenshot: %d frames in %.3fs = %.1f fps (%.2f ms/frame)\n",
			measured, elapsed, fps, 1000.0/fps)
	}
}

// cameraOverrides bundles the `-camera-*` CLI flags. Each field uses NaN
// as a sentinel for "unset — fall back to the auto-fit default" so a
// partial override (e.g. only zoom) doesn't force the caller to also
// supply pitch/yaw/target.
type cameraOverrides struct {
	targetX, targetZ float64
	yaw, pitch       float64
	zoom             float64
}

// applyCameraOverrides centres + auto-zooms the orthographic camera to
// frame the whole map, then overlays any non-NaN flag values. PushScene
// has already run Init → installWorld so the renderer knows the terrain
// dims; we read them back through the scenario accessor. Shared between
// the screenshot path and the -testbed UI path so the same -camera-*
// flags work for both modes.
func applyCameraOverrides(cam *render.Camera, sc *scene.Scenario, ov cameraOverrides, logPrefix string) {
	const cellSize = float32(5.0)
	w, h := sc.TerrainSize()

	// Auto-fit defaults — terrain centre, current pitch/yaw, zoom sized
	// to the longer terrain axis so the whole map fits on-screen.
	target := mgl32.Vec3{
		float32(w) * cellSize * 0.5,
		0,
		float32(h) * cellSize * 0.5,
	}
	dim := float32(w)
	if h > w {
		dim = float32(h)
	}
	zoom := dim * cellSize * 0.55
	yaw := cam.Yaw
	pitch := cam.Pitch

	if !math.IsNaN(ov.targetX) {
		target[0] = float32(ov.targetX)
	}
	if !math.IsNaN(ov.targetZ) {
		target[2] = float32(ov.targetZ)
	}
	if !math.IsNaN(ov.yaw) {
		yaw = float32(ov.yaw)
	}
	if !math.IsNaN(ov.pitch) {
		pitch = float32(ov.pitch)
	}
	if !math.IsNaN(ov.zoom) && ov.zoom > 0 {
		zoom = float32(ov.zoom)
	}

	cam.Target = target
	cam.Yaw = yaw
	cam.Pitch = pitch
	cam.OrthoScale = zoom
	cam.Recalculate()

	fmt.Printf("%s: camera target=(%.1f,%.1f,%.1f) yaw=%.1f pitch=%.1f zoom=%.1f\n",
		logPrefix, target[0], target[1], target[2], yaw, pitch, zoom)
}

// runProfile builds a representative two-lift resort with a parking lot
// and drives the simulation at the given TimeScale for wallSeconds real
// seconds, capturing a CPU profile to cpu.prof and a heap profile to
// mem.prof. Used to diagnose where the simulation spends time / allocs
// at fast-forward speeds where the rendering path is out of the loop.
func runProfile(wallSeconds, scale float64) {
	// Build a small but realistic resort: 80×60 cells (~400 m × 300 m),
	// 10° slope, one parking lot at the base, two lifts side-by-side
	// rising to the top. Both advertise green + blue so all three skill
	// groups have terrainMatch = 1 and the demand poll fires for each.
	const cellSize = 5.0
	w, h := 80, 60
	terrain := world.NewTerrain(w, h)
	for x := 0; x < w; x++ {
		for z := 0; z < h; z++ {
			// Slope rises with z (top of map = top of mountain).
			elev := float32(h-z) * cellSize * 0.176 // tan(10°) ≈ 0.176
			terrain.Cells[x][z].GroundElevation = elev
		}
	}
	wld := world.NewWorld(terrain)

	// Parking lot at the base (z near max), two lifts running up-slope.
	lotX, lotZ := float32(w/2)*cellSize, float32(h-3)*cellSize
	lot := wld.PlaceBuildingType(world.BuildingParking, lotX, lotZ)
	wld.EnsureParkingDriveway(lot)

	wld.PlaceLift(world.LiftDouble,
		float32(w/2-15)*cellSize, float32(h-5)*cellSize,
		float32(w/2-15)*cellSize, float32(5)*cellSize)
	wld.PlaceLift(world.LiftDouble,
		float32(w/2+15)*cellSize, float32(h-5)*cellSize,
		float32(w/2+15)*cellSize, float32(5)*cellSize)

	// Pathfind groomed corridors so skiers don't pile up at the lots.
	// Simpler: leave terrain bare; the L1 controller handles it.

	// Seed a default 10k catchment so the per-Guest demand poll has
	// someone to draw from. Fixed seed for reproducible profile runs.
	world.SeedGuests(wld, 1, world.DefaultGuestPoolSize)
	wld.History = world.NewHistory()

	s := sim.NewSimulationWithSeed(wld, 1)
	s.TimeScale = scale

	// CPU profile — captures the busy portion of the run.
	cpuFile, err := os.Create("cpu.prof")
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile: create cpu.prof:", err)
		os.Exit(1)
	}
	defer cpuFile.Close()
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		fmt.Fprintln(os.Stderr, "profile: start cpu:", err)
		os.Exit(1)
	}
	defer pprof.StopCPUProfile()

	// Memstats before — to measure cumulative allocations over the run.
	var msStart, msEnd runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&msStart)

	const frameDt = 1.0 / 60.0
	frames := int(wallSeconds / frameDt)
	fmt.Printf("profile: resort=2 lifts + 1 lot, scale=%.0f×, frames=%d (%.1fs wall, %.1fs sim)\n",
		scale, frames, wallSeconds, wallSeconds*scale)

	tStart := time.Now()
	for i := 0; i < frames; i++ {
		s.Tick(frameDt)
	}
	tElapsed := time.Since(tStart)

	runtime.ReadMemStats(&msEnd)

	// Heap profile at end of run.
	memFile, err := os.Create("mem.prof")
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile: create mem.prof:", err)
		os.Exit(1)
	}
	defer memFile.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(memFile); err != nil {
		fmt.Fprintln(os.Stderr, "profile: write heap:", err)
		os.Exit(1)
	}

	// Summary stats.
	activeGuests := len(wld.OnMountain)
	totalAllocBytes := msEnd.TotalAlloc - msStart.TotalAlloc
	totalAllocMB := float64(totalAllocBytes) / (1024 * 1024)
	gcCount := msEnd.NumGC - msStart.NumGC
	heapMB := float64(msEnd.HeapAlloc) / (1024 * 1024)

	fmt.Printf("profile: ran in %.2fs wall (%.0fµs/frame)\n",
		tElapsed.Seconds(), float64(tElapsed.Microseconds())/float64(frames))
	fmt.Printf("profile: active agents at end=%d, resort rating=%.3f\n",
		activeGuests, s.Demand.ResortRating)
	// Report catchment composition: who's currently dormant by skill bucket.
	var atHomeB, atHomeI, atHomeA int
	for _, g := range wld.Guests {
		if g.State != world.AtHome {
			continue
		}
		switch {
		case g.Traits.Skill < ai.SkillIntermediateThreshold:
			atHomeB++
		case g.Traits.Skill < ai.SkillAdvancedThreshold:
			atHomeI++
		default:
			atHomeA++
		}
	}
	fmt.Printf("profile: at-home now=B%d I%d A%d (pool=%d)\n",
		atHomeB, atHomeI, atHomeA, len(wld.Guests))
	if wld.History != nil {
		fmt.Printf("profile: history samples=%d (cap=%d) arrivals-in-progress=%d departures-in-progress=%d\n",
			wld.History.Len(), world.HistoryCapacity, wld.History.ArrivalsToday, wld.History.DeparturesToday)
	}
	fmt.Printf("profile: alloc=%.1f MB over run, %d GC cycles, heap=%.1f MB at end\n",
		totalAllocMB, gcCount, heapMB)
	fmt.Println("profile: wrote cpu.prof, mem.prof")
	fmt.Println("profile: inspect with:  go tool pprof -top cpu.prof")
	fmt.Println("profile:                 go tool pprof -alloc_space -top mem.prof")
}

func runHeadless(name string, simSeconds, samplePeriod float64, seed int64) {
	if name == "list" {
		fmt.Println("registered testbeds:")
		for _, t := range sim.Testbeds {
			fmt.Printf("  %s\n", t.Name)
		}
		return
	}
	err := sim.RunHeadless(os.Stdout, name, sim.HeadlessOptions{
		SimSeconds:   simSeconds,
		SamplePeriod: samplePeriod,
		Seed:         seed,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runHeadlessAggregate(name string, simSeconds float64, seed int64, runs int) {
	if name == "list" {
		fmt.Println("registered testbeds:")
		for _, t := range sim.Testbeds {
			fmt.Printf("  %s\n", t.Name)
		}
		return
	}
	err := sim.RunHeadlessAggregate(os.Stdout, name, sim.AggregateOptions{
		SimSeconds: simSeconds,
		Seed:       seed,
		Runs:       runs,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
