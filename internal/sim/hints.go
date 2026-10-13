package sim

import (
	"fmt"

	"mountain-mogul/internal/world"
)

// Setup hints: a light tutorial in the notifications. While something a
// resort needs before guests can ski is missing (parking, a ticket window,
// a lift, runs off a lift's top), or a groomed run has no cat to groom it,
// they say so, briefly: when the game starts and then once a game day
// until it's sorted.

// setupHint is one thing a resort is missing.
type setupHint struct {
	key string // what's missing, for posting it once a day
	msg string
	at  *[2]float32 // where to look, if anywhere
}

// setupHints is what the resort is missing now, in the order to build it.
func setupHints(w *world.World) []setupHint {
	var out []setupHint
	lifts := 0
	for _, l := range w.Lifts {
		lifts++
		if w.TerrainForLift(l.ID) == 0 {
			top := [2]float32{l.Top[0], l.Top[1]}
			out = append(out, setupHint{
				key: fmt.Sprintf("runs:%d", l.ID),
				msg: fmt.Sprintf("No runs off %s", l.Name),
				at:  &top,
			})
		}
	}
	if lifts == 0 {
		out = append([]setupHint{{key: "lifts",
			msg: "No lifts"}}, out...)
	}
	if !hasTicketOffice(w) {
		out = append(out, setupHint{key: "tickets",
			msg: "No ticket window"})
	}
	parking := false
	for _, b := range w.Buildings {
		if b.Type == world.BuildingParking {
			parking = true
			break
		}
	}
	if !parking {
		out = append(out, setupHint{key: "parking",
			msg: "No parking"})
	}
	return out
}

// logSetupHints posts each setup hint that holds and hasn't been posted
// today, and takes down the ones that no longer hold. Called when the sim
// starts, every clock hour, and by the scene often while the player
// builds (RefreshSetupHints). They're posted last first, so the feed
// (newest at the top) lists them in build order.
func (s *Simulation) logSetupHints() {
	if s.hintsPosted == nil {
		s.hintsPosted = map[string]int{}
	}
	day := dayIndex(s.SimTime) + 1 // 0 means never
	hints := append(setupHints(s.World), s.groomHints()...)
	hints = append(s.bankruptcyHint(), hints...)
	// A hint that no longer holds comes down, wherever it is in the feed.
	holds := make(map[string]bool, len(hints))
	keys := make(map[string]bool, len(hints))
	for _, h := range hints {
		holds[h.msg], keys[h.key] = true, true
	}
	s.World.Events.RemoveIf(func(e world.Event) bool { return e.Kind == world.EventHint && !holds[e.Message] })
	for k := range s.hintsPosted {
		if !keys[k] {
			delete(s.hintsPosted, k) // so it's posted again at once if it comes back
		}
	}
	for i := len(hints) - 1; i >= 0; i-- {
		h := hints[i]
		if s.hintsPosted[h.key] == day {
			continue
		}
		s.hintsPosted[h.key] = day
		if h.at != nil {
			s.World.LogEventAt(world.EventHint, s.SimTime, h.msg, *h.at, 0)
		} else {
			s.World.LogEvent(world.EventHint, s.SimTime, h.msg)
		}
	}
}

// RefreshSetupHints checks the setup hints now, paused or not: the scene
// calls it while the player builds, so a hint comes down as soon as what
// it asked for is built.
func (s *Simulation) RefreshSetupHints() { s.logSetupHints() }

// groomHints is the groomed runs no cat can groom: one hint when there are
// no active snowcats at all, else one for each run no cat has passes on
// (under trees, or too tight to drive).
func (s *Simulation) groomHints() []setupHint {
	if s.sectionsStale || s.groomCovered == nil {
		s.reassignAllSections()
		s.sectionsStale = false
	}
	var groomed []*world.Trail
	for _, t := range s.World.Trails {
		if t.Groomed && len(t.Cells) > 0 {
			groomed = append(groomed, t)
		}
	}
	if len(groomed) == 0 {
		return nil
	}
	if s.groomCats == 0 {
		return []setupHint{{key: "cats", msg: "No snowcats to groom runs"}}
	}
	var out []setupHint
	for _, t := range groomed {
		if s.groomCovered[t.ID] {
			continue
		}
		var at *[2]float32
		if len(t.Nodes) > 0 {
			p := [2]float32{t.Nodes[0].Pos[0], t.Nodes[0].Pos[1]}
			at = &p
		}
		out = append(out, setupHint{key: fmt.Sprintf("groom:%d", t.ID), msg: t.Name + " can't be groomed", at: at})
	}
	return out
}

// bankruptcyHint is the days left before bankruptcy, while cash is below
// the credit floor: one notification, replaced each day as it counts down.
func (s *Simulation) bankruptcyHint() []setupHint {
	w := s.World
	if w.Bankrupt || w.Cash >= -w.CreditLimit {
		return nil
	}
	left := max(world.BankruptcyGraceDays-w.DaysBelowFloor, 1)
	msg := fmt.Sprintf("%d days to bankruptcy", left)
	if left == 1 {
		msg = "1 day to bankruptcy"
	}
	return []setupHint{{key: msg, msg: msg}}
}
