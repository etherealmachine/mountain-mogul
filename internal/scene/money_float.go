package scene

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/render"
	"mountain-mogul/internal/settings"
)

// moneyFloat is an amount the player just spent (negative) or earned,
// drawn rising from where they clicked and fading away.
type moneyFloat struct {
	x, y   float32
	amount int
	born   float32 // Scenario.time when it appeared
}

const (
	moneyFloatLife = float32(1.6) // seconds on screen
	moneyFloatRise = float32(42)  // pixels it rises over its life
	// moneyFloatPool is how soon after the last, seconds, more money the
	// same way adds to it: a glade stroke or a drag of paint counts up
	// one figure under the cursor, which rises and fades once it stops.
	moneyFloatPool = float32(0.35)
)

// addMoneyFloat shows amount at (x, y), or adds it to the float still
// collecting there.
func (s *Scenario) addMoneyFloat(x, y float32, amount int) {
	if n := len(s.moneyFloats); n > 0 {
		f := &s.moneyFloats[n-1]
		if s.time-f.born < moneyFloatPool && (f.amount < 0) == (amount < 0) {
			f.amount += amount
			f.x, f.y, f.born = x, y, s.time
			return
		}
	}
	s.moneyFloats = append(s.moneyFloats, moneyFloat{x: x, y: y, amount: amount, born: s.time})
}

// drawMoneyFloats draws the live money floats and drops the finished ones.
func (s *Scenario) drawMoneyFloats(r *render.Renderer) {
	live := s.moneyFloats[:0]
	for _, f := range s.moneyFloats {
		age := s.time - f.born
		if age >= moneyFloatLife {
			continue
		}
		live = append(live, f)
		if r.Font == nil {
			continue
		}
		k := age / moneyFloatLife
		alpha := min(1, 3*(1-k)) // holds, then fades over the last third
		text := settings.FormatDollars(f.amount)
		col := mgl32.Vec4{1.00, 0.45, 0.40, alpha}
		if f.amount > 0 {
			text = "+" + text
			col = mgl32.Vec4{0.45, 0.95, 0.50, alpha}
		}
		x := f.x - r.Font.TextWidth(text)/2
		y := f.y + 14 - moneyFloatRise*k*(2-k) // eases out as it rises
		r.Font.DrawText(r, text, x+1.5, y+1.5, mgl32.Vec4{0, 0, 0, alpha * 0.6})
		r.Font.DrawText(r, text, x, y, col)
	}
	s.moneyFloats = live
}
