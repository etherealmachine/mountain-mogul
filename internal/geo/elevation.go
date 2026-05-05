package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type latLon struct{ lat, lon float64 }

// FetchGrid fetches elevation data for a cols×rows grid over the bounding box.
// Returns [row][col] elevations in metres, north-to-south, west-to-east.
// progressFn is called with values 0..1 as batches complete; may be nil.
// ctx is used for cancellation.
func FetchGrid(ctx context.Context, minLat, maxLat, minLon, maxLon float64, cols, rows int, progressFn func(float32)) ([][]float32, error) {
	pts := make([]latLon, 0, cols*rows)
	for row := 0; row < rows; row++ {
		lat := maxLat - float64(row)*(maxLat-minLat)/float64(rows-1)
		for col := 0; col < cols; col++ {
			lon := minLon + float64(col)*(maxLon-minLon)/float64(cols-1)
			pts = append(pts, latLon{lat, lon})
		}
	}

	elevations := make([]float64, len(pts))
	const batchSize = 100

	for start := 0; start < len(pts); start += batchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		end := start + batchSize
		if end > len(pts) {
			end = len(pts)
		}
		batch := pts[start:end]

		elev, err := fetchBatch(ctx, "ned10m", batch)
		if err != nil {
			elev, err = fetchBatch(ctx, "srtm30m", batch)
			if err != nil {
				return nil, fmt.Errorf("elevation fetch: %w", err)
			}
		}
		copy(elevations[start:end], elev)

		if progressFn != nil {
			progressFn(float32(end) / float32(len(pts)))
		}

		if end < len(pts) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}
	}

	grid := make([][]float32, rows)
	for row := 0; row < rows; row++ {
		grid[row] = make([]float32, cols)
		for col := 0; col < cols; col++ {
			grid[row][col] = float32(elevations[row*cols+col])
		}
	}
	return grid, nil
}

func fetchBatch(ctx context.Context, dataset string, pts []latLon) ([]float64, error) {
	locs := make([]string, len(pts))
	for i, p := range pts {
		locs[i] = fmt.Sprintf("%.6f,%.6f", p.lat, p.lon)
	}
	apiURL := fmt.Sprintf("https://api.opentopodata.org/v1/%s?locations=%s", dataset, strings.Join(locs, "|"))
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mountain-mogul/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			Elevation *float64 `json:"elevation"`
		} `json:"results"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Status != "OK" {
		return nil, fmt.Errorf("opentopodata status: %s", result.Status)
	}

	elev := make([]float64, len(result.Results))
	for i, r := range result.Results {
		if r.Elevation != nil {
			elev[i] = *r.Elevation
		}
	}
	return elev, nil
}
