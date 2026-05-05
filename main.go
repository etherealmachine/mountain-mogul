package main

import (
	"runtime"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/scene"
)

func init() { runtime.LockOSThread() }

func main() {
	app := engine.NewApp("Mountain Mogul", 1280, 720, "assets")
	defer app.Destroy()

	app.PushScene(scene.NewStartMenu())
	app.Run()
}
