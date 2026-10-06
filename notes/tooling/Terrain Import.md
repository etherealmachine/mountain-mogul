---
title: Terrain Import
kind: tooling
status: shipped
---

# Terrain Import

Builds a new map from a real place. The import scene searches by place name, shows a map preview around the result, and lets the player pick a square. The tool then fetches public elevation tiles (AWS Terrain Tiles at zoom 14, about 7.5 m per pixel at Tahoe) and repairs outlier pixels and tile seams. The result becomes the ground of the [[Terrain]], which the [[Scenario Editor]] can then snow, forest, and divide into [[Parcels]].

Where USGS 3DEP 1 m lidar covers the square (most of the US), the import uses it instead: it reads only the parts of the survey files it needs, samples the 1.25 m detail lattice directly, and averages it over each 5 m cell, so the sim's slopes carry the cliffs and creek beds too. Gaps in the lidar fall back to the tiles, fading over about 20 m so the seam doesn't show. Without lidar (outside the US, or if the download fails) the import works as before. The editor shows which it used. The square's latitude and longitude bounds are saved with the scenario (`geo`), so later passes line up with real data without searching.

The map preview draws OpenStreetMap's ski data over the satellite imagery so the square can be framed around the real resort: lifts in red with their names, runs coloured by difficulty (green, blue, black, wider black for double black), and the resort boundary in white. A line in the corner counts the lifts and runs inside the square. O hides or shows the overlay. It loads in the background for the visible area plus a margin (at most 0.35° a side, from preview zoom 11), and again when the view leaves what's been loaded. Overpass is asked first because it filters by tag on the server (about 85 KB for Kirkwood). When it fails or is busy, the main OSM API is used instead, but it returns every feature in the box, so the fallback only covers 0.08° around the centre. The screen credits OpenStreetMap, as its licence (ODbL) requires. The overlay is only a guide for now; nothing from it goes into the save.

The import also fetches the OpenStreetMap roads in the square, then hands the surveyed ground to the editor, which keeps it as the map's base and runs the terrain layers on it, all on to start. The [[Terrain Layers]] panel switches each one afterwards and reopens this screen on the same square to change it. There are three ground layers, then Auto trees and Auto snow. With Auto snow on and a climate, the scenario starts on the season's opening day.

"Smooth roads" removes road scars from the ground. Lidar shows highway cuts, embankments, and the flat strip of pavement as sharp lines; at Boreal, I-80's cuts drew long parallel streaks across the map. The import fetches OpenStreetMap roads (motorways down to service roads, not tracks, paths, parking aisles, or driveways) and, along each one, rebuilds the ground as a smooth surface spanning from the real ground either side, then adds back bumps as rough as the ground around it so the corridor doesn't read as a blank strip. The corridor covers the road and its banks: about 1.5 times the road's width beyond the pavement, growing over steep ground that touches it (built banks are steeper than about 27°) for up to 20 m more, closing gaps between carriageways and ramps, and taking in islands up to 1.5 ha between them; it fades out over 12 m. Bridges are smoothed like roads, since the fill follows the valley sides; tunnels are skipped. Boreal: 26 roads, changes up to 13 m, about 3.5 s.

Two more layers clean up what the lidar sees that a resort would rather build itself: run grading, cat tracks, and other small earthworks. Both need lidar, so they do nothing on tile-only imports, and both run after road smoothing. "Smooth ground" blurs out features narrower than about 8 m (three 2.5 m box blurs) but fades back to the raw lidar on slopes from 35° to 45°, so rock faces keep their edges. "Erode" then runs droplet hydraulic erosion: one simulated drop per 1.25 m sample rolls downhill for up to 48 steps, picking up ground as it speeds up and dropping it as it slows or fills a pit, so shared flow lines cut rills and small gullies. It runs on 256-sample tiles in a four-colour checkerboard across all cores, with a seed from the bounds, so the same square always gives the same ground. Boreal: mean change 12 cm, deepest cut 5 m; 0.1 s to smooth and 1.4 s to erode.

A 3 km square at Kirkwood takes about a minute to a minute and a half, mostly lidar download, and peaks around 400 MB of memory; a 6.4 km square would be roughly four times both.

Code: `internal/geo/` (`ImportTerrain`, `SmoothRoads` in `roadsmooth.go`, `SmoothGround`, `Erode` in `erosion.go`, `Layers` and `LayerStack` in `layers.go`, `BuildWorld`), `internal/scene/terrain_import.go`. `tools/import` runs an import without the editor ([[Debug Tools]]).

## Log

- 2026-10-01: Place search, preview, square selection, tile fetch, artifact repair, and resampling are in.
- 2026-10-05: Search also asks for "<name> resort" and lists ski areas first, since OpenStreetMap often only knows a resort by its full name ("boreal" alone found only suburbs abroad). One result goes straight to the map; no results says so. Back button and Esc step back a screen.
- 2026-10-05: Lidar where it exists, for both the cells and the 1.25 m detail, with tile fallback and feathered seams. The import's bounds are saved. Kirkwood: 100% coverage from 4 survey tiles, 70 s.
- 2026-10-05: OpenStreetMap lifts, runs, and resort boundaries drawn over the map preview, with a count for the square. Kirkwood: 13 lifts and 63 runs in under 2 s from either source.
- 2026-10-05: "Auto-smooth roads" at import, from OpenStreetMap roads. Boreal's I-80 cuts and interchange are gone in both the slope overlay and fresh snow.
- 2026-10-05: "Smooth ground" and "Erode" at import. Boreal's cat-track benches are gone from the slope overlay, and the tubing lanes have mostly faded under snow.
- 2026-10-06: The checkboxes became terrain layers in the editor ([[Terrain Layers]]); the import keeps the surveyed ground as the base. Reopening it starts on the saved square.
