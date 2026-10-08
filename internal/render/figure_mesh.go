package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// The figure is the one jointed person mesh every guest and patroller is
// drawn with: on skis, walking, sitting on a chair, or down in the snow.
// Each vertex belongs to one bone (posed per instance, figure_pose.go) and
// takes its colour from one slot (the instance's outfit, or a fixed
// colour; see figure.vert).
//
// Model space: +X forward, +Y up, +Z to the figure's right, origin on the
// ground between the feet. Sizes are an adult of about 1.75 m in boots.

// Bones.
const (
	bonePelvis = iota
	boneChest
	boneHead
	boneThighL
	boneThighR
	boneShinL
	boneShinR
	boneBootL
	boneBootR
	boneSkiL
	boneSkiR
	boneUpperArmL
	boneUpperArmR
	boneForearmL
	boneForearmR
	bonePoleL
	bonePoleR
	figureBones
)

// Colour slots: the first figureOutfitSlots come from the instance's
// outfit, the rest are fixed in figure.vert.
const (
	slotJacket = iota
	slotPants
	slotHelmet
	slotSkis
	slotSkin
	slotBoots
	slotGloves
	slotGoggles
	slotPoles
	figureOutfitSlots = slotSkin + 1
)

// Rest-pose joints, standing straight (y up). Side-indexed values are
// left (0) then right (1); sideSign gives each side's Z.
const (
	figAnkleY    = 0.15
	figKneeY     = 0.60
	figHipY      = 1.00
	figWaistY    = 1.08
	figShoulderY = 1.44
	figNeckY     = 1.52
	figElbowY    = 1.16
	figHandY     = 0.89
	figLegZ      = 0.12
	figShoulderZ = 0.24
	figThighLen  = figHipY - figKneeY
	figShinLen   = figKneeY - figAnkleY
	figUpperArm  = figShoulderY - figElbowY
	figForearm   = figElbowY - figHandY
	figPoleLen   = 1.10
	figSkiLen    = 1.65
)

var sideSign = [2]float32{-1, 1}

// figureRestJoint is where each bone's joint sits in the rest pose: the
// point the bone turns about.
func figureRestJoint(b int) mgl32.Vec3 {
	side := 0
	switch b {
	case boneThighR, boneShinR, boneBootR, boneSkiR, boneUpperArmR, boneForearmR, bonePoleR:
		side = 1
	}
	z := sideSign[side]
	switch b {
	case bonePelvis:
		return mgl32.Vec3{0, figHipY, 0}
	case boneChest:
		return mgl32.Vec3{0, figWaistY, 0}
	case boneHead:
		return mgl32.Vec3{0, figNeckY, 0}
	case boneThighL, boneThighR:
		return mgl32.Vec3{0, figHipY, z * figLegZ}
	case boneShinL, boneShinR:
		return mgl32.Vec3{0, figKneeY, z * figLegZ}
	case boneBootL, boneBootR, boneSkiL, boneSkiR:
		return mgl32.Vec3{0, figAnkleY, z * figLegZ}
	case boneUpperArmL, boneUpperArmR:
		return mgl32.Vec3{0, figShoulderY, z * figShoulderZ}
	case boneForearmL, boneForearmR:
		return mgl32.Vec3{0, figElbowY, z * figShoulderZ}
	}
	return mgl32.Vec3{0, figHandY, z * figShoulderZ} // poles
}

// figureMeshBuilder collects vertices: position, normal, bone, slot.
type figureMeshBuilder struct {
	verts []float32
	idx   []uint32
}

func (m *figureMeshBuilder) vert(p, n mgl32.Vec3, bone, slot int) uint32 {
	i := uint32(len(m.verts) / 8)
	m.verts = append(m.verts, p[0], p[1], p[2], n[0], n[1], n[2], float32(bone), float32(slot))
	return i
}

// box adds an axis-aligned box.
func (m *figureMeshBuilder) box(c, size mgl32.Vec3, bone, slot int) {
	h := size.Mul(0.5)
	faces := [6]struct{ n, u, v mgl32.Vec3 }{
		{mgl32.Vec3{1, 0, 0}, mgl32.Vec3{0, 0, -1}, mgl32.Vec3{0, 1, 0}},
		{mgl32.Vec3{-1, 0, 0}, mgl32.Vec3{0, 0, 1}, mgl32.Vec3{0, 1, 0}},
		{mgl32.Vec3{0, 1, 0}, mgl32.Vec3{1, 0, 0}, mgl32.Vec3{0, 0, -1}},
		{mgl32.Vec3{0, -1, 0}, mgl32.Vec3{1, 0, 0}, mgl32.Vec3{0, 0, 1}},
		{mgl32.Vec3{0, 0, 1}, mgl32.Vec3{1, 0, 0}, mgl32.Vec3{0, 1, 0}},
		{mgl32.Vec3{0, 0, -1}, mgl32.Vec3{-1, 0, 0}, mgl32.Vec3{0, 1, 0}},
	}
	for _, f := range faces {
		scale := func(v mgl32.Vec3) mgl32.Vec3 { return mgl32.Vec3{v[0] * h[0], v[1] * h[1], v[2] * h[2]} }
		o := c.Add(scale(f.n))
		u, v := scale(f.u), scale(f.v)
		a := m.vert(o.Sub(u).Sub(v), f.n, bone, slot)
		b := m.vert(o.Add(u).Sub(v), f.n, bone, slot)
		cc := m.vert(o.Add(u).Add(v), f.n, bone, slot)
		d := m.vert(o.Sub(u).Add(v), f.n, bone, slot)
		m.idx = append(m.idx, a, b, cc, a, cc, d)
	}
}

// cylinder adds a capped cylinder standing on the Y axis at (x, z).
func (m *figureMeshBuilder) cylinder(x, z, y0, y1, r float32, seg, bone, slot int) {
	ring := func(y float32, n func(dx, dz float32) mgl32.Vec3) []uint32 {
		out := make([]uint32, seg)
		for i := range seg {
			a := float64(i) / float64(seg) * 2 * math.Pi
			dx, dz := float32(math.Cos(a)), float32(math.Sin(a))
			out[i] = m.vert(mgl32.Vec3{x + dx*r, y, z + dz*r}, n(dx, dz), bone, slot)
		}
		return out
	}
	side := func(dx, dz float32) mgl32.Vec3 { return mgl32.Vec3{dx, 0, dz} }
	lo, hi := ring(y0, side), ring(y1, side)
	for i := range seg {
		j := (i + 1) % seg
		m.idx = append(m.idx, lo[i], hi[j], lo[j], lo[i], hi[i], hi[j])
	}
	for _, cap := range []struct {
		y  float32
		ny float32
	}{{y0, -1}, {y1, 1}} {
		rim := ring(cap.y, func(float32, float32) mgl32.Vec3 { return mgl32.Vec3{0, cap.ny, 0} })
		c := m.vert(mgl32.Vec3{x, cap.y, z}, mgl32.Vec3{0, cap.ny, 0}, bone, slot)
		for i := range seg {
			j := (i + 1) % seg
			if cap.ny > 0 {
				m.idx = append(m.idx, c, rim[j], rim[i])
			} else {
				m.idx = append(m.idx, c, rim[i], rim[j])
			}
		}
	}
}

// sphere adds a sphere, or its top half when top is set.
func (m *figureMeshBuilder) sphere(c mgl32.Vec3, r float32, top bool, bone, slot int) {
	const seg, rings = 10, 6
	lat0 := -math.Pi / 2
	if top {
		lat0 = 0
	}
	grid := make([][]uint32, rings+1)
	for i := range rings + 1 {
		lat := lat0 + (math.Pi/2-lat0)*float64(i)/rings
		grid[i] = make([]uint32, seg)
		for j := range seg {
			lon := float64(j) / seg * 2 * math.Pi
			n := mgl32.Vec3{float32(math.Cos(lat) * math.Cos(lon)), float32(math.Sin(lat)), float32(math.Cos(lat) * math.Sin(lon))}
			grid[i][j] = m.vert(c.Add(n.Mul(r)), n, bone, slot)
		}
	}
	for i := range rings {
		for j := range seg {
			k := (j + 1) % seg
			m.idx = append(m.idx, grid[i][j], grid[i+1][k], grid[i][k], grid[i][j], grid[i+1][j], grid[i+1][k])
		}
	}
}

// buildFigureMesh lays out the figure in its rest pose.
func buildFigureMesh() (verts []float32, idx []uint32) {
	m := &figureMeshBuilder{}
	// Hips and torso.
	m.box(mgl32.Vec3{0, figHipY, 0}, mgl32.Vec3{0.24, 0.20, 0.36}, bonePelvis, slotPants)
	m.box(mgl32.Vec3{0, 1.27, 0}, mgl32.Vec3{0.27, 0.40, 0.42}, boneChest, slotJacket)
	m.box(mgl32.Vec3{0, 1.48, 0}, mgl32.Vec3{0.18, 0.06, 0.24}, boneChest, slotJacket) // collar
	// Head: neck, face, helmet, goggles.
	m.cylinder(0, 0, 1.47, 1.56, 0.05, 8, boneHead, slotSkin)
	m.sphere(mgl32.Vec3{0.01, 1.64, 0}, 0.115, false, boneHead, slotSkin)
	m.sphere(mgl32.Vec3{-0.01, 1.655, 0}, 0.13, true, boneHead, slotHelmet)
	m.box(mgl32.Vec3{0.105, 1.665, 0}, mgl32.Vec3{0.05, 0.065, 0.21}, boneHead, slotGoggles)
	for s := range 2 {
		z := sideSign[s]
		// Legs.
		m.cylinder(0, z*figLegZ, figKneeY, figHipY, 0.08, 8, boneThighL+s, slotPants)
		m.cylinder(0, z*figLegZ, 0.22, figKneeY+0.02, 0.07, 8, boneShinL+s, slotPants)
		m.box(mgl32.Vec3{0.02, 0.135, z * figLegZ}, mgl32.Vec3{0.30, 0.21, 0.13}, boneBootL+s, slotBoots)
		// Ski, with the tip turned up.
		m.box(mgl32.Vec3{0.03, 0.015, z * figLegZ}, mgl32.Vec3{figSkiLen, 0.03, 0.09}, boneSkiL+s, slotSkis)
		m.box(mgl32.Vec3{0.03 + figSkiLen/2 + 0.04, 0.05, z * figLegZ}, mgl32.Vec3{0.08, 0.06, 0.09}, boneSkiL+s, slotSkis)
		// Arms and gloves.
		m.cylinder(0, z*figShoulderZ, figElbowY, figShoulderY+0.02, 0.06, 8, boneUpperArmL+s, slotJacket)
		m.cylinder(0, z*figShoulderZ, figHandY+0.03, figElbowY+0.01, 0.055, 8, boneForearmL+s, slotJacket)
		m.sphere(mgl32.Vec3{0, figHandY, z * figShoulderZ}, 0.055, false, boneForearmL+s, slotGloves)
		// Pole: shaft from the hand down, a basket near the tip.
		m.cylinder(0, z*figShoulderZ, figHandY-figPoleLen, figHandY, 0.013, 6, bonePoleL+s, slotPoles)
		m.cylinder(0, z*figShoulderZ, figHandY-figPoleLen+0.08, figHandY-figPoleLen+0.095, 0.05, 8, bonePoleL+s, slotGloves)
	}
	return m.verts, m.idx
}
