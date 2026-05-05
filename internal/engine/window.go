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

	win, err := glfw.CreateWindow(width, height, title, nil, nil)
	if err != nil {
		glfw.Terminate()
		return nil, fmt.Errorf("glfw.CreateWindow: %w", err)
	}

	win.MakeContextCurrent()
	glfw.SwapInterval(1) // vsync

	if err := gl.Init(); err != nil {
		// gl.Init returns a string on error, wrapped for us
		win.Destroy()
		glfw.Terminate()
		return nil, fmt.Errorf("gl.Init: %v", err)
	}

	return win, nil
}
