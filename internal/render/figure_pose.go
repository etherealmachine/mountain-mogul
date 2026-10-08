package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// figPose is a figure's pose in model space (see figure_mesh.go): where
// the hips, feet, and pole tips are, and which way the body faces. Legs
// are solved to reach the feet (two-bone IK); arms are set by angles from
// the chest. Poses blend with lerpPose.
type figPose struct {
	pelvis    mgl32.Vec3
	pelvisRot mgl32.Quat
	chestRot  mgl32.Quat
	headRot   mgl32.Quat
	ankle     [2]mgl32.Vec3
	footRot   [2]mgl32.Quat
	kneeHint  mgl32.Vec3 // which way the knees bend, before the pelvis turns it
	// Arms: forward swing, outward reach, elbow bend (rad, from the chest).
	armPitch, armOut, elbow [2]float32
	poleTip                 [2]mgl32.Vec3
	// root turns and lifts the whole posed figure (a lie on a slope).
	root mgl32.Quat
	// carry puts both skis over the right shoulder instead of on the feet.
	carry bool
	hide  uint32 // bits of bones not drawn
}

func qx(a float32) mgl32.Quat { return mgl32.QuatRotate(a, mgl32.Vec3{1, 0, 0}) }
func qy(a float32) mgl32.Quat { return mgl32.QuatRotate(a, mgl32.Vec3{0, 1, 0}) }
func qz(a float32) mgl32.Quat { return mgl32.QuatRotate(a, mgl32.Vec3{0, 0, 1}) }

// stanceParams describes a skier on their skis.
type stanceParams struct {
	crouch  float32 // 0 tall to 1 low
	lean    float32 // extra forward bend of the chest, rad
	wedge   float32 // tips in (snowplough), rad each ski
	incline float32 // the whole body leaning into a turn, rad; + to the right
	pitch   float32 // slope along the skis: + downhill ahead, rad
	roll    float32 // slope across: + lower on the right, rad
	plant   [2]float32
}

// groundY is the height of the slope under model (x, z).
func groundY(pitch, roll, x, z float32) float32 {
	return -float32(math.Tan(float64(pitch)))*x - float32(math.Tan(float64(roll)))*z
}

func stancePose(p stanceParams) figPose {
	var f figPose
	slope := qz(-p.pitch).Mul(qx(p.roll))
	width := figLegZ + 0.05*p.wedge/0.25
	var midY float32
	for s := range 2 {
		z := sideSign[s] * width
		f.ankle[s] = mgl32.Vec3{0, groundY(p.pitch, p.roll, 0, z) + figAnkleY, z}
		f.footRot[s] = slope.Mul(qy(sideSign[s] * p.wedge))
		midY += f.ankle[s][1] / 2
	}
	c := p.crouch
	// Hips over the feet, dropping and sitting back as they crouch; the
	// body stays upright against gravity, not the slope.
	f.pelvis = mgl32.Vec3{-0.06 - 0.12*c, midY + 0.83 - 0.28*c, 0}
	f.pelvisRot = qz(-(0.12 + 0.3*c))
	bend := 0.2 + 0.35*c + p.lean
	f.chestRot = qz(-bend).Mul(qx(-0.4 * p.incline))
	f.headRot = f.chestRot.Mul(qz(0.7 * bend))
	f.kneeHint = mgl32.Vec3{1, 0, 0}
	for s := range 2 {
		f.armPitch[s] = 0.45 + 0.25*c + 0.5*p.plant[s]
		f.armOut[s] = 0.25
		f.elbow[s] = 0.7 - 0.3*p.plant[s]
		tip := mgl32.Vec3{-0.45 + 1.0*p.plant[s], 0, sideSign[s] * (0.45 - 0.1*p.plant[s])}
		tip[1] = groundY(p.pitch, p.roll, tip[0], tip[2])
		f.poleTip[s] = tip
	}
	f.root = mgl32.QuatIdent()
	if p.incline != 0 {
		f.root = qx(p.incline)
	}
	return f
}

// walkPose is a guest walking in boots: stride is the phase of their
// step, stride amplitude grows with speed. carry shoulders their skis,
// the right hand holding them, the left arm still swinging.
func walkPose(phase, speed, pitch, roll float32, carry bool) figPose {
	var f figPose
	amp := min(speed/1.2, 1) * 0.22
	slope := qz(-pitch).Mul(qx(roll))
	var midY float32
	for s := range 2 {
		ph := float64(phase) + math.Pi*float64(s)
		x := amp * float32(math.Sin(ph))
		z := sideSign[s] * 0.1
		lift := max(0, float32(math.Cos(ph))) * amp * 0.3
		f.ankle[s] = mgl32.Vec3{x, groundY(pitch, roll, x, z) + figAnkleY + lift, z}
		f.footRot[s] = slope
		midY += f.ankle[s][1] / 2
		f.armPitch[s] = -1.3 * amp * float32(math.Sin(ph))
		f.armOut[s] = 0.12
		f.elbow[s] = 0.3
	}
	bob := float32(math.Abs(math.Cos(float64(phase)))) * amp * 0.15
	f.pelvis = mgl32.Vec3{0, midY + 0.82 + bob, 0}
	f.pelvisRot = qz(-0.05)
	f.chestRot = qz(-0.08)
	f.headRot = f.chestRot.Mul(qz(0.05))
	f.kneeHint = mgl32.Vec3{1, 0, 0}
	f.root = mgl32.QuatIdent()
	f.hide = 1<<boneSkiL | 1<<boneSkiR | 1<<bonePoleL | 1<<bonePoleR
	if carry {
		f.carry = true
		f.hide = 1<<bonePoleL | 1<<bonePoleR
		f.armPitch[1], f.elbow[1], f.armOut[1] = 0.6, 1.87, 0.05
	}
	return f
}

// Shouldered skis: how far the pair tips down in front (rad), and where
// each ski rests across the right shoulder (chest frame, from the waist).
const (
	carryTilt   = float32(0.35)
	carryHeight = figShoulderY - figWaistY + 0.06
)

var carryZ = [2]float32{0.20, 0.29}

// chairPose sits on a chair, boots on the footrest at the origin.
func chairPose() figPose {
	var f figPose
	for s := range 2 {
		z := sideSign[s] * figLegZ
		f.ankle[s] = mgl32.Vec3{0.1, figAnkleY, z}
		f.footRot[s] = qz(-0.08)
		f.armPitch[s] = 0.7
		f.armOut[s] = 0.1
		f.elbow[s] = 0.9
		f.poleTip[s] = mgl32.Vec3{0.55, -0.4, sideSign[s] * 0.3}
	}
	f.pelvis = mgl32.Vec3{-0.2, 0.6, 0}
	f.pelvisRot = qz(0.05)
	f.chestRot = qz(0.08)
	f.headRot = f.chestRot.Mul(qz(-0.08))
	f.kneeHint = mgl32.Vec3{1, 0.6, 0}
	f.root = mgl32.QuatIdent()
	return f
}

// lyingPose is a guest down in the snow: dir as world.FallDir (forward,
// back, side, thrown), side the hip they went down on (-1 left, +1
// right).
func lyingPose(dir int, side float32) figPose {
	var f figPose
	f.root = mgl32.QuatIdent()
	for s := range 2 {
		f.armOut[s] = 0.9
		f.armPitch[s] = 0.4
		f.elbow[s] = 0.5
	}
	switch dir {
	case 0: // forward: face down, head ahead, legs trailing
		f.pelvis = mgl32.Vec3{0.25, 0.16, 0}
		f.pelvisRot = qz(-1.45)
		f.chestRot = qz(-1.5).Mul(qx(0.1 * side))
		f.headRot = f.chestRot.Mul(qz(0.5))
		for s := range 2 {
			z := sideSign[s] * 0.18
			f.ankle[s] = mgl32.Vec3{-0.5, 0.2, z}
			f.footRot[s] = qy(sideSign[s] * 0.35).Mul(qx(sideSign[s] * 0.3))
			f.armPitch[s] = 2.2
		}
		f.kneeHint = mgl32.Vec3{1, 0, 0}
	case 1, 3: // sat back, then onto their back; thrown off a trunk the same, sprawled
		f.pelvis = mgl32.Vec3{-0.45, 0.14, 0}
		f.pelvisRot = qz(1.1)
		recline := float32(1.3)
		if dir == 3 {
			recline = 1.5
			for s := range 2 {
				f.armOut[s] = 1.3
			}
		}
		f.chestRot = qz(recline).Mul(qx(0.15 * side))
		f.headRot = f.chestRot.Mul(qz(-0.5))
		for s := range 2 {
			z := sideSign[s] * 0.17
			f.ankle[s] = mgl32.Vec3{0.25 - 0.1*float32(s), 0.18, z}
			f.footRot[s] = qy(sideSign[s] * -0.2)
		}
		f.kneeHint = mgl32.Vec3{1, 0, 0}
	default: // on their hip, legs bent, skis on edge
		f.pelvis = mgl32.Vec3{-0.15, 0.17, side * 0.15}
		f.pelvisRot = qx(side * 1.35)
		f.chestRot = qx(side * 1.4).Mul(qz(-0.3))
		f.headRot = f.chestRot.Mul(qx(-side * 0.4))
		for s := range 2 {
			lower := sideSign[s] == side
			y := float32(0.32)
			if lower {
				y = 0.17
			}
			f.ankle[s] = mgl32.Vec3{0.25, y, -side * 0.55}
			f.footRot[s] = qx(side * 1.2)
		}
		f.kneeHint = mgl32.Vec3{1, 0, 0}
		f.armOut[0], f.armOut[1] = 0.3, 0.3
	}
	for s := range 2 {
		f.poleTip[s] = mgl32.Vec3{f.pelvis[0] + 0.3, 0, sideSign[s] * 1.0}
	}
	return f
}

// snowSitPose sits in the snow, skis off: a guest who gave up.
func snowSitPose() figPose {
	f := lyingPose(1, 0)
	f.pelvisRot = qz(0.5)
	f.chestRot = qz(0.35)
	f.headRot = f.chestRot.Mul(qz(-0.2))
	for s := range 2 {
		f.ankle[s] = mgl32.Vec3{0.25, 0.17, sideSign[s] * 0.15}
		f.armPitch[s] = 0.9
		f.armOut[s] = 0.15
		f.elbow[s] = 1.2
	}
	f.kneeHint = mgl32.Vec3{1, 0.5, 0}
	f.hide = 1<<boneSkiL | 1<<boneSkiR | 1<<bonePoleL | 1<<bonePoleR
	return f
}

// lerpPose blends a toward b by t (0..1). Hidden bones are a's until
// the last stretch of the blend.
func lerpPose(a, b figPose, t float32) figPose {
	t = clampF(t, 0, 1)
	slerp := func(p, q mgl32.Quat) mgl32.Quat { return mgl32.QuatNlerp(p, q, t) }
	lerp := func(p, q mgl32.Vec3) mgl32.Vec3 { return p.Add(q.Sub(p).Mul(t)) }
	f := figPose{
		pelvis:    lerp(a.pelvis, b.pelvis),
		pelvisRot: slerp(a.pelvisRot, b.pelvisRot),
		chestRot:  slerp(a.chestRot, b.chestRot),
		headRot:   slerp(a.headRot, b.headRot),
		kneeHint:  lerp(a.kneeHint, b.kneeHint),
		root:      slerp(a.root, b.root),
		hide:      a.hide,
	}
	if t > 0.9 {
		f.hide = b.hide
	}
	for s := range 2 {
		f.ankle[s] = lerp(a.ankle[s], b.ankle[s])
		f.footRot[s] = slerp(a.footRot[s], b.footRot[s])
		f.armPitch[s] = a.armPitch[s] + (b.armPitch[s]-a.armPitch[s])*t
		f.armOut[s] = a.armOut[s] + (b.armOut[s]-a.armOut[s])*t
		f.elbow[s] = a.elbow[s] + (b.elbow[s]-a.elbow[s])*t
		f.poleTip[s] = lerp(a.poleTip[s], b.poleTip[s])
	}
	return f
}

func clampF(v, lo, hi float32) float32 { return max(lo, min(hi, v)) }

// easeStep eases 0..1.
func easeStep(t float32) float32 {
	t = clampF(t, 0, 1)
	return t * t * (3 - 2*t)
}

// boneFrame is a bone's posed frame: joint position and orientation.
type boneFrame struct {
	at  mgl32.Vec3
	rot mgl32.Mat3
}

// frameAlong is the orientation whose +Y runs along up (unit) with +X as
// near fwd as it can be.
func frameAlong(up, fwd mgl32.Vec3) mgl32.Mat3 {
	x := fwd.Sub(up.Mul(fwd.Dot(up)))
	if x.Len() < 1e-4 {
		x = mgl32.Vec3{1, 0, 0}.Sub(up.Mul(up[0]))
		if x.Len() < 1e-4 {
			x = mgl32.Vec3{0, 0, 1}
		}
	}
	x = x.Normalize()
	z := x.Cross(up)
	return mgl32.Mat3FromCols(x, up, z)
}

// solve poses every bone: each bone's matrix takes its rest-pose vertices
// to model space. Hidden bones get a zero matrix, which collapses their
// triangles.
func (f *figPose) solve(out *[figureBones]mgl32.Mat4) {
	var fr [figureBones]boneFrame
	pr := f.pelvisRot.Mat4().Mat3()
	fr[bonePelvis] = boneFrame{f.pelvis, pr}
	waist := f.pelvis.Add(pr.Mul3x1(mgl32.Vec3{0, figWaistY - figHipY, 0}))
	cr := f.chestRot.Mat4().Mat3()
	fr[boneChest] = boneFrame{waist, cr}
	fr[boneHead] = boneFrame{waist.Add(cr.Mul3x1(mgl32.Vec3{0, figNeckY - figWaistY, 0})), f.headRot.Mat4().Mat3()}
	hint := pr.Mul3x1(f.kneeHint)
	for s := range 2 {
		z := sideSign[s]
		hip := f.pelvis.Add(pr.Mul3x1(mgl32.Vec3{0, 0, z * figLegZ}))
		ankle := f.ankle[s]
		knee := solveKnee(hip, ankle, figThighLen, figShinLen, hint)
		fr[boneThighL+s] = boneFrame{hip, frameAlong(hip.Sub(knee).Normalize(), hint)}
		fr[boneShinL+s] = boneFrame{knee, frameAlong(knee.Sub(ankle).Normalize(), hint)}
		foot := f.footRot[s].Mat4().Mat3()
		fr[boneBootL+s] = boneFrame{ankle, foot}
		fr[boneSkiL+s] = boneFrame{ankle, foot}
		if f.carry {
			// The ski's joint sits figAnkleY above its base: lift it so the
			// base rests on the shoulder.
			at := waist.Add(cr.Mul3x1(mgl32.Vec3{0, carryHeight + figAnkleY, carryZ[s]}))
			fr[boneSkiL+s] = boneFrame{at, cr.Mul3(qz(-carryTilt).Mat4().Mat3())}
		}
		// Arms swing from the shoulder; the pole runs from the hand to its
		// tip.
		shoulder := waist.Add(cr.Mul3x1(mgl32.Vec3{0, figShoulderY - figWaistY, z * figShoulderZ}))
		upper := cr.Mul3(qz(f.armPitch[s]).Mat4().Mat3()).Mul3(qx(-z * f.armOut[s]).Mat4().Mat3())
		fr[boneUpperArmL+s] = boneFrame{shoulder, upper}
		elbow := shoulder.Add(upper.Mul3x1(mgl32.Vec3{0, -figUpperArm, 0}))
		fore := upper.Mul3(qz(f.elbow[s]).Mat4().Mat3())
		fr[boneForearmL+s] = boneFrame{elbow, fore}
		hand := elbow.Add(fore.Mul3x1(mgl32.Vec3{0, -figForearm, 0}))
		pole := hand.Sub(f.poleTip[s])
		if pole.Len() < 1e-3 {
			pole = mgl32.Vec3{0, 1, 0}
		}
		fr[bonePoleL+s] = boneFrame{hand, frameAlong(pole.Normalize(), mgl32.Vec3{1, 0, 0})}
	}
	root := f.root.Mat4()
	for b := range figureBones {
		if f.hide&(1<<b) != 0 {
			out[b] = mgl32.Mat4{}
			continue
		}
		rest := figureRestJoint(b)
		m := mgl32.Translate3D(fr[b].at[0], fr[b].at[1], fr[b].at[2]).
			Mul4(fr[b].rot.Mat4()).
			Mul4(mgl32.Translate3D(-rest[0], -rest[1], -rest[2]))
		out[b] = root.Mul4(m)
	}
}

// solveKnee places the knee between hip and ankle for bones of length
// l1 (thigh) and l2 (shin), bending toward hint. Out of reach, the leg
// points straight at the ankle.
func solveKnee(hip, ankle mgl32.Vec3, l1, l2 float32, hint mgl32.Vec3) mgl32.Vec3 {
	d := ankle.Sub(hip)
	dist := d.Len()
	if dist < 1e-4 {
		return hip.Add(hint.Normalize().Mul(l1))
	}
	dir := d.Mul(1 / dist)
	if dist >= l1+l2 {
		return hip.Add(dir.Mul(l1))
	}
	dist = max(dist, abs32f(l1-l2)+1e-3)
	cosA := (l1*l1 + dist*dist - l2*l2) / (2 * l1 * dist)
	a := float32(math.Acos(float64(clampF(cosA, -1, 1))))
	perp := hint.Sub(dir.Mul(hint.Dot(dir)))
	if perp.Len() < 1e-4 {
		perp = mgl32.Vec3{1, 0, 0}.Sub(dir.Mul(dir[0]))
	}
	perp = perp.Normalize()
	return hip.Add(dir.Mul(l1 * float32(math.Cos(float64(a))))).Add(perp.Mul(l1 * float32(math.Sin(float64(a)))))
}

func abs32f(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
