package scene

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// drawSkiArea shades the ski-area boundary into a cell overlay with set:
// cells inside at alpha inner (0 skips them), cells on its edge at alpha
// edge, in lavender. Shared by the game and the editor; reads the
// world's cached mask, so it's cheap every frame.
func drawSkiArea(w *world.World, inner, edge uint8, set func(cx, cz int, r, g, b, a uint8)) {
	m := w.SkiAreaMask()
	if m == nil {
		return
	}
	t := w.Terrain
	in := func(x, z int) bool {
		return x >= 0 && z >= 0 && x < t.Width && z < t.Height && m[z*t.Width+x]
	}
	for z := 0; z < t.Height; z++ {
		for x := 0; x < t.Width; x++ {
			if !m[z*t.Width+x] {
				continue
			}
			a := inner
			if !in(x-1, z) || !in(x+1, z) || !in(x, z-1) || !in(x, z+1) {
				a = edge
			}
			if a > 0 {
				set(x, z, 170, 90, 220, a)
			}
		}
	}
}

// buildSkiAreaFence is the ski-area boundary as a rope line: thin orange
// poles every skiFencePoleGap metres along each outline, standing on the
// snow, with a drooping rope between them, as resorts mark their
// boundary. It leaves a gap wherever it would cross a building, a parking
// lot, or a road (fenceBlocked). Same buffers as the parcel fence
// (render.SetFencePostVerts, SetBoundaryLines); rebuilt every clock hour
// so it rides the snow as it falls and melts.
func buildSkiAreaFence(w *world.World) (postVerts []float32, ropeLines []render.DebugLine) {
	t := w.Terrain
	const (
		poleHW     = float32(0.04) // half-width: an 8 cm pole
		poleTop    = float32(1.6)
		ropeAttach = float32(1.1)
		ropeSag    = float32(0.25)
	)
	orange1 := [3]float32{0.85, 0.36, 0.07}
	orange2 := [3]float32{0.66, 0.27, 0.05}
	ropeCol := [3]float32{0.80, 0.66, 0.14}
	quad := func(v0, v1, v2, v3 [3]float32, c [3]float32) {
		for _, v := range [6][3]float32{v0, v1, v2, v0, v2, v3} {
			postVerts = append(postVerts, v[0], v[1], v[2], c[0], c[1], c[2])
		}
	}
	pole := func(px, py, pz float32) {
		hw, top := poleHW, py+poleTop
		quad([3]float32{px - hw, top, pz + hw}, [3]float32{px + hw, top, pz + hw}, [3]float32{px + hw, py, pz + hw}, [3]float32{px - hw, py, pz + hw}, orange1)
		quad([3]float32{px + hw, top, pz - hw}, [3]float32{px - hw, top, pz - hw}, [3]float32{px - hw, py, pz - hw}, [3]float32{px + hw, py, pz - hw}, orange1)
		quad([3]float32{px + hw, top, pz - hw}, [3]float32{px + hw, top, pz + hw}, [3]float32{px + hw, py, pz + hw}, [3]float32{px + hw, py, pz - hw}, orange2)
		quad([3]float32{px - hw, top, pz + hw}, [3]float32{px - hw, top, pz - hw}, [3]float32{px - hw, py, pz - hw}, [3]float32{px - hw, py, pz + hw}, orange2)
	}
	maxX, maxZ := float32(t.Width)*world.CellSize, float32(t.Height)*world.CellSize
	ground := func(x, z float32) float32 {
		return t.InterpolatedSurfaceElevationAt(min(max(x, 0), maxX-0.01), min(max(z, 0), maxZ-0.01))
	}
	for _, o := range w.SkiArea {
		n := len(o.Points)
		if n < 3 {
			continue
		}
		// Poles evenly along the closed outline.
		var poles []mgl32.Vec2
		for i := 0; i < n; i++ {
			a, b := o.Points[i], o.Points[(i+1)%n]
			l := b.Sub(a).Len()
			steps := max(int(math.Ceil(float64(l/skiFencePoleGap))), 1)
			for k := 0; k < steps; k++ {
				poles = append(poles, a.Add(b.Sub(a).Mul(float32(k)/float32(steps))))
			}
		}
		ys := make([]float32, len(poles))
		kept := make([]bool, len(poles))
		for i, p := range poles {
			if fenceBlocked(w, p) {
				continue
			}
			kept[i] = true
			ys[i] = ground(p[0], p[1])
			pole(p[0], ys[i], p[1])
		}
		for i := range poles {
			j := (i + 1) % len(poles)
			if !kept[i] || !kept[j] ||
				fenceBlocked(w, poles[i].Add(poles[j].Sub(poles[i]).Mul(1.0/3))) ||
				fenceBlocked(w, poles[i].Add(poles[j].Sub(poles[i]).Mul(2.0/3))) {
				continue
			}
			a := mgl32.Vec3{poles[i][0], ys[i] + ropeAttach, poles[i][1]}
			b := mgl32.Vec3{poles[j][0], ys[j] + ropeAttach, poles[j][1]}
			mid := a.Add(b).Mul(0.5)
			mid[1] -= ropeSag
			ropeLines = append(ropeLines,
				render.DebugLine{A: a, B: mid, Color: ropeCol},
				render.DebugLine{A: mid, B: b, Color: ropeCol})
		}
	}
	return postVerts, ropeLines
}

// skiFencePoleGap is the spacing between boundary poles, in metres.
const skiFencePoleGap = 8.0

// fenceBlocked reports whether a boundary pole or rope at p would stand
// in a building, a parking lot, or a road, where the fence leaves a gap.
func fenceBlocked(w *world.World, p mgl32.Vec2) bool {
	for _, b := range w.Buildings {
		if b.FootprintContains(p[0], p[1], 1) {
			return true
		}
	}
	const reach = world.RoadHalfWidth + 1
	for _, e := range w.RoadEdges {
		a, b := w.RoadNodeByID(e.A), w.RoadNodeByID(e.B)
		if a != nil && b != nil && pointSegmentDistSq(p, a.Pos, b.Pos) <= reach*reach {
			return true
		}
	}
	return false
}
