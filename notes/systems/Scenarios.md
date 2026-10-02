---
title: Scenarios
kind: system
status: partial
---

# Scenarios

A scenario is the starting state for a new game: a save file in `assets/scenarios/` with the [[Terrain]], [[Parcels]], starting cash and credit line ([[Finance]]), start date ([[Calendar]]), and anything already built. New Game lists every file there and starts from the one picked ([[Scenes]]). The [[Scenario Editor]] creates and edits them, and maps usually start from [[Terrain Import]].

Each scenario also carries a display name, location, difficulty (1 to 5), campaign order, tutorial flag, and description, set in the editor's Scenario details dialog ([[Scenario Metadata]]). The picker lists scenarios by name, tutorial first and then in campaign order, and a pick opens a panel with the description before Play. Games started from a scenario keep its info, so the save list shows which mountain each save is on.

The only scenario is [[Boreal]] (`tutorial.save`).

What a scenario can't do yet: set goals or rules. A scenario sets where you start, not what you're trying to do or what's different about this mountain.

[[Scenario Goals and Rules]] plans objectives and per-scenario constraints. [[Scenario Campaign]] is the set of scenarios to build.

Code: `internal/scene/scenariopicker.go`, `editor_file.go`, `editor_details.go`; the info in `internal/world/scenario_info.go`; the save format in `internal/save/format.go` ([[Save Format]]).

## Log

- 2026-10-01: One scenario (Boreal, as `tutorial.save`). No names, descriptions, goals, or rules.
- 2026-10-02: Scenarios have names, descriptions, locations, difficulty, and campaign order, shown in the picker and set in the editor.
