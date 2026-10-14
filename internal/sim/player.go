//go:build !demo

package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// SkiFree Mode (notes/next/SkiFree Mode.md): the player skis the resort
// as one skier (World.Player). Their keys build the steering Decision in
// place of decide(); everything after it (applySkier: grip, skids,
// moguls, trees, balance and falls) is the guests' own. They're never
// injured and patrol never comes: after a fall they get up and carry on.
// They aren't one of World.OnMountain, so planning, departures, reviews
// and the resort's numbers never see them, but the guests dodge them.

const (
	// playerSkill is the player skier's skill: advanced.
	playerSkill = 0.8
	// playerBrakeScrub and playerStopScrub are the braking (S) and the
	// hard stop (space), m/s² past friction.
	playerBrakeScrub = 3.0
	playerStopScrub  = 7.0
	// playerLook is how far ahead along their heading the player's
	// perception aims, metres.
	playerLook = 20
	// playerJoinReach and playerJoinSpeed: stopping (under the speed,
	// m/s) within the reach (metres) of the back of a lift's line joins
	// it. No ticket check: it's the player's resort.
	playerJoinReach = 6
	playerJoinSpeed = 1.5
)

// PlayerInput is what the player is pressing this frame.
type PlayerInput struct {
	Left, Right bool // turn
	Sharp       bool // turn at the pivot rate, skidding round
	Brake       bool // check speed
	Stop        bool // stop hard
	Push        bool // pole along on the flat
}

// StartPlayer puts the player's skier at lift l's top, on the apron a
// run off it starts from, facing down the fall line, and returns them.
func (s *Simulation) StartPlayer(l *world.Lift) *world.Guest {
	w := s.World
	t := w.Terrain
	at := l.TopRunEnd(l.Base)
	n := t.NormalAt(at[0]/CellSize, at[1]/CellSize)
	heading := float32(math.Atan2(float64(l.Base[0]-at[0]), float64(l.Base[1]-at[1])))
	if fall := (mgl32.Vec2{n[0], n[2]}); fall.Len() > 0.05 {
		heading = float32(math.Atan2(float64(fall[0]), float64(fall[1])))
	}
	g := &world.Guest{
		ID:         w.NextID(),
		State:      world.OnMountain,
		Discipline: world.Ski,
		Pos:        mgl32.Vec3{at[0], t.InterpolatedSurfaceElevationAt(at[0], at[1]), at[1]},
		Heading:    heading,
		Traits:     ai.TraitsFor(playerSkill),
		Balance:    1,
		Patience:   1,
		Energy:     1,
		Hunger:     1,
		Thirst:     1,
		SkisOn:     true,
		Player:     true,
	}
	w.Player = g
	s.PlayerInput = PlayerInput{}
	return g
}

// StopPlayer takes the player's skier off the mountain, out of any lift
// line or off any chair.
func (s *Simulation) StopPlayer() {
	if p := s.World.Player; p != nil {
		for _, l := range s.World.Lifts {
			l.LeaveLine(p)
			for i := range l.Chairs {
				for j, q := range l.Chairs[i].Passengers {
					if q == p {
						l.Chairs[i].Passengers[j] = nil
					}
				}
			}
		}
	}
	s.World.Player = nil
	s.PlayerInput = PlayerInput{}
}

// tickPlayer moves the player's skier a step: getting up after a fall,
// or skiing on their keys.
func (s *Simulation) tickPlayer(dt float64) {
	p := s.World.Player
	if p == nil {
		return
	}
	switch {
	case p.Fallen:
		s.tickFallen(p, dt)
		return
	case p.Unload.LiftID != 0:
		s.tickUnloading(p, dt)
		return
	case p.OnLiftID != 0:
		s.tickRiding(p, dt)
		return
	case p.Queued:
		s.tickQueued(p, dt)
		return
	}
	defer s.playerJoinLine(p)
	t := s.World.Terrain
	p.SkisOn = true
	hx, hz := float32(math.Sin(float64(p.Heading))), float32(math.Cos(float64(p.Heading)))
	ax, az := p.Pos[0]+hx*playerLook, p.Pos[2]+hz*playerLook
	ahead := mgl32.Vec3{ax, t.InterpolatedSurfaceElevationAt(ax, az), az}
	perc := perceive(t, p, ahead)
	dec := playerDecision(p, perc, s.PlayerInput)
	s.applySkier(p, ahead, playerLook, perc, dec, dt)
}

// playerDecision is the steering the player's keys ask for: turning at
// the carve rate (the pivot rate, skidding, with Sharp), braking, a hard
// stop, or poling along.
func playerDecision(p *world.Guest, perc Perception, in PlayerInput) Decision {
	carve, pivot := turnRates(p.Traits.Skill, perc.Speed)
	dec := Decision{DesiredHeading: p.Heading, TurnRate: carve, CarveRate: carve, Push: in.Push, Mode: "straight"}
	turn := float32(0)
	if in.Left {
		turn++
	}
	if in.Right {
		turn--
	}
	if turn != 0 {
		// Heading grows toward +x from +z: to the skier's left.
		dec.DesiredHeading = wrapAngle(p.Heading + turn*math.Pi/2)
		dec.Mode = "carve"
		if in.Sharp {
			dec.TurnRate = pivot
			dec.Mode = "skid"
		}
	}
	switch {
	case in.Stop:
		dec.Scrub, dec.Mode = playerStopScrub, "brake"
	case in.Brake:
		dec.Scrub, dec.Mode = playerBrakeScrub, "brake"
	}
	return dec
}

// playerJoinLine puts the player in a lift's line when they've stopped at
// the back of it, as a guest joins (lift.JoinLine); the chairs do the
// rest, as for the guests (tickLifts, tickRiding, tickUnloading).
func (s *Simulation) playerJoinLine(p *world.Guest) {
	if p.Fallen || p.Speed > playerJoinSpeed {
		return
	}
	w := s.World
	pos := mgl32.Vec2{p.Pos[0], p.Pos[2]}
	for _, l := range w.Lifts {
		if !l.Open || l.OnHold || l.Cableless() {
			continue
		}
		back := l.BackOfQueueWorldPos(pos, w.Terrain)
		if (mgl32.Vec2{back[0], back[2]}).Sub(pos).Len() > playerJoinReach {
			continue
		}
		if l.JoinLine(p, s.SimTime) {
			w.RebuildRopes()
		}
		p.Queued, p.Speed, p.TurnSide = true, 0, 0
		return
	}
}
