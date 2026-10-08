package scene

import (
	"fmt"

	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// The trail popup, shared by the game and the scenario editor. Trails
// are drawn with the run tool (run_tool.go). Each scene hooks in what's
// particular to it (the game reroutes snowcats and guests).

// trailHooks is what a scene does when the trail popup changes things.
// Each may be nil.
type trailHooks struct {
	changed func() // a difficulty, grooming, or name edit
	groomed func() // grooming switched (the game re-plans snowcat sections)
	deleted func() // the trail is gone (after world.DeleteTrail)
	reopen  func(confirm bool)
}

// newTrailWindow builds the trail popup: name, difficulty, grooming,
// shape (length, drop, pitch, what its ends attach to), and Delete (with
// confirmClear, Confirm and Cancel instead). While it's open the run's
// handles show and clicks on it edit it (run_tool.go).
func newTrailWindow(w *world.World, t *world.Trail, confirmClear bool, h trailHooks) *ui.Window {
	call := func(f func()) {
		if f != nil {
			f()
		}
	}
	win := ui.NewWindow(t.Kind.Label(), 0, 0)
	win.AddTextInput("Name", t.Name, func(text string) {
		t.Name = text
		call(h.changed)
	})
	win.AddDifficultyToggles("Difficulty",
		func(bit uint8) bool { return t.Difficulty == world.TerrainDifficulty(bit) },
		func(bit uint8) {
			t.Difficulty = world.TerrainDifficulty(bit)
			w.RebuildTrailGraph()
			call(h.changed)
		},
	)
	if !t.Kind.IsArea() {
		win.AddBoolToggle("Groomed", func() bool { return t.Groomed }, func(v bool) {
			t.Groomed = v
			call(h.groomed)
			call(h.changed)
		})
	}
	win.AddLabel("Shape", func() string { return runSummary(w, t) })
	if !t.Kind.IsArea() {
		win.AddLabel("From", func() string { return runEndLabel(w, t.Start) })
		win.AddLabel("To", func() string { return runEndLabel(w, t.End) })
		if !t.Start.Set || !t.End.Set {
			win.AddLabel("Note", func() string { return "a loose end connects only where the run overlaps something" })
		}
	}
	win.AddLabel("Groom", func() string {
		if len(t.Cells) == 0 {
			return "—"
		}
		var sum float32
		for _, c := range t.Cells {
			sum += w.Terrain.Cells[c[0]][c[1]].Grooming
		}
		return fmt.Sprintf("%.0f%%", sum/float32(len(t.Cells))*100)
	})
	if confirmClear {
		win.AddLabel("Confirm", func() string { return "Delete this trail?" })
		win.AddActionButton("Confirm", func() {
			w.DeleteTrail(t.ID)
			w.RebuildTrailGraph()
			call(h.deleted)
		})
		win.AddActionButton("Cancel", func() {
			if h.reopen != nil {
				h.reopen(false)
			}
		})
	} else {
		win.AddActionButton("Delete", func() {
			if h.reopen != nil {
				h.reopen(true)
			}
		})
	}
	return win
}
