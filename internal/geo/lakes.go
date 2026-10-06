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

// FlattenLakes levels the w × ht heights h inside each lake to its
// water surface, in place.
func FlattenLakes(h []float32, w, ht int, b Bounds, lakes []world.BaseLake) {
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
	for k, li := range mask {
		if li > 0 {
			h[k] = level[li-1]
		}
	}
}
