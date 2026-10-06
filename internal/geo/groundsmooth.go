package geo

import "math"

// Ground smoothing removes features narrower than about 8 m, the scale
// of the resort's own earthworks in lidar: cat tracks, graded benches,
// terrain park features, building pads. Three box passes of
// groundSmoothRadius make a near-Gaussian blur of about 3 m. Steep ground
// keeps its shape: the blur fades out between groundRockFrom and
// groundRockTo degrees, so cliffs stay crisp.
const (
	groundSmoothRadius = 2.5
	groundSmoothPasses = 3
	groundRockFrom     = 35.0
	groundRockTo       = 45.0
)

// SmoothGround blurs the w × ht lattice h (spacing metres apart) in
// place, sparing steep rock. radius scales groundSmoothRadius, and
// amount (0–1) is how far the ground moves towards the blur.
func SmoothGround(h []float32, w, ht int, spacing, radius, amount float64) {
	r := max(int(math.Round(groundSmoothRadius*radius/spacing)), 1)
	s := append([]float32(nil), h...)
	for p := 0; p < groundSmoothPasses; p++ {
		boxBlur(s, w, ht, r)
	}
	for j := 0; j < ht; j++ {
		for i := 0; i < w; i++ {
			k := j*w + i
			i0, i1 := max(i-1, 0), min(i+1, w-1)
			j0, j1 := max(j-1, 0), min(j+1, ht-1)
			gx := float64(s[j*w+i1]-s[j*w+i0]) / (float64(i1-i0) * spacing)
			gz := float64(s[j1*w+i]-s[j0*w+i]) / (float64(j1-j0) * spacing)
			slope := math.Atan(math.Hypot(gx, gz)) * 180 / math.Pi
			keep := 1 - float32(amount)*(1-float32(smoothstep(groundRockFrom, groundRockTo, slope)))
			h[k] = s[k] + (h[k]-s[k])*keep
		}
	}
}
