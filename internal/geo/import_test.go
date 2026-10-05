package geo

import (
	"math"
	"testing"
)

func TestCellsFromLatticeAveragesFootprint(t *testing.T) {
	const per, cols, rows = 4, 5, 4
	w, h := (cols-1)*per+1, (rows-1)*per+1
	lat := make([]float32, w*h)
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			lat[j*w+i] = float32(2*i + 3*j)
		}
	}
	cells := cellsFromLattice(lat, w, h, cols, rows, per)
	// A plane averages to its value at the centre, away from the edges.
	if got, want := cells[1][2], float32(2*2*per+3*1*per); got != want {
		t.Fatalf("cell (2, 1) = %v, want %v", got, want)
	}
	lat[1*per*w+2*per] += 20
	if got := cellsFromLattice(lat, w, h, cols, rows, per)[1][2]; math.Abs(float64(got-cells[1][2])-20.0/16) > 1e-4 {
		t.Fatalf("a spike moved the cell by %v, want 1.25", got-cells[1][2])
	}
}

func TestBlendIntoTilesFillsAndFeathers(t *testing.T) {
	const w, h = 80, 10
	nan := float32(math.NaN())
	v := make([]float32, w*h)
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			v[j*w+i] = 10
			if i < 20 {
				v[j*w+i] = nan
			}
		}
	}
	blendIntoTiles(v, w, h, func(i, j int) float32 { return 0 })
	for j := 0; j < h; j++ {
		row := v[j*w : (j+1)*w]
		if row[5] != 0 {
			t.Fatalf("gap not filled from tiles: %v", row[5])
		}
		if row[20] > 1 {
			t.Fatalf("lidar edge not feathered: %v", row[20])
		}
		if row[79] != 10 {
			t.Fatalf("lidar far from the edge changed: %v", row[79])
		}
		for i := 1; i < w; i++ {
			if row[i] < row[i-1] {
				t.Fatalf("blend not monotonic at %d: %v < %v", i, row[i], row[i-1])
			}
		}
	}
}
