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
//
// Grooming convention: the surface starts ungroomed; each testbed paints
// the lanes the skier should prefer with groomRect. For tree-free slopes
// that's the centre-third column from top to bottom. For testbeds with
// trees the groomed strips go *around* the obstacle so the skier has to
// pick a side and commit to a corduroy lane.
var Testbeds = []Testbed{
	{
		Name: "Flat plane, beginner skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(60, 40).flat(100).
				groomRect(0, 13, 59, 26). // centre third along z (trail runs x: 1→30 at z=20)
				lodgeAt(30, 20).skierAt(1, 20, ai.SkillBeginner).build()
		},
	},
	{
		Name: "10 degree slope, intermediate skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(40, 60).slope(10).
				groomRect(13, 0, 26, 59). // centre third of slope width
				lodge().skier(ai.SkillIntermediate).build()
		},
	},
	{
		Name: "15 degree slope, intermediate skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(40, 60).slope(15).
				groomRect(13, 0, 26, 59).
				lodge().skier(ai.SkillIntermediate).build()
		},
	},
	{
		Name: "20 degree slope, advanced skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(40, 60).slope(20).
				groomRect(13, 0, 26, 59).
				lodge().skier(ai.SkillAdvanced).build()
		},
	},
	{
		// 18° upper section transitioning to a ~3° flat runout. Tests the
		// transition from linked turns into point-and-shoot on flats.
		Name: "Slope with flat runout, advanced skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			return scene(40, 80).runout(50, 18, 3).
				groomRect(13, 0, 26, 79).
				lodge().skier(ai.SkillAdvanced).build()
		},
	},
	{
		// Dense circular grove on the line between skier and lodge. The
		// grove is wider than the skier's forward probe cone, so they
		// must commit to a side rather than thread through.
		//
		// Grooming is two continuous Y-branch corridors (one per side)
		// painted as polygons: each covers the centre stem, curves out
		// around the tree patch as a narrower arm, and curves back into
		// the centre stem. The two branches overlap in the stems so the
		// trail reads as a single lane that splits and rejoins.
		//
		// Tree patch: (cx=20, cz=30, r=6) covers x=[14,26], z=[24,36].
		Name: "15 degree slope with tree patch, intermediate skier",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			westBranch := [][2]float32{
				// Outer (west) boundary: top-left down through arm to bottom-left.
				{13, 0}, {11, 20}, {5, 24}, {5, 36}, {11, 40}, {13, 59},
				// Bottom edge across to the right.
				{26, 59},
				// Inner (east) boundary: bottom-right up around the patch back to top-right.
				{24, 40}, {14, 36}, {12, 30}, {14, 24}, {24, 22}, {26, 0},
			}
			eastBranch := [][2]float32{
				// Outer (east) boundary: top-right down through arm to bottom-right.
				{26, 0}, {28, 20}, {35, 24}, {35, 36}, {28, 40}, {26, 59},
				// Bottom edge across to the left.
				{13, 59},
				// Inner (west) boundary: bottom-left up around the patch back to top-left.
				{15, 40}, {25, 36}, {27, 30}, {25, 24}, {15, 22}, {13, 0},
			}
			return scene(40, 60).slope(15).
				groomPolygon(westBranch).
				groomPolygon(eastBranch).
				lodge().skier(ai.SkillIntermediate).
				treePatch(20, 30, 6, 0.8).build()
		},
	},
	{
		// Trail divergence: side walls channel the skier, a center patch
		// blocks the straight line, and the skier spawns 1 cell east of
		// the corridor centerline. The patch sits well below the spawn so
		// the skier has ~10 s of skiing before they reach it — enough
		// time for the controller to commit to a side and traverse.
		// Tests whether skiers DELIBERATELY choose paths or merely
		// avoid the nearest hazard.
		//
		// Layout (40 cells wide × 80 tall, 200 m × 400 m world):
		//   walls at x=[3,4],   z=[5,75]
		//   walls at x=[35,36], z=[5,75]
		//   patch at (cx=15, cz=50, r=6)  → cells x=[9,21], z=[44,56]
		//   skier at (gx=21, gz=1), lodge at (gx=21, gz=78)
		//
		// Grooming is two Y-branch polygons sharing top and bottom
		// stems (centred on the lodge/skier at x=21). The west branch
		// has to squeeze through a 3-cell corridor between the left
		// wall and the off-axis patch; the east branch gets a wider
		// lane. Stems overlap so the trail reads as one lane up top,
		// splits around the patch, and rejoins below.
		Name: "Trail diverge, offset skier east, intermediate",
		Seed: 1,
		Build: func(_ *rand.Rand) *world.World {
			westBranch := [][2]float32{
				// Outer (west) boundary, top → arm → bottom.
				{14, 0}, {11, 36}, {5, 44}, {5, 58}, {11, 64}, {14, 79},
				{28, 79},
				// Inner (east) boundary, bottom → west of patch → top.
				{25, 64}, {10, 58}, {8, 52}, {10, 46}, {25, 36}, {28, 0},
			}
			eastBranch := [][2]float32{
				// Outer (east) boundary, top → arm → bottom.
				{28, 0}, {32, 36}, {34, 44}, {34, 58}, {32, 64}, {28, 79},
				{14, 79},
				// Inner (west) boundary, bottom → east of patch → top.
				{18, 64}, {22, 58}, {22, 46}, {18, 40}, {14, 36}, {14, 0},
			}
			return scene(40, 80).slope(15).
				lodgeAt(21, 78).
				treeRect(3, 5, 4, 75, 0.8).    // left wall
				treeRect(35, 5, 36, 75, 0.8).  // right wall
				treePatch(15, 50, 6, 0.8).     // center obstacle (off-axis west)
				groomPolygon(westBranch).
				groomPolygon(eastBranch).
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

// flat fills every cell to the given elevation. The surface starts
// ungroomed (Grooming = 0); testbeds paint groomed lanes explicitly
// with groomRect so the skier sees a contrast between groomed and
// ungroomed snow rather than blanket corduroy.
func (b *builder) flat(elev float32) *builder {
	t := b.w.Terrain
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			t.Cells[x][z].GroundElevation = elev
			t.Cells[x][z].SnowDepth = world.DefaultSnowDepth
			t.Cells[x][z].Packed = 0.7
			t.Cells[x][z].Passable = true
		}
	}
	t.SnapshotNatural()
	return b
}

// slope tilts the terrain at slopeDeg degrees in +z (top of grid is high,
// bottom of grid is low). Elevation steps by CellSize · tan(angle) per cell
// so the produced surface really has the requested angle. Surface starts
// ungroomed — see flat() for the rationale.
func (b *builder) slope(slopeDeg float64) *builder {
	t := b.w.Terrain
	rate := float32(math.Tan(slopeDeg * math.Pi / 180))
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			t.Cells[x][z].GroundElevation = float32(t.Height-z) * CellSize * rate
			t.Cells[x][z].SnowDepth = world.DefaultSnowDepth
			t.Cells[x][z].Packed = 0.7
			t.Cells[x][z].Passable = true
		}
	}
	t.SnapshotNatural()
	return b
}

// runout shapes a steep upper section (z < upperEndZ) joining a gentler
// runout below. Both angles in degrees. Elevation steps by CellSize ·
// tan(angle) per cell so the produced surface really has the requested
// angle. Surface starts ungroomed — see flat() for the rationale.
func (b *builder) runout(upperEndZ int, upperDeg, runoutDeg float64) *builder {
	t := b.w.Terrain
	upperRate := float32(math.Tan(upperDeg * math.Pi / 180))
	runoutRate := float32(math.Tan(runoutDeg * math.Pi / 180))
	// Build elevations from bottom upward so the join elevation is consistent.
	for z := t.Height - 1; z >= 0; z-- {
		var elev float32
		if z >= upperEndZ {
			elev = float32(t.Height-z) * CellSize * runoutRate
		} else {
			joinElev := float32(t.Height-upperEndZ) * CellSize * runoutRate
			elev = joinElev + float32(upperEndZ-z)*CellSize*upperRate
		}
		for x := 0; x < t.Width; x++ {
			t.Cells[x][z].GroundElevation = elev
			t.Cells[x][z].SnowDepth = world.DefaultSnowDepth
			t.Cells[x][z].Packed = 0.7
			t.Cells[x][z].Passable = true
		}
	}
	t.SnapshotNatural()
	return b
}

// lodge places a target lodge at the bottom centre of the grid (z = H-2,
// x = W/2) with spawning disabled. Subsequent skier() calls auto-target it
// as the despawn destination. Testbeds use lodges (not parking lots) as
// the despawn target — they're exercising skier behaviour, not the
// player-facing arrival/departure system.
func (b *builder) lodge() *builder {
	t := b.w.Terrain
	return b.lodgeAt(t.Width/2, t.Height-2)
}

// lodgeAt is the explicit form of lodge — caller picks the cell. The
// continuous Pos lands at the cell centre so the lodge sits in the
// historical position from before continuous coords landed.
func (b *builder) lodgeAt(gx, gz int) *builder {
	const cellSize = 5.0
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
	const cellSize = 5.0
	elev := b.w.Terrain.SurfaceElevationAt(gx, gz)
	pos := mgl32.Vec3{(float32(gx) + 0.5) * cellSize, elev, (float32(gz) + 0.5) * cellSize}

	// Lodge target — continuous Pos already in world metres.
	lx := b.lastLodge.Pos[0]
	lz := b.lastLodge.Pos[1]
	heading := float32(math.Atan2(float64(lx-pos[0]), float64(lz-pos[2])))

	a := &world.Agent{
		ID:       b.w.NextID(),
		Pos:      pos,
		Heading:  heading,
		TargetID: b.lastLodge.ID,
		Traits:   ai.TraitsFor(skill),
		Balance:  1.0,
		Energy:   1.0,
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
			t.Cells[x][z].NaturalTrees = density
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
			t.Cells[x][z].NaturalTrees = density
		}
	}
	return b
}

// groomRect sets Grooming = 1.0 (corduroy) on every cell in
// [x1, x2] × [z1, z2] (inclusive, clamped to terrain bounds). Pairs
// with the ungroomed default left by flat / slope / runout: callers
// paint the lanes they want groomed and the rest of the surface stays
// raw. Order vs. tree-placement calls doesn't matter — groomRect
// touches only the Grooming field.
func (b *builder) groomRect(x1, z1, x2, z2 int) *builder {
	t := b.w.Terrain
	for x := x1; x <= x2; x++ {
		for z := z1; z <= z2; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			t.Cells[x][z].Grooming = 1.0
		}
	}
	return b
}

// groomPolygon sets Grooming = 1.0 on every cell whose centre lies
// inside the closed polygon defined by `pts`. Points are in grid
// coordinates (cell indices, fractional allowed); the polygon is
// implicitly closed from the last point back to the first. Winding
// order doesn't matter — uses the even-odd rule via horizontal-ray
// casting. Out-of-bounds cells are skipped.
//
// Lets testbeds describe organic, curving groomed lanes that the
// rectangular axis-aligned groomRect can't express cleanly — e.g. a
// trail that diverges around a tree patch and rejoins.
func (b *builder) groomPolygon(pts [][2]float32) *builder {
	t := b.w.Terrain
	if len(pts) < 3 {
		return b
	}
	minX, maxX := pts[0][0], pts[0][0]
	minZ, maxZ := pts[0][1], pts[0][1]
	for _, p := range pts {
		if p[0] < minX {
			minX = p[0]
		}
		if p[0] > maxX {
			maxX = p[0]
		}
		if p[1] < minZ {
			minZ = p[1]
		}
		if p[1] > maxZ {
			maxZ = p[1]
		}
	}
	x0 := int(math.Floor(float64(minX)))
	x1 := int(math.Ceil(float64(maxX)))
	z0 := int(math.Floor(float64(minZ)))
	z1 := int(math.Ceil(float64(maxZ)))
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			if pointInPolygon(float32(x)+0.5, float32(z)+0.5, pts) {
				t.Cells[x][z].Grooming = 1.0
			}
		}
	}
	return b
}

// pointInPolygon returns true if (px, pz) lies inside the polygon
// `pts` under the even-odd rule. Standard horizontal-ray cast: count
// edge crossings to the right of the point; odd count → inside.
func pointInPolygon(px, pz float32, pts [][2]float32) bool {
	inside := false
	n := len(pts)
	j := n - 1
	for i := 0; i < n; i++ {
		xi, zi := pts[i][0], pts[i][1]
		xj, zj := pts[j][0], pts[j][1]
		if (zi > pz) != (zj > pz) &&
			px < (xj-xi)*(pz-zi)/(zj-zi)+xi {
			inside = !inside
		}
		j = i
	}
	return inside
}

// build returns the finished *world.World. The builder should not be used
// after this call.
func (b *builder) build() *world.World {
	return b.w
}
