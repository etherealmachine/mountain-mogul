---
title: Scenario Editor
kind: tooling
status: shipped
---

# Scenario Editor

A separate scene for authoring the scenarios New Game starts from, with no simulation running. It has the same build tools as play (lifts, buildings, roads, terrain brushes), plus:

- [[Parcels]]: drag rectangles to create or edit parcels and set state and price
- automatic [[Snow]] cover from snowline, elevation, slope, curvature, drainage, wind aspect, and treeline
- automatic forest from layered noise and drainage, thinned above treeline and on cliffs, placed as individual spaced trees (see [[Trees]])
- a Layers panel (Layers button) for an imported map: the base import, which reopens the import to change the square (Change...) or imports the same square again in the background with the same grid and layer settings (the ↻ reload button, which asks first when there's anything to lose), then each terrain layer with a checkbox and a strength slider: the ground layers, then Auto trees and Auto snow, which keep what's built ([[Terrain Layers]]). Last is an OpenStreetMap checkbox, visual only and off on opening, that drapes the import's real-world features on the ground with labels, for lining up what's built: lifts in red with a pad at each station and their kind ("quad chair"), runs coloured by difficulty, roads in amber at their paved width, and the ski-area boundary in purple. It's drawn under everything built (roads, parking, buildings, lifts, and placement ghosts all sit on top), though labels draw over everything. Labels skip any that would overlap, lifts first. The ribbons are redraped twice a second, so they follow layer rebuilds and snow. Auto snow follows the start date.
- the scenario's start date, which sets day one of the [[Calendar]]
- a Scenario details dialog (Details button or the escape menu) for the name, location, difficulty, campaign order, tutorial flag, and a multi-line description ([[Scenario Metadata]])

Files open and save in `assets/scenarios/` in the [[Save Format]], with an unsaved-changes marker in the title. New maps come from [[Terrain Import]]. What it makes are [[Scenarios]]; the title shows the display name and the file name.

Code: `internal/scene/editor*.go` (`editor_layers.go` for the Layers panel, `editor_osm.go` for the OpenStreetMap overlay), `world_layers.go`, `snowgen.go`, `forestgen.go`, `autogen.go`.

## Log

- 2026-10-01: Editor scene, parcels, snow and forest generators, start date, and file handling are in.
- 2026-10-02: The forest generator and the plant and glade brushes work on stored trees ([[Stored Trees]]).
- 2026-10-02: Scenario details dialog for names, descriptions, and campaign order.
- 2026-10-06: Layers panel for imported terrain; a new import keeps the scenario's name, details, and start date.
- 2026-10-06: A strength slider on every layer row; timings dropped.
- 2026-10-06: Auto trees and Auto snow layers. A new import now starts on the season's opening day, in the season the editor was set to. The Auto tool's treeline slider starts from the climate.
- 2026-10-06: OpenStreetMap overlay in the Layers panel: lifts, runs, roads, and the boundary draped on the ground with labels.
- 2026-10-06: The OpenStreetMap overlay draws water in cyan (streams and lake outlines, with names); the Layers panel is wider to fit the counts.
- 2026-10-06: A ↻ reload button beside Change... re-imports the base's square in place, for fresher data or what newer imports fetch (water, for older maps).
- 2026-10-06: The Scenario details dialog has a Goals tab for scenario goals and rules ([[Scenario Goals and Rules]]).
