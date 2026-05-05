package engine

import (
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"mountain-mogul/internal/render"
)

// App is the main application container.
type App struct {
	Window   *glfw.Window
	Renderer *render.Renderer
	Input    *Input

	scenes   []Scene
	AssetDir string
}

// NewApp creates the window, initialises OpenGL and the renderer.
func NewApp(title string, width, height int, assetDir string) *App {
	win, err := CreateWindow(title, width, height)
	if err != nil {
		panic(err)
	}

	r, err := render.NewRenderer(width, height, assetDir)
	if err != nil {
		panic(err)
	}

	app := &App{
		Window:   win,
		Renderer: r,
		Input:    NewInput(win),
		AssetDir: assetDir,
	}

	// Seed the actual framebuffer size — on Retina this is 2× the logical size
	// passed to NewRenderer, so we must query it explicitly.
	fbW, fbH := win.GetFramebufferSize()
	r.SetViewport(fbW, fbH)

	// Framebuffer-size callback: only updates gl.Viewport (physical pixels).
	win.SetFramebufferSizeCallback(func(_ *glfw.Window, w, h int) {
		r.SetViewport(w, h)
	})

	// Window-size callback: updates camera and UI ortho (logical/point pixels).
	// GLFW mouse coordinates are also in logical pixels, so these must match.
	win.SetSizeCallback(func(_ *glfw.Window, w, h int) {
		r.SetLogicalSize(w, h)
	})

	return app
}

// Destroy cleans up GLFW.
func (app *App) Destroy() {
	for i := len(app.scenes) - 1; i >= 0; i-- {
		app.scenes[i].Destroy()
	}
	glfw.Terminate()
}

// PushScene pushes a new scene onto the stack and calls Init.
func (app *App) PushScene(s Scene) {
	if err := s.Init(app); err != nil {
		panic(err)
	}
	app.scenes = append(app.scenes, s)
}

// PopScene destroys and removes the top scene.
func (app *App) PopScene() {
	if len(app.scenes) == 0 {
		return
	}
	top := app.scenes[len(app.scenes)-1]
	top.Destroy()
	app.scenes = app.scenes[:len(app.scenes)-1]
}

// ReplaceScene pops the top scene and pushes a new one.
func (app *App) ReplaceScene(s Scene) {
	app.PopScene()
	app.PushScene(s)
}

// Run is the main game loop.
func (app *App) Run() {
	prev := time.Now()
	for !app.Window.ShouldClose() {
		now := time.Now()
		dt := now.Sub(prev).Seconds()
		prev = now

		app.Input.BeginFrame()
		glfw.PollEvents()

		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		if len(app.scenes) > 0 {
			top := app.scenes[len(app.scenes)-1]
			top.Update(dt)
			top.Render(app.Renderer)
		}

		app.Window.SwapBuffers()
	}
}
