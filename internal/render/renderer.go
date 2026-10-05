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

// Terrain-overlay bits. Each bit toggles one overlay drawn on top of the
// base terrain shading; bits are independent so any combination can stack.
// Kept in sync with the bitand checks in assets/shaders/terrain.frag.
const (
	OverlayContour       = 1 << 0
	OverlaySlope         = 1 << 1
	OverlaySnowDepth     = 1 << 2
	OverlayGrooming      = 1 << 3
	OverlayAvalancheRisk = 1 << 4
	OverlayMoguls        = 1 << 6
	OverlayBumpNormal    = 1 << 7  // debug: render the perturbed shading normal as RGB
	OverlaySurfaceDetail = 1 << 8  // debug: render the surface-detail RGBA texture directly
	OverlayTrails        = 1 << 9  // show painted trail areas as semi-opaque colour patches
	OverlayParcels       = 1 << 10 // editor: show parcel tints and price labels (CPU-side, not in shader)
)

// DebugLine is a single world-space line segment for tuning overlays.
type DebugLine struct {
	A, B  mgl32.Vec3
	Color [3]float32
}

// partBatch pairs a DynamicBatch for an animated sub-part with its
// declaration (pivot offset, spin axis) and spin rate in rad/s.
type partBatch struct {
	batch    *Batch
	decl     PartDecl
	spinRate float32
}

// Renderer coordinates all rendering passes.
type Renderer struct {
	TerrainShader *Shader
	StaticShader  *Shader
	DynamicShader *Shader
	UIShader      *Shader
	DebugShader   *Shader
	WeatherShader *Shader
	Camera        *Camera
	Font          *Font // may be nil; gracefully skip text

	// scene owns all GPU state coupled to the current World. Replaced wholesale
	// by ResetSceneState on every scene transition.
	scene *SceneResources

	staticBatches map[uint32]*Batch
	// cutawayLodgeID is the lodge drawn roofless with low walls so its
	// floor plan shows while it's being edited; 0 for none.
	cutawayLodgeID        uint64
	dynamicBatch          *Batch // skier mesh (SkisOn == true)
	walkerBatch           *Batch // walker mesh (SkisOn == false)
	chairBatch            *Batch
	chairQuadBatch        *Batch
	chair6PackBatch       *Batch
	gondolaBatch          *Batch
	snowcatBatch          *Batch
	catLensBatch          *Batch // emissive headlight / work-light lenses
	catBeaconBatch        *Batch // emissive amber roof beacon
	terrainShadow         terrainShadow
	shadowMap             shadowMap
	patrollerBatch        *Batch // ski patrol snowmobiles (red snowcat mesh when active)
	carBatch              *Batch
	helicopterBodyBatch   *Batch      // rigid fuselage, tail, skids
	helicopterPartBatches []partBatch // spinning rotor parts (loaded from # part metadata)

	uiVAO, uiVBO     uint32
	whiteTexID       uint32 // 1×1 white texture; always bound to unit 0 during UI pass
	transparentTexID uint32 // 1×1 all-zero RGBA; fallback for uCellOverlay when no overlay is set

	// uiVerts is the per-frame UI quad accumulator. Every UI primitive
	// (text glyphs, coloured rects, textured rects, discs) appends 6 verts
	// here; flushUI uploads the slice and emits one DrawArrays. Vertex
	// layout: pos.xy + uv.xy + color.rgba = 8 floats = 32 bytes.
	//
	// Pre-batching, every glyph cost one BufferSubData + one DrawArrays
	// CGo call. A typical F5 inspector frame went through ~450 GL calls
	// just for text; the profile showed gl.BufferSubData at 48 % of
	// total CPU. Batching collapses all UI in a frame into a small
	// number of draws (one per texture-binding change).
	uiVerts    []float32
	uiBoundTex uint32 // current texture on unit 0 for the active batch

	debugVAO, debugVBO uint32
	debugVertCount     int32 // number of debug vertices currently in the VBO

	catPathVAO, catPathVBO uint32
	catPathVertCount       int32

	boundaryVAO, boundaryVBO uint32
	boundaryVertCount        int32 // fence rope lines around owned parcels

	fencePostVAO, fencePostVBO uint32
	fencePostVertCount         int32 // fence post triangles (square vertical posts)

	weatherVAO uint32 // empty VAO for the full-screen-triangle weather pass

	// WeatherOverlay controls the precipitation/sky overlay drawn after the
	// 3D passes. Values mirror sim.WeatherState: 0=clear, 1=overcast,
	// 2=lightSnow, 3=heavySnow, 4=rain. Set by the scene each frame before
	// calling DrawWorld; defaults to 0 (no overlay).
	WeatherOverlay int

	// CanopySnow is how much snow sits on tree branches, 0..1. Set by the
	// scene each frame (see CanopySnowFor); 0 in the editor.
	CanopySnow float32

	// Lighting is the sun, fill and sky for this frame, set by the scene
	// from the sim clock before DrawWorld. The zero value draws with
	// DefaultLighting (fixed midday).
	Lighting Lighting

	brushCenter mgl32.Vec2
	brushRadius float32

	// rangeCenter/rangeRadius drive the snow-gun range ring drawn by the
	// terrain shader. Set via SetRange before DrawWorld; cleared after.
	rangeCenter mgl32.Vec2
	rangeRadius float32

	// Perception-cone highlight (followed-skier diagnostic). When
	// perceptionRadius > 0 the static shader tints any instance whose
	// origin falls inside the fan toward warm yellow. Mirrors the brush
	// uniform pattern.
	perceptionOrigin       mgl32.Vec3
	perceptionForwardXZ    mgl32.Vec2 // unit vector (sin(h), cos(h))
	perceptionCosHalfAngle float32
	perceptionRadius       float32 // 0 disables

	HighlightGuestID uint64
	HighlightCatID   uint64
	HiddenGuestID    uint64     // skip this agent in the dynamic pass (used by first-person camera)
	HiddenGuestPos   mgl32.Vec3 // anchor for HiddenRadius proximity culling
	HiddenRadius     float32    // when >0, also skip agents within this XZ radius of HiddenGuestPos
	// TerrainOverlayMode is a bitmask of view overlays applied to the
	// terrain mesh. Each enabled bit alpha-blends its overlay onto the
	// base shading, so several can stack at once. Bits, in order:
	//
	//	0  contour lines
	//	1  slope debug
	//	2  snow depth heatmap
	//	3  grooming heatmap
	//	4  packed snow heatmap
	//	5  ice heatmap
	//	6  mogul size heatmap
	//
	// 0 means no overlays. The same int is uploaded to the terrain
	// fragment shader as uOverlayMode and decoded with bitand there.
	TerrainOverlayMode int

	// icons holds GL texture IDs for the UI icon set under assets/icons/.
	// Populated by LoadIcons() at renderer construction.
	icons map[IconName]uint32

	// logicalW/H are the window size in logical (point) pixels — these match
	// GLFW mouse coordinates and are used for UI ortho projection and camera
	// ray-casting. frameW/H are the actual OpenGL framebuffer dimensions and
	// are only used for gl.Viewport. On Retina displays frameW = 2×logicalW.
	logicalW, logicalH int
	frameW, frameH     int

	// terrainChunksDrawn is how many terrain chunks the last frame drew.
	terrainChunksDrawn int

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
	r.TerrainShader, err = LoadShaderTess(shaderDir+"terrain.vert", shaderDir+"terrain.tesc", shaderDir+"terrain.tese", shaderDir+"terrain.frag", lightingPath)
	if err != nil {
		return nil, fmt.Errorf("terrain shader: %w", err)
	}

	r.StaticShader, err = LoadShader(shaderDir+"static.vert", shaderDir+"static.frag", lightingPath)
	if err != nil {
		return nil, fmt.Errorf("static shader: %w", err)
	}

	r.shadowMap.init(shaderDir)

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

	r.WeatherShader, err = LoadShader(shaderDir+"weather.vert", shaderDir+"weather.frag")
	if err != nil {
		return nil, fmt.Errorf("weather shader: %w", err)
	}
	// Empty VAO required by OpenGL 4.1 core even when gl_VertexID supplies
	// all geometry — no attributes are enabled on this array object.
	gl.GenVertexArrays(1, &r.weatherVAO)

	// White fallback texture — always bound to unit 0 in the UI pass.
	r.whiteTexID = whiteTexture()
	// Transparent fallback — bound to the cell-overlay unit when no overlay is set.
	r.transparentTexID = transparentTexture()

	// Font — try Barlow Condensed Medium TTF first, fall back to the built-in
	// basicfont.Face7x13 bitmap if the file is missing or unparseable.
	const ttfPointSize = 20.0
	if f, err := LoadTTFFont(assetDir+"/fonts/BarlowCondensed-Medium.ttf", ttfPointSize); err == nil {
		r.Font = f
	} else {
		r.Font = NewFont()
	}

	// Setup UI quad VAO. Vertex layout: pos.xy (8 B) + uv.xy (8 B) +
	// color.rgba (16 B) = 32 B per vertex. The initial buffer size is a
	// hint; flushUI calls glBufferData each frame to reallocate as the
	// per-frame batch grows.
	gl.GenVertexArrays(1, &r.uiVAO)
	gl.GenBuffers(1, &r.uiVBO)
	gl.BindVertexArray(r.uiVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.uiVBO)
	gl.BufferData(gl.ARRAY_BUFFER, 32*6, nil, gl.DYNAMIC_DRAW)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 32, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 32, 8)
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointerWithOffset(2, 4, gl.FLOAT, false, 32, 16)
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

	// Cat-path line VAO/VBO — same vertex layout, drawn thicker.
	gl.GenVertexArrays(1, &r.catPathVAO)
	gl.GenBuffers(1, &r.catPathVBO)
	gl.BindVertexArray(r.catPathVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.catPathVBO)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, 24, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, 24, 12)
	gl.BindVertexArray(0)

	// Boundary fence rope VAO/VBO — same vertex layout as debug, drawn as lines.
	gl.GenVertexArrays(1, &r.boundaryVAO)
	gl.GenBuffers(1, &r.boundaryVBO)
	gl.BindVertexArray(r.boundaryVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.boundaryVBO)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, 24, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, 24, 12)
	gl.BindVertexArray(0)

	// Fence post VAO/VBO — same vertex layout, drawn as triangles.
	gl.GenVertexArrays(1, &r.fencePostVAO)
	gl.GenBuffers(1, &r.fencePostVBO)
	gl.BindVertexArray(r.fencePostVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.fencePostVBO)
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

	// Constant fallback for the per-vertex base-colour attribute (location 8).
	// Meshes that don't supply per-vertex colour leave this attribute
	// unbound; GL reads the global constant set here, so they render as
	// iColorTint × 1 instead of iColorTint × 0.
	gl.VertexAttrib3f(VertexColorLoc, 1, 1, 1)

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
		{MeshShed, "shed"},
		{MeshParkingPad, "parking"}, // built by models-src/parking.scad
		{MeshRoadConnect, "road_connect"},
		{MeshSnowGun, "snow_gun"}, // built by models-src/snow_gun.scad
		{MeshBar, "bar"},
		{MeshTicketOffice, "ticket_office"},
	}

	for _, def := range meshDefs {
		objPath := modelDir + def.name + ".obj"
		mesh, texID := LoadOBJ(objPath)
		r.staticBatches[def.id] = NewStaticBatch(mesh, texID)
		// Footprint metadata is optional — only the building-type meshes
		// publish it today. Missing values just leave the registry empty
		// and the placement-effects pass falls back to a default extent.
		if fp, ok := LoadOBJFootprint(objPath); ok {
			world.RegisterMeshFootprint(def.id, fp)
		}
		// Slot metadata — used by the parking lot's driveway position
		// today, and by anything else that needs a mesh-local anchor
		// in the future. Missing slots register as nil; consumers fall
		// back to a sensible default (parking's DrivewayPosition does
		// a halfZ-edge guess if no slot 0 exists).
		if slots := LoadOBJSlots(objPath); len(slots) > 0 {
			world.RegisterMeshSlots(def.id, slots)
		}
	}

	for kind, name := range lodgeTileNames {
		mesh, texID := LoadOBJ(modelDir + name + ".obj")
		r.staticBatches[MeshLodgeTileBase+uint32(kind)] = NewStaticBatch(mesh, texID)
	}

	// Road node marker — procedural disc (thin cylinder slice). Sits
	// flush on the snow surface and reads as a "click target" puck
	// from the top-down gameplay camera. White texture + per-instance
	// tint keeps the colour palette decision in the placement code.
	r.staticBatches[MeshRoadNode] = NewStaticBatch(NewCylinderMesh(1.5, 0.15, 24), r.whiteTexID)

	// Skier — dynamic batch. One instance per world.Guest with skis on.
	skierMesh, skierTexID := LoadOBJ(modelDir + "skier.obj")
	r.dynamicBatch = NewDynamicBatch(skierMesh, skierTexID)

	// Walker — same guest figure but without skis; drawn when SkisOn==false.
	walkerMesh, walkerTexID := LoadOBJ(modelDir + "walker.obj")
	r.walkerBatch = NewDynamicBatch(walkerMesh, walkerTexID)

	// Chair — dynamic batch (heading rotates each chair along the cable).
	// Three variants: double, fixed-grip quad, and high-speed 6-pack. Each
	// has its own batch + slot registration so per-rider seating works
	// for all types without per-frame mesh switching.
	chairPath := modelDir + "chair.obj"
	chairMesh, chairTexID := LoadOBJ(chairPath)
	r.chairBatch = NewDynamicBatch(chairMesh, chairTexID)
	world.RegisterMeshSlots(world.MeshChair, LoadOBJSlots(chairPath))

	chairQuadPath := modelDir + "chair_quad.obj"
	chairQuadMesh, chairQuadTexID := LoadOBJ(chairQuadPath)
	r.chairQuadBatch = NewDynamicBatch(chairQuadMesh, chairQuadTexID)
	world.RegisterMeshSlots(world.MeshChairQuad, LoadOBJSlots(chairQuadPath))

	chair6PackPath := modelDir + "chair_6pack.obj"
	chair6PackMesh, chair6PackTexID := LoadOBJ(chair6PackPath)
	r.chair6PackBatch = NewDynamicBatch(chair6PackMesh, chair6PackTexID)
	world.RegisterMeshSlots(world.MeshChair6Pack, LoadOBJSlots(chair6PackPath))

	gondolaCabinPath := modelDir + "gondola_cabin.obj"
	gondolaCabinMesh, gondolaCabinTexID := LoadOBJ(gondolaCabinPath)
	r.gondolaBatch = NewDynamicBatch(gondolaCabinMesh, gondolaCabinTexID)
	world.RegisterMeshSlots(world.MeshGondolaCabin, LoadOBJSlots(gondolaCabinPath))

	// Snowcat — dynamic batch. Snowcats drive over the terrain every
	// tick so they share the agent-style instance path rather than
	// sitting in the static batches.
	snowcatMesh, snowcatTexID := LoadOBJ(modelDir + "snowcat.obj")
	r.snowcatBatch = NewDynamicBatch(snowcatMesh, snowcatTexID)
	// Lamp lenses and roof beacon, drawn unlit on top of the cat mesh.
	r.catLensBatch = NewDynamicBatch(NewBoxMesh(0.14, 0.24, 0.42), r.whiteTexID)
	r.catBeaconBatch = NewDynamicBatch(NewBoxMesh(0.3, 0.26, 0.3), r.whiteTexID)

	// Ski patrol snowmobiles — dedicated snowmobile mesh, drawn with a red
	// tint when active so they're visually distinct from grooming snowcats.
	patrollerMesh, patrollerTexID := LoadOBJ(modelDir + "snowmobile.obj")
	r.patrollerBatch = NewDynamicBatch(patrollerMesh, patrollerTexID)

	// Cars — dynamic batch. Each parking lot's CurrentCars fluctuates as
	// skiers arrive / depart; rather than rebuild the whole static batch
	// every time the count ticks, we draw cars from a per-frame instance
	// list keyed off live parking-lot state.
	carMesh, carTexID := LoadOBJ(modelDir + "car.obj")
	r.carBatch = NewDynamicBatch(carMesh, carTexID)

	// Helipad — static-batch mesh placed at Base and Top of each HeliLift.
	helipadMesh, helipadTexID := LoadOBJ(modelDir + "helipad.obj")
	r.staticBatches[MeshHelipad] = NewStaticBatch(helipadMesh, helipadTexID)
	if fp, ok := LoadOBJFootprint(modelDir + "helipad.obj"); ok {
		world.RegisterMeshFootprint(MeshHelipad, fp)
	}

	// Helicopter body (rigid fuselage, tail, skids, mast) and animated
	// rotor parts declared via # part metadata in the body OBJ.
	heliBodyPath := modelDir + "helicopter_body.obj"
	heliBodyMesh, heliBodyTexID := LoadOBJ(heliBodyPath)
	r.helicopterBodyBatch = NewDynamicBatch(heliBodyMesh, heliBodyTexID)
	for _, pd := range LoadOBJParts(heliBodyPath) {
		partMesh, partTexID := LoadOBJ(modelDir + pd.Name + ".obj")
		rate := float32(25.0) // rad/s default (main rotor ≈ 240 RPM)
		if pd.Axis == "spin_z" {
			rate = 90.0 // tail rotor ≈ 860 RPM
		}
		r.helicopterPartBatches = append(r.helicopterPartBatches, partBatch{
			batch:    NewDynamicBatch(partMesh, partTexID),
			decl:     pd,
			spinRate: rate,
		})
	}
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

// BuildSnowSurfaceTex (re)allocates the GPU mirror of Terrain.Surface and
// uploads its current contents. Called at scene install time after
// BuildTerrainMesh. Subsequent edits go through FlushSnowSurface, which
// uploads only the dirty sub-region.
func (r *Renderer) BuildSnowSurfaceTex(t *world.Terrain) {
	if t == nil || t.Surface == nil {
		return
	}
	sd := t.Surface

	if r.scene.snowSurfaceTex != 0 {
		gl.DeleteTextures(1, &r.scene.snowSurfaceTex)
		r.scene.snowSurfaceTex = 0
	}

	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8,
		int32(sd.PxWidth), int32(sd.PxHeight), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(sd.Pixels))
	gl.BindTexture(gl.TEXTURE_2D, 0)

	r.scene.snowSurfaceTex = tex
	r.scene.snowSurfaceTexW = int32(sd.PxWidth)
	r.scene.snowSurfaceTexH = int32(sd.PxHeight)
	sd.Dirty = false
	sd.DirtyBox = image.Rectangle{}
}

// FlushSnowSurface uploads the dirty sub-region of Terrain.Surface to the
// GPU texture and clears the buffer's dirty flag. No-op if the texture
// hasn't been built yet or nothing is dirty. Coalesced to once per frame
// by the caller.
func (r *Renderer) FlushSnowSurface(t *world.Terrain) {
	if t == nil || t.Surface == nil || r.scene.snowSurfaceTex == 0 {
		return
	}
	sd := t.Surface
	if !sd.Dirty || sd.DirtyBox.Empty() {
		return
	}
	x0, y0 := sd.DirtyBox.Min.X, sd.DirtyBox.Min.Y
	w := sd.DirtyBox.Dx()
	h := sd.DirtyBox.Dy()
	if w <= 0 || h <= 0 {
		sd.Dirty = false
		sd.DirtyBox = image.Rectangle{}
		return
	}
	// Sub-row uploads need UNPACK_ROW_LENGTH so glTexSubImage2D walks
	// the buffer's full stride (PxWidth × 4) while only reading the
	// dirty sub-region. ALIGNMENT=1 because we're feeding tightly packed
	// uint8s, not 4-aligned floats.
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, int32(sd.PxWidth))
	offset := (y0*sd.PxWidth + x0) * 4
	gl.BindTexture(gl.TEXTURE_2D, r.scene.snowSurfaceTex)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0,
		int32(x0), int32(y0), int32(w), int32(h),
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(sd.Pixels[offset:]))
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)
	gl.BindTexture(gl.TEXTURE_2D, 0)

	sd.Dirty = false
	sd.DirtyBox = image.Rectangle{}
}

// groomTexUnit is the terrain pass's texture unit for the groom map.
const groomTexUnit = 5

// BuildGroomTex (re)allocates the GPU mirror of Terrain.Groom and uploads
// it. Later stamps go through FlushGroom.
func (r *Renderer) BuildGroomTex(t *world.Terrain) {
	if t == nil || t.Groom == nil {
		return
	}
	g := t.Groom
	if r.scene.groomTex != 0 {
		gl.DeleteTextures(1, &r.scene.groomTex)
	}
	gl.GenTextures(1, &r.scene.groomTex)
	gl.BindTexture(gl.TEXTURE_2D, r.scene.groomTex)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 2)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA16, int32(g.W), int32(g.H), 0,
		gl.RGBA, gl.UNSIGNED_SHORT, gl.Ptr(g.Pixels))
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	g.Dirty = false
	g.DirtyBox = image.Rectangle{}
}

// FlushGroom uploads the dirty sub-region of Terrain.Groom.
func (r *Renderer) FlushGroom(t *world.Terrain) {
	if t == nil || t.Groom == nil || r.scene.groomTex == 0 {
		return
	}
	g := t.Groom
	box := g.DirtyBox
	g.Dirty = false
	g.DirtyBox = image.Rectangle{}
	if box.Empty() {
		return
	}
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 2)
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, int32(g.W))
	gl.BindTexture(gl.TEXTURE_2D, r.scene.groomTex)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0,
		int32(box.Min.X), int32(box.Min.Y), int32(box.Dx()), int32(box.Dy()),
		gl.RGBA, gl.UNSIGNED_SHORT, gl.Ptr(g.Pixels[(box.Min.Y*g.W+box.Min.X)*4:]))
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	gl.BindTexture(gl.TEXTURE_2D, 0)
}

// SetCellOverlay uploads a per-cell RGBA8 overlay texture (w×h texels, one
// per terrain cell). The terrain shader linearly interpolates and alpha-blends
// this over the surface, so cell boundaries feather naturally. Pass nil pixels
// (or w/h=0) to clear the overlay.
func (r *Renderer) SetCellOverlay(pixels []uint8, w, h int) {
	if r.scene.cellOverlayTex != 0 {
		gl.DeleteTextures(1, &r.scene.cellOverlayTex)
		r.scene.cellOverlayTex = 0
	}
	if len(pixels) == 0 || w == 0 || h == 0 {
		return
	}
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8,
		int32(w), int32(h), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
	gl.BindTexture(gl.TEXTURE_2D, 0)
	r.scene.cellOverlayTex = tex
}

// treeMatrix places one forest tree, its scale multiplied by grow.
// Trees root in the ground, then poke through the snow above: the mesh
// is anchored just above ground, capped by the visible snow column so
// light snow lets the full trunk show while deeper snow raises the
// anchor and hides the lower trunk below the surface mesh. Mesh trees
// are ~7 m tall in model units; ti.Scale brings them to ~11–14 m.
func treeMatrix(t *world.Terrain, ti world.TreeInstance, grow float32) mgl32.Mat4 {
	elev := t.GroundElevationAt(ti.X, ti.Z) + t.DetailAt(ti.WX, ti.WZ)
	if snow := t.Cells[ti.X][ti.Z].VisibleSnowDepth(); snow > 0 {
		const maxBury = float32(1.5)
		elev += min(snow, maxBury)
	}
	s := ti.Scale * grow
	return mgl32.Translate3D(ti.WX, elev, ti.WZ).
		Mul4(mgl32.HomogRotate3DY(ti.Rotation)).
		Mul4(mgl32.Scale3D(s, s, s))
}

// TreeTransform is treeMatrix as a StaticInstance transform, for ghost
// overlays that sit on a forest tree.
func TreeTransform(t *world.Terrain, ti world.TreeInstance, grow float32) [16]float32 {
	m := treeMatrix(t, ti, grow)
	var out [16]float32
	copy(out[:], m[:])
	return out
}

// RebuildStaticBatch rebuilds all static instance buffers from world state.
func (r *Renderer) RebuildStaticBatch(w *world.World) {
	// Clear all static batches
	for _, b := range r.staticBatches {
		b.ClearStatic()
	}

	const cellSize = float32(5.0)

	// Forest layer — the stored trees, via the same iterator the
	// tree-well stamp uses.
	w.Terrain.ForEachTree(MeshTree, func(ti world.TreeInstance) {
		if batch, ok := r.staticBatches[ti.Variant]; ok {
			batch.AddStatic(treeMatrix(w.Terrain, ti, 1), treeTintForVariant(ti.Variant))
		}
	})

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
		// Decorative natural objects (lone trees, rocks, stumps) anchor at
		// ground and let snow bury their base — same rule as forest cover.
		y := w.Terrain.GroundElevationAt(obj.Pos[0], obj.Pos[1])
		if w.Terrain.InBounds(obj.Pos[0], obj.Pos[1]) {
			snow := w.Terrain.Cells[obj.Pos[0]][obj.Pos[1]].VisibleSnowDepth()
			const maxBury = float32(1.5)
			if snow > maxBury {
				snow = maxBury
			}
			if snow > 0 {
				y += snow
			}
		}
		t := mgl32.Translate3D(x, y, z).Mul4(mgl32.HomogRotate3DY(obj.Rotation))
		tint := mgl32.Vec3{1, 1, 1}
		if obj.Type == world.ObjTree {
			tint = treeTintForVariant(batchID)
		}
		batch.AddStatic(t, tint)
	}

	// Buildings — mesh varies by Type. Lodges use the building mesh;
	// sheds use the dedicated shed mesh; parking lots use a flat asphalt
	// pad and draw their cars dynamically. New types add another case.
	for _, bldg := range w.Buildings {
		if bldg.IsCellLot() {
			continue // drawn procedurally by RebuildParkingLots
		}
		if bldg.IsShell() {
			r.addLodgeShell(w, bldg)
			continue
		}
		meshID := MeshBuilding
		switch bldg.Type {
		case world.BuildingShed:
			meshID = MeshShed
		case world.BuildingParking:
			meshID = MeshParkingPad
		case world.BuildingSnowGun:
			meshID = MeshSnowGun
		case world.BuildingBar:
			meshID = MeshBar
		case world.BuildingTicketOffice:
			meshID = MeshTicketOffice
		}
		if batch, ok := r.staticBatches[meshID]; ok {
			batch.AddStatic(BuildingTransform(bldg.Pos, bldg.Rotation, w.Terrain), mgl32.Vec3{1, 1, 1})
		}
	}

	// Lift stations — both ends of each cable lift. Skipped for
	// helicopter lifts, which use the helipad mesh instead.
	if stationBatch, ok := r.staticBatches[MeshLiftStation]; ok {
		for _, lift := range w.Lifts {
			if lift.IsHeli() {
				continue
			}
			stationBatch.AddStatic(LiftStationTransform(lift.Base, lift.Top, w.Terrain), mgl32.Vec3{1, 1, 1})
			stationBatch.AddStatic(LiftStationTransform(lift.Top, lift.Base, w.Terrain), mgl32.Vec3{1, 1, 1})
		}
	}

	// Towers — cable lifts only.
	if towerBatch, ok := r.staticBatches[MeshTower]; ok {
		for _, lift := range w.Lifts {
			if lift.IsHeli() {
				continue
			}
			for _, m := range TowerInstancesForLift(lift.Base, lift.Top, w.Terrain) {
				towerBatch.AddStatic(m, mgl32.Vec3{1, 1, 1})
			}
		}
	}

	// Helipads — one at Base, one at Top for every HeliLift.
	// Oriented so the pad's +X axis points toward the other end.
	if helipadBatch, ok := r.staticBatches[MeshHelipad]; ok {
		for _, lift := range w.Lifts {
			if !lift.IsHeli() {
				continue
			}
			helipadBatch.AddStatic(HelipadTransform(lift.Base, lift.Top, w.Terrain), mgl32.Vec3{1, 1, 1})
			helipadBatch.AddStatic(HelipadTransform(lift.Top, lift.Base, w.Terrain), mgl32.Vec3{1, 1, 1})
		}
	}

	// Road edge-connection markers — one yellow-flag post per node with
	// kind RoadNodeEdgeConnection. Editor-placed scenario metadata, but
	// rendered in scenarios too so the player can see where the road
	// network meets the map perimeter.
	if connectBatch, ok := r.staticBatches[MeshRoadConnect]; ok {
		for _, n := range w.RoadNodes {
			if n.Kind != world.RoadNodeEdgeConnection {
				continue
			}
			connectBatch.AddStatic(RoadConnectTransform(n.Pos, w.Terrain), mgl32.Vec3{1, 1, 1})
		}
	}

	r.RebuildParkingLots(w)
}

// RoadConnectTransform builds the world-space transform for an edge-
// connection marker at world XZ `pos`. Y comes from the snow surface so
// the post sits on whatever's currently underfoot. No rotation: the flag
// is small enough that orientation isn't load-bearing visually.
func RoadConnectTransform(pos mgl32.Vec2, terrain *world.Terrain) mgl32.Mat4 {
	y := VisualElevationAt(terrain, pos[0], pos[1])
	return mgl32.Translate3D(pos[0], y, pos[1])
}

// RoadNodeMarkerTransform builds the world-space transform for a node-
// highlight marker at world XZ `pos`. Lifted a touch above the surface
// so the disc sits cleanly on top of the road quad (which already rides
// 5 cm above the terrain) without z-fighting.
func RoadNodeMarkerTransform(pos mgl32.Vec2, terrain *world.Terrain) mgl32.Mat4 {
	const markerHover = float32(0.10)
	y := VisualElevationAt(terrain, pos[0], pos[1]) + markerHover
	return mgl32.Translate3D(pos[0], y, pos[1])
}

// BuildingTransform builds the world-space transform for a building
// placed at world XZ pos with the given Y rotation. Used by both live
// placement (RebuildStaticBatch) and ghost preview.
// addLodgeShell instances lodge b's resolved kit tiles on its floor.
func (r *Renderer) addLodgeShell(w *world.World, b *world.Building) {
	floor := w.ShellFloorY(b)
	wallTint, roofTint := world.ShellPalette(b.StyleSeed)
	cutaway := b.ID == r.cutawayLodgeID
	for _, t := range world.ResolveLodgeShell(b) {
		if cutaway && (t.Kind.IsRoof() || t.Kind == world.TileChimney) {
			continue
		}
		batch, ok := r.staticBatches[MeshLodgeTileBase+uint32(t.Kind)]
		if !ok {
			continue
		}
		tint := shellTileTint(t, wallTint, roofTint)
		m := mgl32.Translate3D(t.Pos[0], floor+t.Pos[1], t.Pos[2]).Mul4(mgl32.HomogRotate3DY(t.Rot))
		if cutaway {
			m = m.Mul4(mgl32.Scale3D(1, lodgeCutawayWallScale, 1))
		}
		batch.AddStatic(m, tint)
	}
}

// shellTileTint is a kit tile's colour: the roof palette on roofs, the
// wall palette elsewhere, with walls shaded by their tile's service.
func shellTileTint(t world.ShellTile, wall, roof mgl32.Vec3) mgl32.Vec3 {
	if t.Kind.IsRoof() {
		return roof
	}
	if t.Service != world.ServiceNone {
		a := t.Service.Accent()
		return mgl32.Vec3{wall[0] * a[0], wall[1] * a[1], wall[2] * a[2]}
	}
	return wall
}

// SetShellGhost previews kit tiles at floor height floor, every tile
// tinted tint and scaled by scale about the tiles' centre (a little over
// 1 keeps a ghost over an existing tile from z-fighting). The ghost
// batches are cleared each frame by ClearAllGhosts.
func (r *Renderer) SetShellGhost(tiles []world.ShellTile, floor, scale float32, tint [3]float32) {
	if len(tiles) == 0 {
		return
	}
	var c mgl32.Vec3
	for _, t := range tiles {
		c = c.Add(t.Pos)
	}
	c = c.Mul(1 / float32(len(tiles)))
	c[1] = 0
	byMesh := map[uint32][]StaticInstance{}
	for _, t := range tiles {
		p := c.Add(t.Pos.Sub(c).Mul(scale))
		m := mgl32.Translate3D(p[0], floor+p[1], p[2]).
			Mul4(mgl32.HomogRotate3DY(t.Rot)).
			Mul4(mgl32.Scale3D(scale, scale, scale))
		inst := StaticInstance{ColorTint: tint}
		copy(inst.Transform[:], m[:])
		id := MeshLodgeTileBase + uint32(t.Kind)
		byMesh[id] = append(byMesh[id], inst)
	}
	for id, insts := range byMesh {
		r.SetGhosts(id, insts)
	}
}

// lodgeCutawayWallScale squashes a cutaway lodge's walls to about 1.25 m
// so the floor plan reads from the default camera pitch.
const lodgeCutawayWallScale = 0.25

// SetLodgeCutaway picks the lodge drawn as a cutaway (0 for none) and
// rebuilds the static batch when that changes.
func (r *Renderer) SetLodgeCutaway(w *world.World, id uint64) {
	if id == r.cutawayLodgeID {
		return
	}
	r.cutawayLodgeID = id
	r.RebuildStaticBatch(w)
}

func BuildingTransform(pos mgl32.Vec2, rotation float32, terrain *world.Terrain) mgl32.Mat4 {
	y := VisualElevationAt(terrain, pos[0], pos[1])
	return mgl32.Translate3D(pos[0], y, pos[1]).Mul4(mgl32.HomogRotate3DY(rotation))
}

// carInstancesFor enumerates the parked-car instances across every parking
// lot in the world: one car on each of the first CurrentCars stalls, so
// MaxCars in the lot's popup matches what the renderer actually fills.
func carInstancesFor(w *world.World) []DynamicInstance {
	var instances []DynamicInstance
	for _, b := range w.Buildings {
		if b.Type != world.BuildingParking {
			continue
		}
		count := int(b.CurrentCars)
		if count > len(b.Stalls) {
			count = len(b.Stalls)
		}
		for i := 0; i < count; i++ {
			s := b.Stalls[i]
			// A small lift above the stripes keeps the wheels from
			// z-fighting with the paint.
			y := parkingSurfaceY(w.Terrain, s.Pos[0], s.Pos[1]) + parkingHoverOffset + parkingStripeHover + 0.02
			// Subtle deterministic tint variation so the lot doesn't read
			// as a single flat block — derived from the lot ID and stall
			// index so the same car at the same stall keeps the same colour.
			hash := uint32(b.ID*31 + uint64(i)*17)
			r := 0.35 + float32(hash&0x3f)/255.0
			g := 0.35 + float32((hash>>6)&0x3f)/255.0
			bl := 0.35 + float32((hash>>12)&0x3f)/255.0
			instances = append(instances, DynamicInstance{
				Position: [3]float32{s.Pos[0], y, s.Pos[1]},
				Heading:  s.Heading,
				Color:    [3]float32{r, g, bl},
			})
		}
	}
	return instances
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
		cy := t.InterpolatedSurfaceElevationAt(cx, cz) + world.CableHeightAt(frac, length)

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
	return NewMesh(verts, indices, []int{3, 3, 2}, nil)
}

// generateQueueMesh builds a world-space mesh (poles + ropes) for the
// queue-line dividers at a lift's base station. Returns nil when the lift
// has no lane configuration.
//
// Ropes run LATERALLY (perpendicular to the cable) at each downhill boundary
// returned by QueueRopeBoundaries. They extend RopeDepth metres to the left
// and right of the cable axis, with poles spaced every polePitch metres.
func generateQueueMesh(lift *world.Lift, t *world.Terrain) *Mesh {
	downhillOffsets := lift.QueueRopeBoundaries()
	if len(downhillOffsets) == 0 {
		return nil
	}

	const (
		polePitch  = float32(3.0)  // metres between consecutive poles along a rope
		poleRadius = float32(0.05) // pole cross-section radius
		poleHeight = float32(1.10) // pole height above snow surface
		ropeHeight = float32(0.95) // rope centre height above snow surface
		ropeHW     = float32(0.02) // rope half-width / half-thickness
		poleFaces  = 6
	)

	lateralDepth := lift.QueueConfig.EffectiveRopeDepth() // how far left/right ropes extend
	qx, qz := lift.QueueDirXZ()
	rx, rz := lift.LateralRightXZ()

	var verts []float32
	var idxs []uint32

	appendCylinder := func(cx, cy, cz float32) {
		base := uint32(len(verts) / 8)
		twoPi := float32(2 * math.Pi)
		for i := 0; i < poleFaces; i++ {
			a0 := twoPi * float32(i) / float32(poleFaces)
			a1 := twoPi * float32(i+1) / float32(poleFaces)
			for _, a := range [2]float32{a0, a1} {
				nx := float32(math.Cos(float64(a)))
				nz := float32(math.Sin(float64(a)))
				u := float32(i) / float32(poleFaces)
				verts = append(verts,
					cx+poleRadius*nx, cy, cz+poleRadius*nz, nx, 0, nz, u, 0,
					cx+poleRadius*nx, cy+poleHeight, cz+poleRadius*nz, nx, 0, nz, u, 1,
				)
			}
			vi := base + uint32(i)*4
			idxs = append(idxs, vi, vi+2, vi+1, vi+1, vi+2, vi+3)
		}
		topCtr := uint32(len(verts) / 8)
		verts = append(verts, cx, cy+poleHeight, cz, 0, 1, 0, 0.5, 0.5)
		twoPi2 := float32(2 * math.Pi)
		for i := 0; i < poleFaces; i++ {
			a := twoPi2 * float32(i) / float32(poleFaces)
			nx := float32(math.Cos(float64(a)))
			nz := float32(math.Sin(float64(a)))
			verts = append(verts, cx+poleRadius*nx, cy+poleHeight, cz+poleRadius*nz, 0, 1, 0, 0.5+0.5*nx, 0.5+0.5*nz)
		}
		for i := 0; i < poleFaces; i++ {
			idxs = append(idxs, topCtr, topCtr+1+uint32(i), topCtr+1+uint32((i+1)%poleFaces))
		}
	}

	appendRope := func(p0x, p0z, p1x, p1z float32) {
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
		corners := [8][3]float32{
			{mx - fx*hlen - lx*ropeHW, my - ropeHW, mz - fz*hlen - lz*ropeHW},
			{mx + fx*hlen - lx*ropeHW, my - ropeHW, mz + fz*hlen - lz*ropeHW},
			{mx + fx*hlen + lx*ropeHW, my - ropeHW, mz + fz*hlen + lz*ropeHW},
			{mx - fx*hlen + lx*ropeHW, my - ropeHW, mz - fz*hlen + lz*ropeHW},
			{mx - fx*hlen - lx*ropeHW, my + ropeHW, mz - fz*hlen - lz*ropeHW},
			{mx + fx*hlen - lx*ropeHW, my + ropeHW, mz + fz*hlen - lz*ropeHW},
			{mx + fx*hlen + lx*ropeHW, my + ropeHW, mz + fz*hlen + lz*ropeHW},
			{mx - fx*hlen + lx*ropeHW, my + ropeHW, mz - fz*hlen + lz*ropeHW},
		}
		faces := [6][4]int{{3, 2, 1, 0}, {4, 5, 6, 7}, {0, 1, 5, 4}, {2, 3, 7, 6}, {3, 0, 4, 7}, {1, 2, 6, 5}}
		normals := [6][3]float32{
			{0, -1, 0}, {0, 1, 0},
			{fx, 0, fz}, {-fx, 0, -fz},
			{-lx, 0, -lz}, {lx, 0, lz},
		}
		for fi, face := range faces {
			base := uint32(len(verts) / 8)
			nx, ny, nz := normals[fi][0], normals[fi][1], normals[fi][2]
			for vi, ci := range face {
				c := corners[ci]
				u := float32(vi & 1)
				v := float32(vi >> 1)
				verts = append(verts, c[0], c[1], c[2], nx, ny, nz, u, v)
			}
			idxs = append(idxs, base, base+1, base+2, base, base+2, base+3)
		}
	}

	// Each rope runs laterally from -lateralDepth to +lateralDepth at a
	// specific downhill offset from the base. Poles at every polePitch metres.
	nPoles := int(lateralDepth/polePitch)*2 + 1 // one per pitch each side plus centre
	for _, downhill := range downhillOffsets {
		// Base point of this rope (on the cable axis, at this downhill offset).
		baseX := lift.Base[0] + downhill*qx
		baseZ := lift.Base[1] + downhill*qz

		// Pole lateral offsets from −lateralDepth to +lateralDepth.
		poleWX := make([]float32, nPoles)
		poleWZ := make([]float32, nPoles)
		for i := 0; i < nPoles; i++ {
			lat := -lateralDepth + float32(i)*polePitch
			if lat > lateralDepth {
				lat = lateralDepth
			}
			poleWX[i] = baseX + lat*rx
			poleWZ[i] = baseZ + lat*rz
		}
		for i, px := range poleWX {
			pz := poleWZ[i]
			appendCylinder(px, VisualElevationAt(t, px, pz), pz)
		}
		for i := 0; i < nPoles-1; i++ {
			appendRope(poleWX[i], poleWZ[i], poleWX[i+1], poleWZ[i+1])
		}
	}

	if len(verts) == 0 {
		return nil
	}
	return NewMesh(verts, idxs, []int{3, 3, 2}, nil)
}

// DrawWorld renders the full 3D world.
func (r *Renderer) DrawWorld(w *world.World, time float32) {
	light := r.Lighting
	if light == (Lighting{}) {
		light = DefaultLighting
		light.Sky = weatherSky(r.WeatherOverlay)
	}
	r.shadowMap.ready = false
	if light.Shadows && w.Terrain != nil {
		t := r.Camera.Target
		r.shadowMap.render(r, light.SunDir, w.Terrain.InterpolatedSurfaceElevationAt(t[0], t[2]))
	}
	r.shadowMap.bind()

	gl.ClearColor(light.Sky[0], light.Sky[1], light.Sky[2], 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	vp := r.Camera.ViewProj()
	lamps, lenses, beacons := snowcatLights(w, w.Snowcats, r.Camera.WorldPos(), time)
	if light.Shadows && w.Terrain != nil {
		r.terrainShadow.update(w.Terrain, light.SunDir)
	}
	r.terrainShadow.bind()

	// Terrain pass
	if r.scene.terrainMesh != nil {
		r.TerrainShader.Use()
		light.apply(r.TerrainShader)
		r.applyHaze(r.TerrainShader)
		applyLamps(r.TerrainShader, lamps, light.Night)
		r.terrainShadow.apply(r.TerrainShader, light.Shadows)
		r.shadowMap.apply(r.TerrainShader, light.Shadows)
		r.TerrainShader.SetMat4("uViewProj", vp)
		r.TerrainShader.SetVec2("uBrushCenter", r.brushCenter)
		r.TerrainShader.SetFloat("uBrushRadius", r.brushRadius)
		r.TerrainShader.SetVec2("uRangeCenter", r.rangeCenter)
		r.TerrainShader.SetFloat("uRangeRadius", r.rangeRadius)
		r.TerrainShader.SetInt("uOverlayMode", r.TerrainOverlayMode)
		r.TerrainShader.SetVec3("uCameraPos", r.Camera.WorldPos())
		r.TerrainShader.SetFloat("uTime", time)
		r.TerrainShader.SetFloat("uTerrainMinY", r.scene.terrainMinY)
		r.TerrainShader.SetFloat("uTerrainMaxY", r.scene.terrainMaxY)

		// Sub-cell surface-detail texture (skier tracks, tree wells,
		// groom edges). Bound on unit 1 to leave unit 0 for the
		// static / UI passes that follow. The texture lives in world
		// metres × metres so the FS samples by vWorldPos.xz / uWorldSize.
		// uWorldSize is the terrain extent in metres = cells × 5.
		const cellSize = float32(5.0)
		r.TerrainShader.SetInt("uSnowSurface", 1)
		r.TerrainShader.SetVec2("uWorldSize", mgl32.Vec2{
			float32(r.scene.terrainWidth) * cellSize,
			float32(r.scene.terrainHeight) * cellSize,
		})
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, r.scene.snowSurfaceTex)
		r.TerrainShader.SetInt("uCellOverlay", 2)
		gl.ActiveTexture(gl.TEXTURE2)
		if r.scene.cellOverlayTex != 0 {
			gl.BindTexture(gl.TEXTURE_2D, r.scene.cellOverlayTex)
		} else {
			gl.BindTexture(gl.TEXTURE_2D, r.transparentTexID)
		}
		r.TerrainShader.SetInt("uGroomMap", groomTexUnit)
		gl.ActiveTexture(gl.TEXTURE0 + groomTexUnit)
		if r.scene.groomTex != 0 {
			gl.BindTexture(gl.TEXTURE_2D, r.scene.groomTex)
		} else {
			gl.BindTexture(gl.TEXTURE_2D, r.transparentTexID)
		}
		gl.ActiveTexture(gl.TEXTURE0)

		r.drawTerrain(r.TerrainShader, vp)
	}

	// Static pass
	r.StaticShader.Use()
	light.apply(r.StaticShader)
	r.applyHaze(r.StaticShader)
	applyLamps(r.StaticShader, lamps, light.Night)
	r.terrainShadow.apply(r.StaticShader, light.Shadows)
	r.shadowMap.apply(r.StaticShader, light.Shadows)
	r.StaticShader.SetMat4("uViewProj", vp)
	r.StaticShader.SetInt("uTexture", 0)
	r.StaticShader.SetFloat("uAlpha", 1.0)
	r.StaticShader.SetVec3("uPerceptionOrigin", r.perceptionOrigin)
	r.StaticShader.SetVec2("uPerceptionForwardXZ", r.perceptionForwardXZ)
	r.StaticShader.SetFloat("uPerceptionCosHalfAngle", r.perceptionCosHalfAngle)
	r.StaticShader.SetFloat("uPerceptionRadius", r.perceptionRadius)
	r.StaticShader.SetFloat("uCanopySnow", r.CanopySnow)
	gl.ActiveTexture(gl.TEXTURE0)

	// Road pass — drawn before the static batch so buildings/trees sit on
	// top of road quads where they overlap. Shares the cable shader path
	// (identity instance transform, dark asphalt tint via VertexAttrib).
	// Lane dashes ride on top of the asphalt with a brighter tint.
	if r.scene.roadMesh != nil {
		gl.BindTexture(gl.TEXTURE_2D, r.whiteTexID)
		setRoadTransformAttribs()
		r.scene.roadMesh.Draw()
	}
	if r.scene.roadLanesMesh != nil {
		gl.BindTexture(gl.TEXTURE_2D, r.whiteTexID)
		setRoadLaneTransformAttribs()
		r.scene.roadLanesMesh.Draw()
	}
	if r.scene.parkingMesh != nil {
		gl.BindTexture(gl.TEXTURE_2D, r.whiteTexID)
		setRoadTransformAttribs()
		r.scene.parkingMesh.Draw()
	}
	if r.scene.parkingStripesMesh != nil {
		gl.BindTexture(gl.TEXTURE_2D, r.whiteTexID)
		setParkingStripeTransformAttribs()
		r.scene.parkingStripesMesh.Draw()
	}

	for id, batch := range r.staticBatches {
		if isTreeMesh(id) {
			r.StaticShader.SetInt("uFoliage", 1)
			r.StaticShader.SetFloat("uFoliageHeight", batch.mesh.MaxY)
		} else {
			r.StaticShader.SetInt("uFoliage", 0)
		}
		batch.Draw()
	}
	r.StaticShader.SetInt("uFoliage", 0)

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
	// Queue poles and ropes — same world-space pass, lighter tint.
	setQueueTransformAttribs()
	for _, m := range r.scene.liftQueueMeshes {
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
	if r.scene.roadGhostMesh != nil {
		setRoadTransformAttribs()
		r.scene.roadGhostMesh.Draw()
	}
	r.StaticShader.SetFloat("uAlpha", 1.0)
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)

	// Dynamic pass (agents)
	r.DynamicShader.Use()
	light.apply(r.DynamicShader)
	r.applyHaze(r.DynamicShader)
	r.DynamicShader.SetFloat("uEmissive", 0)
	r.terrainShadow.apply(r.DynamicShader, light.Shadows)
	r.shadowMap.apply(r.DynamicShader, light.Shadows)
	r.DynamicShader.SetMat4("uViewProj", vp)
	r.DynamicShader.SetFloat("uTime", time)
	r.DynamicShader.SetFloat("uSpinRate", 0) // default; overridden per rotor draw

	if r.dynamicBatch != nil || r.walkerBatch != nil {
		skierInst := make([]DynamicInstance, 0, len(w.OnMountain))
		walkerInst := make([]DynamicInstance, 0)
		hr2 := r.HiddenRadius * r.HiddenRadius
		for _, agent := range w.OnMountain {
			if r.HiddenGuestID != 0 && agent.ID == r.HiddenGuestID {
				continue
			}
			if agent.OnPatrollerID != 0 {
				continue // being transported by snowmobile; rendered as part of that unit
			}
			if hr2 > 0 {
				dx := agent.Pos[0] - r.HiddenGuestPos[0]
				dz := agent.Pos[2] - r.HiddenGuestPos[2]
				if dx*dx+dz*dz < hr2 {
					continue
				}
			}
			posY := agent.Pos[1]
			if agent.OnLiftID == 0 {
				posY = VisualElevationAt(w.Terrain, agent.Pos[0], agent.Pos[2])
			}
			color := guestColor(w, agent)
			if r.HighlightGuestID != 0 && agent.ID == r.HighlightGuestID {
				color = [3]float32{1.0, 0.95, 0.1}
			}
			inst := DynamicInstance{
				Position: [3]float32{agent.Pos[0], posY, agent.Pos[2]},
				Heading:  agent.Heading,
				Color:    color,
				SpinMode: 1.0,
			}
			if agent.SkisOn {
				skierInst = append(skierInst, inst)
			} else {
				walkerInst = append(walkerInst, inst)
			}
		}
		if r.dynamicBatch != nil {
			r.dynamicBatch.SetDynamic(skierInst)
			r.dynamicBatch.Draw()
		}
		if r.walkerBatch != nil {
			r.walkerBatch.SetDynamic(walkerInst)
			r.walkerBatch.Draw()
		}
	}

	// Snowcats — same dynamic-instance path as agents. Driven by
	// world.Snowcats; the sim updates Pos and Heading every tick.
	if r.snowcatBatch != nil && len(w.Snowcats) > 0 {
		catInstances := make([]DynamicInstance, 0, len(w.Snowcats))
		for _, cat := range w.Snowcats {
			y := VisualElevationAt(w.Terrain, cat.Pos[0], cat.Pos[2])
			color := [3]float32{1, 1, 1}
			if r.HighlightCatID != 0 && cat.ID == r.HighlightCatID {
				color = [3]float32{1.0, 0.95, 0.1} // yellow tint when selected
			}
			catInstances = append(catInstances, DynamicInstance{
				Position: [3]float32{cat.Pos[0], y, cat.Pos[2]},
				Heading:  cat.Heading,
				Color:    color,
			})
		}
		r.snowcatBatch.SetDynamic(catInstances)
		r.snowcatBatch.Draw()

		r.DynamicShader.SetFloat("uEmissive", 1)
		r.catLensBatch.SetDynamic(lenses)
		r.catLensBatch.Draw()
		r.catBeaconBatch.SetDynamic(beacons)
		r.catBeaconBatch.Draw()
		r.DynamicShader.SetFloat("uEmissive", 0)
	}

	// Ski patrol snowmobiles — red when active, white when at hut.
	if r.patrollerBatch != nil && len(w.Patrollers) > 0 {
		pInst := make([]DynamicInstance, 0, len(w.Patrollers))
		for _, p := range w.Patrollers {
			y := VisualElevationAt(w.Terrain, p.Pos[0], p.Pos[2])
			color := [3]float32{1, 1, 1}
			if p.State != world.PatrollerAtHut {
				color = [3]float32{1.0, 0.2, 0.1} // red = responding
			}
			pInst = append(pInst, DynamicInstance{
				Position: [3]float32{p.Pos[0], y, p.Pos[2]},
				Heading:  p.Heading,
				Color:    color,
			})
		}
		r.patrollerBatch.SetDynamic(pInst)
		r.patrollerBatch.Draw()
	}

	// Helicopters — body + animated rotor parts, one set per HeliLift.
	if r.helicopterBodyBatch != nil {
		var bodyInst []DynamicInstance
		partInsts := make([][]DynamicInstance, len(r.helicopterPartBatches))
		for _, lift := range w.Lifts {
			if !lift.IsHeli() {
				continue
			}
			pos := lift.HeliWorldPos(w.Terrain)
			heading := lift.HeliHeading()
			s := float32(math.Sin(float64(heading)))
			c := float32(math.Cos(float64(heading)))
			bodyInst = append(bodyInst, DynamicInstance{
				Position: [3]float32{pos[0], pos[1], pos[2]},
				Heading:  heading,
				Color:    [3]float32{1, 1, 1},
			})
			// Each part: world pivot = body_pos + rotY(heading) * partOffset.
			// rotY in dynamic.vert: result.x = s*x - c*z, result.z = c*x + s*z
			for pi, pb := range r.helicopterPartBatches {
				off := pb.decl.Offset
				partInsts[pi] = append(partInsts[pi], DynamicInstance{
					Position: [3]float32{
						pos[0] + s*off[0] - c*off[2],
						pos[1] + off[1],
						pos[2] + c*off[0] + s*off[2],
					},
					Heading:  heading,
					Color:    [3]float32{1, 1, 1},
					SpinMode: spinModeFor(pb.decl.Axis),
				})
			}
		}
		r.helicopterBodyBatch.SetDynamic(bodyInst)
		r.helicopterBodyBatch.Draw()
		for pi, pb := range r.helicopterPartBatches {
			if len(partInsts[pi]) == 0 {
				continue
			}
			r.DynamicShader.SetFloat("uSpinRate", pb.spinRate)
			pb.batch.SetDynamic(partInsts[pi])
			pb.batch.Draw()
		}
		r.DynamicShader.SetFloat("uSpinRate", 0) // reset after rotor draws
	}

	// Parked cars — one box per filled parking-lot stall, count driven by
	// the lot's CurrentCars. Dynamic so the count can fluctuate per tick
	// without forcing a full static-batch rebuild.
	if r.carBatch != nil {
		carInstances := carInstancesFor(w)
		if len(carInstances) > 0 {
			r.carBatch.SetDynamic(carInstances)
			r.carBatch.Draw()
		}
	}

	// Debug-line pass (steering overlay etc.) — runs before chairs so chairs draw over.
	if r.DebugShader != nil {
		r.DebugShader.Use()
		r.DebugShader.SetMat4("uViewProj", vp)
		if r.debugVertCount > 0 {
			gl.BindVertexArray(r.debugVAO)
			gl.LineWidth(2.5)
			gl.DrawArrays(gl.LINES, 0, r.debugVertCount)
			gl.BindVertexArray(0)
		}
		if r.catPathVertCount > 0 {
			gl.BindVertexArray(r.catPathVAO)
			gl.LineWidth(5.0)
			gl.DrawArrays(gl.LINES, 0, r.catPathVertCount)
			gl.BindVertexArray(0)
			gl.LineWidth(1.0)
		}
		if r.fencePostVertCount > 0 {
			gl.BindVertexArray(r.fencePostVAO)
			gl.DrawArrays(gl.TRIANGLES, 0, r.fencePostVertCount)
			gl.BindVertexArray(0)
		}
		if r.boundaryVertCount > 0 {
			gl.BindVertexArray(r.boundaryVAO)
			gl.LineWidth(2.0)
			gl.DrawArrays(gl.LINES, 0, r.boundaryVertCount)
			gl.BindVertexArray(0)
			gl.LineWidth(1.0)
		}
	}

	// Chair pass — one dynamic batch per chair mesh variant. Group lift
	// instances by type so each batch draws its own chairs in a single
	// call, regardless of how many lifts of each type the resort has.
	// Re-bind the dynamic shader: the debug pass above may have switched it.
	if r.chairBatch != nil || r.chairQuadBatch != nil || r.chair6PackBatch != nil || r.gondolaBatch != nil {
		r.DynamicShader.Use()
		var doubles, quads, sixPacks, gondolas []DynamicInstance
		for _, lift := range w.Lifts {
			for _, chair := range lift.Chairs {
				pos, heading := lift.ChairPos(chair.Progress, w.Terrain)
				hasPax := false
				for _, p := range chair.Passengers {
					if p != nil {
						hasPax = true
						break
					}
				}
				color := [3]float32{0.7, 0.7, 0.7}
				if hasPax {
					color = [3]float32{0.55, 0.65, 0.85}
				}
				inst := DynamicInstance{
					Position: [3]float32{pos[0], pos[1], pos[2]},
					Heading:  heading,
					Color:    color,
				}
				switch lift.Type {
				case world.LiftFixedQuad, world.LiftHSQuad:
					quads = append(quads, inst)
				case world.LiftHS6Pack:
					sixPacks = append(sixPacks, inst)
				case world.LiftGondola:
					gondolas = append(gondolas, inst)
				default:
					doubles = append(doubles, inst)
				}
			}
		}
		if r.chairBatch != nil {
			r.chairBatch.SetDynamic(doubles)
			r.chairBatch.Draw()
		}
		if r.chairQuadBatch != nil {
			r.chairQuadBatch.SetDynamic(quads)
			r.chairQuadBatch.Draw()
		}
		if r.chair6PackBatch != nil {
			r.chair6PackBatch.SetDynamic(sixPacks)
			r.chair6PackBatch.Draw()
		}
		if r.gondolaBatch != nil {
			r.gondolaBatch.SetDynamic(gondolas)
			r.gondolaBatch.Draw()
		}
	}

	r.drawWeatherOverlay(time, light)
}

// drawWeatherOverlay draws a full-screen precipitation/atmosphere effect after
// all 3D geometry. It uses a single large triangle (no VBO; gl_VertexID only)
// with a procedural GLSL shader that animates falling snow or rain.
func (r *Renderer) drawWeatherOverlay(time float32, light Lighting) {
	if r.WeatherShader == nil || r.WeatherOverlay == 0 {
		return
	}
	gl.Disable(gl.DEPTH_TEST)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	defer func() {
		gl.Disable(gl.BLEND)
		gl.Enable(gl.DEPTH_TEST)
	}()

	aspect := float32(r.frameW) / float32(r.frameH)
	r.WeatherShader.Use()
	r.WeatherShader.SetFloat("uTime", time)
	r.WeatherShader.SetInt("uWeather", r.WeatherOverlay)
	r.WeatherShader.SetFloat("uAspect", aspect)
	r.WeatherShader.SetVec3("uLight", light.light())

	gl.BindVertexArray(r.weatherVAO)
	gl.DrawArrays(gl.TRIANGLES, 0, 3)
	gl.BindVertexArray(0)
}

func guestColor(w *world.World, a *world.Guest) [3]float32 {
	switch world.Activity(w, a) {
	case "Walking":
		return [3]float32{0.2, 0.6, 0.9}
	case "Queuing":
		return [3]float32{0.9, 0.7, 0.2}
	case "On Lift":
		return [3]float32{0.9, 0.4, 0.1}
	case "To Lift":
		return [3]float32{0.1, 0.8, 0.3}
	case "Departing":
		return [3]float32{0.8, 0.3, 0.8}
	case "Injured":
		return [3]float32{1.0, 0.1, 0.8}
	case "Fallen":
		return [3]float32{0.8, 0.1, 0.1}
	}
	return [3]float32{1, 1, 1}
}

// DrawUI renders screen-space UI elements. Sets up shader + blend state,
// then walks the drawables, each of which appends quads to the batch.
// flushUI at the end emits one (or a few, on texture-switches) DrawArrays
// for the whole frame's UI instead of one per glyph.
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

	// Reset batch state. Bind the white fallback so color-only quads
	// sample 1.0 and reduce to their vertex color.
	r.uiVerts = r.uiVerts[:0]
	r.uiBoundTex = 0
	r.useUITexture(r.whiteTexID)

	for _, e := range elements {
		e.Draw(r)
	}
	r.flushUI()
}

// useUITexture binds `tex` to texture unit 0 for the active UI batch.
// If a different texture was bound, the pending verts are flushed first
// so each draw call sees one consistent texture. Callers (DrawText for
// the font atlas, DrawIcon for icons, DrawTexturedRect for arbitrary
// textures) call this before appending their quads.
func (r *Renderer) useUITexture(tex uint32) {
	if r.uiBoundTex == tex {
		return
	}
	r.flushUI()
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	r.uiBoundTex = tex
}

// appendUIQuad pushes six vertices (two triangles) for an axis-aligned
// rect. Each vertex carries pos.xy + uv.xy + color.rgba (8 floats =
// 32 B). Hot path; no allocation if the capacity headroom holds.
func (r *Renderer) appendUIQuad(x, y, w, h, u0, v0, u1, v1 float32, c mgl32.Vec4) {
	x1, y1 := x+w, y+h
	r.uiVerts = append(r.uiVerts,
		x, y, u0, v0, c[0], c[1], c[2], c[3],
		x1, y, u1, v0, c[0], c[1], c[2], c[3],
		x1, y1, u1, v1, c[0], c[1], c[2], c[3],
		x, y, u0, v0, c[0], c[1], c[2], c[3],
		x1, y1, u1, v1, c[0], c[1], c[2], c[3],
		x, y1, u0, v1, c[0], c[1], c[2], c[3],
	)
}

// appendUITriangle pushes a single triangle with the given vertex
// positions and a flat color. UVs are all (0.5, 0.5) so the white
// fallback texture contributes neutral 1.0 — this is for the disc /
// diamond primitives, which use the same shader path as rects.
func (r *Renderer) appendUITriangle(x0, y0, x1, y1, x2, y2 float32, c mgl32.Vec4) {
	r.uiVerts = append(r.uiVerts,
		x0, y0, 0.5, 0.5, c[0], c[1], c[2], c[3],
		x1, y1, 0.5, 0.5, c[0], c[1], c[2], c[3],
		x2, y2, 0.5, 0.5, c[0], c[1], c[2], c[3],
	)
}

// flushUI uploads the pending verts and emits one DrawArrays. No-op if
// nothing is pending. Called automatically on texture changes and at
// the end of DrawUI.
func (r *Renderer) flushUI() {
	if len(r.uiVerts) == 0 {
		return
	}
	gl.BindVertexArray(r.uiVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.uiVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(r.uiVerts)*4, gl.Ptr(r.uiVerts), gl.DYNAMIC_DRAW)
	gl.DrawArrays(gl.TRIANGLES, 0, int32(len(r.uiVerts)/8))
	gl.BindVertexArray(0)
	r.uiVerts = r.uiVerts[:0]
}

// DrawTexturedRect draws a textured rectangle. The texture is bound (and
// any pending non-matching batch flushed) before the quad is appended.
func (r *Renderer) DrawTexturedRect(x, y, w, h float32, texID uint32, color mgl32.Vec4) {
	r.useUITexture(texID)
	r.appendUIQuad(x, y, w, h, 0, 0, 1, 1, color)
}

// DrawColorRect draws a solid color rectangle. Uses the white fallback
// texture so the fragment shader's `sample * vColor` reduces to vColor.
func (r *Renderer) DrawColorRect(x, y, w, h float32, color mgl32.Vec4) {
	r.useUITexture(r.whiteTexID)
	r.appendUIQuad(x, y, w, h, 0, 0, 1, 1, color)
}

// DrawColorRectOutline draws a 1-pixel border around a rectangle using
// four thin filled rects — no fill, just the frame. Used for button
// outlines and panel chrome.
func (r *Renderer) DrawColorRectOutline(x, y, w, h float32, color mgl32.Vec4) {
	r.DrawColorRect(x, y, w, 1, color)     // top
	r.DrawColorRect(x, y+h-1, w, 1, color) // bottom
	r.DrawColorRect(x, y, 1, h, color)     // left
	r.DrawColorRect(x+w-1, y, 1, h, color) // right
}

// DrawColorLine draws a thick line segment from (x0, y0) to (x1, y1) as
// two triangles forming a quad perpendicular to the segment direction.
// `thickness` is the total width in pixels. Degenerate (zero-length)
// segments draw nothing.
func (r *Renderer) DrawColorLine(x0, y0, x1, y1, thickness float32, color mgl32.Vec4) {
	dx := x1 - x0
	dy := y1 - y0
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length < 1e-6 {
		return
	}
	// Perpendicular unit vector × half-thickness.
	nx := -dy / length * (thickness * 0.5)
	ny := dx / length * (thickness * 0.5)
	r.useUITexture(r.whiteTexID)
	r.appendUITriangle(x0+nx, y0+ny, x1+nx, y1+ny, x1-nx, y1-ny, color)
	r.appendUITriangle(x0+nx, y0+ny, x1-nx, y1-ny, x0-nx, y0-ny, color)
}

// DrawColorDisc draws a filled circle centred at (cx, cy) with the
// given radius. Built from 24 triangles fanning out from the centre so
// it batches alongside rects and glyphs in the same DrawArrays call.
func (r *Renderer) DrawColorDisc(cx, cy, radius float32, color mgl32.Vec4) {
	const segments = 24
	r.useUITexture(r.whiteTexID)
	prevX := cx + radius
	prevY := cy
	for i := 1; i <= segments; i++ {
		theta := float64(i) / float64(segments) * 2 * math.Pi
		nx := cx + radius*float32(math.Cos(theta))
		ny := cy + radius*float32(math.Sin(theta))
		r.appendUITriangle(cx, cy, prevX, prevY, nx, ny, color)
		prevX, prevY = nx, ny
	}
}

// DrawColorDiamond draws a filled diamond (45-degree-rotated square)
// inscribed in the bounding box (cx, cy, half-diagonal=radius).
func (r *Renderer) DrawColorDiamond(cx, cy, radius float32, color mgl32.Vec4) {
	r.useUITexture(r.whiteTexID)
	top := [2]float32{cx, cy - radius}
	right := [2]float32{cx + radius, cy}
	bot := [2]float32{cx, cy + radius}
	left := [2]float32{cx - radius, cy}
	r.appendUITriangle(cx, cy, top[0], top[1], right[0], right[1], color)
	r.appendUITriangle(cx, cy, right[0], right[1], bot[0], bot[1], color)
	r.appendUITriangle(cx, cy, bot[0], bot[1], left[0], left[1], color)
	r.appendUITriangle(cx, cy, left[0], left[1], top[0], top[1], color)
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

// SetCatPathLines uploads cat-path line segments drawn thicker than debug lines.
func (r *Renderer) SetCatPathLines(lines []DebugLine) {
	if len(lines) == 0 {
		r.catPathVertCount = 0
		return
	}
	verts := make([]float32, 0, len(lines)*12)
	for _, l := range lines {
		verts = append(verts,
			l.A[0], l.A[1], l.A[2], l.Color[0], l.Color[1], l.Color[2],
			l.B[0], l.B[1], l.B[2], l.Color[0], l.Color[1], l.Color[2],
		)
	}
	gl.BindBuffer(gl.ARRAY_BUFFER, r.catPathVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.DYNAMIC_DRAW)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	r.catPathVertCount = int32(len(lines) * 2)
}

// SetBoundaryLines uploads fence line segments marking the ski-area boundary.
// Pass nil or an empty slice to clear.
func (r *Renderer) SetBoundaryLines(lines []DebugLine) {
	if len(lines) == 0 {
		r.boundaryVertCount = 0
		return
	}
	verts := make([]float32, 0, len(lines)*12)
	for _, l := range lines {
		verts = append(verts,
			l.A[0], l.A[1], l.A[2], l.Color[0], l.Color[1], l.Color[2],
			l.B[0], l.B[1], l.B[2], l.Color[0], l.Color[1], l.Color[2],
		)
	}
	gl.BindBuffer(gl.ARRAY_BUFFER, r.boundaryVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.DYNAMIC_DRAW)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	r.boundaryVertCount = int32(len(lines) * 2)
}

// SetFencePostVerts uploads flat-shaded triangle vertices for fence posts.
// Each vertex is pos(xyz) + color(rgb) = 6 floats. Pass nil to clear.
func (r *Renderer) SetFencePostVerts(verts []float32) {
	if len(verts) == 0 {
		r.fencePostVertCount = 0
		return
	}
	gl.BindBuffer(gl.ARRAY_BUFFER, r.fencePostVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.DYNAMIC_DRAW)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	r.fencePostVertCount = int32(len(verts) / 6)
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

// SetRange configures the terrain shader range ring (blue/white, used for
// snow gun radius display).
func (r *Renderer) SetRange(center mgl32.Vec2, radius float32) {
	r.rangeCenter = center
	r.rangeRadius = radius
}

// ClearRange disables the terrain shader range ring.
func (r *Renderer) ClearRange() {
	r.rangeRadius = 0
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

// HelipadTransform builds the world-space transform for a helipad placed at
// `pos`, oriented so the model's +X axis (the approach axis with the H
// pointing toward it) faces `otherEnd`. Pass `otherEnd == pos` to get no
// rotation (e.g. when the second point isn't known yet).
func HelipadTransform(pos, otherEnd mgl32.Vec2, terrain *world.Terrain) mgl32.Mat4 {
	y := VisualElevationAt(terrain, pos[0], pos[1])
	var rot float32
	if otherEnd != pos {
		dx := otherEnd[0] - pos[0]
		dz := otherEnd[1] - pos[1]
		rot = float32(math.Atan2(-float64(dz), float64(dx)))
	}
	return mgl32.Translate3D(pos[0], y, pos[1]).Mul4(mgl32.HomogRotate3DY(rot))
}

// AddLiftMeshes generates and stores cable and queue meshes for a lift.
// Towers are added to the static batch by RebuildStaticBatch. No-op for
// helicopter lifts, which have no cable infrastructure.
func (r *Renderer) AddLiftMeshes(lift *world.Lift, t *world.Terrain) {
	if lift.IsHeli() {
		return
	}
	r.scene.liftUpCables[lift.ID] = generateCableMesh(lift, t, world.CableGap)
	r.scene.liftDownCables[lift.ID] = generateCableMesh(lift, t, -world.CableGap)
	if m := generateQueueMesh(lift, t); m != nil {
		r.scene.liftQueueMeshes[lift.ID] = m
	}
}

// RemoveLiftMeshes removes the cable and queue meshes for a lift.
func (r *Renderer) RemoveLiftMeshes(liftID uint64) {
	if m, ok := r.scene.liftUpCables[liftID]; ok {
		m.Delete()
		delete(r.scene.liftUpCables, liftID)
	}
	if m, ok := r.scene.liftDownCables[liftID]; ok {
		m.Delete()
		delete(r.scene.liftDownCables, liftID)
	}
	if m, ok := r.scene.liftQueueMeshes[liftID]; ok {
		m.Delete()
		delete(r.scene.liftQueueMeshes, liftID)
	}
}

// AddLiftCable is kept for call-site compatibility; delegates to AddLiftMeshes.
func (r *Renderer) AddLiftCable(lift *world.Lift, t *world.Terrain) {
	r.AddLiftMeshes(lift, t)
}

// RebuildRoads regenerates the world's road meshes (asphalt body + dashed
// centre-line) from the current road graph. Call after any add/remove on
// RoadNodes / RoadEdges.
func (r *Renderer) RebuildRoads(w *world.World) {
	if r.scene.roadMesh != nil {
		r.scene.roadMesh.Delete()
		r.scene.roadMesh = nil
	}
	if r.scene.roadLanesMesh != nil {
		r.scene.roadLanesMesh.Delete()
		r.scene.roadLanesMesh = nil
	}
	r.scene.roadMesh = generateRoadsMesh(w, w.Terrain)
	r.scene.roadLanesMesh = generateRoadLanesMesh(w, w.Terrain)
}

// SetGhostRoad regenerates the in-flight road preview between a and b.
// Mirrors SetGhostCable: a single fresh mesh per frame as the cursor
// moves. The tint parameter is currently advisory — road meshes share
// the cable shader path, which uses a fixed dark grey for the strip;
// once roads pick up an instance-tinted path the tint will pass through.
func (r *Renderer) SetGhostRoad(a, b mgl32.Vec2, t *world.Terrain, tint [3]float32) {
	if r.scene.roadGhostMesh != nil {
		r.scene.roadGhostMesh.Delete()
	}
	r.scene.roadGhostMesh = generateRoadEdgeMesh(a, b, t)
	_ = tint // see doc comment — reserved for future use
}

// ClearGhostRoad removes the in-flight road preview.
func (r *Renderer) ClearGhostRoad() {
	if r.scene.roadGhostMesh != nil {
		r.scene.roadGhostMesh.Delete()
		r.scene.roadGhostMesh = nil
	}
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
	r.cutawayLodgeID = 0

	// Engine-owned static-batch shells survive scene transitions, but their
	// per-world instance lists must be cleared so trees/buildings/lifts from
	// the previous World don't keep drawing.
	for _, b := range r.staticBatches {
		b.ClearStatic()
	}

	r.brushCenter = mgl32.Vec2{}
	r.brushRadius = 0
	r.rangeCenter = mgl32.Vec2{}
	r.rangeRadius = 0
	r.perceptionOrigin = mgl32.Vec3{}
	r.perceptionForwardXZ = mgl32.Vec2{}
	r.perceptionCosHalfAngle = 0
	r.perceptionRadius = 0
	r.HighlightGuestID = 0
	r.TerrainOverlayMode = 0
	r.debugVertCount = 0
}

// ScreenWidth returns the window's logical width in points (matches mouse coords).
func (r *Renderer) ScreenWidth() int { return r.logicalW }

// ScreenHeight returns the window's logical height in points (matches mouse coords).
func (r *Renderer) ScreenHeight() int { return r.logicalH }

// WorldToScreen projects a world-space position to logical screen coordinates
// using the current camera ViewProj. Returns visible=false when the point is
// behind the camera (clip.W ≤ 0) or outside the NDC cube.
func (r *Renderer) WorldToScreen(pos mgl32.Vec3) (sx, sy float32, visible bool) {
	clip := r.Camera.ViewProj().Mul4x1(mgl32.Vec4{pos[0], pos[1], pos[2], 1})
	if clip[3] <= 0 {
		return 0, 0, false
	}
	ndcX := clip[0] / clip[3]
	ndcY := clip[1] / clip[3]
	ndcZ := clip[2] / clip[3]
	if ndcZ < -1 || ndcZ > 1 {
		return 0, 0, false
	}
	sx = (ndcX + 1) * 0.5 * float32(r.logicalW)
	sy = (1 - ndcY) * 0.5 * float32(r.logicalH)
	return sx, sy, true
}

// ReadFrame reads the current framebuffer into an *image.NRGBA. Must be called
// after rendering and before SwapBuffers. Returns nil if the framebuffer size
// is invalid.
func (r *Renderer) ReadFrame() *image.NRGBA {
	w, h := r.frameW, r.frameH
	if w <= 0 || h <= 0 {
		return nil
	}
	buf := make([]byte, w*h*4)
	gl.PixelStorei(gl.PACK_ALIGNMENT, 1)
	gl.ReadPixels(0, 0, int32(w), int32(h), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(buf))

	// glReadPixels origin is bottom-left; image origin is top-left. Flip rows.
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	stride := w * 4
	for y := 0; y < h; y++ {
		src := buf[(h-1-y)*stride : (h-y)*stride]
		copy(img.Pix[y*img.Stride:y*img.Stride+stride], src)
	}
	return img
}

// SaveScreenshot reads the default framebuffer with glReadPixels and writes it
// as a PNG to path. Must be called after rendering and before SwapBuffers (so
// the back buffer still holds the freshly drawn frame). Creates parent dirs as
// needed.
func (r *Renderer) SaveScreenshot(path string) error {
	img := r.ReadFrame()
	if img == nil {
		return fmt.Errorf("screenshot: invalid framebuffer size %dx%d", r.frameW, r.frameH)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
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
	gl.VertexAttrib4f(3, 1, 0, 0, 0)       // identity col 0
	gl.VertexAttrib4f(4, 0, 1, 0, 0)       // identity col 1
	gl.VertexAttrib4f(5, 0, 0, 1, 0)       // identity col 2
	gl.VertexAttrib4f(6, 0, 0, 0, 1)       // identity col 3
	gl.VertexAttrib3f(7, 0.15, 0.15, 0.15) // dark charcoal tint
}

// setQueueTransformAttribs sets instance attributes for queue pole/rope meshes:
// identity transform with a warm off-white tint so the poles read as painted
// wood or light metal without blending into the snow.
func setQueueTransformAttribs() {
	gl.VertexAttrib4f(3, 1, 0, 0, 0)
	gl.VertexAttrib4f(4, 0, 1, 0, 0)
	gl.VertexAttrib4f(5, 0, 0, 1, 0)
	gl.VertexAttrib4f(6, 0, 0, 0, 1)
	gl.VertexAttrib3f(7, 0.85, 0.80, 0.70) // warm off-white (bare wood / primed metal)
}

// setRoadTransformAttribs is the road-mesh counterpart to setCableTransformAttribs:
// identity transform + asphalt-grey tint. Slightly lighter than the cable tint
// so roads read as a distinct surface treatment rather than a cable shadow.
func setRoadTransformAttribs() {
	gl.VertexAttrib4f(3, 1, 0, 0, 0)
	gl.VertexAttrib4f(4, 0, 1, 0, 0)
	gl.VertexAttrib4f(5, 0, 0, 1, 0)
	gl.VertexAttrib4f(6, 0, 0, 0, 1)
	gl.VertexAttrib3f(7, 0.22, 0.22, 0.23) // asphalt grey, faint blue tinge
}

// setRoadLaneTransformAttribs is the lane-dash counterpart: identity transform
// + warm off-white so the dashes read as paint on asphalt without going
// pure white (which would over-bloom against a snowy backdrop).
func setRoadLaneTransformAttribs() {
	gl.VertexAttrib4f(3, 1, 0, 0, 0)
	gl.VertexAttrib4f(4, 0, 1, 0, 0)
	gl.VertexAttrib4f(5, 0, 0, 1, 0)
	gl.VertexAttrib4f(6, 0, 0, 0, 1)
	gl.VertexAttrib3f(7, 0.92, 0.88, 0.55) // warm cream — reads as faded yellow lane paint
}

// setParkingStripeTransformAttribs is the stall-line counterpart: identity
// transform + off-white, distinct from the road's cream lane paint.
func setParkingStripeTransformAttribs() {
	gl.VertexAttrib4f(3, 1, 0, 0, 0)
	gl.VertexAttrib4f(4, 0, 1, 0, 0)
	gl.VertexAttrib4f(5, 0, 0, 1, 0)
	gl.VertexAttrib4f(6, 0, 0, 0, 1)
	gl.VertexAttrib3f(7, 0.90, 0.90, 0.88)
}

// applyHaze sets the atmospheric haze uniforms (lighting.glsl applyHaze)
// from the camera and the terrain's elevation range.
func (r *Renderer) applyHaze(s *Shader) {
	if r.Camera.Perspective || r.scene.terrainMaxY <= r.scene.terrainMinY {
		s.SetInt("uHazeOn", 0)
		return
	}
	s.SetInt("uHazeOn", 1)
	s.SetVec3("uHazeOrigin", r.Camera.Target)
	s.SetVec3("uHazeForward", r.Camera.Target.Sub(r.Camera.WorldPos()).Normalize())
	s.SetVec2("uHazeLowHigh", mgl32.Vec2{r.scene.terrainMinY, r.scene.terrainMaxY})
}

// isTreeMesh reports whether a static batch ID is one of the conifer meshes,
// which the static shader lights as foliage.
func isTreeMesh(id uint32) bool {
	return id == MeshTree || id == MeshTree2 || id == MeshTree3
}

// treeTintForVariant returns the per-instance ColorTint for a tree variant.
// Foliage colour now lives in the .scad source per variant (medium pine /
// dark spruce / blue-green fir) and flows through as per-vertex base
// colour, so the tint is white — reserved for future per-instance effects
// (selection highlight, seasonal mood) on top of the SCAD-authored palette.
func treeTintForVariant(variant uint32) mgl32.Vec3 {
	return mgl32.Vec3{1, 1, 1}
}

// spinModeFor maps a PartDecl.Axis string to the DynamicInstance.SpinMode
// value consumed by dynamic.vert.
func spinModeFor(axis string) float32 {
	switch axis {
	case "spin_y":
		return 2.0
	case "spin_z":
		return 3.0
	}
	return 0.0
}
