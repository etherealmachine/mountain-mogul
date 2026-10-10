// stages builds one stage of the Boreal tutorial's build order
// (notes/next/Boreal Tutorial.md) from the Boreal Goals Test save, by
// removing what comes later, prices it, and runs it headless for some
// days with every lift open, printing each day's visitors, stars and
// money. The building, lift, cat and snowmobile IDs are that save's.
//
//	go run ./tools/stages "assets/scenarios/Boreal (Goals Test).save" <stage 1-8> <days>
//
// Stages: 1 the lot, California Cruiser and tickets; 2 a garage and one
// cat; 3 patrol and a snowmobile; 4 a small lodge (6 food court, 2
// lounge tiles); 5 Accelerator; 6 a second cat; 7 the second lot; 8 49er,
// the full lodge, a third cat and a second snowmobile.
package main

import (
	"fmt"
	"os"
	"sort"

	"mountain-mogul/internal/save"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

const (
	lot1, lot2     = 30, 10060
	tickets        = 10037
	garage, patrol = 10036, 10039
	lodge          = 10038
	cruiser, accel = 10047, 33
	fortyNiner     = 10035
	cat1, cat2     = 10044, 10045
	sled1          = 10042
)

func main() {
	w, _, err := save.LoadScenario(os.Args[1])
	if err != nil {
		panic(err)
	}
	var stage, days int
	fmt.Sscan(os.Args[2], &stage)
	fmt.Sscan(os.Args[3], &days)
	keepB := map[uint64]bool{lot1: true, tickets: true}
	keepL := map[uint64]bool{cruiser: true}
	cats := 0
	sleds := 0
	food, lounge := 0, 0
	if stage >= 2 {
		keepB[garage], cats = true, 1
	}
	if stage >= 3 {
		keepB[patrol], sleds = true, 1
	}
	if stage >= 4 {
		keepB[lodge], food, lounge = true, 6, 2
	}
	if stage >= 5 {
		keepL[accel] = true
	}
	if stage >= 6 {
		cats = 2
	}
	if stage >= 7 {
		keepB[lot2] = true
	}
	if stage >= 8 {
		keepL[fortyNiner], cats, sleds, food, lounge = true, 3, 2, 16, 8
	}
	for _, l := range append([]*world.Lift(nil), w.Lifts...) {
		if !keepL[l.ID] {
			w.RemoveLift(l.ID)
		}
	}
	for _, b := range append([]*world.Building(nil), w.Buildings...) {
		if !keepB[b.ID] {
			w.RemoveBuilding(b.ID)
		}
	}
	for i, c := range append([]*world.Snowcat(nil), w.Snowcats...) {
		if i >= cats {
			w.RemoveSnowcat(c.ID)
		}
	}
	for i, m := range append([]*world.Snowmobile(nil), w.Snowmobiles...) {
		if i >= sleds {
			w.RemoveSnowmobile(m.ID)
		}
	}
	if b := w.BuildingByID(lodge); b != nil {
		// Shrink to food food-court and lounge lounge tiles, taking out
		// those furthest from the doors first.
		steps := b.Entrances()
		var cells [][2]int
		for c := range b.TileMap() {
			cells = append(cells, c)
		}
		dist := func(c [2]int) float32 {
			p := b.TileCentre(c)
			best := float32(1e9)
			for _, e := range steps {
				best = min(best, p.Sub(e).Len())
			}
			return best
		}
		sort.Slice(cells, func(i, j int) bool { return dist(cells[i]) < dist(cells[j]) })
		left := map[world.Service]int{world.ServiceFood: food, world.ServiceLounge: lounge}
		for _, c := range cells {
			s := b.ServiceAt(c)
			if left[s] > 0 {
				left[s]--
				continue
			}
			w.RemoveTile(b, c)
		}
	}
	// Price what's built.
	total := 0
	for _, l := range w.Lifts {
		total += world.LiftCost(l.Type, l.Base, l.Top)
	}
	for _, b := range w.Buildings {
		if len(b.Tiles) > 0 {
			c := world.ShellCost(b.Kind, b.Floors(), len(b.Tiles), true)
			for _, s := range b.Tiles {
				c += world.FitOutCost(b.Kind, b.Floors(), s)
			}
			total += c
		} else {
			total += world.BuildingCost(b.Type)
		}
	}
	total += len(w.Snowcats)*world.CatPurchasePrice + len(w.Snowmobiles)*world.SnowmobilePrice
	stalls := 0
	for _, b := range w.Buildings {
		if b.Type == world.BuildingParking {
			stalls += len(b.Stalls)
		}
	}
	fmt.Printf("stage %d: lifts %d, cats %d, sleds %d, lodge tiles %v, stalls %d; built $%d; open-day costs $%d\n",
		stage, len(w.Lifts), len(w.Snowcats), len(w.Snowmobiles), lodgeTiles(w), stalls, total, w.DailyOperatingCost())
	if days == 0 {
		return
	}
	s := sim.NewSimulationWithSeed(w, 1)
	s.SetResortOpen(true)
	for _, l := range w.Lifts {
		l.Open, l.OnHold = true, false
	}
	s.TimeScale = 1e5
	day := int(s.SimTime / world.SecondsPerSimDay)
	for d := 0; d < days; d++ {
		date := s.DateAt(s.SimTime)
		for int(s.SimTime/world.SecondsPerSimDay) == day {
			s.Tick(1.0 / s.TimeScale)
		}
		day++
		h := w.History
		smp := h.Samples[(h.Head+len(h.Samples)-1)%len(h.Samples)]
		_, name := world.HolidayAt(date)
		r := smp.Reviews
		fmt.Printf("  %-7s %-10s visitors %4d  stars %.2f  revenue %6d  costs %5d  net %6d  %v\n",
			world.FormatGameDate(date, false), name, smp.ArrivalsToday, r.Stars/float32(max(r.N, 1)), smp.Revenue, smp.Costs, smp.Revenue-smp.Costs, smp.RevenueByKind)
	}
}

func lodgeTiles(w *world.World) map[string]int {
	out := map[string]int{}
	for _, b := range w.Buildings {
		for _, s := range b.Tiles {
			out[s.Label()]++
		}
	}
	return out
}
