package render

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	fontFirst    = 32  // space
	fontLast     = 126 // ~
	fontNumChars = fontLast - fontFirst + 1

	// Glyph dimensions after 2× upscale — use these everywhere layout math is needed.
	GlyphW       = 14 // pixel width of one glyph
	GlyphH       = 26 // pixel height of one glyph
	GlyphAdvance = 15 // horizontal step per character (GlyphW + 1 px letter spacing)
)

// Font renders bitmap text using basicfont.Face7x13 baked into a texture atlas.
type Font struct {
	atlasID uint32
	charW   int
	charH   int
	atlasW  int
}

// NewFont generates a texture atlas from the built-in 7×13 bitmap face.
func NewFont() *Font {
	f := &Font{
		charW: basicfont.Face7x13.Width,
		charH: basicfont.Face7x13.Height,
	}
	f.atlasW = f.charW * fontNumChars

	// Render each printable ASCII glyph into a horizontal strip.
	img := image.NewRGBA(image.Rect(0, 0, f.atlasW, f.charH))
	draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.White),
		Face: basicfont.Face7x13,
	}
	for i := 0; i < fontNumChars; i++ {
		d.Dot = fixed.P(i*f.charW, basicfont.Face7x13.Ascent)
		d.DrawString(string(rune(fontFirst + i)))
	}

	// Scale up 2× for readability (nearest-neighbour, stays crisp).
	scale := 2
	scaled := image.NewRGBA(image.Rect(0, 0, f.atlasW*scale, f.charH*scale))
	xdraw.NearestNeighbor.Scale(scaled, scaled.Bounds(), img, img.Bounds(), xdraw.Src, nil)

	sw := int32(scaled.Bounds().Dx())
	sh := int32(scaled.Bounds().Dy())
	f.charW *= scale
	f.charH *= scale
	f.atlasW = int(sw)

	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, sw, sh, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(scaled.Pix))
	gl.BindTexture(gl.TEXTURE_2D, 0)

	f.atlasID = texID
	return f
}

// DrawText renders a string at screen position (x, y) using the atlas.
// Appends one quad per glyph to the renderer's per-frame UI batch;
// flushUI inside DrawUI emits a single DrawArrays for the whole frame
// (or one per font-atlas switch, which doesn't happen since we have one
// atlas). Pre-batching this was ~one BufferSubData per character — the
// dominant CPU cost in the F5 inspector.
func (f *Font) DrawText(r *Renderer, text string, x, y float32, col mgl32.Vec4) {
	if r == nil || r.UIShader == nil || f.atlasID == 0 {
		return
	}
	r.useUITexture(f.atlasID)

	cw := float32(f.charW)
	ch := float32(f.charH)
	atlasWF := float32(f.atlasW)

	for i, ch2 := range text {
		idx := int(ch2) - fontFirst
		if idx < 0 || idx >= fontNumChars {
			idx = 0
		}
		u0 := float32(idx*f.charW) / atlasWF
		u1 := float32((idx+1)*f.charW) / atlasWF
		r.appendUIQuad(x+float32(i)*GlyphAdvance, y, cw, ch, u0, 0, u1, 1, col)
	}
}

// CharSize returns the pixel dimensions of one glyph.
func (f *Font) CharSize() (w, h int) { return f.charW, f.charH }
