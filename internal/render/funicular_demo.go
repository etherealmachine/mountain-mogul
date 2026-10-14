//go:build demo

package render

import "mountain-mogul/internal/world"

// The demo has no funicular.
func newFunicBatch(*Renderer, *world.Lift, *world.Terrain, [3]float32) *railBatch { return nil }
