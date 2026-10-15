package engine

import (
	"fmt"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// CreateWindow initialises GLFW and OpenGL and returns the window.
func CreateWindow(title string, width, height int) (*glfw.Window, error) {
	if err := glfw.Init(); err != nil {
		return nil, fmt.Errorf("glfw.Init: %w", err)
	}

	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.Samples, 0)     // anti-aliasing is the scene's own framebuffer (render/msaa.go)
	glfw.WindowHint(glfw.StencilBits, 8) // drawn trails shade each pixel once (render.drawTrailLayers)

	win, err := glfw.CreateWindow(width, height, title, nil, nil)
	if err != nil {
		glfw.Terminate()
		return nil, fmt.Errorf("glfw.CreateWindow: %w", err)
	}

	win.MakeContextCurrent()
	glfw.SwapInterval(SwapInterval()) // vsync

	if err := gl.Init(); err != nil {
		// gl.Init returns a string on error, wrapped for us
		win.Destroy()
		glfw.Terminate()
		return nil, fmt.Errorf("gl.Init: %v", err)
	}

	return win, nil
}

// SwapInterval is the vsync interval: every refresh on a display of
// 60 Hz or so, every other one (60 fps) on a faster one, such as a 120 Hz
// ProMotion screen. The game doesn't need more, and drawing every
// refresh there keeps the GPU flat out, so frames that run long stutter.
func SwapInterval() int {
	if m := glfw.GetPrimaryMonitor(); m != nil {
		if mode := m.GetVideoMode(); mode != nil && mode.RefreshRate >= 90 {
			return 2
		}
	}
	return 1
}
