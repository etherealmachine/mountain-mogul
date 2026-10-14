package sim

import (
	"fmt"
	"math"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// Avalanches by slide path (world/slide_paths.go;
// notes/next/Avalanche Control.md). Each day's storm loads a path's start
// zone, more on a lee slope, and so does a dry day's wind; the load
// settles away day by day. Every clock hour each path's danger is worked
// out from its load (afternoon warming on a sunny slope adds to it), and
// a path may release on its own, more likely the higher the danger, so a
// storm or a warm afternoon can bring one down with guests out.

const (
	// settleCold and settleWarm are the share of a path's load left after
	// a day: snow bonds as it settles, faster when it's warm.
	settleCold = float32(0.75)
	settleWarm = float32(0.6)
	// windLoadMS is a dry day's wind at the base, m/s, from which it
	// moves snow onto lee slopes, windLoadSWE a day's load from it per
	// 5 m/s more.
	windLoadMS  = float32(7.5)
	windLoadSWE = float32(0.006)
	// initialLoadShare is the share of the snow lying loose on a start
	// zone (powder, wind slab) taken as its load when paths are first
	// found.
	initialLoadShare = float32(0.15)
	// releaseKeep is the load left on a path after it slides.
	releaseKeep = float32(0.15)
	// avyCaughtInjury is the chance someone caught is badly hurt.
	avyCaughtInjury = float32(0.70)
)

// releasePerHour is a path's chance of releasing on its own in a clock
// hour, by danger.
var releasePerHour = [world.DangerLevels]float32{0, 0.0005, 0.003, 0.02, 0.08}

// ensureSlidePaths finds the slide paths if they haven't been, taking each
// one's load from the loose snow lying on its start zone.
func (s *Simulation) ensureSlidePaths() {
	w := s.World
	if w.SlidePaths != nil || w.Terrain == nil {
		return
	}
	w.SlidePaths = world.BuildSlidePaths(w.Terrain, w.Trails)
	t := w.Terrain
	for _, p := range w.SlidePaths {
		var loose float32
		for _, c := range p.Start {
			if top := t.Cells[c[0]][c[1]].Top; top.Kind == world.KindPowder || top.Kind == world.KindWindSlab {
				loose += top.Accumulation
			}
		}
		p.Load = loose / float32(len(p.Start)) * initialLoadShare
	}
	s.updateAvalancheDanger()
}

// RebuildSlidePaths finds the paths again (after the ground or the runs
// change), each new one keeping the load of the old one nearest it.
func (s *Simulation) RebuildSlidePaths() {
	old := s.World.SlidePaths
	s.World.SlidePaths = nil
	s.ensureSlidePaths()
	for _, p := range s.World.SlidePaths {
		best, bestD := float32(-1), float32(30*30)
		for _, o := range old {
			dx, dz := o.CentreX-p.CentreX, o.CentreZ-p.CentreZ
			if d := dx*dx + dz*dz; d < bestD {
				best, bestD = o.Load, d
			}
		}
		if best >= 0 {
			p.Load = best
		}
	}
	s.updateAvalancheDanger()
}

// loadSlidePaths is a day's weather on the paths: yesterday's load
// settles, then the day's snowfall loads them, lee slopes more in the
// wind, and a dry windy day carries snow onto the lee ones.
func (s *Simulation) loadSlidePaths(dw DayWeather) {
	s.ensureSlidePaths()
	keep := settleCold
	if dw.TempHigh > 2 {
		keep = settleWarm
	}
	windTo := float64(dw.WindDeg) * math.Pi / 180
	for _, p := range s.World.SlidePaths {
		p.Load *= keep
		// How square the slope faces the way the wind blows (a lee
		// slope faces downwind).
		lee := max(float32(math.Cos(float64(p.Aspect)-windTo)), 0)
		if dw.AccumSWE > 0 {
			p.Load += dw.AccumSWE * (1 + 0.8*lee*min(dw.WindMS/8, 1.5))
		} else if dw.WindMS > windLoadMS {
			p.Load += windLoadSWE * lee * (dw.WindMS - windLoadMS) / 5
		}
		if dw.IsRaining() {
			p.Load *= 1.3 // rain on new snow: wet slides
		}
	}
}

// updateAvalancheDanger works out each path's danger now, and shows it on
// the map view.
func (s *Simulation) updateAvalancheDanger() {
	w := s.World
	t := w.Terrain
	base := s.TempNow()
	baseElev := s.snowmakingBaseElev()
	sun := s.Site.SunAt(s.DateAt(s.SimTime), HourOfDay(s.SimTime))
	for _, p := range w.SlidePaths {
		load := p.Load
		// Warming: air over freezing at the start zone, more on a slope
		// facing the sun.
		cx, cz := int(p.CentreX/world.CellSize), int(p.CentreZ/world.CellSize)
		if t.InBounds(cx, cz) {
			temp := base - world.LapseRate*(t.Cells[cx][cz].GroundElevation-baseElev)
			if temp > 0 && sun.Dir[1] > 0 {
				facing := float32(math.Sin(float64(p.Aspect)))*sun.Dir[0] + float32(math.Cos(float64(p.Aspect)))*sun.Dir[2]
				load *= 1 + min(temp/6, 1)*(0.5+0.5*max(facing, 0))
			}
		}
		p.Danger, _ = world.DangerFor(load, p.Steep)
	}
	t.SetAvyDanger(w.SlidePaths)
}

// avalancheHour is the paths' clock hour: danger worked out afresh, and
// each path rolls whether it slides on its own.
func (s *Simulation) avalancheHour() {
	s.ensureSlidePaths()
	s.refreshPathTrails()
	s.updateAvalancheDanger()
	defer s.updateAvyClosures()
	for _, p := range s.World.SlidePaths {
		if rng.Global().Float32() < releasePerHour[p.Danger] {
			if len(p.Trails) > 0 {
				s.releasePath(p, false) // across a run: its own line in the feed
				continue
			}
			if s.releasePathQuietly(p) {
				if s.quietSlides == 0 {
					s.quietSlideAt = [2]int{int(p.CentreX / world.CellSize), int(p.CentreZ / world.CellSize)}
				}
				s.quietSlides++
			}
		}
	}
	// Slides away from the runs go in the feed together, every
	// quietSlideHours.
	if s.quietSlides > 0 && int(HourOfDay(s.SimTime))%quietSlideHours == 0 {
		s.logAvalanches(s.quietSlides, s.quietSlideAt)
		s.quietSlides = 0
	}
}

// quietSlideHours is how often slides away from the runs are logged.
const quietSlideHours = 4

// refreshPathTrails works out again which runs each slide path crosses,
// when the runs have changed.
func (s *Simulation) refreshPathTrails() {
	key := 0
	for _, tr := range s.World.Trails {
		key = key*31 + int(tr.ID) + len(tr.Cells)*7
	}
	if key == s.pathTrailsKey {
		return
	}
	s.pathTrailsKey = key
	at := map[[2]int][]uint64{}
	for _, tr := range s.World.Trails {
		for _, c := range tr.Cells {
			at[c] = append(at[c], tr.ID)
		}
	}
	for _, p := range s.World.SlidePaths {
		p.Trails = p.Trails[:0]
		seen := map[uint64]bool{}
		for _, c := range p.Cells {
			for _, id := range at[c] {
				if !seen[id] {
					seen[id] = true
					p.Trails = append(p.Trails, id)
				}
			}
		}
	}
}

// releasePathQuietly is releasePath for a path away from the runs, logged
// with the hour's others; false when there was no snow to slide.
func (s *Simulation) releasePathQuietly(p *world.SlidePath) bool {
	t := s.World.Terrain
	started := 0
	for _, c := range p.Start {
		if s.startAvalanche(t, c[0], c[1]) {
			started++
		}
	}
	p.Load *= releaseKeep
	return started > 0
}

// releasePath sets path p sliding: the loose snow on its start zone runs
// down it (startAvalanche). controlled is control work's release.
func (s *Simulation) releasePath(p *world.SlidePath, controlled bool) {
	t := s.World.Terrain
	started := 0
	for _, c := range p.Start {
		if s.startAvalanche(t, c[0], c[1]) {
			started++
		}
	}
	p.Load *= releaseKeep
	if started == 0 {
		return
	}
	s.updateAvalancheDanger()
	msg := fmt.Sprintf("Avalanche: a %s slide path ran", sizeWord(len(p.Start)))
	if controlled {
		msg = fmt.Sprintf("Control work brought down a %s slide path", sizeWord(len(p.Start)))
	}
	if n := len(p.Trails); n > 0 {
		names := ""
		for i, id := range p.Trails {
			if tr := s.World.FindTrail(id); tr != nil {
				if i > 0 {
					names += ", "
				}
				names += tr.Name
			}
			if i == 2 {
				break
			}
		}
		msg += " across " + names
	}
	s.World.LogEventAt(world.EventAvalanche, s.SimTime, msg, cellCentre([2]int{int(p.CentreX / world.CellSize), int(p.CentreZ / world.CellSize)}), 0)
}

// sizeWord is a slide path's size in a word, by its start zone's cells.
func sizeWord(cells int) string {
	switch {
	case cells < 20:
		return "small"
	case cells < 120:
		return "sizeable"
	}
	return "big"
}

// catchInAvalanche catches anyone on foot or on skis (not on a lift) where
// the snow is running now: knocked down, and usually badly hurt.
func (s *Simulation) catchInAvalanche() {
	t := s.World.Terrain
	for _, a := range s.World.OnMountain {
		if a.OnLiftID != 0 || a.Fallen || a.Injured {
			continue
		}
		x, z := int(a.Pos[0]/CellSize), int(a.Pos[2]/CellSize)
		if !t.InBounds(x, z) || t.Cells[x][z].AvySnow <= avyMinSnow {
			continue
		}
		if a.Queued {
			for _, l := range s.World.Lifts {
				l.LeaveLine(a)
			}
			a.Queued = false
		}
		s.applyEvent(a, ai.ThoughtCaughtInAvalanche)
		s.knockDown(a, world.FallForward)
		if rng.Global().Float32() < avyCaughtInjury {
			s.injure(a, true)
		} else {
			s.applyTrailEvent(a, ai.ThoughtFell)
		}
	}
}

// updateAvyClosures closes, with avalanche control, the runs under a
// slide path at or over the resort's limit, and opens those no longer
// under one, saying which in the feed.
func (s *Simulation) updateAvyClosures() {
	w := s.World
	control := w.AvyControlled()
	closed := map[uint64]bool{}
	if control {
		for _, p := range w.SlidePaths {
			if p.Danger >= w.AvyLimit {
				for _, id := range p.Trails {
					closed[id] = true
				}
			}
		}
	}
	var shut, opened []string
	for _, t := range w.Trails {
		if closed[t.ID] == t.AvyClosed {
			continue
		}
		t.AvyClosed = closed[t.ID]
		name := t.Name
		if name == "" {
			name = "a run"
		}
		if t.AvyClosed {
			shut = append(shut, name)
		} else {
			opened = append(opened, name)
		}
	}
	if len(shut)+len(opened) > 0 {
		w.RebuildTrailGraph()
		w.SetAvyClosedCells()
	}
	// A lift with nothing open off its top stops with its runs.
	for _, l := range w.Lifts {
		any, open := false, false
		if w.TrailGraph != nil {
			for _, e := range w.TrailGraph.EdgesFrom(l.ID) {
				if e.FromKind != world.KindLiftTop {
					continue
				}
				if t := w.FindTrail(e.TrailID); t != nil {
					any = true
					open = open || !t.AvyClosed
				}
			}
		}
		was := l.AvyClosed
		l.AvyClosed = any && !open
		if l.AvyClosed && !was {
			ejectQueue(l)
			shut = append(shut, l.Name)
		} else if was && !l.AvyClosed {
			opened = append(opened, l.Name)
		}
	}
	if len(shut) > 0 {
		w.LogEvent(world.EventAvalanche, s.SimTime, "Closed for avalanche danger: "+listNames(shut))
	}
	if len(opened) > 0 {
		w.LogEvent(world.EventAvalanche, s.SimTime, "Reopened after avalanche danger: "+listNames(opened))
	}
}

// listNames joins names for the feed, at most four and how many more.
func listNames(names []string) string {
	out := ""
	for i, n := range names {
		if i == 4 {
			return out + fmt.Sprintf(" and %d more", len(names)-4)
		}
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}
