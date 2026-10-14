package render

import (
	"fmt"
	"math"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/edition"
	"mountain-mogul/internal/world"
)

// Menu thumbnails: pictures of the game's own models for the build menu,
// rendered once at startup into small textures the UI draws like icons.

// ThumbID names one thumbnail.
type ThumbID int

const (
	ThumbNone ThumbID = iota
	ThumbLiftStation
	ThumbChairDouble
	ThumbChairTriple
	ThumbChairQuad
	ThumbChair6Pack
	ThumbGondola
	ThumbHeli
	ThumbTicketOffice
	ThumbCar
	ThumbSkiRack
	ThumbSnowGun
	ThumbTree
	ThumbSnowcat
	ThumbBulldozer // the Remove tool
	ThumbBelt      // the magic carpet
	ThumbRope      // the rope tow
	ThumbTBar      // the T-bar
	ThumbTram      // the aerial tram
	ThumbChairSingle
	ThumbChair8Pack
	ThumbFunitel
	ThumbTri       // the 3S gondola
	ThumbFunicular // the funicular
	// ThumbShellBase + world.ShellKind is that kind's building.
	ThumbShellBase
)

// ShellThumb is the thumbnail of a building of kind k.
func ShellThumb(k world.ShellKind) ThumbID { return ThumbShellBase + ThumbID(k) }

// thumbSize is each thumbnail's texture size in pixels; mipmaps scale it
// down smoothly to button size.
const thumbSize = 256

// thumbPart is one model in a thumbnail.
type thumbPart struct {
	model string // OBJ name under assets/models, without .obj
	m     mgl32.Mat4
	tint  mgl32.Vec3
}

// thumbSpec is one thumbnail: its parts, the view's turn about the
// vertical in radians (0 looks from +X+Z towards the origin), and, when
// above 0, how much of the model's height from the bottom to frame (a
// chair's hanger is mostly bare pole).
type thumbSpec struct {
	parts  []thumbPart
	yaw    float32
	bottom float32
}

var thumbWhite = mgl32.Vec3{1, 1, 1}

func onePart(model string, tint mgl32.Vec3, yaw float32) thumbSpec {
	return thumbSpec{parts: []thumbPart{{model, mgl32.Ident4(), tint}}, yaw: yaw}
}

// chairThumb frames a chair's seat and the foot of its hanger.
func chairThumb(model string) thumbSpec {
	s := onePart(model, thumbWhite, 0.6)
	s.bottom = 0.45
	return s
}

// thumbSpecs lists every thumbnail this edition draws.
func thumbSpecs() map[ThumbID]thumbSpec {
	specs := map[ThumbID]thumbSpec{
		ThumbLiftStation:  onePart("lift_station", thumbWhite, 0.5),
		ThumbChairDouble:  chairThumb("chair"),
		ThumbChairTriple:  chairThumb("chair_triple"),
		ThumbChairQuad:    chairThumb("chair_quad"),
		ThumbTicketOffice: onePart("ticket_office", thumbWhite, 0),
		ThumbCar:          onePart("car_suv", mgl32.Vec3{0.80, 0.18, 0.15}, 0.4),
		ThumbSkiRack:      onePart("ski_rack", thumbWhite, 0.3),
		ThumbTree:         onePart("tree", mgl32.Vec3{0.75, 1.0, 0.75}, 0),
		ThumbSnowcat:      onePart("snowcat", thumbWhite, 0.5),
		ThumbBulldozer:    onePart("bulldozer", thumbWhite, 0.4),
	}
	if !edition.Demo {
		specs[ThumbSnowGun] = onePart("snow_gun", thumbWhite, 0.4)
	}
	addBigLiftThumbs(specs)
	for k, prefix := range shellKitPrefix {
		if prefix != "" {
			specs[ShellThumb(world.ShellKind(k))] = shellThumbSpec(world.ShellKind(k), prefix)
		}
	}
	return specs
}

// shellThumbSpec is a small building of kind k, three cells by two.
func shellThumbSpec(k world.ShellKind, prefix string) thumbSpec {
	const seed = 7
	cells := [][2]int{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {1, 1}, {2, 1}}
	wall, roof := world.ShellPalette(k, seed)
	var spec thumbSpec
	for _, t := range world.PreviewShell(mgl32.Vec2{}, 0, k, 1, cells, seed) {
		m := mgl32.Translate3D(t.Pos[0], t.Pos[1], t.Pos[2]).Mul4(mgl32.HomogRotate3DY(t.Rot))
		spec.parts = append(spec.parts, thumbPart{prefix + lodgeTileNames[t.Kind], m, shellTileTint(t, wall, roof)})
	}
	return spec
}

// thumbModel is a model loaded for drawing thumbnails.
type thumbModel struct {
	mesh *Mesh
	tex  uint32
}

// Thumb returns thumbnail id's texture, or 0 if there is none.
func (r *Renderer) Thumb(id ThumbID) uint32 { return r.thumbs[id] }

// renderThumbnails draws every thumbnail into its own texture. Called
// once after the meshes and shaders load; a failure leaves the menu on
// its icons.
func (r *Renderer) renderThumbnails() {
	r.thumbs = map[ThumbID]uint32{}
	shaderDir := r.assetDir + "/shaders/"
	sh, err := LoadShader(shaderDir+"thumb.vert", shaderDir+"thumb.frag")
	if err != nil {
		fmt.Printf("thumbnails: %v\n", err)
		return
	}
	defer gl.DeleteProgram(sh.id)

	cull := gl.IsEnabled(gl.CULL_FACE)
	var fbo, depth uint32
	gl.GenFramebuffers(1, &fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	gl.GenRenderbuffers(1, &depth)
	gl.BindRenderbuffer(gl.RENDERBUFFER, depth)
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT24, thumbSize, thumbSize)
	gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, depth)
	defer func() {
		gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
		gl.DeleteFramebuffers(1, &fbo)
		gl.DeleteRenderbuffers(1, &depth)
		gl.Viewport(0, 0, int32(r.frameW), int32(r.frameH))
		if cull {
			gl.Enable(gl.CULL_FACE)
		}
		gl.ClearColor(0.635, 0.682, 0.918, 1.0)
	}()

	models := map[string]thumbModel{}
	defer func() {
		for _, l := range models {
			l.mesh.Delete()
		}
	}()
	modelDir := r.assetDir + "/models/"

	sh.Use()
	sh.SetInt("uTexture", 0)
	sh.SetVec3("uLightDir", mgl32.Vec3{0.45, 0.8, 0.4}.Normalize())
	gl.ActiveTexture(gl.TEXTURE0)
	gl.Viewport(0, 0, thumbSize, thumbSize)
	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	gl.Disable(gl.BLEND)
	gl.VertexAttrib3f(VertexColorLoc, 1, 1, 1)

	for id, spec := range thumbSpecs() {
		meshes := make([]thumbModel, len(spec.parts))
		for i, p := range spec.parts {
			l, ok := models[p.model]
			if !ok {
				mesh, tex, err := loadOBJFile(modelDir+p.model+".obj", nil)
				if err != nil {
					fmt.Printf("thumbnails: %v\n", err)
					mesh, tex = fallbackMesh()
				}
				l = thumbModel{mesh, tex}
				models[p.model] = l
			}
			meshes[i] = l
		}

		var tex uint32
		gl.GenTextures(1, &tex)
		gl.BindTexture(gl.TEXTURE_2D, tex)
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, thumbSize, thumbSize, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
		if gl.CheckFramebufferStatus(gl.FRAMEBUFFER) != gl.FRAMEBUFFER_COMPLETE {
			fmt.Println("thumbnails: framebuffer incomplete")
			gl.DeleteTextures(1, &tex)
			return
		}
		gl.ClearColor(0, 0, 0, 0)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		sh.SetMat4("uViewProj", thumbViewProj(spec, meshes))
		for i, p := range spec.parts {
			sh.SetMat4("uModel", p.m)
			sh.SetVec3("uTint", p.tint)
			gl.BindTexture(gl.TEXTURE_2D, meshes[i].tex)
			meshes[i].mesh.Draw()
		}

		gl.BindTexture(gl.TEXTURE_2D, tex)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
		gl.GenerateMipmap(gl.TEXTURE_2D)
		r.thumbs[id] = tex
	}
}

// thumbViewProj frames a thumbnail's parts from above at a three-quarter
// angle, fitting their bounds square with a small margin. The image is
// flipped vertically so it reads top-down like the UI's textures.
func thumbViewProj(spec thumbSpec, meshes []thumbModel) mgl32.Mat4 {
	const pitch = 0.5 // radians above the horizon
	yaw := float64(spec.yaw) + math.Pi/4
	eye := mgl32.Vec3{
		float32(math.Sin(yaw) * math.Cos(pitch)),
		float32(math.Sin(pitch)),
		float32(math.Cos(yaw) * math.Cos(pitch)),
	}
	view := mgl32.LookAtV(eye.Mul(1000), mgl32.Vec3{}, mgl32.Vec3{0, 1, 0})

	lo := mgl32.Vec3{math.MaxFloat32, math.MaxFloat32, math.MaxFloat32}
	hi := lo.Mul(-1)
	for i, p := range spec.parts {
		b := meshes[i].mesh.Bounds
		if spec.bottom > 0 {
			b[1][1] = b[0][1] + (b[1][1]-b[0][1])*spec.bottom
		}
		mv := view.Mul4(p.m)
		for c := range 8 {
			corner := mgl32.Vec3{b[c&1][0], b[(c>>1)&1][1], b[(c>>2)&1][2]}
			v := mv.Mul4x1(corner.Vec4(1)).Vec3()
			for k := range 3 {
				lo[k] = min(lo[k], v[k])
				hi[k] = max(hi[k], v[k])
			}
		}
	}
	cx, cy := (lo[0]+hi[0])/2, (lo[1]+hi[1])/2
	half := max(hi[0]-lo[0], hi[1]-lo[1]) / 2 * 1.06
	proj := mgl32.Ortho(cx-half, cx+half, cy+half, cy-half, -hi[2]-1, -lo[2]+1)
	return proj.Mul4(view)
}
