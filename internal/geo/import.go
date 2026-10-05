package geo

import (
	"context"
	"errors"
	"math"
	"runtime"
	"sync"

	"mountain-mogul/internal/world"
)

// Bounds is a latitude/longitude box, in degrees.
type Bounds struct {
	MinLat, MaxLat, MinLon, MaxLon float64
}

// ImportResult is the elevation for a map imported from a real place, in
// metres above sea level. Cell (col, row) and detail sample (i, j) sit
// at the same fractions across Bounds: col/(cols-1) west to east, row/
// (rows-1) north to south, and i/(DetailW-1), j/(DetailH-1) likewise.
type ImportResult struct {
	Cells  [][]float32 // [row][col]
	Bounds Bounds

	// Detail is ground height on a lattice detailPerCell times finer
	// than the cells, row-major, from lidar blended into the coarse
	// tiles where lidar is missing; nil when no lidar covers the area.
	Detail           []float32
	DetailW, DetailH int

	LidarCoverage float32  // fraction of the detail lattice from lidar
	LidarSources  []string // surveys used, newest first
	LidarNote     string   // why there's no lidar, when there isn't

	Climate     *world.Climate // nil if it couldn't be fetched
	TimeZone    string         // IANA zone; "" if unknown
	ClimateNote string         // why there's no climate, when there isn't
}

// ImportProgress reports a stage name and how far through it the import is.
type ImportProgress func(stage string, frac float32)

// lidarFeather is how far, in detail samples, lidar fades into the
// coarse tiles at the edge of its coverage.
const lidarFeather = 16

// ImportTerrain fetches elevation for b on a cols × rows cell grid. The
// coarse Terrain Tiles cover everywhere; where USGS 1 m lidar exists, it
// replaces them: the detail lattice samples it directly and each cell
// is the average of the lidar over its 5 m footprint. Lidar failures
// aren't fatal; the result falls back to the tiles and says why.
func ImportTerrain(ctx context.Context, b Bounds, cols, rows, detailPerCell int, progress ImportProgress) (*ImportResult, error) {
	report := func(stage string, f float32) {
		if progress != nil {
			progress(stage, f)
		}
	}
	report("Fetching elevation tiles", 0)
	tiles, err := FetchGrid(ctx, b.MinLat, b.MaxLat, b.MinLon, b.MaxLon, cols, rows, func(f float32) {
		report("Fetching elevation tiles", f)
	})
	if err != nil {
		return nil, err
	}
	tileAt := func(fx, fz float64) float32 {
		return bilinearSample(tiles, float32(fz)*float32(len(tiles)-1), float32(fx)*float32(len(tiles[0])-1))
	}
	res := &ImportResult{Bounds: b}

	dw, dh := (cols-1)*detailPerCell+1, (rows-1)*detailPerCell+1
	heights, err := lidarLattice(ctx, b, dw, dh, res, report)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		res.LidarNote = err.Error()
		if !errors.Is(err, ErrNoLidar) {
			res.LidarNote = "lidar download failed: " + res.LidarNote
		}
		heights = nil
	}
	if heights == nil {
		res.LidarSources = nil
		res.Cells = make([][]float32, rows)
		for r := range res.Cells {
			res.Cells[r] = make([]float32, cols)
			for c := range res.Cells[r] {
				res.Cells[r][c] = tileAt(float64(c)/float64(cols-1), float64(r)/float64(rows-1))
			}
		}
	} else {
		report("Blending lidar", 0)
		blendIntoTiles(heights, dw, dh, func(i, j int) float32 {
			return tileAt(float64(i)/float64(dw-1), float64(j)/float64(dh-1))
		})
		res.Detail, res.DetailW, res.DetailH = heights, dw, dh
		res.Cells = cellsFromLattice(heights, dw, dh, cols, rows, detailPerCell)
	}

	report("Fetching climate", 0)
	lo, hi := float32(math.Inf(1)), float32(math.Inf(-1))
	for _, row := range res.Cells {
		for _, v := range row {
			lo, hi = min(lo, v), max(hi, v)
		}
	}
	res.Climate, res.TimeZone, err = FetchClimate(ctx, (b.MinLat+b.MaxLat)/2, (b.MinLon+b.MaxLon)/2, lo, (lo+hi)/2)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		res.ClimateNote = err.Error()
	}
	return res, nil
}

// lidarLattice samples lidar on the dw × dh lattice over b (NaN where
// there's none), or returns nil when no lidar covers any of it.
func lidarLattice(ctx context.Context, b Bounds, dw, dh int, res *ImportResult, report func(string, float32)) ([]float32, error) {
	report("Looking for lidar", 0)
	l, err := OpenLidar(ctx, b.MinLat, b.MaxLat, b.MinLon, b.MaxLon)
	if err != nil {
		return nil, err
	}
	for _, s := range l.Sources {
		res.LidarSources = appendUnique(res.LidarSources, s.Title)
	}
	if err := l.Prefetch(ctx, b.MinLat, b.MaxLat, b.MinLon, b.MaxLon, func(done, total int) {
		report("Fetching lidar", float32(done)/float32(max(total, 1)))
	}); err != nil {
		return nil, err
	}

	heights := make([]float32, dw*dh)
	rowsCh := make(chan int)
	var mu sync.Mutex
	var firstErr error
	done, valid := 0, 0
	var wg sync.WaitGroup
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range rowsCh {
				lat := b.MaxLat - float64(j)/float64(dh-1)*(b.MaxLat-b.MinLat)
				n := 0
				var rowErr error
				for i := 0; i < dw && rowErr == nil; i++ {
					lon := b.MinLon + float64(i)/float64(dw-1)*(b.MaxLon-b.MinLon)
					var v float32
					v, rowErr = l.At(ctx, lat, lon)
					heights[j*dw+i] = v
					if v == v {
						n++
					}
				}
				mu.Lock()
				if rowErr != nil && firstErr == nil {
					firstErr = rowErr
				}
				done++
				valid += n
				if done%32 == 0 {
					report("Sampling lidar", float32(done)/float32(dh))
				}
				mu.Unlock()
			}
		}()
	}
	for j := 0; j < dh; j++ {
		if ctx.Err() != nil {
			break
		}
		rowsCh <- j
	}
	close(rowsCh)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	res.LidarCoverage = float32(valid) / float32(len(heights))
	if valid == 0 {
		res.LidarNote = ErrNoLidar.Error()
		return nil, nil
	}
	return heights, nil
}

// blendIntoTiles fills NaNs in h from tile, and fades lidar into the
// tiles over lidarFeather samples at the edge of its coverage so the
// seam between the two doesn't show as a step.
func blendIntoTiles(h []float32, w, ht int, tile func(i, j int) float32) {
	mask := make([]float32, len(h))
	for k, v := range h {
		if v == v {
			mask[k] = 1
		}
	}
	boxBlur(mask, w, ht, lidarFeather)
	for j := 0; j < ht; j++ {
		for i := 0; i < w; i++ {
			k := j*w + i
			base := tile(i, j)
			if h[k] != h[k] {
				h[k] = base
				continue
			}
			if m := mask[k]; m < 1 {
				f := float32(smoothstep(0.5, 1, float64(m)))
				h[k] = base + (h[k]-base)*f
			}
		}
	}
}

// boxBlur blurs v (w × h, row-major) in place with a (2r+1)² box,
// treating samples beyond the edge as copies of the edge.
func boxBlur(v []float32, w, h, r int) {
	line := make([]float32, max(w, h))
	pass := func(n int, get func(int) float32, set func(int, float32)) {
		var sum float32
		for k := -r; k <= r; k++ {
			sum += get(min(max(k, 0), n-1))
		}
		for k := 0; k < n; k++ {
			line[k] = sum / float32(2*r+1)
			sum += get(min(k+r+1, n-1)) - get(max(k-r, 0))
		}
		for k := 0; k < n; k++ {
			set(k, line[k])
		}
	}
	for j := 0; j < h; j++ {
		row := v[j*w : (j+1)*w]
		pass(w, func(i int) float32 { return row[i] }, func(i int, x float32) { row[i] = x })
	}
	for i := 0; i < w; i++ {
		pass(h, func(j int) float32 { return v[j*w+i] }, func(j int, x float32) { v[j*w+i] = x })
	}
}

// cellsFromLattice averages the lattice over each cell's footprint, one
// cell wide and centred on the cell's lattice point (trapezoid weights,
// clamped at the map edge).
func cellsFromLattice(h []float32, w, ht, cols, rows, per int) [][]float32 {
	half := per / 2
	weight := func(k int) float32 {
		if k == -half || k == half {
			return 0.5
		}
		return 1
	}
	out := make([][]float32, rows)
	for r := 0; r < rows; r++ {
		out[r] = make([]float32, cols)
		for c := 0; c < cols; c++ {
			var sum, wsum float32
			for dz := -half; dz <= half; dz++ {
				j := min(max(r*per+dz, 0), ht-1)
				for dx := -half; dx <= half; dx++ {
					i := min(max(c*per+dx, 0), w-1)
					wt := weight(dx) * weight(dz)
					sum += h[j*w+i] * wt
					wsum += wt
				}
			}
			out[r][c] = sum / wsum
		}
	}
	return out
}

func smoothstep(e0, e1, x float64) float64 {
	t := math.Min(math.Max((x-e0)/(e1-e0), 0), 1)
	return t * t * (3 - 2*t)
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}
