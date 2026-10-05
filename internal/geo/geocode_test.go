package geo

import "testing"

func TestIsSkiArea(t *testing.T) {
	for _, tc := range []struct {
		class, typ, name string
		want             bool
	}{
		{"landuse", "winter_sports", "Kirkwood Mountain Resort, Alpine County, California", true},
		{"leisure", "sports_centre", "Boreal Mountain Resort, Alan S. Hart Freeway, Soda Springs", true},
		{"leisure", "sports_centre", "Soda Springs Community Pool, Soda Springs", false},
		{"tourism", "hotel", "Four Seasons Resort Whistler, Blackcomb Way", false},
		{"landuse", "residential", "Twin Peaks Resort, Blackcomb Base, Whistler Village", false},
		{"leisure", "resort", "Tantalus Resort Lodge, 4200, Whistler Way", false},
		{"leisure", "resort", "Snowbird Ski Resort, Little Cottonwood Canyon", true},
		{"place", "suburb", "Boreal, Palazu Mare, Constanța", false},
	} {
		if got := isSkiArea(tc.class, tc.typ, tc.name); got != tc.want {
			t.Errorf("isSkiArea(%s, %s, %q) = %v, want %v", tc.class, tc.typ, tc.name, got, tc.want)
		}
	}
}
