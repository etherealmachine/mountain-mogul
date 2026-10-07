package world

import "math"

// The drawn mogul field, mirrored from assets/shaders/mogul.glsl so
// skiers ride the bumps the GPU draws (render.VisualElevationAt). The
// constants and hashes must match the shader's.
const (
	mogulAmp   = 0.45 // metres, crest above the mean at full moguls
	mogulSU    = 6.0  // metres between bumps across the fall line
	mogulSV    = 7.0  // and down it
	mogulTile  = 12.0
	mogulWarp  = 0.9
	mogulWarpL = 9.0
)

// RefreshMogulFallLines fills MogulMap.Dir from the terrain: each cell's
// fall line averaged over its 3×3 neighbourhood, so the field bends
// smoothly. Call after the ground changes.
func (t *Terrain) RefreshMogulFallLines() {
	m := t.Moguls
	if m == nil {
		return
	}
	if len(m.Dir) != 2*m.wCells*m.hCells {
		m.Dir = make([]uint8, 2*m.wCells*m.hCells)
	}
	for x := 0; x < m.wCells; x++ {
		for z := 0; z < m.hCells; z++ {
			var sx, sz float32
			for dx := -1; dx <= 1; dx++ {
				for dz := -1; dz <= 1; dz++ {
					if t.InBounds(x+dx, z+dz) {
						gx, gz := t.GradientAt(x+dx, z+dz)
						sx, sz = sx-gx, sz-gz
					}
				}
			}
			l := float32(math.Sqrt(float64(sx*sx + sz*sz)))
			if l < 1e-6 {
				sx, sz, l = 0, 1, 1
			}
			i := 2 * (z*m.wCells + x)
			m.Dir[i] = uint8(math.Round(float64((sx/l*0.5 + 0.5) * 255)))
			m.Dir[i+1] = uint8(math.Round(float64((sz/l*0.5 + 0.5) * 255)))
		}
	}
}

// MogulHeightAt is the drawn mogul surface's height above the snow at
// world (wx, wz), in metres.
func (t *Terrain) MogulHeightAt(wx, wz float32) float32 {
	m := t.Moguls
	if m == nil {
		return 0
	}
	s := mogulSizeAt(m, wx, wz)
	if s <= 0.002 {
		return 0
	}
	if len(m.Dir) == 0 {
		t.RefreshMogulFallLines()
	}
	dx, dz := mogulFallLineAt(m, wx, wz)
	return mogulAmp * s * mogulShape(wx, wz, dx, dz)
}

// MogulSizeAt is how big the moguls are at world (wx, wz), 0..1, from the
// 1 m map.
func (t *Terrain) MogulSizeAt(wx, wz float32) float32 {
	if t.Moguls == nil {
		return 0
	}
	return mogulSizeAt(t.Moguls, wx, wz)
}

// mogulSizeAt samples the map as the GPU's linear filter does, texel
// centres at +0.5 m.
func mogulSizeAt(m *MogulMap, wx, wz float32) float32 {
	return bilinear(wx-0.5, wz-0.5, m.W, m.H, func(x, z int) float32 {
		return float32(m.Px[z*m.W+x]) / mogulFull
	})
}

// mogulFallLineAt samples Dir as the GPU does, texel centres at the cell
// centres, and normalises it.
func mogulFallLineAt(m *MogulMap, wx, wz float32) (float32, float32) {
	gx, gz := wx/MogulPxPerCell-0.5, wz/MogulPxPerCell-0.5
	dx := bilinear(gx, gz, m.wCells, m.hCells, func(x, z int) float32 { return float32(m.Dir[2*(z*m.wCells+x)]) / 255 })*2 - 1
	dz := bilinear(gx, gz, m.wCells, m.hCells, func(x, z int) float32 { return float32(m.Dir[2*(z*m.wCells+x)+1]) / 255 })*2 - 1
	l := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	if l <= 1e-3 {
		return 0, 1
	}
	return dx / l, dz / l
}

// bilinear blends texel(x, z) around (gx, gz), clamped to the edge.
func bilinear(gx, gz float32, w, h int, texel func(x, z int) float32) float32 {
	x0, z0 := int(math.Floor(float64(gx))), int(math.Floor(float64(gz)))
	fx, fz := gx-float32(x0), gz-float32(z0)
	at := func(x, z int) float32 { return texel(min(max(x, 0), w-1), min(max(z, 0), h-1)) }
	a := at(x0, z0) + (at(x0+1, z0)-at(x0, z0))*fx
	b := at(x0, z0+1) + (at(x0+1, z0+1)-at(x0, z0+1))*fx
	return a + (b-a)*fz
}

func mogulHash(px, pz int32, s uint32) float32 {
	h := uint32(px)*0x8da6b343 ^ uint32(pz)*0xd8163841 ^ s*0xcb1ab31f
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12
	h *= 0x297a2d39
	h ^= h >> 15
	return float32(h>>8) / 16777216
}

func mogulNoise(x, z float32, s uint32) float32 {
	ix, iz := int32(math.Floor(float64(x))), int32(math.Floor(float64(z)))
	fx, fz := x-float32(ix), z-float32(iz)
	ux, uz := fx*fx*(3-2*fx), fz*fz*(3-2*fz)
	a := mogulHash(ix, iz, s) + (mogulHash(ix+1, iz, s)-mogulHash(ix, iz, s))*ux
	b := mogulHash(ix, iz+1, s) + (mogulHash(ix+1, iz+1, s)-mogulHash(ix, iz+1, s))*ux
	return a + (b-a)*uz
}

func smoothstep01(e0, e1, x float32) float32 {
	t := min(max((x-e0)/(e1-e0), 0), 1)
	return t * t * (3 - 2*t)
}

// mogulShape is the mogul surface at world (x, z), about -1..1, for fall
// line (dx, dz).
func mogulShape(x, z, dx, dz float32) float32 {
	qx, qz := x/mogulWarpL, z/mogulWarpL
	x += (mogulNoise(qx, qz, 3)*2 - 1) * mogulWarp
	z += (mogulNoise(qx, qz, 4)*2 - 1) * mogulWarp
	qx, qz = x/(mogulWarpL/3), z/(mogulWarpL/3)
	x += (mogulNoise(qx, qz, 6)*2 - 1) * (mogulWarp * 0.6)
	z += (mogulNoise(qx, qz, 7)*2 - 1) * (mogulWarp * 0.6)
	gx, gz := x/mogulTile-0.5, z/mogulTile-0.5
	ix, iz := int32(math.Floor(float64(gx))), int32(math.Floor(float64(gz)))
	wx := smoothstep01(0.3, 0.7, gx-float32(ix))
	wz := smoothstep01(0.3, 0.7, gz-float32(iz))
	var sum, w2 float32
	for k := 0; k < 4; k++ {
		ox, oz := int32(k&1), int32(k>>1)
		wk := wx
		if ox == 0 {
			wk = 1 - wx
		}
		if oz == 1 {
			wk *= wz
		} else {
			wk *= 1 - wz
		}
		if wk <= 0 {
			continue
		}
		tx, tz := ix+ox, iz+oz
		rx, rz := x-(float32(tx)+0.5)*mogulTile, z-(float32(tz)+0.5)*mogulTile
		u := (rx*dz-rz*dx)/mogulSU + mogulHash(tx, tz, 1)
		v := (rx*dx+rz*dz)/mogulSV + mogulHash(tx, tz, 2)
		sum += wk * float32(math.Cos(2*math.Pi*float64(u))*math.Cos(2*math.Pi*float64(v)))
		w2 += wk * wk
	}
	// Mounds pushed up and troughs carved down as far, each fattened so
	// neighbours meet over saddles, and each its own size.
	h := min(max(sum/float32(math.Sqrt(float64(max(w2, 1e-6)))), -1), 1)
	return h * (1.5 - 0.5*h*h) * (0.65 + 0.35*mogulNoise(x/4, z/4, 5))
}
