package world

// SnowTile is the side, in cells, of the tiles the renderer re-uploads
// snow state by. Writers mark the cells they change; the renderer
// rebuilds only the tiles touched since its last flush.
const SnowTile = 16

// snowDirty records which tiles' snow state (layers, grooming, moguls)
// has changed since the renderer last flushed it.
type snowDirty struct {
	all   bool
	tiles []bool // [tz*tilesW+tx]
	list  []int  // indices set in tiles, in marking order
}

func (t *Terrain) snowTilesW() int { return (t.Width + SnowTile - 1) / SnowTile }
func (t *Terrain) snowTilesH() int { return (t.Height + SnowTile - 1) / SnowTile }

// MarkSnowDirty records that cell (x, z)'s snow state changed.
func (t *Terrain) MarkSnowDirty(x, z int) {
	if !t.InBounds(x, z) || t.snowDirty.all {
		return
	}
	d := &t.snowDirty
	tw := t.snowTilesW()
	if n := tw * t.snowTilesH(); len(d.tiles) != n {
		d.tiles = make([]bool, n)
		d.list = d.list[:0]
	}
	i := (z/SnowTile)*tw + x/SnowTile
	if !d.tiles[i] {
		d.tiles[i] = true
		d.list = append(d.list, i)
	}
}

// MarkSnowDirtyRect records a change to every cell in [x0, x1] × [z0, z1]
// (inclusive, clipped to the map).
func (t *Terrain) MarkSnowDirtyRect(x0, z0, x1, z1 int) {
	x0, z0 = max(x0, 0), max(z0, 0)
	x1, z1 = min(x1, t.Width-1), min(z1, t.Height-1)
	for z := z0 - z0%SnowTile; z <= z1; z += SnowTile {
		for x := x0 - x0%SnowTile; x <= x1; x += SnowTile {
			t.MarkSnowDirty(max(x, x0), max(z, z0))
		}
	}
}

// MarkAllSnowDirty records a change across the whole map (snowfall,
// melt, editor brushes, a new terrain).
func (t *Terrain) MarkAllSnowDirty() {
	t.snowDirty.all = true
}

// SnowDirtyAll reports whether the whole map needs a snow flush.
func (t *Terrain) SnowDirtyAll() bool { return t.snowDirty.all }

// SnowDirtyAny reports whether any snow state needs flushing.
func (t *Terrain) SnowDirtyAny() bool {
	return t.snowDirty.all || len(t.snowDirty.list) > 0
}

// ForEachSnowDirtyTile calls fn with the inclusive cell range of each tile
// marked since the last ClearSnowDirty. It doesn't visit anything when the
// whole map is marked; check SnowDirtyAll first.
func (t *Terrain) ForEachSnowDirtyTile(fn func(x0, z0, x1, z1 int)) {
	if t.snowDirty.all {
		return
	}
	tw := t.snowTilesW()
	for _, i := range t.snowDirty.list {
		x0, z0 := (i%tw)*SnowTile, (i/tw)*SnowTile
		fn(x0, z0, min(x0+SnowTile, t.Width)-1, min(z0+SnowTile, t.Height)-1)
	}
}

// ClearSnowDirty forgets every mark, after the renderer has flushed.
func (t *Terrain) ClearSnowDirty() {
	d := &t.snowDirty
	d.all = false
	for _, i := range d.list {
		d.tiles[i] = false
	}
	d.list = d.list[:0]
}
