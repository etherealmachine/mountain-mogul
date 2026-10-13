package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// A lift with a line attendant has a rope corral (Lift Lines), straight lanes with no turns (skiers would rather not): two lanes
// side by side, each a chair wide (a row of a chair's seats, a chair's
// worth to a row), that funnel into one short run a chair wide to the
// load. Arrivals pick a lane and come in at its open back. The lanes
// take turns sending a chair's worth into the run, so the line's order
// (Lift.Queue) is the run, then the lanes interleaved a chair's worth at
// a time. The lanes are as long as the line needs, where there's room;
// a line longer than that goes on in rows behind them, unroped. The
// ropes are ropes like the player's (rope.go): skiers and walkers go
// round them.
//
// In the line's frame (toQueueFrame), along runs back from the load point and
// lat across it; the run is centred on the riders' cable, the first lane on the
// negative side and the second on the positive.

const (
	// corralAcross is the space each guest has across a row, metres: a
	// seat's width, so a row is as wide as the chair it boards and stands
	// between the station's posts.
	corralAcross = float32(0.6)
	// corralRunRows is how many rows the run to the load holds: long
	// enough that a fixed-grip station's wider lanes start clear of its
	// legs and hut (8 m behind the bullwheel), and a detachable's behind
	// its front posts.
	corralRunRows = 5
	// corralFunnel is how far back the run's ropes widen out to the lanes'
	// outer sides, metres.
	corralFunnel = float32(2.0)
	// corralMaxRows is the most rows a lane has, where there's room.
	corralMaxRows = 30
	// corralMinRows is the fewest, however close the obstacle.
	corralMinRows = 2
	// corralEntranceOut is how far beyond the back of a lane an arrival
	// heads for its entrance, metres.
	corralEntranceOut = float32(1.5)
)

// HasCorral reports whether the lift's line is a rope corral.
func (l *Lift) HasCorral() bool { return l.CorralRows > 0 }

// corralWidth is a row's places: the chair's seats.
func (l *Lift) corralWidth() int { return max(l.Type.Capacity(), 1) }

// corralRunLen is how many places the run to the load holds.
func (l *Lift) corralRunLen() int { return corralRunRows * l.corralWidth() }

// corralRunEnd is where the run's rows end and its ropes start widening,
// metres back from the load point.
func (l *Lift) corralRunEnd() float32 {
	return (float32(corralRunRows) - 0.5) * QueueSpacing
}

// corralLaneStart is where the lanes' ropes start, past the funnel.
func (l *Lift) corralLaneStart() float32 { return l.corralRunEnd() + corralFunnel }

// corralRowAlong is how far back from the load point a lane's row stands.
func (l *Lift) corralRowAlong(row int) float32 {
	return l.corralLaneStart() + (float32(row)+0.5)*QueueSpacing
}

// corralLaneBack is where lane rows' ropes end, metres back from the load point.
func (l *Lift) corralLaneBack() float32 {
	return l.corralLaneStart() + float32(l.CorralRows)*QueueSpacing
}

// corralLaneLat is the middle of lane (1 or 2) across the line.
func (l *Lift) corralLaneLat(lane int) float32 {
	half := float32(l.corralWidth()) * corralAcross / 2
	if lane == 1 {
		return -half
	}
	return half
}

// corralColLat is how far across a row of the run (lane 0) or a lane the
// col-th place stands.
func (l *Lift) corralColLat(lane, col int) float32 {
	off := (float32(col) - float32(l.corralWidth()-1)/2) * corralAcross
	if lane == 0 {
		return off
	}
	return l.corralLaneLat(lane) + off
}

// CorralCapacity is how many places the corral's ropes hold.
func (l *Lift) CorralCapacity() int {
	if !l.HasCorral() {
		return 0
	}
	return l.corralRunLen() + 2*l.CorralRows*l.corralWidth()
}

// corralLaneCounts is how many are in each lane (index 1 and 2).
func (l *Lift) corralLaneCounts() (n [3]int) {
	for _, g := range l.Queue {
		n[g.LineLane]++
	}
	return n
}

// corralRowsFor is how many rows each lane needs: the fuller lane's guests
// and a row spare, within the room there is.
func (l *Lift) corralRowsFor() int {
	n := l.corralLaneCounts()
	w := l.corralWidth()
	rows := (max(n[1], n[2])+w-1)/w + 1
	return min(max(rows, corralMinRows), max(l.corralRoom, corralMinRows))
}

// growCorral lengthens the lanes when the line outgrows them, and reports
// whether it did.
func (l *Lift) growCorral() bool {
	if !l.HasCorral() {
		return false
	}
	if n := l.corralRowsFor(); n > l.CorralRows {
		l.CorralRows = n
		return true
	}
	return false
}

// fitCorral sizes the lanes to the line. Reports whether it changed.
func (l *Lift) fitCorral() bool {
	n := l.corralRowsFor()
	if n == l.CorralRows {
		return false
	}
	l.CorralRows = n
	return true
}

// startCorral puts everyone already in line into the corral: the front into
// the run, the rest into the lanes a chair's worth at a time, in turn,
// so their order holds.
func (l *Lift) startCorral() {
	w, run := l.corralWidth(), l.corralRunLen()
	for i, g := range l.Queue {
		g.LineLane = 0
		if i >= run {
			g.LineLane = 1 + ((i-run)/w)%2
		}
	}
	l.mergeTurn, l.mergeTaken = 1, 0
	l.CorralRows = max(l.CorralRows, 1)
	l.fitCorral()
}

// stopCorral takes everyone in line out of the lanes.
func (l *Lift) stopCorral() {
	for _, g := range l.Queue {
		g.LineLane = 0
	}
	l.CorralRows = 0
}

// chooseCorralLane is the lane an arrival at from joins: their group's,
// when some of it is in a lane; otherwise the emptier, or the one whose
// entrance is nearer.
func (l *Lift) chooseCorralLane(from mgl32.Vec2, g *Guest) int {
	if g != nil {
		head := g.PartyHead()
		for _, q := range l.Queue {
			if q.LineLane != 0 && q.PartyHead() == head {
				return q.LineLane
			}
		}
	}
	n := l.corralLaneCounts()
	if n[1] != n[2] {
		if n[1] < n[2] {
			return 1
		}
		return 2
	}
	if l.corralEntrance(1).Sub(from).Len() <= l.corralEntrance(2).Sub(from).Len() {
		return 1
	}
	return 2
}

// corralEntrance is where arrivals come into lane: beyond its ropes, or
// beyond the back of a line that has outgrown them.
func (l *Lift) corralEntrance(lane int) mgl32.Vec2 {
	n := l.corralLaneCounts()[lane]
	back := max(l.corralLaneBack(), l.corralRowAlong(n/l.corralWidth())+QueueSpacing/2)
	x, z := l.fromQueueFrame(back+corralEntranceOut, l.corralLaneLat(lane))
	return mgl32.Vec2{x, z}
}

// CorralEntranceXZ is where an arrival at from heads to join the corral: the
// entrance of the lane they'd choose.
func (l *Lift) CorralEntranceXZ(from mgl32.Vec2) mgl32.Vec2 {
	return l.corralEntrance(l.chooseCorralLane(from, nil))
}

// joinCorral puts g at the back of their lane.
func (l *Lift) joinCorral(g *Guest) {
	g.LineLane = l.chooseCorralLane(mgl32.Vec2{g.Pos[0], g.Pos[2]}, g)
	l.Queue = append(l.Queue, g)
	l.mergeCorral()
}

// mergeCorral moves guests from the lanes into the run while it has room,
// the lanes taking turns a chair's worth at a time (an empty lane
// passes its turn), and puts the line in boarding order: the run, then
// the lanes interleaved the same way.
func (l *Lift) mergeCorral() {
	w, run := l.corralWidth(), l.corralRunLen()
	var lanes [3][]*Guest
	var front []*Guest
	for _, g := range l.Queue {
		if g.LineLane == 0 {
			front = append(front, g)
		} else {
			lanes[g.LineLane] = append(lanes[g.LineLane], g)
		}
	}
	if l.mergeTurn == 0 {
		l.mergeTurn = 1
	}
	next := func() {
		l.mergeTurn, l.mergeTaken = 3-l.mergeTurn, 0
	}
	// Into the run.
	for len(front) < run && len(lanes[1])+len(lanes[2]) > 0 {
		lane := lanes[l.mergeTurn]
		if len(lane) == 0 {
			next()
			continue
		}
		g := lane[0]
		lanes[l.mergeTurn] = lane[1:]
		g.LineLane = 0
		front = append(front, g)
		if l.mergeTaken++; l.mergeTaken >= w {
			next()
		}
	}
	// The order the lanes will go in.
	q := l.Queue[:0]
	q = append(q, front...)
	turn, taken := l.mergeTurn, l.mergeTaken
	for len(lanes[1])+len(lanes[2]) > 0 {
		if len(lanes[turn]) == 0 {
			turn, taken = 3-turn, 0
			continue
		}
		q = append(q, lanes[turn][0])
		lanes[turn] = lanes[turn][1:]
		if taken++; taken >= w {
			turn, taken = 3-turn, 0
		}
	}
	l.Queue = q
}

// CorralPlaceXZ is where g, at index in lift l's corral, stands: their place
// in the run, or in their lane behind those of it ahead of them.
func (l *Lift) CorralPlaceXZ(g *Guest, index int) (x, z float32) {
	w := l.corralWidth()
	if g.LineLane == 0 {
		row, col := index/w, index%w
		return l.fromQueueFrame(float32(row)*QueueSpacing, l.corralColLat(0, col))
	}
	rank := 0
	for _, q := range l.Queue[:min(index, len(l.Queue))] {
		if q.LineLane == g.LineLane {
			rank++
		}
	}
	row, col := rank/w, rank%w
	return l.fromQueueFrame(l.corralRowAlong(row), l.corralColLat(g.LineLane, col))
}

// CorralRopes is the corral's ropes, each a line of nodes: along both sides
// of the run from beside the load, widening out to the lanes' outer
// sides, and down those and between the lanes to their open backs. Nil
// without a corral.
func (l *Lift) CorralRopes() [][]mgl32.Vec2 {
	if !l.HasCorral() {
		return nil
	}
	at := func(along, lat float32) mgl32.Vec2 {
		x, z := l.fromQueueFrame(along, lat)
		return mgl32.Vec2{x, z}
	}
	half := float32(l.corralWidth()) * corralAcross / 2
	runFrom := float32(QueueSpacing / 2)
	runEnd, laneStart, back := l.corralRunEnd(), l.corralLaneStart(), l.corralLaneBack()
	var out [][]mgl32.Vec2
	for _, s := range [2]float32{-1, 1} {
		out = append(out, []mgl32.Vec2{
			at(runFrom, s*half), at(runEnd, s*half), at(laneStart, s*2*half), at(back, s*2*half),
		})
	}
	// Between the lanes, from where the run widens.
	out = append(out, []mgl32.Vec2{at(laneStart, 0), at(back, 0)})
	return out
}

// CorralRopeLines is every lift's corral ropes.
func (w *World) CorralRopeLines() [][]mgl32.Vec2 {
	var out [][]mgl32.Vec2
	for _, l := range w.Lifts {
		out = append(out, l.CorralRopes()...)
	}
	return out
}

// updateCorralRoom sets how many rows lift l's lanes can have: as many as
// are clear of lots and blocked cells across both lanes.
func (w *World) updateCorralRoom(l *Lift) {
	t := w.Terrain
	clear := func(along, lat float32) bool {
		x, z := l.fromQueueFrame(along, lat)
		cx, cz := int(math.Floor(float64(x/CellSize))), int(math.Floor(float64(z/CellSize)))
		if !t.InBounds(cx, cz) || !t.Cells[cx][cz].Walkable() {
			return false
		}
		return w.ParkingLotAt(cx, cz) == nil
	}
	half := float32(l.corralWidth()) * corralAcross
	l.corralRoom = 0
	for l.corralRoom < corralMaxRows {
		along := l.corralRowAlong(l.corralRoom)
		ok := true
		for lat := -half; lat <= half && ok; lat += corralAcross {
			ok = clear(along-QueueSpacing/2, lat) && clear(along+QueueSpacing/2, lat)
		}
		if !ok {
			break
		}
		l.corralRoom++
	}
}
