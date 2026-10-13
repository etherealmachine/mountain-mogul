//go:build !demo

package scene

import (
	"fmt"

	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// Buying land parcels. Not in the demo, where openLandBuyPopup stays nil
// and the Land tool isn't on the tool bar.

func init() { openLandBuyPopup = landBuyPopup }

// landBuyPopup shows a purchase confirmation popup for the parcel at
// terrain cell (gx, gz). Does nothing if the cell is not in a purchasable
// parcel.
func landBuyPopup(s *Scenario, gx, gz, screenW, screenH int) {
	p := s.world.ParcelAt(gx, gz)
	if p == nil || p.State != world.ParcelPurchasable {
		if p != nil && p.State == world.ParcelOffLimits {
			s.setToast("This land is off-limits and cannot be purchased")
		}
		return
	}
	parcel := p
	w := ui.NewWindow(parcel.Name, 0, 0)
	w.AddLabel("Price", func() string { return fmt.Sprintf("$%d", parcel.Price) })
	w.AddLabel("Status", func() string { return "Available for purchase" })
	w.AddActionButton(fmt.Sprintf("Buy for $%d", parcel.Price), func() {
		if !s.world.BuyParcel(parcel.ID) {
			s.setToast(fmt.Sprintf("Need $%d — short by $%d", parcel.Price, parcel.Price-s.world.Available()))
		} else {
			s.parcelBoundaryDirty = true
		}
		w.Visible = false
		s.popup = nil
	})
	w.AddActionButton("Cancel", func() {
		w.Visible = false
		s.popup = nil
	})
	w.Visible = true
	w.Center(screenW, screenH)
	s.popup = w
}
