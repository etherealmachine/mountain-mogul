package scene

import (
	"math"
	"testing"
	"time"

	"mountain-mogul/internal/world"
)

// layerTestWorld is a 64-cell valley rising 300 m to the north and south
// from 2000 m, with a snowy climate.
func layerTestWorld() *world.World {
	const n = 64
	ter := world.NewTerrain(n, n)
	for x := range ter.Cells {
		for z := range ter.Cells[x] {
			d := math.Abs(float64(z-n/2)) / (n / 2)
			ter.Cells[x][z].GroundElevation = float32(300 * d * d)
			ter.Cells[x][z].Base, ter.Cells[x][z].Top = 0, world.SnowLayer{}
		}
	}
	w := world.NewWorld(ter)
	w.Geo = &world.GeoBounds{MinLat: 39.3, MaxLat: 39.31, MinLon: -120.35, MaxLon: -120.34}
	w.BaseAltitude = 2000
	w.Climate = &world.Climate{RefAltitude: 2000, WindDeg: 90}
	temps := [12]float32{-5, -4, -2, 2, 6, 11, 15, 14, 10, 5, 0, -4}
	for m := range w.Climate.Months {
		w.Climate.Months[m] = world.ClimateMonth{TempMean: temps[m], TempRange: 10, WetDays: 0.35, WetMM: 20, Cloud: 0.5}
	}
	w.TerrainBase = &world.TerrainBase{}
	return w
}

func snowSWE(t *world.Terrain) (total float32, powder bool) {
	for x := range t.Cells {
		for z := range t.Cells[x] {
			c := t.Cells[x][z]
			total += c.Base + c.Top.Accumulation
			powder = powder || c.Top.Kind == world.KindPowder && c.Top.Accumulation > 0
		}
	}
	return total, powder
}

func TestDressWorldOpensAndCovers(t *testing.T) {
	w := layerTestWorld()
	took := DressWorld(w, nil)
	if _, ok := took["trees"]; !ok {
		t.Errorf("trees didn't run: %v", took)
	}
	if m := w.StartDate.Month(); m < time.November && m > time.February {
		t.Errorf("opening day %v", w.StartDate)
	}
	if w.Terrain.TotalTrees() == 0 {
		t.Error("no trees with the treeline above the map")
	}
	early, powder := snowSWE(w.Terrain)
	if early <= 0 || !powder {
		t.Fatalf("snow on the opening day %.2f, fresh powder %v", early, powder)
	}

	// Later in the season there's more; switched off there's none.
	w.StartDate = time.Date(seasonYear(w.StartDate)+1, time.March, 1, 0, 0, 0, 0, time.UTC)
	c := &layerCache{}
	runSnowLayer(w, c)
	if late, _ := snowSWE(w.Terrain); late <= early {
		t.Errorf("snow on 1 March %.2f, not more than the opening day's %.2f", late, early)
	}
	clearSnow(w)
	if v, _ := snowSWE(w.Terrain); v != 0 {
		t.Errorf("snow after clearing: %.2f", v)
	}
}

func TestTreelineFromClimate(t *testing.T) {
	w := layerTestWorld()
	f := computeElevFields(w.Terrain)
	// Warmest month 15 °C at 2000 m: 10 °C is 770 m up, over the 300 m map.
	if got := treelineFrac(w, f); got < 2.5 || got > 2.6 {
		t.Errorf("treeline fraction %.2f, want about 2.56", got)
	}
	for m := range w.Climate.Months {
		w.Climate.Months[m].TempMean -= 4
	}
	// Now 11 °C: 154 m up, about half way.
	if got := treelineFrac(w, f); got < 0.45 || got > 0.6 {
		t.Errorf("treeline fraction %.2f, want about 0.51", got)
	}
}
