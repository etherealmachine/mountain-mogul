package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Rail lines (notes/next/Trains and Tunnels.md): a line from the map's
// edge to a station, in one of three styles that differ in cost, speed
// and how many they carry. The player lays it point by point like a
// road; it's smoothed into a curve, held to its style's grade limit by
// cutting and filling the ground, and ends at a platform as long as its
// train.

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

// RailLine is one line.
type RailLine struct {
	ID    uint64
	Style RailStyle
	// Nodes is the points it was laid through, from the map edge to the
	// station's far end.
	Nodes []mgl32.Vec2

	// StationID is its station's building (BuildingStation), where its
	// guests arrive and leave; Interval the clock minutes between trains.
	StationID uint64
	Interval  float32

	// Track is the laid-out line (LayOut): every RailSampleStep along
	// it, with the track's height. Not saved.
	Track []RailSample
	// Train is its train, and Bound the guests booked on the next one
	// in (sim/trains.go). Not saved.
	Train Train
	Bound [][]*Guest
}

// Train is a line's one train: away off the map, coming in, standing at
// the platform, or going out again; Front is how far along the line its
// front is, metres, and Speed its speed.
type Train struct {
	State      TrainState
	Front      float32
	Speed      float32
	DwellUntil float64 // sim time it leaves the platform
	NextAt     float64 // sim time the next one sets off for the resort
}

// TrainState is where a line's train is.
type TrainState uint8

const (
	TrainAway TrainState = iota
	TrainInbound
	TrainDwelling
	TrainOutbound
)

// DefaultRailInterval is a new line's clock minutes between trains.
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

// RailLineCost is what laying line l (laid out) costs: its track, its
// tunnels, and its station.
func RailLineCost(l *RailLine) int {
	s := l.Style.Spec()
	var track, tunnel float32
	for i := 1; i < len(l.Track); i++ {
		d := l.Track[i].Dist - l.Track[i-1].Dist
		if l.Track[i].Tunnel {
			tunnel += d
		} else {
			track += d
		}
	}
	return int(track)*s.CostPerM + int(tunnel)*TunnelCostPerM + s.StationCost
}

// LayOut smooths the line through its nodes and works out the track's
// height: as near the ground as its grade limit allows, level along the
// platform at the station end.
func (l *RailLine) LayOut(t *Terrain) {
	l.Track = l.Track[:0]
	if len(l.Nodes) < 2 {
		return
	}
	nodes := make([]TrailNode, len(l.Nodes))
	for i, p := range l.Nodes {
		nodes[i] = TrailNode{Pos: p}
	}
	curve := smoothLine(nodes)
	// Resample evenly.
	var dist float32
	next := float32(0)
	for i := 1; i < len(curve); i++ {
		a, b := curve[i-1].Pos, curve[i].Pos
		seg := b.Sub(a).Len()
		for next <= dist+seg {
			p := a.Add(b.Sub(a).Mul((next - dist) / max(seg, 1e-6)))
			l.Track = append(l.Track, RailSample{Pos: p, Dist: next})
			next += RailSampleStep
		}
		dist += seg
	}
	if last := curve[len(curve)-1].Pos; len(l.Track) == 0 || l.Track[len(l.Track)-1].Pos != last {
		l.Track = append(l.Track, RailSample{Pos: last, Dist: dist})
	}
	n := len(l.Track)
	g := make([]float32, n)
	for i, s := range l.Track {
		g[i] = t.InterpolatedGroundElevationAt(s.Pos[0], s.Pos[1]) + RailBallast
	}
	// The platform: the last PlatformLength metres level, at their
	// ground's average.
	spec := l.Style.Spec()
	plat := 0
	var platY float32
	for i := n - 1; i >= 0 && l.Track[n-1].Dist-l.Track[i].Dist <= spec.PlatformLength(); i-- {
		plat++
		platY += g[i]
	}
	platY /= float32(max(plat, 1))
	y := append([]float32(nil), g...)
	for i := n - plat; i < n; i++ {
		y[i] = platY
	}
	// Hold the grade: relax toward the ground, then clamp each step to
	// the grade from its neighbours, forward and back, a few times.
	step := spec.MaxGrade * RailSampleStep
	for pass := 0; pass < 8; pass++ {
		for i := 1; i < n; i++ {
			if i >= n-plat {
				continue
			}
			y[i] = min(max(y[i], y[i-1]-step), y[i-1]+step)
		}
		for i := n - 2; i >= 0; i-- {
			if i >= n-plat {
				continue
			}
			y[i] = min(max(y[i], y[i+1]-step), y[i+1]+step)
		}
		if pass < 7 {
			for i := 1; i < n-plat-1; i++ {
				y[i] = 0.5*y[i] + 0.25*(y[i-1]+y[i+1])
			}
		}
	}
	for i := range l.Track {
		l.Track[i].Y = y[i]
	}
	l.markTunnels(t)
}

// At is where the track is d metres along the line: its position, height
// and heading (as Guest.Heading) and pitch (radians, up positive).
func (l *RailLine) At(d float32) (pos mgl32.Vec2, y, heading, pitch float32) {
	n := len(l.Track)
	if n == 0 {
		return
	}
	d = min(max(d, 0), l.Track[n-1].Dist)
	i := min(int(d/RailSampleStep), n-2)
	if n == 1 {
		return l.Track[0].Pos, l.Track[0].Y, 0, 0
	}
	a, b := l.Track[i], l.Track[i+1]
	f := (d - a.Dist) / max(b.Dist-a.Dist, 1e-6)
	pos = a.Pos.Add(b.Pos.Sub(a.Pos).Mul(f))
	y = a.Y + (b.Y-a.Y)*f
	dir := b.Pos.Sub(a.Pos)
	heading = float32(math.Atan2(float64(dir[0]), float64(dir[1])))
	pitch = float32(math.Atan2(float64(b.Y-a.Y), float64(max(dir.Len(), 1e-6))))
	return
}

// Station is the line's station: the middle of its platform, its
// heading along the track, and its height.
func (l *RailLine) Station() (pos mgl32.Vec2, y, heading float32) {
	d := l.Length() - l.Style.Spec().PlatformLength()/2
	pos, y, heading, _ = l.At(d)
	return
}

// GradeRailBed cuts and fills the ground under line l to its track: the
// bed level with the ballast's foot, and shoulders banked out to the
// natural ground; trees in the way go.
func (t *Terrain) GradeRailBed(l *RailLine) {
	if len(l.Track) < 2 {
		return
	}
	reach := RailBedHalf + RailShoulderMax
	minX, minZ := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxZ := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	for _, s := range l.Track {
		minX, maxX = min(minX, s.Pos[0]), max(maxX, s.Pos[0])
		minZ, maxZ = min(minZ, s.Pos[1]), max(maxZ, s.Pos[1])
	}
	x0, z0 := int((minX-reach)/CellSize), int((minZ-reach)/CellSize)
	x1, z1 := int((maxX+reach)/CellSize)+1, int((maxZ+reach)/CellSize)+1
	p := t.GroundPatch(x0, z0, x1, z1, 0)
	best := make([]float32, len(p.After))
	for i := range best {
		best[i] = math.MaxFloat32
	}
	bedY := make([]float32, len(p.After))
	// Each sample's ground takes the nearest point on the track's
	// segments (not its sample points, which leave the banks toothed).
	for n := 0; n+1 < len(l.Track); n++ {
		a, b := l.Track[n], l.Track[n+1]
		if a.Tunnel && b.Tunnel {
			continue // the hill stays
		}
		ci, cj := p.index(a.Pos)
		r := int(reach/p.Spacing) + 2
		ab := b.Pos.Sub(a.Pos)
		ab2 := max(ab.LenSqr(), 1e-6)
		for j := max(cj-r, 0); j <= min(cj+r, p.H-1); j++ {
			for i := max(ci-r, 0); i <= min(ci+r, p.W-1); i++ {
				wx, wz := p.Pos(i, j)
				q := mgl32.Vec2{wx, wz}
				f := clamp01(q.Sub(a.Pos).Dot(ab) / ab2)
				d := q.Sub(a.Pos.Add(ab.Mul(f))).Len()
				if k := j*p.W + i; d < best[k] {
					best[k], bedY[k] = d, a.Y+(b.Y-a.Y)*f-RailBallast
				}
			}
		}
	}
	for k, d := range best {
		if d > reach {
			continue
		}
		before := p.Before[k]
		if d <= RailBedHalf {
			p.After[k] = bedY[k]
			continue
		}
		allowed := (d - RailBedHalf) / RailBank
		p.After[k] = bedY[k] + min(max(before-bedY[k], -allowed), allowed)
	}
	p.Commit(nil)
	hw2 := (RailBedHalf + 6) * (RailBedHalf + 6) // and a clear strip either side
	t.RemoveTreesIn(x0, z0, x1, z1, func(tr Tree) bool {
		for _, s := range l.Track {
			if !s.Tunnel && (mgl32.Vec2{tr.X - s.Pos[0], tr.Z - s.Pos[1]}).LenSqr() <= hw2 {
				return true
			}
		}
		return false
	})
}

// index is the patch sample nearest world position w.
func (p *GroundPatch) index(w mgl32.Vec2) (int, int) {
	if p.t.Detail != nil {
		return int(w[0]/p.Spacing+0.5) - p.I0, int(w[1]/p.Spacing+0.5) - p.J0
	}
	return int(w[0]/CellSize) - p.I0, int(w[1]/CellSize) - p.J0
}

// PlaceRailLine lays a new line of style through nodes: laid out, the
// ground graded under it.
func (w *World) PlaceRailLine(style RailStyle, nodes []mgl32.Vec2) *RailLine {
	l := &RailLine{ID: w.NextID(), Style: style, Nodes: append([]mgl32.Vec2(nil), nodes...), Interval: DefaultRailInterval}
	l.LayOut(w.Terrain)
	w.Terrain.GradeRailBed(l)
	w.Terrain.RecomputeSlopes()
	w.RailLines = append(w.RailLines, l)
	w.PlowRails()
	st := &Building{ID: w.NextID(), Type: BuildingStation, Pos: l.PlatformSpot()}
	w.Buildings = append(w.Buildings, st)
	l.StationID = st.ID
	return l
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

// PlowRails clears the snow off every railway's bed: run when it's
// built and after each day's snow, as a plow train would.
func (w *World) PlowRails() {
	t := w.Terrain
	reach := RailPlowInner + RailPlowFalloff
	for _, l := range w.RailLines {
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
}

// PlatformSpot is where on line l's platform guests get off and wait:
// its middle, on the platform beside the track.
func (l *RailLine) PlatformSpot() mgl32.Vec2 {
	pos, _, heading := l.Station()
	s, c := float32(math.Sin(float64(heading))), float32(math.Cos(float64(heading)))
	return pos.Add(mgl32.Vec2{c, -s}.Mul(3.45))
}

// RailLineByStation is the line whose station is building id.
func (w *World) RailLineByStation(id uint64) *RailLine {
	for _, l := range w.RailLines {
		if l.StationID == id {
			return l
		}
	}
	return nil
}

// AddTrainCost charges a train's trip to the day's costs.
func (w *World) AddTrainCost(c int) { w.TrainsToday += float64(c) }

// SetProfile puts back a saved line's track heights and tunnel spans
// (sample indices) over its laid-out curve; ignored if they don't fit.
func (l *RailLine) SetProfile(heights []float32, tunnels [][2]int) {
	if len(heights) != len(l.Track) {
		return
	}
	for i := range l.Track {
		l.Track[i].Y, l.Track[i].Tunnel = heights[i], false
	}
	for _, r := range tunnels {
		for i := max(r[0], 0); i <= min(r[1], len(l.Track)-1); i++ {
			l.Track[i].Tunnel = true
		}
	}
}
