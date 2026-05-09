package sim

import (
	"fmt"
	"io"
	"math"
	"math/rand"

	"mountain-mogul/internal/world"
)

// AggregateOptions configures a multi-run headless sweep over a testbed.
// Per-run trace output is suppressed; the runner emits one terse line per
// run plus an overall tally so we can sanity-check whether outcomes are
// balanced (e.g., a symmetric tree patch should split L/R ~50/50).
type AggregateOptions struct {
	// SimSeconds caps each run's length (sim time). Default 240.
	SimSeconds float64
	// Seed is the base seed; per-run seed = Seed + runIndex. When 0, the
	// testbed's recommended seed is used as the base.
	Seed int64
	// Runs is the number of iterations. Must be ≥ 1.
	Runs int
}

type sideOutcome int

const (
	sideUnknown sideOutcome = iota // never crossed the patch latitude
	sideLeft
	sideRight
	sideExact // crossed exactly on cx — vanishingly rare with float coords
)

type runResult struct {
	seed      int64
	side      sideOutcome
	crossX    float32 // skier's interpolated x at z = cz
	arrived   bool
	fellOnce  bool
	finalDist float32
}

// RunHeadlessAggregate runs the named testbed `Runs` times with seeds
// Seed, Seed+1, …, Seed+Runs-1 and prints per-run + aggregate L/R stats
// for whichever side of the tree-patch centroid each skier ended up on.
// Intended for verifying that a symmetric obstacle produces a balanced
// avoidance distribution across seeds.
func RunHeadlessAggregate(out io.Writer, name string, opts AggregateOptions) error {
	tb, err := FindTestbed(name)
	if err != nil {
		return err
	}
	if opts.SimSeconds <= 0 {
		opts.SimSeconds = 240
	}
	if opts.Runs <= 0 {
		opts.Runs = 1
	}
	baseSeed := tb.Seed
	if opts.Seed != 0 {
		baseSeed = opts.Seed
	}

	// Probe-build once to detect whether the testbed has a tree patch and,
	// if so, where its centroid is. The patch is purely a function of the
	// builder DSL (deterministic), so any seed gives the same answer.
	probe := tb.Build(rand.New(rand.NewSource(baseSeed)))
	cx, cz, hasPatch := treePatchCenter(probe)

	fmt.Fprintf(out, "testbed: %s  base_seed=%d  runs=%d  sim_seconds=%.1f\n",
		tb.Name, baseSeed, opts.Runs, opts.SimSeconds)
	if hasPatch {
		fmt.Fprintf(out, "tree patch centroid: world=(%.1f,%.1f)  side reported as L/R of x=%.1f at z=%.1f\n\n",
			cx, cz, cx, cz)
	} else {
		fmt.Fprintf(out, "no tree patch detected — side stats will all read \"-\"\n\n")
	}

	fmt.Fprintf(out, "  %4s  %12s  %6s  %9s  %5s  %s\n",
		"run", "seed", "side", "x@cz", "fell", "outcome")

	counts := map[sideOutcome]int{}
	fellCount, arrivedCount := 0, 0

	for i := 0; i < opts.Runs; i++ {
		seed := baseSeed + int64(i)
		r := runOnceForAggregate(tb, seed, opts.SimSeconds, cx, cz, hasPatch)
		counts[r.side]++
		if r.fellOnce {
			fellCount++
		}
		if r.arrived {
			arrivedCount++
		}

		outcome := fmt.Sprintf("d=%.1f", r.finalDist)
		if r.arrived {
			outcome = "arrived"
		}
		fellTxt := "no"
		if r.fellOnce {
			fellTxt = "yes"
		}
		fmt.Fprintf(out, "  %4d  %12d  %6s  %+9.1f  %5s  %s\n",
			i+1, seed, sideLabel(r.side), r.crossX, fellTxt, outcome)
	}

	fmt.Fprintln(out)
	total := opts.Runs
	L, R, U, E := counts[sideLeft], counts[sideRight], counts[sideUnknown], counts[sideExact]
	fmt.Fprintf(out, "aggregate: runs=%d  left=%d (%.1f%%)  right=%d (%.1f%%)",
		total, L, pct(L, total), R, pct(R, total))
	if U > 0 || E > 0 {
		fmt.Fprintf(out, "  no-cross=%d  exact=%d", U, E)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "outcomes:  arrived=%d (%.1f%%)  fellOnce=%d (%.1f%%)\n",
		arrivedCount, pct(arrivedCount, total), fellCount, pct(fellCount, total))

	// Two-sided binomial sanity check vs 50/50. The standard deviation of L
	// (or R) under H0 is sqrt(N/4), so z = (|L-R|/2) / sqrt(N/4) =
	// |L-R| / sqrt(N).
	if N := L + R; N > 0 {
		diff := L - R
		if diff < 0 {
			diff = -diff
		}
		z := float64(diff) / math.Sqrt(float64(N))
		verdict := "consistent with 50/50 (|z| < 1.96)"
		if z > 1.96 {
			verdict = "outside 95% CI — investigate bias"
		}
		fmt.Fprintf(out, "balance:   |L-R|=%d over %d crossings  z≈%.2f  (%s)\n",
			diff, N, z, verdict)
	}
	return nil
}

// runOnceForAggregate is a stripped-down RunHeadless: no recorder, no
// per-tick printing. Captures only the side outcome + a few coarse stats.
func runOnceForAggregate(tb *Testbed, seed int64, simSeconds float64, cx, cz float32, hasPatch bool) runResult {
	w := tb.Build(rand.New(rand.NewSource(seed)))
	s := NewSimulationWithSeed(w, seed)
	s.TimeScale = 1.0

	res := runResult{seed: seed, side: sideUnknown}

	const dt = 1.0 / 60.0
	steps := int(simSeconds / dt)

	var prevX, prevZ float32
	hasPrev := false
	sideSet := false
	prevFallen := false

	for i := 0; i < steps; i++ {
		s.Tick(dt)
		if len(w.Agents) == 0 {
			res.arrived = true
			break
		}
		a := w.Agents[0]

		if a.Fallen && !prevFallen {
			res.fellOnce = true
		}
		prevFallen = a.Fallen

		if hasPatch && !sideSet && hasPrev {
			// Detect the first tick where z transitions across cz; linearly
			// interpolate x at z = cz so the crossing point is sub-tick
			// accurate even at high speed.
			if prevZ < cz && a.Pos[2] >= cz {
				span := a.Pos[2] - prevZ
				var crossX float32
				if span > 0 {
					t := (cz - prevZ) / span
					crossX = prevX + t*(a.Pos[0]-prevX)
				} else {
					crossX = a.Pos[0]
				}
				res.crossX = crossX
				switch {
				case crossX < cx:
					res.side = sideLeft
				case crossX > cx:
					res.side = sideRight
				default:
					res.side = sideExact
				}
				sideSet = true
			}
		}
		prevX = a.Pos[0]
		prevZ = a.Pos[2]
		hasPrev = true
	}

	if !res.arrived && len(w.Agents) > 0 {
		a := w.Agents[0]
		target := targetPos(w, a)
		res.finalDist = dist3(a.Pos[0]-target[0], a.Pos[1]-target[1], a.Pos[2]-target[2])
	}
	return res
}

// treePatchCenter returns the world-space centroid of cells with non-zero
// TreeDensity. Works for any builder-stamped patch (circular, irregular,
// or composite) since it averages all tree cells equally. ok=false when
// the testbed has no trees.
func treePatchCenter(w *world.World) (cx, cz float32, ok bool) {
	const cellSize = 5.0
	t := w.Terrain
	var sumX, sumZ, count int
	for x := 0; x < t.Width; x++ {
		for z := 0; z < t.Height; z++ {
			if t.Cells[x][z].TreeDensity > 0 {
				sumX += x
				sumZ += z
				count++
			}
		}
	}
	if count == 0 {
		return 0, 0, false
	}
	return float32(sumX) / float32(count) * cellSize,
		float32(sumZ) / float32(count) * cellSize,
		true
}

func sideLabel(s sideOutcome) string {
	switch s {
	case sideLeft:
		return "LEFT"
	case sideRight:
		return "RIGHT"
	case sideExact:
		return "EXACT"
	}
	return "-"
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}
