//go:build !demo

package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Where night skiing's lights are (lights.go): poles along each lit run,
// and lamps on each lit lift's towers and stations. The renderer draws
// them and bakes their light into a map over the snow
// (render/night_lights.go). Not in the demo.

// LightPoleSpacing is the metres between poles along a lit run, which
// alternate sides.
const LightPoleSpacing = 30

// LightSpot is a light: where it stands, its heading (toward what it
// lights, as Guest.Heading), how high its lamps are over the ground, and
// whether it's a pole (else a lamp on a lift's tower or station).
type LightSpot struct {
	Pos     mgl32.Vec2
	Heading float32
	Height  float32
	Pole    bool
}

// LightSpots is every light on the lit runs and lifts.
func (w *World) LightSpots(t *Terrain) []LightSpot {
	var out []LightSpot
	for _, tr := range w.Trails {
		if tr.Lit && !tr.Kind.IsArea() {
			out = append(out, runPoles(tr)...)
		}
	}
	for _, l := range w.Lifts {
		if !l.Lit || l.Cableless() {
			continue
		}
		dir, _ := l.lineAxes()
		h := float32(math.Atan2(float64(dir[0]), float64(dir[1])))
		out = append(out, LightSpot{Pos: l.LoadXZ(), Heading: h, Height: 6})
		out = append(out, LightSpot{Pos: l.UnloadXZ(), Heading: h, Height: 6})
		for _, tw := range l.Towers(t) {
			out = append(out, LightSpot{Pos: tw.XZ, Heading: h, Height: tw.Height})
		}
	}
	return out
}

// runPoles is the poles along run tr, LightPoleSpacing apart, alternating
// edges, each aimed across it.
func runPoles(tr *Trail) []LightSpot {
	line := tr.Centerline()
	var out []LightSpot
	next := float32(LightPoleSpacing / 2)
	var along float32
	side := float32(1)
	for i := 1; i < len(line); i++ {
		a, b := line[i-1], line[i]
		seg := b.Pos.Sub(a.Pos)
		l := seg.Len()
		if l < 1e-3 {
			continue
		}
		dir := seg.Mul(1 / l)
		right := mgl32.Vec2{-dir[1], dir[0]}
		for next <= along+l {
			f := (next - along) / l
			p := a.Pos.Add(seg.Mul(f))
			w := a.Width + (b.Width-a.Width)*f
			out_ := right.Mul(side)
			in := out_.Mul(-1)
			out = append(out, LightSpot{
				Pos:     p.Add(out_.Mul(w/2 + 1.5)),
				Heading: float32(math.Atan2(float64(in[0]), float64(in[1]))),
				Height:  10,
				Pole:    true,
			})
			side = -side
			next += LightPoleSpacing
		}
		along += l
	}
	return out
}
