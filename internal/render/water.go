package render

import (
	"math"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Water surfaces: open water is a flat, see-through sheet at the water's
// level over the material map's open-water samples, drawn after the
// solid scene, so a lake or creek sits in its hollow rather than being a
// colour painted on its floor. A lake's sheet is at the lake's level; a
// creek's at its bed (the ground along its line) plus creekWaterDepth,
// flat across the channel and never rising downstream, its edges tucked
// into the banks. Ice and snow-bridged water are the terrain's.

const (
	creekWaterDepth = 0.35 // metres of water over a creek's bed
	lakeWaterLift   = 0.08 // metres a lake's sheet sits over its levelled floor
	creekReach      = 3.5  // metres either side of a creek's line its sheet covers
)

// waterSurface is the uploaded water sheet and what it was built from.
type waterSurface struct {
	buf      vertBuffer
	ready    bool
	version  int  // the material map's Version it was built for
	dirty    bool // the ground changed since
	material *world.TerrainMaterial
	builtAt  time.Time
}

// waterRebuildGap is the least time between rebuilds for a ground change,
// which a brush makes every frame.
const waterRebuildGap = 500 * time.Millisecond

// MarkWaterDirty has the water rebuilt on the next frame, after the
// ground under it changed.
func (r *Renderer) MarkWaterDirty() { r.scene.water.dirty = true }

// syncWater rebuilds the water sheet when the material map or the ground
// has changed.
func (r *Renderer) syncWater(w *world.World) {
	ws := &r.scene.water
	m := w.Terrain.Material
	if !ws.ready {
		ws.buf.init()
		ws.ready = true
		ws.dirty = true
	}
	if m == nil {
		if ws.material != nil {
			ws.buf.set(nil)
			ws.material = nil
		}
		return
	}
	changed := ws.material != m || ws.version != m.Version
	if !changed && (!ws.dirty || time.Since(ws.builtAt) < waterRebuildGap) {
		return
	}
	ws.dirty, ws.material, ws.version, ws.builtAt = false, m, m.Version, time.Now()
	ws.buf.set(buildWater(w))
}

// buildWater is the water sheet's triangles: pos (xyz) and (alpha, depth,
// 0) per vertex.
func buildWater(w *world.World) []float32 {
	t, m := w.Terrain, w.Terrain.Material
	const per = world.CellSize / world.DetailPerCell
	W, H := m.W, m.H
	nan := float32(math.NaN())
	level := make([]float32, W*H)
	depth := make([]float32, W*H)
	water := make([]bool, W*H)
	for k := range level {
		level[k] = nan
	}
	ground := func(x, z float32) float32 {
		return t.MeshGroundAt(x/world.CellSize, z/world.CellSize) + t.DetailAt(x, z)
	}
	// Lakes: at their level.
	any := false
	for k, v := range m.M {
		if v != world.MatOpenWater {
			continue
		}
		any = true
		i, j := k%W, k/W
		if id := t.LakeNear(i, j); id != 0 && int(id) <= len(w.Lakes) {
			x, z := float32(i)*per, float32(j)*per
			level[k] = w.Lakes[id-1].Altitude - w.BaseAltitude + lakeWaterLift
			depth[k] = t.LakeDepthAt(x, z)
			water[k] = true
		}
	}
	if !any {
		return nil
	}
	// Creeks: their open samples take the level of the nearest point of
	// their line, walked a metre at a time from the source with the bed
	// never rising.
	if w.Water != nil {
		best := make([]float32, W*H)
		reach := int(math.Ceil(creekReach / per))
		for _, c := range w.Water.Creeks {
			y := float32(math.Inf(1))
			course := c.Curve()
			for s := 1; s < len(course); s++ {
				a, b := course[s-1], course[s]
				l := float32(math.Hypot(float64(b[0]-a[0]), float64(b[1]-a[1])))
				steps := max(int(l), 1)
				for st := 0; st <= steps; st++ {
					u := float32(st) / float32(steps)
					px, pz := a[0]+(b[0]-a[0])*u, a[1]+(b[1]-a[1])*u
					y = min(y, ground(px, pz)+creekWaterDepth)
					ci, cj := int(px/per+0.5), int(pz/per+0.5)
					for dj := -reach; dj <= reach; dj++ {
						for di := -reach; di <= reach; di++ {
							i, j := ci+di, cj+dj
							if i < 0 || j < 0 || i >= W || j >= H {
								continue
							}
							k := j*W + i
							if m.M[k] != world.MatOpenWater || (water[k] && best[k] == 0) {
								continue // not open, or a lake's
							}
							dx, dz := float32(i)*per-px, float32(j)*per-pz
							d := dx*dx + dz*dz + 1e-3
							if d > creekReach*creekReach || (best[k] > 0 && d >= best[k]) {
								continue
							}
							level[k], depth[k], water[k], best[k] = y, creekWaterDepth, true, d
						}
					}
				}
			}
		}
	}
	// The rim: samples next to water take a neighbour's level, faded out,
	// so the sheet runs on under the bank instead of stopping in a step.
	rim := make([]float32, W*H)
	copy(rim, level)
	for j := 0; j < H; j++ {
		for i := 0; i < W; i++ {
			k := j*W + i
			if water[k] {
				continue
			}
			for _, d := range [8][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
				ni, nj := i+d[0], j+d[1]
				if ni >= 0 && nj >= 0 && ni < W && nj < H && water[nj*W+ni] {
					rim[k] = level[nj*W+ni]
					depth[k] = 0
					break
				}
			}
		}
	}
	var verts []float32
	vert := func(i, j int) {
		k := j*W + i
		a := float32(0)
		if water[k] {
			a = 1
		}
		verts = append(verts, float32(i)*per, rim[k], float32(j)*per, a, depth[k], 0)
	}
	isSet := func(i, j int) bool { return rim[j*W+i] == rim[j*W+i] }
	for j := 0; j+1 < H; j++ {
		for i := 0; i+1 < W; i++ {
			if !isSet(i, j) || !isSet(i+1, j) || !isSet(i, j+1) || !isSet(i+1, j+1) {
				continue
			}
			if !water[j*W+i] && !water[j*W+i+1] && !water[(j+1)*W+i] && !water[(j+1)*W+i+1] {
				continue
			}
			vert(i, j)
			vert(i+1, j)
			vert(i+1, j+1)
			vert(i, j)
			vert(i+1, j+1)
			vert(i, j+1)
		}
	}
	return verts
}

// drawWater draws the water sheet over the solid scene: blended, tested
// against the depth but not writing it.
func (r *Renderer) drawWater(w *world.World, vp mgl32.Mat4, light Lighting, time float32) {
	if r.waterShader == nil || w == nil || w.Terrain == nil {
		return
	}
	r.syncWater(w)
	ws := &r.scene.water
	if ws.buf.n == 0 {
		return
	}
	s := r.waterShader
	s.Use()
	light.apply(s)
	r.applyHaze(s)
	s.SetMat4("uViewProj", vp)
	s.SetVec3("uViewDir", r.Camera.Target.Sub(r.Camera.WorldPos()).Normalize())
	s.SetFloat("uTime", time)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.DepthMask(false)
	ws.buf.draw()
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)
}
