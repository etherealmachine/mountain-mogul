package world

import (
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
)

// A lift with no line attendant has a crowd, not a line (Lift Lines):
// guests bunch in a rough wedge fanning out from the loading spot, where
// arrivals take the nearest free place on its edge, everyone shuffles up
// into places opening ahead of them, and now and then someone slips in
// ahead of a guest who came first. Lift.Queue stays in the order they
// came, but for those slips, and boarding takes from its front: the
// shuffle keeps the longest waiters at the front of the crowd.

const (
	// crowdRow is how far apart the wedge's rows are along the line,
	// metres; crowdGap how far apart places in a row are.
	crowdRow = float32(1.2)
	crowdGap = float32(1.0)
	// crowdSpread is what a metre out from the line's axis costs against
	// a metre along it: the wedge widens by a place every row.
	crowdSpread = crowdRow * 2 / crowdGap
	// crowdWide is how far out from the axis places cost only
	// crowdSpread, metres (about 6 across); beyond it they cost
	// crowdWideCost a metre, so the crowd goes wider only round
	// something in its way.
	crowdWide     = float32(3)
	crowdWideCost = float32(6)
	// crowdMaxAlong and crowdMaxLat bound where places can be, metres.
	crowdMaxAlong = float32(50)
	crowdMaxLat   = float32(8)
	// crowdPlaces is the most places a crowd has.
	crowdPlaces = 300
	// crowdJitter is how far a place sits off its row, metres, so the
	// crowd doesn't stand in ranks.
	crowdJitter = float32(0.25)
	// crowdStationClear is how far places keep from the station's legs
	// and hut, metres.
	crowdStationClear = float32(1.0)
	// crowdStep is how far a guest shuffles into a place opening ahead
	// of them, metres.
	crowdStep = float32(2.6)
	// crowdEdge is how much dearer than the crowd's dearest occupied
	// place an arrival's place can be.
	crowdEdge = float32(1.5)
	// crowdGroupEdge is the same for an arrival joining their group, who
	// squeezes in a little further to stand by them.
	crowdGroupEdge = float32(3)
	// crowdEdgeBand is how much cheaper than the dearest occupied place
	// an arrival's place can be: the crowd's edge, not a gap inside it.
	crowdEdgeBand = float32(2.4)
	// CrowdReach is how close to their place an arrival joins the crowd,
	// metres.
	CrowdReach = float32(2.5)
)

// CrowdPlace is one place in a lift's crowd.
type CrowdPlace struct {
	Pos   mgl32.Vec2
	Cost  float32 // how far back it is: boarding order
	Guest *Guest  // who stands there, nil when free
}

// IsCrowd reports whether the lift's line is a crowd.
func (l *Lift) IsCrowd() bool { return len(l.Crowd) > 0 }

// buildCrowd lays out lift l's crowd places, cheapest first: the loading
// spot, then a grid of staggered rows in the line's frame, each
// jittered, leaving out places that aren't walkable, are in a lot or a
// building, or stand by the station's legs or in its hut.
func (w *World) buildCrowd(l *Lift) []CrowdPlace {
	t := w.Terrain
	stations := l.StationXZs()
	boxes := l.StationBoxes()
	var out []CrowdPlace
	for r := 0; float32(r)*crowdRow <= crowdMaxAlong; r++ {
		along := float32(r) * crowdRow
		off := float32(0)
		if r%2 == 1 {
			off = crowdGap / 2
		}
		for k := -int(crowdMaxLat / crowdGap); float32(k)*crowdGap <= crowdMaxLat; k++ {
			lat := float32(k)*crowdGap + off
			if abs32(lat) > crowdMaxLat || (r == 0 && k != 0) {
				continue
			}
			cost := along + crowdSpread*min(abs32(lat), crowdWide) + crowdWideCost*max(abs32(lat)-crowdWide, 0)
			ja, jl := crowdJitterAt(l.ID, r, k)
			if r == 0 {
				ja, jl = 0, 0 // the loading spot
			}
			x, z := l.fromLineFrame(along+ja, lat+jl)
			p := mgl32.Vec2{x, z}
			cx, cz := int(x/CellSize), int(z/CellSize)
			if r > 0 && (!t.InBounds(cx, cz) || !t.Cells[cx][cz].Walkable() || w.ParkingLotAt(cx, cz) != nil ||
				w.onFootprint(p) || nearAny(p, stations, crowdStationClear) || inHut(p, boxes)) {
				continue
			}
			out = append(out, CrowdPlace{Pos: p, Cost: cost})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Cost < out[j].Cost })
	if len(out) > crowdPlaces {
		out = out[:crowdPlaces]
	}
	return out
}

// crowdJitterAt is a place's fixed offset from its row: along and out.
func crowdJitterAt(id uint64, r, k int) (float32, float32) {
	h := uint32(id)*2654435761 ^ uint32(r)*40503 ^ uint32(k+64)*9973
	h ^= h >> 13
	h *= 0x5bd1e995
	h ^= h >> 15
	u := float32(h&0xffff)/0xffff*2 - 1
	v := float32(h>>16)/0xffff*2 - 1
	return u * crowdJitter, v * crowdJitter
}

// fromLineFrame converts a position in the line's frame (metres along
// queueDir from the base, and out to the right looking uphill) to world
// XZ.
func (l *Lift) fromLineFrame(along, lat float32) (x, z float32) {
	dx, dz := l.queueDir()
	rx, rz := l.lateralRight()
	return l.Base[0] + dx*along + rx*lat, l.Base[1] + dz*along + rz*lat
}

// onFootprint reports whether p is on a building's footprint.
func (w *World) onFootprint(p mgl32.Vec2) bool {
	for _, b := range w.Buildings {
		if b.FootprintContains(p[0], p[1], 0.5) {
			return true
		}
	}
	return false
}

// inHut reports whether p is within half a metre of a station's hut,
// which walkers keep out of.
func inHut(p mgl32.Vec2, boxes []StationBox) bool {
	for _, b := range boxes {
		if b.Hut && b.Contains(p, 0.5) {
			return true
		}
	}
	return false
}

func nearAny(p mgl32.Vec2, pts []mgl32.Vec2, r float32) bool {
	for _, q := range pts {
		if p.Sub(q).Len() < r {
			return true
		}
	}
	return false
}

// setCrowd gives lift l its crowd places, or none (nil), and puts
// everyone in line in a place in line order.
func (l *Lift) setCrowd(places []CrowdPlace) {
	l.Crowd = places
	for i, g := range l.Queue {
		g.CrowdPlace = 0
		if i < len(l.Crowd) {
			l.Crowd[i].Guest = g
			g.CrowdPlace = i + 1
		}
	}
}

// crowdOccupied is the dearest occupied place's cost, ok false when
// nobody has a place.
func (l *Lift) crowdOccupied() (cost float32, ok bool) {
	for i := range l.Crowd {
		if l.Crowd[i].Guest != nil && (!ok || l.Crowd[i].Cost > cost) {
			cost, ok = l.Crowd[i].Cost, true
		}
	}
	return cost, ok
}

// crowdPlaceNear is the free place on the crowd's edge nearest p: no
// dearer than the dearest occupied place plus slack, and no cheaper than
// it less crowdEdgeBand, so an arrival can't take a gap deep in the
// crowd. Failing that the cheapest free place; -1 when the crowd is
// full.
func (l *Lift) crowdPlaceNear(p mgl32.Vec2, slack float32) int {
	hi, lo := slack, float32(math.Inf(-1))
	if c, ok := l.crowdOccupied(); ok {
		hi, lo = c+slack, c-crowdEdgeBand
	}
	best, bestD, cheapest := -1, float32(math.MaxFloat32), -1
	for i := range l.Crowd {
		c := &l.Crowd[i]
		if c.Guest != nil {
			continue
		}
		if cheapest < 0 {
			cheapest = i
		}
		if c.Cost > hi || c.Cost < lo {
			continue
		}
		if d := c.Pos.Sub(p).Len(); d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 {
		return cheapest
	}
	return best
}

// CrowdArrivalXZ is where an arrival at from heads for: the free place
// on the crowd's edge nearest them.
func (l *Lift) CrowdArrivalXZ(from mgl32.Vec2) mgl32.Vec2 {
	if i := l.crowdPlaceNear(from, crowdEdge); i >= 0 {
		return l.Crowd[i].Pos
	}
	return l.Crowd[len(l.Crowd)-1].Pos
}

// JoinLine puts g in lift l's line at time now: at the back of a single
// file, or in a crowd at the free place on its edge nearest them, or
// nearest someone from their group already in it.
func (l *Lift) JoinLine(g *Guest, now float64) {
	g.LineJoined, g.CrowdPlace, g.LinePassed = now, 0, 0
	if !l.IsCrowd() {
		l.Queue = append(l.Queue, g)
		return
	}
	near, slack := mgl32.Vec2{g.Pos[0], g.Pos[2]}, crowdEdge
	head := g.PartyHead()
	for _, q := range l.Queue {
		if q.PartyHead() == head && q.CrowdPlace > 0 {
			near, slack = l.Crowd[q.CrowdPlace-1].Pos, crowdGroupEdge
			break
		}
	}
	if i := l.crowdPlaceNear(near, slack); i >= 0 {
		l.Crowd[i].Guest = g
		g.CrowdPlace = i + 1
	}
	l.Queue = append(l.Queue, g)
}

// LeaveLine takes g out of lift l's line, wherever they are in it.
func (l *Lift) LeaveLine(g *Guest) {
	for i, q := range l.Queue {
		if q == g {
			l.Queue = append(l.Queue[:i], l.Queue[i+1:]...)
			break
		}
	}
	if g.CrowdPlace > 0 && g.CrowdPlace <= len(l.Crowd) && l.Crowd[g.CrowdPlace-1].Guest == g {
		l.Crowd[g.CrowdPlace-1].Guest = nil
	}
	g.CrowdPlace = 0
}

// ClearLine empties lift l's line and returns who was in it.
func (l *Lift) ClearLine() []*Guest {
	out := append([]*Guest(nil), l.Queue...)
	l.Queue = l.Queue[:0]
	for i := range l.Crowd {
		l.Crowd[i].Guest = nil
	}
	for _, g := range out {
		g.CrowdPlace = 0
	}
	return out
}

// ShuffleCrowd moves guests up into the places opening ahead of them,
// frontmost place first. Of those within a step of a place, whoever
// joined first takes it, unless slip says someone else gets there
// first: then the nearest of the others does, and goes ahead of whoever
// came first in the line. Guests without a place take the cheapest free
// ones. Returns who was passed.
func (l *Lift) ShuffleCrowd(slip func() bool) (passed []*Guest) {
	if !l.IsCrowd() {
		return nil
	}
	for i := range l.Crowd {
		free := &l.Crowd[i]
		if free.Guest != nil {
			continue
		}
		first, nearest := -1, -1
		var nearestD float32
		for j := i + 1; j < len(l.Crowd); j++ {
			if g := l.Crowd[j].Guest; g != nil && l.Crowd[j].Pos.Sub(free.Pos).Len() <= crowdStep {
				if first < 0 || g.LineJoined < l.Crowd[first].Guest.LineJoined {
					first = j
				}
			}
		}
		for j := i + 1; j < len(l.Crowd) && first >= 0; j++ {
			if g := l.Crowd[j].Guest; g != nil && j != first {
				if d := l.Crowd[j].Pos.Sub(free.Pos).Len(); d <= crowdStep && (nearest < 0 || d < nearestD) {
					nearest, nearestD = j, d
				}
			}
		}
		if first < 0 {
			continue
		}
		mover := first
		if nearest >= 0 && slip() {
			mover = nearest
			passed = append(passed, l.Crowd[first].Guest)
			l.queueAhead(l.Crowd[nearest].Guest, l.Crowd[first].Guest)
		}
		g := l.Crowd[mover].Guest
		l.Crowd[mover].Guest = nil
		free.Guest = g
		g.CrowdPlace = i + 1
	}
	for _, g := range l.Queue {
		if g.CrowdPlace != 0 {
			continue
		}
		for i := range l.Crowd {
			if l.Crowd[i].Guest == nil {
				l.Crowd[i].Guest = g
				g.CrowdPlace = i + 1
				break
			}
		}
	}
	return passed
}

// queueAhead moves g ahead of other in lift l's line if they're behind.
func (l *Lift) queueAhead(g, other *Guest) {
	gi, oi := -1, -1
	for i, q := range l.Queue {
		switch q {
		case g:
			gi = i
		case other:
			oi = i
		}
	}
	if gi < 0 || oi < 0 || gi < oi {
		return
	}
	copy(l.Queue[oi+1:gi+1], l.Queue[oi:gi])
	l.Queue[oi] = g
}

// LinePlaceXZ is where g, at index in lift l's line, stands: their
// crowd place, or their place in a single file. A guest the crowd has
// no room for stands at its dearest place.
func (l *Lift) LinePlaceXZ(g *Guest, index int) (x, z float32) {
	if !l.IsCrowd() {
		return l.QueueSlotXZ(index)
	}
	if g.CrowdPlace > 0 && g.CrowdPlace <= len(l.Crowd) {
		p := l.Crowd[g.CrowdPlace-1].Pos
		return p[0], p[1]
	}
	p := l.Crowd[len(l.Crowd)-1].Pos
	return p[0], p[1]
}

// LineEndXZ is where an arrival at from joins lift l's line: the edge of
// its crowd nearest them, or the back of its single file.
func (l *Lift) LineEndXZ(from mgl32.Vec2) mgl32.Vec2 {
	if l.IsCrowd() {
		return l.CrowdArrivalXZ(from)
	}
	x, z := l.QueueSlotXZ(len(l.Queue))
	return mgl32.Vec2{x, z}
}

// UpdateLineLayouts lays out every lift's line: a crowd where there's no
// line attendant, a folded single file where there is (until the
// attendant's maze), nothing for a helicopter. Everyone in line keeps
// their order.
func (w *World) UpdateLineLayouts() {
	if w.Terrain == nil {
		return
	}
	w.UpdateQueueFolds()
	for _, l := range w.Lifts {
		w.UpdateLineLayout(l)
	}
}

// UpdateLineLayout lays out lift l's line, as UpdateLineLayouts does.
func (w *World) UpdateLineLayout(l *Lift) {
	if w.Terrain == nil || l.IsHeli() || l.Staff.LineAttendant {
		l.setCrowd(nil)
		return
	}
	if places := w.buildCrowd(l); len(places) > 0 {
		l.setCrowd(places)
		return
	}
	l.setCrowd(nil)
}
