package scene

import (
	"mountain-mogul/internal/world"
)

// The Snowmaking map view: every run with snowmaking by its depth against
// its target, from red (bare) through orange to blue (there), brighter
// where it's making snow now; and on runs without it, the cells thin
// enough for rock to show (world.ThinSnowDepth), in red.

// snowmakingOverlayKey changes when the view would: each clock hour, and
// when a run starts or stops making snow or gets snowmaking.
func (s *Scenario) snowmakingOverlayKey() int {
	key := int(s.sim.SimTime / 900)
	for _, t := range s.world.Trails {
		key *= 3
		if t.Snowmaking {
			key++
			if t.MakingSnow {
				key++
			}
		}
		key %= 1_000_000_007
	}
	return key + 2
}

// snowmakingOverlay paints the view with set.
func (s *Scenario) snowmakingOverlay(set func(cx, cz int, r, g, b, a uint8)) {
	t := s.world.Terrain
	painted := map[[2]int]bool{}
	for _, tr := range s.world.Trails {
		if !tr.Snowmaking {
			continue
		}
		for _, c := range tr.Cells {
			if !t.InBounds(c[0], c[1]) {
				continue
			}
			painted[c] = true
			f := min(t.Cells[c[0]][c[1]].VisibleSnowDepth()/max(tr.SnowTarget, 0.01), 1)
			var r, g, b float32
			if f < 0.5 { // red to orange
				r, g, b = 0.90, 0.20+0.70*f, 0.15
			} else { // orange to blue
				k := (f - 0.5) * 2
				r, g, b = 0.90-0.55*k, 0.55+0.15*k, 0.15+0.85*k
			}
			a := float32(0.45)
			if tr.MakingSnow {
				r, g, b, a = r+(1-r)*0.4, g+(1-g)*0.4, b+(1-b)*0.4, 0.7
			}
			set(c[0], c[1], uint8(r*255), uint8(g*255), uint8(b*255), uint8(a*255))
		}
	}
	for _, tr := range s.world.Trails {
		for _, c := range tr.Cells {
			if painted[c] || !t.InBounds(c[0], c[1]) {
				continue
			}
			if d := t.Cells[c[0]][c[1]].VisibleSnowDepth(); d < world.ThinSnowDepth {
				set(c[0], c[1], 230, 60, 40, uint8(90+100*(1-d/world.ThinSnowDepth)))
			}
		}
	}
}
