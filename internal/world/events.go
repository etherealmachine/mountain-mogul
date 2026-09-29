package world

import "github.com/go-gl/mathgl/mgl32"

// EventLogCapacity is the number of events World.Events retains. Older
// entries are overwritten once the ring is full.
const EventLogCapacity = 256

// EventKind classifies an Event for icon / colour selection in the UI.
// Values are persisted in saves — append new kinds, never renumber.
type EventKind uint8

const (
	EventAvalanche        EventKind = 0 // natural or triggered slab release
	EventRescue           EventKind = 1 // patrol delivered an injured guest to the base
	EventLiftOpened       EventKind = 2 // player opened a lift, or a hold cleared
	EventLiftClosed       EventKind = 3 // player closed a lift, or it went on hold
	EventBuildPlaced      EventKind = 4 // player placed a building or lift
	EventDaySummary       EventKind = 5 // end-of-day recap written at rollover
	EventFinance          EventKind = 6 // interest charged, credit floor crossed, bankruptcy
	EventGuestsTurnedAway EventKind = 7 // guests would have come but couldn't buy a ticket
	EventResortOpened     EventKind = 8 // player opened the resort at a ticket office
	EventResortClosed     EventKind = 9 // player closed the resort at a ticket office
)

// Event is one entry in the World-level event feed: something the player
// would otherwise only learn about by noticing it. Written by the sim (and
// by the scene for player actions); read by the UI event panel.
type Event struct {
	Kind    EventKind
	SimTime float64 // sim seconds at which the event happened
	Message string

	// HasPos marks Pos as meaningful. Pos is world XZ in metres — clicking
	// the event in the UI moves the camera there.
	HasPos bool
	Pos    mgl32.Vec2

	// EntityID is the lift / building / guest the event concerns, or 0.
	EntityID uint64
}

// EventLog is a fixed-capacity ring of Events. The zero value is an empty,
// ready-to-use log, so World needs no constructor work for it.
type EventLog struct {
	entries [EventLogCapacity]Event
	head    int // next write index
	count   int // number of valid entries, ≤ EventLogCapacity

	// Seq counts every Push over the log's lifetime (not reset by
	// wrap-around). The UI compares it against its last-seen value to
	// detect new events without diffing the ring.
	Seq uint64
}

// Push appends e, overwriting the oldest entry when the ring is full.
func (l *EventLog) Push(e Event) {
	l.entries[l.head] = e
	l.head = (l.head + 1) % EventLogCapacity
	if l.count < EventLogCapacity {
		l.count++
	}
	l.Seq++
}

// ScaleTimes multiplies every retained event's SimTime by k (save
// migration between day lengths).
func (l *EventLog) ScaleTimes(k float64) {
	for i := range l.entries {
		l.entries[i].SimTime *= k
	}
}

// Len returns the number of events currently retained.
func (l *EventLog) Len() int { return l.count }

// At returns the i-th retained event, oldest-first (0 = oldest,
// Len()-1 = newest). Panics when i is out of range.
func (l *EventLog) At(i int) Event {
	if i < 0 || i >= l.count {
		panic("EventLog.At: index out of range")
	}
	start := l.head - l.count
	if start < 0 {
		start += EventLogCapacity
	}
	return l.entries[(start+i)%EventLogCapacity]
}

// Recent returns up to n events, newest-first. Allocates; intended for the
// UI (a few dozen rows per frame) and save serialisation.
func (l *EventLog) Recent(n int) []Event {
	if n > l.count {
		n = l.count
	}
	out := make([]Event, n)
	for i := 0; i < n; i++ {
		out[i] = l.At(l.count - 1 - i)
	}
	return out
}

// LogEvent appends an event to the world's feed.
func (w *World) LogEvent(kind EventKind, simTime float64, msg string) {
	w.Events.Push(Event{Kind: kind, SimTime: simTime, Message: msg})
}

// LogEventAt appends an event carrying a world XZ position and entity ID so
// the UI can jump the camera to it.
func (w *World) LogEventAt(kind EventKind, simTime float64, msg string, pos mgl32.Vec2, entityID uint64) {
	w.Events.Push(Event{Kind: kind, SimTime: simTime, Message: msg, HasPos: true, Pos: pos, EntityID: entityID})
}
