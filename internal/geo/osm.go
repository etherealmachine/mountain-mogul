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
	"sort"
	"strconv"
	"strings"
	"time"
)

// LatLon is a WGS84 point in degrees.
type LatLon struct{ Lat, Lon float64 }

// OSMLayer picks which OpenStreetMap features a fetch asks for.
type OSMLayer uint8

const (
	// OSMSki is lifts (aerialway=*), downhill runs (piste:type=downhill)
	// and ski-area boundaries (landuse=winter_sports).
	OSMSki OSMLayer = 1 << iota
	// OSMRoads is roads cars drive on (see roadWidths).
	OSMRoads
)

// OSMMap is the OpenStreetMap features inside Covered. Data ©
// OpenStreetMap contributors, ODbL; anything showing it must credit them.
type OSMMap struct {
	Lifts   []SkiLift
	Runs    []SkiRun
	Areas   []SkiAreaOutline
	Roads   []Road
	Covered Bounds

	layers OSMLayer // what the parse keeps
}

// Road is a highway=* way. Width is the paved width in metres, from
// the width or lanes tags, else typical for its class.
type Road struct {
	ID     int64
	Name   string
	Kind   string // highway value: motorway, trunk, primary, residential...
	Width  float64
	Tunnel bool
	Path   []LatLon
}

// roadWidths is the typical paved width, in metres, of each highway
// class that counts as a road. Paths, tracks, and footways are left out:
// they barely cut the ground, and on a ski hill a track is often a run.
var roadWidths = map[string]float64{
	"motorway": 12, "motorway_link": 6, "trunk": 11, "trunk_link": 6,
	"primary": 10, "primary_link": 6, "secondary": 9, "secondary_link": 6,
	"tertiary": 8, "tertiary_link": 6, "unclassified": 6, "residential": 6,
	"service": 4,
}

const laneWidth = 3.7

// minorService are service=* values too small to cut the ground.
var minorService = map[string]bool{"parking_aisle": true, "driveway": true}

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
func (m *OSMMap) Merge(o *OSMMap) {
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

// FetchOSM downloads the given layers inside b. It asks Overpass first,
// which filters by tag server-side, and falls back to the main OSM API
// on a box of at most osmAPIMaxSpanDeg around b's centre (enough for any
// import square) when Overpass is down or busy. Covered reports the box
// actually fetched.
func FetchOSM(ctx context.Context, b Bounds, layers OSMLayer) (*OSMMap, error) {
	m, err := overpassFetch(ctx, b, layers)
	if err == nil {
		return m, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	small := b.ClampSpan(osmAPIMaxSpanDeg)
	m, err2 := osmAPIFetch(ctx, small, layers)
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

func overpassFetch(ctx context.Context, b Bounds, layers OSMLayer) (*OSMMap, error) {
	box := fmt.Sprintf("(%.6f,%.6f,%.6f,%.6f)", b.MinLat, b.MinLon, b.MaxLat, b.MaxLon)
	q := "[out:json][timeout:25];("
	if layers&OSMSki != 0 {
		q += `way["aerialway"]` + box + ";" +
			`way["piste:type"="downhill"]` + box + ";" +
			`wr["landuse"="winter_sports"]` + box + ";"
	}
	if layers&OSMRoads != 0 {
		kinds := make([]string, 0, len(roadWidths))
		for k := range roadWidths {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		q += `way["highway"~"^(` + strings.Join(kinds, "|") + `)$"]` + box + ";"
	}
	q += ");out geom;"
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
	m, err := parseOverpass(body, layers)
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

func parseOverpass(body []byte, layers OSMLayer) (*OSMMap, error) {
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
	m := &OSMMap{layers: layers}
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

func osmAPIFetch(ctx context.Context, b Bounds, layers OSMLayer) (*OSMMap, error) {
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
	m, err := parseOSMXML(body, layers)
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

func parseOSMXML(body []byte, layers OSMLayer) (*OSMMap, error) {
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
	m := &OSMMap{layers: layers}
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

func (m *OSMMap) addWay(id int64, tags map[string]string, path []LatLon) {
	if len(path) < 2 {
		return
	}
	if m.layers&OSMRoads != 0 {
		if w, ok := roadWidths[tags["highway"]]; ok && !minorService[tags["service"]] {
			if lanes, err := strconv.Atoi(tags["lanes"]); err == nil && lanes > 0 {
				w = float64(lanes)*laneWidth + 2
			}
			if tw, err := strconv.ParseFloat(strings.TrimSuffix(tags["width"], " m"), 64); err == nil && tw > 0 {
				w = tw
			}
			m.Roads = append(m.Roads, Road{ID: id, Name: tags["name"], Kind: tags["highway"], Width: w,
				Tunnel: tags["tunnel"] != "" && tags["tunnel"] != "no", Path: path})
		}
	}
	if m.layers&OSMSki == 0 {
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
func (m *OSMMap) addRelation(id int64, tags map[string]string, paths [][]LatLon) {
	if m.layers&OSMSki != 0 && tags["landuse"] == "winter_sports" && len(paths) > 0 {
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
