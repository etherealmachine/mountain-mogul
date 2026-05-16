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
		"q":               {query},
		"format":          {"json"},
		"limit":           {"5"},
		"accept-language": {"en"},
	}
	apiURL := "https://nominatim.openstreetmap.org/search?" + params.Encode()

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mountain-mogul/1.0")
	req.Header.Set("Accept-Language", "en")

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
		const minSpanDeg = 0.1 // ~10 km; expand pinpoint results (peaks, nodes) to be usable
		var bbox [4]float64
		if len(item.BoundingBox) == 4 {
			for i, s := range item.BoundingBox {
				bbox[i], _ = strconv.ParseFloat(s, 64)
			}
			// Nominatim returns a ~0.0001° pinpoint for peaks/nodes — expand it.
			if bbox[1]-bbox[0] < minSpanDeg {
				bbox[0] = lat - minSpanDeg/2
				bbox[1] = lat + minSpanDeg/2
			}
			if bbox[3]-bbox[2] < minSpanDeg {
				bbox[2] = lon - minSpanDeg/2
				bbox[3] = lon + minSpanDeg/2
			}
		} else {
			bbox = [4]float64{lat - minSpanDeg/2, lat + minSpanDeg/2, lon - minSpanDeg/2, lon + minSpanDeg/2}
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
