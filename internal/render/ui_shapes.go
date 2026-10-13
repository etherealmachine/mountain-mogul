package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Rounded UI shapes. Each is built from triangles that never overlap, so a
// translucent fill blends once everywhere and batches with rects and glyphs.

// Corner flags pick which corners of a rounded rect are rounded.
const (
	CornerTL uint8 = 1 << iota
	CornerTR
	CornerBR
	CornerBL
	CornersAll    = CornerTL | CornerTR | CornerBR | CornerBL
	CornersTop    = CornerTL | CornerTR
	CornersBottom = CornerBL | CornerBR
)

// cornerSegments is how many triangles round each corner.
const cornerSegments = 8

// DrawRoundRect fills a rect with every corner rounded by radius.
func (r *Renderer) DrawRoundRect(x, y, w, h, radius float32, color mgl32.Vec4) {
	r.DrawRoundRectCorners(x, y, w, h, radius, CornersAll, color)
}

// DrawRoundRectCorners fills a rect with the corners in mask rounded by
// radius and the rest square.
func (r *Renderer) DrawRoundRectCorners(x, y, w, h, radius float32, mask uint8, color mgl32.Vec4) {
	radius = clampRadius(radius, w, h)
	if radius <= 0.5 || mask == 0 {
		r.DrawColorRect(x, y, w, h, color)
		return
	}
	r.useUITexture(r.whiteTexID)
	rad := radius
	// Middle column, full height.
	r.appendUIQuad(x+rad, y, w-2*rad, h, 0, 0, 1, 1, color)
	// Side columns between the corners.
	r.appendUIQuad(x, y+rad, rad, h-2*rad, 0, 0, 1, 1, color)
	r.appendUIQuad(x+w-rad, y+rad, rad, h-2*rad, 0, 0, 1, 1, color)
	// Corners: a quarter fan where rounded, a square where not.
	corners := [4]struct {
		bit    uint8
		cx, cy float32 // arc centre
		a0     float64 // start angle (screen y down)
		sx, sy float32 // square corner origin
	}{
		{CornerTL, x + rad, y + rad, math.Pi, x, y},
		{CornerTR, x + w - rad, y + rad, 1.5 * math.Pi, x + w - rad, y},
		{CornerBR, x + w - rad, y + h - rad, 0, x + w - rad, y + h - rad},
		{CornerBL, x + rad, y + h - rad, 0.5 * math.Pi, x, y + h - rad},
	}
	for _, c := range corners {
		if mask&c.bit == 0 {
			r.appendUIQuad(c.sx, c.sy, rad, rad, 0, 0, 1, 1, color)
			continue
		}
		r.appendArcFan(c.cx, c.cy, rad, c.a0, color)
	}
}

// appendArcFan adds a quarter-circle fan from angle a0 to a0+π/2.
func (r *Renderer) appendArcFan(cx, cy, rad float32, a0 float64, color mgl32.Vec4) {
	px := cx + rad*float32(math.Cos(a0))
	py := cy + rad*float32(math.Sin(a0))
	for i := 1; i <= cornerSegments; i++ {
		a := a0 + float64(i)/cornerSegments*math.Pi/2
		nx := cx + rad*float32(math.Cos(a))
		ny := cy + rad*float32(math.Sin(a))
		r.appendUITriangle(cx, cy, px, py, nx, ny, color)
		px, py = nx, ny
	}
}

// DrawRoundRectOutline strokes a rounded rect's edge, thickness pixels
// wide, inside its bounds.
func (r *Renderer) DrawRoundRectOutline(x, y, w, h, radius, thickness float32, color mgl32.Vec4) {
	r.DrawRoundRectOutlineCorners(x, y, w, h, radius, thickness, CornersAll, color)
}

// DrawRoundRectOutlineCorners strokes a rect with the corners in mask
// rounded.
func (r *Renderer) DrawRoundRectOutlineCorners(x, y, w, h, radius, thickness float32, mask uint8, color mgl32.Vec4) {
	radius = clampRadius(radius, w, h)
	t := thickness
	if radius <= t || mask == 0 {
		r.DrawColorRect(x, y, w, t, color)
		r.DrawColorRect(x, y+h-t, w, t, color)
		r.DrawColorRect(x, y+t, t, h-2*t, color)
		r.DrawColorRect(x+w-t, y+t, t, h-2*t, color)
		return
	}
	r.useUITexture(r.whiteTexID)
	rad := radius
	// Straight edges between the corner squares.
	r.appendUIQuad(x+rad, y, w-2*rad, t, 0, 0, 1, 1, color)
	r.appendUIQuad(x+rad, y+h-t, w-2*rad, t, 0, 0, 1, 1, color)
	r.appendUIQuad(x, y+rad, t, h-2*rad, 0, 0, 1, 1, color)
	r.appendUIQuad(x+w-t, y+rad, t, h-2*rad, 0, 0, 1, 1, color)
	corners := [4]struct {
		bit    uint8
		cx, cy float32
		a0     float64
		sx, sy float32 // square corner origin
		hx, vy float32 // where the square corner's two strokes start
	}{
		{CornerTL, x + rad, y + rad, math.Pi, x, y, x, y},
		{CornerTR, x + w - rad, y + rad, 1.5 * math.Pi, x + w - rad, y, x + w - t, y},
		{CornerBR, x + w - rad, y + h - rad, 0, x + w - rad, y + h - rad, x + w - t, y + h - t},
		{CornerBL, x + rad, y + h - rad, 0.5 * math.Pi, x, y + h - rad, x, y + h - t},
	}
	for _, c := range corners {
		if mask&c.bit == 0 {
			// Square corner: the two strokes run on to the corner.
			r.appendUIQuad(c.sx, c.vy, rad, t, 0, 0, 1, 1, color)
			if c.vy == y {
				r.appendUIQuad(c.hx, y+t, t, rad-t, 0, 0, 1, 1, color)
			} else {
				r.appendUIQuad(c.hx, y+h-rad, t, rad-t, 0, 0, 1, 1, color)
			}
			continue
		}
		r.appendArcBand(c.cx, c.cy, rad-t, rad, c.a0, math.Pi/2, cornerSegments, color)
	}
}

// DrawRing strokes a circle centred at (cx, cy), thickness pixels wide
// inside radius.
func (r *Renderer) DrawRing(cx, cy, radius, thickness float32, color mgl32.Vec4) {
	r.useUITexture(r.whiteTexID)
	r.appendArcBand(cx, cy, radius-thickness, radius, 0, 2*math.Pi, 32, color)
}

// appendArcBand adds the band between radii r0 and r1 from angle a0
// through sweep, in n segments.
func (r *Renderer) appendArcBand(cx, cy, r0, r1 float32, a0, sweep float64, n int, color mgl32.Vec4) {
	c0, s0 := float32(math.Cos(a0)), float32(math.Sin(a0))
	for i := 1; i <= n; i++ {
		a := a0 + float64(i)/float64(n)*sweep
		c1, s1 := float32(math.Cos(a)), float32(math.Sin(a))
		ix0, iy0 := cx+r0*c0, cy+r0*s0
		ox0, oy0 := cx+r1*c0, cy+r1*s0
		ix1, iy1 := cx+r0*c1, cy+r0*s1
		ox1, oy1 := cx+r1*c1, cy+r1*s1
		r.appendUITriangle(ix0, iy0, ox0, oy0, ox1, oy1, color)
		r.appendUITriangle(ix0, iy0, ox1, oy1, ix1, iy1, color)
		c0, s0 = c1, s1
	}
}

// clampRadius keeps a corner radius within half the rect's shorter side.
func clampRadius(radius, w, h float32) float32 {
	return max(0, min(radius, w/2, h/2))
}

// DrawStar fills a five-pointed star centred at (cx, cy) with outer
// radius radius (the font has no ★).
func (r *Renderer) DrawStar(cx, cy, radius float32, color mgl32.Vec4) {
	r.useUITexture(r.whiteTexID)
	inner := radius * 0.42
	pt := func(i int) (float32, float32) {
		rad := radius
		if i%2 == 1 {
			rad = inner
		}
		a := -math.Pi/2 + float64(i)*math.Pi/5
		return cx + rad*float32(math.Cos(a)), cy + rad*float32(math.Sin(a))
	}
	for i := range 10 {
		x0, y0 := pt(i)
		x1, y1 := pt(i + 1)
		r.appendUITriangle(cx, cy, x0, y0, x1, y1, color)
	}
}

// DrawColorTriangle fills one screen-space triangle.
func (r *Renderer) DrawColorTriangle(x0, y0, x1, y1, x2, y2 float32, color mgl32.Vec4) {
	r.useUITexture(r.whiteTexID)
	r.appendUITriangle(x0, y0, x1, y1, x2, y2, color)
}
