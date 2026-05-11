// convertsave — one-shot tool to migrate legacy JSON scenarios into
// the project's binary .save format (msgpack + gzip). Kept around
// after the migration so anyone with an old hand-edited tutorial.json
// or test fixture can run it through the same pipeline:
//
//	go run ./tools/convertsave in.json out.save
//
// The runtime save package only reads and writes .save; this tool is
// the only place that still touches JSON, so the runtime stays
// greenfield-binary.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"mountain-mogul/internal/save"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <in.json> <out.save>\n", os.Args[0])
		os.Exit(2)
	}
	in, out := os.Args[1], os.Args[2]

	raw, err := os.ReadFile(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var data save.ScenarioData
	if err := json.Unmarshal(raw, &data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := save.WriteScenarioData(out, data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	inSize, _ := os.Stat(in)
	outSize, _ := os.Stat(out)
	fmt.Printf("converted %s (%d bytes) → %s (%d bytes)\n",
		in, inSize.Size(), out, outSize.Size())
}
