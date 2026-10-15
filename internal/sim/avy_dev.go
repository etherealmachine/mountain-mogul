package sim

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// DevAvalanche sets off the slide path nearest world (x, z), for the
// -screenshot harness: how is "slide" (it runs on its own), "mortar" (a
// patroller shells it from their patrol room) or "cut" (a patroller ski
// cuts it). Returns the path's start zone centre, or false when there's
// no path or no patroller.
func (s *Simulation) DevAvalanche(x, z float32, how string) (mgl32.Vec2, bool) {
	s.ensureSlidePaths()
	s.refreshPathTrails()
	s.updateAvalancheDanger()
	var best *world.SlidePath
	var bestD float32
	for _, p := range s.World.SlidePaths {
		d := mgl32.Vec2{p.CentreX - x, p.CentreZ - z}.Len()
		if best == nil || d < bestD {
			best, bestD = p, d
		}
	}
	if best == nil {
		return mgl32.Vec2{}, false
	}
	best.Load = max(best.Load, 0.1) // loaded enough to run
	best.Danger = max(best.Danger, world.DangerDanger)
	at := mgl32.Vec2{best.CentreX, best.CentreZ}
	if how == "slide" {
		s.releasePath(best, false)
		return at, true
	}
	var p *world.Patroller
	for _, q := range s.World.Patrollers {
		if q.AvyPathID == 0 {
			p = q
			break
		}
	}
	if p == nil {
		return at, false
	}
	p.AvyPathID, p.Fired = best.ID, false
	p.OnSkis = true
	switch how {
	case "mortar":
		if b := findBuilding(s.World, p.HutID); b != nil {
			p.Pos = s.World.PatrollerHutPos(b)
		}
		p.State, p.TaskUntil = world.PatrollerFiring, s.SimTime
	default: // "cut"
		target := s.startZoneTarget(best)
		p.Pos = target
		p.State, p.TaskUntil = world.PatrollerCutting, s.SimTime+clockMinutes(cutMinutes)
		p.TargetPos = s.cutEnd(p, 1)
	}
	fmt.Printf("DevAvalanche: patroller %d at %.0f,%.0f\n", p.ID, p.Pos[0], p.Pos[2])
	return at, true
}
