package sim

import (
	"math"

	"mountain-mogul/internal/world"
)

// SetResortOpen flips the resort-wide open/closed switch (World.ResortOpen)
// and logs it to the event feed. Opening takes effect on the next demand
// poll and bills the day at operating cost. Closing stops arrivals at once;
// guests already on the mountain finish what they are doing and go home
// (see tickResortClosed). No-op when the state is unchanged.
func (s *Simulation) SetResortOpen(open bool) {
	w := s.World
	if w.ResortOpen == open {
		return
	}
	w.ResortOpen = open
	if open {
		s.openToday = true
		w.LogEvent(world.EventResortOpened, s.SimTime, "Resort opened")
		return
	}
	w.LogEvent(world.EventResortClosed, s.SimTime, "Resort closed")
	s.sendQueuesHome()
}

// sendQueuesHome empties every lift line; the guests in them head for the
// parking lot. Riders stay on their chairs, unload as usual, and head home
// from the top (onPlanStepStart's closed JoinQueue check).
func (s *Simulation) sendQueuesHome() {
	for _, l := range s.World.Lifts {
		for _, g := range ejectQueue(l) {
			s.directHomePlan(g)
		}
	}
}

// preOpenArrivalHours is how long before OpenHour guests start arriving to
// get in line; the mountain counts as "closed for the day" before that.
const preOpenArrivalHours = 0.5

// ClosedForDay reports whether the mountain is done for the day: the
// resort is closed for the season, or it's past closing time, or it's
// night before the morning's first arrivals. Guests on the mountain head
// home. Before opening (the pre-open window) guests wait in line instead.
func (s *Simulation) ClosedForDay() bool {
	w := s.World
	if !w.ResortOpen {
		return true
	}
	h := float32(HourOfDay(s.SimTime))
	return h >= w.CloseHour || h < w.OpenHour-preOpenArrivalHours
}

// tickResortClosed keeps every on-mountain guest's Patience at zero while
// the mountain is closed for the day, so each replan picks GoHome over
// more skiing. Riding and skiing regenerate Patience each tick, hence
// doing this every tick rather than once at closing. At closing time the
// lift lines are sent home.
func (s *Simulation) tickResortClosed() {
	closed := s.ClosedForDay()
	if closed && !s.closedForDay && s.World.ResortOpen {
		s.sendQueuesHome()
	}
	s.closedForDay = closed
	if !closed {
		return
	}
	for _, g := range s.World.OnMountain {
		g.Patience = 0
	}
}

// arrivalTimeConstHours shapes the arrival curve: arrivals peak as the
// pre-open window starts and decay with this time constant, so most of
// the day's guests are on the hill by late morning.
const arrivalTimeConstHours = 1.5

// arrivalLastHourBeforeClose stops arrivals this long before CloseHour —
// nobody drives up for the last hour.
const arrivalLastHourBeforeClose = 1.0

// arrivalShare is the fraction of the day's arrivals that land between
// clock hours h0 and h1 (same day): an exponential decay from
// OpenHour − preOpenArrivalHours to CloseHour − arrivalLastHourBeforeClose,
// normalised to 1 over the day.
func arrivalShare(w *world.World, h0, h1 float64) float64 {
	a := float64(w.OpenHour) - preOpenArrivalHours
	b := float64(w.CloseHour) - arrivalLastHourBeforeClose
	if b <= a {
		b = float64(w.CloseHour)
	}
	if b <= a {
		return 0
	}
	cdf := func(h float64) float64 {
		h = math.Max(a, math.Min(b, h))
		return (1 - math.Exp(-(h-a)/arrivalTimeConstHours)) / (1 - math.Exp(-(b-a)/arrivalTimeConstHours))
	}
	return cdf(h1) - cdf(h0)
}

// ejectQueue empties a lift's queue (and lines, for lifts with loading
// lines) and clears each ejected guest's plan so they replan next tick.
// Returns the ejected guests.
func ejectQueue(l *world.Lift) []*world.Guest {
	ejected := append([]*world.Guest(nil), l.Queue...)
	l.Queue = l.Queue[:0]
	if len(l.Lines) > 0 {
		ejected = append(ejected, l.EjectLinesGuests()...)
	}
	for _, g := range ejected {
		g.Queued = false
		g.Plan.Steps = nil
	}
	return ejected
}
