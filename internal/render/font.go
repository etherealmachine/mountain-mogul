package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	fontFirst    = 32  // space
	fontLast     = 126 // ~
	fontNumChars = fontLast - fontFirst + 1
)

// GlyphW, GlyphH, GlyphAdvance are updated when a font is loaded.
// GlyphH is the full line-box height (ascent + descent).
// GlyphAdvance is the advance width of 'W' — a conservative upper-bound
// suitable for sizing containers. For exact text centering use Font.TextWidth.
var (
	GlyphW       = 14
	GlyphH       = 26
	GlyphAdvance = 15
)

// glyphInfo stores per-glyph atlas coordinates and layout metrics.
type glyphInfo struct {
	u0, v0, u1, v1 float32 // normalised UV in the atlas texture
	w, h           float32 // glyph bitmap dimensions in pixels
	offsetX        float32 // horizontal offset from pen to glyph left (bearingX)
	offsetY        float32 // vertical offset from line-top to glyph top
	advance        float32 // horizontal advance in pixels
}

// Font rasterises text via a per-frame batched quad approach.
// All printable ASCII glyphs are pre-baked into a GL texture atlas.
type Font struct {
	atlasID uint32
	glyphs  [fontNumChars]glyphInfo
	ascent  float32 // distance from line-top to baseline in pixels
}

// TextWidth returns the pixel advance width of s in this font.
// Use this wherever text must be precisely centred or right-aligned.
func (f *Font) TextWidth(s string) float32 {
	var w float32
	for _, ch := range s {
		i := int(ch) - fontFirst
		if i >= 0 && i < fontNumChars {
			w += f.glyphs[i].advance
		} else {
			w += float32(GlyphAdvance)
		}
	}
	return w
}

// CharSize returns the pixel dimensions of one glyph cell.
func (f *Font) CharSize() (w, h int) { return GlyphW, GlyphH }

// DrawText renders text starting at screen position (x, y) — the top-left
// of the line box. Each glyph is placed at (pen+offsetX, y+offsetY) so
// descenders and varied cap-heights all sit on the correct baseline.
func (f *Font) DrawText(r *Renderer, text string, x, y float32, col mgl32.Vec4) {
	if r == nil || r.UIShader == nil || f.atlasID == 0 {
		return
	}
	r.useUITexture(f.atlasID)
	penX := x
	for _, ch := range text {
		i := int(ch) - fontFirst
		if i < 0 || i >= fontNumChars {
			penX += float32(GlyphAdvance)
			continue
		}
		g := &f.glyphs[i]
		if g.w > 0 && g.h > 0 {
			r.appendUIQuad(penX+g.offsetX, y+g.offsetY, g.w, g.h,
				g.u0, g.v0, g.u1, g.v1, col)
		}
		penX += g.advance
	}
}

// NewFont creates a fallback bitmap atlas from the built-in 7×13 face.
// Used when no TTF file can be loaded.
func NewFont() *Font {
	face := basicfont.Face7x13
	charW := face.Width
	charH := face.Height

	atlasW := charW * fontNumChars
	img := image.NewRGBA(image.Rect(0, 0, atlasW, charH))
	draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.White),
		Face: face,
	}
	for i := 0; i < fontNumChars; i++ {
		d.Dot = fixed.P(i*charW, face.Ascent)
		d.DrawString(string(rune(fontFirst + i)))
	}

	const scale = 2
	scaled := image.NewRGBA(image.Rect(0, 0, atlasW*scale, charH*scale))
	xdraw.NearestNeighbor.Scale(scaled, scaled.Bounds(), img, img.Bounds(), xdraw.Src, nil)

	fw := charW * scale
	fh := charH * scale
	fw2 := atlasW * scale

	f := &Font{ascent: float32(face.Ascent * scale)}
	f.atlasID = uploadAtlasRGBA(scaled.Pix, int32(fw2), int32(fh))
	atlasWF := float32(fw2)
	advPx := float32(fw + 2) // charW + 1 px letter spacing, matches old GlyphAdvance
	for i := 0; i < fontNumChars; i++ {
		u0 := float32(i*fw) / atlasWF
		u1 := float32((i+1)*fw) / atlasWF
		f.glyphs[i] = glyphInfo{
			u0: u0, v0: 0, u1: u1, v1: 1,
			w: float32(fw), h: float32(fh),
			advance: advPx,
		}
	}
	// Space has advance but no visible pixels.
	f.glyphs[' '-fontFirst].w = 0
	f.glyphs[' '-fontFirst].h = 0

	GlyphW = fw
	GlyphH = fh
	GlyphAdvance = int(advPx)
	return f
}

// LoadTTFFont loads a TTF file, rasterises all printable ASCII glyphs into a
// GL texture atlas, and returns the resulting Font. pointSize is in points at
// 72 DPI. On success the global GlyphH and GlyphAdvance vars are updated to
// match the new font's metrics.
func LoadTTFFont(path string, pointSize float64) (*Font, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    pointSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create face %s: %w", path, err)
	}
	defer face.Close()

	m := face.Metrics()
	atlasBaseline := m.Ascent.Ceil()
	atlasH := m.Height.Ceil() + 4 // extra rows so descenders don't clip

	// First pass: accumulate atlas width (ceil advance + gap per glyph).
	const glyphGap = 2
	cellW := make([]int, fontNumChars)
	var totalW int
	for i := 0; i < fontNumChars; i++ {
		adv, ok := face.GlyphAdvance(rune(fontFirst + i))
		if ok {
			cellW[i] = adv.Ceil() + glyphGap
		} else {
			cellW[i] = glyphGap
		}
		totalW += cellW[i]
	}
	if totalW < 1 {
		return nil, fmt.Errorf("font %s has no usable glyphs", path)
	}

	// Build CPU-side atlas image.
	atlasImg := image.NewRGBA(image.Rect(0, 0, totalW, atlasH))
	draw.Draw(atlasImg, atlasImg.Bounds(), image.Transparent, image.Point{}, draw.Src)

	f := &Font{ascent: float32(atlasBaseline)}
	atlasWF := float32(totalW)
	atlasHF := float32(atlasH)

	penX := 0
	for i := 0; i < fontNumChars; i++ {
		ch := rune(fontFirst + i)
		dot := fixed.P(penX, atlasBaseline)
		dr, mask, maskp, adv, ok := face.Glyph(dot, ch)
		if ok && dr.Dx() > 0 && dr.Dy() > 0 {
			// Composite white pixels through the glyph mask onto the atlas.
			// Result: RGBA (α,α,α,α) at each pixel where α = glyph coverage.
			// The UI shader multiplies this by vColor, giving correctly-tinted
			// anti-aliased text with no separate blend-mode change needed.
			draw.DrawMask(atlasImg, dr, image.White, image.Point{}, mask, maskp, draw.Over)
			f.glyphs[i] = glyphInfo{
				u0:      float32(dr.Min.X) / atlasWF,
				v0:      float32(dr.Min.Y) / atlasHF,
				u1:      float32(dr.Max.X) / atlasWF,
				v1:      float32(dr.Max.Y) / atlasHF,
				w:       float32(dr.Dx()),
				h:       float32(dr.Dy()),
				offsetX: float32(dr.Min.X - penX),
				offsetY: float32(dr.Min.Y),
				advance: float32(adv) / 64.0,
			}
		} else if ok {
			f.glyphs[i].advance = float32(adv) / 64.0
		}
		penX += cellW[i]
	}

	f.atlasID = uploadAtlasRGBA(atlasImg.Pix, int32(totalW), int32(atlasH))

	// Update global layout metrics. GlyphH = full line box. GlyphAdvance =
	// advance of 'W' — a wide capital — so container sizing never clips.
	GlyphH = int(math.Ceil(float64(m.Height) / 64.0))
	if adv, ok := face.GlyphAdvance('W'); ok {
		GlyphAdvance = int(math.Ceil(float64(adv) / 64.0))
	}
	GlyphW = GlyphAdvance

	return f, nil
}

// uploadAtlasRGBA uploads a raw RGBA pixel slice to a new GL texture and
// returns the texture ID. Shared by NewFont and LoadTTFFont.
func uploadAtlasRGBA(pix []byte, w, h int32) uint32 {
	var texID uint32
	gl.GenTextures(1, &texID)
	gl.BindTexture(gl.TEXTURE_2D, texID)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pix))
	gl.BindTexture(gl.TEXTURE_2D, 0)
	return texID
}
