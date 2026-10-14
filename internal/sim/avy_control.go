package sim

import (
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"

	"github.com/go-gl/mathgl/mgl32"
)

// Avalanche control work (notes/next/Avalanche Control.md): a patroller
// at an avalanche-control patrol room with nothing else to do takes the
// most dangerous slide path over the runs that needs control, once
// nobody's on it: within mortar range of the room they shell it from
// there; beyond it they go up and ski cut it. Either brings the slope
// down on purpose, and the runs below reopen if the danger's dropped.

const (
	// mortarShotMinutes and cutMinutes are clock minutes a round and a
	// ski cut take.
	mortarShotMinutes = 4.0
	cutMinutes        = 10.0
	// cutCaughtChance is a ski cut's chance of catching the patroller.
	cutCaughtChance = float32(0.03)
)

// clockMinutes is m clock minutes in sim seconds.
func clockMinutes(m float64) float64 { return m / 60 * simSecondsPerHour }

// startAvyWork sends idle patroller p out on control work, if a path
// needs it; reports whether it did.
func (s *Simulation) startAvyWork(p *world.Patroller, base *world.Building) bool {
	if !base.AvyControl {
		return false
	}
	path := s.pathNeedingControl(p.Pos)
	if path == nil {
		return false
	}
	p.AvyPathID = path.ID
	start := mgl32.Vec3{path.CentreX, s.World.Terrain.InterpolatedSurfaceElevationAt(path.CentreX, path.CentreZ), path.CentreZ}
	if (mgl32.Vec2{start[0] - p.Pos[0], start[2] - p.Pos[2]}).Len() <= world.MortarRange {
		p.State = world.PatrollerFiring
		p.TaskUntil = s.SimTime + clockMinutes(mortarShotMinutes)
		return true
	}
	// Out of range: up to the start zone on skis and hiking.
	secs := patrolTravelTime(p.Pos, start)
	p.ActionTimer = mgl32.Vec2{start[0] - p.Pos[0], start[2] - p.Pos[2]}.Len() / max(secs, 1)
	p.TargetPos = start
	p.OnSkis = true
	p.State = world.PatrollerToCut
	return true
}

// pathNeedingControl is the slide path over the runs most in need of
// control work that no patroller has taken and nobody's on: at or over
// the resort's limit any time, Moderate or worse before the lifts open;
// the most dangerous first, then the nearest from.
func (s *Simulation) pathNeedingControl(from mgl32.Vec3) *world.SlidePath {
	w := s.World
	h := float32(HourOfDay(s.SimTime))
	before := h < w.OpenHour
	taken := map[int]bool{}
	for _, q := range w.Patrollers {
		if q.AvyPathID != 0 {
			taken[q.AvyPathID] = true
		}
	}
	var best *world.SlidePath
	var bestD float32
	for _, p := range w.SlidePaths {
		if len(p.Trails) == 0 || taken[p.ID] || p.Danger < world.DangerModerate {
			continue // nothing to bring down on a Low path
		}
		if p.Danger < w.AvyLimit && !(before && p.Danger >= world.DangerModerate) {
			continue
		}
		if !s.pathClear(p) {
			continue
		}
		d := mgl32.Vec2{p.CentreX - from[0], p.CentreZ - from[2]}.Len()
		if best == nil || p.Danger > best.Danger || (p.Danger == best.Danger && d < bestD) {
			best, bestD = p, d
		}
	}
	return best
}

// pathClear reports whether nobody but patrol is on slide path p.
func (s *Simulation) pathClear(p *world.SlidePath) bool {
	cells := make(map[[2]int]bool, len(p.Cells))
	for _, c := range p.Cells {
		cells[c] = true
	}
	for _, g := range s.World.OnMountain {
		if g.OnLiftID != 0 {
			continue
		}
		if cells[[2]int{int(g.Pos[0] / CellSize), int(g.Pos[2] / CellSize)}] {
			return false
		}
	}
	return true
}

// patrollerFiring shells the patroller's path from the patrol room when
// the round's ready.
func (s *Simulation) patrollerFiring(p *world.Patroller) {
	path := s.World.SlidePathByID(p.AvyPathID)
	if path == nil {
		p.AvyPathID, p.State = 0, world.PatrollerAtHut
		return
	}
	if s.SimTime < p.TaskUntil {
		return
	}
	if !s.pathClear(path) {
		p.TaskUntil = s.SimTime + clockMinutes(mortarShotMinutes) // wait for them to get off it
		return
	}
	s.World.AvyToday += world.MortarShotCost
	s.controlRelease(path)
	p.AvyPathID, p.State = 0, world.PatrollerAtHut
}

// patrollerToCut takes the patroller up to the path's start zone.
func (s *Simulation) patrollerToCut(p *world.Patroller, dt float64) {
	if s.World.SlidePathByID(p.AvyPathID) == nil {
		p.AvyPathID, p.State = 0, world.PatrollerSkiingBack
		return
	}
	if s.patrollerGlide(p, max(p.ActionTimer, patrollerWalkSpeed), dt) {
		p.State = world.PatrollerCutting
		p.TaskUntil = s.SimTime + clockMinutes(cutMinutes)
	}
}

// patrollerCutting ski cuts the start zone, then skis back to the patrol
// room. Now and then the slope catches them.
func (s *Simulation) patrollerCutting(p *world.Patroller, base *world.Building) {
	path := s.World.SlidePathByID(p.AvyPathID)
	if path == nil || s.SimTime < p.TaskUntil {
		if path == nil {
			p.AvyPathID, p.State = 0, world.PatrollerSkiingBack
		}
		return
	}
	if !s.pathClear(path) {
		p.TaskUntil = s.SimTime + clockMinutes(cutMinutes)
		return
	}
	caught := path.Danger >= world.DangerModerate && rng.Global().Float32() < cutCaughtChance
	s.controlRelease(path)
	if caught && s.patrollerCaught(p, path) {
		return // killed
	}
	p.AvyPathID = 0
	p.TargetPos = s.World.PatrollerHutPos(base)
	p.State = world.PatrollerSkiingBack
}

// controlRelease brings path p down on purpose; with the danger down the
// runs below reopen.
func (s *Simulation) controlRelease(p *world.SlidePath) {
	if p.Danger >= world.DangerModerate {
		s.releasePath(p, true)
	} else {
		p.Load *= 0.5 // a little sloughs off, nothing runs
	}
	s.updateAvalancheDanger()
	s.updateAvyClosures()
}

// patrollerCaught is a patroller caught in their own ski cut: dug out by
// their crew, or killed; reports whether they died.
func (s *Simulation) patrollerCaught(p *world.Patroller, path *world.SlidePath) bool {
	s.World.SafetyIncident(world.SafetyPatrollerCaught)
	if rng.Global().Float32() < patrollerDeathChance {
		s.patrollerKilled(p)
		return true
	}
	s.World.LogEventAt(world.EventAvalanche, s.SimTime,
		"A patroller was caught ski cutting a slide path, and dug out alive", mgl32.Vec2{p.Pos[0], p.Pos[2]}, 0)
	return false
}

// Deaths in avalanches. A buried guest's chance of being dug out alive
// falls the longer patrol takes: buriedDeathBase if they're reached at
// once, rising to buriedDeathBase + buriedDeathRise by buriedDeathMinutes
// (the real survival curve falls off fast after a quarter of an hour).
// Nobody coming at all is fatal. A patroller caught is dug out by their
// crew at once, but dies patrollerDeathChance of the time.
const (
	buriedDeathBase      = float32(0.10)
	buriedDeathRise      = float32(0.60)
	buriedDeathMinutes   = 45.0
	patrollerDeathChance = float32(0.20)
)

// buriedSurvives rolls whether buried guest g, dug out now, is alive.
func (s *Simulation) buriedSurvives(g *world.Guest) bool {
	waited := float32((s.SimTime - g.BuriedAt) / clockMinutes(buriedDeathMinutes))
	return rng.Global().Float32() >= buriedDeathBase+buriedDeathRise*clamp32(waited, 0, 1)
}

// guestKilled is a guest killed in an avalanche: a dire line in the feed,
// the Safety score, and the people they came with going home.
func (s *Simulation) guestKilled(g *world.Guest) {
	w := s.World
	w.LogEventAt(world.EventAvalanche, s.SimTime, "A guest was killed in an avalanche", mgl32.Vec2{g.Pos[0], g.Pos[2]}, g.ID)
	w.SafetyIncident(world.SafetyDeath)
	g.Injured, g.Buried, g.Fallen, g.OnPatrollerID = false, false, false, 0
	g.Tumble = world.Tumble{}
	s.setDepartReason(g, ai.DepartDied)
	g.Leaving = world.Leaving{Pending: true, Review: w.FamilyReview(g), Reason: g.DepartReason}
	s.finishDeparture(g)
	g.Removed = true
	for _, m := range w.GroupOf(g) {
		if m.Party.Leader == g {
			m.Party.Leader = nil // they lead themselves home
		}
		if m == g || m.Removed || m.State != world.OnMountain {
			continue
		}
		s.applyEvent(m, ai.ThoughtLostSomeone)
		s.setDepartReason(m, ai.DepartHurt)
		if !m.Injured {
			s.directHomePlan(m)
		}
	}
}

// patrollerKilled is a patroller killed in their ski cut.
func (s *Simulation) patrollerKilled(p *world.Patroller) {
	w := s.World
	w.LogEventAt(world.EventAvalanche, s.SimTime, "A patroller was killed in an avalanche", mgl32.Vec2{p.Pos[0], p.Pos[2]}, 0)
	w.SafetyIncident(world.SafetyDeath)
	w.RemovePatroller(p.ID)
}
