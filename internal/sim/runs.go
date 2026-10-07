package sim

import (
	"math"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// A run is one descent as the guest skied it, recorded tick by tick in
// Guest.Run and judged when it ends: which trails it was really on, how
// steep and crowded it was, how much height it covered, and whether they
// fell. The verdict is a handful of thoughts, each with its ai.Effects.

const (
	// runMinSec is the shortest descent worth a verdict; shorter ones
	// are hops between a lodge, a lot, and a lift.
	runMinSec = float32(20)
	// runOnTrailShare is how much of a run must be on painted trails for
	// its difficulty to count.
	runOnTrailShare = float32(0.5)
	// runSteepShare is the share of a run more than runSteepMargin past
	// the guest's ComfortSlope that makes it too much for them. The
	// margin keeps a normal green, which runs a little past a beginner's
	// 10° here and there, from counting.
	runSteepShare  = float32(0.25)
	runSteepMargin = float32(5 * math.Pi / 180)
	// runCrowded is the average number of other skiers within about 7 m
	// that makes a run feel crowded.
	runCrowded = float32(1.5)
	// greatRunMinVertical is the least height, in metres, for a run to be
	// a great one: a beginner hill's worth.
	greatRunMinVertical = float32(40)
)

// startRun begins a fresh descent record at the guest's position.
func (s *Simulation) startRun(a *world.Guest) {
	a.Run = world.Run{Start: s.SimTime, StartY: a.Pos[1]}
}

// recordRun adds one skiing tick to the guest's run: time on the trail
// under them (or off-trail), steepness against their comfort, skiers
// nearby, grooming, and distance.
func (s *Simulation) recordRun(a *world.Guest, cx, cz int, grooming, slope, dt float32) {
	r := &a.Run
	r.Time += dt
	r.Distance += a.Speed * dt
	r.Groomed += grooming * dt
	if slope > a.Traits.ComfortSlope+runSteepMargin {
		r.Steep += dt
	}
	if t := s.World.TrailAt(cx, cz); t != nil {
		if i := diffIndex(t.Difficulty); i >= 0 {
			r.ByDiff[i] += dt
		}
		r.AddTrailTime(t.ID, dt)
	} else {
		r.OffTrail += dt
	}
	near := 0
	s.spatial.forEachNear(a.Pos[0], a.Pos[2], func(o *world.Guest) {
		if o != a && o.SkisOn && o.OnLiftID == 0 && !o.Queued && o.Speed > 1 {
			near++
		}
	})
	r.Crowd += float32(near) * dt
}

// judgeRun turns a finished run into thoughts. The run's difficulty is
// the trail difficulty it spent the most time on, compared with the
// guest's level: easier keeps or starts the too-easy condition, at or
// above their level ends it, and above it (or too much time on slopes
// past their comfort) is too much for them. A run at their level with
// enough vertical, no fall, and room to ski is a great run.
func (s *Simulation) judgeRun(a *world.Guest) {
	r := a.Run
	a.Run = world.Run{}
	if r.Time < runMinSec {
		return
	}
	trail := r.MainTrail()
	level := diffIndex(skillToDifficulty(a.Traits.Skill))
	main := -1
	onTrail := r.ByDiff[0] + r.ByDiff[1] + r.ByDiff[2]
	if onTrail >= runOnTrailShare*r.Time {
		for i := range r.ByDiff {
			if main < 0 || r.ByDiff[i] > r.ByDiff[main] {
				main = i
			}
		}
	}
	tooHard := r.Steep >= runSteepShare*r.Time || (main >= 0 && main > level)
	crowded := r.Crowd/r.Time >= runCrowded
	fell := fellSince(a, r.Start)

	if main >= 0 {
		s.setCondition(a, ai.ThoughtTooEasy, main < level, trail)
	}
	if tooHard {
		s.applyEvent(a, ai.ThoughtTooHard, trail)
	}
	if crowded {
		s.applyEvent(a, ai.ThoughtCrowdedRun, trail)
	}
	if a.Traits.PrefersGroomed && r.Groomed >= 0.9*r.Time {
		s.applyEvent(a, ai.ThoughtLovingCorduroy, trail)
	}
	if main == level && !tooHard && !crowded && !fell && r.StartY-a.Pos[1] >= greatRunMinVertical {
		s.applyEvent(a, ai.ThoughtGreatRun, trail)
	}
}

// diffIndex maps a single trail difficulty to 0 (green), 1 (blue), or 2
// (black); -1 for none.
func diffIndex(d world.TerrainDifficulty) int {
	switch d {
	case world.DiffGreen:
		return 0
	case world.DiffBlue:
		return 1
	case world.DiffBlack:
		return 2
	}
	return -1
}
