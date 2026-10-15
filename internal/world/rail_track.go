package world

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Track and joints (notes/next/Rail Network.md): the player lays track as
// lines of points, each in one style, smoothed into a curve and laid on
// its grade like a road (grade.go): cut and filled, tunnelled where it
// runs deep under a hill. Each end of a track is at a joint, where it
// meets other tracks (a junction), ends (a buffer stop), or comes in from
// beyond the map (an entry, on its edge). A track's ends sit at its
// joints' heights, so tracks meet level; between them it climbs no
// steeper than its style allows.

// RailJoint is where tracks meet, end, or enter the map.
type RailJoint struct {
	ID  uint64
	Pos mgl32.Vec2
	// Y is the top of the rail here, which every track ending here
	// meets.
	Y float32
	// Entry is on the map's edge: trains come from and go to beyond it.
	Entry bool
}

// RailTrack is one stretch of track between two joints.
type RailTrack struct {
	ID    uint64
	Style RailStyle
	// Points is the line it was laid through, the first and last at its
	// joints (Ends).
	Points []mgl32.Vec2
	Ends   [2]uint64
	// RailLine is it laid out (LayOutTrack): its samples, heights and
	// tunnels; laidFor is the points they were laid out for.
	RailLine
	laidFor []mgl32.Vec2
}

// RailJointReach is how near a joint a point counts as on it, and
// RailTrackReach how near a track's line.
const (
	RailJointReach = float32(8)
	RailTrackReach = float32(6)
	// RailMinTrack is the shortest stretch of track between two joints.
	RailMinTrack = float32(20)
)

// RailJointByID is joint id, or nil.
func (w *World) RailJointByID(id uint64) *RailJoint {
	for _, j := range w.RailJoints {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// RailTrackByID is track id, or nil.
func (w *World) RailTrackByID(id uint64) *RailTrack {
	for _, tr := range w.RailTracks {
		if tr.ID == id {
			return tr
		}
	}
	return nil
}

// TracksAt is the tracks ending at joint id.
func (w *World) TracksAt(id uint64) []*RailTrack {
	var out []*RailTrack
	for _, tr := range w.RailTracks {
		if tr.Ends[0] == id || tr.Ends[1] == id {
			out = append(out, tr)
		}
	}
	return out
}

// RailCurve is the smoothed line through points.
func RailCurve(points []mgl32.Vec2) []mgl32.Vec2 {
	nodes := make([]TrailNode, len(points))
	for i, p := range points {
		nodes[i] = TrailNode{Pos: p}
	}
	return centrePoints(smoothLine(nodes))
}

// EdgeSnap is p moved onto the map's edge, if it's within RailEdgeReach
// of it.
func (w *World) EdgeSnap(p mgl32.Vec2) (mgl32.Vec2, bool) {
	t := w.Terrain
	mx := float32(t.Width-1) * CellSize
	mz := float32(t.Height-1) * CellSize
	d := [4]float32{p[0], mx - p[0], p[1], mz - p[1]}
	best := 0
	for i := range d {
		if d[i] < d[best] {
			best = i
		}
	}
	if d[best] > RailEdgeReach {
		return p, false
	}
	switch best {
	case 0:
		p[0] = 0
	case 1:
		p[0] = mx
	case 2:
		p[1] = 0
	case 3:
		p[1] = mz
	}
	return p, true
}

// NewRailJoint is a joint at p (on the map's edge, an entry, if it's near
// it), the rail at the ballast over the ground there. Not yet added.
func (w *World) NewRailJoint(p mgl32.Vec2) *RailJoint {
	p, entry := w.EdgeSnap(p)
	return &RailJoint{Pos: p, Y: w.Terrain.InterpolatedGroundElevationAt(p[0], p[1]) + RailBallast, Entry: entry}
}

// AddRailJoint adds joint j, giving it an ID.
func (w *World) AddRailJoint(j *RailJoint) *RailJoint {
	j.ID = w.NextID()
	w.RailJoints = append(w.RailJoints, j)
	return j
}

// LayOutTrack lays track tr out between its joints: along its curve, its
// ends at their joints' heights, held to its style's grade between, its
// tunnels marked.
func (w *World) LayOutTrack(tr *RailTrack) {
	a, b := w.RailJointByID(tr.Ends[0]), w.RailJointByID(tr.Ends[1])
	if a == nil || b == nil || len(tr.Points) < 2 {
		tr.Track = nil
		return
	}
	tr.Points[0], tr.Points[len(tr.Points)-1] = a.Pos, b.Pos
	tr.Track = layTrack(w.Terrain, tr.Style, tr.Points, a.Y, b.Y)
	tr.laidFor = append(tr.laidFor[:0], tr.Points...)
}

// layTrack lays out a track of style through points from height ya to
// yb.
func layTrack(t *Terrain, style RailStyle, points []mgl32.Vec2, ya, yb float32) []RailSample {
	track := sampleLine(RailCurve(points))
	n := len(track)
	if n == 0 {
		return nil
	}
	y := make([]float32, n)
	for i, s := range track {
		y[i] = t.InterpolatedGroundElevationAt(s.Pos[0], s.Pos[1]) + RailBallast
	}
	y[0], y[n-1] = ya, yb
	holdGrade(y, func(i int) bool { return i == 0 || i == n-1 }, style.Spec().MaxGrade*RailSampleStep)
	for i := range track {
		track[i].Y = y[i]
	}
	markTunnelsIn(track, t, TunnelDepth)
	return track
}

// PreviewTrack is a track of style through points laid out, its ends at
// ya and yb, for a ghost and its price: nothing's built.
func (w *World) PreviewTrack(style RailStyle, points []mgl32.Vec2, ya, yb float32) *RailTrack {
	tr := &RailTrack{Style: style, Points: points}
	tr.Track = layTrack(w.Terrain, style, points, ya, yb)
	return tr
}

// TrackProblem says why a track of style through points, its ends at ya
// and yb, can't be laid, "" if it can: too short, or climbing faster
// than its style allows between its ends.
func TrackProblem(style RailStyle, points []mgl32.Vec2, ya, yb float32) string {
	var length float32
	curve := RailCurve(points)
	for i := 1; i < len(curve); i++ {
		length += curve[i].Sub(curve[i-1]).Len()
	}
	if length < RailMinTrack {
		return fmt.Sprintf("Too short: track runs at least %.0f m between joints", RailMinTrack)
	}
	spec := style.Spec()
	if g := abs32(yb-ya) / length; g > spec.MaxGrade {
		return fmt.Sprintf("Too steep: %s climbs at most %.1f%%, this %.1f%%. Lay it longer, or wind it.", lowerFirst(spec.Label), spec.MaxGrade*100, g*100)
	}
	return ""
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]+'a'-'A') + s[1:]
}

// TrackCost is what track tr costs to lay: a metre of its style's
// track, or of tunnel.
func TrackCost(tr *RailTrack) int {
	tunnel := tunnelLength(tr.Track)
	return int((tr.Length()-tunnel)*float32(tr.Style.Spec().CostPerM)) + int(tunnel*TunnelCostPerM)
}

// UpgradeCost is what upgrading track tr to style costs: the difference
// a metre of open track (its tunnels stay as they are).
func UpgradeCost(tr *RailTrack, style RailStyle) int {
	open := tr.Length() - tunnelLength(tr.Track)
	return int(open * float32(style.Spec().CostPerM-tr.Style.Spec().CostPerM))
}

// AddRailTrack lays a new track of style through points from joint a to
// joint b (both added), the ground graded under it, the cells keep says
// to (a building's) left be.
func (w *World) AddRailTrack(style RailStyle, points []mgl32.Vec2, a, b *RailJoint, keep func(x, z int) bool) *RailTrack {
	tr := &RailTrack{ID: w.NextID(), Style: style, Points: append([]mgl32.Vec2(nil), points...), Ends: [2]uint64{a.ID, b.ID}}
	w.LayOutTrack(tr)
	w.RailTracks = append(w.RailTracks, tr)
	w.GradeTrack(tr, keep)
	w.PlaceStations()
	return tr
}

// GradeTrack grades the ground to track tr and plows it.
func (w *World) GradeTrack(tr *RailTrack, keep func(x, z int) bool) {
	w.Terrain.GradeBed(tr.Track, railBed, keep)
	w.Terrain.RecomputeSlopes()
	w.plowLine(&tr.RailLine)
	w.RailRev++
}

// TrackNear is the track whose line passes nearest p within reach: the
// track, its sample nearest p, and how far p is from it; nil if none.
func (w *World) TrackNear(p mgl32.Vec2, reach float32) (*RailTrack, int, float32) {
	var best *RailTrack
	bestI, bestD := 0, reach
	for _, tr := range w.RailTracks {
		for i, s := range tr.Track {
			if d := s.Pos.Sub(p).Len(); d < bestD {
				best, bestI, bestD = tr, i, d
			}
		}
	}
	return best, bestI, bestD
}

// JointNear is the joint nearest p within reach, or nil, leaving out
// skip.
func (w *World) JointNear(p mgl32.Vec2, reach float32, skip uint64) *RailJoint {
	var best *RailJoint
	bestD := reach
	for _, j := range w.RailJoints {
		if d := j.Pos.Sub(p).Len(); j.ID != skip && d < bestD {
			best, bestD = j, d
		}
	}
	return best
}

// SplitTrack splits track tr at its sample i into two, meeting at a new
// joint there (returned): a junction for a track ending on it. Each half
// keeps its piece of the profile and its tunnels.
func (w *World) SplitTrack(tr *RailTrack, i int) *RailJoint {
	s := tr.Track[min(max(i, 0), len(tr.Track)-1)]
	j := w.AddRailJoint(&RailJoint{Pos: s.Pos, Y: s.Y})
	// The control segment the split falls in: the one nearest the point.
	k, best := 0, float32(math.MaxFloat32)
	for n := 0; n+1 < len(tr.Points); n++ {
		if d, _ := segDist(s.Pos, tr.Points[n], tr.Points[n+1]); d < best {
			k, best = n, d
		}
	}
	first := append(append([]mgl32.Vec2(nil), tr.Points[:k+1]...), s.Pos)
	second := append([]mgl32.Vec2{s.Pos}, tr.Points[k+1:]...)
	t1 := &RailTrack{ID: w.NextID(), Style: tr.Style, Points: first, Ends: [2]uint64{tr.Ends[0], j.ID}}
	t2 := &RailTrack{ID: w.NextID(), Style: tr.Style, Points: second, Ends: [2]uint64{j.ID, tr.Ends[1]}}
	for _, half := range []*RailTrack{t1, t2} {
		half.Track = carryProfile(tr.Track, sampleLine(RailCurve(half.Points)))
		half.laidFor = append([]mgl32.Vec2(nil), half.Points...)
	}
	w.removeTrack(tr.ID)
	w.RailTracks = append(w.RailTracks, t1, t2)
	w.PlaceStations()
	w.RailRev++
	return j
}

// carryProfile gives each sample of fresh the height and tunnel of the
// nearest sample of old.
func carryProfile(old, fresh []RailSample) []RailSample {
	k := 0
	for i := range fresh {
		best := float32(math.MaxFloat32)
		for n := max(k-20, 0); n < len(old); n++ {
			if d := old[n].Pos.Sub(fresh[i].Pos).LenSqr(); d < best {
				best, k = d, n
			} else if d > best+400 {
				break
			}
		}
		fresh[i].Y, fresh[i].Tunnel = old[k].Y, old[k].Tunnel
	}
	return fresh
}

// MergeJoints joins joint from into joint into: from's tracks end at
// into instead (laid out again), a track between the two goes, and from
// is removed.
func (w *World) MergeJoints(from, into *RailJoint) {
	for _, tr := range w.TracksAt(from.ID) {
		if tr.Ends[0] == into.ID || tr.Ends[1] == into.ID {
			w.removeTrack(tr.ID)
			continue
		}
		for e := range tr.Ends {
			if tr.Ends[e] == from.ID {
				tr.Ends[e] = into.ID
			}
		}
		w.LayOutTrack(tr)
	}
	w.removeJoint(from.ID)
	w.PlaceStations()
	w.RailRev++
}

// MoveJoint moves joint j to p (an entry if it's then on the map's edge),
// its rail to the ground there, and lays its tracks out again (not
// graded: GradeTrack each when the move's done).
func (w *World) MoveJoint(j *RailJoint, p mgl32.Vec2) {
	nj := w.NewRailJoint(p)
	j.Pos, j.Y, j.Entry = nj.Pos, nj.Y, nj.Entry
	for _, tr := range w.TracksAt(j.ID) {
		w.LayOutTrack(tr)
	}
	w.PlaceStations()
	w.RailRev++
}

// RemoveTrack removes track tr, its stations, and any joint it leaves
// with no track.
func (w *World) RemoveTrack(tr *RailTrack) {
	for _, st := range append([]*RailStation(nil), w.RailStations...) {
		if st.TrackID == tr.ID {
			w.RemoveStation(st.ID)
		}
	}
	w.removeTrack(tr.ID)
	for _, id := range tr.Ends {
		if len(w.TracksAt(id)) == 0 {
			w.removeJoint(id)
		}
	}
	w.RailRev++
}

func (w *World) removeTrack(id uint64) {
	for i, tr := range w.RailTracks {
		if tr.ID == id {
			w.RailTracks = append(w.RailTracks[:i], w.RailTracks[i+1:]...)
			return
		}
	}
}

func (w *World) removeJoint(id uint64) {
	for i, j := range w.RailJoints {
		if j.ID == id {
			w.RailJoints = append(w.RailJoints[:i], w.RailJoints[i+1:]...)
			return
		}
	}
}

// TrackMoved reports whether track tr's points have changed since it was
// laid out.
func (tr *RailTrack) TrackMoved() bool {
	if len(tr.laidFor) != len(tr.Points) {
		return true
	}
	for i := range tr.Points {
		if tr.laidFor[i] != tr.Points[i] {
			return true
		}
	}
	return false
}

// SetTrackProfile puts a saved track's heights and tunnels back, over its
// curve; false if they don't fit (it's then laid out afresh).
func (tr *RailTrack) SetTrackProfile(heights []float32, tunnels [][2]int) bool {
	track := sampleLine(RailCurve(tr.Points))
	if !setProfile(track, heights, tunnels) {
		return false
	}
	tr.Track = track
	tr.laidFor = append([]mgl32.Vec2(nil), tr.Points...)
	return true
}

// TrackProfile is track tr's heights and tunnel spans, to save.
func (tr *RailTrack) TrackProfile() (heights []float32, tunnels [][2]int) {
	return profileOf(tr.Track)
}

// RailSnapshot is the railway's track and joints as they were, to put
// back an edit that can't be made.
type RailSnapshot struct {
	tracks []RailTrack
	joints []RailJoint
}

// SnapshotRail copies the track and joints.
func (w *World) SnapshotRail() RailSnapshot {
	var s RailSnapshot
	for _, tr := range w.RailTracks {
		c := *tr
		c.Points = append([]mgl32.Vec2(nil), tr.Points...)
		c.Track = append([]RailSample(nil), tr.Track...)
		c.laidFor = append([]mgl32.Vec2(nil), tr.laidFor...)
		s.tracks = append(s.tracks, c)
	}
	for _, j := range w.RailJoints {
		s.joints = append(s.joints, *j)
	}
	return s
}

// RestoreRail puts the track and joints back as snapshot s had them.
func (w *World) RestoreRail(s RailSnapshot) {
	w.RailTracks = w.RailTracks[:0]
	for i := range s.tracks {
		c := s.tracks[i]
		w.RailTracks = append(w.RailTracks, &c)
	}
	w.RailJoints = w.RailJoints[:0]
	for i := range s.joints {
		c := s.joints[i]
		w.RailJoints = append(w.RailJoints, &c)
	}
	w.PlaceStations()
	w.RailRev++
}

// RailCost is what all the track costs, to charge an edit the
// difference.
func (w *World) RailCost() int {
	n := 0
	for _, tr := range w.RailTracks {
		n += TrackCost(tr)
	}
	return n
}
