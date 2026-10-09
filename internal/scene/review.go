package scene

import (
	"fmt"
	"time"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// ReviewLine is a guest's review as the player reads it: the moment that
// set its stars, quoted, with how often it happened.
func ReviewLine(r world.Review, resolve func(uint64) string) string {
	quote := func() string {
		var ctx []uint64
		if r.Context != 0 {
			ctx = []uint64{r.Context}
		}
		q := `"` + ai.Thought{Kind: r.Kind, Context: ctx}.Display(resolve) + `"`
		if r.Count > 1 {
			q += fmt.Sprintf(" x%d", r.Count)
		}
		return q
	}
	switch {
	case r.Level == 1 && r.Kind == ai.ThoughtNone:
		return "never got a run in"
	case r.Level == 2 && r.Kind == ai.ThoughtNone:
		return "nothing special"
	case r.Level == 2 && ai.Effects[r.Kind].Class == ai.Annoyance:
		return fmt.Sprintf("too many little things (%d): %s", r.Annoyances, quote())
	}
	return quote()
}

// Review chart colours by what set the review.
var (
	reviewGood        = mgl32.Vec4{0.35, 0.85, 0.45, 1}
	reviewDealbreaker = mgl32.Vec4{0.90, 0.25, 0.25, 1}
	reviewLetdown     = mgl32.Vec4{0.95, 0.60, 0.25, 1}
	reviewAnnoyance   = mgl32.Vec4{0.95, 0.85, 0.35, 1}
	reviewQuiet       = mgl32.Vec4{0.60, 0.62, 0.70, 1}
)

// reviewKinds are the moments that can set a review below 3★, in
// enum order: the Reviews chart's rows between "a good day" and the
// fixed rows at the end.
func reviewKinds() []ai.ThoughtKind {
	var out []ai.ThoughtKind
	for k := ai.ThoughtKind(1); int(k) < ai.ThoughtKindCount; k++ {
		switch ai.Effects[k].Class {
		case ai.Dealbreaker, ai.Letdown, ai.Annoyance:
			out = append(out, k)
		}
	}
	return out
}

// reviewChartSeries is the Reviews chart's rows: a good day, each moment
// that can set a review below 3★, nothing special, no runs, and the
// guests who went home without renting skis (lost business, no review).
func reviewChartSeries() []ui.ChartSeries {
	out := []ui.ChartSeries{{Name: "a good day (3 stars)", Color: reviewGood}}
	for _, k := range reviewKinds() {
		name, col := ai.ThoughtLabel[k], reviewLetdown
		switch ai.Effects[k].Class {
		case ai.Dealbreaker:
			col = reviewDealbreaker
		case ai.Annoyance:
			name, col = "too many little things: "+name, reviewAnnoyance
		}
		out = append(out, ui.ChartSeries{Name: name, Color: col})
	}
	return append(out,
		ui.ChartSeries{Name: "nothing special", Color: reviewQuiet},
		ui.ChartSeries{Name: "never got a run in", Color: reviewDealbreaker},
		ui.ChartSeries{Name: "couldn't rent skis (lost business)", Color: reviewQuiet},
	)
}

// lastReviews is the last completed day's reviews and departures, or
// today's so far before the first day is done.
func lastReviews(w *world.World) (world.ReviewTally, int, time.Time) {
	h := w.History
	if samples := h.Ordered(); len(samples) > 0 {
		last := samples[len(samples)-1]
		return last.Reviews, last.DepartReasons[ai.DepartNoRentals], last.Day
	}
	return h.ReviewsToday, h.DepartReasonsToday[ai.DepartNoRentals], time.Time{}
}

// reviewsToDistribution is the Reviews chart's values, in
// reviewChartSeries order.
func reviewsToDistribution(w *world.World) []ui.ChartPoint {
	if w == nil || w.History == nil {
		return nil
	}
	t, lost, day := lastReviews(w)
	vals := []float64{float64(t.Good)}
	for _, k := range reviewKinds() {
		vals = append(vals, float64(t.Why[k]))
	}
	vals = append(vals, float64(t.NothingSpecial), float64(t.NoRuns), float64(lost))
	return []ui.ChartPoint{{Day: day, Values: vals}}
}

// reviewsTitle heads the Reviews chart with the day's average stars and
// how many reviews were left at each level.
func reviewsTitle(w *world.World) string {
	if w == nil || w.History == nil {
		return "Reviews"
	}
	t, _, _ := lastReviews(w)
	if t.N == 0 {
		return "Reviews"
	}
	return fmt.Sprintf("Reviews: %.1f stars from %d guests (3 stars: %d, 2: %d, 1: %d)",
		t.Stars/float32(t.N), t.N, t.Levels[3], t.Levels[2], t.Levels[1])
}
