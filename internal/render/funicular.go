//go:build !demo

package render

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// A funicular's track drawn (world/funicular.go): ballast, sleepers and
// rails up the line, opening to two tracks on the passing loop, and at
// each end its station: a platform stepping down the slope beside the
// track on the riders' side, under a roof on posts, and buffers at the
// track's end. Built like a railway's (railLineMesh), one mesh a lift.

var funicRoofCol = [3]float32{0.55, 0.56, 0.58}

// funicularTrackMesh builds funicular lift's track and stations; tint
// multiplies every colour.
func funicularTrackMesh(lift *world.Lift, t *world.Terrain, tint [3]float32) *Mesh {
	l := lift.FunicularTrack(t)
	if l == nil || len(l.Track) < 2 {
		return nil
	}
	tr := l.Track
	col := func(c [3]float32) [3]float32 { return [3]float32{c[0] * tint[0], c[1] * tint[1], c[2] * tint[2]} }
	up := mgl32.Vec3{0, 1, 0}
	var b meshBuilder
	open := func(i int) bool { return !tr[i].Tunnel && !tr[i+1].Tunnel }
	// loop is how far the car on side s's track is off the line at
	// sample i, across (left of uphill: the riders' side is -across).
	loop := func(i int, s float32) float32 { return -lift.FunicularLoopOffset(tr[i].Dist, s) }
	// strip lays a strip along the track, at off(i) across, half wide,
	// h over the rail top; skip leaves a segment out.
	strip := func(off, half func(int) float32, h float32, c [3]float32, skip func(int) bool) {
		for i := 0; i+1 < len(tr); i++ {
			if !open(i) || (skip != nil && skip(i)) {
				continue
			}
			p0, _, x0 := railFrame(l, i)
			p1, _, x1 := railFrame(l, i+1)
			o0, o1 := x0.Mul(off(i)), x1.Mul(off(i+1))
			l0, r0 := p0.Add(o0).Add(x0.Mul(half(i))).Add(up.Mul(h)), p0.Add(o0).Sub(x0.Mul(half(i))).Add(up.Mul(h))
			l1, r1 := p1.Add(o1).Add(x1.Mul(half(i+1))).Add(up.Mul(h)), p1.Add(o1).Sub(x1.Mul(half(i+1))).Add(up.Mul(h))
			b.quad(l0, r0, r1, l1, col(c))
		}
	}
	zero := func(int) float32 { return 0 }
	spread := func(i int) float32 { return abs32f(loop(i, 1)) }
	strip(zero, func(i int) float32 { return 1.5 + spread(i) }, -0.18, ballastCol, nil)
	// Sleepers every 0.7 m, across both tracks on the loop.
	for d := float32(0.35); d < l.Length(); d += 0.7 {
		if l.InTunnel(d) {
			continue
		}
		pos, y, _, _ := l.At(d)
		i := min(int(d/world.RailSampleStep), len(tr)-1)
		_, along, across := railFrame(l, i)
		b.box(mgl32.Vec3{pos[0], y - 0.11, pos[1]}, across, up, along, 1.0+spread(i), 0.07, 0.12, col(sleeperCol))
	}
	// The rails: each car's pair, one pair off the loop.
	for _, s := range [2]float32{1, -1} {
		single := func(i int) bool { return s < 0 && spread(i) < 0.01 && spread(i+1) < 0.01 }
		for _, g := range [2]float32{-0.5, 0.5} {
			strip(func(i int) float32 { return loop(i, s) + g }, func(int) float32 { return 0.04 }, 0, railCol, single)
		}
	}
	// The stations: platforms stepped down the slope, the roof over
	// them, buffers at the track's ends.
	length := l.Length()
	for _, end := range [2]struct{ from, to float32 }{{0, world.FunicularStationLen}, {length - world.FunicularStationLen, length}} {
		const stepLen = 1.0
		for d := max(end.from, 0); d < min(end.to, length); d += stepLen {
			pos, y, _, _ := l.At(d + stepLen/2)
			i := min(int(d/world.RailSampleStep), len(tr)-1)
			_, along, across := railFrame(l, i)
			flat := mgl32.Vec3{along[0], 0, along[2]}.Normalize()
			c := mgl32.Vec3{pos[0], y, pos[1]}.Sub(across.Mul(2.6))
			b.box(c.Add(up.Mul(-0.6)), across, up, flat, 1.2, 0.9, stepLen/2+0.02, col(platformCol))
			b.box(c.Add(across.Mul(1.1)).Add(up.Mul(0.31)), across, up, flat, 0.1, 0.01, stepLen/2+0.02, col(edgeCol))
		}
		// Posts each side every 4.5 m, the roof sloping with the track.
		const roofH = 3.6
		for d := end.from + 1; d < end.to; d += 4.5 {
			pos, y, _, _ := l.At(d)
			i := min(int(d/world.RailSampleStep), len(tr)-1)
			_, along, across := railFrame(l, i)
			flat := mgl32.Vec3{along[0], 0, along[2]}.Normalize()
			for _, o := range [2]float32{1.9, -3.7} {
				foot := mgl32.Vec3{pos[0], y, pos[1]}.Add(across.Mul(o))
				b.box(foot.Add(up.Mul(roofH/2-0.4)), across, up, flat, 0.12, roofH/2+0.4, 0.12, col(mastCol))
			}
		}
		i0 := min(int(max(end.from, 0)/world.RailSampleStep), len(tr)-1)
		i1 := min(int(min(end.to, length)/world.RailSampleStep), len(tr)-1)
		p0, _, x0 := railFrame(l, i0)
		p1, _, x1 := railFrame(l, i1)
		h := up.Mul(roofH)
		a0, c0 := p0.Add(h).Add(x0.Mul(2.4)), p0.Add(h).Sub(x0.Mul(4.2))
		a1, c1 := p1.Add(h).Add(x1.Mul(2.4)), p1.Add(h).Sub(x1.Mul(4.2))
		b.quad(a0, c0, c1, a1, col(funicRoofCol))
	}
	for _, d := range [2]float32{0.4, length - 0.4} {
		pos, y, _, _ := l.At(d)
		i := min(int(d/world.RailSampleStep), len(tr)-1)
		_, along, across := railFrame(l, i)
		b.box(mgl32.Vec3{pos[0], y + 0.4, pos[1]}, across, up, along, 0.9, 0.45, 0.2, col(edgeCol))
	}
	tunnelDashes(&b, l, t, col)
	return b.mesh()
}

// newFunicBatch is funicular lift's track as a batch (nil for any other
// lift).
func newFunicBatch(r *Renderer, lift *world.Lift, t *world.Terrain, tint [3]float32) *railBatch {
	m := funicularTrackMesh(lift, t, tint)
	if m == nil {
		return nil
	}
	b := NewStaticBatch(m, r.whiteTexID)
	b.AddStatic(mgl32.Ident4(), mgl32.Vec3{1, 1, 1})
	return &railBatch{mesh: m, batch: b}
}
