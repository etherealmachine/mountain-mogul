package geo

import (
	"container/heap"
	"math"

	"mountain-mogul/internal/world"
)

// Streams: where water really runs. OpenStreetMap stream lines often
// climb far up a mountainside, where in winter there's no water, and can
// sit a few metres off the channel in the lidar. Catchment area decides
// both: each line is moved onto the strongest channel near it, and it
// only flows where enough ground drains through it.

// StreamStartArea is the catchment, in square metres, where a stream
// starts flowing; above it the channel is a dry gully.
const StreamStartArea = 0.5e6

// streamSnap is how far, in cells, a stream line moves to find its
// channel.
const streamSnap = 2

// Catchment is the area in square metres that drains through each cell
// of a w × ht grid h (row-major, spacing metres apart). Depressions are
// filled first, with a tiny slope across flats (priority-flood), so water
// crosses meadows and levelled lakes and leaves by the map's edge, and
// each cell drains to its steepest downhill neighbour.
func Catchment(h []float32, w, ht int, spacing float64) []float32 {
	n := w * ht
	filled := make([]float64, n)
	done := make([]bool, n)
	pq := &cellHeap{}
	push := func(k int, e float64) {
		filled[k] = e
		done[k] = true
		heap.Push(pq, cellItem{k, e})
	}
	for i := 0; i < w; i++ {
		push(i, float64(h[i]))
		push((ht-1)*w+i, float64(h[(ht-1)*w+i]))
	}
	for j := 1; j < ht-1; j++ {
		push(j*w, float64(h[j*w]))
		push(j*w+w-1, float64(h[j*w+w-1]))
	}
	const eps = 1e-4 // metres: enough to order a flat, too little to see
	order := make([]int, 0, n)
	for pq.Len() > 0 {
		c := heap.Pop(pq).(cellItem)
		order = append(order, c.k)
		i, j := c.k%w, c.k/w
		for _, d := range neighbours8 {
			ni, nj := i+d[0], j+d[1]
			if ni < 0 || nj < 0 || ni >= w || nj >= ht {
				continue
			}
			nk := nj*w + ni
			if done[nk] {
				continue
			}
			push(nk, math.Max(float64(h[nk]), c.e+eps))
		}
	}
	// order runs from the edges inward and upward; accumulate from the
	// top down, each cell into its steepest downhill neighbour.
	area := make([]float32, n)
	cellArea := float32(spacing * spacing)
	for k := range area {
		area[k] = cellArea
	}
	for o := n - 1; o >= 0; o-- {
		k := order[o]
		i, j := k%w, k/w
		best, bestDrop := -1, 0.0
		for _, d := range neighbours8 {
			ni, nj := i+d[0], j+d[1]
			if ni < 0 || nj < 0 || ni >= w || nj >= ht {
				continue
			}
			nk := nj*w + ni
			drop := filled[k] - filled[nk]
			if d[0] != 0 && d[1] != 0 {
				drop /= math.Sqrt2
			}
			if drop > bestDrop {
				best, bestDrop = nk, drop
			}
		}
		if best >= 0 {
			area[best] += area[k]
		}
	}
	return area
}

// smoothPath takes the stair-steps out of a line snapped cell to cell:
// each point moves to the average of the points within streamSmooth of
// it along the line, keeping the ends.
func smoothPath(ps [][2]float32) [][2]float32 {
	const streamSmooth = 3
	out := make([][2]float32, len(ps))
	for i := range ps {
		if i == 0 || i == len(ps)-1 {
			out[i] = ps[i]
			continue
		}
		var sx, sz float32
		n := 0
		for k := max(i-streamSmooth, 0); k <= min(i+streamSmooth, len(ps)-1); k++ {
			sx += ps[k][0]
			sz += ps[k][1]
			n++
		}
		out[i] = [2]float32{sx / float32(n), sz / float32(n)}
	}
	return out
}

// nearLake reports whether a lake cell lies within streamSnap+1 cells of
// cell (i, j).
func nearLake(lake []bool, w, ht, i, j int) bool {
	if lake == nil {
		return false
	}
	r := streamSnap + 1
	for dj := -r; dj <= r; dj++ {
		for di := -r; di <= r; di++ {
			ni, nj := i+di, j+dj
			if ni >= 0 && nj >= 0 && ni < w && nj < ht && lake[nj*w+ni] {
				return true
			}
		}
	}
	return false
}

var neighbours8 = [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

type cellItem struct {
	k int
	e float64
}

type cellHeap []cellItem

func (h cellHeap) Len() int           { return len(h) }
func (h cellHeap) Less(a, b int) bool { return h[a].e < h[b].e }
func (h cellHeap) Swap(a, b int)      { h[a], h[b] = h[b], h[a] }
func (h *cellHeap) Push(x any)        { *h = append(*h, x.(cellItem)) }
func (h *cellHeap) Pop() any {
	old := *h
	it := old[len(old)-1]
	*h = old[:len(old)-1]
	return it
}

// Stream is an OpenStreetMap stream moved onto its channel: points in
// world metres (x, z) from upstream down, each with the catchment that
// drains through it.
type Stream struct {
	Name      string
	Kind      string
	Points    [][2]float32
	Catchment []float32
}

// Flowing reports whether the stream has water at point i.
func (s *Stream) Flowing(i int) bool { return s.Catchment[i] >= StreamStartArea }

// TraceStreams moves each OpenStreetMap stream onto the cell grid's
// channels: every 2.5 m along the line, to the cell with the most
// catchment within streamSnap cells. h is the ground on the w × ht
// cells, area its Catchment, b the map's bounds, and lake marks the cells
// of lakes (nil for none). Water runs downhill, so a line is turned to
// start at its higher end (by catchment it could look backwards: snapping
// at a junction picks up the bigger stream's catchment). Water doesn't vanish, so catchment only grows
// downstream; and a stream that enters across the map's edge or leaves a
// lake carries water from its start, since what drains into it lies off
// the map or behind the lake.
func TraceStreams(streams []world.BaseStream, b Bounds, w, ht int, h, area []float32, lake []bool) []Stream {
	maxX := float64(w-1) * world.CellSize
	maxZ := float64(ht-1) * world.CellSize
	toWorld := func(p [2]float64) (float64, float64) {
		return (p[1] - b.MinLon) / (b.MaxLon - b.MinLon) * maxX, (b.MaxLat - p[0]) / (b.MaxLat - b.MinLat) * maxZ
	}
	var out []Stream
	for _, bs := range streams {
		s := Stream{Name: bs.Name, Kind: bs.Kind}
		last := -1
		add := func(x, z float64) {
			ci, cj := int(x/world.CellSize), int(z/world.CellSize)
			if ci < 0 || cj < 0 || ci >= w || cj >= ht {
				return
			}
			best, bestA := -1, float32(-1)
			for dj := -streamSnap; dj <= streamSnap; dj++ {
				for di := -streamSnap; di <= streamSnap; di++ {
					ni, nj := ci+di, cj+dj
					if ni < 0 || nj < 0 || ni >= w || nj >= ht {
						continue
					}
					if a := area[nj*w+ni]; a > bestA {
						best, bestA = nj*w+ni, a
					}
				}
			}
			if best == last {
				return
			}
			last = best
			s.Points = append(s.Points, [2]float32{(float32(best%w) + 0.5) * world.CellSize, (float32(best/w) + 0.5) * world.CellSize})
			s.Catchment = append(s.Catchment, bestA)
		}
		for k := 1; k < len(bs.Path); k++ {
			x0, z0 := toWorld(bs.Path[k-1])
			x1, z1 := toWorld(bs.Path[k])
			steps := max(int(math.Hypot(x1-x0, z1-z0)/2.5), 1)
			for st := 0; st < steps; st++ {
				f := float64(st) / float64(steps)
				add(x0+(x1-x0)*f, z0+(z1-z0)*f)
			}
		}
		if len(s.Points) < 2 {
			continue
		}
		elev := func(p [2]float32) float32 {
			return h[int(p[1]/world.CellSize)*w+int(p[0]/world.CellSize)]
		}
		if elev(s.Points[0]) < elev(s.Points[len(s.Points)-1]) {
			for i, j := 0, len(s.Points)-1; i < j; i, j = i+1, j-1 {
				s.Points[i], s.Points[j] = s.Points[j], s.Points[i]
				s.Catchment[i], s.Catchment[j] = s.Catchment[j], s.Catchment[i]
			}
		}
		p0 := s.Points[0]
		ci, cj := int(p0[0]/world.CellSize), int(p0[1]/world.CellSize)
		if ci <= streamSnap || cj <= streamSnap || ci >= w-1-streamSnap || cj >= ht-1-streamSnap || nearLake(lake, w, ht, ci, cj) {
			s.Catchment[0] = max(s.Catchment[0], StreamStartArea)
		}
		for i := 1; i < len(s.Catchment); i++ {
			s.Catchment[i] = max(s.Catchment[i], s.Catchment[i-1])
		}
		s.Points = smoothPath(s.Points)
		out = append(out, s)
	}
	return out
}
