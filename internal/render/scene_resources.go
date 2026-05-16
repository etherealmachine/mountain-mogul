package render

import "github.com/go-gl/gl/v4.1-core/gl"

// SceneResources owns GPU-side state coupled to a particular World — meshes
// keyed by entity ID and other per-scene previews. Replaced wholesale on every
// scene transition by Renderer.ResetSceneState so resources from a previous
// scenario can't bleed into the next.
//
// Engine-scoped resources (shaders, fonts, the static-batch shells, UI/debug
// VAOs, the dynamic batch for agents) live on Renderer directly and survive
// scene transitions.
type SceneResources struct {
	terrainMesh   *Mesh
	terrainVBO    uint32
	terrainWidth  int
	terrainHeight int
	terrainMinY   float32 // surface min/max Y (skirts excluded), drives topo shader
	terrainMaxY   float32

	// snowSurfaceTex is the GPU mirror of Terrain.Surface — a 1 m
	// resolution RGBA8 texture sampled by terrain.frag for sub-cell
	// features (skier tracks, tree wells, groom edges). Sized to
	// (Width*PxPerCell, Height*PxPerCell).
	snowSurfaceTex      uint32
	snowSurfaceTexW     int32
	snowSurfaceTexH     int32

	// cellOverlayTex is a per-cell RGBA8 overlay texture (one texel per
	// terrain cell). The terrain shader alpha-blends it over the surface.
	// Used for trail and grooming-route highlights. Linear-filtered so
	// cell edges feather naturally.
	cellOverlayTex uint32

	// Cached CPU-side terrain vertex array. Held so the snow-state
	// flush path can rewrite a few floats per vertex and re-upload
	// without rerunning the expensive AO/smoothY/jitter precompute.
	// Invalidated by anything that changes ground elevation (terrain
	// raise/lower, full rebuild on scene load).
	terrainVerts        []float32
	terrainSurfaceVerts int // number of leading vertices that hold surface (snow-bearing) cells; the rest are skirt walls/floor

	liftUpCables   map[uint64]*Mesh
	liftDownCables map[uint64]*Mesh

	// roadMesh is a single quad strip covering every road edge in the
	// world; roadLanesMesh is a parallel mesh of dashed centre-line
	// quads drawn over it in a second pass. Both regenerated wholesale
	// by RebuildRoads whenever the road graph changes.
	roadMesh      *Mesh
	roadLanesMesh *Mesh
	roadGhostMesh *Mesh

	ghostBatches   map[uint32]*Batch
	ghostUpCable   *Mesh
	ghostDownCable *Mesh
}

func newSceneResources() *SceneResources {
	return &SceneResources{
		liftUpCables:   make(map[uint64]*Mesh),
		liftDownCables: make(map[uint64]*Mesh),
		ghostBatches:   make(map[uint32]*Batch),
	}
}

// Delete releases every GPU resource owned by the scene.
func (s *SceneResources) Delete() {
	if s.terrainMesh != nil {
		s.terrainMesh.Delete()
		s.terrainMesh = nil
	}
	if s.snowSurfaceTex != 0 {
		gl.DeleteTextures(1, &s.snowSurfaceTex)
		s.snowSurfaceTex = 0
		s.snowSurfaceTexW = 0
		s.snowSurfaceTexH = 0
	}
	if s.cellOverlayTex != 0 {
		gl.DeleteTextures(1, &s.cellOverlayTex)
		s.cellOverlayTex = 0
	}
	for id, m := range s.liftUpCables {
		m.Delete()
		delete(s.liftUpCables, id)
	}
	for id, m := range s.liftDownCables {
		m.Delete()
		delete(s.liftDownCables, id)
	}
	for id, b := range s.ghostBatches {
		b.Delete()
		delete(s.ghostBatches, id)
	}
	if s.ghostUpCable != nil {
		s.ghostUpCable.Delete()
		s.ghostUpCable = nil
	}
	if s.ghostDownCable != nil {
		s.ghostDownCable.Delete()
		s.ghostDownCable = nil
	}
	if s.roadMesh != nil {
		s.roadMesh.Delete()
		s.roadMesh = nil
	}
	if s.roadLanesMesh != nil {
		s.roadLanesMesh.Delete()
		s.roadLanesMesh = nil
	}
	if s.roadGhostMesh != nil {
		s.roadGhostMesh.Delete()
		s.roadGhostMesh = nil
	}
}
