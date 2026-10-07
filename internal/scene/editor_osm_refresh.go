package scene

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mountain-mogul/internal/geo"
	"mountain-mogul/internal/world"
)

// osmRefreshJob fetches the map's OpenStreetMap features again in the
// background (the Layers panel's Refresh on the OpenStreetMap row), for
// imports made before a feature was fetched (the ski-area boundary) or
// when the map's been updated. The ground is untouched.
type osmRefreshJob struct {
	base *world.TerrainBase
	mu   sync.Mutex
	m    *geo.OSMMap
	err  error
	done bool
}

func (e *Editor) requestOSMRefresh() {
	base := e.world.TerrainBase
	if base == nil || e.layers.osmRefresh != nil {
		return
	}
	j := &osmRefreshJob{base: base}
	e.layers.osmRefresh = j
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		m, err := geo.RefreshOSM(ctx, base)
		j.mu.Lock()
		j.m, j.err, j.done = m, err, true
		j.mu.Unlock()
	}()
}

// pollOSMRefresh applies a finished refresh, unless the map was replaced
// while it ran.
func (e *Editor) pollOSMRefresh() {
	j := e.layers.osmRefresh
	if j == nil {
		return
	}
	j.mu.Lock()
	m, err, done := j.m, j.err, j.done
	j.mu.Unlock()
	if !done {
		return
	}
	e.layers.osmRefresh = nil
	switch {
	case e.world.TerrainBase != j.base:
	case err != nil:
		e.setToast("OpenStreetMap refresh failed: " + err.Error())
	default:
		geo.ApplyOSM(j.base, m)
		e.osm.built = time.Time{} // redrape the overlay
		e.markDirty()
		e.setToast(fmt.Sprintf("OpenStreetMap refreshed: %s; %d ski-area outline(s).", osmCounts(j.base), len(j.base.Areas)))
	}
}
