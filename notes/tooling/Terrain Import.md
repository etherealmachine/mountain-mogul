---
title: Terrain Import
kind: tooling
status: shipped
---

# Terrain Import

Builds a new map from a real place. The import scene searches by place name, shows a map preview around the result, and lets the player pick a square. The tool then fetches public elevation tiles (AWS Terrain Tiles at zoom 14, about 7.5 m per pixel at Tahoe) and repairs outlier pixels and tile seams. The result becomes the ground of the [[Terrain]], which the [[Scenario Editor]] can then snow, forest, and divide into [[Parcels]].

Where USGS 3DEP 1 m lidar covers the square (most of the US), the import uses it instead: it reads only the parts of the survey files it needs, samples the 1.25 m detail lattice directly, and averages it over each 5 m cell, so the sim's slopes carry the cliffs and creek beds too. Gaps in the lidar fall back to the tiles, fading over about 20 m so the seam doesn't show. Without lidar (outside the US, or if the download fails) the import works as before. The editor shows which it used. The square's latitude and longitude bounds are saved with the scenario (`geo`), so later passes line up with real data without searching.

The map preview draws OpenStreetMap's ski data over the satellite imagery so the square can be framed around the real resort: lifts in red with their names, runs coloured by difficulty (green, blue, black, wider black for double black), and the resort boundary in white. A line in the corner counts the lifts and runs inside the square. O hides or shows the overlay. It loads in the background for the visible area plus a margin (at most 0.35° a side, from preview zoom 11), and again when the view leaves what's been loaded. Overpass is asked first because it filters by tag on the server (about 85 KB for Kirkwood). When it fails or is busy, the main OSM API is used instead, but it returns every feature in the box, so the fallback only covers 0.08° around the centre. The screen credits OpenStreetMap, as its licence (ODbL) requires. The overlay is only a guide for now; nothing from it goes into the save.

A 3 km square at Kirkwood takes about a minute to a minute and a half, mostly lidar download, and peaks around 400 MB of memory; a 6.4 km square would be roughly four times both.

Code: `internal/geo/` (`ImportTerrain`), `internal/scene/terrain_import.go`.

## Log

- 2026-10-01: Place search, preview, square selection, tile fetch, artifact repair, and resampling are in.
- 2026-10-05: Search also asks for "<name> resort" and lists ski areas first, since OpenStreetMap often only knows a resort by its full name ("boreal" alone found only suburbs abroad). One result goes straight to the map; no results says so. Back button and Esc step back a screen.
- 2026-10-05: Lidar where it exists, for both the cells and the 1.25 m detail, with tile fallback and feathered seams. The import's bounds are saved. Kirkwood: 100% coverage from 4 survey tiles, 70 s.
- 2026-10-05: OpenStreetMap lifts, runs, and resort boundaries drawn over the map preview, with a count for the square. Kirkwood: 13 lifts and 63 runs in under 2 s from either source.
