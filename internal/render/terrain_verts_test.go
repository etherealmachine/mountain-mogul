package render

import (
	"testing"

	"mountain-mogul/internal/world"
)

// Patching instability after a tree-density edit must leave the vertex
// stream exactly as a full rebuild would.
func TestPatchInstabilityMatchesRebuild(t *testing.T) {
	const n = 20
	tr := world.NewTerrain(n, n)
	for x := 0; x < n; x++ {
		for z := 0; z < n; z++ {
			c := &tr.Cells[x][z]
			c.Slope = 0.6 + float32((x+z)%5)*0.05
			c.Top.Accumulation = 0.3
			c.TreeDensity = float32((x*3+z)%4) / 3
		}
	}
	verts, _, _, _, surfaceVerts := buildTerrainVerts(tr)

	for x := 6; x <= 10; x++ {
		for z := 4; z <= 8; z++ {
			tr.Cells[x][z].TreeDensity *= 0.2
		}
	}
	first, last := patchInstability(verts, surfaceVerts, tr, 6, 4, 10, 8)
	want, _, _, _, _ := buildTerrainVerts(tr)

	if first >= last || last-first >= surfaceVerts*17/2 {
		t.Errorf("patched range [%d, %d) of %d floats; want a small non-empty band", first, last, surfaceVerts*17)
	}
	changed := false
	for i := range want {
		if verts[i] != want[i] {
			t.Fatalf("float %d (vertex %d, attr %d): patched %v, rebuilt %v", i, i/17, i%17, verts[i], want[i])
		}
		if i%17 == 16 && i >= first && i < last && want[i] != 0 {
			changed = true
		}
	}
	if !changed {
		t.Fatalf("test terrain has no instability; nothing was checked")
	}
}
