package render

import (
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Where roads meet, at a junction or a parking lot's entrance, each road
// stops short of the meeting point and a patch fills the gap: a polygon
// round the point whose sides curve from one road's edge to the next
// (kerb returns), so the asphalt joins with rounded corners instead of
// square road ends overlapping. A lot's entrance is a junction of the
// driveway with the lot's edge, which counts as two roads of no width
// running each way along it; the lot's own asphalt fills the rest.

const (
	// roadFilletR is how far a kerb return runs along each road's edge
	// from where the two edges would meet.
	roadFilletR = float32(4.0)
	// roadFilletSteps is the segments drawn along each kerb return.
	roadFilletSteps = 6
	// roadTrimMax caps how far a road is trimmed back from a junction,
	// for roads that meet at a very sharp angle.
	roadTrimMax = float32(25)
	// roadDashClear is the gap between a road's last dash and the patch.
	roadDashClear = float32(1.5)
)

// roadArm is one road leaving a junction: the chain and which end of it,
// its direction away from the junction, its half width, and how far it's
// trimmed back. A lot edge's arms have no chain (chain -1) and no width;
// trimMax caps their trim at the lot's rounded corner.
type roadArm struct {
	chain   int
	atStart bool
	dir     mgl32.Vec2
	angle   float32
	hw      float32
	trim    float32
	trimMax float32
	// end and endDir are where the drawn road stops and its direction
	// there, away from the junction: on a curving road, not quite at
	// dir × trim.
	end, endDir mgl32.Vec2
}

// roadJunction is a meeting point and its arms in angle order.
type roadJunction struct {
	at   mgl32.Vec2
	arms []*roadArm
}

// roadLayout is the drawn roads: each chain's centreline samples,
// trimmed back from the junctions at its ends, and the junctions.
type roadLayout struct {
	samples   [][]mgl32.Vec2
	junctions []*roadJunction
}

// layoutRoads samples every chain and trims it back from the junctions
// and lot entrances at its ends.
func layoutRoads(w *world.World, t *world.Terrain) roadLayout {
	chains := w.FindRoadChains()
	var lay roadLayout
	lay.samples = make([][]mgl32.Vec2, len(chains))
	degree := map[uint64]int{}
	for _, e := range w.RoadEdges {
		degree[e.A]++
		degree[e.B]++
	}
	gates := map[uint64]*world.Building{}
	for _, b := range w.Buildings {
		if b.IsRectLot() && len(b.DrivewayNodeIDs) > 0 {
			gates[b.DrivewayNodeIDs[0]] = b
		}
	}
	byNode := map[uint64]*roadJunction{}
	junctionAt := func(n *world.RoadNode) *roadJunction {
		if degree[n.ID] < 3 && gates[n.ID] == nil {
			return nil
		}
		j := byNode[n.ID]
		if j == nil {
			j = &roadJunction{at: n.Pos}
			byNode[n.ID] = j
			lay.junctions = append(lay.junctions, j)
			if b := gates[n.ID]; b != nil {
				j.arms = append(j.arms, lotEdgeArms(b, n.Pos)...)
			}
		}
		return j
	}
	for ci, c := range chains {
		s := world.SampleRoadChain(c, t, world.RoadChainSamplesPerSegment)
		lay.samples[ci] = s
		if len(s) < 2 || len(c.Nodes) < 2 || c.Nodes[0] == c.Nodes[len(c.Nodes)-1] {
			continue
		}
		for _, end := range []struct {
			n       *world.RoadNode
			atStart bool
		}{{c.Nodes[0], true}, {c.Nodes[len(c.Nodes)-1], false}} {
			j := junctionAt(end.n)
			if j == nil {
				continue
			}
			dir := armDir(s, end.atStart)
			if dir == (mgl32.Vec2{}) {
				continue
			}
			length := world.CumulativeChainDist(s)[len(s)-1]
			j.arms = append(j.arms, &roadArm{chain: ci, atStart: end.atStart, dir: dir, hw: world.RoadHalfWidth,
				trim: world.RoadHalfWidth, trimMax: min(roadTrimMax, length*0.45)})
		}
	}
	for _, j := range lay.junctions {
		j.fit()
	}
	// Trim each chain by its arms' trims.
	trimStart := make([]float32, len(chains))
	trimEnd := make([]float32, len(chains))
	for _, j := range lay.junctions {
		for _, a := range j.arms {
			if a.chain < 0 {
				continue
			}
			if a.atStart {
				trimStart[a.chain] = a.trim
			} else {
				trimEnd[a.chain] = a.trim
			}
		}
	}
	for _, j := range lay.junctions {
		for _, a := range j.arms {
			a.end, a.endDir = j.at.Add(a.dir.Mul(a.trim)), a.dir
			if a.chain < 0 {
				continue
			}
			sm := lay.samples[a.chain]
			cum := world.CumulativeChainDist(sm)
			d := a.trim
			if !a.atStart {
				d = cum[len(cum)-1] - a.trim
			}
			a.end = pointAlongChain(sm, cum, d)
			a.endDir = tangentAlongChain(sm, cum, d)
			if !a.atStart {
				a.endDir = a.endDir.Mul(-1)
			}
		}
	}
	for ci, s := range lay.samples {
		if trimStart[ci] > 0 || trimEnd[ci] > 0 {
			lay.samples[ci] = trimSamples(s, trimStart[ci], trimEnd[ci])
		}
	}
	return lay
}

// armDir is the direction a chain leaves its start (or end) by, from the
// end to a point a few metres along it.
func armDir(s []mgl32.Vec2, atStart bool) mgl32.Vec2 {
	cum := world.CumulativeChainDist(s)
	d := min(float32(4), cum[len(cum)-1])
	var from, to mgl32.Vec2
	if atStart {
		from, to = s[0], pointAlongChain(s, cum, d)
	} else {
		from, to = s[len(s)-1], pointAlongChain(s, cum, cum[len(cum)-1]-d)
	}
	v := to.Sub(from)
	if v.Len() < 1e-3 {
		return mgl32.Vec2{}
	}
	return v.Normalize()
}

// lotEdgeArms are the two arms along the edge of lot b that its entrance
// at is on, each able to trim back as far as the lot's rounded corner.
func lotEdgeArms(b *world.Building, at mgl32.Vec2) []*roadArm {
	r := b.LotRect()
	ax, az := r.Axes()
	rel := at.Sub(r.Center)
	lx, lz := rel.Dot(ax), rel.Dot(az)
	// The entrance is on whichever edge it's nearest.
	edge, along, half := az, lz, r.HalfZ
	if math.Abs(float64(math.Abs(float64(lz))-float64(r.HalfZ))) < math.Abs(float64(math.Abs(float64(lx))-float64(r.HalfX))) {
		edge, along, half = ax, lx, r.HalfX
	}
	var arms []*roadArm
	for _, s := range []float32{1, -1} {
		room := half - s*along - world.LotCornerRadius
		arms = append(arms, &roadArm{chain: -1, dir: edge.Mul(s), trimMax: max(room, 0)})
	}
	return arms
}

// fit sorts the arms by angle and trims each back far enough for the
// kerb returns to its neighbours.
func (j *roadJunction) fit() {
	for _, a := range j.arms {
		a.angle = float32(math.Atan2(float64(a.dir[1]), float64(a.dir[0])))
	}
	sort.Slice(j.arms, func(p, q int) bool { return j.arms[p].angle < j.arms[q].angle })
	n := len(j.arms)
	if n < 2 {
		return
	}
	for i := range j.arms {
		a, b := j.arms[i], j.arms[(i+1)%n]
		s, t, ok := edgeMeet(a, b)
		if !ok {
			continue
		}
		a.trim = max(a.trim, s+roadFilletR)
		b.trim = max(b.trim, t+roadFilletR)
	}
	for _, a := range j.arms {
		a.trim = min(a.trim, a.trimMax)
	}
}

// edgeMeet is where arm a's anticlockwise edge meets arm b's clockwise
// edge (b being a's next arm anticlockwise), as distances along each from
// the junction (negative when the edges cross behind it, as on the wide
// side of a road meeting a lot edge at a slant); ok is false when the
// arms are half a turn or more apart, so there's no corner between them.
func edgeMeet(a, b *roadArm) (s, t float32, ok bool) {
	gap := b.angle - a.angle
	if gap <= 0 {
		gap += 2 * math.Pi
	}
	if gap > math.Pi-0.05 {
		return 0, 0, false
	}
	return lineMeet(leftOf(a.dir).Mul(a.hw), a.dir, leftOf(b.dir).Mul(-b.hw), b.dir)
}

// lineMeet is where the lines p + u·dp and q + v·dq cross, as u and v.
func lineMeet(p, dp, q, dq mgl32.Vec2) (u, v float32, ok bool) {
	det := dp[0]*(-dq[1]) - (-dq[0])*dp[1]
	if math.Abs(float64(det)) < 0.05 {
		return 0, 0, false
	}
	rx, rz := q[0]-p[0], q[1]-p[1]
	u = (rx*(-dq[1]) - (-dq[0])*rz) / det
	v = (dp[0]*rz - dp[1]*rx) / det
	return u, v, true
}

// leftOf is d turned a quarter anticlockwise (toward increasing angle).
func leftOf(d mgl32.Vec2) mgl32.Vec2 { return mgl32.Vec2{-d[1], d[0]} }

// outline is the junction patch's boundary, anticlockwise: across each
// arm's trimmed end, then along the kerb return to the next arm.
func (j *roadJunction) outline() []mgl32.Vec2 {
	var pts []mgl32.Vec2
	n := len(j.arms)
	for i, a := range j.arms {
		b := j.arms[(i+1)%n]
		na := leftOf(a.endDir)
		from := a.end.Add(na.Mul(a.hw))
		to := b.end.Sub(leftOf(b.endDir).Mul(b.hw))
		pts = append(pts, a.end.Sub(na.Mul(a.hw)), from)
		if _, _, ok := edgeMeet(a, b); !ok {
			continue // a straight side
		}
		// The kerb return bends round the corner where the two edges,
		// running back from the road ends, would meet.
		u, v, ok := lineMeet(from, a.endDir, to, b.endDir)
		if !ok || u > 0 || v > 0 {
			continue
		}
		ctrl := from.Add(a.endDir.Mul(u))
		for k := 1; k < roadFilletSteps; k++ {
			u := float32(k) / roadFilletSteps
			p := from.Mul((1 - u) * (1 - u)).Add(ctrl.Mul(2 * u * (1 - u))).Add(to.Mul(u * u))
			pts = append(pts, p)
		}
	}
	return pts
}

// appendJunctionPatch adds junction j's patch to a road mesh: a fan from
// the meeting point, in rings about roadStripStep apart so the asphalt
// follows the ground.
func appendJunctionPatch(verts []float32, idx []uint32, j *roadJunction, t *world.Terrain, base uint32) ([]float32, []uint32) {
	o := j.outline()
	if len(o) < 3 {
		return verts, idx
	}
	pts := densify(append(o, o[0]), roadStripStep)
	pts = pts[:len(pts)-1]
	var far float32
	for _, p := range pts {
		far = max(far, p.Sub(j.at).Len())
	}
	rings := max(1, int(math.Ceil(float64(far/roadStripStep))))
	vert := func(p mgl32.Vec2) {
		verts = append(verts, p[0], VisualElevationAt(t, p[0], p[1])+roadHoverOffset, p[1], 0, 1, 0, 0, 0)
	}
	vert(j.at)
	for _, p := range pts {
		for r := 1; r <= rings; r++ {
			vert(j.at.Add(p.Sub(j.at).Mul(float32(r) / float32(rings))))
		}
	}
	n, m := uint32(len(pts)), uint32(rings)
	at := func(i, r uint32) uint32 { return base + 1 + i*m + r } // ring r (0 innermost) on spoke i
	for i := uint32(0); i < n; i++ {
		k := (i + 1) % n
		idx = append(idx, base, at(k, 0), at(i, 0))
		for r := uint32(0); r+1 < m; r++ {
			idx = append(idx, at(i, r), at(k, r), at(k, r+1), at(i, r), at(k, r+1), at(i, r+1))
		}
	}
	return verts, idx
}

// trimSamples cuts a centreline from arc length a to its length less b.
func trimSamples(s []mgl32.Vec2, a, b float32) []mgl32.Vec2 {
	cum := world.CumulativeChainDist(s)
	end := cum[len(cum)-1] - b
	if end-a < 0.5 {
		return nil
	}
	// Samples within a few centimetres of a cut are left out, so no
	// segment is too short to have a direction.
	const near = 0.05
	out := []mgl32.Vec2{pointAlongChain(s, cum, a)}
	for i, p := range s {
		if cum[i] > a+near && cum[i] < end-near {
			out = append(out, p)
		}
	}
	return append(out, pointAlongChain(s, cum, end))
}
