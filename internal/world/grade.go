package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Grade-held lines (notes/next/Trains and Tunnels.md): a railway, a
// funicular, a road edge or a footpath is laid out as samples every
// RailSampleStep metres along it, each with its surface's height, held to
// a grade, the ground cut and filled to it (GradeBed), and tunnelled
// where the cut would run too deep (markTunnelsIn).

// sampleLine resamples the polyline pts every RailSampleStep metres,
// keeping its last point.
func sampleLine(pts []mgl32.Vec2) []RailSample {
	var out []RailSample
	if len(pts) == 0 {
		return out
	}
	var dist float32
	next := float32(0)
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		seg := b.Sub(a).Len()
		for next <= dist+seg {
			p := a.Add(b.Sub(a).Mul((next - dist) / max(seg, 1e-6)))
			out = append(out, RailSample{Pos: p, Dist: next})
			next += RailSampleStep
		}
		dist += seg
	}
	if last := pts[len(pts)-1]; len(out) == 0 || out[len(out)-1].Pos != last {
		out = append(out, RailSample{Pos: last, Dist: dist})
	}
	return out
}

// holdGrade holds heights y (one a sample) to rise at most step from
// sample to sample, leaving the fixed ones be: relaxed toward where they
// start, then clamped to the grade from their neighbours, forward and
// back, a few times.
func holdGrade(y []float32, fixed func(i int) bool, step float32) {
	n := len(y)
	for pass := 0; pass < 8; pass++ {
		for i := 1; i < n; i++ {
			if !fixed(i) {
				y[i] = min(max(y[i], y[i-1]-step), y[i-1]+step)
			}
		}
		for i := n - 2; i >= 0; i-- {
			if !fixed(i) {
				y[i] = min(max(y[i], y[i+1]-step), y[i+1]+step)
			}
		}
		if pass < 7 {
			for i := 1; i < n-1; i++ {
				if !fixed(i) {
					y[i] = 0.5*y[i] + 0.25*(y[i-1]+y[i+1])
				}
			}
		}
	}
}

// layOutGraded lays a line out along pts: its surface base over the
// ground, held to grade (rise over run) with both ends kept at their
// ground, and its tunnels marked where it runs more than depth under.
func layOutGraded(pts []mgl32.Vec2, t *Terrain, base, grade, depth float32) []RailSample {
	track := sampleLine(pts)
	n := len(track)
	y := make([]float32, n)
	for i, s := range track {
		y[i] = t.InterpolatedGroundElevationAt(s.Pos[0], s.Pos[1]) + base
	}
	holdGrade(y, func(i int) bool { return i == 0 || i == n-1 }, grade*RailSampleStep)
	for i := range track {
		track[i].Y = y[i]
	}
	markTunnelsIn(track, t, depth)
	return track
}

// onGround lays a line out along pts on the ground as it is: no grade
// held, no tunnels (a road or path laid before they had profiles, or
// made in passing, like a lot's driveway).
func onGround(pts []mgl32.Vec2, t *Terrain, base float32) []RailSample {
	track := sampleLine(pts)
	for i, s := range track {
		track[i].Y = t.InterpolatedGroundElevationAt(s.Pos[0], s.Pos[1]) + base
	}
	return track
}

// trackAt is where track is d metres along: its position, height,
// heading (as Guest.Heading) and pitch (radians, up positive).
func trackAt(track []RailSample, d float32) (pos mgl32.Vec2, y, heading, pitch float32) {
	n := len(track)
	if n == 0 {
		return
	}
	if n == 1 {
		return track[0].Pos, track[0].Y, 0, 0
	}
	d = min(max(d, 0), track[n-1].Dist)
	i := min(int(d/RailSampleStep), n-2)
	a, b := track[i], track[i+1]
	f := (d - a.Dist) / max(b.Dist-a.Dist, 1e-6)
	pos = a.Pos.Add(b.Pos.Sub(a.Pos).Mul(f))
	y = a.Y + (b.Y-a.Y)*f
	dir := b.Pos.Sub(a.Pos)
	heading = float32(math.Atan2(float64(dir[0]), float64(dir[1])))
	pitch = float32(math.Atan2(float64(b.Y-a.Y), float64(max(dir.Len(), 1e-6))))
	return
}

// tunnelLength is the metres of track in tunnels.
func tunnelLength(track []RailSample) float32 {
	var l float32
	for i := 1; i < len(track); i++ {
		if track[i].Tunnel && track[i-1].Tunnel {
			l += track[i].Dist - track[i-1].Dist
		}
	}
	return l
}

// profileOf is track's heights and tunnel spans (sample indices,
// inclusive), as saved.
func profileOf(track []RailSample) (heights []float32, tunnels [][2]int) {
	heights = make([]float32, len(track))
	deep := make([]bool, len(track))
	for i, s := range track {
		heights[i], deep[i] = s.Y, s.Tunnel
	}
	return heights, runsOf(deep)
}

// setProfile puts saved heights and tunnel spans back on track; false,
// leaving it be, if they don't fit.
func setProfile(track []RailSample, heights []float32, tunnels [][2]int) bool {
	if len(heights) != len(track) {
		return false
	}
	for i := range track {
		track[i].Y, track[i].Tunnel = heights[i], false
	}
	for _, r := range tunnels {
		for i := max(r[0], 0); i <= min(r[1], len(track)-1); i++ {
			track[i].Tunnel = true
		}
	}
	return true
}

// BedSpec is how a line's bed is graded: Half either side of it level
// with its surface less Base, shoulders banked Bank (run over rise) out
// to the natural ground at most ShoulderMax wide, and trees cleared
// Clear past the bed (0 for none).
type BedSpec struct {
	Half, Base, Bank, ShoulderMax, Clear float32
}

// GradeBed cuts and fills the ground under track to its bed (b), except
// where it tunnels (the hill stays) and the cells keep says to keep (a
// building's pad).
func (t *Terrain) GradeBed(track []RailSample, b BedSpec, keep func(x, z int) bool) {
	if len(track) < 2 {
		return
	}
	reach := b.Half + b.ShoulderMax
	minX, minZ := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxZ := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	for _, s := range track {
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
	for n := 0; n+1 < len(track); n++ {
		a, c := track[n], track[n+1]
		if a.Tunnel && c.Tunnel {
			continue // the hill stays
		}
		ci, cj := p.index(a.Pos)
		r := int(reach/p.Spacing) + 2
		ac := c.Pos.Sub(a.Pos)
		ac2 := max(ac.LenSqr(), 1e-6)
		for j := max(cj-r, 0); j <= min(cj+r, p.H-1); j++ {
			for i := max(ci-r, 0); i <= min(ci+r, p.W-1); i++ {
				wx, wz := p.Pos(i, j)
				q := mgl32.Vec2{wx, wz}
				f := clamp01(q.Sub(a.Pos).Dot(ac) / ac2)
				d := q.Sub(a.Pos.Add(ac.Mul(f))).Len()
				if k := j*p.W + i; d < best[k] {
					best[k], bedY[k] = d, a.Y+(c.Y-a.Y)*f-b.Base
				}
			}
		}
	}
	for k, d := range best {
		if d > reach {
			continue
		}
		before := p.Before[k]
		if d <= b.Half {
			p.After[k] = bedY[k]
			continue
		}
		allowed := (d - b.Half) / b.Bank
		p.After[k] = bedY[k] + min(max(before-bedY[k], -allowed), allowed)
	}
	p.Commit(keep)
	if b.Clear <= 0 {
		return
	}
	hw2 := (b.Half + b.Clear) * (b.Half + b.Clear) // and a clear strip either side
	t.RemoveTreesIn(x0, z0, x1, z1, func(tr Tree) bool {
		for _, s := range track {
			if !s.Tunnel && (mgl32.Vec2{tr.X - s.Pos[0], tr.Z - s.Pos[1]}).LenSqr() <= hw2 {
				return true
			}
		}
		return false
	})
}
