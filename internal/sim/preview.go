//go:build !demo

package sim

import "mountain-mogul/internal/world"

// Preview runs a world's lifts and trains empty, without guests or a
// clock, so they can be watched in the scenario editor: every cable
// lift's chairs go round at its speed, and each station with a service
// has a train in from its entry, through its stops and out, the next
// setting off as soon as it's gone (the other way, on a through route).
type Preview struct {
	w   *world.World
	now float64
	// running is each station's train on its way.
	running map[uint64]*world.Train
}

// NewPreview starts a preview of world w.
func NewPreview(w *world.World) *Preview {
	return &Preview{w: w, running: map[uint64]*world.Train{}}
}

// Tick runs the preview on dt seconds.
func (p *Preview) Tick(dt float64) {
	dt = min(dt, maxWallDtSec)
	p.now += dt
	for _, l := range p.w.Lifts {
		if l.Cableless() {
			continue
		}
		loop := l.LoopLength()
		if loop < 1 {
			continue
		}
		for i := range l.Chairs {
			c := &l.Chairs[i]
			c.Progress += l.SpeedAt(c.Progress) * float32(dt) / loop
			for c.Progress >= 1 {
				c.Progress--
			}
		}
	}
	w := p.w
	for _, st := range w.RailStations {
		if p.running[st.ID] != nil {
			continue
		}
		route, style, _ := w.StationRoute(st, st.Flip)
		st.Flip = !st.Flip
		if route == nil {
			continue
		}
		spec := style.Spec()
		tr := &world.Train{Route: route, Style: style, Speed: spec.Speed, Seats: spec.Capacity}
		p.running[st.ID] = tr
		w.Trains = append(w.Trains, tr)
	}
	kept := w.Trains[:0]
	for _, tr := range w.Trains {
		if runTrain(tr, float32(dt), p.now, func(*world.RailStation) {}) {
			kept = append(kept, tr)
		}
	}
	w.Trains = kept
	for id, tr := range p.running {
		if !trainOn(w, tr) {
			delete(p.running, id)
		}
	}
}

// Stop ends the preview: the trains go.
func (p *Preview) Stop() {
	p.w.Trains = nil
}

func trainOn(w *world.World, tr *world.Train) bool {
	for _, t := range w.Trains {
		if t == tr {
			return true
		}
	}
	return false
}
