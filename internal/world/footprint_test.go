package world

import (
	"math"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

func TestFootprintRectContainsRotated(t *testing.T) {
	// 10 m along local X, 2 m along local Z.
	r := FootprintRect{Center: mgl32.Vec2{100, 100}, HalfX: 5, HalfZ: 1}
	if !r.Contains(mgl32.Vec2{104, 100}, 0) || r.Contains(mgl32.Vec2{100, 104}, 0) {
		t.Fatal("unrotated rect: long axis should be world X")
	}
	// A quarter turn swings local X onto world ∓Z.
	r.Rotation = math.Pi / 2
	if r.Contains(mgl32.Vec2{104, 100}, 0) || !r.Contains(mgl32.Vec2{100, 104}, 0) {
		t.Fatal("quarter-turned rect: long axis should be world Z")
	}
	minX, minZ, maxX, maxZ := r.Bounds()
	if abs32(minX-99) > 1e-4 || abs32(maxX-101) > 1e-4 || abs32(minZ-95) > 1e-4 || abs32(maxZ-105) > 1e-4 {
		t.Fatalf("quarter-turned bounds = (%v,%v)-(%v,%v)", minX, minZ, maxX, maxZ)
	}
}

func TestFootprintRectOverlapsRotated(t *testing.T) {
	long := func(x, z, rot float32) FootprintRect {
		return FootprintRect{Center: mgl32.Vec2{x, z}, HalfX: 5, HalfZ: 1, Rotation: rot}
	}
	cases := []struct {
		name string
		a, b FootprintRect
		want bool
	}{
		{"side by side along Z", long(0, 0, 0), long(0, 3, 0), false},
		{"flush edges touch only", long(0, 0, 0), long(0, 2, 0), false},
		{"both turned, stacked along Z", long(0, 0, math.Pi/2), long(0, 3, math.Pi/2), true},
		{"both turned, side by side along X", long(0, 0, math.Pi/2), long(3, 0, math.Pi/2), false},
		{"crossed", long(0, 0, 0), long(0, 0, math.Pi/2), true},
		// A 45° bar clears a box its AABB would hit.
		{"diagonal misses corner", long(0, 0, math.Pi/4), FootprintRect{Center: mgl32.Vec2{3.5, 3.5}, HalfX: 0.5, HalfZ: 0.5}, false},
		{"diagonal hits on its axis", long(0, 0, math.Pi/4), FootprintRect{Center: mgl32.Vec2{2.5, -2.5}, HalfX: 0.5, HalfZ: 0.5}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.a.Overlaps(c.b); got != c.want {
				t.Fatalf("a.Overlaps(b) = %v, want %v", got, c.want)
			}
			if got := c.b.Overlaps(c.a); got != c.want {
				t.Fatalf("b.Overlaps(a) = %v, want %v", got, c.want)
			}
		})
	}
}
