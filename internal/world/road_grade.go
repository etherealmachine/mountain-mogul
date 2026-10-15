package world

import (
	"fmt"

	"mountain-mogul/internal/edition"

	"github.com/go-gl/mathgl/mgl32"
)

// Roads and footpaths on a grade (notes/next/Trains and Tunnels.md): a
// road edge or a path laid by the player is held to a grade like a
// railway (grade.go): the ground cut and filled to it, and where the cut
// would run deeper than its tunnel depth it tunnels instead, the hill
// left standing over it. Each keeps its profile (Track); one laid before
// profiles, or made in passing (a lot's driveway, an editor stub), lies
// on the ground as it is. Roads, paths and whoever's on them are still
// drawn on the ground, which the grading has brought to the profile;
// the profile marks the tunnels, where they're hidden. The demo has no
// tunnels: its roads and paths lie on the ground as they always have.

const (
	// RoadMaxGrade and FootpathMaxGrade are the steepest a road and a
	// path climb, rise over run: a mountain road's 12%, a path's ramps
	// and steps at 25%.
	RoadMaxGrade     = float32(0.12)
	FootpathMaxGrade = float32(0.25)
	// FootpathTunnelDepth is the cut from which a path tunnels: its
	// tunnel is smaller than a road's (TunnelDepth).
	FootpathTunnelDepth = float32(8)
	// RoadTunnelCostPerM and FootpathTunnelCostPerM are a metre of
	// tunnel, on top of the road or path.
	RoadTunnelCostPerM     = 8_000
	FootpathTunnelCostPerM = 3_000
)

// roadBed is a road's graded bed: the carriageway and half a metre
// either side, shoulders banked out to the ground (trees are cleared by
// the plowing, road_cells.go).
var roadBed = BedSpec{Half: RoadHalfWidth + 0.5, Bank: 1.5, CutBank: 0.75, ShoulderMax: 20, PortalTop: 8.4, PortalHalf: 9.5}

// footpathBed is a path's, for a path half wide.
func footpathBed(half float32) BedSpec {
	return BedSpec{Half: half + 0.3, Bank: 1.5, CutBank: 1, ShoulderMax: 10, PortalTop: 5.6, PortalHalf: 6.6}
}

// RoadLine is a road from a to b laid out on its grade, with its tunnels.
func (w *World) RoadLine(a, b mgl32.Vec2) []RailSample {
	if edition.Demo {
		return onGround([]mgl32.Vec2{a, b}, w.Terrain, 0)
	}
	return layOutGraded([]mgl32.Vec2{a, b}, w.Terrain, 0, RoadMaxGrade, TunnelDepth)
}

// RoadGradeProblem is why a road can't run from a to b, or "": its ends
// stay at their ground, so between them it can climb no faster than
// RoadMaxGrade.
func (w *World) RoadGradeProblem(a, b mgl32.Vec2) string {
	return w.gradeProblem("road", a, b, RoadMaxGrade)
}

// FootpathGradeProblem is why a path can't run through nodes, or "": as
// a road's, between each pair of its nodes.
func (w *World) FootpathGradeProblem(nodes []TrailNode) string {
	for i := 1; i < len(nodes); i++ {
		if why := w.gradeProblem("path", nodes[i-1].Pos, nodes[i].Pos, FootpathMaxGrade); why != "" {
			return why
		}
	}
	return ""
}

func (w *World) gradeProblem(what string, a, b mgl32.Vec2, grade float32) string {
	t := w.Terrain
	run := b.Sub(a).Len()
	if edition.Demo || t == nil || run < 1 {
		return ""
	}
	rise := abs32(t.InterpolatedGroundElevationAt(b[0], b[1]) - t.InterpolatedGroundElevationAt(a[0], a[1]))
	if rise/run > grade {
		return fmt.Sprintf("Too steep: a %s climbs at most %.0f%%, this one %.0f%%. Wind it across the slope.", what, grade*100, rise/run*100)
	}
	return ""
}

// FootpathLine is a path through nodes laid out on its grade.
func (w *World) FootpathLine(nodes []TrailNode) []RailSample {
	if edition.Demo {
		return onGround(centrePoints(smoothLine(nodes)), w.Terrain, 0)
	}
	return layOutGraded(centrePoints(smoothLine(nodes)), w.Terrain, 0, FootpathMaxGrade, FootpathTunnelDepth)
}

func centrePoints(line []TrailSample) []mgl32.Vec2 {
	pts := make([]mgl32.Vec2, len(line))
	for i, s := range line {
		pts[i] = s.Pos
	}
	return pts
}

// EdgeTrack is road edge e's profile: on the ground if it has none for
// where its ends are now.
func (w *World) EdgeTrack(e *RoadEdge) []RailSample {
	a, b := w.RoadNodeByID(e.A), w.RoadNodeByID(e.B)
	if a == nil || b == nil || w.Terrain == nil {
		return nil
	}
	if e.Track == nil || e.trackFrom != [2]mgl32.Vec2{a.Pos, b.Pos} {
		e.Track = onGround([]mgl32.Vec2{a.Pos, b.Pos}, w.Terrain, 0)
		e.trackFrom = [2]mgl32.Vec2{a.Pos, b.Pos}
	}
	return e.Track
}

// EdgeMoved reports whether road edge e's ends have moved since its
// profile was laid out.
func (w *World) EdgeMoved(e *RoadEdge) bool {
	a, b := w.RoadNodeByID(e.A), w.RoadNodeByID(e.B)
	return a != nil && b != nil && e.trackFrom != [2]mgl32.Vec2{a.Pos, b.Pos}
}

// PathTrack is footpath f's profile: on the ground if it has none for
// its nodes as they are now.
func (w *World) PathTrack(f *Footpath) []RailSample {
	if w.Terrain == nil {
		return nil
	}
	if f.Track == nil || !sameNodes(f.trackNodes, f.Nodes) {
		f.Track = onGround(centrePoints(f.Centerline()), w.Terrain, 0)
		f.trackNodes = append([]TrailNode(nil), f.Nodes...)
	}
	return f.Track
}

func sameNodes(a, b []TrailNode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GradeRoadEdge lays road edge e out on its grade and grades the ground
// to it, leaving the cells keep says to (a building's pad).
func (w *World) GradeRoadEdge(e *RoadEdge, keep func(x, z int) bool) {
	a, b := w.RoadNodeByID(e.A), w.RoadNodeByID(e.B)
	if a == nil || b == nil || w.Terrain == nil {
		return
	}
	e.Track = w.RoadLine(a.Pos, b.Pos)
	e.trackFrom = [2]mgl32.Vec2{a.Pos, b.Pos}
	if edition.Demo {
		return
	}
	w.Terrain.GradeBed(e.Track, roadBed, keep)
	w.RebuildTunnels()
}

// GradeFootpath lays path f out on its grade and grades the ground to
// it, as GradeRoadEdge.
func (w *World) GradeFootpath(f *Footpath, keep func(x, z int) bool) {
	if w.Terrain == nil {
		return
	}
	f.Track = w.FootpathLine(f.Nodes)
	f.trackNodes = append([]TrailNode(nil), f.Nodes...)
	if edition.Demo {
		return
	}
	w.Terrain.GradeBed(f.Track, footpathBed(pathHalf(f.Nodes)), keep)
	w.RebuildTunnels()
}

// SetEdgeProfile puts a saved road edge's heights and tunnels back.
func (w *World) SetEdgeProfile(e *RoadEdge, heights []float32, tunnels [][2]int) {
	a, b := w.RoadNodeByID(e.A), w.RoadNodeByID(e.B)
	if a == nil || b == nil || len(heights) == 0 {
		return
	}
	track := sampleLine([]mgl32.Vec2{a.Pos, b.Pos})
	if setProfile(track, heights, tunnels) {
		e.Track, e.trackFrom = track, [2]mgl32.Vec2{a.Pos, b.Pos}
	}
}

// SetPathProfile puts a saved path's heights and tunnels back.
func (w *World) SetPathProfile(f *Footpath, heights []float32, tunnels [][2]int) {
	if len(heights) == 0 {
		return
	}
	track := sampleLine(centrePoints(f.Centerline()))
	if setProfile(track, heights, tunnels) {
		f.Track, f.trackNodes = track, append([]TrailNode(nil), f.Nodes...)
	}
}

// Profile is a laid-out line's heights and tunnel spans, to save: nil
// when it has no tunnels (it's then as good as on the ground).
func Profile(track []RailSample) (heights []float32, tunnels [][2]int) {
	if tunnelLength(track) == 0 {
		return nil, nil
	}
	return profileOf(track)
}

// TunnelCost is what the tunnels on a road (road) or path's track cost.
func TunnelCost(track []RailSample, road bool) int {
	if road {
		return int(tunnelLength(track) * RoadTunnelCostPerM)
	}
	return int(tunnelLength(track) * FootpathTunnelCostPerM)
}

// splitTrack is the profile of the piece of a line along track from
// from to to (metres along it), laid along a fresh line between the two
// points: a split road edge keeps its grade and tunnels.
func splitTrack(track []RailSample, a, b mgl32.Vec2, from float32) []RailSample {
	out := sampleLine([]mgl32.Vec2{a, b})
	for i := range out {
		d := from + out[i].Dist
		_, out[i].Y, _, _ = trackAt(track, d)
		k := min(max(int(d/RailSampleStep+0.5), 0), len(track)-1)
		out[i].Tunnel = track[k].Tunnel
	}
	return out
}

// Tunnel is a stretch of road or path under the hill: its course (as a
// bridge's, for the same nearest-point queries), and its mouths.
type Tunnel struct {
	Course Bridge
	Mouths []TunnelPortal
}

// RebuildTunnels remakes the roads' and paths' tunnels from their
// profiles.
func (w *World) RebuildTunnels() {
	var out []Tunnel
	for _, e := range w.RoadEdges {
		out = append(out, TunnelsAlong(w.EdgeTrack(e), RoadHalfWidth, true)...)
	}
	for _, f := range w.Footpaths {
		out = append(out, TunnelsAlong(w.PathTrack(f), pathHalf(f.Nodes), false)...)
	}
	w.Tunnels = out
	w.TunnelsRev++
}

// HalfWidth is half path f's widest width.
func (f *Footpath) HalfWidth() float32 { return pathHalf(f.Nodes) }

// pathHalf is half a path's widest width.
func pathHalf(nodes []TrailNode) float32 {
	var half float32
	for _, n := range nodes {
		half = max(half, n.Width/2)
	}
	return half
}

// tunnelRuns is track's tunnels, as sample index spans.
func tunnelRuns(track []RailSample) [][2]int {
	deep := make([]bool, len(track))
	for i, s := range track {
		deep[i] = s.Tunnel
	}
	return runsOf(deep)
}

// InTunnel reports whether p is in a road's tunnel (road) or a path's.
func (w *World) InTunnel(p mgl32.Vec2, road bool) bool {
	return InTunnelAmong(w.Tunnels, p, road)
}

// InTunnelAmong is InTunnel over tunnels: within a metre of a course's
// side, and between its mouths.
func InTunnelAmong(tunnels []Tunnel, p mgl32.Vec2, road bool) bool {
	for i := range tunnels {
		c := &tunnels[i].Course
		if c.Road != road || !c.near(p, c.Half+1) {
			continue
		}
		if along, off := c.Nearest(p); off <= c.Half+1 && along > 0 && along < c.Length() {
			return true
		}
	}
	return false
}

// TunnelsAlong is the tunnels on a laid-out road (road) or path line,
// half wide: each course runs from mouth to mouth (or the line's end).
func TunnelsAlong(track []RailSample, half float32, road bool) []Tunnel {
	var out []Tunnel
	for _, r := range tunnelRuns(track) {
		tn := Tunnel{Course: Bridge{Road: road, Half: half}}
		from, to := track[r[0]].Dist, track[r[1]].Dist
		mouth := func(i int, out bool) {
			// Between the last sample outside and the first in, facing
			// into the tunnel.
			d := (track[i-1].Dist + track[i].Dist) / 2
			pos, y, heading, _ := trackAt(track, d)
			if out {
				heading += 3.14159265
			}
			tn.Mouths = append(tn.Mouths, TunnelPortal{Pos: pos, Y: y, Heading: heading})
		}
		if r[0] > 0 {
			mouth(r[0], false)
			from = (track[r[0]-1].Dist + track[r[0]].Dist) / 2
		}
		if r[1] < len(track)-1 {
			mouth(r[1]+1, true)
			to = (track[r[1]].Dist + track[r[1]+1].Dist) / 2
		}
		for d := from; ; d += RailSampleStep {
			d = min(d, to)
			pos, _, _, _ := trackAt(track, d)
			tn.Course.Points = append(tn.Course.Points, pos)
			tn.Course.Along = append(tn.Course.Along, d-from)
			if d >= to {
				break
			}
		}
		if len(tn.Course.Points) >= 2 {
			out = append(out, tn)
		}
	}
	return out
}
