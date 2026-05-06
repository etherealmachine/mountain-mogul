package render

import (
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
type Mesh struct {
	VAO, VBO, EBO uint32
	IndexCount    int32
}

// NewMesh creates a GPU mesh from vertex and index data.
// layout is the vertex attribute sizes in floats, e.g. [3,3,2] for pos/normal/uv.
func NewMesh(vertices []float32, indices []uint32, layout []int) *Mesh {
	m := &Mesh{IndexCount: int32(len(indices))}

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
