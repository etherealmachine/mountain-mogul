package render

import (
	"math"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Figures: every guest and patroller is the one jointed mesh
// (figure_mesh.go) in a pose worked out each frame from what the sim says
// they're doing (figure_pose.go). Each instance's bone matrices and
// outfit go into a buffer texture that figure.vert reads by instance.
const (
	figureTexUnit = 12
	// Texels per instance: three rows per bone, the outfit, and a tint
	// (rgb, mix). Must match STRIDE in figure.vert and figure_shadow.vert.
	figureTexels = figureBones*3 + figureOutfitSlots + 1
)

// figures holds the figure mesh and this frame's instances.
type figures struct {
	mesh    *Mesh
	buf     uint32 // texture buffer storage
	tex     uint32
	data    []float32
	count   int32
	shader  *Shader
	anim    map[uint64]*figureAnim // guests
	patrol  map[uint64]*figureAnim // patrollers
	frame   uint32
	simTime float64
}

// figureAnim is what a figure's pose carries from frame to frame: the
// smoothed heading and lean, the step it's on, and how it last stood (a
// fall starts from there).
type figureAnim struct {
	frame    uint32
	heading  float32
	prevHead float32
	prevPos  mgl32.Vec3
	turnRate float32
	crouch   float32
	stride   float32
	plant    [2]float32
	turnSide int8
	upright  figPose
	hasPose  bool
}

func (f *figures) init(shaderDir string) error {
	verts, idx := buildFigureMesh()
	f.mesh = NewMesh(verts, idx, []int{3, 3, 1, 1}, nil)
	var err error
	if f.shader, err = LoadShader(shaderDir+"figure.vert", shaderDir+"dynamic.frag", shaderDir+"lighting.glsl"); err != nil {
		return err
	}
	gl.GenBuffers(1, &f.buf)
	gl.GenTextures(1, &f.tex)
	f.anim = map[uint64]*figureAnim{}
	f.patrol = map[uint64]*figureAnim{}
	return nil
}

// outfit is a guest's clothes and gear: jacket, pants, helmet, skis, and
// skin, picked from their ID so they keep them all day. Beginners are on
// rental skis.
func outfit(id uint64, skill float32) [figureOutfitSlots]mgl32.Vec3 {
	jackets := [...]mgl32.Vec3{
		{0.85, 0.18, 0.15}, {0.12, 0.35, 0.80}, {0.95, 0.75, 0.10}, {0.15, 0.60, 0.30},
		{0.95, 0.45, 0.10}, {0.50, 0.20, 0.65}, {0.10, 0.60, 0.65}, {0.90, 0.35, 0.55},
		{0.12, 0.12, 0.14}, {0.90, 0.90, 0.88}, {0.55, 0.75, 0.20}, {0.25, 0.25, 0.45},
	}
	pants := [...]mgl32.Vec3{
		{0.10, 0.10, 0.12}, {0.15, 0.18, 0.32}, {0.35, 0.35, 0.38}, {0.30, 0.32, 0.20},
		{0.85, 0.85, 0.82}, {0.45, 0.15, 0.15},
	}
	helmets := [...]mgl32.Vec3{
		{0.10, 0.10, 0.11}, {0.92, 0.92, 0.90}, {0.85, 0.15, 0.12}, {0.15, 0.30, 0.70},
		{0.60, 0.62, 0.65}, {0.95, 0.70, 0.10},
	}
	skis := [...]mgl32.Vec3{
		{0.10, 0.10, 0.12}, {0.85, 0.20, 0.15}, {0.15, 0.45, 0.85}, {0.95, 0.85, 0.20},
		{0.20, 0.70, 0.40}, {0.92, 0.92, 0.90}, {0.95, 0.45, 0.10},
	}
	skins := [...]mgl32.Vec3{
		{0.96, 0.80, 0.68}, {0.88, 0.68, 0.52}, {0.72, 0.52, 0.38}, {0.55, 0.38, 0.26},
		{0.38, 0.26, 0.18},
	}
	h := id*0x9E3779B97F4A7C15 + 0x632BE59BD9B4E019
	pick := func(n int) int {
		h ^= h >> 29
		h *= 0xBF58476D1CE4E5B9
		return int(h>>33) % n
	}
	o := [figureOutfitSlots]mgl32.Vec3{
		jackets[pick(len(jackets))],
		pants[pick(len(pants))],
		helmets[pick(len(helmets))],
		skis[pick(len(skis))],
		skins[pick(len(skins))],
	}
	if skill < 0.33 {
		o[slotSkis] = mgl32.Vec3{0.55, 0.58, 0.62} // rental grey
	}
	return o
}

// patrolOutfit is ski patrol: red jacket, black pants, red helmet.
var patrolOutfit = [figureOutfitSlots]mgl32.Vec3{
	{0.80, 0.08, 0.06}, {0.08, 0.08, 0.09}, {0.80, 0.08, 0.06}, {0.10, 0.10, 0.12}, {0.90, 0.72, 0.58},
}

// liftStaffOutfit is lift staff: the resort's navy jacket with a hi-vis
// yellow, dark pants, and a black toque.
var liftStaffOutfit = [figureOutfitSlots]mgl32.Vec3{
	{0.95, 0.80, 0.10}, {0.10, 0.11, 0.16}, {0.08, 0.08, 0.09}, {0.10, 0.10, 0.12}, {0.90, 0.72, 0.58},
}

// headingBasis turns model space (+X forward) to face heading, the same
// way dynamic.vert does.
func headingBasis(h float32) mgl32.Mat4 {
	s, c := float32(math.Sin(float64(h))), float32(math.Cos(float64(h)))
	return mgl32.Mat4{s, 0, c, 0, 0, 1, 0, 0, -c, 0, s, 0, 0, 0, 0, 1}
}

// add appends one figure instance: bone matrices (model to world through
// place), outfit, and tint (rgb, mix).
func (f *figures) add(place mgl32.Mat4, bones *[figureBones]mgl32.Mat4, o *[figureOutfitSlots]mgl32.Vec3, tint mgl32.Vec4) {
	for b := range figureBones {
		m := bones[b]
		if m == (mgl32.Mat4{}) {
			f.data = append(f.data, make([]float32, 12)...)
			continue
		}
		m = place.Mul4(m)
		for r := range 3 {
			f.data = append(f.data, m[r], m[4+r], m[8+r], m[12+r])
		}
	}
	for _, c := range o {
		f.data = append(f.data, c[0], c[1], c[2], 1)
	}
	f.data = append(f.data, tint[0], tint[1], tint[2], tint[3])
	f.count++
}

// slopeAngles is the slope under pos as pitch along heading (+ downhill
// ahead) and roll across it (+ lower on the right), and the slope normal
// in model space.
func slopeAngles(t *world.Terrain, pos mgl32.Vec3, heading float32) (pitch, roll float32, n mgl32.Vec3) {
	wn := t.NormalAt(pos[0]/world.CellSize, pos[2]/world.CellSize)
	if wn[1] < 0.2 {
		return 0, 0, mgl32.Vec3{0, 1, 0}
	}
	fx, fz := float32(math.Sin(float64(heading))), float32(math.Cos(float64(heading)))
	rx, rz := -fz, fx
	along := wn[0]*fx + wn[2]*fz
	across := wn[0]*rx + wn[2]*rz
	pitch = float32(math.Atan(float64(along / wn[1])))
	roll = float32(math.Atan(float64(across / wn[1])))
	return pitch, roll, mgl32.Vec3{along, wn[1], across}
}

// liesOnSlope tilts a pose lying in the snow to the slope, by amount t.
func liesOnSlope(p *figPose, n mgl32.Vec3, t float32) {
	q := mgl32.QuatBetweenVectors(mgl32.Vec3{0, 1, 0}, n.Normalize())
	p.root = mgl32.QuatNlerp(mgl32.QuatIdent(), q, clampF(t, 0, 1)).Mul(p.root)
}

// animFor returns the figure's carried state, creating it facing heading.
func (f *figures) animFor(m map[uint64]*figureAnim, id uint64, pos mgl32.Vec3, heading float32) *figureAnim {
	a := m[id]
	if a == nil {
		a = &figureAnim{heading: heading, prevHead: heading, prevPos: pos, crouch: 0.3}
		m[id] = a
	}
	a.frame = f.frame
	return a
}

// steer advances the smoothed heading and turn rate by dt sim seconds.
func (a *figureAnim) steer(heading, dt float32) {
	if dt <= 0 {
		return
	}
	rate := wrapAngleF(heading-a.prevHead) / dt
	a.prevHead = heading
	k := clampF(dt*6, 0, 1)
	a.turnRate += (rate - a.turnRate) * k
	a.heading += wrapAngleF(heading-a.heading) * clampF(dt*12, 0, 1)
}

func wrapAngleF(a float32) float32 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

// skiPose is a skier going along at speed: crouching lower the faster,
// leaning into turns, beginners in a wedge, poles planted as turns link.
func (a *figureAnim) skiPose(t *world.Terrain, pos mgl32.Vec3, speed, skill float32, turnSide int8, dt float32) figPose {
	if turnSide != a.turnSide && turnSide != 0 {
		s := 0
		if turnSide > 0 {
			s = 1
		}
		a.plant[s] = 1
	}
	a.turnSide = turnSide
	for s := range 2 {
		a.plant[s] = max(0, a.plant[s]-dt*3)
	}
	target := 0.15 + 0.5*clampF(speed/14, 0, 1)
	a.crouch += (target - a.crouch) * clampF(dt*3, 0, 1)
	pitch, roll, _ := slopeAngles(t, pos, a.heading)
	incline := -float32(math.Atan(float64(speed * a.turnRate / 9.81)))
	incline = clampF(incline, -0.6, 0.6) * (0.4 + 0.6*clampF(skill, 0, 1))
	var wedge float32
	if speed < 9 {
		wedge = clampF((0.4-skill)/0.3, 0, 1) * 0.22
	}
	p := stancePose(stanceParams{
		crouch:  a.crouch,
		lean:    0.12 * (1 - clampF(skill, 0, 1)),
		wedge:   wedge,
		incline: incline,
		pitch:   pitch,
		roll:    roll,
		plant:   [2]float32{easeStep(a.plant[0]), easeStep(a.plant[1])},
	})
	return p
}

// walk advances the step and poses a walker, shouldering their skis
// when carry is set.
func (a *figureAnim) walk(t *world.Terrain, pos mgl32.Vec3, speed, dt float32, carry bool) figPose {
	a.stride += speed * dt / 0.65 * math.Pi
	a.stride = float32(math.Mod(float64(a.stride), 2*math.Pi))
	pitch, roll, _ := slopeAngles(t, pos, a.heading)
	return walkPose(a.stride, speed, pitch, roll, carry)
}

// guestPose poses a guest from their sim state.
func (f *figures) guestPose(w *world.World, g *world.Guest, a *figureAnim, dt float32) figPose {
	t := w.Terrain
	switch {
	case g.OnPatrollerID != 0:
		p := lyingPose(1, 0)
		p.hide = 1<<boneSkiL | 1<<boneSkiR | 1<<bonePoleL | 1<<bonePoleR
		return p
	case g.OnLiftID != 0:
		return chairPose()
	case g.Unload.LiftID != 0 && !g.Fallen:
		up := clampF(g.Unload.Gone/world.UnloadRise, 0, 1)
		return lerpPose(chairPose(), a.skiPose(t, g.Pos, 0, g.Traits.Skill, 0, dt), easeStep(up))
	case g.Fallen:
		return f.fallPose(t, g, a, dt)
	case g.SkiTransitionTimer != 0:
		// Bent down to the bindings, skis on the snow at their feet.
		pitch, roll, _ := slopeAngles(t, g.Pos, a.heading)
		return stancePose(stanceParams{crouch: 1, lean: 0.7, pitch: pitch, roll: roll})
	case g.GearTimer > 0:
		// Putting skis in a rack or the snow, or taking them out.
		pitch, roll, _ := slopeAngles(t, g.Pos, a.heading)
		p := stancePose(stanceParams{crouch: 0.5, lean: 0.5, pitch: pitch, roll: roll})
		p.hide = 1<<boneSkiL | 1<<boneSkiR | 1<<bonePoleL | 1<<bonePoleR
		return p
	case !g.SkisOn:
		// Anyone who came with skis or has rented some carries them,
		// until they leave them outside.
		return a.walk(t, g.Pos, g.Speed, dt, g.CarriesSkis())
	}
	return a.skiPose(t, g.Pos, g.Speed, g.Traits.Skill, g.TurnSide, dt)
}

// Fall timing on screen: the topple takes fallTopple seconds; getting up
// spends its first getUpKneel share coming up onto the skis.
const (
	fallTopple = float32(0.35)
	getUpKneel = float32(0.55)
)

// fallPose poses a guest through their fall (world.Tumble): toppling
// from how they were skiing, lying where they slid, getting up onto
// skis set across the slope, and walking back for skis knocked off.
func (f *figures) fallPose(t *world.Terrain, g *world.Guest, a *figureAnim, dt float32) figPose {
	tb := &g.Tumble
	_, _, n := slopeAngles(t, g.Pos, a.heading)
	lying := lyingPose(int(tb.Dir), float32(tb.Side))
	liesOnSlope(&lying, n, 1)
	from := a.upright
	if !a.hasPose {
		from = stancePose(stanceParams{crouch: 0.3})
	}
	var p figPose
	switch tb.Phase {
	case world.FallSliding:
		p = lerpPose(from, lying, easeStep(tb.T/fallTopple))
	case world.FallDown:
		p = lying
		if g.Injured && g.Stranded {
			sit := snowSitPose()
			liesOnSlope(&sit, n, 0.6)
			p = lerpPose(lying, sit, easeStep(tb.T))
		}
	case world.FallGettingUp:
		pitch, roll, _ := slopeAngles(t, g.Pos, a.heading)
		kneel := stancePose(stanceParams{crouch: 1, lean: 0.3, pitch: pitch, roll: roll})
		stand := stancePose(stanceParams{crouch: 0.35, pitch: pitch, roll: roll})
		u := tb.T / max(tb.Dur, 0.01)
		if u < getUpKneel {
			p = lerpPose(lying, kneel, easeStep(u/getUpKneel))
		} else {
			p = lerpPose(kneel, stand, easeStep((u-getUpKneel)/(1-getUpKneel)))
		}
	case world.FallCollecting:
		if tb.SkiLost() {
			p = a.walk(t, g.Pos, g.Speed, dt, false) // fetching them
		} else {
			pitch, roll, _ := slopeAngles(t, g.Pos, a.heading)
			p = stancePose(stanceParams{crouch: 0.6, lean: 0.4, pitch: pitch, roll: roll})
		}
	}
	for s := range 2 {
		if tb.SkisOff[s] {
			p.hide |= 1 << (boneSkiL + s)
		}
	}
	if !g.SkisOn {
		p.hide |= 1<<boneSkiL | 1<<boneSkiR
	}
	return p
}

// lyingSki is a ski knocked off in a yard sale, lying in the snow.
func lyingSki(t *world.Terrain, side int, at [2]float32, yaw float32) (mgl32.Mat4, [figureBones]mgl32.Mat4) {
	var bones [figureBones]mgl32.Mat4
	y := VisualElevationAt(t, at[0], at[1])
	place := mgl32.Translate3D(at[0], y, at[1]).Mul4(headingBasis(yaw))
	rest := figureRestJoint(boneSkiL + side)
	bones[boneSkiL+side] = mgl32.Translate3D(0, 0, -rest[2])
	return place, bones
}

// Skis left standing (world.SkiStash): sunk this far into the snow, and
// leaning back this far, rad, against a rack's rail or loose in the snow.
const (
	standingSkiSink    = 0.15
	standingSkiRackTip = 0.24
	standingSkiSnowTip = 0.1
)

// standingSkis is a pair of skis stood up at at, facing yaw, leaning back
// by tip.
func standingSkis(t *world.Terrain, at mgl32.Vec2, yaw, tip float32) (mgl32.Mat4, [figureBones]mgl32.Mat4) {
	var bones [figureBones]mgl32.Mat4
	y := VisualElevationAt(t, at[0], at[1])
	place := mgl32.Translate3D(at[0], y, at[1]).Mul4(headingBasis(yaw))
	for side := range 2 {
		rest := figureRestJoint(boneSkiL + side)
		// Centre the ski on the origin, stand it on its tail (its
		// length, model X, up), lean the top back (-X), and set the pair
		// side by side.
		bones[boneSkiL+side] = mgl32.Translate3D(0, figSkiLen/2-standingSkiSink, sideSign[side]*0.05).
			Mul4(mgl32.HomogRotate3DZ(math.Pi/2 - tip)).
			Mul4(mgl32.Translate3D(-0.03, -0.015, -rest[2]))
	}
	return place, bones
}

// build poses every guest and patroller on the mountain into this
// frame's instances.
func (f *figures) build(r *Renderer, w *world.World) {
	f.data = f.data[:0]
	f.count = 0
	f.frame++
	dt := float32(0)
	if f.simTime != 0 {
		dt = clampF(float32(w.SimTime-f.simTime), 0, 2)
	}
	f.simTime = w.SimTime
	t := w.Terrain
	hr2 := r.HiddenRadius * r.HiddenRadius
	rescuers := map[uint64]*world.Patroller{}
	for _, p := range w.Patrollers {
		rescuers[p.ID] = p
	}
	var bones [figureBones]mgl32.Mat4
	for _, g := range w.OnMountain {
		if g.Stash.Out {
			tip := float32(standingSkiSnowTip)
			if g.Stash.RackID != 0 {
				tip = standingSkiRackTip
			}
			place, skis := standingSkis(t, g.Stash.Pos, g.Stash.Yaw, tip)
			o := outfit(g.ID, g.Traits.Skill)
			f.add(place, &skis, &o, mgl32.Vec4{})
		}
		if r.HiddenGuestID != 0 && g.ID == r.HiddenGuestID || g.Indoors() {
			continue
		}
		// A patient lies where they fell while patrol loads them, rides
		// out of sight on a snowmobile, and is towed a little behind a
		// patroller's toboggan.
		pos := g.Pos
		heading := g.Heading
		if p := rescuers[g.OnPatrollerID]; p != nil {
			switch p.State {
			case world.PatrollerReturning:
				continue
			case world.PatrollerToboggan:
				pos[0] = p.Pos[0] - float32(math.Sin(float64(p.Heading)))*patientTow
				pos[2] = p.Pos[2] - float32(math.Cos(float64(p.Heading)))*patientTow
				heading = p.Heading
			}
		}
		if hr2 > 0 {
			dx, dz := pos[0]-r.HiddenGuestPos[0], pos[2]-r.HiddenGuestPos[2]
			if dx*dx+dz*dz < hr2 {
				continue
			}
		}
		a := f.animFor(f.anim, g.ID, pos, heading)
		a.steer(heading, dt)
		if g.OnLiftID != 0 || g.OnPatrollerID != 0 {
			a.heading = heading
		}
		pose := f.guestPose(w, g, a, dt)
		if !g.Fallen && g.OnLiftID == 0 {
			a.upright, a.hasPose = pose, true
		}
		pose.solve(&bones)
		y := pos[1]
		switch {
		case g.OnLiftID != 0:
		case g.OnPatrollerID != 0:
			y = VisualElevationAt(t, pos[0], pos[2]) + 0.1
		default:
			// An unloading rider stands up from their seat's height.
			y = VisualElevationAt(t, pos[0], pos[2]) + g.Unload.UnloadLift()
		}
		place := mgl32.Translate3D(pos[0], y, pos[2]).Mul4(headingBasis(a.heading))
		o := outfit(g.ID, g.Traits.Skill)
		tint := mgl32.Vec4{}
		if r.ActivityTint {
			c := guestColor(w, g)
			tint = mgl32.Vec4{c[0], c[1], c[2], 0.75}
		}
		if r.HighlightGuestID != 0 && g.ID == r.HighlightGuestID {
			tint = mgl32.Vec4{1.0, 0.95, 0.1, 0.35}
		}
		f.add(place, &bones, &o, tint)
		if g.Fallen {
			for s := range 2 {
				if g.Tumble.SkisOff[s] {
					place, ski := lyingSki(t, s, g.Tumble.Skis[s], g.Tumble.SkiYaw[s])
					f.add(place, &ski, &o, tint)
				}
			}
		}
	}
	for _, p := range w.Patrollers {
		var pose figPose
		switch {
		case p.State == world.PatrollerRiding:
			pose = chairPose()
		case p.State.OnSkis(), p.State.OnFoot(), p.State == world.PatrollerOnScene:
		default:
			continue // indoors or on a snowmobile
		}
		a := f.animFor(f.patrol, p.ID, p.Pos, p.Heading)
		var speed float32
		if dt > 0 {
			speed = mgl32.Vec2{p.Pos[0] - a.prevPos[0], p.Pos[2] - a.prevPos[2]}.Len() / dt
		}
		a.prevPos = p.Pos
		a.steer(p.Heading, dt)
		y := VisualElevationAt(t, p.Pos[0], p.Pos[2])
		switch {
		case p.State == world.PatrollerRiding:
			a.heading = p.Heading
			y = p.Pos[1]
		case p.State == world.PatrollerOnScene:
			pitch, roll, _ := slopeAngles(t, p.Pos, a.heading)
			pose = stancePose(stanceParams{crouch: 1, lean: 0.5, pitch: pitch, roll: roll})
			pose.hide = 1<<boneSkiL | 1<<boneSkiR | 1<<bonePoleL | 1<<bonePoleR
		case p.State.OnSkis():
			pose = a.skiPose(t, p.Pos, speed, 0.9, 0, dt)
		default:
			// Headed for a lift to ski to a call: skis on the shoulder.
			pose = a.walk(t, p.Pos, speed, dt, p.State == world.PatrollerToLift)
		}
		pose.solve(&bones)
		place := mgl32.Translate3D(p.Pos[0], y, p.Pos[2]).Mul4(headingBasis(a.heading))
		f.add(place, &bones, &patrolOutfit, mgl32.Vec4{})
	}
	// Lift staff stand at their posts while the resort and the lift are
	// open.
	if w.ResortOpen {
		for _, l := range w.Lifts {
			if !l.Open {
				continue
			}
			for _, post := range l.StaffPosts() {
				pos := mgl32.Vec3{post.Pos[0], 0, post.Pos[1]}
				pitch, roll, _ := slopeAngles(t, pos, post.Heading)
				pose := walkPose(0, 0, pitch, roll, false)
				pose.solve(&bones)
				place := mgl32.Translate3D(pos[0], VisualElevationAt(t, pos[0], pos[2]), pos[2]).Mul4(headingBasis(post.Heading))
				f.add(place, &bones, &liftStaffOutfit, mgl32.Vec4{})
			}
		}
	}
	if r.FigureGallery {
		f.gallery(r, w)
	}
	// Forget figures that have left.
	for _, m := range []map[uint64]*figureAnim{f.anim, f.patrol} {
		for id, a := range m {
			if a.frame != f.frame {
				delete(m, id)
			}
		}
	}
	f.upload()
}

func (f *figures) upload() {
	gl.BindBuffer(gl.TEXTURE_BUFFER, f.buf)
	if len(f.data) > 0 {
		gl.BufferData(gl.TEXTURE_BUFFER, len(f.data)*4, gl.Ptr(f.data), gl.STREAM_DRAW)
	} else {
		gl.BufferData(gl.TEXTURE_BUFFER, 16, nil, gl.STREAM_DRAW)
	}
	gl.BindBuffer(gl.TEXTURE_BUFFER, 0)
	gl.BindTexture(gl.TEXTURE_BUFFER, f.tex)
	gl.TexBuffer(gl.TEXTURE_BUFFER, gl.RGBA32F, f.buf)
	gl.BindTexture(gl.TEXTURE_BUFFER, 0)
}

// draw issues this frame's figures with the shader sh, already set up
// with whatever else it needs.
func (f *figures) draw(sh *Shader) {
	if f.count == 0 || f.mesh == nil {
		return
	}
	gl.ActiveTexture(gl.TEXTURE0 + figureTexUnit)
	gl.BindTexture(gl.TEXTURE_BUFFER, f.tex)
	sh.SetInt("uFigures", figureTexUnit)
	gl.BindVertexArray(f.mesh.VAO)
	gl.DrawElementsInstanced(gl.TRIANGLES, f.mesh.IndexCount, gl.UNSIGNED_INT, nil, f.count)
	gl.BindVertexArray(0)
	gl.ActiveTexture(gl.TEXTURE0)
}

// gallery lines up one figure in each pose at the camera target, for
// checking poses in a screenshot (-figure-gallery).
func (f *figures) gallery(r *Renderer, w *world.World) {
	t := w.Terrain
	c := r.Camera.Target
	type entry struct {
		pose figPose
		skis bool
	}
	plain := func(p stanceParams) figPose { return stancePose(p) }
	poses := []figPose{
		plain(stanceParams{crouch: 0.15}),
		plain(stanceParams{crouch: 0.6, incline: 0.45}),
		plain(stanceParams{crouch: 0.6, incline: -0.45, plant: [2]float32{1, 0}}),
		plain(stanceParams{crouch: 0.25, lean: 0.12, wedge: 0.22}),
		walkPose(0.8, 0.8, 0, 0, false),
		walkPose(2.4, 0.8, 0, 0, true),
		chairPose(),
		lyingPose(0, 1),
		lyingPose(1, -1),
		lyingPose(2, 1),
		lyingPose(3, 1),
		snowSitPose(),
		lerpPose(lyingPose(1, 1), plain(stanceParams{crouch: 1, lean: 0.3}), 0.5),
	}
	var bones [figureBones]mgl32.Mat4
	for i, p := range poses {
		x := c[0] + (float32(i)-float32(len(poses))/2)*2.6
		z := c[2]
		y := VisualElevationAt(t, x, z)
		if i == 6 {
			y += 0.5
		}
		p.solve(&bones)
		o := outfit(uint64(i+1), 0.5)
		f.add(mgl32.Translate3D(x, y, z).Mul4(headingBasis(math.Pi/2)), &bones, &o, mgl32.Vec4{})
	}
}
