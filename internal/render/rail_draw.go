package render

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Railways' meshes and their batches (rail_mesh.go): one per track and
// one of the stations' platforms, rebuilt when the railway changes
// (World.RailRev), and a placement ghost.

type railBatch struct {
	mesh  *Mesh
	batch *Batch
	// track is the funicular track it was built for (funicular.go).
	track *world.RailLine
}

type railDrawing struct {
	rails     map[uint64]*railBatch
	railGhost *railBatch
	// stations is the platforms; railRev the World.RailRev they and
	// rails were built for (railBuilt once they have been).
	stations  *railBatch
	railRev   int
	railBuilt bool
	// funics is the funiculars' tracks, by lift, and funicGhost one being
	// placed.
	funics     map[uint64]*railBatch
	funicGhost *railBatch
	// tunnelCourse is the dashes over the roads' and paths' tunnels.
	tunnelCourse *railBatch
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

func newRailBatch(r *Renderer, l *world.RailLine, wired bool, t *world.Terrain, tint [3]float32) *railBatch {
	return meshBatch(r, railLineMesh(l, wired, t, tint))
}

// meshBatch is a static batch of mesh m drawn once where it is; nil for
// no mesh.
func meshBatch(r *Renderer, m *Mesh) *railBatch {
	if m == nil {
		return nil
	}
	b := NewStaticBatch(m, r.whiteTexID)
	b.AddStatic(mgl32.Ident4(), mgl32.Vec3{1, 1, 1})
	return &railBatch{mesh: m, batch: b}
}

// RebuildRails brings the railway meshes up to date with the world's
// track and stations.
func (r *Renderer) RebuildRails(w *world.World) {
	if !r.scene.railBuilt || r.scene.railRev != w.RailRev {
		for _, rb := range r.scene.rails {
			rb.delete()
		}
		r.scene.rails = map[uint64]*railBatch{}
		for _, tr := range w.RailTracks {
			r.scene.rails[tr.ID] = newRailBatch(r, &tr.RailLine, tr.Style.Spec().Wired, w.Terrain, [3]float32{1, 1, 1})
		}
		r.scene.stations.delete()
		r.scene.stations = meshBatch(r, stationsMesh(w))
		r.scene.railRev, r.scene.railBuilt = w.RailRev, true
	}
	r.rebuildFunics(w)
	r.scene.tunnelCourse.delete()
	r.scene.tunnelCourse = nil
	if m := tunnelCourseMesh(w.Tunnels, w.Terrain); m != nil {
		b := NewStaticBatch(m, r.whiteTexID)
		b.AddStatic(mgl32.Ident4(), mgl32.Vec3{1, 1, 1})
		r.scene.tunnelCourse = &railBatch{mesh: m, batch: b}
	}
}

// rebuildFunics brings the funiculars' track meshes up to date: rebuilt
// when a lift's track is laid out afresh.
func (r *Renderer) rebuildFunics(w *world.World) {
	if r.scene.funics == nil {
		r.scene.funics = map[uint64]*railBatch{}
	}
	live := map[uint64]bool{}
	for _, lift := range w.Lifts {
		tr := lift.FunicularTrack(w.Terrain)
		if tr == nil {
			continue
		}
		live[lift.ID] = true
		if rb := r.scene.funics[lift.ID]; rb != nil && rb.track == tr {
			continue
		}
		r.scene.funics[lift.ID].delete()
		rb := newFunicBatch(r, lift, w.Terrain, [3]float32{1, 1, 1})
		if rb != nil {
			rb.track = tr
		}
		r.scene.funics[lift.ID] = rb
	}
	for id, rb := range r.scene.funics {
		if !live[id] {
			rb.delete()
			delete(r.scene.funics, id)
		}
	}
}

// setFunicGhost shows funicular lift (nil for none) as a placement
// preview, tinted.
func (r *Renderer) setFunicGhost(lift *world.Lift, t *world.Terrain, tint [3]float32) {
	r.scene.funicGhost.delete()
	r.scene.funicGhost = nil
	if lift != nil {
		r.scene.funicGhost = newFunicBatch(r, lift, t, tint)
	}
}

// SetRailGhost shows track tr (nil for none) as a placement preview,
// tinted.
func (r *Renderer) SetRailGhost(tr *world.RailTrack, t *world.Terrain, tint [3]float32) {
	r.scene.railGhost.delete()
	r.scene.railGhost = nil
	if tr != nil && len(tr.Track) >= 2 {
		r.scene.railGhost = newRailBatch(r, &tr.RailLine, tr.Style.Spec().Wired, t, tint)
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
	if rb := r.scene.stations; rb != nil {
		rb.batch.Draw()
	}
	for _, rb := range r.scene.funics {
		if rb != nil {
			rb.batch.Draw()
		}
	}
	if rb := r.scene.funicGhost; rb != nil {
		rb.batch.Draw()
	}
	if rb := r.scene.tunnelCourse; rb != nil {
		rb.batch.Draw()
	}
}
