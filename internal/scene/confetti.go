package scene

import (
	"math"
	"math/rand/v2"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
)

// Winning a scenario sets off confetti: bright paper strips falling from
// above the screen, swaying and turning over, gone in a few seconds; the
// result panel drops in under it (goals_panel.go).

const (
	confettiPieces = 220
	confettiLife   = float32(6) // seconds until the last piece is gone
)

// confetti is one strip of paper.
type confetti struct {
	x, y0    float32 // start: across the screen, and how far above it
	fall     float32 // pixels a second
	sway     float32 // side-to-side, pixels
	phase    float32
	spin     float32 // turns a second
	col      mgl32.Vec4
	len, wid float32
}

var confettiColours = []mgl32.Vec4{
	{0.95, 0.30, 0.35, 1}, {1.00, 0.80, 0.25, 1}, {0.30, 0.80, 0.45, 1},
	{0.30, 0.60, 1.00, 1}, {0.75, 0.45, 0.95, 1}, {1, 1, 1, 1},
}

// celebrate starts the confetti.
func (s *Scenario) celebrate() {
	s.celebratedAt = s.time
	s.confetti = s.confetti[:0]
	sw, sh := float32(s.app.Renderer.ScreenWidth()), float32(s.app.Renderer.ScreenHeight())
	for range confettiPieces {
		s.confetti = append(s.confetti, confetti{
			x:     rand.Float32() * sw,
			y0:    -10 - rand.Float32()*sh*0.9, // staggered, so it keeps coming
			fall:  160 + rand.Float32()*160,
			sway:  8 + rand.Float32()*22,
			phase: rand.Float32() * 2 * math.Pi,
			spin:  0.5 + rand.Float32()*1.5,
			col:   confettiColours[rand.IntN(len(confettiColours))],
			len:   7 + rand.Float32()*7,
			wid:   3 + rand.Float32()*3,
		})
	}
}

// drawConfetti draws the falling confetti while it lasts.
func (s *Scenario) drawConfetti(r *render.Renderer) {
	if len(s.confetti) == 0 {
		return
	}
	age := s.time - s.celebratedAt
	if age > confettiLife {
		s.confetti = s.confetti[:0]
		return
	}
	fade := min(1, (confettiLife-age)/1.0) // the last second fades out
	for _, c := range s.confetti {
		y := c.y0 + c.fall*age
		x := c.x + c.sway*float32(math.Sin(float64(c.phase+age*2.2)))
		// Turning over: the strip's length foreshortens as it spins.
		a := float64(c.phase + age*c.spin*2*math.Pi)
		half := c.len / 2 * float32(0.25+0.75*math.Abs(math.Cos(a)))
		dx, dy := half*float32(math.Cos(a*0.7)), half*float32(math.Sin(a*0.7))
		col := c.col
		col[3] = fade
		r.DrawColorLine(x-dx, y-dy, x+dx, y+dy, c.wid, col)
	}
}
