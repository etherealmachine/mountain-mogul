package render

import (
	"mountain-mogul/internal/world"

	"github.com/go-gl/mathgl/mgl32"
)

// forestCache keeps each forest tree's instance between static rebuilds,
// so placing a building or a lift doesn't redo every tree on the map
// (Kirkwood's 300,000 took 150 ms). The cache holds the parts that come
// from the tree alone (its rotation, scale and variant); the height,
// which follows the ground, the detail and the snow, is read again on
// every rebuild, so trees still sit on regraded ground. A cell is
// recomputed when its trees no longer match the cached ones.
type forestCache struct {
	terrain *world.Terrain
	cells   []forestCell // x*Height + z
}

type forestCell struct {
	trees   []world.Tree     // the trees the instances were made from
	inst    []StaticInstance // translation Y left at 0
	variant []uint32
}

// appendForest adds every drawn forest tree to the static batches.
func (r *Renderer) appendForest(t *world.Terrain) {
	fc := &r.scene.forest
	if fc.terrain != t || len(fc.cells) != t.Width*t.Height {
		*fc = forestCache{terrain: t, cells: make([]forestCell, t.Width*t.Height)}
	}
	tint := treeTintForVariant(MeshTree)
	// The mesh is (W-1)×(H-1) quads: trees in the last row and column sit
	// past its edge (as world.Terrain.ForEachTree).
	for x := 0; x < t.Width-1; x++ {
		for z := 0; z < t.Height-1; z++ {
			trees := t.TreesInCell(x, z)
			c := &fc.cells[x*t.Height+z]
			if len(trees) == 0 && len(c.trees) == 0 {
				continue
			}
			if !sameTrees(c.trees, trees) {
				c.refresh(trees, x, z, tint)
			}
			y := t.GroundElevationAt(x, z)
			if snow := t.Cells[x][z].VisibleSnowDepth(); snow > 0 {
				const maxBury = float32(1.5) // as treeMatrix
				y += min(snow, maxBury)
			}
			for i, in := range c.inst {
				batch, ok := r.staticBatches[c.variant[i]]
				if !ok {
					continue
				}
				in.Transform[13] = y + t.DetailAt(in.Transform[12], in.Transform[14])
				batch.staticInstances = append(batch.staticInstances, in)
				batch.dirty = true
			}
		}
	}
}

// refresh remakes the cell's instances from trees.
func (c *forestCell) refresh(trees []world.Tree, x, z int, tint mgl32.Vec3) {
	c.trees = append(c.trees[:0], trees...)
	c.inst = c.inst[:0]
	c.variant = c.variant[:0]
	for _, tr := range trees {
		ti := world.TreeInstanceOf(tr, x, z, MeshTree)
		m := mgl32.Translate3D(ti.WX, 0, ti.WZ).
			Mul4(mgl32.HomogRotate3DY(ti.Rotation)).
			Mul4(mgl32.Scale3D(ti.Scale, ti.Scale, ti.Scale))
		in := StaticInstance{ColorTint: [3]float32{tint[0], tint[1], tint[2]}}
		copy(in.Transform[:], m[:])
		c.inst = append(c.inst, in)
		c.variant = append(c.variant, ti.Variant)
	}
}

func sameTrees(a, b []world.Tree) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
