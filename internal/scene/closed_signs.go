package scene

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// Closed signs: a red circle-slash floats over each closed lift's base
// station, at the top of each run closed for avalanche danger, and over
// each ticket window while the resort is closed, so a player who's built
// them sees what still has to be opened.

// closedSignRadius is the sign's size on screen, pixels.
const closedSignRadius = float32(13)

// drawClosedSigns draws the signs at their places on screen.
func (s *Scenario) drawClosedSigns(r *render.Renderer) {
	w := s.world
	t := w.Terrain
	sign := func(p mgl32.Vec3) {
		if x, y, ok := r.WorldToScreen(p); ok {
			ui.DrawClosedSign(r, x, y, closedSignRadius)
		}
	}
	for _, l := range w.Lifts {
		if l.InService(w) {
			continue
		}
		// Over the base station's roof (about 4.3 m).
		sign(mgl32.Vec3{l.Base[0], t.InterpolatedSurfaceElevationAt(l.Base[0], l.Base[1]) + 7, l.Base[1]})
	}
	// Runs closed for avalanche danger: a sign at the top.
	for _, tr := range w.Trails {
		if !tr.AvyClosed || len(tr.Nodes) == 0 {
			continue
		}
		p := tr.Nodes[0].Pos
		sign(mgl32.Vec3{p[0], t.InterpolatedSurfaceElevationAt(p[0], p[1]) + 3, p[1]})
	}
	if w.ResortOpen {
		return
	}
	for _, b := range w.Buildings {
		if !b.Offers(world.ServiceTickets) {
			continue
		}
		top := w.ShellFloorY(b) + b.Kind.WallHeight()*float32(b.Floors()) + 4
		for _, c := range b.Cells {
			if b.ServiceAt(c) == world.ServiceTickets {
				p := b.TileCentre(c)
				sign(mgl32.Vec3{p[0], top, p[1]})
				break
			}
		}
	}
}
