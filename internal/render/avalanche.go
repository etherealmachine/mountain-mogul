package render

import (
	"math"
	"math/rand"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Avalanches drawn: the blocks of their debris (world.Cell's Debris; the
// terrain shader draws the smaller stuff between), the powder cloud over
// a slide while it runs (world.AvyFront), and avalanche control's work:
// a mortar round's flash and smoke, its flight and its blast on the
// start zone (world.AvyShots, AvyBursts), and the crack of a ski cut.
// Everything moves on the game's clock, as the slides do.

type avalancheDrawing struct {
	blockBatch, puffBatch *Batch

	// The debris blocks, by tile of debrisTile cells, worked out again
	// when the snow there changes (FlushSnowCells, FlushSnowState).
	terrain   *world.Terrain
	tiles     map[[2]int][]DynamicInstance
	stale     map[[2]int]bool
	staleAll  bool
	blocks    []DynamicInstance
	blocksOld bool // blocks needs gathering from tiles

	puffs      []avyPuff
	lastSim    float64
	spawnDebt  float32
	seenShots  map[float64]bool // by Fired
	seenBursts map[float64]bool // by At
	insts      []DynamicInstance
	rng        *rand.Rand
}

// avyPuff is one billow of cloud, smoke or flash.
type avyPuff struct {
	pos, vel     mgl32.Vec3
	age, life    float32
	size0, size1 float32
	color        [3]float32
	alpha        float32 // at its thickest, 0..1
	flash        bool    // drawn unlit: a muzzle flash or blast
}

const (
	debrisTile = 32
	// Debris blocks: one per debrisBlockDepth metres of debris in a cell
	// (a fraction by chance), up to debrisBlocksMax a cell.
	debrisBlockDepth = float32(0.25)
	debrisBlocksMax  = 5
	avyPuffsMax      = 2500
)

func loadAvalanches(r *Renderer, modelDir string) {
	a := &r.avy
	mesh, tex := LoadOBJ(modelDir + "snow_block.obj")
	a.blockBatch = NewDynamicBatch(mesh, tex)
	mesh, tex = LoadOBJ(modelDir + "avy_puff.obj")
	a.puffBatch = NewDynamicBatch(mesh, tex)
	a.rng = rand.New(rand.NewSource(1))
}

// markDebrisStale notes that the snow on cells [x0, x1] × [z0, z1] changed.
func (a *avalancheDrawing) markDebrisStale(x0, z0, x1, z1 int) {
	if a.stale == nil {
		a.stale = map[[2]int]bool{}
	}
	for tz := z0 / debrisTile; tz <= z1/debrisTile; tz++ {
		for tx := x0 / debrisTile; tx <= x1/debrisTile; tx++ {
			a.stale[[2]int{tx, tz}] = true
		}
	}
}

// refreshBlocks works out the debris blocks again where the snow changed.
func (a *avalancheDrawing) refreshBlocks(t *world.Terrain) {
	if t != a.terrain {
		a.terrain, a.tiles, a.staleAll = t, map[[2]int][]DynamicInstance{}, true
	} else if a.staleAll && len(a.tiles) == 0 {
		// The whole map's snow changed, but there was no debris to move:
		// debris only comes from slides, which mark their own tiles.
		a.staleAll = false
	}
	if a.staleAll {
		a.staleAll, a.stale = false, nil
		clear(a.tiles)
		for tz := 0; tz*debrisTile < t.Height; tz++ {
			for tx := 0; tx*debrisTile < t.Width; tx++ {
				a.tileBlocks(t, tx, tz)
			}
		}
		a.blocksOld = true
	}
	if len(a.stale) > 0 {
		for k := range a.stale {
			a.tileBlocks(t, k[0], k[1])
		}
		clear(a.stale)
		a.blocksOld = true
	}
	if a.blocksOld {
		a.blocksOld = false
		a.blocks = a.blocks[:0]
		for _, b := range a.tiles {
			a.blocks = append(a.blocks, b...)
		}
	}
}

// tileBlocks scatters the blocks on tile (tx, tz)'s debris: the same
// blocks for the same debris, each turned, tilted and sized by a hash of
// its cell, and sunk part way into the snow.
func (a *avalancheDrawing) tileBlocks(t *world.Terrain, tx, tz int) {
	key := [2]int{tx, tz}
	var out []DynamicInstance
	for x := tx * debrisTile; x < min((tx+1)*debrisTile, t.Width-1); x++ {
		for z := tz * debrisTile; z < min((tz+1)*debrisTile, t.Height-1); z++ {
			c := &t.Cells[x][z]
			if c.Debris <= 0 {
				continue
			}
			d := c.DebrisDepth()
			want := d / debrisBlockDepth
			n := int(want)
			if cellHash(x, z, 99) < want-float32(n) {
				n++
			}
			n = min(n, debrisBlocksMax)
			for i := range n {
				h := func(k int) float32 { return cellHash(x, z, i*7+k) }
				px := (float32(x) + h(1)) * world.CellSize
				pz := (float32(z) + h(2)) * world.CellSize
				size := (0.7 + 1.4*h(3)*h(3)) * min(0.5+d, 1.5)
				y := VisualElevationAt(t, px, pz) - size*(0.1+0.2*h(4))
				tone := 0.95 + 0.08*h(5)
				out = append(out, DynamicInstance{
					Position: [3]float32{px, y, pz},
					Heading:  h(6) * 2 * math.Pi,
					Pitch:    (h(7) - 0.5) * 0.7,
					Color:    [3]float32{tone, tone, tone * 1.02},
					SpinMode: 10 + size,
				})
			}
		}
	}
	if len(out) == 0 {
		delete(a.tiles, key)
	} else {
		a.tiles[key] = out
	}
}

// cellHash is a fixed pseudo-random 0..1 for cell (x, z) and salt k.
func cellHash(x, z, k int) float32 {
	h := uint32(x)*73856093 ^ uint32(z)*19349663 ^ uint32(k)*83492791
	h ^= h >> 13
	h *= 0x5bd1e995
	h ^= h >> 15
	return float32(h&0xffffff) / float32(0x1000000)
}

// drawDebrisBlocks draws the debris blocks with the solid things.
func drawDebrisBlocks(r *Renderer, w *world.World) {
	a := &r.avy
	if a.blockBatch == nil || w.Terrain == nil {
		return
	}
	a.refreshBlocks(w.Terrain)
	a.blockBatch.SetDynamic(a.blocks)
	if len(a.blocks) > 0 {
		r.DynamicShader.Use()
		a.blockBatch.Draw()
	}
}

// drawAvalancheEffects moves and draws the cloud, smoke and flashes,
// see-through, after everything solid.
func drawAvalancheEffects(r *Renderer, w *world.World) {
	a := &r.avy
	if a.puffBatch == nil {
		return
	}
	dt := float32(0)
	if a.lastSim != 0 {
		dt = float32(min(max(w.SimTime-a.lastSim, 0), 2))
	}
	a.lastSim = w.SimTime
	a.spawnCloud(w, dt)
	a.spawnControl(w)
	a.age(dt)
	a.shellFlights(w)
	if len(a.puffs) == 0 && len(a.insts) == 0 {
		return
	}

	r.DynamicShader.Use()
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.DepthMask(false)
	// Shells are solid; drawn first, from shellFlights.
	if len(a.insts) > 0 {
		a.puffBatch.SetDynamic(a.insts)
		a.puffBatch.Draw()
	}
	// Four steps of thinning, the thinnest first; flashes unlit.
	for _, flash := range []bool{false, true} {
		if flash {
			r.DynamicShader.SetFloat("uEmissive", 1)
		}
		for step := 3; step >= 0; step-- {
			a.insts = a.insts[:0]
			for _, p := range a.puffs {
				if p.flash != flash {
					continue
				}
				f := p.age / p.life
				// Thickest a fifth of the way through, then thinning.
				thick := p.alpha * min(f*5, 1) * (1 - f)
				if min(int((1-thick)*4), 3) != step {
					continue
				}
				size := p.size0 + (p.size1-p.size0)*float32(math.Sqrt(float64(f)))
				a.insts = append(a.insts, DynamicInstance{
					Position: [3]float32{p.pos[0], p.pos[1], p.pos[2]},
					Heading:  float32(int(p.size1*97)%628) / 100,
					Color:    p.color,
					SpinMode: 10 + size,
				})
			}
			if len(a.insts) == 0 {
				continue
			}
			r.DynamicShader.SetFloat("uFade", 0.25+0.2*float32(step))
			a.puffBatch.SetDynamic(a.insts)
			a.puffBatch.Draw()
		}
	}
	a.insts = a.insts[:0]
	r.DynamicShader.SetFloat("uEmissive", 0)
	r.DynamicShader.SetFloat("uFade", 0)
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)
}

// spawnCloud raises the powder cloud off a running slide: billows from
// the front cells, more where more snow is running and faster, rolling on
// the way it goes and rising as they spread.
func (a *avalancheDrawing) spawnCloud(w *world.World, dt float32) {
	t := w.Terrain
	if dt <= 0 || t == nil || len(w.AvyFront) == 0 {
		return
	}
	var total float32
	for _, c := range w.AvyFront {
		total += t.Cells[c[0]][c[1]].AvySnow
	}
	if total <= 0 {
		return
	}
	// About 60 billows a second of game time for a big slide's front.
	a.spawnDebt += dt * min(20+total*40, 80)
	for a.spawnDebt >= 1 && len(a.puffs) < avyPuffsMax {
		a.spawnDebt--
		// A front cell, by its share of the snow.
		pick := a.rng.Float32() * total
		var cell [2]int
		for _, c := range w.AvyFront {
			pick -= t.Cells[c[0]][c[1]].AvySnow
			cell = c
			if pick <= 0 {
				break
			}
		}
		c := &t.Cells[cell[0]][cell[1]]
		dir := mgl32.Vec2{c.AvyDirX, c.AvyDirZ}
		if l := dir.Len(); l > 0 {
			dir = dir.Mul(1 / l)
		}
		speed := 4 + 6*min(c.AvyMomentum/8, 1)
		x := (float32(cell[0]) + a.rng.Float32()) * world.CellSize
		z := (float32(cell[1]) + a.rng.Float32()) * world.CellSize
		y := VisualElevationAt(t, x, z) + 0.5
		size := 2 + 3*a.rng.Float32()
		a.puffs = append(a.puffs, avyPuff{
			pos:   mgl32.Vec3{x, y, z},
			vel:   mgl32.Vec3{dir[0] * speed, 1.5 + 2*a.rng.Float32(), dir[1] * speed},
			life:  20 + 15*a.rng.Float32(),
			size0: size, size1: size * (3 + 2*a.rng.Float32()),
			color: [3]float32{1, 1, 1},
			alpha: 0.75,
		})
	}
	a.spawnDebt = min(a.spawnDebt, 5)
}

// spawnControl sets off the flash and smoke of each round fired, and the
// blast or crack of each burst, the first frame they're seen.
func (a *avalancheDrawing) spawnControl(w *world.World) {
	if a.seenShots == nil {
		a.seenShots, a.seenBursts = map[float64]bool{}, map[float64]bool{}
	}
	for _, sh := range w.AvyShots {
		if a.seenShots[sh.Fired] || w.SimTime-sh.Fired > 5 {
			continue
		}
		a.seenShots[sh.Fired] = true
		up := sh.To.Sub(sh.From)
		up[1] = 0
		if l := up.Len(); l > 0 {
			up = up.Mul(1 / l)
		}
		a.puffs = append(a.puffs, avyPuff{
			pos: sh.From.Add(up.Mul(2)), life: 2, size0: 3, size1: 6,
			color: [3]float32{3, 1.8, 0.7}, alpha: 1, flash: true,
		})
		for range 14 {
			a.puffs = append(a.puffs, avyPuff{
				pos:  sh.From.Add(up.Mul(1 + 4*a.rng.Float32())).Add(mgl32.Vec3{0, 1, 0}),
				vel:  mgl32.Vec3{up[0]*3 + 2*a.rng.Float32() - 1, 1.5 + 1.5*a.rng.Float32(), up[2]*3 + 2*a.rng.Float32() - 1},
				life: 25 + 15*a.rng.Float32(), size0: 2, size1: 9 + 5*a.rng.Float32(),
				color: [3]float32{0.62, 0.62, 0.64}, alpha: 0.75,
			})
		}
	}
	for _, b := range w.AvyBursts {
		if a.seenBursts[b.At] || w.SimTime-b.At > 5 {
			continue
		}
		a.seenBursts[b.At] = true
		if b.Blast {
			a.puffs = append(a.puffs, avyPuff{
				pos: b.Pos.Add(mgl32.Vec3{0, 3, 0}), life: 2.5, size0: 6, size1: 16,
				color: [3]float32{3, 1.7, 0.6}, alpha: 1, flash: true,
			})
			for i := range 36 {
				ang := float64(i) / 36 * 2 * math.Pi
				out := mgl32.Vec3{float32(math.Cos(ang)), 0, float32(math.Sin(ang))}
				grey := float32(1)
				if i%4 == 0 {
					grey = 0.45 // smoke among the snow thrown up
				}
				a.puffs = append(a.puffs, avyPuff{
					pos:  b.Pos.Add(out.Mul(2)).Add(mgl32.Vec3{0, 1, 0}),
					vel:  out.Mul(5 + 6*a.rng.Float32()).Add(mgl32.Vec3{0, 6 + 8*a.rng.Float32(), 0}),
					life: 25 + 15*a.rng.Float32(), size0: 3, size1: 12 + 8*a.rng.Float32(),
					color: [3]float32{grey, grey, grey * 1.02}, alpha: 0.8,
				})
			}
			continue
		}
		// A ski cut: the slab cracking round the patroller.
		for i := range 10 {
			off := mgl32.Vec3{(a.rng.Float32() - 0.5) * 24, 0.5, (a.rng.Float32() - 0.5) * 24}
			a.puffs = append(a.puffs, avyPuff{
				pos: b.Pos.Add(off), vel: mgl32.Vec3{0, 1 + a.rng.Float32(), 0},
				life: 12 + 8*a.rng.Float32(), size0: 2, size1: 5 + 3*float32(i%3),
				color: [3]float32{1, 1, 1}, alpha: 0.7,
			})
		}
	}
	for k := range a.seenShots {
		if w.SimTime-k > 30 || k > w.SimTime {
			delete(a.seenShots, k)
		}
	}
	for k := range a.seenBursts {
		if w.SimTime-k > 30 || k > w.SimTime {
			delete(a.seenBursts, k)
		}
	}
}

// age moves the puffs on dt game seconds: they drift, slow, rise and go.
func (a *avalancheDrawing) age(dt float32) {
	kept := a.puffs[:0]
	for _, p := range a.puffs {
		p.age += dt
		if p.age >= p.life {
			continue
		}
		p.pos = p.pos.Add(p.vel.Mul(dt))
		drag := float32(math.Exp(float64(-0.25 * dt)))
		p.vel[0] *= drag
		p.vel[2] *= drag
		p.vel[1] = p.vel[1]*drag + 0.3*dt // the cloud's warm air lifts it
		kept = append(kept, p)
	}
	a.puffs = kept
}

// shellFlights puts each round in the air on its arc, dark against the
// snow, with a thin trail of smoke.
func (a *avalancheDrawing) shellFlights(w *world.World) {
	a.insts = a.insts[:0]
	for _, sh := range w.AvyShots {
		if w.SimTime < sh.Fired || w.SimTime >= sh.Lands {
			continue
		}
		pos := shellAt(sh, w.SimTime)
		a.insts = append(a.insts, DynamicInstance{
			Position: [3]float32{pos[0], pos[1], pos[2]},
			Color:    [3]float32{0.12, 0.12, 0.13},
			SpinMode: 10 + 1.5, // larger than life, to be seen
		})
		if len(a.puffs) < avyPuffsMax {
			a.puffs = append(a.puffs, avyPuff{
				pos: pos, life: 8, size0: 1.2, size1: 4,
				color: [3]float32{0.85, 0.85, 0.87}, alpha: 0.55,
			})
		}
	}
}

// shellAt is where round sh is at sim time now: a lob from the gun to the
// start zone, highest halfway.
func shellAt(sh world.AvyShot, now float64) mgl32.Vec3 {
	f := float32((now - sh.Fired) / (sh.Lands - sh.Fired))
	apex := 30 + 0.2*sh.To.Sub(sh.From).Len()
	p := sh.From.Add(sh.To.Sub(sh.From).Mul(f))
	p[1] += apex * 4 * f * (1 - f)
	return p
}
