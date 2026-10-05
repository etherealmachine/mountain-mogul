package geo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SearchResult holds a single Nominatim place result.
type SearchResult struct {
	DisplayName string
	Lat, Lon    float64
	BBox        [4]float64 // [minLat, maxLat, minLon, maxLon]
	SkiArea     bool       // OSM tags it as a ski area or resort
}

// searchLimit is how many results Search returns at most.
const searchLimit = 8

// Search finds places named query, ski areas first. OSM often only
// knows a resort by its full name ("Boreal Mountain Resort", not
// "Boreal"), so unless the query already names a resort it also asks
// for "<query> resort" and merges the two.
func Search(query string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	results, err := nominatimSearch(query)
	if err != nil {
		return nil, err
	}
	// Ski areas from either search come first, then the places the query
	// itself found; the extra search's other hits (hotels, campgrounds)
	// only fill what's left.
	rank := func(r SearchResult, extra bool) int {
		switch {
		case r.SkiArea:
			return 0
		case !extra:
			return 1
		}
		return 2
	}
	type ranked struct {
		r    SearchResult
		rank int
	}
	var all []ranked
	for _, r := range results {
		all = append(all, ranked{r, rank(r, false)})
	}
	if !strings.Contains(strings.ToLower(query), "resort") {
		// Nominatim's usage policy allows one request a second.
		time.Sleep(time.Second)
		if more, err := nominatimSearch(query + " resort"); err == nil {
			for _, r := range more {
				all = append(all, ranked{r, rank(r, true)})
			}
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].rank < all[j].rank })
	seen := map[string]bool{}
	var out []SearchResult
	for _, a := range all {
		if !seen[a.r.DisplayName] {
			seen[a.r.DisplayName] = true
			out = append(out, a.r)
		}
	}
	if len(out) > searchLimit {
		out = out[:searchLimit]
	}
	return out, nil
}

func nominatimSearch(query string) ([]SearchResult, error) {
	params := url.Values{
		"q":               {query},
		"format":          {"json"},
		"limit":           {strconv.Itoa(searchLimit)},
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
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("place search: HTTP %s", resp.Status)
	}

	var raw []struct {
		DisplayName string   `json:"display_name"`
		Lat         string   `json:"lat"`
		Lon         string   `json:"lon"`
		BoundingBox []string `json:"boundingbox"` // [minLat, maxLat, minLon, maxLon]
		Class       string   `json:"class"`
		Type        string   `json:"type"`
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
			SkiArea:     isSkiArea(item.Class, item.Type, item.DisplayName),
		})
	}
	return results, nil
}

// isSkiArea reports whether an OSM result is a ski area: tagged as winter
// sports land, or a sports centre or resort whose name says so (Boreal is
// mapped as a sports centre). A resort needs "ski" or "mountain" in its
// name; plain "resort" is usually a hotel.
func isSkiArea(class, typ, name string) bool {
	if class == "landuse" && typ == "winter_sports" {
		return true
	}
	if class != "leisure" {
		return false
	}
	first, _, _ := strings.Cut(strings.ToLower(name), ",")
	ski := strings.Contains(first, "ski") || strings.Contains(first, "mountain")
	switch typ {
	case "sports_centre":
		return ski || strings.Contains(first, "resort")
	case "resort":
		return ski
	}
	return false
}
