//go:build !demo

package render

import (
	"math"

	"mountain-mogul/internal/world"
)

// Trains drawn (world.Train): each train on its way, while it's on the
// map, as its engine and coaches along its route, pitched with the grade,
// the engine leading (a high-speed set has a power car at each end). Not in the demo (trains_demo.go).

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
	for _, tr := range w.Trains {
		spec := tr.Style.Spec()
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
		// Running along its route, trailing back along it.
		at := tr.Front
		for _, c := range cars {
			mid := at - c.len/2
			at -= c.len
			if mid < 0 || mid > tr.Route.Length {
				continue // beyond the map
			}
			pos, y, heading, pitch, tunnel := tr.Route.At(mid)
			if tunnel {
				continue
			}
			if c.flip {
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
