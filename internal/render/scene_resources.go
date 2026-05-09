package render

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

	liftUpCables   map[uint64]*Mesh
	liftDownCables map[uint64]*Mesh

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
}
