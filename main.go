package main

import (
	"flag"
	"fmt"
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
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/scene"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

func init() { runtime.LockOSThread() }

func main() {
	testbed := flag.String("testbed", "", "run a registered testbed headless and log debug to stdout (matches a name prefix, e.g. \"10 degree slope\", or 'list')")
	simSeconds := flag.Float64("sim-seconds", 240, "max sim seconds for headless testbed runs")
	samplePeriod := flag.Float64("sample", 0.5, "sim seconds between trace rows in headless testbed runs")
	seed := flag.Int64("seed", 0, "override testbed seed (0 = use the testbed's recommended seed)")
	runs := flag.Int("runs", 1, "number of headless iterations; >1 switches to aggregate mode (silent per-run, summary at end). Per-run seed = base seed + iteration index.")
	screenshot := flag.String("screenshot", "", "load a save, render a few frames, write a PNG to this path, then exit")
	loadPath := flag.String("load", "", "save file path to use with -screenshot (default: most recent save)")
	warmupFrames := flag.Int("warmup", 30, "frames to render before capturing -screenshot (so the world settles)")
	regenForest := flag.Bool("regen-forest", false, "with -screenshot: regenerate auto-forest on the loaded terrain before capture")
	forestSeed := flag.Int64("forest-seed", 0, "with -regen-forest: seed for the regen (0 = wall-clock)")
	skipIntro := flag.Bool("skip-intro", false, "skip the Minty Fresh splash and jump straight to the start menu")
	profile := flag.Bool("profile", false, "run a headless 50× sim profile (representative resort, demand on) → cpu.prof + mem.prof")
	profileSeconds := flag.Float64("profile-seconds", 10, "wall-clock seconds to run with -profile")
	profileScale := flag.Float64("profile-scale", 50, "TimeScale to run with -profile")
	trace := flag.Bool("trace", false, "start a localhost pprof endpoint on :6060 for live profiling (curl http://localhost:6060/debug/pprof/profile?seconds=10 > out.prof during a stall)")
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

	if *testbed != "" {
		if *runs > 1 {
			runHeadlessAggregate(*testbed, *simSeconds, *seed, *runs)
		} else {
			runHeadless(*testbed, *simSeconds, *samplePeriod, *seed)
		}
		return
	}

	if *screenshot != "" {
		runScreenshot(*loadPath, *screenshot, *warmupFrames, *regenForest, *forestSeed)
		return
	}

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

// runScreenshot opens a window, loads the given save (or the most recent one
// when path is empty), optionally re-runs the auto-forest generator on its
// terrain, runs the scene loop for warmupFrames, captures the back buffer
// to outPath, and exits. Used to inspect rendering and procgen changes from
// CI / agents that can't drive the GUI.
func runScreenshot(loadPath, outPath string, warmupFrames int, regenForest bool, forestSeed int64) {
	if loadPath == "" {
		p, ok := save.MostRecentSave()
		if !ok {
			fmt.Fprintln(os.Stderr, "screenshot: no saves found, supply -load=<path>")
			os.Exit(1)
		}
		loadPath = p
	}
	if warmupFrames < 1 {
		warmupFrames = 1
	}
	fmt.Printf("screenshot: loading %s (warmup=%d frames) → %s\n",
		loadPath, warmupFrames, outPath)

	app := engine.NewApp("Mountain Mogul (screenshot)", 1280, 720, "assets")
	defer app.Destroy()

	sc := scene.NewScenarioFromFile(loadPath)
	app.PushScene(sc)

	if regenForest {
		seed := forestSeed
		if seed == 0 {
			seed = time.Now().UnixNano()
		}
		fmt.Printf("screenshot: regenerating auto-forest (seed=%d)\n", seed)
		sc.RegenForest(seed)
	}

	// Centre the orthographic camera on the map and zoom out far enough to
	// frame the whole thing. PushScene already ran Init → installWorld so
	// the renderer knows the terrain dims; we read them back through the
	// scenario's accessor.
	w, h := sc.TerrainSize()
	const cellSize = float32(5.0)
	app.Renderer.Camera.Target = mgl32.Vec3{
		float32(w) * cellSize * 0.5,
		0,
		float32(h) * cellSize * 0.5,
	}
	dim := float32(w)
	if h > w {
		dim = float32(h)
	}
	app.Renderer.Camera.OrthoScale = dim * cellSize * 0.55
	app.Renderer.Camera.Recalculate()

	// Run a manual scene loop. Mirrors engine.App.Run but exits after
	// warmup, capturing the back buffer between Render and SwapBuffers so
	// the PNG matches exactly what the user would see on screen. Timing
	// the steady-state portion (after a few warmup frames the GPU pipeline
	// fills) gives a rough FPS reading we can report.
	const benchSkip = 3
	prev := time.Now()
	var benchStart time.Time
	for frame := 0; frame < warmupFrames; frame++ {
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

		if frame == warmupFrames-1 {
			if err := app.Renderer.SaveScreenshot(outPath); err != nil {
				fmt.Fprintln(os.Stderr, "screenshot: write failed:", err)
				os.Exit(1)
			}
			fmt.Println("screenshot: wrote", outPath)
		}

		app.Window.SwapBuffers()
	}
	if warmupFrames > benchSkip {
		elapsed := time.Since(benchStart).Seconds()
		measured := warmupFrames - benchSkip
		fps := float64(measured) / elapsed
		fmt.Printf("screenshot: %d frames in %.3fs = %.1f fps (%.2f ms/frame)\n",
			measured, elapsed, fps, 1000.0/fps)
	}
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
			terrain.Cells[x][z].NaturalElev = elev
		}
	}
	wld := world.NewWorld(terrain)

	// Parking lot at the base (z near max), two lifts running up-slope.
	lotX, lotZ := float32(w/2)*cellSize, float32(h-3)*cellSize
	lot := wld.PlaceBuildingType(world.BuildingParking, lotX, lotZ)
	wld.EnsureParkingDriveway(lot)

	liftA := wld.PlaceLift(world.LiftDouble,
		float32(w/2-15)*cellSize, float32(h-5)*cellSize,
		float32(w/2-15)*cellSize, float32(5)*cellSize)
	liftA.Services = world.DiffGreen | world.DiffBlue
	liftB := wld.PlaceLift(world.LiftDouble,
		float32(w/2+15)*cellSize, float32(h-5)*cellSize,
		float32(w/2+15)*cellSize, float32(5)*cellSize)
	liftB.Services = world.DiffGreen | world.DiffBlue

	// Pathfind groomed corridors so skiers don't pile up at the lots.
	// Simpler: leave terrain bare; the L1 controller handles it.

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
	activeAgents := len(wld.Agents)
	totalAllocBytes := msEnd.TotalAlloc - msStart.TotalAlloc
	totalAllocMB := float64(totalAllocBytes) / (1024 * 1024)
	gcCount := msEnd.NumGC - msStart.NumGC
	heapMB := float64(msEnd.HeapAlloc) / (1024 * 1024)

	fmt.Printf("profile: ran in %.2fs wall (%.0fµs/frame)\n",
		tElapsed.Seconds(), float64(tElapsed.Microseconds())/float64(frames))
	fmt.Printf("profile: active agents at end=%d, resort rating=%.3f\n",
		activeAgents, s.Demand.ResortRating)
	fmt.Printf("profile: pool now=B%d I%d A%d (started 6000/3000/1000)\n",
		s.Demand.Groups[ai.SkillBeginner].Pool,
		s.Demand.Groups[ai.SkillIntermediate].Pool,
		s.Demand.Groups[ai.SkillAdvanced].Pool)
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
