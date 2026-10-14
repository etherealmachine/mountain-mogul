package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// An aerial lift's cable profile: where its towers stand along the line,
// how tall each is, and the cable between them, laid out from the ground
// as a line engineer would. Towers go where the cable would otherwise
// come down too near the ground, on knolls and breaks of slope; a hollow
// or gully is crossed in one span, high over the ground, if the next
// tower is within the type's longest span. Towers are as short as will
// do, so on even ground they're short and evenly spread, and the terrain
// alone makes one lift look unlike another.
//
// Between supports the cable runs straight with a parabolic sag, sag per
// metre of span at its middle. Through each terminal's flat run it stays
// at terminal height over the snow (CableHeightAt), and the profile's end
// supports are there, read from the snow as it is now, so the cable meets
// the terminal whatever the snow does. The towers' tops are fixed when
// the profile is laid out, from the bare ground.
//
// The T-bar's cable and the surface lifts keep their height over the
// snow and evenly spaced towers (towersEven).

// ropewaySpec is how a lift type's line is laid out.
type ropewaySpec struct {
	// minTower and maxTower bound a tower's height: the cable's over the
	// bare ground at its foot.
	minTower, maxTower float32
	// margin is how far over the ground, past its terminal height (where
	// riders' feet or a cabin's floor are at the ground), the cable must
	// stay between towers, clear of a season's snow.
	margin float32
	// minSpan and maxSpan bound the metres between towers; firstTower and
	// firstMax the least and most from a terminal's flat run to its
	// nearest tower.
	minSpan, maxSpan, firstTower, firstMax float32
	// sag is the cable's sag at the middle of a span, per metre of span.
	sag float32
	// towerFixed is what a tower costs before its height, in metres of
	// tower: higher spreads towers further apart, lower keeps them
	// shorter at the price of more of them.
	towerFixed float32
}

func (t LiftType) ropeway() ropewaySpec {
	switch {
	case t.Tram(), t == LiftTri:
		// Few, tall lattice towers; spans of a kilometre and more.
		return ropewaySpec{minTower: 20, maxTower: 80, margin: 5, minSpan: 80, maxSpan: 1500, firstTower: 40, firstMax: 600, sag: 0.03, towerFixed: 150}
	case t == LiftFunitel:
		return ropewaySpec{minTower: 12, maxTower: 45, margin: 4, minSpan: 40, maxSpan: 400, firstTower: 20, firstMax: 150, sag: 0.025, towerFixed: 60}
	case t == LiftGondola:
		return ropewaySpec{minTower: 9, maxTower: 30, margin: 3.5, minSpan: 30, maxSpan: 140, firstTower: 12, firstMax: 50, sag: 0.02, towerFixed: 25}
	case t.Detachable():
		return ropewaySpec{minTower: 9, maxTower: 26, margin: 3.5, minSpan: 30, maxSpan: 100, firstTower: 12, firstMax: 45, sag: 0.02, towerFixed: 20}
	}
	return ropewaySpec{minTower: 8, maxTower: 24, margin: 3.5, minSpan: 25, maxSpan: 85, firstTower: 10, firstMax: 40, sag: 0.02, towerFixed: 20}
}

// TowerSpot is one of a lift's towers: where it stands, and its height,
// the cable's over the ground at its foot.
type TowerSpot struct {
	XZ     mgl32.Vec2
	Height float32
}

// cableSupport is a point the cable runs over: dist metres along the
// line from the base wheel, at y.
type cableSupport struct {
	dist, y float32
}

// cableProfile is a lift's laid-out line: its supports in order from the
// base terminal's to the top's, towers between.
type cableProfile struct {
	supports []cableSupport
	towers   []TowerSpot
	sag      float32
}

// profiled reports whether a lift of this type has a laid-out cable.
func (t LiftType) profiled() bool { return t.HasTowers() && !t.Hangers() }

// profileStep is the metres between ground samples along the line.
const profileStep = 2.5

// Profile is the lift's cable profile, laid out over t on first use and
// kept until ResetTowerCache. Nil for a lift with none.
func (l *Lift) profile(t *Terrain) *cableProfile {
	if !l.Type.profiled() || t == nil {
		return nil
	}
	if l.profileCache == nil {
		l.profileCache = l.layOutProfile(t)
	}
	return l.profileCache
}

// terminalLow is the cable's height over the snow through a terminal.
func (l *Lift) terminalLow() float32 {
	if low := l.Station().CableH; low > 0 {
		return low
	}
	return BullwheelHeight
}

// layOutProfile places the towers along the line, greedily from the base:
// from each support, of the places a tower could stand within the type's
// spans, the one that costs least per metre of line gained, as short as
// keeps the span clear of the ground. It stops once the top terminal
// can be reached clear in one span.
func (l *Lift) layOutProfile(t *Terrain) *cableProfile {
	spec := l.Type.ropeway()
	g := l.Station()
	length := l.cableLen()
	dir, perp := l.lineAxes()
	low := l.terminalLow()
	a, b := g.Flat, length-g.Flat
	at := func(d float32) mgl32.Vec2 { return l.Base.Add(dir.Mul(d)) }
	endY := func(d float32) float32 { p := at(d); return t.InterpolatedSurfaceElevationAt(p[0], p[1]) + low }
	p := &cableProfile{sag: spec.sag}
	if b-a < 2*spec.firstTower {
		// Too short for a tower: one span between the terminals.
		p.supports = []cableSupport{{a, endY(a)}, {b, endY(b)}}
		return p
	}

	// The ground along the line: centre for the towers' feet, and the
	// highest across the cables and what hangs from them for clearance.
	n := int(math.Ceil(float64((b-a)/profileStep))) + 1
	xs := make([]float32, n)
	centre := make([]float32, n)
	need := make([]float32, n) // the least cable height there
	reach := l.CableOffset() + 2
	clear := low + spec.margin
	for i := range xs {
		x := min(a+float32(i)*profileStep, b)
		xs[i] = x
		c := at(x)
		centre[i] = t.InterpolatedGroundElevationAt(c[0], c[1])
		ground := centre[i]
		for _, s := range [2]float32{-reach, reach} {
			q := c.Add(perp.Mul(s))
			ground = max(ground, t.InterpolatedGroundElevationAt(q[0], q[1]))
		}
		// Near a terminal the cable comes down to its height: the
		// clearance eases from terminal height at the flat run's end to
		// the full margin over the first tower's distance.
		e := min(x-a, b-x)
		k := min(e/spec.firstTower, 1)
		need[i] = ground + low - 0.3 + (clear-low+0.3)*k
	}
	last := n - 1
	yB := endY(b)

	// lowestTop is the lowest the cable can be at j, spanning from the
	// support at i (height y0), and stay clear of the ground between.
	lowestTop := func(i int, y0 float32, j int) float32 {
		span := xs[j] - xs[i]
		f := spec.sag * span
		top := float32(math.Inf(-1))
		for k := i + 1; k < j; k++ {
			u := (xs[k] - xs[i]) / span
			top = max(top, y0+(need[k]-y0+4*f*u*(1-u))/u)
		}
		return top
	}

	cur := 0
	y0 := endY(a)
	p.supports = append(p.supports, cableSupport{a, y0})
	stepsOf := func(m float32) int { return int(math.Ceil(float64(m / profileStep))) }
	for {
		if xs[last]-xs[cur] <= spec.maxSpan && lowestTop(cur, y0, last) <= yB {
			break
		}
		lo := cur + stepsOf(spec.minSpan)
		if cur == 0 {
			lo = stepsOf(spec.firstTower)
		}
		span := spec.maxSpan
		if cur == 0 {
			span = spec.firstMax
		}
		hi := min(cur+int(span/profileStep), last-stepsOf(spec.firstTower))
		if hi < lo {
			break // no room for another tower: the last span will have to do
		}
		best, bestTop := -1, float32(0)
		bestCost := float32(math.Inf(1))
		for j := lo; j <= hi; j++ {
			top := max(lowestTop(cur, y0, j), centre[j]+spec.minTower)
			h := top - centre[j]
			if h > spec.maxTower {
				continue
			}
			if cost := (spec.towerFixed + h) / (xs[j] - xs[cur]); cost < bestCost {
				best, bestTop, bestCost = j, top, cost
			}
		}
		if best < 0 {
			// Nothing clears (a wall of rock): the tallest tower as near
			// as can be, and on from there.
			best, bestTop = lo, centre[lo]+spec.maxTower
		}
		cur, y0 = best, bestTop
		p.supports = append(p.supports, cableSupport{xs[cur], y0})
		p.towers = append(p.towers, TowerSpot{XZ: at(xs[cur]), Height: y0 - centre[cur]})
	}
	p.supports = append(p.supports, cableSupport{b, yB})
	return p
}

// yAt is the profile's cable height (world Y) dist metres along the line,
// with the end supports at ends (the terminals' cable over the snow now).
func (p *cableProfile) yAt(dist, yA, yB float32) float32 {
	s := p.supports
	k := 0
	for k < len(s)-2 && dist > s[k+1].dist {
		k++
	}
	d0, d1 := s[k].dist, s[k+1].dist
	y0, y1 := s[k].y, s[k+1].y
	if k == 0 {
		y0 = yA
	}
	if k+1 == len(s)-1 {
		y1 = yB
	}
	span := d1 - d0
	if span <= 0 {
		return y0
	}
	u := clamp01((dist - d0) / span)
	return y0 + (y1-y0)*u - 4*p.sag*span*u*(1-u)
}

// CableYAt is the cable's height (world Y) at, dist metres along the line
// from the base wheel: over a terminal, at its height over the snow;
// between them, on the laid-out profile for a lift with one, else
// CableHeightAt over the snow.
func (l *Lift) CableYAt(dist float32, at mgl32.Vec2, t *Terrain) float32 {
	p := l.profile(t)
	length := l.cableLen()
	flat := l.Station().Flat
	if p == nil || dist <= flat || dist >= length-flat {
		return t.InterpolatedSurfaceElevationAt(at[0], at[1]) + l.CableHeightAt(dist)
	}
	dir, _ := l.lineAxes()
	low := l.terminalLow()
	a, b := l.Base.Add(dir.Mul(flat)), l.Base.Add(dir.Mul(length-flat))
	yA := t.InterpolatedSurfaceElevationAt(a[0], a[1]) + low
	yB := t.InterpolatedSurfaceElevationAt(b[0], b[1]) + low
	return p.yAt(dist, yA, yB)
}

// TowerDists is how far along the line each tower stands, for sampling
// the cable at its kinks.
func (l *Lift) TowerDists(t *Terrain) []float32 {
	p := l.profile(t)
	if p == nil {
		return nil
	}
	out := make([]float32, 0, len(p.towers))
	for _, s := range p.supports[1 : len(p.supports)-1] {
		out = append(out, s.dist)
	}
	return out
}
