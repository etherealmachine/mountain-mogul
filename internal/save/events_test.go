package save

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

func TestEventsRoundTrip(t *testing.T) {
	var src world.EventLog
	src.Push(world.Event{Kind: world.EventDaySummary, SimTime: 240, Message: "recap"})
	src.Push(world.Event{Kind: world.EventAvalanche, SimTime: 300, Message: "avy",
		HasPos: true, Pos: mgl32.Vec2{15, 25}, EntityID: 9})

	var dst world.EventLog
	eventsFromData(&dst, eventsToData(&src))
	if dst.Len() != src.Len() {
		t.Fatalf("Len = %d, want %d", dst.Len(), src.Len())
	}
	for i := 0; i < src.Len(); i++ {
		if dst.At(i) != src.At(i) {
			t.Fatalf("event %d: got %+v, want %+v", i, dst.At(i), src.At(i))
		}
	}

	// Saves without the field load an empty feed; an empty feed saves as nil.
	var empty world.EventLog
	eventsFromData(&empty, nil)
	if empty.Len() != 0 || eventsToData(&empty) != nil {
		t.Fatalf("empty round trip not empty")
	}
}
