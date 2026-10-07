package sim

import (
	"math"
	"mountain-mogul/internal/ai"

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
			s.setDepartReason(g, ai.DepartClosing)
			s.directHomePlan(g)
		}
	}
}

// preOpenArrivalHours is how long before OpenHour guests start arriving to
// get in line, the eagerest of them (world.RollArrivalOffset); the
// mountain counts as "closed for the day" before that.
const preOpenArrivalHours = 3.0

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

// tickResortClosed tells the planner whether the mountain is closed for
// the day (World.ClosedForDay), which makes GoHome outrank every other
// goal; guests' stats are left alone, so the trip home still costs them.
// At closing time the lift lines are sent home.
func (s *Simulation) tickResortClosed() {
	closed := s.ClosedForDay()
	if closed && !s.closedForDay && s.World.ResortOpen {
		s.sendQueuesHome()
	}
	s.closedForDay = closed
	s.World.ClosedForDay = closed
}

// arrivalLastHourBeforeClose stops arrivals this long before CloseHour —
// nobody drives up for the last hour.
const arrivalLastHourBeforeClose = 1.0

// arrivalWindow is the clock hours guests arrive in: from
// OpenHour − preOpenArrivalHours to CloseHour − arrivalLastHourBeforeClose.
// ok is false when the hours leave no window.
func arrivalWindow(w *world.World) (a, b float64, ok bool) {
	a = float64(w.OpenHour) - preOpenArrivalHours
	b = float64(w.CloseHour) - arrivalLastHourBeforeClose
	if b <= a {
		b = float64(w.CloseHour)
	}
	return a, b, b > a
}

// arrivalSpreadHours is how far a guest's arrival strays from the time
// they like to arrive (the standard deviation).
const arrivalSpreadHours = 1.0 / 3

// arrivalShare is the fraction of guest g's arrivals on a day they come
// that land between clock hours h0 and h1: a normal spread around their
// preferred time (OpenHour + ArrivalOffset), kept inside the arrival
// window and normalised to 1 over it.
func arrivalShare(w *world.World, g *world.Guest, h0, h1 float64) float64 {
	a, b, ok := arrivalWindow(w)
	if !ok {
		return 0
	}
	mean := float64(w.OpenHour + g.ArrivalOffset)
	cdf := func(h float64) float64 {
		h = math.Max(a, math.Min(b, h))
		return 0.5 * (1 + math.Erf((h-mean)/(arrivalSpreadHours*math.Sqrt2)))
	}
	total := cdf(b) - cdf(a)
	if total <= 0 {
		return 0
	}
	return (cdf(h1) - cdf(h0)) / total
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
