package sim

import (
	"math"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/ai/goap"
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
	// runLevelShare is how much of a run's time on trails must be at or
	// above the easiest difficulty the guest wants for it to suit them.
	runLevelShare = float32(0.25)
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
	// runMiserable is the average taste match at or below which a run
	// was miserable for the guest (their dislikes all the way down), and
	// can't be great.
	runMiserable = float32(-0.3)
	// First tracks: a powder lover (powder taste ≥ firstTracksLover) who
	// skied fresh powder for at least firstTracksShare of the run. Fresh
	// means a powder feature of freshPowderMin on a cell with less than
	// freshTrafficMax of SkierTraffic since it fell.
	firstTracksLover = float32(0.4)
	// Boredom: a lift's value to a guest is how well its terrain suits
	// them, less lapStaleness for each run they've had off it today. When
	// the best value left falls below boredFloor they're bored (if they've
	// lapped it) or it was never their kind of skiing.
	lapStaleness     = float32(0.1)
	boredFloor       = float32(-0.3)
	firstTracksShare = float32(1.0 / 3)
	freshPowderMin   = float32(0.5)
	freshTrafficMax  = float32(0.5)
)

// startRun begins a fresh descent record at the guest's position, from
// the lift they're getting off if they are.
func (s *Simulation) startRun(a *world.Guest) {
	a.Run = world.Run{Start: s.SimTime, StartY: a.Pos[1], LiftID: a.Unload.LiftID}
	a.Underfoot, a.UnderfootFear = [ai.TasteCount]float32{}, 0
}

// recordRun adds one skiing tick to the guest's run: time on the trail
// under them (or off-trail), steepness against their comfort, skiers
// nearby, grooming, how well the snow suits them (match, from
// tickUnderfoot), fresh untracked powder, and distance.
func (s *Simulation) recordRun(a *world.Guest, cell *world.Cell, cx, cz int, grooming, slope, match, powder, dt float32) {
	r := &a.Run
	r.Time += dt
	r.Distance += a.Speed * dt
	r.Groomed += grooming * dt
	r.Taste += match * dt
	if cell != nil && powder >= freshPowderMin && cell.SkierTraffic < freshTrafficMax {
		r.Fresh += dt
	}
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

// judgeRun turns a finished run into thoughts and its score. How well the
// snow suited the guest (the run's average taste match) scales a great
// run's bonus by 1 + the match; a run at or below runMiserable is
// miserable instead. A powder lover who skied enough fresh, untracked
// powder gets first tracks. The run's difficulty is
// the trail difficulty it spent the most time on, compared with the
// levels the guest is happy skiing (runRange: their skill level and the
// level they want, which their taste for steeps shifts): easier keeps or
// starts the too-easy condition, within the range ends it, and harder
// (or too much time on slopes past their comfort) is too much for them.
// A run within the range with enough vertical, no fall, and room to ski
// is a great run.
func (s *Simulation) judgeRun(a *world.Guest) {
	r := a.Run
	a.Run = world.Run{}
	if r.Time < runMinSec {
		return
	}
	defer s.checkBoredom(a)
	trail := r.MainTrail()
	lo, hi := runRange(a.Traits)
	main := -1
	onTrail := r.ByDiff[0] + r.ByDiff[1] + r.ByDiff[2]
	if onTrail >= runOnTrailShare*r.Time {
		for i := range r.ByDiff {
			if main < 0 || r.ByDiff[i] > r.ByDiff[main] {
				main = i
			}
		}
	}
	// At their level: a run that spends some real time on terrain at or
	// above the easiest they want, since a short black often runs out on
	// a long green to the lift.
	var atLevel float32
	for i := max(lo, 0); i < len(r.ByDiff); i++ {
		atLevel += r.ByDiff[i]
	}
	levelOK := main >= 0 && atLevel >= runLevelShare*onTrail
	tooHard := r.Steep >= runSteepShare*r.Time || (main >= 0 && main > hi)
	crowded := r.Crowd/r.Time >= runCrowded
	fell := fellSince(a, r.Start)
	taste := clamp32(r.Taste/r.Time, -1, 1)
	if taste <= runMiserable {
		s.applyEvent(a, ai.ThoughtMiserableRun, trail)
	}
	if a.Traits.Tastes[ai.TastePowder] >= firstTracksLover && r.Fresh >= firstTracksShare*r.Time {
		s.applyEvent(a, ai.ThoughtFirstTracks, trail)
	}

	if main >= 0 {
		s.setCondition(a, ai.ThoughtTooEasy, !levelOK, trail)
	}
	if tooHard {
		s.applyEvent(a, ai.ThoughtTooHard, trail)
	}
	// Crowding bothers a guest as much as they dislike crowds.
	if mind := -a.Traits.Tastes[ai.TasteCrowds]; crowded && mind > 0.1 {
		s.applyEvent(a, ai.ThoughtCrowdedRun, trail)
	}
	great := levelOK && !tooHard && !crowded && !fell && taste > runMiserable && r.StartY-a.Pos[1] >= greatRunMinVertical
	if trail != 0 {
		a.TrailTally = world.CountRun(a.TrailTally, trail, great)
	}
	if r.LiftID != 0 {
		a.LiftTally = world.CountRun(a.LiftTally, r.LiftID, great)
	}
	if great {
		// Why it was great, when it was the corduroy they love: a report
		// only, so the run counts once.
		if a.Traits.Tastes.PrefersGroomed() && r.Groomed >= 0.9*r.Time {
			s.recordThought(a, ai.ThoughtLovingCorduroy, trail)
		}
		s.applyEvent(a, ai.ThoughtGreatRun, trail)
	}
}

// checkBoredom values each lift the guest would ride (its terrain's taste
// match, less lapStaleness per run off it today) and starts the boredom
// conditions when the best is below boredFloor: "skied this place to
// death" when they've lapped it, "nothing here is my kind of skiing" when
// it never suited them. Run after each run, so every guest gets one.
func (s *Simulation) checkBoredom(a *world.Guest) {
	w := s.World
	level := skillToDifficulty(a.Traits.Skill)
	rideable := level | (level - 1)
	if a.Traits.Skill >= ai.SkillAdvancedThreshold {
		rideable = 0
	}
	best, laps, any := float32(math.Inf(-1)), int32(0), false
	for _, l := range w.Lifts {
		if !l.Open || l.OnHold || (rideable != 0 && !w.ServicesForLift(l.ID).Has(rideable)) || !goap.LiftAccessible(l, a.Traits.Skill, w) {
			continue
		}
		c, ok := w.LiftConditions(l.ID)
		if !ok {
			continue
		}
		var runs int32
		for _, t := range a.LiftTally {
			if t.ID == l.ID {
				runs = t.Runs
			}
		}
		if v := tasteMatch(a.Traits.Tastes, c) - lapStaleness*float32(runs); v > best {
			best, laps, any = v, runs, true
		}
	}
	bored := any && best < boredFloor
	s.setCondition(a, ai.ThoughtBored, bored && laps > 0)
	s.setCondition(a, ai.ThoughtNotMySkiing, bored && laps == 0)
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
