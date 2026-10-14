//go:build !demo

package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// The guns drawn along snowmaking runs (render/snowmaking.go). Not in the
// demo, which has no snowmaking.

// SnowGunSpacing is the metres between the guns drawn along a run with
// snowmaking.
const SnowGunSpacing = 40

// SnowGun is one of the guns along a snowmaking run: where it stands, at
// the run's edge, and its heading (radians, as Guest.Heading) across the
// run.
type SnowGun struct {
	Pos     mgl32.Vec2
	Heading float32
}

// SnowGuns is where the guns stand along run t, SnowGunSpacing apart on
// its right-hand edge going down, each blowing across it; nil without
// snowmaking.
func (t *Trail) SnowGuns() []SnowGun {
	if !t.Snowmaking || t.Kind.IsArea() {
		return nil
	}
	line := t.Centerline()
	var out []SnowGun
	next := float32(SnowGunSpacing / 2)
	var along float32
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
			pos := p.Add(right.Mul(w/2 + 1))
			in := right.Mul(-1)
			out = append(out, SnowGun{Pos: pos, Heading: float32(math.Atan2(float64(in[0]), float64(in[1])))})
			next += SnowGunSpacing
		}
		along += l
	}
	return out
}
