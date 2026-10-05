package save

import (
	"path/filepath"
	"testing"

	"mountain-mogul/internal/world"
)

func TestDetailAndSiteRoundTrip(t *testing.T) {
	tr := world.NewTerrain(16, 12)
	tr.FillDetailTest()
	w := world.NewWorld(tr)
	w.Geo = &world.GeoBounds{MinLat: 38.66, MaxLat: 38.69, MinLon: -120.09, MaxLon: -120.05}
	w.BaseAltitude = 2361
	w.TimeZone = "America/Los_Angeles"
	w.Climate = &world.Climate{Source: "test", RefAltitude: 2548, WindDeg: 38}
	w.Climate.Months[0] = world.ClimateMonth{TempMean: -3.6, TempRange: 9.4, WetDays: 0.36, WetMM: 17.2, Cloud: 0.64}
	path := filepath.Join(t.TempDir(), "detail.save")
	if err := SaveScenario(path, w, nil); err != nil {
		t.Fatal(err)
	}
	got, _, err := LoadScenario(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Geo == nil || *got.Geo != *w.Geo {
		t.Fatalf("Geo = %+v, want %+v", got.Geo, w.Geo)
	}
	if got.BaseAltitude != w.BaseAltitude || got.TimeZone != w.TimeZone {
		t.Fatalf("base altitude %v, zone %q; want %v, %q", got.BaseAltitude, got.TimeZone, w.BaseAltitude, w.TimeZone)
	}
	if got.Climate == nil || *got.Climate != *w.Climate {
		t.Fatalf("Climate = %+v, want %+v", got.Climate, w.Climate)
	}
	d := got.Terrain.Detail
	if d == nil || d.W != tr.Detail.W || d.H != tr.Detail.H {
		t.Fatalf("detail didn't load: %+v", d)
	}
	for k, v := range tr.Detail.Off {
		if diff := d.Off[k] - v; diff > 0.006 || diff < -0.006 {
			t.Fatalf("offset %d = %v, want %v", k, d.Off[k], v)
		}
	}

	plain := world.NewWorld(world.NewTerrain(16, 12))
	if err := SaveScenario(path, plain, nil); err != nil {
		t.Fatal(err)
	}
	got, _, err = LoadScenario(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Geo != nil || got.Terrain.Detail != nil || got.Climate != nil || got.TimeZone != "" {
		t.Fatalf("plain world loaded with Geo %+v, detail %v", got.Geo, got.Terrain.Detail != nil)
	}
}
