// tutorial plays the Boreal tutorial headless (notes/next/Boreal
// Tutorial.md): the tutorial's start (its money, credit and goals, from
// boreal.save) on the Boreal Goals Test's layout, building four stages in
// order as money allows, and running days until the scenario is won.
// The layout stands in for what a player would build on the tutorial's
// map (the two maps line up in world coordinates); clearing a run's trees
// is charged from the tutorial map's trees on the run's cells.
//
//	go run ./tools/tutorial assets/scenarios/boreal.save "assets/scenarios/Boreal (Goals Test).save" [-credit] [-days n] [-seed n] [-cash n]
//
// A player builds the next stage once the cash covers it; with -credit,
// once cash and the credit line together do. Each day prints the cash,
// the day's visitors, stars and net, and what was built. -cash sets the
// starting cash instead of the tutorial's; -lodge food,lounge the
// lodge's tiles (6,2); -cats n the cats in the garage (1); -limit n
// the credit line instead of the tutorial's; -price n the day ticket;
// -attendants hires a line attendant at each lift as it's built;
// -upgrade upgrades each lift one step as it's built (double to fixed
// quad, fixed quad to high-speed quad, high-speed quad to six-pack).
//
// Stage 5, for the rating goal: the lodge grows to its full size (16 food
// court, 8 lounge), the garage to 12 tiles, and the other cats come.
//
// Stages (the user, 2026-10-12): 1 a beginner lift (Cruiser) with two
// green runs, a garage with one cat, and tickets; 2 a lodge with a food
// court, and patrol with a snowmobile; 3 a second lift (Accelerator) with
// green and blue runs; 4 the third lift (49er) and the black runs.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

// The Goals Test's IDs.
const (
	lot1, lot2     = 30, 10060
	tickets        = 10037
	garage, patrol = 10036, 10039
	lodge          = 10038
	cruiser, accel = 10047, 33
	fortyNiner     = 10035
	kissABear      = 10050
)

// What's built: lot1's stalls (the tutorial's lot), the garage's and the
// lodge's tiles.
const (
	lotStalls    = 300
	garageTiles  = 4
	secondRunOut = 55 // metres beside Kiss A Bear the second green runs
)

type stage struct {
	name      string
	lifts     []uint64
	trails    []uint64
	buildings []uint64
	grow      bool // the lodge and garage to their full size, and the other cats
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: tutorial <boreal.save> <Goals Test.save> [-credit] [-days n] [-seed n]")
		os.Exit(2)
	}
	start, _, err := save.LoadScenario(os.Args[1])
	if err != nil {
		panic(err)
	}
	w, _, err := save.LoadScenario(os.Args[2])
	if err != nil {
		panic(err)
	}
	credit, days, seed, cash, limit := false, 40, int64(1), -1, -1
	lodgeFood, lodgeLounge, cats := 6, 2, 1
	attendants, upgrade, price := false, false, 0
	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-credit":
			credit = true
		case "-days":
			i++
			fmt.Sscan(os.Args[i], &days)
		case "-seed":
			i++
			fmt.Sscan(os.Args[i], &seed)
		case "-lodge":
			i++
			fmt.Sscanf(os.Args[i], "%d,%d", &lodgeFood, &lodgeLounge)
		case "-cats":
			i++
			fmt.Sscan(os.Args[i], &cats)
		case "-attendants":
			attendants = true
		case "-upgrade":
			upgrade = true
		case "-price":
			i++
			fmt.Sscan(os.Args[i], &price)
		case "-limit":
			i++
			fmt.Sscan(os.Args[i], &limit)
		case "-cash":
			i++
			fmt.Sscan(os.Args[i], &cash)
		}
	}

	// The tutorial's start.
	w.Cash, w.CreditLimit, w.CreditRate = start.Cash, start.CreditLimit, start.CreditRate
	if cash >= 0 {
		w.Cash = cash
	}
	if limit >= 0 {
		w.CreditLimit = limit
	}
	w.Goals, w.GoalProgress, w.Outcome = start.Goals, nil, world.Playing
	w.DayTicketPrice, w.SeasonPassPrice, w.ParkingPrice = start.DayTicketPrice, start.SeasonPassPrice, start.ParkingPrice
	if price > 0 {
		w.DayTicketPrice = price
	}
	w.RemoveBuilding(lot2)
	if b := w.BuildingByID(lot1); b != nil && lotStalls < len(b.Stalls) {
		b.Stalls, b.MaxCars = b.Stalls[:lotStalls], lotStalls
	}
	g := grown{lodge: w.BuildingByID(lodge).TileMap(), garage: w.BuildingByID(garage).TileMap()}
	trimTiles(w, w.BuildingByID(garage), map[world.Service]int{world.ServiceGarage: garageTiles})
	trimTiles(w, w.BuildingByID(lodge), map[world.Service]int{world.ServiceFood: lodgeFood, world.ServiceLounge: lodgeLounge})
	for len(w.Snowcats) > cats {
		c := w.Snowcats[len(w.Snowcats)-1]
		g.cats = append(g.cats, c)
		w.RemoveSnowcat(c.ID)
	}
	for len(w.Snowmobiles) > 1 {
		w.RemoveSnowmobile(w.Snowmobiles[len(w.Snowmobiles)-1].ID)
	}
	second := secondRun(w)

	stages := []stage{
		{"1 Cruiser, two greens, garage and cat, tickets", []uint64{cruiser}, []uint64{kissABear, second}, []uint64{garage, tickets}, false},
		{"2 lodge, patrol and snowmobile", nil, nil, []uint64{lodge, patrol}, false},
		{"3 Accelerator, greens and blues", []uint64{accel}, nil, nil, false},
		{"4 49er, blacks", []uint64{fortyNiner}, nil, nil, false},
		{"5 the full lodge, a bigger garage, more cats", nil, nil, nil, true},
	}
	for _, t := range w.Trails {
		if t.ID == kissABear || t.ID == second {
			continue
		}
		if t.Difficulty >= world.DiffBlack {
			stages[3].trails = append(stages[3].trails, t.ID)
		} else {
			stages[2].trails = append(stages[2].trails, t.ID)
		}
	}

	h := hide(w)
	costs := make([]int, len(stages))
	for i, st := range stages {
		costs[i] = stageCost(w, h, start, st)
		if st.grow {
			costs[i] += g.cost(h)
		}
	}
	fmt.Printf("start: cash $%d, credit $%d at %.0f%%, lot %d stalls, goals %d\n", w.Cash, w.CreditLimit, w.CreditRate*100, lotStalls, len(w.Goals))
	for i, st := range stages {
		fmt.Printf("stage %s: $%d\n", st.name, costs[i])
	}

	s := sim.NewSimulationWithSeed(w, seed)
	s.SetResortOpen(true)
	s.TimeScale = 1e5
	next := 0
	afford := func(cost int) bool {
		if credit {
			return w.Available() >= cost
		}
		return w.Cash >= cost
	}
	build := func(day int) []string {
		var built []string
		for next < len(stages) && afford(costs[next]) {
			w.Cash -= costs[next]
			h.restore(w, stages[next])
			if stages[next].grow {
				g.apply(w, h)
			}
			for _, id := range stages[next].lifts {
				l := h.lifts[id]
				if upgrade {
					for _, to := range []world.LiftType{world.LiftFixedQuad, world.LiftHSQuad, world.LiftHS6Pack} {
						if world.LiftUpgradeCost(l.Type, to) > 0 {
							w.UpgradeLift(l, to)
							break
						}
					}
				}
				l.Staff.LineAttendant = attendants
			}
			w.UpdateLineLayouts()
			s.InvalidateSections()
			built = append(built, fmt.Sprintf("stage %d ($%d)", next+1, costs[next]))
			next++
		}
		return built
	}
	built := build(0)
	fmt.Printf("day  0 %-10s built %v; cash $%d\n", "", built, w.Cash)
	day := int(s.SimTime / world.SecondsPerSimDay)
	for d := 1; d <= days; d++ {
		date := s.DateAt(s.SimTime)
		for int(s.SimTime/world.SecondsPerSimDay) == day {
			s.Tick(1.0 / s.TimeScale)
		}
		day++
		hist := w.History
		smp := hist.Samples[(hist.Head+len(hist.Samples)-1)%len(hist.Samples)]
		_, holiday := world.HolidayAt(date)
		r := smp.Reviews
		built := build(d)
		fmt.Printf("day %2d %-7s %-10s visitors %4d away %4d stars %.2f rating %.2f net %7d cash %8d %v\n",
			d, world.FormatGameDate(date, false), holiday, smp.ArrivalsToday, smp.TurnedAway,
			r.Stars/float32(max(r.N, 1)), w.Rating, smp.Revenue-smp.Costs, w.Cash, built)
		var why []string
		for k, n := range smp.DepartReasons {
			if n > 0 {
				why = append(why, fmt.Sprintf("%s %d", ai.DepartReasonLabel[k], n))
			}
		}
		fmt.Printf("    went home: %s\n", strings.Join(why, ", "))
		k := smp.RevenueByKind
		fmt.Printf("    revenue: tickets %d, passes %d, food %d, bar %d, rentals %d; costs %v\n",
			k[world.RevenueDayTickets], k[world.RevenueSeasonPasses], k[world.RevenueFood], k[world.RevenueBar], k[world.RevenueRentals], smp.CostsByKind)
		type kc struct {
			k ai.ThoughtKind
			n int
		}
		var ks []kc
		for k, n := range r.Why {
			if n > 0 {
				ks = append(ks, kc{ai.ThoughtKind(k), n})
			}
		}
		sort.Slice(ks, func(i, j int) bool { return ks[i].n > ks[j].n })
		var top []string
		for _, x := range ks[:min(len(ks), 5)] {
			top = append(top, fmt.Sprintf("%s %d", ai.ThoughtLabel[x.k], x.n))
		}
		fmt.Printf("    reviews: 1★ %d, 2★ %d (nothing special %d), 3★ %d; why: %s\n", r.Levels[1], r.Levels[2], r.NothingSpecial, r.Levels[3], strings.Join(top, ", "))
		if w.Outcome != world.Playing {
			fmt.Printf("outcome %v on day %d\n", w.Outcome, w.OutcomeDay+1)
			break
		}
	}
	for i, g := range w.Goals {
		if i < len(w.GoalProgress) {
			p := w.GoalProgress[i]
			fmt.Printf("goal %s: met %v day %d best %.2f\n", g.Describe(), p.Met, p.MetDay+1, p.Best)
		}
	}
}

// trimTiles cuts b down to keep[s] tiles of each service s, taking out
// those furthest from its doors first.
func trimTiles(w *world.World, b *world.Building, keep map[world.Service]int) {
	if b == nil {
		return
	}
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
	for _, c := range cells {
		s := b.ServiceAt(c)
		if n, ok := keep[s]; ok {
			if n > 0 {
				keep[s]--
				continue
			}
			w.RemoveTile(b, c)
		}
	}
}

// secondRun paints a second green run for Cruiser beside Kiss A Bear,
// from the same ends, and clears its trees. Returns its ID.
func secondRun(w *world.World) uint64 {
	k := w.FindTrail(kissABear)
	var l *world.Lift
	for _, x := range w.Lifts {
		if x.ID == cruiser {
			l = x
		}
	}
	axis := l.Top.Sub(l.Base).Normalize()
	side := mgl32.Vec2{axis[1], -axis[0]}
	t := w.PlaceTrail("Kiss A Bear 2", world.DiffGreen)
	t.Kind, t.Groomed, t.Start, t.End = k.Kind, true, k.Start, k.End
	for i, n := range k.Nodes {
		if i > 0 && i < len(k.Nodes)-1 {
			n.Pos = n.Pos.Add(side.Mul(secondRunOut))
		}
		n.Width = min(n.Width, 30)
		t.Nodes = append(t.Nodes, n)
	}
	w.RebuildTrailGraph()
	for _, c := range t.Cells {
		w.Terrain.ClearTreesInCell(c[0], c[1])
	}
	return t.ID
}

// stageCost is what building st costs: lifts, buildings with their
// tiles, cats and snowmobiles housed in them, and clearing the trees the
// tutorial's map has on its runs.
func stageCost(w *world.World, h *hidden, start *world.World, st stage) int {
	total := 0
	for _, id := range st.lifts {
		l := h.lifts[id]
		total += world.LiftCost(l.Type, l.Base, l.Top)
	}
	for _, id := range st.buildings {
		b := h.buildings[id]
		if len(b.Tiles) > 0 {
			c := world.ShellCost(b.Kind, b.Floors(), len(b.Tiles), true)
			for _, s := range b.Tiles {
				c += world.FitOutCost(b.Kind, b.Floors(), s)
			}
			total += c
		} else {
			total += world.BuildingCost(b.Type)
		}
		for _, c := range h.cats {
			if c.ShedID == id {
				total += world.CatPurchasePrice
			}
		}
		for _, m := range h.sleds {
			if m.GarageID == id {
				total += world.SnowmobilePrice
			}
		}
	}
	for _, id := range st.trails {
		t := h.trails[id]
		trees := 0
		for _, c := range t.Cells {
			if start.Terrain.InBounds(c[0], c[1]) {
				trees += int(start.Terrain.Cells[c[0]][c[1]].TreeCount)
			}
		}
		total += trees * world.GladeCostPerTree
	}
	return total
}

// hidden is everything taken out of the world until its stage is built.
type hidden struct {
	lifts      map[uint64]*world.Lift
	trails     map[uint64]*world.Trail
	buildings  map[uint64]*world.Building
	cats       []*world.Snowcat
	sleds      []*world.Snowmobile
	patrollers []*world.Patroller
}

// hide takes every lift, run and building but the lot out of w, with
// the cats, snowmobiles and patrollers they house.
func hide(w *world.World) *hidden {
	h := &hidden{lifts: map[uint64]*world.Lift{}, trails: map[uint64]*world.Trail{}, buildings: map[uint64]*world.Building{}}
	for _, l := range w.Lifts {
		h.lifts[l.ID] = l
		setPassable(w, l.QueueCell(), true)
		setPassable(w, l.TopCell(), true)
	}
	w.Lifts = nil
	for _, t := range w.Trails {
		h.trails[t.ID] = t
	}
	w.Trails = nil
	var keep []*world.Building
	for _, b := range w.Buildings {
		if b.ID == lot1 {
			keep = append(keep, b)
			continue
		}
		h.buildings[b.ID] = b
		for _, c := range b.Ground {
			setPassable(w, c, true)
		}
	}
	w.Buildings = keep
	h.cats, h.sleds, h.patrollers = w.Snowcats, w.Snowmobiles, w.Patrollers
	w.Snowcats, w.Snowmobiles, w.Patrollers = nil, nil, nil
	w.RebuildTrailGraph()
	return h
}

// restore puts st back.
func (h *hidden) restore(w *world.World, st stage) {
	for _, id := range st.lifts {
		l := h.lifts[id]
		l.Open, l.OnHold = true, false
		w.Lifts = append(w.Lifts, l)
		setPassable(w, l.QueueCell(), false)
		setPassable(w, l.TopCell(), false)
	}
	for _, id := range st.trails {
		w.Trails = append(w.Trails, h.trails[id])
	}
	for _, id := range st.buildings {
		b := h.buildings[id]
		w.Buildings = append(w.Buildings, b)
		for _, c := range b.Ground {
			setPassable(w, c, false)
		}
		for _, c := range h.cats {
			if c.ShedID == id {
				w.Snowcats = append(w.Snowcats, c)
			}
		}
		for _, m := range h.sleds {
			if m.GarageID == id {
				w.Snowmobiles = append(w.Snowmobiles, m)
			}
		}
		for _, p := range h.patrollers {
			if p.HutID == id {
				w.Patrollers = append(w.Patrollers, p)
			}
		}
	}
	w.RebuildTrailGraph()
	w.UpdateLineLayouts()
}

func setPassable(w *world.World, c [2]int, ok bool) {
	if w.Terrain.InBounds(c[0], c[1]) {
		w.Terrain.Cells[c[0]][c[1]].Passable = ok
	}
}

// grown is what the lodge and garage grow back to, and the cats bought
// with them.
type grown struct {
	lodge, garage map[[2]int]world.Service
	cats          []*world.Snowcat
}

// cost is what growing the lodge and garage back and buying the cats
// costs.
func (g grown) cost(h *hidden) int {
	total := len(g.cats) * world.CatPurchasePrice
	for id, full := range map[uint64]map[[2]int]world.Service{lodge: g.lodge, garage: g.garage} {
		b := h.buildings[id]
		added := 0
		for c, s := range full {
			if !b.HasCell(c) {
				added++
				total += world.FitOutCost(b.Kind, b.Floors(), s)
			}
		}
		total += world.ShellCost(b.Kind, b.Floors(), added, false)
	}
	return total
}

// apply grows the lodge and garage back and parks the cats.
func (g grown) apply(w *world.World, h *hidden) {
	w.SetTiles(h.buildings[lodge], g.lodge)
	w.SetTiles(h.buildings[garage], g.garage)
	w.Snowcats = append(w.Snowcats, g.cats...)
	w.RebuildTrailGraph()
}
