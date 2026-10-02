---
title: Trees
kind: system
status: shipped
---

# Trees

Every tree is stored as its own world XZ position, saved with the [[Terrain]]. Brushes, generators, and gameplay all work on those trees, so what the player sees is what skiers, walkers, and avalanches react to. Built in [[Stored Trees]].

## Storage

Trees sit in per-cell buckets on the `Terrain` (`internal/world/trees.go`), so brushes and skier queries touch only nearby cells. Each cell's `TreeCount` mirrors its bucket; it only changes through the `trees.go` methods (`AddTree`, `RemoveTreesIn`, `ClearTreesInCell`, and so on), which keeps the two in step.

A tree's rotation, scale, and model variant come from a hash of its position rounded to the centimetre (`TreeInstanceOf`). Nothing beyond the position is stored, and a tree looks the same after a reload. New trees keep 1.8 m from their neighbours (`TreeMinSpacing`).

Saves hold the trees as a flat list of XZ pairs. Older saves carry a per-cell `TreeDensity` instead, which converts on load by running the old density rule once (count from density and a per-cell hash, offset up to ±1.2 m from the cell centre), so a converted forest keeps exactly the same trunk positions. Old lone decorative trees in `World.Objects` fold into the store at their cell centre. See [[Save Format]].

## Tree cover

Some readers want a per-cell summary rather than trunks. `TreeCover()` is the cell's tree count over 2 (`MaxTreesPerCell`), capped at 1: one tree gives 0.5, two or more give 1. It stands in everywhere the old density was read.

## What reads trees

- [[Skiing]] steering scores the larger of the cell's cover and the distance to the nearest trunk within 3 m, so a lone tree is avoided even at the edge of its cell. A skier who comes within 0.6 m of a trunk while heading at it faster than 2 m/s hits it: a fall, the thought "I hit a tree!", and an injury chance that grows with speed up to 60%. Balance drains in cells with any tree, and glade-loving or tree-shy guests react to cover ([[Satisfaction]]).
- [[Pathfinding]]: a cell with two or more trunks can't be walked.
- [[Avalanche]]: cover anchors a slope against release (cover × 0.4), and a front stops in a cell with two or more trees.
- [[Rendering]]: the static forest batch and the tree wells in the snow detail texture read the stored trees.

## What writes trees

- **Glade tool** (play). The brush takes a share of the trees within its ring, set by the Thin slider (5–100%, default 25%). Trees are picked by a hash of their position, so the choice is stable. The trees about to go are highlighted in red before the click, the cost label shows $100 per tree, and the click removes exactly those trees.
- **Editor plant brush.** Adds at most one spaced tree per cell per stroke, up to a slider-set target count.
- **Editor glade brush.** Removes the same hashed selection at a fixed 40% share. There is no highlight in the editor.
- **Automatic forest generator** ([[Scenario Editor]]). Keeps its density field and turns each cell's density into a count of trees placed at random spacing-checked spots in the cell, so the forest has no grid pattern.
- **Clearing.** Lift corridors and station aprons remove trees by trunk distance, roads remove trees within reach of the centreline, and parking lots and [[Lodge Shell]] footprints clear whole cells.

## Log

- 2026-10-01: Documented the density model and the glade-tool problem. The fix is planned in [[Stored Trees]].
- 2026-10-02: Trees are now stored individually and gameplay reads them. The glade tool previews and prices exactly the trees it removes, and skiers can hit trees.
