package sim

import (
	"math"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Snow underfoot is what the snow and terrain under a skiing guest are
// like, feature by feature, against their tastes (ai.Tastes): the more a
// guest loves what's under their skis, the higher their mood target, and
// the more they dislike it, the lower it goes and the faster they tire.
// Slopes past a guest's comfort frighten them whatever they like. Strong
// matches and mismatches are condition thoughts that report the pull; the
// pull itself is the taste term, so the thoughts carry no effect of their
// own.

const (
	// underfootWeight turns a taste × feature (each −1..1, 0..1) into a
	// pull on the mood target; the whole taste term is held to
	// ±underfootMaxPull.
	underfootWeight  = float32(0.15)
	underfootMaxPull = float32(0.25)
	// A taste × feature of underfootOnMatch starts that feature's
	// thought; it ends below underfootOffMatch, or when the guest stops
	// skiing.
	underfootOnMatch  = float32(0.4)
	underfootOffMatch = float32(0.2)
	// Fear: a pull of up to fearPull, reached fearSpan past the guest's
	// ComfortSlope.
	fearPull = float32(0.15)
	// underfootTiring is how much faster a guest tires per unit of
	// dislike for the snow under them.
	underfootTiring = float32(0.5)
	// underfootSmoothSec is the time constant, in sim seconds of skiing,
	// of the running average the thoughts and the pull read.
	underfootSmoothSec = float32(6)
)

var (
	fearSpan       = float32(10 * math.Pi / 180)
	steepFrom      = float32(10 * math.Pi / 180) // steep feature: 0 here
	steepTo        = float32(35 * math.Pi / 180) // … to 1 here
	freshPowderFul = float32(0.03)               // metres SWE of fresh powder (about 30 cm) for a full powder feature
	crowdFull      = float32(3)                  // moving skiers within ~7 m for a full crowd feature
)

// underfootThoughts are each taste's love and dislike thoughts (ThoughtNone
// where a feature has none).
var underfootThoughts = [ai.TasteCount][2]ai.ThoughtKind{
	ai.TastePowder: {ai.ThoughtLovingPowder, ai.ThoughtDeepSnow},
	ai.TasteMoguls: {ai.ThoughtLovingBumps, ai.ThoughtHatingBumps},
	ai.TasteTrees:  {ai.ThoughtLovingGlades, ai.ThoughtScaredInTrees},
	ai.TasteIce:    {ai.ThoughtNone, ai.ThoughtIcy},
}

// underfootConditions are every condition snow underfoot sets, cleared
// when the guest isn't skiing.
var underfootConditions = []ai.ThoughtKind{
	ai.ThoughtLovingPowder, ai.ThoughtDeepSnow, ai.ThoughtLovingBumps, ai.ThoughtHatingBumps,
	ai.ThoughtLovingGlades, ai.ThoughtScaredInTrees, ai.ThoughtIcy, ai.ThoughtTooSteep,
}

// underfootFeatures is how much of each taste's feature the cell under a
// skiing guest has, 0..1: grooming, fresh ungroomed powder, moguls, tree
// cover, steepness, an icy surface, and other skiers close by.
func (s *Simulation) underfootFeatures(a *world.Guest, cell *world.Cell, slope float32) [ai.TasteCount]float32 {
	var f [ai.TasteCount]float32
	if cell == nil {
		return f
	}
	f[ai.TasteGroomed] = clamp32(cell.Grooming, 0, 1)
	if top := cell.TopLayer(); top != nil {
		switch top.Kind {
		case world.KindPowder:
			f[ai.TastePowder] = clamp32(top.Accumulation/freshPowderFul, 0, 1) * (1 - f[ai.TasteGroomed])
		case world.KindBoilerplate, world.KindFrozenGranular:
			f[ai.TasteIce] = 1
		case world.KindCrust:
			// Mostly the overnight firming of corduroy on a cold clear
			// day: firm, not sheet ice.
			f[ai.TasteIce] = 0.25
		}
	}
	f[ai.TasteMoguls] = clamp32(cell.MogulSize, 0, 1)
	f[ai.TasteTrees] = cell.TreeCover()
	f[ai.TasteSteep] = clamp32((slope-steepFrom)/(steepTo-steepFrom), 0, 1)
	near := 0
	s.spatial.forEachNear(a.Pos[0], a.Pos[2], func(o *world.Guest) {
		if o != a && o.SkisOn && o.OnLiftID == 0 && !o.Queued && o.Speed > 1 {
			near++
		}
	})
	f[ai.TasteCrowds] = clamp32(float32(near)/crowdFull, 0, 1)
	return f
}

// tickUnderfoot reads the snow under a skiing guest against their tastes,
// averaged over the last few seconds of skiing: it starts and ends the
// underfoot thoughts, and returns the pull on the mood target (the taste
// term plus fear) and how much the guest dislikes what they're on (for
// tiring).
func (s *Simulation) tickUnderfoot(a *world.Guest, cell *world.Cell, slope, dt float32) (pull, dislike float32) {
	f := s.underfootFeatures(a, cell, slope)
	t := a.Traits.Tastes
	blend := min(dt/underfootSmoothSec, 1)
	for k := range f {
		a.Underfoot[k] += (t[k]*f[k] - a.Underfoot[k]) * blend
		m := a.Underfoot[k]
		pull += m
		dislike += max(0, -m)
		for side, kind := range underfootThoughts[k] {
			if kind == ai.ThoughtNone {
				continue
			}
			sm := m
			if side == 1 {
				sm = -m
			}
			on := sm >= underfootOnMatch || (a.Conditions.Has(kind) && sm >= underfootOffMatch)
			s.setCondition(a, kind, on)
		}
	}
	pull = clamp32(pull*underfootWeight, -underfootMaxPull, underfootMaxPull)
	a.UnderfootFear += (clamp32((slope-a.Traits.ComfortSlope)/fearSpan, 0, 1) - a.UnderfootFear) * blend
	fear := a.UnderfootFear
	s.setCondition(a, ai.ThoughtTooSteep, fear >= 0.5 || (a.Conditions.Has(ai.ThoughtTooSteep) && fear >= 0.2))
	return pull - fear*fearPull, dislike
}
