package render

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/world"
)

// Painted parking lots are drawn as one asphalt quad per lot cell plus
// thin stripes between stalls. Heights follow the terrain mesh's own
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
// every painted lot. Either return is nil when there is nothing to draw.
func generateParkingMeshes(w *world.World) (asphalt, stripes *Mesh) {
	t := w.Terrain
	const cs = float32(world.CellSize)
	var av []float32
	var ai []uint32
	var sv []float32
	var si []uint32
	for _, b := range w.Buildings {
		if !b.IsCellLot() {
			continue
		}
		for _, c := range b.Cells {
			x0, z0 := float32(c[0])*cs, float32(c[1])*cs
			y00 := terrainCornerY(t, c[0], c[1]) + parkingHoverOffset
			y10 := terrainCornerY(t, c[0]+1, c[1]) + parkingHoverOffset
			y01 := terrainCornerY(t, c[0], c[1]+1) + parkingHoverOffset
			y11 := terrainCornerY(t, c[0]+1, c[1]+1) + parkingHoverOffset
			base := uint32(len(av) / 8)
			av = append(av,
				x0, y00, z0, 0, 1, 0, 0, 0,
				x0+cs, y10, z0, 0, 1, 0, 1, 0,
				x0, y01, z0+cs, 0, 1, 0, 0, 1,
				x0+cs, y11, z0+cs, 0, 1, 0, 1, 1,
			)
			ai = append(ai, base, base+2, base+1, base+1, base+2, base+3)
		}
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
