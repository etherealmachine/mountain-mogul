package scene

import (
	"math"
	"runtime"
	"sync"

	"mountain-mogul/internal/geo"
	"mountain-mogul/internal/world"
)

// Auto material: rock or not, from the ground's shape on the 1.25 m
// detail lattice. Rock is generous: everything steeper than matRockAt,
// plus the sharp convex lips above faces. The threshold is jittered by
// noise so edges come out ragged rather than contoured. Everything else
// is meadow.
const (
	matRockAt      = 40.0 // degrees
	matLipBelow    = 12.0 // a lip is rock this many degrees under matRockAt...
	matLipCurve    = 0.3  // ...when its convexity (negative Laplacian, 1/m) is at least this
	matJitter      = 3.0  // ± degrees of noise on the threshold
	matJitterScale = 10.0 // metres, the jitter's feature size
)

// runMaterialLayer sets w's material map from its drawn ground. Maps
// without detail get none.
func runMaterialLayer(w *world.World, _ *layerCache) {
	t := w.Terrain
	if t.Detail == nil {
		t.Material = nil
		return
	}
	m := world.NewTerrainMaterial(t.Width, t.Height)
	W, H := m.W, m.H
	const per = world.CellSize / world.DetailPerCell
	h := make([]float32, W*H)
	eachRow(H, func(j int) {
		for i := 0; i < W; i++ {
			k := j*W + i
			h[k] = t.MeshGroundAt(float32(i)/world.DetailPerCell, float32(j)/world.DetailPerCell) + t.Detail.Off[k]
		}
	})

	seed := int(layerSeed(w))
	rock := make([]float32, W*H)
	at := func(i, j int) float32 { return h[min(max(j, 0), H-1)*W+min(max(i, 0), W-1)] }
	eachRow(H, func(j int) {
		for i := 0; i < W; i++ {
			k := j*W + i
			gx := float64(at(i+1, j)-at(i-1, j)) / (2 * per)
			gz := float64(at(i, j+1)-at(i, j-1)) / (2 * per)
			deg := math.Atan(math.Hypot(gx, gz)) * 180 / math.Pi
			lap := float64(at(i-2, j)+at(i+2, j)+at(i, j-2)+at(i, j+2)-4*h[k]) / (4 * per * per)
			x, z := float32(i)*per, float32(j)*per
			thr := matRockAt + (float64(fbm2D(x/matJitterScale, z/matJitterScale, 3, seed+11))-0.5)*2*matJitter
			if deg > thr || (deg > thr-matLipBelow && -lap >= matLipCurve) {
				rock[k] = 1
			}
		}
	})
	// Lidar noise speckles the mask at single samples; keep what most of
	// each sample's neighbourhood agrees on.
	geo.BoxBlur(rock, W, H, 1)
	for k, v := range rock {
		if v > 0.45 {
			m.M[k] = world.MatRock
		}
	}
	t.Material = m
}

// clearMaterial drops w's material map.
func clearMaterial(w *world.World) { w.Terrain.Material = nil }

// removeTreesOnBare removes the trees standing on rock or scree.
func removeTreesOnBare(t *world.Terrain) {
	if t.Material == nil {
		return
	}
	m := t.Material
	t.RemoveTreesIn(0, 0, t.Width-1, t.Height-1, func(tr world.Tree) bool { return m.At(tr.X, tr.Z).Bare() })
}

// eachRow runs f for rows 0..n-1 across the CPUs.
func eachRow(n int, f func(j int)) {
	var wg sync.WaitGroup
	workers := runtime.NumCPU()
	for wk := 0; wk < workers; wk++ {
		wg.Add(1)
		go func(wk int) {
			defer wg.Done()
			for j := wk; j < n; j += workers {
				f(j)
			}
		}(wk)
	}
	wg.Wait()
}
