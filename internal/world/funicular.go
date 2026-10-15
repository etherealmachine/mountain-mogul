package world

import (
	"mountain-mogul/internal/edition"

	"github.com/go-gl/mathgl/mgl32"
)

// The funicular (notes/next/Trains and Tunnels.md): two counterbalanced
// cars on a rail up the slope, shuttling between its stations like the
// tram's cabins (tram.go, tramAt), but on the ground: one track at a
// steady grade from station to station, cut and filled into the slope
// and tunnelled where it runs deep under it (a RailLine's machinery,
// FunicularTrack), the cars passing on a loop in the middle. On rails,
// it never holds for the wind.

const (
	FunicularCapacity = 100
	// FunicularLoopGap is how far each car swings off the line on the
	// passing loop, FunicularLoopHalf the loop's half-length (with
	// FunicularLoopEase more to swing out over).
	FunicularLoopGap  = float32(1.8)
	FunicularLoopHalf = float32(30)
	FunicularLoopEase = float32(20)
	// FunicularStationLen is how far a station's platform and roof
	// reach along the track from its end.
	FunicularStationLen = float32(14)
	// FunicularMinLength is the shortest funicular.
	FunicularMinLength = float32(150)
)

// Funicular reports whether lifts of this type are a funicular.
func (t LiftType) Funicular() bool { return !edition.Demo && t == LiftFunicular }

// Shuttle reports whether lifts of this type are two cars shuttling
// between their stations (the tram, the funicular), not a loop.
func (t LiftType) Shuttle() bool { return t.Tram() || t.Funicular() }

// funicularStation is a funicular's station: the end of the track, the
// car docked LoadAhead up it, a stepped platform beside it on the
// riders' side under a roof, drawn with the track (render/funicular.go).
// It stands on the slope; nothing's levelled.
var funicularStation = StationGeom{
	LoadAhead: 7,
	Flat:      FunicularStationLen,
	operator:  [2]float32{-1, 2.6},
	rects:     []stationRect{{front: -FunicularStationLen, rear: 1, in: -2, out: 4.5}},
}

// FunicularTrack is funicular l's track: a straight line from its base
// to its top at one steady grade, through its stations too (they're
// built on the slope, their platforms stepped beside the track), its
// tunnels marked; laid out on first use and kept (ResetTowerCache drops
// it). The ground at each end sets the grade, and grading the bed
// (GradeRailBed) leaves it there, so it lays out the same again.
func (l *Lift) FunicularTrack(t *Terrain) *RailLine {
	if !l.Type.Funicular() || t == nil {
		return nil
	}
	if l.funicTrack != nil {
		return l.funicTrack
	}
	tr := &RailLine{}
	span := l.Top.Sub(l.Base)
	length := span.Len()
	dir := span.Mul(1 / max(length, 1e-6))
	ya := t.InterpolatedGroundElevationAt(l.Base[0], l.Base[1]) + RailBallast
	yb := t.InterpolatedGroundElevationAt(l.Top[0], l.Top[1]) + RailBallast
	for d := float32(0); ; d += RailSampleStep {
		d = min(d, length)
		y := ya + (yb-ya)*d/max(length, 1e-6)
		tr.Track = append(tr.Track, RailSample{Pos: l.Base.Add(dir.Mul(d)), Y: y, Dist: d})
		if d >= length {
			break
		}
	}
	tr.markTunnels(t)
	l.funicTrack = tr
	return tr
}

// FunicularLoopOffset is how far a car at x metres along the line swings
// off it on the passing loop, for the car on track side side.
func (l *Lift) FunicularLoopOffset(x, side float32) float32 {
	mid := l.cableLen() / 2
	d := abs32(x - mid)
	k := clamp01((FunicularLoopHalf + FunicularLoopEase - d) / FunicularLoopEase)
	k = k * k * (3 - 2*k)
	return side * FunicularLoopGap * k
}

// PlacePitch is how far chair i is pitched nose up, radians: a
// funicular's car on its grade; 0 for any other lift's.
func (l *Lift) PlacePitch(i int, t *Terrain) float32 {
	tr := l.FunicularTrack(t)
	if tr == nil || i >= len(l.Chairs) {
		return 0
	}
	_, _, x := l.tramAt(l.Chairs[i].Progress, tramSide(i))
	_, _, _, p := tr.At(x)
	return p
}

// PlaceHidden reports whether chair i is out of sight: a funicular's car
// in a tunnel.
func (l *Lift) PlaceHidden(i int, t *Terrain) bool {
	tr := l.FunicularTrack(t)
	if tr == nil || i >= len(l.Chairs) || len(tr.Track) == 0 {
		return false
	}
	pos, _, _ := l.tramAt(l.Chairs[i].Progress, tramSide(i))
	return l.Underground(pos, t)
}

// Underground reports whether p, on funicular l's line, is in one of its
// tunnels: a car or its riders out of sight.
func (l *Lift) Underground(p mgl32.Vec2, t *Terrain) bool {
	tr := l.FunicularTrack(t)
	if tr == nil || len(tr.Track) == 0 {
		return false
	}
	dir, _ := l.lineAxes()
	x := p.Sub(l.Base).Dot(dir)
	k := min(max(int(x/RailSampleStep+0.5), 0), len(tr.Track)-1)
	return tr.Track[k].Tunnel
}
