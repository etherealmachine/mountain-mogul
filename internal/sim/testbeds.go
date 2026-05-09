package sim

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Testbed is a small, named world that drives the in-game testbed picker
// and the headless `-testbed` CLI runner. The CLI matches a user-supplied
// prefix against Name (case-insensitive), so names should start with the
// distinguishing detail — "10 degree slope, intermediate skier" lets a
// user write `-testbed "10 degree slope"`.
type Testbed struct {
	Name  string
	Build func(rng *rand.Rand) *world.World
	Seed  int64
}

// Testbeds is the registry surfaced by the start-menu testbed picker and
// by the headless runner. Append new entries here.
var Testbeds = []Testbed{
	{
		Name: "Flat plane, beginner skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(60, 40).flat(100).lodgeAt(30, 20).skierAt(1, 20, ai.SkillBeginner).build()
		},
	},
	{
		Name: "10 degree slope, intermediate skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(40, 60).slope(10).lodge().skier(ai.SkillIntermediate).build()
		},
	},
	{
		Name: "15 degree slope, intermediate skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(40, 60).slope(15).lodge().skier(ai.SkillIntermediate).build()
		},
	},
	{
		Name: "20 degree slope, advanced skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(40, 60).slope(20).lodge().skier(ai.SkillAdvanced).build()
		},
	},
	{
		// 18° upper section transitioning to a ~3° flat runout. Tests the
		// transition from linked turns into point-and-shoot on flats.
		Name: "Slope with flat runout, advanced skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(40, 80).runout(50, 18, 3).lodge().skier(ai.SkillAdvanced).build()
		},
	},
	{
		// Dense circular grove on the line between skier and lodge. The
		// grove is wider than the skier's forward probe cone, so they
		// must commit to a side rather than thread through.
		Name: "15 degree slope with tree patch, intermediate skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(40, 60).slope(15).lodge().skier(ai.SkillIntermediate).
				treePatch(20, 30, 6, 0.8).build()
		},
	},
	{
		// Trail divergence: side walls channel the skier, a center patch
		// blocks the straight line, and the skier spawns 1 cell east of
		// the corridor centerline. This puts the skier next to a ~130 m
		// "easy" gap on the right (5 m of drift to clear the patch's east
		// edge) and a ~40 m "hard" gap on the left (~125 m of drift to
		// reach). Used to test whether skiers DELIBERATELY choose paths or
		// merely avoid the nearest hazard.
		//
		// Layout (40 cells wide × 50 tall):
		//   walls at x=[3,4]   (world  30–50)
		//   walls at x=[35,36] (world 350–370)
		//   patch at (cx=15, cz=25, r=6) → x=[90,220), z=[190,320)
		//   skier at (gx=21, gz=1) → world (215, 5), 1 cell east of axis
		//   lodge at (gx=21, gz=48)
		Name: "Trail diverge, offset skier east, intermediate",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(40, 50).slope(15).
				lodgeAt(21, 48).
				treeRect(3, 5, 4, 45, 0.8).    // left wall
				treeRect(35, 5, 36, 45, 0.8).  // right wall
				treePatch(15, 25, 6, 0.8).     // center obstacle (off-axis west)
				skierAt(21, 1, ai.SkillIntermediate).
				build()
		},
	},
}

// FindTestbed returns the testbed whose Name starts with `prefix`
// (case-insensitive). Returns an error if zero or more than one testbed
// matches, with the candidate list embedded in the message.
func FindTestbed(prefix string) (*Testbed, error) {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return nil, fmt.Errorf("testbed: empty name")
	}
	matches := make([]int, 0, 2)
	for i := range Testbeds {
		if strings.HasPrefix(strings.ToLower(Testbeds[i].Name), prefix) {
			matches = append(matches, i)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("testbed: no match for %q (known: %s)", prefix, strings.Join(testbedNames(), ", "))
	case 1:
		return &Testbeds[matches[0]], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = Testbeds[m].Name
		}
		return nil, fmt.Errorf("testbed: %q matches multiple — %s", prefix, strings.Join(names, " | "))
	}
}

func testbedNames() []string {
	out := make([]string, len(Testbeds))
	for i, t := range Testbeds {
		out[i] = t.Name
	}
	return out
}

// =============================================================================
// Fluent builder
// =============================================================================

// builder collects the steps of a testbed scene. Methods return the
// receiver so calls can be chained: scene(40, 60).slope(15).lodge().skier(...).
//
// Convention: terrain-shaping methods (flat / slope / runout) come first,
// then lodge() (which subsequent skier() calls auto-target), then any
// number of skier() and obstacle steps. Calling build() returns the
// finished *world.World.
type builder struct {
	w         *world.World
	lastLodge *world.Building // most recently placed lodge; auto-target for skier()
}

// scene starts a fresh testbed builder over a wCells × hCells terrain
// (each cell is 10 m on a side). The terrain is initialised flat at
// elevation 0; call flat / slope / runout to shape it.
func scene(wCells, hCells int) *builder {
	t := world.NewTerrain(wCells, hCells)
	return &builder{w: world.NewWorld(t)}
}

// flat fills every cell to the given elevation, full snow, passable.
func (b *builder) flat(elev float32) *builder {
	t := b.w.Terrain
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			t.Cells[x][z].Elevation = elev
			t.Cells[x][z].SnowDepth = 1.0
			t.Cells[x][z].Passable = true
		}
	}
	return b
}

// slope tilts the terrain at slopeDeg degrees in +z (top of grid is high,
// bottom of grid is low).
func (b *builder) slope(slopeDeg float64) *builder {
	t := b.w.Terrain
	rate := float32(math.Tan(slopeDeg * math.Pi / 180))
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			t.Cells[x][z].Elevation = float32(t.Height-z) * 10.0 * rate
			t.Cells[x][z].SnowDepth = 1.0
			t.Cells[x][z].Passable = true
		}
	}
	return b
}

// runout shapes a steep upper section (z < upperEndZ) joining a gentler
// runout below. Both angles in degrees.
func (b *builder) runout(upperEndZ int, upperDeg, runoutDeg float64) *builder {
	t := b.w.Terrain
	upperRate := float32(math.Tan(upperDeg * math.Pi / 180))
	runoutRate := float32(math.Tan(runoutDeg * math.Pi / 180))
	// Build elevations from bottom upward so the join elevation is consistent.
	for z := t.Height - 1; z >= 0; z-- {
		var elev float32
		if z >= upperEndZ {
			elev = float32(t.Height-z) * 10.0 * runoutRate
		} else {
			joinElev := float32(t.Height-upperEndZ) * 10.0 * runoutRate
			elev = joinElev + float32(upperEndZ-z)*10.0*upperRate
		}
		for x := 0; x < t.Width; x++ {
			t.Cells[x][z].Elevation = elev
			t.Cells[x][z].SnowDepth = 1.0
			t.Cells[x][z].Passable = true
		}
	}
	return b
}

// lodge places a target lodge at the bottom centre of the grid (z = H-2,
// x = W/2) with spawning disabled. Subsequent skier() calls auto-target it.
func (b *builder) lodge() *builder {
	t := b.w.Terrain
	return b.lodgeAt(t.Width/2, t.Height-2)
}

// lodgeAt is the explicit form of lodge — caller picks the cell. The
// continuous Pos lands at the cell centre so the lodge sits in the
// historical position from before continuous coords landed.
func (b *builder) lodgeAt(gx, gz int) *builder {
	const cellSize = 10.0
	lodge := b.w.PlaceBuilding((float32(gx)+0.5)*cellSize, (float32(gz)+0.5)*cellSize)
	lodge.SkierCount = 0
	lodge.MeanSpawnRate = 0
	b.lastLodge = lodge
	return b
}

// skier spawns a single skier at the top centre of the grid (z = 1,
// x = W/2) targeting the most recently placed lodge.
func (b *builder) skier(skill ai.SkillLevel) *builder {
	t := b.w.Terrain
	return b.skierAt(t.Width/2, 1, skill)
}

// skierAt is the explicit form of skier — caller picks the cell. The skier
// is positioned at the CELL CENTER (gx+0.5, gz+0.5)*cellSize rather than the
// cell edge. With cell-edge spawning the skier-lodge axis lies on a cell
// boundary, so any tiny lateral drift flips the strategic L/R probes
// between adjacent cells with very different patch coverage — the symmetric
// tree-patch testbed showed an 80%+ left bias that was actually quantization
// asymmetry, not a behavioural bias. Cell-center spawn aligns the axis with
// the patch's world center (also cell-center based) so drift up to ½ cell
// either direction stays in the same probe-cell pair.
func (b *builder) skierAt(gx, gz int, skill ai.SkillLevel) *builder {
	if b.lastLodge == nil {
		panic("skierAt: no lodge placed; call lodge() or lodgeAt() first")
	}
	const cellSize = 10.0
	elev := b.w.Terrain.ElevationAt(gx, gz)
	pos := mgl32.Vec3{(float32(gx) + 0.5) * cellSize, elev, (float32(gz) + 0.5) * cellSize}

	// Lodge target — continuous Pos already in world metres.
	lx := b.lastLodge.Pos[0]
	lz := b.lastLodge.Pos[1]
	heading := float32(math.Atan2(float64(lx-pos[0]), float64(lz-pos[2])))

	a := &world.Agent{
		ID:         b.w.NextID(),
		Pos:        pos,
		Heading:    heading,
		TargetID:   b.lastLodge.ID,
		Traits:     ai.TraitsFor(skill),
		Balance:    1.0,
		Confidence: spawnConfidence,
		Energy:     1.0,
	}
	b.w.Agents = append(b.w.Agents, a)
	return b
}

// treePatch stamps a circular grove of the given density at (cx, cz). The
// density value is set absolutely on cells inside the circle — existing
// values are overwritten rather than added to.
func (b *builder) treePatch(cx, cz, radius int, density float32) *builder {
	t := b.w.Terrain
	for x := cx - radius; x <= cx+radius; x++ {
		for z := cz - radius; z <= cz+radius; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			dx := x - cx
			dz := z - cz
			if dx*dx+dz*dz > radius*radius {
				continue
			}
			t.Cells[x][z].TreeDensity = density
		}
	}
	return b
}

// treeRect stamps a rectangular grove [x1, x2] × [z1, z2] (inclusive) at
// the given density. Used for trail walls / corridors. Density is set
// absolutely.
func (b *builder) treeRect(x1, z1, x2, z2 int, density float32) *builder {
	t := b.w.Terrain
	for x := x1; x <= x2; x++ {
		for z := z1; z <= z2; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			t.Cells[x][z].TreeDensity = density
		}
	}
	return b
}

// build returns the finished *world.World. The builder should not be used
// after this call.
func (b *builder) build() *world.World {
	return b.w
}
