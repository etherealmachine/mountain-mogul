package world

// ParcelState describes the player's ownership relationship to a land parcel.
type ParcelState uint8

const (
	ParcelOwned       ParcelState = iota // player owns this land
	ParcelPurchasable ParcelState = iota // player can buy it
	ParcelOffLimits   ParcelState = iota // permanently inaccessible
)

// Parcel is a named region of the map with a single ownership state. Parcels
// are scenario-authored: their cell lists and initial states are embedded in
// scenario data, not generated at runtime.
type Parcel struct {
	ID    uint16
	Name  string
	State ParcelState
	Price int      // only meaningful when State == ParcelPurchasable
	Cells [][2]int // grid cells belonging to this parcel
}

// ApplyParcels derives Terrain.accessible from w.Parcels. Call after loading
// parcels from a save or initialising a scenario. When Parcels is empty,
// accessible is cleared (all in-bounds cells are accessible).
func (w *World) ApplyParcels() {
	t := w.Terrain
	if len(w.Parcels) == 0 {
		t.accessible = nil
		return
	}
	// Allocate grid defaulting to fully accessible.
	t.accessible = make([][]bool, t.Width)
	for x := range t.accessible {
		t.accessible[x] = make([]bool, t.Height)
		for z := range t.accessible[x] {
			t.accessible[x][z] = true
		}
	}
	// Block cells belonging to non-owned parcels.
	for _, p := range w.Parcels {
		if p.State == ParcelOwned {
			continue
		}
		for _, c := range p.Cells {
			if t.InBounds(c[0], c[1]) {
				t.accessible[c[0]][c[1]] = false
			}
		}
	}
}

// BuyParcel purchases the parcel with the given ID, deducting its price from
// World.Cash. Returns false when the parcel is not found, is not purchasable,
// or the player cannot afford it.
func (w *World) BuyParcel(id uint16) bool {
	for i := range w.Parcels {
		p := &w.Parcels[i]
		if p.ID != id {
			continue
		}
		if p.State != ParcelPurchasable {
			return false
		}
		if w.Cash < p.Price {
			return false
		}
		w.Cash -= p.Price
		p.State = ParcelOwned
		// Update the accessibility grid for the purchased cells.
		if w.Terrain.accessible != nil {
			for _, c := range p.Cells {
				if w.Terrain.InBounds(c[0], c[1]) {
					w.Terrain.accessible[c[0]][c[1]] = true
				}
			}
		}
		return true
	}
	return false
}

// ParcelAt returns the parcel that contains terrain cell (x, z), or nil if
// no parcel covers that cell.
func (w *World) ParcelAt(x, z int) *Parcel {
	for i := range w.Parcels {
		for _, c := range w.Parcels[i].Cells {
			if c[0] == x && c[1] == z {
				return &w.Parcels[i]
			}
		}
	}
	return nil
}
