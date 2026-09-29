package world

import (
	"math"
	"runtime"
	"sync"
)

// HorizonDirs is the number of compass directions the horizon map
// samples. Direction k points along azimuth 2πk/HorizonDirs in the world
// XZ plane: k = 0 is +X (east), increasing toward +Z (south).
const HorizonDirs = 16

// horizonBias lifts the observer above the ground so gentle bilinear
// bumps in the cell's own neighbourhood don't shade it.
const horizonBias = float32(0.5)

// horizonMaxCells caps how far rays march (3 km at 5 m cells); farther
// terrain rarely rises enough to matter.
const horizonMaxCells = 600

// HorizonMap holds, per cell and direction, the elevation angle
// (radians) of the terrain horizon: the sun is hidden from the cell while
// it sits below that angle in that direction. Derived from
// GroundElevation; not saved.
type HorizonMap struct {
	w, h   int
	angles []float32 // [(x*h+z)*HorizonDirs + k]
}

// Horizon returns the terrain's horizon map, computing it on first use
// and after any elevation change flagged by RecomputeSlopes.
func (t *Terrain) Horizon() *HorizonMap {
	if t.horizon == nil || t.horizonStale || t.horizon.w != t.Width || t.horizon.h != t.Height {
		t.horizon = computeHorizon(t)
		t.horizonStale = false
	}
	return t.horizon
}

// InvalidateHorizon marks the horizon map for recomputation on next use.
func (t *Terrain) InvalidateHorizon() { t.horizonStale = true }

func computeHorizon(t *Terrain) *HorizonMap {
	W, H := t.Width, t.Height
	hm := &HorizonMap{w: W, h: H, angles: make([]float32, W*H*HorizonDirs)}
	if W < 2 || H < 2 {
		return hm
	}
	elev := make([]float32, W*H)
	maxElev := float32(math.Inf(-1))
	for x := 0; x < W; x++ {
		for z := 0; z < H; z++ {
			e := t.Cells[x][z].GroundElevation
			elev[x*H+z] = e
			maxElev = max(maxElev, e)
		}
	}
	at := func(fx, fz float32) float32 {
		x0, z0 := int(fx), int(fz)
		x0, z0 = min(x0, W-2), min(z0, H-2)
		tx, tz := fx-float32(x0), fz-float32(z0)
		a := elev[x0*H+z0]*(1-tz) + elev[x0*H+z0+1]*tz
		b := elev[(x0+1)*H+z0]*(1-tz) + elev[(x0+1)*H+z0+1]*tz
		return a*(1-tx) + b*tx
	}
	var dirs [HorizonDirs][2]float32
	for k := range dirs {
		a := 2 * math.Pi * float64(k) / HorizonDirs
		dirs[k] = [2]float32{float32(math.Cos(a)), float32(math.Sin(a))}
	}
	cellM := float32(CellSize)

	var wg sync.WaitGroup
	rows := make(chan int, W)
	for x := 0; x < W; x++ {
		rows <- x
	}
	close(rows)
	for n := 0; n < runtime.NumCPU(); n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for x := range rows {
				for z := 0; z < H; z++ {
					h0 := elev[x*H+z] + horizonBias
					out := hm.angles[(x*H+z)*HorizonDirs:]
					for k, d := range dirs {
						best := float32(-1) // tan of the horizon angle; below −45° is irrelevant
						for dist := float32(1); dist <= horizonMaxCells; {
							fx := float32(x) + d[0]*dist
							fz := float32(z) + d[1]*dist
							if fx < 0 || fz < 0 || fx > float32(W-1) || fz > float32(H-1) {
								break
							}
							if (maxElev-h0)/(dist*cellM) <= best {
								break // nothing farther can rise above the current horizon
							}
							if tan := (at(fx, fz) - h0) / (dist * cellM); tan > best {
								best = tan
							}
							if dist < 12 {
								dist++
							} else {
								dist *= 1.08
							}
						}
						out[k] = float32(math.Atan(float64(best)))
					}
				}
			}
		}()
	}
	wg.Wait()
	return hm
}

// Angle is the horizon elevation angle (radians) at cell (x, z) toward
// world azimuth az (radians, atan2(dz, dx)), interpolated between the
// sampled directions.
func (hm *HorizonMap) Angle(x, z int, az float64) float32 {
	f := az / (2 * math.Pi) * HorizonDirs
	f -= math.Floor(f/HorizonDirs) * HorizonDirs
	k0 := int(f) % HorizonDirs
	k1 := (k0 + 1) % HorizonDirs
	t := float32(f - math.Floor(f))
	a := hm.angles[(x*hm.h+z)*HorizonDirs:]
	return a[k0]*(1-t) + a[k1]*t
}

// sunPenumbra is the half-width of the soft edge where the sun sinks
// behind terrain: the sun's own disc (0.25°) widened so 5 m cells don't
// show hard steps.
const sunPenumbra = 1.5 * math.Pi / 180

// SunVisibility is how much of a light from unit direction dir (toward
// the light, Y up) reaches cell (x, z) past the terrain: 1 in the open,
// 0 behind a ridge, soft in between.
func (hm *HorizonMap) SunVisibility(x, z int, dir [3]float32) float32 {
	elev := math.Asin(math.Max(-1, math.Min(1, float64(dir[1]))))
	az := math.Atan2(float64(dir[2]), float64(dir[0]))
	d := float32(elev) - hm.Angle(x, z, az)
	t := min(max((d+sunPenumbra)/(2*sunPenumbra), 0), 1)
	return t * t * (3 - 2*t)
}
