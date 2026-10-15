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
	glfw.SwapInterval(1) // vsync (the frame rate's held by App.Run's pacing)

	if err := gl.Init(); err != nil {
		// gl.Init returns a string on error, wrapped for us
		win.Destroy()
		glfw.Terminate()
		return nil, fmt.Errorf("gl.Init: %v", err)
	}

	return win, nil
}

// FramePeriod is the seconds between frames the game draws: the
// display's refresh, but no faster than 60 a second (on a 120 Hz
// ProMotion screen, drawing every refresh kept the GPU flat out, so long
// frames stuttered; macOS's OpenGL ignores a swap interval over 1, so
// App.Run holds it). 0 if unknown.
func FramePeriod() float64 {
	m := glfw.GetPrimaryMonitor()
	if m == nil {
		return 0
	}
	mode := m.GetVideoMode()
	if mode == nil || mode.RefreshRate <= 0 {
		return 0
	}
	return max(1/float64(mode.RefreshRate), 1.0/60)
}
