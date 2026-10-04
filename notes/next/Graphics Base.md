---
title: Graphics Base
kind: plan
status: partial
---

# Graphics Base

Cheap fixes that lift every scene before the bigger [[Terrain Realism]] work: a solid visual baseline to build on. Each step is small and mostly in the shaders. Covers [[Rendering]]. Listed in [[Next Steps]].

## Where it started

From screenshots of [[Boreal]], [[Kirkwood]], and the snowcat testbed at 11 AM:

- No anti-aliasing: jagged tree edges, cables, and outlines.
- Shade on snow was dark navy (about 15% brightness): a flat grey fill multiplied by a strong blue tint. Kirkwood's long winter tree shadows turned whole slopes dark.
- Sunlit snow was grey, not white: no exposure or tone curve.
- Baked ambient occlusion measured blockers against the horizontal, so every slope was darkened just for being tilted (a 17° slope lost about 16%), and it dimmed direct sun as well as fill. This flattened the whole mountain.
- Snow color followed elevation, from a cool tint at the bottom of the map to white at the top.
- Sparkle flickered on a timer.
- Trees were flat-lit cones.
- No sense of distance; the map edge was a black wall.

## Done

1. **Anti-aliasing.** 4× multisampling. Costs about 4 ms of GPU time where trees fill the screen (Boreal: 3.6 → 8.1 ms at 2560 × 1440), so Settings has an Anti-aliasing On/Off switch (`settings.NoAntiAliasing`), applied live.
2. **Light balance.**
   - Fill is a sky light from above (blue on clear days, grey under cloud) and bounce from the snow below (`Lighting.Ground`, `fillLight` in `lighting.glsl`).
   - The terrain shader's shade tint is much milder, so it doesn't stack with the blue sky fill.
   - Exposure lifts a low sun toward a target, lower under cloud so overcast stays a little grey. A shared tone curve (`toneMap`) leaves colors below 0.85 alone and rolls brighter ones off toward white; terrain, objects, and skiers all use it.
   - Ambient occlusion is measured against the local slope plane and shades only the fill.
   - The terrain's soft terminator is tighter, so slope angle reads in the wide view.
3. **Snow surface breakup.** One snow albedo instead of the elevation band, with broad patches (sheltered a touch warmer, scoured a touch cooler) and gentle wind drifts in the shading. Sparkle comes from 25 cm facets tilted at random that glint only when they mirror the sun into the camera, so they twinkle when the view moves.
4. **Trees with depth.** Foliage mode in the static shader for the three conifer meshes: soft light through the canopy, per-tree brightness and hue variation, darker low branches, lighter tips. Snow on the boughs in soft bands during snowfall (`CanopySnowFor`: full in heavy snow, partial in light).
5. **Haze.** Thickens with distance past the camera's focus and pools in the lowest third of the map's relief (`applyHaze`).
6. **Map edge.** The skirt walls show banded rock, darker with depth, instead of black.

The shader changes together cost about 0.4 ms of GPU time on Boreal and nothing measurable on Kirkwood.

## Left

- **Bough snow that lingers.** The weather sim only knows today, so snow vanishes from the trees the day a storm ends. Needs a small sim value for recent snowfall that sun, warmth, and wind wear down ([[Weather]]).
- **Gamma-correct lighting.** Shifts every palette, so it's its own pass with a retune of colors.
- **Post-processing.** Bloom on sparkle and lit windows, gentle color grading, a vignette. Needs an offscreen framebuffer.

## Checking

Take before and after screenshots with the same flags. `-screenshot` prints the median GPU render time from a timer query, which is the number to compare; wall-clock frame time is swamped by vsync and the sim.

- `-load assets/scenarios/tutorial.save -camera-zoom 150 -clock-hour 11`
- `-load assets/scenarios/kirkwood.save -camera-yaw 180 -camera-zoom 250 -camera-target-x 1800 -camera-target-z 800 -clock-hour 11`
- `-load assets/scenarios/kirkwood.save -camera-yaw 180 -camera-zoom 60 -camera-target-x 1720 -camera-target-z 620 -clock-hour 11` (close-up of trees and the lot)
- `-load assets/scenarios/kirkwood.save -camera-yaw 180 -camera-zoom 700 -camera-pitch 30 -camera-target-x 1500 -camera-target-z 1500 -clock-hour 11` (wide, for relief)
- `-testbed "Snowcat U-shaped" -clock-hour 11` (overcast, map edge)

Add `-clock-hour 8`, `16.3`, `17.2`, and `21` for dawn, dusk, twilight, and night.

## Log

- 2026-10-05: Planned from screenshots of Boreal, Kirkwood, and the snowcat testbed, after the user asked for easy graphical wins as a base for later work.
- 2026-10-05: Shipped steps 1–6, plus the ambient occlusion fix found along the way. Added GPU timing to `-screenshot`. Lingering bough snow, gamma-correct lighting, and post-processing are left.
