package scene

import "mountain-mogul/internal/render"

// uiDrawFunc adapts a bare function to the render.UIDrawable interface.
type uiDrawFunc func(*render.Renderer)

func (f uiDrawFunc) Draw(r *render.Renderer) { f(r) }

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// smoothstep32 is the standard Hermite ramp from 0 to 1 between edge0 and
// edge1. Inlined elsewhere in the codebase as raw arithmetic; named here
// for readability since we use it multiple ways.
func smoothstep32(edge0, edge1, x float32) float32 {
	if edge1 <= edge0 {
		if x < edge0 {
			return 0
		}
		return 1
	}
	t := (x - edge0) / (edge1 - edge0)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

// openLandBuyPopup shows the purchase popup for the parcel at terrain
// cell (gx, gz) (land_buy.go). Nil in the demo, which has no buying land.
var openLandBuyPopup func(s *Scenario, gx, gz, screenW, screenH int)
