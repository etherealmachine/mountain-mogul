package sim

import "mountain-mogul/internal/world"

const (
	snowGunSWEPerCellPerSec = 0.002 / 3600.0 // m SWE/cell/sim-sec ≈ 4.8 mm/24 h per cell
)

// tickSnowGuns applies artificial snow (KindBase) to terrain cells within each
// enabled snow gun's range, gated by ambient temperature.
func (s *Simulation) tickSnowGuns(dt float64) {
	if s.Weather.Today().TempLow > world.SnowGunMinTempC {
		return
	}
	t := s.World.Terrain
	swePerCell := float32(snowGunSWEPerCellPerSec * dt)
	modified := false
	for _, b := range s.World.Buildings {
		if b.Type != world.BuildingSnowGun || !b.SnowGunEnabled {
			continue
		}
		cx := int(b.Pos[0] / world.CellSize)
		cz := int(b.Pos[1] / world.CellSize)
		for dz := -world.SnowGunRangeCells; dz <= world.SnowGunRangeCells; dz++ {
			for dx := -world.SnowGunRangeCells; dx <= world.SnowGunRangeCells; dx++ {
				if dx*dx+dz*dz > world.SnowGunRangeCells*world.SnowGunRangeCells {
					continue
				}
				nx, nz := cx+dx, cz+dz
				if !t.InBounds(nx, nz) {
					continue
				}
				cell := &t.Cells[nx][nz]
				top := cell.TopLayer()
				if top != nil && top.Kind == world.KindBase {
					top.Accumulation += swePerCell
				} else {
					cell.Layers = append(cell.Layers, world.SnowLayer{
						Accumulation: swePerCell,
						Kind:         world.KindBase,
					})
					if len(cell.Layers) > maxLayerStack {
						cell.Layers[0].Accumulation += cell.Layers[1].Accumulation
						cell.Layers = append(cell.Layers[:1], cell.Layers[2:]...)
					}
				}
				modified = true
			}
		}
	}
	if modified {
		t.SnowDirty = true
	}
}
