package sim

import (
	"container/heap"
	"hash/fnv"
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// Traffic: carloads of guests drive from their home entry to a parking
// lot, park in a stall, and drive home again when the whole carload is
// back (notes/next/Transit.md step 2).
//
// Each direction of each road edge is a lane, a polyline offset to the
// right of the road's centre spline. Cars on a lane keep a gap to the car
// ahead and never overtake. Junctions (nodes where three or more roads
// meet) and lot driveways are one car at a time: a car asks for the node
// as it nears, stops short of it until it's granted (first to ask goes
// first), and lets go once it's clear on the far side. Inside a lot a car
// follows the aisles from the entrance to its stall; cars in a lot don't
// see each other.
//
// Lots no road reaches, and maps without entries, still work: the car
// appears in its stall and vanishes when it leaves.

const (
	roadSpeed     = float32(15.6) // 35 mph, the two-lane road
	drivewaySpeed = float32(6.7)  // 15 mph, legs touching a lot's driveway
	lotSpeed      = float32(4.5)  // 10 mph, lot aisles
	stallSpeed    = float32(2.2)  // 5 mph, pulling into a stall

	carAccel = float32(2.5) // m/s²
	carBrake = float32(4.0) // m/s², comfortable braking used to plan stops

	// carSpacing is the closest a car's centre comes to the one ahead:
	// a car length plus a couple of metres.
	carSpacing = world.CarLength + 2.5
	// carReact is the following driver's reaction time, in seconds.
	carReact = float32(0.8)
	// carLookahead is how far down its route a car looks for the car
	// ahead.
	carLookahead = float32(120)
	// stopBack is where a car waits short of a node it hasn't been
	// granted, and clearDist how far past the node it lets go of it.
	stopBack  = float32(4)
	clearDist = float32(5)
	// holdTimeout breaks gridlock: a car that has waited this long at a
	// node goes anyway.
	holdTimeout = float32(20)

	laneOffset = world.RoadHalfWidth / 2

	// trafficStep is the longest single traffic step; a frame's sim time
	// is cut into steps no longer than this.
	trafficStep = float32(0.5)
)

// lane is a drivable polyline with arc lengths and a speed limit.
type lane struct {
	pts   []mgl32.Vec2
	cum   []float32
	limit float32
}

func newLane(pts []mgl32.Vec2, limit float32) *lane {
	l := &lane{pts: pts, cum: world.CumulativeChainDist(pts), limit: limit}
	return l
}

func (l *lane) length() float32 { return l.cum[len(l.cum)-1] }

// at is the point and unit direction d metres along the lane.
func (l *lane) at(d float32) (mgl32.Vec2, mgl32.Vec2) {
	n := len(l.pts)
	if n == 1 {
		return l.pts[0], mgl32.Vec2{0, 1}
	}
	i := sort.Search(n, func(i int) bool { return l.cum[i] > d }) - 1
	if i < 0 {
		i = 0
	}
	if i > n-2 {
		i = n - 2
	}
	a, b := l.pts[i], l.pts[i+1]
	seg := l.cum[i+1] - l.cum[i]
	dir := b.Sub(a)
	if seg <= 1e-4 {
		if dir.Len() > 1e-6 {
			dir = dir.Normalize()
		} else {
			dir = mgl32.Vec2{0, 1}
		}
		return a, dir
	}
	t := (d - l.cum[i]) / seg
	t = max(0, min(1, t))
	return a.Add(dir.Mul(t)), dir.Mul(1 / seg)
}

// laneKey is a directed road edge, from node to node.
type laneKey [2]uint64

// roadNet is the drivable view of the road graph, rebuilt whenever the
// graph changes.
type roadNet struct {
	sig    uint64
	lanes  map[laneKey]*lane
	adj    map[uint64][]uint64
	kind   map[uint64]world.RoadNodeKind
	routes map[laneKey]route
	// entrances are each lot's ways in: its driveway node when a road
	// reaches it, and any road dead end touching the lot. gate marks
	// those nodes.
	entrances map[uint64][]uint64
	gate      map[uint64]bool
}

type route struct {
	nodes  []uint64
	length float32
	ok     bool
}

// roadSignature hashes the road graph so a change (a new road, a moved
// node, a lot's new driveway) rebuilds the network.
func roadSignature(w *world.World) uint64 {
	h := fnv.New64a()
	var buf [8]byte
	put := func(v uint64) {
		for i := range buf {
			buf[i] = byte(v >> (8 * i))
		}
		h.Write(buf[:])
	}
	for _, n := range w.RoadNodes {
		put(n.ID)
		put(uint64(math.Float32bits(n.Pos[0]))<<32 | uint64(math.Float32bits(n.Pos[1])))
		put(uint64(n.Kind))
	}
	for _, e := range w.RoadEdges {
		put(e.A<<32 ^ e.B)
	}
	for _, b := range w.Buildings {
		if b.Type == world.BuildingParking {
			put(b.ID)
			put(uint64(len(b.Ground)))
			if len(b.Ground) > 0 {
				first, last := b.Ground[0], b.Ground[len(b.Ground)-1]
				put(uint64(uint32(first[0]))<<32 | uint64(uint32(first[1])))
				put(uint64(uint32(last[0]))<<32 | uint64(uint32(last[1])))
			}
		}
	}
	return h.Sum64()
}

func buildRoadNet(w *world.World, sig uint64) *roadNet {
	net := &roadNet{
		sig:       sig,
		lanes:     map[laneKey]*lane{},
		adj:       map[uint64][]uint64{},
		kind:      map[uint64]world.RoadNodeKind{},
		routes:    map[laneKey]route{},
		entrances: map[uint64][]uint64{},
		gate:      map[uint64]bool{},
	}
	for _, n := range w.RoadNodes {
		net.kind[n.ID] = n.Kind
	}
	k := world.RoadChainSamplesPerSegment
	for _, ch := range w.FindRoadChains() {
		ok := true
		for _, n := range ch.Nodes {
			if n == nil {
				ok = false
			}
		}
		if !ok || len(ch.Nodes) < 2 {
			continue
		}
		samples := world.SampleRoadChain(ch, w.Terrain, k)
		for i := 0; i+1 < len(ch.Nodes); i++ {
			lo, hi := i*k, (i+1)*k+1
			if hi > len(samples) {
				break
			}
			seg := samples[lo:hi]
			a, b := ch.Nodes[i], ch.Nodes[i+1]
			limit := roadSpeed
			if a.Kind == world.RoadNodeParkingDriveway || b.Kind == world.RoadNodeParkingDriveway {
				limit = drivewaySpeed
			}
			net.lanes[laneKey{a.ID, b.ID}] = newLane(offsetLane(seg, false), limit)
			net.lanes[laneKey{b.ID, a.ID}] = newLane(offsetLane(seg, true), limit)
			net.adj[a.ID] = append(net.adj[a.ID], b.ID)
			net.adj[b.ID] = append(net.adj[b.ID], a.ID)
		}
	}
	for _, b := range w.Buildings {
		if b.Type != world.BuildingParking {
			continue
		}
		var ids []uint64
		if len(b.DrivewayNodeIDs) > 0 && len(net.adj[b.DrivewayNodeIDs[0]]) > 0 {
			ids = append(ids, b.DrivewayNodeIDs[0])
		}
		for _, n := range w.RoadNodes {
			if len(net.adj[n.ID]) == 1 && n.Kind != world.RoadNodeEdgeConnection && n.Kind != world.RoadNodeParkingDriveway &&
				b.FootprintContains(n.Pos[0], n.Pos[1], lotReach) {
				ids = append(ids, n.ID)
			}
		}
		net.entrances[b.ID] = ids
		for _, id := range ids {
			net.gate[id] = true
		}
	}
	return net
}

// lotReach is how far outside a lot a road's dead end can stop and still
// lead into it.
const lotReach = float32(6)

// lotRoute is the shortest drive between node and any of lot's
// entrances: from node to the lot, or (out) from the lot to node.
func (n *roadNet) lotRoute(lot uint64, node uint64, out bool) route {
	var best route
	for _, e := range n.entrances[lot] {
		r := n.route(node, e)
		if out {
			r = n.route(e, node)
		}
		if r.ok && len(r.nodes) >= 2 && (!best.ok || r.length < best.length) {
			best = r
		}
	}
	return best
}

// offsetLane is the right-hand lane along a road centreline, reversed
// for the opposite direction.
func offsetLane(centre []mgl32.Vec2, reverse bool) []mgl32.Vec2 {
	n := len(centre)
	pts := make([]mgl32.Vec2, n)
	for i := range centre {
		j := i
		if reverse {
			j = n - 1 - i
		}
		pts[i] = centre[j]
	}
	out := make([]mgl32.Vec2, n)
	for i := range pts {
		a, b := pts[max(i-1, 0)], pts[min(i+1, n-1)]
		d := b.Sub(a)
		if l := d.Len(); l > 1e-6 {
			d = d.Mul(1 / l)
		}
		right := mgl32.Vec2{-d[1], d[0]}
		out[i] = pts[i].Add(right.Mul(laneOffset))
	}
	return out
}

// needsHold reports whether node id is taken one car at a time: a
// junction or a lot's entrance.
func (n *roadNet) needsHold(id uint64) bool {
	if id == 0 {
		return false
	}
	return len(n.adj[id]) >= 3 || n.gate[id]
}

// route is the shortest drive from node a to node b (Dijkstra over lane
// lengths), cached until the network changes.
func (n *roadNet) route(a, b uint64) route {
	key := laneKey{a, b}
	if r, ok := n.routes[key]; ok {
		return r
	}
	r := n.shortest(a, b)
	n.routes[key] = r
	return r
}

type routeItem struct {
	id   uint64
	dist float32
}
type routeHeap []routeItem

func (h routeHeap) Len() int           { return len(h) }
func (h routeHeap) Less(i, j int) bool { return h[i].dist < h[j].dist }
func (h routeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *routeHeap) Push(x any)        { *h = append(*h, x.(routeItem)) }
func (h *routeHeap) Pop() any {
	old := *h
	it := old[len(old)-1]
	*h = old[:len(old)-1]
	return it
}

func (n *roadNet) shortest(a, b uint64) route {
	if a == 0 || b == 0 {
		return route{}
	}
	if a == b {
		return route{nodes: []uint64{a}, ok: true}
	}
	dist := map[uint64]float32{a: 0}
	prev := map[uint64]uint64{}
	h := &routeHeap{{a, 0}}
	for h.Len() > 0 {
		it := heap.Pop(h).(routeItem)
		if it.dist > dist[it.id] {
			continue
		}
		if it.id == b {
			break
		}
		for _, nb := range n.adj[it.id] {
			l := n.lanes[laneKey{it.id, nb}]
			if l == nil {
				continue
			}
			nd := it.dist + l.length()
			if d, ok := dist[nb]; !ok || nd < d {
				dist[nb] = nd
				prev[nb] = it.id
				heap.Push(h, routeItem{nb, nd})
			}
		}
	}
	total, ok := dist[b]
	if !ok {
		return route{}
	}
	var nodes []uint64
	for cur := b; ; cur = prev[cur] {
		nodes = append(nodes, cur)
		if cur == a {
			break
		}
	}
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	return route{nodes: nodes, length: total, ok: true}
}

// holding is a car through a node, and its movement: the nodes it came
// from and goes to (0 for a lot).
type holding struct {
	car uint64
	mv  [2]uint64
}

// yieldAfter is how long a car making a different movement waits before
// a stream through its node stops admitting new cars.
const yieldAfter = float32(3)

func (t *traffic) dropHolder(node, car uint64) {
	hs := t.holder[node]
	for i, h := range hs {
		if h.car == car {
			hs = append(hs[:i], hs[i+1:]...)
			break
		}
	}
	if len(hs) == 0 {
		delete(t.holder, node)
	} else {
		t.holder[node] = hs
	}
}

// compatible reports whether two movements can share a node: the same
// way in (they follow each other, then split) or the same way out (they
// merge, keeping their gap on the lane out).
func compatible(a, b [2]uint64) bool {
	return a[0] == b[0] || a[1] == b[1]
}

// movement is car c's way through the node at the end of its leg.
func movement(c *world.Car) [2]uint64 {
	var mv [2]uint64
	switch c.State {
	case world.CarArriving, world.CarTurnedAway:
		if c.Leg < len(c.Route) {
			mv[0] = c.Route[c.Leg]
		}
		if c.Leg+2 < len(c.Route) {
			mv[1] = c.Route[c.Leg+2]
		}
	case world.CarLeaving:
		if c.Leg >= 1 {
			mv[0] = c.Route[c.Leg-1]
		}
		if c.Leg+1 < len(c.Route) {
			mv[1] = c.Route[c.Leg+1]
		}
	}
	return mv
}

// carHold is a node a car has been granted, held until it is clearDist
// along leg (the leg past the node).
type carHold struct {
	node uint64
	leg  int
}

// traffic is the sim's traffic state. Only the cars themselves are
// saved; holds and queues rebuild as cars drive.
type traffic struct {
	net *roadNet

	holder  map[uint64][]holding // node → cars through it now
	waiting map[uint64][]uint64  // node → cars asking for it, first first
	waitFor map[uint64]float32   // car → seconds waited at its current node
	holds   map[uint64][]carHold // car → nodes it holds
	asked   map[uint64]uint64    // car → node it's asking for
	ready   map[uint64]bool      // car → it had room past the node when it last asked

	lotLanes map[uint64]lotLane // car → its lot leg

	// Rebuilt every step.
	lanes    map[laneKey][]*world.Car
	laneIdx  map[*world.Car]int
	lots     map[uint64]*world.Building
	taken    map[uint64][]bool // lot → stall in use
	inbound  map[uint64]int    // lot → arriving cars
	carsByID map[uint64]*world.Car

	turnedAwayDay int // 1 + day index "lots full" was last logged
}

type lotLane struct {
	lot     uint64
	stall   int
	leaving bool
	lane    *lane
}

func newTraffic() *traffic {
	return &traffic{
		holder:   map[uint64][]holding{},
		waiting:  map[uint64][]uint64{},
		waitFor:  map[uint64]float32{},
		holds:    map[uint64][]carHold{},
		asked:    map[uint64]uint64{},
		ready:    map[uint64]bool{},
		lotLanes: map[uint64]lotLane{},
		lanes:    map[laneKey][]*world.Car{},
		laneIdx:  map[*world.Car]int{},
		lots:     map[uint64]*world.Building{},
		taken:    map[uint64][]bool{},
		inbound:  map[uint64]int{},
		carsByID: map[uint64]*world.Car{},
	}
}

// ensureNet rebuilds the road network when the graph has changed and
// settles every car on it.
func (s *Simulation) ensureNet() *roadNet {
	t := s.traffic
	sig := roadSignature(s.World)
	if t.net != nil && t.net.sig == sig {
		return t.net
	}
	t.net = buildRoadNet(s.World, sig)
	t.holder = map[uint64][]holding{}
	t.waiting = map[uint64][]uint64{}
	t.waitFor = map[uint64]float32{}
	t.holds = map[uint64][]carHold{}
	t.asked = map[uint64]uint64{}
	t.ready = map[uint64]bool{}
	t.lotLanes = map[uint64]lotLane{}
	s.indexTraffic()
	for _, c := range append([]*world.Car(nil), s.World.Cars...) {
		if c.State == world.CarParked {
			continue
		}
		if c.State == world.CarQueued && len(c.Route) >= 2 && t.net.lanes[laneKey{c.Route[0], c.Route[1]}] != nil {
			continue
		}
		ok := c.State != world.CarQueued
		for i := c.Leg; ok && i < s.carLegCount(c); i++ {
			ok = s.carLeg(c, i) != nil
		}
		if !ok {
			s.strandCar(c)
		}
	}
	return t.net
}

// strandCar finishes the trip of a car whose road went away: an arriving
// car parks if it can, anything else is taken off the map.
func (s *Simulation) strandCar(c *world.Car) {
	switch c.State {
	case world.CarQueued, world.CarArriving:
		lot := s.traffic.lots[c.Lot]
		if lot != nil && c.Stall < 0 {
			c.Stall = s.freeStall(lot)
		}
		if lot != nil && c.Stall >= 0 {
			s.parkCar(c, lot)
			return
		}
		s.removeCar(c)
	default:
		s.removeCar(c)
	}
}

// carLegCount is how many legs the car's trip has. Arriving: a road leg
// per route step, then the lot leg into its stall. Leaving: the lot leg
// out of its stall, then a road leg per route step. Turned away: road
// legs only.
func (s *Simulation) carLegCount(c *world.Car) int {
	switch c.State {
	case world.CarQueued, world.CarArriving, world.CarLeaving:
		return len(c.Route)
	case world.CarTurnedAway:
		return len(c.Route) - 1
	}
	return 0
}

// carLeg is leg i of the car's trip, or nil if it doesn't exist.
func (s *Simulation) carLeg(c *world.Car, i int) *lane {
	if i < 0 || i >= s.carLegCount(c) {
		return nil
	}
	switch c.State {
	case world.CarQueued, world.CarArriving:
		if i == len(c.Route)-1 {
			return s.lotLeg(c, false)
		}
		return s.traffic.net.lanes[laneKey{c.Route[i], c.Route[i+1]}]
	case world.CarLeaving:
		if i == 0 {
			return s.lotLeg(c, true)
		}
		return s.traffic.net.lanes[laneKey{c.Route[i-1], c.Route[i]}]
	case world.CarTurnedAway:
		return s.traffic.net.lanes[laneKey{c.Route[i], c.Route[i+1]}]
	}
	return nil
}

// legRoadKey is the lane a road leg drives, ok false for a lot leg.
func legRoadKey(c *world.Car, i int) (laneKey, bool) {
	switch c.State {
	case world.CarQueued, world.CarArriving, world.CarTurnedAway:
		if i+1 < len(c.Route) {
			return laneKey{c.Route[i], c.Route[i+1]}, true
		}
	case world.CarLeaving:
		if i >= 1 && i < len(c.Route) {
			return laneKey{c.Route[i-1], c.Route[i]}, true
		}
	}
	return laneKey{}, false
}

// legEndNode is the road node at the end of leg i, 0 for the stall.
func legEndNode(c *world.Car, i int) uint64 {
	switch c.State {
	case world.CarQueued, world.CarArriving, world.CarTurnedAway:
		if i+1 < len(c.Route) {
			return c.Route[i+1]
		}
	case world.CarLeaving:
		if i < len(c.Route) {
			return c.Route[i]
		}
	}
	return 0
}

// lotLeg is the drive between the lot's entrance and the car's stall,
// along the lot's aisles (Building.LotDrive).
func (s *Simulation) lotLeg(c *world.Car, leaving bool) *lane {
	t := s.traffic
	if ll, ok := t.lotLanes[c.ID]; ok && ll.lot == c.Lot && ll.stall == c.Stall && ll.leaving == leaving {
		return ll.lane
	}
	lot := t.lots[c.Lot]
	if lot == nil || c.Stall < 0 || c.Stall >= len(lot.Stalls) || len(c.Route) == 0 {
		return nil
	}
	drive := c.Route[len(c.Route)-1]
	if leaving {
		drive = c.Route[0]
	}
	dn := s.World.RoadNodeByID(drive)
	if dn == nil {
		return nil
	}
	pts := []mgl32.Vec2{dn.Pos}
	for _, p := range lot.LotDrive(c.Stall) {
		if p.Sub(pts[len(pts)-1]).Len() > 0.05 {
			pts = append(pts, p)
		}
	}
	if leaving {
		for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
			pts[i], pts[j] = pts[j], pts[i]
		}
	}
	l := newLane(pts, lotSpeed)
	t.lotLanes[c.ID] = lotLane{lot: c.Lot, stall: c.Stall, leaving: leaving, lane: l}
	return l
}

// indexTraffic rebuilds the per-step lookups: lots, stalls in use,
// cars by lane.
func (s *Simulation) indexTraffic() {
	t := s.traffic
	w := s.World
	clear(t.lots)
	clear(t.inbound)
	clear(t.carsByID)
	for _, b := range w.Buildings {
		if b.Type == world.BuildingParking {
			t.lots[b.ID] = b
			used := t.taken[b.ID]
			if cap(used) < len(b.Stalls) {
				used = make([]bool, len(b.Stalls))
			}
			used = used[:len(b.Stalls)]
			clear(used)
			t.taken[b.ID] = used
		}
	}
	for id := range t.taken {
		if t.lots[id] == nil {
			delete(t.taken, id)
		}
	}
	for k, cars := range t.lanes {
		t.lanes[k] = cars[:0]
	}
	clear(t.laneIdx)
	for _, c := range w.Cars {
		t.carsByID[c.ID] = c
		if used := t.taken[c.Lot]; c.Stall >= 0 && c.Stall < len(used) {
			used[c.Stall] = true
		}
		if (c.State == world.CarArriving || c.State == world.CarQueued) && c.Stall < 0 {
			t.inbound[c.Lot]++
		}
		if c.Moving() {
			if k, ok := legRoadKey(c, c.Leg); ok {
				t.lanes[k] = append(t.lanes[k], c)
			}
		}
	}
	for _, cars := range t.lanes {
		sort.Slice(cars, func(i, j int) bool { return cars[i].D < cars[j].D })
		for i, c := range cars {
			t.laneIdx[c] = i
		}
	}
}

// freeStall is a stall in lot nobody has, nearest the driveway first, or
// -1.
func (s *Simulation) freeStall(lot *world.Building) int {
	for i, used := range s.traffic.taken[lot.ID] {
		if !used {
			s.traffic.taken[lot.ID][i] = true
			return i
		}
	}
	return -1
}

// pickLot is the lot a car from node `from` heads for, and the route
// there: the nearest by road with a stall to spare (counting cars
// already on their way), else the nearest by road. ok is false when no
// lot can be reached.
func (s *Simulation) pickLot(from uint64, skip uint64) (*world.Building, route, bool) {
	t := s.traffic
	var best, nearest *world.Building
	var bestR, nearR route
	for _, lot := range s.sortedLots() {
		if lot.ID == skip {
			continue
		}
		r := t.net.lotRoute(lot.ID, from, false)
		if !r.ok {
			continue
		}
		if nearest == nil || r.length < nearR.length {
			nearest, nearR = lot, r
		}
		if s.lotSpare(lot) > 0 && (best == nil || r.length < bestR.length) {
			best, bestR = lot, r
		}
	}
	if best != nil {
		return best, bestR, true
	}
	if skip != 0 {
		return nil, route{}, false // turned away: only a lot with room is worth trying
	}
	return nearest, nearR, nearest != nil
}

// lotSpare is how many stalls lot has free once the cars on their way
// to it park.
func (s *Simulation) lotSpare(lot *world.Building) int {
	free := 0
	for _, used := range s.traffic.taken[lot.ID] {
		if !used {
			free++
		}
	}
	return free - s.traffic.inbound[lot.ID]
}

func (s *Simulation) sortedLots() []*world.Building {
	var lots []*world.Building
	for _, b := range s.World.Buildings {
		if b.Type == world.BuildingParking {
			lots = append(lots, b)
		}
	}
	return lots
}

// spawnCar sends a carload of guests from their entry: onto the road
// when a lot can be reached, otherwise straight into a free stall.
// Returns false when there's nowhere to park (the guests stay home).
func (s *Simulation) spawnCar(guests []*world.Guest, entry uint64) bool {
	w := s.World
	s.ensureNet()
	s.indexTraffic()
	c := &world.Car{ID: w.NextID(), Guests: guests, Entry: entry, Stall: -1}
	c.Kind, c.Roof = world.RollCar(c.ID, len(guests))
	if entry != 0 {
		if lot, r, ok := s.pickLot(entry, 0); ok {
			c.Lot, c.Route, c.State = lot.ID, r.nodes, world.CarQueued
			if n := w.RoadNodeByID(entry); n != nil {
				c.Pos = n.Pos
			}
		}
	}
	if c.Route == nil {
		// No road to a lot: appear in a free stall.
		var spare []*world.Building
		for _, lot := range s.sortedLots() {
			if s.lotSpare(lot) > 0 {
				spare = append(spare, lot)
			}
		}
		if len(spare) == 0 {
			s.logLotsFull()
			return false
		}
		lot := spare[rng.Global().Intn(len(spare))]
		c.Lot, c.Stall = lot.ID, s.freeStall(lot)
		if c.Stall < 0 {
			return false
		}
	}
	for _, g := range guests {
		g.State = world.InCar
		g.CarID, g.CarLot = c.ID, 0
	}
	w.Cars = append(w.Cars, c)
	if c.Route == nil {
		st := s.traffic.lots[c.Lot].Stalls[c.Stall]
		c.Pos, c.Heading = st.Pos, st.Heading+math.Pi
		s.parkCar(c, s.traffic.lots[c.Lot])
	}
	return true
}

// parkCar settles a car in its stall and lets its guests out. Each pays
// a share of the lot's fee; a guest with no plan goes back home.
func (s *Simulation) parkCar(c *world.Car, lot *world.Building) {
	w := s.World
	s.releaseAll(c)
	c.State, c.Speed, c.InLot = world.CarParked, 0, true
	st := lot.Stalls[c.Stall]
	c.Pos = st.Pos
	price := max(w.ParkingPrice, 0)
	n := len(c.Guests)
	kept := c.Guests[:0]
	for k, g := range c.Guests {
		share := price*(k+1)/n - price*k/n
		g.CarLot = lot.ID
		if s.spawnGuestAt(lot, g, st.Pos, share) {
			kept = append(kept, g)
		} else {
			g.CarID, g.CarLot = 0, 0
		}
	}
	c.Guests = kept
	if len(kept) == 0 {
		s.startLeaving(c)
	}
}

// startLeaving sends a parked car home once its carload is aboard.
func (s *Simulation) startLeaving(c *world.Car) {
	lot := s.traffic.lots[c.Lot]
	if lot == nil || c.Entry == 0 {
		s.removeCar(c)
		return
	}
	r := s.traffic.net.lotRoute(lot.ID, c.Entry, true)
	if !r.ok {
		s.removeCar(c)
		return
	}
	c.State, c.Route, c.Leg, c.D, c.Speed = world.CarLeaving, r.nodes, 0, 0, 0
}

// removeCar takes a car off the map and sends anyone aboard home.
// Guests still on the mountain lose their car and leave from any lot.
func (s *Simulation) removeCar(c *world.Car) {
	w := s.World
	s.releaseAll(c)
	delete(s.traffic.lotLanes, c.ID)
	for _, g := range c.Guests {
		if g.CarID != c.ID {
			continue
		}
		s.finishDeparture(g) // off the map: their day is done
		g.CarID, g.CarLot = 0, 0
		if g.State == world.InCar {
			g.State = world.AtHome
		}
	}
	for i, o := range w.Cars {
		if o == c {
			w.Cars = append(w.Cars[:i], w.Cars[i+1:]...)
			break
		}
	}
	delete(s.traffic.carsByID, c.ID)
}

// turnAway sends an arriving car with no stall on to the nearest lot
// with room, or home.
func (s *Simulation) turnAway(c *world.Car) {
	drive := c.Route[len(c.Route)-1]
	if lot, r, ok := s.pickLot(drive, c.Lot); ok {
		c.Lot = lot.ID
		c.Route = append(c.Route, r.nodes[1:]...)
		return
	}
	s.logLotsFull()
	r := s.traffic.net.route(drive, c.Entry)
	if !r.ok || len(r.nodes) < 2 {
		s.removeCar(c)
		return
	}
	c.State = world.CarTurnedAway
	c.Route = append(c.Route, r.nodes[1:]...)
}

func (s *Simulation) logLotsFull() {
	day := int(s.SimTime/secondsPerSimDay) + 1
	if s.traffic.turnedAwayDay == day {
		return
	}
	s.traffic.turnedAwayDay = day
	s.World.LogEvent(world.EventGuestsTurnedAway, s.SimTime, "Cars turned away: every parking lot is full")
}

// releaseAll drops every node a car holds or is asking for.
func (s *Simulation) releaseAll(c *world.Car) {
	t := s.traffic
	for _, h := range t.holds[c.ID] {
		t.dropHolder(h.node, c.ID)
	}
	delete(t.holds, c.ID)
	if n, ok := t.asked[c.ID]; ok {
		t.waiting[n] = removeID(t.waiting[n], c.ID)
		delete(t.asked, c.ID)
	}
	delete(t.waitFor, c.ID)
	delete(t.ready, c.ID)
}

func removeID(ids []uint64, id uint64) []uint64 {
	for i, x := range ids {
		if x == id {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}

func (s *Simulation) holds(c *world.Car, node uint64) bool {
	for _, h := range s.traffic.holds[c.ID] {
		if h.node == node {
			return true
		}
	}
	return false
}

// ask puts the car in line for node and reports whether it's granted.
// A car is let through only when there's room for it past the node
// (room: it won't stop inside the junction and block it). Cars making
// compatible movements (same way in or same way out) go through
// together as a stream; a crossing movement waits for the node to empty, and once it has waited
// yieldAfter the stream stops admitting cars so it gets its turn. With
// several cars ready to go, the first in line goes. A car that has
// waited holdTimeout goes when it has room, whatever is in the node.
func (s *Simulation) ask(c *world.Car, node uint64, room bool) bool {
	t := s.traffic
	if s.holds(c, node) {
		return true
	}
	if t.asked[c.ID] != node {
		if prev, ok := t.asked[c.ID]; ok {
			t.waiting[prev] = removeID(t.waiting[prev], c.ID)
		}
		t.asked[c.ID] = node
		t.waiting[node] = append(t.waiting[node], c.ID)
		t.waitFor[c.ID] = 0
	}
	t.ready[c.ID] = room
	if !room {
		return false
	}
	mv := movement(c)
	if t.waitFor[c.ID] < holdTimeout {
		in := t.holder[node]
		for _, h := range in {
			if !compatible(h.mv, mv) && t.carsByID[h.car] != nil {
				return false // a crossing movement is in the node
			}
		}
		for _, id := range t.waiting[node] {
			if id == c.ID {
				continue
			}
			o := t.carsByID[id]
			if o == nil || !t.ready[id] {
				continue
			}
			if !compatible(movement(o), mv) && (len(in) == 0 || t.waitFor[id] >= yieldAfter) {
				// Someone crossing is next: first in line when the node
				// is empty, or the stream yields once they've waited.
				if len(in) > 0 || t.waitFor[id] >= t.waitFor[c.ID] {
					return false
				}
			}
		}
	}
	t.holder[node] = append(t.holder[node], holding{car: c.ID, mv: mv})
	t.holds[c.ID] = append(t.holds[c.ID], carHold{node: node, leg: c.Leg + 1})
	t.waiting[node] = removeID(t.waiting[node], c.ID)
	delete(t.asked, c.ID)
	delete(t.waitFor, c.ID)
	delete(t.ready, c.ID)
	return true
}

// roomPast reports whether car c, once through the node at the end of
// its leg, can get clear of it: the next leg is a lot leg (no queue) or
// the last car to enter that lane is far enough along.
func (s *Simulation) roomPast(c *world.Car) bool {
	k, ok := legRoadKey(c, c.Leg+1)
	if !ok {
		return true
	}
	cars := s.traffic.lanes[k]
	if len(cars) == 0 {
		return true
	}
	need := clearDist + carSpacing
	if l := s.traffic.net.lanes[k]; l != nil {
		need = min(need, max(l.length()-stopBack-0.5, 0)+carSpacing)
	}
	return cars[0].D >= need
}

// releasePassed lets go of nodes the car is clear of.
func (s *Simulation) releasePassed(c *world.Car) {
	t := s.traffic
	hs := t.holds[c.ID]
	if len(hs) == 0 {
		return
	}
	kept := hs[:0]
	for _, h := range hs {
		clearAt := clearDist
		if l := s.carLeg(c, h.leg); l != nil {
			clearAt = min(clearDist, max(l.length()-stopBack-0.5, 0))
		}
		if c.Leg > h.leg || (c.Leg == h.leg && c.D >= clearAt) {
			t.dropHolder(h.node, c.ID)
			continue
		}
		kept = append(kept, h)
	}
	if len(kept) == 0 {
		delete(t.holds, c.ID)
	} else {
		t.holds[c.ID] = kept
	}
}

// leaderGap is the distance from c to the car ahead on its route, up to
// carLookahead, and that car's speed; lot legs see no one.
func (s *Simulation) leaderGap(c *world.Car, leg *lane) (float32, float32) {
	t := s.traffic
	k, ok := legRoadKey(c, c.Leg)
	if !ok {
		return math.MaxFloat32, 0
	}
	acc := leg.length() - c.D
	if cars := t.lanes[k]; len(cars) > 0 {
		if i, ok := t.laneIdx[c]; ok && i+1 < len(cars) {
			ahead := cars[i+1]
			if ak, ok := legRoadKey(ahead, ahead.Leg); ok && ak == k && ahead.Moving() {
				return ahead.D - c.D, ahead.Speed
			}
			if ahead.Moving() {
				return acc + ahead.D, ahead.Speed // just moved onto its next leg
			}
		}
	}
	for i := c.Leg + 1; i < s.carLegCount(c) && acc < carLookahead; i++ {
		k, ok := legRoadKey(c, i)
		if !ok {
			break
		}
		if cars := t.lanes[k]; len(cars) > 0 {
			return acc + cars[0].D, cars[0].Speed
		}
		l := t.net.lanes[k]
		if l == nil {
			break
		}
		acc += l.length()
	}
	return math.MaxFloat32, 0
}

// tickTraffic drives every car through dt sim seconds.
func (s *Simulation) tickTraffic(dt float64) {
	if len(s.World.Cars) == 0 || dt <= 0 {
		return
	}
	s.ensureNet()
	for rem := float32(dt); rem > 0; rem -= trafficStep {
		s.stepTraffic(min(rem, trafficStep))
	}
}

func (s *Simulation) stepTraffic(dt float32) {
	s.indexTraffic()
	t := s.traffic
	for _, c := range append([]*world.Car(nil), s.World.Cars...) {
		if t.carsByID[c.ID] == nil {
			continue // removed earlier this step
		}
		if t.lots[c.Lot] == nil {
			s.removeCar(c)
			continue
		}
		switch c.State {
		case world.CarQueued:
			s.tryEnterRoad(c)
		case world.CarParked:
			if carloadAboard(c) {
				s.startLeaving(c)
			}
		default:
			s.driveCar(c, dt)
		}
	}
}

func carloadAboard(c *world.Car) bool {
	for _, g := range c.Guests {
		if g.State == world.OnMountain && g.CarID == c.ID {
			return false
		}
	}
	return true
}

// tryEnterRoad puts a queued car on the road at its entry when the lane
// there has room.
func (s *Simulation) tryEnterRoad(c *world.Car) {
	t := s.traffic
	k := laneKey{c.Route[0], c.Route[1]}
	if cars := t.lanes[k]; len(cars) > 0 && cars[0].D < carSpacing {
		return
	}
	c.State, c.Leg, c.D, c.Speed = world.CarArriving, 0, 0, roadSpeed*0.6
	t.lanes[k] = append([]*world.Car{c}, t.lanes[k]...)
	for i, o := range t.lanes[k] {
		t.laneIdx[o] = i
	}
	if l := t.net.lanes[k]; l != nil {
		p, d := l.at(0)
		c.Pos, c.Heading = p, headingOf(d)
	}
}

func headingOf(d mgl32.Vec2) float32 {
	return float32(math.Atan2(float64(d[0]), float64(d[1])))
}

func brakeSpeed(dist, vEnd float32) float32 {
	return float32(math.Sqrt(float64(max(vEnd*vEnd+2*carBrake*max(dist, 0), 0))))
}

// driveCar moves one car along its legs for dt seconds: speed up toward
// the leg's limit, slow for the next leg, a node it hasn't been granted,
// the car ahead, and the end of the trip.
func (s *Simulation) driveCar(c *world.Car, dt float32) {
	t := s.traffic
	leg := s.carLeg(c, c.Leg)
	if leg == nil {
		s.strandCar(c)
		return
	}
	n := s.carLegCount(c)
	L := leg.length()
	toEnd := L - c.D
	vMax := leg.limit
	room := float32(math.MaxFloat32)

	last := c.Leg == n-1
	switch {
	case last && c.State == world.CarArriving:
		vMax = min(vMax, brakeSpeed(toEnd, stallSpeed)) // ease into the stall
	case !last:
		if next := s.carLeg(c, c.Leg+1); next != nil {
			vMax = min(vMax, brakeSpeed(toEnd, next.limit))
		}
	}

	if node := legEndNode(c, c.Leg); !last && t.net.needsHold(node) && !s.holds(c, node) {
		granted := false
		if toEnd <= c.Speed*c.Speed/(2*carBrake)+stopBack+4 {
			if c.State == world.CarArriving && c.Leg == n-2 && c.Stall < 0 {
				// Nearing the lot: claim a stall, or go elsewhere.
				if lot := t.lots[c.Lot]; lot != nil {
					c.Stall = s.freeStall(lot)
				}
				if c.Stall < 0 {
					s.turnAway(c)
					if t.carsByID[c.ID] == nil {
						return
					}
					n = s.carLegCount(c)
				}
			}
			granted = s.ask(c, node, s.roomPast(c))
			if !granted {
				t.waitFor[c.ID] += dt
			}
		}
		if !granted {
			room = min(room, toEnd-stopBack)
		}
	}
	room = max(room, 0)
	vMax = min(vMax, brakeSpeed(room, 0))
	if gap, vLead := s.leaderGap(c, leg); gap < math.MaxFloat32 {
		// Follow the car ahead: never closer than carSpacing, and slow
		// enough to stop behind it if it brakes, after a moment to react.
		ahead := max(gap-carSpacing, 0)
		room = min(room, ahead)
		vMax = min(vMax, brakeSpeed(max(ahead-carReact*vLead, 0), vLead))
	}

	v := min(c.Speed+carAccel*dt, vMax)
	move := min(v*dt, room)
	c.Speed = v
	c.D += move

	for c.D >= L {
		if c.Leg >= n-1 {
			s.finishTrip(c)
			return
		}
		c.D -= L
		c.Leg++
		leg = s.carLeg(c, c.Leg)
		if leg == nil {
			s.strandCar(c)
			return
		}
		L = leg.length()
	}
	s.releasePassed(c)
	p, d := leg.at(c.D)
	c.Pos, c.Heading = p, headingOf(d)
	c.InLot = (c.State == world.CarArriving && c.Leg == n-1) || (c.State == world.CarLeaving && c.Leg == 0)
}

// finishTrip ends a car's last leg: in its stall, or off the map.
func (s *Simulation) finishTrip(c *world.Car) {
	if c.State == world.CarArriving {
		if lot := s.traffic.lots[c.Lot]; lot != nil && c.Stall >= 0 && c.Stall < len(lot.Stalls) {
			if l := s.carLeg(c, c.Leg); l != nil {
				_, d := l.at(l.length())
				c.Heading = headingOf(d)
			}
			s.parkCar(c, lot)
			return
		}
	}
	s.removeCar(c)
}

// arrivingGuests is how many guests are in cars on their way in.
func arrivingGuests(w *world.World) int {
	n := 0
	for _, c := range w.Cars {
		if c.State == world.CarQueued || c.State == world.CarArriving {
			n += len(c.Guests)
		}
	}
	return n
}
