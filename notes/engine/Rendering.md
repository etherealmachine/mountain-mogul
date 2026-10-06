---
title: Rendering
kind: engine
status: shipped
---

# Rendering

The renderer reads the world and never writes it. It draws in passes: terrain, static instanced batches (buildings, [[Trees]], lift towers, [[Lodge Shell]] tiles), dynamic objects (skiers, chairs, snowcats, cars), a full-screen precipitation overlay, and the UI. The camera is a 45° isometric orthographic view, with a first-person option for following a guest.

The terrain mesh has a control point at every cell corner, split into 32 × 32-cell chunks that are skipped when off screen. The tessellation stage subdivides each triangle edge by its on-screen size (up to 1.25 m, or 0.625 m where there's detail), lifts it by snow depth and powder and mogul bumps, and adds sub-cell height from a 1.25 m detail texture ([[Terrain Realism]]). `VisualElevationAt` reproduces the same surface on the CPU. Per-corner [[Snow]] state (grooming, packed and icy tints, moguls, depth, instability) lives in two small textures the vertex shader reads, so snow changes don't touch the vertex buffer. A 25 cm detail texture adds ski tracks and tree wells. A 1 m RGBA16 groom texture records where cats drove; the shader turns it into corduroy ridges lit as a normal bump, lane seams, and the groomed edge ([[Grooming]]). Overlays toggle contour lines, slope, snow depth, grooming, and the like. [[Trails]] draw as colored areas with name labels.

Light follows the [[Calendar]] sun with moonlight at night. Fill light comes from the sky above (blue on clear days, grey under cloud) and bounces off the snow below, so shade on snow reads light blue. Exposure lifts a low winter sun, and a shared tone curve in `lighting.glsl` rolls bright snow off toward white. Baked ambient occlusion darkens valleys and cliff bases (measured against the local slope, so even slopes stay lit) and shades only the fill. Haze thickens with distance and pools in low valleys. Terrain shadows come from per-cell horizon angles, the same data that drives melt. Objects cast shadows through a 4096² shadow map. Up to 16 spotlights light the snow at night. Sky color and the snow or rain overlay follow [[Weather]].

Snow has one albedo with broad patchy variation and gentle wind drifts in the shading; sparkle comes from tiny randomly tilted facets that glint when they mirror the sun into the camera. Trees are lit as foliage: soft light through the canopy, per-tree color variation, darker low branches, and snow on the boughs during snowfall. The map edge shows a cross-section of banded rock. Edges are 4× multisampled, with an off switch in Settings.

Meshes come from the [[Model Pipeline]].

Gameplay runs on 5 m cells, and the parcel fence and painted overlays draw those cells directly, so the grid shows. [[Hiding the Grid]] plans smoothing each from the cell data.

Code: `internal/render/`, shaders in `assets/shaders/`.

Cliffs, creeks, and finer terrain are planned in [[Terrain Realism]]. Gamma-correct lighting and a post-processing pass are left over from [[Graphics Base]].

## Log

- 2026-10-01: Passes, snow shading, detail texture, overlays, sun and horizon shadows, object shadow map, night lamps, and weather overlay are in.
- 2026-10-05: [[Graphics Base]]: anti-aliasing with a setting, sky and bounce fill, exposure and tone curve, AO fix, uniform snow albedo with patches and drifts, facet sparkle, foliage shading and bough snow, haze, rock map edge.
- 2026-10-05: Fresh snow after a storm cost about 4 ms of GPU time zoomed out. The terrain shader's normal kicks (powder, drifts, moguls, debris) now use an analytic noise gradient instead of three samples, and powder, moguls, and sparkle fade out once they're smaller than a pixel. Zoomed-out Boreal after a storm: 10.6 → 7.2 ms.
- 2026-10-05: [[Real Grooming]]: groom texture and lit corduroy ridges; retired the surface-detail groom-edge channel.
- 2026-10-05: Chunked terrain with screen-size subdivision, snow state in textures, and a detail height texture ([[Terrain Realism]] step 1). Kirkwood: 16.1 → 6.5 ms GPU zoomed out, CPU render 7.8 → 0.9 ms. Fixed snow drawn at double depth after snow changes.
- 2026-10-06: Bare ground is coloured from the material map ([[Ground Materials]]), and a Ground overlay shows it.
- 2026-10-06: One procedural rock texture on rock ground, modelled on Kirkwood's volcanic breccia and built in 3D so walls don't stretch ([[Ground Materials]]).
- 2026-10-06: Snow draws on a cell's ground and ledges, not its rock, and doesn't raise rock by its depth ([[Ground Materials]]).
- 2026-10-06: Frozen lakes: flat snow with wind-scoured grey-blue ice where it's thin ([[Creeks and Lakes]]).
- 2026-10-06: Lake surfaces: open water reflects the sky and glints, thin ice is dark with patchy snow, and frozen lakes are snowy ([[Creeks and Lakes]]).
