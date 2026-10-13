//go:build demo

package world

import "github.com/go-gl/mathgl/mgl32"

// The demo has no heli (heli.go): no flight state, and these are never
// reached, since Cableless is always false.

type liftExtras struct{}

func initHeli(*Lift)                          {}
func HeliWorldPos(*Lift, *Terrain) mgl32.Vec3 { return mgl32.Vec3{} }
func HeliHeading(*Lift) float32               { return 0 }
