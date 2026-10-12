package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// A single-file lift line runs out from the base along queueDir. Left
// alone a long line runs straight on into whatever is downhill (a parking
// lot, a lodge), so it folds into switchback lanes QueueFoldLaneGap apart
// before it gets there. Arrivals come to the back of the line from
// outside it (QueueApproachXZ) instead of walking through it. A stopgap
// until line attendants lay out mazes (Boreal Playtest).

const (
	// QueueFoldLaneGap is the distance between switchback lanes, metres.
	QueueFoldLaneGap = float32(1.5)
	// queueFoldMax is the most places in one lane: 40 m.
	queueFoldMax = 20
	// queueFoldMin is the fewest, however close the obstacle.
	queueFoldMin = 6
	// queueFoldClear is how far from the base the lanes after the first
	// start, metres: clear of the station's legs and hut, which stand
	// beside the first few places.
	queueFoldClear = float32(6)
	// queueFoldLanes is how many lanes the fold keeps clear of lots and
	// blocked cells: room for a line of about 60.
	queueFoldLanes = 4
	// queueApproachOut is how far outside the back of the line an arrival
	// walks round it, metres: within queueApproachReach, so from beside
	// the back they step straight in.
	queueApproachOut = float32(2.0)
	// queueApproachReach is how close to the back of the line an arrival
	// heads straight for it.
	queueApproachReach = float32(2.5)
)

// queueSlotFrame is place index's position in the line's frame: metres
// along queueDir from the base, and metres out to the fold side.
//
// The first lane runs QueueFold places out from the base; the lanes after
// it double back and forth beside it, from queueFoldClear out to the same
// far end, so none stands beside the station.
func (l *Lift) queueSlotFrame(index int) (along, lat float32) {
	n := l.QueueFold
	if n <= 0 || index < n {
		return float32(index) * QueueSpacing, 0
	}
	skip := l.queueFoldSkip()
	m := n - skip // places in each lane after the first
	k := index - n
	lane, j := 1+k/m, k%m
	if lane%2 == 1 {
		j = m - 1 - j
	}
	return float32(skip+j) * QueueSpacing, float32(lane) * QueueFoldLaneGap
}

// queueFoldSkip is how many places from the base the lanes after the
// first leave out.
func (l *Lift) queueFoldSkip() int {
	return int(math.Ceil(float64(queueFoldClear / QueueSpacing)))
}

// fromQueueFrame converts a position in the line's frame to world XZ.
func (l *Lift) fromQueueFrame(along, lat float32) (x, z float32) {
	dx, dz := l.queueDir()
	rx, rz := l.lateralRight()
	s := l.QueueFoldSide
	if s == 0 {
		s = 1
	}
	return l.Base[0] + dx*along + rx*s*lat, l.Base[1] + dz*along + rz*s*lat
}

// toQueueFrame converts world XZ p to the line's frame.
func (l *Lift) toQueueFrame(p mgl32.Vec2) (along, lat float32) {
	dx, dz := l.queueDir()
	rx, rz := l.lateralRight()
	s := l.QueueFoldSide
	if s == 0 {
		s = 1
	}
	ox, oz := p[0]-l.Base[0], p[1]-l.Base[1]
	return ox*dx + oz*dz, s * (ox*rx + oz*rz)
}

// QueueApproachXZ is where an arrival at from should head for the back of
// the single-file line: straight there from outside the line, or round
// the line first when it's in the way. Lane-mode lifts use the back of
// their lines.
func (l *Lift) QueueApproachXZ(from mgl32.Vec2) mgl32.Vec2 {
	bx, bz := l.QueueSlotXZ(len(l.Queue))
	back := mgl32.Vec2{bx, bz}
	if len(l.Lines) > 0 || from.Sub(back).Len() < queueApproachReach {
		return back
	}
	al, lat := l.toQueueFrame(from)
	backAl, backLat := l.queueSlotFrame(len(l.Queue))
	// The line's body: along 0 to its far end, lat 0 to the outer lane.
	far := float32(len(l.Queue)) * QueueSpacing
	if l.QueueFold > 0 && len(l.Queue) >= l.QueueFold {
		far = float32(l.QueueFold-1) * QueueSpacing
	}
	outer := backLat + queueApproachOut
	at := func(along, lat float32) mgl32.Vec2 {
		x, z := l.fromQueueFrame(along, lat)
		return mgl32.Vec2{x, z}
	}
	switch {
	case lat >= backLat+QueueFoldLaneGap:
		// Outside the outer lane: along it to beside the back, then in
		// (a straight line to the back would cut back into the lanes).
		return at(backAl, outer)
	case al > far+1:
		// Beyond the line's far end: step out past the outer lane.
		return at(al, outer)
	case lat < -QueueFoldLaneGap:
		// On the inner side: round the far end.
		return at(far+queueApproachOut+1, lat)
	case al < 0.5:
		// By the station: step out past the outer lane before going on.
		return at(al-0.5, outer)
	default:
		// In the line's way: out by the nearer end.
		if al < far/2 {
			return at(-1, lat)
		}
		return at(far+queueApproachOut+1, lat)
	}
}

// UpdateQueueFolds sets every single-file lift's fold: its lanes run out
// from the base until the first parking lot or blocked cell past the
// station (at most queueFoldMax places), on the side away from the
// station's hut unless the other has the longer clear run.
func (w *World) UpdateQueueFolds() {
	t := w.Terrain
	if t == nil {
		return
	}
	clear := func(l *Lift, along, lat, side float32) bool {
		l.QueueFoldSide = side
		x, z := l.fromQueueFrame(along, lat)
		cx, cz := int(x/CellSize), int(z/CellSize)
		if !t.InBounds(cx, cz) || !t.Cells[cx][cz].Walkable() {
			return false
		}
		return w.ParkingLotAt(cx, cz) == nil
	}
	// The first queueFoldLanes lanes, which hold a long line, are all
	// clear at along.
	lanesClear := func(l *Lift, along, side float32) bool {
		for k := 0; k < queueFoldLanes; k++ {
			if !clear(l, along, float32(k)*QueueFoldLaneGap, side) {
				return false
			}
		}
		return true
	}
	for _, l := range w.Lifts {
		if len(l.Lines) > 0 || l.IsHeli() {
			l.QueueFold = 0
			continue
		}
		// The hut's side, from the base station's parts.
		var hutLat float32
		for _, p := range l.StationXZs() {
			if p.Sub(l.Base).Len() < 8 {
				l.QueueFoldSide = 1
				_, lat := l.toQueueFrame(p)
				hutLat += lat
			}
		}
		away := float32(1)
		if hutLat > 0 {
			away = -1
		}
		best, bestSide := -1, away
		for _, side := range [2]float32{away, -away} {
			// The station's own cells are blocked: start past it.
			n := l.queueFoldSkip()
			for n < queueFoldMax && lanesClear(l, float32(n)*QueueSpacing, side) {
				n++
			}
			if n > best {
				best, bestSide = n, side
			}
		}
		l.QueueFoldSide = bestSide
		l.QueueFold = max(best, queueFoldMin)
	}
}
