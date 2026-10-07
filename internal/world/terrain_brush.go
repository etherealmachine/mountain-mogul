package world

import "math"

// TerrainBrush is a scenario-editor ground brush.
type TerrainBrush uint8

const (
	BrushSmooth  TerrainBrush = iota // pulls the ground toward its local average
	BrushFlatten                     // pulls the ground toward a fixed height
	BrushRaise
	BrushLower
)

// BrushStroke is one application of a terrain brush: centred on world
// (X, Z), Radius metres across its falloff, at Strength (0..1, the share
// of the way to its target this application goes at the centre).
// Target is the height Flatten pulls toward. Raise and Lower move the
// ground by up to RaiseRate × Strength metres.
type BrushStroke struct {
	Brush    TerrainBrush
	X, Z     float32
	Radius   float32
	Strength float32
	Target   float32
}

// RaiseRate is how far Raise and Lower move the ground per application
// at full strength, in metres.
const RaiseRate = float32(0.5)

// GroundHeightAt is the drawn ground's height at world (wx, wz): the 5 m
// mesh surface plus the 1.25 m detail. What the terrain brushes edit.
func (t *Terrain) GroundHeightAt(wx, wz float32) float32 {
	gx, gz := wx/CellSize, wz/CellSize
	h := t.MeshGroundAt(gx, gz)
	if t.Detail != nil {
		h += t.Detail.At(gx, gz)
	}
	return h
}

// ApplyBrush edits the ground under the brush (a GroundPatch, so cells
// and detail stay consistent and ground outside the brush keeps its
// height) and reports the cells whose ground changed (inclusive); ok is
// false when nothing did.
func (t *Terrain) ApplyBrush(s BrushStroke) (x0, z0, x1, z1 int, ok bool) {
	if s.Radius <= 0 || s.Strength <= 0 {
		return
	}
	x0 = max(int(math.Floor(float64((s.X-s.Radius)/CellSize))), 0)
	z0 = max(int(math.Floor(float64((s.Z-s.Radius)/CellSize))), 0)
	x1 = min(int(math.Floor(float64((s.X+s.Radius)/CellSize))), t.Width-1)
	z1 = min(int(math.Floor(float64((s.Z+s.Radius)/CellSize))), t.Height-1)
	if x0 > x1 || z0 > z1 {
		return
	}
	spacing := float32(CellSize)
	if t.Detail != nil {
		spacing /= DetailPerCell
	}
	k := smoothReach(s.Radius, spacing)
	p := t.GroundPatch(x0, z0, x1, z1, k)
	for j := 0; j < p.H; j++ {
		for i := 0; i < p.W; i++ {
			px, pz := p.Pos(i, j)
			wt := brushWeight(s, float32(math.Hypot(float64(px-s.X), float64(pz-s.Z))))
			if wt <= 0 {
				continue
			}
			h := p.Before[j*p.W+i]
			var avg float32
			if s.Brush == BrushSmooth {
				var n float32
				for dj := -k; dj <= k; dj++ {
					for di := -k; di <= k; di++ {
						avg += p.BeforeAt(i+di, j+dj)
						n++
					}
				}
				avg /= n
			}
			p.After[j*p.W+i] = h + wt*(brushTarget(s, h, avg)-h)
		}
	}
	p.Commit(nil)
	return x0, z0, x1, z1, true
}

// brushWeight is how much of the stroke reaches a point dist metres from
// its centre: full near the middle, easing to nothing at the radius.
func brushWeight(s BrushStroke, dist float32) float32 {
	if dist >= s.Radius {
		return 0
	}
	f := 1 - dist/s.Radius
	return s.Strength * f * f * (3 - 2*f)
}

// brushTarget is where the stroke pulls a point of height h whose local
// average is avg.
func brushTarget(s BrushStroke, h, avg float32) float32 {
	switch s.Brush {
	case BrushSmooth:
		return avg
	case BrushFlatten:
		return s.Target
	case BrushRaise:
		return h + RaiseRate
	case BrushLower:
		return h - RaiseRate
	}
	return h
}

// smoothReach is how many samples either side the smooth brush averages
// over: a third of its radius, so a wide brush smooths away wide
// features (a half-pipe), at least two samples.
func smoothReach(radius, spacing float32) int {
	return max(int(radius/3/spacing+0.5), 2)
}

// recomputeSlopesIn recomputes Cell.Slope for the cells in
// [x0, x1] × [z0, z1] (clamped), and marks what depends on slopes stale.
func (t *Terrain) recomputeSlopesIn(x0, z0, x1, z1 int) {
	t.horizonStale = true
	t.shed = nil
	for x := max(x0, 0); x <= min(x1, t.Width-1); x++ {
		for z := max(z0, 0); z <= min(z1, t.Height-1); z++ {
			gx, gz := t.GradientAt(x, z)
			t.Cells[x][z].Slope = float32(math.Sqrt(float64(gx*gx + gz*gz)))
		}
	}
}
