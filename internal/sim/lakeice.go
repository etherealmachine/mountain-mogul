package sim

import (
	"math"
	"time"

	"mountain-mogul/internal/world"
)

// Lake ice (see world.Lake): each lake's frost and thaw step daily with
// the air at its altitude, and each spot freezes, thickens, and opens
// again by its own depth, so the ice creeps out from the shore over weeks
// and opens from the shallows in spring.

// iceEdgeJitter is how much the depth each spot freezes at varies, as a
// fraction, so the ice front is ragged rather than following the
// estimated depth contours; iceEdgeScale is the size of its wiggles, m.
const (
	iceEdgeJitter = 0.3
	iceEdgeScale  = 12.0
)

// LakeSeason is a lake's frost, day by day, through a typical season up
// to a scenario's start date, from SeasonLakes.
type LakeSeason struct {
	Start time.Time   // 1 September
	Frost [][]float32 // per lake, per day from Start
}

// FrozeOn is the day the spot depth metres deep in lake i last froze,
// and false when it's open on the last day.
func (s *LakeSeason) FrozeOn(w *world.World, i int, depth float32) (time.Time, bool) {
	need := world.FreezeFrostPerM * depth
	f := s.Frost[i]
	if len(f) == 0 || f[len(f)-1] < need {
		return time.Time{}, false
	}
	d := len(f) - 1
	for d > 0 && f[d-1] >= need {
		d--
	}
	return s.Start.AddDate(0, 0, d), true
}

// SeasonLakes runs w's lakes through a typical season from 1 September
// to until, from the climate's daily means, so a scenario starts with
// the ice its date would have. Without a climate it does nothing and
// returns nil.
func SeasonLakes(w *world.World, until time.Time) *LakeSeason {
	c := w.Climate
	if c == nil || len(w.Lakes) == 0 {
		return nil
	}
	year := until.Year()
	if until.Month() < time.September {
		year--
	}
	s := &LakeSeason{Start: time.Date(year, time.September, 1, 0, 0, 0, 0, time.UTC), Frost: make([][]float32, len(w.Lakes))}
	for i := range w.Lakes {
		l := &w.Lakes[i]
		lapse := world.LapseRate * (l.Altitude - c.RefAltitude)
		l.Frost, l.Thaw = 0, 0
		for d := s.Start; d.Before(until); d = d.AddDate(0, 0, 1) {
			l.Step(climateOn(c, d).TempMean - lapse)
			s.Frost[i] = append(s.Frost[i], l.Frost)
		}
	}
	return s
}

// edgeNoise is smooth value noise in 0..1 at world (x, z), for the
// ragged ice edge.
func edgeNoise(x, z float32) float32 {
	gx, gz := x/iceEdgeScale, z/iceEdgeScale
	x0, z0 := int(math.Floor(float64(gx))), int(math.Floor(float64(gz)))
	fx, fz := gx-float32(x0), gz-float32(z0)
	fx, fz = fx*fx*(3-2*fx), fz*fz*(3-2*fz)
	at := func(i, j int) float32 {
		h := uint32(int32(i))*0x8da6b343 ^ uint32(int32(j))*0xd8163841
		h ^= h >> 13
		h *= 0x5bd1e995
		h ^= h >> 15
		return float32(h) / float32(math.MaxUint32)
	}
	a := at(x0, z0)*(1-fx) + at(x0+1, z0)*fx
	b := at(x0, z0+1)*(1-fx) + at(x0+1, z0+1)*fx
	return a*(1-fz) + b*fz
}

// spotDepth is the depth a spot freezes and thaws as: its estimated depth,
// jittered by edgeNoise so the ice front is ragged.
func spotDepth(t *world.Terrain, x, z float32) float32 {
	return t.LakeDepthAt(x, z) * (1 + iceEdgeJitter*(2*edgeNoise(x, z)-1))
}

// ApplyLakes shows each lake's ice spot by spot: its surface in the
// material map (ice, thin ice, or open water), and no snow on cells
// whose water is open, where it falls in.
func ApplyLakes(w *world.World) {
	t, m := w.Terrain, w.Terrain.Material
	if m == nil || t.LakeOf == nil || t.LakeDepth == nil || len(w.Lakes) == 0 {
		return
	}
	const per = world.CellSize / world.DetailPerCell
	changed := false
	for k, v := range m.M {
		if !v.IsWater() {
			continue
		}
		i, j := k%m.W, k/m.W
		x := min(i/world.DetailPerCell, t.Width-1)
		z := min(j/world.DetailPerCell, t.Height-1)
		id := t.LakeOf[x*t.Height+z]
		if id == 0 {
			continue
		}
		wx, wz := float32(i)*per, float32(j)*per
		if s := w.Lakes[id-1].SurfaceAt(spotDepth(t, wx, wz)); s != v {
			m.M[k] = s
			changed = true
		}
	}
	if changed {
		m.Version++
	}
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			id := t.LakeOf[x*t.Height+z]
			if id == 0 {
				continue
			}
			cx, cz := (float32(x)+0.5)*world.CellSize, (float32(z)+0.5)*world.CellSize
			if w.Lakes[id-1].IceAt(spotDepth(t, cx, cz)) > 0 {
				continue
			}
			c := &t.Cells[x][z]
			if c.Base > 0 || c.Top.Accumulation > 0 {
				c.Base, c.Top = 0, world.SnowLayer{}
				t.MarkSnowDirty(x, z)
			}
		}
	}
}

// stepLakes advances every lake by today's weather and shows the result.
func (s *Simulation) stepLakes(dw DayWeather) {
	w := s.World
	if len(w.Lakes) == 0 {
		return
	}
	for i := range w.Lakes {
		l := &w.Lakes[i]
		l.Step(dw.TempC - world.LapseRate*(l.Altitude-w.BaseAltitude))
	}
	ApplyLakes(w)
}
