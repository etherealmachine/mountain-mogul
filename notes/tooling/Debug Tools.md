---
title: Debug Tools
kind: tooling
status: shipped
---

# Debug Tools

In-game, after clicking a skier to follow them:

- F3 draws the [[Skiing]] controller's fall line, chosen heading, and probe rays.
- F4 shows [[GOAP]] goal weights, the current plan with step costs, and the snapshot it planned from.
- F5 inspects the [[Snow]] and trees in the cell under the cursor, plus frame timings.
- L records the followed skier to a CSV in `debug/`, one row per skiing tick.
- F12 saves a screenshot.

The tilde console has cheats: more or less money, more snow, a heatwave, a forced [[Avalanche]], `groom` to give every snowcat section a full pass now, and `regrade` to rebuild embankments on old saves.

`-screenshot` also prints a benchmark: wall-clock frame time, the median GPU render time from a timer query (which isn't thrown off by vsync or the sim), and median and worst CPU time for the sim update and the render submission. `-storm` drops a heavy-snow day on the terrain and makes today a heavy-snow day first, for checking how fresh snow renders and performs. `-groom-now` grooms every snowcat section before the capture, for checking corduroy. `-detail-test` replaces terrain detail with rolling bumps and a 6 m step across the map's middle, for checking mesh seams; the benchmark also reports how many terrain chunks were drawn. `-detail-file PATH` draws the ground from a heights file made by `go run ./tools/lidar -save <scenario.save> -lat <lat> -lon <lon> -out PATH`, which fetches USGS 1 m lidar around the given map centre and aligns it to the map. `-write-save PATH` bakes the heights into a copy of the save instead (`-from` reuses a heights file rather than fetching again). `go run ./tools/import -lat <lat> -lon <lon> -cells 512 -out PATH` runs the editor's terrain import without the editor and writes a save with nothing built (ground, trees, snow, the opening-day start date, and the base for the editor's Layers panel); `-off roads,smooth,erode,trees,snow` switches terrain layers off. `-editor-layers LIST` opens `-load` in the scenario editor with the Layers panel open, switches off the comma-separated layers in LIST (`-` for none), and captures once the ground has rebuilt, for comparing layers. `-import-preview lat,lon,zoom` captures the terrain import's map screen there instead of a scenario, once the tiles and the OpenStreetMap overlay have loaded (e.g. `-screenshot /tmp/kirk.png -import-preview 38.678,-120.068,14`).

From the command line: `-trace` serves pprof and logs slow frames, `-cpuprofile` and `-memprofile` write profiles on exit, `-profile` runs a headless benchmark, and `-screenshot` renders one frame with chosen overlays. [[Sim Queries]] can inspect a live game with SQL.

Spec: [[Debug Spec]]. Console: `internal/scene/debugconsole.go`.

## Log

- 2026-10-01: Overlays, the planner panel, the cell inspector, the CSV log, screenshots, the console, and profiling flags are in.
- 2026-10-05: `-screenshot` reports median GPU render time.
- 2026-10-05: `-screenshot` reports CPU update and render time; added `-storm`.
- 2026-10-05: Added `-groom-now` and the console `groom` command.
- 2026-10-05: Added `-detail-test` and the terrain chunk count.
- 2026-10-05: Added `tools/lidar` and `-detail-file`.
- 2026-10-05: Added `-import-preview`.
- 2026-10-05: Added `tools/import`.
- 2026-10-06: `tools/import -off` replaces its per-pass flags; added `-editor-layers`.
- 2026-10-06: `-camera-target-x/z` put the target on the ground, so close-ups centre where asked (it sat at height 0 before).
