package engine

import "mountain-mogul/internal/render"

// Scene is the interface implemented by all game scenes.
// Defined here (in engine) so that engine.App can hold []Scene
// without importing the scene package (which imports engine).
type Scene interface {
	Init(app *App) error
	Update(dt float64)
	Render(r *render.Renderer)
	Destroy()
}
