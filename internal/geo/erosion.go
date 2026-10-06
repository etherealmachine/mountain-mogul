package geo

import (
	"math"
	"math/rand"
	"runtime"
	"sync"
)

// Droplet erosion: water drops land at random, roll downhill with some
// inertia, pick up ground where they speed up and drop it where they
// slow down or fill a pit. Many drops share the same flow lines, so
// those cut into rills and gullies while ridges stay put. Distances are
// in lattice samples; heights in metres.
const (
	erodeDropsPerSample = 1.0
	erodeLifetime       = 48   // steps of one sample
	erodeInertia        = 0.1  // how much a drop keeps its direction
	erodeCapacity       = 0.3  // metres of ground a drop carries per unit of slope × speed × water
	erodeMinSlope       = 0.02 // keeps drops on flats carrying a little
	erodeSpeed          = 0.3  // fraction of spare capacity taken per step
	erodeDeposit        = 0.3  // fraction of excess dropped per step
	erodeEvaporate      = 0.02 // water lost per step
	erodeGravity        = 1.0  // speed² gained per metre of drop
	erodeMaxSpeed       = 4.0
	erodeBrush          = 2 // radius, in samples, ground is taken from
)

// erodeTile is the side of the tiles drops start in. Tiles of one colour
// in a 2 × 2 checkerboard are a whole tile apart, more than two drops
// can travel towards each other, so they run in parallel and the result
// doesn't depend on scheduling.
const erodeTile = 256

// Erode runs droplet erosion over the w × ht lattice h, spacing metres
// apart, in place. drops and depth scale erodeDropsPerSample and
// erodeCapacity. seed makes it repeatable.
func Erode(h []float32, w, ht int, spacing, drops, depth float64, seed int64) {
	brush := erosionBrush(erodeBrush)
	tw, th := (w+erodeTile-1)/erodeTile, (ht+erodeTile-1)/erodeTile
	for phase := 0; phase < 4; phase++ {
		var tiles []int
		for tj := 0; tj < th; tj++ {
			for ti := 0; ti < tw; ti++ {
				if (ti%2)+2*(tj%2) == phase {
					tiles = append(tiles, tj*tw+ti)
				}
			}
		}
		ch := make(chan int)
		var wg sync.WaitGroup
		for n := 0; n < runtime.NumCPU(); n++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for t := range ch {
					ti, tj := t%tw, t/tw
					x0, y0 := ti*erodeTile, tj*erodeTile
					x1, y1 := min(x0+erodeTile, w-1), min(y0+erodeTile, ht-1)
					rng := rand.New(rand.NewSource(seed + int64(t)*7919))
					n := int(float64((x1-x0)*(y1-y0)) * erodeDropsPerSample * drops)
					for d := 0; d < n; d++ {
						x := float64(x0) + rng.Float64()*float64(x1-x0)
						y := float64(y0) + rng.Float64()*float64(y1-y0)
						erodeDrop(h, w, ht, spacing, erodeCapacity*depth, x, y, brush)
					}
				}
			}()
		}
		for _, t := range tiles {
			ch <- t
		}
		close(ch)
		wg.Wait()
	}
}

type brushTap struct {
	di, dj int
	w      float32
}

// erosionBrush weights samples within r by how close they are, summing
// to 1, so a gully is cut with a rounded bed rather than one sample wide.
func erosionBrush(r int) []brushTap {
	var taps []brushTap
	var sum float32
	for dj := -r; dj <= r; dj++ {
		for di := -r; di <= r; di++ {
			d := math.Hypot(float64(di), float64(dj))
			if d >= float64(r)+0.5 {
				continue
			}
			wt := float32(float64(r) + 0.5 - d)
			taps = append(taps, brushTap{di, dj, wt})
			sum += wt
		}
	}
	for k := range taps {
		taps[k].w /= sum
	}
	return taps
}

// heightGrad is the bilinear height at (x, y) and its gradient per sample.
func heightGrad(h []float32, w int, x, y float64) (hgt, gx, gy float64) {
	i, j := int(x), int(y)
	u, v := x-float64(i), y-float64(j)
	k := j*w + i
	h00, h10 := float64(h[k]), float64(h[k+1])
	h01, h11 := float64(h[k+w]), float64(h[k+w+1])
	gx = (h10-h00)*(1-v) + (h11-h01)*v
	gy = (h01-h00)*(1-u) + (h11-h10)*u
	hgt = h00*(1-u)*(1-v) + h10*u*(1-v) + h01*(1-u)*v + h11*u*v
	return
}

func erodeDrop(h []float32, w, ht int, spacing, capacityK, x, y float64, brush []brushTap) {
	var dx, dy, sediment float64
	speed, water := 1.0, 1.0
	for life := 0; life < erodeLifetime; life++ {
		i, j := int(x), int(y)
		u, v := x-float64(i), y-float64(j)
		hgt, gx, gy := heightGrad(h, w, x, y)
		dx = dx*erodeInertia - gx*(1-erodeInertia)
		dy = dy*erodeInertia - gy*(1-erodeInertia)
		l := math.Hypot(dx, dy)
		if l < 1e-9 {
			return
		}
		dx, dy = dx/l, dy/l
		nx, ny := x+dx, y+dy
		if nx < 0 || ny < 0 || nx >= float64(w-1) || ny >= float64(ht-1) {
			return
		}
		nh, _, _ := heightGrad(h, w, nx, ny)
		dh := nh - hgt

		capacity := math.Max(-dh/spacing, erodeMinSlope) * speed * water * capacityK
		if sediment > capacity || dh > 0 {
			drop := (sediment - capacity) * erodeDeposit
			if dh > 0 {
				drop = math.Min(dh, sediment)
			}
			sediment -= drop
			k := j*w + i
			h[k] += float32(drop * (1 - u) * (1 - v))
			h[k+1] += float32(drop * u * (1 - v))
			h[k+w] += float32(drop * (1 - u) * v)
			h[k+w+1] += float32(drop * u * v)
		} else {
			take := math.Min((capacity-sediment)*erodeSpeed, -dh)
			for _, t := range brush {
				bi, bj := i+t.di, j+t.dj
				if bi < 0 || bj < 0 || bi >= w || bj >= ht {
					continue
				}
				h[bj*w+bi] -= float32(take) * t.w
			}
			sediment += take
		}
		speed = math.Min(math.Sqrt(math.Max(speed*speed-dh*erodeGravity, 0)), erodeMaxSpeed)
		water *= 1 - erodeEvaporate
		x, y = nx, ny
	}
}
