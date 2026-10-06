package geo

import (
	"container/heap"
	"math"
)

// Meadows: the flat, wet floors of valleys, where the water table sits
// near the surface and trees give way to grass and willow. A cell is
// valley floor when it stands little above the nearest stream channel
// (catchment past ChannelArea) that it can reach without climbing, and
// the ground around it is gentle.
const (
	ChannelArea     = 0.25e6 // m² of catchment that makes a channel worth a floor
	meadowReach     = 400.0  // metres from a channel a valley floor can spread
	meadowRise      = 6.0    // metres above the channel by which it's fully dry
	meadowWet       = 1.0    // metres above the channel that are fully wet
	meadowSlopeFrom = 0.04   // rise over run (≈2.5°) where floors start to dry...
	meadowSlopeTo   = 0.20   // ...and by ≈11° are slopes, not floors
	meadowSlopeR    = 3      // cells of averaging for the slope
)

// MeadowWetness is how much each cell of a w × ht grid h (row-major,
// spacing metres apart) is wet valley floor, 0–1, given its Catchment.
// Height above the channel is found by spreading outward from channel
// cells, nearest first, carrying the channel's height.
func MeadowWetness(h []float32, w, ht int, spacing float64, area []float32) []float32 {
	n := w * ht
	dist := make([]float64, n)
	chanH := make([]float32, n)
	for k := range dist {
		dist[k] = math.Inf(1)
	}
	pq := &cellHeap{}
	for k, a := range area {
		if a >= ChannelArea {
			dist[k], chanH[k] = 0, h[k]
			*pq = append(*pq, cellItem{k, 0})
		}
	}
	heap.Init(pq)
	for pq.Len() > 0 {
		c := heap.Pop(pq).(cellItem)
		if c.e > dist[c.k] || c.e > meadowReach {
			continue
		}
		i, j := c.k%w, c.k/w
		for _, d := range neighbours8 {
			ni, nj := i+d[0], j+d[1]
			if ni < 0 || nj < 0 || ni >= w || nj >= ht {
				continue
			}
			nk := nj*w + ni
			step := spacing
			if d[0] != 0 && d[1] != 0 {
				step *= math.Sqrt2
			}
			if nd := c.e + step; nd < dist[nk] {
				dist[nk], chanH[nk] = nd, chanH[c.k]
				heap.Push(pq, cellItem{nk, nd})
			}
		}
	}
	// Gentle ground: the slope across a few cells, so a single flat cell
	// on a hillside isn't a floor.
	at := func(i, j int) float32 { return h[min(max(j, 0), ht-1)*w+min(max(i, 0), w-1)] }
	out := make([]float32, n)
	r := meadowSlopeR
	for j := 0; j < ht; j++ {
		for i := 0; i < w; i++ {
			k := j*w + i
			if math.IsInf(dist[k], 1) {
				continue
			}
			gx := float64(at(i+r, j)-at(i-r, j)) / (2 * float64(r) * spacing)
			gz := float64(at(i, j+r)-at(i, j-r)) / (2 * float64(r) * spacing)
			slope := math.Hypot(gx, gz)
			rise := float64(h[k] - chanH[k])
			wet := 1 - smoothstep(meadowWet, meadowRise, rise)
			flat := 1 - smoothstep(meadowSlopeFrom, meadowSlopeTo, slope)
			out[k] = float32(wet * flat)
		}
	}
	return out
}
