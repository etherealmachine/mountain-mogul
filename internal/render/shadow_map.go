package render

import (
	"math"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

// Object shadows: a single directional shadow map from the key light,
// covering the ground the camera sees. Trees, buildings, lifts, chairs,
// cats and guests cast into it; everything (terrain included) receives.
// Terrain doesn't cast here — the horizon map handles ridges.
const (
	shadowMapSize    = 4096
	shadowMapTexUnit = 4
	shadowMaxRadius  = 1600.0 // metres; zoomed-out views share the texels
	shadowDepthRange = 8000.0 // metres along the light direction
)

type shadowMap struct {
	fbo, tex       uint32
	staticShader   *Shader
	dynamicShader  *Shader
	figureShader   *Shader
	lightVP        mgl32.Mat4
	texelWorld     float32 // metres per shadow-map texel
	ready          bool
	failedToCreate bool
}

func (s *shadowMap) init(shaderDir string) {
	if s.fbo != 0 || s.failedToCreate {
		return
	}
	var err error
	if s.staticShader, err = LoadShader(shaderDir+"shadow_static.vert", shaderDir+"shadow.frag"); err != nil {
		s.failedToCreate = true
		return
	}
	if s.dynamicShader, err = LoadShader(shaderDir+"shadow_dynamic.vert", shaderDir+"shadow.frag"); err != nil {
		s.failedToCreate = true
		return
	}
	if s.figureShader, err = LoadShader(shaderDir+"figure_shadow.vert", shaderDir+"shadow.frag"); err != nil {
		s.failedToCreate = true
		return
	}
	gl.GenTextures(1, &s.tex)
	gl.BindTexture(gl.TEXTURE_2D, s.tex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.DEPTH_COMPONENT24, shadowMapSize, shadowMapSize, 0, gl.DEPTH_COMPONENT, gl.FLOAT, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_BORDER)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_BORDER)
	border := [4]float32{1, 1, 1, 1}
	gl.TexParameterfv(gl.TEXTURE_2D, gl.TEXTURE_BORDER_COLOR, &border[0])
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_MODE, gl.COMPARE_REF_TO_TEXTURE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_FUNC, gl.LEQUAL)
	gl.BindTexture(gl.TEXTURE_2D, 0)

	gl.GenFramebuffers(1, &s.fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, s.fbo)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, s.tex, 0)
	gl.DrawBuffer(gl.NONE)
	gl.ReadBuffer(gl.NONE)
	if gl.CheckFramebufferStatus(gl.FRAMEBUFFER) != gl.FRAMEBUFFER_COMPLETE {
		s.failedToCreate = true
	}
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
}

// shadowFocus is the ground centre and radius (metres) the shadow map
// should cover for the camera's current view.
func shadowFocus(c *Camera, groundY float32) (mgl32.Vec3, float32) {
	if c.Perspective {
		return mgl32.Vec3{c.LookAt[0], groundY, c.LookAt[2]}, 400
	}
	aspect := c.viewport[0] / max(c.viewport[1], 1)
	halfW := c.OrthoScale * aspect
	halfD := c.OrthoScale / float32(math.Sin(float64(mgl32.DegToRad(c.Pitch))))
	r := float32(math.Sqrt(float64(halfW*halfW+halfD*halfD))) * 1.1
	return mgl32.Vec3{c.Target[0], groundY, c.Target[2]}, min(r, shadowMaxRadius)
}

// lightMatrix builds an orthographic view-projection looking along
// -lightDir at centre, radius r, snapped to whole texels so shadows don't
// crawl as the camera pans.
func lightMatrix(lightDir, centre mgl32.Vec3, r float32) (mgl32.Mat4, float32) {
	up := mgl32.Vec3{0, 1, 0}
	if math.Abs(float64(lightDir[1])) > 0.99 {
		up = mgl32.Vec3{0, 0, 1}
	}
	eye := centre.Add(lightDir.Mul(shadowDepthRange / 2))
	view := mgl32.LookAtV(eye, centre, up)
	proj := mgl32.Ortho(-r, r, -r, r, 1, shadowDepthRange)
	m := proj.Mul4(view)
	half := float32(shadowMapSize) / 2
	o := m.Mul4x1(mgl32.Vec4{0, 0, 0, 1})
	dx := (float32(math.Round(float64(o[0]*half))) - o[0]*half) / half
	dy := (float32(math.Round(float64(o[1]*half))) - o[1]*half) / half
	return mgl32.Translate3D(dx, dy, 0).Mul4(m), 2 * r / shadowMapSize
}

// render draws the casters into the shadow map. Dynamic batches carry the
// instances uploaded during the previous frame's main pass.
func (s *shadowMap) render(r *Renderer, lightDir mgl32.Vec3, groundY float32) {
	s.ready = false
	if s.failedToCreate || s.fbo == 0 || lightDir[1] <= 0.01 {
		return
	}
	centre, radius := shadowFocus(r.Camera, groundY)
	s.lightVP, s.texelWorld = lightMatrix(lightDir.Normalize(), centre, radius)

	var prevFBO int32
	var vp [4]int32
	gl.GetIntegerv(gl.FRAMEBUFFER_BINDING, &prevFBO)
	gl.GetIntegerv(gl.VIEWPORT, &vp[0])

	gl.BindFramebuffer(gl.FRAMEBUFFER, s.fbo)
	gl.Viewport(0, 0, shadowMapSize, shadowMapSize)
	gl.Clear(gl.DEPTH_BUFFER_BIT)
	gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.POLYGON_OFFSET_FILL)
	gl.PolygonOffset(2, 4)

	s.staticShader.Use()
	s.staticShader.SetMat4("uLightVP", s.lightVP)
	for _, b := range r.staticBatches {
		b.Draw()
	}
	s.dynamicShader.Use()
	s.dynamicShader.SetMat4("uLightVP", s.lightVP)
	batches := []*Batch{
		r.snowcatBatch, r.patrollerBatch,
		r.chairBatch, r.chairTripleBatch, r.chairQuadBatch, r.chair6PackBatch, r.gondolaBatch, r.helicopterBodyBatch,
	}
	batches = append(append(batches, r.carBatches[:]...), r.carRoofBatches[:]...)
	for _, b := range batches {
		if b != nil {
			b.Draw()
		}
	}
	s.figureShader.Use()
	s.figureShader.SetMat4("uLightVP", s.lightVP)
	r.figures.draw(s.figureShader)

	gl.Disable(gl.POLYGON_OFFSET_FILL)
	gl.BindFramebuffer(gl.FRAMEBUFFER, uint32(prevFBO))
	gl.Viewport(vp[0], vp[1], vp[2], vp[3])
	s.ready = true
}

func (s *shadowMap) bind() {
	gl.ActiveTexture(gl.TEXTURE0 + shadowMapTexUnit)
	gl.BindTexture(gl.TEXTURE_2D, s.tex)
	gl.ActiveTexture(gl.TEXTURE0)
}

func (s *shadowMap) apply(sh *Shader, on bool) {
	enabled := 0
	if on && s.ready {
		enabled = 1
	}
	sh.SetInt("uShadowOn", enabled)
	sh.SetInt("uShadowMap", shadowMapTexUnit)
	sh.SetMat4("uShadowVP", s.lightVP)
	sh.SetFloat("uShadowTexelWorld", s.texelWorld)
}
