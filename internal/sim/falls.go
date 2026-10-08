package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// A fall plays out in phases (world.Tumble): the guest goes down the way
// the fall threw them and slides to a stop, lies there a moment, gets up
// (slower for beginners and on steeper ground, with their skis set across
// the fall line first), and, if a fast fall knocked their skis off, walks
// back to collect them and clips in again. An injured guest stops lying
// where they slid, for patrol.
const (
	// Sliding: the share of their speed the body keeps by fall direction,
	// how hard snow drags a body (by snow kind), and the longest slide.
	slideKeepForward  = float32(0.8)
	slideKeepSide     = float32(0.7)
	slideKeepBack     = float32(0.5)
	treeBounceMax     = float32(2) // m/s back off a trunk
	slideFriction     = 0.6        // body on packed snow
	slideFrictionLow  = 0.25       // on boilerplate
	slideFrictionIcy  = 0.35       // on frozen granular, crust
	slideFrictionDeep = 0.9        // in powder, cement, slush
	slideStopSpeed    = float32(0.25)
	slideMaxTime      = float32(6)
	slideTreeStop     = float32(0.6) // m from a trunk: they come up against it

	// Lying, then getting up: seconds, by skill. Ground past
	// getUpEasySlope of the guest's comfort slope stretches getting up by
	// getUpSlopeScale per radian past it; a face-down fall takes
	// getUpRollOver longer (rolling over first).
	lieBase         = float32(1)
	lieUnskilled    = float32(1.5)
	getUpBase       = float32(1.5)
	getUpUnskilled  = float32(5)
	getUpEasySlope  = float32(0.6)
	getUpSlopeScale = float32(5)
	getUpRollOver   = float32(1.3)

	// Yard sales: a fall faster than yardSaleSpeed can knock the skis off,
	// more likely the faster (up to yardSaleChance at yardSaleSpeed +
	// yardSaleRange). Skis come to rest behind the body, along the line
	// of the fall; the guest walks back for them at collectSpeed and
	// takes clipInBase (plus clipInUnskilled for a beginner) to clip in.
	yardSaleSpeed   = float32(6)
	yardSaleRange   = float32(8)
	yardSaleChance  = float32(0.8)
	collectSpeed    = float32(0.9)
	collectReach    = float32(0.6)
	clipInBase      = float32(2)
	clipInUnskilled = float32(2)
)

// knockDown puts a guest down: the fall itself, before the caller rolls
// any injury. Their speed goes into the slide (Tumble.Vel), and a fast
// fall may knock their skis off.
func (s *Simulation) knockDown(a *world.Guest, dir world.FallDir) {
	a.Balance = 0
	a.Fallen = true
	a.Energy = clamp32(a.Energy-energyFallDrain, 0, 1)
	s.recordFall(a)

	f := mgl32.Vec2{float32(math.Sin(float64(a.Heading))), float32(math.Cos(float64(a.Heading)))}
	left := mgl32.Vec2{f[1], -f[0]}
	tb := world.Tumble{Phase: world.FallSliding, Dir: dir, Side: 1}
	if rng.Global().Float32() < 0.5 {
		tb.Side = -1
	}
	var keep float32
	switch dir {
	case world.FallForward:
		keep = slideKeepForward
	case world.FallBack:
		keep = slideKeepBack
	case world.FallSide:
		keep = slideKeepSide
		// Onto the uphill hip.
		if fx, fz := fallLine(s.World.Terrain, a.Pos); fx != 0 || fz != 0 {
			if left.Dot(mgl32.Vec2{-fx, -fz}) > 0 {
				tb.Side = -1
			} else {
				tb.Side = 1
			}
		}
	}
	vel := f.Mul(a.Speed * keep)
	if dir == world.FallThrown {
		vel = f.Mul(-min(a.Speed*0.3, treeBounceMax))
	}
	tb.Vel = [2]float32{vel[0], vel[1]}

	if a.SkisOn && a.Unload.LiftID == 0 {
		chance := clamp32((a.Speed-yardSaleSpeed)/yardSaleRange, 0, 1) * yardSaleChance
		if dir == world.FallSide || dir == world.FallBack {
			chance /= 2
		}
		if rng.Global().Float32() < chance {
			r := rng.Global()
			for i := range 2 {
				tb.SkisOff[i] = r.Float32() < 0.75
			}
			if !tb.SkiLost() {
				tb.SkisOff[r.Intn(2)] = true
			}
			for i := range 2 {
				if !tb.SkisOff[i] {
					continue
				}
				p := mgl32.Vec2{a.Pos[0], a.Pos[2]}.
					Add(f.Mul(0.5 + r.Float32()*0.2*a.Speed)).
					Add(left.Mul(r.Float32()*4 - 2))
				tb.Skis[i] = [2]float32{p[0], p[1]}
				tb.SkiYaw[i] = r.Float32() * 2 * math.Pi
			}
		}
	}
	a.Tumble = tb
	a.Speed = 0
	a.TurnSide = 0
}

// fallDirection is which way a guest who just lost their balance goes
// down, from what was costing them most: speed or bumps throw them
// forward, a pitch past their comfort sits them back, and the rest
// (skidding, scrubbing, trees) puts them on their hip.
func fallDirection(traits ai.GuestTraits, perc Perception, dec Decision) world.FallDir {
	var fwd, back float32
	side := dec.Scrub*0.02 + float32(skidBalanceCost)*perc.Skid*(1-clamp32(traits.Skill, 0, 1))
	if perc.AtCellDensity > inTreesThreshold {
		side += (perc.AtCellDensity - inTreesThreshold) * 0.4
	}
	if traits.ComfortSpeed > 0 {
		fwd = max(0, perc.Speed/traits.ComfortSpeed-1.2) * 0.3
	}
	fwd += mogulStress(traits, perc)
	if traits.ComfortSlope > 0 {
		back = max(0, feltSlope(perc)/traits.ComfortSlope-1.2) * 0.5
	}
	switch {
	case fwd > back && fwd > side:
		return world.FallForward
	case back > side:
		return world.FallBack
	}
	return world.FallSide
}

// tickFallen plays a fall out, phase by phase.
func (s *Simulation) tickFallen(a *world.Guest, dt float64) {
	tb := &a.Tumble
	tb.T += float32(dt)
	if tb.Phase == world.FallSliding {
		if !s.slide(a, float32(dt)) {
			return
		}
		tb.Phase, tb.T, tb.Vel = world.FallDown, 0, [2]float32{}
		skill := clamp32(a.Traits.Skill, 0, 1)
		tb.Dur = lieBase + lieUnskilled*(1-skill)
	}
	if a.Injured {
		s.tickInjured(a, dt)
		return
	}
	switch tb.Phase {
	case world.FallDown:
		if tb.T < tb.Dur {
			return
		}
		a.Patience = max(a.Patience-fallPatienceDrain, 0)
		if a.Run.Falls++; a.Run.Falls >= fallGiveUpCount && !a.HurtGoHome && a.OnLiftID == 0 && a.Unload.LiftID == 0 {
			tb.SkisOff = [2]bool{}
			s.giveUpRun(a)
			return
		}
		tb.Phase, tb.T, tb.Dur = world.FallGettingUp, 0, s.getUpTime(a)
		if a.Unload.LiftID == 0 { // getting off a lift keeps its line
			s.faceAcrossFallLine(a)
			getUpClearOfTrunk(a)
		}
	case world.FallGettingUp:
		if tb.T < tb.Dur {
			return
		}
		if !tb.SkiLost() {
			s.backOnFeet(a)
			return
		}
		tb.Phase, tb.T, tb.Dur = world.FallCollecting, 0, 0
	case world.FallCollecting:
		if tb.SkiLost() {
			s.collectSki(a, float32(dt))
			return
		}
		if tb.Dur == 0 {
			tb.T = 0
			tb.Dur = clipInBase + clipInUnskilled*(1-clamp32(a.Traits.Skill, 0, 1))
			a.Speed = 0
			s.faceAcrossFallLine(a)
		}
		if tb.T >= tb.Dur {
			s.backOnFeet(a)
		}
	}
}

// tickInjured holds an injured (or stranded) guest where they lie until
// patrol comes, or until they give up waiting and walk down.
func (s *Simulation) tickInjured(a *world.Guest, dt float64) {
	if a.OnPatrollerID != 0 || s.guestClaimed(a.ID) {
		return // help is loading them or on the way
	}
	a.InjuryWaitTimer -= float32(dt)
	if a.InjuryWaitTimer > 0 {
		return
	}
	stranded := a.Stranded
	a.Injured, a.Stranded, a.Fallen = false, false, false
	a.Tumble = world.Tumble{}
	a.Balance = float32(fallStartBalance)
	a.Speed, a.TurnSide = 0, 0
	a.SkisOn, a.OnFoot = false, true
	s.applyEvent(a, ai.ThoughtAbandoned)
	if stranded {
		// Gave up waiting too: they walk the rest of the way down.
		s.setDepartReason(a, ai.DepartGaveUp)
		s.directHomePlan(a)
		return
	}
	a.Patience = 0
	// Build a direct walk-to-parking plan so the guest crawls to the
	// nearest lot without routing through the lift system. Bypassing
	// GOAP is intentional: the planner would route WalkToLift →
	// RideLift → SkiToParking, which reads as "continued skiing."
	s.injuredGiveUpPlan(a)
}

// backOnFeet ends a fall: the guest skis on.
func (s *Simulation) backOnFeet(a *world.Guest) {
	a.Fallen = false
	a.Tumble = world.Tumble{}
	a.Balance = float32(fallStartBalance)
	a.Speed = 0
	a.TurnSide = 0
	if a.HurtGoHome {
		// A minor injury: up again, but done for the day.
		a.HurtGoHome = false
		s.setDepartReason(a, ai.DepartHurt)
		s.directHomePlan(a)
	}
}

// slide moves a fallen guest's body over the snow: gravity down the fall
// line against the drag of snow on a body. Reports whether they've come
// to rest.
func (s *Simulation) slide(a *world.Guest, dt float32) bool {
	tb := &a.Tumble
	t := s.World.Terrain
	v := mgl32.Vec2{tb.Vel[0], tb.Vel[1]}
	n := t.NormalAt(a.Pos[0]/CellSize, a.Pos[2]/CellSize)
	cos := n[1]
	sin := float32(math.Sqrt(float64(max(0, 1-cos*cos))))
	down := mgl32.Vec2{n[0], n[2]}
	if l := down.Len(); l > 1e-4 {
		down = down.Mul(1 / l)
	}
	mu := float32(bodyFriction(t, a.Pos))
	g := float32(gravity)
	v = v.Add(down.Mul(g * sin * dt))
	sp := v.Len()
	if sp > 1e-4 {
		v = v.Mul(max(0, sp-mu*g*cos*dt) / sp)
		sp = v.Len()
	}
	// At rest once slow, unless the slope is too steep for the snow to
	// hold them; never longer than slideMaxTime.
	if (sp < slideStopSpeed && sin < mu*1.2*cos) || tb.T >= slideMaxTime {
		return true
	}
	next := mgl32.Vec2{a.Pos[0] + v[0]*dt, a.Pos[2] + v[1]*dt}
	maxX, maxZ := float32(t.Width)*CellSize-0.01, float32(t.Height)*CellSize-0.01
	if next[0] < 0 || next[1] < 0 || next[0] > maxX || next[1] > maxZ {
		return true
	}
	blocked := false
	t.ForEachTreeNear(next[0], next[1], slideTreeStop, func(world.Tree, int, int) { blocked = true })
	if blocked {
		return true
	}
	a.Pos[0], a.Pos[2] = next[0], next[1]
	a.Pos[1] = t.InterpolatedSurfaceElevationAt(a.Pos[0], a.Pos[2])
	tb.Vel = [2]float32{v[0], v[1]}
	return false
}

// bodyFriction is how hard the snow at pos drags a sliding body.
func bodyFriction(t *world.Terrain, pos mgl32.Vec3) float64 {
	cx, cz := int(pos[0]/CellSize), int(pos[2]/CellSize)
	if !t.InBounds(cx, cz) {
		return slideFriction
	}
	top := t.Cells[cx][cz].TopLayer()
	if top == nil {
		return slideFrictionDeep
	}
	switch top.Kind {
	case world.KindBoilerplate:
		return slideFrictionLow
	case world.KindFrozenGranular, world.KindCrust:
		return slideFrictionIcy
	case world.KindPowder, world.KindCement, world.KindSlush:
		return slideFrictionDeep
	}
	return slideFriction
}

// getUpTime is how long the guest takes to get up where they lie.
func (s *Simulation) getUpTime(a *world.Guest) float32 {
	skill := clamp32(a.Traits.Skill, 0, 1)
	d := getUpBase + getUpUnskilled*(1-skill)
	n := s.World.Terrain.NormalAt(a.Pos[0]/CellSize, a.Pos[2]/CellSize)
	slope := float32(math.Acos(float64(clamp32(n[1], -1, 1))))
	d *= 1 + max(0, slope-getUpEasySlope*a.Traits.ComfortSlope)*getUpSlopeScale
	if a.Tumble.Dir == world.FallForward {
		d *= getUpRollOver
	}
	return d
}

// faceAcrossFallLine turns the guest's skis across the slope, the way a
// skier sets them to stand up, on the side nearer where they're going.
func (s *Simulation) faceAcrossFallLine(a *world.Guest) {
	fx, fz := fallLine(s.World.Terrain, a.Pos)
	if fx == 0 && fz == 0 {
		return
	}
	across := mgl32.Vec2{-fz, fx}
	want := mgl32.Vec2{a.Plan.Target[0] - a.Pos[0], a.Plan.Target[2] - a.Pos[2]}
	if want.Dot(across) < 0 {
		across = across.Mul(-1)
	}
	a.Heading = float32(math.Atan2(float64(across[0]), float64(across[1])))
}

// collectSki walks the guest to the nearest ski still lying in the snow
// and picks it up.
func (s *Simulation) collectSki(a *world.Guest, dt float32) {
	tb := &a.Tumble
	best, bestD := -1, float32(math.Inf(1))
	for i := range 2 {
		if !tb.SkisOff[i] {
			continue
		}
		if d := (mgl32.Vec2{tb.Skis[i][0] - a.Pos[0], tb.Skis[i][1] - a.Pos[2]}).Len(); d < bestD {
			best, bestD = i, d
		}
	}
	if bestD <= collectReach {
		tb.SkisOff[best] = false
		a.Speed = 0
		return
	}
	to := mgl32.Vec2{tb.Skis[best][0] - a.Pos[0], tb.Skis[best][1] - a.Pos[2]}.Mul(1 / bestD)
	a.Heading = float32(math.Atan2(float64(to[0]), float64(to[1])))
	// Slower climbing back up in ski boots.
	speed := collectSpeed
	if fx, fz := fallLine(s.World.Terrain, a.Pos); fx != 0 || fz != 0 {
		speed *= 1 - 0.5*max(0, -(to[0]*fx+to[1]*fz))
	}
	step := min(speed*dt, bestD)
	a.Pos[0] += to[0] * step
	a.Pos[2] += to[1] * step
	a.Pos[1] = s.World.Terrain.InterpolatedSurfaceElevationAt(a.Pos[0], a.Pos[2])
	a.Speed = speed
}

// fallLine is the downhill direction at pos (unit XZ), or zero on the
// flat.
func fallLine(t *world.Terrain, pos mgl32.Vec3) (dx, dz float32) {
	n := t.NormalAt(pos[0]/CellSize, pos[2]/CellSize)
	d := mgl32.Vec2{n[0], n[2]}
	if l := d.Len(); l > 0.02 {
		return d[0] / l, d[1] / l
	}
	return 0, 0
}
