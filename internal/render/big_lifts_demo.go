//go:build demo

package render

import (
	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/world"
)

// The demo has none of the bigger lifts or the heli (big_lifts.go).

type bigLifts struct{}

func loadBigLifts(*Renderer, string)          {}
func addHelipads(*Renderer, *world.World)     {}
func drawHelicopters(*Renderer, *world.World) {}
func drawBigLiftChairs(*Renderer)             {}
func drawBubbleHoods(*Renderer)               {}

func addBigLiftChair(*Renderer, *world.Lift, float32, DynamicInstance, bool) {}
func bigLiftShadowBatches(*Renderer) []*Batch                                { return nil }
func addBigLiftThumbs(map[ThumbID]thumbSpec)                                 {}

// HelipadTransform is never reached in the demo (no heli lift type).
func HelipadTransform(_, _ mgl32.Vec2, _ *world.Terrain) mgl32.Mat4 { return mgl32.Ident4() }
