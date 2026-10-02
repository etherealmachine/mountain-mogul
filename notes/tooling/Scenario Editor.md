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
- the scenario's start date, which sets day one of the [[Calendar]]
- a Scenario details dialog (Details button or the escape menu) for the name, location, difficulty, campaign order, tutorial flag, and a multi-line description ([[Scenario Metadata]])

Files open and save in `assets/scenarios/` in the [[Save Format]], with an unsaved-changes marker in the title. New maps come from [[Terrain Import]]. What it makes are [[Scenarios]]; the title shows the display name and the file name.

Code: `internal/scene/editor*.go`, `snowgen.go`, `forestgen.go`, `autogen.go`.

## Log

- 2026-10-01: Editor scene, parcels, snow and forest generators, start date, and file handling are in.
- 2026-10-02: The forest generator and the plant and glade brushes work on stored trees ([[Stored Trees]]).
- 2026-10-02: Scenario details dialog for names, descriptions, and campaign order.
