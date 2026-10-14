package sim

import (
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// Boarding: who gets on each chair at the base. A line attendant fills
// every seat from whoever is next, splitting groups and calling singles
// up. Without one, the front of the line gets on with their own group
// (from wherever they stand in it), and the next people join them only if their whole group fits and they
// think to (strangerShare); otherwise they wait for the next chair, and
// it leaves with seats empty. Groups bigger than a chair always split.

// strangerShare is how often, with no line attendant, the group next in
// line shares a chair with strangers when they'd all fit. Each further
// group is another roll, so a quad of singles rarely fills: on Boreal,
// unattended quads leave about three quarters full with a line behind.
const strangerShare = 0.9

// fillSmoothing is how much each busy chair moves Lift.Fill.
const fillSmoothing = 0.05

// loadChair takes up to seats guests off lift's line for the chair at the
// base, and updates the lift's Fill when people are left waiting.
func loadChair(lift *world.Lift, seats int) []*world.Guest {
	var taken []*world.Guest
	// A tram's cabin takes the line as it comes: everyone crowds in.
	for len(lift.Queue) > 0 && len(taken) < seats && (lift.Staff.LineAttendant || lift.Type.Tram() || joins(taken, lift.Queue, seats)) {
		g := lift.Queue[0]
		lift.LeaveLine(g)
		taken = append(taken, g)
		// The rest of their group in line gets on with them.
		for i := 0; i < len(lift.Queue) && len(taken) < seats; {
			if q := lift.Queue[i]; q.PartyHead() == g.PartyHead() {
				lift.LeaveLine(q)
				taken = append(taken, q)
				continue
			}
			i++
		}
	}
	if len(taken) > 0 && lift.QueueLen() > 0 {
		fill := float32(len(taken)) / float32(seats)
		if lift.Fill == 0 {
			lift.Fill = fill
		} else {
			lift.Fill += (fill - lift.Fill) * fillSmoothing
		}
	}
	return taken
}

// joins reports whether line's first guest gets on a chair of seats with
// taken already on it, with no line attendant: always when the chair's
// empty or they're with someone on it; otherwise only if the rest of
// their group standing right behind them fits too, and they think to.
// (Members further back come along if there's room: loadChair.)
func joins(taken, line []*world.Guest, seats int) bool {
	if len(taken) == 0 {
		return true
	}
	p := line[0].PartyHead()
	for _, g := range taken {
		if g.PartyHead() == p {
			return true
		}
	}
	n := 1
	for n < len(line) && line[n].PartyHead() == p {
		n++
	}
	return n <= seats-len(taken) && rng.Global().Float32() < strangerShare
}

// crowdSlip is how often, when a place opens in a crowd, someone other
// than whoever came first gets there first.
const crowdSlip = 0.1

// cutInsNoticed is how many times a guest is passed in one wait before
// it annoys them (once).
const cutInsNoticed = 2

// shuffleLine moves lift's crowd up into places opening ahead, and
// annoys whoever is passed cutInsNoticed times in a wait.
func (s *Simulation) shuffleLine(lift *world.Lift) {
	slip := func() bool { return rng.Global().Float32() < crowdSlip }
	for _, g := range lift.ShuffleCrowd(slip) {
		if g.LinePassed++; g.LinePassed == cutInsNoticed {
			s.applyEvent(g, ai.ThoughtCutIn, lift.ID)
		}
	}
}
