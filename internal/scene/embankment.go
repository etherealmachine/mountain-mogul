package scene

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Every graded pad (lift aprons, building pads, painted lots and lodges)
// meets natural ground through an embankment no steeper than
// embankmentGrade. Without it a raised pad on a slope leaves a wall
// that guests heading downhill ski straight off.
const (
	embankmentGrade = float32(0.2) // rise over run, ≈11° — beginners' comfort limit
	embankmentReach = float32(60)  // metres; how far one pad may reshape the hillside
	embankmentToe   = float32(15)  // metres before reach where the ramp eases back to natural
)

// clampToEmbankment pulls cur into the band an embankment dist metres
// from a pad edge at padElev allows. fill raises ground that sits below
// the band; cut lowers ground above it. Over the toe the result fades
// back to cur, so a fill too tall to ramp out within reach ends in a
// steeper slope rather than a wall.
func clampToEmbankment(cur, padElev, dist float32, fill, cut bool) float32 {
	drop := embankmentGrade * dist
	eased := cur
	if fill && cur < padElev-drop {
		eased = padElev - drop
	}
	if cut && cur > padElev+drop {
		eased = padElev + drop
	}
	if toeStart := embankmentReach - embankmentToe; dist > toeStart {
		f := min(1, (dist-toeStart)/embankmentToe)
		eased += (cur - eased) * f
	}
	return eased
}

// fillApronEmbankment raises ground around a station apron's flat inner
// zone (the inner 70 % that buildStationApron grades to full weight) so
// it slopes down to natural terrain at embankmentGrade. Same geometry
// arguments as buildStationApron; claimed cells (painted pads) and
// structure cells are left alone.
func fillApronEmbankment(t *world.Terrain, station, axis mgl32.Vec2, side, halfWidth, depth, target float32, claimed map[[2]int]bool) {
	const cellSize = float32(5.0)
	innerDepth := 0.7 * depth
	innerHalf := 0.7 * halfWidth
	bound := float32(math.Hypot(float64(halfWidth), float64(depth))) + embankmentReach
	x0 := int((station[0] - bound) / cellSize)
	x1 := int((station[0]+bound)/cellSize) + 1
	z0 := int((station[1] - bound) / cellSize)
	z1 := int((station[1]+bound)/cellSize) + 1
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if !t.InBounds(x, z) || claimed[[2]int{x, z}] || !t.Cells[x][z].Passable {
				continue
			}
			cx := (float32(x) + 0.5) * cellSize
			cz := (float32(z) + 0.5) * cellSize
			dx := cx - station[0]
			dz := cz - station[1]
			alongRaw := dx*axis[0] + dz*axis[1]
			signedAlong := alongRaw * side
			perpX := dx - alongRaw*axis[0]
			perpZ := dz - alongRaw*axis[1]
			perpDist := float32(math.Sqrt(float64(perpX*perpX + perpZ*perpZ)))
			outAlong := max(0, signedAlong-innerDepth, -signedAlong)
			outPerp := max(0, perpDist-innerHalf)
			d := float32(math.Hypot(float64(outAlong), float64(outPerp)))
			if d == 0 || d > embankmentReach {
				continue
			}
			cell := &t.Cells[x][z]
			cell.GroundElevation = clampToEmbankment(cell.GroundElevation, target, d, true, false)
		}
	}
}

// claimedGround returns the cells other structures have graded: painted
// pads, building aprons and lift station aprons, except skipBuilding's
// and skipLift's own. Embankments leave these alone so one pad never
// reshapes another.
func claimedGround(w *world.World, skipBuilding *world.Building, skipLift *world.Lift) map[[2]int]bool {
	const cellSize = float32(5.0)
	out := map[[2]int]bool{}
	for _, b := range w.Buildings {
		if b == skipBuilding {
			continue
		}
		if b.IsPainted() {
			for c := range parkingPadCells(w.Terrain, b) {
				out[c] = true
			}
			continue
		}
		halfX, halfZ := buildingFootprint(b.Type)
		rect := world.FootprintRect{
			Center:   b.Pos,
			HalfX:    (halfX + buildingApronBareGround) / apronInnerFraction,
			HalfZ:    (halfZ + buildingApronBareGround) / apronInnerFraction,
			Rotation: b.Rotation,
		}
		minX, minZ, maxX, maxZ := rect.Bounds()
		for x := int(minX / cellSize); x <= int(maxX/cellSize); x++ {
			for z := int(minZ / cellSize); z <= int(maxZ/cellSize); z++ {
				centre := mgl32.Vec2{(float32(x) + 0.5) * cellSize, (float32(z) + 0.5) * cellSize}
				if w.Terrain.InBounds(x, z) && rect.Contains(centre, 0) {
					out[[2]int{x, z}] = true
				}
			}
		}
	}
	for _, l := range w.Lifts {
		if l == skipLift || l.IsHeli() {
			continue
		}
		axis := l.Top.Sub(l.Base)
		if n := axis.Len(); n > 0 {
			axis = axis.Mul(1 / n)
		}
		markApron(w.Terrain, out, l.Top, axis, +1, liftApronHalfWidth, liftApronDepth)
		markApron(w.Terrain, out, l.Base, axis, -1, liftApronHalfWidth, liftApronDepth)
	}
	return out
}

// markApron adds the cells of one station apron rectangle (same geometry
// as buildStationApron) to out.
func markApron(t *world.Terrain, out map[[2]int]bool, station, axis mgl32.Vec2, side, halfWidth, depth float32) {
	const cellSize = float32(5.0)
	bound := float32(math.Hypot(float64(halfWidth), float64(depth)))
	for x := int((station[0] - bound) / cellSize); x <= int((station[0]+bound)/cellSize)+1; x++ {
		for z := int((station[1] - bound) / cellSize); z <= int((station[1]+bound)/cellSize)+1; z++ {
			if !t.InBounds(x, z) {
				continue
			}
			dx := (float32(x)+0.5)*cellSize - station[0]
			dz := (float32(z)+0.5)*cellSize - station[1]
			along := dx*axis[0] + dz*axis[1]
			if s := along * side; s < 0 || s > depth {
				continue
			}
			px, pz := dx-along*axis[0], dz-along*axis[1]
			if px*px+pz*pz <= halfWidth*halfWidth {
				out[[2]int{x, z}] = true
			}
		}
	}
}

// regradeEmbankments eases the ground around every existing lift station
// and painted pad onto an embankment, keeping the pads' current heights.
// Repairs terrain graded before embankments existed.
func regradeEmbankments(w *world.World) {
	t := w.Terrain
	for _, l := range w.Lifts {
		if l.IsHeli() {
			continue
		}
		axis := l.Top.Sub(l.Base)
		if n := axis.Len(); n > 0 {
			axis = axis.Mul(1 / n)
		}
		claimed := claimedGround(w, nil, l)
		fillApronEmbankment(t, l.Top, axis, +1, liftApronHalfWidth, liftApronDepth, stationGroundElev(t, l.Top), claimed)
		fillApronEmbankment(t, l.Base, axis, -1, liftApronHalfWidth, liftApronDepth, stationGroundElev(t, l.Base), claimed)
	}
	for _, b := range w.Buildings {
		switch {
		case b.IsCellLot():
			gradePaintedPad(w, b, parkingMaxGrade)
		case b.IsShell():
			gradePaintedPad(w, b, 0)
		}
	}
	t.RecomputeSlopes()
	t.RestampTreeWells()
}
