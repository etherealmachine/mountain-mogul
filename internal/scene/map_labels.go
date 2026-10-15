package scene

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// peakLabel is a peak's label: its name and height above sea level, as
// "Castle Peak 2,775 m". ele is the label's height; with none, alt is
// used (the ground's), and with neither just the name.
func peakLabel(name string, ele, alt float32) string {
	if ele <= 0 {
		ele = alt
	}
	if ele <= 0 {
		return name
	}
	m := int(ele + 0.5)
	if m >= 1000 {
		return fmt.Sprintf("%s %d,%03d m", name, m/1000, m%1000)
	}
	return fmt.Sprintf("%s %d m", name, m)
}

// mapLabels draws the world's labels (World.Labels), each by its kind
// (a peak: a small summit mark on the top, and the name and height above
// it), under the game's windows and bars. The editor passes the
// selected label, drawn highlighted.
type mapLabels struct {
	world    *world.World
	selected uint64
}

var (
	labelMarkCol   = mgl32.Vec4{0.97, 0.97, 1, 0.95}
	labelTextCol   = mgl32.Vec4{0.97, 0.97, 1, 0.95}
	labelShadow    = mgl32.Vec4{0.02, 0.03, 0.06, 0.75}
	labelSelectCol = mgl32.Vec4{1, 0.85, 0.2, 1}
)

// labelLayout is where a label draws on screen: its anchor (the summit),
// its text and the text's top left, and the box a click picks it by.
type labelLayout struct {
	sx, sy     float32
	text       string
	tx, ty     float32
	x0, y0, x1 float32
	y1         float32
}

// layoutLabel places l on screen; ok false when it's off screen.
func layoutLabel(r *render.Renderer, w *world.World, l *world.Label) (labelLayout, bool) {
	t := w.Terrain
	y := t.InterpolatedSurfaceElevationAt(l.Pos[0], l.Pos[1])
	sx, sy, vis := r.WorldToScreen(mgl32.Vec3{l.Pos[0], y + 2, l.Pos[1]})
	if !vis || r.Font == nil {
		return labelLayout{}, false
	}
	text := l.Name
	if l.Kind == world.LabelPeak {
		text = peakLabel(l.Name, l.Ele, y+w.BaseAltitude)
	}
	gh := float32(render.GlyphH)
	tw := r.Font.TextWidth(text)
	tx, ty := sx-tw/2, sy-14-gh
	return labelLayout{sx: sx, sy: sy, text: text, tx: tx, ty: ty,
		x0: min(tx, sx-8) - 3, y0: ty - 3, x1: max(tx+tw, sx+8) + 3, y1: sy + 3}, true
}

func (ml *mapLabels) Draw(r *render.Renderer) {
	w := ml.world
	if r.Font == nil || w == nil || len(w.Labels) == 0 {
		return
	}
	for _, l := range w.Labels {
		lay, ok := layoutLabel(r, w, l)
		if !ok {
			continue
		}
		text := labelTextCol
		if l.ID == ml.selected {
			r.DrawRoundRectOutline(lay.x0, lay.y0, lay.x1-lay.x0, lay.y1-lay.y0, 4, 2, labelSelectCol)
			text = labelSelectCol
		}
		sx, sy := lay.sx, lay.sy
		// The summit mark: a small triangle, its foot on the top.
		r.DrawColorTriangle(sx-7, sy+1, sx+7, sy+1, sx, sy-11, labelShadow)
		r.DrawColorTriangle(sx-6, sy, sx+6, sy, sx, sy-10, labelMarkCol)
		r.Font.DrawText(r, lay.text, lay.tx+1, lay.ty+1, labelShadow)
		r.Font.DrawText(r, lay.text, lay.tx, lay.ty, text)
	}
}

// labelAt is the label whose drawn box holds screen point (mx, my), the
// last drawn (on top) first; nil for none.
func labelAt(r *render.Renderer, w *world.World, mx, my float32) *world.Label {
	for i := len(w.Labels) - 1; i >= 0; i-- {
		l := w.Labels[i]
		if lay, ok := layoutLabel(r, w, l); ok && mx >= lay.x0 && mx <= lay.x1 && my >= lay.y0 && my <= lay.y1 {
			return l
		}
	}
	return nil
}
