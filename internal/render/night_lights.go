//go:build !demo

package render

import (
	"math"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Night skiing's lights (world.LightSpots): the poles along lit runs,
// their lamps glowing while the lights are on, and the light they throw
// baked into a map over the ground that the terrain, static and dynamic
// shaders add after dark (lighting.glsl: nightLight). Any number of
// lights costs the same to draw. Not in the demo (night_lights_demo.go).

// nightLightTexUnit is the unit the light map binds to.
const nightLightTexUnit = 13

// nightLightTexel is the light map's texel, metres.
const nightLightTexel = 2.5

// A pole's pool of light, and a lift lamp's: how far it reaches and how
// bright it is at its foot (the map holds up to 2).
const (
	poleReach, poleBright = float32(40), float32(0.65)
	lampReach, lampBright = float32(24), float32(0.5)
)

var nightLightColor = mgl32.Vec3{1.0, 0.90, 0.74}

// nightLights is the Renderer's lights: the spots as last built, the
// light map, and the pole and lens batches.
type nightLights struct {
	nightKey             uint64
	nightSpots           []world.LightSpot
	nightTex             uint32
	nightW, nightH       int
	poleBatch, lensBatch *Batch
	poleInsts, lensInsts []DynamicInstance
}

func loadNightLights(r *Renderer, modelDir string) {
	mesh, tex := LoadOBJ(modelDir + "light_pole.obj")
	r.poleBatch = NewDynamicBatch(mesh, tex)
	r.lensBatch = NewDynamicBatch(NewBoxMesh(0.42, 0.10, 0.36), r.whiteTexID)
}

// nightLightsKey changes when the lit runs or lifts, or their shape, do.
func nightLightsKey(w *world.World) uint64 {
	var k uint64 = 1469598103934665603
	mix := func(v uint64) { k = (k ^ v) * 1099511628211 }
	for _, t := range w.Trails {
		if t.Lit {
			mix(t.ID)
			for _, n := range t.Nodes {
				mix(uint64(math.Float32bits(n.Pos[0])) ^ uint64(math.Float32bits(n.Pos[1]))<<20 ^ uint64(math.Float32bits(n.Width))<<40)
			}
		}
	}
	for _, l := range w.Lifts {
		if l.Lit {
			mix(l.ID ^ 0x9e3779b97f4a7c15)
		}
	}
	mix(uint64(w.Terrain.Width)<<32 | uint64(w.Terrain.Height))
	return k
}

// updateNightLights rebuilds the spots and the light map when what's lit
// has changed.
func (r *Renderer) updateNightLights(w *world.World) {
	if w.Terrain == nil {
		return
	}
	key := nightLightsKey(w)
	if key == r.nightKey && r.nightTex != 0 {
		return
	}
	r.nightKey = key
	r.nightSpots = w.LightSpots(w.Terrain)
	W := int(float32(w.Terrain.Width) * world.CellSize / nightLightTexel)
	H := int(float32(w.Terrain.Height) * world.CellSize / nightLightTexel)
	light := make([]float32, W*H)
	for _, sp := range r.nightSpots {
		reach, bright := lampReach, lampBright
		if sp.Pole {
			reach, bright = poleReach, poleBright
		}
		// Aimed across the run: the pool's centre sits a third of its
		// reach out from the pole.
		c := sp.Pos
		if sp.Pole {
			c = c.Add(mgl32.Vec2{float32(math.Sin(float64(sp.Heading))), float32(math.Cos(float64(sp.Heading)))}.Mul(reach / 3))
		}
		x0, x1 := int((c[0]-reach)/nightLightTexel), int((c[0]+reach)/nightLightTexel)
		z0, z1 := int((c[1]-reach)/nightLightTexel), int((c[1]+reach)/nightLightTexel)
		for z := max(z0, 0); z <= min(z1, H-1); z++ {
			for x := max(x0, 0); x <= min(x1, W-1); x++ {
				p := mgl32.Vec2{(float32(x) + 0.5) * nightLightTexel, (float32(z) + 0.5) * nightLightTexel}
				d := p.Sub(c).Len() / reach
				if d >= 1 {
					continue
				}
				f := 1 - d*d
				light[z*W+x] += bright * f * f
			}
		}
	}
	pix := make([]uint8, W*H*4)
	for i, v := range light {
		v = min(v, 2) / 2
		pix[i*4+0] = uint8(nightLightColor[0] * v * 255)
		pix[i*4+1] = uint8(nightLightColor[1] * v * 255)
		pix[i*4+2] = uint8(nightLightColor[2] * v * 255)
		pix[i*4+3] = 255
	}
	if r.nightTex == 0 {
		gl.GenTextures(1, &r.nightTex)
	}
	gl.ActiveTexture(gl.TEXTURE0 + nightLightTexUnit)
	gl.BindTexture(gl.TEXTURE_2D, r.nightTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(W), int32(H), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pix))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.ActiveTexture(gl.TEXTURE0)
	r.nightW, r.nightH = W, H
}

// bindNightLights binds the light map for the frame.
func (r *Renderer) bindNightLights() {
	if r.nightTex == 0 {
		return
	}
	gl.ActiveTexture(gl.TEXTURE0 + nightLightTexUnit)
	gl.BindTexture(gl.TEXTURE_2D, r.nightTex)
	gl.ActiveTexture(gl.TEXTURE0)
}

// applyNightLights sets a shader's light-map uniforms: on at night's
// strength while the lights are on.
func (r *Renderer) applyNightLights(sh *Shader, w *world.World, night float32) {
	on := float32(0)
	if w.LightsOn && r.nightTex != 0 && len(r.nightSpots) > 0 {
		on = night
	}
	sh.SetInt("uNightLight", nightLightTexUnit)
	sh.SetFloat("uNightLightOn", on)
	sh.SetVec2("uNightLightWorld", mgl32.Vec2{float32(r.nightW) * nightLightTexel, float32(r.nightH) * nightLightTexel})
}

// drawLightPoles draws the poles, and their lamps' lenses glowing while
// the lights are on (dark grey while they're off).
func (r *Renderer) drawLightPoles(w *world.World, night float32) {
	r.poleInsts, r.lensInsts = r.poleInsts[:0], r.lensInsts[:0]
	if r.poleBatch == nil || len(r.nightSpots) == 0 {
		return
	}
	lens := [3]float32{0.25, 0.25, 0.27}
	if w.LightsOn && night > 0 {
		lens = [3]float32{1, 0.95, 0.85}
	}
	for _, sp := range r.nightSpots {
		ground := w.Terrain.InterpolatedGroundElevationAt(sp.Pos[0], sp.Pos[1])
		s, c := float32(math.Sin(float64(sp.Heading))), float32(math.Cos(float64(sp.Heading)))
		if sp.Pole {
			r.poleInsts = append(r.poleInsts, DynamicInstance{Position: [3]float32{sp.Pos[0], ground, sp.Pos[1]}, Heading: sp.Heading, Color: [3]float32{1, 1, 1}})
			// The two lamps at the arm's end (light_pole.py).
			side := mgl32.Vec2{-c, s}
			for _, y := range [2]float32{-0.32, 0.32} {
				p := sp.Pos.Add(mgl32.Vec2{s, c}.Mul(0.9)).Add(side.Mul(y))
				r.lensInsts = append(r.lensInsts, DynamicInstance{Position: [3]float32{p[0], ground + sp.Height - 0.45, p[1]}, Heading: sp.Heading, Color: lens})
			}
			continue
		}
		// A lift lamp, on its tower or station.
		r.lensInsts = append(r.lensInsts, DynamicInstance{Position: [3]float32{sp.Pos[0], ground + sp.Height + 1.2, sp.Pos[1]}, Heading: sp.Heading, Color: lens})
	}
	r.DynamicShader.Use()
	r.poleBatch.SetDynamic(r.poleInsts)
	r.poleBatch.Draw()
	r.DynamicShader.SetFloat("uEmissive", 1)
	r.lensBatch.SetDynamic(r.lensInsts)
	r.lensBatch.Draw()
	r.DynamicShader.SetFloat("uEmissive", 0)
}
