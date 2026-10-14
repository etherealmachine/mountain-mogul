package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Bridges (notes/next/Water Crossings.md): wherever a footpath or a road
// crosses a creek or a lake (the material map's water: open, snow-bridged
// or frozen, so a bridge doesn't come and go with the season), it's
// carried over on a bridge, from a little way up one bank to the same on
// the other. They're made from the paths and roads (RebuildBridges), not
// placed or saved. A bridge's deck runs straight between its ends' ground
// with a slight arch, and the path or road, and whoever's on it, is drawn
// on the deck; its cells are open to the route searches over the water
// (RefreshOpenWater).

// Bridge is a stretch of footpath or road carried over water: its centre
// line from bank to bank, and half its deck's width.
type Bridge struct {
	Road   bool
	Points []mgl32.Vec2
	Half   float32
	// Along is each point's distance from the first, metres.
	Along []float32
}

const (
	// bridgeAbutment is how far past the water's edge a bridge reaches
	// onto each bank, metres.
	bridgeAbutment = 1.5
	// bridgeJoin is the widest dry gap, metres, between two stretches of
	// water one bridge spans.
	bridgeJoin = 3.0
	// bridgeStep is the spacing a line is checked for water at, and of a
	// bridge's points, metres.
	bridgeStep = 0.5
	// BridgeArch is how much higher than its ends a bridge's middle is,
	// metres, for a span of bridgeArchSpan or more.
	BridgeArch     = 0.5
	bridgeArchSpan = 12.0
	// FootbridgeCostPerM2 is what a square metre of footbridge deck
	// costs, over the path's own; RoadBridgeCostPerM is a metre of road
	// bridge.
	FootbridgeCostPerM2 = 150
	// bridgeEdgeSlack is how far past its deck's edge a point still
	// counts as on a bridge, metres: a path's edge is the deck's.
	bridgeEdgeSlack    = 0.1
	RoadBridgeCostPerM = 1500
)

// Length is the bridge's length along its centre line.
func (b *Bridge) Length() float32 {
	if len(b.Along) == 0 {
		return 0
	}
	return b.Along[len(b.Along)-1]
}

// DeckAt is the deck's height at u metres along the bridge, given its
// ends' heights.
func (b *Bridge) DeckAt(u, y0, y1 float32) float32 {
	l := b.Length()
	if l <= 0 {
		return y0
	}
	f := min(max(u/l, 0), 1)
	arch := BridgeArch * min(l/bridgeArchSpan, 1) * float32(math.Sin(math.Pi*float64(f)))
	return y0 + (y1-y0)*f + arch
}

// Nearest is where on the bridge's centre line p is nearest: metres
// along it, and how far off it.
func (b *Bridge) Nearest(p mgl32.Vec2) (along, off float32) {
	off = float32(math.Inf(1))
	for i := 1; i < len(b.Points); i++ {
		d, q := segDist(p, b.Points[i-1], b.Points[i])
		if d < off {
			off = d
			along = b.Along[i-1] + q.Sub(b.Points[i-1]).Len()
		}
	}
	return along, off
}

// BridgeDeckAt is the deck's height at world point p, when p is on a
// bridge, with ground giving the height of the ground at its ends (the
// sim's snow surface, or what the renderer draws).
func (w *World) BridgeDeckAt(p mgl32.Vec2, ground func(x, z float32) float32) (float32, bool) {
	return DeckAmong(w.Bridges, p, ground)
}

// DeckAmong is BridgeDeckAt over bridges: the deck's height at p when
// it's on one of them.
func DeckAmong(bridges []Bridge, p mgl32.Vec2, ground func(x, z float32) float32) (float32, bool) {
	for i := range bridges {
		b := &bridges[i]
		if !b.near(p, b.Half+bridgeEdgeSlack) {
			continue
		}
		along, off := b.Nearest(p)
		if off > b.Half+bridgeEdgeSlack {
			continue
		}
		y0, y1 := b.EndHeights(ground)
		return b.DeckAt(along, y0, y1), true
	}
	return 0, false
}

// EndHeights is the ground's height at the bridge's two ends.
func (b *Bridge) EndHeights(ground func(x, z float32) float32) (float32, float32) {
	p, q := b.Points[0], b.Points[len(b.Points)-1]
	return ground(p[0], p[1]), ground(q[0], q[1])
}

// near reports whether p is within reach of the bridge's bounding box.
func (b *Bridge) near(p mgl32.Vec2, reach float32) bool {
	lo, hi := b.Points[0], b.Points[0]
	for _, q := range b.Points {
		lo = mgl32.Vec2{min(lo[0], q[0]), min(lo[1], q[1])}
		hi = mgl32.Vec2{max(hi[0], q[0]), max(hi[1], q[1])}
	}
	return p[0] >= lo[0]-reach && p[0] <= hi[0]+reach && p[1] >= lo[1]-reach && p[1] <= hi[1]+reach
}

// RebuildBridges remakes the bridges from the footpaths and roads and the
// water under them.
func (w *World) RebuildBridges() {
	var out []Bridge
	if t := w.Terrain; t != nil && t.Material != nil {
		for _, f := range w.Footpaths {
			out = append(out, bridgesAlong(t.Material, f.Centerline(), false)...)
		}
		for _, c := range w.FindRoadChains() {
			var line []TrailSample
			for _, p := range SampleRoadChain(c, t, RoadChainSamplesPerSegment) {
				line = append(line, TrailSample{Pos: p, Width: 2 * RoadHalfWidth})
			}
			out = append(out, bridgesAlong(t.Material, line, true)...)
		}
	}
	w.Bridges = out
	w.BridgesRev++
	w.RebuildTunnels()
}

// BridgesFor is the bridges a footpath (or, with road, a road) along
// line would need, for a ghost before it's laid.
func (w *World) BridgesFor(line []TrailSample, road bool) []Bridge {
	if w.Terrain == nil || w.Terrain.Material == nil {
		return nil
	}
	return bridgesAlong(w.Terrain.Material, line, road)
}

// BridgeCostAlong is what the bridges a footpath (or, with road, a road)
// along line would need cost, over the path or road itself.
func (w *World) BridgeCostAlong(line []TrailSample, road bool) int {
	if w.Terrain == nil || w.Terrain.Material == nil {
		return 0
	}
	cost := 0
	for _, b := range w.BridgesFor(line, road) {
		if road {
			cost += int(b.Length() * RoadBridgeCostPerM)
		} else {
			cost += int(b.Length() * 2 * b.Half * FootbridgeCostPerM2)
		}
	}
	return cost
}

// bridgesAlong finds where line crosses water (at its centre or either
// edge) and makes a bridge over each crossing.
func bridgesAlong(m *TerrainMaterial, line []TrailSample, road bool) []Bridge {
	if len(line) < 2 {
		return nil
	}
	// Resample the line every bridgeStep metres, with its width.
	var pts []TrailSample
	var along []float32
	pts = append(pts, line[0])
	along = append(along, 0)
	for i := 1; i < len(line); i++ {
		a, b := line[i-1], line[i]
		l := b.Pos.Sub(a.Pos).Len()
		n := max(int(math.Ceil(float64(l/bridgeStep))), 1)
		for k := 1; k <= n; k++ {
			f := float32(k) / float32(n)
			pts = append(pts, TrailSample{Pos: a.Pos.Add(b.Pos.Sub(a.Pos).Mul(f)), Width: a.Width + (b.Width-a.Width)*f})
			along = append(along, along[len(along)-1]+l/float32(n))
		}
	}
	wet := func(i int) bool {
		p := pts[i].Pos
		j := max(i-1, 0)
		k := min(i+1, len(pts)-1)
		d := pts[k].Pos.Sub(pts[j].Pos)
		if d.Len() < 1e-4 {
			return m.At(p[0], p[1]).IsWater()
		}
		side := mgl32.Vec2{-d[1], d[0]}.Normalize().Mul(pts[i].Width / 2)
		for _, q := range []mgl32.Vec2{p, p.Add(side), p.Sub(side)} {
			if m.At(q[0], q[1]).IsWater() {
				return true
			}
		}
		return false
	}
	// Stretches of water, as distances along the line, joined across
	// short dry gaps.
	type span struct{ s0, s1 float32 }
	var spans []span
	for i := range pts {
		if !wet(i) {
			continue
		}
		if n := len(spans); n > 0 && along[i]-spans[n-1].s1 <= bridgeJoin {
			spans[n-1].s1 = along[i]
		} else {
			spans = append(spans, span{along[i], along[i]})
		}
	}
	total := along[len(along)-1]
	var out []Bridge
	for _, sp := range spans {
		s0, s1 := max(sp.s0-bridgeAbutment, 0), min(sp.s1+bridgeAbutment, total)
		b := Bridge{Road: road}
		for i := range pts {
			if along[i] < s0 || along[i] > s1 {
				continue
			}
			b.Points = append(b.Points, pts[i].Pos)
			b.Half = max(b.Half, pts[i].Width/2)
		}
		if len(b.Points) < 2 {
			continue
		}
		b.Along = make([]float32, len(b.Points))
		for i := 1; i < len(b.Points); i++ {
			b.Along[i] = b.Along[i-1] + b.Points[i].Sub(b.Points[i-1]).Len()
		}
		out = append(out, b)
	}
	return out
}

// FootpathPrice is what laying a path through nodes costs, with the
// footbridges it needs over water and the tunnels under hills.
func (w *World) FootpathPrice(nodes []TrailNode) int {
	f := Footpath{Nodes: nodes}
	cost := FootpathCost(nodes) + w.BridgeCostAlong(f.Centerline(), false)
	if w.Terrain != nil && len(nodes) >= 2 {
		cost += TunnelCost(w.FootpathLine(nodes), false)
	}
	return cost
}

// RoadPrice is what a road from a to b costs, with the bridge it needs
// over water and the tunnels under hills.
func (w *World) RoadPrice(a, b mgl32.Vec2) int {
	line := []TrailSample{{Pos: a, Width: 2 * RoadHalfWidth}, {Pos: b, Width: 2 * RoadHalfWidth}}
	cost := RoadCost(a, b) + w.BridgeCostAlong(line, true)
	if w.Terrain != nil {
		cost += TunnelCost(w.RoadLine(a, b), true)
	}
	return cost
}
