package scene

import (
	"math"
	"sort"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// treePlaceAttempts is how many random positions placeTreesInCell tries
// per tree before giving up on a crowded cell.
const treePlaceAttempts = 8

// placeTreesInCell adds up to n trees to cell (x, z) at random positions
// inside it, each at least world.TreeMinSpacing from every other trunk.
// rnd returns floats in [0, 1). Returns how many were placed.
func placeTreesInCell(t *world.Terrain, x, z, n int, rnd func() float32) int {
	placed := 0
	for i := 0; i < n; i++ {
		for a := 0; a < treePlaceAttempts; a++ {
			tr := world.Tree{
				X: (float32(x) + rnd()) * world.CellSize,
				Z: (float32(z) + rnd()) * world.CellSize,
			}
			if t.CanPlaceTree(tr, world.TreeMinSpacing) {
				t.AddTree(tr)
				placed++
				break
			}
		}
	}
	return placed
}

// plantTreesUpTo is the plant brush: every cell within `radius` cells of
// (cx, cz) gains at most one tree per stroke until it holds the count
// `target` density calls for. The per-cell roll is fixed, so painting
// over an area twice converges instead of flickering, and cells already
// at or above target are left alone — lowering the slider after
// painting doesn't erase forest; the glade tool does that.
func plantTreesUpTo(t *world.Terrain, cx, cz, radius int, target float32, rnd func() float32) {
	r2 := radius * radius
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dz*dz > r2 {
				continue
			}
			x, z := cx+dx, cz+dz
			if !t.InBounds(x, z) {
				continue
			}
			want := world.TreeCountFromDensity(target, world.TreeInstanceHash(x, z, -1))
			if int(t.Cells[x][z].TreeCount) < want {
				placeTreesInCell(t, x, z, 1, rnd)
			}
		}
	}
}

// gladeBrushMetres is the glade brush's reach from the hovered cell's
// centre: the same circle the terrain shader draws as the brush ring.
func gladeBrushMetres(radius int) float32 {
	return (float32(radius) + 0.5) * world.CellSize
}

// gladeSelection returns the trees one glade stroke centred on cell
// (cx, cz) takes: of the trunks inside the brush ring, the first
// ceil(share × n) in per-tree hash order. The order is fixed, so the
// highlighted preview is exactly what a click removes, and repeated
// clicks keep thinning the same spot evenly.
func gladeSelection(t *world.Terrain, cx, cz, radius int, share float32) []world.Tree {
	if !t.InBounds(cx, cz) || share <= 0 {
		return nil
	}
	wx := (float32(cx) + 0.5) * world.CellSize
	wz := (float32(cz) + 0.5) * world.CellSize
	var in []world.Tree
	t.ForEachTreeNear(wx, wz, gladeBrushMetres(radius), func(tr world.Tree, _, _ int) {
		in = append(in, tr)
	})
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool { return in[i].Hash() < in[j].Hash() })
	n := int(math.Ceil(float64(share) * float64(len(in))))
	if n > len(in) {
		n = len(in)
	}
	return in[:n]
}

// gladeCost is what removing trees costs.
func gladeCost(trees []world.Tree) int {
	return len(trees) * world.GladeCostPerTree
}

// removeTrees removes each tree in trees from the terrain.
func removeTrees(t *world.Terrain, trees []world.Tree) {
	for _, tr := range trees {
		t.RemoveTree(tr)
	}
}

// gladeHighlightTint marks the trees a glade stroke would take. It
// multiplies the trees' dark green base colour, so red is pushed well
// past 1 to read as red.
var gladeHighlightTint = [3]float32{6, 0.7, 0.5}

// setGladeHighlight draws trees as tinted ghosts over the forest, so the
// player sees exactly which ones the glade brush will remove.
func setGladeHighlight(r *render.Renderer, t *world.Terrain, trees []world.Tree) {
	byVariant := map[uint32][]render.StaticInstance{}
	for _, tr := range trees {
		x, z := int(tr.X/world.CellSize), int(tr.Z/world.CellSize)
		ti := world.TreeInstanceOf(tr, x, z, render.MeshTree)
		const grow = 1.04 // just proud of the real tree so the tint shows
		byVariant[ti.Variant] = append(byVariant[ti.Variant], render.StaticInstance{
			Transform: render.TreeTransform(t, ti, grow),
			ColorTint: gladeHighlightTint,
		})
	}
	for _, v := range []uint32{render.MeshTree, render.MeshTree2, render.MeshTree3} {
		r.SetGhosts(v, byVariant[v])
	}
}
