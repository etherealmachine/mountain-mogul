package scene

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/world"
)

// fallHeatSpread is the blur over fall counts, in cells either side, so a
// single fall reads as a spot and a cluster as one hot patch.
var fallHeatSpread = [5]float32{1, 4, 6, 4, 1}

// fallHeatFloor is the blurred count that shows as fully hot when no cell
// has more: about five falls on one cell. A day with a handful of falls
// stays faint rather than scaling them up to look like a hot spot.
const fallHeatFloor = 5 * (6.0 / 16) * (6.0 / 16)

// cellOverlayContent identifies what the cell overlay should show, so
// it's uploaded only when that changes: 0 when it can't tell (the land
// hover or a lodge's floor plan, which follow the mouse), 1 when it shows
// nothing, else 2 plus the number of falls the falls overlay shows.
func (s *Scenario) cellOverlayContent() int {
	if s.hoverParcel != nil || s.editedLodge() != nil {
		return 0
	}
	if s.overlayPanel == nil || s.overlayPanel.Mask()&render.OverlayFalls == 0 {
		return 1
	}
	return 2 + len(s.world.History.FallsToday)
}

// fallHeatPixels is the falls overlay as cell-overlay pixels: today's
// falls per cell, blurred, from clear through yellow to red.
func (s *Scenario) fallHeatPixels() []uint8 {
	falls := s.world.History.FallsToday
	tw, th := s.world.Terrain.Width, s.world.Terrain.Height
	if s.fallHeat != nil && len(s.fallHeat) == tw*th*4 && s.fallHeatN == len(falls) {
		return s.fallHeat
	}
	s.fallHeatN = len(falls)
	s.fallHeat = make([]uint8, tw*th*4)
	if len(falls) == 0 {
		return s.fallHeat
	}

	count := make([]float32, tw*th)
	for _, f := range falls {
		cx, cz := int(f.X/world.CellSize), int(f.Z/world.CellSize)
		if cx >= 0 && cx < tw && cz >= 0 && cz < th {
			count[cz*tw+cx]++
		}
	}
	// Separable blur: along x into tmp, then along z back into count.
	tmp := make([]float32, tw*th)
	for z := range th {
		for x := range tw {
			var v float32
			for k, wt := range fallHeatSpread {
				if xx := x + k - 2; xx >= 0 && xx < tw {
					v += wt * count[z*tw+xx]
				}
			}
			tmp[z*tw+x] = v / 16
		}
	}
	top := float32(fallHeatFloor)
	for z := range th {
		for x := range tw {
			var v float32
			for k, wt := range fallHeatSpread {
				if zz := z + k - 2; zz >= 0 && zz < th {
					v += wt * tmp[zz*tw+x]
				}
			}
			count[z*tw+x] = v / 16
			top = max(top, v/16)
		}
	}

	for i, v := range count {
		if v < 0.005 {
			continue
		}
		h := min(v/top, 1)
		// Yellow (one fall) to orange to red (the day's worst).
		var c mgl32.Vec3
		if h < 0.5 {
			c = mgl32.Vec3{1, 0.90, 0.25}.Mul(1 - h*2).Add(mgl32.Vec3{1, 0.55, 0.05}.Mul(h * 2))
		} else {
			c = mgl32.Vec3{1, 0.55, 0.05}.Mul(2 - h*2).Add(mgl32.Vec3{0.85, 0.08, 0.08}.Mul(h*2 - 1))
		}
		p := s.fallHeat[i*4:]
		p[0], p[1], p[2] = uint8(c[0]*255), uint8(c[1]*255), uint8(c[2]*255)
		p[3] = uint8((0.35 + 0.5*h) * 255)
	}
	return s.fallHeat
}

// fallenMarkers pins a marker over every guest who is down, at a fixed
// size on screen so it can be spotted from across the mountain: red for
// a fall they'll get up from, magenta for someone waiting for patrol.
type fallenMarkers struct {
	world *world.World
}

func (m *fallenMarkers) Draw(r *render.Renderer) {
	for _, a := range m.world.OnMountain {
		if !a.Injured && (!a.Fallen || a.Tumble.Phase == world.FallCollecting) {
			continue // up, or back on their feet fetching skis
		}
		if a.OnPatrollerID != 0 || a.ID == r.HiddenGuestID {
			continue // patrol has them, or it's the first-person camera's own
		}
		sx, sy, ok := r.WorldToScreen(a.Pos)
		if !ok {
			continue
		}
		col := mgl32.Vec4{0.90, 0.12, 0.10, 1}
		if a.Injured {
			col = mgl32.Vec4{0.95, 0.15, 0.80, 1}
		}
		const lift, radius = float32(26), float32(10)
		cy := sy - lift
		r.DrawColorLine(sx, sy, sx, cy, 2, mgl32.Vec4{0, 0, 0, 0.6})
		r.DrawColorDisc(sx, cy, radius+1.5, mgl32.Vec4{1, 1, 1, 0.95})
		r.DrawColorDisc(sx, cy, radius, col)
		if r.Font != nil {
			tw := r.Font.TextWidth("!")
			r.Font.DrawText(r, "!", sx-tw/2, cy-float32(render.GlyphH)/2, mgl32.Vec4{1, 1, 1, 1})
		}
	}
}
