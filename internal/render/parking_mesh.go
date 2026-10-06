package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Parking lots are drawn as an asphalt rectangle with rounded corners
// plus thin stripes between stalls. Heights follow the terrain mesh's own
// corner rule (4-cell average, see buildTerrainVerts) so the asphalt
// sits exactly on the graded pad rather than on the half-cell-shifted
// InterpolatedSurfaceElevationAt.
const (
	parkingHoverOffset     = float32(0.12) // below roadHoverOffset so driveway roads draw over the lot
	parkingStripeHover     = float32(0.03) // above the asphalt
	parkingStripeHalfWidth = float32(0.06)
)

// terrainCornerY is the rendered height of terrain grid corner (cx, cz):
// the average ground + visible snow of the up-to-four cells around it.
func terrainCornerY(t *world.Terrain, cx, cz int) float32 {
	var sum, n float32
	for dx := -1; dx <= 0; dx++ {
		for dz := -1; dz <= 0; dz++ {
			x, z := cx+dx, cz+dz
			if !t.InBounds(x, z) {
				continue
			}
			c := &t.Cells[x][z]
			sum += c.GroundElevation + c.VisibleSnowDepth()
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

// parkingSurfaceY bilinearly interpolates terrainCornerY at world XZ.
func parkingSurfaceY(t *world.Terrain, x, z float32) float32 {
	const cs = float32(world.CellSize)
	fx, fz := x/cs, z/cs
	cx, cz := int(math.Floor(float64(fx))), int(math.Floor(float64(fz)))
	ux, uz := fx-float32(cx), fz-float32(cz)
	y00 := terrainCornerY(t, cx, cz)
	y10 := terrainCornerY(t, cx+1, cz)
	y01 := terrainCornerY(t, cx, cz+1)
	y11 := terrainCornerY(t, cx+1, cz+1)
	return (1-uz)*((1-ux)*y00+ux*y10) + uz*((1-ux)*y01+ux*y11)
}

// generateParkingMeshes builds the asphalt and stall-stripe meshes for
// every lot. Either return is nil when there is nothing to draw.
func generateParkingMeshes(w *world.World) (asphalt, stripes *Mesh) {
	t := w.Terrain
	var av []float32
	var ai []uint32
	var sv []float32
	var si []uint32
	for _, b := range w.Buildings {
		if !b.IsRectLot() {
			continue
		}
		av, ai = appendLotAsphalt(av, ai, t, b.LotRect())
		sv, si = appendStallStripes(sv, si, t, b.Stalls)
	}
	if len(ai) > 0 {
		asphalt = NewMesh(av, ai, []int{3, 3, 2}, nil)
	}
	if len(si) > 0 {
		stripes = NewMesh(sv, si, []int{3, 3, 2}, nil)
	}
	return asphalt, stripes
}

// lotGridStep is the asphalt's grid spacing: fine enough to follow the
// graded pad (and any snow on it) like the terrain mesh does.
const lotGridStep = float32(2.5)

// appendLotAsphalt adds a lot rectangle with rounded corners: a grid in
// the lot's frame whose lines pass through the corner arcs' centres, the
// corner squares replaced by fans over the arcs.
func appendLotAsphalt(verts []float32, idx []uint32, t *world.Terrain, r world.FootprintRect) ([]float32, []uint32) {
	ax, az := r.Axes()
	rc := min(world.LotCornerRadius, r.HalfX, r.HalfZ)
	at := func(lx, lz float32) mgl32.Vec2 { return r.Center.Add(ax.Mul(lx)).Add(az.Mul(lz)) }
	vert := func(p mgl32.Vec2) uint32 {
		i := uint32(len(verts) / 8)
		y := parkingSurfaceY(t, p[0], p[1]) + parkingHoverOffset
		verts = append(verts, p[0], y, p[1], 0, 1, 0, 0, 0)
		return i
	}
	lines := func(half float32) []float32 {
		out := []float32{-half}
		inner := 2 * (half - rc)
		n := max(int(math.Ceil(float64(inner/lotGridStep))), 1)
		for i := 0; i <= n; i++ {
			out = append(out, -half+rc+inner*float32(i)/float32(n))
		}
		return append(out, half)
	}
	xs, zs := lines(r.HalfX), lines(r.HalfZ)
	grid := make([]uint32, len(xs)*len(zs))
	for i, x := range xs {
		for j, z := range zs {
			grid[i*len(zs)+j] = vert(at(x, z))
		}
	}
	lastX, lastZ := len(xs)-2, len(zs)-2
	for i := 0; i <= lastX; i++ {
		for j := 0; j <= lastZ; j++ {
			if (i == 0 || i == lastX) && (j == 0 || j == lastZ) {
				continue // corner square: a fan below
			}
			a, b := grid[i*len(zs)+j], grid[(i+1)*len(zs)+j]
			c, d := grid[i*len(zs)+j+1], grid[(i+1)*len(zs)+j+1]
			idx = append(idx, a, c, b, b, c, d)
		}
	}
	// Corner fans around each arc's centre.
	const arcSegs = 6
	for _, sx := range [2]float32{-1, 1} {
		for _, sz := range [2]float32{-1, 1} {
			cx, cz := sx*(r.HalfX-rc), sz*(r.HalfZ-rc)
			centre := vert(at(cx, cz))
			var prev uint32
			for k := 0; k <= arcSegs; k++ {
				ang := float64(k) / arcSegs * math.Pi / 2
				p := at(cx+sx*rc*float32(math.Cos(ang)), cz+sz*rc*float32(math.Sin(ang)))
				cur := vert(p)
				if k > 0 {
					// Keep the winding facing up whichever corner this is.
					if sx*sz > 0 {
						idx = append(idx, centre, cur, prev)
					} else {
						idx = append(idx, centre, prev, cur)
					}
				}
				prev = cur
			}
		}
	}
	return verts, idx
}

// appendStallStripes adds one painted line along each long side of every
// stall. Neighbouring stalls share a side, so lines are de-duplicated by
// their (rounded) midpoint.
func appendStallStripes(verts []float32, idx []uint32, t *world.Terrain, stalls []world.ParkingStall) ([]float32, []uint32) {
	seen := make(map[[2]int32]bool, len(stalls)*2)
	for _, s := range stalls {
		// Length axis is local +Z rotated by Heading (HomogRotate3DY:
		// (0,0,1) → (sin, 0, cos)); the width axis is perpendicular.
		sin, cos := math.Sincos(float64(s.Heading))
		along := mgl32.Vec2{float32(sin), float32(cos)}
		across := mgl32.Vec2{-along[1], along[0]} // same perpendicular as road strips, so the winding faces up
		halfL := world.ParkingStallLength / 2
		for _, side := range [2]float32{-1, 1} {
			mid := s.Pos.Add(across.Mul(side * world.ParkingStallWidth / 2))
			key := [2]int32{int32(math.Round(float64(mid[0] * 10))), int32(math.Round(float64(mid[1] * 10)))}
			if seen[key] {
				continue
			}
			seen[key] = true
			a := mid.Sub(along.Mul(halfL))
			b := mid.Add(along.Mul(halfL))
			off := across.Mul(parkingStripeHalfWidth)
			base := uint32(len(verts) / 8)
			for _, p := range [4]mgl32.Vec2{a.Sub(off), a.Add(off), b.Sub(off), b.Add(off)} {
				y := parkingSurfaceY(t, p[0], p[1]) + parkingHoverOffset + parkingStripeHover
				verts = append(verts, p[0], y, p[1], 0, 1, 0, 0, 0)
			}
			idx = append(idx, base, base+1, base+2, base+1, base+3, base+2)
		}
	}
	return verts, idx
}

// RebuildParkingLots regenerates the painted-lot meshes. Called from
// RebuildStaticBatch so any building or terrain change picks it up.
func (r *Renderer) RebuildParkingLots(w *world.World) {
	if r.scene.parkingMesh != nil {
		r.scene.parkingMesh.Delete()
		r.scene.parkingMesh = nil
	}
	if r.scene.parkingStripesMesh != nil {
		r.scene.parkingStripesMesh.Delete()
		r.scene.parkingStripesMesh = nil
	}
	r.scene.parkingMesh, r.scene.parkingStripesMesh = generateParkingMeshes(w)
}
