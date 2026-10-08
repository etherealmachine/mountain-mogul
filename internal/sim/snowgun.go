package sim

import "mountain-mogul/internal/world"

// snowGunSWEPerCellPerHour is machine snow per covered cell per clock hour
// of running: 3 mm SWE ≈ 7 mm of dense machine snow, so a cold 12-hour
// night lays ~9 cm.
const snowGunSWEPerCellPerHour = 0.003

// tickSnowGuns applies artificial snow (KindBase) to terrain cells within each
// enabled snow gun's range whenever the air is cold enough.
func (s *Simulation) tickSnowGuns(dt float64) {
	if s.TempNow() > world.SnowGunMinTempC {
		return
	}
	const snowGunSWEPerCellPerSec = snowGunSWEPerCellPerHour / simSecondsPerHour
	t := s.World.Terrain
	swePerCell := float32(snowGunSWEPerCellPerSec * dt)
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
			}
		}
		r := world.SnowGunRangeCells
		t.MarkSnowDirtyRect(cx-r, cz-r, cx+r, cz+r)
	}
}
