package world

import "time"

// HistoryCapacity is the ring-buffer length, in days. 376 ≈ 2 ski
// seasons (~188 days each), matching Planet Coaster's "last two years"
// graph window. When deeper history is wanted later, the plan is to
// downsample older entries into monthly buckets rather than grow this.
const HistoryCapacity = 376

// DailySample is one row in the History ring. Each in-game day rollover
// pushes one of these; the readers iterate via History.Ordered to walk
// them oldest-first regardless of where the ring head currently sits.
type DailySample struct {
	Day              time.Time // calendar date this sample covers
	GuestsOnMountain int       // active OnMountain count at EOD
	ArrivalsToday    int       // spawns during this day
	DeparturesToday  int       // departures during this day
	Cash             int       // resort cash balance at EOD
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
	ArrivalsToday   int
	DeparturesToday int
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

// RecordDeparture bumps the in-progress departures counter. Safe to
// call when h is nil — does nothing.
func (h *History) RecordDeparture() {
	if h == nil {
		return
	}
	h.DeparturesToday++
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
