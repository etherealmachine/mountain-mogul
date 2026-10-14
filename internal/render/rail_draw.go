package render

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Railways' meshes and their batches (rail_mesh.go): one per line, kept
// until the line changes, and a placement ghost.

type railBatch struct {
	key   [2]int // track samples and nodes it was built for
	mesh  *Mesh
	batch *Batch
}

type railDrawing struct {
	rails     map[uint64]*railBatch
	railGhost *railBatch
}

func (rb *railBatch) delete() {
	if rb == nil {
		return
	}
	if rb.batch != nil {
		rb.batch.Delete()
	}
	if rb.mesh != nil {
		rb.mesh.Delete()
	}
}

func newRailBatch(r *Renderer, l *world.RailLine, t *world.Terrain, tint [3]float32) *railBatch {
	m := railLineMesh(l, t, tint)
	if m == nil {
		return nil
	}
	b := NewStaticBatch(m, r.whiteTexID)
	b.AddStatic(mgl32.Ident4(), mgl32.Vec3{1, 1, 1})
	return &railBatch{key: [2]int{len(l.Track), len(l.Nodes)}, mesh: m, batch: b}
}

// RebuildRails brings the railway meshes up to date with the world's
// lines.
func (r *Renderer) RebuildRails(w *world.World) {
	if r.scene.rails == nil {
		r.scene.rails = map[uint64]*railBatch{}
	}
	live := map[uint64]bool{}
	for _, l := range w.RailLines {
		live[l.ID] = true
		rb := r.scene.rails[l.ID]
		if rb != nil && rb.key == [2]int{len(l.Track), len(l.Nodes)} {
			continue
		}
		rb.delete()
		r.scene.rails[l.ID] = newRailBatch(r, l, w.Terrain, [3]float32{1, 1, 1})
	}
	for id, rb := range r.scene.rails {
		if !live[id] {
			rb.delete()
			delete(r.scene.rails, id)
		}
	}
}

// SetRailGhost shows line l (nil for none) as a placement preview,
// tinted.
func (r *Renderer) SetRailGhost(l *world.RailLine, t *world.Terrain, tint [3]float32) {
	r.scene.railGhost.delete()
	r.scene.railGhost = nil
	if l != nil && len(l.Track) >= 2 {
		r.scene.railGhost = newRailBatch(r, l, t, tint)
	}
}

// drawRails draws the railways with the static shader (bound by the
// caller).
func (r *Renderer) drawRails() {
	for _, rb := range r.scene.rails {
		if rb != nil {
			rb.batch.Draw()
		}
	}
	if rb := r.scene.railGhost; rb != nil {
		rb.batch.Draw()
	}
}
