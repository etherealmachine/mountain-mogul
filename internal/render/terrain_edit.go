package render

import (
	"unsafe"

	"mountain-mogul/internal/world"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// terrainEditMargin is how far round changed ground the mesh is rebuilt,
// in corners: AO reads aoReachCorners away and the smoothed elevation
// and its normals 3, plus a corner kept fixed at the edge (see
// FlushTerrainEdit).
const terrainEditMargin = aoReachCorners + 2

// FlushTerrainEdit is FlushTerrainVerts for a local edit (a building,
// lift, road or brush stroke): it compares the ground with what the mesh
// was built from and rebuilds and resends only the chunks round what
// changed, and only the changed part of the detail texture. Rebuilding
// all of Kirkwood took about 180 ms plus 100 MB of uploads per edit.
//
// Unlike FlushTerrainVerts it doesn't bring the snow-shaped parts of the
// mesh (corner jitter, the smoothed elevation) up to date away from the
// edit, so whole-map snow changes (Auto snow, Clear Snow) use that.
func (r *Renderer) FlushTerrainEdit(t *world.Terrain) {
	s := r.scene
	c := s.terrainCorners
	if s.terrainMesh == nil || c == nil || c.w != t.Width || c.h != t.Height {
		r.FlushTerrainVerts(t)
		return
	}
	r.MarkWaterDirty()

	// The changed corners' box, taking their new ground as we go: AO
	// round the box reads it.
	W, H := c.w, c.h
	x0, z0, x1, z1 := W, H, -1, -1
	for x := 0; x < W; x++ {
		for z := 0; z < H; z++ {
			y := t.CornerGroundY(x, z)
			if i := c.at(x, z); c.y[i] != y {
				c.y[i] = y
				x0, z0 = min(x0, x), min(z0, z)
				x1, z1 = max(x1, x), max(z1, z)
			}
		}
	}
	if x1 >= 0 {
		const vertBytes = terrainFloatsPerVert * 4
		gl.BindBuffer(gl.ARRAY_BUFFER, s.terrainMesh.VBO)
		s.rebuildTerrainChunks(t, x0-terrainEditMargin, z0-terrainEditMargin, x1+terrainEditMargin, z1+terrainEditMargin,
			func(vert int, verts []float32) {
				gl.BufferSubData(gl.ARRAY_BUFFER, vert*vertBytes, len(verts)*4, gl.Ptr(verts))
			})
		gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	}
	r.FlushSnowState(t)
	r.flushTerrainDetailEdit(t)
	r.FlushTerrainMaterial(t)
}

// rebuildTerrainChunks rebuilds the chunks touching corners [x0, x1] ×
// [z0, z1], handing upload each run of vertices from vertex vert on.
func (s *SceneResources) rebuildTerrainChunks(t *world.Terrain, x0, z0, x1, z1 int, upload func(vert int, verts []float32)) {
	c := s.terrainCorners
	W, H := c.w, c.h
	chunksX := (W - 2 + terrainChunkCells) / terrainChunkCells
	chunksZ := (H - 2 + terrainChunkCells) / terrainChunkCells
	ci0, cj0 := max(x0, 0)/terrainChunkCells, max(z0, 0)/terrainChunkCells
	ci1 := min(min(x1, W-2)/terrainChunkCells, chunksX-1)
	cj1 := min(min(z1, H-2)/terrainChunkCells, chunksZ-1)

	// The chunks' corners. Their outer ring is shared with chunks left as
	// they are, so it keeps its values (it is margin away from the
	// change); only the map's own edges are recomputed.
	rx0, rz0 := ci0*terrainChunkCells, cj0*terrainChunkCells
	rx1 := min((ci1+1)*terrainChunkCells, W-1)
	rz1 := min((cj1+1)*terrainChunkCells, H-1)
	fx0, fz0, fx1, fz1 := rx0+1, rz0+1, rx1-1, rz1-1
	if rx0 == 0 {
		fx0 = 0
	}
	if rz0 == 0 {
		fz0 = 0
	}
	if rx1 == W-1 {
		fx1 = W - 1
	}
	if rz1 == H-1 {
		fz1 = H - 1
	}
	c.fill(t, fx0, fz0, fx1, fz1)

	var verts []float32
	for cj := cj0; cj <= cj1; cj++ {
		for ci := ci0; ci <= ci1; ci++ {
			ch := &s.terrainChunks[cj*chunksX+ci]
			var lo, hi float32
			verts, lo, hi = c.appendChunkVerts(verts[:0], ch.cells[0], ch.cells[1], ch.cells[2], ch.cells[3])
			ch.setBounds(lo, hi)
			upload(int(ch.vert), verts)
		}
	}
	if rx0 == 0 || rz0 == 0 || rx1 == W-1 || rz1 == H-1 {
		verts, _ = c.appendSkirt(verts[:0], nil)
		upload(s.terrainSkirtVert, verts)
	}

	minY, maxY := s.terrainChunks[0].min[1], s.terrainChunks[0].max[1]
	for i := range s.terrainChunks {
		if ch := &s.terrainChunks[i]; !ch.always {
			minY, maxY = min(minY, ch.min[1]), max(maxY, ch.max[1])
		}
	}
	if minY < maxY {
		s.terrainMinY, s.terrainMaxY = minY, maxY
	}
}

// flushTerrainDetailEdit resends the box of detail samples that changed
// since the last upload.
func (r *Renderer) flushTerrainDetailEdit(t *world.Terrain) {
	s := r.scene
	d := t.Detail
	if d == nil || s.detailTex == 0 || s.detailW != d.W || s.detailH != d.H || len(s.detailCopy) != len(d.Off) {
		r.FlushTerrainDetail(t)
		return
	}
	x0, z0, x1, z1 := d.W, d.H, -1, -1
	for j := 0; j < d.H; j++ {
		row, old := d.Off[j*d.W:(j+1)*d.W], s.detailCopy[j*d.W:(j+1)*d.W]
		for i := range row {
			if row[i] != old[i] {
				x0, x1 = min(x0, i), max(x1, i)
				z0, z1 = min(z0, j), max(z1, j)
			}
		}
	}
	if x1 < 0 {
		return
	}
	for j := z0; j <= z1; j++ {
		copy(s.detailCopy[j*d.W+x0:j*d.W+x1+1], d.Off[j*d.W+x0:j*d.W+x1+1])
	}
	gl.BindTexture(gl.TEXTURE_2D, s.detailTex)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, int32(d.W))
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, int32(x0), int32(z0), int32(x1-x0+1), int32(z1-z0+1),
		gl.RED, gl.FLOAT, unsafe.Pointer(&d.Off[z0*d.W+x0]))
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	s.detailPad = d.MaxAbs()
}
