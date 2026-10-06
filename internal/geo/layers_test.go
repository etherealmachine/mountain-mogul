package geo

import (
	"slices"
	"testing"

	"mountain-mogul/internal/world"
)

func testBase() *world.TerrainBase {
	const cells = 41
	n := (cells-1)*world.DetailPerCell + 1
	return &world.TerrainBase{
		Geo:     world.GeoBounds{MinLat: 39.3, MaxLat: 39.302, MinLon: -120.35, MaxLon: -120.347},
		W:       n,
		H:       n,
		Detail:  true,
		Heights: testHills(n, n),
	}
}

func TestLayerStackReusesAndRepeats(t *testing.T) {
	base := testBase()
	s := NewLayerStack(base)
	var ran []string
	record := func(name string) { ran = append(ran, name) }

	all := slices.Clone(s.Run(nil, record))
	if !slices.Equal(ran, []string{"Smooth ground", "Erode"}) {
		t.Fatalf("first run ran %v; want smooth and erode (no roads here)", ran)
	}
	if slices.Equal(all, base.Heights) {
		t.Fatal("layers changed nothing")
	}

	ran = nil
	noErode := slices.Clone(s.Run([]string{"erode"}, record))
	if len(ran) != 0 {
		t.Errorf("switching off the last layer reran %v", ran)
	}
	ran = nil
	again := s.Run(nil, record)
	if !slices.Equal(ran, []string{"Erode"}) {
		t.Errorf("switching erode back on ran %v; want only Erode", ran)
	}
	if !slices.Equal(again, all) {
		t.Error("the same switches gave different ground")
	}

	ran = nil
	none := s.Run([]string{"roads", "smooth", "erode"}, record)
	if len(ran) != 0 || !slices.Equal(none, base.Heights) {
		t.Error("all layers off should give the base unchanged")
	}
	if slices.Equal(noErode, all) {
		t.Error("erode made no difference")
	}
}

func TestApplyHeightsMatchesLattice(t *testing.T) {
	base := testBase()
	terrain := world.NewTerrain(41, 41)
	if err := ApplyHeights(terrain, base, base.Heights); err != nil {
		t.Fatal(err)
	}
	const per = world.DetailPerCell
	for _, p := range [][2]int{{0, 0}, {37, 51}, {base.W - 1, base.H - 1}, {80, 3}} {
		i, j := p[0], p[1]
		got := terrain.MeshGroundAt(float32(i)/per, float32(j)/per) + terrain.Detail.Off[j*base.W+i]
		if want := base.Heights[j*base.W+i]; got-want > 0.01 || want-got > 0.01 {
			t.Errorf("sample (%d, %d): drawn %.3f, want %.3f", i, j, got, want)
		}
	}
}
