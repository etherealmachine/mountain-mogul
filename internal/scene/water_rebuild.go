//go:build !demo

package scene

import (
	"fmt"

	"mountain-mogul/internal/geo"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/world"
)

// rebuildWater rebuilds the ground and water after w's water changed
// from old to w.Water (Water Editing): the layers from layer from (the
// Creeks layer for an edit, the Water layer to refit) rerun with the old
// water and the new, and the ground is patched only where the two differ, so ground
// edited since the import keeps its shape elsewhere. Then the material
// map and lake grid are redone (each lake keeping its ice), open water
// loses its snow, ground that left a lake gets its neighbours' snow, and
// trees left standing in water go. Trees and snow elsewhere stay.
func rebuildWater(w *world.World, stack *geo.LayerStack, c *layerCache, old *world.Water, from string) error {
	base, t := w.TerrainBase, w.Terrain
	if base == nil || stack == nil || stack.Base() != base || stack.Water() != w.Water || old == nil {
		return fmt.Errorf("no terrain base")
	}
	set := geo.Settings(base)
	now := *w.Water
	*w.Water = *old
	stack.Invalidate(from)
	before := stack.Run(set, nil)
	*w.Water = now
	stack.Invalidate(from)
	after := stack.Run(set, nil)
	patchGroundWhere(t, base, before, after)

	oldLakeOf, oldIce := t.LakeOf, w.Lakes
	*c = layerCache{}
	if base.LayerOn("material") {
		runMaterialLayer(w, c)
	}
	// Each lake keeps its ice: by name where it has one, else by place
	// in the list. One that was open and now freezes is thawed.
	thawed := make([]bool, len(w.Lakes))
	for i := range w.Lakes {
		for j, o := range oldIce {
			if o.Name == w.Lakes[i].Name && (o.Name != "" || i == j) {
				w.Lakes[i].Frost, w.Lakes[i].Thaw = o.Frost, o.Thaw
				thawed[i] = o.Type.Open() && !w.Lakes[i].Type.Open()
				break
			}
		}
	}
	sim.ApplyLakes(w)
	if oldLakeOf != nil && t.LakeOf != nil {
		snowOntoOldLake(t, oldLakeOf)
	}
	snowOntoThawedLakes(w, thawed)
	removeTreesInWater(t)
	return nil
}

// patchGroundWhere sets t's ground to after wherever after differs from
// before (both on base's lattice), a block at a time.
func patchGroundWhere(t *world.Terrain, base *world.TerrainBase, before, after []float32) {
	const block = 64 // cells
	per := 1
	if base.Detail {
		per = world.DetailPerCell
	}
	changed := map[[2]int][]int{}
	for k := range after {
		if after[k] != before[k] {
			i, j := k%base.W, k/base.W
			b := [2]int{i / per / block, j / per / block}
			changed[b] = append(changed[b], k)
		}
	}
	for b, ks := range changed {
		x0, z0 := b[0]*block, b[1]*block
		p := t.GroundPatch(x0-1, z0-1, x0+block, z0+block, 0)
		for _, k := range ks {
			i, j := k%base.W-p.I0, k/base.W-p.J0
			if i >= 0 && j >= 0 && i < p.W && j < p.H {
				p.After[j*p.W+i] = after[k]
			}
		}
		p.Commit(nil)
	}
}

// snowOntoOldLake gives each grid point that was a lake's (oldLakeOf)
// and no longer is the mean snow of the dry points round it.
func snowOntoOldLake(t *world.Terrain, oldLakeOf []uint8) {
	W, H := t.Width, t.Height
	for x := 0; x < W; x++ {
		for z := 0; z < H; z++ {
			if oldLakeOf[x*H+z] == 0 || t.LakeOf[x*H+z] != 0 {
				continue
			}
			var base, acc float32
			var n int
			var kind world.SnowKind
			for dx := -1; dx <= 1; dx++ {
				for dz := -1; dz <= 1; dz++ {
					nx, nz := x+dx, z+dz
					if nx < 0 || nz < 0 || nx >= W || nz >= H || oldLakeOf[nx*H+nz] != 0 || t.LakeOf[nx*H+nz] != 0 {
						continue
					}
					c := t.Cells[nx][nz]
					base += c.Base
					acc += c.Top.Accumulation
					kind = c.Top.Kind
					n++
				}
			}
			if n > 0 {
				c := &t.Cells[x][z]
				c.Base, c.Top.Accumulation, c.Top.Kind = base/float32(n), acc/float32(n), kind
				t.MarkSnowDirty(x, z)
			}
		}
	}
}

// removeTreesInWater removes the trees standing on a lake or creek.
func removeTreesInWater(t *world.Terrain) {
	if m := t.Material; m != nil {
		t.RemoveTreesIn(0, 0, t.Width-1, t.Height-1, func(tr world.Tree) bool { return m.At(tr.X, tr.Z).IsWater() })
	}
}

// snowOntoThawedLakes gives the ice of each thawed lake (one that was
// open water and now freezes) at least the mean snow of the dry ground
// round its shore (lakeShoreRing grid points out), where it has less: it
// takes the snow it would have held.
func snowOntoThawedLakes(w *world.World, thawed []bool) {
	t := w.Terrain
	if t.LakeOf == nil {
		return
	}
	W, H := t.Width, t.Height
	sum := make([]float64, len(w.Lakes))
	n := make([]int, len(w.Lakes))
	for x := 0; x < W; x++ {
		for z := 0; z < H; z++ {
			if t.LakeOf[x*H+z] != 0 {
				continue
			}
		near:
			for dx := -lakeShoreRing; dx <= lakeShoreRing; dx++ {
				for dz := -lakeShoreRing; dz <= lakeShoreRing; dz++ {
					nx, nz := x+dx, z+dz
					if nx < 0 || nz < 0 || nx >= W || nz >= H {
						continue
					}
					if id := t.LakeOf[nx*H+nz]; id != 0 && thawed[id-1] {
						c := t.Cells[x][z]
						sum[id-1] += float64(c.Base + c.Top.Accumulation)
						n[id-1]++
						break near
					}
				}
			}
		}
	}
	for x := 0; x < W; x++ {
		for z := 0; z < H; z++ {
			id := t.LakeOf[x*H+z]
			if id == 0 || n[id-1] == 0 || w.Lakes[id-1].IceAt(t.LakeDepth[x*H+z]) <= 0 {
				continue
			}
			want := float32(sum[id-1] / float64(n[id-1]))
			c := &t.Cells[x][z]
			if have := c.Base + c.Top.Accumulation; have < want {
				c.Base += want - have
				t.MarkSnowDirty(x, z)
			}
		}
	}
}
