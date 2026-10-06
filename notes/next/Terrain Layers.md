---
title: Terrain Layers
kind: plan
status: in progress
---

# Terrain Layers

Terrain passes run as layers in a Layers panel in the [[Scenario Editor]] instead of being chosen once on the import screen ([[Terrain Import]]). The panel shows the base import, then each layer on top of it. Unchecking a layer shows the map without it, and clicking the base import goes back to change the square. Listed in [[Next Steps]], part of [[Terrain Realism]].

## The panel

- **Base import** at the top: the square's size and lidar coverage. "Change..." reopens the import screen on the saved square and grid size, keeping which layers are off. Importing again replaces the whole map. It asks first when there's anything on the map, and cancelling the import keeps the old one. The scenario's name, details, and start date carry over. On a drawn map the button is "Import..." instead.
- **One row per layer**, in a fixed order (never reorderable): the ground layers Smooth roads, Smooth ground, Erode, then the world layers Auto trees and Auto snow. Each row has a checkbox, its name, and how long it last took, or why it does nothing here ("needs lidar", "no roads mapped here", "no climate"). Checking or unchecking a ground layer rebuilds the ground in the background from the base through every checked layer, then reruns the world layers that are on. The editor stays usable while it runs. Checking a world layer runs it, and unchecking it clears all trees or all snow.
- A new import starts with every layer on.

## Rules

- **Each layer decides what survives it.** There's nothing to track: a layer does what a player would expect from it. A layer that moves the ground clears every lift, building, and road (snowcats and patrollers go with their buildings), after asking. It doesn't try to check whether the ground under them actually moved. Trails, trees, snow, and parcels stay. Hand edits shouldn't survive a layer that reshapes the ground, just as they wouldn't survive picking a different place to import.
- **Layers that don't change the ground keep things.** Auto Trees and Auto Snow rerun without clearing what's built, then clear trees and snow from under buildings, parking lots, lift lines, and roads as the Auto tool does. They replace all trees or all snow when they run, so painted trees and snow don't survive them. A ground rebuild leaves them alone when they're off.

## How it works

- **The base is saved in the scenario** (`world.TerrainBase`, `terrain_base` in the [[Save Format]]): the surveyed heights on the 1.25 m lattice (or the cells without lidar), stored as whole centimetres. That's about 3.5 MB at Boreal, doubling the file to 7 MB. It also keeps the bounds, the OpenStreetMap roads, the lidar coverage, and the list of switched-off layers, so new layers start on. Player games drop the base on load.
- **Layers** (`geo.Layers`) each change the heights in place, with a fixed seed so results repeat. A `geo.LayerStack` keeps each layer's output, so switching a layer reruns only it and the layers after it. Switching off the last layer is instant. `geo.ApplyHeights` then sets the cells (the lattice averaged over each cell) and the detail from the result.
- If the switches change while a rebuild runs, another starts when it finishes. A rebuild from a different map is dropped.
- Boreal, 512 cells: roads 3.6 s, smoothing 0.08 s, erosion 1.4 s. The first switch after loading a file runs every checked layer once, since outputs aren't saved.
- **World layers** (`scene.worldLayers`, `world_layers.go`) run on the cells after the ground, with a seed fixed by the place. They share the Auto tool's terrain analysis (flow, curvature, elevation range), which is redone when the ground changes. The import runs them before handing the world over, and so does `tools/import`.
- **Auto trees** is the Auto tool's forest at 55% coverage, with the treeline from the climate: where the warmest month averages 10 °C. That's about 3,300 m for Boreal, above the whole map, so trees reach the top. Without a climate it's 70% of the way up.
- **Auto snow** is a typical season's snowpack on the start date (`sim.SeasonSnowpack`, `sim/snowpack.go`). It steps day by day from 1 September through the climate's monthly averages, interpolated. Snowfall and the rain split come from the weather sim's numbers. Melt is the sim's degree-day melt with the sun for the date, slope, and aspect, plus rain melt, and the day-to-day temperature spread (about 4 °C) is folded into the degree-days. It's tabled by altitude (25 m bands), slope and aspect, and how much sun the terrain lets through, so every cell is a cheap lookup. Each cell gets that table value for its altitude, slope, aspect, and shade (terrain shading on the start date, from the horizon map), times the Auto tool's drift factor (slope shedding, bowls and gullies, lee faces from the climate's storm wind, noise). The last week's snowfall is fresh powder on top, and the rest is base. Without a climate it falls back to the Auto tool's generator. Changing the start date reruns it.
- **The start date comes from the snow.** An import with Auto snow on and a climate starts on the opening day: the first day flat open ground a quarter of the way up the map holds 0.15 m of water (about half a metre of settled snow), or 15 December if it never does. It's kept in the season the editor was set to.
- Boreal: trees 26 ms, snow 135 ms, after a one-off 0.3 s terrain analysis. The flat-ground pack is 0.16 m of water at the base on 1 January and 0.57 m by 1 April (0.86 m at the top), melted out at the base by May. North faces hold noticeably more snow than south faces in spring. Opens 27 December. The Central Sierra Snow Lab's median 1 April pack is deeper, about 0.9 m, so the climate's precipitation may be on the low side.

## Steps

1. Done: the saved base, the layer stack with the three ground layers, the panel, confirm-and-clear, and reopening the import on the base's square.
2. Done: Auto Trees as a layer that keeps what's built and avoids buildings, lift lines, and roads, with the treeline from the climate.
3. Done: Auto Snow as a layer from the climate, with sun, shade, and drift, setting the start date to the opening day ([[Terrain Realism]] step 3).
4. Strength sliders, the next priority ([[Next Steps]] item 0):
   - Every row gets a strength slider, 0–100%, default 50%. 50% is today's strength, and 100% pushes each layer as far as it can usefully go. Roads: a wider corridor and a smoother fill. Smooth ground: a larger blur radius. Erode: more drops and deeper cuts. Auto trees: coverage. Auto snow: the snowpack depth.
   - Strengths are saved with the base, like the switches. Releasing a slider reruns that layer and the ones after it, the same as switching it. A ground layer asks first when something is built.
   - Drop the timings beside each row, keeping only the skip notes and "running", and drop the "Uncheck a layer to see without it" line.
5. Re-import Boreal from scratch with every layer.
6. Later ground layers join in order: creeks and lakes from OpenStreetMap water, then cliffs.

## Open questions

- Should Auto snow follow the start date when it changes, or offer to move the date to the opening day again?
- Snowmaking isn't in the snowpack, so the opening day is natural snow only. Real resorts with snowmaking open weeks earlier.
- Should the base also keep the import's climate notes and road fetch time, so the panel can offer "Fetch roads again" when OpenStreetMap was down?

## Log

- 2026-10-05: Planned after trying the import checkboxes on Boreal: each pass looks good, but you can only judge it by re-importing, so they move to a layers panel in the editor.
- 2026-10-06: Decided: layers are never reorderable, each layer decides what it clears (ground layers clear what's built, without diffing the ground), and centimetre heights are precise enough. Built step 1. Boreal: unchecking Smooth ground and Erode brings the cat tracks back in about 5 s, and unchecking Smooth roads brings back the I-80 cut.
- 2026-10-06: Steps 2–3: Auto trees and Auto snow as world layers, run on import and after ground rebuilds. The treeline comes from the climate. The snow is a day-by-day season from the climate with the sim's melt and sun, shaded by the terrain and drifted by the ground. Imports start on the opening day (Boreal: 27 December).
- 2026-10-06: Decided from the user's notes: one strength slider per layer in its row (default 50%, stronger maximums), and no timings or helper line in the panel. Added as step 4 and priority 0.
