package render

import (
	"fmt"
	"math"
	"sort"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// maxLamps mirrors MAX_LAMPS in lighting.glsl.
const maxLamps = 16

// Lamp is a spotlight on the snow and trees (lighting.glsl lampLight).
type Lamp struct {
	Pos      mgl32.Vec3
	Dir      mgl32.Vec3 // unit beam direction
	Range    float32    // metres; light falls to zero here
	CosOuter float32    // cos of the cone's outer half-angle; -1 = all round
	Color    mgl32.Vec3 // colour × intensity
}

// applyLamps uploads lamps to s, scaled by night (0 disables them).
func applyLamps(s *Shader, lamps []Lamp, night float32) {
	if night <= 0 {
		lamps = nil
	}
	lamps = lamps[:min(len(lamps), maxLamps)]
	s.SetInt("uLampCount", len(lamps))
	for i, l := range lamps {
		s.SetVec4(fmt.Sprintf("uLampPos[%d]", i), mgl32.Vec4{l.Pos[0], l.Pos[1], l.Pos[2], l.Range})
		s.SetVec4(fmt.Sprintf("uLampDir[%d]", i), mgl32.Vec4{l.Dir[0], l.Dir[1], l.Dir[2], l.CosOuter})
		s.SetVec3(fmt.Sprintf("uLampColor[%d]", i), l.Color.Mul(night))
	}
}

// Snowcat lamp layout in the model frame (+X forward, +Y up, Z lateral),
// matching snowcat.scad: roof at 3.4 m, cab front at x = 1.2, hull
// front at x = 2.5.
var (
	catHeadlightColor = mgl32.Vec3{1.00, 0.93, 0.80}
	catBeaconColor    = mgl32.Vec3{1.00, 0.55, 0.08}
	catLensOff        = [3]float32{0.25, 0.25, 0.25}

	catLenses = [][3]float32{
		{1.25, 3.25, 0.6}, {1.25, 3.25, -0.6}, // roof light bar
		{2.52, 1.55, 0.85}, {2.52, 1.55, -0.85}, // hull headlights
		{-0.85, 3.25, 0}, // rear work light over the tiller
	}
	catBeacon = [3]float32{0.2, 3.48, 0}
)

// catFrame returns the world-space forward and lateral unit vectors for
// a heading (the rotation dynamic.vert applies).
func catFrame(heading float32) (fwd, side mgl32.Vec3) {
	s, c := float32(math.Sin(float64(heading))), float32(math.Cos(float64(heading)))
	return mgl32.Vec3{s, 0, c}, mgl32.Vec3{-c, 0, s}
}

func catLocal(base, fwd, side mgl32.Vec3, p [3]float32) mgl32.Vec3 {
	return base.Add(fwd.Mul(p[0])).Add(mgl32.Vec3{0, p[1], 0}).Add(side.Mul(p[2]))
}

// beaconPulse is the rotating beacon's brightness (0..1) at time t:
// a short sharp flash about 1.5 times a second.
func beaconPulse(t float32) float32 {
	s := float32(math.Sin(float64(t) * 2 * math.Pi * 1.5))
	if s <= 0 {
		return 0
	}
	return s * s * s * s
}

// snowcatLights builds the lamp list for working cats, nearest the
// camera first (three lamps per cat), plus the emissive lens and beacon
// instances for every cat.
func snowcatLights(w *world.World, cats []*world.Snowcat, camPos mgl32.Vec3, t float32) (lamps []Lamp, lenses, beacons []DynamicInstance) {
	pulse := beaconPulse(t)
	type lit struct {
		base, fwd mgl32.Vec3
		d2        float32
	}
	var working []lit
	for _, cat := range cats {
		base := mgl32.Vec3{cat.Pos[0], VisualElevationAt(w.Terrain, cat.Pos[0], cat.Pos[2]), cat.Pos[2]}
		fwd, side := catFrame(cat.Heading)
		on := w.CatWorking(cat)
		lensCol, beaconCol := catLensOff, [3]float32{0.35, 0.2, 0.05}
		if on {
			lensCol = [3]float32{catHeadlightColor[0], catHeadlightColor[1], catHeadlightColor[2]}
			b := catBeaconColor.Mul(0.3 + 0.7*pulse)
			beaconCol = [3]float32{b[0], b[1], b[2]}
			d := base.Sub(camPos)
			working = append(working, lit{base, fwd, d.Dot(d)})
		}
		for _, p := range catLenses {
			pos := catLocal(base, fwd, side, p)
			lenses = append(lenses, DynamicInstance{Position: pos, Heading: cat.Heading, Color: lensCol})
		}
		pos := catLocal(base, fwd, side, catBeacon)
		beacons = append(beacons, DynamicInstance{Position: pos, Heading: cat.Heading, Color: beaconCol})
	}
	sort.Slice(working, func(i, j int) bool { return working[i].d2 < working[j].d2 })
	down := mgl32.Vec3{0, -1, 0}
	for _, c := range working {
		if len(lamps)+3 > maxLamps {
			break
		}
		lamps = append(lamps,
			Lamp{
				Pos:      c.base.Add(c.fwd.Mul(1.3)).Add(mgl32.Vec3{0, 3.3, 0}),
				Dir:      c.fwd.Add(down.Mul(0.3)).Normalize(),
				Range:    55,
				CosOuter: float32(math.Cos(38 * math.Pi / 180)),
				Color:    catHeadlightColor.Mul(2.2),
			},
			Lamp{
				Pos:      c.base.Sub(c.fwd.Mul(1.0)).Add(mgl32.Vec3{0, 3.3, 0}),
				Dir:      c.fwd.Mul(-1).Add(down.Mul(0.7)).Normalize(),
				Range:    22,
				CosOuter: float32(math.Cos(55 * math.Pi / 180)),
				Color:    catHeadlightColor.Mul(1.0),
			},
			Lamp{
				Pos:      c.base.Add(mgl32.Vec3{0, 3.6, 0}),
				Dir:      down,
				Range:    14,
				CosOuter: -1,
				Color:    catBeaconColor.Mul(0.5 * pulse),
			},
		)
	}
	return lamps, lenses, beacons
}
