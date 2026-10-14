package render

import (
	"math"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Bridges (world/bridge.go) carry footpaths and roads over water. The
// path or road strip itself is drawn on the deck (deckOrGround); this
// adds what shows of the bridge round it: the deck's edges and
// underside, and wooden rails on a footbridge or low concrete walls on a
// road bridge.

const (
	bridgeDeckDepth   = float32(0.35) // the deck's thickness under the path or road
	bridgePostEvery   = float32(1.6)  // metres between a footbridge's rail posts
	bridgePostSize    = float32(0.09) // a rail post's width
	bridgeRailHeight  = float32(1.0)  // a footbridge's top rail, over the deck
	bridgeRailSize    = float32(0.07) // the rail's thickness
	bridgeWallHeight  = float32(0.6)  // a road bridge's wall, over the deck
	bridgeWallWidth   = float32(0.25) // and its thickness
	bridgeSurfaceLift = float32(0.02) // the deck's top over the path or road strip
)

// The bridges' colours: weathered wood, and concrete.
var (
	footbridgeRGB = [3]float32{0.42, 0.31, 0.22}
	roadBridgeRGB = [3]float32{0.58, 0.57, 0.54}
)

// heightFunc is the height of what's drawn at world (x, z).
type heightFunc func(x, z float32) float32

// terrainHeight is the drawn ground's height (VisualElevationAt).
func terrainHeight(t *world.Terrain) heightFunc {
	return func(x, z float32) float32 { return VisualElevationAt(t, x, z) }
}

// deckOrGround is the drawn ground's height, or a bridge's deck where
// one carries a path or road over it.
func deckOrGround(w *world.World) heightFunc { return deckOrGroundWith(w, nil) }

// deckOrGroundWith is deckOrGround with the bridges of a ghost path or
// road (extra) too.
func deckOrGroundWith(w *world.World, extra []world.Bridge) heightFunc {
	ground := terrainHeight(w.Terrain)
	if len(w.Bridges) == 0 && len(extra) == 0 {
		return ground
	}
	return func(x, z float32) float32 {
		y := ground(x, z)
		p := mgl32.Vec2{x, z}
		if d, ok := w.BridgeDeckAt(p, ground); ok && d > y {
			y = d
		}
		if d, ok := world.DeckAmong(extra, p, ground); ok && d > y {
			y = d
		}
		return y
	}
}

// ghostBridgeMesh is the mesh for a ghost path or road's bridges, nil
// for none.
func ghostBridgeMesh(t *world.Terrain, bridges []world.Bridge) *Mesh {
	var m bridgeMesher
	for i := range bridges {
		m.bridge(&bridges[i], terrainHeight(t))
	}
	if len(m.idx) == 0 {
		return nil
	}
	return NewMesh(m.verts, m.idx, []int{3, 3, 2}, nil)
}

// bridgeMesher collects the faces of one kind of bridge.
type bridgeMesher struct {
	verts []float32
	idx   []uint32
}

// quad adds the quad a, b, c, d (in order round it) facing n.
func (m *bridgeMesher) quad(a, b, c, d, n mgl32.Vec3) {
	base := uint32(len(m.verts) / 8)
	for i, p := range [4]mgl32.Vec3{a, b, c, d} {
		u, v := float32(i&1), float32(i>>1)
		m.verts = append(m.verts, p[0], p[1], p[2], n[0], n[1], n[2], u, v)
	}
	// Both windings, so it shows from either side.
	m.idx = append(m.idx, base, base+1, base+2, base, base+2, base+3,
		base, base+2, base+1, base, base+3, base+2)
}

// box adds an upright box centred at c (its base), size s across along
// side and t along fwd, h tall.
func (m *bridgeMesher) box(c mgl32.Vec3, side, fwd mgl32.Vec2, s, t, h float32) {
	sx, fx := side.Mul(s/2), fwd.Mul(t/2)
	corner := func(a, b float32, y float32) mgl32.Vec3 {
		p := mgl32.Vec2{c[0], c[2]}.Add(sx.Mul(a)).Add(fx.Mul(b))
		return mgl32.Vec3{p[0], c[1] + y, p[1]}
	}
	up := mgl32.Vec3{0, 1, 0}
	n3 := func(v mgl32.Vec2) mgl32.Vec3 { return mgl32.Vec3{v[0], 0, v[1]} }
	m.quad(corner(-1, -1, 0), corner(1, -1, 0), corner(1, -1, h), corner(-1, -1, h), n3(fwd.Mul(-1)))
	m.quad(corner(-1, 1, 0), corner(1, 1, 0), corner(1, 1, h), corner(-1, 1, h), n3(fwd))
	m.quad(corner(-1, -1, 0), corner(-1, 1, 0), corner(-1, 1, h), corner(-1, -1, h), n3(side.Mul(-1)))
	m.quad(corner(1, -1, 0), corner(1, 1, 0), corner(1, 1, h), corner(1, -1, h), n3(side))
	m.quad(corner(-1, -1, h), corner(1, -1, h), corner(1, 1, h), corner(-1, 1, h), up)
}

// bridge adds one bridge: its deck's sides and underside, and its rails
// or walls.
func (m *bridgeMesher) bridge(b *world.Bridge, ground heightFunc) {
	n := len(b.Points)
	if n < 2 {
		return
	}
	y0, y1 := b.EndHeights(ground)
	hover := footpathHover
	if b.Road {
		hover = roadHoverOffset
	}
	type frame struct {
		c, side mgl32.Vec2
		y       float32
	}
	fr := make([]frame, n)
	for i := range b.Points {
		d := b.Points[min(i+1, n-1)].Sub(b.Points[max(i-1, 0)])
		if d.Len() < 1e-4 {
			d = mgl32.Vec2{1, 0}
		}
		d = d.Normalize()
		y := b.DeckAt(b.Along[i], y0, y1)
		fr[i] = frame{b.Points[i], mgl32.Vec2{-d[1], d[0]}, max(y, ground(b.Points[i][0], b.Points[i][1])) + hover + bridgeSurfaceLift}
	}
	at := func(f frame, s, dy float32) mgl32.Vec3 {
		p := f.c.Add(f.side.Mul(s))
		return mgl32.Vec3{p[0], f.y + dy, p[1]}
	}
	h := b.Half
	for i := 1; i < n; i++ {
		a, c := fr[i-1], fr[i]
		for _, s := range [2]float32{-1, 1} {
			out := mgl32.Vec3{a.side[0] * s, 0, a.side[1] * s}
			m.quad(at(a, s*h, -bridgeDeckDepth), at(c, s*h, -bridgeDeckDepth), at(c, s*h, 0), at(a, s*h, 0), out)
		}
		m.quad(at(a, -h, -bridgeDeckDepth), at(c, -h, -bridgeDeckDepth), at(c, h, -bridgeDeckDepth), at(a, h, -bridgeDeckDepth), mgl32.Vec3{0, -1, 0})
		if b.Road {
			// A low wall along each edge, on the deck.
			for _, s := range [2]float32{-1, 1} {
				in, outer := s*(h-bridgeWallWidth), s*h
				up := mgl32.Vec3{0, 1, 0}
				m.quad(at(a, in, 0), at(c, in, 0), at(c, in, bridgeWallHeight), at(a, in, bridgeWallHeight), mgl32.Vec3{-a.side[0] * s, 0, -a.side[1] * s})
				m.quad(at(a, outer, 0), at(c, outer, 0), at(c, outer, bridgeWallHeight), at(a, outer, bridgeWallHeight), mgl32.Vec3{a.side[0] * s, 0, a.side[1] * s})
				m.quad(at(a, in, bridgeWallHeight), at(c, in, bridgeWallHeight), at(c, outer, bridgeWallHeight), at(a, outer, bridgeWallHeight), up)
			}
			continue
		}
		// A footbridge's top rail along each edge.
		for _, s := range [2]float32{-1, 1} {
			e := s * (h - bridgePostSize/2)
			lo, hi := bridgeRailHeight-bridgeRailSize, bridgeRailHeight
			m.quad(at(a, e, lo), at(c, e, lo), at(c, e, hi), at(a, e, hi), mgl32.Vec3{a.side[0] * s, 0, a.side[1] * s})
			m.quad(at(a, e-s*bridgeRailSize, hi), at(c, e-s*bridgeRailSize, hi), at(c, e, hi), at(a, e, hi), mgl32.Vec3{0, 1, 0})
		}
	}
	if b.Road {
		return
	}
	// Posts along each edge, at both ends and evenly between.
	length := b.Length()
	posts := max(int(math.Ceil(float64(length/bridgePostEvery))), 1)
	k := 0
	for p := 0; p <= posts; p++ {
		u := length * float32(p) / float32(posts)
		for k+1 < n-1 && b.Along[k+1] < u {
			k++
		}
		a, c := fr[k], fr[min(k+1, n-1)]
		f := float32(0)
		if span := b.Along[min(k+1, n-1)] - b.Along[k]; span > 0 {
			f = min(max((u-b.Along[k])/span, 0), 1)
		}
		fm := frame{a.c.Add(c.c.Sub(a.c).Mul(f)), a.side, a.y + (c.y-a.y)*f}
		fwd := mgl32.Vec2{fm.side[1], -fm.side[0]}
		for _, s := range [2]float32{-1, 1} {
			m.box(at(fm, s*(h-bridgePostSize/2), 0), fm.side, fwd, bridgePostSize, bridgePostSize, bridgeRailHeight)
		}
	}
}

// syncBridges rebuilds the bridge meshes when the bridges have changed.
func (r *Renderer) syncBridges(w *world.World) {
	if r.scene.bridgeRev == w.BridgesRev && r.scene.bridgesBuilt {
		return
	}
	r.scene.bridgeRev, r.scene.bridgesBuilt = w.BridgesRev, true
	for _, m := range []**Mesh{&r.scene.footbridgeMesh, &r.scene.roadBridgeMesh} {
		if *m != nil {
			(*m).Delete()
			*m = nil
		}
	}
	var foot, road bridgeMesher
	ground := terrainHeight(w.Terrain)
	for i := range w.Bridges {
		b := &w.Bridges[i]
		if b.Road {
			road.bridge(b, ground)
		} else {
			foot.bridge(b, ground)
		}
	}
	if len(foot.idx) > 0 {
		r.scene.footbridgeMesh = NewMesh(foot.verts, foot.idx, []int{3, 3, 2}, nil)
	}
	if len(road.idx) > 0 {
		r.scene.roadBridgeMesh = NewMesh(road.verts, road.idx, []int{3, 3, 2}, nil)
	}
}

// drawBridges draws the bridges in the road pass (setFootpathAttribs'
// identity transform), weathered wood and concrete.
func (r *Renderer) drawBridges(w *world.World) {
	r.syncBridges(w)
	for _, b := range []struct {
		m   *Mesh
		rgb [3]float32
	}{{r.scene.footbridgeMesh, footbridgeRGB}, {r.scene.roadBridgeMesh, roadBridgeRGB}} {
		if b.m == nil {
			continue
		}
		gl.BindTexture(gl.TEXTURE_2D, r.whiteTexID)
		setFootpathAttribs()
		gl.VertexAttrib3f(7, b.rgb[0], b.rgb[1], b.rgb[2])
		b.m.Draw()
	}
}
