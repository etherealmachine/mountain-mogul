package sim

import (
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// Boarding: who gets on each chair at the base. A line attendant fills
// every seat from whoever is next, splitting groups and calling singles
// up. Without one, the front of the line gets on with their own group,
// and the next people join them only if their whole group fits and they
// think to (strangerShare); otherwise they wait for the next chair, and
// it leaves with seats empty. Groups bigger than a chair always split.
// Single-rider lanes fill gaps either way: that's what they're for.

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
	sit := func(taken, line []*world.Guest, single bool) bool {
		return lift.Staff.LineAttendant || single || joins(taken, line, seats)
	}
	var taken []*world.Guest
	if len(lift.Lines) > 0 {
		taken = lift.BoardNextPair(seats, sit)
	} else {
		for len(lift.Queue) > 0 && len(taken) < seats && sit(taken, lift.Queue, false) {
			taken = append(taken, lift.Queue[0])
			lift.Queue = lift.Queue[1:]
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
// empty or they're with someone on it; otherwise only if their whole
// group, as it stands in line behind them, fits, and they think to.
func joins(taken, line []*world.Guest, seats int) bool {
	if len(taken) == 0 {
		return true
	}
	p := partyOf(line[0])
	for _, g := range taken {
		if partyOf(g) == p {
			return true
		}
	}
	n := 1
	for n < len(line) && partyOf(line[n]) == p {
		n++
	}
	return n <= seats-len(taken) && rng.Global().Float32() < strangerShare
}

// partyOf is who leads a's group: their leader, or a themselves.
func partyOf(a *world.Guest) *world.Guest {
	if a.Party.Leader != nil {
		return a.Party.Leader
	}
	return a
}
