package sim

import (
	"math"
	"math/rand"
	"strings"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Testbed is a small, named world that drives the in-game testbed picker and
// the headless `-testbed` CLI runner. The contract: Build returns a fresh
// *world.World containing terrain, an optional target lodge, and one or more
// pre-spawned skiers. Builders are expected to be deterministic given the rng
// so the visual mode and the headless runner see identical worlds.
//
// Key is the stable identifier used by the headless runner; it matches the
// build function name so `go run . -testbed BuildSlope10Intermediate` is
// unambiguous. Name is the human-readable label shown in the menu.
type Testbed struct {
	Key   string
	Name  string
	Build func(rng *rand.Rand) *world.World
	Seed  int64
}

// Testbeds is the registry surfaced by the start-menu testbed picker and by
// the headless runner. Append new entries here.
var Testbeds = []Testbed{
	{Key: "BuildFlatPlane", Name: "Flat plane (beginner)", Build: BuildFlatPlane, Seed: 1},
	{Key: "BuildSlope10Intermediate", Name: "10° slope (intermediate)", Build: BuildSlope10Intermediate, Seed: 1},
	{Key: "BuildInclinedPlane", Name: "15° slope (intermediate)", Build: BuildInclinedPlane, Seed: 1},
	{Key: "BuildSlope20Advanced", Name: "20° slope (advanced)", Build: BuildSlope20Advanced, Seed: 1},
	{Key: "BuildRunout", Name: "Slope with flat runout (advanced)", Build: BuildRunout, Seed: 1},
	{Key: "BuildBowl", Name: "Bowl (intermediate)", Build: BuildBowl, Seed: 1},
}

// FindTestbed returns the testbed whose Key matches `key` (case-insensitive).
// Returns nil if not found.
func FindTestbed(key string) *Testbed {
	for i := range Testbeds {
		if strings.EqualFold(Testbeds[i].Key, key) {
			return &Testbeds[i]
		}
	}
	return nil
}

// BuildFlatPlane: 600 × 400 m flat terrain, lodge at the centre, beginner
// skier on the west edge. Validates that on near-flat terrain the skier
// points at the goal and walk-shuffles directly to it.
func BuildFlatPlane(_ *rand.Rand) *world.World {
	const wCells, hCells = 60, 40
	t := world.NewTerrain(wCells, hCells)
	fillFlat(t, 100)
	w := world.NewWorld(t)

	lodge := placeTargetLodge(w, wCells/2, hCells/2)
	spawnTestbedSkier(w, 1, hCells/2, lodge, ai.SkillBeginner)
	return w
}

// BuildSlope10Intermediate: gentle 10° slope, intermediate skier. Should
// link shallow parallel turns and arrive comfortably.
func BuildSlope10Intermediate(_ *rand.Rand) *world.World {
	return slopeWith(40, 60, 10, ai.SkillIntermediate)
}

// BuildInclinedPlane: 15° slope, intermediate skier. The original go-to
// testbed.
func BuildInclinedPlane(_ *rand.Rand) *world.World {
	return slopeWith(40, 60, 15, ai.SkillIntermediate)
}

// BuildSlope20Advanced: 20° slope, advanced skier. Should link wider arcs
// and modulate speed near the comfort ceiling.
func BuildSlope20Advanced(_ *rand.Rand) *world.World {
	return slopeWith(40, 60, 20, ai.SkillAdvanced)
}

// BuildRunout: 18° upper section transitioning to ~3° flat runout. Tests
// the transition from linked turns into point-and-shoot on flats.
func BuildRunout(_ *rand.Rand) *world.World {
	const wCells, hCells = 40, 80
	const upperEnd = 50 // z-cell where the steep section ends
	const upperRate = 0.32 // tan(~18°)
	const runoutRate = 0.05 // tan(~3°)

	t := world.NewTerrain(wCells, hCells)
	// Build elevations from bottom upward so the join elevation is consistent.
	for z := hCells - 1; z >= 0; z-- {
		var elev float32
		switch {
		case z >= upperEnd:
			// Below the join: gentle runout.
			elev = float32(hCells-z) * 10.0 * runoutRate
		default:
			// Above the join: steep upper section, continuing from where
			// the runout ended.
			joinElev := float32(hCells-upperEnd) * 10.0 * runoutRate
			elev = joinElev + float32(upperEnd-z)*10.0*upperRate
		}
		for x := 0; x < wCells; x++ {
			t.Cells[x][z].Elevation = elev
			t.Cells[x][z].SnowDepth = 1.0
			t.Cells[x][z].Passable = true
		}
	}
	w := world.NewWorld(t)
	lodge := placeTargetLodge(w, wCells/2, hCells-2)
	spawnTestbedSkier(w, wCells/2, 1, lodge, ai.SkillAdvanced)
	return w
}

// BuildBowl: a curved descent where the fall line bends ~60° between top
// and bottom. The lodge sits at the bowl's mouth so seek and fall agree at
// the exit but disagree higher up. Intermediate skier.
func BuildBowl(_ *rand.Rand) *world.World {
	const wCells, hCells = 60, 60
	const radius = 50.0 // m
	t := world.NewTerrain(wCells, hCells)

	// Bowl centre in world coords (offset so the bowl sweeps across the map).
	cx := float64(wCells/2) * 10.0
	cz := float64(hCells/2-10) * 10.0

	for x := 0; x < wCells; x++ {
		for z := 0; z < hCells; z++ {
			wx := float64(x) * 10.0
			wz := float64(z) * 10.0
			dx := wx - cx
			dz := wz - cz
			dist := math.Sqrt(dx*dx + dz*dz)
			// Paraboloid: elevation rises with distance from centre.
			depth := 60.0
			elev := float32(depth * (1 - math.Min(1, dist/radius/3)))
			t.Cells[x][z].Elevation = 100 - elev
			t.Cells[x][z].SnowDepth = 1.0
			t.Cells[x][z].Passable = true
		}
	}
	w := world.NewWorld(t)
	// Lodge near the bowl exit (down and to the side from the bowl centre).
	lodge := placeTargetLodge(w, wCells/2, hCells-3)
	// Spawn skier on the rim, off-axis from the lodge so seek and fall start
	// disagreeing immediately.
	spawnTestbedSkier(w, wCells/2-15, 5, lodge, ai.SkillIntermediate)
	return w
}

// =============================================================================
// Helpers
// =============================================================================

// fillFlat sets every cell to the given elevation, full snow, passable.
func fillFlat(t *world.Terrain, elev float32) {
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			t.Cells[x][z].Elevation = elev
			t.Cells[x][z].SnowDepth = 1.0
			t.Cells[x][z].Passable = true
		}
	}
}

// slopeWith builds a wCells × hCells terrain with a constant slope angle
// (degrees) tilting in +z, places a lodge at bottom centre, and spawns a
// skier of the given skill at top centre.
func slopeWith(wCells, hCells, slopeDeg int, skill ai.SkillLevel) *world.World {
	rate := float32(math.Tan(float64(slopeDeg) * math.Pi / 180))
	t := world.NewTerrain(wCells, hCells)
	for x := 0; x < wCells; x++ {
		for z := 0; z < hCells; z++ {
			t.Cells[x][z].Elevation = float32(hCells-z) * 10.0 * rate
			t.Cells[x][z].SnowDepth = 1.0
			t.Cells[x][z].Passable = true
		}
	}
	w := world.NewWorld(t)
	lodge := placeTargetLodge(w, wCells/2, hCells-2)
	spawnTestbedSkier(w, wCells/2, 1, lodge, skill)
	return w
}

// placeTargetLodge places a lodge with spawning disabled (we only want it as
// a navigation target, not a skier source).
func placeTargetLodge(w *world.World, gx, gz int) *world.Building {
	lodge := w.PlaceBuilding(gx, gz)
	lodge.SkierCount = 0
	lodge.MeanSpawnRate = 0
	return lodge
}

// spawnTestbedSkier places a single skier at (gx, gz) heading toward `lodge`
// in StateReturningToLodge so the existing tickReturning → tickSkier path
// drives them. Traits are populated from the skill level.
func spawnTestbedSkier(w *world.World, gx, gz int, lodge *world.Building, skill ai.SkillLevel) *world.Agent {
	const cellSize = 10.0
	elev := w.Terrain.ElevationAt(gx, gz)
	pos := mgl32.Vec3{float32(gx) * cellSize, elev, float32(gz) * cellSize}

	lodgeX := float32(lodge.Pos[0]) * cellSize
	lodgeZ := float32(lodge.Pos[1]) * cellSize
	heading := float32(math.Atan2(float64(lodgeX-pos[0]), float64(lodgeZ-pos[2])))

	a := &world.Agent{
		ID:               w.NextID(),
		Pos:              pos,
		Heading:          heading,
		State:            world.StateReturningToLodge,
		TargetBuildingID: lodge.ID,
		Traits:           ai.TraitsFor(skill),
		Balance:          1.0,
	}
	w.Agents = append(w.Agents, a)
	return a
}
