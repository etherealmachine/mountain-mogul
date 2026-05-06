package render

import (
	"fmt"
	"math"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// UIDrawable is something that can be drawn in screen space.
type UIDrawable interface {
	Draw(r *Renderer)
}

// Renderer coordinates all rendering passes.
type Renderer struct {
	TerrainShader *Shader
	StaticShader  *Shader
	DynamicShader *Shader
	UIShader      *Shader
	Camera        *Camera
	Font          *Font // may be nil; gracefully skip text

	terrainMesh   *Mesh
	terrainVBO    uint32
	terrainWidth  int
	terrainHeight int

	staticBatches map[uint32]*Batch
	dynamicBatch  *Batch
	chairBatch    *Batch

	// Per-lift procedural meshes (keyed by lift ID).
	liftTowerMeshes map[uint64]*Mesh
	liftUpCables    map[uint64]*Mesh
	liftDownCables  map[uint64]*Mesh

	ghostBatches   map[uint32]*Batch
	ghostTowerMesh *Mesh
	ghostUpCable   *Mesh
	ghostDownCable *Mesh

	uiVAO, uiVBO uint32
	whiteTexID   uint32 // 1×1 white texture; always bound to unit 0 during UI pass

	brushCenter mgl32.Vec2
	brushRadius float32

	HighlightAgentID   uint64
	TerrainOverlayMode int // 0 = normal, 1 = slope + contour

	// logicalW/H are the window size in logical (point) pixels — these match
	// GLFW mouse coordinates and are used for UI ortho projection and camera
	// ray-casting. frameW/H are the actual OpenGL framebuffer dimensions and
	// are only used for gl.Viewport. On Retina displays frameW = 2×logicalW.
	logicalW, logicalH int
	frameW, frameH     int

	assetDir string
}

// NewRenderer initialises all shaders and GPU resources.
func NewRenderer(w, h int, assetDir string) (*Renderer, error) {
	r := &Renderer{
		logicalW:        w,
		logicalH:        h,
		frameW:          w,
		frameH:          h,
		staticBatches:   make(map[uint32]*Batch),
		liftTowerMeshes: make(map[uint64]*Mesh),
		liftUpCables:    make(map[uint64]*Mesh),
		liftDownCables:  make(map[uint64]*Mesh),
		ghostBatches:    make(map[uint32]*Batch),
		assetDir:        assetDir,
	}

	r.Camera = NewCamera(w, h)

	shaderDir := assetDir + "/shaders/"
	lightingPath := shaderDir + "lighting.glsl"

	var err error
	r.TerrainShader, err = LoadShader(shaderDir+"terrain.vert", shaderDir+"terrain.frag", lightingPath)
	if err != nil {
		return nil, fmt.Errorf("terrain shader: %w", err)
	}

	r.StaticShader, err = LoadShader(shaderDir+"static.vert", shaderDir+"static.frag", lightingPath)
	if err != nil {
		return nil, fmt.Errorf("static shader: %w", err)
	}

	r.DynamicShader, err = LoadShader(shaderDir+"dynamic.vert", shaderDir+"dynamic.frag", lightingPath)
	if err != nil {
		return nil, fmt.Errorf("dynamic shader: %w", err)
	}

	r.UIShader, err = LoadShader(shaderDir+"ui.vert", shaderDir+"ui.frag")
	if err != nil {
		return nil, fmt.Errorf("ui shader: %w", err)
	}

	// White fallback texture — always bound to unit 0 in the UI pass.
	r.whiteTexID = whiteTexture()

	// Font atlas generated from basicfont.Face7x13.
	r.Font = NewFont()

	// Setup UI quad VAO
	gl.GenVertexArrays(1, &r.uiVAO)
	gl.GenBuffers(1, &r.uiVBO)
	gl.BindVertexArray(r.uiVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.uiVBO)
	gl.BufferData(gl.ARRAY_BUFFER, 4*6*4, nil, gl.DYNAMIC_DRAW)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 16, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 16, 8)
	gl.BindVertexArray(0)

	// Load all mesh types
	r.initStaticMeshes()

	gl.Enable(gl.DEPTH_TEST)
	gl.ClearColor(0.635, 0.682, 0.918, 1.0)

	return r, nil
}

func (r *Renderer) initStaticMeshes() {
	modelDir := r.assetDir + "/models/"
	meshDefs := []struct {
		id   uint32
		name string
	}{
		{MeshTree, "tree"},
		{MeshTree2, "tree2"},
		{MeshTree3, "tree3"},
		{MeshRock, "rock"},
		{MeshStump, "stump"},
		{MeshBuilding, "building"},
		{MeshLiftStation, "lift_station"},
	}

	for _, def := range meshDefs {
		objPath := modelDir + def.name + ".obj"
		mesh, texID := LoadOBJ(objPath, def.id)
		r.staticBatches[def.id] = NewStaticBatch(mesh, texID)
	}

	// Agent — dynamic batch.
	agentMesh, agentTexID := LoadOBJ(modelDir+"agent.obj", MeshAgent)
	r.dynamicBatch = NewDynamicBatch(agentMesh, agentTexID)

	// Chair — dynamic batch with procedural mesh.
	chairMesh := NewChairMesh()
	r.chairBatch = NewDynamicBatch(chairMesh, whiteTexture())
}

// SetViewport updates the OpenGL framebuffer viewport. Call this from the
// GLFW framebuffer-size callback. Do not use for UI layout or camera math.
func (r *Renderer) SetViewport(frameW, frameH int) {
	r.frameW = frameW
	r.frameH = frameH
	gl.Viewport(0, 0, int32(frameW), int32(frameH))
}

// SetLogicalSize updates the window's logical (point) dimensions. Call this
// from the GLFW window-size callback. The camera and UI ortho use these
// values so that they stay in sync with GLFW mouse coordinates.
func (r *Renderer) SetLogicalSize(w, h int) {
	r.logicalW = w
	r.logicalH = h
	r.Camera.SetViewport(w, h)
}

// buildTerrainVerts generates vertex and index data for the terrain mesh.
// Vertex layout per vertex: pos(3) + flatNormal(3) + smoothNormal(3) + color(3) = 12 floats.
// flatNormal is the per-triangle face normal (used for flat shading).
// smoothNormal is the averaged normal across neighbouring faces (used for slope viz).
// Diagonals alternate in a checkerboard pattern; each corner gets a small deterministic Y jitter.
// Also emits 4 side walls and a bottom face to form a diorama-style block.
func buildTerrainVerts(t *world.Terrain) ([]float32, []uint32) {
	const cellSize = float32(10.0)
	const skirtBaseY = float32(-50.0)

	numSurface := (t.Width - 1) * (t.Height - 1)
	numSkirt := 2*(t.Width-1) + 2*(t.Height-1) + 1
	verts := make([]float32, 0, (numSurface+numSkirt)*6*12)
	indices := make([]uint32, 0, (numSurface+numSkirt)*6)

	idx := uint32(0)

	// ── Pre-compute jittered elevations ───────────────────────────────────────
	jit := make([]float32, t.Width*t.Height)
	for z := 0; z < t.Height; z++ {
		for x := 0; x < t.Width; x++ {
			jit[x*t.Height+z] = t.ElevationAt(x, z) + terrainJitter(x, z, cellSize)
		}
	}
	jitAt := func(x, z int) float32 { return jit[x*t.Height+z] }

	// ── Smooth normal accumulation ─────────────────────────────────────────────
	// For each grid point, sum the face normals of every triangle that touches it,
	// then normalise. This gives a per-vertex normal that represents the average
	// surface direction across neighbouring triangles.
	smoothAcc := make([][3]float32, t.Width*t.Height)
	addAcc := func(x, z int, n [3]float32) {
		if x >= 0 && x < t.Width && z >= 0 && z < t.Height {
			i := x*t.Height + z
			smoothAcc[i][0] += n[0]
			smoothAcc[i][1] += n[1]
			smoothAcc[i][2] += n[2]
		}
	}
	for z := 0; z < t.Height-1; z++ {
		for x := 0; x < t.Width-1; x++ {
			p00 := [3]float32{float32(x) * cellSize, jitAt(x, z), float32(z) * cellSize}
			p10 := [3]float32{float32(x+1) * cellSize, jitAt(x+1, z), float32(z) * cellSize}
			p01 := [3]float32{float32(x) * cellSize, jitAt(x, z+1), float32(z+1) * cellSize}
			p11 := [3]float32{float32(x+1) * cellSize, jitAt(x+1, z+1), float32(z+1) * cellSize}
			if (x+z)%2 == 0 {
				n1 := upwardNormal(p00, p10, p11)
				n2 := upwardNormal(p00, p11, p01)
				addAcc(x, z, n1); addAcc(x+1, z, n1); addAcc(x+1, z+1, n1)
				addAcc(x, z, n2); addAcc(x+1, z+1, n2); addAcc(x, z+1, n2)
			} else {
				n1 := upwardNormal(p00, p10, p01)
				n2 := upwardNormal(p10, p11, p01)
				addAcc(x, z, n1); addAcc(x+1, z, n1); addAcc(x, z+1, n1)
				addAcc(x+1, z, n2); addAcc(x+1, z+1, n2); addAcc(x, z+1, n2)
			}
		}
	}
	smoothAt := func(x, z int) [3]float32 {
		n := smoothAcc[x*t.Height+z]
		l := float32(math.Sqrt(float64(n[0]*n[0] + n[1]*n[1] + n[2]*n[2])))
		if l < 1e-6 {
			return [3]float32{0, 1, 0}
		}
		return [3]float32{n[0] / l, n[1] / l, n[2] / l}
	}

	// ── Surface ───────────────────────────────────────────────────────────────
	for z := 0; z < t.Height-1; z++ {
		for x := 0; x < t.Width-1; x++ {
			color := terrainColor(t.Cells[x][z])

			p00 := [3]float32{float32(x) * cellSize, jitAt(x, z), float32(z) * cellSize}
			p10 := [3]float32{float32(x+1) * cellSize, jitAt(x+1, z), float32(z) * cellSize}
			p01 := [3]float32{float32(x) * cellSize, jitAt(x, z+1), float32(z+1) * cellSize}
			p11 := [3]float32{float32(x+1) * cellSize, jitAt(x+1, z+1), float32(z+1) * cellSize}

			// Track which grid point each triangle corner maps to for smooth normal lookup.
			var tris [2][3][3]float32
			var corners [2][3][2]int
			if (x+z)%2 == 0 {
				tris[0] = [3][3]float32{p00, p10, p11}
				tris[1] = [3][3]float32{p00, p11, p01}
				corners[0] = [3][2]int{{x, z}, {x + 1, z}, {x + 1, z + 1}}
				corners[1] = [3][2]int{{x, z}, {x + 1, z + 1}, {x, z + 1}}
			} else {
				tris[0] = [3][3]float32{p00, p10, p01}
				tris[1] = [3][3]float32{p10, p11, p01}
				corners[0] = [3][2]int{{x, z}, {x + 1, z}, {x, z + 1}}
				corners[1] = [3][2]int{{x + 1, z}, {x + 1, z + 1}, {x, z + 1}}
			}

			for ti, tri := range tris {
				n := upwardNormal(tri[0], tri[1], tri[2])
				for vi, p := range tri {
					cx, cz := corners[ti][vi][0], corners[ti][vi][1]
					sn := smoothAt(cx, cz)
					verts = append(verts,
						p[0], p[1], p[2],
						n[0], n[1], n[2],
						sn[0], sn[1], sn[2],
						color[0], color[1], color[2],
					)
				}
				indices = append(indices, idx, idx+1, idx+2)
				idx += 3
			}
		}
	}

	// ── Skirt (walls + bottom) ────────────────────────────────────────────────
	wallColor := [3]float32{0.50, 0.40, 0.30}
	upNorm := [3]float32{0, 1, 0} // smooth normal placeholder for skirt faces

	emitTri := func(a, b, c, n, color [3]float32) {
		for _, p := range [][3]float32{a, b, c} {
			verts = append(verts,
				p[0], p[1], p[2],
				n[0], n[1], n[2],
				upNorm[0], upNorm[1], upNorm[2],
				color[0], color[1], color[2],
			)
		}
		indices = append(indices, idx, idx+1, idx+2)
		idx += 3
	}
	emitQuad := func(tl, tr, br, bl, n [3]float32, color [3]float32) {
		emitTri(tl, tr, br, n, color)
		emitTri(tl, br, bl, n, color)
	}

	maxX := float32(t.Width-1) * cellSize
	maxZ := float32(t.Height-1) * cellSize

	// North wall (z=0, normal −Z)
	for x := 0; x < t.Width-1; x++ {
		xL, xR := float32(x)*cellSize, float32(x+1)*cellSize
		yL := t.ElevationAt(x, 0) + terrainJitter(x, 0, cellSize)
		yR := t.ElevationAt(x+1, 0) + terrainJitter(x+1, 0, cellSize)
		emitQuad(
			[3]float32{xL, yL, 0}, [3]float32{xR, yR, 0},
			[3]float32{xR, skirtBaseY, 0}, [3]float32{xL, skirtBaseY, 0},
			[3]float32{0, 0, -1}, wallColor,
		)
	}

	// South wall (z=maxZ, normal +Z)
	for x := 0; x < t.Width-1; x++ {
		xL, xR := float32(x)*cellSize, float32(x+1)*cellSize
		yL := t.ElevationAt(x, t.Height-1) + terrainJitter(x, t.Height-1, cellSize)
		yR := t.ElevationAt(x+1, t.Height-1) + terrainJitter(x+1, t.Height-1, cellSize)
		emitQuad(
			[3]float32{xR, yR, maxZ}, [3]float32{xL, yL, maxZ},
			[3]float32{xL, skirtBaseY, maxZ}, [3]float32{xR, skirtBaseY, maxZ},
			[3]float32{0, 0, 1}, wallColor,
		)
	}

	// West wall (x=0, normal −X)
	for z := 0; z < t.Height-1; z++ {
		zN, zS := float32(z)*cellSize, float32(z+1)*cellSize
		yN := t.ElevationAt(0, z) + terrainJitter(0, z, cellSize)
		yS := t.ElevationAt(0, z+1) + terrainJitter(0, z+1, cellSize)
		emitQuad(
			[3]float32{0, yS, zS}, [3]float32{0, yN, zN},
			[3]float32{0, skirtBaseY, zN}, [3]float32{0, skirtBaseY, zS},
			[3]float32{-1, 0, 0}, wallColor,
		)
	}

	// East wall (x=maxX, normal +X)
	for z := 0; z < t.Height-1; z++ {
		zN, zS := float32(z)*cellSize, float32(z+1)*cellSize
		yN := t.ElevationAt(t.Width-1, z) + terrainJitter(t.Width-1, z, cellSize)
		yS := t.ElevationAt(t.Width-1, z+1) + terrainJitter(t.Width-1, z+1, cellSize)
		emitQuad(
			[3]float32{maxX, yN, zN}, [3]float32{maxX, yS, zS},
			[3]float32{maxX, skirtBaseY, zS}, [3]float32{maxX, skirtBaseY, zN},
			[3]float32{1, 0, 0}, wallColor,
		)
	}

	// Bottom face (normal −Y)
	emitQuad(
		[3]float32{0, skirtBaseY, 0}, [3]float32{maxX, skirtBaseY, 0},
		[3]float32{maxX, skirtBaseY, maxZ}, [3]float32{0, skirtBaseY, maxZ},
		[3]float32{0, -1, 0}, wallColor,
	)

	return verts, indices
}

// terrainJitter returns a small deterministic elevation offset for a grid corner.
func terrainJitter(gx, gz int, cellSize float32) float32 {
	h := uint32(gx)*2654435761 ^ uint32(gz)*2246822519
	h ^= h >> 16
	h *= 0x45d9f3b
	h ^= h >> 16
	return (float32(h)/float32(^uint32(0)) - 0.5) * 0.2 * cellSize
}

// visualElevationAt returns the exact terrain mesh surface height at world
// position (wx, wz). It replicates the same triangle selection and barycentric
// interpolation used in buildTerrainVerts — including per-vertex jitter and the
// checkerboard diagonal pattern — so agents always sit on (never below) the mesh.
func visualElevationAt(t *world.Terrain, wx, wz float32) float32 {
	const cellSize = float32(10.0)
	xi := int(wx / cellSize)
	zi := int(wz / cellSize)
	if xi < 0 {
		xi = 0
	}
	if xi >= t.Width-1 {
		xi = t.Width - 2
	}
	if zi < 0 {
		zi = 0
	}
	if zi >= t.Height-1 {
		zi = t.Height - 2
	}
	fx := wx/cellSize - float32(xi)
	fz := wz/cellSize - float32(zi)
	if fx < 0 {
		fx = 0
	}
	if fx > 1 {
		fx = 1
	}
	if fz < 0 {
		fz = 0
	}
	if fz > 1 {
		fz = 1
	}

	e00 := t.ElevationAt(xi, zi) + terrainJitter(xi, zi, cellSize)
	e10 := t.ElevationAt(xi+1, zi) + terrainJitter(xi+1, zi, cellSize)
	e01 := t.ElevationAt(xi, zi+1) + terrainJitter(xi, zi+1, cellSize)
	e11 := t.ElevationAt(xi+1, zi+1) + terrainJitter(xi+1, zi+1, cellSize)

	// Mirror the checkerboard diagonal from buildTerrainVerts.
	if (xi+zi)%2 == 0 {
		// Diagonal runs from p00→p11.
		// Lower-left triangle (p00, p10, p11): fz ≤ fx
		// Upper-right triangle (p00, p11, p01): fz > fx
		if fz <= fx {
			return (1-fx)*e00 + (fx-fz)*e10 + fz*e11
		}
		return (1-fz)*e00 + fx*e11 + (fz-fx)*e01
	}
	// Diagonal runs from p10→p01.
	// Lower-right triangle (p00, p10, p01): fx+fz ≤ 1
	// Upper-left  triangle (p10, p11, p01): fx+fz > 1
	if fx+fz <= 1 {
		return (1-fx-fz)*e00 + fx*e10 + fz*e01
	}
	return (1-fz)*e10 + (fx+fz-1)*e11 + (1-fx)*e01
}

// upwardNormal computes the face normal for a triangle and ensures it points upward.
func upwardNormal(a, b, c [3]float32) [3]float32 {
	n := computeNormal(a, b, c)
	if n[1] < 0 {
		n[0], n[1], n[2] = -n[0], -n[1], -n[2]
	}
	return n
}

// BuildTerrainMesh creates the terrain mesh from the given Terrain.
// Vertex layout: pos(3) + flatNormal(3) + smoothNormal(3) + color(3) = 12 floats per vertex.
func (r *Renderer) BuildTerrainMesh(t *world.Terrain) {
	r.terrainWidth = t.Width
	r.terrainHeight = t.Height

	verts, indices := buildTerrainVerts(t)

	if r.terrainMesh != nil {
		r.terrainMesh.Delete()
	}

	var vao, vbo, ebo uint32
	gl.GenVertexArrays(1, &vao)
	gl.GenBuffers(1, &vbo)
	gl.GenBuffers(1, &ebo)

	gl.BindVertexArray(vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.DYNAMIC_DRAW)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

	stride := int32(12 * 4)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)  // aPos
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, stride, 12) // aNormal (flat)
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointerWithOffset(2, 3, gl.FLOAT, false, stride, 24) // aSmoothNormal
	gl.EnableVertexAttribArray(3)
	gl.VertexAttribPointerWithOffset(3, 3, gl.FLOAT, false, stride, 36) // aColor
	gl.BindVertexArray(0)

	r.terrainVBO = vbo
	r.terrainMesh = &Mesh{
		VAO:        vao,
		VBO:        vbo,
		EBO:        ebo,
		IndexCount: int32(len(indices)),
	}
}

// FlushTerrainVerts regenerates vertex data from the terrain and uploads it to
// the existing VBO. Use this after sculpting individual cells instead of
// doing a full BuildTerrainMesh (which allocates new GL objects).
func (r *Renderer) FlushTerrainVerts(t *world.Terrain) {
	if r.terrainMesh == nil {
		return
	}
	verts, _ := buildTerrainVerts(t)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.terrainVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.DYNAMIC_DRAW)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
}

func terrainColor(cell world.Cell) [3]float32 {
	if cell.SnowDepth > 0.5 {
		if cell.Groomed {
			return [3]float32{0.85, 0.92, 0.98} // groomed: light blue-white
		}
		return [3]float32{0.95, 0.97, 1.0} // snow white
	}
	return [3]float32{0.4, 0.55, 0.3} // bare ground
}

// RebuildStaticBatch rebuilds all static instance buffers from world state.
func (r *Renderer) RebuildStaticBatch(w *world.World) {
	// Clear all static batches
	for _, b := range r.staticBatches {
		b.ClearStatic()
	}

	const cellSize = float32(10.0)

	// Forest layer — derive tree instances from terrain cell TreeDensity.
	for z := 0; z < w.Terrain.Height; z++ {
		for x := 0; x < w.Terrain.Width; x++ {
			density := w.Terrain.Cells[x][z].TreeDensity
			count := treeCountFromDensity(density)
			if count == 0 {
				continue
			}
			elev := w.Terrain.ElevationAt(x, z)
			for i := 0; i < count; i++ {
				h := treeInstanceHash(x, z, i)
				offsetX := (float32(h&0xFF)/127.5 - 1.0) * 3.5
				offsetZ := (float32((h>>8)&0xFF)/127.5 - 1.0) * 3.5
				rotation := float32((h>>16)&0xFFFF) / 65535.0 * 2 * math.Pi
				scale := 0.85 + float32((h>>32)&0xFF)/255.0*0.30
				variant := MeshTree + uint32((h>>40)%3)

				wx := float32(x)*cellSize + offsetX
				wz := float32(z)*cellSize + offsetZ
				transform := mgl32.Translate3D(wx, elev, wz).
					Mul4(mgl32.HomogRotate3DY(rotation)).
					Mul4(mgl32.Scale3D(scale, scale, scale))

				if batch, ok := r.staticBatches[variant]; ok {
					batch.AddStatic(transform, mgl32.Vec3{1, 1, 1})
				}
			}
		}
	}

	// Decorative placed objects (rocks, stumps, lone hand-placed trees).
	for _, obj := range w.Objects {
		batchID := obj.Type.MeshID()
		if obj.Type == world.ObjTree {
			batchID = MeshTree + uint32(obj.ID%3)
		}
		batch, ok := r.staticBatches[batchID]
		if !ok {
			continue
		}
		x := float32(obj.Pos[0]) * cellSize
		z := float32(obj.Pos[1]) * cellSize
		y := w.Terrain.ElevationAt(obj.Pos[0], obj.Pos[1])
		t := mgl32.Translate3D(x, y, z).Mul4(mgl32.HomogRotate3DY(obj.Rotation))
		batch.AddStatic(t, mgl32.Vec3{1, 1, 1})
	}

	// Buildings
	for _, bldg := range w.Buildings {
		batch, ok := r.staticBatches[MeshBuilding]
		if !ok {
			continue
		}
		x := float32(bldg.Pos[0]) * cellSize
		z := float32(bldg.Pos[1]) * cellSize
		y := w.Terrain.ElevationAt(bldg.Pos[0], bldg.Pos[1])
		t := mgl32.Translate3D(x, y, z).Mul4(mgl32.HomogRotate3DY(bldg.Rotation))
		batch.AddStatic(t, mgl32.Vec3{1, 1, 1})
	}

	// Lift stations (base and top of each lift)
	if stationBatch, ok := r.staticBatches[MeshLiftStation]; ok {
		for _, lift := range w.Lifts {
			for _, cell := range [][2]int{lift.Base, lift.Top} {
				x := float32(cell[0]) * cellSize
				z := float32(cell[1]) * cellSize
				y := w.Terrain.ElevationAt(cell[0], cell[1])
				t := mgl32.Translate3D(x, y, z)
				stationBatch.AddStatic(t, mgl32.Vec3{1, 1, 1})
			}
		}
	}
}

// ComputeLiftTowerInstances returns StaticInstances for towers evenly spaced
// along the path from base to top at terrain elevation.
// Retained for ghost preview during lift placement.
func ComputeLiftTowerInstances(base, top [2]int, t *world.Terrain) []StaticInstance {
	const cellSize = float32(10.0)
	const towerSpacing = float32(50.0)

	bx := float32(base[0]) * cellSize
	bz := float32(base[1]) * cellSize
	tx := float32(top[0]) * cellSize
	tz := float32(top[1]) * cellSize

	dx := tx - bx
	dz := tz - bz
	length := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if length < 1 {
		return nil
	}

	steps := int(length / towerSpacing)
	if steps < 1 {
		steps = 1
	}

	instances := make([]StaticInstance, 0, steps+1)
	for i := 0; i <= steps; i++ {
		frac := float32(i) / float32(steps)
		wx := bx + dx*frac
		wz := bz + dz*frac
		gx := int(wx / cellSize)
		gz := int(wz / cellSize)
		wy := t.ElevationAt(gx, gz)

		m := mgl32.Translate3D(wx, wy, wz)
		inst := StaticInstance{ColorTint: [3]float32{0.7, 0.7, 0.7}}
		copy(inst.Transform[:], m[:])
		instances = append(instances, inst)
	}
	return instances
}

// GenerateTowerMesh creates a world-space mesh of T-shaped towers for a lift.
// Vertex layout: pos(3) + normal(3) + uv(2) — drawn with the static shader via
// identity instance transform (setCableTransformAttribs).
func GenerateTowerMesh(lift *world.Lift, t *world.Terrain) *Mesh {
	const cellSize = float32(10.0)
	const towerSpacing = float32(50.0)
	const poleHalf = float32(0.35)
	const towerH = world.TowerHeight
	const barHalf = world.CrossbarHalf
	const barThick = float32(0.3)

	bx := float32(lift.Base[0]) * cellSize
	bz := float32(lift.Base[1]) * cellSize
	tx := float32(lift.Top[0]) * cellSize
	tz := float32(lift.Top[1]) * cellSize
	dx := tx - bx
	dz := tz - bz
	length := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if length < 1 {
		length = 1
	}
	dirX := dx / length
	dirZ := dz / length
	perpX := -dirZ
	perpZ := dirX

	steps := int(length / towerSpacing)
	if steps < 1 {
		steps = 1
	}

	var verts []float32
	var indices []uint32

	emitBox := func(cx, cy, cz, halfFwd, halfH, halfPerp float32) {
		// fwd axis = cable direction (dirX, dirZ), perp = (perpX, perpZ)
		// Emit 6 faces of an oriented box centered at (cx, cy+halfH, cz).
		type faceData struct {
			corners [4][3]float32
			normal  [3]float32
		}

		hw := halfFwd
		hh := halfH
		hp := halfPerp

		// World-space corners relative to center base (cx, cy, cz):
		// fwd axis: (dirX, 0, dirZ), up: (0,1,0), perp: (perpX, 0, perpZ)
		c000 := [3]float32{cx - hw*dirX - hp*perpX, cy, cz - hw*dirZ - hp*perpZ}
		c100 := [3]float32{cx + hw*dirX - hp*perpX, cy, cz + hw*dirZ - hp*perpZ}
		c010 := [3]float32{cx - hw*dirX - hp*perpX, cy + 2*hh, cz - hw*dirZ - hp*perpZ}
		c110 := [3]float32{cx + hw*dirX - hp*perpX, cy + 2*hh, cz + hw*dirZ - hp*perpZ}
		c001 := [3]float32{cx - hw*dirX + hp*perpX, cy, cz - hw*dirZ + hp*perpZ}
		c101 := [3]float32{cx + hw*dirX + hp*perpX, cy, cz + hw*dirZ + hp*perpZ}
		c011 := [3]float32{cx - hw*dirX + hp*perpX, cy + 2*hh, cz - hw*dirZ + hp*perpZ}
		c111 := [3]float32{cx + hw*dirX + hp*perpX, cy + 2*hh, cz + hw*dirZ + hp*perpZ}

		faces := []faceData{
			{[4][3]float32{c000, c100, c110, c010}, [3]float32{-perpX, 0, -perpZ}}, // -perp face
			{[4][3]float32{c101, c001, c011, c111}, [3]float32{perpX, 0, perpZ}},   // +perp face
			{[4][3]float32{c001, c000, c010, c011}, [3]float32{-dirX, 0, -dirZ}},   // -fwd face
			{[4][3]float32{c100, c101, c111, c110}, [3]float32{dirX, 0, dirZ}},     // +fwd face
			{[4][3]float32{c010, c110, c111, c011}, [3]float32{0, 1, 0}},           // top
			{[4][3]float32{c001, c101, c100, c000}, [3]float32{0, -1, 0}},          // bottom
		}
		for _, f := range faces {
			base := uint32(len(verts) / 8)
			for _, p := range f.corners {
				verts = append(verts, p[0], p[1], p[2], f.normal[0], f.normal[1], f.normal[2], 0, 0)
			}
			indices = append(indices, base, base+1, base+2, base, base+2, base+3)
		}
	}

	for i := 0; i <= steps; i++ {
		frac := float32(i) / float32(steps)
		wx := bx + dx*frac
		wz := bz + dz*frac
		gx := int(wx / cellSize)
		gz := int(wz / cellSize)
		wy := t.ElevationAt(gx, gz)

		// Vertical pole centered on the cable line at this point.
		emitBox(wx, wy, wz, poleHalf, towerH/2, poleHalf)
		// Horizontal T crossbar at the top of the pole, perpendicular to cable.
		crossY := wy + towerH - barThick
		emitBox(wx, crossY, wz, barThick/2, barThick/2, barHalf)
	}

	return NewMesh(verts, indices, []int{3, 3, 2})
}

// generateCableMesh creates a quad strip for one cable at the given lateral offset.
// perpOff > 0 = up cable side, perpOff < 0 = down cable side.
func generateCableMesh(lift *world.Lift, t *world.Terrain, perpOff float32) *Mesh {
	const cellSize = float32(10.0)
	const cableWidth = float32(0.15)
	const steps = 30

	bx := float32(lift.Base[0]) * cellSize
	bz := float32(lift.Base[1]) * cellSize
	tx := float32(lift.Top[0]) * cellSize
	tz := float32(lift.Top[1]) * cellSize
	dx := tx - bx
	dz := tz - bz
	length := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if length < 1 {
		length = 1
	}
	perpX := -dz / length
	perpZ := dx / length

	verts := make([]float32, 0, (steps+1)*2*8)
	indices := make([]uint32, 0, steps*6)

	for i := 0; i <= steps; i++ {
		frac := float32(i) / float32(steps)
		cx := bx + dx*frac + perpX*perpOff
		cz := bz + dz*frac + perpZ*perpOff
		cy := t.InterpolatedElevationAt(cx, cz) + world.CableHeight

		for side := -1; side <= 1; side += 2 {
			s := float32(side) * cableWidth * 0.5
			verts = append(verts,
				cx+perpX*s, cy, cz+perpZ*s,
				0, 1, 0,
				frac, float32((side+1)/2),
			)
		}
		if i < steps {
			base := uint32(i * 2)
			indices = append(indices, base, base+1, base+2, base+1, base+3, base+2)
		}
	}
	return NewMesh(verts, indices, []int{3, 3, 2})
}

// DrawWorld renders the full 3D world.
func (r *Renderer) DrawWorld(w *world.World, time float32) {
	gl.ClearColor(0.635, 0.682, 0.918, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	vp := r.Camera.ViewProj()

	// Terrain pass
	if r.terrainMesh != nil {
		r.TerrainShader.Use()
		r.TerrainShader.SetMat4("uViewProj", vp)
		r.TerrainShader.SetVec2("uBrushCenter", r.brushCenter)
		r.TerrainShader.SetFloat("uBrushRadius", r.brushRadius)
		r.TerrainShader.SetInt("uOverlayMode", r.TerrainOverlayMode)
		r.terrainMesh.Draw()
	}

	// Static pass
	r.StaticShader.Use()
	r.StaticShader.SetMat4("uViewProj", vp)
	r.StaticShader.SetInt("uTexture", 0)
	r.StaticShader.SetFloat("uAlpha", 1.0)
	gl.ActiveTexture(gl.TEXTURE0)
	for _, batch := range r.staticBatches {
		batch.Draw()
	}

	// Tower + cable pass — world-space meshes, drawn with identity instance transform.
	gl.BindTexture(gl.TEXTURE_2D, r.whiteTexID)
	setCableTransformAttribs()
	for _, m := range r.liftTowerMeshes {
		m.Draw()
	}
	for _, m := range r.liftUpCables {
		m.Draw()
	}
	for _, m := range r.liftDownCables {
		m.Draw()
	}

	// Ghost pass — translucent preview of in-progress placements.
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.DepthMask(false)
	r.StaticShader.SetFloat("uAlpha", 0.4)
	for _, batch := range r.ghostBatches {
		batch.Draw()
	}
	gl.BindTexture(gl.TEXTURE_2D, r.whiteTexID)
	setCableTransformAttribs()
	if r.ghostTowerMesh != nil {
		r.ghostTowerMesh.Draw()
	}
	if r.ghostUpCable != nil {
		r.ghostUpCable.Draw()
	}
	if r.ghostDownCable != nil {
		r.ghostDownCable.Draw()
	}
	r.StaticShader.SetFloat("uAlpha", 1.0)
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)

	// Dynamic pass (agents)
	r.DynamicShader.Use()
	r.DynamicShader.SetMat4("uViewProj", vp)
	r.DynamicShader.SetFloat("uTime", time)

	if r.dynamicBatch != nil {
		instances := make([]DynamicInstance, 0, len(w.Agents))
		for _, agent := range w.Agents {
			posY := agent.Pos[1]
			if agent.State != world.StateRiding {
				posY = visualElevationAt(w.Terrain, agent.Pos[0], agent.Pos[2])
			}
			color := agentColor(agent.State)
			if r.HighlightAgentID != 0 && agent.ID == r.HighlightAgentID {
				color = [3]float32{1.0, 0.95, 0.1}
			}
			instances = append(instances, DynamicInstance{
				Position: [3]float32{agent.Pos[0], posY, agent.Pos[2]},
				Heading:  agent.Heading,
				Color:    color,
			})
		}
		r.dynamicBatch.SetDynamic(instances)
		r.dynamicBatch.Draw()
	}

	// Chair pass
	if r.chairBatch != nil {
		chairInstances := make([]DynamicInstance, 0)
		for _, lift := range w.Lifts {
			for _, chair := range lift.Chairs {
				pos, heading := lift.ChairPos(chair.Progress, w.Terrain)
				// Chair color: grey when empty, blue-tint when carrying passengers.
				hasPax := chair.Passengers[0] != nil || chair.Passengers[1] != nil
				color := [3]float32{0.7, 0.7, 0.7}
				if hasPax {
					color = [3]float32{0.55, 0.65, 0.85}
				}
				chairInstances = append(chairInstances, DynamicInstance{
					Position: [3]float32{pos[0], pos[1], pos[2]},
					Heading:  heading,
					Color:    color,
				})
			}
		}
		r.chairBatch.SetDynamic(chairInstances)
		r.chairBatch.Draw()
	}
}

func agentColor(state world.AgentState) [3]float32 {
	switch state {
	case world.StateWalking:
		return [3]float32{0.2, 0.6, 0.9}
	case world.StateQueuing:
		return [3]float32{0.9, 0.7, 0.2}
	case world.StateRiding:
		return [3]float32{0.9, 0.4, 0.1}
	case world.StateSkiing:
		return [3]float32{0.1, 0.8, 0.3}
	case world.StateReturningToLodge:
		return [3]float32{0.8, 0.3, 0.8}
	}
	return [3]float32{1, 1, 1}
}

// DrawUI renders screen-space UI elements.
func (r *Renderer) DrawUI(elements []UIDrawable) {
	gl.Disable(gl.DEPTH_TEST)
	defer gl.Enable(gl.DEPTH_TEST)

	proj := mgl32.Ortho(0, float32(r.logicalW), float32(r.logicalH), 0, -1, 1)

	r.UIShader.Use()
	r.UIShader.SetMat4("uProjection", proj)
	r.UIShader.SetInt("uTexture", 0)

	// Always bind a valid texture to unit 0 — macOS validates the sampler
	// even when uUseTexture is false, and warns if the unit is empty.
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, r.whiteTexID)

	for _, e := range elements {
		e.Draw(r)
	}
}

// drawRect draws a filled rectangle at screen coordinates using the UI shader.
// The UI shader must already be bound with uProjection set.
func (r *Renderer) drawRect(x, y, w, h float32) {
	// vertices: pos(2) + uv(2)
	verts := []float32{
		x, y, 0, 0,
		x + w, y, 1, 0,
		x + w, y + h, 1, 1,
		x, y, 0, 0,
		x + w, y + h, 1, 1,
		x, y + h, 0, 1,
	}

	gl.BindVertexArray(r.uiVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.uiVBO)
	gl.BufferSubData(gl.ARRAY_BUFFER, 0, len(verts)*4, gl.Ptr(verts))
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
	gl.BindVertexArray(0)
}

// drawRectUV draws a quad with explicit UV coordinates (used by font rendering).
func (r *Renderer) drawRectUV(x, y, w, h, u0, v0, u1, v1 float32) {
	verts := []float32{
		x, y, u0, v0,
		x + w, y, u1, v0,
		x + w, y + h, u1, v1,
		x, y, u0, v0,
		x + w, y + h, u1, v1,
		x, y + h, u0, v1,
	}
	gl.BindVertexArray(r.uiVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.uiVBO)
	gl.BufferSubData(gl.ARRAY_BUFFER, 0, len(verts)*4, gl.Ptr(verts))
	gl.DrawArrays(gl.TRIANGLES, 0, 6)
	gl.BindVertexArray(0)
}

// DrawTexturedRect draws a textured rectangle.
func (r *Renderer) DrawTexturedRect(x, y, w, h float32, texID uint32, color mgl32.Vec4) {
	r.UIShader.Use()
	r.UIShader.SetInt("uUseTexture", 1)
	r.UIShader.SetVec4("uColor", color)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, texID)
	r.drawRect(x, y, w, h)
}

// DrawColorRect draws a solid color rectangle.
func (r *Renderer) DrawColorRect(x, y, w, h float32, color mgl32.Vec4) {
	r.UIShader.SetInt("uUseTexture", 0)
	r.UIShader.SetVec4("uColor", color)
	r.drawRect(x, y, w, h)
}

// SetBrush configures the terrain shader brush ring.
func (r *Renderer) SetBrush(center mgl32.Vec2, radius float32) {
	r.brushCenter = center
	r.brushRadius = radius
}

// ClearBrush disables the terrain shader brush ring.
func (r *Renderer) ClearBrush() {
	r.brushRadius = 0
}

// SetGhosts replaces the ghost instances for one mesh type.
// The ghost batch is lazily created from the corresponding static batch's mesh and texture.
func (r *Renderer) SetGhosts(meshID uint32, instances []StaticInstance) {
	if _, ok := r.ghostBatches[meshID]; !ok {
		sb, ok := r.staticBatches[meshID]
		if !ok {
			return
		}
		r.ghostBatches[meshID] = NewStaticBatch(sb.mesh, sb.textureID)
	}
	r.ghostBatches[meshID].SetStaticInstances(instances)
}

// ClearAllGhosts zeros all ghost batch instance lists.
func (r *Renderer) ClearAllGhosts() {
	for _, b := range r.ghostBatches {
		b.SetStaticInstances(nil)
	}
}

// SetGhostCable regenerates the ghost tower + cable meshes from base to top.
func (r *Renderer) SetGhostCable(base, top [2]int, t *world.Terrain) {
	if r.ghostTowerMesh != nil {
		r.ghostTowerMesh.Delete()
	}
	if r.ghostUpCable != nil {
		r.ghostUpCable.Delete()
	}
	if r.ghostDownCable != nil {
		r.ghostDownCable.Delete()
	}
	tempLift := &world.Lift{Base: base, Top: top}
	r.ghostTowerMesh = GenerateTowerMesh(tempLift, t)
	r.ghostUpCable = generateCableMesh(tempLift, t, world.CableGap)
	r.ghostDownCable = generateCableMesh(tempLift, t, -world.CableGap)
}

// ClearGhostCable removes the ghost tower + cable meshes.
func (r *Renderer) ClearGhostCable() {
	if r.ghostTowerMesh != nil {
		r.ghostTowerMesh.Delete()
		r.ghostTowerMesh = nil
	}
	if r.ghostUpCable != nil {
		r.ghostUpCable.Delete()
		r.ghostUpCable = nil
	}
	if r.ghostDownCable != nil {
		r.ghostDownCable.Delete()
		r.ghostDownCable = nil
	}
}

// AddLiftMeshes generates and stores tower + cable meshes for a lift.
func (r *Renderer) AddLiftMeshes(lift *world.Lift, t *world.Terrain) {
	r.liftTowerMeshes[lift.ID] = GenerateTowerMesh(lift, t)
	r.liftUpCables[lift.ID] = generateCableMesh(lift, t, world.CableGap)
	r.liftDownCables[lift.ID] = generateCableMesh(lift, t, -world.CableGap)
}

// RemoveLiftMeshes removes all procedural meshes for a lift.
func (r *Renderer) RemoveLiftMeshes(liftID uint64) {
	if m, ok := r.liftTowerMeshes[liftID]; ok {
		m.Delete()
		delete(r.liftTowerMeshes, liftID)
	}
	if m, ok := r.liftUpCables[liftID]; ok {
		m.Delete()
		delete(r.liftUpCables, liftID)
	}
	if m, ok := r.liftDownCables[liftID]; ok {
		m.Delete()
		delete(r.liftDownCables, liftID)
	}
}

// AddLiftCable is kept for call-site compatibility; delegates to AddLiftMeshes.
func (r *Renderer) AddLiftCable(lift *world.Lift, t *world.Terrain) {
	r.AddLiftMeshes(lift, t)
}

// RemoveLiftCable is kept for call-site compatibility; delegates to RemoveLiftMeshes.
func (r *Renderer) RemoveLiftCable(liftID uint64) {
	r.RemoveLiftMeshes(liftID)
}

// ScreenWidth returns the window's logical width in points (matches mouse coords).
func (r *Renderer) ScreenWidth() int { return r.logicalW }

// ScreenHeight returns the window's logical height in points (matches mouse coords).
func (r *Renderer) ScreenHeight() int { return r.logicalH }

// setCableTransformAttribs sets the generic vertex attributes for locations 3-7 to an
// identity transform with white tint. Cable meshes have no instance VBO at those locations,
// so OpenGL falls back to these global values — making the cable render in world space.
func setCableTransformAttribs() {
	gl.VertexAttrib4f(3, 1, 0, 0, 0) // identity col 0
	gl.VertexAttrib4f(4, 0, 1, 0, 0) // identity col 1
	gl.VertexAttrib4f(5, 0, 0, 1, 0) // identity col 2
	gl.VertexAttrib4f(6, 0, 0, 0, 1) // identity col 3
	gl.VertexAttrib3f(7, 0.15, 0.15, 0.15) // dark charcoal tint
}

// treeCountFromDensity maps a cell's TreeDensity to the number of tree instances to place.
func treeCountFromDensity(density float32) int {
	switch {
	case density < 0.2:
		return 0
	case density < 0.5:
		return 1
	case density < 0.8:
		return 2
	default:
		return 3
	}
}

// treeInstanceHash returns a stable 64-bit hash for deriving per-tree visual properties.
// Uses the same style as terrainJitter to keep hashing consistent across the package.
func treeInstanceHash(x, z, i int) uint64 {
	h := uint64(uint32(x)*2654435761 ^ uint32(z)*2246822519 ^ uint32(i)*2692343)
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	return h
}
