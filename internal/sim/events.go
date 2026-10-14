package sim

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Event-feed emit helpers. The feed carries what the player would
// otherwise miss: things the sim did (avalanches, rescues, lift holds),
// what guests think, and the day's results. It doesn't echo the player's
// own actions (building, opening a lift).

// logLiftHoldChanged records a sim-driven hold transition (no snow at the
// base station). Call after updating l.OnHold.
func (s *Simulation) logLiftHoldChanged(l *world.Lift) {
	if l.SnowHold {
		s.World.LogEventAt(world.EventLiftClosed, s.SimTime,
			fmt.Sprintf("%s on hold: too little snow at its base", l.Name), l.Base, l.ID)
	} else {
		s.World.LogEventAt(world.EventLiftOpened, s.SimTime,
			fmt.Sprintf("%s hold lifted", l.Name), l.Base, l.ID)
	}
}

// logAvalanches records n slab releases from one stability check as a
// single event positioned at the first release cell.
func (s *Simulation) logAvalanches(n int, cell [2]int) {
	if n <= 0 {
		return
	}
	msg := "Avalanche released"
	if n > 1 {
		msg = fmt.Sprintf("%d avalanches released", n)
	}
	s.World.LogEventAt(world.EventAvalanche, s.SimTime, msg, cellCentre(cell), 0)
}

// logDaySummary records the end-of-day recap for the sample just pushed,
// with the moment behind most of the day's reviews below 3★.
func (s *Simulation) logDaySummary(d world.DailySample, why ai.ThoughtKind) {
	if !d.Open {
		return // nothing to sum up on a day the resort stayed shut
	}
	msg := fmt.Sprintf("%s: %d visitors, %.1f stars, $%s in, $%s out",
		world.FormatGameDate(d.Day, false), d.ArrivalsToday, d.Rating, world.CommaInt(d.Revenue), world.CommaInt(d.Costs))
	if why != ai.ThoughtNone {
		msg += "; most heard: " + ai.ThoughtLabel[why]
	}
	s.World.LogEvent(world.EventDaySummary, s.SimTime, msg)
}

// guestsSayMin is how many of today's reviews a complaint must have set
// before the feed reports it.
const guestsSayMin = 5

// logGuestsSay reports, once a clock hour, the complaint behind most of
// today's reviews below 3★ when it differs from the last one reported,
// so the player hears what guests are unhappy about while it's happening.
func (s *Simulation) logGuestsSay() {
	h := s.World.History
	why := h.TopWhy()
	if why == ai.ThoughtNone || why == s.lastSaid || h.ReviewsToday.Why[why] < guestsSayMin {
		return
	}
	s.lastSaid = why
	s.World.LogEvent(world.EventGuestsSay, s.SimTime, fmt.Sprintf("Guests are saying %s (%d of %d today)",
		ai.ThoughtLabel[why], h.ReviewsToday.Why[why], h.ReviewsToday.N))
}

// cellCentre converts a terrain cell index to its world XZ in metres.
func cellCentre(cell [2]int) mgl32.Vec2 {
	return mgl32.Vec2{float32(cell[0]) * world.CellSize, float32(cell[1]) * world.CellSize}
}

// longLineReport is the wait at which a lift's line makes the feed.
const longLineReport = world.LongLineWait

// logLongLines reports, once a lift a day, a running lift whose line has
// reached a long wait, with what would help if it has no attendant.
func (s *Simulation) logLongLines() {
	if s.longLinesSaid == nil {
		s.longLinesSaid = map[uint64]bool{}
	}
	for _, l := range s.World.Lifts {
		if !l.InService(s.World) || s.longLinesSaid[l.ID] {
			continue
		}
		wait := l.LineWait()
		if wait < longLineReport {
			continue
		}
		s.longLinesSaid[l.ID] = true
		mins := int(wait / (world.SimSecondsPerHour / 60))
		msg := fmt.Sprintf("%s has a %d-minute line", l.Name, mins)
		if !l.Cableless() && !l.Staff.LineAttendant {
			msg += "; a line attendant would fill its chairs"
		}
		s.World.LogEventAt(world.EventLongLine, s.SimTime, msg, l.Base, l.ID)
	}
}

// freshSnowReport is the new snow, metres of water, worth a feed line:
// about 8 cm of fresh snow.
const freshSnowReport = 0.008

// logFreshSnow reports the night's new snow, the day's powder.
func (s *Simulation) logFreshSnow(dw DayWeather) {
	if dw.AccumSWE < freshSnowReport {
		return
	}
	cm := dw.AccumSWE * 1000 // fresh snow is about a tenth water
	s.World.LogEvent(world.EventSnowfall, s.SimTime, fmt.Sprintf(
		"Fresh snow: about %.0f cm (%.0f in) fell overnight", cm, cm/2.54))
}

// logRecordDay reports a day with more visitors than any before it, once
// there are a few days to beat.
func (s *Simulation) logRecordDay(d world.DailySample) {
	days := s.World.History.Ordered()
	if len(days) < 3 || d.ArrivalsToday == 0 {
		return
	}
	for _, p := range days[:len(days)-1] {
		if p.ArrivalsToday >= d.ArrivalsToday {
			return
		}
	}
	s.World.LogEvent(world.EventRecord, s.SimTime, fmt.Sprintf(
		"Record day: %d visitors on %s", d.ArrivalsToday, world.FormatGameDate(d.Day, false)))
}

// logRescues reports the last hour's patrol rescues as one line, placed
// where the latest ended, rather than a line for each.
func (s *Simulation) logRescues() {
	n := s.rescues.n
	if n == 0 {
		return
	}
	s.rescues.n = 0
	msg := "Patrol brought an injured guest down in the last hour"
	if n > 1 {
		msg = fmt.Sprintf("Patrol brought %d injured guests down in the last hour", n)
	}
	s.World.LogEventAt(world.EventRescue, s.SimTime, msg, s.rescues.last, 0)
}

// logCutOff reports the last hour's guests cut off by open water as one
// line, placed where the latest was.
func (s *Simulation) logCutOff() {
	n := s.cutOff.n
	if n == 0 {
		return
	}
	s.cutOff.n = 0
	msg := "A guest was cut off by open water and is waiting for patrol; a footpath across makes a bridge"
	if n > 1 {
		msg = fmt.Sprintf("%d guests were cut off by open water in the last hour; a footpath across makes a bridge", n)
	}
	s.World.LogEventAt(world.EventCutOff, s.SimTime, msg, s.cutOff.last, 0)
}
