package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// A station's ground (notes/next/Rail Network.md): a level pad on its
// side of the track, as long as its platform and out past its house, at
// the track's bed (where the platform and house stand), banked out to
// the ground like the track's bed (railBed).

const (
	// stationPadOut is how far the pad reaches from the track's middle
	// on the station's side: past the house (rail_station.py, set back
	// StationPlatformSide + 7.35 m, reaching 9.5 m behind that).
	stationPadOut = float32(22)
	// stationPadHalf is the pad's least half-length: the house's 9 m and
	// a little.
	stationPadHalf = float32(11)
	// stationShoulder is the most its banks reach past the pad.
	stationShoulder = float32(15)
)

// GradeStations grades every station's pad (GradeStation).
func (w *World) GradeStations(keep func(x, z int) bool) {
	for _, st := range w.RailStations {
		w.GradeStation(st, keep)
	}
}

// GradeStation levels station st's pad to its track's bed, cut and
// filled, banked out to the ground, and clears its trees; the cells keep
// says to keep (another building's pad) stay.
func (w *World) GradeStation(st *RailStation, keep func(x, z int) bool) {
	tr := w.RailTrackByID(st.TrackID)
	pos, _, heading, side, ok := w.StationFrame(st)
	if tr == nil || !ok {
		return
	}
	_, of := w.RailNetworks()
	half := max(of[tr.ID].Train.Spec().PlatformLength()/2, stationPadHalf)
	along := mgl32.Vec2{float32(math.Sin(float64(heading))), float32(math.Cos(float64(heading)))}
	reach := stationPadOut + stationShoulder + half
	t := w.Terrain
	x0, z0 := int((pos[0]-reach)/CellSize), int((pos[1]-reach)/CellSize)
	x1, z1 := int((pos[0]+reach)/CellSize)+1, int((pos[1]+reach)/CellSize)+1
	p := t.GroundPatch(x0, z0, x1, z1, 0)
	for j := 0; j < p.H; j++ {
		for i := 0; i < p.W; i++ {
			wx, wz := p.Pos(i, j)
			q := mgl32.Vec2{wx, wz}.Sub(pos)
			a, c := q.Dot(along), q.Dot(side)
			if c < 0 {
				continue // the far side of the track is the track's
			}
			// How far outside the pad, and the bed there (the track's,
			// along it, held at the pad's ends).
			da := max(float32(math.Abs(float64(a)))-half, 0)
			dc := max(c-stationPadOut, 0)
			d := float32(math.Hypot(float64(da), float64(dc)))
			if d > stationShoulder {
				continue
			}
			_, y, _, _ := tr.At(st.Dist + min(max(a, -half), half))
			bed := y - RailBallast
			k := j*p.W + i
			if d == 0 {
				p.After[k] = bed
				continue
			}
			if rise := p.Before[k] - bed; rise > 0 {
				p.After[k] = bed + min(rise, d/railBed.CutBank)
			} else {
				p.After[k] = bed + max(rise, -d/railBed.Bank)
			}
		}
	}
	p.Commit(keep)
	// Trees go from the pad, and wherever its banks moved the ground.
	t.RemoveTreesIn(x0, z0, x1, z1, func(tr Tree) bool {
		q := mgl32.Vec2{tr.X, tr.Z}.Sub(pos)
		if a, c := q.Dot(along), q.Dot(side); c >= 0 && c <= stationPadOut && float32(math.Abs(float64(a))) <= half {
			return true
		}
		i, j := p.index(mgl32.Vec2{tr.X, tr.Z})
		if i < 0 || j < 0 || i >= p.W || j >= p.H {
			return false
		}
		k := j*p.W + i
		return float32(math.Abs(float64(p.After[k]-p.Before[k]))) > 0.3
	})
}
