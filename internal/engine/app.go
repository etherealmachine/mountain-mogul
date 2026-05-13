package engine

import (
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
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

	// LogSlowFrames toggles the per-frame stderr line emitted when a
	// frame exceeds slowFrameThreshold. Off by default — the stall
	// watchdog stays on and only fires on real beachball-scale freezes.
	// main.go's -trace flag flips this on.
	LogSlowFrames bool

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

// Frame-tracing thresholds. slowFrameThreshold logs a stderr line each
// time a frame exceeds it with an update/render split — useful for
// finding which side is the bottleneck. stallDumpThreshold triggers a
// full goroutine stack dump when the main loop heartbeat goes stale
// for that long (= macOS beachball territory).
const (
	slowFrameThreshold = 100 * time.Millisecond
	stallDumpThreshold = 3 * time.Second
)

// Run is the main game loop. Always-on instrumentation: per-frame
// stderr logging when a frame exceeds slowFrameThreshold and a
// background watchdog that dumps all goroutine stacks if the main
// thread stops heartbeating for stallDumpThreshold.
func (app *App) Run() {
	var heartbeat atomic.Int64
	heartbeat.Store(time.Now().UnixNano())
	stopWatchdog := make(chan struct{})
	go stallWatchdog(&heartbeat, stopWatchdog)
	defer close(stopWatchdog)

	prev := time.Now()
	for !app.Window.ShouldClose() {
		now := time.Now()
		dt := now.Sub(prev).Seconds()
		prev = now
		heartbeat.Store(now.UnixNano())

		frameStart := time.Now()
		app.Input.BeginFrame()
		glfw.PollEvents()

		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		var updateDur, renderDur time.Duration
		if len(app.scenes) > 0 {
			top := app.scenes[len(app.scenes)-1]
			uStart := time.Now()
			top.Update(dt)
			updateDur = time.Since(uStart)
			rStart := time.Now()
			top.Render(app.Renderer)
			renderDur = time.Since(rStart)
		}

		app.Window.SwapBuffers()

		if app.LogSlowFrames {
			frameDur := time.Since(frameStart)
			if frameDur > slowFrameThreshold {
				fmt.Fprintf(os.Stderr, "[slow-frame] total=%v update=%v render=%v swap=%v\n",
					frameDur.Truncate(time.Millisecond),
					updateDur.Truncate(time.Millisecond),
					renderDur.Truncate(time.Millisecond),
					(frameDur - updateDur - renderDur).Truncate(time.Millisecond))
			}
		}
	}
}

// stallWatchdog dumps all goroutine stacks to stderr if the main loop
// stops heartbeating for stallDumpThreshold. Polls every 500 ms; logs
// once per stall event by tracking the last heartbeat it acted on.
func stallWatchdog(heartbeat *atomic.Int64, stop <-chan struct{}) {
	const poll = 500 * time.Millisecond
	var lastDumpHeartbeat int64
	for {
		select {
		case <-stop:
			return
		case <-time.After(poll):
		}
		last := heartbeat.Load()
		if last == lastDumpHeartbeat {
			continue // already dumped for this stall
		}
		age := time.Since(time.Unix(0, last))
		if age <= stallDumpThreshold {
			continue
		}
		lastDumpHeartbeat = last
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		fmt.Fprintf(os.Stderr,
			"\n=== STALL DETECTED (main loop frozen %v) ===\n%s=== end stall dump ===\n\n",
			age.Truncate(100*time.Millisecond), buf[:n])
	}
}
