package world

import (
	"image"
	"math"
	"runtime"
	"sync"
)

// SurfaceDetail is a CPU-side RGBA8 buffer mirroring the terrain at 1 m
// resolution (5× finer than the 5 m cell grid). Writers stamp sub-cell
// features into named channels:
//
//	R — skier track intensity as of the pixel's clock
//	G — tree-well depth      (persistent until tree edits)
//	B, A — track clock       (game minute R was last stamped, 16 bits, B low)
//
// The renderer mirrors this to a GL_RGBA8 texture; the terrain fragment
// shader samples it to render sub-cell features the 5 m mesh can't carry
// (skier tracks, tree wells). Grooming has its own map (groom_map.go).
//
// Tracks fade with age in the shader (TrackFadePerMinute) rather than by
// rewriting pixels, so ageing them costs nothing per frame. A pixel's R is
// its intensity at its clock; anything that restamps the clock first folds
// the fade since then into R, so (R, clock) always describes the same
// visible track. AgeTracks does that for the whole buffer once a game day,
// which keeps every live clock far from the 16-bit wrap (about 45 days).
//
// The buffer is fully re-derivable: G from the stored trees, R resets
// to zero on load. So it is not saved.
type SurfaceDetail struct {
	PxWidth, PxHeight int
	Pixels            []uint8 // flat RGBA8, row-major (px-major)
	Dirty             bool
	DirtyBox          image.Rectangle // px-space, inclusive on Min, exclusive on Max

}

// PxPerCell is the surface-detail resolution multiplier: each 5 m terrain
// cell maps to this many pixels per side. At PxPerCell=20 the texture
// resolves to 0.25 m/px — fine enough to draw ski-lane-width tracks
// instead of single-cell-wide blobs.
const PxPerCell = 20

// PxPerMeter returns the spatial pitch of the surface-detail buffer in
// pixels per world metre. Public callers (sim splats, world stamps) work
// in world metres; conversion to pixel space happens at the buffer's
// public API boundary.
func PxPerMeter() float32 {
	return float32(PxPerCell) / 5.0
}

// NewSurfaceDetail allocates the buffer for a terrain of `wCells × hCells`
// 5 m cells. Pixels start zeroed; nothing dirty yet.
func NewSurfaceDetail(wCells, hCells int) *SurfaceDetail {
	pw := wCells * PxPerCell
	ph := hCells * PxPerCell
	return &SurfaceDetail{
		PxWidth:  pw,
		PxHeight: ph,
		Pixels:   make([]uint8, pw*ph*4),
	}
}

// MarkDirty flags the buffer as needing a GPU re-upload and extends
// DirtyBox to cover `r` (clipped to the buffer bounds). Writers call this
// after stamping pixels so the renderer's next FlushSnowSurface uploads
// the minimum sub-region.
func (s *SurfaceDetail) MarkDirty(r image.Rectangle) {
	if s == nil {
		return
	}
	bounds := image.Rect(0, 0, s.PxWidth, s.PxHeight)
	r = r.Intersect(bounds)
	if r.Empty() {
		return
	}
	if !s.Dirty {
		s.DirtyBox = r
		s.Dirty = true
		return
	}
	s.DirtyBox = s.DirtyBox.Union(r)
}

// MarkAllDirty flags the entire buffer for re-upload. Used after bulk
// regeneration passes (e.g. after RestampTreeWells).
func (s *SurfaceDetail) MarkAllDirty() {
	if s == nil {
		return
	}
	s.DirtyBox = image.Rect(0, 0, s.PxWidth, s.PxHeight)
	s.Dirty = true
}

// channel indices into the per-pixel RGBA byte stream.
const (
	chTrack    = 0 // R — skier track intensity
	chTreeWell = 1 // G — tree-well depth
	chClockLo  = 2 // B — track clock, low byte
	chClockHi  = 3 // A — track clock, high byte
)

// TrackFadePerMinute is how much of a track's intensity is left after
// one game minute: 0.985 per 30 s, about a 23-minute half-life. The
// terrain shader applies the same rate (kTrackFade in terrain.frag).
const TrackFadePerMinute = 0.985 * 0.985

// TrackClock is the track clock for sim time simTime: whole game minutes,
// wrapping at 16 bits.
func TrackClock(simTime float64) uint16 {
	return uint16(int64(simTime/60) & 0xffff)
}

// trackFade[age] is TrackFadePerMinute^age; ages past the table read as
// fully faded (below 1/255 long before then).
var trackFade = func() []float32 {
	f := make([]float32, 512)
	for i := range f {
		f[i] = float32(math.Pow(TrackFadePerMinute, float64(i)))
	}
	return f
}()

// foldTrack restamps the pixel at off (its R byte) to clock now, folding
// the fade since its old clock and then scale into R.
func (s *SurfaceDetail) foldTrack(off int, now uint16, scale float32) {
	px := s.Pixels[off : off+4 : off+4]
	then := uint16(px[chClockLo]) | uint16(px[chClockHi])<<8
	if then == now && scale == 1 {
		return // stamped this minute: nothing has faded yet
	}
	if v := px[chTrack]; v != 0 {
		age := int(now - then)
		f := scale
		if age < len(trackFade) {
			f *= trackFade[age]
		} else {
			f = 0
		}
		px[chTrack] = uint8(float32(v) * f)
	}
	px[chClockLo], px[chClockHi] = uint8(now), uint8(now>>8)
}

// stampMaxChannelDisk writes a Gaussian-falloff disk into one channel,
// taking the max with whatever's already there. Used by RestampTreeWells
// (G channel) and equally suitable for any one-shot stamp. `peak` and
// the resulting pixel value are 0..255.
//
// Center is in pixel space (1 px = 1 m); radius is in pixels. Falloff is
// 1 − (d/r)² clipped at 0 — gives a smooth dome that goes to zero at the
// edge, cheaper than a true Gaussian and indistinguishable at this scale.
func (s *SurfaceDetail) stampMaxChannelDisk(cx, cz float32, radius float32, channel int, peak uint8) {
	if s == nil || radius <= 0 {
		return
	}
	r2 := radius * radius
	x0 := int(math.Floor(float64(cx - radius)))
	x1 := int(math.Ceil(float64(cx + radius)))
	z0 := int(math.Floor(float64(cz - radius)))
	z1 := int(math.Ceil(float64(cz + radius)))
	if x0 < 0 {
		x0 = 0
	}
	if z0 < 0 {
		z0 = 0
	}
	if x1 > s.PxWidth {
		x1 = s.PxWidth
	}
	if z1 > s.PxHeight {
		z1 = s.PxHeight
	}
	if x0 >= x1 || z0 >= z1 {
		return
	}
	peakF := float32(peak)
	stride := s.PxWidth * 4
	for z := z0; z < z1; z++ {
		dz := float32(z) + 0.5 - cz
		row := z * stride
		for x := x0; x < x1; x++ {
			dx := float32(x) + 0.5 - cx
			d2 := dx*dx + dz*dz
			if d2 >= r2 {
				continue
			}
			falloff := 1 - d2/r2
			v := uint8(falloff * peakF)
			off := row + x*4 + channel
			if v > s.Pixels[off] {
				s.Pixels[off] = v
			}
		}
	}
	s.MarkDirty(image.Rect(x0, z0, x1, z1))
}

// zeroChannel writes 0 into one channel across the whole buffer. Used by
// RestampTreeWells before re-stamping — wells are not additive across
// frames; the new tree set is the new wells.
func (s *SurfaceDetail) zeroChannel(channel int) {
	if s == nil {
		return
	}
	for i := channel; i < len(s.Pixels); i += 4 {
		s.Pixels[i] = 0
	}
	s.MarkAllDirty()
}

// SplatTrack adds intensity to R over the 2×2 pixels from the one under
// world position (wx, wz), stamped at track clock now; R caps at 255.
// Marks Dirty and extends DirtyBox.
//
// Skier physics drives this (via SplatTrackSegment) on every substep
// when the agent is actively skiing. At PxPerCell=20 the splat covers
// 0.5 m square, about a skier's width.
func (s *SurfaceDetail) SplatTrack(wx, wz float32, intensity uint8, now uint16) {
	if s == nil || intensity == 0 {
		return
	}
	ppm := PxPerMeter()
	cx := int(wx * ppm)
	cz := int(wz * ppm)
	if cx < -1 || cx > s.PxWidth || cz < -1 || cz > s.PxHeight {
		return
	}
	x0 := cx
	x1 := cx + 2
	z0 := cz
	z1 := cz + 2
	if x0 < 0 {
		x0 = 0
	}
	if z0 < 0 {
		z0 = 0
	}
	if x1 > s.PxWidth {
		x1 = s.PxWidth
	}
	if z1 > s.PxHeight {
		z1 = s.PxHeight
	}
	if x0 >= x1 || z0 >= z1 {
		return
	}
	stride := s.PxWidth * 4
	add := int(intensity)
	for z := z0; z < z1; z++ {
		for x := x0; x < x1; x++ {
			off := z*stride + x*4 + chTrack
			s.foldTrack(off, now, 1)
			v := int(s.Pixels[off]) + add
			if v > 255 {
				v = 255
			}
			s.Pixels[off] = uint8(v)
		}
	}
	s.MarkDirty(image.Rect(x0, z0, x1, z1))
}

// SplatTrackSegment interpolates SplatTrack along the straight world-space
// segment (wx0, wz0)→(wx1, wz1), one splat every trackSplatSpacing
// pixels: the 2×2 splats meet into a continuous line at any
// TimeScale. Cheap for short segments (the per-tick agent step is a
// handful of pixels).
func (s *SurfaceDetail) SplatTrackSegment(wx0, wz0, wx1, wz1 float32, intensity uint8, now uint16) {
	if s == nil {
		return
	}
	dx := wx1 - wx0
	dz := wz1 - wz0
	distM := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	steps := int(distM*PxPerMeter()/trackSplatSpacing) + 1
	if steps > 256 {
		steps = 256 // safety cap for an unexpectedly large segment
	}
	inv := 1.0 / float32(steps)
	for i := 0; i <= steps; i++ {
		f := float32(i) * inv
		s.SplatTrack(wx0+dx*f, wz0+dz*f, intensity, now)
	}
}

// trackSplatSpacing is the pixels between splats along a track: every
// second pixel, so the 2×2 splats just meet. Tracks were 3×3 splats on
// every pixel until 2026-10-09; in a crowd they cost a sixth of the sim
// (every pixel touched is a cache miss in a texture of hundreds of
// megabytes), so they're now a slightly thinner line for under half the
// pixels.
const trackSplatSpacing = 2

// AgeTracks restamps every track pixel to clock now, folding in the fade
// since it was skied and then multiplying by factor (clamped to [0, 1]):
// 1 just ages them, less buries them under new snow. Run once a game day
// so no live clock gets near the wrap. Split across cores: the buffer is
// tens of millions of pixels.
func (s *SurfaceDetail) AgeTracks(now uint16, factor float32) {
	if s == nil {
		return
	}
	factor = min(max(factor, 0), 1)
	rows := s.PxHeight
	workers := min(runtime.NumCPU(), rows)
	if workers < 1 {
		return
	}
	stride := s.PxWidth * 4
	var wg sync.WaitGroup
	for w := range workers {
		z0, z1 := rows*w/workers, rows*(w+1)/workers
		wg.Add(1)
		go func() {
			defer wg.Done()
			for off := z0*stride + chTrack; off < z1*stride; off += 4 {
				if s.Pixels[off] != 0 {
					s.foldTrack(off, now, factor)
				}
			}
		}()
	}
	wg.Wait()
	s.MarkAllDirty()
}

// ClearTrackSwath zeros R within halfWidth metres of the world-space
// segment (x0, z0)→(x1, z1). Used by snowcats: grooming wipes tracks.
func (s *SurfaceDetail) ClearTrackSwath(x0, z0, x1, z1, halfWidth float32) {
	if s == nil {
		return
	}
	ppm := PxPerMeter()
	box := image.Rect(
		int(math.Floor(float64((min(x0, x1)-halfWidth)*ppm))), int(math.Floor(float64((min(z0, z1)-halfWidth)*ppm))),
		int(math.Ceil(float64((max(x0, x1)+halfWidth)*ppm))), int(math.Ceil(float64((max(z0, z1)+halfWidth)*ppm))),
	).Intersect(image.Rect(0, 0, s.PxWidth, s.PxHeight))
	if box.Empty() {
		return
	}
	dx, dz := x1-x0, z1-z0
	l2 := dx*dx + dz*dz
	stride := s.PxWidth * 4
	any := false
	for pz := box.Min.Y; pz < box.Max.Y; pz++ {
		for px := box.Min.X; px < box.Max.X; px++ {
			off := pz*stride + px*4 + chTrack
			if s.Pixels[off] == 0 {
				continue
			}
			cx, cz := (float32(px)+0.5)/ppm, (float32(pz)+0.5)/ppm
			t := float32(0)
			if l2 > 0 {
				t = min(max(((cx-x0)*dx+(cz-z0)*dz)/l2, 0), 1)
			}
			ex, ez := cx-(x0+dx*t), cz-(z0+dz*t)
			if ex*ex+ez*ez <= halfWidth*halfWidth {
				s.Pixels[off] = 0
				any = true
			}
		}
	}
	if any {
		s.MarkDirty(box)
	}
}

// ClearTrackBox zeros R inside the pixel-space rectangle `box` (clipped
// to bounds). Used by the snowcat tick: real grooming destroys tracks.
func (s *SurfaceDetail) ClearTrackBox(box image.Rectangle) {
	if s == nil {
		return
	}
	bounds := image.Rect(0, 0, s.PxWidth, s.PxHeight)
	box = box.Intersect(bounds)
	if box.Empty() {
		return
	}
	stride := s.PxWidth * 4
	any := false
	for z := box.Min.Y; z < box.Max.Y; z++ {
		row := z * stride
		for x := box.Min.X; x < box.Max.X; x++ {
			off := row + x*4 + chTrack
			if s.Pixels[off] != 0 {
				s.Pixels[off] = 0
				any = true
			}
		}
	}
	if any {
		s.MarkDirty(box)
	}
}
