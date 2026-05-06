package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/scene"
	"mountain-mogul/internal/sim"
)

func init() { runtime.LockOSThread() }

func main() {
	testbed := flag.String("testbed", "", "run a registered testbed headless and log debug to stdout (e.g. BuildSlope10Intermediate, or 'list')")
	simSeconds := flag.Float64("sim-seconds", 240, "max sim seconds for headless testbed runs")
	samplePeriod := flag.Float64("sample", 0.5, "sim seconds between trace rows in headless testbed runs")
	seed := flag.Int64("seed", 0, "override testbed seed (0 = use the testbed's recommended seed)")
	flag.Parse()

	if *testbed != "" {
		runHeadless(*testbed, *simSeconds, *samplePeriod, *seed)
		return
	}

	app := engine.NewApp("Mountain Mogul", 1280, 720, "assets")
	defer app.Destroy()

	app.PushScene(scene.NewStartMenu())
	app.Run()
}

func runHeadless(key string, simSeconds, samplePeriod float64, seed int64) {
	if key == "list" {
		fmt.Println("registered testbeds:")
		for _, t := range sim.Testbeds {
			fmt.Printf("  %-28s %s\n", t.Key, t.Name)
		}
		return
	}
	err := sim.RunHeadless(os.Stdout, key, sim.HeadlessOptions{
		SimSeconds:   simSeconds,
		SamplePeriod: samplePeriod,
		Seed:         seed,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
