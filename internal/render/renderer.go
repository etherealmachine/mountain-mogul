package render

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// UIDrawable is something that can be drawn in screen space.
type UIDrawable interface {
	Draw(r *Renderer)
}

// DebugLine is a single world-space line segment for tuning overlays.
type DebugLine struct {
	A, B  mgl32.Vec3
	Color [3]float32
}

// Renderer coordinates all rendering passes.
type Renderer struct {
	TerrainShader *Shader
	StaticShader  *Shader
	DynamicShader *Shader
	UIShader      *Shader
	DebugShader   *Shader
	Camera        *Camera
	Font          *Font // may be nil; gracefully skip text

	// scene owns all GPU state coupled to the current World. Replaced wholesale
	// by ResetSceneState on every scene transition.
	scene *SceneResources

	staticBatches map[uint32]*Batch
	dynamicBatch  *Batch
	chairBatch    *Batch

	uiVAO, uiVBO uint32
	whiteTexID   uint32 // 1×1 white texture; always bound to unit 0 during UI pass

	// Terrain triplanar samplers — snow projected onto the top (Y) plane,
	// rock onto the side (X, Z) planes. Each material has diffuse, normal
	// (OpenGL convention), and roughness maps. Loaded once at construction;
	// the terrain shader binds them on every terrain draw.
	snowDiffID, snowNormID, snowRoughID uint32
	rockDiffID, rockNormID, rockRoughID uint32

	debugVAO, debugVBO uint32
	debugVertCount     int32 // number of debug vertices currently in the VBO

	brushCenter mgl32.Vec2
	brushRadius float32

	// Perception-cone highlight (followed-skier diagnostic). When
	// perceptionRadius > 0 the static shader tints any instance whose
	// origin falls inside the fan toward warm yellow. Mirrors the brush
	// uniform pattern.
	perceptionOrigin       mgl32.Vec3
	perceptionForwardXZ    mgl32.Vec2 // unit vector (sin(h), cos(h))
	perceptionCosHalfAngle float32
	perceptionRadius       float32 // 0 disables

	HighlightAgentID   uint64
	TerrainOverlayMode int // 0 = normal, 1 = slope + contour

	// icons holds GL texture IDs for the UI icon set under assets/icons/.
	// Populated by LoadIcons() at renderer construction.
	icons map[IconName]uint32

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
		logicalW:      w,
		logicalH:      h,
		frameW:        w,
		frameH:        h,
		scene:         newSceneResources(),
		staticBatches: make(map[uint32]*Batch),
		assetDir:      assetDir,
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

	r.DebugShader, err = LoadShader(shaderDir+"debug.vert", shaderDir+"debug.frag")
	if err != nil {
		return nil, fmt.Errorf("debug shader: %w", err)
	}

	// White fallback texture — always bound to unit 0 in the UI pass.
	r.whiteTexID = whiteTexture()

	// Terrain triplanar textures (Poly Haven, CC0). Failures fall back
	// to a 1×1 white texture so terrain still renders.
	texDir := assetDir + "/textures/"
	r.snowDiffID, _ = LoadTexture(texDir + "snow_02_diff_2k.jpg")
	r.snowNormID, _ = LoadTexture(texDir + "snow_02_nor_gl_2k.jpg")
	r.snowRoughID, _ = LoadTexture(texDir + "snow_02_rough_2k.jpg")
	r.rockDiffID, _ = LoadTexture(texDir + "rock_face_03_diff_2k.jpg")
	r.rockNormID, _ = LoadTexture(texDir + "rock_face_03_nor_gl_2k.jpg")
	r.rockRoughID, _ = LoadTexture(texDir + "rock_face_03_rough_2k.jpg")

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

	// Debug-line VAO/VBO. Vertex layout: pos(3) + color(3) = 6 floats = 24 bytes.
	gl.GenVertexArrays(1, &r.debugVAO)
	gl.GenBuffers(1, &r.debugVBO)
	gl.BindVertexArray(r.debugVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.debugVBO)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, 24, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, 24, 12)
	gl.BindVertexArray(0)

	// Load all mesh types
	r.initStaticMeshes()

	// Load UI icons from assets/icons/ — best-effort, missing files fall
	// back to a 1×1 white texture so the UI still renders.
	r.LoadIcons()

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
		{MeshTower, "tower"},
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

// VisualElevationAt returns the exact terrain mesh surface height at world
// position (wx, wz). It replicates the same triangle selection and barycentric
// interpolation used in buildTerrainVerts — including per-vertex jitter and the
// checkerboard diagonal pattern — so agents always sit on (never below) the mesh.
func VisualElevationAt(t *world.Terrain, wx, wz float32) float32 {
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
	r.scene.terrainWidth = t.Width
	r.scene.terrainHeight = t.Height

	verts, indices := buildTerrainVerts(t)

	if r.scene.terrainMesh != nil {
		r.scene.terrainMesh.Delete()
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

	r.scene.terrainVBO = vbo
	r.scene.terrainMesh = &Mesh{
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
	if r.scene.terrainMesh == nil {
		return
	}
	verts, _ := buildTerrainVerts(t)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.scene.terrainVBO)
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
		x := (float32(obj.Pos[0]) + 0.5) * cellSize
		z := (float32(obj.Pos[1]) + 0.5) * cellSize
		y := w.Terrain.ElevationAt(obj.Pos[0], obj.Pos[1])
		t := mgl32.Translate3D(x, y, z).Mul4(mgl32.HomogRotate3DY(obj.Rotation))
		batch.AddStatic(t, mgl32.Vec3{1, 1, 1})
	}

	// Buildings
	if buildingBatch, ok := r.staticBatches[MeshBuilding]; ok {
		for _, bldg := range w.Buildings {
			buildingBatch.AddStatic(BuildingTransform(bldg.Pos, bldg.Rotation, w.Terrain), mgl32.Vec3{1, 1, 1})
		}
	}

	// Lift stations — both ends of each lift, oriented so the model's +X
	// axis (cable-exit side) points toward the other end. Bullwheel ends
	// up on the outboard side at both base and top.
	if stationBatch, ok := r.staticBatches[MeshLiftStation]; ok {
		for _, lift := range w.Lifts {
			stationBatch.AddStatic(LiftStationTransform(lift.Base, lift.Top, w.Terrain), mgl32.Vec3{1, 1, 1})
			stationBatch.AddStatic(LiftStationTransform(lift.Top, lift.Base, w.Terrain), mgl32.Vec3{1, 1, 1})
		}
	}

	// Towers — between (not at) the stations. Endpoints are skipped so
	// they don't sit atop the bullwheel beams.
	if towerBatch, ok := r.staticBatches[MeshTower]; ok {
		for _, lift := range w.Lifts {
			for _, m := range TowerInstancesForLift(lift.Base, lift.Top, w.Terrain) {
				towerBatch.AddStatic(m, mgl32.Vec3{1, 1, 1})
			}
		}
	}
}

// BuildingTransform builds the world-space transform for a building
// placed at world XZ pos with the given Y rotation. Used by both live
// placement (RebuildStaticBatch) and ghost preview.
func BuildingTransform(pos mgl32.Vec2, rotation float32, terrain *world.Terrain) mgl32.Mat4 {
	y := VisualElevationAt(terrain, pos[0], pos[1])
	return mgl32.Translate3D(pos[0], y, pos[1]).Mul4(mgl32.HomogRotate3DY(rotation))
}

// LiftStationTransform builds the world-space transform for a lift station
// at `pos`, rotated so the model's +X axis points toward `otherEnd` (the
// far end of the cable). Pass `otherEnd == pos` when the other end is
// not yet known (e.g. the place-base ghost preview); the station then
// renders with no rotation applied.
//
// The convention matches the SCAD model in models-src/lift_station.scad:
// +X = cable-exit side, -X = bullwheel side. So at the base station +X
// points up the lift line, and at the top station +X points back down.
func LiftStationTransform(pos, otherEnd mgl32.Vec2, terrain *world.Terrain) mgl32.Mat4 {
	y := VisualElevationAt(terrain, pos[0], pos[1])

	var rot float32
	if otherEnd != pos {
		dx := otherEnd[0] - pos[0]
		dz := otherEnd[1] - pos[1]
		// HomogRotate3DY(θ) takes (1,0,0) → (cos θ, 0, -sin θ); we want
		// that result to be the unit cable direction (dx, dz)/len. So
		// θ = atan2(-dz, dx). Length normalisation falls out of atan2.
		rot = float32(math.Atan2(-float64(dz), float64(dx)))
	}

	return mgl32.Translate3D(pos[0], y, pos[1]).Mul4(mgl32.HomogRotate3DY(rot))
}

// TowerInstancesForLift returns world transforms for the lift's tower
// instances. Endpoints (where the stations sit) are skipped — towers
// start world.StationOffset metres inboard from each station, then
// space at roughly towerSpacing along the cable. Returns nil when the
// lift is shorter than 2× StationOffset (no room for towers between
// stations); the cable then stays at BullwheelHeight throughout.
//
// Each transform rotates the tower so its model-space +X (the cable
// axis convention from models-src/tower.scad) aligns with the cable
// direction in world space.
func TowerInstancesForLift(base, top mgl32.Vec2, t *world.Terrain) []mgl32.Mat4 {
	const towerSpacing = float32(50.0)
	stationOffset := float32(world.StationOffset)

	bx, bz := base[0], base[1]
	tx, tz := top[0], top[1]
	dx := tx - bx
	dz := tz - bz
	length := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if length <= 2*stationOffset {
		return nil
	}

	innerLen := length - 2*stationOffset
	intervals := int(innerLen / towerSpacing)
	if intervals < 1 {
		intervals = 1
	}
	spacing := innerLen / float32(intervals)

	dirX := dx / length
	dirZ := dz / length
	// HomogRotate3DY(θ) takes (1,0,0) → (cos θ, 0, -sin θ); we want that
	// to equal the cable direction (dirX, 0, dirZ), so θ = atan2(-dz, dx).
	rot := float32(math.Atan2(-float64(dirZ), float64(dirX)))

	out := make([]mgl32.Mat4, 0, intervals+1)
	for i := 0; i <= intervals; i++ {
		d := stationOffset + float32(i)*spacing
		wx := bx + dirX*d
		wz := bz + dirZ*d
		wy := VisualElevationAt(t, wx, wz)

		m := mgl32.Translate3D(wx, wy, wz).Mul4(mgl32.HomogRotate3DY(rot))
		out = append(out, m)
	}
	return out
}

// generateCableMesh creates a quad strip for one cable at the given lateral offset.
// perpOff > 0 = up cable side, perpOff < 0 = down cable side.
func generateCableMesh(lift *world.Lift, t *world.Terrain, perpOff float32) *Mesh {
	const cableWidth = float32(0.15)
	// Step count scales with length (one segment per ~5 m) so the ramps
	// at each station get enough vertices to read as a smooth slope on
	// long lifts. Floor of 30 keeps short lifts from looking polygonal.
	const minSteps = 30

	bx, bz := lift.Base[0], lift.Base[1]
	tx, tz := lift.Top[0], lift.Top[1]
	dx := tx - bx
	dz := tz - bz
	length := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if length < 1 {
		length = 1
	}
	steps := int(length / 5)
	if steps < minSteps {
		steps = minSteps
	}
	perpX := -dz / length
	perpZ := dx / length

	verts := make([]float32, 0, (steps+1)*2*8)
	indices := make([]uint32, 0, steps*6)

	for i := 0; i <= steps; i++ {
		frac := float32(i) / float32(steps)
		cx := bx + dx*frac + perpX*perpOff
		cz := bz + dz*frac + perpZ*perpOff
		cy := t.InterpolatedElevationAt(cx, cz) + world.CableHeightAt(frac, length)

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
	if r.scene.terrainMesh != nil {
		r.TerrainShader.Use()
		r.TerrainShader.SetMat4("uViewProj", vp)
		r.TerrainShader.SetVec2("uBrushCenter", r.brushCenter)
		r.TerrainShader.SetFloat("uBrushRadius", r.brushRadius)
		r.TerrainShader.SetInt("uOverlayMode", r.TerrainOverlayMode)
		r.TerrainShader.SetVec3("uEyeDir", r.Camera.EyeDirection())
		r.TerrainShader.SetInt("uSnowDiff", 0)
		r.TerrainShader.SetInt("uSnowNorm", 1)
		r.TerrainShader.SetInt("uSnowRough", 2)
		r.TerrainShader.SetInt("uRockDiff", 3)
		r.TerrainShader.SetInt("uRockNorm", 4)
		r.TerrainShader.SetInt("uRockRough", 5)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, r.snowDiffID)
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, r.snowNormID)
		gl.ActiveTexture(gl.TEXTURE2)
		gl.BindTexture(gl.TEXTURE_2D, r.snowRoughID)
		gl.ActiveTexture(gl.TEXTURE3)
		gl.BindTexture(gl.TEXTURE_2D, r.rockDiffID)
		gl.ActiveTexture(gl.TEXTURE4)
		gl.BindTexture(gl.TEXTURE_2D, r.rockNormID)
		gl.ActiveTexture(gl.TEXTURE5)
		gl.BindTexture(gl.TEXTURE_2D, r.rockRoughID)
		r.scene.terrainMesh.Draw()
		gl.ActiveTexture(gl.TEXTURE0) // leave unit 0 active for downstream passes
	}

	// Static pass
	r.StaticShader.Use()
	r.StaticShader.SetMat4("uViewProj", vp)
	r.StaticShader.SetInt("uTexture", 0)
	r.StaticShader.SetFloat("uAlpha", 1.0)
	r.StaticShader.SetVec3("uPerceptionOrigin", r.perceptionOrigin)
	r.StaticShader.SetVec2("uPerceptionForwardXZ", r.perceptionForwardXZ)
	r.StaticShader.SetFloat("uPerceptionCosHalfAngle", r.perceptionCosHalfAngle)
	r.StaticShader.SetFloat("uPerceptionRadius", r.perceptionRadius)
	gl.ActiveTexture(gl.TEXTURE0)
	for _, batch := range r.staticBatches {
		batch.Draw()
	}

	// Cable pass — world-space meshes, drawn with identity instance
	// transform. Towers are instanced through the static batch above.
	gl.BindTexture(gl.TEXTURE_2D, r.whiteTexID)
	setCableTransformAttribs()
	for _, m := range r.scene.liftUpCables {
		m.Draw()
	}
	for _, m := range r.scene.liftDownCables {
		m.Draw()
	}

	// Ghost pass — translucent preview of in-progress placements.
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.DepthMask(false)
	r.StaticShader.SetFloat("uAlpha", 0.4)
	for _, batch := range r.scene.ghostBatches {
		batch.Draw()
	}
	gl.BindTexture(gl.TEXTURE_2D, r.whiteTexID)
	setCableTransformAttribs()
	if r.scene.ghostUpCable != nil {
		r.scene.ghostUpCable.Draw()
	}
	if r.scene.ghostDownCable != nil {
		r.scene.ghostDownCable.Draw()
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
			if agent.OnLiftID == 0 {
				posY = VisualElevationAt(w.Terrain, agent.Pos[0], agent.Pos[2])
			}
			color := agentColor(w, agent)
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

	// Debug-line pass (steering overlay etc.) — runs before chairs so chairs draw over.
	if r.debugVertCount > 0 && r.DebugShader != nil {
		r.DebugShader.Use()
		r.DebugShader.SetMat4("uViewProj", vp)
		gl.BindVertexArray(r.debugVAO)
		gl.LineWidth(2.5)
		gl.DrawArrays(gl.LINES, 0, r.debugVertCount)
		gl.BindVertexArray(0)
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

func agentColor(w *world.World, a *world.Agent) [3]float32 {
	switch world.Activity(w, a) {
	case "Walking":
		return [3]float32{0.2, 0.6, 0.9}
	case "Queuing":
		return [3]float32{0.9, 0.7, 0.2}
	case "On Lift":
		return [3]float32{0.9, 0.4, 0.1}
	case "To Lift":
		return [3]float32{0.1, 0.8, 0.3}
	case "To Lodge":
		return [3]float32{0.8, 0.3, 0.8}
	case "Fallen":
		return [3]float32{0.8, 0.1, 0.1}
	}
	return [3]float32{1, 1, 1}
}

// DrawUI renders screen-space UI elements.
func (r *Renderer) DrawUI(elements []UIDrawable) {
	gl.Disable(gl.DEPTH_TEST)
	defer gl.Enable(gl.DEPTH_TEST)

	// Alpha blending is required so font atlas transparent pixels don't
	// overwrite the background with black.
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	defer gl.Disable(gl.BLEND)

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

// SetDebugLines uploads a fresh batch of debug line segments for the
// next frame. Pass nil or an empty slice to clear.
func (r *Renderer) SetDebugLines(lines []DebugLine) {
	if len(lines) == 0 {
		r.debugVertCount = 0
		return
	}
	verts := make([]float32, 0, len(lines)*12)
	for _, l := range lines {
		verts = append(verts,
			l.A[0], l.A[1], l.A[2], l.Color[0], l.Color[1], l.Color[2],
			l.B[0], l.B[1], l.B[2], l.Color[0], l.Color[1], l.Color[2],
		)
	}
	gl.BindBuffer(gl.ARRAY_BUFFER, r.debugVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.DYNAMIC_DRAW)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	r.debugVertCount = int32(len(lines) * 2)
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

// SetPerceptionCone configures the static-shader perception fan used to
// highlight trees the followed skier currently perceives. forward is a unit
// XZ vector; cosHalfAngle is precomputed so the shader test is a single dot
// product. radius == 0 disables the highlight.
func (r *Renderer) SetPerceptionCone(origin mgl32.Vec3, forwardXZ mgl32.Vec2, cosHalfAngle, radius float32) {
	r.perceptionOrigin = origin
	r.perceptionForwardXZ = forwardXZ
	r.perceptionCosHalfAngle = cosHalfAngle
	r.perceptionRadius = radius
}

// ClearPerceptionCone disables the perception-cone highlight.
func (r *Renderer) ClearPerceptionCone() {
	r.perceptionRadius = 0
}

// SetGhosts replaces the ghost instances for one mesh type.
// The ghost batch is lazily created from the corresponding static batch's mesh and texture.
func (r *Renderer) SetGhosts(meshID uint32, instances []StaticInstance) {
	if _, ok := r.scene.ghostBatches[meshID]; !ok {
		sb, ok := r.staticBatches[meshID]
		if !ok {
			return
		}
		r.scene.ghostBatches[meshID] = NewStaticBatch(sb.mesh, sb.textureID)
	}
	r.scene.ghostBatches[meshID].SetStaticInstances(instances)
}

// ClearAllGhosts zeros all ghost batch instance lists.
func (r *Renderer) ClearAllGhosts() {
	for _, b := range r.scene.ghostBatches {
		b.SetStaticInstances(nil)
	}
}

// SetGhostCable regenerates the ghost cable meshes and tower-instance
// previews from base to top. The tower ghosts go through the standard
// ghost-batch path so they share geometry with the live towers.
func (r *Renderer) SetGhostCable(base, top mgl32.Vec2, t *world.Terrain) {
	if r.scene.ghostUpCable != nil {
		r.scene.ghostUpCable.Delete()
	}
	if r.scene.ghostDownCable != nil {
		r.scene.ghostDownCable.Delete()
	}
	tempLift := &world.Lift{Base: base, Top: top}
	r.scene.ghostUpCable = generateCableMesh(tempLift, t, world.CableGap)
	r.scene.ghostDownCable = generateCableMesh(tempLift, t, -world.CableGap)

	mats := TowerInstancesForLift(base, top, t)
	ghosts := make([]StaticInstance, len(mats))
	for i, m := range mats {
		ghosts[i].ColorTint = [3]float32{1, 1, 1}
		copy(ghosts[i].Transform[:], m[:])
	}
	r.SetGhosts(MeshTower, ghosts)
}

// ClearGhostCable removes the ghost cable meshes and tower previews.
func (r *Renderer) ClearGhostCable() {
	if r.scene.ghostUpCable != nil {
		r.scene.ghostUpCable.Delete()
		r.scene.ghostUpCable = nil
	}
	if r.scene.ghostDownCable != nil {
		r.scene.ghostDownCable.Delete()
		r.scene.ghostDownCable = nil
	}
	r.SetGhosts(MeshTower, nil)
}

// AddLiftMeshes generates and stores cable meshes for a lift. Towers are
// added to the static batch by RebuildStaticBatch.
func (r *Renderer) AddLiftMeshes(lift *world.Lift, t *world.Terrain) {
	r.scene.liftUpCables[lift.ID] = generateCableMesh(lift, t, world.CableGap)
	r.scene.liftDownCables[lift.ID] = generateCableMesh(lift, t, -world.CableGap)
}

// RemoveLiftMeshes removes the cable meshes for a lift.
func (r *Renderer) RemoveLiftMeshes(liftID uint64) {
	if m, ok := r.scene.liftUpCables[liftID]; ok {
		m.Delete()
		delete(r.scene.liftUpCables, liftID)
	}
	if m, ok := r.scene.liftDownCables[liftID]; ok {
		m.Delete()
		delete(r.scene.liftDownCables, liftID)
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

// ResetSceneState releases every GPU resource and UI flag tied to the current
// World, returning the renderer to a clean slate. Call this on every scene
// transition (entering a new scenario, leaving one) so resources from a
// previous World can't bleed into the next.
//
// Engine-scoped resources (shaders, fonts, the agent/chair batches, UI/debug
// VAOs) are preserved.
func (r *Renderer) ResetSceneState() {
	if r.scene != nil {
		r.scene.Delete()
	}
	r.scene = newSceneResources()

	// Engine-owned static-batch shells survive scene transitions, but their
	// per-world instance lists must be cleared so trees/buildings/lifts from
	// the previous World don't keep drawing.
	for _, b := range r.staticBatches {
		b.ClearStatic()
	}

	r.brushCenter = mgl32.Vec2{}
	r.brushRadius = 0
	r.perceptionOrigin = mgl32.Vec3{}
	r.perceptionForwardXZ = mgl32.Vec2{}
	r.perceptionCosHalfAngle = 0
	r.perceptionRadius = 0
	r.HighlightAgentID = 0
	r.TerrainOverlayMode = 0
	r.debugVertCount = 0
}

// ScreenWidth returns the window's logical width in points (matches mouse coords).
func (r *Renderer) ScreenWidth() int { return r.logicalW }

// ScreenHeight returns the window's logical height in points (matches mouse coords).
func (r *Renderer) ScreenHeight() int { return r.logicalH }

// SaveScreenshot reads the default framebuffer with glReadPixels and writes it
// as a PNG to path. Must be called after rendering and before SwapBuffers (so
// the back buffer still holds the freshly drawn frame). Creates parent dirs as
// needed.
func (r *Renderer) SaveScreenshot(path string) error {
	w, h := r.frameW, r.frameH
	if w <= 0 || h <= 0 {
		return fmt.Errorf("screenshot: invalid framebuffer size %dx%d", w, h)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	buf := make([]byte, w*h*4)
	gl.PixelStorei(gl.PACK_ALIGNMENT, 1)
	gl.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(buf))

	// glReadPixels origin is bottom-left; PNG origin is top-left. Flip rows.
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	stride := w * 4
	for y := 0; y < h; y++ {
		src := buf[(h-1-y)*stride : (h-y)*stride]
		copy(img.Pix[y*img.Stride:y*img.Stride+stride], src)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

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
