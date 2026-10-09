package world

import "mountain-mogul/internal/ai"

// A guest's day is a collection of moments, each with a class
// (ai.Effects); the classes set the level of the star rating they leave
// and the quality of the services they used adds to it. See
// notes/next/Scoreless Rating.md.

// QualityStars is how many stars excellent service (quality 1) adds.
const QualityStars = 1.2

// MaxStars is the top of the rating.
const MaxStars = 5

// Moment is a count of one kind of moment in a guest's visit, with the
// entity (trail, lift, building) of the latest.
type Moment struct {
	Kind    ai.ThoughtKind
	N       uint16
	Context uint64
}

// HeldCondition is a condition that's on, since when, and whether it has
// held long enough to count as a moment.
type HeldCondition struct {
	Kind    ai.ThoughtKind
	Since   float64
	Counted bool
}

// Review is the star rating a guest leaves and why.
type Review struct {
	Level   int8    // 1–3: dealbreaker, letdown, a good day
	Stars   float32 // Level plus QualityStars × Quality, at most MaxStars
	Quality float32 // average quality of the services used, 0–1
	// Kind is the moment that set the level: the dealbreaker, the
	// letdown, the most frequent annoyance when there were too many, or
	// the most frequent highlight at 3★. ThoughtNone at 2★ means nothing
	// special happened; at 1★, that they never got a run in.
	Kind       ai.ThoughtKind
	Count      int
	Context    uint64
	Annoyances int
	NoRuns     bool // left without skiing a run
}

// AddMoment counts one moment of kind.
func (g *Guest) AddMoment(kind ai.ThoughtKind, context uint64) {
	for i := range g.Moments {
		if g.Moments[i].Kind == kind {
			g.Moments[i].N++
			if context != 0 {
				g.Moments[i].Context = context
			}
			return
		}
	}
	g.Moments = append(g.Moments, Moment{Kind: kind, N: 1, Context: context})
}

// UseService adds one use of a service of quality q (0–1) to the guest's
// quality: a building visit or a lift ride.
func (g *Guest) UseService(q float32) {
	g.QualitySum += q
	g.QualityUses++
}

// ServiceQuality is the average quality of the services the guest used,
// 0 when they used none.
func (g *Guest) ServiceQuality() float32 {
	if g.QualityUses == 0 {
		return 0
	}
	return g.QualitySum / float32(g.QualityUses)
}

// DayReview is the review the guest would leave now.
func (g *Guest) DayReview() Review {
	var deal, letdown, annoy, high *Moment
	r := Review{NoRuns: len(g.TrailTally) == 0}
	for i := range g.Moments {
		m := &g.Moments[i]
		switch ai.Effects[m.Kind].Class {
		case ai.Dealbreaker:
			if deal == nil {
				deal = m
			}
		case ai.Letdown:
			if letdown == nil {
				letdown = m
			}
		case ai.Annoyance:
			r.Annoyances += int(m.N)
			if annoy == nil || m.N > annoy.N {
				annoy = m
			}
		case ai.Highlight:
			if high == nil || m.N > high.N {
				high = m
			}
		}
	}
	var why *Moment
	switch {
	case deal != nil:
		r.Level, why = 1, deal
	case r.NoRuns:
		r.Level = 1
		r.Kind = g.noRunsReason()
	case letdown != nil:
		r.Level, why = 2, letdown
	case r.Annoyances >= ai.AnnoyancesPerLetdown:
		r.Level, why = 2, annoy
	case high == nil:
		r.Level = 2
	default:
		r.Level, why = 3, high
	}
	if why != nil {
		r.Kind, r.Count, r.Context = why.Kind, int(why.N), why.Context
	}
	r.Quality = g.ServiceQuality()
	r.Stars = min(float32(r.Level)+QualityStars*r.Quality, MaxStars)
	return r
}

// noRunsReason is what kept a guest who never skied off the snow: a
// letdown condition still on, or the last letdown or annoyance they
// thought, before it had time to count as a moment.
func (g *Guest) noRunsReason() ai.ThoughtKind {
	for k := ai.ThoughtKind(1); int(k) < ai.ThoughtKindCount; k++ {
		if g.Conditions.Has(k) && ai.Effects[k].Class == ai.Letdown {
			return k
		}
	}
	for i := 0; i < thoughtsCap; i++ {
		t := g.Thoughts[(g.ThoughtsHead-1-i+thoughtsCap)%thoughtsCap]
		if c := ai.Effects[t.Kind].Class; t.Kind != ai.ThoughtNone && (c == ai.Letdown || c == ai.Annoyance || c == ai.Dealbreaker) {
			return t.Kind
		}
	}
	return ai.ThoughtNone
}
