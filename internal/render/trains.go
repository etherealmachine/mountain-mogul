//go:build !demo

package render

import (
	"math"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Trains drawn (world.Train): each train on its way, while it's on the
// map, as its engine and coaches along its route, pitched with the grade,
// the engine leading (a high-speed set has a power car at each end). Not in the demo (trains_demo.go).

type trainDrawing struct {
	trainBatches map[string]*Batch
	trainInsts   map[string][]DynamicInstance
	modelDir     string
	steam        steamPuffs
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
			if tunnel && inTunnel(tr.Route, mid-c.len/2) && inTunnel(tr.Route, mid+c.len/2) {
				continue // all of it under the hill; a car partly out is drawn
				// whole, the hill and the portal's dark inside hiding the rest
			}
			if c.flip {
				heading += math.Pi
				pitch = -pitch
			}
			if c.model == "steam_loco" {
				r.steam.engine(tr, pos, y, heading, tr.Dwelling || tr.Speed < 0.5)
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

// inTunnel reports whether d metres along route r is in a tunnel.
func inTunnel(r *world.RailRoute, d float32) bool {
	_, _, _, _, tunnel := r.At(d)
	return tunnel
}

// Steam from a steam engine's chimney: puffs let go at its top, every
// metre or so on the move (however fast the game runs) and lazily
// standing, each rising, spreading and thinning where it was let go, so a
// moving train leaves a trail that merges into a plume. The puffs age on
// the game's clock, as the trains move (the wall's in the editor's test
// run, where it stands still), so the plume trails as far at any speed.

const (
	steamLife      = 3.5  // seconds a puff lasts
	steamSpacing   = 1.2  // metres of travel between puffs on the move
	steamEveryIdle = 0.35 // seconds between puffs standing
	steamRise      = 1.8  // metres a second, slowing as it spreads
	steamMax       = 600
)

// steamChimney is the chimney's top in the engine's model: forward, up
// (steam_loco.py).
var steamChimney = [2]float32{3.45, 3.95}

type steamPuff struct {
	pos  mgl32.Vec3
	age  float32
	seed float32
}

type steamPuffs struct {
	puffs []steamPuff
	// engines is the steam engines on the map: where each one's chimney
	// is, where it last let a puff go and the time since, and whether
	// it's standing and was seen this frame.
	engines map[*world.Train]*steamEngine
	last    time.Time
	lastSim float64
	batch   *Batch
	insts   []DynamicInstance
}

type steamEngine struct {
	at, lastPuff   mgl32.Vec3
	since          float32
	standing, seen bool
}

// engine notes train tr's steam engine this frame: its position, rail
// height and heading, and whether it's standing still.
func (s *steamPuffs) engine(tr *world.Train, pos mgl32.Vec2, y, heading float32, standing bool) {
	sn, cs := float32(math.Sin(float64(heading))), float32(math.Cos(float64(heading)))
	at := mgl32.Vec3{pos[0] + sn*steamChimney[0], y + steamChimney[1], pos[1] + cs*steamChimney[0]}
	if s.engines == nil {
		s.engines = map[*world.Train]*steamEngine{}
	}
	e := s.engines[tr]
	if e == nil {
		e = &steamEngine{lastPuff: at}
		s.engines[tr] = e
	}
	e.at, e.standing, e.seen = at, standing, true
}

// drawSteam ages the puffs, lets new ones go from this frame's engines,
// and draws them see-through, thinner as they age, after everything
// solid.
func drawSteam(r *Renderer, w *world.World) {
	s := &r.steam
	now := time.Now()
	dt := float32(0)
	switch {
	case w.SimTime != s.lastSim && s.lastSim != 0:
		dt = float32(min(max(w.SimTime-s.lastSim, 0), 2))
	case !s.last.IsZero() && w.SimTime == s.lastSim:
		dt = min(float32(now.Sub(s.last).Seconds()), 0.1) // the editor: no game clock
	}
	s.last, s.lastSim = now, w.SimTime
	kept := s.puffs[:0]
	for _, p := range s.puffs {
		p.age += dt
		if p.age < steamLife {
			f := p.age / steamLife
			p.pos[1] += steamRise * (1 - 0.6*f) * dt
			p.pos[0] += 0.4 * dt // a breath of wind
			kept = append(kept, p)
		}
	}
	s.puffs = kept
	for tr, e := range s.engines {
		if !e.seen {
			delete(s.engines, tr) // gone, or in a tunnel
			continue
		}
		e.seen = false
		e.since += dt
		seed := func() float32 { return float32(len(s.puffs)%7) / 7 }
		if e.standing {
			if e.since >= steamEveryIdle && len(s.puffs) < steamMax {
				s.puffs = append(s.puffs, steamPuff{pos: e.at, seed: seed()})
				e.since, e.lastPuff = 0, e.at
			}
			continue
		}
		// Along the way from the last puff, one every steamSpacing.
		d := e.at.Sub(e.lastPuff)
		n := int(d.Len() / steamSpacing)
		for k := 1; k <= n && len(s.puffs) < steamMax; k++ {
			f := float32(k) / float32(n)
			s.puffs = append(s.puffs, steamPuff{pos: e.lastPuff.Add(d.Mul(f)), age: (1 - f) * min(e.since, 0.5), seed: seed()})
		}
		if n > 0 {
			e.since, e.lastPuff = 0, e.at
		}
	}
	if len(s.puffs) == 0 {
		return
	}
	if s.batch == nil {
		mesh, tex := LoadOBJ(r.modelDir + "steam_puff.obj")
		s.batch = NewDynamicBatch(mesh, tex)
	}
	r.DynamicShader.Use()
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.DepthMask(false)
	// Four steps of thinning, oldest first so the fresher draw over them.
	for step := 3; step >= 0; step-- {
		s.insts = s.insts[:0]
		for _, p := range s.puffs {
			f := p.age / steamLife
			if min(int(f*4), 3) != step {
				continue
			}
			size := 0.9 + 3.6*float32(math.Pow(float64(f), 0.6)) + 0.4*p.seed
			s.insts = append(s.insts, DynamicInstance{
				Position: [3]float32{p.pos[0], p.pos[1], p.pos[2]},
				Heading:  p.seed * 6.28,
				Color:    [3]float32{1, 1, 1},
				SpinMode: 10 + size, // scaled (dynamic.vert)
			})
		}
		if len(s.insts) == 0 {
			continue
		}
		r.DynamicShader.SetFloat("uFade", 0.45+0.14*float32(step))
		s.batch.SetDynamic(s.insts)
		s.batch.Draw()
	}
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)
	r.DynamicShader.SetFloat("uFade", 0)
}
