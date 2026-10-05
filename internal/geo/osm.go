package geo

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LatLon is a WGS84 point in degrees.
type LatLon struct{ Lat, Lon float64 }

// SkiMap is the OpenStreetMap ski infrastructure inside Covered: lifts
// (aerialway=*), downhill runs (piste:type=downhill) and ski-area
// boundaries (landuse=winter_sports). Data © OpenStreetMap contributors,
// ODbL; anything showing it must credit them.
type SkiMap struct {
	Lifts   []SkiLift
	Runs    []SkiRun
	Areas   []SkiAreaOutline
	Covered Bounds
}

type SkiLift struct {
	ID    int64
	Name  string
	Kind  string // aerialway value: chair_lift, gondola, t-bar, magic_carpet...
	Seats int    // aerialway:occupancy, 0 when untagged
	Path  []LatLon
}

type SkiRun struct {
	ID         int64
	Name       string
	Difficulty string // piste:difficulty: novice, easy, intermediate, advanced, expert, freeride, extreme
	Area       bool   // mapped as a piste polygon rather than a centre line
	Path       []LatLon
}

// SkiAreaOutline is a resort boundary. A multipolygon's member ways are
// kept as separate paths; they are only drawn, never filled.
type SkiAreaOutline struct {
	ID    int64
	Name  string
	Paths [][]LatLon
}

// Merge adds o's features that m doesn't already have.
func (m *SkiMap) Merge(o *SkiMap) {
	lifts := map[int64]bool{}
	for _, l := range m.Lifts {
		lifts[l.ID] = true
	}
	for _, l := range o.Lifts {
		if !lifts[l.ID] {
			m.Lifts = append(m.Lifts, l)
		}
	}
	runs := map[int64]bool{}
	for _, r := range m.Runs {
		runs[r.ID] = true
	}
	for _, r := range o.Runs {
		if !runs[r.ID] {
			m.Runs = append(m.Runs, r)
		}
	}
	areas := map[int64]bool{}
	for _, a := range m.Areas {
		areas[a.ID] = true
	}
	for _, a := range o.Areas {
		if !areas[a.ID] {
			m.Areas = append(m.Areas, a)
		}
	}
}

// osmAPIMaxSpanDeg caps the fallback request's side. The main API returns
// every node and way in the box (no tag filter) and refuses more than
// 50,000 nodes; Kirkwood's 0.055° × 0.045° was 3 MB.
const osmAPIMaxSpanDeg = 0.08

var osmClient = &http.Client{Timeout: 45 * time.Second}

// FetchSkiMap downloads the ski features inside b. It asks Overpass first,
// which filters by tag server-side, and falls back to the main OSM API
// on a box of at most osmAPIMaxSpanDeg around b's centre when Overpass is
// down or busy. Covered reports the box actually fetched.
func FetchSkiMap(ctx context.Context, b Bounds) (*SkiMap, error) {
	m, err := overpassSkiMap(ctx, b)
	if err == nil {
		return m, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	small := b.ClampSpan(osmAPIMaxSpanDeg)
	m, err2 := osmAPISkiMap(ctx, small)
	if err2 != nil {
		return nil, fmt.Errorf("overpass: %v; osm api: %v", err, err2)
	}
	return m, nil
}

// ClampSpan shrinks b about its centre so neither side exceeds span degrees.
func (b Bounds) ClampSpan(span float64) Bounds {
	if b.MaxLat-b.MinLat > span {
		c := (b.MinLat + b.MaxLat) / 2
		b.MinLat, b.MaxLat = c-span/2, c+span/2
	}
	if b.MaxLon-b.MinLon > span {
		c := (b.MinLon + b.MaxLon) / 2
		b.MinLon, b.MaxLon = c-span/2, c+span/2
	}
	return b
}

func osmGet(ctx context.Context, req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", "mountain-mogul/1.0 (terrain import)")
	resp, err := osmClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 120 || strings.HasPrefix(msg, "<") {
			msg = ""
		}
		return nil, fmt.Errorf("HTTP %d %s", resp.StatusCode, msg)
	}
	return body, nil
}

func overpassSkiMap(ctx context.Context, b Bounds) (*SkiMap, error) {
	box := fmt.Sprintf("(%.6f,%.6f,%.6f,%.6f)", b.MinLat, b.MinLon, b.MaxLat, b.MaxLon)
	q := "[out:json][timeout:25];(" +
		`way["aerialway"]` + box + ";" +
		`way["piste:type"="downhill"]` + box + ";" +
		`wr["landuse"="winter_sports"]` + box + ";" +
		");out geom;"
	req, err := http.NewRequest(http.MethodPost, "https://overpass-api.de/api/interpreter",
		strings.NewReader(url.Values{"data": {q}}.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := osmGet(ctx, req)
	if err != nil {
		return nil, err
	}
	m, err := parseOverpass(body)
	if err != nil {
		return nil, err
	}
	m.Covered = b
	return m, nil
}

type overpassElement struct {
	Type     string            `json:"type"`
	ID       int64             `json:"id"`
	Tags     map[string]string `json:"tags"`
	Geometry []LatLon          `json:"geometry"`
	Members  []struct {
		Type     string   `json:"type"`
		Role     string   `json:"role"`
		Geometry []LatLon `json:"geometry"`
	} `json:"members"`
}

func (p *LatLon) UnmarshalJSON(b []byte) error {
	var v struct{ Lat, Lon float64 }
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	p.Lat, p.Lon = v.Lat, v.Lon
	return nil
}

func parseOverpass(body []byte) (*SkiMap, error) {
	var resp struct {
		Elements []overpassElement `json:"elements"`
		Remark   string            `json:"remark"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	// Overpass reports a query timeout as a 200 with a remark and
	// truncated elements.
	if strings.Contains(resp.Remark, "error") {
		return nil, errors.New(resp.Remark)
	}
	m := &SkiMap{}
	for _, e := range resp.Elements {
		switch e.Type {
		case "way":
			m.addWay(e.ID, e.Tags, e.Geometry)
		case "relation":
			var paths [][]LatLon
			for _, mem := range e.Members {
				if mem.Type == "way" && mem.Role != "inner" && len(mem.Geometry) > 1 {
					paths = append(paths, mem.Geometry)
				}
			}
			m.addRelation(e.ID, e.Tags, paths)
		}
	}
	return m, nil
}

func osmAPISkiMap(ctx context.Context, b Bounds) (*SkiMap, error) {
	u := fmt.Sprintf("https://api.openstreetmap.org/api/0.6/map?bbox=%.6f,%.6f,%.6f,%.6f",
		b.MinLon, b.MinLat, b.MaxLon, b.MaxLat)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	body, err := osmGet(ctx, req)
	if err != nil {
		return nil, err
	}
	m, err := parseOSMXML(body)
	if err != nil {
		return nil, err
	}
	m.Covered = b
	return m, nil
}

type osmXMLTag struct {
	K string `xml:"k,attr"`
	V string `xml:"v,attr"`
}

func tagMap(tags []osmXMLTag) map[string]string {
	m := make(map[string]string, len(tags))
	for _, t := range tags {
		m[t.K] = t.V
	}
	return m
}

func parseOSMXML(body []byte) (*SkiMap, error) {
	var doc struct {
		Nodes []struct {
			ID  int64   `xml:"id,attr"`
			Lat float64 `xml:"lat,attr"`
			Lon float64 `xml:"lon,attr"`
		} `xml:"node"`
		Ways []struct {
			ID   int64 `xml:"id,attr"`
			Refs []struct {
				Ref int64 `xml:"ref,attr"`
			} `xml:"nd"`
			Tags []osmXMLTag `xml:"tag"`
		} `xml:"way"`
		Relations []struct {
			ID      int64 `xml:"id,attr"`
			Members []struct {
				Type string `xml:"type,attr"`
				Ref  int64  `xml:"ref,attr"`
				Role string `xml:"role,attr"`
			} `xml:"member"`
			Tags []osmXMLTag `xml:"tag"`
		} `xml:"relation"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	nodes := make(map[int64]LatLon, len(doc.Nodes))
	for _, n := range doc.Nodes {
		nodes[n.ID] = LatLon{n.Lat, n.Lon}
	}
	wayPaths := make(map[int64][]LatLon, len(doc.Ways))
	m := &SkiMap{}
	for _, w := range doc.Ways {
		path := make([]LatLon, 0, len(w.Refs))
		for _, r := range w.Refs {
			if p, ok := nodes[r.Ref]; ok {
				path = append(path, p)
			}
		}
		wayPaths[w.ID] = path
		m.addWay(w.ID, tagMap(w.Tags), path)
	}
	for _, rel := range doc.Relations {
		var paths [][]LatLon
		for _, mem := range rel.Members {
			if mem.Type == "way" && mem.Role != "inner" && len(wayPaths[mem.Ref]) > 1 {
				paths = append(paths, wayPaths[mem.Ref])
			}
		}
		m.addRelation(rel.ID, tagMap(rel.Tags), paths)
	}
	return m, nil
}

// notLifts are aerialway values for things that aren't a ride.
var notLifts = map[string]bool{"station": true, "pylon": true, "goods": true, "zip_line": true}

func (m *SkiMap) addWay(id int64, tags map[string]string, path []LatLon) {
	if len(path) < 2 {
		return
	}
	if kind := tags["aerialway"]; kind != "" && !notLifts[kind] {
		seats, _ := strconv.Atoi(tags["aerialway:occupancy"])
		m.Lifts = append(m.Lifts, SkiLift{ID: id, Name: tags["name"], Kind: kind, Seats: seats, Path: path})
	}
	if tags["piste:type"] == "downhill" {
		name := tags["piste:name"]
		if name == "" {
			name = tags["name"]
		}
		m.Runs = append(m.Runs, SkiRun{ID: id, Name: name, Difficulty: tags["piste:difficulty"],
			Area: tags["area"] == "yes", Path: path})
	}
	if tags["landuse"] == "winter_sports" {
		m.Areas = append(m.Areas, SkiAreaOutline{ID: id, Name: tags["name"], Paths: [][]LatLon{path}})
	}
}

// addRelation stores relation ids negated so they can't collide with way
// ids in Merge.
func (m *SkiMap) addRelation(id int64, tags map[string]string, paths [][]LatLon) {
	if tags["landuse"] == "winter_sports" && len(paths) > 0 {
		m.Areas = append(m.Areas, SkiAreaOutline{ID: -id, Name: tags["name"], Paths: paths})
	}
}

// Contains reports whether o lies inside b.
func (b Bounds) Contains(o Bounds) bool {
	return o.MinLat >= b.MinLat && o.MaxLat <= b.MaxLat && o.MinLon >= b.MinLon && o.MaxLon <= b.MaxLon
}

// MercatorY is the Web Mercator northing of lat, in radians of arc; map
// tiles are linear in it.
func MercatorY(lat float64) float64 {
	return math.Log(math.Tan(math.Pi/4 + lat*math.Pi/360))
}

// MercatorLat inverts MercatorY.
func MercatorLat(y float64) float64 {
	return (2*math.Atan(math.Exp(y)) - math.Pi/2) * 180 / math.Pi
}
