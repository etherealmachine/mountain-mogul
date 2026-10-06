package world

import (
	"math"
	"slices"
)

// Snow shedding: steep ground can't hold much snow. Each cell holds at
// most SnowHoldSWE for its slope; the rest sluffs down the fall line,
// cell to cell, and settles where the slope eases, building aprons below
// cliffs and filling gullies. It's the small, constant version of an
// avalanche, run whenever snow lands.
//
// Rock in the material map holds only a dusting, whatever the cell's
// slope: its faces are steep at the 1.25 m scale even where the 5 m cell
// isn't. A cell's limit counts its rock and the rest of it separately,
// so snow comes off rock-heavy cells and what stays sits on the ledges
// and ground between (the renderer draws it there).
const (
	shedFrom    = 35.0  // degrees: up to here a cell holds whatever falls
	shedTo      = 60.0  // degrees: past here it holds almost nothing
	shedHoldMax = 3.0   // metres of SWE held at shedFrom, more than any season
	shedHoldMin = 0.005 // metres of SWE held at shedTo and steeper
	rockHoldSWE = 0.01  // metres of SWE rock holds: a dusting, about 3 cm
)

// SnowHoldSWE is the most snow, in metres of SWE, a cell with this slope
// (rise over run) keeps. It falls off steeply between shedFrom and
// shedTo: about 0.3 m at 45°, 5 cm at 50°, 1 cm at 55°.
func SnowHoldSWE(slope float32) float32 {
	deg := math.Atan(float64(slope)) * 180 / math.Pi
	if deg <= shedFrom {
		return float32(math.Inf(1))
	}
	t := min((deg-shedFrom)/(shedTo-shedFrom), 1)
	t = t * t * (3 - 2*t)
	return float32(math.Exp(math.Log(shedHoldMax) + t*(math.Log(shedHoldMin)-math.Log(shedHoldMax))))
}

// cellHold is the most SWE a cell keeps: the slope's limit over the part
// that isn't rock, a dusting over the rock.
func cellHold(slope, rockFrac float32) float32 {
	if rockFrac <= 0 {
		return SnowHoldSWE(slope)
	}
	h := SnowHoldSWE(slope)
	switch {
	case rockFrac >= 1:
		return rockHoldSWE
	case math.IsInf(float64(h), 1):
		return h // the ground and ledges between the rock hold the rest
	}
	return h*(1-rockFrac) + rockHoldSWE*rockFrac
}

// RockFractions is how much of each cell, indexed x*Height+z, is rock in
// the material map: its share of the detail samples on and inside the
// cell's edges. Nil without a material map.
func (t *Terrain) RockFractions() []float32 {
	m := t.Material
	if m == nil {
		return nil
	}
	W, H := t.Width, t.Height
	out := make([]float32, W*H)
	for x := 0; x < W; x++ {
		for z := 0; z < H; z++ {
			rock, n := 0, 0
			for j := z * DetailPerCell; j <= min((z+1)*DetailPerCell, m.H-1); j++ {
				for i := x * DetailPerCell; i <= min((x+1)*DetailPerCell, m.W-1); i++ {
					if m.M[j*m.W+i].IsRock() {
						rock++
					}
					n++
				}
			}
			out[x*H+z] = float32(rock) / float32(n)
		}
	}
	return out
}

// shedCache is ShedSnow's cell order, highest ground first, and each
// cell's rock fraction for the material map it was made from.
type shedCache struct {
	order []int32
	mat   *TerrainMaterial
	rock  []float32
}

// shedPrep returns the cache, rebuilding what the ground or material
// map has changed since.
func (t *Terrain) shedPrep() *shedCache {
	if t.shed == nil {
		order := make([]int32, t.Width*t.Height)
		for k := range order {
			order[k] = int32(k)
		}
		H := t.Height
		elev := func(k int32) float32 { return t.Cells[int(k)/H][int(k)%H].GroundElevation }
		slices.SortFunc(order, func(a, b int32) int {
			ea, eb := elev(a), elev(b)
			switch {
			case ea > eb:
				return -1
			case ea < eb:
				return 1
			}
			return 0
		})
		t.shed = &shedCache{order: order, mat: t.Material, rock: t.RockFractions()}
	}
	if t.shed.mat != t.Material {
		t.shed.mat, t.shed.rock = t.Material, t.RockFractions()
	}
	return t.shed
}

// ShedSnow moves the snow each cell can't hold down the fall line. Cells
// run from the highest down, so a cell has received everything from
// above before it sheds. Each passes its excess to its lower neighbours
// in proportion to how steeply the ground drops to each; the receivers
// add it to their base. A cell with no lower neighbour keeps it. Shed
// snow comes off the top layer first. Returns the SWE moved, in cubic
// metres of water per square metre of cell (summed over cells).
func (t *Terrain) ShedSnow() float32 {
	W, H := t.Width, t.Height
	n := W * H
	cache := t.shedPrep()
	order, rockFrac := cache.order, cache.rock
	incoming := make([]float32, n)
	var moved float32
	type lower struct {
		k int
		w float32
	}
	var lows [8]lower
	for _, k32 := range order {
		k := int(k32)
		x, z := k/H, k%H
		c := &t.Cells[x][z]
		c.Base += incoming[k]
		var rf float32
		if rockFrac != nil {
			rf = rockFrac[k]
		}
		excess := c.Base + c.Top.Accumulation - cellHold(c.Slope, rf)
		if excess <= 0 {
			continue
		}
		nl := 0
		var sum float32
		for dx := -1; dx <= 1; dx++ {
			for dz := -1; dz <= 1; dz++ {
				nx, nz := x+dx, z+dz
				if (dx == 0 && dz == 0) || nx < 0 || nz < 0 || nx >= W || nz >= H {
					continue
				}
				drop := c.GroundElevation - t.Cells[nx][nz].GroundElevation
				if drop <= 0 {
					continue
				}
				dist := float32(CellSize)
				if dx != 0 && dz != 0 {
					dist *= math.Sqrt2
				}
				lows[nl] = lower{nx*H + nz, drop / dist}
				sum += lows[nl].w
				nl++
			}
		}
		if nl == 0 {
			continue
		}
		// Off the top layer first, then the base.
		fromTop := min(excess, c.Top.Accumulation)
		c.Top.Accumulation -= fromTop
		c.Base -= excess - fromTop
		if c.Top.Accumulation <= 0 {
			c.Top = SnowLayer{}
		}
		for _, l := range lows[:nl] {
			incoming[l.k] += excess * l.w / sum
		}
		moved += excess
	}
	if moved > 0 {
		t.SnowDirty = true
	}
	return moved
}
