package render

import (
	"fmt"
	"image"
	"os"

	"mountain-mogul/internal/world"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// The GPU mirror of Terrain.Surface is cut into square tiles, the layers
// of one array texture: a texture's side is capped (16,384 px on Apple
// GPUs), and Kirkwood's surface is 17,880 px across, so one 2D texture
// failed to allocate and the map drew no tracks, wells or groom edges.
//
// Each layer holds surfaceTile × surfaceTile pixels of the surface plus
// a one-pixel apron copied from its neighbours, so filtering across a
// tile's edge reads the right texels. terrain.frag picks the layer
// (surfaceSample, surfaceTexel); it must agree on surfaceTile.
const (
	surfaceTile  = 2048
	surfaceLayer = surfaceTile + 2
)

// surfaceTiles is the tiles across and down a surface of w × h pixels.
func surfaceTiles(w, h int) (int, int) {
	return (w + surfaceTile - 1) / surfaceTile, (h + surfaceTile - 1) / surfaceTile
}

// BuildSnowSurfaceTex (re)allocates the GPU mirror of Terrain.Surface and
// uploads its current contents. Called at scene install time after
// BuildTerrainMesh. Subsequent edits go through FlushSnowSurface, which
// uploads only the dirty sub-region.
func (r *Renderer) BuildSnowSurfaceTex(t *world.Terrain) {
	if t == nil || t.Surface == nil {
		return
	}
	sd := t.Surface

	if r.scene.snowSurfaceTex != 0 {
		gl.DeleteTextures(1, &r.scene.snowSurfaceTex)
		r.scene.snowSurfaceTex = 0
	}

	tx, tz := surfaceTiles(sd.PxWidth, sd.PxHeight)
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, tex)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D_ARRAY, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexImage3D(gl.TEXTURE_2D_ARRAY, 0, gl.RGBA8,
		surfaceLayer, surfaceLayer, int32(tx*tz), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, nil)
	if err := gl.GetError(); err != gl.NO_ERROR {
		fmt.Fprintf(os.Stderr, "snow surface: allocating %d tiles of %d px failed (GL error 0x%x)\n", tx*tz, surfaceLayer, err)
	}
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, 0)

	r.scene.snowSurfaceTex = tex
	r.scene.snowSurfaceTexW = int32(sd.PxWidth)
	r.scene.snowSurfaceTexH = int32(sd.PxHeight)
	r.uploadSnowSurface(sd, image.Rect(0, 0, sd.PxWidth, sd.PxHeight))
	sd.Dirty = false
	sd.DirtyBox = image.Rectangle{}
}

// FlushSnowSurface uploads the dirty sub-region of Terrain.Surface to the
// GPU texture and clears the buffer's dirty flag. No-op if the texture
// hasn't been built yet or nothing is dirty. Coalesced to once per frame
// by the caller.
func (r *Renderer) FlushSnowSurface(t *world.Terrain) {
	if t == nil || t.Surface == nil || r.scene.snowSurfaceTex == 0 {
		return
	}
	sd := t.Surface
	if !sd.Dirty {
		return
	}
	r.uploadSnowSurface(sd, sd.DirtyBox)
	sd.Dirty = false
	sd.DirtyBox = image.Rectangle{}
}

// uploadSnowSurface sends the surface's pixels in box to every tile whose
// layer (apron included) overlaps it.
func (r *Renderer) uploadSnowSurface(sd *world.SurfaceDetail, box image.Rectangle) {
	box = box.Intersect(image.Rect(0, 0, sd.PxWidth, sd.PxHeight))
	if box.Empty() {
		return
	}
	tilesX, tilesZ := surfaceTiles(sd.PxWidth, sd.PxHeight)
	// Sub-row uploads need UNPACK_ROW_LENGTH so glTexSubImage3D walks
	// the buffer's full stride while only reading the box. ALIGNMENT=1
	// because we're feeding tightly packed uint8s.
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, int32(sd.PxWidth))
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, r.scene.snowSurfaceTex)
	// Tiles whose layer, one pixel wider each side, reaches the box.
	for tz := max((box.Min.Y-1)/surfaceTile, 0); tz <= min(box.Max.Y/surfaceTile, tilesZ-1); tz++ {
		for tx := max((box.Min.X-1)/surfaceTile, 0); tx <= min(box.Max.X/surfaceTile, tilesX-1); tx++ {
			// The layer's texel 0 is the surface's pixel tile*surfaceTile - 1.
			ox, oz := tx*surfaceTile-1, tz*surfaceTile-1
			b := box.Intersect(image.Rect(ox, oz, ox+surfaceLayer, oz+surfaceLayer))
			if b.Empty() {
				continue
			}
			off := (b.Min.Y*sd.PxWidth + b.Min.X) * 4
			gl.TexSubImage3D(gl.TEXTURE_2D_ARRAY, 0,
				int32(b.Min.X-ox), int32(b.Min.Y-oz), int32(tz*tilesX+tx),
				int32(b.Dx()), int32(b.Dy()), 1,
				gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(sd.Pixels[off:]))
		}
	}
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)
	gl.BindTexture(gl.TEXTURE_2D_ARRAY, 0)
}
