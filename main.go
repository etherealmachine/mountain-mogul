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
	testbed := flag.String("testbed", "", "run a registered testbed headless and log debug to stdout (matches a name prefix, e.g. \"10 degree slope\", or 'list')")
	simSeconds := flag.Float64("sim-seconds", 240, "max sim seconds for headless testbed runs")
	samplePeriod := flag.Float64("sample", 0.5, "sim seconds between trace rows in headless testbed runs")
	seed := flag.Int64("seed", 0, "override testbed seed (0 = use the testbed's recommended seed)")
	runs := flag.Int("runs", 1, "number of headless iterations; >1 switches to aggregate mode (silent per-run, summary at end). Per-run seed = base seed + iteration index.")
	flag.Parse()

	if *testbed != "" {
		if *runs > 1 {
			runHeadlessAggregate(*testbed, *simSeconds, *seed, *runs)
		} else {
			runHeadless(*testbed, *simSeconds, *samplePeriod, *seed)
		}
		return
	}

	app := engine.NewApp("Mountain Mogul", 1280, 720, "assets")
	defer app.Destroy()

	app.PushScene(scene.NewStartMenu())
	app.Run()
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
