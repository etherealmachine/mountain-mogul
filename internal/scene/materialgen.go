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
func runMaterialLayer(w *world.World, c *layerCache) {
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
	markCreeks(w, c)
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

// Creek water: where a flowing creek's channel is open water and where
// snow bridges it. Bigger creeks are open more of their length; along
// one creek, open stretches and bridges alternate every few tens of
// metres.
const (
	creekOpenFrom = 0.3e6 // m² of catchment: mostly bridged below this...
	creekOpenTo   = 3e6   // ...mostly open above
	creekBridgeM  = 25.0  // metres, the length of a typical bridge or opening
)

// markCreeks marks the channels of w's flowing creeks in the material
// map: open water where the creek runs open, ice (snow-covered, like a
// frozen lake) where snow bridges it. Either way trees stay off it.
func markCreeks(w *world.World, c *layerCache) {
	m := w.Terrain.Material
	if m == nil || c == nil || w.TerrainBase == nil || !w.TerrainBase.LayerOn("creeks") {
		return
	}
	const per = world.CellSize / world.DetailPerCell
	seed := int(layerSeed(w))
	for si, s := range c.streamsFor(w) {
		var along float32
		for i := 1; i < len(s.Points); i++ {
			a, b := s.Points[i-1], s.Points[i]
			l := float32(math.Hypot(float64(b[0]-a[0]), float64(b[1]-a[1])))
			along += l
			if !s.Flowing(i) || l == 0 {
				continue
			}
			open := smoothstep32(creekOpenFrom, creekOpenTo, s.Catchment[i])
			gap := valueNoise2D(along/creekBridgeM, float32(si)*7.3, seed+47)
			surface := world.MatIce
			if gap < open*0.9+0.05 {
				surface = world.MatOpenWater
			}
			half, _ := geo.CreekShape(s.Catchment[i])
			half *= 0.8 // the water, inside the banks
			i0, i1 := max(int((min(a[0], b[0])-half)/per), 0), min(int((max(a[0], b[0])+half)/per)+1, m.W-1)
			j0, j1 := max(int((min(a[1], b[1])-half)/per), 0), min(int((max(a[1], b[1])+half)/per)+1, m.H-1)
			dx, dz := b[0]-a[0], b[1]-a[1]
			for j := j0; j <= j1; j++ {
				for i := i0; i <= i1; i++ {
					px, pz := float32(i)*per, float32(j)*per
					u := min(max(((px-a[0])*dx+(pz-a[1])*dz)/(l*l), 0), 1)
					if math.Hypot(float64(px-(a[0]+u*dx)), float64(pz-(a[1]+u*dz))) > float64(half) {
						continue
					}
					k := j*m.W + i
					if m.M[k] == world.MatMeadow || m.M[k] == world.MatDirt || (surface == world.MatOpenWater && m.M[k] == world.MatIce) {
						m.M[k] = surface
					}
				}
			}
		}
	}
}

// clearMaterial drops w's material map, and with it the lakes.
func clearMaterial(w *world.World) {
	w.Terrain.Material = nil
	w.Terrain.LakeOf, w.Terrain.LakeDepth = nil, nil
	w.Lakes = nil
}

// removeTreesOnMeadows clears the trees off wet valley floors
// (geo.MeadowWetness): grass and willow grow where the water table is
// high. Across the margin trees thin out by chance rather than stopping
// at a line, with broad and fine noise breaking up the edge, so the
// forest blends into the meadow and leaves the odd clump and lone tree.
func removeTreesOnMeadows(w *world.World, c *layerCache) {
	t := w.Terrain
	wet := geo.MeadowWetness(groundRows(t), t.Width, t.Height, world.CellSize, c.catchmentFor(t))
	seed := int(layerSeed(w))
	t.RemoveTreesIn(0, 0, t.Width-1, t.Height-1, func(tr world.Tree) bool {
		x, z := min(int(tr.X/world.CellSize), t.Width-1), min(int(tr.Z/world.CellSize), t.Height-1)
		m := wet[z*t.Width+x]
		if m <= 0 {
			return false
		}
		broad := fbm2D(tr.X/60, tr.Z/60, 3, seed+37) - 0.5
		fine := fbm2D(tr.X/12, tr.Z/12, 2, seed+41) - 0.5
		clear := smoothstep32(0.1, 0.85, m+0.5*broad+0.25*fine)
		return hash22(int(tr.X*10), int(tr.Z*10), seed+43) < clear
	})
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
