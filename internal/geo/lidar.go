package geo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Lidar samples USGS 3DEP 1 m bare-earth DEMs (lidar-derived, US only).
// Where surveys overlap, the most recently published one wins; points
// none of them cover come back NaN.
type Lidar struct {
	Sources []LidarSource
}

// LidarSource is one 10 km DEM tile of one survey.
type LidarSource struct {
	Title     string
	URL       string
	Published string
	Zone      int

	r *cogReader
}

var lidarZone = regexp.MustCompile(`USGS_1M_(\d+)_`)

// ErrNoLidar is OpenLidar's error when no 1 m survey touches the area.
var ErrNoLidar = errors.New("no lidar survey covers this area")

// OpenLidar finds and opens every 1 m DEM tile touching the bounding box.
func OpenLidar(ctx context.Context, minLat, maxLat, minLon, maxLon float64) (*Lidar, error) {
	q := url.Values{}
	q.Set("datasets", "Digital Elevation Model (DEM) 1 meter")
	q.Set("bbox", fmt.Sprintf("%f,%f,%f,%f", minLon, minLat, maxLon, maxLat))
	q.Set("prodFormats", "GeoTIFF")
	q.Set("max", "100")
	client := &http.Client{Timeout: 2 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://tnmaccess.nationalmap.gov/api/v1/products?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("USGS product search: HTTP %s", resp.Status)
	}
	var body struct {
		Items []struct {
			Title           string `json:"title"`
			DownloadURL     string `json:"downloadURL"`
			PublicationDate string `json:"publicationDate"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("USGS product search: %w", err)
	}

	l := &Lidar{}
	for _, it := range body.Items {
		m := lidarZone.FindStringSubmatch(it.DownloadURL)
		if m == nil {
			continue
		}
		zone, _ := strconv.Atoi(m[1])
		l.Sources = append(l.Sources, LidarSource{Title: it.Title, URL: it.DownloadURL, Published: it.PublicationDate, Zone: zone})
	}
	if len(l.Sources) == 0 {
		return nil, ErrNoLidar
	}
	sort.SliceStable(l.Sources, func(i, j int) bool { return l.Sources[i].Published > l.Sources[j].Published })

	var wg sync.WaitGroup
	errs := make([]error, len(l.Sources))
	for i := range l.Sources {
		wg.Add(1)
		go func(s *LidarSource, err *error) {
			defer wg.Done()
			s.r, *err = openCOG(ctx, client, s.URL)
		}(&l.Sources[i], &errs[i])
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return l, nil
}

// Prefetch downloads, in parallel, every tile that sampling inside the
// bounding box will need, so At doesn't fetch them one at a time.
func (l *Lidar) Prefetch(ctx context.Context, minLat, maxLat, minLon, maxLon float64, progress func(done, total int)) error {
	type job struct {
		r  *cogReader
		ti int
	}
	var jobs []job
	for i := range l.Sources {
		r := l.Sources[i].r
		minE, minN := math.Inf(1), math.Inf(1)
		maxE, maxN := math.Inf(-1), math.Inf(-1)
		for _, c := range [4][2]float64{{minLat, minLon}, {minLat, maxLon}, {maxLat, minLon}, {maxLat, maxLon}} {
			e, n := LatLonToUTM(c[0], c[1], l.Sources[i].Zone)
			minE, maxE = math.Min(minE, e), math.Max(maxE, e)
			minN, maxN = math.Min(minN, n), math.Max(maxN, n)
		}
		px0 := int(math.Floor((minE-r.originX)/r.scaleX)) - 1
		px1 := int(math.Ceil((maxE-r.originX)/r.scaleX)) + 1
		py0 := int(math.Floor((r.originY-maxN)/r.scaleY)) - 1
		py1 := int(math.Ceil((r.originY-minN)/r.scaleY)) + 1
		px0, py0 = max(px0, 0), max(py0, 0)
		px1, py1 = min(px1, r.width-1), min(py1, r.height-1)
		if px0 > px1 || py0 > py1 {
			continue
		}
		across := (r.width + r.tileW - 1) / r.tileW
		for ty := py0 / r.tileH; ty <= py1/r.tileH; ty++ {
			for tx := px0 / r.tileW; tx <= px1/r.tileW; tx++ {
				jobs = append(jobs, job{r, ty*across + tx})
			}
		}
	}

	work := make(chan job)
	var mu sync.Mutex
	var firstErr error
	done := 0
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range work {
				_, err := j.r.tile(ctx, j.ti)
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
				}
				done++
				if progress != nil {
					progress(done, len(jobs))
				}
				mu.Unlock()
			}
		}()
	}
	for _, j := range jobs {
		work <- j
	}
	close(work)
	wg.Wait()
	return firstErr
}

// At is the bare-earth elevation in metres at (lat, lon), or NaN where
// no survey has data.
func (l *Lidar) At(ctx context.Context, lat, lon float64) (float32, error) {
	for i := range l.Sources {
		s := &l.Sources[i]
		e, n := LatLonToUTM(lat, lon, s.Zone)
		if !s.r.contains(e, n) {
			continue
		}
		v, err := s.r.sample(ctx, e, n)
		if err != nil {
			return 0, err
		}
		if !math.IsNaN(float64(v)) {
			return v, nil
		}
	}
	return float32(math.NaN()), nil
}
