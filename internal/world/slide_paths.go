package world

import (
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
)

// Slide paths (notes/next/Avalanche Control.md): where avalanches start
// and where they run. Steep open ground (over AvySlopeMin, under two
// trees a cell) is grouped into start zones; each is traced down the
// fall line to where the ground flattens out, and the cells it crosses
// are its track and runout. A path builds up load from storms and wind
// (sim/avalanche.go) and has a danger level from it, the same five as a
// forecast's.

// DangerLevel is a slide path's avalanche danger: Low to Extreme.
// "Danger" is the real scale's Considerable.
type DangerLevel uint8

const (
	DangerLow DangerLevel = iota
	DangerModerate
	DangerDanger
	DangerHigh
	DangerExtreme
	DangerLevels
)

func (d DangerLevel) String() string {
	switch d {
	case DangerLow:
		return "Low"
	case DangerModerate:
		return "Moderate"
	case DangerDanger:
		return "Danger"
	case DangerHigh:
		return "High"
	}
	return "Extreme"
}

// SlidePath is one avalanche path.
type SlidePath struct {
	ID    int
	Start [][2]int // start zone cells
	Cells [][2]int // track and runout cells, start zone included
	// Trails is the runs its track or runout crosses.
	Trails []uint64
	// Aspect is the direction the start zone faces (downhill), radians
	// clockwise from north (+Z); Steep its mean slope angle, radians.
	Aspect, Steep float32
	// Centre is the start zone's middle, world XZ.
	CentreX, CentreZ float32
	// Load is the unstable snow on the start zone, metres of water
	// (storms and wind add it, time and releases take it away); Danger
	// is the level it makes now. Not saved: worked out again on load.
	Load   float32
	Danger DangerLevel
}

const (
	// minStartCells is the smallest start zone that counts (100 m²),
	// maxStartCells the biggest (about 4 ha); startAspectSpread how far
	// (radians) a start cell may face from the zone's first.
	minStartCells     = 4
	maxStartCells     = 160
	startAspectSpread = float32(40 * math.Pi / 180)
	// startMaxTrees is the most trees a start cell can have: a stand of
	// two anchors the snow.
	startMaxTrees = 1
	// runoutFlat is the slope under which a path is running out, and
	// runoutCells how many such cells it runs on before it stops.
	runoutFlat  = float32(0.18)
	runoutCells = 8
)

// BuildSlidePaths finds the slide paths on t, and the runs each crosses.
func BuildSlidePaths(t *Terrain, trails []*Trail) []*SlidePath {
	W, H := t.Width, t.Height
	start := make([]bool, W*H)
	for x := 0; x < W; x++ {
		for z := 0; z < H; z++ {
			c := &t.Cells[x][z]
			if c.Slope >= AvySlopeMin && c.TreeCount <= startMaxTrees && c.Passable {
				start[x*H+z] = true
			}
		}
	}
	seen := make([]bool, W*H)
	trailAt := map[[2]int][]uint64{}
	for _, tr := range trails {
		for _, c := range tr.Cells {
			trailAt[c] = append(trailAt[c], tr.ID)
		}
	}
	var paths []*SlidePath
	for x := 0; x < W; x++ {
		for z := 0; z < H; z++ {
			if !start[x*H+z] || seen[x*H+z] {
				continue
			}
			// The start zone: the start cells joined to this one that
			// face the same way, up to maxStartCells: a face, not a
			// whole mountainside.
			aspect := cellAspect(t, x, z)
			var zone [][2]int
			queue := [][2]int{{x, z}}
			seen[x*H+z] = true
			for len(queue) > 0 && len(zone) < maxStartCells {
				c := queue[0]
				queue = queue[1:]
				zone = append(zone, c)
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, nz := c[0]+d[0], c[1]+d[1]
					if nx < 0 || nz < 0 || nx >= W || nz >= H || !start[nx*H+nz] || seen[nx*H+nz] {
						continue
					}
					if angleDiff(cellAspect(t, nx, nz), aspect) > startAspectSpread {
						continue
					}
					seen[nx*H+nz] = true
					queue = append(queue, [2]int{nx, nz})
				}
			}
			for _, c := range queue {
				seen[c[0]*H+c[1]] = false // left for the next zone
			}
			if len(zone) < minStartCells {
				continue
			}
			paths = append(paths, tracePath(t, zone, trailAt))
		}
	}
	sort.Slice(paths, func(i, j int) bool { return len(paths[i].Start) > len(paths[j].Start) })
	for i, p := range paths {
		p.ID = i + 1
	}
	return paths
}

// tracePath follows the fall line from every cell of a start zone down to
// where it runs out, and sums up the path.
func tracePath(t *Terrain, zone [][2]int, trailAt map[[2]int][]uint64) *SlidePath {
	p := &SlidePath{Start: zone}
	in := map[[2]int]bool{}
	add := func(c [2]int) {
		if !in[c] {
			in[c] = true
			p.Cells = append(p.Cells, c)
		}
	}
	var gx, gz, steep, cx, cz float32
	for _, c := range zone {
		add(c)
		ax, az := t.GradientAt(c[0], c[1])
		gx, gz = gx+ax, gz+az
		steep += float32(math.Atan(float64(t.Cells[c[0]][c[1]].Slope)))
		cx += (float32(c[0]) + 0.5) * CellSize
		cz += (float32(c[1]) + 0.5) * CellSize
	}
	n := float32(len(zone))
	p.Steep, p.CentreX, p.CentreZ = steep/n, cx/n, cz/n
	// Downhill is against the gradient.
	p.Aspect = float32(math.Atan2(float64(-gx), float64(-gz)))
	for _, c := range zone {
		cur := c
		flat := 0
		for steps := 0; steps < 400; steps++ {
			e := t.Cells[cur[0]][cur[1]].GroundElevation
			next, drop := cur, float32(0)
			for dx := -1; dx <= 1; dx++ {
				for dz := -1; dz <= 1; dz++ {
					nx, nz := cur[0]+dx, cur[1]+dz
					if (dx == 0 && dz == 0) || !t.InBounds(nx, nz) {
						continue
					}
					d := e - t.Cells[nx][nz].GroundElevation
					if dx != 0 && dz != 0 {
						d /= math.Sqrt2
					}
					if d > drop {
						next, drop = [2]int{nx, nz}, d
					}
				}
			}
			if next == cur {
				break // a hollow: it piles up here
			}
			nc := &t.Cells[next[0]][next[1]]
			if !nc.Passable || nc.TreeCover() > 0.7 {
				break // a building or a stand of trees stops it
			}
			cur = next
			add(cur)
			if nc.Slope < runoutFlat {
				if flat++; flat >= runoutCells {
					break
				}
			}
		}
	}
	seenTrail := map[uint64]bool{}
	for _, c := range p.Cells {
		for _, id := range trailAt[c] {
			if !seenTrail[id] {
				seenTrail[id] = true
				p.Trails = append(p.Trails, id)
			}
		}
	}
	return p
}

// DangerFor is the danger a path's load makes on a slope of steep
// radians: the load against dangerLoad (about 25 cm of new snow makes
// "Danger" on a 38° slope), less on slopes off the 38° that slides best:
// little under 33°, nothing at 30°, and down to a third on the steepest
// ground, which sloughs as it loads.
func DangerFor(load, steep float32) (DangerLevel, float32) {
	const dangerLoad = 0.03
	deg := steep * 180 / math.Pi
	var slope float32
	switch {
	case deg < 38:
		slope = clamp01((deg - 30) / 8)
		slope *= slope
	default:
		slope = max(1-(deg-38)/15, 0.3)
	}
	index := load / dangerLoad * slope
	switch {
	case index < 0.4:
		return DangerLow, index
	case index < 0.8:
		return DangerModerate, index
	case index < 1.4:
		return DangerDanger, index
	case index < 2.2:
		return DangerHigh, index
	}
	return DangerExtreme, index
}

// SetAvyDanger records each path's danger on its cells for the map view
// (AvyDangerShade), the worst where paths overlap, and marks the snow to
// redraw where it changed.
func (t *Terrain) SetAvyDanger(paths []*SlidePath) {
	W, H := t.Width, t.Height
	next := make([]uint8, W*H)
	for _, p := range paths {
		v := uint8(p.Danger) + 1
		for _, c := range p.Cells {
			if i := c[0]*H + c[1]; next[i] < v {
				next[i] = v
			}
		}
	}
	for i := range next {
		if len(t.avyDanger) != len(next) || t.avyDanger[i] != next[i] {
			t.MarkSnowDirty(i/H, i%H)
		}
	}
	t.avyDanger = next
}

// AvyDangerShade is the danger on cell (x, z) for the map view: 0 off
// every path, else (level + 1) / 5.
func (t *Terrain) AvyDangerShade(x, z int) float32 {
	if len(t.avyDanger) != t.Width*t.Height || !t.InBounds(x, z) {
		return 0
	}
	return float32(t.avyDanger[x*t.Height+z]) / 5
}

// cellAspect is the way cell (x, z) faces downhill, radians clockwise
// from north (+Z).
func cellAspect(t *Terrain, x, z int) float32 {
	gx, gz := t.GradientAt(x, z)
	return float32(math.Atan2(float64(-gx), float64(-gz)))
}

// angleDiff is how far apart angles a and b are, radians, 0 to pi.
func angleDiff(a, b float32) float32 {
	d := float32(math.Mod(float64(a-b), 2*math.Pi))
	if d < 0 {
		d += 2 * math.Pi
	}
	if d > math.Pi {
		d = 2*math.Pi - d
	}
	return d
}

// Avalanche control (notes/next/Avalanche Control.md): an upgrade to a
// patrol room (Building.AvyControl). Its patrollers are trained for
// control work and paid more; with it, runs under a slide path at or
// over the resort's limit (World.AvyLimit) close until the danger
// drops (Trail.AvyClosed).
const (
	AvyControlCost        = 80_000
	AvyPatrollerDailyCost = 250 // on top of PatrollerDailyCost
	// MortarRange is how far from an avalanche-control patrol room its
	// mortar reaches a start zone, metres; MortarShotCost a round.
	MortarRange    = 1500
	MortarShotCost = 400
)

// SlidePathByID is the path with id, nil when there's none.
func (w *World) SlidePathByID(id int) *SlidePath {
	if id >= 1 && id <= len(w.SlidePaths) && w.SlidePaths[id-1].ID == id {
		return w.SlidePaths[id-1]
	}
	for _, p := range w.SlidePaths {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// AvyControlled reports whether the resort has avalanche control: a
// patrol room with the upgrade.
func (w *World) AvyControlled() bool {
	for _, b := range w.Buildings {
		if b.AvyControl && b.Offers(ServicePatrol) {
			return true
		}
	}
	return false
}

// SetAvyClosedCells records the cells of the runs closed for avalanche
// danger that aren't on an open run too, for routes to keep off
// (AvyClosedAt).
func (w *World) SetAvyClosedCells() {
	w.avyClosedCells = nil
	open := map[[2]int]bool{}
	for _, t := range w.Trails {
		if !t.AvyClosed {
			for _, c := range t.Cells {
				open[c] = true
			}
		}
	}
	for _, t := range w.Trails {
		if !t.AvyClosed {
			continue
		}
		for _, c := range t.Cells {
			if !open[c] {
				if w.avyClosedCells == nil {
					w.avyClosedCells = map[[2]int]bool{}
				}
				w.avyClosedCells[c] = true
			}
		}
	}
}

// AvyClosedAt reports whether cell (x, z) is on a run closed for
// avalanche danger, and no open one.
func (w *World) AvyClosedAt(x, z int) bool {
	return w.avyClosedCells != nil && w.avyClosedCells[[2]int{x, z}]
}

// AvyShot is one mortar round of avalanche control: fired from From at
// Fired, landing on To at Lands (sim seconds).
type AvyShot struct {
	From, To     mgl32.Vec3
	Fired, Lands float64
}

// AvyBurst is a burst of snow avalanche control set off at Pos at sim
// time At: a round's blast (Blast), or a ski cut cracking the slope.
type AvyBurst struct {
	Pos   mgl32.Vec3
	At    float64
	Blast bool
}
