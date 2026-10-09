package world

import (
	"cmp"
	"slices"
	"time"

	"mountain-mogul/internal/ai"
)

// HistoryCapacity is the ring-buffer length, in days. 376 ≈ 2 ski
// seasons (~188 days each), matching Planet Coaster's "last two years"
// graph window. When deeper history is wanted later, the plan is to
// downsample older entries into monthly buckets rather than grow this.
const HistoryCapacity = 376

// RevenueKind is a daily income category.
type RevenueKind int

const (
	RevenueDayTickets RevenueKind = iota
	RevenueSeasonPasses
	RevenueHeli
	RevenueParking
	RevenueFood
	RevenueBar
	RevenueRentals
	RevenueKindCount
)

// Label is the category's name in the daily report.
func (k RevenueKind) Label() string {
	switch k {
	case RevenueDayTickets:
		return "Day tickets"
	case RevenueSeasonPasses:
		return "Season passes"
	case RevenueHeli:
		return "Heli fares"
	case RevenueParking:
		return "Parking"
	case RevenueFood:
		return "Food court"
	case RevenueBar:
		return "Bar"
	case RevenueRentals:
		return "Rentals"
	}
	return "Other"
}

// CostKind is a daily expense category.
type CostKind int

const (
	CostLifts CostKind = iota // attendants and running costs
	CostSnowcats
	CostBuildings
	CostSnowGuns
	CostInterest // credit-line interest, charged at month end
	CostKindCount
)

// Label is the category's name in the daily report.
func (k CostKind) Label() string {
	switch k {
	case CostLifts:
		return "Lifts"
	case CostSnowcats:
		return "Snowcats"
	case CostBuildings:
		return "Buildings"
	case CostSnowGuns:
		return "Snow guns"
	case CostInterest:
		return "Interest"
	}
	return "Other"
}

// CostBreakdown is one day's costs by category.
type CostBreakdown [CostKindCount]int

// Total sums every category.
func (c CostBreakdown) Total() int {
	t := 0
	for _, v := range c {
		t += v
	}
	return t
}

// DailySample is one row in the History ring. Each in-game day rollover
// pushes one of these; the readers iterate via History.Ordered to walk
// them oldest-first regardless of where the ring head currently sits.
type DailySample struct {
	Day              time.Time                 // calendar date this sample covers
	GuestsOnMountain int                       // active OnMountain count at EOD
	ArrivalsToday    int                       // spawns during this day
	DeparturesToday  int                       // departures during this day
	Cash             int                       // resort cash balance at EOD
	Revenue          int                       // all income this day
	Costs            int                       // all costs this day (operating + interest)
	RevenueByKind    [RevenueKindCount]int     // Revenue split by category
	CostsByKind      CostBreakdown             // Costs split by category
	Open             bool                      // the resort was open at some point in the day
	Rating           float32                   // resort rating at EOD, in stars (1–5)
	ThoughtCounts    [ai.ThoughtKindCount]int  // per-kind thought totals emitted during the day
	DepartReasons    [ai.DepartReasonCount]int // why each guest who left that day went home
	Falls            int                       // times a guest went down
	Reviews          ReviewTally               // the day's reviews
}

// ReviewTally is a day's reviews in aggregate, for the Reviews chart.
type ReviewTally struct {
	N      int     // reviews left
	Stars  float32 // their stars summed
	Levels [4]int  // by level (1–3)
	// Why counts the moment that set each review: below 3★ the
	// dealbreaker, letdown, or most frequent annoyance; at 3★ Good.
	Why            [ai.ThoughtKindCount]int
	NothingSpecial int // 2★: no highlight
	NoRuns         int // 1★: never got a run in, with nothing to name
	Good           int // 3★
}

// FallRecord is one guest going down, for the patrol report and the falls
// overlay.
type FallRecord struct {
	X, Z    float32 // where, in world metres
	TrailID uint64  // the run they were skiing; 0 off any run
	LiftID  uint64  // the lift they were getting off, for a fall unloading
}

// History is a per-world ring of DailySamples plus the day-in-progress
// counters. Lives off World via a *History pointer so a fresh World (no
// history yet) is zero-cost and older saves serialise without padding.
// Sim writes RecordArrival / RecordDeparture during the day, then Push
// at day-rollover flips the running totals into a sample.
type History struct {
	Samples [HistoryCapacity]DailySample
	Head    int  // next write index
	Filled  bool // false until the ring has wrapped at least once

	// Day-in-progress counters. Reset by Push.
	ArrivalsToday      int
	DeparturesToday    int
	RevenueToday       int
	RevenueByKindToday [RevenueKindCount]int
	ThoughtCountsToday [ai.ThoughtKindCount]int
	DepartReasonsToday [ai.DepartReasonCount]int
	ReviewsToday       ReviewTally
	FallsToday         []FallRecord
}

// NewHistory returns an empty History ready to start recording. The
// underlying Samples array is zero-initialised; Head=0, Filled=false.
func NewHistory() *History {
	return &History{}
}

// RecordArrival bumps the in-progress arrivals counter. Safe to call
// when h is nil — does nothing.
func (h *History) RecordArrival() {
	if h == nil {
		return
	}
	h.ArrivalsToday++
}

// RecordDeparture counts one departing guest: why they left, and their
// review toward the day's average stars. A guest who came without skis
// and couldn't rent any leaves no review: they're lost business, not a
// rating. Safe to call when h is nil — does nothing.
func (h *History) RecordDeparture(r Review, why ai.DepartReason) {
	if h == nil {
		return
	}
	h.DeparturesToday++
	h.DepartReasonsToday[why]++
	if why == ai.DepartNoRentals {
		return
	}
	t := &h.ReviewsToday
	t.N++
	t.Stars += r.Stars
	t.Levels[r.Level]++
	switch {
	case r.Level >= 3:
		t.Good++
	case r.Kind != ai.ThoughtNone:
		t.Why[r.Kind]++
	case r.Level == 2:
		t.NothingSpecial++
	default:
		t.NoRuns++
	}
}

// DayRating is the average stars of the guests who left a review today,
// and false when nobody has.
func (h *History) DayRating() (float32, bool) {
	if h == nil || h.ReviewsToday.N == 0 {
		return 0, false
	}
	return h.ReviewsToday.Stars / float32(h.ReviewsToday.N), true
}

// TopWhy is the moment that set the most of today's reviews below 3★,
// ThoughtNone when there's none to name.
func (h *History) TopWhy() ai.ThoughtKind {
	if h == nil {
		return ai.ThoughtNone
	}
	best, n := ai.ThoughtNone, 0
	for k := ai.ThoughtKind(1); int(k) < ai.ThoughtKindCount; k++ {
		if h.ReviewsToday.Why[k] > n {
			best, n = k, h.ReviewsToday.Why[k]
		}
	}
	return best
}

// RecordRevenue adds amount to the in-progress revenue counters. Safe to
// call when h is nil — does nothing.
func (h *History) RecordRevenue(kind RevenueKind, amount int) {
	if h == nil {
		return
	}
	h.RevenueToday += amount
	h.RevenueByKindToday[kind] += amount
}

// RecordThought increments ThoughtCountsToday for one kind. Called at the
// moment a thought is emitted (not at departure) so the daily sample
// reflects the day thoughts actually occurred. Safe to call when h is nil.
func (h *History) RecordThought(kind ai.ThoughtKind) {
	if h == nil {
		return
	}
	h.ThoughtCountsToday[kind]++
}

// RecordFall adds one fall to the day's. Safe to call when h is nil.
func (h *History) RecordFall(f FallRecord) {
	if h == nil {
		return
	}
	h.FallsToday = append(h.FallsToday, f)
}

// FallReport sums the day's falls for the patrol report.
type FallReport struct {
	Total     int
	OffRun    int        // off any run
	Unloading int        // getting off a lift
	Runs      []RunFalls // per run, most first
}

// RunFalls is one run's falls.
type RunFalls struct {
	TrailID uint64
	Count   int
}

// FallReport sums FallsToday.
func (h *History) FallReport() FallReport {
	var r FallReport
	if h == nil {
		return r
	}
	r.Total = len(h.FallsToday)
	byRun := map[uint64]int{}
	for _, f := range h.FallsToday {
		switch {
		case f.LiftID != 0:
			r.Unloading++
		case f.TrailID == 0:
			r.OffRun++
		default:
			byRun[f.TrailID]++
		}
	}
	for id, n := range byRun {
		r.Runs = append(r.Runs, RunFalls{TrailID: id, Count: n})
	}
	slices.SortFunc(r.Runs, func(a, b RunFalls) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return cmp.Compare(a.TrailID, b.TrailID)
	})
	return r
}

// Push writes one finalised DailySample into the ring and resets the
// per-day counters. Caller has already populated sample.ArrivalsToday /
// sample.DeparturesToday from h.ArrivalsToday / h.DeparturesToday (or
// can leave them zero and let Push read them, but explicit is clearer).
func (h *History) Push(sample DailySample) {
	if h == nil {
		return
	}
	h.Samples[h.Head] = sample
	h.Head = (h.Head + 1) % HistoryCapacity
	if h.Head == 0 {
		h.Filled = true
	}
	h.ArrivalsToday = 0
	h.DeparturesToday = 0
	h.RevenueToday = 0
	h.RevenueByKindToday = [RevenueKindCount]int{}
	h.ThoughtCountsToday = [ai.ThoughtKindCount]int{}
	h.DepartReasonsToday = [ai.DepartReasonCount]int{}
	h.ReviewsToday = ReviewTally{}
	h.FallsToday = nil
}

// Ordered returns the samples in chronological order (oldest first).
// Result is a fresh slice; the caller can iterate freely without
// worrying about ring-head bookkeeping. Empty pre-first-Push.
func (h *History) Ordered() []DailySample {
	if h == nil {
		return nil
	}
	if !h.Filled {
		return append([]DailySample{}, h.Samples[:h.Head]...)
	}
	out := make([]DailySample, 0, HistoryCapacity)
	out = append(out, h.Samples[h.Head:]...)
	out = append(out, h.Samples[:h.Head]...)
	return out
}

// Len returns how many samples have been recorded so far (≤ HistoryCapacity).
func (h *History) Len() int {
	if h == nil {
		return 0
	}
	if h.Filled {
		return HistoryCapacity
	}
	return h.Head
}
