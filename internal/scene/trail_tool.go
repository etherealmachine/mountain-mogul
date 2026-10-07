package scene

import (
	"fmt"

	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// Trail painting, shared by the game and the scenario editor: the paint
// stroke, the cell-overlay colours, and the trail popup. Each scene keeps
// its own tool state and hooks in what's particular to it (the game
// reroutes snowcats and guests).

// paintTrail adds the cells under the trail brush at (gx, gz) to trail
// id, or removes them when erase is set.
func paintTrail(w *world.World, id uint64, gx, gz int, erase bool) {
	if id == 0 {
		return
	}
	cells := world.BrushCells(gx, gz, trailPaintBrushRadius)
	if erase {
		w.RemoveTrailCells(id, cells)
		return
	}
	var valid [][2]int
	for _, c := range cells {
		if w.Terrain.InBounds(c[0], c[1]) {
			valid = append(valid, c)
		}
	}
	if len(valid) > 0 {
		w.AddTrailCells(id, valid)
	}
}

// trailColor is a trail difficulty's overlay colour.
func trailColor(d world.TerrainDifficulty) (r, g, b uint8) {
	switch d {
	case world.DiffGreen:
		return 55, 160, 55
	case world.DiffBlue:
		return 55, 120, 210
	}
	return 60, 60, 60 // black
}

// drawTrailOverlay paints trails into a cell overlay with set: every
// trail when showAll, and the one being edited (activeID) always,
// brighter. Groomed trails get a light wash.
func drawTrailOverlay(trails []*world.Trail, activeID uint64, showAll bool, set func(cx, cz int, r, g, b, a uint8)) {
	for _, t := range trails {
		active := t.ID == activeID
		if !showAll && !active {
			continue
		}
		r, g, b := trailColor(t.Difficulty)
		a := uint8(130)
		if active {
			a = 200
		}
		for _, c := range t.Cells {
			set(c[0], c[1], r, g, b, a)
		}
		if t.Groomed && !active {
			for _, c := range t.Cells {
				set(c[0], c[1], 255, 255, 255, 40)
			}
		}
	}
}

// trailHooks is what a scene does when the trail popup changes things.
// Each may be nil.
type trailHooks struct {
	changed func()           // a difficulty, grooming, or name edit
	groomed func()           // grooming switched (the game re-plans snowcat sections)
	edit    func(erase bool) // Add or Remove: start painting this trail
	deleted func()           // the trail is gone (after world.DeleteTrail)
	reopen  func(confirm bool)
}

// newTrailWindow builds the trail popup: name, difficulty, grooming,
// size, Add and Remove cells, and Clear (with confirmClear, Confirm and
// Cancel instead).
func newTrailWindow(w *world.World, t *world.Trail, confirmClear bool, h trailHooks) *ui.Window {
	call := func(f func()) {
		if f != nil {
			f()
		}
	}
	win := ui.NewWindow("Trail", 0, 0)
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
	win.AddBoolToggle("Groomed", func() bool { return t.Groomed }, func(v bool) {
		t.Groomed = v
		call(h.groomed)
		call(h.changed)
	})
	win.AddLabel("Cells", func() string { return fmt.Sprintf("%d", len(t.Cells)) })
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
	win.AddActionButton("Add", func() {
		if h.edit != nil {
			h.edit(false)
		}
	})
	win.AddActionButton("Remove", func() {
		if h.edit != nil {
			h.edit(true)
		}
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
		win.AddActionButton("Clear", func() {
			if h.reopen != nil {
				h.reopen(true)
			}
		})
	}
	return win
}
