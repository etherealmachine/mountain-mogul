package world

import "github.com/go-gl/mathgl/mgl32"

// TrailLine is a run's centre line with the distance along it to each
// sample, built when trails change (rebuildTrailAt) so skiers can read
// it every step, from any goroutine, without rebuilding the curve.
type TrailLine struct {
	Samples []TrailSample
	Along   []float32 // metres from the first sample
}

// Nearest is the index of the sample nearest p.
func (l *TrailLine) Nearest(p mgl32.Vec2) int {
	best, bi := float32(-1), 0
	for i, s := range l.Samples {
		if d := s.Pos.Sub(p).LenSqr(); best < 0 || d < best {
			best, bi = d, i
		}
	}
	return bi
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
		w.trailLines[t.ID] = l
	}
}
