---
title: Terrain Layers
kind: plan
status: in progress
---

# Terrain Layers

Terrain passes run as layers in a Layers panel in the [[Scenario Editor]] instead of being chosen once on the import screen ([[Terrain Import]]). The panel shows the base import, then each layer on top of it. Unchecking a layer shows the map without it, and clicking the base import goes back to change the square. Listed in [[Next Steps]], part of [[Terrain Realism]].

## The panel

- **Base import** at the top: the square's size and lidar coverage. "Change..." reopens the import screen on the saved square and grid size, keeping which layers are off. Importing again replaces the whole map. It asks first when there's anything on the map, and cancelling the import keeps the old one. The scenario's name, details, and start date carry over. On a drawn map the button is "Import..." instead.
- **One row per layer**, in a fixed order (never reorderable): the ground layers Smooth roads, Smooth ground, Erode, then the world layers Auto trees and Auto snow. Each row has a checkbox, its name, a strength slider (0–100%, a tick at the default 50%), and "running" while it runs. A ground layer that does nothing here shows why ("needs lidar", "no roads mapped here") in place of its slider; a world layer without a climate notes "no climate". Checking or unchecking a ground layer rebuilds the ground in the background from the base through every checked layer, then reruns the world layers that are on. The editor stays usable while it runs. Checking a world layer runs it, and unchecking it clears all trees or all snow.
- **Strength**: releasing a slider reruns that layer and the ones after it, like switching it, and asks first when a ground layer would clear what's built. A layer that's off keeps its new strength for when it's switched on. 50% is the standard pass, 0% does nothing (the layer is skipped), and 100% goes as far past standard as the layer usefully can.
- A new import starts with every layer on.
- **OpenStreetMap**, the last row, is a view, not a layer: a checkbox (off when the editor opens, not saved) that draws the base's lifts, runs, roads, and boundary over the ground with labels, with a count of each beside it. It never changes the world. See [[Scenario Editor]].

## Rules

- **Each layer decides what survives it.** There's nothing to track: a layer does what a player would expect from it. A layer that moves the ground clears every lift, building, and road (snowcats and patrollers go with their buildings), after asking. It doesn't try to check whether the ground under them actually moved. Trails, trees, snow, and parcels stay. Hand edits shouldn't survive a layer that reshapes the ground, just as they wouldn't survive picking a different place to import.
- **Layers that don't change the ground keep things.** Auto Trees and Auto Snow rerun without clearing what's built, then clear trees and snow from under buildings, parking lots, lift lines, and roads as the Auto tool does. They replace all trees or all snow when they run, so painted trees and snow don't survive them. A ground rebuild leaves them alone when they're off.

## How it works

- **The base is saved in the scenario** (`world.TerrainBase`, `terrain_base` in the [[Save Format]]): the surveyed heights on the 1.25 m lattice (or the cells without lidar), stored as whole centimetres. That's about 3.5 MB at Boreal, doubling the file to 7 MB. It also keeps the bounds, the OpenStreetMap roads, the lidar coverage, and the list of switched-off layers, so new layers start on. Player games drop the base on load.
- **Layers** (`geo.Layers`) each change the heights in place, with a fixed seed so results repeat. A `geo.LayerStack` keeps each layer's output, so switching a layer reruns only it and the layers after it. Switching off the last layer is instant. `geo.ApplyHeights` then sets the cells (the lattice averaged over each cell) and the detail from the result.
- If the switches or strengths change while a rebuild runs, another starts when it finishes. A rebuild from a different map is dropped.
- **Strengths are saved with the base** (`TerrainBase.Strengths`, `strength` in `terrain_base`), only where they aren't the default, and carry over when the import is reopened. `world.LayerScale` turns a strength into a multiplier of the standard pass, linear from 0% to 50% and from 50% to 100%:
  - Smooth roads: the bank allowance and feather from 0.5× to 2×, the bumps put back over the fill from 1× down to 0.25×, and below 50% the ground moves only partway to the fill.
  - Smooth ground: the blur radius from 0.5× to 3× (2.5 m standard, 7.5 m at 100%), and below 50% only partway to the blur. Steep rock is still spared.
  - Erode: drops per sample from 0 to 3×, and how much each carries from 0.6× to 2×. At 100% Boreal's erosion takes about three times as long.
  - Auto trees: coverage from 0 through 55% to 95%.
  - Auto snow: the whole snowpack (base and powder) from 0 to 2.5×, a big year. The opening day on import is figured from the scaled pack, so a stronger season opens earlier; moving the slider afterwards doesn't move the start date.
- Boreal, 512 cells, at 50%: roads 3.6 s, smoothing 0.08 s, erosion 1.4 s. The first switch after loading a file runs every checked layer once, since outputs aren't saved.
- **World layers** (`scene.worldLayers`, `world_layers.go`) run on the cells after the ground, with a seed fixed by the place, in the order Auto material ([[Ground Materials]], no strength slider), Auto trees, Auto snow. Rerunning one reruns the ones after it that are on. They share the Auto tool's terrain analysis (flow, curvature, elevation range), which is redone when the ground changes. The import runs them before handing the world over, and so does `tools/import`.
- **Auto trees** is the Auto tool's forest at 55% coverage, with the treeline from the climate: where the warmest month averages 10 °C. That's about 3,300 m for Boreal, above the whole map, so trees reach the top. Without a climate it's 70% of the way up.
- **Auto snow** is a typical season's snowpack on the start date (`sim.SeasonSnowpack`, `sim/snowpack.go`). It steps day by day from 1 September through the climate's monthly averages, interpolated. Snowfall and the rain split come from the weather sim's numbers. Melt is the sim's degree-day melt with the sun for the date, slope, and aspect, plus rain melt, and the day-to-day temperature spread (about 4 °C) is folded into the degree-days. It's tabled by altitude (25 m bands), slope and aspect, and how much sun the terrain lets through, so every cell is a cheap lookup. Each cell gets that table value for its altitude, slope, aspect, and shade (terrain shading on the start date, from the horizon map), times the Auto tool's drift factor (slope shedding, bowls and gullies, lee faces from the climate's storm wind, noise). The last week's snowfall is fresh powder on top, and the rest is base. Without a climate it falls back to the Auto tool's generator. Changing the start date reruns it.
- **The start date comes from the snow.** An import with Auto snow on and a climate starts on the opening day: the first day flat open ground a quarter of the way up the map holds 0.15 m of water (about half a metre of settled snow), or 15 December if it never does. It's kept in the season the editor was set to.
- Boreal: trees 26 ms, snow 135 ms, after a one-off 0.3 s terrain analysis. The flat-ground pack is 0.16 m of water at the base on 1 January and 0.57 m by 1 April (0.86 m at the top), melted out at the base by May. North faces hold noticeably more snow than south faces in spring. Opens 27 December. The Central Sierra Snow Lab's median 1 April pack is deeper, about 0.9 m, so the climate's precipitation may be on the low side.

## Steps

1. Done: the saved base, the layer stack with the three ground layers, the panel, confirm-and-clear, and reopening the import on the base's square.
2. Done: Auto Trees as a layer that keeps what's built and avoids buildings, lift lines, and roads, with the treeline from the climate.
3. Done: Auto Snow as a layer from the climate, with sun, shade, and drift, setting the start date to the opening day ([[Terrain Realism]] step 3).
4. Done: a strength slider on every row (0–100%, default 50%), saved with the base, rerunning the layer on release. Timings and the helper line are gone.
5. Done: Boreal and Kirkwood re-imported from scratch with every layer (2026-10-06).
6. Later ground layers: creeks and lakes from OpenStreetMap water ([[Creeks and Lakes]]). Cliffs became the Auto material layer instead ([[Ground Materials]]).

## Open questions

- Should Auto snow follow the start date when it changes, or offer to move the date to the opening day again? The same goes for its strength: today a deeper season only opens earlier on import.
- Snowmaking isn't in the snowpack, so the opening day is natural snow only. Real resorts with snowmaking open weeks earlier.
- Should the base also keep the import's climate notes and road fetch time, so the panel can offer "Fetch roads again" when OpenStreetMap was down?

## Log

- 2026-10-05: Planned after trying the import checkboxes on Boreal: each pass looks good, but you can only judge it by re-importing, so they move to a layers panel in the editor.
- 2026-10-06: Decided: layers are never reorderable, each layer decides what it clears (ground layers clear what's built, without diffing the ground), and centimetre heights are precise enough. Built step 1. Boreal: unchecking Smooth ground and Erode brings the cat tracks back in about 5 s, and unchecking Smooth roads brings back the I-80 cut.
- 2026-10-06: Steps 2–3: Auto trees and Auto snow as world layers, run on import and after ground rebuilds. The treeline comes from the climate. The snow is a day-by-day season from the climate with the sim's melt and sun, shaded by the terrain and drifted by the ground. Imports start on the opening day (Boreal: 27 December).
- 2026-10-06: Decided from the user's notes: one strength slider per layer in its row (default 50%, stronger maximums), and no timings or helper line in the panel. Added as step 4 and priority 0.
- 2026-10-06: Step 4: a strength slider in every row, saved with the base. 50% is the old pass; 100% means roads up to twice as wide with a quarter of the bumps, a 7.5 m ground blur, three times the erosion drops carrying twice as much, 95% tree coverage, and 2.5× the snowpack. 0% skips the layer. Timings and the helper line dropped.
- 2026-10-06: Added the OpenStreetMap overlay row. Imports now keep lifts, runs, and the boundary in the base; older imports have roads only until they're imported again.
- 2026-10-06: Auto material joins as the first world layer ([[Ground Materials]]); rerunning a world layer now reruns the ones after it.
- 2026-10-06: Lakes joins as the last ground layer, with no slider: OpenStreetMap lakes levelled to their water surface ([[Creeks and Lakes]]).
- 2026-10-06: Creeks joins as a ground layer, between Erode and Lakes, with no slider ([[Creeks and Lakes]]).
