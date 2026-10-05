package geo

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestParseOverpass(t *testing.T) {
	body := []byte(`{"elements":[
		{"type":"way","id":1,"tags":{"aerialway":"chair_lift","name":"Sunrise","aerialway:occupancy":"4"},
		 "geometry":[{"lat":38.68,"lon":-120.06},{"lat":38.67,"lon":-120.07}]},
		{"type":"way","id":2,"tags":{"aerialway":"station"},"geometry":[{"lat":1,"lon":1},{"lat":2,"lon":2}]},
		{"type":"way","id":3,"tags":{"piste:type":"downhill","piste:difficulty":"expert","name":"The Wall"},
		 "geometry":[{"lat":38.69,"lon":-120.06},{"lat":38.68,"lon":-120.05}]},
		{"type":"way","id":4,"tags":{"piste:type":"nordic"},"geometry":[{"lat":1,"lon":1},{"lat":2,"lon":2}]},
		{"type":"relation","id":5,"tags":{"landuse":"winter_sports","name":"Kirkwood"},"members":[
			{"type":"way","role":"outer","geometry":[{"lat":38.6,"lon":-120.1},{"lat":38.7,"lon":-120.0}]},
			{"type":"way","role":"inner","geometry":[{"lat":38.65,"lon":-120.05},{"lat":38.66,"lon":-120.04}]}]}
	]}`)
	m, err := parseOverpass(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Lifts) != 1 || m.Lifts[0].Name != "Sunrise" || m.Lifts[0].Seats != 4 || m.Lifts[0].Path[1].Lon != -120.07 {
		t.Errorf("lifts = %+v", m.Lifts)
	}
	if len(m.Runs) != 1 || m.Runs[0].Difficulty != "expert" || m.Runs[0].Name != "The Wall" {
		t.Errorf("runs = %+v", m.Runs)
	}
	if len(m.Areas) != 1 || m.Areas[0].ID != -5 || len(m.Areas[0].Paths) != 1 {
		t.Errorf("areas = %+v", m.Areas)
	}
}

func TestParseOSMXML(t *testing.T) {
	body := []byte(`<osm>
		<node id="10" lat="38.68" lon="-120.06"/><node id="11" lat="38.67" lon="-120.07"/>
		<way id="1"><nd ref="10"/><nd ref="11"/><tag k="aerialway" v="gondola"/><tag k="name" v="G"/></way>
		<way id="2"><nd ref="10"/><nd ref="11"/><tag k="highway" v="service"/></way>
		<way id="3"><nd ref="11"/><nd ref="10"/></way>
		<relation id="7"><member type="way" ref="3" role="outer"/><tag k="landuse" v="winter_sports"/></relation>
	</osm>`)
	m, err := parseOSMXML(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Lifts) != 1 || m.Lifts[0].Kind != "gondola" || len(m.Lifts[0].Path) != 2 {
		t.Errorf("lifts = %+v", m.Lifts)
	}
	if len(m.Runs) != 0 || len(m.Areas) != 1 || m.Areas[0].Paths[0][0].Lat != 38.67 {
		t.Errorf("runs = %+v areas = %+v", m.Runs, m.Areas)
	}
}

func TestSkiMapMergeDedupes(t *testing.T) {
	a := &SkiMap{Lifts: []SkiLift{{ID: 1}}, Runs: []SkiRun{{ID: 2}}}
	a.Merge(&SkiMap{Lifts: []SkiLift{{ID: 1}, {ID: 3}}, Runs: []SkiRun{{ID: 2}}, Areas: []SkiAreaOutline{{ID: -1}}})
	if len(a.Lifts) != 2 || len(a.Runs) != 1 || len(a.Areas) != 1 {
		t.Errorf("merged = %+v", a)
	}
}

func TestMercatorRoundTrip(t *testing.T) {
	for _, lat := range []float64{-60, 0, 38.68, 47.1} {
		if got := MercatorLat(MercatorY(lat)); got-lat > 1e-9 || lat-got > 1e-9 {
			t.Errorf("MercatorLat(MercatorY(%v)) = %v", lat, got)
		}
	}
}

// TestFetchSkiMapLive hits the real servers; set MM_NET_TESTS=1.
func TestFetchSkiMapLive(t *testing.T) {
	if os.Getenv("MM_NET_TESTS") == "" {
		t.Skip("set MM_NET_TESTS=1")
	}
	kirkwood := Bounds{MinLat: 38.655, MaxLat: 38.70, MinLon: -120.095, MaxLon: -120.04}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for name, fetch := range map[string]func(context.Context, Bounds) (*SkiMap, error){
		"overpass": overpassSkiMap, "osm api": osmAPISkiMap,
	} {
		start := time.Now()
		m, err := fetch(ctx, kirkwood)
		if err != nil {
			t.Logf("%s: %v", name, err)
			continue
		}
		t.Logf("%s: %d lifts, %d runs, %d areas in %v", name, len(m.Lifts), len(m.Runs), len(m.Areas), time.Since(start))
		if len(m.Lifts) < 10 || len(m.Runs) < 40 {
			t.Errorf("%s: too few features", name)
		}
	}
}
