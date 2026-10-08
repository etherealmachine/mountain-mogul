package world

// Stored trees live in per-cell buckets on the Terrain (trees[x*Height+z]),
// so brushes and queries touch only nearby cells. Cell.TreeCount mirrors
// each bucket's length; every write goes through the methods here so the
// two stay in step.

// TreeCover is the cell's trees as a fraction of a full stand: 0 for no
// trees, 1 for MaxTreesPerCell or more. Gameplay reads it where it used
// to read density, so it reacts to the trees the player can see.
func (c Cell) TreeCover() float32 {
	f := float32(c.TreeCount) / MaxTreesPerCell
	if f > 1 {
		return 1
	}
	return f
}

// treeCellOf returns the cell holding world XZ (wx, wz).
func (t *Terrain) treeCellOf(wx, wz float32) (x, z int, ok bool) {
	if wx < 0 || wz < 0 {
		return 0, 0, false
	}
	x, z = int(wx/CellSize), int(wz/CellSize)
	return x, z, t.InBounds(x, z)
}

func (t *Terrain) setCellTrees(x, z int, trees []Tree) {
	if len(trees) == 0 {
		trees = nil
	}
	t.trees[x*t.Height+z] = trees
	n := len(trees)
	if n > 255 {
		n = 255
	}
	t.Cells[x][z].TreeCount = uint8(n)
	t.markTrunksDirty(x, z)
}

// TreesInCell returns the trees stored in cell (x, z). The slice is the
// terrain's own; don't modify it.
func (t *Terrain) TreesInCell(x, z int) []Tree {
	if !t.InBounds(x, z) {
		return nil
	}
	return t.trees[x*t.Height+z]
}

// AddTree stores a tree. Returns false when its position is off the map.
func (t *Terrain) AddTree(tr Tree) bool {
	x, z, ok := t.treeCellOf(tr.X, tr.Z)
	if !ok {
		return false
	}
	t.setCellTrees(x, z, append(t.trees[x*t.Height+z], tr))
	return true
}

// CanPlaceTree reports whether a writer that samples positions may put a
// tree at tr: inside the drawn part of the map (the last row and column
// sit past the terrain mesh), on a passable cell, and at least
// minSpacing from every other trunk.
func (t *Terrain) CanPlaceTree(tr Tree, minSpacing float32) bool {
	x, z, ok := t.treeCellOf(tr.X, tr.Z)
	if !ok || x >= t.Width-1 || z >= t.Height-1 || !t.Cells[x][z].Passable {
		return false
	}
	clear := true
	t.ForEachTreeNear(tr.X, tr.Z, minSpacing, func(Tree, int, int) { clear = false })
	return clear
}

// RemoveTreesIn removes every tree in cells [x0, x1] × [z0, z1]
// (inclusive, clipped to the map) for which remove returns true, and
// returns how many went. A nil remove takes every tree in the range.
func (t *Terrain) RemoveTreesIn(x0, z0, x1, z1 int, remove func(Tree) bool) int {
	x0, z0 = max(x0, 0), max(z0, 0)
	x1, z1 = min(x1, t.Width-1), min(z1, t.Height-1)
	removed := 0
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			bucket := t.trees[x*t.Height+z]
			if len(bucket) == 0 {
				continue
			}
			kept := bucket[:0]
			for _, tr := range bucket {
				if remove == nil || remove(tr) {
					removed++
					continue
				}
				kept = append(kept, tr)
			}
			if len(kept) != len(bucket) {
				t.setCellTrees(x, z, kept)
			}
		}
	}
	return removed
}

// RemoveTree removes the stored tree at exactly tr's position. Returns
// false when there is none.
func (t *Terrain) RemoveTree(tr Tree) bool {
	x, z, ok := t.treeCellOf(tr.X, tr.Z)
	if !ok {
		return false
	}
	return t.RemoveTreesIn(x, z, x, z, func(o Tree) bool { return o == tr }) > 0
}

// Hash is a stable per-tree hash of its position, the same one its
// visual values derive from. Tools use it to pick trees in an order
// that is arbitrary but repeatable.
func (tr Tree) Hash() uint64 { return treeHash(tr) }

// ClearTreesInCell removes every tree in cell (x, z).
func (t *Terrain) ClearTreesInCell(x, z int) int {
	if !t.InBounds(x, z) {
		return 0
	}
	n := len(t.trees[x*t.Height+z])
	if n > 0 {
		t.setCellTrees(x, z, nil)
	}
	return n
}

// ClearAllTrees removes every tree on the map.
func (t *Terrain) ClearAllTrees() {
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			if t.Cells[x][z].TreeCount > 0 {
				t.setCellTrees(x, z, nil)
			}
		}
	}
}

// ForEachTreeNear calls fn for every tree within radius metres of world
// XZ (wx, wz), with the cell that holds it.
func (t *Terrain) ForEachTreeNear(wx, wz, radius float32, fn func(tr Tree, x, z int)) {
	r2 := radius * radius
	x0, z0 := int((wx-radius)/CellSize), int((wz-radius)/CellSize)
	x1, z1 := int((wx+radius)/CellSize), int((wz+radius)/CellSize)
	x0, z0 = max(x0, 0), max(z0, 0)
	x1, z1 = min(x1, t.Width-1), min(z1, t.Height-1)
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			for _, tr := range t.trees[x*t.Height+z] {
				dx, dz := tr.X-wx, tr.Z-wz
				if dx*dx+dz*dz <= r2 {
					fn(tr, x, z)
				}
			}
		}
	}
}

// ForEachStoredTree calls fn for every stored tree, cell by cell in
// x-major order, including trees in the undrawn last row and column.
// ForEachTree is the drawn subset with visual values.
func (t *Terrain) ForEachStoredTree(fn func(Tree)) {
	for _, bucket := range t.trees {
		for _, tr := range bucket {
			fn(tr)
		}
	}
}

// TotalTrees returns how many trees are stored.
func (t *Terrain) TotalTrees() int {
	n := 0
	for _, bucket := range t.trees {
		n += len(bucket)
	}
	return n
}

// SetCellTreesFromDensity replaces cell (x, z)'s trees with what the old
// per-cell density rule drew there. Old saves convert through it, so
// their forests keep the same trunk positions.
func (t *Terrain) SetCellTreesFromDensity(x, z int, density float32) {
	if !t.InBounds(x, z) {
		return
	}
	t.setCellTrees(x, z, legacyTreePositions(x, z, density))
}
