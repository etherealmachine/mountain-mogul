---
title: Terrain Import
kind: tooling
status: shipped
---

# Terrain Import

Builds a new map from a real place. The import scene searches by place name, shows a map preview around the result, and lets the player pick a square. The tool then fetches public elevation tiles (AWS Terrain Tiles at zoom 14), repairs outlier pixels and tile seams, and resamples to 5 m cells. The result becomes the ground of the [[Terrain]], which the [[Scenario Editor]] can then snow, forest, and divide into [[Parcels]].

Code: `internal/geo/`, `internal/scene/terrain_import.go`.

## Log

- 2026-10-01: Place search, preview, square selection, tile fetch, artifact repair, and resampling are in.
