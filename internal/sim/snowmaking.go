package sim

import (
	"fmt"

	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/world"

	"github.com/go-gl/mathgl/mgl32"
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
		// An unlit run makes snow through a night session too.
		if !tr.Snowmaking || !tr.SnowmakingOn || !(stopped || (w.Dark && !tr.Lit)) {
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
			cost := float64(water) * world.SnowmakingWaterCost
			w.SnowmakingToday += cost
			tr.SnowWater += water
			tr.SnowCost += cost
		}
	}
	if s.snowmakingStopped && !stopped {
		s.endSnowmakingNight()
	}
	s.snowmakingStopped = stopped
}

// endSnowmakingNight closes a night of snowmaking as the lifts are about
// to start: each run's depth laid and cost go to its popup, and the feed
// says what the night made.
func (s *Simulation) endSnowmakingNight() {
	w := s.World
	var runs int
	var cost float64
	var lo, hi float32
	var best *world.Trail
	cellArea := float32(world.CellSize * world.CellSize)
	for _, tr := range w.Trails {
		if tr.SnowWater <= 0 || len(tr.Cells) == 0 {
			continue
		}
		depth := tr.SnowWater / (float32(len(tr.Cells)) * cellArea) / world.KindDensity(world.KindMachine)
		tr.LastSnowDepth, tr.LastSnowCost = depth, float32(tr.SnowCost)
		if runs == 0 || depth < lo {
			lo = depth
		}
		if runs == 0 || depth > hi {
			hi, best = depth, tr
		}
		runs++
		cost += tr.SnowCost
		tr.SnowWater, tr.SnowCost = 0, 0
	}
	if runs == 0 {
		return
	}
	cm := func(m float32) int { return int(m*100 + 0.5) }
	var msg string
	if runs == 1 {
		msg = fmt.Sprintf("Snowmaking laid %d cm on %s overnight (%s)", cm(hi), best.Name, settings.FormatDollars(int(cost+0.5)))
	} else if cm(lo) == cm(hi) {
		msg = fmt.Sprintf("Snowmaking laid %d cm on %d runs overnight (%s)", cm(hi), runs, settings.FormatDollars(int(cost+0.5)))
	} else {
		msg = fmt.Sprintf("Snowmaking laid %d–%d cm on %d runs overnight (%s)", cm(lo), cm(hi), runs, settings.FormatDollars(int(cost+0.5)))
	}
	c := best.Cells[len(best.Cells)/2]
	at := mgl32.Vec2{(float32(c[0]) + 0.5) * world.CellSize, (float32(c[1]) + 0.5) * world.CellSize}
	w.LogEventAt(world.EventSnowfall, s.SimTime, msg, at, 0)
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
