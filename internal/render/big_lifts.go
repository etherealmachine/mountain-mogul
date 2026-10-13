//go:build !demo

package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// The 6-pack, the gondola, and the heli: their models and drawing. Not
// in the demo (big_lifts_demo.go).

// bigLifts is the Renderer's batches for them.
type bigLifts struct {
	chair6PackBatch       *Batch
	gondolaBatch          *Batch
	helicopterBodyBatch   *Batch      // rigid fuselage, tail, skids
	helicopterPartBatches []partBatch // spinning rotor parts (loaded from # part metadata)
}

// loadBigLifts loads the 6-pack chair, gondola cabin, helipad and
// helicopter models.
func loadBigLifts(r *Renderer, modelDir string) {
	chair6PackPath := modelDir + "chair_6pack.obj"
	chair6PackMesh, chair6PackTexID := LoadOBJ(chair6PackPath)
	r.chair6PackBatch = NewDynamicBatch(chair6PackMesh, chair6PackTexID)
	world.RegisterMeshSlots(world.MeshChair6Pack, LoadOBJSlots(chair6PackPath))

	gondolaCabinPath := modelDir + "gondola_cabin.obj"
	gondolaCabinMesh, gondolaCabinTexID := LoadOBJ(gondolaCabinPath)
	r.gondolaBatch = NewDynamicBatch(gondolaCabinMesh, gondolaCabinTexID)
	world.RegisterMeshSlots(world.MeshGondolaCabin, LoadOBJSlots(gondolaCabinPath))

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

// drawBigLiftChairs draws the 6-pack chairs and gondola cabins.
func drawBigLiftChairs(r *Renderer, sixPacks, gondolas []DynamicInstance) {
	if r.chair6PackBatch != nil {
		r.chair6PackBatch.SetDynamic(sixPacks)
		r.chair6PackBatch.Draw()
	}
	if r.gondolaBatch != nil {
		r.gondolaBatch.SetDynamic(gondolas)
		r.gondolaBatch.Draw()
	}
}

// bigLiftShadowBatches is the batches that cast shadows.
func bigLiftShadowBatches(r *Renderer) []*Batch {
	return []*Batch{r.chair6PackBatch, r.gondolaBatch, r.helicopterBodyBatch}
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

// addBigLiftThumbs adds the 6-pack's, gondola's and heli's menu pictures.
func addBigLiftThumbs(specs map[ThumbID]thumbSpec) {
	specs[ThumbChair6Pack] = chairThumb("chair_6pack")
	specs[ThumbGondola] = chairThumb("gondola_cabin")
	specs[ThumbGondola] = thumbSpec{parts: specs[ThumbGondola].parts, yaw: 0.6, bottom: 0.7}
	specs[ThumbHeli] = onePart("helicopter_body", thumbWhite, 0.6)
	specs[ThumbBelt] = onePart("magic_carpet_end", thumbWhite, 0.4)
}
