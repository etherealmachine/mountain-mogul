package geo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// SearchResult holds a single Nominatim place result.
type SearchResult struct {
	DisplayName string
	Lat, Lon    float64
	BBox        [4]float64 // [minLat, maxLat, minLon, maxLon]
}

// Search queries Nominatim for the given place name, returning up to 5 results.
func Search(query string) ([]SearchResult, error) {
	params := url.Values{
		"q":      {query},
		"format": {"json"},
		"limit":  {"5"},
	}
	apiURL := "https://nominatim.openstreetmap.org/search?" + params.Encode()

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mountain-mogul/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nominatim request: %w", err)
	}
	defer resp.Body.Close()

	var raw []struct {
		DisplayName string   `json:"display_name"`
		Lat         string   `json:"lat"`
		Lon         string   `json:"lon"`
		BoundingBox []string `json:"boundingbox"` // [minLat, maxLat, minLon, maxLon]
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("nominatim decode: %w", err)
	}

	results := make([]SearchResult, 0, len(raw))
	for _, item := range raw {
		lat, _ := strconv.ParseFloat(item.Lat, 64)
		lon, _ := strconv.ParseFloat(item.Lon, 64)
		var bbox [4]float64
		if len(item.BoundingBox) == 4 {
			for i, s := range item.BoundingBox {
				bbox[i], _ = strconv.ParseFloat(s, 64)
			}
		} else {
			// Fallback: 0.05° box around the point
			bbox = [4]float64{lat - 0.05, lat + 0.05, lon - 0.05, lon + 0.05}
		}
		results = append(results, SearchResult{
			DisplayName: item.DisplayName,
			Lat:         lat,
			Lon:         lon,
			BBox:        bbox,
		})
	}
	return results, nil
}
