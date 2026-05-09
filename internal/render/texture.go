package render

import (
	"image"
	_ "image/jpeg" // register JPEG decoder for image.Decode
	"image/png"
	"os"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// LoadTexture loads a PNG or JPEG file and uploads it to OpenGL as an
// RGBA8 texture. Returns a 1×1 white texture on error.
func LoadTexture(path string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return whiteTexture(), err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return whiteTexture(), err
	}

	rgba := toRGBA(img)
	bounds := rgba.Bounds()
	w := int32(bounds.Dx())
	h := int32(bounds.Dy())

	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))
	gl.GenerateMipmap(gl.TEXTURE_2D)

	gl.BindTexture(gl.TEXTURE_2D, 0)
	return texID, nil
}

// LoadIconTexture loads a black-on-transparent PNG (the standard Phosphor
// pattern) and uploads it as a white-on-transparent alpha mask. The UI
// fragment shader is `texture * uColor`, so a white-RGB texture lets the
// caller tint the icon by passing a colour to DrawTexturedRect — exactly
// how the bitmap font already works.
//
// Falls back to a 1×1 white texture on error so a missing icon doesn't
// crash the render path.
func LoadIconTexture(path string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return whiteTexture(), err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return whiteTexture(), err
	}

	rgba := toRGBA(img)
	// Force RGB to white, preserve alpha. After this, sampling the texture
	// returns vec4(1, 1, 1, alpha) and the shader's multiply with uColor
	// yields the tinted icon over the alpha mask.
	for i := 0; i+3 < len(rgba.Pix); i += 4 {
		rgba.Pix[i] = 255
		rgba.Pix[i+1] = 255
		rgba.Pix[i+2] = 255
	}
	bounds := rgba.Bounds()
	w := int32(bounds.Dx())
	h := int32(bounds.Dy())

	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))
	gl.GenerateMipmap(gl.TEXTURE_2D)

	gl.BindTexture(gl.TEXTURE_2D, 0)
	return texID, nil
}

// whiteTexture returns a 1x1 white texture.
func whiteTexture() uint32 {
	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)
	pixels := []uint8{255, 255, 255, 255}
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixels))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	return texID
}

// toRGBA converts an image.Image to *image.RGBA.
func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	return rgba
}
