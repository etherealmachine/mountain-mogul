//go:build demo

// The demo's entry point: no flags, straight to the splash.
package main

import (
	"os"
	"path/filepath"

	"mountain-mogul/internal/engine"
	"mountain-mogul/internal/scene"
)

func main() {
	app := engine.NewApp("Mountain Mogul Demo", 1280, 720, assetDir())
	defer app.Destroy()
	app.PushScene(scene.NewIntroScene())
	app.Run()
}

// assetDir finds the assets folder beside the executable, falling back
// to one in the working directory.
func assetDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "assets")
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	return "assets"
}
