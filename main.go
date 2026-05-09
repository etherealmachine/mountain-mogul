package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/scene"
	"mountain-mogul/internal/sim"
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
	flag.Parse()

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

	app.PushScene(scene.NewStartMenu())
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
