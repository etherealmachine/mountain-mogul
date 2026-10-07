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

// ApplyBrush edits the ground under the brush and reports the cells whose
// ground changed (inclusive); ok is false when nothing did.
//
// With detail, it works on the true heights on the 1.25 m lattice: it
// changes them under the brush, sets each touched cell's elevation to the
// lattice's average over the cell (as the import does), then re-derives
// the detail around it from the new cells, so the ground outside the
// brush keeps its height. Without detail it edits the cells directly.
// Slopes are recomputed around the edit.
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
	if t.Detail == nil {
		t.brushCells(s, x0, z0, x1, z1)
	} else {
		t.brushDetail(s, x0, z0, x1, z1)
	}
	t.recomputeSlopesIn(x0-1, z0-1, x1+1, z1+1)
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

// brushCells edits cell elevations directly, for terrain without detail.
func (t *Terrain) brushCells(s BrushStroke, x0, z0, x1, z1 int) {
	k := smoothReach(s.Radius, CellSize)
	old := map[[2]int]float32{}
	at := func(x, z int) float32 {
		x, z = min(max(x, 0), t.Width-1), min(max(z, 0), t.Height-1)
		if v, ok := old[[2]int{x, z}]; ok {
			return v
		}
		return t.Cells[x][z].GroundElevation
	}
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			old[[2]int{x, z}] = t.Cells[x][z].GroundElevation
		}
	}
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			cx, cz := (float32(x)+0.5)*CellSize, (float32(z)+0.5)*CellSize
			wt := brushWeight(s, float32(math.Hypot(float64(cx-s.X), float64(cz-s.Z))))
			if wt <= 0 {
				continue
			}
			h := at(x, z)
			var avg float32
			if s.Brush == BrushSmooth {
				var n float32
				for dx := -k; dx <= k; dx++ {
					for dz := -k; dz <= k; dz++ {
						avg += at(x+dx, z+dz)
						n++
					}
				}
				avg /= n
			}
			t.Cells[x][z].GroundElevation = h + wt*(brushTarget(s, h, avg)-h)
		}
	}
}

// brushDetail edits the true heights on the detail lattice.
func (t *Terrain) brushDetail(s BrushStroke, x0, z0, x1, z1 int) {
	d := t.Detail
	const spacing = CellSize / DetailPerCell
	const half = DetailPerCell / 2
	k := smoothReach(s.Radius, spacing)
	// The lattice the edit reads and writes: the touched cells' samples
	// (cell c averages lattice c×4 ± 2), plus a cell's worth either side,
	// whose detail must be re-derived once the touched cells' corners
	// move, plus the smoothing reach.
	pad := DetailPerCell + half + k
	i0, j0 := max(x0*DetailPerCell-pad, 0), max(z0*DetailPerCell-pad, 0)
	i1, j1 := min(x1*DetailPerCell+pad, d.W-1), min(z1*DetailPerCell+pad, d.H-1)
	w, h := i1-i0+1, j1-j0+1
	true0 := make([]float32, w*h) // true heights before the stroke
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			gi, gj := i0+i, j0+j
			true0[j*w+i] = t.MeshGroundAt(float32(gi)/DetailPerCell, float32(gj)/DetailPerCell) + d.Off[gj*d.W+gi]
		}
	}
	at := func(i, j int) float32 {
		return true0[min(max(j, 0), h-1)*w+min(max(i, 0), w-1)]
	}
	heights := append([]float32(nil), true0...)
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			px, pz := float32(i0+i)*spacing, float32(j0+j)*spacing
			wt := brushWeight(s, float32(math.Hypot(float64(px-s.X), float64(pz-s.Z))))
			if wt <= 0 {
				continue
			}
			hh := true0[j*w+i]
			var avg float32
			if s.Brush == BrushSmooth {
				var n float32
				for dj := -k; dj <= k; dj++ {
					for di := -k; di <= k; di++ {
						avg += at(i+di, j+dj)
						n++
					}
				}
				avg /= n
			}
			heights[j*w+i] = hh + wt*(brushTarget(s, hh, avg)-hh)
		}
	}
	// Each touched cell is the lattice's average over it, edge samples at
	// half weight, as geo's import builds cells.
	weight := func(k int) float32 {
		if k == -half || k == half {
			return 0.5
		}
		return 1
	}
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			var sum, wsum float32
			for dz := -half; dz <= half; dz++ {
				for dx := -half; dx <= half; dx++ {
					gi := min(max(x*DetailPerCell+dx, 0), d.W-1)
					gj := min(max(z*DetailPerCell+dz, 0), d.H-1)
					wt := weight(dx) * weight(dz)
					sum += heights[(gj-j0)*w+(gi-i0)] * wt
					wsum += wt
				}
			}
			t.Cells[x][z].GroundElevation = sum / wsum
		}
	}
	// The detail over the lattice is what's left above the new mesh.
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			gi, gj := i0+i, j0+j
			d.Off[gj*d.W+gi] = heights[j*w+i] - t.MeshGroundAt(float32(gi)/DetailPerCell, float32(gj)/DetailPerCell)
		}
	}
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
