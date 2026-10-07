// redress reruns a save's world layers (materials, trees, snow) the way
// an import does, recomputing the opening day from the snowpack, so a
// scenario picks up changes to the weather or snow model:
//
//	go run ./tools/redress [-strength snow=0.5] in.save out.save
//
// -strength sets a layer's strength (0..1, 0.5 is the default) first.
// The ground isn't rebuilt; trees and snow are replaced.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"mountain-mogul/internal/save"
	"mountain-mogul/internal/scene"
)

func main() {
	strength := flag.String("strength", "", "layer strength to set first, as id=value (e.g. snow=0.5)")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s [-strength id=value] <in.save> <out.save>\n", os.Args[0])
		os.Exit(2)
	}
	w, cam, err := save.LoadScenario(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if w.TerrainBase == nil {
		fmt.Fprintln(os.Stderr, "no terrain base: not an imported scenario")
		os.Exit(1)
	}
	if *strength != "" {
		id, val, ok := strings.Cut(*strength, "=")
		v, err := strconv.ParseFloat(val, 32)
		if !ok || err != nil {
			fmt.Fprintln(os.Stderr, "-strength wants id=value")
			os.Exit(2)
		}
		w.TerrainBase.SetStrength(id, float32(v))
	}
	scene.DressWorld(w, func(name string) { fmt.Println("running", name) })
	if err := save.SaveScenario(flag.Arg(1), w, cam); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("start date %s → %s\n", w.StartDate.Format("2 January 2006"), flag.Arg(1))
}
