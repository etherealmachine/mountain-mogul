//go:build !demo

// menuscene bakes the main menu's backdrop (scene/menu_backdrop.go): the
// built-out Boreal resort (the Goals Test layout), opened and run headless
// to mid-morning of its second day, so the menu opens on a busy mountain
// with skiers on every lift and run, groomed corduroy, and tracks in it.
//
//	go run ./tools/menuscene ["assets/scenarios/Boreal (Goals Test).save"] [assets/menu.save]
package main

import (
	"fmt"
	"os"

	"mountain-mogul/internal/save"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

const (
	bakeDay  = 1    // the second day, so yesterday's grooming and tracks show
	bakeHour = 10.5 // mid-morning: the lines are full and the runs busy
)

func main() {
	in, out := "assets/scenarios/Boreal (Goals Test).save", "assets/menu.save"
	if len(os.Args) > 1 {
		in = os.Args[1]
	}
	if len(os.Args) > 2 {
		out = os.Args[2]
	}
	w, cam, err := save.LoadScenario(in)
	if err != nil {
		panic(err)
	}
	if w.SimTime == 0 {
		w.SimTime = sim.SimTimeAt(0, float64(w.StartHour))
	}
	// Plenty of money, so nothing closes for want of it.
	w.Cash, w.Goals, w.GoalProgress, w.Outcome = 50_000_000, nil, nil, world.Playing

	s := sim.NewSimulationWithSeed(w, 1)
	s.SetResortOpen(true)
	for _, l := range w.Lifts {
		l.Open = true
	}
	w.UpdateLineLayouts()
	s.TimeScale = 1e5
	end := sim.SimTimeAt(bakeDay, bakeHour)
	for s.SimTime < end {
		s.Tick(1.0 / s.TimeScale)
	}
	on := len(w.OnMountain)
	if err := save.SaveScenario(out, w, cam); err != nil {
		panic(err)
	}
	fmt.Printf("%s: %s, %d guests on the mountain\n", out, world.FormatGameDate(s.DateAt(s.SimTime), true), on)
}
