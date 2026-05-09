package scene

import (
	"math"
	"math/rand"

	"mountain-mogul/internal/world"
)

// generateTreeCover overwrites Terrain.TreeDensity with patches produced by
// multi-octave value noise, modulated so trees thin out above the treeline
// and on cliff-steep slopes. Existing density is replaced — pair this with
// a "fresh" map; the per-cell brushes remain the way to refine afterwards.
//
// patchScale is roughly "cells per patch" of the largest octave (24 ≈ 120 m
// patches at 5 m cells). coverage in [0, 1] sets how much of the map is
// forested before treeline/slope masking — 0.5 leaves about half open.
func generateTreeCover(t *world.Terrain, patchScale, coverage float32, seed int64) {
	if patchScale <= 0 {
		patchScale = 24
	}
	if coverage < 0 {
		coverage = 0
	} else if coverage > 1 {
		coverage = 1
	}

	rng := rand.New(rand.NewSource(seed))
	hashSeed := int(rng.Int31())

	minE, maxE := float32(math.Inf(1)), float32(math.Inf(-1))
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			e := t.Cells[x][z].Elevation
			if e < minE {
				minE = e
			}
			if e > maxE {
				maxE = e
			}
		}
	}
	span := maxE - minE
	// Treeline: density tapers from `treeStart` and is fully gone by `treelineMax`.
	// On flat maps the band collapses; the slope mask still gives variation.
	treeStart := minE + 0.65*span
	treelineMax := minE + 0.85*span

	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			fx := float32(x) / patchScale
			fz := float32(z) / patchScale
			n := fbm2D(fx, fz, 4, hashSeed)

			// Re-map noise so the bottom (1 - coverage) of the range clamps to
			// 0 (open meadow). The remainder ramps toward 1.
			floor := 1 - coverage
			d := float32(0)
			if coverage > 0 && n > floor {
				d = (n - floor) / coverage
			}

			elev := t.Cells[x][z].Elevation
			if elev >= treelineMax {
				d = 0
			} else if elev > treeStart && treelineMax > treeStart {
				d *= 1 - (elev-treeStart)/(treelineMax-treeStart)
			}

			// Slope (rise/run, dimensionless). 0.7 ≈ 35°, 1.2 ≈ 50°.
			slope := slopeAt(t, x, z)
			const slopeStart = float32(0.7)
			const slopeEnd = float32(1.2)
			if slope >= slopeEnd {
				d = 0
			} else if slope > slopeStart {
				d *= 1 - (slope-slopeStart)/(slopeEnd-slopeStart)
			}

			if d < 0 {
				d = 0
			} else if d > 1 {
				d = 1
			}
			t.Cells[x][z].TreeDensity = d
		}
	}
}

// slopeAt returns the magnitude of the elevation gradient at cell (x, z),
// in metres of rise per metre of run. Central differences at the edges
// degrade gracefully via clamping.
func slopeAt(t *world.Terrain, x, z int) float32 {
	const cellSize = float32(5.0)
	x0, x1 := x-1, x+1
	if x0 < 0 {
		x0 = 0
	}
	if x1 >= t.Width {
		x1 = t.Width - 1
	}
	z0, z1 := z-1, z+1
	if z0 < 0 {
		z0 = 0
	}
	if z1 >= t.Height {
		z1 = t.Height - 1
	}
	dxRun := float32(x1-x0) * cellSize
	dzRun := float32(z1-z0) * cellSize
	dx := float32(0)
	if dxRun > 0 {
		dx = (t.Cells[x1][z].Elevation - t.Cells[x0][z].Elevation) / dxRun
	}
	dz := float32(0)
	if dzRun > 0 {
		dz = (t.Cells[x][z1].Elevation - t.Cells[x][z0].Elevation) / dzRun
	}
	return float32(math.Sqrt(float64(dx*dx + dz*dz)))
}

// hash22 returns a deterministic float in [0, 1) from a 2D integer lattice
// point. Cheap integer mixing — good enough for value noise, not crypto.
func hash22(x, y, seed int) float32 {
	h := uint32(int32(x))*374761393 + uint32(int32(y))*668265263 + uint32(int32(seed))*2147483647
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float32(h&0x7fffffff) / float32(0x7fffffff)
}

// valueNoise2D bilinearly interpolates lattice hashes with a smoothstep
// fade — cheaper than Perlin, plenty smooth for density patches.
func valueNoise2D(x, y float32, seed int) float32 {
	xi := int(math.Floor(float64(x)))
	yi := int(math.Floor(float64(y)))
	fx := x - float32(xi)
	fy := y - float32(yi)
	sx := fx * fx * (3 - 2*fx)
	sy := fy * fy * (3 - 2*fy)
	n00 := hash22(xi, yi, seed)
	n10 := hash22(xi+1, yi, seed)
	n01 := hash22(xi, yi+1, seed)
	n11 := hash22(xi+1, yi+1, seed)
	return (1-sy)*((1-sx)*n00+sx*n10) + sy*((1-sx)*n01+sx*n11)
}

// fbm2D is multi-octave value noise — adding finer octaves breaks up patch
// edges so they don't all share the same blob shape.
func fbm2D(x, y float32, octaves, seed int) float32 {
	amp := float32(1)
	freq := float32(1)
	sum, norm := float32(0), float32(0)
	for i := 0; i < octaves; i++ {
		sum += amp * valueNoise2D(x*freq, y*freq, seed+i)
		norm += amp
		amp *= 0.5
		freq *= 2
	}
	if norm == 0 {
		return 0
	}
	return sum / norm
}
