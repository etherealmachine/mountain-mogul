---
title: Gridless Drawing
kind: plan
status: idea
---

# Gridless Drawing

Drawing parking lots and trails cell by cell doesn't feel good, and buildings snap to the same 5 m grid. The idea: the player draws shapes (polygons for lots, trails, and building footprints, placed freely and rotated), and those shapes are the source of truth. The grid stays underneath only for what needs it, like a navigation mesh built from the shapes. Lining things up with the real place under the editor's OpenStreetMap overlay ([[Scenario Editor]]) makes the grid's stair-steps more obvious. Listed in [[Next Steps]].

This goes further than [[Hiding the Grid]], which keeps painted cells as the truth and only draws them smoothly.

## What the grid does today

Before deciding, list what each system gets from cells and whether a shape could do the same:

- **[[Trails]]**: a trail is a set of painted cells. The trail graph comes from which cells touch lift ends, buildings, parking, and other trails. [[Skiing]] picks a line inside the painted area, and leaving it costs beginners more.
- **[[Parking and Roads]]**: a lot is painted cells, and stalls and aisles are laid out from them automatically. Roads are already a graph, not cells.
- **[[Pathfinding]]**: A* over the cell grid. Building cells, lift ends, and cells with two or more trunks block the way.
- **[[Terrain]]**: lift stations and painted pads level the ground beneath them, with an embankment around them.
- **Per-cell state**: [[Snow]], [[Grooming]], [[Trees]], and [[Parcels]] are all stored per cell, and the sim reads them per cell.

## Possible shape

- Lots, trails, and buildings are polygons (or a centre line with a width, for trails) in world metres, saved as shapes.
- Each shape is rasterised to the cells it covers whenever it changes, so the sim, the trail graph, and pathfinding keep reading cells as they do now. Rendering draws the shape itself.
- Optionally, walking moves from A* on cells to a navigation mesh built from the shapes, so routes curve around buildings instead of stepping.

## Open questions

- What does the grid actually buy us, system by system? Some uses (snow, grooming, trees) are really a sampling grid and could stay. Others (trails, lots, building footprints) are things the player draws, where the grid is only an input constraint.
- Trails: a polygon, or a centre line with a width the player can widen? How does painting additions onto an existing trail work?
- Parking: how do stalls and aisles lay out inside any polygon, compared with the rectangles of cells they fill today?
- Buildings: free placement and rotation (rotation partly exists), and how a footprint maps to the levelled pad and to the cells that block walking.
- Navigation: keep A* on cells built from the shapes, or build a real navigation mesh? What would routes around buildings and lift lines ([[Lifts]]) gain?
- Saves: shapes replace painted cells in the [[Save Format]]. Pre-release, older saves can break.

## Log

- 2026-10-06: Raised after drawing parking lots and trails next to the OpenStreetMap overlay: cell painting feels poor. Idea only; nothing decided.
- 2026-10-06: Lots became rectangles ([[Transit]] step 3). For buildings the user chose a rotated 5 m grid with tile painting over free polygons: see [[Rotated Buildings]].
