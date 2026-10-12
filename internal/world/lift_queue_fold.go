package world

import "github.com/go-gl/mathgl/mgl32"

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
	queueFoldMin = 4
	// queueApproachOut is how far outside the line an arrival walks
	// round it, metres.
	queueApproachOut = float32(2.5)
	// queueApproachReach is how close to the back of the line an arrival
	// heads straight for it.
	queueApproachReach = float32(2.5)
)

// queueSlotFrame is place index's position in the line's frame: metres
// along queueDir from the base, and metres out to the fold side.
func (l *Lift) queueSlotFrame(index int) (along, lat float32) {
	n := l.QueueFold
	if n <= 0 {
		return float32(index) * QueueSpacing, 0
	}
	lane, j := index/n, index%n
	if lane%2 == 1 {
		j = n - 1 - j
	}
	return float32(j) * QueueSpacing, float32(lane) * QueueFoldLaneGap
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
	_, backLat := l.queueSlotFrame(len(l.Queue))
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
		// Outside the outer lane: the back of the line is in reach.
		return back
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
// from the base until the first parking lot or blocked cell (at most
// queueFoldMax places), on whichever side has the longer clear run.
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
	for _, l := range w.Lifts {
		if len(l.Lines) > 0 || l.IsHeli() {
			l.QueueFold = 0
			continue
		}
		best, bestSide := -1, float32(1)
		for _, side := range [2]float32{1, -1} {
			n := 1
			for n < queueFoldMax && clear(l, float32(n)*QueueSpacing, 0, side) && clear(l, float32(n)*QueueSpacing, QueueFoldLaneGap, side) {
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
