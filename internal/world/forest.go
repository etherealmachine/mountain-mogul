package world

import (
	"image"
	"math"
)

// MaxTreesPerCell is how many trees a cell holds at density 1.0. With
// 5×5 m cells this is 800 trees/ha — slightly under the densest
// subalpine stands but a sensible ceiling given camera distance and the
// GPU cost of every extra instance. Writers that think in density
// (generator, plant brush, old-save conversion) scale by it, and
// Cell.TreeCover divides by it.
const MaxTreesPerCell = 2

// TreeMinSpacing is the closest two trunks may stand when a writer
// places trees by sampling (metres).
const TreeMinSpacing = float32(1.8)

// Tree is one stored tree: its trunk's world-space XZ in metres.
// Rotation, scale, and model variant derive from a hash of the position
// (TreeInstanceOf), so they aren't stored and survive save/load.
type Tree struct {
	X, Z float32
}

// TreeInstanceHash returns a stable 64-bit hash of three integers. The
// old density rule used it per cell (x, z) and tree index i, with i = -1
// for the cell's "any tree at all?" roll; stored trees hash their
// centimetre-quantised position.
func TreeInstanceHash(x, z, i int) uint64 {
	h := uint64(uint32(x)*2654435761 ^ uint32(z)*2246822519 ^ uint32(i)*2692343)
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	return h
}

// TreeCountFromDensity maps a density in [0, 1] to a per-cell tree count
// in [0, MaxTreesPerCell]. Density × max gives the expected count; we
// emit the whole part deterministically and roll the fractional part
// against cellHash so density scales smoothly through every count
// without dead zones.
func TreeCountFromDensity(density float32, cellHash uint64) int {
	if density <= 0 {
		return 0
	}
	if density >= 1 {
		return MaxTreesPerCell
	}
	target := density * MaxTreesPerCell
	whole := int(target)
	frac := target - float32(whole)
	p := float32(cellHash&0xFFFF) / 65535.0
	if p < frac {
		whole++
	}
	return whole
}

// TreeInstance is one stored tree with its derived visual values. The
// renderer reads Rotation, Scale, and Variant; sub-cell passes (tree
// wells) only need WX/WZ.
type TreeInstance struct {
	X, Z     int     // owning cell index
	WX, WZ   float32 // world-space XZ in metres
	Rotation float32 // radians
	Scale    float32 // model-units multiplier
	Variant  uint32  // MeshTree + (0|1|2)
}

// RecomputeGroomEdges rebuilds the B channel of Surface from per-cell
// Grooming: each pixel within 1 m of a cell boundary across which the
// grooming value steps gets a falloff value 1 − dist/1m on B. The
// resulting band is one pixel deep on each side of the boundary, which
// the shader uses to draw a sharp lip / shadow line where untracked
// powder meets a groomed lane.
//
// Writes B in-place — every pixel of every cell gets its new B (0 if
// no neighbour differs, falloff otherwise). The dirty box accumulates
// only the cells whose B actually changed since the previous compute,
// so steady-state frames upload nothing even though we ran the scan.
//
// Per-meter band scales with PxPerCell: at PxPerCell=20 the band is
// 4 pixels = 1 m on each side of the edge.
func (t *Terrain) RecomputeGroomEdges() {
	if t == nil || t.Surface == nil {
		return
	}
	sd := t.Surface
	// groomedSide returns true when a cell is clearly on the groomed side
	// of the groomed/ungroomed boundary. Using a threshold of 0.5 means
	// wear-variation within an active route (all cells typically 0.6–1.0)
	// never triggers internal edges — only true groomed↔ungroomed crossings
	// produce the lip/scrape band.
	const groomedSide = float32(0.5)
	groomedAt := func(g float32) bool { return g >= groomedSide }

	// 1 m band in pixel units.
	bandPx := PxPerMeter()
	bandLen := PxPerCell // worst case minD horizon (anything > bandPx → 0)

	dirty := image.Rectangle{}
	anyDirty := false
	markCell := func(px0, pz0 int) {
		r := image.Rect(px0, pz0, px0+PxPerCell, pz0+PxPerCell)
		if !anyDirty {
			dirty = r
			anyDirty = true
		} else {
			dirty = dirty.Union(r)
		}
	}

	for cz := 0; cz < t.Height; cz++ {
		for cx := 0; cx < t.Width; cx++ {
			g0 := t.Cells[cx][cz].Grooming
			var diffL, diffR, diffD, diffU bool
			if cx > 0 {
				diffL = groomedAt(g0) != groomedAt(t.Cells[cx-1][cz].Grooming)
			}
			if cx < t.Width-1 {
				diffR = groomedAt(g0) != groomedAt(t.Cells[cx+1][cz].Grooming)
			}
			if cz > 0 {
				diffD = groomedAt(g0) != groomedAt(t.Cells[cx][cz-1].Grooming)
			}
			if cz < t.Height-1 {
				diffU = groomedAt(g0) != groomedAt(t.Cells[cx][cz+1].Grooming)
			}
			hasEdge := diffL || diffR || diffD || diffU
			ci := cx*t.Height + cz
			hadEdge := sd.EdgeCells[ci]
			px0 := cx * PxPerCell
			pz0 := cz * PxPerCell
			if !hasEdge {
				// No edge now. If we didn't have an edge last frame
				// either, the cell's B pixels are already 0 — skip the
				// per-pixel walk entirely. Steady-state cost: just the
				// 4-neighbour check above.
				if !hadEdge {
					continue
				}
				// Edge moved away — clear the band we last wrote.
				for dz := 0; dz < PxPerCell; dz++ {
					row := (pz0 + dz) * sd.PxWidth
					for dx := 0; dx < PxPerCell; dx++ {
						off := (row + px0 + dx) * 4
						sd.Pixels[off+chGroomEdge] = 0
					}
				}
				sd.EdgeCells[ci] = false
				markCell(px0, pz0)
				continue
			}
			cellDirty := false
			for dz := 0; dz < PxPerCell; dz++ {
				row := (pz0 + dz) * sd.PxWidth
				for dx := 0; dx < PxPerCell; dx++ {
					minD := float32(bandLen)
					if diffL {
						minD = minF32(minD, float32(dx)+0.5)
					}
					if diffR {
						minD = minF32(minD, float32(PxPerCell-1-dx)+0.5)
					}
					if diffD {
						minD = minF32(minD, float32(dz)+0.5)
					}
					if diffU {
						minD = minF32(minD, float32(PxPerCell-1-dz)+0.5)
					}
					var v uint8
					if minD < bandPx {
						v = uint8((1.0 - minD/bandPx) * 255)
					}
					off := (row + px0 + dx) * 4
					if sd.Pixels[off+chGroomEdge] != v {
						sd.Pixels[off+chGroomEdge] = v
						cellDirty = true
					}
				}
			}
			sd.EdgeCells[ci] = true
			if cellDirty {
				markCell(px0, pz0)
			}
		}
	}
	if anyDirty {
		sd.MarkDirty(dirty)
	}
}

func absDiff(a, b float32) float32 {
	d := a - b
	if d < 0 {
		return -d
	}
	return d
}

func minF32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// RestampTreeWells zeros the G channel of Surface and writes a
// Gaussian-falloff disk for every tree in the terrain. Use after bulk
// tree changes (auto-forest regenerate, world load, lift/road clears).
// It walks every tree and marks the whole
// texture for re-upload, which on a large map costs tens of ms plus a
// full-texture upload; brushes use RestampTreeWellsCells instead.
//
// Visual scale: 2.0 m radius matches the per-tree footprint the doc
// calls for; peak 255 (full G) at the trunk so the shader's `well`
// channel reads 1.0 at the centre and tapers to 0 at the radius edge.
func (t *Terrain) RestampTreeWells() {
	if t == nil || t.Surface == nil {
		return
	}
	sd := t.Surface
	sd.zeroChannel(chTreeWell)
	const wellRadiusM = float32(2.0)
	ppm := PxPerMeter()
	radiusPx := wellRadiusM * ppm
	t.ForEachTree(0, func(ti TreeInstance) {
		sd.stampMaxChannelDisk(ti.WX*ppm, ti.WZ*ppm, radiusPx, chTreeWell, 255)
	})
}

// RestampTreeWellsCells redoes tree wells for the cell rectangle
// [x0, x1] × [z0, z1] (inclusive) after its trees changed, and marks
// only the surrounding area dirty. A well spills at most its 2 m radius
// into the neighbouring cell, so the clear covers one extra cell and
// trees two cells out are restamped; where their wells fall outside the
// cleared area, max-stamping rewrites the same values.
func (t *Terrain) RestampTreeWellsCells(x0, z0, x1, z1 int) {
	if t == nil || t.Surface == nil {
		return
	}
	sd := t.Surface
	x0, z0 = max(x0-1, 0), max(z0-1, 0)
	x1, z1 = min(x1+1, t.Width-1), min(z1+1, t.Height-1)
	if x0 > x1 || z0 > z1 {
		return
	}
	box := image.Rect(x0*PxPerCell, z0*PxPerCell, (x1+1)*PxPerCell, (z1+1)*PxPerCell).
		Intersect(image.Rect(0, 0, sd.PxWidth, sd.PxHeight))
	stride := sd.PxWidth * 4
	for z := box.Min.Y; z < box.Max.Y; z++ {
		row := z * stride
		for x := box.Min.X; x < box.Max.X; x++ {
			sd.Pixels[row+x*4+chTreeWell] = 0
		}
	}
	const wellRadiusM = float32(2.0)
	ppm := PxPerMeter()
	radiusPx := wellRadiusM * ppm
	t.forEachTreeIn(x0-1, z0-1, x1+1, z1+1, 0, func(ti TreeInstance) {
		sd.stampMaxChannelDisk(ti.WX*ppm, ti.WZ*ppm, radiusPx, chTreeWell, 255)
	})
	sd.MarkDirty(box)
}

// ForEachTree iterates every visible tree on the terrain at the same
// world XZ the renderer uses. Skips the right/back cell edge because the
// visible terrain is (W-1)×(H-1) quads — trees in Cells[W-1][*] /
// Cells[*][H-1] sit past the mesh edge and would float.
//
// `variantBase` is the renderer's MeshTree base ID (or 0 if the caller
// doesn't care about variant — typical for sim/world consumers that only
// need positions). Sub-cell passes (tree wells) should pass 0.
func (t *Terrain) ForEachTree(variantBase uint32, fn func(TreeInstance)) {
	t.forEachTreeIn(0, 0, t.Width-2, t.Height-2, variantBase, fn)
}

// forEachTreeIn is ForEachTree limited to cells [x0, x1] × [z0, z1]
// (inclusive, clipped to the visible grid).
func (t *Terrain) forEachTreeIn(x0, z0, x1, z1 int, variantBase uint32, fn func(TreeInstance)) {
	x0, z0 = max(x0, 0), max(z0, 0)
	x1, z1 = min(x1, t.Width-2), min(z1, t.Height-2)
	for z := z0; z <= z1; z++ {
		for x := x0; x <= x1; x++ {
			for _, tr := range t.trees[x*t.Height+z] {
				fn(TreeInstanceOf(tr, x, z, variantBase))
			}
		}
	}
}

// TreeInstanceOf derives a stored tree's visual values from a hash of
// its centimetre-quantised position, so the same tree looks the same
// after save/load. (x, z) is the owning cell.
func TreeInstanceOf(tr Tree, x, z int, variantBase uint32) TreeInstance {
	h := treeHash(tr)
	return TreeInstance{
		X: x, Z: z,
		WX: tr.X, WZ: tr.Z,
		Rotation: float32((h>>16)&0xFFFF) / 65535.0 * 2 * math.Pi,
		Scale:    1.55 + float32((h>>32)&0xFF)/255.0*0.4,
		Variant:  variantBase + uint32((h>>40)%3),
	}
}

func treeHash(tr Tree) uint64 {
	qx := int(math.Round(float64(tr.X) * 100))
	qz := int(math.Round(float64(tr.Z) * 100))
	return TreeInstanceHash(qx, qz, 0x7EE5)
}

// legacyTreePositions returns where the old density rule put a cell's
// trees: the count from TreeCountFromDensity, each offset within ±1.2 m
// of the cell centre by the per-tree hash.
func legacyTreePositions(x, z int, density float32) []Tree {
	count := TreeCountFromDensity(density, TreeInstanceHash(x, z, -1))
	out := make([]Tree, 0, count)
	for i := 0; i < count; i++ {
		h := TreeInstanceHash(x, z, i)
		offsetX := (float32(h&0xFF)/127.5 - 1.0) * 1.2
		offsetZ := (float32((h>>8)&0xFF)/127.5 - 1.0) * 1.2
		out = append(out, Tree{
			X: (float32(x)+0.5)*float32(CellSize) + offsetX,
			Z: (float32(z)+0.5)*float32(CellSize) + offsetZ,
		})
	}
	return out
}
