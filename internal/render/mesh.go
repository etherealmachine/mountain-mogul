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
	MeshSkier       uint32 = 7 // skier figure; instanced per world.Agent
	MeshLiftStation uint32 = 8
	MeshChair       uint32 = 9
	MeshShed        uint32 = 10
	MeshSnowcat     uint32 = 11
	MeshParkingPad  uint32 = 12 // flat asphalt-coloured pad for a parking lot footprint
	MeshCar         uint32 = 13 // small box used per parked car (dynamic instance per lot)
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
	Layout        []int    // per-vertex attribute sizes in floats
	Locations     []uint32 // optional GL location overrides per layout entry; nil → 0..N-1
}

// VertexColorLoc is the GL attribute location reserved for per-vertex base
// colour (multiplied with per-instance ColorTint in the static/dynamic
// shaders). Pinned to 8 so per-vertex colour can coexist with the static
// batch's per-instance attributes at locations 3-7 and the dynamic batch's
// at 2-4 without colliding.
const VertexColorLoc uint32 = 8

// NewMesh creates a GPU mesh from vertex and index data.
// layout is the vertex attribute sizes in floats, e.g. [3,3,2] for pos/normal/uv.
// locations may be nil (locations default to 0..N-1) or must match len(layout).
func NewMesh(vertices []float32, indices []uint32, layout []int, locations []uint32) *Mesh {
	m := &Mesh{IndexCount: int32(len(indices)), Layout: layout, Locations: locations}

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
		loc := uint32(i)
		if locations != nil {
			loc = locations[i]
		}
		gl.EnableVertexAttribArray(loc)
		gl.VertexAttribPointerWithOffset(loc, int32(size), gl.FLOAT, false, strideBytes, uintptr(offset*4))
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

// NewBoxMesh creates a simple box mesh with the given dimensions. The
// origin is at the bottom-centre so placement Y is the ground level.
// Per-vertex colour isn't part of the layout; callers tint via the
// per-instance ColorTint that the static/dynamic batches multiply on top.
func NewBoxMesh(w, h, d float32) *Mesh {
	hw := w / 2
	hd := d / 2

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

	indices := []uint32{
		0, 1, 2, 0, 2, 3, // front
		4, 5, 6, 4, 6, 7, // back
		8, 9, 10, 8, 10, 11, // left
		12, 13, 14, 12, 14, 15, // right
		16, 17, 18, 16, 18, 19, // top
		20, 21, 22, 20, 22, 23, // bottom
	}

	return NewMesh(vertices, indices, []int{3, 3, 2}, nil)
}
