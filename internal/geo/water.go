package geo

import (
	"math"
	"slices"

	"mountain-mogul/internal/world"
)

// The map's water (world.Water): OpenStreetMap's creeks and lakes fitted
// to the ground once, by the Water layer, and kept as world-space lines
// and outlines that the Creeks and Lakes layers build from from then on,
// and the scenario editor changes.

// FitWater fits base's OpenStreetMap water to the w × ht ground h,
// spacing metres apart, into water: the creeks (SeedCreeks), then the
// lakes on the ground with those creeks carved (SeedLakes), as the Creeks
// and Lakes layers will build them.
func FitWater(h []float32, w, ht int, spacing float64, base *world.TerrainBase, water *world.Water) {
	water.Creeks = SeedCreeks(h, w, ht, spacing, base)
	water.Lakes = nil
	carved := append([]float32(nil), h...)
	CarveCreeks(carved, w, ht, spacing, WaterCreekStreams(carved, w, ht, spacing, base, water))
	water.Lakes = SeedLakes(carved, w, ht, spacing, base)
	water.Fitted = true
}

// creekJoin is how near, in metres, a creek line's end must come to
// another creek's line to be joined onto it: OpenStreetMap draws one
// creek as several lines meeting end to end, and tributaries ending on
// the creek they feed, and each line's ends move across their own
// channel when it's refined, leaving gaps.
const creekJoin = 8.0

// SeedCreeks fits base's OpenStreetMap streams to the w × ht ground h,
// spacing metres apart (CreekStreams: traced onto the cell grid's
// channels, then refined onto the fine ground's), keeping only where
// they flow, with ends that lie near another creek's line joined back
// onto it, simplified (creekSimplify).
func SeedCreeks(h []float32, w, ht int, spacing float64, base *world.TerrainBase) []world.WaterCreek {
	var kept []Stream
	var lines [][][2]float32
	for _, s := range CreekStreams(h, w, ht, spacing, base) {
		// Only where it flows: OpenStreetMap often runs a creek's line up
		// the dry gully above where water starts.
		first := 0
		for first < len(s.Points) && !s.Flowing(first) {
			first++
		}
		if len(s.Points)-first < 2 {
			continue
		}
		kept = append(kept, s)
		lines = append(lines, append([][2]float32(nil), s.Points[first:]...))
	}
	joinCreekEnds(lines)
	out := make([]world.WaterCreek, 0, len(kept))
	for i, s := range kept {
		out = append(out, world.WaterCreek{Name: s.Name, Kind: s.Kind, Type: world.CreekTypeOf(s.Kind), Points: simplifyLine(easeLine(lines[i]), creekSimplify)})
	}
	return out
}

// easeLine takes the side-to-side wobble out of a line refined point by
// point onto its channel, which the creek's curve (world.WaterCreek.Curve)
// would otherwise follow: each point moves to a quarter of the way to
// the mean of its neighbours and back, twice, keeping the ends (where
// creeks join).
func easeLine(ps [][2]float32) [][2]float32 {
	out := append([][2]float32(nil), ps...)
	for pass := 0; pass < 2; pass++ {
		prev := append([][2]float32(nil), out...)
		for i := 1; i+1 < len(out); i++ {
			for k := range 2 {
				out[i][k] = (prev[i-1][k] + 2*prev[i][k] + prev[i+1][k]) / 4
			}
		}
	}
	return out
}

// joinCreekEnds moves each line's ends onto the nearest point of another
// line within creekJoin, in place.
func joinCreekEnds(lines [][][2]float32) {
	for i, l := range lines {
		for _, e := range []int{0, len(l) - 1} {
			p := l[e]
			best, bestD := p, float32(creekJoin)
			for j, o := range lines {
				if j == i {
					continue
				}
				for k := 1; k < len(o); k++ {
					q := nearestOnSegment(o[k-1], o[k], p)
					if d := dist2(p, q); d < bestD {
						best, bestD = q, d
					}
				}
			}
			l[e] = best
		}
	}
}

func nearestOnSegment(a, b, p [2]float32) [2]float32 {
	dx, dz := b[0]-a[0], b[1]-a[1]
	l2 := dx*dx + dz*dz
	if l2 == 0 {
		return a
	}
	u := min(max(((p[0]-a[0])*dx+(p[1]-a[1])*dz)/l2, 0), 1)
	return [2]float32{a[0] + u*dx, a[1] + u*dz}
}

func dist2(a, b [2]float32) float32 {
	return float32(math.Hypot(float64(a[0]-b[0]), float64(a[1]-b[1])))
}

// CreekLines is creeks as Streams on the cell grid (h the ground at its
// w × ht points, area its Catchment, lake its lakes' points or nil): each
// point's catchment the most draining through a grid point beside it,
// grown downstream, and at least StreamStartArea: every creek in the
// water flows its whole length, wherever the editor has put it, and its
// catchment only sizes its channel (CreekShape). Each runs along its
// curve (world.WaterCreek.Curve).
func CreekLines(creeks []world.WaterCreek, w, ht int, area []float32, lake []bool) []Stream {
	var out []Stream
	for _, c := range creeks {
		if len(c.Points) < 2 {
			continue
		}
		s := Stream{Name: c.Name, Kind: c.Kind, Type: c.Type, Points: c.Curve()}
		for _, p := range s.Points {
			ci := min(max(int(math.Round(float64(p[0]/world.CellSize))), 0), w-1)
			cj := min(max(int(math.Round(float64(p[1]/world.CellSize))), 0), ht-1)
			best := float32(0)
			for dj := -1; dj <= 1; dj++ {
				for di := -1; di <= 1; di++ {
					ni, nj := ci+di, cj+dj
					if ni >= 0 && nj >= 0 && ni < w && nj < ht {
						best = max(best, area[nj*w+ni])
					}
				}
			}
			s.Catchment = append(s.Catchment, best)
		}
		s.Catchment[0] = max(s.Catchment[0], StreamStartArea)
		finishStream(&s, w, ht, lake)
		out = append(out, s)
	}
	return out
}

// WaterCreekStreams is water's creeks as Streams for the w × ht ground
// h, spacing metres apart: CreekLines over the cell grid's catchment, as
// CreekStreams works it out.
func WaterCreekStreams(h []float32, w, ht int, spacing float64, base *world.TerrainBase, water *world.Water) []Stream {
	coarse, cw, ch := coarseGround(h, w, ht, spacing)
	area := Catchment(coarse, cw, ch, world.CellSize)
	return CreekLines(water.Creeks, cw, ch, area, coarseLakes(cw, ch, base, water))
}

// coarseGround is h, spacing metres apart, at the cell grid's points.
func coarseGround(h []float32, w, ht int, spacing float64) ([]float32, int, int) {
	step := max(int(math.Round(world.CellSize/spacing)), 1)
	cw, ch := (w-1)/step+1, (ht-1)/step+1
	coarse := make([]float32, cw*ch)
	for j := 0; j < ch; j++ {
		for i := 0; i < cw; i++ {
			coarse[j*cw+i] = h[j*step*w+i*step]
		}
	}
	return coarse, cw, ch
}

// coarseLakes marks the lakes on the cw × ch cell grid: water's, else
// (while it's being fitted) OpenStreetMap's; nil for none.
func coarseLakes(cw, ch int, base *world.TerrainBase, water *world.Water) []bool {
	var mask []int32
	switch {
	case len(water.Lakes) > 0:
		mask = ShapeMask(cw, ch, world.CellSize, water.Lakes)
	case !water.Fitted && len(base.Lakes) > 0:
		mask = LakeMask(cw, ch, boundsOf(base.Geo), base.Lakes)
	default:
		return nil
	}
	lake := make([]bool, cw*ch)
	for k, li := range mask {
		lake[k] = li > 0
	}
	return lake
}

// SeedLakes fits base's OpenStreetMap lakes to the w × ht ground h,
// spacing metres apart (WaterMask), and traces each one's water into
// outline rings round its samples, simplified to within lakeSimplify of
// a sample's edge. A lake with no water on the map is left out.
func SeedLakes(h []float32, w, ht int, spacing float64, base *world.TerrainBase) []world.WaterLake {
	mask, _ := WaterMask(h, w, ht, spacing, boundsOf(base.Geo), base.Lakes)
	var out []world.WaterLake
	for li, bl := range base.Lakes {
		rings := traceRings(mask, w, ht, int32(li+1), float32(spacing))
		if len(rings) == 0 {
			continue
		}
		ha := float32(LakeAreaHa(bl))
		out = append(out, world.WaterLake{Name: bl.Name, Kind: bl.Kind, Type: world.LakeTypeOf(bl.Kind, ha), AreaHa: ha, Rings: rings})
	}
	return out
}

// Seeded water is simplified for editing by hand: a lake's traced
// outline to within lakeSimplify metres of the samples' edges, a creek's
// refined line to within creekSimplify of its points, leaving a point
// every several metres instead of one a sample. Small next to how far
// OpenStreetMap is off, and the editor can drag what matters.
const (
	lakeSimplify  = 1.5
	creekSimplify = 1.0
)

// traceRings outlines the samples of mask equal to id, each sample a
// square spacing wide round its point: the squares' edges against the
// rest, linked into closed rings (water on the left going round), in
// world metres, simplified (lakeSimplify).
func traceRings(mask []int32, w, ht int, id int32, spacing float32) [][][2]float32 {
	in := func(i, j int) bool { return i >= 0 && j >= 0 && i < w && j < ht && mask[j*w+i] == id }
	// Edges between corner points (i, j) of the squares, keyed by start.
	type vert struct{ i, j int }
	next := map[vert][]vert{}
	for j := 0; j < ht; j++ {
		for i := 0; i < w; i++ {
			if !in(i, j) {
				continue
			}
			if !in(i, j-1) {
				next[vert{i, j}] = append(next[vert{i, j}], vert{i + 1, j})
			}
			if !in(i+1, j) {
				next[vert{i + 1, j}] = append(next[vert{i + 1, j}], vert{i + 1, j + 1})
			}
			if !in(i, j+1) {
				next[vert{i + 1, j + 1}] = append(next[vert{i + 1, j + 1}], vert{i, j + 1})
			}
			if !in(i-1, j) {
				next[vert{i, j + 1}] = append(next[vert{i, j + 1}], vert{i, j})
			}
		}
	}
	starts := make([]vert, 0, len(next))
	for v := range next {
		starts = append(starts, v)
	}
	slices.SortFunc(starts, func(a, b vert) int {
		if a.j != b.j {
			return a.j - b.j
		}
		return a.i - b.i
	})
	maxX, maxZ := float32(w-1)*spacing, float32(ht-1)*spacing
	var rings [][][2]float32
	for _, s := range starts {
		if len(next[s]) == 0 {
			continue
		}
		var ring [][2]float32
		v := s
		for len(next[v]) > 0 {
			ring = append(ring, [2]float32{
				min(max((float32(v.i)-0.5)*spacing, 0), maxX),
				min(max((float32(v.j)-0.5)*spacing, 0), maxZ),
			})
			n := next[v][0]
			next[v] = next[v][1:]
			v = n
		}
		if ring = simplifyRing(ring, lakeSimplify); len(ring) >= 3 {
			rings = append(rings, ring)
		}
	}
	return rings
}

// simplifyRing drops the points of closed ring ps that lie within tol of
// the line through their neighbours that's kept (Douglas–Peucker, from
// the ring's first point and the point furthest from it).
func simplifyRing(ps [][2]float32, tol float32) [][2]float32 {
	if len(ps) < 4 {
		return ps
	}
	far, farD := 0, float32(-1)
	for k, p := range ps {
		if d := dist2(ps[0], p); d > farD {
			far, farD = k, d
		}
	}
	a := simplifyLine(ps[:far+1], tol)
	b := simplifyLine(append(append([][2]float32(nil), ps[far:]...), ps[0]), tol)
	out := append(a[:len(a)-1], b[:len(b)-1]...)
	return out
}

// simplifyLine is Douglas–Peucker on an open line, keeping its ends.
func simplifyLine(ps [][2]float32, tol float32) [][2]float32 {
	if len(ps) < 3 {
		return append([][2]float32(nil), ps...)
	}
	a, b := ps[0], ps[len(ps)-1]
	far, farD := 0, float32(-1)
	for k := 1; k < len(ps)-1; k++ {
		if d := dist2(ps[k], nearestOnSegment(a, b, ps[k])); d > farD {
			far, farD = k, d
		}
	}
	if farD <= tol {
		return [][2]float32{a, b}
	}
	l := simplifyLine(ps[:far+1], tol)
	r := simplifyLine(ps[far:], tol)
	return append(l[:len(l)-1], r...)
}

// ShapeMask marks the points of a w × ht lattice, spacing metres apart
// from the world's origin, inside each lake's outline (even-odd over all
// its rings): its index in shapes plus 1, 0 for dry ground.
func ShapeMask(w, ht int, spacing float64, shapes []world.WaterLake) []int32 {
	mask := make([]int32, w*ht)
	var xs []float64
	for li, l := range shapes {
		minZ, maxZ := math.Inf(1), math.Inf(-1)
		for _, r := range l.Rings {
			for _, p := range r {
				minZ, maxZ = min(minZ, float64(p[1])), max(maxZ, float64(p[1]))
			}
		}
		j0, j1 := max(int(math.Ceil(minZ/spacing)), 0), min(int(math.Floor(maxZ/spacing)), ht-1)
		for j := j0; j <= j1; j++ {
			z := float64(j) * spacing
			xs = xs[:0]
			for _, r := range l.Rings {
				for k := range r {
					a, b := r[k], r[(k+1)%len(r)]
					z0, z1 := float64(a[1]), float64(b[1])
					if (z0 <= z) != (z1 <= z) {
						xs = append(xs, float64(a[0])+(z-z0)/(z1-z0)*float64(b[0]-a[0]))
					}
				}
			}
			slices.Sort(xs)
			for k := 0; k+1 < len(xs); k += 2 {
				i0, i1 := max(int(math.Ceil(xs[k]/spacing)), 0), min(int(math.Floor(xs[k+1]/spacing)), w-1)
				for i := i0; i <= i1; i++ {
					mask[j*w+i] = int32(li + 1)
				}
			}
		}
	}
	return mask
}

// A lake's shore: within lakeBankM metres of its water the ground is
// cut down to no more than lakeBankSlope (rise over run) above the
// water's level, so it slopes into the water wherever the outline is put
// rather than standing over it as a step. Ground already below that is
// left alone.
const (
	lakeBankM     = 8.0
	lakeBankSlope = 0.36 // 20°
)

// LevelLakes levels the w × ht heights h, spacing metres apart, inside
// each lake's outline to its surface (a low percentile of the ground
// inside, lakeLevelPercentile), and cuts its shore down to it
// (lakeBankSlope), in place.
func LevelLakes(h []float32, w, ht int, spacing float64, shapes []world.WaterLake) {
	mask := ShapeMask(w, ht, spacing, shapes)
	inside := make([][]float32, len(shapes))
	for k, li := range mask {
		if li > 0 {
			inside[li-1] = append(inside[li-1], h[k])
		}
	}
	level := make([]float32, len(shapes))
	for li, hs := range inside {
		if len(hs) > 0 {
			slices.Sort(hs)
			level[li] = hs[int(float64(len(hs)-1)*lakeLevelPercentile)]
		}
	}
	// The shore, breadth first out from the water (chamfer distances:
	// 1 across, √2 diagonally).
	steps := int(lakeBankM/spacing) + 1
	dist := make([]float32, len(mask))
	from := make([]int32, len(mask))
	var queue []int
	for k, li := range mask {
		if li > 0 {
			h[k] = level[li-1]
			from[k] = li
			queue = append(queue, k)
		} else {
			dist[k] = float32(math.Inf(1))
		}
	}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		i, j := k%w, k/w
		for _, d := range neighbours8 {
			ni, nj := i+d[0], j+d[1]
			if ni < 0 || nj < 0 || ni >= w || nj >= ht {
				continue
			}
			n := nj*w + ni
			step := float32(1)
			if d[0] != 0 && d[1] != 0 {
				step = math.Sqrt2
			}
			if nd := dist[k] + step; nd < dist[n] && nd <= float32(steps) {
				first := math.IsInf(float64(dist[n]), 1)
				dist[n], from[n] = nd, from[k]
				if first {
					queue = append(queue, n)
				}
			}
		}
	}
	for k, li := range from {
		if li == 0 || mask[k] > 0 {
			continue
		}
		if top := level[li-1] + lakeBankSlope*dist[k]*float32(spacing); h[k] > top {
			h[k] = top
		}
	}
}
