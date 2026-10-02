---
title: Scenario Editor
kind: tooling
status: shipped
---

# Scenario Editor

A separate scene for authoring the scenarios New Game starts from, with no simulation running. It has the same build tools as play (lifts, buildings, roads, terrain brushes), plus:

- [[Parcels]]: drag rectangles to create or edit parcels and set state and price
- automatic [[Snow]] cover from snowline, elevation, slope, curvature, drainage, wind aspect, and treeline
- automatic forest from layered noise and drainage, thinned above treeline and on cliffs (see [[Terrain]])
- the scenario's start date, which sets day one of the [[Calendar]]

Files open and save in `assets/scenarios/` in the [[Save Format]], with an unsaved-changes marker in the title. New maps come from [[Terrain Import]]. What it makes are [[Scenarios]]; the title shows the file name, since scenarios have no display name yet ([[Scenario Metadata]]).

Code: `internal/scene/editor*.go`, `snowgen.go`, `forestgen.go`, `autogen.go`.

## Log

- 2026-10-01: Editor scene, parcels, snow and forest generators, start date, and file handling are in.
