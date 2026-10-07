package sim

import (
	"math"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Snow underfoot is what the snow and terrain under a skiing guest are
// like, feature by feature, against their tastes (ai.Tastes). Strong
// matches and mismatches are condition thoughts, and slopes past a
// guest's comfort frighten them whatever they like; the more a guest
// dislikes what's under their skis, the faster they tire. What it does to
// their score comes at the end of each run, in its verdict (Snow Tastes
// step 3), so these thoughts carry no effect of their own.

const (
	// A taste × feature of underfootOnMatch starts that feature's
	// thought; it ends below underfootOffMatch, or when the guest stops
	// skiing.
	underfootOnMatch  = float32(0.4)
	underfootOffMatch = float32(0.2)
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

// cellFeatures is how much of each taste's snow and terrain feature a
// cell has, 0..1: grooming, fresh ungroomed powder, moguls, tree cover,
// steepness (slope in radians), and an icy surface. Crowds aren't a cell's
// own: underfootFeatures adds them for a guest. Every place a guest judges
// snow reads it here (underfoot, steering, trail conditions), so a richer
// source, like a mogul map behind Cell.MogulSize, reaches them all.
func cellFeatures(cell *world.Cell, slope float32) [ai.TasteCount]float32 {
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
	return f
}

// cellSlope is a cell's slope angle in radians.
func cellSlope(cell *world.Cell) float32 {
	return float32(math.Atan(float64(cell.Slope)))
}

// tasteMatch is how well features f suit tastes t: the sum of taste ×
// feature.
func tasteMatch(t ai.Tastes, f [ai.TasteCount]float32) float32 {
	var m float32
	for k := range f {
		m += t[k] * f[k]
	}
	return m
}

// underfootFeatures is cellFeatures for the cell under a skiing guest,
// plus how crowded it is around them.
func (s *Simulation) underfootFeatures(a *world.Guest, cell *world.Cell, slope float32) [ai.TasteCount]float32 {
	f := cellFeatures(cell, slope)
	if cell == nil {
		return f
	}
	near := 0
	s.spatial.forEachNear(a.Pos[0], a.Pos[2], func(o *world.Guest) {
		if o != a && o.SkisOn && o.OnLiftID == 0 && !o.Queued && o.Speed > 1 {
			near++
		}
	})
	f[ai.TasteCrowds] = clamp32(float32(near)/crowdFull, 0, 1)
	return f
}

// refreshTrailConditions sets every trail's Conditions to the average of
// its cells' features: what skiing it offers right now, read by guests
// choosing a lift. Runs every clock hour, which catches grooming, storms,
// and moguls building through the day.
func (s *Simulation) refreshTrailConditions() {
	t := s.World.Terrain
	for _, tr := range s.World.Trails {
		var sum [ai.TasteCount]float32
		n := 0
		for _, c := range tr.Cells {
			if !t.InBounds(c[0], c[1]) {
				continue
			}
			cell := &t.Cells[c[0]][c[1]]
			f := cellFeatures(cell, cellSlope(cell))
			for k := range sum {
				sum[k] += f[k]
			}
			n++
		}
		if n > 0 {
			for k := range sum {
				sum[k] /= float32(n)
			}
		}
		tr.Conditions = sum
	}
}

// tickUnderfoot reads the snow under a skiing guest against their tastes,
// averaged over the last few seconds of skiing: it starts and ends the
// underfoot thoughts (fear, fearSpan past ComfortSlope at its fullest,
// among them). It returns how much the guest dislikes what they're on,
// for tiring; and, for the run's verdict, how well this cell suits them
// (the sum of taste × feature) and how much fresh powder it has.
func (s *Simulation) tickUnderfoot(a *world.Guest, cell *world.Cell, slope, dt float32) (dislike, match, powder float32) {
	f := s.underfootFeatures(a, cell, slope)
	t := a.Traits.Tastes
	blend := min(dt/underfootSmoothSec, 1)
	powder = f[ai.TastePowder]
	for k := range f {
		match += t[k] * f[k]
		a.Underfoot[k] += (t[k]*f[k] - a.Underfoot[k]) * blend
		m := a.Underfoot[k]
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
	a.UnderfootFear += (clamp32((slope-a.Traits.ComfortSlope)/fearSpan, 0, 1) - a.UnderfootFear) * blend
	fear := a.UnderfootFear
	s.setCondition(a, ai.ThoughtTooSteep, fear >= 0.5 || (a.Conditions.Has(ai.ThoughtTooSteep) && fear >= 0.2))
	return dislike, match, powder
}
