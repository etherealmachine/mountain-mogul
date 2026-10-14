//go:build !demo

package scene

import (
	"fmt"

	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// addSnowmakingRows adds a run's snowmaking to its popup: a button to put
// it in, then its switch, target depth, the run's depth, and what it's
// doing (world/snowmaking.go).
func addSnowmakingRows(win *ui.Window, w *world.World, t *world.Trail, h trailHooks) {
	if !world.SnowmakingAvailable() || t.Kind.IsArea() {
		return
	}
	if !t.Snowmaking {
		cost := world.SnowmakingInstallCost(t)
		win.AddActionButton(fmt.Sprintf("Add snowmaking (%s)", settings.FormatDollars(cost)), func() {
			if h.spend != nil && !h.spend(cost) {
				return
			}
			t.Snowmaking, t.SnowmakingOn, t.SnowTarget = true, true, world.DefaultSnowTarget
			if h.reopen != nil {
				h.reopen(false)
			}
		})
		return
	}
	win.AddBoolToggle("Snowmaking", func() bool { return t.SnowmakingOn }, func(v bool) { t.SnowmakingOn = v })
	const step = 0.1
	win.AddIntStepperFn("Target depth", func() string { return settings.FormatDepth(t.SnowTarget) },
		func() { t.SnowTarget = max(t.SnowTarget-step, 0.2) },
		func() { t.SnowTarget = min(t.SnowTarget+step, 3) })
	win.AddLabel("Snow depth", func() string {
		d, low := runSnowDepth(w, t)
		if low == 0 {
			return settings.FormatDepth(d)
		}
		return fmt.Sprintf("%s (%d%% below target)", settings.FormatDepth(d), low)
	})
	if h.snowStatus != nil {
		win.AddLabel("Now", h.snowStatus)
	}
}

// runSnowDepth is a run's average snow depth (metres) and the share of
// its cells under its snowmaking target, percent.
func runSnowDepth(w *world.World, t *world.Trail) (avg float32, lowPct int) {
	if len(t.Cells) == 0 {
		return 0, 0
	}
	var sum float32
	low := 0
	for _, c := range t.Cells {
		if !w.Terrain.InBounds(c[0], c[1]) {
			continue
		}
		d := w.Terrain.Cells[c[0]][c[1]].VisibleSnowDepth()
		sum += d
		if d < t.SnowTarget {
			low++
		}
	}
	return sum / float32(len(t.Cells)), low * 100 / len(t.Cells)
}

// snowmakingStatus says what a run's snowmaking is doing, for the game's
// popup.
func (s *Scenario) snowmakingStatus(t *world.Trail) string {
	switch {
	case !t.SnowmakingOn:
		return "Off"
	case t.MakingSnow:
		return fmt.Sprintf("Making snow (%s at the base)", settings.FormatTemp(s.sim.TempNow()))
	case s.sim.LiftsRunning():
		return "Waits for the lifts to stop"
	}
	if _, low := runSnowDepth(s.world, t); low == 0 {
		return "At target depth"
	}
	return fmt.Sprintf("Too warm (%s at the base; needs %s)",
		settings.FormatTemp(s.sim.TempNow()), settings.FormatTemp(world.SnowmakingMinTempC))
}
