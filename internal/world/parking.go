package world

import "math"

// Parking-lot stall layout. The pad mesh is just a flat slab; this
// file decides how many cars fit on it and where each one sits. Both
// the renderer (carInstancesFor) and PlaceBuildingType call into here
// so MaxCars never drifts from what the screen actually shows.

const (
	// CarLength and CarWidth mirror models-src/car.scad — keep in sync
	// when the car mesh changes size. body_len = 4.0, body_w = 1.7.
	CarLength = float32(4.00)
	CarWidth  = float32(1.70)

	// Stall pitch = car footprint + clearance. Width gets a door-gap
	// margin between neighbours; length gets bumper clearance on each
	// end. Lands on the real-world 2.5 × 5.0 m metric stall.
	parkingStallWidth  = CarWidth + 0.80  // 2.50 m
	parkingStallLength = CarLength + 1.00 // 5.00 m

	// Perimeter inset from the pad edge to the first row of stalls.
	// Reads as "the pad has a clear edge" rather than cars right at
	// the boundary.
	parkingPerimeterMargin = float32(3.0)

	// Drive aisle running through the middle of the lot along the
	// stall-width direction (pad +X). Splits the rows into a front
	// bank and a back bank.
	parkingCenterAisle = float32(6.0)
)

// ParkingLayout describes the stall grid for a parking lot: how many
// columns across, how many rows in each bank, and the local-frame
// origin of the first stall in each bank. Local coords are in the
// building's untranslated, unrotated frame (centred on Pos).
type ParkingLayout struct {
	Cols        int
	FrontRows   int
	BackRows    int
	startX      float32
	frontStartZ float32
	backStartZ  float32
}

// ParkingLotLayout computes the stall grid for a parking-lot mesh
// from its registered footprint. Returns ok=false if the mesh has no
// footprint metadata yet (asset still building) or the pad is too
// small to fit even one stall.
func ParkingLotLayout(typ BuildingType) (ParkingLayout, bool) {
	fp, ok := FootprintFor(typ.MeshID())
	if !ok {
		return ParkingLayout{}, false
	}
	usableX := fp.HalfX*2 - 2*parkingPerimeterMargin
	usableZ := fp.HalfZ*2 - 2*parkingPerimeterMargin - parkingCenterAisle
	cols := int(math.Floor(float64(usableX / parkingStallWidth)))
	totalRows := int(math.Floor(float64(usableZ / parkingStallLength)))
	if cols < 1 || totalRows < 1 {
		return ParkingLayout{}, false
	}
	// Split rows evenly across the aisle; on an odd row count the
	// back bank gets the extra so the front edge of the lot (closer
	// to the road) reads as the smaller bank.
	front := totalRows / 2
	back := totalRows - front
	startX := -fp.HalfX + parkingPerimeterMargin + parkingStallWidth/2
	frontStartZ := -fp.HalfZ + parkingPerimeterMargin + parkingStallLength/2
	backStartZ := -fp.HalfZ + parkingPerimeterMargin +
		float32(front)*parkingStallLength + parkingCenterAisle + parkingStallLength/2
	return ParkingLayout{
		Cols:        cols,
		FrontRows:   front,
		BackRows:    back,
		startX:      startX,
		frontStartZ: frontStartZ,
		backStartZ:  backStartZ,
	}, true
}

// Capacity is the total stall count.
func (l ParkingLayout) Capacity() int {
	return l.Cols * (l.FrontRows + l.BackRows)
}

// StallPosition returns the local-frame (X, Z) centre of stall index
// i, filling row-major within the front bank first, then the back
// bank. Caller is responsible for clamping i < Capacity().
func (l ParkingLayout) StallPosition(i int) (float32, float32) {
	frontStalls := l.Cols * l.FrontRows
	if i < frontStalls {
		col := i % l.Cols
		row := i / l.Cols
		return l.startX + float32(col)*parkingStallWidth,
			l.frontStartZ + float32(row)*parkingStallLength
	}
	i -= frontStalls
	col := i % l.Cols
	row := i / l.Cols
	return l.startX + float32(col)*parkingStallWidth,
		l.backStartZ + float32(row)*parkingStallLength
}
