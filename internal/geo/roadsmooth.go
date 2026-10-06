package geo

import (
	"math"
)

// Road corridor shape, in metres. A road's cut and fill banks reach
// roughly 1.5 times its width out from the pavement edge (more for wide
// roads, which sit in deeper cuts), clamped to roadBankMin..roadBankMax.
// Gaps between corridors narrower than about twice roadFeather are
// closed, and the smoothed ground fades back into the real ground over
// roadFeather.
const (
	roadBankPerWidth = 1.5
	roadBankMin      = 6
	roadBankMax      = 25
	roadFeather      = 12
)

// Banks taller than the allowance above show as steep strips along the
// corridor's edge. The corridor grows across ground steeper than
// roadBankSlope (cut and fill banks are built at about 1.5–2 to 1) that
// touches it, up to roadBankGrow beyond the allowance.
const (
	roadBankSlope = 27.0
	roadBankGrow  = 20.0
)

// roadHoleArea is the largest island, in square metres, enclosed by
// corridors that's smoothed with them.
const roadHoleArea = 15000.0

// roadSmoothIters is the relaxation passes over the corridor. Push-pull
// gets the fill close, so this only has to smooth it out.
const roadSmoothIters = 300

// Roughness put back over the smoothed ground: the real ground's bumps
// finer than roughBlur metres are measured around each corridor and
// matched with noise of roughWavelengths inside it, so the fill doesn't
// read as a blank strip.
const roughBlur = 5.0

var roughWavelengths = []float64{3, 6, 12, 24}

// SmoothRoads rebuilds the ground along each road as a smooth surface
// spanning from the terrain either side, so highway cuts, embankments,
// and the flat strip of pavement stop showing as scars, then adds back
// bumps as rough as the ground around it. h is a w × ht lattice over b
// (row 0 at MaxLat, column 0 at MinLon), spacing metres apart. Tunnels
// are skipped. Bridges aren't: the fill follows the ground either side
// of the corridor, so a valley under a bridge keeps its shape. Returns
// how many samples changed.
func SmoothRoads(h []float32, w, ht int, b Bounds, spacing float64, roads []Road) int {
	core, reach := roadCores(w, ht, b, spacing, roads)
	growOverBanks(h, w, ht, spacing, core, reach)
	r := max(int(math.Round(roadFeather/spacing)), 1)
	closeGaps(core, w, ht, r)
	fillSmallHoles(core, w, ht, int(roadHoleArea/(spacing*spacing)))
	weight := featherMask(core, w, ht, r)
	n := 0
	for _, v := range weight {
		if v > 0 {
			n++
		}
	}
	if n == 0 {
		return 0
	}
	rough := roughnessAround(h, w, ht, weight, spacing)
	fill := harmonicFill(h, w, ht, weight)
	for k, wt := range weight {
		if wt > 0 {
			i, j := k%w, k/w
			bump := rough[k] * groundNoise(float64(i)*spacing, float64(j)*spacing)
			h[k] += (fill[k] + bump - h[k]) * wt
		}
	}
	return n
}

// roughnessAround is the RMS of the ground's fine bumps, measured where
// unknown is 0 and carried into the corridors.
func roughnessAround(h []float32, w, ht int, unknown []float32, spacing float64) []float32 {
	r := int(math.Round(roughBlur / spacing))
	blur := append([]float32(nil), h...)
	boxBlur(blur, w, ht, r)
	sq := make([]float32, len(h))
	for k := range h {
		d := h[k] - blur[k]
		sq[k] = d * d
	}
	// Measure only well clear of the road, where the ground is natural,
	// and away from the map edge, where the blur is lopsided.
	clear := append([]float32(nil), unknown...)
	boxBlur(clear, w, ht, 2*r)
	for k := range clear {
		i, j := k%w, k/w
		if clear[k] > 0 || unknown[k] > 0 || i < r || j < r || i >= w-r || j >= ht-r {
			clear[k] = 1
		}
	}
	sq = pushPull(sq, w, ht, clear)
	boxBlur(sq, w, ht, 4*r)
	for k, v := range sq {
		sq[k] = float32(math.Sqrt(float64(max(v, 0))))
	}
	return sq
}

// groundNoise is smooth noise with unit RMS, summed over
// roughWavelengths with amplitude proportional to wavelength, like real
// ground.
func groundNoise(x, y float64) float32 {
	var sum, norm float64
	for o, wl := range roughWavelengths {
		sum += wl * valueNoise(x/wl, y/wl, uint32(o))
		norm += wl * wl
	}
	// valueNoise's RMS is about 0.44 (measured).
	return float32(2.25 * sum / math.Sqrt(norm))
}

// valueNoise is bilinear-smoothstep noise in -1..1 on the integer
// lattice, seeded per octave.
func valueNoise(x, y float64, seed uint32) float64 {
	xi, yi := math.Floor(x), math.Floor(y)
	fx, fy := x-xi, y-yi
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	at := func(dx, dy int) float64 {
		hsh := uint32(int32(xi)+int32(dx))*0x8da6b343 ^ uint32(int32(yi)+int32(dy))*0xd8163841 ^ seed*0xcb1ab31f
		hsh ^= hsh >> 13
		hsh *= 0x5bd1e995
		hsh ^= hsh >> 15
		return float64(hsh)/float64(math.MaxUint32)*2 - 1
	}
	top := at(0, 0)*(1-fx) + at(1, 0)*fx
	bot := at(0, 1)*(1-fx) + at(1, 1)*fx
	return top*(1-fy) + bot*fy
}

// roadCores is 1 over each road and its bank allowance. reach marks
// where the corridor may grow over steep banks (within roadBankGrow of
// the allowance).
func roadCores(w, ht int, b Bounds, spacing float64, roads []Road) (core []float32, reach []bool) {
	core = make([]float32, w*ht)
	reach = make([]bool, w*ht)
	toLattice := func(p LatLon) (float64, float64) {
		return (p.Lon - b.MinLon) / (b.MaxLon - b.MinLon) * float64(w-1),
			(b.MaxLat - p.Lat) / (b.MaxLat - b.MinLat) * float64(ht-1)
	}
	for _, r := range roads {
		if r.Tunnel {
			continue
		}
		half := r.Width/2 + math.Min(math.Max(r.Width*roadBankPerWidth, roadBankMin), roadBankMax)
		pad := (half + roadBankGrow) / spacing
		for s := 1; s < len(r.Path); s++ {
			x0, y0 := toLattice(r.Path[s-1])
			x1, y1 := toLattice(r.Path[s])
			i0 := max(int(math.Floor(math.Min(x0, x1)-pad)), 0)
			i1 := min(int(math.Ceil(math.Max(x0, x1)+pad)), w-1)
			j0 := max(int(math.Floor(math.Min(y0, y1)-pad)), 0)
			j1 := min(int(math.Ceil(math.Max(y0, y1)+pad)), ht-1)
			dx, dy := x1-x0, y1-y0
			l2 := dx*dx + dy*dy
			for j := j0; j <= j1; j++ {
				for i := i0; i <= i1; i++ {
					t := 0.0
					if l2 > 0 {
						t = math.Min(math.Max(((float64(i)-x0)*dx+(float64(j)-y0)*dy)/l2, 0), 1)
					}
					d := math.Hypot(float64(i)-(x0+t*dx), float64(j)-(y0+t*dy)) * spacing
					k := j*w + i
					if d < half {
						core[k] = 1
					}
					if d < half+roadBankGrow {
						reach[k] = true
					}
				}
			}
		}
	}
	return core, reach
}

// growOverBanks adds to core the steep ground, within reach, that
// touches it.
func growOverBanks(h []float32, w, ht int, spacing float64, core []float32, reach []bool) {
	steep := float32(math.Tan(roadBankSlope*math.Pi/180) * 2 * spacing)
	isSteep := func(k int) bool {
		i, j := k%w, k/w
		if i == 0 || j == 0 || i == w-1 || j == ht-1 {
			return false
		}
		sx, sz := h[k+1]-h[k-1], h[k+w]-h[k-w]
		return sx*sx+sz*sz > steep*steep
	}
	var queue []int
	for k, v := range core {
		if v > 0 {
			queue = append(queue, k)
		}
	}
	for len(queue) > 0 {
		k := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		i := k % w
		for _, n := range [4]int{k - 1, k + 1, k - w, k + w} {
			if (n == k-1 && i == 0) || (n == k+1 && i == w-1) || n < 0 || n >= len(h) {
				continue
			}
			if core[n] == 0 && reach[n] && isSteep(n) {
				core[n] = 1
				queue = append(queue, n)
			}
		}
	}
}

// closeGaps fills gaps in the 0/1 mask m narrower than about 2r samples
// (a dilation then an erosion with a square of side 2r+1). Between two
// carriageways or a ramp and the road it leaves, the ground is part of
// the same earthworks; a strip left half-smoothed there shows as a step.
func closeGaps(m []float32, w, ht, r int) {
	boxBlur(m, w, ht, r)
	for k, v := range m {
		if v > 0 {
			m[k] = 1
		}
	}
	boxBlur(m, w, ht, r)
	for k, v := range m {
		if v > 0.9999 {
			m[k] = 1
		} else {
			m[k] = 0
		}
	}
}

// fillSmallHoles adds to the 0/1 mask m every region it encloses that is
// smaller than maxSamples: islands of ground between ramps and roads
// are earthworks too, and a leftover island shows as a ring.
func fillSmallHoles(m []float32, w, ht, maxSamples int) {
	seen := make([]bool, len(m))
	var region, stack []int
	for start := range m {
		if m[start] > 0 || seen[start] {
			continue
		}
		region, stack = region[:0], append(stack[:0], start)
		seen[start] = true
		touchesEdge := false
		for len(stack) > 0 {
			k := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			region = append(region, k)
			i, j := k%w, k/w
			if i == 0 || j == 0 || i == w-1 || j == ht-1 {
				touchesEdge = true
			}
			for _, n := range [4]int{k - 1, k + 1, k - w, k + w} {
				if (n == k-1 && i == 0) || (n == k+1 && i == w-1) || n < 0 || n >= len(m) {
					continue
				}
				if m[n] == 0 && !seen[n] {
					seen[n] = true
					stack = append(stack, n)
				}
			}
		}
		if !touchesEdge && len(region) < maxSamples {
			for _, k := range region {
				m[k] = 1
			}
		}
	}
}

// featherMask turns the 0/1 corridor into weights: 1 inside, fading to
// 0 over about r samples outside.
func featherMask(core []float32, w, ht, r int) []float32 {
	weight := append([]float32(nil), core...)
	boxBlur(weight, w, ht, r)
	for k, v := range weight {
		if core[k] > 0 {
			weight[k] = 1
		} else {
			weight[k] = float32(smoothstep(0, 0.5, float64(v)))
		}
	}
	return weight
}

// harmonicFill returns h with every sample where unknown > 0 replaced by
// a smooth (harmonic) surface held to the known samples around it.
// Push-pull over a pyramid gives the first guess, then over-relaxation
// smooths it.
func harmonicFill(h []float32, w, ht int, unknown []float32) []float32 {
	out := pushPull(h, w, ht, unknown)
	var idx []int32
	for k, u := range unknown {
		if u > 0 {
			idx = append(idx, int32(k))
		}
	}
	// A sample on the map edge only averages along the edge: where a road
	// leaves the map there's no ground beyond to hold the fill, so the
	// edge interpolates between the corridor's two sides like every other
	// cross-section. Corners keep their push-pull value.
	const omega = 1.8
	for it := 0; it < roadSmoothIters; it++ {
		for _, k32 := range idx {
			k := int(k32)
			i, j := k%w, k/w
			xEdge, zEdge := i == 0 || i == w-1, j == 0 || j == ht-1
			var sum float32
			n := 0
			if !xEdge || zEdge {
				if i > 0 && i < w-1 {
					sum += out[k-1] + out[k+1]
					n += 2
				}
			}
			if !zEdge || xEdge {
				if j > 0 && j < ht-1 {
					sum += out[k-w] + out[k+w]
					n += 2
				}
			}
			if n > 0 {
				out[k] += omega * (sum/float32(n) - out[k])
			}
		}
	}
	return out
}

// pushPull fills unknown samples from the known ones: average known
// values down a pyramid of halvings until every coarse sample has some,
// then pull back up, taking the coarser level wherever a finer one has
// nothing known.
func pushPull(h []float32, w, ht int, unknown []float32) []float32 {
	type level struct {
		v, wt []float32
		w, h  int
	}
	l0 := level{v: make([]float32, len(h)), wt: make([]float32, len(h)), w: w, h: ht}
	for k := range h {
		if unknown[k] == 0 {
			l0.v[k], l0.wt[k] = h[k], 1
		}
	}
	levels := []level{l0}
	for {
		f := levels[len(levels)-1]
		full := true
		for _, x := range f.wt {
			if x == 0 {
				full = false
				break
			}
		}
		if full || (f.w == 1 && f.h == 1) {
			break
		}
		c := level{w: (f.w + 1) / 2, h: (f.h + 1) / 2}
		c.v, c.wt = make([]float32, c.w*c.h), make([]float32, c.w*c.h)
		for j := 0; j < f.h; j++ {
			for i := 0; i < f.w; i++ {
				fk, ck := j*f.w+i, (j/2)*c.w+i/2
				c.v[ck] += f.v[fk] * f.wt[fk]
				c.wt[ck] += f.wt[fk]
			}
		}
		for k := range c.v {
			if c.wt[k] > 0 {
				c.v[k] /= c.wt[k]
				c.wt[k] = 1
			}
		}
		levels = append(levels, c)
	}
	for li := len(levels) - 2; li >= 0; li-- {
		f, c := levels[li], levels[li+1]
		for j := 0; j < f.h; j++ {
			for i := 0; i < f.w; i++ {
				k := j*f.w + i
				if f.wt[k] > 0 {
					continue
				}
				// Bilinear from the coarse level, whose sample (ci, cj)
				// covers fine samples 2ci and 2ci+1.
				cx := math.Max((float64(i)-0.5)/2, 0)
				cy := math.Max((float64(j)-0.5)/2, 0)
				ci, cj := min(int(cx), c.w-1), min(int(cy), c.h-1)
				ci1, cj1 := min(ci+1, c.w-1), min(cj+1, c.h-1)
				fx, fy := float32(cx-float64(ci)), float32(cy-float64(cj))
				top := c.v[cj*c.w+ci]*(1-fx) + c.v[cj*c.w+ci1]*fx
				bot := c.v[cj1*c.w+ci]*(1-fx) + c.v[cj1*c.w+ci1]*fx
				f.v[k] = top*(1-fy) + bot*fy
				f.wt[k] = 1
			}
		}
	}
	return levels[0].v
}
