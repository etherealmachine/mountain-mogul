package render

import (
	"github.com/go-gl/gl/v4.1-core/gl"
)

// Anti-aliasing: the 3D scene draws into a multisampled framebuffer of
// the chosen sample count, resolved onto the window's framebuffer before
// the UI draws over it, so the count can change without reopening the
// window. With anti-aliasing off the scene draws straight to the window.

// AASampleOptions is the sample counts the settings offer (0 is off).
var AASampleOptions = []int{0, 2, 4}

// sceneTarget is the multisampled framebuffer the scene draws into.
type sceneTarget struct {
	fbo, color, depth uint32
	w, h, samples     int
	bound             bool // the scene is drawing into it this frame
}

// SetAntiAliasing sets the scene's sample count: 0 for off, else 2 or 4
// (clamped to what the GPU allows). Takes effect next frame.
func (r *Renderer) SetAntiAliasing(samples int) {
	var maxSamples int32
	gl.GetIntegerv(gl.MAX_SAMPLES, &maxSamples)
	r.aaSamples = max(0, min(samples, int(maxSamples)))
}

// AntiAliasing is the scene's sample count, 0 when off.
func (r *Renderer) AntiAliasing() int { return r.aaSamples }

// beginScene points the scene's drawing at the multisampled framebuffer
// (made or remade to the window's size and the sample count), when
// anti-aliasing is on. Called at the start of DrawWorld.
func (r *Renderer) beginScene() {
	t := &r.sceneTarget
	if r.aaSamples < 2 {
		return
	}
	if t.fbo == 0 || t.w != r.frameW || t.h != r.frameH || t.samples != r.aaSamples {
		t.release()
		t.w, t.h, t.samples = r.frameW, r.frameH, r.aaSamples
		gl.GenFramebuffers(1, &t.fbo)
		gl.BindFramebuffer(gl.FRAMEBUFFER, t.fbo)
		gl.GenRenderbuffers(1, &t.color)
		gl.BindRenderbuffer(gl.RENDERBUFFER, t.color)
		gl.RenderbufferStorageMultisample(gl.RENDERBUFFER, int32(t.samples), gl.RGBA8, int32(t.w), int32(t.h))
		gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.RENDERBUFFER, t.color)
		gl.GenRenderbuffers(1, &t.depth)
		gl.BindRenderbuffer(gl.RENDERBUFFER, t.depth)
		gl.RenderbufferStorageMultisample(gl.RENDERBUFFER, int32(t.samples), gl.DEPTH24_STENCIL8, int32(t.w), int32(t.h))
		gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_STENCIL_ATTACHMENT, gl.RENDERBUFFER, t.depth)
		if gl.CheckFramebufferStatus(gl.FRAMEBUFFER) != gl.FRAMEBUFFER_COMPLETE {
			t.release()
			r.aaSamples = 0
			gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
			return
		}
	}
	gl.BindFramebuffer(gl.FRAMEBUFFER, t.fbo)
	gl.Viewport(0, 0, int32(t.w), int32(t.h))
	t.bound = true
}

// resolveScene copies the multisampled scene onto the window's
// framebuffer and draws there from then on. Called before the UI, and at
// the end of the frame in case nothing drew UI.
func (r *Renderer) resolveScene() {
	t := &r.sceneTarget
	if !t.bound {
		return
	}
	t.bound = false
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, t.fbo)
	gl.BindFramebuffer(gl.DRAW_FRAMEBUFFER, 0)
	gl.BlitFramebuffer(0, 0, int32(t.w), int32(t.h), 0, 0, int32(t.w), int32(t.h), gl.COLOR_BUFFER_BIT, gl.NEAREST)
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	gl.Viewport(0, 0, int32(r.frameW), int32(r.frameH))
}

// EndFrame finishes the frame's drawing before the swap.
func (r *Renderer) EndFrame() { r.resolveScene() }

func (t *sceneTarget) release() {
	if t.fbo != 0 {
		gl.DeleteFramebuffers(1, &t.fbo)
		gl.DeleteRenderbuffers(1, &t.color)
		gl.DeleteRenderbuffers(1, &t.depth)
	}
	*t = sceneTarget{}
}
