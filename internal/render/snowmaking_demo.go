//go:build demo

package render

import "mountain-mogul/internal/world"

// The demo has no snowmaking (snowmaking.go).

type snowmakingRender struct{}

func loadSnowmaking(*Renderer, string)              {}
func drawSnowGuns(*Renderer, *world.World, float32) {}
func drawSnowPlumes(*Renderer)                      {}
