//go:build demo

package render

import "mountain-mogul/internal/world"

// The demo sells no lights (night_lights.go).

type nightLights struct{}

func loadNightLights(*Renderer, string)            {}
func (r *Renderer) updateNightLights(*world.World) {}
func (r *Renderer) bindNightLights()               {}
func (r *Renderer) applyNightLights(sh *Shader, _ *world.World, _ float32) {
	sh.SetFloat("uNightLightOn", 0)
}
func (r *Renderer) drawLightPoles(*world.World, float32) {}
