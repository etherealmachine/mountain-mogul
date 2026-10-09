package sim

import (
	"mountain-mogul/internal/world"

	"github.com/go-gl/mathgl/mgl32"
)

// Steering (decide, and sampleTactical's fan of hazard samples under it)
// is most of a skier's tick, and it only reads the world. So each step,
// tickGuests collects every skier's perception in its usual serial pass,
// decides them all at once across cores (the worker pool,
// worker_pool.go), then applies the decisions one by one in the same
// order. Applying in a fixed order, and giving each
// decision its own random stream seeded from the global RNG in the
// serial pass, keeps seeded runs repeatable.
//
// The one change from deciding inline: a skier no longer sees snow
// packed or moguls grown by skiers earlier in the same 1/30 s step.
// Other skiers' positions are those at the start of the step for everyone.

// skiJob is one skier waiting on their steering decision this step.
type skiJob struct {
	a      *world.Guest
	target mgl32.Vec3
	dist   float32
	perc   Perception
	seed   uint64
	dec    Decision
}

// skiersPerChunk is the fewest skiers a chunk of steering is cut into:
// smaller and handing out chunks costs more than the steering in them.
const skiersPerChunk = 16

// decideSkiers fills in jobs' decisions, splitting them across cores.
func (s *Simulation) decideSkiers(jobs []skiJob, dt float64) {
	chunks := min(workers.size()*2, len(jobs)/skiersPerChunk)
	if chunks <= 1 {
		decideRange(s, jobs, dt, &s.steerScratch)
		return
	}
	for len(s.steerScratches) < chunks {
		s.steerScratches = append(s.steerScratches, steerScratch{})
	}
	// The pool's workers and this goroutine pull chunks until they're
	// gone; nobody waits on a worker that hasn't woken (worker_pool.go).
	workers.run(chunks, func(c int) {
		lo, hi := len(jobs)*c/chunks, len(jobs)*(c+1)/chunks
		decideRange(s, jobs[lo:hi], dt, &s.steerScratches[c])
	})
}

func decideRange(s *Simulation, jobs []skiJob, dt float64, sc *steerScratch) {
	for i := range jobs {
		j := &jobs[i]
		r := stepRand(j.seed)
		j.dec = decide(s.World, s.towersScratch, s.spatial, j.a, j.perc, float32(dt), sc, &r)
	}
}

// steerScratch is one goroutine's reusable buffers for decide: the
// swerve check's hazard list and the fan's nearby towers and skiers.
type steerScratch struct {
	hazards []hazardPoint
	near    steerNear
}

// stepRand is a small random stream (splitmix64) for one decision, so
// skiers deciding in parallel don't share the global RNG.
type stepRand uint64

// Float32 returns a float in [0, 1).
func (r *stepRand) Float32() float32 {
	*r += 0x9E3779B97F4A7C15
	z := uint64(*r)
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	z ^= z >> 31
	return float32(z>>40) / (1 << 24)
}

// routeJob is a guest whose route round the trees is due this step.
type routeJob struct {
	a    *world.Guest
	goal mgl32.Vec3
}

// routesPerChunk is the fewest due routes a chunk is cut into.
const routesPerChunk = 2

// planRoutes plans, across cores, the routes round the trees that guests
// about to move will need this step (route planning was a quarter of a
// skiing guest's cost). It runs before the serial pass, with every
// guest's position and goal as that pass will see them, so the pass finds
// the routes ready and does exactly what it would have; a guest whose
// goal changes in the meantime (their plan moves on) is re-planned there
// as before.
func (s *Simulation) planRoutes() {
	w := s.World
	jobs := s.routeJobs[:0]
	for _, a := range w.OnMountain {
		if a.Removed || a.Unload.LiftID != 0 || a.OnPatrollerID != 0 || a.Fallen || a.OnLiftID != 0 ||
			a.Queued || a.Visit.Waiting || a.RestTimer > 0 || a.SkiTransitionTimer != 0 ||
			(len(a.Path) > 0 && a.PathIdx < len(a.Path)) {
			continue
		}
		goal, ok := liveTarget(w, a)
		goal = s.trailCarrot(a, goal)
		if !ok || !s.routeDue(a, goal) {
			continue
		}
		jobs = append(jobs, routeJob{a: a, goal: goal})
	}
	s.routeJobs = jobs
	// Always planned here, in parallel or not, so a run doesn't depend on
	// how many cores split the work.
	chunks := max(min(workers.size()*2, len(jobs)/routesPerChunk), 1)
	workers.run(chunks, func(c int) {
		for _, j := range jobs[len(jobs)*c/chunks : len(jobs)*(c+1)/chunks] {
			s.prepareRoute(j.a, j.goal)
		}
	})
}
