package world

import "testing"

func TestEventLogOrderAndWrap(t *testing.T) {
	var l EventLog
	if l.Len() != 0 || len(l.Recent(5)) != 0 {
		t.Fatalf("zero-value log not empty: len=%d", l.Len())
	}

	total := EventLogCapacity + 10
	for i := 0; i < total; i++ {
		l.Push(Event{SimTime: float64(i)})
	}
	if l.Len() != EventLogCapacity {
		t.Fatalf("Len = %d, want %d", l.Len(), EventLogCapacity)
	}
	if l.Seq != uint64(total) {
		t.Fatalf("Seq = %d, want %d", l.Seq, total)
	}
	// Oldest retained is the 11th push; newest is the last.
	if got := l.At(0).SimTime; got != 10 {
		t.Fatalf("At(0).SimTime = %v, want 10", got)
	}
	if got := l.At(l.Len() - 1).SimTime; got != float64(total-1) {
		t.Fatalf("At(last).SimTime = %v, want %d", got, total-1)
	}
	for i := 1; i < l.Len(); i++ {
		if l.At(i).SimTime != l.At(i-1).SimTime+1 {
			t.Fatalf("At(%d) out of order: %v after %v", i, l.At(i).SimTime, l.At(i-1).SimTime)
		}
	}

	recent := l.Recent(3)
	want := []float64{float64(total - 1), float64(total - 2), float64(total - 3)}
	for i, e := range recent {
		if e.SimTime != want[i] {
			t.Fatalf("Recent[%d].SimTime = %v, want %v", i, e.SimTime, want[i])
		}
	}
	if n := len(l.Recent(EventLogCapacity * 2)); n != EventLogCapacity {
		t.Fatalf("Recent(over-cap) returned %d, want %d", n, EventLogCapacity)
	}
}

func TestEventLogPartialFill(t *testing.T) {
	w := NewWorld(NewTerrain(4, 4))
	w.LogEvent(EventDaySummary, 1, "a")
	w.LogEventAt(EventAvalanche, 2, "b", [2]float32{5, 10}, 42)
	if w.Events.Len() != 2 {
		t.Fatalf("Len = %d, want 2", w.Events.Len())
	}
	e := w.Events.At(1)
	if e.Kind != EventAvalanche || !e.HasPos || e.Pos[0] != 5 || e.Pos[1] != 10 || e.EntityID != 42 {
		t.Fatalf("At(1) = %+v", e)
	}
	if w.Events.At(0).HasPos {
		t.Fatalf("LogEvent should not set HasPos")
	}
}
