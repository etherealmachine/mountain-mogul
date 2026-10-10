// days runs a save headless for some game days with the resort and every
// lift open, and reports each day from its history sample: visitors,
// stars and why, falls, rides per guest, peak crowd, and wall time.
//
//	go run ./tools/days <save> [days]
//
// STOP_HOUR=h SAVE_TO=path instead runs to hour h of the first day and
// saves there, for a starting point to look at in the game.
package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/scene"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

func main() {
	w, _, err := save.LoadScenario(os.Args[1])
	if err != nil {
		panic(err)
	}
	days := 1
	fmt.Sscan(os.Args[2], &days)
	s := sim.NewSimulationWithSeed(w, 1)
	s.SetResortOpen(true)
	for _, l := range w.Lifts {
		l.Open, l.OnHold = true, false
	}
	s.TimeScale = 1e5
	step := func() { s.Tick(1.0 / 30 / s.TimeScale * 30) } // one sim second
	if hs := os.Getenv("STOP_HOUR"); hs != "" {
		var stop float64
		fmt.Sscan(hs, &stop)
		for sim.HourOfDay(s.SimTime) < stop {
			step()
		}
		if err := save.SaveScenario(os.Getenv("SAVE_TO"), w, nil); err != nil {
			panic(err)
		}
		fmt.Printf("saved at %.2f h: %d on the mountain\n", sim.HourOfDay(s.SimTime), len(w.OnMountain))
		return
	}
	rides := map[uint64]int{}
	day := int(s.SimTime / world.SecondsPerSimDay)
	for d := 0; d < days; d++ {
		date := s.DateAt(s.SimTime)
		start := time.Now()
		var openWall float64
		peak, lastScan := 0, 0.0
		for int(s.SimTime/world.SecondsPerSimDay) == day {
			t0 := time.Now()
			h := sim.HourOfDay(s.SimTime)
			step()
			if h >= 9 && h < 16 {
				openWall += time.Since(t0).Seconds()
			}
			peak = max(peak, len(w.OnMountain))
			if s.SimTime-lastScan >= 30 {
				lastScan = s.SimTime
				for _, g := range w.OnMountain {
					n := 0
					for _, t := range g.LiftTally {
						n += int(t.Runs)
					}
					rides[g.ID] = max(rides[g.ID], n)
				}
			}
		}
		day++
		h := w.History
		smp := h.Samples[(h.Head+len(h.Samples)-1)%len(h.Samples)]
		r := smp.Reviews
		var total int
		for _, n := range rides {
			total += n
		}
		_, name := world.HolidayAt(date)
		avg := float32(0)
		if r.N > 0 {
			avg = r.Stars / float32(r.N)
		}
		fmt.Printf("%-7s %-15s visitors %4d  %.2f★ (1★ %d 2★ %d 3★ %d)  falls %5d  rides/guest %.1f  peak %4d  wall %.0fs (open %.0fs)\n",
			world.FormatGameDate(date, false), name, smp.ArrivalsToday, avg, r.Levels[1], r.Levels[2], r.Levels[3],
			smp.Falls, float64(total)/float64(max(len(rides), 1)), peak, time.Since(start).Seconds(), openWall)
		if gs := s.GroupStats(); gs.Led > 0 {
			fmt.Printf("        groups: follower steps %d, on the line %.0f%%; plans copied %d, own %d, led %d; wait steps %d\n",
				gs.Steps, 100*float64(gs.OnLine)/float64(max(gs.Steps, 1)), gs.Copied, gs.Own, gs.Led, gs.Waits)
		}
		type kv struct {
			k string
			v int
		}
		var why []kv
		for k, v := range r.Why {
			if v > 0 {
				why = append(why, kv{scene.ReviewLine(world.Review{Level: 2, Kind: ai.ThoughtKind(k)}, func(uint64) string { return "" }), v})
			}
		}
		sort.Slice(why, func(i, j int) bool { return why[i].v > why[j].v })
		for i, x := range why {
			if i < 5 {
				fmt.Printf("        %4d %s\n", x.v, x.k)
			}
		}
		rides = map[uint64]int{}
	}
}
