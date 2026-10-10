package render

import (
	"math"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Footpaths (world/footpath.go) are drawn as strips of cleared, gritted
// ground over the snow, as wide as each node says, with rounded ends.
// They sit just under the roads, so a path meeting a road tucks beneath
// its asphalt, and just over the parking lots.

const (
	footpathHover    = float32(0.16) // between parkingHoverOffset and roadHoverOffset
	footpathCapSteps = 6             // segments round a path's rounded end
)

// generateFootpathMesh builds every footpath's strip, nil when there are
// none.
func generateFootpathMesh(w *world.World) *Mesh {
	var verts []float32
	var idx []uint32
	for _, f := range w.Footpaths {
		verts, idx = appendFootpath(verts, idx, w.Terrain, f.Centerline())
	}
	if len(idx) == 0 {
		return nil
	}
	return NewMesh(verts, idx, []int{3, 3, 2}, nil)
}

// appendFootpath adds one path's strip and end caps along line.
func appendFootpath(verts []float32, idx []uint32, t *world.Terrain, line []world.TrailSample) ([]float32, []uint32) {
	if len(line) < 2 {
		return verts, idx
	}
	vert := func(p mgl32.Vec2) uint32 {
		verts = append(verts, p[0], VisualElevationAt(t, p[0], p[1])+footpathHover, p[1], 0, 1, 0, 0, 0)
		return uint32(len(verts)/8 - 1)
	}
	dirAt := func(i int) mgl32.Vec2 {
		a, b := line[max(i-1, 0)].Pos, line[min(i+1, len(line)-1)].Pos
		if d := b.Sub(a); d.Len() > 1e-4 {
			return d.Normalize()
		}
		return mgl32.Vec2{1, 0}
	}
	var prevL, prevR uint32
	for i, s := range line {
		d := dirAt(i)
		n := mgl32.Vec2{-d[1], d[0]}.Mul(s.Width / 2)
		l, r := vert(s.Pos.Add(n)), vert(s.Pos.Sub(n))
		if i > 0 {
			idx = append(idx, prevL, prevR, l, prevR, r, l)
		}
		prevL, prevR = l, r
	}
	// Rounded ends: a half disc beyond each end.
	for _, end := range []struct {
		s   world.TrailSample
		out mgl32.Vec2
	}{{line[0], dirAt(0).Mul(-1)}, {line[len(line)-1], dirAt(len(line) - 1)}} {
		c := vert(end.s.Pos)
		side := mgl32.Vec2{-end.out[1], end.out[0]}
		var prev uint32
		for k := 0; k <= footpathCapSteps; k++ {
			a := math.Pi * float64(k) / footpathCapSteps
			p := end.s.Pos.Add(side.Mul(float32(math.Cos(a)) * end.s.Width / 2)).Add(end.out.Mul(float32(math.Sin(a)) * end.s.Width / 2))
			v := vert(p)
			if k > 0 {
				idx = append(idx, c, prev, v)
			}
			prev = v
		}
	}
	return verts, idx
}

// SetFootpathGhost shows the path being drawn through nodes (with the
// next node at the cursor), red when it can't be laid; nil nodes clear it.
func (r *Renderer) SetFootpathGhost(w *world.World, nodes []world.TrailNode, ok bool) {
	if r.scene.footpathGhost != nil {
		r.scene.footpathGhost.Delete()
		r.scene.footpathGhost = nil
	}
	if len(nodes) < 2 {
		return
	}
	f := world.Footpath{Nodes: nodes}
	verts, idx := appendFootpath(nil, nil, w.Terrain, f.Centerline())
	if len(idx) > 0 {
		r.scene.footpathGhost = NewMesh(verts, idx, []int{3, 3, 2}, nil)
		r.scene.footpathGhostOK = ok
	}
}

// syncFootpaths rebuilds the footpath mesh when the paths have changed.
func (r *Renderer) syncFootpaths(w *world.World) {
	if r.scene.footpathRev == w.FootpathsRev && (r.scene.footpathMesh != nil || len(w.Footpaths) == 0) {
		return
	}
	if r.scene.footpathMesh != nil {
		r.scene.footpathMesh.Delete()
		r.scene.footpathMesh = nil
	}
	r.scene.footpathMesh = generateFootpathMesh(w)
	r.scene.footpathRev = w.FootpathsRev
}

// setFootpathAttribs is the footpath counterpart of setRoadTransformAttribs:
// identity transform and gritted packed snow, a warm grey between the
// snow and the asphalt.
func setFootpathAttribs() {
	gl.VertexAttrib4f(3, 1, 0, 0, 0)
	gl.VertexAttrib4f(4, 0, 1, 0, 0)
	gl.VertexAttrib4f(5, 0, 0, 1, 0)
	gl.VertexAttrib4f(6, 0, 0, 0, 1)
	gl.VertexAttrib3f(7, 0.55, 0.49, 0.42)
}
