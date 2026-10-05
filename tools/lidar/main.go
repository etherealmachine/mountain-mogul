// lidar fetches USGS 3DEP 1 m lidar for a map imported from a real place
// and writes it as ground heights on the map's 1.25 m detail lattice:
//
//	go run ./tools/lidar -save assets/scenarios/kirkwood.save \
//	    -lat 38.676 -lon -120.068 -out kirkwood.heights
//
// Imported maps don't record their exact bounds, so the tool starts from
// the given centre and the map's size, then searches shift and scale for
// the best match to the map's own heightmap before sampling. Heights are
// in the map's frame (its lowest imported point is 0); NaN where no
// survey has data. Load the result with `-detail-file` on the game, or
// pass -write-save to bake it into a copy of the save (-from reuses an
// earlier heights file instead of fetching).
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"mountain-mogul/internal/geo"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/world"
)

func main() {
	savePath := flag.String("save", "", "scenario save the heights are for")
	lat := flag.Float64("lat", math.NaN(), "approximate map centre latitude")
	lon := flag.Float64("lon", math.NaN(), "approximate map centre longitude")
	search := flag.Float64("search", 300, "metres to search around the centre when aligning")
	out := flag.String("out", "", "heights file to write")
	from := flag.String("from", "", "heights file from an earlier run, instead of fetching")
	writeSave := flag.String("write-save", "", "copy of -save to write with the detail baked in")
	flag.Parse()
	fetch := *from == ""
	if *savePath == "" || (*out == "" && *writeSave == "") || (fetch && (math.IsNaN(*lat) || math.IsNaN(*lon))) {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*savePath, *lat, *lon, *search, *out, *from, *writeSave); err != nil {
		fmt.Fprintln(os.Stderr, "lidar:", err)
		os.Exit(1)
	}
}

func run(savePath string, lat, lon, search float64, out, from, writeSave string) error {
	var w, h int
	var heights []float32
	var err error
	if from != "" {
		w, h, heights, err = world.ReadDetailHeights(from)
	} else {
		w, h, heights, err = fetchHeights(savePath, lat, lon, search)
	}
	if err != nil {
		return err
	}
	if out != "" {
		if err := world.WriteDetailHeights(out, w, h, heights); err != nil {
			return err
		}
	}
	if writeSave != "" {
		return bakeIntoSave(savePath, writeSave, w, h, heights)
	}
	return nil
}

// bakeIntoSave writes a copy of the save at src to dst with the heights
// stored as its terrain detail. Only the detail field changes.
func bakeIntoSave(src, dst string, w, h int, heights []float32) error {
	wd, _, err := save.LoadScenario(src)
	if err != nil {
		return err
	}
	t := wd.Terrain
	if err := t.SetDetailFromHeights(w, h, heights); err != nil {
		return err
	}
	data, err := save.ReadScenarioData(src)
	if err != nil {
		return err
	}
	data.Detail = t.Detail.Bytes()
	if err := save.WriteScenarioData(dst, data); err != nil {
		return err
	}
	fmt.Printf("wrote %s (detail up to %.1f m)\n", dst, t.Detail.MaxAbs())
	return nil
}

// frame maps map metres (x east, z south from the north-west corner) to
// latitude and longitude, the way the terrain import laid the map out:
// size metres per side, linear in latitude and longitude.
type frame struct {
	lat0, lon0 float64 // centre
	size       float64 // metres the map's span of cells covers
	mapSpan    float64 // metres the map draws that span as
}

func (f frame) latLon(x, z float64) (float64, float64) {
	latM, lonM := geo.MetresPerDegree(f.lat0)
	s := f.size / f.mapSpan
	dx := (x - f.mapSpan/2) * s
	dz := (z - f.mapSpan/2) * s
	return f.lat0 - dz/latM, f.lon0 + dx/lonM
}

func fetchHeights(savePath string, lat, lon, search float64) (int, int, []float32, error) {
	w, _, err := save.LoadScenario(savePath)
	if err != nil {
		return 0, 0, nil, err
	}
	t := w.Terrain
	cell := float64(world.CellSize)
	mapSpan := float64(t.Width-1) * cell
	// The import's square is Width cells × 5 m, sampled by Width points.
	f := frame{lat0: lat, lon0: lon, size: float64(t.Width) * cell, mapSpan: mapSpan}
	fmt.Printf("map %d × %d cells, %.0f m drawn, %.0f m real\n", t.Width, t.Height, mapSpan, f.size)

	ctx := context.Background()
	margin := search + 100
	latM, lonM := geo.MetresPerDegree(lat)
	half := f.size/2 + margin
	minLat, maxLat := lat-half/latM, lat+half/latM
	minLon, maxLon := lon-half/lonM, lon+half/lonM
	l, err := geo.OpenLidar(ctx, minLat, maxLat, minLon, maxLon)
	if err != nil {
		return 0, 0, nil, err
	}
	for _, s := range l.Sources {
		fmt.Printf("source: %s (published %s, UTM %d)\n", s.Title, s.Published, s.Zone)
	}
	start := time.Now()
	last := time.Now()
	if err := l.Prefetch(ctx, minLat, maxLat, minLon, maxLon, func(done, total int) {
		if time.Since(last) > 2*time.Second || done == total {
			fmt.Printf("  tiles %d/%d\n", done, total)
			last = time.Now()
		}
	}); err != nil {
		return 0, 0, nil, err
	}
	fmt.Printf("fetched in %.1fs\n", time.Since(start).Seconds())

	// Reference raster at 2.5 m in the nominal frame, wide enough to
	// shift the map anywhere within the search.
	const refStep = 2.5
	ref0 := -margin
	refN := int((mapSpan+2*margin)/refStep) + 1
	ref := make([]float32, refN*refN)
	for j := 0; j < refN; j++ {
		for i := 0; i < refN; i++ {
			la, lo := f.latLon(ref0+float64(i)*refStep, ref0+float64(j)*refStep)
			v, err := l.At(ctx, la, lo)
			if err != nil {
				return 0, 0, nil, err
			}
			ref[j*refN+i] = v
		}
	}
	refAt := func(x, z float64) float32 {
		fx, fz := (x-ref0)/refStep, (z-ref0)/refStep
		i, j := int(math.Floor(fx)), int(math.Floor(fz))
		if i < 0 || j < 0 || i >= refN-1 || j >= refN-1 {
			return float32(math.NaN())
		}
		tx, tz := float32(fx-float64(i)), float32(fz-float64(j))
		a := ref[j*refN+i]*(1-tx) + ref[j*refN+i+1]*tx
		b := ref[(j+1)*refN+i]*(1-tx) + ref[(j+1)*refN+i+1]*tx
		return a*(1-tz) + b*tz
	}

	// misfit is the RMS of lidar minus map heights, after removing the
	// mean difference, with the map shifted by (dx, dz) metres and scaled
	// by s about its centre; and that mean.
	type sample struct{ x, z, g float64 }
	var samples []sample
	for x := 0; x < t.Width; x += 6 {
		for z := 0; z < t.Height; z += 6 {
			samples = append(samples, sample{float64(x) * cell, float64(z) * cell, float64(t.Cells[x][z].GroundElevation)})
		}
	}
	misfit := func(dx, dz, s float64) (rms, mean float64, n int) {
		var sum, sum2 float64
		for _, p := range samples {
			x := (p.x-mapSpan/2)*s + mapSpan/2 + dx
			z := (p.z-mapSpan/2)*s + mapSpan/2 + dz
			v := float64(refAt(x, z))
			if math.IsNaN(v) {
				continue
			}
			d := v - p.g
			sum += d
			sum2 += d * d
			n++
		}
		if n < len(samples)/2 {
			return math.Inf(1), 0, n
		}
		mean = sum / float64(n)
		return math.Sqrt(math.Max(sum2/float64(n)-mean*mean, 0)), mean, n
	}

	best := struct{ dx, dz, s, rms float64 }{0, 0, 1, math.Inf(1)}
	try := func(dx, dz, s float64) {
		if r, _, _ := misfit(dx, dz, s); r < best.rms {
			best.dx, best.dz, best.s, best.rms = dx, dz, s, r
		}
	}
	r0, _, _ := misfit(0, 0, 1)
	fmt.Printf("nominal frame: RMS %.2f m\n", r0)
	for dx := -search; dx <= search; dx += 10 {
		for dz := -search; dz <= search; dz += 10 {
			try(dx, dz, 1)
		}
	}
	for _, step := range []float64{2, 0.5} {
		c := best
		for dx := c.dx - 6*step; dx <= c.dx+6*step; dx += step {
			for dz := c.dz - 6*step; dz <= c.dz+6*step; dz += step {
				for s := c.s - 0.004; s <= c.s+0.0041; s += 0.001 {
					try(dx, dz, s)
				}
			}
		}
	}
	rms, mean, n := misfit(best.dx, best.dz, best.s)
	fmt.Printf("aligned: shift %+.1f m east, %+.1f m south, scale %.4f: RMS %.2f m over %d samples, lidar = map %+.1f m\n",
		best.dx, best.dz, best.s, rms, n, mean)

	// Sample at the detail lattice in the aligned frame.
	d := world.NewTerrainDetail(t.Width, t.Height)
	heights := make([]float32, d.W*d.H)
	step := cell / world.DetailPerCell
	missing := 0
	for j := 0; j < d.H; j++ {
		for i := 0; i < d.W; i++ {
			x := (float64(i)*step-mapSpan/2)*best.s + mapSpan/2 + best.dx
			z := (float64(j)*step-mapSpan/2)*best.s + mapSpan/2 + best.dz
			la, lo := f.latLon(x, z)
			v, err := l.At(ctx, la, lo)
			if err != nil {
				return 0, 0, nil, err
			}
			if math.IsNaN(float64(v)) {
				missing++
			}
			heights[j*d.W+i] = v - float32(mean)
		}
	}
	la0, lo0 := f.latLon(mapSpan/2+best.dx, mapSpan/2+best.dz)
	fmt.Printf("map centre is %.6f N, %.6f E; %d × %d heights, %.2f%% missing\n", la0, lo0, d.W, d.H, 100*float64(missing)/float64(len(heights)))
	return d.W, d.H, heights, nil
}
