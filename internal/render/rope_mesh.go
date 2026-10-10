package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Ropes on poles: the lift-line maze dividers (generateQueueMesh) and the
// player's rope lines (world/rope.go), drawn in the same world-space pass
// with the queue tint.

const (
	ropePoleRadius = float32(0.05) // pole cross-section radius
	ropePoleHeight = float32(1.10) // pole height above snow surface
	ropeHeight     = float32(0.95) // rope centre height above snow surface
	ropeHW         = float32(0.02) // rope half-width / half-thickness
	ropePoleFaces  = 6
)

// ropeMesher collects poles and rope stretches into one mesh.
type ropeMesher struct {
	verts []float32
	idxs  []uint32
}

// pole adds a pole standing at (cx, cz) on ground height cy.
func (m *ropeMesher) pole(cx, cy, cz float32) {
	base := uint32(len(m.verts) / 8)
	twoPi := float32(2 * math.Pi)
	for i := 0; i < ropePoleFaces; i++ {
		a0 := twoPi * float32(i) / float32(ropePoleFaces)
		a1 := twoPi * float32(i+1) / float32(ropePoleFaces)
		for _, a := range [2]float32{a0, a1} {
			nx := float32(math.Cos(float64(a)))
			nz := float32(math.Sin(float64(a)))
			u := float32(i) / float32(ropePoleFaces)
			m.verts = append(m.verts,
				cx+ropePoleRadius*nx, cy, cz+ropePoleRadius*nz, nx, 0, nz, u, 0,
				cx+ropePoleRadius*nx, cy+ropePoleHeight, cz+ropePoleRadius*nz, nx, 0, nz, u, 1,
			)
		}
		vi := base + uint32(i)*4
		m.idxs = append(m.idxs, vi, vi+2, vi+1, vi+1, vi+2, vi+3)
	}
	topCtr := uint32(len(m.verts) / 8)
	m.verts = append(m.verts, cx, cy+ropePoleHeight, cz, 0, 1, 0, 0.5, 0.5)
	for i := 0; i < ropePoleFaces; i++ {
		a := twoPi * float32(i) / float32(ropePoleFaces)
		nx := float32(math.Cos(float64(a)))
		nz := float32(math.Sin(float64(a)))
		m.verts = append(m.verts, cx+ropePoleRadius*nx, cy+ropePoleHeight, cz+ropePoleRadius*nz, 0, 1, 0, 0.5+0.5*nx, 0.5+0.5*nz)
	}
	for i := 0; i < ropePoleFaces; i++ {
		m.idxs = append(m.idxs, topCtr, topCtr+1+uint32(i), topCtr+1+uint32((i+1)%ropePoleFaces))
	}
}

// rope adds a straight stretch of rope between poles at (p0x, p0z) and
// (p1x, p1z), at rope height over the snow at each end.
func (m *ropeMesher) rope(t *world.Terrain, p0x, p0z, p1x, p1z float32) {
	y0 := VisualElevationAt(t, p0x, p0z) + ropeHeight
	y1 := VisualElevationAt(t, p1x, p1z) + ropeHeight
	mx := (p0x + p1x) / 2
	my := (y0 + y1) / 2
	mz := (p0z + p1z) / 2
	fdx, fdz := p1x-p0x, p1z-p0z
	flen := float32(math.Sqrt(float64(fdx*fdx + fdz*fdz)))
	if flen < 1e-4 {
		return
	}
	fx, fz := fdx/flen, fdz/flen
	lx, lz := -fz, fx
	hlen := flen / 2
	dy := (y1 - y0) / 2
	corners := [8][3]float32{
		{mx - fx*hlen - lx*ropeHW, my - dy - ropeHW, mz - fz*hlen - lz*ropeHW},
		{mx + fx*hlen - lx*ropeHW, my + dy - ropeHW, mz + fz*hlen - lz*ropeHW},
		{mx + fx*hlen + lx*ropeHW, my + dy - ropeHW, mz + fz*hlen + lz*ropeHW},
		{mx - fx*hlen + lx*ropeHW, my - dy - ropeHW, mz - fz*hlen + lz*ropeHW},
		{mx - fx*hlen - lx*ropeHW, my - dy + ropeHW, mz - fz*hlen - lz*ropeHW},
		{mx + fx*hlen - lx*ropeHW, my + dy + ropeHW, mz + fz*hlen - lz*ropeHW},
		{mx + fx*hlen + lx*ropeHW, my + dy + ropeHW, mz + fz*hlen + lz*ropeHW},
		{mx - fx*hlen + lx*ropeHW, my - dy + ropeHW, mz - fz*hlen + lz*ropeHW},
	}
	faces := [6][4]int{{3, 2, 1, 0}, {4, 5, 6, 7}, {0, 1, 5, 4}, {2, 3, 7, 6}, {3, 0, 4, 7}, {1, 2, 6, 5}}
	normals := [6][3]float32{
		{0, -1, 0}, {0, 1, 0},
		{fx, 0, fz}, {-fx, 0, -fz},
		{-lx, 0, -lz}, {lx, 0, lz},
	}
	for fi, face := range faces {
		base := uint32(len(m.verts) / 8)
		nx, ny, nz := normals[fi][0], normals[fi][1], normals[fi][2]
		for vi, ci := range face {
			c := corners[ci]
			m.verts = append(m.verts, c[0], c[1], c[2], nx, ny, nz, float32(vi&1), float32(vi>>1))
		}
		m.idxs = append(m.idxs, base, base+1, base+2, base, base+2, base+3)
	}
}

// line adds a rope through nodes: a pole at every node and about every
// world.RopePolePitch metres between, roped together.
func (m *ropeMesher) line(t *world.Terrain, nodes []mgl32.Vec2) {
	var poles []mgl32.Vec2
	for i := 1; i < len(nodes); i++ {
		a, b := nodes[i-1], nodes[i]
		n := max(int(math.Ceil(float64(b.Sub(a).Len()/world.RopePolePitch))), 1)
		if i == 1 {
			poles = append(poles, a)
		}
		for k := 1; k <= n; k++ {
			poles = append(poles, a.Add(b.Sub(a).Mul(float32(k)/float32(n))))
		}
	}
	for i, p := range poles {
		m.pole(p[0], VisualElevationAt(t, p[0], p[1]), p[1])
		if i > 0 {
			m.rope(t, poles[i-1][0], poles[i-1][1], p[0], p[1])
		}
	}
}

// mesh is what's been added, nil when nothing has.
func (m *ropeMesher) mesh() *Mesh {
	if len(m.verts) == 0 {
		return nil
	}
	return NewMesh(m.verts, m.idxs, []int{3, 3, 2}, nil)
}

// generateRopeMesh builds every rope line, nil when there are none.
func generateRopeMesh(w *world.World) *Mesh {
	var m ropeMesher
	for _, r := range w.Ropes {
		m.line(w.Terrain, r.Nodes)
	}
	return m.mesh()
}

// syncRopes rebuilds the rope mesh when the ropes have changed.
func (r *Renderer) syncRopes(w *world.World) {
	if r.scene.ropeRev == w.RopesRev && (r.scene.ropeMesh != nil || len(w.Ropes) == 0) {
		return
	}
	if r.scene.ropeMesh != nil {
		r.scene.ropeMesh.Delete()
		r.scene.ropeMesh = nil
	}
	r.scene.ropeMesh = generateRopeMesh(w)
	r.scene.ropeRev = w.RopesRev
}

// SetRopeGhost shows the rope being strung through nodes (with the next
// node at the cursor), red when it can't be; fewer than two nodes clears
// it.
func (r *Renderer) SetRopeGhost(w *world.World, nodes []mgl32.Vec2, ok bool) {
	if r.scene.ropeGhost != nil {
		r.scene.ropeGhost.Delete()
		r.scene.ropeGhost = nil
	}
	if len(nodes) < 2 {
		return
	}
	var m ropeMesher
	m.line(w.Terrain, nodes)
	r.scene.ropeGhost = m.mesh()
	r.scene.ropeGhostOK = ok
}
