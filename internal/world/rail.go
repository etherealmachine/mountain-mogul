package world

import (
	"github.com/go-gl/mathgl/mgl32"
)

// Railways (notes/next/Rail Network.md): track laid by the player,
// joined at joints into a network that comes in from the map's edge at
// entries (rail_track.go), each stretch in one of three styles that differ
// in cost, speed and how many their trains carry. This file has the
// styles, a laid-out line's samples (RailLine, also a funicular's track),
// grading and plowing.

// RailStyle is a line's kind of train.
type RailStyle uint8

const (
	RailSteam     RailStyle = iota // an old tank engine and wooden coaches
	RailElectric                   // a German-style electric and double-deckers
	RailHighSpeed                  // a streamlined high-speed trainset
	RailStyles
)

// RailSpec is what a style is.
type RailSpec struct {
	Label string
	// Speed is the line speed, m/s; Capacity the riders a train carries.
	Speed    float32
	Capacity int
	// CostPerM is the track's price a metre, StationCost the station's,
	// and TripCost what each train costs to run.
	CostPerM, StationCost, TripCost int
	// MaxGrade is the steepest the track may climb (rise over run).
	MaxGrade float32
	// Engine and Coach are the models, Coaches how many coaches, and
	// EngineLen and CoachLen their lengths over the couplers, metres. A
	// high-speed set has an engine at each end (BothEnds).
	Engine, Coach       string
	Coaches             int
	EngineLen, CoachLen float32
	BothEnds            bool
	// Wired is overhead wire on masts along the line.
	Wired bool
}

var railSpecs = [RailStyles]RailSpec{
	RailSteam: {Label: "Steam railway", Speed: 11, Capacity: 150, CostPerM: 600, StationCost: 150_000, TripCost: 300,
		MaxGrade: 0.04, Engine: "steam_loco", Coach: "steam_coach", Coaches: 3, EngineLen: 9.6, CoachLen: 12.4},
	RailElectric: {Label: "Electric railway", Speed: 28, Capacity: 400, CostPerM: 1_500, StationCost: 300_000, TripCost: 600,
		MaxGrade: 0.04, Engine: "electric_loco", Coach: "electric_coach", Coaches: 4, EngineLen: 18.9, CoachLen: 26.8, Wired: true},
	RailHighSpeed: {Label: "High-speed railway", Speed: 70, Capacity: 600, CostPerM: 4_000, StationCost: 600_000, TripCost: 1_200,
		MaxGrade: 0.035, Engine: "hsr_power", Coach: "hsr_coach", Coaches: 6, EngineLen: 20.5, CoachLen: 24.8, BothEnds: true, Wired: true},
}

// Spec is the style's RailSpec.
func (s RailStyle) Spec() *RailSpec { return &railSpecs[min(s, RailStyles-1)] }

// TrainLength is a train's length, metres.
func (r *RailSpec) TrainLength() float32 {
	n := float32(1)
	if r.BothEnds {
		n = 2
	}
	return n*r.EngineLen + float32(r.Coaches)*r.CoachLen
}

// PlatformLength is a station's platform for the style: its train and
// some to spare.
func (r *RailSpec) PlatformLength() float32 { return r.TrainLength() + 10 }

// RailLine is a laid-out line: its samples every RailSampleStep, with
// the rail's height and its tunnels. A track's (RailTrack) and a
// funicular's (FunicularTrack).
type RailLine struct {
	Track []RailSample
}

// DefaultRailInterval is a new station's clock minutes between trains.
const DefaultRailInterval = 60

// RailSample is a point on a line's track.
type RailSample struct {
	Pos  mgl32.Vec2
	Y    float32 // top of the rail
	Dist float32 // metres from the line's start
	// Tunnel is under the hill here: the ground's left as it is, the
	// track and trains hidden (tunnels.go).
	Tunnel bool
}

// Rail bed and grading.
const (
	RailSampleStep = 2.0
	// RailBedHalf is half the graded bed's width; RailBallast the
	// ballast's depth under the top of the rail; RailBank the shoulder's
	// slope (run over rise) out to the natural ground, at most
	// RailShoulderMax wide.
	RailBedHalf     = float32(3.0)
	RailBallast     = float32(0.6)
	RailBank        = float32(1.5)
	RailShoulderMax = float32(25)
	// RailEdgeReach is how near the map's edge a line must start.
	RailEdgeReach = float32(25)
)

// Length is the line's track length, metres.
func (l *RailLine) Length() float32 {
	if len(l.Track) == 0 {
		return 0
	}
	return l.Track[len(l.Track)-1].Dist
}

// At is where the track is d metres along the line: its position, height
// and heading (as Guest.Heading) and pitch (radians, up positive).
func (l *RailLine) At(d float32) (pos mgl32.Vec2, y, heading, pitch float32) {
	return trackAt(l.Track, d)
}

// GradeRailBed cuts and fills the ground under line l to its track: the
// bed level with the ballast's foot, and shoulders banked out to the
// natural ground; trees in the way go.
func (t *Terrain) GradeRailBed(l *RailLine) { t.GradeBed(l.Track, railBed, nil) }

// railBed is a railway's bed: RailBedHalf either side, the ballast
// under the rail's top, banked RailBank out to RailShoulderMax, and trees
// cleared 6 m past the bed.
var railBed = BedSpec{Half: RailBedHalf, Base: RailBallast, Bank: RailBank, ShoulderMax: RailShoulderMax, Clear: 6}

// index is the patch sample nearest world position w.
func (p *GroundPatch) index(w mgl32.Vec2) (int, int) {
	if p.t.Detail != nil {
		return int(w[0]/p.Spacing+0.5) - p.I0, int(w[1]/p.Spacing+0.5) - p.J0
	}
	return int(w[0]/CellSize) - p.I0, int(w[1]/CellSize) - p.J0
}

// NearMapEdge reports whether p is within RailEdgeReach of the map's
// edge.
func (w *World) NearMapEdge(p mgl32.Vec2) bool {
	mx := float32(w.Terrain.Width) * CellSize
	mz := float32(w.Terrain.Height) * CellSize
	return p[0] < RailEdgeReach || p[1] < RailEdgeReach || mx-p[0] < RailEdgeReach || mz-p[1] < RailEdgeReach
}

// Plowing: cells whose centre is within RailPlowInner of a railway's
// track are cleared bare, and the snow comes back over the next
// RailPlowFalloff, so the cells' edges don't show (as roads are plowed).
const (
	RailPlowInner   = float32(6)
	RailPlowFalloff = float32(6)
)

// PlowRails clears the snow off every railway's bed, and every
// funicular's: run when it's built and after each day's snow, as a plow
// train would.
func (w *World) PlowRails() {
	for _, tr := range w.RailTracks {
		w.plowLine(&tr.RailLine)
	}
	for _, l := range w.Lifts {
		if tr := l.FunicularTrack(w.Terrain); tr != nil {
			w.plowLine(tr)
		}
	}
}

// plowLine clears the snow off line l's bed.
func (w *World) plowLine(l *RailLine) {
	t := w.Terrain
	reach := RailPlowInner + RailPlowFalloff
	near := map[[2]int]float32{}
	for n := 0; n+1 < len(l.Track); n++ {
		a, b := l.Track[n], l.Track[n+1]
		if a.Tunnel && b.Tunnel {
			continue
		}
		ab := b.Pos.Sub(a.Pos)
		ab2 := max(ab.LenSqr(), 1e-6)
		x0, z0 := int((min(a.Pos[0], b.Pos[0])-reach)/CellSize), int((min(a.Pos[1], b.Pos[1])-reach)/CellSize)
		x1, z1 := int((max(a.Pos[0], b.Pos[0])+reach)/CellSize), int((max(a.Pos[1], b.Pos[1])+reach)/CellSize)
		for x := x0; x <= x1; x++ {
			for z := z0; z <= z1; z++ {
				q := mgl32.Vec2{(float32(x) + 0.5) * CellSize, (float32(z) + 0.5) * CellSize}
				f := clamp01(q.Sub(a.Pos).Dot(ab) / ab2)
				d := q.Sub(a.Pos.Add(ab.Mul(f))).Len()
				k := [2]int{x, z}
				if old, ok := near[k]; !ok || d < old {
					near[k] = d
				}
			}
		}
	}
	for k, d := range near {
		if d >= reach || !t.InBounds(k[0], k[1]) {
			continue
		}
		c := &t.Cells[k[0]][k[1]]
		keep := clamp01((d - RailPlowInner) / RailPlowFalloff)
		if c.Base == 0 && c.Top.Accumulation == 0 {
			continue
		}
		c.Base *= keep
		c.Top.Accumulation *= keep
		if c.Top.Accumulation <= 0 {
			c.Top = SnowLayer{}
		}
		t.MarkSnowDirty(k[0], k[1])
	}
}

// AddTrainCost charges a train's trip to the day's costs.
func (w *World) AddTrainCost(c int) { w.TrainsToday += float64(c) }
