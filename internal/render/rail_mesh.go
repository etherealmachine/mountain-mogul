package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Rail lines drawn (world/rail.go): along each line's laid-out track, a
// ballast bed, sleepers and two rails, the station's platform beside its
// end, and for an electric or high-speed line, masts and the contact
// wire. One mesh a line with per-vertex colours, drawn by the static
// shader so it's lit, shadowed and lit at night like the rest; rebuilt
// when the line changes (RebuildRails).

var (
	ballastCol  = [3]float32{0.47, 0.44, 0.40}
	sleeperCol  = [3]float32{0.30, 0.22, 0.16}
	railCol     = [3]float32{0.55, 0.55, 0.58}
	platformCol = [3]float32{0.62, 0.61, 0.58}
	edgeCol     = [3]float32{0.90, 0.85, 0.30}
	mastCol     = [3]float32{0.40, 0.42, 0.44}
	wireCol     = [3]float32{0.12, 0.12, 0.13}
)

// meshBuilder gathers coloured triangles: pos, normal, uv, colour.
type meshBuilder struct {
	verts []float32
	idx   []uint32
}

func (b *meshBuilder) quad(a, c, d, e mgl32.Vec3, col [3]float32) {
	n := c.Sub(a).Cross(e.Sub(a))
	if l := n.Len(); l > 0 {
		n = n.Mul(1 / l)
	}
	base := uint32(len(b.verts) / 11)
	for _, p := range [4]mgl32.Vec3{a, c, d, e} {
		b.verts = append(b.verts, p[0], p[1], p[2], n[0], n[1], n[2], 0, 0, col[0], col[1], col[2])
	}
	b.idx = append(b.idx, base, base+1, base+2, base, base+2, base+3)
}

// box adds a box centred at c, its axes along (unit) x, y (up) and z,
// half-extents hx, hy, hz.
func (b *meshBuilder) box(c, x, y, z mgl32.Vec3, hx, hy, hz float32, col [3]float32) {
	X, Y, Z := x.Mul(hx), y.Mul(hy), z.Mul(hz)
	p := func(sx, sy, sz float32) mgl32.Vec3 { return c.Add(X.Mul(sx)).Add(Y.Mul(sy)).Add(Z.Mul(sz)) }
	b.quad(p(-1, 1, -1), p(-1, 1, 1), p(1, 1, 1), p(1, 1, -1), col)     // top
	b.quad(p(-1, -1, -1), p(1, -1, -1), p(1, -1, 1), p(-1, -1, 1), col) // bottom
	b.quad(p(-1, -1, 1), p(1, -1, 1), p(1, 1, 1), p(-1, 1, 1), col)     // +z
	b.quad(p(1, -1, -1), p(-1, -1, -1), p(-1, 1, -1), p(1, 1, -1), col) // -z
	b.quad(p(1, -1, 1), p(1, -1, -1), p(1, 1, -1), p(1, 1, 1), col)     // +x
	b.quad(p(-1, -1, -1), p(-1, -1, 1), p(-1, 1, 1), p(-1, 1, -1), col) // -x
}

func (b *meshBuilder) mesh() *Mesh {
	if len(b.idx) == 0 {
		return nil
	}
	return NewMesh(b.verts, b.idx, []int{3, 3, 2, 3}, []uint32{0, 1, 2, VertexColorLoc})
}

// railFrame is the track's frame at sample i: along, up and across (to
// the left facing along).
func railFrame(l *world.RailLine, i int) (pos mgl32.Vec3, along, across mgl32.Vec3) {
	tr := l.Track
	a, b := tr[max(i-1, 0)], tr[min(i+1, len(tr)-1)]
	d := mgl32.Vec3{b.Pos[0] - a.Pos[0], b.Y - a.Y, b.Pos[1] - a.Pos[1]}
	if d.Len() < 1e-6 {
		d = mgl32.Vec3{1, 0, 0}
	}
	along = d.Normalize()
	flat := mgl32.Vec3{along[0], 0, along[2]}.Normalize()
	across = mgl32.Vec3{flat[2], 0, -flat[0]} // left of along
	s := tr[i]
	return mgl32.Vec3{s.Pos[0], s.Y, s.Pos[1]}, along, across
}

// railLineMesh builds line l's mesh: tint multiplies every colour (white
// for the built line, a placement tint for the ghost).
func railLineMesh(l *world.RailLine, t *world.Terrain, tint [3]float32) *Mesh {
	tr := l.Track
	if len(tr) < 2 {
		return nil
	}
	col := func(c [3]float32) [3]float32 { return [3]float32{c[0] * tint[0], c[1] * tint[1], c[2] * tint[2]} }
	up := mgl32.Vec3{0, 1, 0}
	var b meshBuilder
	spec := l.Style.Spec()
	// Ballast and rails: strips along the track.
	open := func(i int) bool { return !tr[i].Tunnel && !tr[i+1].Tunnel }
	strip := func(off, half, h float32, c [3]float32) {
		for i := 0; i+1 < len(tr); i++ {
			if !open(i) {
				continue
			}
			p0, _, x0 := railFrame(l, i)
			p1, _, x1 := railFrame(l, i+1)
			o0, o1 := x0.Mul(off), x1.Mul(off)
			l0, r0 := p0.Add(o0).Add(x0.Mul(half)).Add(up.Mul(h)), p0.Add(o0).Sub(x0.Mul(half)).Add(up.Mul(h))
			l1, r1 := p1.Add(o1).Add(x1.Mul(half)).Add(up.Mul(h)), p1.Add(o1).Sub(x1.Mul(half)).Add(up.Mul(h))
			b.quad(l0, r0, r1, l1, col(c))
		}
	}
	strip(0, 1.9, -0.18, ballastCol)
	// The ballast's sloping sides down to the bed.
	for _, s := range [2]float32{-1, 1} {
		for i := 0; i+1 < len(tr); i++ {
			if !open(i) {
				continue
			}
			p0, _, x0 := railFrame(l, i)
			p1, _, x1 := railFrame(l, i+1)
			t0, t1 := p0.Add(x0.Mul(s*1.9)).Add(up.Mul(-0.18)), p1.Add(x1.Mul(s*1.9)).Add(up.Mul(-0.18))
			f0, f1 := p0.Add(x0.Mul(s*2.6)).Add(up.Mul(-world.RailBallast)), p1.Add(x1.Mul(s*2.6)).Add(up.Mul(-world.RailBallast))
			if s > 0 {
				b.quad(f0, t0, t1, f1, col(ballastCol))
			} else {
				b.quad(t0, f0, f1, t1, col(ballastCol))
			}
		}
	}
	// Sleepers every 0.7 m.
	for d := float32(0.35); d < l.Length(); d += 0.7 {
		if l.InTunnel(d) {
			continue
		}
		pos, y, _, _ := l.At(d)
		i := min(int(d/world.RailSampleStep), len(tr)-1)
		_, along, across := railFrame(l, i)
		b.box(mgl32.Vec3{pos[0], y - 0.11, pos[1]}, across, up, along, 1.3, 0.07, 0.12, col(sleeperCol))
	}
	// The two rails.
	for _, s := range [2]float32{-0.75, 0.75} {
		strip(s, 0.04, 0, railCol)
	}
	// The platform: along the last PlatformLength metres, on the left.
	platLen := spec.PlatformLength()
	start := l.Length() - platLen
	for d := max(start, 0); d < l.Length(); d += world.RailSampleStep {
		pos, y, _, _ := l.At(d)
		i := min(int(d/world.RailSampleStep), len(tr)-1)
		_, along, across := railFrame(l, i)
		c := mgl32.Vec3{pos[0], y, pos[1]}.Add(across.Mul(3.45))
		b.box(c.Add(up.Mul(-0.6)), across, up, along, 1.75, 1.15, world.RailSampleStep/2+0.02, col(platformCol))
		b.box(c.Add(across.Mul(-1.6)).Add(up.Mul(0.56)), across, up, along, 0.15, 0.01, world.RailSampleStep/2+0.02, col(edgeCol))
	}
	// Masts and the wire for an electric or high-speed line.
	if spec.Wired {
		const mastEvery, wireH, mastOff = 45.0, 5.6, 3.3
		for d := float32(10); d < l.Length(); d += mastEvery {
			if l.InTunnel(d) {
				continue
			}
			pos, y, _, _ := l.At(d)
			i := min(int(d/world.RailSampleStep), len(tr)-1)
			_, along, across := railFrame(l, i)
			foot := mgl32.Vec3{pos[0], y, pos[1]}.Add(across.Mul(-mastOff))
			b.box(foot.Add(up.Mul(3.2)), across, up, along, 0.15, 3.6, 0.15, col(mastCol))
			b.box(foot.Add(up.Mul(wireH+0.4)).Add(across.Mul(mastOff/2)), across, up, along, mastOff/2+0.2, 0.06, 0.06, col(mastCol))
		}
		for i := 0; i+1 < len(tr); i++ {
			if !open(i) {
				continue
			}
			p0, _, x0 := railFrame(l, i)
			p1, _, x1 := railFrame(l, i+1)
			h := up.Mul(wireH)
			b.quad(p0.Add(h).Sub(x0.Mul(0.03)), p0.Add(h).Add(x0.Mul(0.03)), p1.Add(h).Add(x1.Mul(0.03)), p1.Add(h).Sub(x1.Mul(0.03)), col(wireCol))
		}
	}
	tunnelDashes(&b, l, t, col)
	return b.mesh()
}

// tunnelDashes adds a tunnel's course: a dashed line on the ground over
// it.
func tunnelDashes(b *meshBuilder, l *world.RailLine, t *world.Terrain, col func([3]float32) [3]float32) {
	tr := l.Track
	for i := 0; i+1 < len(tr); i += 2 {
		if !tr[i].Tunnel || !tr[i+1].Tunnel || (i/2)%2 == 1 {
			continue
		}
		a, c := tr[i].Pos, tr[i+1].Pos
		ya := t.InterpolatedSurfaceElevationAt(a[0], a[1]) + 0.3
		yc := t.InterpolatedSurfaceElevationAt(c[0], c[1]) + 0.3
		_, _, x0 := railFrame(l, i)
		p0, p1 := mgl32.Vec3{a[0], ya, a[1]}, mgl32.Vec3{c[0], yc, c[1]}
		b.quad(p0.Add(x0.Mul(0.6)), p0.Sub(x0.Mul(0.6)), p1.Sub(x0.Mul(0.6)), p1.Add(x0.Mul(0.6)), col([3]float32{0.30, 0.28, 0.26}))
	}
}

// stationTransform places a line's station house beside its platform.
func stationTransform(l *world.RailLine) mgl32.Mat4 {
	pos, y, heading := l.Station()
	s, c := float32(math.Sin(float64(heading))), float32(math.Cos(float64(heading)))
	along := mgl32.Vec2{s, c}
	left := mgl32.Vec2{c, -s}
	at := pos.Add(left.Mul(3.45 + 1.75 + 5.6))
	// Model +X along the track; HomogRotate3DY(θ) takes +X to (cos θ, -sin θ).
	rot := float32(math.Atan2(-float64(along[1]), float64(along[0])))
	return mgl32.Translate3D(at[0], y-0.6, at[1]).Mul4(mgl32.HomogRotate3DY(rot))
}

// portalTransform places a tunnel portal: its face at the mouth, facing
// out, the opening's foot at the line's level.
func portalTransform(p world.TunnelPortal) mgl32.Mat4 {
	return portalAt(p, p.Y-world.RailBallast)
}

// portalAt places a tunnel portal with the opening's foot at foot.
func portalAt(p world.TunnelPortal, foot float32) mgl32.Mat4 {
	s, c := float32(math.Sin(float64(p.Heading))), float32(math.Cos(float64(p.Heading)))
	rot := float32(math.Atan2(-float64(c), float64(s)))
	return mgl32.Translate3D(p.Pos[0], foot, p.Pos[1]).Mul4(mgl32.HomogRotate3DY(rot))
}

// tunnelCourseMesh is the dashed lines on the ground over the roads' and
// paths' tunnels (a railway's are in its own mesh).
func tunnelCourseMesh(tunnels []world.Tunnel, t *world.Terrain) *Mesh {
	var b meshBuilder
	for _, tn := range tunnels {
		c := tn.Course
		half := min(c.Half/3, 0.6)
		for i := 0; i+1 < len(c.Points); i += 2 {
			if (i/2)%2 == 1 {
				continue
			}
			a, e := c.Points[i], c.Points[i+1]
			d := e.Sub(a)
			if d.Len() < 1e-4 {
				continue
			}
			x := mgl32.Vec3{d[1], 0, -d[0]}.Normalize().Mul(half) // left of along
			p0 := mgl32.Vec3{a[0], t.InterpolatedSurfaceElevationAt(a[0], a[1]) + 0.3, a[1]}
			p1 := mgl32.Vec3{e[0], t.InterpolatedSurfaceElevationAt(e[0], e[1]) + 0.3, e[1]}
			b.quad(p0.Add(x), p0.Sub(x), p1.Sub(x), p1.Add(x), [3]float32{0.30, 0.28, 0.26})
		}
	}
	return b.mesh()
}
