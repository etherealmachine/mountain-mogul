---
title: Creeks and Lakes
kind: plan
status: planned
---

# Creeks and Lakes

Water at [[Kirkwood]]: the creeks through the meadow and the valley floor, and lakes, Caples Lake among them if it's on the map. Creeks matter as the shape of the ground (beds, banks, snow bridges), not as rendered water; lakes are frozen, snow-covered flats in winter. OpenStreetMap says where the real ones are. Step 5 of [[Terrain Realism]], listed in [[Next Steps]].

## What we know

- The import already fetches OpenStreetMap roads, lifts, runs, and the ski-area boundary ([[Terrain Import]]) and keeps them in the scenario's base, where the editor's OpenStreetMap overlay draws them ([[Scenario Editor]]). Water would come the same way: lakes (`natural=water`) and streams and rivers (`waterway=*`).
- Kirkwood has 1.25 m lidar, which may already show the creek beds; the cliffs turned out to be there all along, buried under snow ([[Ground Materials]]). Lidar over open water is often missing or noisy, so a lake's surface may need flattening.
- Terrain passes run as layers in the editor's Layers panel ([[Terrain Layers]]); a creek or lake pass would be a ground layer there (Terrain Layers step 6). The road layer's corridor-and-fill code can carve or flatten along a line or inside a polygon.
- The material map ([[Ground Materials]]) can mark water and creek beds, so trees stay off them and the renderer can draw ice, gravel, or open water where snow is thin.
- The forest generator already computes flow accumulation, which could trace creeks OpenStreetMap doesn't have.

## Steps

1. Look first: fetch Kirkwood's OpenStreetMap water, draw it in the OpenStreetMap overlay, and compare it with the lidar and the snow as they are. Report whether the beds are already in the ground, whether Caples Lake is on the map, and what actually looks wrong.
2. Keep the water with the base: fetch it at import, save it in the scenario, and draw it in the overlay.
3. The rest follows from step 1. Likely: a Creeks and lakes ground layer that cuts or cleans up beds along the waterways and flattens lakes to their shore level; water and creek-bed materials; and snow that bridges small creeks, leaves bigger ones open, and lies flat on frozen lakes.

## Open questions

- Is it a shape problem (beds missing or smoothed away by Smooth ground and Erode) or a look problem (creeks buried under even snow)?
- Do guests and pathing need to know about creeks (crossings, bridges), or is the ground's slope enough?
- Where should creeks come from where OpenStreetMap has none: flow accumulation, or nowhere?

## Log

- 2026-10-06: Planned as priority 0 after Ground Materials steps 1–4, from Terrain Realism step 5 (creeks and lakes, OpenStreetMap water).
