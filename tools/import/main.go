// import runs the editor's terrain import without the editor and writes
// a scenario save with nothing built:
//
//	go run ./tools/import -lat 39.336 -lon -120.348 -cells 512 -out /tmp/boreal.save
//
// The square is centred on -lat/-lon and -cells × 5 m on a side, sized
// like the import screen's square. Every terrain layer runs except those
// named in -off (e.g. -off roads,erode); the save keeps the base, so the
// editor's Layers panel can switch them afterwards. Capture results with
// -screenshot -load.
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"mountain-mogul/internal/geo"
	"mountain-mogul/internal/save"
	"mountain-mogul/internal/scene"
	"mountain-mogul/internal/world"
)

func main() {
	lat := flag.Float64("lat", math.NaN(), "square centre latitude")
	lon := flag.Float64("lon", math.NaN(), "square centre longitude")
	cells := flag.Int("cells", 512, "cells on a side (5 m each)")
	out := flag.String("out", "", "save file to write")
	var ids []string
	for _, l := range geo.Layers {
		ids = append(ids, l.ID)
	}
	ids = append(ids, scene.WorldLayerIDs()...)
	offFlag := flag.String("off", "", "comma-separated terrain layers to switch off: "+strings.Join(ids, ", "))
	flag.Parse()
	if math.IsNaN(*lat) || math.IsNaN(*lon) || *out == "" || *cells < 2 {
		flag.Usage()
		os.Exit(2)
	}
	var off []string
	if *offFlag != "" {
		off = strings.Split(*offFlag, ",")
	}

	half := float64(*cells) * world.CellSize / 2
	latM, lonM := geo.MetresPerDegree(*lat)
	dLat, dLon := half/latM, half/lonM
	b := geo.Bounds{MinLat: *lat - dLat, MaxLat: *lat + dLat, MinLon: *lon - dLon, MaxLon: *lon + dLon}

	start := time.Now()
	last := ""
	stage := func(s string) {
		if s != last {
			fmt.Printf("%6.1fs %s\n", time.Since(start).Seconds(), s)
			last = s
		}
	}
	res, err := geo.ImportTerrain(context.Background(), b, *cells, *cells, world.DetailPerCell,
		func(s string, _ float32) { stage(s) })
	if err != nil {
		fmt.Fprintln(os.Stderr, "import:", err)
		os.Exit(1)
	}
	fmt.Printf("%6.1fs fetched: lidar %.0f%% %s, %d roads %s, climate %s\n", time.Since(start).Seconds(),
		100*res.LidarCoverage, res.LidarNote, len(res.Roads), res.RoadNote, res.ClimateNote)
	w, _, err := geo.BuildWorld(res, off, nil, stage)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import:", err)
		os.Exit(1)
	}
	took := scene.DressWorld(w, stage)
	for id, d := range took {
		fmt.Printf("        %s took %v\n", id, d.Round(time.Millisecond))
	}
	fmt.Printf("        starts %s\n", w.StartDate.Format("Jan 2 2006"))
	if err := save.SaveScenario(*out, w, nil); err != nil {
		fmt.Fprintln(os.Stderr, "import:", err)
		os.Exit(1)
	}
	fmt.Printf("%6.1fs wrote %s\n", time.Since(start).Seconds(), *out)
}
