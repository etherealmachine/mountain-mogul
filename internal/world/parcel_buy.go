//go:build !demo

package world

// Buying land. Not in the demo, where purchasable parcels stay unbought.

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
		if !w.CanAfford(p.Price) {
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
