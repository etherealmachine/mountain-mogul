package sim

import (
	"math"
	"testing"

	"mountain-mogul/internal/world"
)

// checkPasses verifies every trail cell is under a pass and no pass point
// strays off the trail, and returns the length-weighted mean |cos| between
// lane direction and dir.
func checkPasses(t *testing.T, w *world.World, trail *world.Trail, dir vec2) float32 {
	t.Helper()
	passes := planTrailPasses(w.Terrain, trail)
	if len(passes) == 0 {
		t.Fatal("no passes")
	}
	in := map[[2]int]bool{}
	for _, c := range trail.Cells {
		in[c] = true
	}
	covered := passCells(w.Terrain, passes, func(c [2]int) bool { return in[c] })
	if len(covered) < len(trail.Cells) {
		t.Errorf("passes cover %d of %d trail cells", len(covered), len(trail.Cells))
	}
	var align, total float32
	for _, ps := range passes {
		for i, q := range ps.Pts {
			// Allow the half-metre stub on corner fillers.
			if !in[[2]int{int((q[0] - 0.5) / world.CellSize), int(q[1] / world.CellSize)}] &&
				!in[[2]int{int((q[0] + 0.5) / world.CellSize), int(q[1] / world.CellSize)}] &&
				!in[[2]int{int(q[0] / world.CellSize), int((q[1] - 0.5) / world.CellSize)}] &&
				!in[[2]int{int(q[0] / world.CellSize), int((q[1] + 0.5) / world.CellSize)}] {
				t.Errorf("pass point %v off the trail", q)
				break
			}
			if i > 0 {
				d := norm2(vec2{q[0] - ps.Pts[i-1][0], q[1] - ps.Pts[i-1][1]})
				align += float32(math.Abs(float64(dot2(d, dir))))
				total++
			}
		}
	}
	return align / total
}

// On an open slope, lanes run down the fall line (+z).
func TestPassesFollowFallLine(t *testing.T) {
	w := scene(30, 40).slope(15).groomedRect(world.DiffBlue, 8, 4, 13, 31).build()
	if a := checkPasses(t, w, w.Trails[0], vec2{0, 1}); a < 0.95 {
		t.Errorf("lane alignment with fall line = %.2f, want ≥ 0.95", a)
	}
}

// A narrow cat track across the slope gets lanes along its length (x).
func TestPassesAlongCatTrack(t *testing.T) {
	w := scene(60, 30).slope(12).groomedRect(world.DiffGreen, 5, 12, 50, 3).build()
	if a := checkPasses(t, w, w.Trails[0], vec2{1, 0}); a < 0.95 {
		t.Errorf("lane alignment with the track = %.2f, want ≥ 0.95", a)
	}
}

// A run that bends from the fall line onto a traverse is fully covered.
func TestPassesCoverCurvingRun(t *testing.T) {
	w := scene(50, 45).slope(14).groomedRun(world.DiffBlue, [][2]int{{10, 3}, {12, 20}, {20, 30}, {40, 33}}, 35).build()
	checkPasses(t, w, w.Trails[0], vec2{0, 1})
}

// Two cats from one shed split a trail's passes about evenly by length.
func TestSectionsSplitByLength(t *testing.T) {
	w := scene(30, 40).slope(15).groomedRect(world.DiffBlue, 6, 4, 17, 31).shedAt(14, 37).build()
	w.SpawnSnowcat(w.Buildings[0])
	s := NewSimulationWithSeed(w, 1)
	s.reassignAllSections()
	var lens []float32
	for _, cat := range w.Snowcats {
		var l float32
		for _, ps := range cat.Section {
			l += ps.Length()
		}
		lens = append(lens, l)
	}
	if len(lens) != 2 || lens[0] == 0 || lens[1] == 0 {
		t.Fatalf("section lengths %v, want two non-empty", lens)
	}
	if r := min(lens[0], lens[1]) / max(lens[0], lens[1]); r < 0.7 {
		t.Errorf("section lengths %v, ratio %.2f, want ≥ 0.7", lens, r)
	}
}
