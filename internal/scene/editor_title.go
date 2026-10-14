//go:build !demo

package scene

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/ui"
)

// titlePanel is the editor's top-left pod: the open scenario's name, with
// a "*" while there are unsaved edits. Clicking it opens the scenario
// details.
type titlePanel struct {
	name       func() string
	onClick    func()
	hovered    bool
	x, y, w, h float32 // background rect, from the last draw
}

const (
	titlePanelInset = float32(6)  // gap round the pod, as the top bar's
	titlePanelPad   = float32(12) // either side of the name
)

func newTitlePanel(name func() string, onClick func()) *titlePanel {
	return &titlePanel{name: name, onClick: onClick, x: titlePanelInset, y: titlePanelInset}
}

// layout sizes the pod to the name, measured with r's font; it keeps the
// last size without one. Its height matches the start date strip's.
func (p *titlePanel) layout(r *render.Renderer) {
	p.h = float32(render.GlyphH) + 6 + 2*6
	if r != nil && r.Font != nil {
		p.w = r.Font.TextWidth(p.name()) + 2*titlePanelPad
	}
}

// Contains reports whether the point is over the pod.
func (p *titlePanel) Contains(mx, my float32) bool {
	return mx >= p.x && mx <= p.x+p.w && my >= p.y && my <= p.y+p.h
}

// HandleInput hovers the pod and opens the details on a click, consuming
// it so it doesn't reach the world tools.
func (p *titlePanel) HandleInput(inp *engine.Input) {
	mx, my := inp.MousePos[0], inp.MousePos[1]
	p.hovered = p.Contains(mx, my)
	if inp.LeftClick && !inp.LeftClickConsumed && p.hovered {
		inp.LeftClickConsumed = true
		p.onClick()
	}
}

func (p *titlePanel) Draw(r *render.Renderer) {
	p.layout(r)
	fill := ui.PanelFill
	if p.hovered {
		fill = ui.PanelHeader
	}
	ui.DrawPanelColor(r, p.x, p.y, p.w, p.h, fill)
	if r.Font != nil {
		r.Font.DrawText(r, p.name(), p.x+titlePanelPad, p.y+(p.h-float32(render.GlyphH))/2, mgl32.Vec4{1, 1, 1, 1})
	}
	if p.hovered {
		ui.DrawTooltip(r, "Scenario details", p.x+p.w/2, p.y+p.h, true)
	}
}
