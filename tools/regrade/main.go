// regrade re-carves every lift station's apron and every painted pad's
// embankment in a save, the same as the debug console's "regrade", so
// worlds saved before a grading change pick it up:
//
//	go run ./tools/regrade in.save out.save
//
// Only the ground changes; everything else round-trips as it was, so it
// works on bundled scenarios and on game saves alike.
package main

import (
	"flag"
	"fmt"
	"os"

	"mountain-mogul/internal/save"
	"mountain-mogul/internal/scene"
)

func main() {
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <in.save> <out.save>\n", os.Args[0])
		os.Exit(2)
	}
	in, out := flag.Arg(0), flag.Arg(1)
	w, cam, err := save.LoadScenario(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	scene.RegradeEmbankments(w)
	if err := save.SaveScenario(out, w, cam); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("regraded %d lifts → %s\n", len(w.Lifts), out)
}
