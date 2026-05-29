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
				// Snow guns deposit KindBase snow on the surface.
				if cell.Top.Accumulation > 0 && cell.Top.Kind == world.KindBase {
					cell.Top.Accumulation += swePerCell
				} else {
					cell.Base += cell.Top.Accumulation
					cell.Top = world.SnowLayer{Kind: world.KindBase, Accumulation: swePerCell}
				}
				modified = true
			}
		}
	}
	if modified {
		t.SnowDirty = true
	}
}
