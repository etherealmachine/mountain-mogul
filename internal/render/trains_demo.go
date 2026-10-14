//go:build demo

package render

import "mountain-mogul/internal/world"

// The demo has no railways (trains.go).

type trainDrawing struct{}

func loadTrains(*Renderer, string)       {}
func drawTrains(*Renderer, *world.World) {}
