package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Rail lines drawn (world/rail.go): along each line's laid-out track, its
// style's bed, sleepers and two rails (railLooks), and for an electric or
// high-speed line, masts and the contact wire; stations' platforms in
// their network's train's style. One mesh a line with per-vertex colours, drawn by the static
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

// railLook is how a style's track is built.
type railLook struct {
	// The bed: its top's half-width and its foot's, and its colour. A
	// slab is a concrete slab with straight sides on a gravel shoulder
	// (high-speed) instead of a ballast bank.
	bedTop, bedFoot float32
	bed             [3]float32
	slab            bool
	shoulder        [3]float32
	// Sleepers: every so far, their half-length and colour; twin blocks
	// (one under each rail) on a slab.
	sleeperEvery, sleeperHalf float32
	sleeper                   [3]float32
	rail                      [3]float32
	// Masts: none, one side with an arm over the track (cantilever), or a
	// gantry over both (portal), every mastEvery; a messenger wire over
	// the contact wire on a portal line. Troughs are the cable troughs
	// along both sides.
	masts     int
	mastEvery float32
	mast      [3]float32
	troughs   bool
}

const (
	mastsNone = iota
	mastsCantilever
	mastsPortal
)

var railLooks = [world.RailStyles]railLook{
	// An old branch line: brown ballast, wooden sleepers, rusty rails.
	world.RailSteam: {bedTop: 1.6, bedFoot: 2.3, bed: [3]float32{0.45, 0.40, 0.34},
		sleeperEvery: 0.8, sleeperHalf: 1.25, sleeper: [3]float32{0.27, 0.19, 0.12}, rail: [3]float32{0.42, 0.35, 0.30}},
	// A main line: grey ballast, concrete sleepers, masts with arms.
	world.RailElectric: {bedTop: 1.9, bedFoot: 2.6, bed: [3]float32{0.52, 0.51, 0.49},
		sleeperEvery: 0.6, sleeperHalf: 1.3, sleeper: [3]float32{0.68, 0.67, 0.64}, rail: [3]float32{0.58, 0.58, 0.62},
		masts: mastsCantilever, mastEvery: 45, mast: [3]float32{0.36, 0.42, 0.38}},
	// A high-speed line: slab track, gantries, cable troughs.
	world.RailHighSpeed: {bedTop: 1.7, bedFoot: 2.9, bed: [3]float32{0.76, 0.75, 0.72}, slab: true, shoulder: [3]float32{0.50, 0.49, 0.48},
		sleeperEvery: 0.65, sleeperHalf: 0.35, sleeper: [3]float32{0.62, 0.61, 0.59}, rail: [3]float32{0.74, 0.75, 0.79},
		masts: mastsPortal, mastEvery: 55, mast: [3]float32{0.62, 0.64, 0.66}, troughs: true},
}

// railLineMesh builds line l's mesh as style's track: tint multiplies
// every colour (white for built track, a placement tint for the ghost).
func railLineMesh(l *world.RailLine, style world.RailStyle, t *world.Terrain, tint [3]float32) *Mesh {
	tr := l.Track
	if len(tr) < 2 {
		return nil
	}
	lk := &railLooks[min(style, world.RailStyles-1)]
	col := func(c [3]float32) [3]float32 { return [3]float32{c[0] * tint[0], c[1] * tint[1], c[2] * tint[2]} }
	up := mgl32.Vec3{0, 1, 0}
	var b meshBuilder
	open := func(i int) bool { return !tr[i].Tunnel && !tr[i+1].Tunnel }
	// along runs f over each open stretch between samples, with their
	// frames.
	along := func(f func(p0, x0, p1, x1 mgl32.Vec3)) {
		for i := 0; i+1 < len(tr); i++ {
			if !open(i) {
				continue
			}
			p0, _, x0 := railFrame(l, i)
			p1, _, x1 := railFrame(l, i+1)
			f(p0, x0, p1, x1)
		}
	}
	// strip is a flat strip along the track, off across it, half wide, h
	// up.
	strip := func(off, half, h float32, c [3]float32) {
		along(func(p0, x0, p1, x1 mgl32.Vec3) {
			o0, o1 := x0.Mul(off), x1.Mul(off)
			l0, r0 := p0.Add(o0).Add(x0.Mul(half)).Add(up.Mul(h)), p0.Add(o0).Sub(x0.Mul(half)).Add(up.Mul(h))
			l1, r1 := p1.Add(o1).Add(x1.Mul(half)).Add(up.Mul(h)), p1.Add(o1).Sub(x1.Mul(half)).Add(up.Mul(h))
			b.quad(l0, r0, r1, l1, col(c))
		})
	}
	// side is a face along the track from (off0, h0) to (off1, h1) across
	// and up, facing out on side s.
	side := func(s, off0, h0, off1, h1 float32, c [3]float32) {
		along(func(p0, x0, p1, x1 mgl32.Vec3) {
			t0, t1 := p0.Add(x0.Mul(s*off0)).Add(up.Mul(h0)), p1.Add(x1.Mul(s*off0)).Add(up.Mul(h0))
			f0, f1 := p0.Add(x0.Mul(s*off1)).Add(up.Mul(h1)), p1.Add(x1.Mul(s*off1)).Add(up.Mul(h1))
			if s > 0 {
				b.quad(f0, t0, t1, f1, col(c))
			} else {
				b.quad(t0, f0, f1, t1, col(c))
			}
		})
	}
	// The bed: ballast sloping to the ground, or a slab on a shoulder.
	strip(0, lk.bedTop, -0.18, lk.bed)
	for _, s := range [2]float32{-1, 1} {
		if lk.slab {
			side(s, lk.bedTop, -0.18, lk.bedTop, -0.42, lk.bed)
			strip(s*(lk.bedTop+lk.bedFoot)/2, (lk.bedFoot-lk.bedTop)/2, -0.42, lk.shoulder)
			side(s, lk.bedFoot, -0.42, lk.bedFoot+0.5, -world.RailBallast, lk.shoulder)
		} else {
			side(s, lk.bedTop, -0.18, lk.bedFoot, -world.RailBallast, lk.bed)
		}
	}
	// Cable troughs along both sides.
	if lk.troughs {
		for _, s := range [2]float32{-1, 1} {
			strip(s*2.55, 0.22, -0.26, [3]float32{0.80, 0.79, 0.76})
			side(s, 2.77, -0.26, 2.77, -0.42, [3]float32{0.80, 0.79, 0.76})
		}
	}
	// Sleepers (twin blocks on a slab).
	for d := lk.sleeperEvery / 2; d < l.Length(); d += lk.sleeperEvery {
		if l.InTunnel(d) {
			continue
		}
		pos, y, _, _ := l.At(d)
		i := min(int(d/world.RailSampleStep), len(tr)-1)
		_, al, across := railFrame(l, i)
		c := mgl32.Vec3{pos[0], y - 0.11, pos[1]}
		if lk.slab {
			for _, s := range [2]float32{-0.75, 0.75} {
				b.box(c.Add(across.Mul(s)), across, up, al, lk.sleeperHalf, 0.06, 0.13, col(lk.sleeper))
			}
		} else {
			b.box(c, across, up, al, lk.sleeperHalf, 0.07, 0.12, col(lk.sleeper))
		}
	}
	// The two rails: a web and a brighter head.
	for _, s := range [2]float32{-0.75, 0.75} {
		side(1, s+0.04, 0, s+0.04, -0.12, lk.rail)
		side(-1, -s+0.04, 0, -s+0.04, -0.12, lk.rail)
		strip(s, 0.04, 0, lk.rail)
	}
	if lk.masts != mastsNone {
		railMasts(&b, l, lk, col)
	}
	tunnelDashes(&b, l, t, col)
	return b.mesh()
}

// railMasts adds an electrified line's masts and wires.
func railMasts(b *meshBuilder, l *world.RailLine, lk *railLook, col func([3]float32) [3]float32) {
	tr := l.Track
	up := mgl32.Vec3{0, 1, 0}
	const wireH, messengerH, mastOff = 5.6, 6.7, 3.3
	for d := float32(10); d < l.Length(); d += lk.mastEvery {
		if l.InTunnel(d) {
			continue
		}
		pos, y, _, _ := l.At(d)
		i := min(int(d/world.RailSampleStep), len(tr)-1)
		_, along, across := railFrame(l, i)
		at := mgl32.Vec3{pos[0], y, pos[1]}
		switch lk.masts {
		case mastsCantilever:
			foot := at.Add(across.Mul(-mastOff))
			b.box(foot.Add(up.Mul(3.2)), across, up, along, 0.15, 3.6, 0.15, col(lk.mast))
			b.box(foot.Add(up.Mul(wireH+0.4)).Add(across.Mul(mastOff/2)), across, up, along, mastOff/2+0.2, 0.06, 0.06, col(lk.mast))
		case mastsPortal:
			off := float32(mastOff + 0.3)
			for _, s := range [2]float32{-1, 1} {
				foot := at.Add(across.Mul(s * off))
				b.box(foot.Add(up.Mul(0.15)), across, up, along, 0.35, 0.35, 0.35, col([3]float32{0.70, 0.69, 0.66}))
				b.box(foot.Add(up.Mul(3.9)), across, up, along, 0.18, 3.9, 0.18, col(lk.mast))
			}
			b.box(at.Add(up.Mul(7.6)), across, up, along, off+0.25, 0.22, 0.12, col(lk.mast))
			b.box(at.Add(up.Mul((messengerH+7.4)/2)), across, up, along, 0.03, (7.4-messengerH)/2, 0.03, col(wireCol))
		}
	}
	wire := func(h, half float32) {
		for i := 0; i+1 < len(tr); i++ {
			if tr[i].Tunnel || tr[i+1].Tunnel {
				continue
			}
			p0, _, x0 := railFrame(l, i)
			p1, _, x1 := railFrame(l, i+1)
			u := up.Mul(h)
			b.quad(p0.Add(u).Sub(x0.Mul(half)), p0.Add(u).Add(x0.Mul(half)), p1.Add(u).Add(x1.Mul(half)), p1.Add(u).Sub(x1.Mul(half)), col(wireCol))
		}
	}
	wire(wireH, 0.03)
	if lk.masts == mastsPortal {
		wire(messengerH, 0.025)
		// Droppers between the two, every few metres.
		for d := float32(4); d < l.Length(); d += 8 {
			if l.InTunnel(d) {
				continue
			}
			pos, y, _, _ := l.At(d)
			i := min(int(d/world.RailSampleStep), len(tr)-1)
			_, along, across := railFrame(l, i)
			b.box(mgl32.Vec3{pos[0], y + (wireH+messengerH)/2, pos[1]}, across, up, along, 0.015, (messengerH-wireH)/2, 0.015, col(wireCol))
		}
	}
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

// platformLook is a station's platform in its network's train's style:
// its surface and edge, and a canopy over it (none for steam).
var platformLooks = [world.RailStyles]struct {
	surface, edge, canopy [3]float32
	canopyHalf            float32
}{
	world.RailSteam:     {surface: [3]float32{0.56, 0.49, 0.40}, edge: [3]float32{0.88, 0.86, 0.80}},
	world.RailElectric:  {surface: [3]float32{0.62, 0.61, 0.58}, edge: [3]float32{0.90, 0.85, 0.30}, canopy: [3]float32{0.30, 0.42, 0.36}, canopyHalf: 0.25},
	world.RailHighSpeed: {surface: [3]float32{0.80, 0.79, 0.77}, edge: [3]float32{0.92, 0.86, 0.25}, canopy: [3]float32{0.86, 0.88, 0.90}, canopyHalf: 0.5},
}

// stationsMesh is every station's platform: beside its track, as long as
// its network's train, with an edge, and a canopy over the middle of it
// (its half's length canopyHalf of the platform's) for an electric or
// high-speed network.
func stationsMesh(w *world.World) *Mesh {
	_, of := w.RailNetworks()
	var b meshBuilder
	up := mgl32.Vec3{0, 1, 0}
	for _, st := range w.RailStations {
		tr := w.RailTrackByID(st.TrackID)
		if tr == nil || len(tr.Track) < 2 {
			continue
		}
		train := of[tr.ID].Train
		lk := platformLooks[min(train, world.RailStyles-1)]
		half := train.Spec().PlatformLength() / 2
		k := 0
		for d := max(st.Dist-half, 0); d < min(st.Dist+half, tr.Length()); d += world.RailSampleStep {
			pos, y, _, _ := tr.At(d)
			i := min(int(d/world.RailSampleStep), len(tr.Track)-1)
			_, along, across := railFrame(&tr.RailLine, i)
			side := across.Mul(st.Side)
			c := mgl32.Vec3{pos[0], y, pos[1]}.Add(side.Mul(world.StationPlatformSide))
			b.box(c.Add(up.Mul(-0.6)), across, up, along, 1.75, 1.15, world.RailSampleStep/2+0.02, lk.surface)
			b.box(c.Sub(side.Mul(1.6)).Add(up.Mul(0.56)), across, up, along, 0.15, 0.01, world.RailSampleStep/2+0.02, lk.edge)
			if lk.canopyHalf > 0 && float32(math.Abs(float64(d-st.Dist))) < half*lk.canopyHalf {
				b.box(c.Add(side.Mul(0.3)).Add(up.Mul(3.6)), across, up, along, 1.6, 0.08, world.RailSampleStep/2+0.02, lk.canopy)
				if k%3 == 0 {
					b.box(c.Add(side.Mul(0.6)).Add(up.Mul(1.8)), across, up, along, 0.08, 1.75, 0.08, [3]float32{0.30, 0.31, 0.33})
				}
				k++
			}
		}
	}
	return b.mesh()
}

// StationTransform places station st's house beside its platform's
// middle; ok false if it's off any track.
func StationTransform(w *world.World, st *world.RailStation) (mgl32.Mat4, bool) {
	pos, y, heading, side, ok := w.StationFrame(st)
	if !ok {
		return mgl32.Mat4{}, false
	}
	s, c := float32(math.Sin(float64(heading))), float32(math.Cos(float64(heading)))
	along := mgl32.Vec2{s, c}
	at := pos.Add(side.Mul(world.StationPlatformSide + 1.75 + 5.6))
	// Model +X along the track, its front (-Z) toward the platform:
	// HomogRotate3DY(θ) takes +X to (cos θ, -sin θ).
	rot := float32(math.Atan2(-float64(along[1]), float64(along[0])))
	if st.Side < 0 {
		rot += math.Pi
	}
	return mgl32.Translate3D(at[0], y-0.6, at[1]).Mul4(mgl32.HomogRotate3DY(rot)), true
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
