---
title: Rendering
kind: engine
status: shipped
---

# Rendering

The renderer reads the world and never writes it. It draws in passes: terrain, static instanced batches (buildings, [[Trees]], lift towers, [[Lodge Shell]] tiles), dynamic objects (skiers, chairs, snowcats, cars), a full-screen precipitation overlay, and the UI. The camera is a 45° isometric orthographic view, with a first-person option for following a guest.

Terrain carries the [[Snow]] state per vertex: grooming, packed and icy tints, mogul shading. A 25 cm detail texture adds ski tracks and tree wells. A 1 m RGBA16 groom texture records where cats drove; the shader turns it into corduroy ridges lit as a normal bump, lane seams, and the groomed edge ([[Grooming]]). Overlays toggle contour lines, slope, snow depth, grooming, and the like. [[Trails]] draw as colored areas with name labels.

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
