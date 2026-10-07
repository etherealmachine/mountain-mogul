package world

// GroundPatch is the true ground over a block of the terrain, for edits
// that reshape it (the editor's brushes, lift aprons): the drawn height
// at each sample of the 1.25 m detail lattice where the terrain has
// detail, else at each cell's centre. Edit After, then Commit: the cells
// in the block are rebuilt from it the way the import builds them, and
// the detail around them re-derived, so ground the edit didn't change
// keeps its height.
type GroundPatch struct {
	t              *Terrain
	I0, J0, W, H   int     // samples: the lattice's (I0, J0) is the patch's (0, 0)
	Spacing        float32 // metres between samples
	Before, After  []float32
	x0, z0, x1, z1 int // the cells Commit rebuilds
}

// GroundPatch reads the ground for editing cells [x0, x1] × [z0, z1]
// (clamped), with reach more samples readable on every side (for edits
// that look around, like smoothing).
func (t *Terrain) GroundPatch(x0, z0, x1, z1, reach int) *GroundPatch {
	x0, z0 = max(x0, 0), max(z0, 0)
	x1, z1 = min(x1, t.Width-1), min(z1, t.Height-1)
	p := &GroundPatch{t: t, x0: x0, z0: z0, x1: x1, z1: z1}
	if d := t.Detail; d != nil {
		// The cells average lattice c×4 ± 2, and once their corners move
		// the detail a cell further out must be re-derived: 8 samples.
		pad := 2*DetailPerCell + reach
		p.Spacing = CellSize / DetailPerCell
		p.I0, p.J0 = max(x0*DetailPerCell-pad, 0), max(z0*DetailPerCell-pad, 0)
		i1, j1 := min(x1*DetailPerCell+pad, d.W-1), min(z1*DetailPerCell+pad, d.H-1)
		p.W, p.H = i1-p.I0+1, j1-p.J0+1
		p.Before = make([]float32, p.W*p.H)
		for j := 0; j < p.H; j++ {
			for i := 0; i < p.W; i++ {
				gi, gj := p.I0+i, p.J0+j
				p.Before[j*p.W+i] = t.MeshGroundAt(float32(gi)/DetailPerCell, float32(gj)/DetailPerCell) + d.Off[gj*d.W+gi]
			}
		}
	} else {
		p.Spacing = CellSize
		p.I0, p.J0 = max(x0-reach, 0), max(z0-reach, 0)
		i1, j1 := min(x1+reach, t.Width-1), min(z1+reach, t.Height-1)
		p.W, p.H = i1-p.I0+1, j1-p.J0+1
		p.Before = make([]float32, p.W*p.H)
		for j := 0; j < p.H; j++ {
			for i := 0; i < p.W; i++ {
				p.Before[j*p.W+i] = t.Cells[p.I0+i][p.J0+j].GroundElevation
			}
		}
	}
	p.After = append([]float32(nil), p.Before...)
	return p
}

// Pos is sample (i, j)'s world position.
func (p *GroundPatch) Pos(i, j int) (wx, wz float32) {
	if p.t.Detail != nil {
		return float32(p.I0+i) * p.Spacing, float32(p.J0+j) * p.Spacing
	}
	return (float32(p.I0+i) + 0.5) * CellSize, (float32(p.J0+j) + 0.5) * CellSize
}

// BeforeAt is the ground before the edit at sample (i, j), clamped to
// the patch.
func (p *GroundPatch) BeforeAt(i, j int) float32 {
	return p.Before[min(max(j, 0), p.H-1)*p.W+min(max(i, 0), p.W-1)]
}

// BeforeAtWorld is the ground before the edit at the sample nearest world
// (wx, wz), clamped to the patch.
func (p *GroundPatch) BeforeAtWorld(wx, wz float32) float32 {
	var fi, fj float32
	if p.t.Detail != nil {
		fi, fj = wx/p.Spacing, wz/p.Spacing
	} else {
		fi, fj = wx/CellSize-0.5, wz/CellSize-0.5
	}
	return p.BeforeAt(int(fi+0.5)-p.I0, int(fj+0.5)-p.J0)
}

// Commit writes After to the terrain: each cell in the block (but those
// keep reports) from the samples over it, then the detail across the
// patch, then the slopes around it.
func (p *GroundPatch) Commit(keep func(x, z int) bool) {
	t := p.t
	if d := t.Detail; d != nil {
		const half = DetailPerCell / 2
		weight := func(k int) float32 {
			if k == -half || k == half {
				return 0.5
			}
			return 1
		}
		for x := p.x0; x <= p.x1; x++ {
			for z := p.z0; z <= p.z1; z++ {
				if keep != nil && keep(x, z) {
					continue
				}
				var sum, wsum float32
				for dz := -half; dz <= half; dz++ {
					for dx := -half; dx <= half; dx++ {
						gi := min(max(x*DetailPerCell+dx, 0), d.W-1)
						gj := min(max(z*DetailPerCell+dz, 0), d.H-1)
						wt := weight(dx) * weight(dz)
						sum += p.After[(gj-p.J0)*p.W+(gi-p.I0)] * wt
						wsum += wt
					}
				}
				t.Cells[x][z].GroundElevation = sum / wsum
			}
		}
		for j := 0; j < p.H; j++ {
			for i := 0; i < p.W; i++ {
				gi, gj := p.I0+i, p.J0+j
				d.Off[gj*d.W+gi] = p.After[j*p.W+i] - t.MeshGroundAt(float32(gi)/DetailPerCell, float32(gj)/DetailPerCell)
			}
		}
	} else {
		for x := p.x0; x <= p.x1; x++ {
			for z := p.z0; z <= p.z1; z++ {
				if keep == nil || !keep(x, z) {
					t.Cells[x][z].GroundElevation = p.After[(z-p.J0)*p.W+(x-p.I0)]
				}
			}
		}
	}
	t.recomputeSlopesIn(p.x0-1, p.z0-1, p.x1+1, p.z1+1)
}
