//go:build !demo

package render

import (
	"math"

	"github.com/go-gl/gl/v4.1-core/gl"

	"mountain-mogul/internal/world"
)

// Snowmaking's guns along the runs that have it (world.Trail.SnowGuns),
// on the snow as it is each frame, and a see-through plume from each
// while its run is making snow. Not in the demo (snowmaking_demo.go).

// snowmakingRender is the Renderer's batches for them.
type snowmakingRender struct {
	snowGunBatch, snowPlumeBatch *Batch
	snowGunInsts, snowPlumeInsts []DynamicInstance
}

// plumeFade is how see-through a plume is.
const plumeFade = 0.62

func loadSnowmaking(r *Renderer, modelDir string) {
	mesh, tex := LoadOBJ(modelDir + "snow_gun.obj")
	r.snowGunBatch = NewDynamicBatch(mesh, tex)
	mesh, tex = LoadOBJ(modelDir + "snow_plume.obj")
	r.snowPlumeBatch = NewDynamicBatch(mesh, tex)
}

// drawSnowGuns draws the guns, solid, with the rest of the dynamic
// meshes, and gathers the plumes for drawSnowPlumes.
func drawSnowGuns(r *Renderer, w *world.World, time float32) {
	r.snowGunInsts, r.snowPlumeInsts = r.snowGunInsts[:0], r.snowPlumeInsts[:0]
	if r.snowGunBatch == nil {
		return
	}
	for _, tr := range w.Trails {
		for i, g := range tr.SnowGuns() {
			y := w.Terrain.InterpolatedSurfaceElevationAt(g.Pos[0], g.Pos[1])
			inst := DynamicInstance{Position: [3]float32{g.Pos[0], y, g.Pos[1]}, Heading: g.Heading, Color: [3]float32{1, 1, 1}}
			r.snowGunInsts = append(r.snowGunInsts, inst)
			if tr.MakingSnow {
				// The gun oscillates slowly across the run, as fan guns do.
				inst.Heading += 0.25 * float32(math.Sin(float64(time*0.4+float32(i)*1.7)))
				r.snowPlumeInsts = append(r.snowPlumeInsts, inst)
			}
		}
	}
	if len(r.snowGunInsts) > 0 {
		r.DynamicShader.Use()
		r.snowGunBatch.SetDynamic(r.snowGunInsts)
		r.snowGunBatch.Draw()
	}
}

// drawSnowPlumes draws this frame's plumes see-through, after everything
// solid.
func drawSnowPlumes(r *Renderer) {
	if len(r.snowPlumeInsts) == 0 {
		return
	}
	r.DynamicShader.Use()
	r.DynamicShader.SetFloat("uFade", plumeFade)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.DepthMask(false)
	r.snowPlumeBatch.SetDynamic(r.snowPlumeInsts)
	r.snowPlumeBatch.Draw()
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)
	r.DynamicShader.SetFloat("uFade", 0)
}
