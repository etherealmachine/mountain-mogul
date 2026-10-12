package scene

import (
	"fmt"

	"mountain-mogul/internal/sim"
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
	reach := newGroomReach(w, t)
	win.AddLabel("Groom", func() string {
		cells := reach.cells()
		if len(cells) == 0 {
			return "—"
		}
		var sum float32
		for _, c := range cells {
			sum += w.Terrain.Cells[c[0]][c[1]].Grooming
		}
		return fmt.Sprintf("%.0f%%", sum/float32(len(cells))*100)
	})
	win.AddLabel("Trees", func() string {
		reach.cells()
		if reach.missed == 0 {
			return "none in the way"
		}
		return fmt.Sprintf("%.0f%% under trees: cats won't groom", float32(reach.missed)/float32(len(t.Cells))*100)
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

// groomReach caches the cells of a trail its cats can groom
// (sim.GroomReach), replanned when the trail's cells, trees, or snow
// cover change. The popup's Groom share counts only these: cells under
// trees stay natural snow and would hold it below 100% for good.
type groomReach struct {
	w      *world.World
	t      *world.Trail
	key    [3]int
	reach  [][2]int
	missed int // snow-covered cells out of reach
}

func newGroomReach(w *world.World, t *world.Trail) *groomReach {
	return &groomReach{w: w, t: t, key: [3]int{-1}}
}

func (g *groomReach) cells() [][2]int {
	key := [3]int{len(g.t.Cells)}
	snow := 0
	for i, c := range g.t.Cells {
		cell := &g.w.Terrain.Cells[c[0]][c[1]]
		key[1] += int(cell.TreeCount)
		key[2] += (c[0]*31 + c[1]) * (i + 1)
		if cell.TopLayer() != nil {
			snow++
		}
	}
	key[1] = key[1]<<20 | snow
	if key != g.key {
		g.key = key
		g.reach = sim.GroomReach(g.w.Terrain, g.t)
		g.missed = max(snow-len(g.reach), 0)
	}
	return g.reach
}
