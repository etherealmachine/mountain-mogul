package render

import (
	"math"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// Mesh ID constants — also mirrored in world/objects.go to avoid circular imports.
const (
	MeshTree        uint32 = 0
	MeshTree2       uint32 = 1
	MeshTree3       uint32 = 2
	MeshRock        uint32 = 3
	MeshStump       uint32 = 4
	MeshBuilding    uint32 = 5
	MeshTower       uint32 = 6
	MeshAgent       uint32 = 7
	MeshLiftStation uint32 = 8
	MeshChair       uint32 = 9
)

// Mesh wraps a GPU vertex/index buffer.
//
// Layout is retained so instance Batches can build their own VAOs that bind
// the same per-vertex attributes — without that, two Batches sharing this
// Mesh would fight over its VAO's instance-attribute slots and corrupt each
// other's draw state.
type Mesh struct {
	VAO, VBO, EBO uint32
	IndexCount    int32
	Layout        []int // per-vertex attribute sizes in floats
}

// NewMesh creates a GPU mesh from vertex and index data.
// layout is the vertex attribute sizes in floats, e.g. [3,3,2] for pos/normal/uv.
func NewMesh(vertices []float32, indices []uint32, layout []int) *Mesh {
	m := &Mesh{IndexCount: int32(len(indices)), Layout: layout}

	gl.GenVertexArrays(1, &m.VAO)
	gl.GenBuffers(1, &m.VBO)
	gl.GenBuffers(1, &m.EBO)

	gl.BindVertexArray(m.VAO)

	gl.BindBuffer(gl.ARRAY_BUFFER, m.VBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, m.EBO)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

	// calculate stride
	stride := 0
	for _, s := range layout {
		stride += s
	}
	strideBytes := int32(stride * 4)

	offset := 0
	for i, size := range layout {
		gl.EnableVertexAttribArray(uint32(i))
		gl.VertexAttribPointerWithOffset(uint32(i), int32(size), gl.FLOAT, false, strideBytes, uintptr(offset*4))
		offset += size
	}

	gl.BindVertexArray(0)

	return m
}

// Draw binds the mesh and draws it.
func (m *Mesh) Draw() {
	gl.BindVertexArray(m.VAO)
	gl.DrawElements(gl.TRIANGLES, m.IndexCount, gl.UNSIGNED_INT, unsafe.Pointer(nil))
	gl.BindVertexArray(0)
}

// Delete frees GPU resources.
func (m *Mesh) Delete() {
	gl.DeleteVertexArrays(1, &m.VAO)
	gl.DeleteBuffers(1, &m.VBO)
	gl.DeleteBuffers(1, &m.EBO)
}

// NewChairMesh generates a gondola/chair shape for instanced rendering.
// The origin (0,0,0) is the cable-attachment point; the seat hangs below.
// Vertex layout: pos(3) + normal(3) — matches the dynamic shader (no UV needed).
func NewChairMesh() *Mesh {
	// Build the chair from a few simple quads.
	// All Y values are <= 0 so the dynamic shader's limb-animation threshold (0.3) never fires.
	type face struct {
		verts  [4][3]float32
		normal [3]float32
	}

	addBox := func(verts *[]float32, indices *[]uint32, minX, maxX, minY, maxY, minZ, maxZ float32) {
		faces := []face{
			{[4][3]float32{{minX, minY, maxZ}, {maxX, minY, maxZ}, {maxX, maxY, maxZ}, {minX, maxY, maxZ}}, [3]float32{0, 0, 1}},
			{[4][3]float32{{maxX, minY, minZ}, {minX, minY, minZ}, {minX, maxY, minZ}, {maxX, maxY, minZ}}, [3]float32{0, 0, -1}},
			{[4][3]float32{{maxX, minY, maxZ}, {maxX, minY, minZ}, {maxX, maxY, minZ}, {maxX, maxY, maxZ}}, [3]float32{1, 0, 0}},
			{[4][3]float32{{minX, minY, minZ}, {minX, minY, maxZ}, {minX, maxY, maxZ}, {minX, maxY, minZ}}, [3]float32{-1, 0, 0}},
			{[4][3]float32{{minX, maxY, maxZ}, {maxX, maxY, maxZ}, {maxX, maxY, minZ}, {minX, maxY, minZ}}, [3]float32{0, 1, 0}},
			{[4][3]float32{{minX, minY, minZ}, {maxX, minY, minZ}, {maxX, minY, maxZ}, {minX, minY, maxZ}}, [3]float32{0, -1, 0}},
		}
		for _, f := range faces {
			base := uint32(len(*verts) / 6)
			for _, p := range f.verts {
				*verts = append(*verts, p[0], p[1], p[2], f.normal[0], f.normal[1], f.normal[2])
			}
			*indices = append(*indices, base, base+1, base+2, base, base+2, base+3)
		}
	}

	var verts []float32
	var indices []uint32

	// Suspension bar connecting cable to seat frame.
	addBox(&verts, &indices, -0.05, 0.05, -1.3, 0, -0.05, 0.05)
	// Seat: 1.5 m wide, 0.25 m thick, 0.7 m deep.
	addBox(&verts, &indices, -0.75, 0.75, -1.55, -1.3, -0.35, 0.35)
	// Seat back: vertical panel behind the seat.
	addBox(&verts, &indices, -0.75, 0.75, -1.3, -0.8, -0.35, -0.2)
	// Foot bar at bottom for passenger feet.
	addBox(&verts, &indices, -0.75, 0.75, -1.85, -1.8, -0.05, 0.05)

	return NewMesh(verts, indices, []int{3, 3})
}

// NewLowPolyTreeMesh builds a stylized conifer — a hexagonal-pyramid cone
// (or two stacked cones) on a short trunk. Origin at base, +Y up. Mesh
// units roughly match the previous OBJ trees (~5–7 m tall) so the
// existing instance-scale range continues to land at 11–14 m world height.
//
// Variant in [0, 3) picks different proportions; each variant lives in
// its own static batch, so distinct shapes show up across the forest
// without the variant logic in RebuildStaticBatch needing to change.
//
// Compared with the OBJ trees this replaces (612–1010 faces each) the
// new meshes are 12–22 faces — a 50× reduction. Combined with cutting
// the per-cell tree cap from 3 to 2, it should keep dense forests well
// inside GPU budget at zoomed-out camera distances.
func NewLowPolyTreeMesh(variant int) *Mesh {
	type preset struct {
		trunkR, trunkH       float32
		baseR                float32 // lower-cone base radius
		stack                bool    // if true, two stacked cones
		stackTopR, stackTopH float32 // when stack: lower cone is truncated to this radius/height
		apexH                float32 // total height from trunk top to apex
	}
	presets := [3]preset{
		// Variant 0 — small bushy: short trunk, fat single cone.
		{trunkR: 0.18, trunkH: 0.5, baseR: 1.6, stack: false, apexH: 4.5},
		// Variant 1 — tall slender: thinner cone, taller proportions.
		{trunkR: 0.15, trunkH: 0.7, baseR: 1.3, stack: false, apexH: 6.0},
		// Variant 2 — christmas-tree double cone (lower frustum + upper apex).
		{trunkR: 0.18, trunkH: 0.55, baseR: 1.7, stack: true, stackTopR: 0.9, stackTopH: 2.4, apexH: 5.2},
	}
	p := presets[((variant%3)+3)%3]

	var verts []float32
	var indices []uint32

	// emitTri appends one flat-shaded triangle (per-vertex normals all equal
	// to the face normal so each tri can be lit independently — matches the
	// terrain's low-poly look).
	emitTri := func(a, b, c [3]float32) {
		ux := b[0] - a[0]
		uy := b[1] - a[1]
		uz := b[2] - a[2]
		vx := c[0] - a[0]
		vy := c[1] - a[1]
		vz := c[2] - a[2]
		nx := uy*vz - uz*vy
		ny := uz*vx - ux*vz
		nz := ux*vy - uy*vx
		l := float32(math.Sqrt(float64(nx*nx + ny*ny + nz*nz)))
		if l > 0 {
			nx, ny, nz = nx/l, ny/l, nz/l
		}
		base := uint32(len(verts) / 8)
		for _, q := range [][3]float32{a, b, c} {
			verts = append(verts, q[0], q[1], q[2], nx, ny, nz, 0.5, 0.5)
		}
		indices = append(indices, base, base+1, base+2)
	}

	// Hex ring helpers — return 6 points around (0, y) at given radius.
	const sides = 6
	hexRing := func(y, r float32) [sides][3]float32 {
		var ring [sides][3]float32
		for i := 0; i < sides; i++ {
			theta := float64(i) * 2 * math.Pi / float64(sides)
			ring[i] = [3]float32{r * float32(math.Cos(theta)), y, r * float32(math.Sin(theta))}
		}
		return ring
	}

	// --- Trunk: hex prism, sides only (the bottom is on the ground and never seen).
	trunkBase := hexRing(0, p.trunkR)
	trunkTop := hexRing(p.trunkH, p.trunkR)
	for i := 0; i < sides; i++ {
		j := (i + 1) % sides
		emitTri(trunkBase[i], trunkBase[j], trunkTop[j])
		emitTri(trunkBase[i], trunkTop[j], trunkTop[i])
	}

	// --- Foliage cones, anchored on top of the trunk.
	if p.stack {
		// Lower frustum: ring at trunk top → ring at stackTopH/stackTopR.
		fbottom := hexRing(p.trunkH, p.baseR)
		fmid := hexRing(p.trunkH+p.stackTopH, p.stackTopR)
		for i := 0; i < sides; i++ {
			j := (i + 1) % sides
			emitTri(fbottom[i], fbottom[j], fmid[j])
			emitTri(fbottom[i], fmid[j], fmid[i])
		}
		// Upper cone: ring at fmid → apex.
		apex := [3]float32{0, p.trunkH + p.apexH, 0}
		for i := 0; i < sides; i++ {
			j := (i + 1) % sides
			emitTri(fmid[i], fmid[j], apex)
		}
	} else {
		// Single cone: ring at trunk top → apex.
		ring := hexRing(p.trunkH, p.baseR)
		apex := [3]float32{0, p.trunkH + p.apexH, 0}
		for i := 0; i < sides; i++ {
			j := (i + 1) % sides
			emitTri(ring[i], ring[j], apex)
		}
	}

	return NewMesh(verts, indices, []int{3, 3, 2})
}

// NewBoxMesh creates a simple box mesh with the given dimensions and color.
// Used as a fallback when OBJ files are not present.
func NewBoxMesh(w, h, d float32, color [3]float32) *Mesh {
	hw := w / 2
	hd := d / 2
	r, g, b := color[0], color[1], color[2]

	// Each face: 4 vertices with pos(3) + normal(3) + uv(2) = 8 floats.
	// Origin is at the bottom-centre so placement Y is the ground level.
	vertices := []float32{
		// Front face (z = +hd), normal (0, 0, 1)
		-hw, 0, hd, 0, 0, 1, 0, 0,
		hw, 0, hd, 0, 0, 1, 1, 0,
		hw, h, hd, 0, 0, 1, 1, 1,
		-hw, h, hd, 0, 0, 1, 0, 1,
		// Back face (z = -hd), normal (0, 0, -1)
		hw, 0, -hd, 0, 0, -1, 0, 0,
		-hw, 0, -hd, 0, 0, -1, 1, 0,
		-hw, h, -hd, 0, 0, -1, 1, 1,
		hw, h, -hd, 0, 0, -1, 0, 1,
		// Left face (x = -hw), normal (-1, 0, 0)
		-hw, 0, -hd, -1, 0, 0, 0, 0,
		-hw, 0, hd, -1, 0, 0, 1, 0,
		-hw, h, hd, -1, 0, 0, 1, 1,
		-hw, h, -hd, -1, 0, 0, 0, 1,
		// Right face (x = +hw), normal (1, 0, 0)
		hw, 0, hd, 1, 0, 0, 0, 0,
		hw, 0, -hd, 1, 0, 0, 1, 0,
		hw, h, -hd, 1, 0, 0, 1, 1,
		hw, h, hd, 1, 0, 0, 0, 1,
		// Top face (y = h), normal (0, 1, 0)
		-hw, h, hd, 0, 1, 0, 0, 0,
		hw, h, hd, 0, 1, 0, 1, 0,
		hw, h, -hd, 0, 1, 0, 1, 1,
		-hw, h, -hd, 0, 1, 0, 0, 1,
		// Bottom face (y = 0), normal (0, -1, 0)
		-hw, 0, -hd, 0, -1, 0, 0, 0,
		hw, 0, -hd, 0, -1, 0, 1, 0,
		hw, 0, hd, 0, -1, 0, 1, 1,
		-hw, 0, hd, 0, -1, 0, 0, 1,
	}

	// Tint the vertex colors into the UV channels (we'll use white texture * color)
	// Actually the box mesh uses pos+normal+uv layout; color comes from instance tint.
	// The _ variable suppresses unused warning.
	_ = r
	_ = g
	_ = b

	indices := []uint32{
		0, 1, 2, 0, 2, 3, // front
		4, 5, 6, 4, 6, 7, // back
		8, 9, 10, 8, 10, 11, // left
		12, 13, 14, 12, 14, 15, // right
		16, 17, 18, 16, 18, 19, // top
		20, 21, 22, 20, 22, 23, // bottom
	}

	return NewMesh(vertices, indices, []int{3, 3, 2})
}
