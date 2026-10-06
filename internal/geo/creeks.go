package geo

import (
	"math"

	"mountain-mogul/internal/world"
)

// Creek channels, cut into the ground along the stretches of
// OpenStreetMap streams that flow (see TraceStreams). A channel widens
// and deepens with the ground that drains into it; its bed only ever
// runs downhill; and its banks ease back into the ground.

// CreekShape is a flowing creek's half-width and depth, metres, where
// catchment square metres drain through it.
func CreekShape(catchment float32) (half, depth float32) {
	k := float32(math.Sqrt(float64(catchment) / 1e6))
	return 0.75 + 1.0*k, 0.4 + 0.4*k
}

// CreekStreams traces base's streams on the w × ht lattice h (spacing
// metres apart): catchment on the 5 m corner grid (every DetailPerCell'th
// sample when the lattice is the detail), streams moved onto its
// channels, lakes counted as sources.
func CreekStreams(h []float32, w, ht int, spacing float64, base *world.TerrainBase) []Stream {
	step := max(int(math.Round(world.CellSize/spacing)), 1)
	cw, ch := (w-1)/step+1, (ht-1)/step+1
	coarse := make([]float32, cw*ch)
	for j := 0; j < ch; j++ {
		for i := 0; i < cw; i++ {
			coarse[j*cw+i] = h[j*step*w+i*step]
		}
	}
	area := Catchment(coarse, cw, ch, world.CellSize)
	var lake []bool
	if len(base.Lakes) > 0 {
		lake = make([]bool, cw*ch)
		for k, li := range LakeMask(cw, ch, boundsOf(base.Geo), base.Lakes) {
			lake[k] = li > 0
		}
	}
	return TraceStreams(base.Streams, boundsOf(base.Geo), cw, ch, coarse, area, lake)
}

// CarveCreeks cuts each stream's flowing stretches into the w × ht
// lattice h (spacing metres apart), in place.
func CarveCreeks(h []float32, w, ht int, spacing float64, streams []Stream) {
	at := func(x, z float32) float32 {
		gx, gz := float64(x)/spacing, float64(z)/spacing
		i0 := min(max(int(gx), 0), w-2)
		j0 := min(max(int(gz), 0), ht-2)
		fx, fz := float32(gx-float64(i0)), float32(gz-float64(j0))
		a := h[j0*w+i0]*(1-fx) + h[j0*w+i0+1]*fx
		b := h[(j0+1)*w+i0]*(1-fx) + h[(j0+1)*w+i0+1]*fx
		return a*(1-fz) + b*fz
	}
	for _, s := range streams {
		n := len(s.Points)
		bed := make([]float32, n)
		prev := float32(math.Inf(1))
		for i, p := range s.Points {
			_, depth := CreekShape(s.Catchment[i])
			bed[i] = min(prev, at(p[0], p[1])-depth)
			prev = bed[i]
		}
		for i := 1; i < n; i++ {
			if !s.Flowing(i) {
				continue
			}
			carveSegment(h, w, ht, spacing, s.Points[i-1], s.Points[i], bed[i-1], bed[i], s.Catchment[i])
		}
	}
}

// carveSegment lowers the lattice around one segment of a creek: a flat
// floor across its middle at the bed height, interpolated along the
// segment, rising to meet the ground at twice its half-width.
func carveSegment(h []float32, w, ht int, spacing float64, a, b [2]float32, bedA, bedB, catchment float32) {
	half, _ := CreekShape(catchment)
	reach := 2 * half
	dx, dz := b[0]-a[0], b[1]-a[1]
	l2 := dx*dx + dz*dz
	i0 := max(int(float64(min(a[0], b[0])-reach)/spacing), 0)
	i1 := min(int(float64(max(a[0], b[0])+reach)/spacing)+1, w-1)
	j0 := max(int(float64(min(a[1], b[1])-reach)/spacing), 0)
	j1 := min(int(float64(max(a[1], b[1])+reach)/spacing)+1, ht-1)
	for j := j0; j <= j1; j++ {
		for i := i0; i <= i1; i++ {
			px, pz := float32(float64(i)*spacing), float32(float64(j)*spacing)
			t := float32(0)
			if l2 > 0 {
				t = min(max(((px-a[0])*dx+(pz-a[1])*dz)/l2, 0), 1)
			}
			d := float32(math.Hypot(float64(px-(a[0]+t*dx)), float64(pz-(a[1]+t*dz))))
			if d >= reach {
				continue
			}
			k := j*w + i
			bed := bedA + (bedB-bedA)*t
			u := d / reach
			rise := u * u * (3 - 2*u) // 0 on the floor's centre line, 1 at the top of the bank
			rise = max(rise-0.15, 0) / 0.85
			if target := bed + (h[k]-bed)*rise; target < h[k] {
				h[k] = target
			}
		}
	}
}
