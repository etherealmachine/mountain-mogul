package world

import (
	"image"
	"math"
)

// GroomMap records where snowcats actually groomed, at 1 m per pixel, so
// corduroy follows the cat's path instead of the 5 m cells. RGBA16:
//
//	R — groomed: full inside a swath, soft over a metre at its edge, so
//	    where two lanes overlap it dips a little and the shader draws a
//	    seam there
//	G — cos φ and
//	B — sin φ of the lateral coordinate: φ = 2π·lat / GroomLatPeriod, where
//	    lat is metres across the lanes of a run. Neighbouring lanes share
//	    one coordinate, so corduroy ridges (drawn at a fixed spacing in
//	    lat) carry across seams and follow every curve of the lanes.
//	    Stored as an angle so it blends between pixels without a wrap.
//	A — which lane owns the pixel: the night it was groomed (low 6 bits
//	    of GroomMap.Night) over 10 bits of depth inside the lane; 0 for
//	    stamps not laid by a cat
//
// Per-cell Grooming stays the gameplay value; the shader multiplies this
// stamp by it, so skier wear fades the corduroy without touching the map.
// Saved with the game; rebuilt from cell grooming for saves without it.
type GroomMap struct {
	W, H     int // pixels, one per metre
	Pixels   []uint16
	Dirty    bool
	DirtyBox image.Rectangle

	// Night tags tonight's cat stamps. A pass replaces anything groomed
	// on an earlier night outright, and among tonight's lanes a pixel
	// goes to whichever centreline is nearest.
	Night uint16
}

const groomDepthBits = 10

// GroomPxPerCell is the groom map resolution: one pixel per metre.
const GroomPxPerCell = 5

// GroomLatPeriod is the lateral distance, in metres, over which φ turns
// once. Ridge spacing must divide it.
const GroomLatPeriod = 40.0

// groomOverlap widens each stamped swath past the tiller, as crews
// overlap passes, so diverging lanes and lane ends leave no slivers.
const groomOverlap = 0.5

// groomEdgeSoft is the width of the soft swath edge, centred on the
// tiller's edge.
const groomEdgeSoft = 1.0

// NewGroomMap allocates an empty map for a terrain of wCells × hCells.
func NewGroomMap(wCells, hCells int) *GroomMap {
	w, h := wCells*GroomPxPerCell, hCells*GroomPxPerCell
	return &GroomMap{W: w, H: h, Pixels: make([]uint16, w*h*4)}
}

func (g *GroomMap) markDirty(r image.Rectangle) {
	r = r.Intersect(image.Rect(0, 0, g.W, g.H))
	if r.Empty() {
		return
	}
	if !g.Dirty {
		g.DirtyBox, g.Dirty = r, true
		return
	}
	g.DirtyBox = g.DirtyBox.Union(r)
}

func unorm16(v float32) uint16 { return uint16(min(max(v, 0), 1)*65535 + 0.5) }

// latChannels encodes a lateral coordinate as cos φ, sin φ.
func latChannels(lat float32) (uint16, uint16) {
	phi := 2 * math.Pi * float64(lat) / GroomLatPeriod
	return unorm16(float32(math.Cos(phi))*0.5 + 0.5), unorm16(float32(math.Sin(phi))*0.5 + 0.5)
}

// StampSegment grooms a swath about SnowcatTillerWidth wide along the segment
// (x0, z0)→(x1, z1) in world metres. The lateral coordinate runs from lat0
// to lat1 along the centreline and grows by sign per metre toward the
// segment's left.
// Where tonight's lanes overlap, the pixel keeps whichever lane it's
// deeper inside.
func (g *GroomMap) StampSegment(x0, z0, x1, z1, lat0, lat1, sign float32) {
	if g == nil {
		return
	}
	dx, dz := x1-x0, z1-z0
	l2 := dx*dx + dz*dz
	hw := float32(SnowcatTillerWidth/2 + groomOverlap)
	reach := hw + groomEdgeSoft/2
	box := image.Rect(
		int(math.Floor(float64(min(x0, x1)-reach))), int(math.Floor(float64(min(z0, z1)-reach))),
		int(math.Ceil(float64(max(x0, x1)+reach))), int(math.Ceil(float64(max(z0, z1)+reach))),
	).Intersect(image.Rect(0, 0, g.W, g.H))
	if box.Empty() {
		return
	}
	const depthMask = 1<<groomDepthBits - 1
	night := (g.Night & 63) << groomDepthBits
	for pz := box.Min.Y; pz < box.Max.Y; pz++ {
		for px := box.Min.X; px < box.Max.X; px++ {
			cx, cz := float32(px)+0.5, float32(pz)+0.5
			t := float32(0)
			if l2 > 0 {
				t = min(max(((cx-x0)*dx+(cz-z0)*dz)/l2, 0), 1)
			}
			ex, ez := cx-(x0+dx*t), cz-(z0+dz*t)
			d := float32(math.Sqrt(float64(ex*ex + ez*ez)))
			if d >= reach {
				continue
			}
			own := night | uint16((reach-d)/reach*depthMask)
			off := (pz*g.W + px) * 4
			if old := g.Pixels[off+3]; old&^depthMask == night && own <= old {
				continue
			}
			// Signed distance to the lane, so the coordinate stays smooth
			// where segments meet on a bend.
			side := d
			if dx*ez-dz*ex < 0 {
				side = -d
			}
			g.Pixels[off] = unorm16((reach - d) / groomEdgeSoft)
			g.Pixels[off+1], g.Pixels[off+2] = latChannels(lat0 + (lat1-lat0)*t + sign*side)
			g.Pixels[off+3] = own
		}
	}
	g.markDirty(box)
}

// StampCell grooms one whole cell with lanes heading (dx, dz). Used where
// grooming is set without a cat: lift aprons, testbeds, and old saves.
func (g *GroomMap) StampCell(cx, cz int, dx, dz float32) {
	if g == nil {
		return
	}
	l := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if l == 0 {
		dx, dz, l = 0, 1, 1
	}
	box := image.Rect(cx*GroomPxPerCell, cz*GroomPxPerCell, (cx+1)*GroomPxPerCell, (cz+1)*GroomPxPerCell).
		Intersect(image.Rect(0, 0, g.W, g.H))
	for pz := box.Min.Y; pz < box.Max.Y; pz++ {
		for px := box.Min.X; px < box.Max.X; px++ {
			x, z := float32(px)+0.5, float32(pz)+0.5
			off := (pz*g.W + px) * 4
			g.Pixels[off] = 65535
			g.Pixels[off+1], g.Pixels[off+2] = latChannels((dx*z - dz*x) / l)
			g.Pixels[off+3] = 0
		}
	}
	g.markDirty(box)
}

// Clear erases every stamp, e.g. when a storm buries the corduroy.
func (g *GroomMap) Clear() {
	if g == nil {
		return
	}
	clear(g.Pixels)
	g.markDirty(image.Rect(0, 0, g.W, g.H))
}

// Bytes is the map as little-endian bytes, for saving.
func (g *GroomMap) Bytes() []byte {
	out := make([]byte, len(g.Pixels)*2)
	for i, v := range g.Pixels {
		out[2*i], out[2*i+1] = byte(v), byte(v>>8)
	}
	return out
}

// Load copies saved bytes in, reporting false if they don't fit the map.
func (g *GroomMap) Load(b []byte) bool {
	if g == nil || len(b) != len(g.Pixels)*2 {
		return false
	}
	for i := range g.Pixels {
		g.Pixels[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	g.markDirty(image.Rect(0, 0, g.W, g.H))
	return true
}

// Empty reports whether nothing has been stamped.
func (g *GroomMap) Empty() bool {
	if g == nil {
		return true
	}
	for i := 0; i < len(g.Pixels); i += 4 {
		if g.Pixels[i] != 0 {
			return false
		}
	}
	return true
}

// FallLineAt is the downhill direction at cell (x, z), or +z on the flat.
func (t *Terrain) FallLineAt(x, z int) (dx, dz float32) {
	gx, gz := t.GradientAt(x, z)
	if gx*gx+gz*gz < 1e-6 {
		return 0, 1
	}
	return -gx, -gz
}

// RestampGroomFromCells stamps every groomed cell whole, heading down the
// fall line. Used when a save has no groom map.
func (t *Terrain) RestampGroomFromCells() {
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			if t.Cells[x][z].Grooming >= 0.5 && t.Cells[x][z].TopLayer() != nil {
				dx, dz := t.FallLineAt(x, z)
				t.Groom.StampCell(x, z, dx, dz)
			}
		}
	}
}

// GroomCell sets a cell fully groomed without a cat and stamps it.
func (t *Terrain) GroomCell(x, z int) {
	if !t.InBounds(x, z) {
		return
	}
	c := &t.Cells[x][z]
	c.Grooming = 1
	if top := c.TopLayer(); top != nil {
		top.Kind = KindPackedPowder
	}
	dx, dz := t.FallLineAt(x, z)
	t.Groom.StampCell(x, z, dx, dz)
}
