package sim

import (
	"container/heap"
	"math"

	"mountain-mogul/internal/world"
)

// node is a grid coordinate with A* bookkeeping.
type node struct {
	pos  [2]int
	g, f float64
	idx  int // heap index
}

type nodeHeap []*node

func (h nodeHeap) Len() int            { return len(h) }
func (h nodeHeap) Less(i, j int) bool { return h[i].f < h[j].f }
func (h nodeHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].idx = i
	h[j].idx = j
}
func (h *nodeHeap) Push(x interface{}) {
	n := x.(*node)
	n.idx = len(*h)
	*h = append(*h, n)
}
func (h *nodeHeap) Pop() interface{} {
	old := *h
	n := old[len(old)-1]
	*h = old[:len(old)-1]
	n.idx = -1
	return n
}

// Pathfinder runs A* on the terrain grid.
type Pathfinder struct {
	terrain *world.Terrain
}

// NewPathfinder creates a new Pathfinder for the given terrain.
func NewPathfinder(t *world.Terrain) *Pathfinder {
	return &Pathfinder{terrain: t}
}

// FindPath returns a path from `from` to `to` using A*, or nil if no path exists.
func (p *Pathfinder) FindPath(from, to [2]int) [][2]int {
	if !p.terrain.InBounds(from[0], from[1]) || !p.terrain.InBounds(to[0], to[1]) {
		return nil
	}
	// If destination is impassable, still try to reach it (lift base may be marked)
	// We allow entering the destination even if impassable.

	type key = [2]int
	came := make(map[key]key)
	gScore := map[key]float64{from: 0}

	h := &nodeHeap{}
	heap.Init(h)
	heap.Push(h, &node{pos: from, g: 0, f: heuristic(from, to)})

	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

	for h.Len() > 0 {
		current := heap.Pop(h).(*node)
		pos := current.pos

		if pos == to {
			return reconstructPath(came, pos)
		}

		for _, d := range dirs {
			nb := [2]int{pos[0] + d[0], pos[1] + d[1]}
			if !p.terrain.InBounds(nb[0], nb[1]) {
				continue
			}
			// Skip unwalkable cells unless it's the destination (lift base may be structural-blocked)
			if !p.terrain.Cells[nb[0]][nb[1]].Walkable() && nb != to {
				continue
			}
			tentative := gScore[pos] + 1.0
			if prev, ok := gScore[nb]; !ok || tentative < prev {
				gScore[nb] = tentative
				came[nb] = pos
				f := tentative + heuristic(nb, to)
				heap.Push(h, &node{pos: nb, g: tentative, f: f})
			}
		}
	}

	return nil // no path
}

func heuristic(a, b [2]int) float64 {
	return math.Abs(float64(a[0]-b[0])) + math.Abs(float64(a[1]-b[1]))
}

func reconstructPath(came map[[2]int][2]int, end [2]int) [][2]int {
	path := [][2]int{end}
	cur := end
	for {
		prev, ok := came[cur]
		if !ok {
			break
		}
		path = append([][2]int{prev}, path...)
		cur = prev
	}
	return path
}
