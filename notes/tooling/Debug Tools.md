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

The tilde console has cheats: more or less money, more snow, a heatwave, a forced [[Avalanche]], and `regrade` to rebuild embankments on old saves.

`-screenshot` also prints a benchmark: wall-clock frame time and the median GPU render time from a timer query, which isn't thrown off by vsync or the sim.

From the command line: `-trace` serves pprof and logs slow frames, `-cpuprofile` and `-memprofile` write profiles on exit, `-profile` runs a headless benchmark, and `-screenshot` renders one frame with chosen overlays. [[Sim Queries]] can inspect a live game with SQL.

Spec: [[Debug Spec]]. Console: `internal/scene/debugconsole.go`.

## Log

- 2026-10-01: Overlays, the planner panel, the cell inspector, the CSV log, screenshots, the console, and profiling flags are in.
- 2026-10-05: `-screenshot` reports median GPU render time.
