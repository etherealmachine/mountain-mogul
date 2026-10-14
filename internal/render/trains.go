//go:build !demo

package render

import (
	"math"

	"mountain-mogul/internal/world"
)

// Trains drawn (world.RailLine.Train): each line's train, while it's on
// the map, as its engine and coaches along the track, pitched with the
// grade, the engine leading whichever way it's going (a high-speed set
// has a power car at each end). Not in the demo (trains_demo.go).

type trainDrawing struct {
	trainBatches map[string]*Batch
	trainInsts   map[string][]DynamicInstance
	modelDir     string
}

func loadTrains(r *Renderer, modelDir string) {
	r.trainBatches = map[string]*Batch{}
	r.trainInsts = map[string][]DynamicInstance{}
	r.modelDir = modelDir
}

func (r *Renderer) trainBatch(name string) *Batch {
	if b, ok := r.trainBatches[name]; ok {
		return b
	}
	mesh, tex := LoadOBJ(r.modelDir + name + ".obj")
	b := NewDynamicBatch(mesh, tex)
	r.trainBatches[name] = b
	return b
}

// drawTrains draws every train on the map.
func drawTrains(r *Renderer, w *world.World) {
	if r.trainBatches == nil {
		return
	}
	for k := range r.trainInsts {
		r.trainInsts[k] = r.trainInsts[k][:0]
	}
	for _, l := range w.RailLines {
		if l.Train.State == world.TrainAway || len(l.Track) < 2 {
			continue
		}
		spec := l.Style.Spec()
		// The cars from the leading end back: engine, coaches, and a
		// second power car at a high-speed set's tail.
		type car struct {
			model string
			len   float32
			flip  bool // faces backward
		}
		cars := []car{{spec.Engine, spec.EngineLen, false}}
		for i := 0; i < spec.Coaches; i++ {
			cars = append(cars, car{spec.Coach, spec.CoachLen, false})
		}
		if spec.BothEnds {
			cars = append(cars, car{spec.Engine, spec.EngineLen, true})
		}
		// Inbound the train runs toward the station (along the line),
		// trailing back toward the edge; outbound it runs back, trailing
		// toward the station.
		dir := float32(-1)
		if l.Train.State == world.TrainOutbound {
			dir = 1
		}
		at := l.Train.Front
		for _, c := range cars {
			mid := at + dir*c.len/2
			at += dir * c.len
			if mid < 0 || mid > l.Length() || l.InTunnel(mid) {
				continue // still off the map, past the buffers, or in a tunnel
			}
			pos, y, heading, pitch := l.At(mid)
			// Facing back along the line when running out (and a tail
			// power car the other way).
			back := dir > 0
			if c.flip {
				back = !back
			}
			if back {
				heading += math.Pi
				pitch = -pitch
			}
			r.trainInsts[c.model] = append(r.trainInsts[c.model], DynamicInstance{
				Position: [3]float32{pos[0], y, pos[1]},
				Heading:  heading,
				Color:    [3]float32{1, 1, 1},
				Pitch:    pitch,
			})
		}
	}
	for name, insts := range r.trainInsts {
		if len(insts) == 0 {
			continue
		}
		b := r.trainBatch(name)
		b.SetDynamic(insts)
		b.Draw()
	}
}
