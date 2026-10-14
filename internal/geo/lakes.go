package geo

import (
	"math"
	"slices"

	"mountain-mogul/internal/world"
)

// Lakes: OpenStreetMap lake and pond outlines levelled to their water
// surface. Lidar over water is the surface plus noise (or a fill where
// there were no returns), so the level is a low percentile of the
// heights inside the outline: below the noise, above any pits.
const lakeLevelPercentile = 0.10

// LakeMask marks the samples of a w × ht lattice over b (row 0 at
// MaxLat, column 0 at MinLon) inside any lake, by even-odd scanlines
// over every edge of every outline, so a multipolygon's outer ring split
// across several ways still fills. Index is the lake's position in
// lakes plus 1; 0 is dry ground.
func LakeMask(w, ht int, b Bounds, lakes []world.BaseLake) []int32 {
	mask := make([]int32, w*ht)
	toLattice := func(p [2]float64) (float64, float64) {
		return (p[1] - b.MinLon) / (b.MaxLon - b.MinLon) * float64(w-1),
			(b.MaxLat - p[0]) / (b.MaxLat - b.MinLat) * float64(ht-1)
	}
	type edge struct{ x0, y0, x1, y1 float64 }
	var xs []float64
	for li, l := range lakes {
		var edges []edge
		minY, maxY := math.Inf(1), math.Inf(-1)
		for _, path := range l.Paths {
			for s := 1; s < len(path); s++ {
				x0, y0 := toLattice(path[s-1])
				x1, y1 := toLattice(path[s])
				edges = append(edges, edge{x0, y0, x1, y1})
				minY, maxY = min(minY, y0, y1), max(maxY, y0, y1)
			}
		}
		j0, j1 := max(int(math.Ceil(minY)), 0), min(int(math.Floor(maxY)), ht-1)
		for j := j0; j <= j1; j++ {
			y := float64(j)
			xs = xs[:0]
			for _, e := range edges {
				if (e.y0 <= y) != (e.y1 <= y) {
					xs = append(xs, e.x0+(y-e.y0)/(e.y1-e.y0)*(e.x1-e.x0))
				}
			}
			slices.Sort(xs)
			for k := 0; k+1 < len(xs); k += 2 {
				i0, i1 := max(int(math.Ceil(xs[k])), 0), min(int(math.Floor(xs[k+1])), w-1)
				for i := i0; i <= i1; i++ {
					mask[j*w+i] = int32(li + 1)
				}
			}
		}
	}
	return mask
}

// LakeAreaHa is l's whole area in hectares, off the map too: the
// largest of its outlines by the shoelace formula, in local metres.
func LakeAreaHa(l world.BaseLake) float64 {
	var best float64
	for _, path := range l.Paths {
		if len(path) < 3 {
			continue
		}
		latM, lonM := MetresPerDegree(path[0][0])
		var a float64
		for k := range path {
			p, q := path[k], path[(k+1)%len(path)]
			a += p[1]*lonM*q[0]*latM - q[1]*lonM*p[0]*latM
		}
		best = max(best, math.Abs(a)/2)
	}
	return best / 10000
}

// An OpenStreetMap outline is drawn by hand over imagery: a few metres
// off, with straight cuts across the bends. WaterMask fits it to the
// ground, where lidar over water is flat at the surface: it trims the
// outline's edge where it takes in ground more than lakeShoreAbove over
// the level (up to lakeTrimM in), then grows it over connected ground
// within the water's surface (lakeFlatBelow under the level to
// lakeFlatAbove over) up to lakeGrowM out. Ground well under the level,
// past a rim where an outlet falls away, stays dry.
const (
	lakeShoreAbove = 0.5
	lakeFlatAbove  = 0.3
	lakeFlatBelow  = 0.4
	lakeTrimM      = 6.0
	lakeGrowM      = 15.0
)

// WaterMask is LakeMask fitted to the ground (see lakeShoreAbove): the
// samples of the w × ht heights h, spacing metres apart, that are each
// lake's water, and each lake's level, a low percentile of the heights
// inside its outline (lakeLevelPercentile). The Lakes layer levels these
// samples, and the lake setup reads them off the levelled ground, so the
// two agree.
func WaterMask(h []float32, w, ht int, spacing float64, b Bounds, lakes []world.BaseLake) ([]int32, []float32) {
	mask := LakeMask(w, ht, b, lakes)
	inside := make([][]float32, len(lakes))
	for k, li := range mask {
		if li > 0 {
			inside[li-1] = append(inside[li-1], h[k])
		}
	}
	level := make([]float32, len(lakes))
	for li, hs := range inside {
		if len(hs) == 0 {
			continue
		}
		slices.Sort(hs)
		level[li] = hs[int(float64(len(hs)-1)*lakeLevelPercentile)]
	}
	neighbours := func(k int, f func(n int)) {
		i, j := k%w, k/w
		if i > 0 {
			f(k - 1)
		}
		if i < w-1 {
			f(k + 1)
		}
		if j > 0 {
			f(k - w)
		}
		if j < ht-1 {
			f(k + w)
		}
	}
	// Trim the shore off the outline's edge, a sample deeper each pass.
	var cut []int
	for pass := 0; pass < max(int(lakeTrimM/spacing+0.5), 1); pass++ {
		cut = cut[:0]
		for k, li := range mask {
			if li == 0 || h[k] <= level[li-1]+lakeShoreAbove {
				continue
			}
			edge := false
			neighbours(k, func(n int) { edge = edge || mask[n] != li })
			if edge {
				cut = append(cut, k)
			}
		}
		if len(cut) == 0 {
			break
		}
		for _, k := range cut {
			mask[k] = 0
		}
	}
	// Grow over the flat water the outline missed, breadth first.
	steps := max(int(lakeGrowM/spacing+0.5), 1)
	dist := make([]int32, len(mask))
	var queue []int
	for k, li := range mask {
		if li > 0 {
			queue = append(queue, k)
		}
	}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		li := mask[k]
		if int(dist[k]) >= steps {
			continue
		}
		neighbours(k, func(n int) {
			if mask[n] != 0 {
				return
			}
			if d := h[n] - level[li-1]; d < -lakeFlatBelow || d > lakeFlatAbove {
				return
			}
			mask[n], dist[n] = li, dist[k]+1
			queue = append(queue, n)
		})
	}
	return mask, level
}
