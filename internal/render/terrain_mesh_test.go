package render

import (
	"math"
	"math/rand"
	"testing"

	"mountain-mogul/internal/world"
)

// testTerrain is uneven ground with varied snow, trees, and detail, sized
// so it spans several chunks.
func testTerrain(n int) *world.Terrain {
	tr := world.NewTerrain(n, n)
	rng := rand.New(rand.NewSource(1))
	for x := 0; x < n; x++ {
		for z := 0; z < n; z++ {
			c := &tr.Cells[x][z]
			c.GroundElevation = 2000 + float32(x)*1.7 - float32(z)*0.9 + rng.Float32()*3
			c.Slope = 0.6 + float32((x+z)%5)*0.05
			c.Top.Accumulation = 0.1 + rng.Float32()*0.5
			tr.SetCellTreesFromDensity(x, z, float32((x*3+z)%4)/3)
		}
	}
	tr.FillDetailTest()
	return tr
}

// Chunks must cover every cell's two triangles exactly once, in the
// checkerboard layout VisualElevationAt assumes.
func TestTerrainChunksCoverGrid(t *testing.T) {
	const n = 70
	tr := testTerrain(n)
	verts, indices, chunks, _, _ := buildTerrainGeometry(tr)
	seen := make(map[[3][2]int]int)
	for _, c := range chunks {
		if c.always {
			continue
		}
		for i := c.first; i < c.first+c.count; i += 3 {
			var key [3][2]int
			for k := 0; k < 3; k++ {
				v := int(indices[i+int32(k)]) * terrainFloatsPerVert
				key[k] = [2]int{int(verts[v+3]), int(verts[v+4])}
			}
			seen[key]++
		}
	}
	for x := 0; x < n-1; x++ {
		for z := 0; z < n-1; z++ {
			for _, tri := range cellTris(x, z) {
				var key [3][2]int
				for k, o := range tri {
					key[k] = [2]int{x + o[0], z + o[1]}
				}
				if seen[key] != 1 {
					t.Fatalf("cell (%d, %d) triangle %v drawn %d times", x, z, key, seen[key])
				}
			}
		}
	}
	if len(seen) != 2*(n-1)*(n-1) {
		t.Fatalf("%d surface triangles, want %d", len(seen), 2*(n-1)*(n-1))
	}
}

// VisualElevationAt must land on the surface the GPU draws: the mesh
// triangle's ground, plus snow depth from the corner textures, plus
// detail, all interpolated at the point.
func TestVisualElevationMatchesMesh(t *testing.T) {
	const n = 40
	tr := testTerrain(n)
	verts, indices, chunks, _, _ := buildTerrainGeometry(tr)
	snowA := make([]float32, n*n*4)
	snowB := make([]float32, n*n*2)
	cornerSnow(tr, 0, 0, n-1, n-1, snowA, snowB)

	rng := rand.New(rand.NewSource(2))
	checked := 0
	for _, c := range chunks {
		if c.always {
			continue
		}
		for i := c.first; i < c.first+c.count; i += 3 {
			var ps [3][2]float32
			var ys, gxs, gzs [3]float32
			for k := 0; k < 3; k++ {
				v := int(indices[i+int32(k)]) * terrainFloatsPerVert
				ps[k] = [2]float32{verts[v], verts[v+2]}
				gx, gz := int(verts[v+3]), int(verts[v+4])
				ys[k] = verts[v+1] + snowB[(gz*n+gx)*2]
				gxs[k], gzs[k] = float32(gx), float32(gz)
			}
			// A point well inside the triangle.
			a, b := 0.1+0.8*rng.Float32(), 0.1+0.8*rng.Float32()
			if a+b > 0.9 {
				a, b = 0.9-a*0.5, 0.9-b*0.5
				if a+b > 0.9 {
					a, b = 0.3, 0.3
				}
			}
			w := [3]float32{1 - a - b, a, b}
			var px, pz, want, gx, gz float32
			for k := 0; k < 3; k++ {
				px += w[k] * ps[k][0]
				pz += w[k] * ps[k][1]
				want += w[k] * ys[k]
				gx += w[k] * gxs[k]
				gz += w[k] * gzs[k]
			}
			want += tr.Detail.At(gx, gz)
			got := VisualElevationAt(tr, px, pz)
			if math.Abs(float64(got-want)) > 1e-3 {
				t.Fatalf("at (%.2f, %.2f): VisualElevationAt %.4f, mesh %.4f", px, pz, got, want)
			}
			checked++
		}
	}
	if checked != 2*(n-1)*(n-1) {
		t.Fatalf("checked %d triangles", checked)
	}
}

// Refreshing a block of corners must write what a full refresh would.
func TestCornerSnowBlockMatchesFull(t *testing.T) {
	const n = 30
	tr := testTerrain(n)
	fullA := make([]float32, n*n*4)
	fullB := make([]float32, n*n*2)
	cornerSnow(tr, 0, 0, n-1, n-1, fullA, fullB)

	x0, z0, x1, z1 := 6, 4, 11, 9
	w := x1 - x0 + 1
	a := make([]float32, w*(z1-z0+1)*4)
	b := make([]float32, w*(z1-z0+1)*2)
	cornerSnow(tr, x0, z0, x1, z1, a, b)
	nonzero := false
	for z := z0; z <= z1; z++ {
		for x := x0; x <= x1; x++ {
			i, j := (z-z0)*w+(x-x0), z*n+x
			for k := 0; k < 4; k++ {
				if a[i*4+k] != fullA[j*4+k] {
					t.Fatalf("corner (%d, %d) A[%d]: block %v, full %v", x, z, k, a[i*4+k], fullA[j*4+k])
				}
			}
			for k := 0; k < 2; k++ {
				if b[i*2+k] != fullB[j*2+k] {
					t.Fatalf("corner (%d, %d) B[%d]: block %v, full %v", x, z, k, b[i*2+k], fullB[j*2+k])
				}
			}
			nonzero = nonzero || b[i*2+1] != 0
		}
	}
	if !nonzero {
		t.Fatal("test terrain has no instability; nothing was checked")
	}
}
