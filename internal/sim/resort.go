package sim

import "mountain-mogul/internal/world"

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
	// Guests standing in line leave it and head for the parking lot.
	// Riders stay on their chairs, unload as usual, and head home from
	// the top (onPlanStepStart's closed-resort JoinQueue check).
	for _, l := range w.Lifts {
		for _, g := range ejectQueue(l) {
			s.directHomePlan(g)
		}
	}
}

// tickResortClosed keeps every on-mountain guest's Patience at zero while
// the resort is closed, so each replan picks GoHome over more skiing.
// Riding and skiing regenerate Patience each tick, hence doing this every
// tick rather than once at closing.
func (s *Simulation) tickResortClosed() {
	if s.World.ResortOpen {
		return
	}
	for _, g := range s.World.OnMountain {
		g.Patience = 0
	}
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
