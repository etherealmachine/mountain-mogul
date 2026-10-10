// econ prices what a save has built (lifts, buildings, cats and
// snowmobiles, roads) with its daily open and standby costs, then runs it
// headless for some days with the resort open and prints each day's
// visitors, revenue and costs by category, net, and cash.
//
//	go run ./tools/econ <save> [days]
//
// For balancing (notes/next/Economy Balance.md).
package main

import (
	"fmt"
	"os"
	"sort"

	"mountain-mogul/internal/save"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

func main() {
	w, _, err := save.LoadScenario(os.Args[1])
	if err != nil {
		panic(err)
	}
	days := 0
	if len(os.Args) > 2 {
		fmt.Sscan(os.Args[2], &days)
	}
	fmt.Printf("cash %d credit %d rate %.2f day ticket %d pass %d parking %d start %s\n",
		w.Cash, w.CreditLimit, w.CreditRate, w.DayTicketPrice, w.SeasonPassPrice, w.ParkingPrice, w.StartDate.Format("Jan 2"))
	total := 0
	fmt.Println("LIFTS")
	for _, l := range w.Lifts {
		length := l.Base.Sub(l.Top).Len()
		vert := w.Terrain.GroundHeightAt(l.Top[0], l.Top[1]) - w.Terrain.GroundHeightAt(l.Base[0], l.Base[1])
		c := world.LiftCost(l.Type, l.Base, l.Top)
		total += c
		fmt.Printf("  %-24s %-16s %5.0f m  %4.0f m vert  $%9d  running $%d/day\n", l.Name, l.Type.Label(), length, vert, c,
			l.Headcount()*world.LiftStaffDailyCost+l.Type.RunningCostDay())
	}
	fmt.Println("BUILDINGS")
	for _, b := range w.Buildings {
		c := 0
		desc := ""
		switch b.Type {
		case world.BuildingParking, world.BuildingSnowGun, world.BuildingTicketOffice:
			c = world.BuildingCost(b.Type)
			if b.Type == world.BuildingParking {
				desc = fmt.Sprintf("%d stalls", len(b.Stalls))
			}
		default:
			if len(b.Tiles) > 0 {
				c = world.ShellCost(b.Kind, b.Floors(), len(b.Tiles), true)
				svc := map[string]int{}
				for _, s := range b.Tiles {
					c += world.FitOutCost(b.Kind, b.Floors(), s)
					svc[s.Label()]++
				}
				var keys []string
				for k := range svc {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					desc += fmt.Sprintf("%s×%d ", k, svc[k])
				}
				desc += fmt.Sprintf("(%d storeys) upkeep $%d/day", b.Floors(), world.LodgeUpkeep(b))
			} else {
				c = world.BuildingCost(b.Type)
			}
		}
		total += c
		fmt.Printf("  %-24s $%9d  %s\n", b.Label(), c, desc)
	}
	fmt.Printf("CATS %d ($%d each)  SNOWMOBILES %d ($%d each)\n", len(w.Snowcats), world.CatPurchasePrice, len(w.Snowmobiles), world.SnowmobilePrice)
	total += len(w.Snowcats)*world.CatPurchasePrice + len(w.Snowmobiles)*world.SnowmobilePrice
	road := 0
	for _, e := range w.RoadEdges {
		a, b := w.RoadNodeByID(e.A), w.RoadNodeByID(e.B)
		if a != nil && b != nil {
			road += world.RoadCost(a.Pos, b.Pos)
		}
	}
	fmt.Printf("ROADS (all edges, maybe pre-built) $%d\n", road)
	fmt.Printf("TOTAL BUILT (excl roads, parcels, glading) $%d\n", total)
	fmt.Printf("open-day costs %v total %d; standby %d\n", w.OperatingCosts(), w.DailyOperatingCost(), w.DailyStandbyCost())
	if days == 0 {
		return
	}
	s := sim.NewSimulationWithSeed(w, 1)
	s.SetResortOpen(true)
	for _, l := range w.Lifts {
		l.Open, l.OnHold = true, false
	}
	s.TimeScale = 1e5
	step := func() { s.Tick(1.0 / 30 / s.TimeScale * 30) }
	day := int(s.SimTime / world.SecondsPerSimDay)
	cash0 := w.Cash
	for d := 0; d < days; d++ {
		date := s.DateAt(s.SimTime)
		for int(s.SimTime/world.SecondsPerSimDay) == day {
			step()
		}
		day++
		h := w.History
		smp := h.Samples[(h.Head+len(h.Samples)-1)%len(h.Samples)]
		_, name := world.HolidayAt(date)
		fmt.Printf("%-7s %-12s visitors %4d  revenue %7d %v  costs %6d %v  net %7d  cash %8d\n",
			world.FormatGameDate(date, false), name, smp.ArrivalsToday, smp.Revenue, smp.RevenueByKind,
			smp.Costs, smp.CostsByKind, smp.Revenue-smp.Costs, w.Cash)
	}
	fmt.Printf("cash change over %d days: %d\n", days, w.Cash-cash0)
}
