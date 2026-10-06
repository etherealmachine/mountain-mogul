package scene

import (
	"math"
	"runtime"
	"sync"

	"mountain-mogul/internal/geo"
	"mountain-mogul/internal/world"
)

// Auto material: rock or not, from the ground's shape on the 1.25 m
// detail lattice, and water inside the lakes the Lakes layer levelled. Rock is generous: everything steeper than matRockAt,
// plus the sharp convex lips above faces. The threshold is jittered by
// noise so edges come out ragged rather than contoured. Everything else
// is meadow.
const (
	matRockAt      = 40.0 // degrees
	matLipBelow    = 12.0 // a lip is rock this many degrees under matRockAt...
	matLipCurve    = 0.3  // ...when its convexity (negative Laplacian, 1/m) is at least this
	matJitter      = 3.0  // ± degrees of noise on the threshold
	matJitterScale = 10.0 // metres, the jitter's feature size
)

// runMaterialLayer sets w's material map from its drawn ground. Maps
// without detail get none.
func runMaterialLayer(w *world.World, _ *layerCache) {
	t := w.Terrain
	if t.Detail == nil {
		t.Material = nil
		return
	}
	m := world.NewTerrainMaterial(t.Width, t.Height)
	W, H := m.W, m.H
	const per = world.CellSize / world.DetailPerCell
	h := make([]float32, W*H)
	eachRow(H, func(j int) {
		for i := 0; i < W; i++ {
			k := j*W + i
			h[k] = t.MeshGroundAt(float32(i)/world.DetailPerCell, float32(j)/world.DetailPerCell) + t.Detail.Off[k]
		}
	})

	seed := int(layerSeed(w))
	rock := make([]float32, W*H)
	at := func(i, j int) float32 { return h[min(max(j, 0), H-1)*W+min(max(i, 0), W-1)] }
	eachRow(H, func(j int) {
		for i := 0; i < W; i++ {
			k := j*W + i
			gx := float64(at(i+1, j)-at(i-1, j)) / (2 * per)
			gz := float64(at(i, j+1)-at(i, j-1)) / (2 * per)
			deg := math.Atan(math.Hypot(gx, gz)) * 180 / math.Pi
			lap := float64(at(i-2, j)+at(i+2, j)+at(i, j-2)+at(i, j+2)-4*h[k]) / (4 * per * per)
			x, z := float32(i)*per, float32(j)*per
			thr := matRockAt + (float64(fbm2D(x/matJitterScale, z/matJitterScale, 3, seed+11))-0.5)*2*matJitter
			if deg > thr || (deg > thr-matLipBelow && -lap >= matLipCurve) {
				rock[k] = 1
			}
		}
	})
	// Lidar noise speckles the mask at single samples; keep what most of
	// each sample's neighbourhood agrees on.
	geo.BoxBlur(rock, W, H, 1)
	for k, v := range rock {
		if v > 0.45 {
			m.M[k] = world.MatRock
		}
	}
	t.Material = m
	setUpLakes(w, h)
}

// Lake depth, which no open data has, estimated as most lake atlases do:
// the land's slope at the shore carried on under the water, gentled
// because lake floors fill with sediment, and capped by the lake's size.
const (
	lakeShoreRing    = 3   // cells around a lake whose slope sets its shore slope
	lakeFloorGentler = 0.5 // the floor drops this fraction as steeply as the shore
	lakeDepthMin     = 4.0 // metres, the cap for the smallest pond
	lakeDepthPerDec  = 6.0 // more metres of cap per tenfold of area, hectares
)

// setUpLakes marks the lakes the Lakes layer levelled as water in the
// material map, records each one's cells in Terrain.LakeOf and their
// estimated depths in Terrain.LakeDepth, and starts World.Lakes; Auto
// snow then runs their ice to the start date. h is the drawn ground on
// the material lattice.
func setUpLakes(w *world.World, h []float32) {
	t, m := w.Terrain, w.Terrain.Material
	w.Lakes, t.LakeOf, t.LakeDepth = nil, nil, nil
	base := w.TerrainBase
	if base == nil || !base.LayerOn("lakes") || len(base.Lakes) == 0 {
		return
	}
	mask := geo.LakeMask(m.W, m.H, geo.BoundsOf(base.Geo), base.Lakes)
	index := map[int32]uint8{} // base lake → 1 + index in w.Lakes
	sum := map[int32]float64{}
	n := map[int32]int{}
	for k, li := range mask {
		if li > 0 {
			sum[li] += float64(h[k])
			n[li]++
		}
	}
	for li := int32(1); li <= int32(len(base.Lakes)); li++ {
		if n[li] == 0 || len(w.Lakes) == 255 {
			continue
		}
		bl := base.Lakes[li-1]
		w.Lakes = append(w.Lakes, world.Lake{
			Name:     bl.Name,
			Altitude: w.BaseAltitude + float32(sum[li]/float64(n[li])),
			AreaHa:   float32(geo.LakeAreaHa(bl)),
		})
		index[li] = uint8(len(w.Lakes))
	}
	W, H := t.Width, t.Height
	t.LakeOf = make([]uint8, W*H)
	for k, li := range mask {
		id, ok := index[li]
		if !ok {
			continue
		}
		m.M[k] = world.MatIce
		x, z := min(k%m.W/world.DetailPerCell, W-1), min(k/m.W/world.DetailPerCell, H-1)
		t.LakeOf[x*H+z] = id
	}

	// Distance from shore, in cells, by a two-pass chamfer from the land
	// cells on the map; the map's edge isn't a shore.
	const inf = float32(1e9)
	dist := make([]float32, W*H)
	for k, id := range t.LakeOf {
		if id != 0 {
			dist[k] = inf
		}
	}
	relax := func(x, z, dx, dz int, cost float32) {
		nx, nz := x+dx, z+dz
		if nx < 0 || nz < 0 || nx >= W || nz >= H {
			return
		}
		if d := dist[nx*H+nz] + cost; d < dist[x*H+z] {
			dist[x*H+z] = d
		}
	}
	for x := 0; x < W; x++ {
		for z := 0; z < H; z++ {
			relax(x, z, -1, 0, 1)
			relax(x, z, 0, -1, 1)
			relax(x, z, -1, -1, math.Sqrt2)
			relax(x, z, 1, -1, math.Sqrt2)
		}
	}
	for x := W - 1; x >= 0; x-- {
		for z := H - 1; z >= 0; z-- {
			relax(x, z, 1, 0, 1)
			relax(x, z, 0, 1, 1)
			relax(x, z, 1, 1, math.Sqrt2)
			relax(x, z, -1, 1, math.Sqrt2)
		}
	}
	// Each lake's shore slope: the mean slope of the land cells within
	// lakeShoreRing cells of it.
	slopeSum := make([]float64, len(w.Lakes))
	slopeN := make([]int, len(w.Lakes))
	for x := 0; x < W; x++ {
		for z := 0; z < H; z++ {
			if t.LakeOf[x*H+z] != 0 {
				continue
			}
		near:
			for dx := -lakeShoreRing; dx <= lakeShoreRing; dx++ {
				for dz := -lakeShoreRing; dz <= lakeShoreRing; dz++ {
					nx, nz := x+dx, z+dz
					if nx < 0 || nz < 0 || nx >= W || nz >= H {
						continue
					}
					if id := t.LakeOf[nx*H+nz]; id != 0 {
						slopeSum[id-1] += float64(t.Cells[x][z].Slope)
						slopeN[id-1]++
						break near
					}
				}
			}
		}
	}
	t.LakeDepth = make([]float32, W*H)
	for k, id := range t.LakeOf {
		if id == 0 {
			continue
		}
		l := &w.Lakes[id-1]
		slope := float32(0.1)
		if slopeN[id-1] > 0 {
			slope = float32(slopeSum[id-1] / float64(slopeN[id-1]))
		}
		capDepth := lakeDepthMin + lakeDepthPerDec*float32(math.Log10(1+float64(l.AreaHa)))
		fromShore := max(dist[k]-0.5, 0.25) * world.CellSize
		d := min(lakeFloorGentler*slope*fromShore, capDepth)
		t.LakeDepth[k] = d
		l.MaxDepth = max(l.MaxDepth, d)
	}
}

// clearMaterial drops w's material map, and with it the lakes.
func clearMaterial(w *world.World) {
	w.Terrain.Material = nil
	w.Terrain.LakeOf, w.Terrain.LakeDepth = nil, nil
	w.Lakes = nil
}

// removeTreesOnBare removes the trees standing on rock or scree.
func removeTreesOnBare(t *world.Terrain) {
	if t.Material == nil {
		return
	}
	m := t.Material
	t.RemoveTreesIn(0, 0, t.Width-1, t.Height-1, func(tr world.Tree) bool { return m.At(tr.X, tr.Z).Bare() })
}

// eachRow runs f for rows 0..n-1 across the CPUs.
func eachRow(n int, f func(j int)) {
	var wg sync.WaitGroup
	workers := runtime.NumCPU()
	for wk := 0; wk < workers; wk++ {
		wg.Add(1)
		go func(wk int) {
			defer wg.Done()
			for j := wk; j < n; j += workers {
				f(j)
			}
		}(wk)
	}
	wg.Wait()
}
