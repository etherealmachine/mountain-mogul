package render

import (
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// sunVisTexUnit is the texture unit the sun-visibility map binds to for
// the terrain, static and dynamic shaders (units 0–2 are taken).
const sunVisTexUnit = 3

// sunVisRedoCos: the map is rebuilt once the key light has moved more
// than ~0.1° since the last build.
var sunVisRedoCos = float32(0.9999985)

// terrainShadow mirrors HorizonMap.SunVisibility for the current key light
// as an R8 texture, one texel per cell, so shaders can darken whatever sits
// in a ridge's shadow.
type terrainShadow struct {
	tex     uint32
	w, h    int
	dir     mgl32.Vec3
	horizon *world.HorizonMap
	pixels  []uint8
}

// update rebuilds the visibility texture if the light or terrain moved.
func (s *terrainShadow) update(t *world.Terrain, dir mgl32.Vec3) {
	hm := t.Horizon()
	if s.tex != 0 && hm == s.horizon && s.w == t.Width && s.h == t.Height && s.dir.Dot(dir) > sunVisRedoCos {
		return
	}
	W, H := t.Width, t.Height
	if len(s.pixels) != W*H {
		s.pixels = make([]uint8, W*H)
	}
	d := [3]float32{dir[0], dir[1], dir[2]}
	for z := 0; z < H; z++ {
		for x := 0; x < W; x++ {
			s.pixels[z*W+x] = uint8(hm.SunVisibility(x, z, d)*255 + 0.5)
		}
	}
	if s.tex == 0 || s.w != W || s.h != H {
		if s.tex == 0 {
			gl.GenTextures(1, &s.tex)
		}
		gl.BindTexture(gl.TEXTURE_2D, s.tex)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
		gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.R8, int32(W), int32(H), 0, gl.RED, gl.UNSIGNED_BYTE, gl.Ptr(s.pixels))
	} else {
		gl.BindTexture(gl.TEXTURE_2D, s.tex)
		gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
		gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, int32(W), int32(H), gl.RED, gl.UNSIGNED_BYTE, gl.Ptr(s.pixels))
	}
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	s.w, s.h, s.dir, s.horizon = W, H, dir, hm
}

// bind attaches the texture to its unit; call once per frame before the
// world passes.
func (s *terrainShadow) bind() {
	gl.ActiveTexture(gl.TEXTURE0 + sunVisTexUnit)
	gl.BindTexture(gl.TEXTURE_2D, s.tex)
	gl.ActiveTexture(gl.TEXTURE0)
}

// apply sets the shader's sun-visibility uniforms; on = false leaves
// everything fully lit (editor, menus).
func (s *terrainShadow) apply(sh *Shader, on bool) {
	enabled := 0
	if on && s.tex != 0 {
		enabled = 1
	}
	sh.SetInt("uSunVisOn", enabled)
	sh.SetInt("uSunVis", sunVisTexUnit)
	sh.SetVec2("uSunVisWorld", mgl32.Vec2{float32(s.w) * world.CellSize, float32(s.h) * world.CellSize})
}
