package geo

import (
	"fmt"

	"mountain-mogul/internal/world"
)

// BuildWorld makes a bare world (no snow, trees, or buildings) from an
// import. The lowest imported point becomes height 0 and its altitude the
// world's BaseAltitude. The surveyed ground is kept as the world's
// TerrainBase, and the terrain is that base with every layer applied
// except those listed in off, at the strengths given (by ID; missing is
// the default). The returned stack holds each layer's output, for
// switching layers afterwards.
func BuildWorld(res *ImportResult, off []string, strengths map[string]float32, progress func(layer string)) (*world.World, *LayerStack, error) {
	rows := len(res.Cells)
	if rows == 0 || len(res.Cells[0]) == 0 {
		return nil, nil, fmt.Errorf("import has no cells")
	}
	cols := len(res.Cells[0])
	minElev := res.Cells[0][0]
	for _, row := range res.Cells {
		for _, v := range row {
			minElev = min(minElev, v)
		}
	}

	b := res.Bounds
	base := &world.TerrainBase{
		Geo:           world.GeoBounds{MinLat: b.MinLat, MaxLat: b.MaxLat, MinLon: b.MinLon, MaxLon: b.MaxLon},
		Roads:         baseRoads(res.Roads),
		Lifts:         baseLifts(res.Lifts),
		Runs:          baseRuns(res.Runs),
		Areas:         baseAreas(res.Areas),
		Streams:       baseStreams(res.Streams),
		Lakes:         baseLakes(res.Lakes),
		RoadNote:      res.RoadNote,
		LidarCoverage: res.LidarCoverage,
		LidarNote:     res.LidarNote,
		LayersOff:     off,
		Strengths:     strengths,
	}
	if res.Detail != nil {
		base.W, base.H, base.Detail = res.DetailW, res.DetailH, true
		base.Heights = make([]float32, len(res.Detail))
		for k, v := range res.Detail {
			base.Heights[k] = v - minElev
		}
	} else {
		base.W, base.H = cols, rows
		base.Heights = make([]float32, 0, cols*rows)
		for _, row := range res.Cells {
			for _, v := range row {
				base.Heights = append(base.Heights, v-minElev)
			}
		}
	}

	t := world.NewTerrain(cols, rows)
	for x := range t.Cells {
		for z := range t.Cells[x] {
			t.Cells[x][z].Base = 0
			t.Cells[x][z].Top = world.SnowLayer{}
		}
	}
	stack := NewLayerStack(base)
	if err := ApplyHeights(t, base, stack.Run(Settings(base), progress)); err != nil {
		return nil, nil, err
	}
	w := world.NewWorld(t)
	g := base.Geo
	w.Geo = &g
	w.BaseAltitude = minElev
	w.TimeZone = res.TimeZone
	w.Climate = res.Climate
	w.TerrainBase = base
	return w, stack, nil
}
