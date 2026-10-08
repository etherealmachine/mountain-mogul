package world

import "math"

// TrunkHazardRadius is how far a trunk's danger reaches for steering: a
// skier's line scores 1 − (d/r)² at distance d from the nearest trunk.
const TrunkHazardRadius = 3.0

// trunkField caches, for each 1 m pixel, where the nearest trunk within
// trunkFieldReach of its centre stands, as an offset from the centre.
// Steering samples the trunk score hundreds of times per skier per step,
// where walking the trees near each sample point was a third of the
// sim's time; with the field a sample reads four pixels and measures the
// true distance to those pixels' nearest trunks, so the score is exact
// whenever the point's own nearest trunk is one of them (nearly always).
// Tree edits mark the cells they touch (setCellTrees); RefreshTrunkField
// redraws just those before the next sim step.
type trunkField struct {
	w, h  int    // pixels: one per metre
	off   []int8 // per pixel: nearest trunk's (dx, dz) from its centre, in trunkFieldUnit; trunkNone when none
	built bool
	// Changed cells since the last refresh, as an inclusive cell box.
	dirty    bool
	dx0, dz0 int
	dx1, dz1 int
}

// trunkFieldPxPerCell is the field's resolution: 1 m pixels.
const trunkFieldPxPerCell = int(CellSize)

// trunkFieldUnit is the offset step: 1/32 m, so ±127 units reach just
// under 4 m.
const trunkFieldUnit = 1.0 / 32

// trunkFieldReach is how far from a pixel centre its nearest trunk is
// kept. A sample point is within 0.71 m of each of its four pixel
// centres, so any trunk within TrunkHazardRadius of the point is within
// reach of them. It must stay under a cell (drawTrunks' margin).
const trunkFieldReach = 127 * trunkFieldUnit

// trunkNone marks a pixel with no trunk within reach.
const trunkNone = math.MinInt8

// markTrunksDirty records that cell (x, z)'s trees changed.
func (t *Terrain) markTrunksDirty(x, z int) {
	f := &t.trunks
	if !f.built {
		return // the first refresh builds everything
	}
	if !f.dirty {
		f.dirty, f.dx0, f.dz0, f.dx1, f.dz1 = true, x, z, x, z
		return
	}
	f.dx0, f.dz0 = min(f.dx0, x), min(f.dz0, z)
	f.dx1, f.dz1 = max(f.dx1, x), max(f.dz1, z)
}

// RefreshTrunkField brings the trunk field up to date with the trees:
// all of it the first time, then only around cells whose trees changed.
// Not safe to run alongside TrunkHazardAt; the sim calls it once per
// tick before any skier steers.
func (t *Terrain) RefreshTrunkField() {
	f := &t.trunks
	if !f.built || f.w != t.Width*trunkFieldPxPerCell || f.h != t.Height*trunkFieldPxPerCell {
		f.w, f.h = t.Width*trunkFieldPxPerCell, t.Height*trunkFieldPxPerCell
		f.off = make([]int8, 2*f.w*f.h)
		f.built, f.dirty = true, false
		t.drawTrunks(0, 0, t.Width-1, t.Height-1)
		return
	}
	if !f.dirty {
		return
	}
	f.dirty = false
	t.drawTrunks(f.dx0, f.dz0, f.dx1, f.dz1)
}

// drawTrunks redraws the field for cells whose trees changed,
// [x0, x1] × [z0, z1]. A trunk reaches under one cell (4 m < 5 m), so
// the pixels that can change are those of the cells one further out, and
// the trunks that can reach them stand at most one cell further still.
func (t *Terrain) drawTrunks(x0, z0, x1, z1 int) {
	f := &t.trunks
	const ppc = trunkFieldPxPerCell
	cx0, cz0 := max(x0-1, 0), max(z0-1, 0)
	cx1, cz1 := min(x1+1, t.Width-1), min(z1+1, t.Height-1)
	px0, pz0, px1, pz1 := cx0*ppc, cz0*ppc, (cx1+1)*ppc, (cz1+1)*ppc
	for pz := pz0; pz < pz1; pz++ {
		row := f.off[2*(pz*f.w+px0) : 2*(pz*f.w+px1)]
		for i := range row {
			row[i] = trunkNone
		}
	}
	const r = trunkFieldReach
	const r2 = r * r
	for x := max(cx0-1, 0); x <= min(cx1+1, t.Width-1); x++ {
		for z := max(cz0-1, 0); z <= min(cz1+1, t.Height-1); z++ {
			for _, tr := range t.trees[x*t.Height+z] {
				// Pixel centres within reach of the trunk, inside the box.
				ax0 := max(int(math.Floor(float64(tr.X-r))), px0)
				ax1 := min(int(math.Ceil(float64(tr.X+r))), px1-1)
				az0 := max(int(math.Floor(float64(tr.Z-r))), pz0)
				az1 := min(int(math.Ceil(float64(tr.Z+r))), pz1-1)
				for pz := az0; pz <= az1; pz++ {
					dz := tr.Z - (float32(pz) + 0.5)
					for px := ax0; px <= ax1; px++ {
						dx := tr.X - (float32(px) + 0.5)
						d2 := dx*dx + dz*dz
						if d2 >= r2 {
							continue
						}
						i := 2 * (pz*f.w + px)
						if f.off[i] != trunkNone {
							ox, oz := float32(f.off[i])*trunkFieldUnit, float32(f.off[i+1])*trunkFieldUnit
							if ox*ox+oz*oz <= d2 {
								continue
							}
						}
						f.off[i] = int8(math.Round(float64(dx / trunkFieldUnit)))
						f.off[i+1] = int8(math.Round(float64(dz / trunkFieldUnit)))
					}
				}
			}
		}
	}
}

// TrunkHazardAt is the trunk score at world (wx, wz), 0..1: 1 − (d/r)²
// for the nearest trunk (NearestTrunk2).
func (t *Terrain) TrunkHazardAt(wx, wz float32) float32 {
	const r2 = TrunkHazardRadius * TrunkHazardRadius
	return max(0, 1-t.NearestTrunk2(wx, wz)/r2)
}

// TrunkClear reports whether no trunk stands within r metres of world
// (wx, wz); r is at most TrunkHazardRadius.
func (t *Terrain) TrunkClear(wx, wz, r float32) bool {
	return t.NearestTrunk2(wx, wz) >= r*r
}

// NearestTrunk2 is the squared distance from world (wx, wz) to the
// nearest trunk, capped at TrunkHazardRadius²: the nearest among those
// nearest the four surrounding pixel centres of the trunk field. Before
// the field is first built (tests, or anything run before the first sim
// tick), it walks the nearby trees instead.
func (t *Terrain) NearestTrunk2(wx, wz float32) float32 {
	const r2 = TrunkHazardRadius * TrunkHazardRadius
	f := &t.trunks
	best := float32(r2)
	if !f.built {
		t.ForEachTreeNear(wx, wz, TrunkHazardRadius, func(tr Tree, _, _ int) {
			dx, dz := tr.X-wx, tr.Z-wz
			best = min(best, dx*dx+dz*dz)
		})
		return best
	}
	if wx < 0 || wz < 0 || wx >= float32(f.w) || wz >= float32(f.h) || f.w < 2 || f.h < 2 {
		return best
	}
	// The four pixels whose centres (at +0.5 m) surround the point.
	ix := min(max(int(wx-0.5), 0), f.w-2)
	iz := min(max(int(wz-0.5), 0), f.h-2)
	for _, p := range [4][2]int{{ix, iz}, {ix + 1, iz}, {ix, iz + 1}, {ix + 1, iz + 1}} {
		i := 2 * (p[1]*f.w + p[0])
		if f.off[i] == trunkNone {
			continue
		}
		dx := float32(p[0]) + 0.5 + float32(f.off[i])*trunkFieldUnit - wx
		dz := float32(p[1]) + 0.5 + float32(f.off[i+1])*trunkFieldUnit - wz
		best = min(best, dx*dx+dz*dz)
	}
	return best
}
