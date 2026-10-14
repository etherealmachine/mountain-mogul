package sim

import (
	"mountain-mogul/internal/world"
)

// tickSnowmaking makes snow on every run with snowmaking switched on
// (world/snowmaking.go): on each of its cells below the run's target
// depth, at a rate set by the air there (the base's temperature lapsed to
// the cell's height), while the lifts are stopped and up to
// SnowmakingStopBeforeOpen hours before they open. It charges the water
// made into snow to World.SnowmakingToday.
func (s *Simulation) tickSnowmaking(dt float64) {
	w := s.World
	stopped := !s.LiftsRunning()
	if w.ResortOpen {
		h := float32(HourOfDay(s.SimTime))
		if h >= w.OpenHour-world.SnowmakingStopBeforeOpen && h < w.OpenHour {
			stopped = false // the cats groom over it before the lifts open
		}
	}
	base := s.TempNow()
	t := w.Terrain
	baseElev := s.snowmakingBaseElev()
	hours := float32(dt / simSecondsPerHour)
	cellArea := float32(world.CellSize * world.CellSize)
	for _, tr := range w.Trails {
		tr.MakingSnow = false
		if !tr.Snowmaking || !tr.SnowmakingOn || !stopped {
			continue
		}
		var water float32
		for _, c := range tr.Cells {
			if !t.InBounds(c[0], c[1]) {
				continue
			}
			cell := &t.Cells[c[0]][c[1]]
			if cell.VisibleSnowDepth() >= tr.SnowTarget {
				continue
			}
			temp := base - world.LapseRate*(cell.GroundElevation-baseElev)
			rate := world.SnowmakingRate(temp)
			if rate <= 0 {
				continue
			}
			swe := world.SnowmakingSWEPerHour * rate * hours
			world.AddMachineSnow(cell, swe)
			t.MarkSnowDirty(c[0], c[1])
			water += swe * cellArea
		}
		if water > 0 {
			tr.MakingSnow = true
			w.SnowmakingToday += float64(water) * world.SnowmakingWaterCost
		}
	}
}

// snowmakingBaseElev is the base area's height, where TempNow is read:
// the lowest ground, found once.
func (s *Simulation) snowmakingBaseElev() float32 {
	if !s.baseElevKnown {
		s.baseElev = terrainMinElevation(s.World.Terrain)
		s.baseElevKnown = true
	}
	return s.baseElev
}
