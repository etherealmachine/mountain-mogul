package scene

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/sim"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// eventPanelMaxRows caps how many recent events the panel converts per
// frame; more than fit on any realistic screen.
const eventPanelMaxRows = 48

// eventKindTint is the marker colour for each world.EventKind in the panel.
func eventKindTint(k world.EventKind) mgl32.Vec4 {
	switch k {
	case world.EventAvalanche:
		return mgl32.Vec4{0.95, 0.40, 0.10, 1}
	case world.EventRescue:
		return mgl32.Vec4{0.90, 0.25, 0.25, 1}
	case world.EventLiftOpened, world.EventResortOpened:
		return mgl32.Vec4{0.30, 0.85, 0.45, 1}
	case world.EventLiftClosed, world.EventResortClosed:
		return mgl32.Vec4{0.93, 0.80, 0.08, 1}
	case world.EventBuildPlaced:
		return mgl32.Vec4{0.40, 0.60, 0.95, 1}
	case world.EventFinance:
		return mgl32.Vec4{1.00, 0.82, 0.20, 1} // coin gold, matches the top-bar cash icon
	case world.EventGuestsTurnedAway:
		return mgl32.Vec4{0.90, 0.45, 0.35, 1}
	}
	return mgl32.Vec4{0.65, 0.70, 0.80, 1} // day summary and unknown kinds
}

// eventRows converts the world's recent events into panel rows, newest-first.
func eventRows(w *world.World) []ui.EventRow {
	events := w.Events.Recent(eventPanelMaxRows)
	rows := make([]ui.EventRow, len(events))
	for i, e := range events {
		rows[i] = ui.EventRow{
			When:     sim.DateAt(w.StartDate, e.SimTime).Format("Jan 2, 2006"),
			Text:     e.Message,
			Tint:     eventKindTint(e.Kind),
			Jumpable: e.HasPos,
			X:        e.Pos[0],
			Z:        e.Pos[1],
		}
	}
	return rows
}
