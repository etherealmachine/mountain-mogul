package render

import (
	"fmt"
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Tunnel mouths' cut-outs in the terrain (terrain.frag, inMouth): at a
// mouth the ground is graded to the bed in front of the portal's face and
// left as the hill behind it, and the ground's grid slopes from one to the
// other across the face, through the opening. The terrain isn't drawn
// where that slope would show: in front of the face, and in the opening
// as far in as the portal's dark inside. A handful at most, so they go to
// the shader as uniforms.

const maxMouths = 16

// mouthFront is how far in front of a portal's face its cut-out reaches
// (terrain.frag's inMouth): the ground's grid's slope up to the hill.
const mouthFront = 2.0

// tunnelMouth is one mouth's cut-out: A (x, z, heading, the bed's height)
// and B (the face's half-width, the opening's half-width and height, the
// dark inside's depth), from its portal's model (models-src/lib/
// portal_kit.py: the face 5 m wider than the opening, the inside 2 m
// deep).
type tunnelMouth struct{ a, b mgl32.Vec4 }

// mouthSizes are a portal's: opening width and height.
var (
	railMouth = mgl32.Vec2{5.0, 6.8}
	roadMouth = mgl32.Vec2{9.0, 6.0}
	pathMouth = mgl32.Vec2{3.2, 3.2}
)

func newMouth(p world.TunnelPortal, bed float32, opening mgl32.Vec2) tunnelMouth {
	return tunnelMouth{
		a: mgl32.Vec4{p.Pos[0], p.Pos[1], p.Heading, bed},
		b: mgl32.Vec4{opening[0]/2 + 2.5, opening[0] / 2, opening[1], 2.0},
	}
}

// collectMouths gathers the world's tunnel mouths: railways', funiculars',
// roads' and paths'.
func collectMouths(w *world.World) []tunnelMouth {
	var out []tunnelMouth
	for _, tr := range w.RailTracks {
		for _, p := range tr.Portals() {
			out = append(out, newMouth(p, p.Y-world.RailBallast, railMouth))
		}
	}
	for _, lift := range w.Lifts {
		if tr := lift.FunicularTrack(w.Terrain); tr != nil {
			for _, p := range tr.Portals() {
				out = append(out, newMouth(p, p.Y-world.RailBallast, railMouth))
			}
		}
	}
	for _, tn := range w.Tunnels {
		size := pathMouth
		if tn.Course.Road {
			size = roadMouth
		}
		for _, p := range tn.Mouths {
			out = append(out, newMouth(p, p.Y, size))
		}
	}
	if len(out) > maxMouths {
		out = out[:maxMouths]
	}
	return out
}

// applyMouths sets the terrain shader's mouth cut-outs.
func applyMouths(s *Shader, mouths []tunnelMouth) {
	s.SetInt("uMouthCount", len(mouths))
	for i, m := range mouths {
		s.SetVec4(fmt.Sprintf("uMouthA[%d]", i), m.a)
		s.SetVec4(fmt.Sprintf("uMouthB[%d]", i), m.b)
	}
}

// mouthGround is ground, except at a tunnel mouth's cut-out, where it's
// no higher than the bed: the ground drawn there (the terrain's slope
// up to the hill isn't), for a road or path running into the tunnel.
func mouthGround(ground heightFunc, mouths []tunnelMouth) heightFunc {
	if len(mouths) == 0 {
		return ground
	}
	return func(x, z float32) float32 {
		y := ground(x, z)
		for _, m := range mouths {
			dir := mgl32.Vec2{float32(math.Sin(float64(m.a[2]))), float32(math.Cos(float64(m.a[2])))}
			d := mgl32.Vec2{x - m.a[0], z - m.a[1]}
			u, v := d.Dot(dir), float32(math.Abs(float64(d[0]*dir[1]-d[1]*dir[0])))
			if u > -mouthFront && u <= m.b[3] && v < m.b[0] {
				y = min(y, m.a[3])
			}
		}
		return y
	}
}
