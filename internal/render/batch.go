package render

import (
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

// StaticInstance holds per-instance data for static objects.
// mat4 transform + color tint, 16-byte aligned.
type StaticInstance struct {
	Transform [16]float32 // mat4 row-major
	ColorTint [3]float32
	Pad       float32
}

// DynamicInstance holds per-instance data for dynamic objects (agents,
// chairs, snowcats, cars). Animate gates the limb-bob in dynamic.vert
// — set it to 1.0 for figures whose geometry should breathe (skiers),
// 0.0 for vehicles and rigid props.
type DynamicInstance struct {
	Position [3]float32
	Heading  float32
	Color    [3]float32
	Animate  float32
}

// batchType distinguishes static vs dynamic batch layout.
type batchType int

const (
	batchStatic  batchType = iota
	batchDynamic batchType = iota
)

// Batch manages instanced rendering for a mesh type.
//
// Each Batch owns its own VAO so that two batches sharing the same Mesh
// (e.g. a static batch and a ghost-preview batch of the same mesh ID) don't
// fight over the mesh VAO's instance-attribute slots. The per-vertex attribs
// are wired against mesh.VBO; the per-instance attribs against b.instanceVBO.
type Batch struct {
	mesh        *Mesh
	textureID   uint32
	vao         uint32
	instanceVBO uint32
	btype       batchType

	staticInstances  []StaticInstance
	dynamicInstances []DynamicInstance
	dirty            bool
}

// bindVertexLayout wires the mesh's vertex attributes (locations 0..len(layout)-1)
// onto the currently-bound VAO using mesh.VBO. The element buffer is also bound
// so DrawElementsInstanced can index from it.
func bindVertexLayout(mesh *Mesh) {
	gl.BindBuffer(gl.ARRAY_BUFFER, mesh.VBO)
	stride := 0
	for _, s := range mesh.Layout {
		stride += s
	}
	strideBytes := int32(stride * 4)
	offset := 0
	for i, size := range mesh.Layout {
		loc := uint32(i)
		if mesh.Locations != nil {
			loc = mesh.Locations[i]
		}
		gl.EnableVertexAttribArray(loc)
		gl.VertexAttribPointerWithOffset(loc, int32(size), gl.FLOAT, false, strideBytes, uintptr(offset*4))
		offset += size
	}
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, mesh.EBO)
}

// NewStaticBatch creates an instanced batch for static objects.
// Instance data: mat4 at locations 3-6, ColorTint at location 7.
func NewStaticBatch(mesh *Mesh, texID uint32) *Batch {
	b := &Batch{
		mesh:      mesh,
		textureID: texID,
		btype:     batchStatic,
		dirty:     true,
	}

	gl.GenVertexArrays(1, &b.vao)
	gl.GenBuffers(1, &b.instanceVBO)

	gl.BindVertexArray(b.vao)
	bindVertexLayout(mesh)
	gl.BindBuffer(gl.ARRAY_BUFFER, b.instanceVBO)

	stride := int32(unsafe.Sizeof(StaticInstance{}))
	// mat4 occupies 4 vec4 attributes at locations 3-6
	for i := uint32(0); i < 4; i++ {
		loc := uint32(3) + i
		gl.EnableVertexAttribArray(loc)
		gl.VertexAttribPointerWithOffset(loc, 4, gl.FLOAT, false, stride, uintptr(i*16))
		gl.VertexAttribDivisor(loc, 1)
	}
	// ColorTint at location 7 (offset 64 bytes past start of Transform)
	gl.EnableVertexAttribArray(7)
	gl.VertexAttribPointerWithOffset(7, 3, gl.FLOAT, false, stride, uintptr(64))
	gl.VertexAttribDivisor(7, 1)

	gl.BindVertexArray(0)

	return b
}

// NewDynamicBatch creates an instanced batch for dynamic agents.
// Position at location 2, Heading at 3, Color at 4.
func NewDynamicBatch(mesh *Mesh, texID uint32) *Batch {
	b := &Batch{
		mesh:      mesh,
		textureID: texID,
		btype:     batchDynamic,
		dirty:     true,
	}

	gl.GenVertexArrays(1, &b.vao)
	gl.GenBuffers(1, &b.instanceVBO)

	gl.BindVertexArray(b.vao)
	bindVertexLayout(mesh)
	gl.BindBuffer(gl.ARRAY_BUFFER, b.instanceVBO)

	stride := int32(unsafe.Sizeof(DynamicInstance{}))

	// Position at location 2
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointerWithOffset(2, 3, gl.FLOAT, false, stride, 0)
	gl.VertexAttribDivisor(2, 1)

	// Heading at location 3
	gl.EnableVertexAttribArray(3)
	gl.VertexAttribPointerWithOffset(3, 1, gl.FLOAT, false, stride, 12)
	gl.VertexAttribDivisor(3, 1)

	// Color at location 4
	gl.EnableVertexAttribArray(4)
	gl.VertexAttribPointerWithOffset(4, 3, gl.FLOAT, false, stride, 16)
	gl.VertexAttribDivisor(4, 1)

	// Animate flag at location 5 (1.0 = bob limbs, 0.0 = rigid).
	gl.EnableVertexAttribArray(5)
	gl.VertexAttribPointerWithOffset(5, 1, gl.FLOAT, false, stride, 28)
	gl.VertexAttribDivisor(5, 1)

	gl.BindVertexArray(0)

	return b
}

// Delete releases the batch's VAO and instance VBO. The mesh and texture are
// owned externally and are not deleted here.
func (b *Batch) Delete() {
	if b.instanceVBO != 0 {
		gl.DeleteBuffers(1, &b.instanceVBO)
		b.instanceVBO = 0
	}
	if b.vao != 0 {
		gl.DeleteVertexArrays(1, &b.vao)
		b.vao = 0
	}
}

// AddStatic appends a static instance and marks the batch dirty.
func (b *Batch) AddStatic(t mgl32.Mat4, tint mgl32.Vec3) {
	inst := StaticInstance{ColorTint: [3]float32{tint[0], tint[1], tint[2]}}
	copy(inst.Transform[:], t[:])
	b.staticInstances = append(b.staticInstances, inst)
	b.dirty = true
}

// ClearStatic removes all static instances.
func (b *Batch) ClearStatic() {
	b.staticInstances = b.staticInstances[:0]
	b.dirty = true
}

// SetStaticInstances replaces all static instances and marks the batch dirty.
func (b *Batch) SetStaticInstances(instances []StaticInstance) {
	b.staticInstances = instances
	b.dirty = true
}

// SetDynamic replaces all dynamic instances each frame.
func (b *Batch) SetDynamic(instances []DynamicInstance) {
	b.dynamicInstances = instances
	b.dirty = true
}

// Draw uploads instance data if dirty and calls glDrawElementsInstanced.
func (b *Batch) Draw() {
	var count int32
	if b.btype == batchStatic {
		count = int32(len(b.staticInstances))
	} else {
		count = int32(len(b.dynamicInstances))
	}
	if count == 0 {
		return
	}

	gl.BindBuffer(gl.ARRAY_BUFFER, b.instanceVBO)
	if b.dirty {
		if b.btype == batchStatic {
			size := int(unsafe.Sizeof(StaticInstance{})) * len(b.staticInstances)
			gl.BufferData(gl.ARRAY_BUFFER, size, gl.Ptr(b.staticInstances), gl.DYNAMIC_DRAW)
		} else {
			size := int(unsafe.Sizeof(DynamicInstance{})) * len(b.dynamicInstances)
			gl.BufferData(gl.ARRAY_BUFFER, size, gl.Ptr(b.dynamicInstances), gl.STREAM_DRAW)
		}
		b.dirty = false
	}

	gl.BindTexture(gl.TEXTURE_2D, b.textureID)
	gl.BindVertexArray(b.vao)
	gl.DrawElementsInstanced(gl.TRIANGLES, b.mesh.IndexCount, gl.UNSIGNED_INT, nil, count)
	gl.BindVertexArray(0)
}
