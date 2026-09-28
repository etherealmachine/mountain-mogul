package sim

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// Event-feed emit helpers. Sim-internal sites (avalanche, rescue, lift
// holds, day summary) call these directly; the scene calls the exported
// Log* methods for player actions so every event is stamped with the same
// sim clock.

// LogBuildingPlaced records a player-placed building in the event feed.
func (s *Simulation) LogBuildingPlaced(b *world.Building) {
	s.World.LogEventAt(world.EventBuildPlaced, s.SimTime,
		fmt.Sprintf("Built %s", b.Type.Label()), b.Pos, b.ID)
}

// LogLiftPlaced records a player-placed lift in the event feed. The event
// position is the lift base.
func (s *Simulation) LogLiftPlaced(l *world.Lift) {
	s.World.LogEventAt(world.EventBuildPlaced, s.SimTime,
		fmt.Sprintf("Built %s (%s)", l.Name, l.Type.Label()), l.Base, l.ID)
}

// LogLiftOpenChanged records the player opening or closing a lift. Call
// after flipping l.Open.
func (s *Simulation) LogLiftOpenChanged(l *world.Lift) {
	if l.Open {
		s.World.LogEventAt(world.EventLiftOpened, s.SimTime,
			fmt.Sprintf("%s opened", l.Name), l.Base, l.ID)
	} else {
		s.World.LogEventAt(world.EventLiftClosed, s.SimTime,
			fmt.Sprintf("%s closed", l.Name), l.Base, l.ID)
	}
}

// logLiftHoldChanged records a sim-driven hold transition (no snow at the
// base station). Call after updating l.OnHold.
func (s *Simulation) logLiftHoldChanged(l *world.Lift) {
	if l.OnHold {
		s.World.LogEventAt(world.EventLiftClosed, s.SimTime,
			fmt.Sprintf("%s on hold: no snow at base", l.Name), l.Base, l.ID)
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

// logDaySummary records the end-of-day recap for the sample just pushed.
func (s *Simulation) logDaySummary(d world.DailySample) {
	s.World.LogEvent(world.EventDaySummary, s.SimTime, fmt.Sprintf(
		"%s recap: %d arrivals, $%d in, $%d out",
		d.Day.Format("Jan 2"), d.ArrivalsToday, d.Revenue, d.Costs))
}

// cellCentre converts a terrain cell index to its world XZ in metres.
func cellCentre(cell [2]int) mgl32.Vec2 {
	return mgl32.Vec2{float32(cell[0]) * world.CellSize, float32(cell[1]) * world.CellSize}
}
