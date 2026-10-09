package world

import "github.com/go-gl/mathgl/mgl32"

// TrailLine is a run's centre line with the distance along it to each
// sample, built when trails change (rebuildTrailAt) so skiers can read
// it every step, from any goroutine, without rebuilding the curve.
type TrailLine struct {
	Samples []TrailSample
	Along   []float32 // metres from the first sample

	// buckets holds the samples in a grid of trailLineBucket-metre
	// squares, from bucketLo to bucketHi, so Nearest looks at the few
	// near p instead of all of them (skiers ask twice a step).
	buckets            map[[2]int32][]int32
	bucketLo, bucketHi [2]int32
}

// trailLineBucket is the side of a TrailLine bucket, in metres.
const trailLineBucket = 25

func trailLineKey(p mgl32.Vec2) [2]int32 {
	return [2]int32{int32(floor32(p[0] / trailLineBucket)), int32(floor32(p[1] / trailLineBucket))}
}

func floor32(v float32) float32 {
	f := float32(int32(v))
	if f > v {
		f--
	}
	return f
}

// index fills the buckets.
func (l *TrailLine) index() {
	l.buckets = make(map[[2]int32][]int32)
	for i, s := range l.Samples {
		k := trailLineKey(s.Pos)
		if i == 0 {
			l.bucketLo, l.bucketHi = k, k
		}
		l.bucketLo = [2]int32{min(l.bucketLo[0], k[0]), min(l.bucketLo[1], k[1])}
		l.bucketHi = [2]int32{max(l.bucketHi[0], k[0]), max(l.bucketHi[1], k[1])}
		l.buckets[k] = append(l.buckets[k], int32(i))
	}
}

// Nearest is the index of the sample nearest p: the buckets in rings
// around p's, until no unseen sample could be nearer than the best.
func (l *TrailLine) Nearest(p mgl32.Vec2) int {
	best, bi := float32(-1), 0
	c := trailLineKey(p)
	if c[0] < l.bucketLo[0]-2 || c[0] > l.bucketHi[0]+2 || c[1] < l.bucketLo[1]-2 || c[1] > l.bucketHi[1]+2 {
		// Well away from the line: the rings would be mostly empty.
		for i, s := range l.Samples {
			if d := s.Pos.Sub(p).LenSqr(); best < 0 || d < best {
				best, bi = d, i
			}
		}
		return bi
	}
	// The ring past which every bucket lies outside the line's box.
	last := max(abs32i(c[0]-l.bucketLo[0]), abs32i(c[0]-l.bucketHi[0]),
		abs32i(c[1]-l.bucketLo[1]), abs32i(c[1]-l.bucketHi[1]))
	for r := int32(0); r <= last; r++ {
		for x := c[0] - r; x <= c[0]+r; x++ {
			for z := c[1] - r; z <= c[1]+r; z++ {
				if x != c[0]-r && x != c[0]+r && z != c[1]-r && z != c[1]+r {
					continue // inside the ring: seen already
				}
				for _, i := range l.buckets[[2]int32{x, z}] {
					if d := l.Samples[i].Pos.Sub(p).LenSqr(); best < 0 || d < best {
						best, bi = d, int(i)
					}
				}
			}
		}
		// Every sample past ring r is at least r buckets from p.
		if reach := float32(r) * trailLineBucket; best >= 0 && best <= reach*reach {
			break
		}
	}
	return bi
}

func abs32i(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// AtAlong is the index of the sample nearest d metres along the line.
func (l *TrailLine) AtAlong(d float32) int {
	lo, hi := 0, len(l.Along)-1
	for lo < hi {
		mid := (lo + hi) / 2
		if l.Along[mid] < d {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// TrailLine is trail id's centre line, nil for an area or an unknown
// trail.
func (w *World) TrailLine(id uint64) *TrailLine {
	return w.trailLines[id]
}

// rebuildTrailLines refills the centre-line cache.
func (w *World) rebuildTrailLines() {
	w.trailLines = make(map[uint64]*TrailLine, len(w.Trails))
	for _, t := range w.Trails {
		s := t.Centerline()
		if len(s) < 2 {
			continue
		}
		l := &TrailLine{Samples: s, Along: make([]float32, len(s))}
		for i := 1; i < len(s); i++ {
			l.Along[i] = l.Along[i-1] + s[i].Pos.Sub(s[i-1].Pos).Len()
		}
		l.index()
		w.trailLines[t.ID] = l
	}
}
