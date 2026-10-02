---
title: Rendering
kind: engine
status: shipped
---

# Rendering

The renderer reads the world and never writes it. It draws in passes: terrain, static instanced batches (buildings, [[Trees]], lift towers, [[Lodge Shell]] tiles), dynamic objects (skiers, chairs, snowcats, cars), a full-screen precipitation overlay, and the UI. The camera is a 45° isometric orthographic view, with a first-person option for following a guest.

Terrain carries the [[Snow]] state per vertex: corduroy, packed and icy tints, mogul shading. A 1 m detail texture adds ski tracks, tree wells, and sharp groom edges. Overlays toggle contour lines, slope, snow depth, grooming, and the like. [[Trails]] draw as colored areas with name labels.

Light follows the [[Calendar]] sun with moonlight at night. Terrain shadows come from per-cell horizon angles, the same data that drives melt. Objects cast shadows through a 4096² shadow map. Up to 16 spotlights light the snow at night. Sky color and the snow or rain overlay follow [[Weather]].

Meshes come from the [[Model Pipeline]].

Gameplay runs on 5 m cells, and the parcel fence, painted overlays, and grooming draw those cells directly, so the grid shows. [[Hiding the Grid]] plans smoothing each from the cell data.

Code: `internal/render/`, shaders in `assets/shaders/`.

## Log

- 2026-10-01: Passes, snow shading, detail texture, overlays, sun and horizon shadows, object shadow map, night lamps, and weather overlay are in.
