---
title: Scenarios
kind: system
status: partial
---

# Scenarios

A scenario is the starting state for a new game: a save file in `assets/scenarios/` with the [[Terrain]], [[Parcels]], starting cash and credit line ([[Finance]]), start date ([[Calendar]]), and anything already built. New Game lists every file there and starts from the one picked ([[Scenes]]). The [[Scenario Editor]] creates and edits them, and maps usually start from [[Terrain Import]].

The only scenario is `tutorial.save`, the Boreal map.

What a scenario can't do yet:

- **No name or description.** The picker's button label is the file name ("tutorial"). The save format has a `name` field, but nothing writes or reads it.
- **No goals or rules.** A scenario sets where you start, not what you're trying to do or what's different about this mountain.

[[Scenario Metadata]] plans names and descriptions. [[Scenario Goals and Rules]] plans objectives and per-scenario constraints. [[Scenario Campaign]] is the set of scenarios to build.

Code: `internal/scene/scenariopicker.go`, `editor_file.go`; the save format in `internal/save/format.go` ([[Save Format]]).

## Log

- 2026-10-01: One scenario (Boreal, as `tutorial.save`). No names, descriptions, goals, or rules.
