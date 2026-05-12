package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// RoadNodeKind tags a node by its purpose so the renderer and any future
// spawner/pathfinder can treat each kind appropriately. Intersections
// are auto-created where two edges cross; edge-connection nodes are
// editor-only spawn/despawn points on the map perimeter; parking
// driveways link the road graph to parking lots.
type RoadNodeKind uint8

const (
	RoadNodeFreestanding    RoadNodeKind = 0 // dragged endpoint with no special role
	RoadNodeEdgeConnection  RoadNodeKind = 1 // map-edge spawn/despawn (editor-only)
	RoadNodeParkingDriveway RoadNodeKind = 2 // entrance to a parking lot
	RoadNodeIntersection    RoadNodeKind = 3 // auto-created where edges cross
)

// RoadNode is one vertex in the road graph. Pos is continuous world XZ
// (metres); Y is derived from terrain elevation at render time.
type RoadNode struct {
	ID   uint64
	Pos  mgl32.Vec2
	Kind RoadNodeKind
}

// RoadEdge is a straight road segment connecting two nodes. Curves come
// from chaining edges through intersection nodes, not per-edge geometry.
type RoadEdge struct {
	ID   uint64
	A, B uint64 // node IDs
}

// Road dimensions and costs. A single two-lane road type — width is
// the same everywhere, costs scale with length. Tuned to be cheap
// relative to lifts: roads are connective tissue, not headline content.
const (
	RoadHalfWidth    = float32(3.0) // 6 m total — one lane each direction
	RoadBaseCost     = 200          // fixed admin cost per segment
	RoadCostPerMeter = 5            // per metre of edge
	// RoadSnapRadius is the metres-to-existing-node tolerance for
	// folding a dragged endpoint onto an existing junction instead of
	// stacking a new node on top of it.
	RoadSnapRadius = float32(4.0)
)

// RoadCost returns the placement cost of a road segment between a and b.
func RoadCost(a, b mgl32.Vec2) int {
	dx := b[0] - a[0]
	dz := b[1] - a[1]
	length := math.Sqrt(float64(dx*dx + dz*dz))
	return RoadBaseCost + int(length*float64(RoadCostPerMeter))
}

// AddRoadNode appends a new node at pos and returns it. The ID is drawn
// from the world's shared NextID counter so node IDs don't collide with
// buildings, lifts, agents, etc.
func (w *World) AddRoadNode(pos mgl32.Vec2, kind RoadNodeKind) *RoadNode {
	n := &RoadNode{ID: w.NextID(), Pos: pos, Kind: kind}
	w.RoadNodes = append(w.RoadNodes, n)
	return n
}

// AddRoadEdge appends an edge between two existing nodes and returns it.
// Caller is responsible for ensuring a and b reference live nodes; this
// is a low-level append, with the placement helpers above the line.
func (w *World) AddRoadEdge(a, b uint64) *RoadEdge {
	e := &RoadEdge{ID: w.NextID(), A: a, B: b}
	w.RoadEdges = append(w.RoadEdges, e)
	return e
}

// SnapRoadNode returns the closest existing node within `radius` metres
// of pos, or nil if there is none. Drives endpoint snapping in the
// placement tool so dragging an edge near an existing junction folds
// onto it rather than creating a duplicate node.
func (w *World) SnapRoadNode(pos mgl32.Vec2, radius float32) *RoadNode {
	r2 := radius * radius
	var best *RoadNode
	bestD2 := r2
	for _, n := range w.RoadNodes {
		dx := n.Pos[0] - pos[0]
		dz := n.Pos[1] - pos[1]
		d2 := dx*dx + dz*dz
		if d2 <= bestD2 {
			best = n
			bestD2 = d2
		}
	}
	return best
}

// RoadNodeByID returns the node with the given ID, or nil if absent.
// Used by the renderer to resolve edge endpoints into positions.
func (w *World) RoadNodeByID(id uint64) *RoadNode {
	for _, n := range w.RoadNodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// RoadChainSamplesPerSegment is the resolution at which a Catmull-Rom
// spline through a chain is sampled per inter-node segment. Dense
// enough that effects-pass closest-sample queries and renderer mesh
// strips both read as smooth curves at the standard gameplay zoom.
const RoadChainSamplesPerSegment = 16

// RoadChain is a maximal run of road edges connected through degree-2
// nodes. The first and last node have degree != 2 (junctions or
// dead-ends); interior nodes have exactly two incident edges. Every
// edge in the road graph belongs to exactly one chain.
//
// Drives the curve treatment: a chain is rendered, effects-stamped,
// and (in the future) traffic-routed as a single Catmull-Rom spline
// rather than as independent straight edges, so adjacent edges blend
// smoothly into each other at degree-2 joints.
type RoadChain struct {
	Nodes []*RoadNode
	Edges []*RoadEdge
}

// FindRoadChains decomposes the road graph into chains. Each edge is
// visited exactly once. Pure cycles (closed loops with no degree-1 or
// degree-≥3 endpoint) are also captured as chains starting wherever
// the seed edge happened to be — the loop terminates on revisit.
func (w *World) FindRoadChains() []RoadChain {
	if len(w.RoadEdges) == 0 {
		return nil
	}
	adj := make(map[uint64][]*RoadEdge, len(w.RoadNodes))
	for _, e := range w.RoadEdges {
		adj[e.A] = append(adj[e.A], e)
		adj[e.B] = append(adj[e.B], e)
	}

	visited := make(map[uint64]bool, len(w.RoadEdges))
	chains := make([]RoadChain, 0)

	for _, seed := range w.RoadEdges {
		if visited[seed.ID] {
			continue
		}
		chains = append(chains, walkRoadChain(w, seed, adj, visited))
	}
	return chains
}

// walkRoadChain extends `seed` in both directions through degree-2
// nodes, stopping at the first node with degree != 2 (or when looping
// back onto an already-visited edge). All edges traversed get marked
// visited.
func walkRoadChain(w *World, seed *RoadEdge, adj map[uint64][]*RoadEdge, visited map[uint64]bool) RoadChain {
	visited[seed.ID] = true

	// Walk forward past seed.B.
	forwardEdges := []*RoadEdge{seed}
	cur := seed.B
	viaEdge := seed
	for {
		next, other, ok := nextChainStep(adj, cur, viaEdge)
		if !ok || visited[next.ID] {
			break
		}
		visited[next.ID] = true
		forwardEdges = append(forwardEdges, next)
		viaEdge = next
		cur = other
	}

	// Walk backward past seed.A.
	backwardEdges := make([]*RoadEdge, 0)
	cur = seed.A
	viaEdge = seed
	for {
		next, other, ok := nextChainStep(adj, cur, viaEdge)
		if !ok || visited[next.ID] {
			break
		}
		visited[next.ID] = true
		backwardEdges = append(backwardEdges, next)
		viaEdge = next
		cur = other
	}

	// Concatenate: reverse(backward) + forward. The chain's start node
	// is wherever the backward walk ended up; replay it from seed.A.
	edges := make([]*RoadEdge, 0, len(backwardEdges)+len(forwardEdges))
	for i := len(backwardEdges) - 1; i >= 0; i-- {
		edges = append(edges, backwardEdges[i])
	}
	edges = append(edges, forwardEdges...)

	startNode := seed.A
	for _, be := range backwardEdges {
		startNode = otherEndpoint(be, startNode)
	}

	nodes := make([]*RoadNode, 0, len(edges)+1)
	walker := startNode
	nodes = append(nodes, w.RoadNodeByID(walker))
	for _, e := range edges {
		walker = otherEndpoint(e, walker)
		nodes = append(nodes, w.RoadNodeByID(walker))
	}

	return RoadChain{Nodes: nodes, Edges: edges}
}

// nextChainStep returns the next edge to follow from `cur`, given we
// arrived via `viaEdge`. ok=false when `cur` has degree != 2 (a
// junction or dead-end terminates the chain).
func nextChainStep(adj map[uint64][]*RoadEdge, cur uint64, viaEdge *RoadEdge) (next *RoadEdge, otherNode uint64, ok bool) {
	edges := adj[cur]
	if len(edges) != 2 {
		return nil, 0, false
	}
	for _, e := range edges {
		if e.ID != viaEdge.ID {
			return e, otherEndpoint(e, cur), true
		}
	}
	return nil, 0, false
}

func otherEndpoint(e *RoadEdge, node uint64) uint64 {
	if e.A == node {
		return e.B
	}
	return e.A
}

// CatmullRomPoint evaluates a uniform Catmull-Rom spline at parameter
// t in [0, 1] between p1 and p2. p0 and p3 are the surrounding control
// points used to set the tangents at p1 and p2. The curve always
// passes through p1 at t=0 and p2 at t=1.
func CatmullRomPoint(p0, p1, p2, p3 mgl32.Vec2, t float32) mgl32.Vec2 {
	t2 := t * t
	t3 := t2 * t
	a0 := 2 * p1[0]
	b0 := -p0[0] + p2[0]
	c0 := 2*p0[0] - 5*p1[0] + 4*p2[0] - p3[0]
	d0 := -p0[0] + 3*p1[0] - 3*p2[0] + p3[0]
	a1 := 2 * p1[1]
	b1 := -p0[1] + p2[1]
	c1 := 2*p0[1] - 5*p1[1] + 4*p2[1] - p3[1]
	d1 := -p0[1] + 3*p1[1] - 3*p2[1] + p3[1]
	return mgl32.Vec2{
		0.5 * (a0 + b0*t + c0*t2 + d0*t3),
		0.5 * (a1 + b1*t + c1*t2 + d1*t3),
	}
}

// SampleRoadChain returns XZ samples along the Catmull-Rom spline
// threaded through the chain's nodes. samplesPerSegment is the number
// of sub-segments (and hence interior sample steps) per inter-node
// edge; the final endpoint is always appended exactly. Endpoint
// tangents use reflected phantom control points so the curve
// continues naturally past degree-1 / degree-≥3 chain endpoints.
//
// For a chain with N nodes, the result has length (N-1)*samplesPerSegment + 1.
func SampleRoadChain(chain RoadChain, samplesPerSegment int) []mgl32.Vec2 {
	if samplesPerSegment < 1 {
		samplesPerSegment = 1
	}
	n := len(chain.Nodes)
	if n < 2 {
		return nil
	}
	samples := make([]mgl32.Vec2, 0, (n-1)*samplesPerSegment+1)
	for i := 0; i < n-1; i++ {
		p1 := chain.Nodes[i].Pos
		p2 := chain.Nodes[i+1].Pos
		var p0, p3 mgl32.Vec2
		if i == 0 {
			p0 = mgl32.Vec2{2*p1[0] - p2[0], 2*p1[1] - p2[1]}
		} else {
			p0 = chain.Nodes[i-1].Pos
		}
		if i+2 >= n {
			p3 = mgl32.Vec2{2*p2[0] - p1[0], 2*p2[1] - p1[1]}
		} else {
			p3 = chain.Nodes[i+2].Pos
		}
		for j := 0; j < samplesPerSegment; j++ {
			t := float32(j) / float32(samplesPerSegment)
			samples = append(samples, CatmullRomPoint(p0, p1, p2, p3, t))
		}
	}
	samples = append(samples, chain.Nodes[n-1].Pos)
	return samples
}

// CumulativeChainDist returns the running arc-length along a sample
// list (Euclidean polyline length). cumDist[0] = 0, cumDist[N-1] =
// total length. Used by effects and renderer to map a "position along
// the curve" to a sample index.
func CumulativeChainDist(samples []mgl32.Vec2) []float32 {
	cum := make([]float32, len(samples))
	for i := 1; i < len(samples); i++ {
		dx := samples[i][0] - samples[i-1][0]
		dz := samples[i][1] - samples[i-1][1]
		cum[i] = cum[i-1] + float32(math.Sqrt(float64(dx*dx+dz*dz)))
	}
	return cum
}

// closestPointOnRoadSegment projects pos onto the line segment ab and
// returns the projection clamped to the segment. Plain Euclidean math
// in the world XZ plane — terrain elevation is ignored.
func closestPointOnRoadSegment(pos, a, b mgl32.Vec2) mgl32.Vec2 {
	dx := b[0] - a[0]
	dz := b[1] - a[1]
	len2 := dx*dx + dz*dz
	if len2 < 1e-6 {
		return a
	}
	t := ((pos[0]-a[0])*dx + (pos[1]-a[1])*dz) / len2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return mgl32.Vec2{a[0] + t*dx, a[1] + t*dz}
}

// SnapToRoadEdge returns the nearest road edge whose closest-point
// projection onto pos is within `tolerance` metres, plus that
// projection. Drives the "click on a road to continue building" flow:
// the placement tool projects the cursor onto the road network so
// the new segment originates / ends on an existing edge.
//
// Returns (nil, _, false) when no edge is in range.
func (w *World) SnapToRoadEdge(pos mgl32.Vec2, tolerance float32) (*RoadEdge, mgl32.Vec2, bool) {
	t2 := tolerance * tolerance
	var best *RoadEdge
	var bestPos mgl32.Vec2
	bestD2 := t2
	for _, e := range w.RoadEdges {
		na := w.RoadNodeByID(e.A)
		nb := w.RoadNodeByID(e.B)
		if na == nil || nb == nil {
			continue
		}
		cp := closestPointOnRoadSegment(pos, na.Pos, nb.Pos)
		dx := pos[0] - cp[0]
		dz := pos[1] - cp[1]
		d2 := dx*dx + dz*dz
		if d2 < bestD2 {
			best = e
			bestPos = cp
			bestD2 = d2
		}
	}
	if best == nil {
		return nil, mgl32.Vec2{}, false
	}
	return best, bestPos, true
}

// SplitRoadEdge replaces an existing edge with two edges meeting at a
// new intersection node placed at `pos`. The caller is expected to
// have produced `pos` from SnapToRoadEdge (or otherwise to ensure it
// lies on the segment) — this function does not re-project.
//
// Returns the new intersection node so callers can wire further
// segments to it.
func (w *World) SplitRoadEdge(edge *RoadEdge, pos mgl32.Vec2) *RoadNode {
	a := edge.A
	b := edge.B
	for i, e := range w.RoadEdges {
		if e.ID == edge.ID {
			w.RoadEdges = append(w.RoadEdges[:i], w.RoadEdges[i+1:]...)
			break
		}
	}
	n := w.AddRoadNode(pos, RoadNodeIntersection)
	w.AddRoadEdge(a, n.ID)
	w.AddRoadEdge(n.ID, b)
	return n
}
