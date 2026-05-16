package geo

import (
	"context"
	"fmt"
	"image/png"
	"math"
	"net/http"
)

const terrainZoom = 14
const tilePixels = 256

// FetchGrid fetches elevation data covering the bounding box using AWS Terrain
// Tiles (Terrarium encoding) at zoom 14 (~2.4 m/pixel at mid-latitudes).
// The cols/rows parameters are ignored; the returned grid is at native tile
// resolution, cropped to the bounding box. Pass the result to ResampleToGrid.
func FetchGrid(ctx context.Context, minLat, maxLat, minLon, maxLon float64, _, _ int, progressFn func(float32)) ([][]float32, error) {
	x0, y0 := lonLatToTile(minLon, maxLat, terrainZoom) // top-left tile
	x1, y1 := lonLatToTile(maxLon, minLat, terrainZoom) // bottom-right tile

	tileCountX := x1 - x0 + 1
	tileCountY := y1 - y0 + 1
	total := tileCountX * tileCountY

	stitchedW := tileCountX * tilePixels
	stitchedH := tileCountY * tilePixels
	stitched := make([][]float32, stitchedH)
	for i := range stitched {
		stitched[i] = make([]float32, stitchedW)
	}

	fetched := 0
	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			tile, err := fetchTerrainTile(ctx, tx, ty, terrainZoom)
			if err != nil {
				return nil, fmt.Errorf("tile %d/%d/%d: %w", terrainZoom, tx, ty, err)
			}
			offX := (tx - x0) * tilePixels
			offY := (ty - y0) * tilePixels
			for py := 0; py < tilePixels; py++ {
				for px := 0; px < tilePixels; px++ {
					stitched[offY+py][offX+px] = tile[py][px]
				}
			}
			fetched++
			if progressFn != nil {
				progressFn(float32(fetched) / float32(total))
			}
		}
	}

	// Crop stitched image to the exact bounding box.
	px0, py0 := lonLatToPixelOffset(minLon, maxLat, terrainZoom, x0, y0)
	px1, py1 := lonLatToPixelOffset(maxLon, minLat, terrainZoom, x0, y0)

	left := int(math.Round(px0))
	top := int(math.Round(py0))
	cropW := int(math.Round(px1)) - left
	cropH := int(math.Round(py1)) - top
	if cropW < 1 {
		cropW = 1
	}
	if cropH < 1 {
		cropH = 1
	}
	if left < 0 {
		left = 0
	}
	if top < 0 {
		top = 0
	}
	if left+cropW > stitchedW {
		cropW = stitchedW - left
	}
	if top+cropH > stitchedH {
		cropH = stitchedH - top
	}

	out := make([][]float32, cropH)
	for row := 0; row < cropH; row++ {
		out[row] = make([]float32, cropW)
		copy(out[row], stitched[top+row][left:left+cropW])
	}
	return repairElevationArtifacts(out), nil
}

// maxRealisticElevStep is the maximum plausible elevation change between
// adjacent raw pixels at zoom-14 resolution (~8 m/px). A near-vertical
// cliff (85°) produces ~100 m/px; data errors are typically 500–3000 m off.
const maxRealisticElevStep = float32(150)

// seamBlendHalf is the number of pixels on each side of a detected seam
// that get replaced with linearly interpolated values. 6 px × 8 m ≈ 48 m
// blend zone — wide enough to be invisible at game scale.
const seamBlendHalf = 6

// repairElevationArtifacts fixes two classes of DEM error:
//  1. Isolated spike pixels — value disagrees with majority of 4-neighbours.
//  2. Full seam lines — a column or row where the majority of cross-boundary
//     gradients exceed maxRealisticElevStep (data-source boundary mismatch).
//
// Both are applied at native tile resolution (~8 m/px) before resampling.
func repairElevationArtifacts(grid [][]float32) [][]float32 {
	grid = fixOutlierPixels(grid)
	grid = repairSeamLines(grid)
	return grid
}

func fixOutlierPixels(grid [][]float32) [][]float32 {
	rows := len(grid)
	if rows == 0 {
		return grid
	}
	cols := len(grid[0])
	out := make([][]float32, rows)
	for r := range out {
		out[r] = make([]float32, cols)
		copy(out[r], grid[r])
	}
	dirs := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			v := grid[r][c]
			var sum float32
			var n, bad int
			for _, d := range dirs {
				nr, nc := r+d[0], c+d[1]
				if nr < 0 || nr >= rows || nc < 0 || nc >= cols {
					continue
				}
				nv := grid[nr][nc]
				sum += nv
				n++
				if absF(v-nv) > maxRealisticElevStep {
					bad++
				}
			}
			if n > 0 && bad*2 > n {
				out[r][c] = sum / float32(n)
			}
		}
	}
	return out
}

// repairSeamLines detects column and row seams (where the majority of
// cross-boundary gradients exceed the threshold) and linearly blends across
// a window of seamBlendHalf pixels on each side.
func repairSeamLines(grid [][]float32) [][]float32 {
	rows := len(grid)
	if rows == 0 {
		return grid
	}
	cols := len(grid[0])

	seamCols, seamRows := detectSeams(grid, rows, cols)
	if len(seamCols) == 0 && len(seamRows) == 0 {
		return grid
	}

	out := make([][]float32, rows)
	for r := range out {
		out[r] = make([]float32, cols)
		copy(out[r], grid[r])
	}

	for _, c := range seamCols {
		for r := 0; r < rows; r++ {
			lo := c - seamBlendHalf
			hi := c + seamBlendHalf
			if lo < 0 {
				lo = 0
			}
			if hi >= cols {
				hi = cols - 1
			}
			vL := grid[r][lo]
			vR := grid[r][hi]
			span := float32(hi - lo)
			if span <= 0 {
				continue
			}
			for col := lo + 1; col < hi; col++ {
				t := float32(col-lo) / span
				out[r][col] = vL*(1-t) + vR*t
			}
		}
	}

	for _, sr := range seamRows {
		for c := 0; c < cols; c++ {
			lo := sr - seamBlendHalf
			hi := sr + seamBlendHalf
			if lo < 0 {
				lo = 0
			}
			if hi >= rows {
				hi = rows - 1
			}
			vT := out[lo][c]
			vB := out[hi][c]
			span := float32(hi - lo)
			if span <= 0 {
				continue
			}
			for row := lo + 1; row < hi; row++ {
				t := float32(row-lo) / span
				out[row][c] = vT*(1-t) + vB*t
			}
		}
	}

	return out
}

func detectSeams(grid [][]float32, rows, cols int) (seamCols, seamRows []int) {
	for c := 1; c < cols; c++ {
		var big int
		for r := 0; r < rows; r++ {
			if absF(grid[r][c]-grid[r][c-1]) > maxRealisticElevStep {
				big++
			}
		}
		if big*2 > rows {
			seamCols = append(seamCols, c)
		}
	}
	for r := 1; r < rows; r++ {
		var big int
		for c := 0; c < cols; c++ {
			if absF(grid[r][c]-grid[r-1][c]) > maxRealisticElevStep {
				big++
			}
		}
		if big*2 > cols {
			seamRows = append(seamRows, r)
		}
	}
	return
}

func absF(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// lonLatToTile converts a lon/lat coordinate to the tile XY at zoom z.
// Tile Y increases southward (standard Slippy Map / Web Mercator convention).
func lonLatToTile(lon, lat float64, z int) (int, int) {
	n := math.Pow(2, float64(z))
	x := int(math.Floor((lon + 180.0) / 360.0 * n))
	latRad := lat * math.Pi / 180.0
	y := int(math.Floor((1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n))
	return x, y
}

// lonLatToPixelOffset returns the pixel position within the stitched image
// for a given lon/lat, given the top-left tile origin (tileX0, tileY0).
func lonLatToPixelOffset(lon, lat float64, z int, tileX0, tileY0 int) (float64, float64) {
	n := math.Pow(2, float64(z))
	worldPx := (lon + 180.0) / 360.0 * n * tilePixels
	latRad := lat * math.Pi / 180.0
	worldPy := (1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n * tilePixels
	return worldPx - float64(tileX0*tilePixels), worldPy - float64(tileY0*tilePixels)
}

func fetchTerrainTile(ctx context.Context, x, y, z int) ([][]float32, error) {
	url := fmt.Sprintf("https://s3.amazonaws.com/elevation-tiles-prod/terrarium/%d/%d/%d.png", z, x, y)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mountain-mogul/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	img, err := png.Decode(resp.Body)
	if err != nil {
		return nil, err
	}

	grid := make([][]float32, tilePixels)
	for py := 0; py < tilePixels; py++ {
		grid[py] = make([]float32, tilePixels)
		for px := 0; px < tilePixels; px++ {
			r, g, b, _ := img.At(px, py).RGBA()
			// RGBA() returns [0, 65535]; shift right 8 to get [0, 255].
			// Terrarium: elevation = R*256 + G + B/256 - 32768 (metres)
			rf := float32(r >> 8)
			gf := float32(g >> 8)
			bf := float32(b >> 8)
			grid[py][px] = rf*256.0 + gf + bf/256.0 - 32768.0
		}
	}
	return grid, nil
}
