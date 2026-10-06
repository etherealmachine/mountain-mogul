package render

import (
	"math"

	"mountain-mogul/internal/world"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

// The terrain mesh has one control point per cell corner, drawn as
// triangle patches that the tessellation stages subdivide by on-screen
// size and lift by snow depth and TerrainDetail. It is split into
// square chunks so off-screen ones are skipped. Per-corner snow state
// lives in two small textures the vertex shader reads, so snow changes
// never touch the vertex buffer.
//
// Vertex layout (terrainFloatsPerVert floats):
//
//	pos(3)          jittered corner XZ, ground Y (snow is added on the GPU)
//	grid(2)         corner-grid coordinate, for the snow textures and detail
//	kind(2)         (detail weight, wall flag): surface (1, 0), skirt wall
//	                top (1, 1), wall bottom and floor (0, 1)
//	smoothY(1)      low-pass elevation for the contour overlay
//	ao(1)           baked ambient occlusion
//	smoothNormal(3) per-corner smoothed normal; the face normal on walls
const (
	terrainChunkCells    = 32
	terrainFloatsPerVert = 12
	terrainCellSize      = float32(world.CellSize)
	terrainSkirtBaseY    = float32(-50.0)

	cornerSnowATexUnit = 6
	cornerSnowBTexUnit = 7
	detailTexUnit      = 8
	materialTexUnit    = 9

	// terrainTessPx is the on-screen length, in logical pixels, each
	// subdivided segment aims for.
	terrainTessPx = 7.0
	// terrainTessMax is the finest subdivision: 1.25 m without detail;
	// detail doubles it so its 1.25 m samples are resolved.
	terrainTessMax       = 4.0
	terrainTessMaxDetail = 8.0

	// terrainDispPad covers what the tessellation stage adds above snow
	// depth (mogul and powder bumps) when bounding chunks.
	terrainDispPad = float32(1.0)
)

// terrainChunk is an index range in the terrain mesh and its bounds
// before snow and detail.
type terrainChunk struct {
	first, count int32 // indices
	min, max     mgl32.Vec3
	always       bool // the skirt: drawn without culling
}

// terrainCorners is everything per corner that depends only on ground
// elevation.
type terrainCorners struct {
	w, h          int
	x, y, z       []float32 // jittered position; y is ground only
	smoothY, ao   []float32
	smoothNormals [][3]float32
}

func (c *terrainCorners) at(x, z int) int { return x*c.h + z }

// buildTerrainCorners precomputes corner positions, the smoothed
// elevation for contours and smooth normals, and baked AO.
func buildTerrainCorners(t *world.Terrain) *terrainCorners {
	W, H := t.Width, t.Height
	c := &terrainCorners{
		w: W, h: H,
		x: make([]float32, W*H), y: make([]float32, W*H), z: make([]float32, W*H),
		smoothY: make([]float32, W*H), ao: make([]float32, W*H),
		smoothNormals: make([][3]float32, W*H),
	}

	for z := 0; z < H; z++ {
		for x := 0; x < W; x++ {
			jx, _, jz := terrainJitterXYZ(x, z, W, H, terrainCellSize)
			s := cornerJitterScale(t, x, z)
			i := c.at(x, z)
			c.x[i] = float32(x)*terrainCellSize + jx*s
			c.y[i] = t.CornerGroundY(x, z)
			c.z[i] = float32(z)*terrainCellSize + jz*s
		}
	}

	// Smoothed elevation: separable 5-tap binomial (1,4,6,4,1)/16 over
	// surface elevation. 10 m of smoothing removes the cell stepping
	// that would make contour lines bend at every triangle edge, small
	// next to the 50 m contour interval.
	{
		base := make([]float32, W*H)
		for z := 0; z < H; z++ {
			for x := 0; x < W; x++ {
				base[c.at(x, z)] = t.SurfaceElevationAt(x, z)
			}
		}
		kernel := [5]float32{1, 4, 6, 4, 1}
		const kSum = float32(16)
		horiz := make([]float32, W*H)
		for z := 0; z < H; z++ {
			for x := 0; x < W; x++ {
				var sum float32
				for k := -2; k <= 2; k++ {
					sum += kernel[k+2] * base[c.at(min(max(x+k, 0), W-1), z)]
				}
				horiz[c.at(x, z)] = sum / kSum
			}
		}
		for z := 0; z < H; z++ {
			for x := 0; x < W; x++ {
				var sum float32
				for k := -2; k <= 2; k++ {
					sum += kernel[k+2] * horiz[c.at(x, min(max(z+k, 0), H-1))]
				}
				c.smoothY[c.at(x, z)] = sum / kSum
			}
		}
	}

	// AO: for each corner, march 8 rays at 3 radii and estimate the
	// elevation angle to the highest blocker above the local slope
	// plane, so an even slope isn't darkened just for being tilted.
	const aoRadiusMax = float32(30.0)
	const aoRings = 3
	const aoDirs = 8
	const aoEpsilon = float32(0.5)
	{
		var sinTab, cosTab [aoDirs]float32
		for d := 0; d < aoDirs; d++ {
			theta := float64(d) * 2 * math.Pi / float64(aoDirs)
			sinTab[d] = float32(math.Sin(theta))
			cosTab[d] = float32(math.Cos(theta))
		}
		bilinear := func(fx, fz float32) float32 {
			fx = min(max(fx, 0), float32(W-1))
			fz = min(max(fz, 0), float32(H-1))
			x0, z0 := int(fx), int(fz)
			x1, z1 := min(x0+1, W-1), min(z0+1, H-1)
			tx, tz := fx-float32(x0), fz-float32(z0)
			a := c.y[c.at(x0, z0)]*(1-tx) + c.y[c.at(x1, z0)]*tx
			b := c.y[c.at(x0, z1)]*(1-tx) + c.y[c.at(x1, z1)]*tx
			return a*(1-tz) + b*tz
		}
		for x := 0; x < W; x++ {
			for z := 0; z < H; z++ {
				p := c.y[c.at(x, z)]
				r1 := aoRadiusMax / float32(aoRings) / terrainCellSize
				gx := (bilinear(float32(x)+r1, float32(z)) - bilinear(float32(x)-r1, float32(z))) / (2 * r1 * terrainCellSize)
				gz := (bilinear(float32(x), float32(z)+r1) - bilinear(float32(x), float32(z)-r1)) / (2 * r1 * terrainCellSize)
				occ := float32(0)
				for ring := 1; ring <= aoRings; ring++ {
					rWorld := aoRadiusMax * float32(ring) / float32(aoRings)
					rCells := rWorld / terrainCellSize
					for d := 0; d < aoDirs; d++ {
						sy := bilinear(float32(x)+rCells*cosTab[d], float32(z)+rCells*sinTab[d])
						plane := p + (gx*cosTab[d]+gz*sinTab[d])*rWorld
						tan := (sy - plane - aoEpsilon) / rWorld
						if tan > 0 {
							occ += tan / (tan + 1.0)
						}
					}
				}
				c.ao[c.at(x, z)] = min(max(1.0-occ/float32(aoRings*aoDirs)*1.4, 0.15), 1.0)
			}
		}
	}

	// Smooth normals from central differences of the smoothed elevation,
	// shared by every triangle at a corner so lighting is continuous
	// across cell edges.
	for x := 0; x < W; x++ {
		for z := 0; z < H; z++ {
			xL, xR := max(x-1, 0), min(x+1, W-1)
			zU, zD := max(z-1, 0), min(z+1, H-1)
			var dYdx, dYdz float32
			if xR > xL {
				dYdx = (c.smoothY[c.at(xR, z)] - c.smoothY[c.at(xL, z)]) / (float32(xR-xL) * terrainCellSize)
			}
			if zD > zU {
				dYdz = (c.smoothY[c.at(x, zD)] - c.smoothY[c.at(x, zU)]) / (float32(zD-zU) * terrainCellSize)
			}
			nx, ny, nz := -dYdx, float32(1.0), -dYdz
			invL := 1 / float32(math.Sqrt(float64(nx*nx+ny*ny+nz*nz)))
			c.smoothNormals[c.at(x, z)] = [3]float32{nx * invL, ny * invL, nz * invL}
		}
	}
	return c
}

func smoothstep01(x float32) float32 {
	x = min(max(x, 0), 1)
	return x * x * (3 - 2*x)
}

// cellTris is the two triangles of cell (x, z) as corner offsets. The
// diagonal alternates in a checkerboard.
func cellTris(x, z int) [2][3][2]int {
	if (x+z)%2 == 0 {
		return [2][3][2]int{{{0, 0}, {1, 0}, {1, 1}}, {{0, 0}, {1, 1}, {0, 1}}}
	}
	return [2][3][2]int{{{0, 0}, {1, 0}, {0, 1}}, {{1, 0}, {1, 1}, {0, 1}}}
}

// buildTerrainGeometry builds the chunked surface and the skirt (four
// walls and a floor that make the map a diorama block). minY/maxY are
// the surface's ground range, for the topo and haze shaders.
func buildTerrainGeometry(t *world.Terrain) (verts []float32, indices []uint32, chunks []terrainChunk, minY, maxY float32) {
	W, H := t.Width, t.Height
	c := buildTerrainCorners(t)
	chunksX := (W - 2 + terrainChunkCells) / terrainChunkCells
	chunksZ := (H - 2 + terrainChunkCells) / terrainChunkCells
	verts = make([]float32, 0, (W+chunksX)*(H+chunksZ)*terrainFloatsPerVert)
	indices = make([]uint32, 0, (W-1)*(H-1)*6)
	minY, maxY = float32(math.Inf(1)), float32(math.Inf(-1))

	vert := func(i int, gx, gz float32) {
		n := c.smoothNormals[i]
		verts = append(verts,
			c.x[i], c.y[i], c.z[i],
			gx, gz,
			1, 0,
			c.smoothY[i], c.ao[i],
			n[0], n[1], n[2],
		)
	}

	for cj := 0; cj < chunksZ; cj++ {
		for ci := 0; ci < chunksX; ci++ {
			x0, z0 := ci*terrainChunkCells, cj*terrainChunkCells
			x1, z1 := min(x0+terrainChunkCells, W-1), min(z0+terrainChunkCells, H-1)
			base := uint32(len(verts) / terrainFloatsPerVert)
			lo := float32(math.Inf(1))
			hi := float32(math.Inf(-1))
			nx := x1 - x0 + 1
			for z := z0; z <= z1; z++ {
				for x := x0; x <= x1; x++ {
					i := c.at(x, z)
					vert(i, float32(x), float32(z))
					lo, hi = min(lo, c.y[i]), max(hi, c.y[i])
				}
			}
			first := int32(len(indices))
			for z := z0; z < z1; z++ {
				for x := x0; x < x1; x++ {
					for _, tri := range cellTris(x, z) {
						for _, o := range tri {
							indices = append(indices, base+uint32((z+o[1]-z0)*nx+(x+o[0]-x0)))
						}
					}
				}
			}
			chunks = append(chunks, terrainChunk{
				first: first,
				count: int32(len(indices)) - first,
				min:   mgl32.Vec3{float32(x0)*terrainCellSize - 1, lo, float32(z0)*terrainCellSize - 1},
				max:   mgl32.Vec3{float32(x1)*terrainCellSize + 1, hi, float32(z1)*terrainCellSize + 1},
			})
			minY, maxY = min(minY, lo), max(maxY, hi)
		}
	}
	if !(minY < maxY) {
		minY, maxY = 0, 1
	}

	// Skirt. Wall tops follow the surface's boundary corners (whose
	// perpendicular jitter is zero, so each wall stays planar) and carry
	// detail; bottoms sit at terrainSkirtBaseY.
	const wallAO = float32(0.75)
	const floorAO = float32(0.20)
	first := int32(len(indices))
	type sv struct {
		p      [3]float32
		gx, gz float32
		top    bool
	}
	emitTri := func(n [3]float32, ao float32, vs ...sv) {
		for _, v := range vs {
			w := float32(0)
			if v.top {
				w = 1
			}
			indices = append(indices, uint32(len(verts)/terrainFloatsPerVert))
			verts = append(verts,
				v.p[0], v.p[1], v.p[2],
				v.gx, v.gz,
				w, 1,
				v.p[1], // smoothY = y so contour bands stay horizontal on walls
				ao,
				n[0], n[1], n[2],
			)
		}
	}
	wall := func(a, b [2]int, n [3]float32) {
		ia, ib := c.at(a[0], a[1]), c.at(b[0], b[1])
		ta := sv{[3]float32{c.x[ia], c.y[ia], c.z[ia]}, float32(a[0]), float32(a[1]), true}
		tb := sv{[3]float32{c.x[ib], c.y[ib], c.z[ib]}, float32(b[0]), float32(b[1]), true}
		bb := sv{[3]float32{c.x[ib], terrainSkirtBaseY, c.z[ib]}, tb.gx, tb.gz, false}
		ba := sv{[3]float32{c.x[ia], terrainSkirtBaseY, c.z[ia]}, ta.gx, ta.gz, false}
		emitTri(n, wallAO, ta, tb, bb)
		emitTri(n, wallAO, ta, bb, ba)
	}
	for x := 0; x < W-1; x++ {
		wall([2]int{x, 0}, [2]int{x + 1, 0}, [3]float32{0, 0, -1})
		wall([2]int{x + 1, H - 1}, [2]int{x, H - 1}, [3]float32{0, 0, 1})
	}
	for z := 0; z < H-1; z++ {
		wall([2]int{0, z + 1}, [2]int{0, z}, [3]float32{-1, 0, 0})
		wall([2]int{W - 1, z}, [2]int{W - 1, z + 1}, [3]float32{1, 0, 0})
	}
	maxX := float32(W-1) * terrainCellSize
	maxZ := float32(H-1) * terrainCellSize
	f00 := sv{p: [3]float32{0, terrainSkirtBaseY, 0}}
	f10 := sv{p: [3]float32{maxX, terrainSkirtBaseY, 0}}
	f11 := sv{p: [3]float32{maxX, terrainSkirtBaseY, maxZ}}
	f01 := sv{p: [3]float32{0, terrainSkirtBaseY, maxZ}}
	emitTri([3]float32{0, -1, 0}, floorAO, f00, f10, f11)
	emitTri([3]float32{0, -1, 0}, floorAO, f00, f11, f01)
	chunks = append(chunks, terrainChunk{first: first, count: int32(len(indices)) - first, always: true})

	return verts, indices, chunks, minY, maxY
}

// terrainJitterXYZ returns deterministic per-corner offsets in X and Z
// (Y is zero) so the cell grid doesn't read as a square lattice from
// above. Boundary corners zero their perpendicular component so the
// skirt walls stay planar.
func terrainJitterXYZ(gx, gz, width, height int, cellSize float32) (float32, float32, float32) {
	hX := uint32(gx)*0x9E3779B1 ^ uint32(gz)*0x85EBCA77
	hX ^= hX >> 16
	hX *= 0xC2B2AE3D
	hX ^= hX >> 16

	hZ := uint32(gx)*0x27D4EB2F ^ uint32(gz)*0x165667B1
	hZ ^= hZ >> 16
	hZ *= 0xD3A2646C
	hZ ^= hZ >> 16

	const inv = 1.0 / float32(^uint32(0))
	fx := (float32(hX)*inv - 0.5) * 0.4 * cellSize
	fz := (float32(hZ)*inv - 0.5) * 0.4 * cellSize

	if gx == 0 || gx == width-1 {
		fx = 0
	}
	if gz == 0 || gz == height-1 {
		fz = 0
	}
	return fx, 0, fz
}

// cornerSnow fills the per-corner snow textures for corners
// [x0, x1] × [z0, z1]: a holds (Grooming, Packed, Ice, MogulSize) and b
// (visible depth, instability), each the average of the corner's cells
// so groomed patches fade into their neighbours instead of stopping at a
// cell edge. Rows run along x. maxDepth is the deepest corner.
func cornerSnow(t *world.Terrain, x0, z0, x1, z1 int, a, b []float32) (maxDepth float32) {
	W, H := t.Width, t.Height
	rw := x1 - x0 + 1
	for cz := z0; cz <= z1; cz++ {
		for cx := x0; cx <= x1; cx++ {
			var g, pk, ic, mg, dp, is, n float32
			for z := max(cz-1, 0); z <= min(cz, H-1); z++ {
				for x := max(cx-1, 0); x <= min(cx, W-1); x++ {
					c := &t.Cells[x][z]
					g += c.Grooming
					pk += c.SurfacePacked()
					ic += c.SurfaceIce()
					mg += c.MogulSize
					dp += c.VisibleSnowDepth()
					is += c.InstabilityScore()
					n++
				}
			}
			i := (cz-z0)*rw + (cx - x0)
			if n == 0 {
				copy(a[i*4:i*4+4], []float32{0, 0, 0, 0})
				b[i*2], b[i*2+1] = 0, 0
				continue
			}
			inv := 1 / n
			a[i*4], a[i*4+1], a[i*4+2], a[i*4+3] = g*inv, pk*inv, ic*inv, mg*inv
			b[i*2], b[i*2+1] = dp*inv, is*inv
			maxDepth = max(maxDepth, dp*inv)
		}
	}
	return maxDepth
}

// cornerSurfaceY is the drawn surface height at corner (cx, cz) before
// detail: ground plus visible snow, each averaged over the corner's
// cells exactly as the vertex buffer and snow textures do.
func cornerSurfaceY(t *world.Terrain, cx, cz int) float32 {
	var dp, n float32
	for x := max(cx-1, 0); x <= min(cx, t.Width-1); x++ {
		for z := max(cz-1, 0); z <= min(cz, t.Height-1); z++ {
			dp += t.Cells[x][z].VisibleSnowDepth()
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return t.CornerGroundY(cx, cz) + dp*(1/n)
}

// VisualElevationAt returns the terrain surface height at world position
// (wx, wz): the jittered triangle under the point, interpolated from its
// corners' ground and snow, plus TerrainDetail. The GPU subdivides the
// same triangles and adds the same values, so agents and objects sit on
// the drawn surface; only the mogul and powder bumps (under a metre,
// noise) are left out.
func VisualElevationAt(t *world.Terrain, wx, wz float32) float32 {
	if t.Width < 2 || t.Height < 2 {
		return 0
	}
	maxX := float32(t.Width-1) * terrainCellSize
	maxZ := float32(t.Height-1) * terrainCellSize
	wx = min(max(wx, 0), maxX)
	wz = min(max(wz, 0), maxZ)
	xi := min(int(wx/terrainCellSize), t.Width-2)
	zi := min(int(wz/terrainCellSize), t.Height-2)

	corner := func(x, z int) (float32, float32) {
		jx, _, jz := terrainJitterXYZ(x, z, t.Width, t.Height, terrainCellSize)
		s := cornerJitterScale(t, x, z)
		return float32(x)*terrainCellSize + jx*s, float32(z)*terrainCellSize + jz*s
	}

	// Jitter moves corners up to 1 m, so the point may lie in a
	// neighbouring cell's triangle. Keep the best fit in case rounding
	// leaves it just outside every one.
	bestScore := float32(math.Inf(-1))
	var best [3][2]int
	var bestW [3]float32
	for _, d := range [9][2]int{{0, 0}, {-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		x, z := xi+d[0], zi+d[1]
		if x < 0 || z < 0 || x > t.Width-2 || z > t.Height-2 {
			continue
		}
		for _, tri := range cellTris(x, z) {
			var ps [3][2]float32
			var cs [3][2]int
			for k, o := range tri {
				cs[k] = [2]int{x + o[0], z + o[1]}
				ps[k][0], ps[k][1] = corner(cs[k][0], cs[k][1])
			}
			w := barycentric2(ps, wx, wz)
			score := min(w[0], w[1], w[2])
			if score > bestScore {
				bestScore, best, bestW = score, cs, w
			}
		}
		if bestScore >= -1e-5 {
			break
		}
	}

	var y, gx, gz float32
	for k := 0; k < 3; k++ {
		y += bestW[k] * cornerSurfaceY(t, best[k][0], best[k][1])
		gx += bestW[k] * float32(best[k][0])
		gz += bestW[k] * float32(best[k][1])
	}
	if t.Detail != nil {
		y += t.Detail.At(gx, gz)
	}
	return y
}

// cornerJitterScale damps jitter at thin-snow corners (lift station
// aprons drive visible depth to ~5 cm) so graded earthwork reads flat
// instead of pebbled. A corner takes the minimum visible depth of its
// cells, so any thin-snow neighbour pulls it toward exact.
func cornerJitterScale(t *world.Terrain, x, z int) float32 {
	minDepth := float32(math.Inf(1))
	for ox := max(x-1, 0); ox <= min(x, t.Width-1); ox++ {
		for oz := max(z-1, 0); oz <= min(z, t.Height-1); oz++ {
			minDepth = min(minDepth, t.Cells[ox][oz].VisibleSnowDepth())
		}
	}
	if math.IsInf(float64(minDepth), 1) {
		return 1
	}
	return smoothstep01((minDepth - 0.5) / 1.0)
}

// barycentric2 is the barycentric weights of (px, pz) in triangle ps.
func barycentric2(ps [3][2]float32, px, pz float32) [3]float32 {
	x0, z0 := ps[0][0], ps[0][1]
	d1x, d1z := ps[1][0]-x0, ps[1][1]-z0
	d2x, d2z := ps[2][0]-x0, ps[2][1]-z0
	det := d1x*d2z - d2x*d1z
	if det == 0 {
		return [3]float32{1, 0, 0}
	}
	qx, qz := px-x0, pz-z0
	u := (qx*d2z - d2x*qz) / det
	v := (d1x*qz - qx*d1z) / det
	return [3]float32{1 - u - v, u, v}
}

// BuildTerrainMesh creates the terrain mesh, snow textures, and detail
// texture for t.
func (r *Renderer) BuildTerrainMesh(t *world.Terrain) {
	r.scene.terrainWidth = t.Width
	r.scene.terrainHeight = t.Height
	if r.scene.terrainMesh != nil {
		r.scene.terrainMesh.Delete()
	}

	m := &Mesh{}
	gl.GenVertexArrays(1, &m.VAO)
	gl.GenBuffers(1, &m.VBO)
	gl.GenBuffers(1, &m.EBO)
	gl.BindVertexArray(m.VAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, m.VBO)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, m.EBO)
	stride := int32(terrainFloatsPerVert * 4)
	for i, a := range []struct{ size, offset int32 }{
		{3, 0}, // aPos
		{2, 3}, // aGrid
		{2, 5}, // aKind
		{1, 7}, // aSmoothY
		{1, 8}, // aAO
		{3, 9}, // aSmoothNormal
	} {
		gl.EnableVertexAttribArray(uint32(i))
		gl.VertexAttribPointerWithOffset(uint32(i), a.size, gl.FLOAT, false, stride, uintptr(a.offset*4))
	}
	gl.BindVertexArray(0)
	r.scene.terrainMesh = m

	r.uploadTerrainGeometry(t)
	r.FlushSnowState(t)
	r.FlushTerrainDetail(t)
	r.FlushTerrainMaterial(t)
}

func (r *Renderer) uploadTerrainGeometry(t *world.Terrain) {
	verts, indices, chunks, minY, maxY := buildTerrainGeometry(t)
	r.scene.terrainMinY = minY
	r.scene.terrainMaxY = maxY
	r.scene.terrainChunks = chunks
	m := r.scene.terrainMesh
	m.IndexCount = int32(len(indices))
	gl.BindVertexArray(m.VAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, m.VBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.STATIC_DRAW)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)
	gl.BindVertexArray(0)
}

// FlushTerrainVerts rebuilds the terrain after ground elevation changes:
// geometry, AO, smoothed elevation, snow, and detail.
func (r *Renderer) FlushTerrainVerts(t *world.Terrain) {
	if r.scene.terrainMesh == nil {
		return
	}
	r.uploadTerrainGeometry(t)
	r.FlushSnowState(t)
	r.FlushTerrainDetail(t)
	r.FlushTerrainMaterial(t)
}

// FlushSnowState re-uploads the per-corner snow textures after snow
// state changes (snowfall, grooming, packing, moguls).
func (r *Renderer) FlushSnowState(t *world.Terrain) {
	if r.scene.terrainMesh == nil {
		return
	}
	W, H := t.Width, t.Height
	a := make([]float32, W*H*4)
	b := make([]float32, W*H*2)
	r.scene.terrainSnowPad = cornerSnow(t, 0, 0, W-1, H-1, a, b) + terrainDispPad
	if r.scene.cornerSnowTexA == 0 {
		r.scene.cornerSnowTexA = newDataTexture(gl.RGBA16F, W, H)
		r.scene.cornerSnowTexB = newDataTexture(gl.RG32F, W, H)
	}
	gl.BindTexture(gl.TEXTURE_2D, r.scene.cornerSnowTexA)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, int32(W), int32(H), gl.RGBA, gl.FLOAT, gl.Ptr(a))
	gl.BindTexture(gl.TEXTURE_2D, r.scene.cornerSnowTexB)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, int32(W), int32(H), gl.RG, gl.FLOAT, gl.Ptr(b))
	gl.BindTexture(gl.TEXTURE_2D, 0)
}

// FlushInstabilityCells refreshes the snow textures around cells
// [x0, x1] × [z0, z1] (inclusive), for edits that change only
// instability, e.g. tree density (trees anchor the snowpack).
func (r *Renderer) FlushInstabilityCells(t *world.Terrain, x0, z0, x1, z1 int) {
	if r.scene.cornerSnowTexA == 0 {
		return
	}
	// Cells feed the corners [x0, x1+1] × [z0, z1+1].
	cx0, cz0 := max(x0, 0), max(z0, 0)
	cx1, cz1 := min(x1+1, t.Width-1), min(z1+1, t.Height-1)
	if cx0 > cx1 || cz0 > cz1 {
		return
	}
	w, h := cx1-cx0+1, cz1-cz0+1
	a := make([]float32, w*h*4)
	b := make([]float32, w*h*2)
	cornerSnow(t, cx0, cz0, cx1, cz1, a, b)
	gl.BindTexture(gl.TEXTURE_2D, r.scene.cornerSnowTexA)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, int32(cx0), int32(cz0), int32(w), int32(h), gl.RGBA, gl.FLOAT, gl.Ptr(a))
	gl.BindTexture(gl.TEXTURE_2D, r.scene.cornerSnowTexB)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, int32(cx0), int32(cz0), int32(w), int32(h), gl.RG, gl.FLOAT, gl.Ptr(b))
	gl.BindTexture(gl.TEXTURE_2D, 0)
}

// FlushTerrainDetail uploads t.Detail, or frees the texture when there
// is none.
func (r *Renderer) FlushTerrainDetail(t *world.Terrain) {
	s := r.scene
	if t.Detail == nil {
		if s.detailTex != 0 {
			gl.DeleteTextures(1, &s.detailTex)
			s.detailTex = 0
		}
		s.detailPad = 0
		return
	}
	d := t.Detail
	if s.detailTex == 0 || s.detailW != d.W || s.detailH != d.H {
		if s.detailTex != 0 {
			gl.DeleteTextures(1, &s.detailTex)
		}
		s.detailTex = newDataTexture(gl.R32F, d.W, d.H)
		gl.BindTexture(gl.TEXTURE_2D, s.detailTex)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		s.detailW, s.detailH = d.W, d.H
	}
	gl.BindTexture(gl.TEXTURE_2D, s.detailTex)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, int32(d.W), int32(d.H), gl.RED, gl.FLOAT, gl.Ptr(d.Off))
	gl.BindTexture(gl.TEXTURE_2D, 0)
	s.detailPad = d.MaxAbs()
}

// FlushTerrainMaterial uploads t.Material when it has changed, or frees
// the texture when there is none.
func (r *Renderer) FlushTerrainMaterial(t *world.Terrain) {
	s := r.scene
	m := t.Material
	if m == s.materialSrc && (m == nil) == (s.materialTex == 0) && (m == nil || m.Version == s.materialVer) {
		return
	}
	s.materialSrc = m
	if m != nil {
		s.materialVer = m.Version
	}
	if m == nil {
		if s.materialTex != 0 {
			gl.DeleteTextures(1, &s.materialTex)
			s.materialTex = 0
		}
		return
	}
	if s.materialTex == 0 || s.materialW != m.W || s.materialH != m.H {
		if s.materialTex != 0 {
			gl.DeleteTextures(1, &s.materialTex)
		}
		gl.GenTextures(1, &s.materialTex)
		gl.BindTexture(gl.TEXTURE_2D, s.materialTex)
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.R8, int32(m.W), int32(m.H), 0, gl.RED, gl.UNSIGNED_BYTE, nil)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
		s.materialW, s.materialH = m.W, m.H
	}
	gl.BindTexture(gl.TEXTURE_2D, s.materialTex)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, int32(m.W), int32(m.H), gl.RED, gl.UNSIGNED_BYTE, gl.Ptr(m.Bytes()))
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	gl.BindTexture(gl.TEXTURE_2D, 0)
}

// newDataTexture allocates a nearest-sampled, edge-clamped float texture.
func newDataTexture(internal int32, w, h int) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	format := uint32(gl.RGBA)
	switch internal {
	case gl.RG32F:
		format = gl.RG
	case gl.R32F:
		format = gl.RED
	}
	gl.TexImage2D(gl.TEXTURE_2D, 0, internal, int32(w), int32(h), 0, format, gl.FLOAT, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	return tex
}

// drawTerrain binds the terrain's textures and tessellation uniforms on
// the already-active terrain shader and draws the chunks in view.
func (r *Renderer) drawTerrain(sh *Shader, vp mgl32.Mat4) {
	s := r.scene
	sh.SetInt("uCornerSnowA", cornerSnowATexUnit)
	sh.SetInt("uCornerSnowB", cornerSnowBTexUnit)
	sh.SetInt("uDetail", detailTexUnit)
	gl.ActiveTexture(gl.TEXTURE0 + cornerSnowATexUnit)
	gl.BindTexture(gl.TEXTURE_2D, s.cornerSnowTexA)
	gl.ActiveTexture(gl.TEXTURE0 + cornerSnowBTexUnit)
	gl.BindTexture(gl.TEXTURE_2D, s.cornerSnowTexB)
	gl.ActiveTexture(gl.TEXTURE0 + detailTexUnit)
	tessMax := float32(terrainTessMax)
	if s.detailTex != 0 {
		gl.BindTexture(gl.TEXTURE_2D, s.detailTex)
		sh.SetFloat("uDetailOn", 1)
		sh.SetVec2("uDetailSize", mgl32.Vec2{float32(s.detailW), float32(s.detailH)})
		tessMax = terrainTessMaxDetail
	} else {
		gl.BindTexture(gl.TEXTURE_2D, r.transparentTexID)
		sh.SetFloat("uDetailOn", 0)
		sh.SetVec2("uDetailSize", mgl32.Vec2{1, 1})
	}
	sh.SetInt("uMaterial", materialTexUnit)
	gl.ActiveTexture(gl.TEXTURE0 + materialTexUnit)
	if s.materialTex != 0 {
		gl.BindTexture(gl.TEXTURE_2D, s.materialTex)
		sh.SetFloat("uMaterialOn", 1)
		sh.SetVec2("uMaterialSize", mgl32.Vec2{float32(s.materialW), float32(s.materialH)})
	} else {
		gl.BindTexture(gl.TEXTURE_2D, r.transparentTexID)
		sh.SetFloat("uMaterialOn", 0)
		sh.SetVec2("uMaterialSize", mgl32.Vec2{1, 1})
	}
	gl.ActiveTexture(gl.TEXTURE0)
	sh.SetVec2("uViewport", mgl32.Vec2{float32(r.logicalW), float32(r.logicalH)})
	sh.SetFloat("uTessPx", terrainTessPx)
	sh.SetFloat("uTessMax", tessMax)

	gl.PatchParameteri(gl.PATCH_VERTICES, 3)
	gl.BindVertexArray(s.terrainMesh.VAO)
	padHi := s.terrainSnowPad + s.detailPad
	drawn := 0
	for _, c := range s.terrainChunks {
		if !c.always && !boxInView(vp, c.min.Sub(mgl32.Vec3{0, s.detailPad, 0}), c.max.Add(mgl32.Vec3{0, padHi, 0})) {
			continue
		}
		gl.DrawElementsWithOffset(gl.PATCHES, c.count, gl.UNSIGNED_INT, uintptr(c.first)*4)
		drawn++
	}
	gl.BindVertexArray(0)
	r.terrainChunksDrawn = drawn
}

// TerrainChunkStats is how many terrain chunks the last frame drew, out
// of how many there are (the skirt counts as one).
func (r *Renderer) TerrainChunkStats() (drawn, total int) {
	return r.terrainChunksDrawn, len(r.scene.terrainChunks)
}

// boxInView reports whether any part of the box may be inside the view
// frustum: false only when all eight corners lie outside one clip plane.
func boxInView(vp mgl32.Mat4, lo, hi mgl32.Vec3) bool {
	var out [6]int
	for i := 0; i < 8; i++ {
		p := mgl32.Vec4{lo[0], lo[1], lo[2], 1}
		if i&1 != 0 {
			p[0] = hi[0]
		}
		if i&2 != 0 {
			p[1] = hi[1]
		}
		if i&4 != 0 {
			p[2] = hi[2]
		}
		c := vp.Mul4x1(p)
		if c[0] < -c[3] {
			out[0]++
		}
		if c[0] > c[3] {
			out[1]++
		}
		if c[1] < -c[3] {
			out[2]++
		}
		if c[1] > c[3] {
			out[3]++
		}
		if c[2] < -c[3] {
			out[4]++
		}
		if c[2] > c[3] {
			out[5]++
		}
	}
	for _, n := range out {
		if n == 8 {
			return false
		}
	}
	return true
}
