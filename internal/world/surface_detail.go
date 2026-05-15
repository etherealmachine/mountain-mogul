package world

import (
	"image"
	"math"
)

// SurfaceDetail is a CPU-side RGBA8 buffer mirroring the terrain at 1 m
// resolution (5× finer than the 5 m cell grid). Writers stamp sub-cell
// features into named channels:
//
//	R — skier track intensity (decays in sim time)
//	G — tree-well depth      (persistent until tree edits)
//	B — groom-edge mask      (derived from per-cell Grooming)
//	A — reserved             (ice patches, footprints, …)
//
// The renderer mirrors this to a GL_RGBA8 texture; the terrain fragment
// shader samples it to render sub-cell features the 5 m mesh can't carry
// (skier tracks, tree wells, sharper groomed/ungroomed edges).
//
// The buffer is fully re-derivable: G from per-cell TreeDensity, B from
// per-cell Grooming, R resets to zero on load. So it is not saved.
type SurfaceDetail struct {
	PxWidth, PxHeight int
	Pixels            []uint8 // flat RGBA8, row-major (px-major)
	Dirty             bool
	DirtyBox          image.Rectangle // px-space, inclusive on Min, exclusive on Max

	// EdgeCells[cx*Hcells + cz] is true if the cell was a groom-edge
	// cell at the most recent RecomputeGroomEdges call. Lets the
	// recompute skip the PxPerCell² per-pixel clear walk on cells that
	// never had an edge — without this, the steady-state scan was
	// ~1.5 M memory loads on a 60×60 map.
	EdgeCells []bool
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
		PxWidth:   pw,
		PxHeight:  ph,
		Pixels:    make([]uint8, pw*ph*4),
		EdgeCells: make([]bool, wCells*hCells),
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
// regeneration passes (e.g. after RestampTreeWells or RecomputeGroomEdges).
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
	chGroomEdge = 2 // B — groom-edge mask
	chReserved = 3 // A
)

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

// SplatTrack writes a 3×3 additive disk into R centred on the pixel
// under world position (wx, wz). `intensity` is the peak R rise
// (0..255); we cap per-pixel at 255. Marks Dirty and extends DirtyBox.
//
// Skier physics drives this from tickSkier on every substep when the
// agent is actively skiing. At PxPerCell=20 the disk covers ~0.75 m
// world-square, roughly skier width; adjacent substep splats overlap
// into a continuous track.
func (s *SurfaceDetail) SplatTrack(wx, wz float32, intensity uint8) {
	if s == nil || intensity == 0 {
		return
	}
	ppm := PxPerMeter()
	cx := int(wx * ppm)
	cz := int(wz * ppm)
	if cx < -1 || cx > s.PxWidth || cz < -1 || cz > s.PxHeight {
		return
	}
	x0 := cx - 1
	x1 := cx + 2
	z0 := cz - 1
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
// segment (wx0, wz0)→(wx1, wz1), one splat per pixel-step so the 3×3
// disks overlap into a continuous line at any TimeScale. Cheap for
// short segments (the per-tick agent step is typically a handful of
// pixels).
func (s *SurfaceDetail) SplatTrackSegment(wx0, wz0, wx1, wz1 float32, intensity uint8) {
	if s == nil {
		return
	}
	dx := wx1 - wx0
	dz := wz1 - wz0
	distM := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	steps := int(distM*PxPerMeter()) + 1
	if steps > 256 {
		steps = 256 // safety cap for an unexpectedly large segment
	}
	inv := 1.0 / float32(steps)
	for i := 0; i <= steps; i++ {
		f := float32(i) * inv
		s.SplatTrack(wx0+dx*f, wz0+dz*f, intensity)
	}
}

// DecayTracks multiplies every R pixel by `factor` (clamped to [0, 1]).
// Used by the slow demand-poll cadence to age skier tracks; factor ≈
// 0.985 per 30 s sim time gives a ~30-min half-life.
func (s *SurfaceDetail) DecayTracks(factor float32) {
	if s == nil {
		return
	}
	if factor < 0 {
		factor = 0
	}
	if factor > 1 {
		factor = 1
	}
	any := false
	for i := chTrack; i < len(s.Pixels); i += 4 {
		v := s.Pixels[i]
		if v == 0 {
			continue
		}
		nv := uint8(float32(v) * factor)
		if nv != v {
			s.Pixels[i] = nv
			any = true
		}
	}
	if any {
		s.MarkAllDirty()
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
