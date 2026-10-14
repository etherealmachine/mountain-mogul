//go:build !demo

package render

import (
	"math"
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// The full game's bigger and odder lifts: the single, the 6- and 8-packs
// (with their bubble covers), the gondola, funitel, 3S and tram cabins,
// the T-bar's hangers, and the heli: their models and drawing. Not in
// the demo (big_lifts_demo.go).

// bigLifts is the Renderer's batches for them.
type bigLifts struct {
	// bigChairs is the chair and cabin batches by model name, and
	// bigInsts this frame's instances of each (addBigLiftChair).
	bigChairs             map[string]*Batch
	bigInsts              map[string][]DynamicInstance
	helicopterBodyBatch   *Batch      // rigid fuselage, tail, skids
	helicopterPartBatches []partBatch // spinning rotor parts (loaded from # part metadata)
}

// bigChairModels is each chair and cabin model, and the world mesh its
// riders' slots register against (0 for a variant sharing another's).
var bigChairModels = []struct {
	name  string
	slots uint32
}{
	{"chair_single", world.MeshChairSingle},
	{"chair_6pack", world.MeshChair6Pack},
	{"chair_6pack_bubble", 0},
	{"chair_6pack_bubble_up", 0},
	{"chair_8pack", world.MeshChair8Pack},
	{"chair_8pack_bubble", 0},
	{"chair_8pack_bubble_up", 0},
	{"gondola_cabin", world.MeshGondolaCabin},
	{"funitel_cabin", world.MeshFunitelCabin},
	{"tri_cabin", world.MeshTriCabin},
	{"tram_cabin", world.MeshTramCabin},
	{"funicular_car", world.MeshFunicularCar},
	{"tbar_hanger", world.MeshTBarHanger},
	{"tbar_hanger_up", 0},
	// The bubble hoods, drawn see-through by drawBubbleHoods.
	{"chair_6pack_hood", 0},
	{"chair_6pack_hood_up", 0},
	{"chair_8pack_hood", 0},
	{"chair_8pack_hood_up", 0},
}

// hoodFade is how see-through a bubble hood is (dynamic.frag's uFade).
const hoodFade = 0.55

// isHood reports whether model name is a bubble hood.
func isHood(name string) bool { return strings.Contains(name, "_hood") }

// loadBigLifts loads the chair and cabin models, the helipad and the
// helicopter.
func loadBigLifts(r *Renderer, modelDir string) {
	r.bigChairs = map[string]*Batch{}
	r.bigInsts = map[string][]DynamicInstance{}
	for _, c := range bigChairModels {
		path := modelDir + c.name + ".obj"
		mesh, tex := LoadOBJ(path)
		r.bigChairs[c.name] = NewDynamicBatch(mesh, tex)
		if c.slots != 0 {
			world.RegisterMeshSlots(c.slots, LoadOBJSlots(path))
		}
	}

	// Helipad — static-batch mesh placed at Base and Top of each HeliLift.
	helipadMesh, helipadTexID := LoadOBJ(modelDir + "helipad.obj")
	r.staticBatches[MeshHelipad] = NewStaticBatch(helipadMesh, helipadTexID)
	if fp, ok := LoadOBJFootprint(modelDir + "helipad.obj"); ok {
		world.RegisterMeshFootprint(MeshHelipad, fp)
	}

	// Helicopter body (rigid fuselage, tail, skids, mast) and animated
	// rotor parts declared via # part metadata in the body OBJ.
	heliBodyPath := modelDir + "helicopter_body.obj"
	heliBodyMesh, heliBodyTexID := LoadOBJ(heliBodyPath)
	r.helicopterBodyBatch = NewDynamicBatch(heliBodyMesh, heliBodyTexID)
	for _, pd := range LoadOBJParts(heliBodyPath) {
		partMesh, partTexID := LoadOBJ(modelDir + pd.Name + ".obj")
		rate := float32(25.0) // rad/s default (main rotor ≈ 240 RPM)
		if pd.Axis == "spin_z" {
			rate = 90.0 // tail rotor ≈ 860 RPM
		}
		r.helicopterPartBatches = append(r.helicopterPartBatches, partBatch{
			batch:    NewDynamicBatch(partMesh, partTexID),
			decl:     pd,
			spinRate: rate,
		})
	}
}

// addHelipads adds a helipad at Base and Top of every heli lift to the
// static batch, oriented so the pad's +X axis points toward the other end.
func addHelipads(r *Renderer, w *world.World) {
	if helipadBatch, ok := r.staticBatches[MeshHelipad]; ok {
		for _, lift := range w.Lifts {
			if !lift.Cableless() {
				continue
			}
			helipadBatch.AddStatic(HelipadTransform(lift.Base, lift.Top, w.Terrain), mgl32.Vec3{1, 1, 1})
			helipadBatch.AddStatic(HelipadTransform(lift.Top, lift.Base, w.Terrain), mgl32.Vec3{1, 1, 1})
		}
	}
}

// drawHelicopters draws each heli lift's body and animated rotor parts.
func drawHelicopters(r *Renderer, w *world.World) {
	if r.helicopterBodyBatch != nil {
		var bodyInst []DynamicInstance
		partInsts := make([][]DynamicInstance, len(r.helicopterPartBatches))
		for _, lift := range w.Lifts {
			if !lift.Cableless() {
				continue
			}
			pos := world.HeliWorldPos(lift, w.Terrain)
			heading := world.HeliHeading(lift)
			s := float32(math.Sin(float64(heading)))
			c := float32(math.Cos(float64(heading)))
			bodyInst = append(bodyInst, DynamicInstance{
				Position: [3]float32{pos[0], pos[1], pos[2]},
				Heading:  heading,
				Color:    [3]float32{1, 1, 1},
			})
			// Each part: world pivot = body_pos + rotY(heading) * partOffset.
			// rotY in dynamic.vert: result.x = s*x - c*z, result.z = c*x + s*z
			for pi, pb := range r.helicopterPartBatches {
				off := pb.decl.Offset
				partInsts[pi] = append(partInsts[pi], DynamicInstance{
					Position: [3]float32{
						pos[0] + s*off[0] - c*off[2],
						pos[1] + off[1],
						pos[2] + c*off[0] + s*off[2],
					},
					Heading:  heading,
					Color:    [3]float32{1, 1, 1},
					SpinMode: spinModeFor(pb.decl.Axis),
				})
			}
		}
		r.helicopterBodyBatch.SetDynamic(bodyInst)
		r.helicopterBodyBatch.Draw()
		for pi, pb := range r.helicopterPartBatches {
			if len(partInsts[pi]) == 0 {
				continue
			}
			r.DynamicShader.SetFloat("uSpinRate", pb.spinRate)
			pb.batch.SetDynamic(partInsts[pi])
			pb.batch.Draw()
		}
		r.DynamicShader.SetFloat("uSpinRate", 0) // reset after rotor draws
	}
}

// addBigLiftChair queues chair or cabin inst of lift, at progress, for
// drawBigLiftChairs: a bubble-covered chair's hood up in the terminals and
// down on the line, a T-bar's hanger let down only to riders.
func addBigLiftChair(r *Renderer, lift *world.Lift, progress float32, inst DynamicInstance, hasPax bool) {
	var name string
	switch lift.Type {
	case world.LiftSingle:
		name = "chair_single"
	case world.LiftHS6Pack, world.LiftHS8Pack:
		name = "chair_6pack"
		if lift.Type == world.LiftHS8Pack {
			name = "chair_8pack"
		}
		if lift.Bubble {
			up := ""
			if lift.InTerminal(progress) {
				up = "_up"
			}
			hood := name[:len("chair_6pack")] + "_hood" + up
			r.bigInsts[hood] = append(r.bigInsts[hood], inst)
			name += "_bubble" + up
		}
	case world.LiftGondola:
		name = "gondola_cabin"
	case world.LiftFunitel:
		name = "funitel_cabin"
	case world.LiftTri:
		name = "tri_cabin"
	case world.LiftTram:
		name = "tram_cabin"
	case world.LiftFunicular:
		name = "funicular_car"
	case world.LiftTBar:
		inst.Color = [3]float32{1, 1, 1}
		name = "tbar_hanger_up"
		if hasPax {
			name = "tbar_hanger"
		}
	default:
		return
	}
	r.bigInsts[name] = append(r.bigInsts[name], inst)
}

// drawBigLiftChairs draws the chairs and cabins addBigLiftChair queued,
// and clears the queue for the next frame; the bubble hoods wait for
// drawBubbleHoods.
func drawBigLiftChairs(r *Renderer) {
	for name, b := range r.bigChairs {
		if isHood(name) {
			continue
		}
		// Set even when empty: the shadow pass draws whatever was set last.
		insts := r.bigInsts[name]
		b.SetDynamic(insts)
		if len(insts) > 0 {
			b.Draw()
		}
		r.bigInsts[name] = insts[:0]
	}
}

// drawBubbleHoods draws the bubble hoods addBigLiftChair queued,
// see-through over the riders, after everything solid: blended, not
// writing depth, so what's behind them still shows.
func drawBubbleHoods(r *Renderer) {
	any := false
	for name := range r.bigChairs {
		if isHood(name) && len(r.bigInsts[name]) > 0 {
			any = true
		}
	}
	if !any {
		return
	}
	r.DynamicShader.Use()
	r.DynamicShader.SetFloat("uFade", hoodFade)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.DepthMask(false)
	for name, b := range r.bigChairs {
		if !isHood(name) {
			continue
		}
		insts := r.bigInsts[name]
		b.SetDynamic(insts)
		if len(insts) > 0 {
			b.Draw()
		}
		r.bigInsts[name] = insts[:0]
	}
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)
	r.DynamicShader.SetFloat("uFade", 0)
}

// bigLiftShadowBatches is the batches that cast shadows: not the
// see-through hoods.
func bigLiftShadowBatches(r *Renderer) []*Batch {
	out := []*Batch{r.helicopterBodyBatch}
	for _, c := range bigChairModels {
		if !isHood(c.name) {
			out = append(out, r.bigChairs[c.name])
		}
	}
	return out
}

// HelipadTransform builds the world-space transform for a helipad placed at
// `pos`, oriented so the model's +X axis (the approach axis with the H
// pointing toward it) faces `otherEnd`. Pass `otherEnd == pos` to get no
// rotation (e.g. when the second point isn't known yet).
func HelipadTransform(pos, otherEnd mgl32.Vec2, terrain *world.Terrain) mgl32.Mat4 {
	y := VisualElevationAt(terrain, pos[0], pos[1])
	var rot float32
	if otherEnd != pos {
		dx := otherEnd[0] - pos[0]
		dz := otherEnd[1] - pos[1]
		rot = float32(math.Atan2(-float64(dz), float64(dx)))
	}
	return mgl32.Translate3D(pos[0], y, pos[1]).Mul4(mgl32.HomogRotate3DY(rot))
}

// addBigLiftThumbs adds the 6-pack's, gondola's, tram's, heli's and
// surface lifts' menu pictures.
func addBigLiftThumbs(specs map[ThumbID]thumbSpec) {
	specs[ThumbChair6Pack] = chairThumb("chair_6pack")
	specs[ThumbGondola] = chairThumb("gondola_cabin")
	specs[ThumbGondola] = thumbSpec{parts: specs[ThumbGondola].parts, yaw: 0.6, bottom: 0.62}
	specs[ThumbHeli] = onePart("helicopter_body", thumbWhite, 0.6)
	specs[ThumbBelt] = onePart("magic_carpet_end", thumbWhite, 0.4)
	specs[ThumbRope] = onePart("rope_tow_base", thumbWhite, 0.5)
	specs[ThumbTBar] = onePart("tbar_base", thumbWhite, 0.5)
	specs[ThumbTram] = thumbSpec{parts: onePart("tram_cabin", thumbWhite, 0.6).parts, yaw: 0.6, bottom: 0.62}
	specs[ThumbChairSingle] = chairThumb("chair_single")
	specs[ThumbChair8Pack] = chairThumb("chair_8pack")
	specs[ThumbFunitel] = thumbSpec{parts: onePart("funitel_cabin", thumbWhite, 0.6).parts, yaw: 0.6, bottom: 0.7}
	specs[ThumbTri] = thumbSpec{parts: onePart("tri_cabin", thumbWhite, 0.6).parts, yaw: 0.6, bottom: 0.6}
	// The car on its design grade, its compartments level.
	specs[ThumbFunicular] = thumbSpec{parts: []thumbPart{{"funicular_car", mgl32.HomogRotate3DZ(0.35), thumbWhite}}, yaw: 0.9}
}
