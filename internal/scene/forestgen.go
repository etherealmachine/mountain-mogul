package scene

import (
	"math"
	"math/rand"
	"sort"

	"mountain-mogul/internal/world"
)

// GenerateTreeCover overwrites Terrain.TreeDensity with patches produced by
// multi-octave value noise plus a drainage boost from D8 flow accumulation,
// then modulated so trees thin out above the treeline and on cliff-steep
// slopes. Existing density is replaced — pair this with a "fresh" map; the
// per-cell brushes remain the way to refine afterwards.
//
// patchScale is roughly "cells per patch" of the largest octave (24 ≈ 120 m
// patches at 5 m cells). coverage in [0, 1] sets how much of the map is
// forested before treeline/slope masking — 0.5 leaves about half open.
// treelineFrac in [0, 1] is the elevation (as a fraction of the map's range)
// at which forest density tapers to zero; a 20%-of-range band straddles
// that midpoint so the treeline isn't a horizontal hard cut.
func GenerateTreeCover(t *world.Terrain, patchScale, coverage, treelineFrac float32, seed int64) {
	computeElevFields(t).generateTreeCover(t, patchScale, coverage, treelineFrac, seed)
}

// generateTreeCover is the cached-fields variant: callers that need to
// re-run the generator several times in a row (e.g. live slider drag)
// can build an *elevFields once and call this directly to skip the
// flow-accumulation and curvature passes.
func (f *elevFields) generateTreeCover(t *world.Terrain, patchScale, coverage, treelineFrac float32, seed int64) {
	if patchScale <= 0 {
		patchScale = 24
	}
	if coverage < 0 {
		coverage = 0
	} else if coverage > 1 {
		coverage = 1
	}
	if treelineFrac < 0 {
		treelineFrac = 0
	} else if treelineFrac > 1 {
		treelineFrac = 1
	}

	rng := rand.New(rand.NewSource(seed))
	hashSeed := int(rng.Int31())

	minE, maxE := f.minE, f.maxE
	span := maxE - minE
	// Treeline: density starts tapering ~10 % of the elevation range below
	// the slider value and is fully gone ~10 % above. On flat maps the
	// band collapses; the slope mask still gives variation.
	const treelineBand = float32(0.20)
	treeStart := minE + (treelineFrac-treelineBand/2)*span
	treelineMax := minE + (treelineFrac+treelineBand/2)*span

	// Drainage signal: MFD flow accumulation, log-normalised. Cells with high
	// upstream contributing area (valleys, gullies, the floor of bowls) get a
	// density boost so trees follow the watercourses instead of the patch
	// noise alone. Applied before treeline/slope masks so alpine streams and
	// near-vertical canyon walls still bare out correctly.
	//
	// Channel width tapers with elevation and slope: at low elevation / gentle
	// terrain the smoothstep range opens up so even modest flow values pick up
	// a boost (wide alluvial-fan corridors), while up at the headwaters the
	// thresholds tighten so only the strongest stems carry forest. Mirrors how
	// real drainages widen as they collect water, soil, and sediment downhill.
	flow := f.flow
	const flowBoost = float32(0.60) // max additive density along main channels
	// Headwater thresholds — tight, so only the strongest stems survive at altitude.
	const flowLowHead = float32(0.70)
	const flowHighHead = float32(0.95)
	// Alluvial-base thresholds — broad, so even modest flow paints a corridor.
	const flowLowBase = float32(0.08)
	const flowHighBase = float32(0.55)
	// Per-cell perturbation amplitude on the width factor — breaks up the
	// strict "elevation determines width" function so adjacent slopes can
	// differ in band thickness.
	const widthJitter = float32(0.35)
	widthSeedA := hashSeed + 0x55B7
	widthSeedB := hashSeed + 0xC09F

	// Concavity signal: discrete Laplacian of elevation (bowl-positive). Boosts
	// density in hollows where soil and moisture collect; subtracts on convex
	// ridges that get wind-blasted and shed soil. Normalised against the map's
	// own curvature distribution so the effect is comparable across terrains
	// with different relief.
	curv := f.curv
	const concavityBoost = float32(0.35)
	const ridgePenalty = float32(0.25)

	// Domain-warp seeds: distinct integer offsets so the X and Z warps decorrelate.
	warpSeedX := hashSeed + 0x1A4F
	warpSeedZ := hashSeed + 0x73C1
	const warpAmount = float32(1.2) // in patch widths — bigger = more swirly stand shapes

	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			fx := float32(x) / patchScale
			fz := float32(z) / patchScale

			// Domain warp: displace the noise sample point with a low-frequency
			// noise field. Turns the round FBM blobs into elongated, swirling
			// stand shapes that look organic instead of cellular.
			wx := fbm2D(fx*0.5, fz*0.5, 2, warpSeedX) - 0.5
			wz := fbm2D(fx*0.5, fz*0.5, 2, warpSeedZ) - 0.5
			n := fbm2D(fx+wx*warpAmount, fz+wz*warpAmount, 4, hashSeed)

			// Re-map noise so the bottom (1 - coverage) of the range clamps to
			// 0 (open meadow). The remainder ramps toward 1.
			floor := 1 - coverage
			d := float32(0)
			if coverage > 0 && n > floor {
				d = (n - floor) / coverage
			}

			// Drainage boost: smoothstep ramp on log-normalised flow, weighted.
			// Width factor in [0, 1]: 0 at headwaters (high + steep), 1 in the
			// alluvial base (low + flat). Combines elevation and local slope so
			// gentle high benches still get some widening and steep low gullies
			// stay narrow. Two low-freq noise samples nudge the factor by
			// ±widthJitter so the taper isn't a perfect function of elevation —
			// real watersheds have wide reaches at altitude and tight choke
			// points downstream too.
			elev := t.Cells[x][z].GroundElevation
			slope := slopeAt(t, x, z)
			elevFrac := float32(0)
			if span > 0 {
				elevFrac = (elev - minE) / span
				if elevFrac < 0 {
					elevFrac = 0
				} else if elevFrac > 1 {
					elevFrac = 1
				}
			}
			baseness := 1 - elevFrac
			flatness := 1 - smoothstep32(0.05, 0.4, slope)
			width := 0.5*baseness + 0.5*flatness
			// Combine two low-freq noise samples (different seeds, scales) for
			// a richer perturbation than a single octave would give.
			wn := (valueNoise2D(fx*0.4, fz*0.4, widthSeedA)*0.65 +
				valueNoise2D(fx*1.1, fz*1.1, widthSeedB)*0.35) - 0.5
			width += wn * 2 * widthJitter // *2 because (noise-0.5) ranges ±0.5
			if width < 0 {
				width = 0
			} else if width > 1 {
				width = 1
			}
			flowLowVar := flowLowHead + (flowLowBase-flowLowHead)*width
			flowHighVar := flowHighHead + (flowHighBase-flowHighHead)*width
			fl := flow[x*t.Height+z]
			fb := smoothstep32(flowLowVar, flowHighVar, fl) * flowBoost
			d += fb

			// Concavity bias: positive curvature in hollows, negative on ridges.
			c := curv[x*t.Height+z]
			if c > 0 {
				d += smoothstep32(0.15, 0.85, c) * concavityBoost
			} else if c < 0 {
				d -= smoothstep32(0.15, 0.85, -c) * ridgePenalty
			}

			if elev >= treelineMax {
				d = 0
			} else if elev > treeStart && treelineMax > treeStart {
				d *= 1 - (elev-treeStart)/(treelineMax-treeStart)
			}

			// Slope mask (rise/run, dimensionless). 0.7 ≈ 35°, 1.2 ≈ 50°.
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

// flowAccumulation runs MFD (multiple-flow-direction) accumulation over the
// terrain's elevation grid. Each cell distributes its upstream area to ALL
// downhill neighbours weighted by slope^p (Quinn et al. 1991, p≈1.1) — so a
// cell with two equally-steep downhill drops splits 50/50 and a cell with a
// clear winner mostly behaves like D8. This eliminates the 0°/45° grid-axis
// channels that single-flow D8 produces, giving feathered, organic-looking
// drainage patterns. The accumulated counts are then log-normalised to
// [0, 1] (raw counts span orders of magnitude) and gently blurred to soften
// any residual stair-stepping along the grid.
//
// Pits (cells with no downhill neighbour) collect flow but don't propagate
// it. We don't pit-fill: procedural heightfields rarely have large basins,
// and the boost behaviour at a true pit (a high-density bowl) is the right
// answer for forests anyway.
func flowAccumulation(t *world.Terrain) []float32 {
	n := t.Width * t.Height
	acc := make([]float32, n)
	for i := range acc {
		acc[i] = 1
	}

	// Index → cell. Sorted descending by elevation so the propagation walks
	// peaks-first and every receiver has its own contributions ready when
	// its turn comes.
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool {
		ax, az := order[a]/t.Height, order[a]%t.Height
		bx, bz := order[b]/t.Height, order[b]%t.Height
		return t.Cells[ax][az].GroundElevation > t.Cells[bx][bz].GroundElevation
	})

	// 8 neighbour offsets with inverse Euclidean distance — diagonals get
	// down-weighted in the steepest-descent search, matching the geometry.
	type dir struct {
		dx, dz  int
		invDist float32
	}
	const invDiag = float32(1.0 / math.Sqrt2)
	dirs := [8]dir{
		{-1, -1, invDiag}, {0, -1, 1}, {1, -1, invDiag},
		{-1, 0, 1}, {1, 0, 1},
		{-1, 1, invDiag}, {0, 1, 1}, {1, 1, invDiag},
	}

	const mfdExponent = 1.1
	for _, idx := range order {
		x := idx / t.Height
		z := idx % t.Height
		e := t.Cells[x][z].GroundElevation
		var weights [8]float32
		totalWeight := float32(0)
		for i, d := range dirs {
			nx, nz := x+d.dx, z+d.dz
			if nx < 0 || nx >= t.Width || nz < 0 || nz >= t.Height {
				continue
			}
			drop := e - t.Cells[nx][nz].GroundElevation
			if drop <= 0 {
				continue
			}
			slope := drop * d.invDist
			w := float32(math.Pow(float64(slope), mfdExponent))
			weights[i] = w
			totalWeight += w
		}
		if totalWeight <= 0 {
			continue // pit — flow stops here
		}
		inv := 1 / totalWeight
		self := acc[idx]
		for i, w := range weights {
			if w == 0 {
				continue
			}
			d := dirs[i]
			ni := (x+d.dx)*t.Height + (z + d.dz)
			acc[ni] += self * w * inv
		}
	}

	// Log-normalise to [0, 1].
	out := make([]float32, n)
	maxLog := float32(0)
	for i, a := range acc {
		v := float32(math.Log1p(float64(a)))
		out[i] = v
		if v > maxLog {
			maxLog = v
		}
	}
	if maxLog > 0 {
		inv := 1 / maxLog
		for i := range out {
			out[i] *= inv
		}
	}

	// Gentle [1, 2, 1] separable blur to soften any remaining grid alignment.
	// Pits and tight channels can still leave near-axis ribbons even with MFD;
	// one pass is enough to break them up without erasing the broad signal.
	out = blur121(t, out)
	return out
}

// blur121 applies a single-pass separable [1, 2, 1] / 4 blur with edge-clamped
// boundaries. Cheap and just enough to take the curse off grid-aligned
// stair-stepping in the flow field.
func blur121(t *world.Terrain, in []float32) []float32 {
	n := t.Width * t.Height
	horiz := make([]float32, n)
	for x := 0; x < t.Width; x++ {
		xl := x - 1
		if xl < 0 {
			xl = 0
		}
		xr := x + 1
		if xr >= t.Width {
			xr = t.Width - 1
		}
		for z := 0; z < t.Height; z++ {
			horiz[x*t.Height+z] = (in[xl*t.Height+z] + 2*in[x*t.Height+z] + in[xr*t.Height+z]) * 0.25
		}
	}
	out := make([]float32, n)
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			zu := z - 1
			if zu < 0 {
				zu = 0
			}
			zd := z + 1
			if zd >= t.Height {
				zd = t.Height - 1
			}
			out[x*t.Height+z] = (horiz[x*t.Height+zu] + 2*horiz[x*t.Height+z] + horiz[x*t.Height+zd]) * 0.25
		}
	}
	return out
}

// curvature returns the per-cell discrete Laplacian of elevation, normalised
// to roughly [-1, 1] against the map's own curvature distribution. Positive
// values mean concave (bowl-like, hollow) cells; negative values mean convex
// (ridge-like) cells. The Laplacian's raw magnitude depends on absolute
// elevation differences which scale with terrain ruggedness, so we divide
// each side by the strongest signed extreme — that way a gently rolling
// landscape and a dramatic alpine ridge both produce comparable values.
func curvature(t *world.Terrain) []float32 {
	n := t.Width * t.Height
	out := make([]float32, n)
	maxPos := float32(0)
	maxNeg := float32(0)
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			c := t.Cells[x][z].GroundElevation
			// 4-neighbour Laplacian with edge clamping. (e_left + e_right +
			// e_up + e_down) - 4·e_center; positive when neighbours are
			// higher than centre (a hollow), negative on a peak.
			xl := x - 1
			if xl < 0 {
				xl = 0
			}
			xr := x + 1
			if xr >= t.Width {
				xr = t.Width - 1
			}
			zu := z - 1
			if zu < 0 {
				zu = 0
			}
			zd := z + 1
			if zd >= t.Height {
				zd = t.Height - 1
			}
			lap := t.Cells[xl][z].GroundElevation + t.Cells[xr][z].GroundElevation +
				t.Cells[x][zu].GroundElevation + t.Cells[x][zd].GroundElevation -
				4*c
			out[x*t.Height+z] = lap
			if lap > maxPos {
				maxPos = lap
			} else if -lap > maxNeg {
				maxNeg = -lap
			}
		}
	}
	for i, v := range out {
		if v > 0 && maxPos > 0 {
			out[i] = v / maxPos
		} else if v < 0 && maxNeg > 0 {
			out[i] = v / maxNeg // result still negative, magnitude in [0, 1]
		}
	}
	return out
}

// smoothstep32 is the standard Hermite ramp from 0 to 1 between edge0 and
// edge1. Inlined elsewhere in the codebase as raw arithmetic; named here
// for readability since we use it multiple ways.
func smoothstep32(edge0, edge1, x float32) float32 {
	if edge1 <= edge0 {
		if x < edge0 {
			return 0
		}
		return 1
	}
	t := (x - edge0) / (edge1 - edge0)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
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
		dx = (t.Cells[x1][z].GroundElevation - t.Cells[x0][z].GroundElevation) / dxRun
	}
	dz := float32(0)
	if dzRun > 0 {
		dz = (t.Cells[x][z1].GroundElevation - t.Cells[x][z0].GroundElevation) / dzRun
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
